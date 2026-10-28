//go:build desktop

package main

// Wails' native file dialogs use UTType on current macOS SDKs.

/*
#cgo LDFLAGS: -framework UniformTypeIdentifiers
*/
import "C"
