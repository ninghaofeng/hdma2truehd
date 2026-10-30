#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
source "$root/scripts/build-env.sh"
build="${1:-$root/.backend}"
mkdir -p "$build"
build="$(cd "$build" && pwd)"
archive="$build/ffmpeg-8.0.1.tar.xz"
source_dir="$build/ffmpeg-8.0.1"

# Never patch/reconfigure an existing tree: it may contain local changes or
# archives built for a different platform. Reuse is handled by build-common.sh.
if [[ -e "$source_dir" || -L "$source_dir" ]]; then
    printf 'Refusing to modify existing source directory: %s\nUse a new build directory, or the platform build entry point with --reuse-backend / --clean-backend (-ReuseBackend / -CleanBackend on Windows).\n' "$source_dir" >&2
    exit 1
fi

# Backend cleanup preserves this cache. Reuse never bypasses the checksum
# below; reject a corrupt archive rather than extracting or overwriting it.
if [[ -e "$archive" || -L "$archive" ]]; then
    printf 'Reusing FFmpeg source archive: %s\n' "$archive"
else
    curl --fail --location --retry 2 https://ffmpeg.org/releases/ffmpeg-8.0.1.tar.xz --output "$archive"
fi
# Verify the pinned upstream bytes before extracting or applying our patch.
expected=05ee0b03119b45c0bdb4df654b96802e909e0a752f72e4fe3794f487229e5a41
if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$archive")"
else
    actual="$(shasum -a 256 "$archive")"
fi
if [[ "${actual%% *}" != "$expected" ]]; then
    printf 'FFmpeg archive checksum mismatch; refusing to extract.\nRemove the corrupt archive and retry: %s\n' "$archive" >&2
    exit 1
fi

tar -xf "$archive" -C "$build"
cd "$source_dir"
patch --batch --forward -p1 < "$root/patches/ffmpeg/0001-truehd-7.1-and-short-streams.patch"
patch --batch --forward -p1 < "$root/patches/ffmpeg/0002-eac3-dependent-7.1.patch"
patch --batch --forward -p1 < "$root/patches/ffmpeg/0003-stream-info-discard.patch"
patch --batch --forward -p1 < "$root/patches/ffmpeg/0004-matroska-audio-scope.patch"
extra_ldflags=""
extra_cflags=""
compiler="${CC:-gcc}"
if [[ "${MSYSTEM:-}" == CLANGARM64 ]]; then compiler="${CC:-clang}"; fi
if [[ "${MSYSTEM:-}" == UCRT64 || "${MSYSTEM:-}" == CLANGARM64 ]]; then
    extra_ldflags="-static"
elif [[ "$(uname -s)" == Darwin ]]; then
    extra_cflags="-mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET"
    extra_ldflags="$extra_cflags"
fi
# Build only in-process codec/container support. Disabling autodetection,
# network, shared libraries and programs keeps system packages from silently
# changing runtime dependencies. PCM/WAV support exists for doctor and tests.
# The version marker is checked by the Go bridge to reject a stock backend.
# FFmpeg's ac3 parser handles both AC-3 and E-AC-3.
./configure \
    --cc="$compiler" \
    --disable-everything --disable-autodetect --disable-doc --disable-network \
    --disable-debug --disable-x86asm \
    --disable-programs --disable-shared --enable-static --enable-pic \
    --disable-avdevice --disable-avfilter --disable-swscale \
    --enable-swresample \
    --enable-protocol=file \
    --enable-decoder=dca,truehd,eac3,ac3,pcm_s16le,pcm_s24le,pcm_s32le \
    --enable-encoder=truehd,eac3,ac3,pcm_s16le,pcm_s24le,pcm_s32le \
    --enable-parser=dca,mlp,ac3 \
    --enable-demuxer=dts,dtshd,truehd,eac3,ac3,matroska,wav,pcm_s16le,pcm_s24le,pcm_s32le \
    --enable-muxer=truehd,eac3,ac3,wav,pcm_s16le,pcm_s24le,pcm_s32le \
    --extra-ldflags="$extra_ldflags" \
    --extra-cflags="$extra_cflags" \
    --extra-version=hdma2truehd-71-v6
make -j"${JOBS:-2}"
printf '\nStatic libraries built in %s\nRun go test ./... and bash scripts/build-release.sh.\n' "$source_dir"
