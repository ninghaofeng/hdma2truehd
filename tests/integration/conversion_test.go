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
	// Low-level encoding accepts synthetic PCM for doctor, but the public
	// conversion pipeline must still enforce the lossless DTS source policy.
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
	// Real compressed fixtures are opt-in and checked separately by layout
	// and X profile. Never substitute synthetic PCM for missing DTS coverage.
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
			t.Run("EAC3", func(t *testing.T) {
				if source.Rate != 48000 || (source.Stream.Layout != "5.1(side)" && source.Stream.Layout != "7.1") {
					t.Skip("E-AC-3 requires a 48 kHz 5.1(side) or 7.1 fixture")
				}
				target := filepath.Join(t.TempDir(), "output.eac3")
				converted, err := b.Convert(t.Context(), path, target, audio.Options{})
				if err != nil {
					t.Fatal(err)
				}
				if !converted.Validated || !converted.Lossy || converted.Verified || converted.Samples != result.Samples ||
					converted.DecodedSamples != (result.Samples+256+1535)/1536*1536 {
					t.Fatalf("wrong lossy validation result: %+v", converted)
				}
				if _, err := b.Convert(t.Context(), path, target, audio.Options{}); err == nil {
					t.Fatal("overwrote E-AC-3 without permission")
				}
				unverified, err := b.Convert(t.Context(), path, target, audio.Options{Overwrite: true, SkipVerify: true})
				if err != nil || unverified.Validated || unverified.Verified || !unverified.Lossy {
					t.Fatalf("incorrect skip/overwrite behavior: %+v %v", unverified, err)
				}
			})
		})
	}
}
