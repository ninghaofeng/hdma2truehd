package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sourceInfo() Info {
	return Info{Path: "source.dts", Rate: 48000, Bits: 24, Stream: Stream{
		Index: 0, CodecType: "audio", Codec: "dts", Profile: "DTS-HD MA",
		SampleRate: "48000", Channels: 8, Layout: "7.1", RawBits: "24",
	}}
}

func TestSourceValidation(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Info)
		want   string
	}{
		{"valid", func(*Info) {}, ""},
		{"core", func(i *Info) { i.Stream.Profile = "DTS" }, "confirmed DTS-HD MA"},
		{"HRA", func(i *Info) { i.Stream.Profile = "DTS-HD HRA" }, "confirmed DTS-HD MA"},
		{"unknown profile", func(i *Info) { i.Stream.Profile = "" }, "confirmed DTS-HD MA"},
		{"unknown DTS:X", func(i *Info) { i.Stream.Profile = "DTS:X" }, "confirmed DTS-HD MA"},
		{"other DTS:X codec", func(i *Info) {
			i.Stream.Codec, i.Stream.Profile = "aac", "DTS-HD MA + DTS:X"
		}, "confirmed DTS-HD MA"},
		{"other codec", func(i *Info) { i.Stream.Codec = "flac" }, "confirmed DTS-HD MA"},
		{"unknown layout", func(i *Info) { i.Stream.Layout = "" }, "layout"},
		{"wide", func(i *Info) { i.Stream.Layout = "7.1(wide)" }, "layout"},
		{"wrong count", func(i *Info) { i.Stream.Channels = 6 }, "disagrees"},
		{"unknown bits", func(i *Info) { i.Bits = 0 }, "bit depth"},
		{"20bit", func(i *Info) { i.Bits = 20 }, "bit depth"},
		{"32bit", func(i *Info) { i.Bits = 32 }, "bit depth"},
		{"bad rate", func(i *Info) { i.Rate = 32000 }, "resampling is disabled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := sourceInfo()
			tt.change(&info)
			err := info.ValidateSource()
			if tt.want == "" && err != nil {
				t.Fatal(err)
			}
			if tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Fatalf("want %q, got %v", tt.want, err)
			}
		})
	}
	for _, profile := range []string{"DTS-HD MA", "DTS-HD MA + DTS:X", "DTS-HD MA + DTS:X IMAX"} {
		for _, layout := range []string{"5.1", "5.1(side)", "7.1"} {
			for _, rate := range []int{44100, 48000, 88200, 96000, 176400, 192000} {
				for _, bits := range []int{16, 24} {
					info := sourceInfo()
					info.Stream.Profile = profile
					info.Stream.Layout = layout
					if layout != "7.1" {
						info.Stream.Channels = 6
					}
					info.Rate, info.Bits = rate, bits
					if err := info.ValidateSource(); err != nil {
						t.Fatalf("%+v: %v", info, err)
					}
				}
			}
		}
	}
}

func pcmData(channels, samples int) []byte {
	data := make([]byte, channels*samples*4)
	for i := 0; i < samples; i++ {
		for ch := 0; ch < channels; ch++ {
			binary.LittleEndian.PutUint32(data[(i*channels+ch)*4:], uint32(i*channels+ch+1)<<8)
		}
	}
	return data
}

func digest(t *testing.T, data []byte, chunks int) PCM {
	t.Helper()
	w := newPCMWriter(8, 24)
	for len(data) > 0 {
		n := min(chunks, len(data))
		if _, err := w.Write(data[:n]); err != nil {
			t.Fatal(err)
		}
		data = data[n:]
	}
	result, err := w.finish()
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestPCMDigest(t *testing.T) {
	data := pcmData(8, 17)
	expected := digest(t, data, len(data))
	// Fragment both int32 words and multichannel frames; callback chunking
	// must not affect channel assignment or the final fingerprints.
	for _, size := range []int{1, 3, 4, 7, 31, 32, 127} {
		if err := ComparePCM(expected, digest(t, data, size)); err != nil {
			t.Fatal(err)
		}
	}
	if err := ComparePCM(expected, digest(t, pcmData(8, 16), 31)); err == nil {
		t.Fatal("accepted truncation")
	}
	changed := bytes.Clone(data)
	// The first byte is padding, so byte 1 contains the lowest effective
	// 24-bit precision. A one-bit change must not survive verification.
	changed[1] ^= 1
	if err := ComparePCM(expected, digest(t, changed, 31)); err == nil {
		t.Fatal("accepted loss of low effective bit")
	}
	swapped := bytes.Clone(data)
	copy(swapped[:4], data[4:8])
	copy(swapped[4:8], data[:4])
	if err := ComparePCM(expected, digest(t, swapped, 31)); err == nil {
		t.Fatal("accepted channel swap")
	}
	for _, data := range [][]byte{nil, {0}, make([]byte, 4)} {
		w := newPCMWriter(8, 24)
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
		if _, err := w.finish(); err == nil {
			t.Fatal("accepted empty/incomplete PCM")
		}
	}
	for _, bits := range []int{16, 24} {
		w := newPCMWriter(8, bits)
		if _, err := w.Write([]byte{1, 0, 0, 0}); err == nil {
			t.Fatal("accepted excess precision")
		}
	}
}

func TestMatchingOutput(t *testing.T) {
	source := sourceInfo()
	output := source
	output.Stream.Codec = "truehd"
	if err := matchingOutput(source, output); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Info){
		func(i *Info) { i.Stream.Codec = "eac3" },
		func(i *Info) { i.Stream.Layout = "7.1(wide)" },
		func(i *Info) { i.Stream.Channels = 6 },
		func(i *Info) { i.Rate = 96000 },
		func(i *Info) { i.Bits = 16 },
		func(i *Info) { i.Bits = 0 },
	} {
		changed := output
		mutate(&changed)
		if err := matchingOutput(source, changed); err == nil {
			t.Fatal("accepted changed output")
		}
	}
}

