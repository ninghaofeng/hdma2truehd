package main

import (
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
