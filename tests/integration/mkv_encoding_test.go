package integration

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ninghaofeng/hdma2truehd/internal/audio"
	"github.com/ninghaofeng/hdma2truehd/internal/diagnostic"
)

func mkvEncodingElement(id []byte, payload ...[]byte) []byte {
	data := bytes.Join(payload, nil)
	if len(data) >= 127 {
		panic("test EBML element exceeds one-byte size")
	}
	return bytes.Join([][]byte{id, {0x80 | byte(len(data))}, data}, nil)
}

// Replace a reserved TrackName with same-sized ContentEncodings + Void.
// Track/segment lengths, seek positions and packet bytes remain unchanged.
func replaceMKVTrackName(t *testing.T, data []byte, marker string, encoding []byte) []byte {
	t.Helper()
	name := mkvEncodingElement([]byte{0x53, 0x6e}, []byte(marker))
	if bytes.Count(data, name) != 1 {
		t.Fatal("reserved TrackName not uniquely found")
	}
	padding := len(name) - len(encoding) - 2
	if padding < 0 {
		t.Fatal("encoding exceeds reserved TrackName")
	}
	replacement := append(encoding, mkvEncodingElement([]byte{0xec}, make([]byte, padding))...)
	return bytes.Replace(data, name, replacement, 1)
}

func TestMKVContentEncodings(t *testing.T) {
	source, encoder := os.Getenv("HDMA_TEST_DTS_51"), os.Getenv("HDMA_TEST_DECODER")
	if source == "" || encoder == "" {
		t.Skip("set HDMA_TEST_DTS_51 and HDMA_TEST_DECODER")
	}
	dir := t.TempDir()
	movie := filepath.Join(dir, "base.mkv")
	subtitles := filepath.Join(dir, "captions.srt")
	if err := os.WriteFile(subtitles, []byte("1\n00:00:00,000 --> 00:00:00,900\nCaption\n"), 0600); err != nil {
		t.Fatal(err)
	}
	markers := []string{
		"video-" + strings.Repeat("v", 90),
		"selected-" + strings.Repeat("a", 87),
		"other-" + strings.Repeat("b", 90),
		"subtitle-" + strings.Repeat("s", 87),
	}
	command := exec.CommandContext(t.Context(), encoder, "-v", "error", "-i", source,
		"-f", "lavfi", "-i", "color=c=black:s=16x16:r=1:d=1", "-i", subtitles,
		"-map", "1:v:0", "-map", "0:a:0", "-map", "0:a:0", "-map", "2:s:0", "-t", "1",
		"-c:v", "mpeg4", "-c:a", "copy", "-c:s", "srt",
		"-metadata:s:0", "title="+markers[0], "-metadata:s:1", "title="+markers[1],
		"-metadata:s:2", "title="+markers[2], "-metadata:s:3", "title="+markers[3], movie)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v\n%s", err, output)
	}
	base, err := os.ReadFile(movie)
	if err != nil {
		t.Fatal(err)
	}
	compression := func(algo byte) []byte {
		return mkvEncodingElement([]byte{0x62, 0x40},
			mkvEncodingElement([]byte{0x50, 0x34}, mkvEncodingElement([]byte{0x42, 0x54}, []byte{algo})))
	}
	for _, tt := range []struct {
		name     string
		stream   int
		encoding []byte
		blocked  bool
	}{
		{"subtitle-zlib", 3, compression(0), false},
		{"subtitle-bzip2", 3, compression(1), false},
		{"video-zlib", 0, compression(0), false},
		{"unselected-audio-zlib", 2, compression(0), false},
		{"selected-audio-zlib", 1, compression(0), true},
		{"selected-audio-encryption", 1, mkvEncodingElement([]byte{0x62, 0x40},
			mkvEncodingElement([]byte{0x50, 0x33}, []byte{1})), true},
		{"selected-audio-chain", 1, append(compression(3), compression(3)...), true},
		{"selected-audio-headerstrip-empty", 1, compression(3), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "encoding.mkv")
			data := replaceMKVTrackName(t, base, markers[tt.stream],
				mkvEncodingElement([]byte{0x6d, 0x80}, tt.encoding))
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			var diagnostics []diagnostic.Event
			ctx := diagnostic.WithSink(t.Context(), func(event diagnostic.Event) {
				diagnostics = append(diagnostics, event)
			})
			ctx = diagnostic.WithJob(ctx, path, "", 0)
			b := audio.Backend{}
			tracks, err := b.ProbeTracks(ctx, path)
			if err != nil || len(tracks) != 2 || tracks[0].Info.Stream.Index != 1 || tracks[1].Info.Stream.Index != 2 {
				t.Fatalf("audio inventory lost indices: %+v %v", tracks, err)
			}
			if tt.stream == 1 || tt.stream == 2 {
				track := tracks[tt.stream-1]
				rejected := tt.blocked || tt.stream == 2
				err := track.Info.ValidateInput()
				if rejected {
					var unsupported *audio.UnsupportedError
					if !errors.As(err, &unsupported) || unsupported.Code != "content_encoding" {
						t.Fatalf("unsupported content encoding not identified: %+v %v", track, err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
			}
			index := 1
			if _, err := b.ProbeStream(ctx, path, index); (err != nil) != tt.blocked {
				t.Fatalf("selected audio probe: %v; blocked=%v", err, tt.blocked)
			}
			output := filepath.Join(t.TempDir(), "output.thd")
			_, err = b.Convert(ctx, path, output, audio.Options{StreamIndex: &index})
			if tt.blocked {
				if err == nil || diagnostic.Describe(err).Code != "unsupported_content_encoding" {
					t.Fatalf("expected explicit content-encoding rejection, got %v", err)
				}
				if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("blocked audio published output: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			for _, event := range diagnostics {
				if event.Code == "ffmpeg" && event.Stage != "check_output" && event.Stage != "verify_output" {
					t.Errorf("source content-encoding diagnostic: %+v", event)
				}
			}
		})
	}
	t.Run("all-audio-unsupported", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "encoding.mkv")
		data := base
		for _, marker := range markers[1:3] {
			data = replaceMKVTrackName(t, data, marker, mkvEncodingElement([]byte{0x6d, 0x80}, compression(0)))
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		tracks, err := (audio.Backend{}).ProbeTracks(t.Context(), path)
		if err != nil || len(tracks) != 2 {
			t.Fatalf("all-unsupported audio inventory failed: %+v %v", tracks, err)
		}
		for _, track := range tracks {
			var unsupported *audio.UnsupportedError
			if err := track.Info.ValidateInput(); !errors.As(err, &unsupported) || unsupported.Code != "content_encoding" {
				t.Fatalf("missing per-track incompatibility: %+v %v", track, err)
			}
		}
	})
}
