package integration

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ninghaofeng/hdma2truehd/internal/audio"
	"github.com/ninghaofeng/hdma2truehd/internal/av"
)

func testBackend(t *testing.T) audio.Backend {
	t.Helper()
	return audio.Backend{}
}

func TestDoctor(t *testing.T) {
	if err := testBackend(t).SelfTest(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestRejectPCMInput(t *testing.T) {
	b := testBackend(t)
	dir := t.TempDir()
	wav := filepath.Join(dir, "not-ma.wav")
	output := filepath.Join(dir, "must-not-exist.thd")
	if err := av.WritePCMFixture(wav, make([]byte, 641*8*4),
		av.Info{Rate: 48000, Bits: 24, Channels: 8, Layout: "7.1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Convert(t.Context(), wav, output, audio.Options{}); err == nil {
		t.Fatal("PCM input was accepted as DTS-HD MA")
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rejected input produced output: %v", err)
	}
}

func TestDTSHDMAConversion(t *testing.T) {
	b := testBackend(t)
	for _, fixture := range []struct {
		env      string
		channels int
		dtsx     bool
	}{
		{"HDMA_TEST_DTS_51", 6, false},
		{"HDMA_TEST_DTS_71", 8, false},
		{"HDMA_TEST_DTSX_51", 6, true},
		{"HDMA_TEST_DTSX_71", 8, true},
	} {
		t.Run(fixture.env, func(t *testing.T) {
			path := os.Getenv(fixture.env)
			if path == "" {
				t.Skip("set " + fixture.env + " to a local, legally usable DTS-HD MA audio fixture")
			}
			source, err := b.Probe(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			if source.Stream.Channels != fixture.channels {
				t.Fatalf("fixture needs %d channels, got %d", fixture.channels, source.Stream.Channels)
			}
			if fixture.dtsx && source.Stream.Profile != "DTS-HD MA + DTS:X" &&
				source.Stream.Profile != "DTS-HD MA + DTS:X IMAX" {
				t.Fatalf("fixture needs a confirmed lossless DTS:X profile, got %q", source.Stream.Profile)
			}
			output := filepath.Join(t.TempDir(), "output.thd")
			result, err := b.Convert(t.Context(), path, output, audio.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if !result.Verified || result.Samples == 0 {
				t.Fatalf("conversion was not fully verified: %+v", result)
			}
		})
	}
}
