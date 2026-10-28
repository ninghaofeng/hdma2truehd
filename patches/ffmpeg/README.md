# Experimental TrueHD backend

Base: [FFmpeg 8.0.1](https://ffmpeg.org/releases/ffmpeg-8.0.1.tar.xz).

Archive SHA-256:
`05ee0b03119b45c0bdb4df654b96802e909e0a752f72e4fe3794f487229e5a41`

The patch modifies `libavcodec/mlpenc.c`:

- Adds `7.1` and rear-surround `5.1` layouts.
- Uses the TrueHD `0x31eb` restart-header noise type for eight channels.
  The decoder cannot handle eight matrix channels with the MLP noise type.
- Maps FFmpeg's native `7.1` PCM order (rear before side surrounds) to
  TrueHD's side-before-rear order. This does not mix or relabel speakers.
- Keeps the five-bit presentation field separate from the full thirteen-bit
  channel map.
- Analyzes an initial partial restart interval before flushing it, avoiding
  empty/undecodable output for very short inputs.

The existing two-substream organization is retained. The full multichannel
presentation uses both substreams. No new 5.1 downmix presentation is generated
for 7.1, and no Dolby Digital companion/core stream is generated.

The Go tool checks the linked library's build marker
`8.0.1-hdma2truehd-71-v1`. This marker is a version check, not cryptographic
provenance or a substitute for testing. `doctor` performs actual in-process
24-bit PCM round trips for each supported layout.

## Verification boundaries

The integration suite tests three layouts, six sample rates, two bit depths,
and eight lengths: 288 cases. Each checks codec, layout, rate, effective
precision, exact sample count, and complete PCM equality. A separate installed
FFmpeg command can optionally be used as an additional decoder; it is still
the same decoder family, not an independent implementation. The default tests
and all production operations call the libraries directly.

These are experimental raw TrueHD streams, not certified Dolby or Blu-ray
encodes. In particular, successful software round trips for eight-channel
176.4/192 kHz streams do not imply Blu-ray compatibility. Dolby documents
Blu-ray limits of eight channels at 96 kHz and six channels at 192 kHz:
[Dolby TrueHD](https://professional.dolby.com/tv/dolby-truehd/).
Hardware playback, bitrate compliance, downmix presentation behavior and
interoperability require further validation.

The build is intentionally minimal, disables command-line programs, shared
libraries, network support and third-party codecs. It includes the native
DTS decoder, TrueHD encoder/decoder, PCM decoders and required containers.
The cgo bridge statically links libavformat, libavcodec, libavutil and
libswresample. Sample conversion uses identical rates and layouts, disables
dithering, and rejects precision loss.

## Licensing

The patch is a modification of FFmpeg source and is provided under the
upstream file's LGPL 2.1-or-later terms. Original copyright notices remain
in the patched source. See [COPYING.LGPLv2.1](COPYING.LGPLv2.1).

Builds from the provided script do not enable GPL or nonfree components.
Verify the actual build configuration before redistribution. If distributing
modified FFmpeg binaries, provide the corresponding source, this patch,
license notices, build instructions and all other materials required by the
applicable license. The CLI now statically links these libraries through cgo.
The release script also provides application object files and a tested
re-linking command so users can replace compatible libraries. See
[re-linking instructions](../../docs/RELINKING.md).
Codec patent or trademark permissions are separate from the source-code license.

[HG3112/md71](https://github.com/HG3112/md71) was a workflow reference only.
Its E-AC-3 encoder changes and stream-muxing code are not used here.
