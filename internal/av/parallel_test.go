package av

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"
)

func TestParallelEncodingSameSource(t *testing.T) {
	path, info := fixture(t)
	var original bytes.Buffer
	if err := Process(t.Context(), path, "", info, &original); err != nil {
		t.Fatal(err)
	}
	for i := range 8 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			output := filepath.Join(t.TempDir(), "output.thd")
			lossy := i%2 != 0
			expectedSamples := uint64(48001)
			if lossy {
				output = filepath.Join(t.TempDir(), "output.eac3")
				stats, err := EncodeEAC3(t.Context(), path, output, info, 1536000)
				if err != nil {
					t.Fatal(err)
				}
				expectedSamples = (48001 + 256 + 1535) / 1536 * 1536
				if stats.Samples != 48001 || stats.Decoded != expectedSamples {
					t.Fatalf("crossed encoder lengths: %+v", stats)
				}
			} else if err := Process(t.Context(), path, output, info, nil); err != nil {
				t.Fatal(err)
			}
			encoded, err := Probe(t.Context(), output)
			if err != nil {
				t.Fatal(err)
			}
			if encoded.Channels != info.Channels || encoded.Layout != info.Layout || encoded.Rate != info.Rate {
				t.Fatalf("crossed output metadata: %+v", encoded)
			}
			if lossy {
				encoded.Bits = 32
			}
			var pcm bytes.Buffer
			if err := Process(t.Context(), output, "", encoded, &pcm); err != nil {
				t.Fatal(err)
			}
			if uint64(pcm.Len()) != expectedSamples*uint64(info.Channels)*4 {
				t.Fatalf("wrong decoded length: %d bytes", pcm.Len())
			}
			if !lossy && !bytes.Equal(pcm.Bytes(), original.Bytes()) {
				t.Fatal("parallel TrueHD output lost PCM equality")
			}
		})
	}
}
