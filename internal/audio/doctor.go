package audio

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ninghaofeng/hdma2truehd/internal/av"
)

// SelfTest exercises the installed encoder/decoder without external fixtures
// or subprocesses. It is not a DTS decode test or a playback certification.
func (b Backend) SelfTest(ctx context.Context) (err error) {
	if err := b.Check(ctx); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "hdma2truehd-doctor-")
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := os.RemoveAll(dir); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("clean up self-test: %w", cleanupErr))
		}
	}()
	for _, layout := range []string{"5.1(side)", "5.1", "7.1"} {
		channels := 6
		if layout == "7.1" {
			channels = 8
		}
		// 641 samples straddle the default 640-sample restart interval at
		// 48 kHz, exercising both a complete interval and the final short tail.
		// Deterministic full-range data also makes channel/order errors visible.
		pcm := make([]byte, 641*channels*4)
		var state uint32 = 1234567
		for i := 0; i < len(pcm); i += 4 {
			state = state*1664525 + 1013904223
			binary.LittleEndian.PutUint32(pcm[i:], state&0xffffff00)
		}
		source := filepath.Join(dir, "source.wav")
		output := filepath.Join(dir, "output.thd")
		if err := av.WritePCMFixture(source, pcm, av.Info{Rate: 48000, Bits: 24, Channels: channels, Layout: layout}); err != nil {
			return err
		}
		input, err := b.Probe(ctx, source)
		if err != nil {
			return err
		}
		if err := b.Encode(ctx, input, output); err != nil {
			return fmt.Errorf("%s self-test encode: %w", layout, err)
		}
		info, err := b.Probe(ctx, output)
		if err != nil {
			return err
		}
		expected := Info{Rate: 48000, Bits: 24, Stream: Stream{Channels: channels, Layout: layout}}
		if err := matchingOutput(expected, info); err != nil {
			return fmt.Errorf("%s self-test: %w", layout, err)
		}
		w := newPCMWriter(channels, 24)
		if _, err := w.Write(pcm); err != nil {
			return err
		}
		original, err := w.finish()
		if err != nil {
			return err
		}
		decoded, err := b.Digest(ctx, info)
		if err != nil {
			return err
		}
		if err := ComparePCM(original, decoded); err != nil {
			return fmt.Errorf("%s self-test: %w", layout, err)
		}
		if layout != "5.1" {
			eac3Output := filepath.Join(dir, "output.eac3")
			bitrate, err := eac3Bitrate(input, 0)
			if err != nil {
				return err
			}
			stats, err := av.EncodeEAC3(ctx, source, eac3Output, nativeInfo(input), bitrate)
			if err != nil {
				return fmt.Errorf("%s E-AC-3 self-test encode: %w", layout, err)
			}
			encoded, err := b.Probe(ctx, eac3Output)
			if err != nil {
				return err
			}
			if err := matchingEAC3(input, encoded); err != nil {
				return err
			}
			if err := validateEAC3(ctx, encoded, stats); err != nil {
				return err
			}
		}
	}
	return nil
}
