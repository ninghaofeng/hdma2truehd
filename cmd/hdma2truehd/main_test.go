package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ninghaofeng/hdma2truehd/internal/audio"
	"github.com/ninghaofeng/hdma2truehd/internal/buildinfo"
)

func TestMKVOutputPlanning(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "test_video.mkv")
	calls := 0
	probe := func(path string) (audio.Info, error) {
		calls++
		return audio.Info{Path: path, Stream: audio.Stream{Index: 2}, Language: "eng", DelayMS: 125}, nil
	}
	jobs, err := planJobs([]string{input}, "", dir, "all", probe)
	if err != nil || len(jobs) != 3 || calls != 1 {
		t.Fatalf("%+v %v, probe calls %d", jobs, err, calls)
	}
	for i, extension := range []string{".thd", ".eac3", ".ac3"} {
		if jobs[i].output != filepath.Join(dir, "test_video_track2_[eng]_DELAY 125ms"+extension) {
			t.Fatalf("%+v", jobs)
		}
	}
	custom := filepath.Join(dir, "custom.thd")
	jobs, err = planJobs([]string{input}, custom, "", "truehd", probe)
	if err != nil || len(jobs) != 1 || jobs[0].output != custom || calls != 1 {
		t.Fatalf("explicit output changed: %+v %v, calls %d", jobs, err, calls)
	}
	failure := errors.New("metadata read failed")
	if _, err := planJobs([]string{input}, "", "", "truehd", func(string) (audio.Info, error) {
		return audio.Info{}, failure
	}); !errors.Is(err, failure) {
		t.Fatalf("probe failure hidden: %v", err)
	}
}

func TestPlanJobs(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.dts")
	b := filepath.Join(dir, "b.dts")
	jobs, err := planJobs([]string{a, b}, "", dir, "truehd", nil)
	if err != nil || len(jobs) != 2 || jobs[0].output != filepath.Join(dir, "a.thd") {
		t.Fatalf("%+v %v", jobs, err)
	}
	for _, tt := range []struct {
		inputs []string
		output string
		dir    string
	}{
		{nil, "", ""},
		{[]string{a, b}, filepath.Join(dir, "out.thd"), ""},
		{[]string{a}, filepath.Join(dir, "out.thd"), dir},
		{[]string{a}, "", filepath.Join(dir, "missing")},
		{[]string{a, filepath.Join(dir, "a.dtshd")}, "", dir},
		{[]string{filepath.Join(dir, "a.thd")}, "", ""},
		{[]string{a, filepath.Join(dir, "a.thd")}, "", dir},
		{[]string{a, "--skip-verify"}, "", ""},
	} {
		if _, err := planJobs(tt.inputs, tt.output, tt.dir, "truehd", nil); err == nil {
			t.Fatalf("accepted invalid job specification %+v", tt)
		}
	}
}

