package audio

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"

	"github.com/ninghaofeng/hdma2truehd/internal/av"
)

type PCM struct {
	Samples uint64
	Hashes  [][sha256.Size]byte
}

type pcmWriter struct {
	hashes []hash.Hash
	word   [4]byte
	used   int
	words  uint64
	bits   int
}

func newPCMWriter(channels, bits int) *pcmWriter {
	w := &pcmWriter{hashes: make([]hash.Hash, channels), bits: bits}
	for ch := range w.hashes {
		w.hashes[ch] = sha256.New()
	}
	return w
}

func (w *pcmWriter) Write(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 {
		copied := copy(w.word[w.used:], p)
		w.used += copied
		p = p[copied:]
		if w.used != 4 {
			continue
		}
		value := binary.LittleEndian.Uint32(w.word[:])
		mask := uint32(0xff)
		if w.bits == 16 {
			mask = 0xffff
		}
		if value&mask != 0 {
			return 0, fmt.Errorf("decoded PCM exceeds declared %d-bit precision", w.bits)
		}
		w.hashes[w.words%uint64(len(w.hashes))].Write(w.word[:])
		w.words++
		w.used = 0
	}
	return n, nil
}

func (w *pcmWriter) finish() (PCM, error) {
	if w.used != 0 || w.words%uint64(len(w.hashes)) != 0 {
		return PCM{}, fmt.Errorf("incomplete PCM sample or channel frame")
	}
	result := PCM{Samples: w.words / uint64(len(w.hashes)), Hashes: make([][sha256.Size]byte, len(w.hashes))}
	if result.Samples == 0 {
		return PCM{}, fmt.Errorf("decoded audio contains no PCM samples")
	}
	for ch, h := range w.hashes {
		copy(result.Hashes[ch][:], h.Sum(nil))
	}
	return result, nil
}

func (b Backend) Digest(ctx context.Context, info Info) (PCM, error) {
	w := newPCMWriter(info.Stream.Channels, info.Bits)
	var err error
	if b.decode != nil {
		err = b.decode(ctx, info, w)
	} else {
		err = av.Process(ctx, info.Path, "", nativeInfo(info), w)
	}
	if err != nil {
		return PCM{}, fmt.Errorf("decode %s: %w", info.Path, err)
	}
	return w.finish()
}

func ComparePCM(source, output PCM) error {
	if source.Samples != output.Samples {
		return fmt.Errorf("PCM sample count mismatch: source=%d output=%d", source.Samples, output.Samples)
	}
	if len(source.Hashes) != len(output.Hashes) {
		return fmt.Errorf("PCM channel count mismatch")
	}
	for ch := range source.Hashes {
		if source.Hashes[ch] != output.Hashes[ch] {
			return fmt.Errorf("PCM mismatch in channel %d", ch+1)
		}
	}
	return nil
}
