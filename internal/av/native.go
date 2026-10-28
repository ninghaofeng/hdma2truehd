// Package av is the synchronous cgo boundary to the patched static FFmpeg
// backend. Each call owns its native contexts and its cancellation/PCM callbacks.
package av

/*
#cgo CFLAGS: -I${SRCDIR}/../../.backend/ffmpeg-8.0.1
#cgo LDFLAGS: ${SRCDIR}/../../.backend/ffmpeg-8.0.1/libavformat/libavformat.a ${SRCDIR}/../../.backend/ffmpeg-8.0.1/libavcodec/libavcodec.a ${SRCDIR}/../../.backend/ffmpeg-8.0.1/libswresample/libswresample.a ${SRCDIR}/../../.backend/ffmpeg-8.0.1/libavutil/libavutil.a -lm
#cgo darwin LDFLAGS: -framework CoreFoundation -framework CoreVideo -framework CoreMedia -lpthread
#cgo windows LDFLAGS: -static -lws2_32 -lbcrypt -lole32 -luuid
#cgo linux LDFLAGS: -lpthread
#include <stdlib.h>
#include "native.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime/cgo"
	"strings"
	"unsafe"
)

// Version rejects an accidentally linked stock/incompatible backend. The marker
// identifies the required patches, but is not a cryptographic provenance check.
const Version = "8.0.1-hdma2truehd-71-v1"

type Info struct {
	Index, Rate, Bits, Channels int
	Codec, Profile, Layout      string
}

// operation stays reachable through a cgo.Handle for the entire synchronous C
// call. Only the integer handle crosses the boundary, never a stored Go pointer.
type operation struct {
	ctx context.Context
	out io.Writer
	err error
}

//export hdmaGoCancelled
func hdmaGoCancelled(handle C.uintptr_t) C.int {
	op := cgo.Handle(handle).Value().(*operation)
	if op.ctx.Err() != nil {
		return 1
	}
	return 0
}

//export hdmaGoPCM
func hdmaGoPCM(handle C.uintptr_t, data *C.uchar, size C.int) C.int {
	op := cgo.Handle(handle).Value().(*operation)
	// C owns this buffer and frees it after the callback returns. The writer
	// must consume it synchronously, without retaining the borrowed slice.
	p := unsafe.Slice((*byte)(unsafe.Pointer(data)), int(size))
	n, err := op.out.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	// Preserve the writer's Go error; C can report only a generic callback
	// failure code, which would otherwise hide the actual cause.
	op.err = err
	if err != nil {
		return -1
	}
	return 0
}

func Check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if version := C.GoString(C.hdma_version()); version != Version {
		return fmt.Errorf("linked FFmpeg version %q, need %q; rebuild the patched static libraries", version, Version)
	}
	if C.hdma_check() != 0 {
		return fmt.Errorf("linked FFmpeg lacks DTS/TrueHD codecs or the raw TrueHD muxer")
	}
	return nil
}

func Probe(ctx context.Context, path string) (Info, error) {
	if err := Check(ctx); err != nil {
		return Info{}, err
	}
	if strings.ContainsRune(path, 0) {
		return Info{}, fmt.Errorf("input path contains NUL")
	}
	op := &operation{ctx: ctx}
	handle := cgo.NewHandle(op)
	defer handle.Delete()
	input := C.CString(path)
	defer C.free(unsafe.Pointer(input))
	var info C.HDMAInfo
	var message [512]C.char
	ret := C.hdma_probe(input, C.uintptr_t(handle), &info, &message[0])
	if err := result(op, ret, &message[0]); err != nil {
		return Info{}, err
	}
	return Info{
		Index: int(info.index), Rate: int(info.rate), Bits: int(info.bits), Channels: int(info.channels),
		Codec: C.GoString(&info.codec[0]), Profile: C.GoString(&info.profile[0]), Layout: C.GoString(&info.layout[0]),
	}, nil
}

// Process reopens path and checks it against previously probed metadata.
// A nonempty output encodes TrueHD; an empty output streams normalized PCM to
// out. This low-level entry point does not perform transactional publication.
func Process(ctx context.Context, path, output string, expected Info, out io.Writer) error {
	if err := Check(ctx); err != nil {
		return err
	}
	if strings.ContainsRune(path, 0) || strings.ContainsRune(output, 0) {
		return fmt.Errorf("file path contains NUL")
	}
	if expected.Channels < 1 || expected.Channels > 8 || expected.Rate <= 0 ||
		(expected.Bits != 16 && expected.Bits != 24) {
		return fmt.Errorf("invalid expected PCM format")
	}
	if output == "" && out == nil {
		return fmt.Errorf("decode requires a PCM writer")
	}
	op := &operation{ctx: ctx, out: out}
	handle := cgo.NewHandle(op)
	defer handle.Delete()
	input := C.CString(path)
	defer C.free(unsafe.Pointer(input))
	var target *C.char
	if output != "" {
		target = C.CString(output)
		defer C.free(unsafe.Pointer(target))
	}
	var info C.HDMAInfo
	info.index, info.rate, info.bits, info.channels = C.int(expected.Index), C.int(expected.Rate), C.int(expected.Bits), C.int(expected.Channels)
	// Leave space for the terminating NUL in every fixed-size C field.
	// Silent truncation could make the native metadata checks compare aliases.
	for _, field := range []struct {
		value string
		dest  []C.char
	}{{expected.Layout, info.layout[:]}, {expected.Codec, info.codec[:]}, {expected.Profile, info.profile[:]}} {
		if len(field.value) >= len(field.dest) || strings.ContainsRune(field.value, 0) {
			return fmt.Errorf("invalid expected stream metadata")
		}
		for i := 0; i < len(field.value); i++ {
			field.dest[i] = C.char(field.value[i])
		}
	}
	var message [512]C.char
	ret := C.hdma_process(input, target, &info, C.uintptr_t(handle), &message[0])
	return result(op, ret, &message[0])
}

func result(op *operation, ret C.int, message *C.char) error {
	// Prefer typed Go errors so errors.Is still recognizes cancellation and
	// writer failures after the native stack has unwound.
	if op.ctx.Err() != nil || op.err != nil {
		return errors.Join(op.ctx.Err(), op.err)
	}
	if ret < 0 {
		return fmt.Errorf("FFmpeg library: %s", C.GoString(message))
	}
	return nil
}
