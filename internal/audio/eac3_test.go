package audio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ninghaofeng/hdma2truehd/internal/av"
)

func TestEAC3RejectsDamagedOutput(t *testing.T) {
	dir := t.TempDir()
	path, output := filepath.Join(dir, "source.wav"), filepath.Join(dir, "output.eac3")
	if err := av.WritePCMFixture(path, pcmData(8, 4801),
		av.Info{Rate: 48000, Bits: 24, Channels: 8, Layout: "7.1"}); err != nil {
		t.Fatal(err)
	}
	source, err := (Backend{}).Probe(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	stats, err := av.EncodeEAC3(t.Context(), path, output, nativeInfo(source), 1536000)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := (Backend{}).Probe(t.Context(), output)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateEAC3(t.Context(), encoded, stats); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"missing access unit", "partial dependent", "CRC"} {
		t.Run(mode, func(t *testing.T) {
			changed := append([]byte(nil), data...)
			switch mode {
			case "missing access unit":
				// At 1536 kb/s and 48 kHz, one paired 1536-sample unit is
				// 6144 bytes. Removing it leaves valid framing but a short stream.
				changed = changed[:len(changed)-6144]
			case "partial dependent":
				changed = changed[:len(changed)-1]
			case "CRC":
				// Leave framing intact so payload corruption exercises CRC checks.
				changed[len(changed)-100] ^= 1
			}
			target := filepath.Join(t.TempDir(), "damaged.eac3")
			if err := os.WriteFile(target, changed, 0600); err != nil {
				t.Fatal(err)
			}
			info := encoded
			info.Path = target
			if err := validateEAC3(t.Context(), info, stats); err == nil {
				t.Fatal("accepted damaged/truncated E-AC-3")
			}
		})
	}
}

func TestEAC3Settings(t *testing.T) {
	for _, tt := range []struct {
		layout           string
		rate, kbps, want int
	}{
		{"7.1", 48000, 0, 1536000},
		{"7.1", 48000, 768, 768000},
		{"7.1", 48000, 640, 0},
		{"7.1", 48000, 1537, 0},
		{"7.1", 48000, -1, 0},
		{"7.1", 96000, 0, 0},
		{"7.1", 44100, 0, 0},
		{"5.1(side)", 48000, 0, 960000},
		{"5.1(side)", 48000, 640, 640000},
		{"5.1(side)", 48000, 384, 384000},
		{"5.1(side)", 48000, 1024, 1024000},
		{"5.1(side)", 48000, 1536, 0},
		{"5.1", 48000, 0, 0},
	} {
		info := sourceInfo()
		info.Rate, info.Stream.Layout = tt.rate, tt.layout
		actual, err := eac3Bitrate(info, tt.kbps)
		if (tt.want == 0) != (err != nil) || actual != tt.want {
			t.Fatalf("%+v: %d %v", tt, actual, err)
		}

	}
	source := sourceInfo()
	encoded := source
	encoded.Stream.Codec = "eac3"
	if err := matchingEAC3(source, encoded); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Info){
		func(i *Info) { i.Stream.Codec = "ac3" },
		func(i *Info) { i.Stream.Layout = "5.1(side)" },
		func(i *Info) { i.Stream.Channels = 6 },
		func(i *Info) { i.Rate = 44100 },
	} {
		changed := encoded
		change(&changed)
		if err := matchingEAC3(source, changed); err == nil {
			t.Fatal("accepted changed E-AC-3 output")
		}
	}
}
