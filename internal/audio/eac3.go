package audio

import (
	"context"
	"fmt"

	"github.com/ninghaofeng/hdma2truehd/internal/av"
)

func eac3Bitrate(info Info, kbps int) (int, error) {
	if info.Rate != 48000 || (info.Stream.Layout != "5.1(side)" && info.Stream.Layout != "7.1") {
		return 0, fmt.Errorf("E-AC-3 supports 48000 Hz with 5.1(side) or 7.1 only; no automatic resampling/remapping")
	}
	minimum, maximum, standard := 384, 1024, 960
	if info.Stream.Layout == "7.1" {
		minimum, maximum, standard = 768, 1536, 1536
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
	if output.Stream.Codec != "eac3" || output.Rate != source.Rate ||
		output.Stream.Channels != source.Stream.Channels || output.Stream.Layout != source.Stream.Layout {
		return fmt.Errorf("E-AC-3 codec/rate/channel layout changed: %+v", output.Stream)
	}
	return nil
}

type pcmCounter uint64

func (c *pcmCounter) Write(p []byte) (int, error) {
	*c += pcmCounter(len(p))
	return len(p), nil
}

func validateEAC3(ctx context.Context, encoded Info, stats av.EAC3Stats) error {
	expected := (stats.Samples + 256 + 1535) / 1536 * 1536
	if stats.Samples == 0 || stats.Decoded != expected {
		return fmt.Errorf("E-AC-3 encoder length mismatch: source=%d decoded=%d expected=%d", stats.Samples, stats.Decoded, expected)
	}
	info := nativeInfo(encoded)
	info.Bits = 32
	var count pcmCounter
	if err := av.Process(ctx, encoded.Path, "", info, &count); err != nil {
		return fmt.Errorf("strict E-AC-3 decode: %w", err)
	}
	if uint64(count) != expected*uint64(encoded.Stream.Channels)*4 {
		return fmt.Errorf("E-AC-3 decoded length mismatch: %d bytes for %d samples/channel", count, expected)
	}
	return nil
}
