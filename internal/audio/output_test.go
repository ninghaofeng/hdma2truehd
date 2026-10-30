package audio

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ninghaofeng/hdma2truehd/internal/av"
)

func TestOutputBase(t *testing.T) {
	for _, tt := range []struct {
		path, language string
		index          int
		delay          int64
		want           string
	}{
		{"test_video.mkv", "eng", 2, 0, "test_video_track2_[eng]_DELAY 0ms"},
		{"movie.part1.MKV", " JPN ", 4, 125, "movie.part1_track4_[jpn]_DELAY 125ms"},
		{"movie.mkv", "zh", 0, -250, "movie_track0_[zh]_DELAY -250ms"},
		{"movie.mkv", "", 0, 0, "movie_track0_[und]_DELAY 0ms"},
		{"movie.mkv", "../../bad", 2, 0, "movie_track2_[und]_DELAY 0ms"},
		{"movie.mkv", "e/g", 2, 0, "movie_track2_[und]_DELAY 0ms"},
		{"movie.mkv", "eng\\bad", 2, 0, "movie_track2_[und]_DELAY 0ms"},
		{"movie.mkv", "e\x00g", 2, 0, "movie_track2_[und]_DELAY 0ms"},
		{"movie.mkv", "english", 2, 0, "movie_track2_[und]_DELAY 0ms"},
		{"movie.dts", "eng", 2, 125, "movie"},
		{"movie.dtshd", "", 0, 0, "movie"},
	} {
		info := Info{Path: filepath.Join(t.TempDir(), tt.path), Stream: Stream{Index: tt.index}, Language: tt.language, DelayMS: tt.delay}
		if got := info.OutputBase(); got != tt.want {
			t.Errorf("%+v: %q, want %q", tt, got, tt.want)
		}
	}
}

func TestOutputPolicies(t *testing.T) {
	for _, test := range []struct {
		profile, layout   string
		rate, channels    int
		truehd, eac3, ac3 bool
	}{
		{"DTS-HD MA", "5.1(side)", 48000, 6, true, true, true},
		{"DTS-HD MA", "7.1", 48000, 8, true, true, false},
		{"DTS-HD MA", "5.1", 48000, 6, true, false, false},
		{"DTS-HD MA", "5.1(side)", 96000, 6, true, false, false},
		{"DTS-HD MA + DTS:X", "5.1(side)", 48000, 6, true, true, true},
		{"DTS", "5.1(side)", 48000, 6, false, true, true},
		{"DTS", "7.1", 48000, 8, false, false, false},
		{"DTS", "5.1", 48000, 6, false, false, false},
		{"DTS", "5.1(side)", 44100, 6, false, false, false},
		{"DTS", "5.1(side)", 48000, 8, false, false, false},
		{"DTS-HD HRA", "5.1(side)", 48000, 6, false, false, false},
		{"", "5.1(side)", 48000, 6, false, false, false},
		{"DTS:X", "5.1(side)", 48000, 6, false, false, false},
	} {
		info := sourceInfo()
		info.Stream.Profile, info.Stream.Layout = test.profile, test.layout
		info.Rate, info.Stream.Channels = test.rate, test.channels
		for format, want := range map[string]bool{"thd": test.truehd, "eac3": test.eac3, "ac3": test.ac3} {
			if err := info.ValidateOutput(format, 0); (err == nil) != want {
				t.Fatalf("%+v -> %s: %v", test, format, err)
			}
		}
	}
	core := sourceInfo()
	core.Stream.Profile, core.Stream.Layout, core.Stream.Channels, core.Bits = "DTS", "5.1(side)", 6, 0
	for _, selected := range []int{0, 384, 640, 960, 1024} {
		if rate, err := eac3Bitrate(core, selected); err != nil || rate != 1024000 {
			t.Fatalf("core bitrate %d: %d %v", selected, rate, err)
		}
	}
	if err := core.ValidateInput(); err != nil {
		t.Fatal("lossy source precision should not imply MA precision: ", err)
	}
}

func TestAC3RejectsDamagedOutput(t *testing.T) {
	dir := t.TempDir()
	path, output := filepath.Join(dir, "source.wav"), filepath.Join(dir, "output.ac3")
	if err := av.WritePCMFixture(path, pcmData(6, 4801),
		av.Info{Rate: 48000, Bits: 24, Channels: 6, Layout: "5.1(side)"}); err != nil {
		t.Fatal(err)
	}
	b := Backend{}
	source, err := b.Probe(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	stats, err := av.EncodeAC3(t.Context(), path, output, nativeInfo(source))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := b.Probe(t.Context(), output)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateLossy(t.Context(), encoded, stats); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"missing frame", "partial frame", "CRC"} {
		t.Run(mode, func(t *testing.T) {
			changed := append([]byte(nil), data...)
			switch mode {
			case "missing frame":
				changed = changed[:len(changed)-2560]
			case "partial frame":
				changed = changed[:len(changed)-1]
			case "CRC":
				changed[len(changed)-100] ^= 1
			}
			target := filepath.Join(t.TempDir(), "damaged.ac3")
			if err := os.WriteFile(target, changed, 0600); err != nil {
				t.Fatal(err)
			}
			info := encoded
			info.Path = target
			if err := validateLossy(t.Context(), info, stats); err == nil {
				t.Fatal("accepted damaged AC-3")
			}
		})
	}
}

func TestLossyOnlyPolicyBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	input, output := filepath.Join(dir, "core.dts"), filepath.Join(dir, "output.thd")
	if err := os.WriteFile(input, []byte("not decoded"), 0600); err != nil {
		t.Fatal(err)
	}
	b := recoveryBackend(input, nil)
	b.probe = func(_ context.Context, path string) (Info, error) {
		info := sourceInfo()
		info.Path, info.Stream.Profile, info.Stream.Layout, info.Stream.Channels = path, "DTS", "5.1(side)", 6
		return info, nil
	}
	if _, err := b.Convert(t.Context(), input, output, Options{}); err == nil {
		t.Fatal("ordinary DTS accepted for TrueHD")
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("policy rejection published output")
	}
}
