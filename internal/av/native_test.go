package av

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (string, Info) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "空 格.wav")
	info := Info{Rate: 48000, Bits: 24, Channels: 8, Layout: "7.1"}
	pcm := make([]byte, 48001*8*4)
	for i := 0; i < len(pcm)/4; i++ {
		binary.LittleEndian.PutUint32(pcm[i*4:], uint32(i*76543)<<8)
	}
	if err := WritePCMFixture(path, pcm, info); err != nil {
		t.Fatal(err)
	}
	probed, err := Probe(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	return path, probed
}

func TestNativeCancellation(t *testing.T) {
	path, info := fixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := Process(ctx, path, filepath.Join(t.TempDir(), "cancelled.thd"), info, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("want cancellation, got %v", err)
	}
	if _, err := Probe(ctx, path); !errors.Is(err, context.Canceled) {
		t.Fatalf("want probe cancellation, got %v", err)
	}
	ctx, cancel = context.WithCancel(t.Context())
	defer cancel()
	w := writerFunc(func(p []byte) (int, error) {
		cancel()
		return len(p), nil
	})
	if err := Process(ctx, path, "", info, w); !errors.Is(err, context.Canceled) {
		t.Fatalf("want in-flight cancellation, got %v", err)
	}
}

type writerFunc func([]byte) (int, error)

func (w writerFunc) Write(p []byte) (int, error) { return w(p) }

func TestNativeErrors(t *testing.T) {
	path, info := fixture(t)
	deliberate := errors.New("deliberate writer error")
	if err := Process(t.Context(), path, "", info, writerFunc(func([]byte) (int, error) {
		return 0, deliberate
	})); !errors.Is(err, deliberate) {
		t.Fatalf("callback error lost: %v", err)
	}
	if err := Process(t.Context(), path, "", info, writerFunc(func([]byte) (int, error) {
		return 0, nil
	})); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write lost: %v", err)
	}
	wrong := info
	wrong.Layout = "5.1(side)"
	if err := Process(t.Context(), path, "", wrong, io.Discard); err == nil {
		t.Fatal("accepted changed layout")
	}
	wrong = info
	wrong.Codec = "dts"
	wrong.Profile = "DTS-HD MA"
	if err := Process(t.Context(), path, "", wrong, io.Discard); err == nil {
		t.Fatal("accepted input codec/profile change")
	}
	wrong = info
	wrong.Bits = 16
	if err := Process(t.Context(), path, "", wrong, io.Discard); err == nil || !strings.Contains(err.Error(), "precision") {
		t.Fatalf("accepted precision loss: %v", err)
	}
	if err := Process(t.Context(), path, filepath.Join(t.TempDir(), "missing", "out.thd"), info, nil); err == nil {
		t.Fatal("lost output-open error")
	}
	broken := filepath.Join(t.TempDir(), "broken.dts")
	if err := os.WriteFile(broken, []byte("not audio"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Probe(t.Context(), broken); err == nil {
		t.Fatal("accepted invalid input")
	}
	if _, err := Probe(t.Context(), "a\x00b"); err == nil {
		t.Fatal("accepted NUL path")
	}
}

func TestNativeConcurrent(t *testing.T) {
	for i := 0; i < 4; i++ {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			t.Parallel()
			path, info := fixture(t)
			if err := Process(t.Context(), path, "", info, io.Discard); err != nil {
				t.Fatal(err)
			}
		})
	}
}
