// Package gui owns the desktop queue independently of the webview, so file
// planning, cancellation and publication behavior can be tested headlessly.
package gui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/ninghaofeng/hdma2truehd/internal/audio"
	"github.com/ninghaofeng/hdma2truehd/internal/batch"
	"github.com/ninghaofeng/hdma2truehd/internal/diagnostic"
)

type Backend interface {
	ProbeTracks(context.Context, string) ([]audio.Track, error)
	ProbeStream(context.Context, string, int) (audio.Info, error)
	Convert(context.Context, string, string, audio.Options) (audio.Result, error)
}

// Problem rules out all conversion; EAC3Problem only rules out the lossy path.
// Keep both visible rather than hiding tracks or silently changing formats.
type Track struct {
	audio.Track
	Problem         string `json:"problem"`
	EAC3Problem     string `json:"eac3Problem"`
	ProblemCode     string `json:"problemCode"`
	EAC3ProblemCode string `json:"eac3ProblemCode"`
}

type Input struct {
	Path    string              `json:"path"`
	Tracks  []Track             `json:"tracks"`
	Error   string              `json:"error"`
	Problem *diagnostic.Problem `json:"problem,omitempty"`
}

type Selection struct {
	Path  string `json:"path"`
	Index int    `json:"index"`
}

// Request is a preflight input, not a trusted copy of Inspect's metadata.
// Bitrate51/71 are total kb/s for their layouts; zero delegates defaults to audio.
type Request struct {
	Tracks      []Selection `json:"tracks"`
	TrueHD      bool        `json:"truehd"`
	EAC3        bool        `json:"eac3"`
	Bitrate51   int         `json:"bitrate51"`
	Bitrate71   int         `json:"bitrate71"`
	OutputDir   string      `json:"outputDir"`
	Overwrite   bool        `json:"overwrite"`
	Concurrency int         `json:"concurrency"`
}

// A job represents one track/format output, so selecting both formats creates
// two independently published jobs. Sample counts are always per channel.
type Job struct {
	Input          string               `json:"input"`
	Index          int                  `json:"index"`
	Output         string               `json:"output"`
	Format         string               `json:"format"`
	Bitrate        int                  `json:"bitrate"`
	Status         string               `json:"status"`
	Error          string               `json:"error"`
	Samples        uint64               `json:"samples"`
	DecodedSamples uint64               `json:"decodedSamples"`
	Progress       audio.Progress       `json:"progress"`
	Problem        *diagnostic.Problem  `json:"problem,omitempty"`
	Warnings       []diagnostic.Problem `json:"warnings,omitempty"`
}

// Revision orders snapshots across RPC responses and asynchronous queue events.
// Consumers must discard older revisions, not assume delivery order is FIFO.
type State struct {
	Revision   uint64              `json:"revision"`
	Running    bool                `json:"running"`
	Jobs       []Job               `json:"jobs"`
	LogPath    string              `json:"logPath"`
	LogProblem *diagnostic.Problem `json:"logProblem,omitempty"`
}

// mu protects queue state and run ownership. Native conversion runs without
// this lock, allowing cancellation and state reads while C is processing audio.
type Service struct {
	backend  Backend
	emit     func(State)
	mu       sync.Mutex
	state    State
	cancel   context.CancelFunc
	done     chan struct{}
	request  Request
	recorder *diagnostic.Recorder
	logError error
}

func New(backend Backend, emit func(State)) *Service {
	return &Service{backend: backend, emit: emit, state: State{Jobs: []Job{}}}
}

// ConfigureDiagnostics is called before the UI starts issuing requests.
func (s *Service) ConfigureDiagnostics(recorder *diagnostic.Recorder, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recorder, s.logError = recorder, err
}

func (s *Service) Logs() (diagnostic.View, error) {
	s.mu.Lock()
	recorder, err := s.recorder, s.logError
	s.mu.Unlock()
	if recorder == nil {
		if err != nil {
			return diagnostic.View{}, err
		}
		return diagnostic.View{}, fmt.Errorf("diagnostic logging is not configured")
	}
	view, readErr := recorder.Read()
	if err != nil && view.Problem == nil {
		view.Problem = &diagnostic.Problem{Code: "log_unavailable", Detail: err.Error()}
	}
	return view, readErr
}

func allowedPath(path string) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".dts", ".dtshd", ".mkv":
		return nil
	default:
		return fmt.Errorf("supported input extensions: .dts, .dtshd, .mkv")
	}
}

