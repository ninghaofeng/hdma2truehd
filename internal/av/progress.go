package av

import "context"

// Progress reports processed PCM time. Duration is an estimate from container
// metadata, not an input to lossless verification. Zero means unknown.
type Progress struct {
	Samples  uint64
	Rate     int
	Duration float64
	// Done completes this native operation, not necessarily its audio stage.
	Done bool
}

type progressKey struct{}

// WithProgress scopes synchronous callbacks to a single native operation.
func WithProgress(ctx context.Context, report func(Progress)) context.Context {
	return context.WithValue(ctx, progressKey{}, report)
}
