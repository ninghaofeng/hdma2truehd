package audio

import (
	"context"
	"time"

	"github.com/ninghaofeng/hdma2truehd/internal/av"
)

type Progress struct {
	Stage    string  `json:"stage"`
	Seconds  float64 `json:"seconds"`
	Duration float64 `json:"duration"`
	Fraction float64 `json:"fraction"`
	Elapsed  float64 `json:"elapsed"`
	Done     bool    `json:"done"`
}

type reporter struct {
	report  func(Progress)
	started time.Time
	current Progress
}

func (r *reporter) stage(ctx context.Context, stage string) context.Context {
	r.started = time.Now()
	r.current = Progress{Stage: stage}
	r.send()
	return av.WithProgress(ctx, func(p av.Progress) {
		if p.Rate > 0 {
			r.current.Seconds = float64(p.Samples) / float64(p.Rate)
		}
		r.current.Duration = p.Duration
		r.current.Fraction = 0
		if p.Duration > 0 {
			// Metadata may underestimate length; reserve 100% for actual completion.
			r.current.Fraction = min(r.current.Seconds/p.Duration, .99)
		}
		r.send()
	})
}

func (r *reporter) send() {
	r.current.Elapsed = time.Since(r.started).Seconds()
	if r.report != nil {
		r.report(r.current)
	}
}

func (r *reporter) finish() {
	r.current.Done = true
	r.current.Fraction = 1
	r.send()
}
