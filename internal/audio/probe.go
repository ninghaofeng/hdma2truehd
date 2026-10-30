package audio

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ninghaofeng/hdma2truehd/internal/av"
	"github.com/ninghaofeng/hdma2truehd/internal/diagnostic"
)

// Stream exposes codec-style JSON fields; Index is the global container stream
// index, not a position within the audio-only list returned by ProbeTracks.
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

// Info keeps codec/container metadata alongside parsed values used to enforce
// the conversion contract. Bits is reported effective precision, not internal
// PCM storage width; ordinary DTS does not require MA's 16/24-bit invariant.
type Info struct {
	Path                       string `json:"path"`
	Stream                     Stream `json:"stream"`
	Rate                       int    `json:"sample_rate"`
	Bits                       int    `json:"effective_bits"`
	Language                   string `json:"language"`
	DelayMS                    int64  `json:"delay_ms"`
	ContentEncodingUnsupported bool   `json:"content_encoding_unsupported,omitempty"`
}

// OutputBase shares automatic naming between CLI and desktop. Container text
// is untrusted: only short ASCII language codes can become filename components.
func (i Info) OutputBase() string {
	base := strings.TrimSuffix(filepath.Base(i.Path), filepath.Ext(i.Path))
	if !strings.EqualFold(filepath.Ext(i.Path), ".mkv") {
		return base
	}
	language := strings.ToLower(strings.TrimSpace(i.Language))
	valid := len(language) == 2 || len(language) == 3
	for _, c := range language {
		valid = valid && c >= 'a' && c <= 'z'
	}
	if !valid {
		language = "und"
	}
	return fmt.Sprintf("%s_track%d_[%s]_DELAY %dms", base, i.Stream.Index, language, i.DelayMS)
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

// Probe reports metadata but does not assert that a file is a supported source:
// it is also used to inspect generated TrueHD and synthetic PCM fixtures.
func (b Backend) Probe(ctx context.Context, path string) (result Info, err error) {
	err = probeWithRetry(ctx, path, func(ctx context.Context) error {
		if b.probe != nil {
			result, err = b.probe(ctx, path)
			return err
		}
		path, err = localFile(path)
		if err != nil {
			return err
		}
		info, probeErr := av.Probe(ctx, path)
		if probeErr != nil {
			return probeErr
		}
		result = fromNative(path, info)
		return nil
	})
	if err != nil {
		return Info{}, err
	}
	return result, nil
}

func fromNative(path string, info av.Info) Info {
	stream := Stream{Index: info.Index, CodecType: "audio", Codec: info.Codec, Profile: info.Profile,
		SampleRate: strconv.Itoa(info.Rate), Channels: info.Channels, Layout: info.Layout,
		RawBits: strconv.Itoa(info.Bits)}
	return Info{Path: path, Stream: stream, Rate: info.Rate, Bits: info.Bits, Language: info.Language, DelayMS: info.DelayMS,
		ContentEncodingUnsupported: info.ContentEncodingUnsupported}
}

// Track adds display metadata to a probe result. Duration is seconds and may be
// a container-level estimate; it is never used to verify encoded sample counts.
type Track struct {
	Info     Info    `json:"info"`
	Language string  `json:"language"`
	Title    string  `json:"title"`
	Default  bool    `json:"default"`
	Duration float64 `json:"duration"`
}

// ProbeTracks inventories audio, including unsupported codecs. Conversion
// eligibility is a separate policy so the UI can explain disabled selections.
func (b Backend) ProbeTracks(ctx context.Context, path string) ([]Track, error) {
	var found []av.Track
	err := probeWithRetry(ctx, path, func(ctx context.Context) error {
		var err error
		path, err = localFile(path)
		if err != nil {
			return err
		}
		found, err = av.ProbeTracks(ctx, path)
		return err
	})
	if err != nil {
		return nil, err
	}
	tracks := make([]Track, 0, len(found))
	for _, track := range found {
		tracks = append(tracks, Track{Info: fromNative(path, track.Info), Language: track.Language,
			Title: track.Title, Default: track.Default, Duration: track.Duration})
	}
	return tracks, nil
}

// ProbeStream selects exactly one global audio index without choosing a
// replacement when that stream is absent or points to video/subtitles.
func (b Backend) ProbeStream(ctx context.Context, path string, index int) (result Info, err error) {
	if index >= 0 {
		ctx = diagnostic.WithStream(ctx, index)
	}
	err = probeWithRetry(ctx, path, func(ctx context.Context) error {
		path, err = localFile(path)
		if err != nil {
			return err
		}
		info, probeErr := av.ProbeStream(ctx, path, index)
		if probeErr != nil {
			return probeErr
		}
		result = fromNative(path, info)
		return nil
	})
	if err != nil {
		return Info{}, err
	}
	return result, nil
}

// ValidateEAC3 applies the source and output policy used by conversion.
// kbps controls MA output only; ordinary DTS always uses 1024 kb/s.
func (i Info) ValidateEAC3(kbps int) error {
	if err := i.ValidateInput(); err != nil {
		return err
	}
	_, err := eac3Bitrate(i, kbps)
	return err
}

func (i Info) ordinaryDTS() bool {
	return i.Stream.Codec == "dts" && i.Stream.Profile == "DTS"
}

// ValidateInput recognizes the union of supported sources. Output-specific
// checks must still run: ordinary DTS is never eligible for TrueHD.
// Its lossy decoder representation does not establish an effective bit depth.
func (i Info) ValidateInput() error {
	if !i.ordinaryDTS() || i.ContentEncodingUnsupported {
		return i.ValidateSource()
	}
	if i.Rate != 48000 || i.Stream.Channels != 6 || i.Stream.Layout != "5.1(side)" {
		return &UnsupportedError{Code: "dts_format", Message: "ordinary DTS supports 48000 Hz 5.1(side) only; no resampling/remapping"}
	}
	return nil
}

// ValidateAC3 requires 48 kHz 5.1(side) without remapping. The encoder fixes
// bitrate at 640 kb/s; there is no user-selectable AC-3 bitrate.
func (i Info) ValidateAC3() error {
	if err := i.ValidateInput(); err != nil {
		return err
	}
	if i.Rate != 48000 || i.Stream.Channels != 6 || i.Stream.Layout != "5.1(side)" {
		return &UnsupportedError{Code: "ac3_format", Message: "AC-3 supports 48000 Hz 5.1(side) only, at 640 kb/s; no resampling/remapping"}
	}
	return nil
}

// ValidateOutput is shared by conversion and preflight; selecting several
// formats requires each one to pass, without silently dropping an output.
// kbps applies only to E-AC-3; "thd" is the output suffix for TrueHD.
func (i Info) ValidateOutput(format string, kbps int) error {
	if i.ContentEncodingUnsupported {
		return i.ValidateSource()
	}
	switch format {
	case "truehd", "thd":
		if i.ordinaryDTS() {
			return &UnsupportedError{Code: "truehd_source", Message: "ordinary DTS is lossy and cannot be converted to TrueHD; choose E-AC-3 or AC-3"}
		}
		return i.ValidateSource()
	case "eac3":
		return i.ValidateEAC3(kbps)
	case "ac3":
		return i.ValidateAC3()
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

// ValidateSource retains the MA-only source contract for lossless TrueHD.
// Use ValidateInput for inventory and ValidateOutput for format eligibility.
func (i Info) ValidateSource() error {
	if i.ContentEncodingUnsupported {
		return &UnsupportedError{Code: "content_encoding", Message: "selected MKV audio uses unsupported content encoding (compression/encryption or encoding chain)"}
	}
	// Match confirmed lossless profiles exactly; a generic "DTS:X" label is
	// not proof of MA. Accepted X profiles still use only compatible channels,
	// without parsing objects or rendering the spatial extension.
	if i.Stream.Codec != "dts" || (i.Stream.Profile != "DTS-HD MA" &&
		i.Stream.Profile != "DTS-HD MA + DTS:X" && i.Stream.Profile != "DTS-HD MA + DTS:X IMAX") {
		code := "codec"
		if i.Stream.Codec == "dts" {
			code = "profile"
		}
		return &UnsupportedError{Code: code, Message: fmt.Sprintf("track %d: expected confirmed DTS-HD MA (including DTS:X/IMAX compatible channels), got codec=%q profile=%q; DTS core/HRA are not lossless MA", i.Stream.Index, i.Stream.Codec, i.Stream.Profile)}
	}
	return i.validateFormat()
}

func (i Info) validateFormat() error {
	// Channel counts alone do not identify speaker positions: rear 5.1 and
	// side 5.1 must remain distinct, and 7.1(wide) is not canonical 7.1.
	channels := 0
	switch i.Stream.Layout {
	case "5.1(side)", "5.1":
		channels = 6
	case "7.1":
		channels = 8
	default:
		return &UnsupportedError{Code: "layout", Message: fmt.Sprintf("unsupported or unknown channel layout %q; need 5.1(side), 5.1 or 7.1; no automatic remapping/downmix", i.Stream.Layout)}
	}
	if i.Stream.Channels != channels {
		return &UnsupportedError{Code: "channels", Message: fmt.Sprintf("layout %s disagrees with channel count %d", i.Stream.Layout, i.Stream.Channels)}
	}
	switch i.Rate {
	case 44100, 48000, 88200, 96000, 176400, 192000:
	default:
		return &UnsupportedError{Code: "rate", Message: fmt.Sprintf("unsupported sample rate %d; resampling is disabled", i.Rate)}
	}
	if i.Bits != 16 && i.Bits != 24 {
		return &UnsupportedError{Code: "bits", Message: fmt.Sprintf("unsupported or unknown effective bit depth %d (need 16 or 24)", i.Bits)}
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
	// TrueHD may report 24 effective bits for a 16-bit source. Widening is
	// allowed; full PCM comparison still requires the added low bits to be zero.
	if output.Bits < source.Bits || output.Bits > 24 {
		return fmt.Errorf("output effective precision %d cannot preserve source precision %d", output.Bits, source.Bits)
	}
	return nil
}
