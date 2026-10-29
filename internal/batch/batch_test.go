package batch

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func wait(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("parallel work did not finish")
	}
}

func TestConcurrency(t *testing.T) {
	for _, concurrency := range []int{1, 2, MaxConcurrency} {
		t.Run(fmt.Sprint(concurrency), func(t *testing.T) {
			started := make(chan struct{}, 12)
			release := make(chan struct{})
			var mu sync.Mutex
			active, peak := 0, 0
			seen := make(map[int]bool)
			done := make(chan struct{})
			go func() {
				defer close(done)
				err := Run(t.Context(), 12, concurrency, func(i int) {
					mu.Lock()
					active++
					peak = max(peak, active)
					if seen[i] {
						t.Errorf("job %d dispatched twice", i)
					}
					seen[i] = true
					mu.Unlock()
					started <- struct{}{}
					<-release
					mu.Lock()
					active--
					mu.Unlock()
				})
				if err != nil {
					t.Error(err)
				}
			}()
			for range concurrency {
				wait(t, started)
			}
			close(release)
			wait(t, done)
			if peak != concurrency || active != 0 || len(seen) != 12 {
				t.Fatalf("peak=%d active=%d jobs=%d", peak, active, len(seen))
			}
		})
	}
}

func TestCancellationWaitsForCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{}, 2)
	cleanup := make(chan struct{})
	done := make(chan struct{})
	calls := 0
	var mu sync.Mutex
	go func() {
		defer close(done)
		if err := Run(ctx, 10, 2, func(int) {
			mu.Lock()
			calls++
			mu.Unlock()
			started <- struct{}{}
			<-ctx.Done()
			<-cleanup
		}); err != nil {
			t.Error(err)
		}
	}()
	wait(t, started)
	wait(t, started)
	cancel()
	select {
	case <-done:
		t.Fatal("returned before cleanup")
	default:
	}
	close(cleanup)
	wait(t, done)
	if calls != 2 {
		t.Fatalf("started queued work after cancellation: %d calls", calls)
	}
}

func TestInvalidAndEmpty(t *testing.T) {
	for _, n := range []int{-1, 0, MaxConcurrency + 1} {
		if err := Run(t.Context(), 1, n, func(int) { t.Error("invalid queue ran") }); err == nil {
			t.Errorf("accepted concurrency %d", n)
		}
	}
	if err := Run(t.Context(), -1, 2, func(int) { t.Error("invalid count ran") }); err == nil {
		t.Fatal("accepted negative job count")
	}
	if err := Run(t.Context(), 0, 2, func(int) { t.Error("empty queue ran") }); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := Run(ctx, 10, 2, func(int) { t.Error("cancelled queue ran") }); err != nil {
		t.Fatal(err)
	}
}
