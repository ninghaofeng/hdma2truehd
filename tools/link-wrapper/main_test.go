package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCapturingRelinkInputs(t *testing.T) {
	dir := t.TempDir()
	relink := filepath.Join(dir, "relink")
	t.Setenv("HDMA_RELINK_DIR", relink)
	// Deliberately fail after capture rather than invoking a platform compiler.
	t.Setenv("HDMA_CC", filepath.Join(dir, "no-compiler"))
	var files []string
	for _, name := range []string{"go.o", "000001.o", "libavcodec.a", "fix_debug_gdb_scripts.ld"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name), 0600); err != nil {
			t.Fatal(err)
		}

		files = append(files, path)
	}
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	os.Args = []string{"link-wrapper", "-o", "ignored-output", files[0], files[1], files[2], "-Wl,-T," + files[3]}
	if err := link(); err == nil {
		t.Fatal("expected missing compiler")
	}
	script, err := os.ReadFile(filepath.Join(relink, "relink.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(script), dir) || !strings.Contains(string(script), "'-Wl,-T,fix_debug_gdb_scripts.ld'") {
		t.Fatalf("relink script contains non-portable input paths: %s", script)
	}
	for _, path := range files {
		data, err := os.ReadFile(filepath.Join(relink, filepath.Base(path)))
		if err != nil || string(data) != filepath.Base(path) {
			t.Fatalf("missing relink input %s: %v", path, err)
		}
	}
}

func TestCapturingWindowsExportDefinition(t *testing.T) {
	dir := t.TempDir()
	inputs := filepath.Join(dir, "go-link temporary inputs")
	if err := os.Mkdir(inputs, 0755); err != nil {
		t.Fatal(err)
	}
	relink := filepath.Join(dir, "relink")
	t.Setenv("HDMA_RELINK_DIR", relink)
	t.Setenv("HDMA_EXE_SUFFIX", ".exe")
	t.Setenv("HDMA_CC", filepath.Join(dir, "no-compiler"))
	definition := []byte("EXPORTS\n_cgo_stub_export\n")
	goObject := filepath.Join(inputs, "go.o")
	exportFile := filepath.Join(inputs, "export_file.def")
	for path, data := range map[string][]byte{goObject: []byte("go object"), exportFile: definition} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	os.Args = []string{"link-wrapper", "-o", "original.exe", exportFile, goObject}
	if err := link(); err == nil {
		t.Fatal("expected missing compiler")
	}
	// Model Go deleting go-link's temporary directory after the build.
	// The replay must remain usable without its original export definition.
	if err := os.RemoveAll(inputs); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(relink, "relink.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(script), filepath.ToSlash(inputs)) ||
		!strings.Contains(string(script), "'../hdma2truehd-relinked.exe' 'export_file.def' 'go.o'") {
		t.Fatalf("Windows relink command depends on temporary inputs: %s", script)
	}
	data, err := os.ReadFile(filepath.Join(relink, "export_file.def"))
	if err != nil || !bytes.Equal(data, definition) {
		t.Fatalf("Windows export definition was not preserved: data=%q err=%v", data, err)
	}
}

func TestDesktopRelinkName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HDMA_RELINK_DIR", filepath.Join(dir, "relink"))
	t.Setenv("HDMA_RELINK_NAME", "hdma2truehd-ui-relinked")
	t.Setenv("HDMA_EXE_SUFFIX", "")
	t.Setenv("HDMA_CC", filepath.Join(dir, "no-compiler"))
	object := filepath.Join(dir, "go.o")
	if err := os.WriteFile(object, []byte("object"), 0600); err != nil {
		t.Fatal(err)
	}
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	os.Args = []string{"link-wrapper", "-o", "ignored", object}
	if err := link(); err == nil {
		t.Fatal("expected missing compiler")
	}
	script, err := os.ReadFile(filepath.Join(dir, "relink", "relink.sh"))
	if err != nil || !strings.Contains(string(script), "../hdma2truehd-ui-relinked") {
		t.Fatalf("%s %v", script, err)
	}
	t.Setenv("HDMA_RELINK_NAME", "../escape")
	if err := link(); err == nil || !strings.Contains(err.Error(), "filename") {
		t.Fatalf("invalid relink filename: %v", err)
	}
}

func TestMissingWindowsExportDefinition(t *testing.T) {
	dir := t.TempDir()
	relink := filepath.Join(dir, "relink")
	t.Setenv("HDMA_RELINK_DIR", relink)
	t.Setenv("HDMA_CC", filepath.Join(dir, "no-compiler"))
	exportFile := filepath.Join(dir, "export_file.def")
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	os.Args = []string{"link-wrapper", "-o", "original.exe", exportFile, filepath.Join(dir, "go.o")}
	err := link()
	var pathError *os.PathError
	if !errors.As(err, &pathError) || pathError.Path != exportFile || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing export definition was not reported: %v", err)
	}

	if _, err := os.Stat(filepath.Join(relink, "relink.sh")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("published a relink script with missing input: %v", err)
	}
}
