package audio

import (
	"context"
	"fmt"

	"github.com/ninghaofeng/hdma2truehd/internal/av"
)

// eac3Bitrate resolves CLI kb/s to native bits/second without changing the
// source format. Limits keep core/dependent encoders on six-block frames;
// 32 kb/s steps also give integral byte budgets after the 7.1 split.
func eac3Bitrate(info Info, kbps int) (int, error) {
	if info.Rate != 48000 || (info.Stream.Layout != "5.1(side)" && info.Stream.Layout != "7.1") {
		return 0, &UnsupportedError{Code: "eac3_format", Message: fmt.Sprintf("E-AC-3 supports 48000 Hz with 5.1(side) or 7.1 only; got %d Hz / %s; no automatic resampling/remapping", info.Rate, info.Stream.Layout)}
	}
	minimum, maximum, standard := 384, 1024, 960
	if info.Stream.Layout == "7.1" {
		minimum, maximum, standard = 768, 1536, 1536
	}
	if info.ordinaryDTS() {
		// Core DTS always uses the highest supported rate, independently of
		// the MA-specific bitrate controls in a mixed queue.
		kbps = maximum
	}
	if kbps == 0 {
		kbps = standard
	}
	if kbps < minimum || kbps > maximum || kbps%32 != 0 {
		return 0, fmt.Errorf("E-AC-3 %s bitrate must be %d..%d kb/s in steps of 32", info.Stream.Layout, minimum, maximum)
	}
	return kbps * 1000, nil
}

func matchingEAC3(source, output Info) error {
	return matchingLossy(source, output, "eac3")
}

func matchingLossy(source, output Info, codec string) error {
	if output.Stream.Codec != codec || output.Rate != source.Rate ||
		output.Stream.Channels != source.Stream.Channels || output.Stream.Layout != source.Stream.Layout {
		return fmt.Errorf("%s codec/rate/channel layout changed: %+v", codec, output.Stream)
	}
	return nil
}

// Lossy PCM cannot match source hashes. Count full-width decoded bytes instead,
// without keeping the whole decoded movie in memory.
type pcmCounter uint64

func (c *pcmCounter) Write(p []byte) (int, error) {
	*c += pcmCounter(len(p))
	return len(p), nil
}

func validateEAC3(ctx context.Context, encoded Info, stats av.EAC3Stats) error {
	return validateLossy(ctx, encoded, stats)
}

func validateLossy(ctx context.Context, encoded Info, stats av.EAC3Stats) error {
	// Raw AC-3/E-AC-3 cannot signal gapless trimming: include the 256-sample MDCT
	// tail and round up to a complete 1536-sample access unit. Even removing
	// an otherwise valid whole access unit must fail this length check.
	expected := (stats.Samples + 256 + 1535) / 1536 * 1536
	if stats.Samples == 0 || stats.Decoded != expected {
		return fmt.Errorf("%s encoder length mismatch: source=%d decoded=%d expected=%d", encoded.Stream.Codec, stats.Samples, stats.Decoded, expected)
	}
	info := nativeInfo(encoded)
	// This is a decoder representation, not a claim of 32 effective bits.
	// Lossy samples need not retain the source's zero low-bit padding.
	info.Bits = 32
	var count pcmCounter
	if err := av.Process(ctx, encoded.Path, "", info, &count); err != nil {
		return fmt.Errorf("strict %s decode: %w", encoded.Stream.Codec, err)
	}
	if uint64(count) != expected*uint64(encoded.Stream.Channels)*4 {
		return fmt.Errorf("%s decoded length mismatch: %d bytes for %d samples/channel", encoded.Stream.Codec, count, expected)
	}
	return nil
}
