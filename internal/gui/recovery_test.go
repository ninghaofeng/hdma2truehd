package gui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ninghaofeng/hdma2truehd/internal/audio"
	"github.com/ninghaofeng/hdma2truehd/internal/diagnostic"
)

func TestRetryFailedPreservesSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.mkv")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	recorder, err := diagnostic.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer recorder.Close()
	ctx := diagnostic.WithSink(t.Context(), recorder.Record)
	var mu sync.Mutex
	calls := map[string]int{}
	s := New(fakeBackend{convert: func(_ context.Context, _, output string, options audio.Options) (audio.Result, error) {
		mu.Lock()
		calls[output]++
		count := calls[output]
		mu.Unlock()
		if strings.HasSuffix(output, ".eac3") && count == 1 {
			options.Warning(diagnostic.Problem{Code: "ffmpeg_warning", Detail: "warning details"})
			return audio.Result{}, diagnostic.Wrap("encode", errors.New("deliberate failure"))
		}
		return audio.Result{Output: output, Verified: strings.HasSuffix(output, ".thd"), Lossy: strings.HasSuffix(output, ".eac3"), Validated: strings.HasSuffix(output, ".eac3")}, nil
	}}, nil)
	s.ConfigureDiagnostics(recorder, nil)
	if _, err := s.Start(ctx, Request{Tracks: []Selection{{path, 0}}, TrueHD: true, EAC3: true}); err != nil {
		t.Fatal(err)
	}
	before := waitQueue(t, s)
	if before.Jobs[0].Status != "verified" || before.Jobs[1].Status != "failed" || before.Jobs[1].Problem.Code != "encode_failed" {
		t.Fatalf("%+v", before)
	}
	before.Jobs[1].Warnings[0].Detail = "mutated snapshot"
	before.Jobs[1].Problem.Code = "mutated snapshot"
	if s.State().Jobs[1].Warnings[0].Detail != "warning details" {
		t.Fatal("warning slice escaped state lock")
	}
	if s.State().Jobs[1].Problem.Code != "encode_failed" {
		t.Fatal("problem pointer escaped state lock")
	}
	if _, err := s.RetryFailed(ctx); err != nil {
		t.Fatal(err)
	}
	after := waitQueue(t, s)
	if after.Jobs[0].Status != "verified" || after.Jobs[1].Status != "validated" || after.Jobs[1].Problem != nil || len(after.Jobs[1].Warnings) != 0 {
		t.Fatalf("retry changed successful job or retained stale errors: %+v", after)
	}
	if calls[after.Jobs[0].Output] != 1 || calls[after.Jobs[1].Output] != 2 {
		t.Fatalf("retried successful output: %v", calls)
	}
	view, err := s.Logs()
	if err != nil || !strings.Contains(view.Content, "manual_retry") || after.LogPath != view.Path {
		t.Fatalf("missing logs: %+v %v", view, err)
	}
	if _, err := s.RetryFailed(ctx); err == nil {
		t.Fatal("retried queue without failed jobs")
	}
}

func TestRetryProtectsAllOriginalInputs(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.dts"), filepath.Join(dir, "b.dts")
	for _, path := range []string{a, b} {
		if err := os.WriteFile(path, []byte("source"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	s := New(fakeBackend{convert: func(_ context.Context, input, output string, _ audio.Options) (audio.Result, error) {
		if input == b {
			return audio.Result{}, errors.New("failed")
		}
		return audio.Result{Output: output, Verified: true}, nil
	}}, nil)
	if _, err := s.Start(t.Context(), Request{Tracks: []Selection{{a, 0}, {b, 0}}, TrueHD: true, Overwrite: true}); err != nil {
		t.Fatal(err)
	}
	state := waitQueue(t, s)
	if err := os.Link(a, state.Jobs[1].Output); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RetryFailed(t.Context()); err == nil {
		t.Fatal("retry could overwrite an original input through a hard link")
	}
	if s.State().Running || s.State().Jobs[0].Status != "verified" {
		t.Fatal("failed retry changed queue")
	}
}

func TestDiagnosticFailureIsVisible(t *testing.T) {
	s := New(fakeBackend{}, nil)
	s.ConfigureDiagnostics(nil, os.ErrPermission)
	if state := s.State(); state.LogProblem == nil || state.LogProblem.Code != "log_unavailable" {
		t.Fatal("log initialization failure hidden")
	}
	if _, err := s.Logs(); !errors.Is(err, os.ErrPermission) {
		t.Fatal(err)
	}
	recorder, err := diagnostic.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := recorder.Close(); err != nil {
		t.Fatal(err)
	}
	recorder.Record(diagnostic.Event{Message: "write after close"})
	s.ConfigureDiagnostics(recorder, nil)
	if state := s.State(); state.LogProblem == nil || state.LogPath == "" {
		t.Fatal("log write failure hidden")
	}
}
