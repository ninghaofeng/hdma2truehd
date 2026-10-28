package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ninghaofeng/hdma2truehd/internal/audio"
)

var version = "dev"

const help = `hdma2truehd - lossless DTS-HD MA to Dolby TrueHD (experimental)

Usage:
  hdma2truehd doctor
  hdma2truehd probe INPUT
  hdma2truehd convert [OPTIONS] INPUT [INPUT ...]
  hdma2truehd version

Convert options (put flags before input paths):
  -o PATH             Single-file .thd output (default: beside input)
  --output-dir DIR    Batch output directory (must already exist)
  --overwrite         Explicitly replace existing outputs after success
  --skip-verify       Skip full PCM comparison; result is UNVERIFIED

Patched FFmpeg libraries are statically linked; no ffmpeg/ffprobe is needed.
Conversion preserves supported sample rates, precision and speaker labels.
DTS:X/IMAX uses compatible 5.1/7.1 channels only, without object rendering or metadata.
No upmixing, downmixing, resampling, Atmos, or MKV processing.
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(out, help)
		return 0
	}
	if args[0] == "version" {
		fmt.Fprintf(out, "hdma2truehd %s\n", version)
		return 0
	}
	command := args[0]
	if command != "convert" && command != "probe" && command != "doctor" {
		fmt.Fprintf(errOut, "unknown command %q; use --help\n", command)
		return 2
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(errOut)
	var output, outputDir string
	var overwrite, skipVerify bool
	if command == "convert" {
		flags.StringVar(&output, "o", "", "single-file output")
		flags.StringVar(&outputDir, "output-dir", "", "existing output directory")
		flags.BoolVar(&overwrite, "overwrite", false, "replace existing outputs")
		flags.BoolVar(&skipVerify, "skip-verify", false, "skip full PCM comparison")
	}
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	backend := audio.Backend{}
	switch command {
	case "probe":
		if flags.NArg() != 1 {
			fmt.Fprintln(errOut, "probe requires exactly one input")
			return 2
		}
		info, err := backend.Probe(ctx, flags.Arg(0))
		if err == nil {
			err = info.ValidateSource()
		}
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(info); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		return 0
	case "doctor":
		if flags.NArg() != 0 {
			fmt.Fprintln(errOut, "doctor takes no input paths")
			return 2
		}
		if err := backend.SelfTest(ctx); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		fmt.Fprintln(out, "PASS: backend version, 5.1(side)/5.1/7.1 metadata and 24-bit PCM self-tests.")
		fmt.Fprintln(out, "Not a DTS-HD MA fixture test or hardware compatibility certification.")
		return 0
	}
	jobs, err := planJobs(flags.Args(), output, outputDir)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if err := backend.Check(ctx); err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	return processJobs(ctx, jobs, audio.Options{Overwrite: overwrite, SkipVerify: skipVerify}, backend.Convert, out, errOut)
}

func processJobs(ctx context.Context, jobs []job, options audio.Options,
	convert func(context.Context, string, string, audio.Options) (audio.Result, error), out, errOut io.Writer) int {
	fmt.Fprintln(errOut, "NOTE: PCM verification covers decoded compatible channels only; DTS:X objects are not rendered and object metadata is not retained.")
	succeeded, failed, unverified := 0, 0, 0
	for i, job := range jobs {
		if ctx.Err() != nil {
			break
		}
		fmt.Fprintf(errOut, "[%d/%d] %s -> %s (encode%s)\n", i+1, len(jobs), job.input, job.output, verificationLabel(options.SkipVerify))
		result, err := convert(ctx, job.input, job.output, options)
		if err != nil {
			failed++
			fmt.Fprintf(errOut, "FAILED: %s: %v\n", job.input, err)
			continue
		}
		succeeded++
		if result.Verified {
			fmt.Fprintf(out, "VERIFIED: %s (%d samples/channel)\n", result.Output, result.Samples)
		} else {
			unverified++
			fmt.Fprintf(out, "UNVERIFIED: %s (PCM comparison explicitly skipped)\n", result.Output)
		}
	}
	fmt.Fprintf(out, "Summary: %d succeeded, %d failed, %d unverified, %d not started\n",
		succeeded, failed, unverified, len(jobs)-succeeded-failed)
	if ctx.Err() != nil {
		fmt.Fprintln(errOut, "Cancelled; previously published outputs are preserved.")
		return 130
	}
	if failed > 0 {
		return 1
	}
	return 0
}

func verificationLabel(skip bool) string {
	if skip {
		return ", PCM verification OFF"
	}
	return " + full PCM verification"
}

type job struct{ input, output string }

func planJobs(inputs []string, output, outputDir string) ([]job, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("convert requires at least one input; put options before input paths")
	}
	if output != "" && (len(inputs) != 1 || outputDir != "") {
		return nil, fmt.Errorf("-o requires a single input and cannot be combined with --output-dir")
	}
	if outputDir != "" {
		stat, err := os.Stat(outputDir)
		if err != nil {
			return nil, err
		}
		if !stat.IsDir() {
			return nil, fmt.Errorf("--output-dir must be an existing directory")
		}
	}
	seen := make(map[string]bool)
	jobs := make([]job, 0, len(inputs))
	for _, input := range inputs {
		if strings.HasPrefix(input, "-") {
			return nil, fmt.Errorf("options must precede input paths; prefix a filename starting with '-' with './'")
		}
		destination := output
		if destination == "" {
			dir := filepath.Dir(input)
			if outputDir != "" {
				dir = outputDir
			}
			destination = filepath.Join(dir, strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))+".thd")
		}
		key, err := pathKey(destination)
		if err != nil {
			return nil, err
		}
		if seen[key] {
			return nil, fmt.Errorf("batch output collision: %s", destination)
		}
		seen[key] = true
		for _, other := range inputs {
			inputKey, err := pathKey(other)
			if err != nil {
				return nil, err
			}
			inStat, inErr := os.Stat(other)
			outStat, outErr := os.Stat(destination)
			if inputKey == key || (inErr == nil && outErr == nil && os.SameFile(inStat, outStat)) {
				return nil, fmt.Errorf("output would replace an input: %s", destination)
			}
		}
		jobs = append(jobs, job{input: input, output: destination})
	}
	return jobs, nil
}

func pathKey(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(filepath.Dir(path)); err == nil {
		path = filepath.Join(resolved, filepath.Base(path))
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		path = strings.ToLower(path)
	}
	return path, nil
}
