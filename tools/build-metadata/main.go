// Command build-metadata renders OS resources from the same metadata as CLI/UI.
package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/ninghaofeng/hdma2truehd/internal/buildinfo"
)

func main() {
	version := flag.Bool("version", false, "print the release tag")
	directory := flag.String("output-dir", "", "write platform metadata into this directory")
	flag.Parse()
	if *version {
		fmt.Println(buildinfo.Version)
		return
	}
	if *directory == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "use --version or --output-dir DIR")
		os.Exit(2)
	}
	if err := generate(*directory); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(directory string) error {
	source, err := os.ReadFile("cmd/hdma2truehd-ui/Info.plist")
	if err != nil {
		return err
	}
	t, err := template.New("plist").Funcs(template.FuncMap{"xml": func(value string) (string, error) {
		var escaped bytes.Buffer
		err := xml.EscapeText(&escaped, []byte(value))
		return escaped.String(), err
	}}).Parse(string(source))
	if err != nil {
		return err
	}
	var plist bytes.Buffer
	if err := t.Execute(&plist, map[string]string{
		"Version": buildinfo.NumericVersion(), "Copyright": buildinfo.Copyright,
		"MacOSMinimum": buildinfo.MacOSMinimum,
	}); err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	for name, data := range map[string][]byte{
		"Info.plist":    plist.Bytes(),
		"cli.rc":        []byte(windowsResource("hdma2truehd.exe")),
		"ui-version.rc": []byte(windowsResource("hdma2truehd-ui.exe")),
		"VERSION.txt":   []byte(buildinfo.Version + "\n" + buildinfo.Copyright + "\n"),
	} {
		if err := os.WriteFile(filepath.Join(directory, name), data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func windowsResource(filename string) string {
	var out strings.Builder
	numeric := strings.ReplaceAll(buildinfo.NumericVersion(), ".", ",") + ",0"
	fmt.Fprintf(&out, `#include <windows.h>
1 VERSIONINFO
FILEVERSION %s
PRODUCTVERSION %s
FILEFLAGSMASK 0x3fL
FILEFLAGS 0x0L
FILEOS 0x40004L
FILETYPE 0x1L
FILESUBTYPE 0x0L
BEGIN
  BLOCK "StringFileInfo"
  BEGIN
    BLOCK "040904b0"
    BEGIN
`, numeric, numeric)
	for _, field := range [][2]string{
		{"FileDescription", "HDMA Converter"}, {"FileVersion", buildinfo.NumericVersion()},
		{"ProductName", "HDMA Converter"}, {"ProductVersion", buildinfo.NumericVersion()},
		{"LegalCopyright", buildinfo.Copyright}, {"OriginalFilename", filename},
	} {
		// Resource string values include an explicit C terminator.
		value := strings.TrimSuffix(strconv.Quote(field[1]), `"`) + `\0"`
		fmt.Fprintf(&out, "      VALUE %q, %s\n", field[0], value)
	}
	out.WriteString(`    END
  END
  BLOCK "VarFileInfo"
  BEGIN
    VALUE "Translation", 0x409, 1200
  END
END
`)
	return out.String()
}
