package audio

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProgressStageReset(t *testing.T) {
	var events []Progress
	r := reporter{report: func(p Progress) { events = append(events, p) }}
	r.stage(t.Context(), "encode")
	r.current.Seconds = 12
	r.finish()
	if last := events[len(events)-1]; !last.Done || last.Fraction != 1 {
		t.Fatal(last)
	}
	r.stage(t.Context(), "verify_source")
	if last := events[len(events)-1]; last.Done || last.Seconds != 0 || last.Fraction != 0 || last.Duration != 0 {
		t.Fatalf("stage did not reset: %+v", last)
	}
}

func TestConversionStages(t *testing.T) {
	for _, mode := range []string{"verified", "skip", "encode failure", "mismatch"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			input, output := filepath.Join(dir, "source.dts"), filepath.Join(dir, "out.thd")
			if err := os.WriteFile(input, []byte("fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			b := Backend{
				probe: func(_ context.Context, path string) (Info, error) {
					info := sourceInfo()
					info.Path = path
					if path != input {
						info.Stream.Codec, info.Stream.Profile = "truehd", ""
					}
					return info, nil
				},
				encode: func(_ context.Context, _ Info, path string) error {
					if mode == "encode failure" {
						return errors.New("encode failure")
					}
					return os.WriteFile(path, []byte("encoded"), 0600)
				},
				decode: func(_ context.Context, info Info, out io.Writer) error {
					data := pcmData(8, 17)
					if mode == "mismatch" && info.Path != input {
						data[1] ^= 1
					}
					_, err := out.Write(data)
					return err
				},
			}
			var events []Progress
			_, err := b.Convert(t.Context(), input, output, Options{SkipVerify: mode == "skip", Progress: func(p Progress) { events = append(events, p) }})
			if (err != nil) != (mode == "encode failure" || mode == "mismatch") {
				t.Fatal(err)
			}
			var stages, finished []string
			for _, event := range events {
				if event.Done {
					finished = append(finished, event.Stage)
				} else {
					stages = append(stages, event.Stage)
				}
				if event.Fraction < 0 || event.Fraction > 1 {
					t.Fatal(event)
				}
			}
			want := []string{"probe", "encode", "check_output", "verify_source", "verify_output", "publish"}
			switch mode {
			case "skip":
				want = []string{"probe", "encode", "check_output", "publish"}
			case "encode failure":
				want = []string{"probe", "encode"}
			case "mismatch":
				want = want[:5]
			}
			if !reflect.DeepEqual(stages, want) {
				t.Fatalf("stages=%v want=%v", stages, want)
			}
			for _, stage := range finished {
				if (mode == "encode failure" && stage == "encode") || (mode == "mismatch" && stage == "verify_output") {
					t.Fatalf("failed stage reported complete: %v", finished)
				}
			}
		})
	}
}

func TestUnsupportedReasons(t *testing.T) {
	for _, test := range []struct {
		code   string
		change func(*Info)
	}{
		{"codec", func(i *Info) { i.Stream.Codec = "aac" }},
		{"profile", func(i *Info) { i.Stream.Profile = "DTS-HD HRA" }},
		{"layout", func(i *Info) { i.Stream.Layout = "stereo" }},
		{"channels", func(i *Info) { i.Stream.Channels = 2 }},
		{"rate", func(i *Info) { i.Rate = 32000 }},
		{"bits", func(i *Info) { i.Bits = 0 }},
	} {
		info := sourceInfo()
		test.change(&info)
		var problem *UnsupportedError
		if !errors.As(info.ValidateSource(), &problem) || problem.Code != test.code {
			t.Fatalf("%s: %v", test.code, problem)
		}
	}
	info := sourceInfo()
	info.Rate = 96000
	var problem *UnsupportedError
	if !errors.As(info.ValidateEAC3(0), &problem) || problem.Code != "eac3_format" {
		t.Fatal(problem)
	}
}
