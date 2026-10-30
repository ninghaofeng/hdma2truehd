package main

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ninghaofeng/hdma2truehd/internal/buildinfo"
)

func TestGeneratedMetadata(t *testing.T) {
	dir := t.TempDir()
	source, err := os.ReadFile("../../cmd/hdma2truehd-ui/Info.plist")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll("cmd/hdma2truehd-ui", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("cmd/hdma2truehd-ui/Info.plist", source, 0600); err != nil {
		t.Fatal(err)
	}
	if err := generate(dir); err != nil {
		t.Fatal(err)
	}
	plist, err := os.ReadFile(filepath.Join(dir, "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		XMLName xml.Name `xml:"plist"`
		Values  []string `xml:"dict>string"`
	}
	if err := xml.Unmarshal(plist, &document); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{buildinfo.NumericVersion(), buildinfo.Copyright, buildinfo.MacOSMinimum} {
		if !strings.Contains(string(plist), "<string>"+value+"</string>") {
			t.Fatalf("plist missing %q:\n%s", value, plist)
		}
	}
	if strings.Contains(string(plist), "{{") {
		t.Fatal("unrendered plist template")
	}
	for _, name := range []string{"cli.rc", "ui-version.rc"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"FILEVERSION 1,0,0,0", "PRODUCTVERSION 1,0,0,0", buildinfo.Copyright, buildinfo.NumericVersion()} {
			if !strings.Contains(string(data), want) {
				t.Fatalf("%s missing %q", name, want)
			}
		}
	}
}
