// Package batch shares bounded scheduling between CLI and desktop callers.
// It owns job claiming, not job results or conversion failure policy.
package batch

import (
	"context"
	"fmt"
	"sync"
)

const (
	DefaultConcurrency = 2
	MaxConcurrency     = 8
)

func ValidateConcurrency(n int) error {
	if n < 1 || n > MaxConcurrency {
		return fmt.Errorf("concurrency must be between 1 and %d", MaxConcurrency)
	}
	return nil
}

// Run bounds active calls, stops claiming jobs on cancellation, and waits for
// all active calls to finish cleanup. Individual failures belong to work.
// A nil return means scheduling finished, not that jobs succeeded or ctx is live.
func Run(ctx context.Context, count, concurrency int, work func(int)) error {
	if err := ValidateConcurrency(concurrency); err != nil {
		return err
	}
	if count < 0 {
		return fmt.Errorf("job count must not be negative")
	}
	var mu sync.Mutex
	next := 0
	var workers sync.WaitGroup
	for range min(concurrency, count) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				mu.Lock()
				// Claim indices once and in order, without holding the scheduler
				// lock during work. Execution/completion order is not guaranteed.
				if ctx.Err() != nil || next == count {
					mu.Unlock()
					return
				}
				index := next
				next++
				mu.Unlock()
				work(index)
			}
		}()
	}
	workers.Wait()
	return nil
}
