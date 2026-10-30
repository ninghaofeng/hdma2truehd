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
	lossy := strings.HasSuffix(out, ".eac3") || strings.HasSuffix(out, ".ac3")
	return audio.Result{Output: out, Verified: strings.HasSuffix(out, ".thd"), Validated: lossy, Lossy: lossy, Samples: 1536, DecodedSamples: 3072}, nil
}

func TestSourceFormatScopes(t *testing.T) {
	dir := t.TempDir()
	for _, tt := range []struct {
		name, profile, layout string
		rate, channels        int
		truehd, eac3, ac3     bool
	}{
		{"MA51", "DTS-HD MA", "5.1(side)", 48000, 6, true, true, true},
		{"MA71", "DTS-HD MA", "7.1", 48000, 8, true, true, false},
		{"MARear", "DTS-HD MA", "5.1", 48000, 6, true, false, false},
		{"MA96", "DTS-HD MA", "5.1(side)", 96000, 6, true, false, false},
		{"DTS51", "DTS", "5.1(side)", 48000, 6, false, true, true},
		{"DTS71", "DTS", "7.1", 48000, 8, false, false, false},
		{"DTSRear", "DTS", "5.1", 48000, 6, false, false, false},
		{"DTS96", "DTS", "5.1(side)", 96000, 6, false, false, false},
		{"HRA", "DTS-HD HRA", "5.1(side)", 48000, 6, false, false, false},
		{"Unknown", "", "5.1(side)", 48000, 6, false, false, false},
		{"Case", "dts", "5.1(side)", 48000, 6, false, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			info := audio.Info{Path: filepath.Join(dir, tt.name+".dts"), Rate: tt.rate, Bits: 24,
				Stream: audio.Stream{Codec: "dts", Profile: tt.profile, Layout: tt.layout, Channels: tt.channels}}
			if tt.profile == "DTS" {
				info.Bits = 32
			}
			s := New(fakeBackend{tracks: []audio.Track{{Info: info}}, probe: func(string, int) (audio.Info, error) { return info, nil }}, nil)
			inputs, err := s.Inspect(t.Context(), []string{info.Path})
			if err != nil {
				t.Fatal(err)
			}
			track := inputs[0].Tracks[0]
			if (track.Problem == "") != (tt.truehd || tt.eac3 || tt.ac3) ||
				(track.TrueHDProblem == "") != tt.truehd || (track.EAC3Problem == "") != tt.eac3 || (track.AC3Problem == "") != tt.ac3 {
				t.Fatalf("incorrect inventory: %+v", track)
			}
			for _, format := range []struct {
				name    string
				allowed bool
			}{{"thd", tt.truehd}, {"eac3", tt.eac3}, {"ac3", tt.ac3}} {
				req := Request{Tracks: []Selection{{info.Path, 0}}, TrueHD: format.name == "thd", EAC3: format.name == "eac3", AC3: format.name == "ac3"}
				if req.EAC3 {
					req.Bitrate51, req.Bitrate71 = 640, 1280
				}
				jobs, err := s.plan(t.Context(), req)
				if (err == nil) != format.allowed {
					t.Fatalf("%s: jobs=%+v err=%v", format.name, jobs, err)
				}
				if err == nil && (len(jobs) != 1 || jobs[0].Format != format.name) {
					t.Fatalf("%s: %+v", format.name, jobs)
				}
				if err == nil && format.name == "eac3" && tt.profile == "DTS" && jobs[0].Bitrate != 1024 {
					t.Fatalf("ordinary DTS bitrate not fixed: %+v", jobs)
				}
				if err == nil && format.name != "eac3" && jobs[0].Bitrate != 0 {
					t.Fatalf("E-AC-3 bitrate leaked: %+v", jobs)
				}
			}
			if tt.profile == "DTS" && tt.eac3 {
				if _, err := s.plan(t.Context(), Request{Tracks: []Selection{{info.Path, 0}}, TrueHD: true, EAC3: true, AC3: true}); err == nil {
					t.Fatal("silently skipped ordinary DTS TrueHD output")
				}
			}
		})
	}
}

