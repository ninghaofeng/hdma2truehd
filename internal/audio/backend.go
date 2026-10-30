// Package audio coordinates format validation, native codec operations and
// verified output publication. It never invokes an external media executable.
package audio

import (
	"context"
	"io"

	"github.com/ninghaofeng/hdma2truehd/internal/av"
)

const BackendVersion = av.Version

// Backend uses the statically linked codec libraries when its hooks are nil.
// The unexported hooks let orchestration tests inject failures without needing
// a malformed compressed fixture for every publication/error-handling case.
type Backend struct {
	probe  func(context.Context, string) (Info, error)
	encode func(context.Context, Info, string) error
	decode func(context.Context, Info, io.Writer) error
}

func (b Backend) Check(ctx context.Context) error {
	return av.Check(ctx)
}

// nativeInfo forwards processing constraints, not display/naming metadata.
// The native operation reopens the input and enforces its container scope again.
func nativeInfo(info Info) av.Info {
	return av.Info{Index: info.Stream.Index, Rate: info.Rate, Bits: info.Bits,
		Channels: info.Stream.Channels, Layout: info.Stream.Layout, Codec: info.Stream.Codec, Profile: info.Stream.Profile}
}

// Encode is the low-level codec operation; it does not validate the source
// profile, verify PCM or publish transactionally. Convert adds those guarantees.
// Keeping this separate also allows doctor to encode synthetic PCM fixtures.
func (b Backend) Encode(ctx context.Context, source Info, output string) error {
	if b.encode != nil {
		return b.encode(ctx, source, output)
	}
	return av.Process(ctx, source.Path, output, nativeInfo(source), nil)
}