// Inspect retains per-file failures so one bad drop does not hide valid files.
// Cancellation checked between files and path-resolution failures abort the call.
func (s *Service) Inspect(ctx context.Context, paths []string) ([]Input, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no input files")
	}
	found := make([]Input, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		key, err := pathKey(absolute)
		if err != nil {
			return nil, err
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		input := Input{Path: absolute, Tracks: []Track{}}
		err = allowedPath(absolute)
		var tracks []audio.Track
		if err == nil {
			tracks, err = s.backend.ProbeTracks(diagnostic.WithJob(ctx, absolute, "", 0), absolute)
		}
		if err != nil {
			input.Error = err.Error()
			input.Problem = diagnostic.Describe(diagnostic.Wrap("probe", err))
			diagnostic.Emit(ctx, diagnostic.Event{Level: "error", Code: input.Problem.Code, Input: absolute, Message: input.Error})
		} else {
			for _, track := range tracks {
				t := Track{Track: track}
				if err := track.Info.ValidateSource(); err != nil {
					t.Problem = err.Error()
					var unsupported *audio.UnsupportedError
					if errors.As(err, &unsupported) {
						t.ProblemCode = unsupported.Code
					}
				} else if err := track.Info.ValidateEAC3(0); err != nil {
					t.EAC3Problem = err.Error()
					var unsupported *audio.UnsupportedError
					if errors.As(err, &unsupported) {
						t.EAC3ProblemCode = unsupported.Code
					}
				}
				input.Tracks = append(input.Tracks, t)
			}
		}
		found = append(found, input)
	}
	return found, nil
}

func pathKey(path string) (string, error) {
	// Outputs may not exist yet, so resolve their parent when the leaf is absent.
	// Case folding deliberately favors collision rejection on macOS/Windows;
	// plan separately checks hard-link identity with os.SameFile.
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	} else if errors.Is(err, os.ErrNotExist) {
		resolved, err := filepath.EvalSymlinks(filepath.Dir(path))
		if err != nil {
			return "", err
		}
		path = filepath.Join(resolved, filepath.Base(path))
	} else {
		return "", err
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return strings.ToLower(path), nil
	}
	return path, nil
}

