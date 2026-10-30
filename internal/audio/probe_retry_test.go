package audio

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestProbeRetry(t *testing.T) {
	calls := 0
	b := Backend{probe: func(context.Context, string) (Info, error) {
		calls++
		if calls == 1 {
			return Info{}, syscall.EINTR
		}
		return sourceInfo(), nil
	}}
	if _, err := b.Probe(t.Context(), "source.dts"); err != nil || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := b.Probe(ctx, "source.dts"); !errors.Is(err, context.Canceled) || calls != 2 {
		t.Fatalf("cancelled inspection made an extra attempt: calls=%d err=%v", calls, err)
	}
}

func TestNestedProbesDoNotMultiplyRetries(t *testing.T) {
	input := filepath.Join(t.TempDir(), "source.dts")
	if err := os.WriteFile(input, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	b := Backend{probe: func(context.Context, string) (Info, error) {
		calls++
		return Info{}, syscall.EINTR
	}}
	_, err := b.Convert(t.Context(), input, filepath.Join(t.TempDir(), "out.thd"), Options{})
	if err == nil || calls != 2 {
		t.Fatalf("nested probes multiplied attempts: calls=%d err=%v", calls, err)
	}
}
