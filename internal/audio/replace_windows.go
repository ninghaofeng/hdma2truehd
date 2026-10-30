package audio

import (
	"syscall"
	"unsafe"
)

func replaceFile(source, target string) error {
	from, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	move := syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")
	// MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH: replace directly
	// rather than delete-then-rename, which could lose the old file on failure.
	// Convert stages on the same filesystem; cross-volume copying is disabled.
	result, _, callErr := move.Call(uintptr(unsafe.Pointer(from)), uintptr(unsafe.Pointer(to)), 0x1|0x8)
	if result == 0 {
		return callErr
	}
	return nil
}