func (s *Service) plan(ctx context.Context, req Request) ([]Job, error) {
	if len(req.Tracks) == 0 || (!req.TrueHD && !req.EAC3) {
		return nil, fmt.Errorf("select audio tracks and at least one output format")
	}
	if !req.EAC3 && (req.Bitrate51 != 0 || req.Bitrate71 != 0) {
		return nil, fmt.Errorf("bitrate applies only to E-AC-3")
	}
	if req.OutputDir != "" {
		dir, err := os.Stat(req.OutputDir)
		if err != nil {
			return nil, err
		}
		if !dir.IsDir() {
			return nil, fmt.Errorf("output directory is not a directory")
		}
	}
	jobs := []Job{}
	destinations := map[string]bool{}
	sources := map[string]bool{}
	// Collect every source first: an early output must not overwrite a later
	// input in the batch, even when replacing existing outputs was authorized.
	for _, track := range req.Tracks {
		path, err := filepath.Abs(track.Path)
		if err != nil {
			return nil, err
		}
		key, err := pathKey(path)
		if err != nil {
			return nil, err
		}
		sources[key] = true
	}
	for _, selected := range req.Tracks {
		if err := allowedPath(selected.Path); err != nil {
			return nil, err
		}
		// Selection may outlive the displayed inspection. Reprobe by the global
		// container index and reapply audio's policy before creating any jobs.
		info, err := s.backend.ProbeStream(diagnostic.WithJob(ctx, selected.Path, "", 0), selected.Path, selected.Index)
		if err != nil {
			return nil, err
		}
		if err := info.ValidateSource(); err != nil {
			return nil, err
		}
		bitrate := req.Bitrate51
		if info.Stream.Layout == "7.1" {
			bitrate = req.Bitrate71
		}
		if req.EAC3 {
			if err := info.ValidateEAC3(bitrate); err != nil {
				return nil, fmt.Errorf("%s track %d: %w", selected.Path, selected.Index, err)
			}
		}
		dir := req.OutputDir
		if dir == "" {
			dir = filepath.Dir(info.Path)
		}
		base := strings.TrimSuffix(filepath.Base(info.Path), filepath.Ext(info.Path))
		// Track indices disambiguate multiple selections from the same MKV.
		// Titles/languages are display metadata, never trusted as output paths.
		if strings.EqualFold(filepath.Ext(info.Path), ".mkv") {
			base += fmt.Sprintf(".track-%d", selected.Index)
		}
		for _, format := range []string{"thd", "eac3"} {
			if (format == "thd" && !req.TrueHD) || (format == "eac3" && !req.EAC3) {
				continue
			}
			target, err := filepath.Abs(filepath.Join(dir, base+"."+format))
			if err != nil {
				return nil, err
			}
			key, err := pathKey(target)
			if err != nil {
				return nil, err
			}
			if sources[key] || destinations[key] {
				return nil, fmt.Errorf("output collision: %s", target)
			}
			for _, source := range req.Tracks {
				in, inErr := os.Stat(source.Path)
				out, outErr := os.Stat(target)
				if inErr == nil && outErr == nil && os.SameFile(in, out) {
					return nil, fmt.Errorf("output aliases an input: %s", target)
				}
			}
			if !req.Overwrite {
				if _, err := os.Lstat(target); err == nil {
					return nil, fmt.Errorf("output exists; enable overwrite explicitly: %s", target)
				} else if !errors.Is(err, os.ErrNotExist) {
					return nil, err
				}
			}
			destinations[key] = true
			job := Job{Input: info.Path, Index: selected.Index, Output: target, Format: format, Status: "pending"}
			// Snapshot the layout-specific setting after preflight. TrueHD jobs
			// carry no lossy bitrate, even when both formats were selected.
			if format == "eac3" {
				job.Bitrate = bitrate
			}
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

func (s *Service) snapshotLocked() State {
	state := s.state
	// Copy mutable diagnostic fields as well as jobs before releasing the lock.
	state.Jobs = append([]Job{}, s.state.Jobs...)
	for i := range state.Jobs {
		state.Jobs[i].Warnings = append([]diagnostic.Problem{}, state.Jobs[i].Warnings...)
		if state.Jobs[i].Problem != nil {
			problem := *state.Jobs[i].Problem
			state.Jobs[i].Problem = &problem
		}
	}
	err := s.logError
	if s.recorder != nil {
		state.LogPath = s.recorder.Path()
		err = errors.Join(err, s.recorder.Error())
	}
	if err != nil {
		state.LogProblem = &diagnostic.Problem{Code: "log_unavailable", Detail: err.Error()}
	}
	return state
}

func (s *Service) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

// Start preflights every selection before starting any write. The state lock
// serializes concurrent start requests, including their preflight phase.
func (s *Service) Start(ctx context.Context, req Request) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Running {
		return State{}, fmt.Errorf("a conversion is already running")
	}
	if req.Concurrency == 0 {
		req.Concurrency = batch.DefaultConcurrency
	}
	if err := batch.ValidateConcurrency(req.Concurrency); err != nil {
		return State{}, err
	}
	jobs, err := s.plan(ctx, req)
	if err != nil {
		return State{}, diagnostic.Wrap("preflight", err)
	}
	return s.startLocked(ctx, req, jobs), nil
}

// startLocked requires mu and installs run ownership before launching workers.
// Retry supplies the full history, with only failed jobs reset to pending.
func (s *Service) startLocked(ctx context.Context, req Request, jobs []Job) State {
	work, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.done = make(chan struct{})
	s.state.Jobs, s.state.Running = jobs, true
	s.request = req
	s.state.Revision++
	state := s.snapshotLocked()
	go s.run(work, req, s.done)
	return state
}

// RetryFailed replans failed outputs under the original settings. Successful
// and cancelled jobs remain history; retries never bypass overwrite protection.
func (s *Service) RetryFailed(ctx context.Context) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Running {
		return State{}, fmt.Errorf("cannot retry while conversions are running")
	}
	jobs := append([]Job{}, s.state.Jobs...)
	// Per-job replanning sees only that job's input. Protect every original
	// input too, including aliases belonging to jobs that will not be retried.
	sources := map[string]bool{}
	var sourceFiles []os.FileInfo
	for _, selection := range s.request.Tracks {
		path, err := filepath.Abs(selection.Path)
		if err != nil {
			return State{}, diagnostic.Wrap("preflight", err)
		}
		key, err := pathKey(path)
		if err != nil {
			return State{}, diagnostic.Wrap("preflight", err)
		}
		stat, err := os.Stat(path)
		if err != nil {
			return State{}, diagnostic.Wrap("preflight", err)
		}
		sources[key] = true
		sourceFiles = append(sourceFiles, stat)
	}
	destinations := map[string]bool{}
	found := false
	for i, job := range jobs {
		if job.Status != "failed" {
			continue
		}
		req := s.request
		req.Tracks = []Selection{{Path: job.Input, Index: job.Index}}
		req.TrueHD, req.EAC3 = job.Format == "thd", job.Format == "eac3"
		if !req.EAC3 {
			req.Bitrate51, req.Bitrate71 = 0, 0
		}
		planned, err := s.plan(ctx, req)
		if err != nil {
			return State{}, diagnostic.Wrap("preflight", err)
		}
		if len(planned) != 1 || planned[0].Output != job.Output {
			return State{}, fmt.Errorf("retry output plan changed")
		}
		key, err := pathKey(planned[0].Output)
		if err != nil {
			return State{}, diagnostic.Wrap("preflight", err)
		}
		if sources[key] || destinations[key] {
			return State{}, fmt.Errorf("retry output collision: %s", planned[0].Output)
		}
		if stat, err := os.Stat(planned[0].Output); err == nil {
			for _, source := range sourceFiles {
				if os.SameFile(stat, source) {
					return State{}, fmt.Errorf("retry output aliases an original input: %s", planned[0].Output)
				}
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return State{}, diagnostic.Wrap("preflight", err)
		}
		destinations[key] = true
		jobs[i] = planned[0]
		found = true
	}
	if !found {
		return State{}, fmt.Errorf("no failed jobs to retry")
	}
	// Commit the replacement queue only after every retry passes preflight;
	// a late planning failure must not partially reset the displayed history.
	diagnostic.Emit(ctx, diagnostic.Event{Level: "info", Code: "manual_retry", Message: "Retrying failed jobs only; preserving successful and cancelled jobs"})
	return s.startLocked(ctx, s.request, jobs), nil
}

func (s *Service) update(change func()) {
	s.mu.Lock()
	change()
	s.state.Revision++
	state := s.snapshotLocked()
	s.mu.Unlock()
	// Parallel workers may emit out of order. Consumers use Revision and must
	// accept concurrent callbacks; keep the queue lock free for State/Cancel.
	if s.emit != nil {
		s.emit(state)
	}
}

func (s *Service) run(ctx context.Context, req Request, done chan struct{}) {
	// LIFO defers publish the final state before releasing Shutdown's waiter.
	// All active Convert calls have returned and cleaned their staging files.
	defer close(done)
	defer func() {
		s.update(func() {
			for i := range s.state.Jobs {
				if s.state.Jobs[i].Status == "pending" {
					s.state.Jobs[i].Status = "cancelled"
				}
			}
			s.cancel()
			s.cancel = nil
			s.state.Running = false
		})
	}()
	err := batch.Run(ctx, len(s.State().Jobs), req.Concurrency, func(i int) {
		// Manual retry keeps terminal jobs in this same list. Only pending
		// entries may execute; successful outputs must never be encoded again.
		if s.State().Jobs[i].Status != "pending" {
			return
		}
		s.update(func() { s.state.Jobs[i].Status = "running" })
		job := s.State().Jobs[i]
		index := job.Index
		result, err := s.backend.Convert(ctx, job.Input, job.Output, audio.Options{
			StreamIndex: &index, EAC3Bitrate: job.Bitrate, Overwrite: req.Overwrite,
			Progress: func(progress audio.Progress) {
				s.update(func() { s.state.Jobs[i].Progress = progress })
			},
			Warning: func(problem diagnostic.Problem) {
				s.update(func() {
					if len(s.state.Jobs[i].Warnings) < 20 {
						s.state.Jobs[i].Warnings = append(s.state.Jobs[i].Warnings, problem)
					} else if problem.Code == "warnings_truncated" || problem.Code == "retrying" {
						s.state.Jobs[i].Warnings[19] = problem
					} else if problem.Code == "duration_short" {
						s.state.Jobs[i].Warnings[18] = problem
					}
				})
			},
		})
		event := diagnostic.Event{Level: "info", Code: "job_complete", Input: job.Input, Output: job.Output, Message: "Job completed"}
		if err != nil {
			problem := diagnostic.Describe(err)
			event.Level, event.Code, event.Stage, event.Message = "error", problem.Code, problem.Stage, problem.Detail
		} else if !result.Verified && !(result.Validated && result.Lossy) {
			event.Level, event.Code, event.Message = "error", "verification_failed", "Backend returned an unverified output"
		}
		diagnostic.Emit(ctx, event)
		// Ordinary job failures do not roll back earlier outputs or abort later
		// jobs. Cancellation stops scheduling and cancels pending work.
		s.update(func() {
			j := &s.state.Jobs[i]
			switch {
			case err != nil:
				j.Status, j.Error = "failed", err.Error()
				j.Problem = diagnostic.Describe(err)
				if errors.Is(err, context.Canceled) {
					j.Status = "cancelled"
				}
			case result.Verified:
				j.Status, j.Samples = "verified", result.Samples
			case result.Validated && result.Lossy:
				j.Status, j.Samples, j.DecodedSamples = "validated", result.Samples, result.DecodedSamples
			default:
				j.Status, j.Error = "failed", "conversion returned an unverified output"
				j.Problem = &diagnostic.Problem{Code: "verification_failed", Detail: j.Error}
			}
		})
	})
	if err != nil {
		s.update(func() {
			for i := range s.state.Jobs {
				if s.state.Jobs[i].Status == "pending" {
					s.state.Jobs[i].Status, s.state.Jobs[i].Error = "failed", err.Error()
				}
			}
		})
	}
}

func (s *Service) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}

// Closing the window waits for all active native calls to unwind and remove
// staging files; it never terminates the process midway through publication.
func (s *Service) Shutdown() {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	done := s.done
	s.mu.Unlock()
	// The worker's final update needs mu; waiting while holding it deadlocks.
	if done != nil {
		<-done
	}
}
