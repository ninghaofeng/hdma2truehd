package integration

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ninghaofeng/hdma2truehd/internal/audio"
)

func TestDTSCoreAndMA51LossyOutputs(t *testing.T) {
	source, ffmpeg := os.Getenv("HDMA_TEST_DTS_51"), os.Getenv("HDMA_TEST_DECODER")
	if source == "" || ffmpeg == "" {
		t.Skip("set HDMA_TEST_DTS_51 and HDMA_TEST_DECODER for real DTS core/MA coverage")
	}
	// Only fixture construction uses the external tool. The actual probe,
	// conversion, strict decoding and publication use our in-process backend.
	core := filepath.Join(t.TempDir(), "ordinary.dts")
	command := exec.CommandContext(t.Context(), ffmpeg, "-v", "error", "-i", source,
		"-t", "1", "-map", "0:a:0", "-c:a", "copy", "-bsf:a", "dca_core", core)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("core fixture: %v\n%s", err, output)
	}
	b := audio.Backend{}
	info, err := b.Probe(t.Context(), core)
	if err != nil {
		t.Fatal(err)
	}
	if info.Stream.Profile != "DTS" || info.Stream.Layout != "5.1(side)" || info.Rate != 48000 || info.Stream.Channels != 6 {
		t.Fatalf("unexpected core fixture: %+v", info)
	}
	movie := filepath.Join(t.TempDir(), "ordinary.mkv")
	command = exec.CommandContext(t.Context(), ffmpeg, "-v", "error", "-i", core, "-c:a", "copy", movie)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("core MKV fixture: %v\n%s", err, output)
	}
	for _, input := range []string{source, core, movie} {
		for _, format := range []string{"eac3", "ac3"} {
			t.Run(filepath.Base(input)+"/"+format, func(t *testing.T) {
				target := filepath.Join(t.TempDir(), "output."+format)
				// MA retains this setting; ordinary DTS must ignore it and use 1024.
				options := audio.Options{EAC3Bitrate: 384}
				if input == movie {
					index := 0
					options.StreamIndex = &index
				}
				result, err := b.Convert(t.Context(), input, target, options)
				if err != nil {
					t.Fatal(err)
				}
				if result.Verified || !result.Validated || !result.Lossy || result.Samples == 0 ||
					result.DecodedSamples != (result.Samples+256+1535)/1536*1536 {
					t.Fatalf("validation: %+v", result)
				}
				data, err := os.ReadFile(target)
				if err != nil {
					t.Fatal(err)
				}
				wantRate := uint64(384000)
				if format == "ac3" {
					wantRate = 640000
				} else if input != source {
					wantRate = 1024000
				}
				if uint64(len(data))*8*48000 != wantRate*result.DecodedSamples {
					t.Fatalf("expected fixed bitrate %d, bytes=%d samples=%d", wantRate, len(data), result.DecodedSamples)
				}
				options.EAC3Bitrate = 0
				if _, err := b.Convert(t.Context(), input, target, options); err == nil {
					t.Fatal("overwrote output without permission")
				}
			})
		}
	}
	output := filepath.Join(t.TempDir(), "rejected.thd")
	if _, err := b.Convert(t.Context(), core, output, audio.Options{}); err == nil {
		t.Fatal("ordinary DTS accepted for TrueHD")
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("policy rejection published output")
	}
}
