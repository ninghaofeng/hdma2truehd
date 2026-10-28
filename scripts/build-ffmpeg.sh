#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
build="${1:-$root/.backend}"
mkdir -p "$build"
build="$(cd "$build" && pwd)"
archive="$build/ffmpeg-8.0.1.tar.xz"
source_dir="$build/ffmpeg-8.0.1"

# Never patch/reconfigure an existing tree: it may contain local changes or
# archives built for a different platform. Reuse is handled by build-common.sh.
if [[ -e "$source_dir" ]]; then
    printf 'Refusing to modify existing source directory: %s\nUse a new build directory.\n' "$source_dir" >&2
    exit 1
fi

curl --fail --location --retry 2 https://ffmpeg.org/releases/ffmpeg-8.0.1.tar.xz --output "$archive"
# Verify the pinned upstream bytes before extracting or applying our patch.
expected=05ee0b03119b45c0bdb4df654b96802e909e0a752f72e4fe3794f487229e5a41
if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$archive")"
else
    actual="$(shasum -a 256 "$archive")"
fi
if [[ "${actual%% *}" != "$expected" ]]; then
    printf 'FFmpeg archive checksum mismatch; refusing to extract.\n' >&2
    exit 1
fi

tar -xf "$archive" -C "$build"
cd "$source_dir"
patch --batch --forward -p1 < "$root/patches/ffmpeg/0001-truehd-7.1-and-short-streams.patch"
extra_ldflags=""
if [[ "${MSYSTEM:-}" == "UCRT64" ]]; then
    extra_ldflags="-static"
fi
# Build only in-process codec/container support. Disabling autodetection,
# network, shared libraries and programs keeps system packages from silently
# changing runtime dependencies. PCM/WAV support exists for doctor and tests.
# The version marker is checked by the Go bridge to reject a stock backend.
./configure \
    --disable-everything --disable-autodetect --disable-doc --disable-network \
    --disable-debug --disable-x86asm \
    --disable-programs --disable-shared --enable-static --enable-pic \
    --disable-avdevice --disable-avfilter --disable-swscale \
    --enable-swresample \
    --enable-protocol=file \
    --enable-decoder=dca,truehd,pcm_s16le,pcm_s24le,pcm_s32le \
    --enable-encoder=truehd,pcm_s16le,pcm_s24le,pcm_s32le \
    --enable-parser=dca,mlp \
    --enable-demuxer=dts,dtshd,truehd,wav,pcm_s16le,pcm_s24le,pcm_s32le \
    --enable-muxer=truehd,wav,pcm_s16le,pcm_s24le,pcm_s32le \
    --extra-ldflags="$extra_ldflags" \
    --extra-version=hdma2truehd-71-v1
make -j"${JOBS:-2}"
printf '\nStatic libraries built in %s\nRun go test ./... and bash scripts/build-release.sh.\n' "$source_dir"