func TestPlanJobsHardLink(t *testing.T) {
	// String/canonical-path comparison cannot detect hard-link aliases.
	dir := t.TempDir()
	input := filepath.Join(dir, "source.dts")
	output := filepath.Join(dir, "target.thd")
	if err := os.WriteFile(input, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(input, output); err != nil {
		t.Fatal(err)
	}
	if _, err := planJobs([]string{input}, output, "", "truehd", nil); err == nil {
		t.Fatal("accepted output aliasing an input")
	}
}

func TestOutputFormats(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "source.dts")
	jobs, err := planJobs([]string{input}, "", dir, "both", nil)
	if err != nil || len(jobs) != 2 || filepath.Ext(jobs[0].output) != ".thd" || filepath.Ext(jobs[1].output) != ".eac3" {
		t.Fatalf("wrong dual-format jobs: %+v %v", jobs, err)
	}
	jobs, err = planJobs([]string{input}, filepath.Join(dir, "custom.eac3"), "", "eac3", nil)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("wrong E-AC-3 job: %+v %v", jobs, err)
	}
	for _, tt := range []struct{ output, format string }{
		{"", "aac"}, {"output.thd", "both"}, {"output.thd", "eac3"}, {"output.eac3", "truehd"},
		{"", ""}, {"", "eac3,eac3"}, {"", "truehd,thd"}, {"", "eac3,"},
		{"output.ac3", "eac3,ac3"}, {"output.thd", "all"}, {"output.eac3", "ac3"},
	} {
		if _, err := planJobs([]string{input}, tt.output, "", tt.format, nil); err == nil {
			t.Fatalf("accepted %+v", tt)
		}
	}
	var out, errOut bytes.Buffer
	code := processJobs(t.Context(), []job{{input, "output.eac3"}}, audio.Options{}, 1,
		func(context.Context, string, string, audio.Options) (audio.Result, error) {
			return audio.Result{Output: "output.eac3", Lossy: true, Validated: true, Samples: 100, DecodedSamples: 1536}, nil
		}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "VALIDATED (LOSSY)") ||
		strings.Contains(out.String(), "VERIFIED:") || !strings.Contains(out.String(), "0 unverified") {
		t.Fatalf("misleading lossy result: code=%d %s", code, &out)
	}
}

func TestAC3FormatPlanning(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "source.dts")
	for _, tt := range []struct {
		format     string
		extensions []string
	}{
		{"ac3", []string{".ac3"}},
		{"eac3,ac3", []string{".eac3", ".ac3"}},
		{"ac3,truehd", []string{".ac3", ".thd"}},
		{"thd", []string{".thd"}},
		{"all", []string{".thd", ".eac3", ".ac3"}},
	} {
		jobs, err := planJobs([]string{input}, "", dir, tt.format, nil)
		if err != nil || len(jobs) != len(tt.extensions) {
			t.Fatalf("%s: %+v %v", tt.format, jobs, err)
		}
		for i, extension := range tt.extensions {
			if filepath.Ext(jobs[i].output) != extension {
				t.Fatalf("%s: %+v", tt.format, jobs)
			}
		}
	}
	jobs := []job{{input, "output.thd"}, {input, "output.eac3"}, {input, "output.ac3"}}
	var out, diagnostics bytes.Buffer
	code := processJobs(t.Context(), jobs, audio.Options{EAC3Bitrate: 960}, 1,
		func(_ context.Context, _, output string, options audio.Options) (audio.Result, error) {
			want := 0
			if strings.HasSuffix(output, ".eac3") {
				want = 960
			}
			if options.EAC3Bitrate != want {
				t.Errorf("%s: bitrate %d, want %d", output, options.EAC3Bitrate, want)
			}
			lossy := lossyOutput(output)
			return audio.Result{Output: output, Verified: !lossy, Validated: lossy, Lossy: lossy}, nil
		}, &out, &diagnostics)
	if code != 0 || !strings.Contains(diagnostics.String(), "AC-3/E-AC-3 is LOSSY") ||
		!strings.Contains(out.String(), "VALIDATED (LOSSY): output.ac3") {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, &out, &diagnostics)
	}
}

func TestRunArguments(t *testing.T) {
	for _, tt := range []struct {
		args []string
		code int
	}{
		{nil, 0}, {[]string{"--help"}, 0}, {[]string{"version"}, 0},
		{[]string{"unknown"}, 2}, {[]string{"convert"}, 2},
		{[]string{"probe"}, 2}, {[]string{"doctor", "input.dts"}, 2},
		{[]string{"convert", "--not-an-option"}, 2},
		{[]string{"convert", "-h"}, 0},
		{[]string{"doctor", "--ffmpeg", "unused"}, 2},
		{[]string{"convert", "--jobs", "0", "source.dts"}, 2},
		{[]string{"convert", "--jobs", "9", "source.dts"}, 2},
		{[]string{"convert", "--jobs", "-1", "source.dts"}, 2},
		{[]string{"convert", "--jobs", "invalid", "source.dts"}, 2},
		{[]string{"convert", "--format", "ac3", "--eac3-bitrate", "640", "source.dts"}, 2},
		{[]string{"convert", "--format", "truehd,ac3", "--eac3-bitrate", "960", "source.dts"}, 2},
	} {
		var out, errOut bytes.Buffer
		if code := run(context.Background(), tt.args, &out, &errOut); code != tt.code {
			t.Fatalf("%v: want %d got %d\n%s\n%s", tt.args, tt.code, code, &out, &errOut)
		}

	}
}

