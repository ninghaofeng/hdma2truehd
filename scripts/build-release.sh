#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
source_dir="$root/.backend/ffmpeg-8.0.1"
if [[ ! -f "$source_dir/libavcodec/libavcodec.a" ]]; then
    printf 'Build the static FFmpeg libraries first: bash scripts/build-ffmpeg.sh\n' >&2
    exit 1
fi
if [[ "$(go env CGO_ENABLED)" != 1 ]]; then
    printf 'cgo is required; set CGO_ENABLED=1 and install a native C compiler.\n' >&2
    exit 1
fi
os="$(go env GOOS)"
arch="$(go env GOARCH)"
destination="${1:-$root/dist/hdma2truehd-$os-$arch}"
if [[ -e "$destination" ]]; then
    printf 'Refusing to overwrite release directory: %s\nChoose a new directory.\n' "$destination" >&2
    exit 1
fi
mkdir -p "$destination"
destination="$(cd "$destination" && pwd)"
export HDMA_CC="${CC:-cc}"
export HDMA_EXE_SUFFIX=""
native_destination="$destination"
if [[ "$os" == windows ]]; then
    HDMA_EXE_SUFFIX=".exe"
    native_destination="$(cygpath -m "$destination")"
fi
export HDMA_RELINK_DIR="$native_destination/relink"
wrapper="$native_destination/hdma2truehd-link-cc$HDMA_EXE_SUFFIX"
CGO_ENABLED=0 go build -o "$wrapper" ./tools/link-wrapper
PATH="$destination:$PATH" go build -a -trimpath \
    -ldflags="-linkmode=external -extld=hdma2truehd-link-cc" \
    -o "$destination/hdma2truehd$HDMA_EXE_SUFFIX" ./cmd/hdma2truehd
rm "$wrapper"
test -s "$HDMA_RELINK_DIR/relink.sh"
mkdir -p "$destination/licenses" "$destination/source/patches/ffmpeg" "$destination/source/scripts"
cp patches/ffmpeg/COPYING.LGPLv2.1 "$destination/licenses/"
cp patches/ffmpeg/README.md "$destination/licenses/FFmpeg-NOTICE.md"
cp "$(go env GOROOT)/LICENSE" "$destination/licenses/Go-LICENSE"
cp "$(go env GOROOT)/PATENTS" "$destination/licenses/Go-PATENTS"
cp docs/RELINKING.md "$destination/RELINKING.md"
cp patches/ffmpeg/0001-truehd-7.1-and-short-streams.patch "$destination/source/patches/ffmpeg/"
cp scripts/build-ffmpeg.sh "$destination/source/scripts/"
cp "$root/.backend/ffmpeg-8.0.1.tar.xz" "$destination/source/"
# Include the exact configured sources and build scripts, not just a source URL.
tar --exclude='*.o' --exclude='*.a' --exclude='*.d' --exclude='ffmpeg' \
    --exclude='ffprobe' --exclude='ffmpeg.exe' --exclude='ffprobe.exe' \
    -czf "$destination/source/ffmpeg-8.0.1-configured.tar.gz" -C "$root/.backend" ffmpeg-8.0.1
go version > "$destination/source/toolchain.txt"
"$HDMA_CC" --version >> "$destination/source/toolchain.txt"
go env GOOS GOARCH CGO_ENABLED >> "$destination/source/toolchain.txt"
printf '\nRelease and re-linkable objects: %s\nTest: bash "%s/relink/relink.sh"\n' "$destination" "$destination"
