package audio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ninghaofeng/hdma2truehd/internal/av"
	"github.com/ninghaofeng/hdma2truehd/internal/diagnostic"
)

type Options struct {
	// Progress runs synchronously on the conversion goroutine; keep it brief.
	Progress func(Progress)
	// Warning also runs synchronously and may report a failed attempt before
	// a retry. Result.Warnings is scoped to the attempt producing that result.
	Warning   func(diagnostic.Problem)
	Overwrite bool
	// StreamIndex selects a container stream explicitly; nil retains the
	// single-stream input contract instead of guessing an MKV audio track.
	StreamIndex *int
	// SkipVerify skips only full decoding/comparison, not output format checks.
	SkipVerify bool
	// EAC3Bitrate is total kb/s for MA; zero selects its layout default.
	// Ordinary DTS overrides it with 1024; AC-3 always uses 640 independently.
	EAC3Bitrate int
}

// Result describes a published output. Verified means lossless PCM equality;
// Validated means a lossy output passed complete decoding and format/length
// checks, not equality. Samples is source length; unverified results leave zero.
type Result struct {
	Input          string
	Output         string
	Verified       bool
	Samples        uint64
	Lossy          bool
	Validated      bool
	DecodedSamples uint64
	Warnings       []diagnostic.Problem
}

// Convert encodes to a private file, validates it and only then publishes it.
// A codec or verification failure must not expose a partial final output.
func (b Backend) Convert(ctx context.Context, input, output string, options Options) (result Result, err error) {
	for attempt := 1; attempt <= 2; attempt++ {
		work := diagnostic.WithJob(ctx, input, output, attempt)
		// Nested probes share this transaction's retry budget. Retrying them
		// independently would multiply attempts and reuse partly completed work.
		work = context.WithValue(work, retryOwnedKey{}, true)
		if options.StreamIndex != nil {
			work = diagnostic.WithStream(work, *options.StreamIndex)
		}
		diagnostic.Emit(work, diagnostic.Event{Level: "info", Code: "attempt_start", Message: "Conversion attempt started"})
		result, err = b.convert(work, input, output, options)
		if err == nil {
			diagnostic.Emit(work, diagnostic.Event{Level: "info", Code: "conversion_complete", Message: "Output verified as requested and published"})
			return result, nil
		}
		problem := diagnostic.Describe(err)
		diagnostic.Emit(work, diagnostic.Event{Level: "error", Code: problem.Code, Stage: problem.Stage, Message: problem.Detail})
		// The attempt has already cleaned its staging directory. Only explicit
		// temporary I/O permits a fresh transaction; never resume partial output.
		if attempt == 2 || !problem.Retryable || ctx.Err() != nil {
			return result, err
		}
		diagnostic.Emit(work, diagnostic.Event{Level: "warning", Code: "retrying", Message: "Temporary I/O failure; retrying the complete transaction once"})
		if options.Warning != nil {
			options.Warning(diagnostic.Problem{Code: "retrying", Stage: problem.Stage, Detail: problem.Detail})
		}
		if err := retryPause(ctx); err != nil {
			return Result{}, err
		}
	}
	return result, err
}

