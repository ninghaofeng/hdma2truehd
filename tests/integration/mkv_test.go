package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ninghaofeng/hdma2truehd/internal/audio"
	"github.com/ninghaofeng/hdma2truehd/internal/diagnostic"
)

func TestMKVTrackMetadata(t *testing.T) {
	encoder := os.Getenv("HDMA_TEST_DECODER")
	if encoder == "" {
		t.Skip("set HDMA_TEST_DECODER")
	}
	for _, delay := range []int64{0, 125, -125} {
		t.Run(fmt.Sprint(delay), func(t *testing.T) {
			movie := filepath.Join(t.TempDir(), "test_video.mkv")
			command := exec.CommandContext(t.Context(), encoder, "-v", "error",
				"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=1",
				"-f", "lavfi", "-i", "color=c=black:s=16x16:r=10:d=1",
				"-map", "1:v:0", "-map", "0:a:0", "-t", "1",
				"-af", fmt.Sprintf("asetpts=PTS%+.3f/TB", float64(delay)/1000),
				"-c:v", "mpeg4", "-c:a", "pcm_s16le", "-avoid_negative_ts", "disabled",
				"-metadata:s:a:0", "language=eng", movie)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("fixture: %v\n%s", err, output)
			}
			b := audio.Backend{}
			tracks, err := b.ProbeTracks(t.Context(), movie)
			if err != nil || len(tracks) != 1 {
				t.Fatalf("%+v %v", tracks, err)
			}
			selected, err := b.ProbeStream(t.Context(), movie, 1)
			if err != nil {
				t.Fatal(err)
			}
			for _, info := range []audio.Info{tracks[0].Info, selected} {
				want := fmt.Sprintf("test_video_track1_[eng]_DELAY %dms", delay)
				if info.Language != "eng" || info.DelayMS != delay || info.OutputBase() != want {
					t.Errorf("metadata/naming: %+v, want %s", info, want)
				}
			}
		})
	}
}

// The external executable only constructs a local test fixture; production
// enumeration and selected-track conversion must remain entirely in-process.
func TestMKVSelectedTracks(t *testing.T) {
	source, encoder := os.Getenv("HDMA_TEST_DTS_51"), os.Getenv("HDMA_TEST_DECODER")
	if source == "" || encoder == "" {
		t.Skip("set HDMA_TEST_DTS_51 and HDMA_TEST_DECODER")
	}
	movie := filepath.Join(t.TempDir(), "two-tracks.mkv")
	subtitles := filepath.Join(t.TempDir(), "captions.srt")
	attachment := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(attachment, []byte("Unrelated attachment"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(subtitles, []byte("1\n00:00:00,000 --> 00:00:00,900\nCaption\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Put video first and unsupported AAC before DTS: audio ordinal 1 must
	// select container index 2. Add subtitles and an unknown-codec attachment
	// to catch stream-info warnings from tracks outside the audio probe scope.
	command := exec.CommandContext(t.Context(), encoder, "-v", "error", "-i", source,
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=1",
		"-f", "lavfi", "-i", "color=c=black:s=16x16:r=1:d=1",
		"-i", subtitles,
		"-map", "2:v:0", "-map", "1:a:0", "-map", "0:a:0", "-map", "3:s:0", "-t", "1",
		"-c:v", "mpeg4", "-c:s", "srt", "-c:a:0", "aac", "-c:a:1", "copy",
		"-attach", attachment, "-metadata:s:t:0", "mimetype=text/plain",
		"-metadata:s:a:0", "language=fra", "-metadata:s:a:1", "language=eng",
		"-metadata:s:a:1", "title=Selected DTS-HD MA", movie)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v\n%s", err, output)
	}
	b := audio.Backend{}
	var diagnostics []diagnostic.Event
	ctx := diagnostic.WithSink(t.Context(), func(event diagnostic.Event) {
		diagnostics = append(diagnostics, event)
	})
	ctx = diagnostic.WithJob(ctx, movie, "", 0)
	tracks, err := b.ProbeTracks(ctx, movie)
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
	for _, nonAudio := range []int{0, 3, 4} {
		if _, err := b.ProbeStream(t.Context(), movie, nonAudio); err == nil {
			t.Fatalf("accepted non-audio stream %d", nonAudio)
		}
	}
	sourceInfo, err := b.ProbeStream(ctx, movie, index)
	if err != nil {
		t.Fatal(err)
	}
	if sourceInfo.Language != tracks[1].Info.Language || sourceInfo.DelayMS != tracks[1].Info.DelayMS {
		t.Fatalf("inventory/selection timing mismatch: %+v / %+v", tracks[1].Info, sourceInfo)
	}
	original, err := b.Digest(ctx, sourceInfo)
	if err != nil {
		t.Fatal(err)
	}
	for _, extension := range []string{".thd", ".eac3", ".ac3"} {
		output := filepath.Join(t.TempDir(), "output"+extension)
		result, err := b.Convert(ctx, movie, output, audio.Options{StreamIndex: &index})
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
	for _, event := range diagnostics {
		// Clean source probing/decoding must not emit video/subtitle codec
		// warnings. Raw output duration/probe diagnostics remain legitimate.
		if event.Code == "ffmpeg" && event.Input == movie && event.Stage != "check_output" && event.Stage != "verify_output" {
			t.Errorf("unrelated track diagnostic: %+v", event)
		}
	}
}
