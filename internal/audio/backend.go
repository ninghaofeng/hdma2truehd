package audio

import (
	"context"
	"io"

	"github.com/ninghaofeng/hdma2truehd/internal/av"
)

const BackendVersion = av.Version

type Backend struct {
	probe  func(context.Context, string) (Info, error)
	encode func(context.Context, Info, string) error
	decode func(context.Context, Info, io.Writer) error
}

func (b Backend) Check(ctx context.Context) error {
	return av.Check(ctx)
}

func nativeInfo(info Info) av.Info {
	return av.Info{Index: info.Stream.Index, Rate: info.Rate, Bits: info.Bits,
		Channels: info.Stream.Channels, Layout: info.Stream.Layout, Codec: info.Stream.Codec, Profile: info.Stream.Profile}
}

func (b Backend) Encode(ctx context.Context, source Info, output string) error {
	if b.encode != nil {
		return b.encode(ctx, source, output)
	}
	return av.Process(ctx, source.Path, output, nativeInfo(source), nil)
}
