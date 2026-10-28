package av

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestEAC3RoundTrip(t *testing.T) {
	for _, channels := range []int{6, 8} {
		for _, bits := range []int{16, 24} {
			for _, count := range []int{1, 255, 256, 1535, 1536, 1537, 96000} {
				t.Run(fmt.Sprintf("%dch/%dbit/%dsamples", channels, bits, count), func(t *testing.T) {
					layout, bitrate := "5.1(side)", 960000
					if channels == 8 {
						layout, bitrate = "7.1", 1536000
					}
					dir := t.TempDir()
					source, output := filepath.Join(dir, "source.wav"), filepath.Join(dir, "output.eac3")
					pcm := make([]byte, count*channels*4)
					frequencies := []float64{379, 499, 619, 59, 751, 877, 1009, 1151}
					for n := 0; n < count; n++ {
						for ch := 0; ch < channels; ch++ {
							value := int32(0.1 * 2147483648 * math.Sin(2*math.Pi*frequencies[ch]*float64(n)/48000))
							value = value >> (32 - bits) << (32 - bits)
							binary.LittleEndian.PutUint32(pcm[(n*channels+ch)*4:], uint32(value))
						}
					}
					info := Info{Rate: 48000, Bits: bits, Channels: channels, Layout: layout}
					if err := WritePCMFixture(source, pcm, info); err != nil {
						t.Fatal(err)
					}
					probed, err := Probe(t.Context(), source)
					if err != nil {
						t.Fatal(err)
					}
					stats, err := EncodeEAC3(t.Context(), source, output, probed, bitrate)
					if err != nil {
						t.Fatal(err)
					}
					expected := uint64((count + 256 + 1535) / 1536 * 1536)
					if stats.Samples != uint64(count) || stats.Decoded != expected {
						t.Fatalf("wrong length: %+v, want %d", stats, expected)
					}
					encoded, err := Probe(t.Context(), output)
					if err != nil {
						t.Fatal(err)
					}
					if encoded.Codec != "eac3" || encoded.Layout != layout || encoded.Channels != channels || encoded.Rate != 48000 {
						t.Fatalf("wrong output format: %+v", encoded)
					}
					encoded.Bits = 32
					var decoded bytes.Buffer
					if err := Process(t.Context(), output, "", encoded, &decoded); err != nil {
						t.Fatal(err)
					}
					if decoded.Len() != int(expected)*channels*4 {
						t.Fatalf("decoded %d bytes", decoded.Len())
					}
					if count == 96000 {
						// Distinct tones detect surround replacement/ordering
						// errors that a successful 7.1 probe cannot detect.
						// Ignore edge transients, but account for the fixed
						// 256-sample MDCT delay instead of searching for a fit.
						for ch := 0; ch < channels; ch++ {
							var signal, noise float64
							for n := 1024; n < count-1024; n++ {
								wanted := float64(int32(binary.LittleEndian.Uint32(pcm[(n*channels+ch)*4:]))) / 2147483648
								actual := float64(int32(binary.LittleEndian.Uint32(decoded.Bytes()[((n+256)*channels+ch)*4:]))) / 2147483648
								signal += wanted * wanted
								noise += (actual - wanted) * (actual - wanted)
							}
							snr := 10 * math.Log10(signal/noise)
							if math.IsNaN(snr) || snr < 25 {
								t.Fatalf("channel %d SNR %.2f dB; wrong route or excessive distortion", ch, snr)
							}
						}
					}
				})
			}
		}
	}
}

func TestEAC3Errors(t *testing.T) {
	path, info := fixture(t)
	for _, change := range []func(*Info, *int){
		func(i *Info, _ *int) { i.Rate = 96000 },
		func(i *Info, _ *int) { i.Layout = "5.1" },
		func(_ *Info, b *int) { *b = -1 },
		func(_ *Info, b *int) { *b = 1536001 },
		func(_ *Info, b *int) { *b = 640000 }, // Too low for 7.1.
	} {
		changed, bitrate := info, 1536000
		change(&changed, &bitrate)
		if _, err := EncodeEAC3(t.Context(), path, filepath.Join(t.TempDir(), "output.eac3"), changed, bitrate); err == nil {
			t.Fatal("accepted invalid E-AC-3 settings")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := EncodeEAC3(ctx, path, filepath.Join(t.TempDir(), "output.eac3"), info, 1536000); err != context.Canceled {
		t.Fatalf("lost cancellation: %v", err)
	}
	if _, err := EncodeEAC3(t.Context(), path, "invalid\x00path", info, 1536000); err == nil {
		t.Fatal("accepted NUL output")
	}
	if _, err := EncodeEAC3(t.Context(), path, filepath.Join(t.TempDir(), "missing", "output.eac3"), info, 1536000); err == nil {
		t.Fatal("accepted missing output parent")
	}
}

func TestEAC3Bitrates(t *testing.T) {
	path, info := fixture(t)
	for _, bitrate := range []int{768000, 800000, 1024000, 1536000} {
		output := filepath.Join(t.TempDir(), "output.eac3")
		stats, err := EncodeEAC3(t.Context(), path, output, info, bitrate)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		if uint64(len(data))*8*48000 != uint64(bitrate)*stats.Decoded {
			t.Fatalf("bitrate mismatch for %d", bitrate)
		}
		encoded, err := Probe(t.Context(), output)
		if err != nil {
			t.Fatal(err)
		}
		encoded.Bits = 32
		var decoded bytes.Buffer
		if err := Process(t.Context(), output, "", encoded, &decoded); err != nil {
			t.Fatal(err)
		}
		if uint64(decoded.Len()) != stats.Decoded*8*4 {
			t.Fatal("decoded bitrate test has wrong duration")
		}
	}
}
