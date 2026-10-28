package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if err := link(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func link() error {
	args := os.Args[1:]
	capture := false
	for _, arg := range args {
		if filepath.Base(arg) == "go.o" {
			capture = true
		}
	}
	if capture {
		dir := os.Getenv("HDMA_RELINK_DIR")
		if dir == "" {
			return fmt.Errorf("HDMA_RELINK_DIR is required")
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		replay := make([]string, 0, len(args))
		output := false
		for _, arg := range args {
			if output {
				replay = append(replay, "../hdma2truehd-relinked"+os.Getenv("HDMA_EXE_SUFFIX"))
				output = false
				continue
			}
			if arg == "-o" {
				replay = append(replay, arg)
				output = true
				continue
			}
			prefix := ""
			input := arg
			if strings.HasPrefix(arg, "-Wl,-T,") {
				prefix = "-Wl,-T,"
				input = strings.TrimPrefix(arg, prefix)
			}
			if strings.HasSuffix(input, ".o") || strings.HasSuffix(input, ".a") || strings.HasSuffix(input, ".ld") || strings.HasSuffix(input, ".def") {
				name := filepath.Base(input)
				data, err := os.ReadFile(input)
				if err != nil {
					return err
				}
				target, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
				if err != nil {
					return err
				}
				_, writeErr := target.Write(data)
				closeErr := target.Close()
				if writeErr != nil {
					return writeErr
				}
				if closeErr != nil {
					return closeErr
				}
				arg = prefix + name
			}
			replay = append(replay, arg)
		}
		var script strings.Builder
		script.WriteString("#!/usr/bin/env bash\nset -euo pipefail\ncd \"$(dirname \"$0\")\"\n\"${CC:-cc}\" ")
		for _, arg := range replay {
			script.WriteString("'" + strings.ReplaceAll(filepath.ToSlash(arg), "'", "'\\''") + "' ")
		}
		script.WriteString("\n")
		if err := os.WriteFile(filepath.Join(dir, "relink.sh"), []byte(script.String()), 0755); err != nil {
			return err
		}
	}
	compiler := os.Getenv("HDMA_CC")
	if compiler == "" {
		compiler = "cc"
	}
	cmd := exec.Command(compiler, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
