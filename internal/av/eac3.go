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

// EAC3Stats separates source duration from raw bitstream delay/padding.
type EAC3Stats struct {
	Samples, Decoded uint64
}

type eac3Writer struct {
	op     *C.HDMAEAC3
	status *operation
}

func (w eac3Writer) Write(p []byte) (int, error) {
	if err := w.status.ctx.Err(); err != nil {
		return 0, err
	}
	if len(p) == 0 || len(p) > 1<<30 {
		return 0, fmt.Errorf("invalid E-AC-3 PCM block size")
	}
	var message [512]C.char
	ret := C.hdma_eac3_write(w.op, (*C.uchar)(unsafe.Pointer(&p[0])), C.int(len(p)), &message[0])
	if err := result(w.status, ret, &message[0]); err != nil {
		return 0, err
	}
	return len(p), nil
}

// EncodeEAC3 keeps decoding in the existing strict native pipeline and forwards
// normalized PCM synchronously to a separate lossy encoder. It never uses an
// external executable. Each output is still staged/published by package audio.
func EncodeEAC3(ctx context.Context, path, output string, expected Info, bitrate int) (EAC3Stats, error) {
	if err := Check(ctx); err != nil {
		return EAC3Stats{}, err
	}
	if expected.Rate != 48000 || (expected.Layout != "5.1(side)" && expected.Layout != "7.1") {
		return EAC3Stats{}, fmt.Errorf("E-AC-3 requires 48000 Hz and 5.1(side) or 7.1; no automatic resampling/remapping")
	}
	if bitrate < 384000 || bitrate > 1536000 || bitrate%32000 != 0 {
		return EAC3Stats{}, fmt.Errorf("invalid E-AC-3 bitrate")
	}
	if strings.ContainsRune(output, 0) || output == "" {
		return EAC3Stats{}, fmt.Errorf("invalid E-AC-3 output path")
	}
	status := &operation{ctx: ctx}
	handle := cgo.NewHandle(status)
	defer handle.Delete()
	target := C.CString(output)
	defer C.free(unsafe.Pointer(target))
	var encoder *C.HDMAEAC3
	var message [512]C.char
	ret := C.hdma_eac3_open(target, C.int(expected.Channels), C.int(bitrate), C.uintptr_t(handle), &encoder, &message[0])
	// Cancellation may arrive after C successfully opened the output but before
	// result observes it. Close even on that path; NULL is safe after C failure.
	defer C.hdma_eac3_close(encoder)
	if err := result(status, ret, &message[0]); err != nil {
		return EAC3Stats{}, err
	}
	if err := Process(ctx, path, "", expected, eac3Writer{encoder, status}); err != nil {
		return EAC3Stats{}, err
	}
	if err := ctx.Err(); err != nil {
		return EAC3Stats{}, err
	}
	var samples, decoded C.uint64_t
	ret = C.hdma_eac3_finish(encoder, &samples, &decoded, &message[0])
	if err := result(status, ret, &message[0]); err != nil {
		return EAC3Stats{}, err
	}
	return EAC3Stats{Samples: uint64(samples), Decoded: uint64(decoded)}, nil
}
