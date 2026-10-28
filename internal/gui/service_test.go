package gui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ninghaofeng/hdma2truehd/internal/audio"
)

type fakeBackend struct {
	probe   func(string, int) (audio.Info, error)
	convert func(context.Context, string, string, audio.Options) (audio.Result, error)
	tracks  []audio.Track
}

func (b fakeBackend) ProbeTracks(context.Context, string) ([]audio.Track, error) {
	return b.tracks, nil
}
func (b fakeBackend) ProbeStream(_ context.Context, path string, index int) (audio.Info, error) {
	if b.probe != nil {
		return b.probe(path, index)
	}
	absolute, err := filepath.Abs(path)
	return audio.Info{Path: absolute, Rate: 48000, Bits: 24, Stream: audio.Stream{Index: index, Codec: "dts", Profile: "DTS-HD MA", Layout: "7.1", Channels: 8}}, err
}
func (b fakeBackend) Convert(ctx context.Context, in, out string, options audio.Options) (audio.Result, error) {
	if b.convert != nil {
		return b.convert(ctx, in, out, options)
	}
	return audio.Result{Output: out, Verified: strings.HasSuffix(out, ".thd"), Validated: strings.HasSuffix(out, ".eac3"), Lossy: strings.HasSuffix(out, ".eac3"), Samples: 1536, DecodedSamples: 3072}, nil
}

