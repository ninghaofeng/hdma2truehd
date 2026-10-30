// Package buildinfo is the shared source of application and platform metadata.
package buildinfo

import (
	"runtime"
	"strings"
)

const (
	Version           = "v1.0.0"
	Copyright         = "Copyright (c) 2026 ninghaofeng"
	MacOSMinimum      = "12.0"
	WindowsX64Minimum = "Windows 10 22H2 (build 19045)"
	WindowsARMMinimum = "Windows 11 24H2 (build 26100)"
)

func NumericVersion() string { return strings.TrimPrefix(Version, "v") }

type Info struct {
	Version       string `json:"version"`
	Copyright     string `json:"copyright"`
	Architecture  string `json:"architecture"`
	MinimumSystem string `json:"minimumSystem"`
}

// Current reports the release support policy for the compiled target. It does
// not inspect the host OS version or enforce a runtime compatibility gate.
func Current() Info {
	minimum := "Not a release target"
	if runtime.GOOS == "darwin" {
		minimum = "macOS " + MacOSMinimum
	}
	if runtime.GOOS == "windows" {
		minimum = WindowsX64Minimum
		if runtime.GOARCH == "arm64" {
			minimum = WindowsARMMinimum
		}
	}
	return Info{Version, Copyright, runtime.GOARCH, minimum}
}
