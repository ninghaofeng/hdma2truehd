package av

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ninghaofeng/hdma2truehd/internal/diagnostic"
)

func TestParallelLogAttribution(t *testing.T) {
	path, info := fixture(t)
	// A tiny raw stream reliably triggers FFmpeg's low-confidence probe warning.
	if err := WritePCMFixture(path, make([]byte, info.Channels*4), info); err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			output := filepath.Join(t.TempDir(), "output.thd")
			var events []diagnostic.Event
			ctx := diagnostic.WithJob(t.Context(), path, output, i+1)
			ctx = diagnostic.WithSink(ctx, func(event diagnostic.Event) { events = append(events, event) })
			if err := Process(ctx, path, output, info, nil); err != nil {
				t.Fatal(err)
			}
			if _, err := Probe(ctx, output); err != nil {
				t.Fatal(err)
			}
			if len(events) == 0 {
				t.Fatal("FFmpeg warnings were not captured")
			}
			for _, event := range events {
				if event.Code != "ffmpeg" || event.Input != path || event.Output != output || event.Attempt != i+1 || event.Message == "" {
					t.Fatalf("crossed native logs: %+v", event)
				}
			}
		})
	}
}
