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

// EAC3Stats is shared by both lossy encoders. Samples counts original PCM per
// channel; Decoded also includes the raw stream's fixed delay and frame padding.
type EAC3Stats struct {
	Samples, Decoded uint64
}

// eac3Writer feeds the shared AC-3/E-AC-3 native lifecycle synchronously from
// Process callbacks; its historical name does not restrict the output codec.
type eac3Writer struct {
	op     *C.HDMAEAC3
	status *operation
}

func (w eac3Writer) Write(p []byte) (int, error) {
	if err := w.status.ctx.Err(); err != nil {
		return 0, err
	}
	if len(p) == 0 || len(p) > 1<<30 {
		return 0, fmt.Errorf("invalid AC-3 / E-AC-3 PCM block size")
	}
	var message [512]C.char
	// C copies this borrowed PCM buffer into its float FIFO before returning;
	// neither the encoder nor its later flush may retain the supplied pointer.
	ret := C.hdma_eac3_write(w.op, (*C.uchar)(unsafe.Pointer(&p[0])), C.int(len(p)), &message[0])
	if err := result(w.status, ret, &message[0]); err != nil {
		return 0, err
	}
	return len(p), nil
}

// EncodeEAC3 keeps decoding in the existing strict native pipeline and forwards
// normalized PCM synchronously to a separate lossy encoder. It never uses an
// external executable. Each output is still staged/published by package audio.
// bitrate is the total bits/second, unlike the CLI's kb/s option. Ordinary DTS
// overrides a valid requested bitrate with the fixed maximum of 1024000.
func EncodeEAC3(ctx context.Context, path, output string, expected Info, bitrate int) (EAC3Stats, error) {
	return encodeDolby(ctx, path, output, expected, bitrate, false)
}

// EncodeAC3 encodes only 48 kHz 5.1(side), at a fixed 640 kb/s. It shares
// E-AC-3's strict decoding, frame accounting and transactional caller contract.
func EncodeAC3(ctx context.Context, path, output string, expected Info) (EAC3Stats, error) {
	return encodeDolby(ctx, path, output, expected, 640000, true)
}

// encodeDolby shares FIFO/flush accounting, not format eligibility. AC-3 uses
// one 5.1 encoder; only E-AC-3 can use paired substreams for 7.1.
func encodeDolby(ctx context.Context, path, output string, expected Info, bitrate int, ac3 bool) (EAC3Stats, error) {
	if err := Check(ctx); err != nil {
		return EAC3Stats{}, err
	}
	if ac3 && (expected.Channels != 6 || expected.Layout != "5.1(side)") {
		return EAC3Stats{}, fmt.Errorf("AC-3 requires 48000 Hz 5.1(side); no automatic resampling/remapping")
	}
	if expected.Rate != 48000 || (expected.Layout != "5.1(side)" && expected.Layout != "7.1") {
		return EAC3Stats{}, fmt.Errorf("AC-3 / E-AC-3 requires 48000 Hz and a supported layout; no automatic resampling/remapping")
	}
	if bitrate < 384000 || bitrate > 1536000 || bitrate%32000 != 0 {
		return EAC3Stats{}, fmt.Errorf("invalid AC-3 / E-AC-3 bitrate")
	}
	if strings.ContainsRune(output, 0) || output == "" {
		return EAC3Stats{}, fmt.Errorf("invalid AC-3 / E-AC-3 output path")
	}
	// Core DTS is already lossy: decode its full-width representation without
	// asserting MA's zero low-bit invariant or allowing TrueHD encoding.
	if expected.Codec == "dts" && expected.Profile == "DTS" {
		if expected.Rate != 48000 || expected.Channels != 6 || expected.Layout != "5.1(side)" {
			return EAC3Stats{}, fmt.Errorf("ordinary DTS requires 48000 Hz 5.1(side)")
		}
		expected.Bits = 32
		if !ac3 {
			bitrate = 1024000
		}
	}
	// The decoder's Process call owns a separate handle. This one stays live
	// through encoder finalization/close, including I/O cancellation callbacks.
	status := &operation{ctx: ctx}
	handle := cgo.NewHandle(status)
	defer handle.Delete()
	target := C.CString(output)
	defer C.free(unsafe.Pointer(target))
	var encoder *C.HDMAEAC3
	var message [512]C.char
	var ret C.int
	if ac3 {
		ret = C.hdma_ac3_open(target, C.int(expected.Channels), C.uintptr_t(handle), &encoder, &message[0])
	} else {
		ret = C.hdma_eac3_open(target, C.int(expected.Channels), C.int(bitrate), C.uintptr_t(handle), &encoder, &message[0])
	}
	// Cancellation may arrive after C successfully opened the output but before
	// result observes it. Close even on that path; NULL is safe after C failure.
	defer C.hdma_eac3_close(encoder)
	if err := result(status, ret, &message[0]); err != nil {
		return EAC3Stats{}, err
	}
	if err := Process(ctx, path, "", expected, eac3Writer{encoder, status}); err != nil {
		return EAC3Stats{}, err
	}
	// Process's final progress event ends source decoding, not lossy encoding:
	// the delayed tail and frame padding are still owned by finish below.
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
