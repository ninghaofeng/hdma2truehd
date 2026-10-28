package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ninghaofeng/hdma2truehd/internal/audio"
)

// The external executable only constructs a local test fixture; production
// enumeration and selected-track conversion must remain entirely in-process.
func TestMKVSelectedTracks(t *testing.T) {
	source, encoder := os.Getenv("HDMA_TEST_DTS_51"), os.Getenv("HDMA_TEST_DECODER")
	if source == "" || encoder == "" {
		t.Skip("set HDMA_TEST_DTS_51 and HDMA_TEST_DECODER")
	}
	movie := filepath.Join(t.TempDir(), "two-tracks.mkv")
	command := exec.CommandContext(t.Context(), encoder, "-v", "error", "-i", source,
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=1",
		"-f", "lavfi", "-i", "color=c=black:s=16x16:r=1:d=1",
		"-map", "2:v:0", "-map", "1:a:0", "-map", "0:a:0", "-t", "1",
		"-c:v", "ffv1", "-c:a:0", "aac", "-c:a:1", "copy",
		"-metadata:s:a:0", "language=fra", "-metadata:s:a:1", "language=eng",
		"-metadata:s:a:1", "title=Selected DTS-HD MA", movie)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v\n%s", err, output)
	}
	b := audio.Backend{}
	tracks, err := b.ProbeTracks(t.Context(), movie)
	if err != nil || len(tracks) != 2 {
		t.Fatalf("%+v %v", tracks, err)
	}
	if tracks[0].Info.Stream.Codec != "aac" || tracks[1].Language != "eng" || tracks[1].Title != "Selected DTS-HD MA" {
		t.Fatalf("%+v", tracks)
	}
	if _, err := b.Probe(t.Context(), movie); err == nil {
		t.Fatal("implicitly selected a container track")
	}
	if _, err := b.ProbeStream(t.Context(), movie, 99); err == nil {
		t.Fatal("accepted absent stream")
	}
	if _, err := b.ProbeStream(t.Context(), movie, -1); err == nil {
		t.Fatal("accepted negative stream")
	}
	if err := tracks[0].Info.ValidateSource(); err == nil {
		t.Fatal("accepted AAC")
	}
	index := tracks[1].Info.Stream.Index
	if index != 2 {
		t.Fatalf("audio-list index confused with container index: %d", index)
	}
	if _, err := b.ProbeStream(t.Context(), movie, 0); err == nil {
		t.Fatal("accepted a video stream")
	}
	sourceInfo, err := b.ProbeStream(t.Context(), movie, index)
	if err != nil {
		t.Fatal(err)
	}
	original, err := b.Digest(t.Context(), sourceInfo)
	if err != nil {
		t.Fatal(err)
	}
	for _, extension := range []string{".thd", ".eac3"} {
		output := filepath.Join(t.TempDir(), "output"+extension)
		result, err := b.Convert(t.Context(), movie, output, audio.Options{StreamIndex: &index})
		if err != nil {
			t.Fatal(err)
		}
		if result.Samples != original.Samples {
			t.Fatalf("wrong selected stream samples: %+v", result)
		}
		if extension == ".thd" {
			encoded, err := b.Probe(t.Context(), output)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := b.Digest(t.Context(), encoded)
			if err != nil {
				t.Fatal(err)
			}
			if err := audio.ComparePCM(original, decoded); err != nil {
				t.Fatal(err)
			}
		} else if !result.Validated || !result.Lossy || result.Verified {
			t.Fatalf("%+v", result)
		}
	}
}
