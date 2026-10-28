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
)

type Backend interface {
	ProbeTracks(context.Context, string) ([]audio.Track, error)
	ProbeStream(context.Context, string, int) (audio.Info, error)
	Convert(context.Context, string, string, audio.Options) (audio.Result, error)
}

type Track struct {
	audio.Track
	Problem     string `json:"problem"`
	EAC3Problem string `json:"eac3Problem"`
}

type Input struct {
	Path   string  `json:"path"`
	Tracks []Track `json:"tracks"`
	Error  string  `json:"error"`
}

type Selection struct {
	Path  string `json:"path"`
	Index int    `json:"index"`
}

type Request struct {
	Tracks    []Selection `json:"tracks"`
	TrueHD    bool        `json:"truehd"`
	EAC3      bool        `json:"eac3"`
	Bitrate51 int         `json:"bitrate51"`
	Bitrate71 int         `json:"bitrate71"`
	OutputDir string      `json:"outputDir"`
	Overwrite bool        `json:"overwrite"`
}

type Job struct {
	Input          string `json:"input"`
	Index          int    `json:"index"`
	Output         string `json:"output"`
	Format         string `json:"format"`
	Bitrate        int    `json:"bitrate"`
	Status         string `json:"status"`
	Error          string `json:"error"`
	Samples        uint64 `json:"samples"`
	DecodedSamples uint64 `json:"decodedSamples"`
}

type State struct {
	Revision uint64 `json:"revision"`
	Running  bool   `json:"running"`
	Jobs     []Job  `json:"jobs"`
}

type Service struct {
	backend Backend
	emit    func(State)
	mu      sync.Mutex
	state   State
	cancel  context.CancelFunc
	done    chan struct{}
}

func New(backend Backend, emit func(State)) *Service {
	return &Service{backend: backend, emit: emit, state: State{Jobs: []Job{}}}
}

func allowedPath(path string) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".dts", ".dtshd", ".mkv":
		return nil
	default:
		return fmt.Errorf("supported input extensions: .dts, .dtshd, .mkv")
	}
}

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
			tracks, err = s.backend.ProbeTracks(ctx, absolute)
		}
		if err != nil {
			input.Error = err.Error()
		} else {
			for _, track := range tracks {
				t := Track{Track: track}
				if err := track.Info.ValidateSource(); err != nil {
					t.Problem = err.Error()
				} else if err := track.Info.ValidateEAC3(0); err != nil {
					t.EAC3Problem = err.Error()
				}
				input.Tracks = append(input.Tracks, t)
			}
		}
		found = append(found, input)
	}
	return found, nil
}

func pathKey(path string) (string, error) {
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
		info, err := s.backend.ProbeStream(ctx, selected.Path, selected.Index)
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
	state.Jobs = append([]Job{}, s.state.Jobs...)
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
	jobs, err := s.plan(ctx, req)
	if err != nil {
		return State{}, err
	}
	work, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.done = make(chan struct{})
	s.state.Jobs, s.state.Running = jobs, true
	s.state.Revision++
	state := s.snapshotLocked()
	go s.run(work, req, s.done)
	return state, nil
}

func (s *Service) update(change func()) {
	s.mu.Lock()
	change()
	s.state.Revision++
	state := s.snapshotLocked()
	s.mu.Unlock()
	if s.emit != nil {
		s.emit(state)
	}
}

func (s *Service) run(ctx context.Context, req Request, done chan struct{}) {
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
	for i := range s.State().Jobs {
		if ctx.Err() != nil {
			break
		}
		s.update(func() { s.state.Jobs[i].Status = "running" })
		job := s.State().Jobs[i]
		index := job.Index
		result, err := s.backend.Convert(ctx, job.Input, job.Output, audio.Options{
			StreamIndex: &index, EAC3Bitrate: job.Bitrate, Overwrite: req.Overwrite,
		})
		s.update(func() {
			j := &s.state.Jobs[i]
			switch {
			case err != nil:
				j.Status, j.Error = "failed", err.Error()
				if errors.Is(err, context.Canceled) {
					j.Status = "cancelled"
				}
			case result.Verified:
				j.Status, j.Samples = "verified", result.Samples
			case result.Validated && result.Lossy:
				j.Status, j.Samples, j.DecodedSamples = "validated", result.Samples, result.DecodedSamples
			default:
				j.Status, j.Error = "failed", "conversion returned an unverified output"
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

// Closing the window waits for the active native call to unwind and remove
// staging files; it never terminates the process midway through publication.
func (s *Service) Shutdown() {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	done := s.done
	s.mu.Unlock()
	if done != nil {
		<-done
	}
}
