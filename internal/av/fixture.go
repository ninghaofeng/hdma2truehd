package av

import (
	"encoding/binary"
	"fmt"
	"os"
)

// WritePCMFixture writes a small integer PCM WAV used by doctor and codec tests.
// Input is interleaved, little-endian int32 PCM with effective bits left-aligned,
// matching the native decoder callback representation.
func WritePCMFixture(path string, pcm []byte, info Info) error {
	if info.Channels < 1 || info.Channels > 8 || info.Rate <= 0 ||
		(info.Bits != 16 && info.Bits != 24) || len(pcm)%(info.Channels*4) != 0 {
		return fmt.Errorf("invalid PCM fixture format")
	}
	// WAVEFORMATEXTENSIBLE masks preserve the distinction between side and
	// back surrounds; a plain channel count would make layout tests ambiguous.
	var mask uint32
	switch info.Layout {
	case "5.1":
		mask = 0x3f
	case "5.1(side)":
		mask = 0x60f
	case "7.1":
		mask = 0x63f
	default:
		return fmt.Errorf("unsupported fixture layout %q", info.Layout)
	}
	if (mask == 0x63f && info.Channels != 8) || (mask != 0x63f && info.Channels != 6) {
		return fmt.Errorf("fixture channel count disagrees with layout")
	}
	width := info.Bits / 8
	size := len(pcm) / 4 * width
	padding := size % 2
	data := make([]byte, 68+size+padding)
	// A 40-byte extensible fmt chunk carries the speaker mask, valid bit
	// count and PCM subtype GUID. RIFF chunks are padded to even byte lengths.
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 40)
	binary.LittleEndian.PutUint16(data[20:], 0xfffe)
	binary.LittleEndian.PutUint16(data[22:], uint16(info.Channels))
	binary.LittleEndian.PutUint32(data[24:], uint32(info.Rate))
	binary.LittleEndian.PutUint32(data[28:], uint32(info.Rate*info.Channels*width))
	binary.LittleEndian.PutUint16(data[32:], uint16(info.Channels*width))
	binary.LittleEndian.PutUint16(data[34:], uint16(info.Bits))
	binary.LittleEndian.PutUint16(data[36:], 22)
	binary.LittleEndian.PutUint16(data[38:], uint16(info.Bits))
	binary.LittleEndian.PutUint32(data[40:], mask)
	copy(data[44:], []byte{1, 0, 0, 0, 0, 0, 0x10, 0, 0x80, 0, 0, 0xaa, 0, 0x38, 0x9b, 0x71})
	copy(data[60:], "data")
	binary.LittleEndian.PutUint32(data[64:], uint32(size))
	for i := 0; i < len(pcm)/4; i++ {
		// Remove only the unused low bytes of the left-aligned int32; the WAV
		// stores packed 16/24-bit samples without changing their effective bits.
		copy(data[68+i*width:], pcm[i*4+4-width:i*4+4])
	}
	return os.WriteFile(path, data, 0600)
}
