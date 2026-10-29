package diagnostic

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
)

func TestDescribe(t *testing.T) {
	for _, test := range []struct {
		err   error
		code  string
		retry bool
	}{
		{Wrap("encode", syscall.EINTR), "temporary_io", true},
		{Wrap("publish", syscall.EINTR), "temporary_io", false},
		{Wrap("encode", syscall.ENOSPC), "no_space", false},
		{Wrap("encode", syscall.EACCES), "permission", false},
		{Wrap("probe", os.ErrNotExist), "not_found", false},
		{Wrap("encode", context.Canceled), "cancelled", false},
		{Wrap("encode", context.DeadlineExceeded), "deadline", false},
		{Wrap("verify_output", errors.New("mismatch")), "verification_failed", false},
		{Wrap("encode", &Failure{Code: "cleanup_failed", Cause: errors.Join(syscall.EINTR, errors.New("cleanup"))}), "cleanup_failed", false},
	} {
		p := Describe(test.err)
		if p.Code != test.code || p.Retryable != test.retry || p.Stage == "" || p.Detail == "" {
			t.Fatalf("%v: %+v", test.err, p)
		}
		var wire Problem
		if err := json.Unmarshal([]byte(Transport(test.err).Error()), &wire); err != nil || wire.Code != test.code {
			t.Fatalf("transport: %+v %v", wire, err)
		}
	}
	if Describe(nil) != nil || Transport(nil) != nil || !errors.Is(Wrap("probe", context.Canceled), context.Canceled) {
		t.Fatal("lost nil or cancellation semantics")
	}
}

func TestConcurrentRecording(t *testing.T) {
	r, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := WithSink(t.Context(), r.Record)
			ctx = WithStage(WithJob(ctx, fmt.Sprintf("input-%d", i), fmt.Sprintf("output-%d", i), 2), "encode")
			ctx = WithStream(ctx, i)
			for range 50 {
				Emit(ctx, Event{Level: "warning", Code: "test", Message: "raw details"})
			}
		}()
	}
	wg.Wait()
	view, err := r.Read()
	if err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(strings.NewReader(view.Content))
	count := 0
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		if event.Code == "test" {
			if event.Stage != "encode" || event.Attempt != 2 || event.Time.IsZero() || event.StreamIndex == nil || !strings.HasPrefix(event.Input, "input-") {
				t.Fatalf("missing context: %+v", event)
			}
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 400 || view.Truncated {
		t.Fatalf("count=%d truncated=%v", count, view.Truncated)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	r.Record(Event{Message: "after close"})
	if r.Error() == nil {
		t.Fatal("closed-log failure was hidden")
	}
}

func TestRotationAndReadLimit(t *testing.T) {
	r, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.limit = 512
	for range 20 {
		r.Record(Event{Level: "info", Code: "rotation", Message: strings.Repeat("x", 100)})
	}
	for _, path := range []string{r.Path(), r.Path() + ".1"} {
		info, err := os.Stat(path)
		if err != nil || info.Size() > r.limit {
			t.Fatalf("rotation %s: %v", path, err)
		}
	}
	r.limit = 1 << 20
	r.Record(Event{Message: strings.Repeat("z", 200<<10)})
	view, err := r.Read()
	if err != nil || !view.Truncated || len(view.Content) != 128<<10 {
		t.Fatalf("read limit: %+v %v", view.Truncated, err)
	}
}

func TestLogErrorsAndBundleDirectory(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(file); err == nil {
		t.Fatal("silently accepted invalid log location")
	}
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.file.Close(); err != nil {
		t.Fatal(err)
	}
	r.Record(Event{Message: "must fail"})
	if r.Error() == nil {
		t.Fatal("write error was not retained")
	}
	if view, err := r.Read(); err != nil || view.Problem == nil || view.Content == "" {
		t.Fatal("earlier log records or explicit write failure were lost")
	}
	if err := r.Close(); err == nil {
		t.Fatal("close omitted prior write error")
	}
	bundle := filepath.Join(dir, "Tool.app", "Contents", "MacOS", "Tool")
	if programDirectory(bundle) != dir || programDirectory(filepath.Join(dir, "Tool")) != dir {
		t.Fatal("logs would be written inside a signed bundle")
	}
}
