package audio

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ninghaofeng/hdma2truehd/internal/av"
)

type Stream struct {
	Index         int    `json:"index"`
	CodecType     string `json:"codec_type"`
	Codec         string `json:"codec_name"`
	Profile       string `json:"profile"`
	SampleRate    string `json:"sample_rate"`
	Channels      int    `json:"channels"`
	Layout        string `json:"channel_layout"`
	BitsPerSample int    `json:"bits_per_sample"`
	RawBits       string `json:"bits_per_raw_sample"`
}

type Info struct {
	Path   string `json:"path"`
	Stream Stream `json:"stream"`
	Rate   int    `json:"sample_rate"`
	Bits   int    `json:"effective_bits"`
}

func localFile(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	stat, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !stat.Mode().IsRegular() {
		return "", fmt.Errorf("%s: input must be a regular local file", path)
	}
	return absolute, nil
}

func (b Backend) Probe(ctx context.Context, path string) (Info, error) {
	if b.probe != nil {
		return b.probe(ctx, path)
	}
	path, err := localFile(path)
	if err != nil {
		return Info{}, err
	}
	info, err := av.Probe(ctx, path)
	if err != nil {
		return Info{}, err
	}
	stream := Stream{Index: info.Index, CodecType: "audio", Codec: info.Codec, Profile: info.Profile,
		SampleRate: strconv.Itoa(info.Rate), Channels: info.Channels, Layout: info.Layout,
		RawBits: strconv.Itoa(info.Bits)}
	return Info{Path: path, Stream: stream, Rate: info.Rate, Bits: info.Bits}, nil
}

func (i Info) ValidateSource() error {
	if i.Stream.Codec != "dts" || (i.Stream.Profile != "DTS-HD MA" &&
		i.Stream.Profile != "DTS-HD MA + DTS:X" && i.Stream.Profile != "DTS-HD MA + DTS:X IMAX") {
		return fmt.Errorf("%s: expected confirmed DTS-HD MA (including DTS:X/IMAX compatible channels), got codec=%q profile=%q; DTS core/HRA are not lossless MA", i.Path, i.Stream.Codec, i.Stream.Profile)
	}
	return i.validateFormat()
}

func (i Info) validateFormat() error {
	channels := 0
	switch i.Stream.Layout {
	case "5.1(side)", "5.1":
		channels = 6
	case "7.1":
		channels = 8
	default:
		return fmt.Errorf("%s: unsupported or unknown channel layout %q; no automatic remapping/downmix", i.Path, i.Stream.Layout)
	}
	if i.Stream.Channels != channels {
		return fmt.Errorf("%s: layout %s disagrees with channel count %d", i.Path, i.Stream.Layout, i.Stream.Channels)
	}
	switch i.Rate {
	case 44100, 48000, 88200, 96000, 176400, 192000:
	default:
		return fmt.Errorf("%s: unsupported sample rate %d; resampling is disabled", i.Path, i.Rate)
	}
	if i.Bits != 16 && i.Bits != 24 {
		return fmt.Errorf("%s: unsupported or unknown effective bit depth %d (need 16 or 24)", i.Path, i.Bits)
	}
	return nil
}

func matchingOutput(source, output Info) error {
	if output.Stream.Codec != "truehd" {
		return fmt.Errorf("output codec %q is not TrueHD", output.Stream.Codec)
	}
	if source.Rate != output.Rate || source.Stream.Channels != output.Stream.Channels || source.Stream.Layout != output.Stream.Layout {
		return fmt.Errorf("output format changed: source=%dHz/%s/%dch output=%dHz/%s/%dch",
			source.Rate, source.Stream.Layout, source.Stream.Channels,
			output.Rate, output.Stream.Layout, output.Stream.Channels)
	}
	if output.Bits < source.Bits || output.Bits > 24 {
		return fmt.Errorf("output effective precision %d cannot preserve source precision %d", output.Bits, source.Bits)
	}
	return nil
}
