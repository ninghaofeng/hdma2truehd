package gui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ninghaofeng/hdma2truehd/internal/audio"
)

func waitParallel(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("parallel queue did not respond")
	}
}

func TestParallelQueue(t *testing.T) {
	for _, concurrency := range []int{0, 1, 3, 8} {
		t.Run(fmt.Sprint(concurrency), func(t *testing.T) {
			limit := concurrency
			if limit == 0 {
				limit = 2
			}
			started := make(chan struct{}, 8)
			release := make(chan struct{})
			var releaseOnce sync.Once
			releaseWork := func() { releaseOnce.Do(func() { close(release) }) }
			defer releaseWork()
			backend := fakeBackend{convert: func(_ context.Context, _, output string, options audio.Options) (audio.Result, error) {
				options.Progress(audio.Progress{Stage: "encode", Seconds: float64(*options.StreamIndex + 1)})
				started <- struct{}{}
				<-release
				if strings.HasSuffix(output, ".thd") {
					return audio.Result{}, errors.New("deliberate encode failure")
				}
				return audio.Result{Output: output, Lossy: true, Validated: true, Samples: 17}, nil
			}}
			s := New(backend, nil)
			path := filepath.Join(t.TempDir(), "same.mkv")
			req := Request{TrueHD: true, EAC3: true, Concurrency: concurrency}
			for i := range 4 {
				req.Tracks = append(req.Tracks, Selection{Path: path, Index: i})
			}
			if _, err := s.Start(t.Context(), req); err != nil {
				t.Fatal(err)
			}
			for range limit {
				waitParallel(t, started)
			}
			state := s.State()
			running := 0
			for _, job := range state.Jobs {
				if job.Status == "running" {
					running++
					if job.Progress.Seconds != float64(job.Index+1) {
						t.Fatalf("crossed job progress: %+v", job)
					}
				}
			}
			if running != limit || !state.Running {
				t.Fatalf("expected %d running jobs: %+v", limit, state)
			}
			releaseWork()
			state = waitQueue(t, s)
			for _, job := range state.Jobs {
				want := "validated"
				if job.Format == "thd" {
					want = "failed"
				}
				if job.Status != want {
					t.Fatalf("failure affected another task: %+v", job)
				}
			}
		})
	}
}

func TestParallelCancellation(t *testing.T) {
	started := make(chan struct{}, 2)
	cleanup := make(chan struct{})
	var cleanupOnce sync.Once
	finishCleanup := func() { cleanupOnce.Do(func() { close(cleanup) }) }
	defer finishCleanup()
	var calls atomic.Int32
	s := New(fakeBackend{convert: func(ctx context.Context, _, _ string, _ audio.Options) (audio.Result, error) {
		calls.Add(1)
		started <- struct{}{}
		<-ctx.Done()
		<-cleanup
		return audio.Result{}, ctx.Err()
	}}, nil)
	path := filepath.Join(t.TempDir(), "same.mkv")
	if _, err := s.Start(t.Context(), Request{Tracks: []Selection{{path, 0}, {path, 1}}, TrueHD: true, EAC3: true, Concurrency: 2}); err != nil {
		t.Fatal(err)
	}
	waitParallel(t, started)
	waitParallel(t, started)
	s.Cancel()
	stopped := make(chan struct{})
	go func() { s.Shutdown(); close(stopped) }()
	if !s.State().Running {
		t.Fatal("queue completed before active cleanup")
	}
	finishCleanup()
	waitParallel(t, stopped)
	state := s.State()
	if state.Running || calls.Load() != 2 {
		t.Fatalf("queue dispatched after cancellation: calls=%d state=%+v", calls.Load(), state)
	}
	for _, job := range state.Jobs {
		if job.Status != "cancelled" {
			t.Fatalf("job not cancelled: %+v", job)
		}
	}
}

func TestInvalidConcurrency(t *testing.T) {
	s := New(fakeBackend{}, nil)
	for _, n := range []int{-1, 9} {
		if _, err := s.Start(t.Context(), Request{Concurrency: n}); err == nil || !strings.Contains(err.Error(), "concurrency") {
			t.Fatalf("accepted invalid concurrency %d: %v", n, err)
		}
	}
}