func waitQueue(t *testing.T, s *Service) State {
	t.Helper()
	// Wait for the worker's done signal, not a sampled Running flag. The
	// timeout detects deadlocks; it is not how the queue synchronizes cleanup.
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
	// Mixed layouts share a queue, not a bitrate. Zero stays automatic until
	// the audio backend resolves it; TrueHD must always receive zero.
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

	if !strings.HasSuffix(jobs[0].Output, "movie_track2_[und]_DELAY 0ms.thd") || !strings.HasSuffix(jobs[3].Output, "movie_track4_[und]_DELAY 0ms.eac3") {
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

func TestMKVOutputMetadata(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "test_video.mkv")
	s := New(fakeBackend{probe: func(path string, index int) (audio.Info, error) {
		info, err := (fakeBackend{}).ProbeStream(t.Context(), path, index)
		info.Stream.Layout, info.Stream.Channels = "5.1(side)", 6
		info.Language, info.DelayMS = "eng", -125
		return info, err
	}}, nil)
	req := Request{Tracks: []Selection{{input, 2}}, TrueHD: true, EAC3: true, AC3: true}
	jobs, err := s.plan(t.Context(), req)
	if err != nil || len(jobs) != 3 {
		t.Fatalf("%+v %v", jobs, err)
	}
	for i, extension := range []string{".thd", ".eac3", ".ac3"} {
		if jobs[i].Output != filepath.Join(dir, "test_video_track2_[eng]_DELAY -125ms"+extension) {
			t.Fatalf("%+v", jobs)
		}
	}
	outputDir := t.TempDir()
	req.OutputDir = outputDir
	jobs, err = s.plan(t.Context(), req)
	if err != nil || filepath.Dir(jobs[0].Output) != outputDir {
		t.Fatalf("%+v %v", jobs, err)
	}
}

func TestProgressSnapshots(t *testing.T) {
	dir := t.TempDir()
	var snapshots []State
	s := New(fakeBackend{convert: func(_ context.Context, _, output string, options audio.Options) (audio.Result, error) {
		if options.Progress == nil {
			t.Error("no progress callback")
			return audio.Result{}, errors.New("missing progress")
		}
		options.Progress(audio.Progress{Stage: "encode", Seconds: 12, Duration: 60, Fraction: .2})
		options.Progress(audio.Progress{Stage: "verify_source", Seconds: 5, Duration: 60, Fraction: 5.0 / 60})
		return audio.Result{Output: output, Verified: true, Samples: 17}, nil
	}}, func(state State) { snapshots = append(snapshots, state) })
	if _, err := s.Start(t.Context(), Request{Tracks: []Selection{{filepath.Join(dir, "source.dts"), 0}}, TrueHD: true}); err != nil {
		t.Fatal(err)
	}
	waitQueue(t, s)
	found := false
	for _, snapshot := range snapshots {
		job := snapshot.Jobs[0]
		if job.Status == "running" && job.Progress.Stage == "encode" && job.Progress.Seconds == 12 && job.Progress.Fraction == .2 {
			found = true
		}
	}
	if !found {
		t.Fatal("in-flight progress never emitted")
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
	revisions := map[uint64]bool{}
	for _, snapshot := range seen {
		if revisions[snapshot.Revision] || snapshot.Revision > state.Revision {
			t.Fatal("duplicate or invalid queue revision")
		}
		revisions[snapshot.Revision] = true
	}
	if !revisions[state.Revision] {
		t.Fatal("final queue snapshot not emitted")
	}
}

func TestCancellationAndShutdown(t *testing.T) {
	// Handshake after Convert starts so cancellation must unwind active work,
	// rather than passing only because the worker never entered the backend.
	started := make(chan struct{})
	backend := fakeBackend{convert: func(ctx context.Context, _ string, _ string, _ audio.Options) (audio.Result, error) {
		close(started)
		<-ctx.Done()
		return audio.Result{}, ctx.Err()
	}}
	s := New(backend, nil)
	req := Request{Tracks: []Selection{{filepath.Join(t.TempDir(), "source.dts"), 0}}, TrueHD: true, EAC3: true, Concurrency: 1}
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
	trueHDOnly := supported
	trueHDOnly.Rate = 96000
	s := New(fakeBackend{tracks: []audio.Track{{Info: supported, Language: "eng", Title: "Original"}, {Info: unsupported}, {Info: trueHDOnly}}}, nil)
	inputs, err := s.Inspect(t.Context(), []string{supported.Path, supported.Path, filepath.Join(t.TempDir(), "wrong.txt")})
	if err != nil || len(inputs) != 2 {
		t.Fatalf("%+v %v", inputs, err)
	}
	if len(inputs[0].Tracks) != 3 || inputs[0].Tracks[0].Problem != "" || inputs[0].Tracks[1].Problem == "" || inputs[1].Error == "" {
		t.Fatalf("%+v", inputs)
	}
	if inputs[0].Tracks[1].ProblemCode != "codec" || inputs[0].Tracks[2].ProblemCode != "" ||
		inputs[0].Tracks[2].EAC3ProblemCode != "eac3_format" {
		t.Fatalf("incorrect format-specific rejection codes: %+v", inputs[0].Tracks)
	}
}