func TestVersionAndCopyright(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--help"}} {
		var out, diagnostics bytes.Buffer
		if code := run(t.Context(), args, &out, &diagnostics); code != 0 {
			t.Fatalf("%v: exit %d: %s", args, code, diagnostics.String())
		}
		for _, want := range []string{buildinfo.Version, buildinfo.Copyright} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("%v: missing %q: %s", args, want, out.String())
			}
		}
	}
}

func TestFormatHelpAndDoctor(t *testing.T) {
	var out, diagnostics bytes.Buffer
	if code := run(t.Context(), []string{"--help"}, &out, &diagnostics); code != 0 {
		t.Fatalf("help: exit %d: %s", code, &diagnostics)
	}
	for _, want := range []string{"both=truehd,eac3", "all=truehd,eac3,ac3", "e.g. eac3,ac3", "ordinary DTS always uses 1024"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help missing %q", want)
		}
	}
	out.Reset()
	if code := run(t.Context(), []string{"doctor"}, &out, &diagnostics); code != 0 {
		t.Fatalf("doctor: exit %d: %s", code, &diagnostics)
	}
	if !strings.Contains(out.String(), "AC-3 5.1(side) decode/format/length self-tests") {
		t.Fatalf("doctor omits AC-3 self-test: %s", &out)
	}
}

func TestStageProgress(t *testing.T) {
	var out, diagnostics bytes.Buffer
	forwarded := 0
	code := processJobs(t.Context(), []job{{"source.dts", "out.thd"}},
		audio.Options{Progress: func(audio.Progress) { forwarded++ }}, 1,
		func(_ context.Context, _, output string, options audio.Options) (audio.Result, error) {
			for _, p := range []audio.Progress{
				{Stage: "encode"},
				{Stage: "encode", Seconds: 1, Duration: 4, Fraction: .25},
				{Stage: "encode", Seconds: 4, Duration: 4, Fraction: 1, Done: true},
				{Stage: "verify_source"},
				{Stage: "verify_output"},
			} {
				options.Progress(p)
			}
			return audio.Result{Output: output, Verified: true}, nil
		}, &out, &diagnostics)
	if code != 0 || forwarded != 5 {
		t.Fatalf("code=%d forwarded=%d", code, forwarded)
	}
	for _, want := range []string{"Encoding complete", "Verifying source PCM", "Verifying output", "4.0s processed"} {
		if !strings.Contains(diagnostics.String(), want) || strings.Contains(out.String(), want) {
			t.Fatalf("missing/misdirected progress %q: stdout=%s stderr=%s", want, &out, &diagnostics)
		}
	}
}

