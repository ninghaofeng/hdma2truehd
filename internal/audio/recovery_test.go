package audio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/ninghaofeng/hdma2truehd/internal/diagnostic"
)

func recoveryBackend(input string, encode func(context.Context, Info, string) error) Backend {
	return Backend{
		probe: func(_ context.Context, path string) (Info, error) {
			info := sourceInfo()
			info.Path = path
			if path != input {
				info.Stream.Codec, info.Stream.Profile = "truehd", ""
			}
			return info, nil
		},
		encode: encode,
		decode: func(_ context.Context, _ Info, writer io.Writer) error {
			_, err := writer.Write(pcmData(8, 17))
			return err
		},
	}
}

func TestRetryPolicy(t *testing.T) {
	for _, mode := range []string{"temporary", "persistent temporary", "space", "corrupt", "permission", "cancel backoff"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			input, output := filepath.Join(dir, "source.dts"), filepath.Join(dir, "out.thd")
			if err := os.WriteFile(input, []byte("source"), 0600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			b := recoveryBackend(input, func(_ context.Context, _ Info, path string) error {
				calls++
				if calls == 1 || mode == "persistent temporary" {
					switch mode {
					case "space":
						return syscall.ENOSPC
					case "permission":
						return syscall.EACCES
					case "corrupt":
						return errors.New("invalid data")
					default:
						return syscall.EINTR
					}
				}
				return os.WriteFile(path, []byte("encoded"), 0600)
			})
			var warnings []diagnostic.Problem
			result, err := b.Convert(ctx, input, output, Options{Warning: func(p diagnostic.Problem) {
				warnings = append(warnings, p)
				if mode == "cancel backoff" && p.Code == "retrying" {
					cancel()
				}
			}})
			wantCalls := 1
			if mode == "temporary" || mode == "persistent temporary" {
				wantCalls = 2
			}
			if calls != wantCalls || (err == nil) != (mode == "temporary") {
				t.Fatalf("calls=%d result=%+v err=%v", calls, result, err)
			}
			if mode == "temporary" && (!result.Verified || len(warnings) != 1 || warnings[0].Code != "retrying") {
				t.Fatalf("missing successful retry: %+v %+v", result, warnings)
			}
			if mode == "cancel backoff" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if err != nil {
				if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("failed conversion published output: %v", err)
				}
			}
			staging, err := filepath.Glob(filepath.Join(dir, ".hdma2truehd-*"))
			if err != nil || len(staging) != 0 {
				t.Fatalf("left staging files: %v %v", staging, err)
			}
		})
	}
}

func TestSourceChangeBlocksPublication(t *testing.T) {
	dir := t.TempDir()
	input, output := filepath.Join(dir, "source.dts"), filepath.Join(dir, "out.thd")
	if err := os.WriteFile(input, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	b := recoveryBackend(input, func(_ context.Context, _ Info, path string) error {
		if err := os.WriteFile(input, []byte("source changed"), 0600); err != nil {
			return err
		}
		return os.WriteFile(path, []byte("encoded"), 0600)
	})
	_, err := b.Convert(t.Context(), input, output, Options{})
	if err == nil || diagnostic.Describe(err).Code != "source_changed" {
		t.Fatalf("accepted modified source: %v", err)
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("published changed source")
	}
}

func TestDurationWarningThreshold(t *testing.T) {
	for _, test := range []struct {
		actual, metadata float64
		warn             bool
	}{
		{0, 100, false}, {100, 0, false}, {100, 100, false},
		{98, 100, false}, {97.9, 100, true}, {8, 10, false},
		{7.9, 10, true}, {990, 1000, false}, {979, 1000, true},
		{120, 100, false},
	} {
		warning := durationWarning(test.actual, test.metadata)
		if (warning != nil) != test.warn {
			t.Fatalf("%+v: %+v", test, warning)
		}

		if warning != nil && (warning.Code != "duration_short" || warning.Detail == "") {
			t.Fatal(warning)
		}
	}
}

func TestWarningsAreBoundedButAllLogged(t *testing.T) {
	dir := t.TempDir()
	input, output := filepath.Join(dir, "source.dts"), filepath.Join(dir, "out.thd")
	if err := os.WriteFile(input, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	logged := 0
	ctx := diagnostic.WithSink(t.Context(), func(event diagnostic.Event) {
		if event.Code == "ffmpeg" {
			logged++
		}
	})
	b := recoveryBackend(input, func(ctx context.Context, _ Info, path string) error {
		for i := range 50 {
			diagnostic.Emit(ctx, diagnostic.Event{Level: "warning", Code: "ffmpeg", Message: fmt.Sprintf("warning-%d", i)})
		}
		return os.WriteFile(path, []byte("encoded"), 0600)
	})
	truncated := false
	result, err := b.Convert(ctx, input, output, Options{Warning: func(p diagnostic.Problem) {
		truncated = truncated || p.Code == "warnings_truncated"
	}})
	if err != nil || !result.Verified || len(result.Warnings) != 20 || logged != 50 || !truncated || result.Warnings[19].Code != "warnings_truncated" {
		t.Fatalf("warning handling: result=%+v logged=%d truncated=%v err=%v", result, logged, truncated, err)
	}
}

func TestVerificationFailureDoesNotRetry(t *testing.T) {
	dir := t.TempDir()
	input, output := filepath.Join(dir, "source.dts"), filepath.Join(dir, "out.thd")
	if err := os.WriteFile(input, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	b := recoveryBackend(input, func(_ context.Context, _ Info, path string) error {
		calls++
		return os.WriteFile(path, []byte("encoded"), 0600)
	})
	b.decode = func(_ context.Context, info Info, writer io.Writer) error {
		data := pcmData(8, 17)
		if info.Path != input {
			data[1] ^= 1
		}
		_, err := writer.Write(data)
		return err
	}
	_, err := b.Convert(t.Context(), input, output, Options{})
	if err == nil || diagnostic.Describe(err).Code != "verification_failed" || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("published mismatched PCM")
	}
}
