package av

/*
#include "native.h"
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"fmt"
	"runtime/cgo"
	"strings"
	"unsafe"
)

type Track struct {
	Info            Info
	Language, Title string
	Default         bool
	Duration        float64
}

// ProbeTracks lists audio streams without requiring that every codec can be
// converted. Unsupported tracks remain visible so the user can identify them.
func ProbeTracks(ctx context.Context, path string) ([]Track, error) {
	if err := Check(ctx); err != nil {
		return nil, err
	}
	if strings.ContainsRune(path, 0) {
		return nil, fmt.Errorf("input path contains NUL")
	}
	op := &operation{ctx: ctx}
	handle := cgo.NewHandle(op)
	defer handle.Delete()
	input := C.CString(path)
	defer C.free(unsafe.Pointer(input))
	var tracks *C.HDMATrack
	var count C.int
	var message [512]C.char
	ret := C.hdma_probe_tracks(input, C.uintptr_t(handle), &tracks, &count, &message[0])
	defer C.hdma_tracks_free(tracks)
	if err := result(op, ret, &message[0]); err != nil {
		return nil, err
	}
	found := make([]Track, 0, int(count))
	for _, track := range unsafe.Slice(tracks, int(count)) {
		found = append(found, Track{
			Info:     nativeMetadata(track.info),
			Language: strings.ToValidUTF8(C.GoString(&track.language[0]), "\uFFFD"),
			Title:    strings.ToValidUTF8(C.GoString(&track.title[0]), "\uFFFD"),
			Default:  track.is_default != 0,
			Duration: float64(track.duration_us) / 1e6,
		})
	}
	return found, nil
}