func TestConvert(t *testing.T) {
	// Inject codec outcomes to isolate publication semantics. Actual native
	// signal fidelity is covered separately by the integration round trips.
	for _, profile := range []string{"DTS-HD MA", "DTS-HD MA + DTS:X", "DTS-HD MA + DTS:X IMAX"} {
		for _, layout := range []string{"5.1", "5.1(side)", "7.1"} {
			t.Run(profile+"/"+layout, func(t *testing.T) {
				for _, mode := range []string{"verified", "skip", "PCM mismatch", "metadata mismatch", "encode failure", "decode failure", "cancel", "exists", "overwrite"} {
					t.Run(mode, func(t *testing.T) {
						dir := t.TempDir()
						input := filepath.Join(dir, "source.dts")
						output := filepath.Join(dir, "result.thd")
						if err := os.WriteFile(input, []byte("fixture placeholder"), 0600); err != nil {
							t.Fatal(err)
						}
						if mode == "exists" || mode == "overwrite" {
							if err := os.WriteFile(output, []byte("old"), 0600); err != nil {
								t.Fatal(err)
							}
						}
						digests := 0
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()
						b := Backend{}
						b.probe = func(_ context.Context, path string) (Info, error) {
							info := sourceInfo()
							info.Path = path
							info.Stream.Profile, info.Stream.Layout = profile, layout
							if layout != "7.1" {
								info.Stream.Channels = 6
							}
							if path != input {
								info.Stream.Codec, info.Stream.Profile = "truehd", ""
								if mode == "metadata mismatch" {
									info.Rate = 96000
								}
							}
							return info, nil
						}
						b.decode = func(_ context.Context, info Info, out io.Writer) error {
							digests++
							if mode == "decode failure" {
								return fmt.Errorf("decode failure")
							}
							data := pcmData(info.Stream.Channels, 17)
							if mode == "PCM mismatch" && digests == 2 {
								data[1] ^= 1
							}
							_, err := out.Write(data)
							return err
						}
						b.encode = func(_ context.Context, _ Info, output string) error {
							if mode == "encode failure" {
								return fmt.Errorf("encode failure")
							}
							if mode == "cancel" {
								cancel()
								return context.Canceled
							}
							return os.WriteFile(output, []byte("encoded"), 0600)
						}
						result, err := b.Convert(ctx, input, output, Options{SkipVerify: mode == "skip", Overwrite: mode == "overwrite"})
						success := mode == "verified" || mode == "skip" || mode == "overwrite"
						if success {
							if err != nil {
								t.Fatal(err)
							}
							if result.Verified != (mode != "skip") {
								t.Fatalf("wrong verification status: %+v", result)
							}
							data, err := os.ReadFile(output)
							if err != nil || string(data) != "encoded" {
								t.Fatalf("output not published: %s %v", data, err)
							}
							if mode == "skip" && digests != 0 {
								t.Fatal("skip performed PCM comparison")
							}
						} else {
							if err == nil {
								t.Fatal("expected failure")
							}
							data, statErr := os.ReadFile(output)
							if mode == "exists" {
								if statErr != nil || string(data) != "old" {
									t.Fatal("existing output changed")
								}
							} else if !errors.Is(statErr, os.ErrNotExist) {
								t.Fatalf("failed output was published: %q %v", data, statErr)
							}
						}
						leftovers, err := filepath.Glob(filepath.Join(dir, ".hdma2truehd-*"))
						// Cleanup is required on success, rejection, codec failure
						// and cancellation, not just on the normal verified path.
						if err != nil || len(leftovers) != 0 {
							t.Fatalf("temporary files remain: %v %v", leftovers, err)
						}
					})
				}
			})
		}
	}
}
