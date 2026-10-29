package audio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ninghaofeng/hdma2truehd/internal/av"
)

type Options struct {
	// Progress runs synchronously on the conversion goroutine; keep it brief.
	Progress  func(Progress)
	Overwrite bool
	// StreamIndex selects a container stream explicitly; nil retains the
	// single-stream input contract instead of guessing an MKV audio track.
	StreamIndex *int
	// SkipVerify skips only full decoding/comparison, not output format checks.
	SkipVerify bool
	// EAC3Bitrate is total kb/s; zero selects the layout-specific default.
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
}

// Convert encodes to a private file, validates it and only then publishes it.
// A codec or verification failure must not expose a partial final output.
func (b Backend) Convert(ctx context.Context, input, output string, options Options) (result Result, err error) {
	progress := reporter{report: options.Progress}
	progress.stage(ctx, "probe")
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
	if err := source.ValidateSource(); err != nil {
		return Result{}, err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return Result{}, err
	}
	extension := strings.ToLower(filepath.Ext(output))
	lossy := extension == ".eac3"
	if extension != ".thd" && !lossy {
		return Result{}, fmt.Errorf("output must use the .thd or .eac3 extension")
	}
	bitrate := 0
	if lossy {
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
			return Result{}, fmt.Errorf("output exists; use --overwrite explicitly: %s", output)
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
			err = errors.Join(err, fmt.Errorf("clean up %s: %w", tempDir, cleanupErr))
		}
	}()
	temp := filepath.Join(tempDir, "output"+extension)
	stageContext := progress.stage(ctx, "encode")
	var stats av.EAC3Stats
	if lossy {
		stats, err = av.EncodeEAC3(stageContext, source.Path, temp, nativeInfo(source), bitrate)
	} else {
		err = b.Encode(stageContext, source, temp)
	}
	if err != nil {
		return Result{}, fmt.Errorf("encode: %w", err)
	}
	progress.finish()
	progress.stage(ctx, "check_output")
	encoded, err := b.Probe(ctx, temp)
	if err != nil {
		return Result{}, err
	}
	if lossy {
		err = matchingEAC3(source, encoded)
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
		if err := validateEAC3(progress.stage(ctx, "verify_output"), encoded, stats); err != nil {
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
	return result, nil
}
