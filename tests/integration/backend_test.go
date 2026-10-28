package integration

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ninghaofeng/hdma2truehd/internal/audio"
	"github.com/ninghaofeng/hdma2truehd/internal/av"
)

func TestTrueHDRoundTrip(t *testing.T) {
	backend := audio.Backend{}
	decoder := os.Getenv("HDMA_TEST_DECODER")
	for _, layout := range []string{"5.1(side)", "5.1", "7.1"} {
		channels := 6
		if layout == "7.1" {
			channels = 8
		}
		for _, rate := range []int{44100, 48000, 88200, 96000, 176400, 192000} {
			for _, bits := range []int{16, 24} {
				for _, count := range []int{1, 39, 40, 41, 639, 640, 641, 4801} {
					t.Run(fmt.Sprintf("%s/%dHz/%dbit/%dsamples", layout, rate, bits, count), func(t *testing.T) {
						dir := t.TempDir()
						input := make([]byte, count*channels*4)
						var state uint32 = 1234567
						for i := 0; i < count; i++ {
							for ch := 0; ch < channels; ch++ {
								state = state*1664525 + 1013904223
								sample := int32(state) >> (32 - bits)
								switch i % 17 {
								case 0:
									sample = 0
								case 1:
									sample = (1 << (bits - 1)) - 1
								case 2:
									sample = -(1 << (bits - 1))
								case 3:
									sample = int32(ch + 1)
								}
								sample <<= 32 - bits
								binary.LittleEndian.PutUint32(input[(i*channels+ch)*4:], uint32(sample))
							}
						}
						source := filepath.Join(dir, "source.wav")
						output := filepath.Join(dir, "encoded.thd")
						decoded := filepath.Join(dir, "decoded.pcm")
						if err := av.WritePCMFixture(source, input, av.Info{Rate: rate, Bits: bits, Channels: channels, Layout: layout}); err != nil {
							t.Fatal(err)
						}
						sourceInfo, err := backend.Probe(t.Context(), source)
						if err != nil {
							t.Fatal(err)
						}
						if err := backend.Encode(t.Context(), sourceInfo, output); err != nil {
							t.Fatal(err)
						}
						info, err := backend.Probe(t.Context(), output)
						if err != nil {
							t.Fatal(err)
						}
						if info.Stream.Codec != "truehd" || info.Stream.Layout != layout || info.Stream.Channels != channels ||
							info.Rate != rate || info.Bits < bits {
							t.Fatalf("output metadata differs: %+v", info)
						}
						var pcm bytes.Buffer
						if err := av.Process(t.Context(), output, "", av.Info{Index: info.Stream.Index,
							Rate: info.Rate, Bits: info.Bits, Channels: channels, Layout: layout}, &pcm); err != nil {
							t.Fatal(err)
						}
						actual := pcm.Bytes()
						if !bytes.Equal(input, actual) {
							first := 0
							for first < len(input) && first < len(actual) && input[first] == actual[first] {
								first++
							}
							t.Fatalf("PCM mismatch: source=%d bytes decoded=%d bytes first difference=%d", len(input), len(actual), first)
						}
						if decoder != "" {
							run(t, decoder, "-v", "error", "-xerror", "-f", "truehd", "-i", output,
								"-c:a", "pcm_s32le", "-f", "s32le", decoded)
							actual, err := os.ReadFile(decoded)
							if err != nil {
								t.Fatal(err)
							}
							if !bytes.Equal(input, actual) {
								t.Fatal("external decoder PCM differs")
							}
						}
					})
				}
			}
		}
	}
}

func run(t *testing.T, name string, args ...string) {
	t.Helper()
	if output, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, output)
	}
}
