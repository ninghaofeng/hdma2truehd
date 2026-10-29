package audio

import (
	"context"
	"time"

	"github.com/ninghaofeng/hdma2truehd/internal/diagnostic"
)

// retryOwnedKey marks contexts whose outer conversion owns the retry budget;
// standalone metadata inspection has its own single retry instead.
type retryOwnedKey struct{}

func retryPause(ctx context.Context) error {
	timer := time.NewTimer(300 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func probeWithRetry(ctx context.Context, path string, probe func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Convert retries the entire transaction, not each nested probe separately.
	if owned, _ := ctx.Value(retryOwnedKey{}).(bool); owned {
		return probe(ctx)
	}
	for attempt := 1; ; attempt++ {
		work := diagnostic.WithStage(diagnostic.WithJob(ctx, path, "", attempt), "probe")
		err := diagnostic.Wrap("probe", probe(work))
		if err == nil {
			return nil
		}
		problem := diagnostic.Describe(err)
		diagnostic.Emit(work, diagnostic.Event{Level: "error", Code: problem.Code, Message: problem.Detail})
		if attempt == 2 || !problem.Retryable || ctx.Err() != nil {
			return err
		}
		diagnostic.Emit(work, diagnostic.Event{Level: "warning", Code: "retrying", Message: "Temporary I/O failure; retrying metadata inspection once"})
		if err := retryPause(ctx); err != nil {
			return err
		}
	}
}
