package audio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Options struct {
	Overwrite  bool
	SkipVerify bool
}

type Result struct {
	Input    string
	Output   string
	Verified bool
	Samples  uint64
}

func (b Backend) Convert(ctx context.Context, input, output string, options Options) (result Result, err error) {
	if err := b.Check(ctx); err != nil {
		return Result{}, err
	}
	source, err := b.Probe(ctx, input)
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
	if !strings.EqualFold(filepath.Ext(output), ".thd") {
		return Result{}, fmt.Errorf("output must use the .thd extension")
	}
	inputStat, err := os.Stat(source.Path)
	if err != nil {
		return Result{}, err
	}
	if outputStat, statErr := os.Stat(output); statErr == nil {
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
	tempDir, err := os.MkdirTemp(filepath.Dir(output), ".hdma2truehd-")
	if err != nil {
		return Result{}, err
	}
	defer func() {
		if cleanupErr := os.RemoveAll(tempDir); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("clean up %s: %w", tempDir, cleanupErr))
		}
	}()
	temp := filepath.Join(tempDir, "output.thd")
	if err := b.Encode(ctx, source, temp); err != nil {
		return Result{}, fmt.Errorf("encode: %w", err)
	}
	encoded, err := b.Probe(ctx, temp)
	if err != nil {
		return Result{}, err
	}
	if err := matchingOutput(source, encoded); err != nil {
		return Result{}, err
	}
	result = Result{Input: source.Path, Output: output}
	if !options.SkipVerify {
		originalPCM, err := b.Digest(ctx, source)
		if err != nil {
			return Result{}, err
		}
		outputPCM, err := b.Digest(ctx, encoded)
		if err != nil {
			return Result{}, err
		}
		if err := ComparePCM(originalPCM, outputPCM); err != nil {
			return Result{}, err
		}
		result.Verified = true
		result.Samples = originalPCM.Samples
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	file, err := os.OpenFile(temp, os.O_RDWR, 0)
	if err != nil {
		return Result{}, err
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(syncErr, closeErr); err != nil {
		return Result{}, err
	}
	if options.Overwrite {
		if err := replaceFile(temp, output); err != nil {
			return Result{}, fmt.Errorf("publish output: %w", err)
		}
	} else if err := os.Link(temp, output); err != nil {
		return Result{}, fmt.Errorf("publish without overwriting (requires hard-link support): %w", err)
	}
	return result, nil
}