func (b Backend) convert(ctx context.Context, input, output string, options Options) (result Result, err error) {
	progress := reporter{report: options.Progress}
	defer func() { err = diagnostic.Wrap(progress.current.Stage, err) }()
	var warnings []diagnostic.Problem
	seen := map[string]bool{}
	warn := func(problem diagnostic.Problem) {
		key := problem.Code + problem.Detail
		if seen[key] {
			return
		}
		if len(warnings) >= 20 {
			// Reserve the last slots for a useful integrity warning and the
			// truncation notice; the diagnostic sink still receives every event.
			if problem.Code == "duration_short" {
				warnings[18] = problem
				seen[key] = true
				if options.Warning != nil {
					options.Warning(problem)
				}
			}
			if warnings[19].Code != "warnings_truncated" {
				warnings[19] = diagnostic.Problem{Code: "warnings_truncated", Detail: "Additional diagnostics are available in the log; per-job display is limited to 20 warnings."}
				if options.Warning != nil {
					options.Warning(warnings[19])
				}
			}
			return
		}
		seen[key] = true
		warnings = append(warnings, problem)
		if options.Warning != nil {
			options.Warning(problem)
		}
	}
	parent := ctx
	ctx = diagnostic.WithSink(ctx, func(event diagnostic.Event) {
		// Forward to the original sink, not this replacement sink, to avoid
		// recursion and retain raw logs beyond the per-job display limit.
		diagnostic.Emit(parent, event)
		if event.Code == "ffmpeg" && (event.Level == "warning" || event.Level == "error") {
			warn(diagnostic.Problem{Code: "ffmpeg_warning", Stage: event.Stage, Detail: event.Message})
		}
	})
	ctx = progress.stage(ctx, "probe")
	if err := b.Check(ctx); err != nil {
		return Result{}, err
	}
	var source Info
	if options.StreamIndex != nil {
		source, err = b.ProbeStream(ctx, input, *options.StreamIndex)
	} else {
		source, err = b.Probe(ctx, input)
	}
	if err != nil {
		return Result{}, err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return Result{}, err
	}
	extension := strings.ToLower(filepath.Ext(output))
	lossy := extension == ".eac3" || extension == ".ac3"
	if extension != ".thd" && !lossy {
		return Result{}, fmt.Errorf("output must use the .thd, .eac3 or .ac3 extension")
	}
	// Eligibility depends on the destination: accepting ordinary DTS for a
	// lossy output must not weaken TrueHD's MA-only publication gate.
	if err := source.ValidateOutput(strings.TrimPrefix(extension, "."), options.EAC3Bitrate); err != nil {
		return Result{}, err
	}
	bitrate := 0
	if extension == ".eac3" {
		bitrate, err = eac3Bitrate(source, options.EAC3Bitrate)
		if err != nil {
			return Result{}, err
		}
	}
	inputStat, err := os.Stat(source.Path)
	if err != nil {
		return Result{}, err
	}
	if outputStat, statErr := os.Stat(output); statErr == nil {
		// Different path strings may still name the same inode via hard links
		// or symlinks. Overwrite permission never allows replacing the source.
		if os.SameFile(inputStat, outputStat) {
			return Result{}, fmt.Errorf("input and output refer to the same file")
		}
		if !outputStat.Mode().IsRegular() {
			return Result{}, fmt.Errorf("output exists and is not a regular file")
		}
		if !options.Overwrite {
			return Result{}, &diagnostic.Failure{Code: "output_exists", Cause: fmt.Errorf("output exists; use --overwrite explicitly: %s", output)}
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return Result{}, statErr
	}
	// Stage on the destination filesystem so rename/hard-link publication
	// does not need a cross-filesystem copy that could leave a partial output.
	tempDir, err := os.MkdirTemp(filepath.Dir(output), ".hdma2truehd-")
	if err != nil {
		return Result{}, err
	}
	defer func() {
		// Join cleanup errors into the named return even after publication;
		// callers must not be told that leftover private files were cleaned.
		if cleanupErr := os.RemoveAll(tempDir); cleanupErr != nil {
			err = &diagnostic.Failure{Code: "cleanup_failed", Cause: errors.Join(err, fmt.Errorf("clean up %s: %w", tempDir, cleanupErr))}
		}
	}()
	temp := filepath.Join(tempDir, "output"+extension)
	stageContext := progress.stage(ctx, "encode")
	var stats av.EAC3Stats
	if extension == ".ac3" {
		stats, err = av.EncodeAC3(stageContext, source.Path, temp, nativeInfo(source))
	} else if lossy {
		stats, err = av.EncodeEAC3(stageContext, source.Path, temp, nativeInfo(source), bitrate)
	} else {
		err = b.Encode(stageContext, source, temp)
	}
	if err != nil {
		return Result{}, fmt.Errorf("encode: %w", err)
	}
	progress.finish()
	if warning := durationWarning(progress.current.Seconds, progress.current.Duration); warning != nil {
		warn(*warning)
		diagnostic.Emit(ctx, diagnostic.Event{Level: "warning", Code: warning.Code, Stage: warning.Stage, Message: warning.Detail})
	}
	stageContext = progress.stage(ctx, "check_output")
	encoded, err := b.Probe(stageContext, temp)
	if err != nil {
		return Result{}, err
	}
	if lossy {
		err = matchingLossy(source, encoded, strings.TrimPrefix(extension, "."))
	} else {
		err = matchingOutput(source, encoded)
	}
	if err != nil {
		return Result{}, err
	}
	result = Result{Input: source.Path, Output: output, Lossy: lossy}
	if lossy && !options.SkipVerify {
		// Retain the same publication gate, but never label lossy format/length
		// validation as proof of the source/output PCM equality used below.
		if err := validateLossy(progress.stage(ctx, "verify_output"), encoded, stats); err != nil {
			return Result{}, err
		}
		progress.finish()
		result.Validated, result.Samples, result.DecodedSamples = true, stats.Samples, stats.Decoded
	} else if !options.SkipVerify {
		// Metadata alone cannot detect truncation, changed low bits or channel
		// swaps. Decode both complete streams before committing the output.
		originalPCM, err := b.Digest(progress.stage(ctx, "verify_source"), source)
		if err != nil {
			return Result{}, err
		}
		progress.finish()
		outputPCM, err := b.Digest(progress.stage(ctx, "verify_output"), encoded)
		if err != nil {
			return Result{}, err
		}
		if err := ComparePCM(originalPCM, outputPCM); err != nil {
			return Result{}, err
		}
		progress.finish()
		result.Verified = true
		result.Samples = originalPCM.Samples
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	currentStat, err := os.Stat(source.Path)
	if err != nil {
		return Result{}, err
	}
	// Catch common mutations across the separate encode/verify reads. This
	// stat snapshot is not a content fingerprint or proof of source completeness.
	if !os.SameFile(inputStat, currentStat) || inputStat.Size() != currentStat.Size() || !inputStat.ModTime().Equal(currentStat.ModTime()) {
		return Result{}, &diagnostic.Failure{Code: "source_changed", Cause: fmt.Errorf("source changed during conversion: %s", source.Path)}
	}
	progress.stage(ctx, "publish")
	// Flush file contents before making the final name visible. This does not
	// promise directory-entry durability across a sudden power failure.
	file, err := os.OpenFile(temp, os.O_RDWR, 0)
	if err != nil {
		return Result{}, err
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(syncErr, closeErr); err != nil {
		return Result{}, err
	}
	// Without overwrite, Link creates the final name only if still absent,
	// closing the race with the earlier existence check. Cleanup removes the
	// private name, not the published link. Replacement uses the OS primitive.
	if options.Overwrite {
		if err := replaceFile(temp, output); err != nil {
			return Result{}, fmt.Errorf("publish output: %w", err)
		}
	} else if err := os.Link(temp, output); err != nil {
		return Result{}, fmt.Errorf("publish without overwriting (requires hard-link support): %w", err)
	}
	progress.finish()
	result.Warnings = warnings
	return result, nil
}
