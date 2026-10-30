package av

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestAC3RoundTrip(t *testing.T) {
	for _, bits := range []int{16, 24} {
		for _, count := range []int{1, 255, 256, 1535, 1536, 1537, 96000} {
			t.Run(fmt.Sprintf("%dbit/%dsamples", bits, count), func(t *testing.T) {
				dir := t.TempDir()
				source, output := filepath.Join(dir, "source.wav"), filepath.Join(dir, "output.ac3")
				pcm := make([]byte, count*6*4)
				frequencies := []float64{379, 499, 619, 59, 751, 877}
				for n := range count {
					for ch := range 6 {
						value := int32(.1 * 2147483648 * math.Sin(2*math.Pi*frequencies[ch]*float64(n)/48000))
						value = value >> (32 - bits) << (32 - bits)
						binary.LittleEndian.PutUint32(pcm[(n*6+ch)*4:], uint32(value))
					}
				}
				if err := WritePCMFixture(source, pcm, Info{Rate: 48000, Bits: bits, Channels: 6, Layout: "5.1(side)"}); err != nil {
					t.Fatal(err)
				}
				info, err := Probe(t.Context(), source)
				if err != nil {
					t.Fatal(err)
				}
				stats, err := EncodeAC3(t.Context(), source, output, info)
				if err != nil {
					t.Fatal(err)
				}
				expected := uint64((count + 256 + 1535) / 1536 * 1536)
				if stats.Samples != uint64(count) || stats.Decoded != expected {
					t.Fatalf("length: %+v, want %d", stats, expected)
				}
				data, err := os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
				if uint64(len(data))*8*48000 != 640000*expected {
					t.Fatal("AC-3 bitrate is not exactly 640 kb/s")
				}
				encoded, err := Probe(t.Context(), output)
				if err != nil {
					t.Fatal(err)
				}
				if encoded.Codec != "ac3" || encoded.Rate != 48000 || encoded.Channels != 6 || encoded.Layout != "5.1(side)" {
					t.Fatalf("format: %+v", encoded)
				}
				encoded.Bits = 32
				var decoded bytes.Buffer
				if err := Process(t.Context(), output, "", encoded, &decoded); err != nil {
					t.Fatal(err)
				}
				if uint64(decoded.Len()) != expected*6*4 {
					t.Fatalf("decoded bytes: %d", decoded.Len())
				}
				if count == 96000 {
					for ch := range 6 {
						var signal, noise float64
						for n := 1024; n < count-1024; n++ {
							wanted := float64(int32(binary.LittleEndian.Uint32(pcm[(n*6+ch)*4:]))) / 2147483648
							actual := float64(int32(binary.LittleEndian.Uint32(decoded.Bytes()[((n+256)*6+ch)*4:]))) / 2147483648
							signal += wanted * wanted
							noise += (actual - wanted) * (actual - wanted)
						}
						// Detect routing regressions, not perceptual certification.
						if snr := 10 * math.Log10(signal/noise); math.IsNaN(snr) || snr < 25 {
							t.Fatalf("channel %d SNR %.2f dB", ch, snr)
						}
					}
				}
			})
		}
	}
}

func TestAC3RejectsUnsupportedAndCancelled(t *testing.T) {
	path, info := fixture(t)
	if _, err := EncodeAC3(t.Context(), path, filepath.Join(t.TempDir(), "out.ac3"), info); err == nil {
		t.Fatal("accepted 7.1 AC-3")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := EncodeAC3(ctx, path, filepath.Join(t.TempDir(), "out.ac3"), info); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", err)
	}
	source := filepath.Join(t.TempDir(), "51.wav")
	if err := WritePCMFixture(source, make([]byte, 48000*6*4), Info{Rate: 48000, Bits: 24, Channels: 6, Layout: "5.1(side)"}); err != nil {
		t.Fatal(err)
	}
	info, err := Probe(t.Context(), source)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(t.Context())
	defer cancel()
	ctx = WithProgress(ctx, func(Progress) { cancel() })
	if _, err := EncodeAC3(ctx, source, filepath.Join(t.TempDir(), "cancelled.ac3"), info); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost in-flight AC-3 cancellation: %v", err)
	}
}
