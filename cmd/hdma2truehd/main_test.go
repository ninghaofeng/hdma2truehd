package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ninghaofeng/hdma2truehd/internal/audio"
)

func TestPlanJobs(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.dts")
	b := filepath.Join(dir, "b.dts")
	jobs, err := planJobs([]string{a, b}, "", dir)
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
		if _, err := planJobs(tt.inputs, tt.output, tt.dir); err == nil {
			t.Fatalf("accepted invalid job specification %+v", tt)
		}
	}
}

func TestPlanJobsHardLink(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "source.dts")
	output := filepath.Join(dir, "target.thd")
	if err := os.WriteFile(input, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(input, output); err != nil {
		t.Fatal(err)
	}
	if _, err := planJobs([]string{input}, output, ""); err == nil {
		t.Fatal("accepted output aliasing an input")
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
	} {
		var out, errOut bytes.Buffer
		if code := run(context.Background(), tt.args, &out, &errOut); code != tt.code {
			t.Fatalf("%v: want %d got %d\n%s\n%s", tt.args, tt.code, code, &out, &errOut)
		}

	}
}

func TestBatchProcessing(t *testing.T) {
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
				audio.Options{SkipVerify: mode == "skip"}, convert, &out, &errOut)
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
