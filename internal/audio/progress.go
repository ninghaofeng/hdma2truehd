package audio

import (
	"context"
	"fmt"
	"time"

	"github.com/ninghaofeng/hdma2truehd/internal/av"
	"github.com/ninghaofeng/hdma2truehd/internal/diagnostic"
)

// Progress is a snapshot of one stage, not a weighted whole-job percentage.
// Fraction is an estimate; Done is set only when the audio layer finishes it.
type Progress struct {
	Stage    string  `json:"stage"`
	Seconds  float64 `json:"seconds"`
	Duration float64 `json:"duration"`
	Fraction float64 `json:"fraction"`
	Elapsed  float64 `json:"elapsed"`
	Done     bool    `json:"done"`
}

// A reporter belongs to one conversion attempt. Synchronous native callbacks
// update it on the calling goroutine, so stage timing needs no shared mutex.
type reporter struct {
	report  func(Progress)
	started time.Time
	current Progress
}

func (r *reporter) stage(ctx context.Context, stage string) context.Context {
	r.started = time.Now()
	r.current = Progress{Stage: stage}
	r.send()
	diagnostic.Emit(ctx, diagnostic.Event{Level: "info", Code: "stage_start", Stage: stage, Message: "Conversion stage started"})
	return av.WithProgress(diagnostic.WithStage(ctx, stage), func(p av.Progress) {
		if p.Rate > 0 {
			r.current.Seconds = float64(p.Samples) / float64(p.Rate)
		}
		r.current.Duration = p.Duration
		r.current.Fraction = 0
		if p.Duration > 0 {
			// Metadata may underestimate length; reserve 100% for actual completion.
			r.current.Fraction = min(r.current.Seconds/p.Duration, .99)
		}
		// Native Done can mean source decoding ended while AC-3/E-AC-3 still has
		// encoder delay/padding to flush. Only finish may complete this stage.
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

// durationWarning is advisory: container estimates can be inaccurate, and a
// cleanly truncated stream can still pass PCM equality against its decoded source.
func durationWarning(seconds, duration float64) *diagnostic.Problem {
	if duration <= 0 || seconds <= 0 || duration-seconds <= max(2, duration*.02) {
		return nil
	}
	return &diagnostic.Problem{Code: "duration_short", Stage: "encode",
		Detail: fmt.Sprintf("Decoded %.3fs; metadata estimates %.3fs. The source may be incomplete or its metadata inaccurate; this is not proof of truncation.", seconds, duration)}
}
