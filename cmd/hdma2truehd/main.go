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
	"github.com/ninghaofeng/hdma2truehd/internal/buildinfo"
)

const help = "hdma2truehd " + buildinfo.Version + "\n" + buildinfo.Copyright + `
DTS-HD MA to TrueHD / E-AC-3 (experimental)

Usage:
  hdma2truehd doctor
  hdma2truehd probe INPUT
  hdma2truehd convert [OPTIONS] INPUT [INPUT ...]
  hdma2truehd version

Convert options (put flags before input paths):
  -o PATH             Single-file .thd/.eac3 output (not with --format both)
  --output-dir DIR    Batch output directory (must already exist)
  --format FORMAT     truehd (default), eac3, or both
  --stream N          Explicit container audio stream index (one input only)
  --eac3-bitrate N    Total kb/s; default 960 for 5.1(side), 1536 for 7.1
  --overwrite         Explicitly replace existing outputs after success
  --skip-verify       Skip full output verification; result is UNVERIFIED

Patched FFmpeg libraries are statically linked; no ffmpeg/ffprobe is needed.
TrueHD preserves supported sample rates, precision and speaker labels.
E-AC-3 is LOSSY: 48 kHz, 5.1(side)/7.1; no automatic resampling/downmixing.
E-AC-3 7.1 uses independent+dependent substreams, not Atmos or a 5.1 fallback.
Raw E-AC-3 retains 256 samples of encoder delay plus final frame padding.
E-AC-3 validation checks complete decoding, format and length, not PCM equality.
DTS:X/IMAX uses compatible 5.1/7.1 channels only, without object rendering or metadata.
Use "probe --list-tracks INPUT.mkv" to list container audio tracks.
No upmixing, downmixing, resampling, Atmos, or MKV output/remuxing.
`

func main() {
	// Signal cancellation reaches FFmpeg's I/O interrupt and frame-processing
	// checks; per-operation cleanup runs before run returns its exit code.
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
		fmt.Fprintf(out, "hdma2truehd %s\n%s\n", buildinfo.Version, buildinfo.Copyright)
		return 0
	}
	command := args[0]
	if command != "convert" && command != "probe" && command != "doctor" {
		fmt.Fprintf(errOut, "unknown command %q; use --help\n", command)
		return 2
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(errOut)
	var output, outputDir, format string
	var eac3Bitrate int
	streamIndex := -1
	var listTracks bool
	var overwrite, skipVerify bool
	if command == "convert" {
		flags.StringVar(&output, "o", "", "single-file output")
		flags.StringVar(&outputDir, "output-dir", "", "existing output directory")
		flags.StringVar(&format, "format", "truehd", "truehd, eac3 or both")
		flags.IntVar(&eac3Bitrate, "eac3-bitrate", 0, "total E-AC-3 kb/s; 0 selects a default")
		flags.IntVar(&streamIndex, "stream", -1, "explicit audio stream index")
		flags.BoolVar(&overwrite, "overwrite", false, "replace existing outputs")
		flags.BoolVar(&skipVerify, "skip-verify", false, "skip full PCM comparison")
	}
	if command == "probe" {
		flags.BoolVar(&listTracks, "list-tracks", false, "list all container audio tracks")
	}
	// flag stops at the first positional argument. planJobs rejects later
	// option-looking paths rather than silently treating misplaced flags as files.
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
		if listTracks {
			tracks, err := backend.ProbeTracks(ctx, flags.Arg(0))
			if err != nil {
				fmt.Fprintln(errOut, err)
				return 1
			}
			encoder := json.NewEncoder(out)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(tracks); err != nil {
				fmt.Fprintln(errOut, err)
				return 1
			}
			return 0
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
		fmt.Fprintln(out, "PASS: backend version, TrueHD 5.1(side)/5.1/7.1 lossless PCM and E-AC-3 5.1(side)/7.1 decode/format/length self-tests.")
		fmt.Fprintln(out, "Not a DTS-HD MA fixture test or hardware compatibility certification.")
		return 0
	}
	// Plan all output paths before writing, including collisions with later
	// inputs. Codec eligibility is checked by Convert for each job, not here.
	if streamIndex < -1 || (streamIndex >= 0 && flags.NArg() != 1) {
		fmt.Fprintln(errOut, "--stream requires a nonnegative index and exactly one input")
		return 2
	}
	if (eac3Bitrate != 0 && (eac3Bitrate < 384 || eac3Bitrate > 1536 || eac3Bitrate%32 != 0)) ||
		(format == "truehd" && eac3Bitrate != 0) {
		fmt.Fprintln(errOut, "--eac3-bitrate requires eac3/both; use 0 for automatic or 384..1536 kb/s in steps of 32 (layout limits also apply)")
		return 2
	}
	jobs, err := planJobs(flags.Args(), output, outputDir, format)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if err := backend.Check(ctx); err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	options := audio.Options{Overwrite: overwrite, SkipVerify: skipVerify, EAC3Bitrate: eac3Bitrate}
	if streamIndex >= 0 {
		options.StreamIndex = &streamIndex
	}
	return processJobs(ctx, jobs, options, backend.Convert, out, errOut)
}

