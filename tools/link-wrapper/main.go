// Command link-wrapper temporarily stands in for Go's external C linker.
// It saves the final link inputs and a replay command for LGPL relinking, then
// delegates the original invocation to the real compiler without rewriting it.
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
	// Go also invokes the external compiler for capability probes. Only the
	// final application link includes go.o and should produce relink materials.
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
				name := os.Getenv("HDMA_RELINK_NAME")
				if name == "" {
					name = "hdma2truehd-relinked"
				}
				if filepath.Base(name) != name || name == "." || name == ".." {
					return fmt.Errorf("HDMA_RELINK_NAME must be a filename")
				}
				replay = append(replay, "../"+name+os.Getenv("HDMA_EXE_SUFFIX"))
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
			// A linker script may be embedded in a compiler forwarding option
			// rather than passed as a bare filename. Preserve that option syntax.
			if strings.HasPrefix(arg, "-Wl,-T,") {
				prefix = "-Wl,-T,"
				input = strings.TrimPrefix(arg, prefix)
			}
			// Go removes its temporary link directory after building. Capture
			// all referenced objects, archives and scripts now, including the
			// Windows export_file.def, or later relinking would depend on it.
			if strings.HasSuffix(input, ".o") || strings.HasSuffix(input, ".a") || strings.HasSuffix(input, ".ld") || strings.HasSuffix(input, ".def") {
				name := filepath.Base(input)
				data, err := os.ReadFile(input)
				if err != nil {
					return err
				}
				// Flatten to portable relative names, but fail on collisions:
				// replacing a same-named input could silently link wrong code.
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
		// Write the replay script last: a missing input must fail before
		// advertising a replay command for this link.
		var script strings.Builder
		script.WriteString("#!/usr/bin/env bash\nset -euo pipefail\ncd \"$(dirname \"$0\")\"\n\"${CC:-cc}\" ")
		for _, arg := range replay {
			// Replay uses Bash on both platforms. Normalize Windows separators
			// and quote every argument, including embedded single quotes.
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
	// Only the saved replay uses relocated inputs. The live build must receive
	// the original argument list, with no shell expansion or replay rewriting.
	cmd := exec.Command(compiler, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