func TestParallelBatch(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseWork := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseWork()
	done := make(chan struct{})
	var out, diagnostics bytes.Buffer
	code, forwarded := -1, 0
	go func() {
		defer close(done)
		code = processJobs(ctx, []job{{"same.mkv", "a.thd"}, {"same.mkv", "b.eac3"}, {"same.mkv", "c.thd"}, {"same.mkv", "d.thd"}},
			audio.Options{Progress: func(audio.Progress) { forwarded++ }}, 2,
			func(ctx context.Context, _, output string, options audio.Options) (audio.Result, error) {
				options.Progress(audio.Progress{Stage: "encode"})
				started <- struct{}{}
				select {
				case <-release:
				case <-ctx.Done():
					return audio.Result{}, ctx.Err()
				}
				if output == "a.thd" {
					return audio.Result{}, fmt.Errorf("deliberate failure")
				}
				return audio.Result{Output: output, Verified: true}, nil
			}, &out, &diagnostics)
	}()
	for range 2 {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("CLI did not run two jobs concurrently")
		}
	}
	// Release the initial workers; subsequent tasks see the closed channel.
	releaseWork()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("CLI parallel batch did not finish")
	}
	if code != 1 || forwarded != 4 || !strings.Contains(out.String(), "3 succeeded, 1 failed, 0 unverified, 0 not started") {
		t.Fatalf("code=%d forwarded=%d stdout=%s stderr=%s", code, forwarded, &out, &diagnostics)
	}
	for _, output := range []string{"a.thd", "b.eac3", "c.thd", "d.thd"} {
		if !strings.Contains(diagnostics.String(), output+": Encoding") {
			t.Fatalf("progress missing job identity %s: %s", output, &diagnostics)
		}
	}
}

func TestParallelBatchCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{}, 4)
	cleanup := make(chan struct{})
	var cleanupOnce sync.Once
	finishCleanup := func() { cleanupOnce.Do(func() { close(cleanup) }) }
	defer finishCleanup()
	done := make(chan struct{})
	var out, diagnostics bytes.Buffer
	code := -1
	go func() {
		defer close(done)
		code = processJobs(ctx, []job{{"a.dts", "a.thd"}, {"b.dts", "b.thd"}, {"c.dts", "c.thd"}, {"d.dts", "d.thd"}},
			audio.Options{}, 2,
			func(ctx context.Context, _, _ string, _ audio.Options) (audio.Result, error) {
				started <- struct{}{}
				<-ctx.Done()
				<-cleanup
				return audio.Result{}, ctx.Err()
			}, &out, &diagnostics)
	}()
	for range 2 {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("CLI parallel workers did not start")
		}
	}
	cancel()
	select {
	case <-done:
		t.Fatal("CLI returned before cleanup")
	default:
	}
	finishCleanup()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("CLI parallel cancellation did not finish")
	}
	if code != 130 || !strings.Contains(out.String(), "0 succeeded, 2 failed, 0 unverified, 2 not started") {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, &out, &diagnostics)
	}
}

func TestBatchProcessing(t *testing.T) {
	// A file failure continues the batch, whereas cancellation stops it with
	// the interrupt status. Explicitly unverified results must never say VERIFIED.
	for _, mode := range []string{"success", "failure", "skip", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			convert := func(_ context.Context, _, output string, options audio.Options) (audio.Result, error) {
				calls++
				if calls == 1 && mode == "failure" {
					return audio.Result{}, fmt.Errorf("deliberate failure")
				}
				if mode == "cancel" {
					cancel()
					return audio.Result{}, context.Canceled
				}
				return audio.Result{Output: output, Verified: !options.SkipVerify, Samples: 17}, nil
			}
			var out, errOut bytes.Buffer
			code := processJobs(ctx, []job{{"a.dts", "a.thd"}, {"b.dts", "b.thd"}},
				audio.Options{SkipVerify: mode == "skip"}, 1, convert, &out, &errOut)
			if !strings.Contains(errOut.String(), "DTS:X objects are not rendered and object metadata is not retained") {
				t.Fatalf("missing compatible-channel conversion notice: %s", &errOut)
			}
			expectedCode, expectedCalls := 0, 2
			switch mode {
			case "failure":
				expectedCode = 1
				if !strings.Contains(out.String(), "1 succeeded, 1 failed") {
					t.Fatal(out.String())
				}
			case "skip":
				if !strings.Contains(out.String(), "2 unverified") || strings.Contains(out.String(), "\nVERIFIED:") {
					t.Fatal(out.String())
				}
			case "cancel":
				expectedCode, expectedCalls = 130, 1
			}
			if code != expectedCode || calls != expectedCalls {
				t.Fatalf("code=%d calls=%d; %s; %s", code, calls, &out, &errOut)
			}
		})
	}
}