func processJobs(ctx context.Context, jobs []job, options audio.Options,
	convert func(context.Context, string, string, audio.Options) (audio.Result, error), out, errOut io.Writer) int {
	// Progress/diagnostics stay on stderr; stdout reports final per-file results
	// and totals. One failed job does not prevent independent later jobs.
	fmt.Fprintln(errOut, "NOTE: PCM verification covers decoded compatible channels only; DTS:X objects are not rendered and object metadata is not retained.")
	for _, job := range jobs {
		if strings.EqualFold(filepath.Ext(job.output), ".eac3") {
			fmt.Fprintln(errOut, "NOTE: E-AC-3 is LOSSY; validation is strict decoding plus format/length checks, not lossless PCM equality. Raw output has 256 samples of delay and frame padding. Outputs are published independently.")
			break
		}
	}
	succeeded, failed, unverified := 0, 0, 0
	for i, job := range jobs {
		if ctx.Err() != nil {
			// Interrupt is distinct from an ordinary conversion error. Already
			// published outputs survive; the active conversion cleans its staging file.
			break
		}
		fmt.Fprintf(errOut, "[%d/%d] %s -> %s (encode%s)\n", i+1, len(jobs), job.input, job.output, verificationLabel(options.SkipVerify, strings.EqualFold(filepath.Ext(job.output), ".eac3")))
		result, err := convert(ctx, job.input, job.output, options)
		if err != nil {
			failed++
			fmt.Fprintf(errOut, "FAILED: %s: %v\n", job.input, err)
			continue
		}
		succeeded++
		if result.Validated {
			fmt.Fprintf(out, "VALIDATED (LOSSY): %s (%d source samples/channel, %d decoded including delay/padding)\n", result.Output, result.Samples, result.DecodedSamples)
		} else if result.Verified {
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

func verificationLabel(skip, lossy bool) string {
	if skip {
		return ", PCM verification OFF"
	}
	if lossy {
		return ", LOSSY + strict decode/format/length validation"
	}
	return " + full PCM verification"
}

type job struct{ input, output string }

func planJobs(inputs []string, output, outputDir, format string) ([]job, error) {
	formats := []string{format}
	switch format {
	case "truehd", "eac3":
	case "both":
		// Each format becomes an independent transaction. A successful TrueHD
		// output is preserved even if the following E-AC-3 conversion fails.
		formats = []string{"truehd", "eac3"}
		if output != "" {
			return nil, fmt.Errorf("-o cannot be combined with --format both; use --output-dir")
		}
	default:
		return nil, fmt.Errorf("--format must be truehd, eac3 or both")
	}
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
		for _, selected := range formats {
			extension := ".thd"
			if selected == "eac3" {
				extension = ".eac3"
			}
			destination := output
			if destination == "" {
				dir := filepath.Dir(input)
				if outputDir != "" {
					dir = outputDir
				}
				destination = filepath.Join(dir, strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))+extension)
			}
			if !strings.EqualFold(filepath.Ext(destination), extension) {
				return nil, fmt.Errorf("%s output must use %s extension", selected, extension)
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
	}
	return jobs, nil
}

// pathKey conservatively detects output collisions before files exist.
// Resolving the parent handles symlinked directories; os.SameFile separately
// catches aliases of existing files. Case folding may over-reject on a
// case-sensitive macOS volume, intentionally preferring safety to overwriting.
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