func waitQueue(t *testing.T, s *Service) State {
	t.Helper()
	done := make(chan struct{})
	go func() {
		s.mu.Lock()
		ch := s.done
		s.mu.Unlock()
		if ch != nil {
			<-ch
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("queue did not finish")
	}
	return s.State()
}

func TestLayoutBitrates(t *testing.T) {
	for _, rates := range [][2]int{{640, 1280}, {0, 0}, {0, 1280}, {640, 0}, {960, 1536}} {
		dir := t.TempDir()
		base := fakeBackend{}
		backend := fakeBackend{
			probe: func(path string, index int) (audio.Info, error) {
				info, err := base.ProbeStream(t.Context(), path, index)
				if strings.Contains(filepath.Base(path), "51") {
					info.Stream.Layout, info.Stream.Channels = "5.1(side)", 6
				}
				return info, err
			},
			convert: func(ctx context.Context, in, out string, options audio.Options) (audio.Result, error) {
				expected := 0
				if strings.HasSuffix(out, ".eac3") {
					expected = rates[1]
					if strings.Contains(filepath.Base(in), "51") {
						expected = rates[0]
					}
				}
				if options.EAC3Bitrate != expected {
					t.Errorf("%s: bitrate %d, want %d", out, options.EAC3Bitrate, expected)
				}
				return base.Convert(ctx, in, out, options)
			},
		}
		s := New(backend, nil)
		req := Request{Tracks: []Selection{{filepath.Join(dir, "51.dts"), 0}, {filepath.Join(dir, "71.dts"), 0}},
			TrueHD: true, EAC3: true, Bitrate51: rates[0], Bitrate71: rates[1]}
		state, err := s.Start(t.Context(), req)
		if err != nil {
			t.Fatal(err)
		}
		if len(state.Jobs) != 4 || state.Jobs[0].Bitrate != 0 || state.Jobs[1].Bitrate != rates[0] ||
			state.Jobs[2].Bitrate != 0 || state.Jobs[3].Bitrate != rates[1] {
			t.Fatalf("%+v", state.Jobs)
		}
		state = waitQueue(t, s)
		for _, job := range state.Jobs {
			if job.Status != "verified" && job.Status != "validated" {
				t.Fatalf("%+v", job)
			}
		}
		// Reject each layout's invalid rate before any output job can start.
		for _, bad := range [][2]int{{1280, 1536}, {960, 640}, {-1, 1536}, {960, 1537}} {
			req.Bitrate51, req.Bitrate71 = bad[0], bad[1]
			if _, err := s.plan(t.Context(), req); err == nil {
				t.Fatalf("accepted bitrates %v", bad)
			}
		}
	}
}

func TestPlanning(t *testing.T) {
	dir := t.TempDir()
	s := New(fakeBackend{}, nil)
	request := Request{Tracks: []Selection{{filepath.Join(dir, "movie.mkv"), 2}, {filepath.Join(dir, "movie.mkv"), 4}}, TrueHD: true, EAC3: true}
	jobs, err := s.plan(t.Context(), request)
	if err != nil || len(jobs) != 4 {
		t.Fatalf("%+v %v", jobs, err)
	}
	if !strings.HasSuffix(jobs[0].Output, "movie.track-2.thd") || !strings.HasSuffix(jobs[3].Output, "movie.track-4.eac3") {
		t.Fatalf("%+v", jobs)
	}
	for _, bad := range []Request{
		{},
		{Tracks: request.Tracks, TrueHD: true, Bitrate51: 960},
		{Tracks: request.Tracks, TrueHD: true, Bitrate71: 1536},
		{Tracks: request.Tracks, EAC3: true, Bitrate71: 640},
		{Tracks: []Selection{request.Tracks[0], request.Tracks[0]}, TrueHD: true},
		{Tracks: request.Tracks, TrueHD: true, OutputDir: filepath.Join(dir, "missing")},
		{Tracks: []Selection{{filepath.Join(dir, "a.txt"), 0}}, TrueHD: true},
	} {
		if _, err := s.plan(t.Context(), bad); err == nil {
			t.Fatalf("accepted invalid request: %+v", bad)
		}
	}
	if err := os.WriteFile(jobs[0].Output, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.plan(t.Context(), request); err == nil {
		t.Fatal("accepted existing output")
	}
	request.Overwrite = true
	if _, err := s.plan(t.Context(), request); err != nil {
		t.Fatal(err)
	}
}

func TestQueueContinuesAndValidates(t *testing.T) {
	var mu sync.Mutex
	seen := []State{}
	backend := fakeBackend{convert: func(_ context.Context, _ string, out string, options audio.Options) (audio.Result, error) {
		if options.SkipVerify || options.StreamIndex == nil || *options.StreamIndex != 3 {
			t.Errorf("wrong options: %+v", options)
		}
		if strings.HasSuffix(out, ".thd") {
			return audio.Result{}, errors.New("encode failed")
		}
		return audio.Result{Output: out, Lossy: true, Validated: true, Samples: 100, DecodedSamples: 1536}, nil
	}}
	s := New(backend, func(state State) { mu.Lock(); seen = append(seen, state); mu.Unlock() })
	_, err := s.Start(t.Context(), Request{Tracks: []Selection{{filepath.Join(t.TempDir(), "source.mkv"), 3}}, TrueHD: true, EAC3: true})
	if err != nil {
		t.Fatal(err)
	}
	state := waitQueue(t, s)
	if state.Running || state.Jobs[0].Status != "failed" || state.Jobs[1].Status != "validated" {
		t.Fatalf("%+v", state)
	}
	mu.Lock()
	defer mu.Unlock()
	for i := 1; i < len(seen); i++ {
		if seen[i].Revision <= seen[i-1].Revision {
			t.Fatal("non-monotonic queue revision")
		}
	}
}

func TestCancellationAndShutdown(t *testing.T) {
	started := make(chan struct{})
	backend := fakeBackend{convert: func(ctx context.Context, _ string, _ string, _ audio.Options) (audio.Result, error) {
		close(started)
		<-ctx.Done()
		return audio.Result{}, ctx.Err()
	}}
	s := New(backend, nil)
	req := Request{Tracks: []Selection{{filepath.Join(t.TempDir(), "source.dts"), 0}}, TrueHD: true, EAC3: true}
	if _, err := s.Start(t.Context(), req); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("queue did not start")
	}
	if _, err := s.Start(t.Context(), req); err == nil {
		t.Fatal("accepted concurrent start")
	}
	s.Cancel()
	s.Shutdown()
	state := waitQueue(t, s)
	if state.Running || state.Jobs[0].Status != "cancelled" || state.Jobs[1].Status != "cancelled" {
		t.Fatalf("%+v", state)
	}
}

func TestInspectUnsupportedTracks(t *testing.T) {
	supported, _ := (fakeBackend{}).ProbeStream(t.Context(), filepath.Join(t.TempDir(), "movie.mkv"), 1)
	unsupported := supported
	unsupported.Stream.Codec = "aac"
	s := New(fakeBackend{tracks: []audio.Track{{Info: supported, Language: "eng", Title: "Original"}, {Info: unsupported}}}, nil)
	inputs, err := s.Inspect(t.Context(), []string{supported.Path, supported.Path, filepath.Join(t.TempDir(), "wrong.txt")})
	if err != nil || len(inputs) != 2 {
		t.Fatalf("%+v %v", inputs, err)
	}
	if len(inputs[0].Tracks) != 2 || inputs[0].Tracks[0].Problem != "" || inputs[0].Tracks[1].Problem == "" || inputs[1].Error == "" {
		t.Fatalf("%+v", inputs)
	}
}
