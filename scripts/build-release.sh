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
app=hdma2truehd
package=./cmd/hdma2truehd
tags=""
case "${HDMA_UI:-0}" in
    0) ;;
    1)
        if [[ "$os" != darwin && "$os" != windows ]]; then
            printf 'The desktop UI supports macOS and Windows only.\n' >&2
            exit 1
        fi
        app=hdma2truehd-ui
        package=./cmd/hdma2truehd-ui
        tags=desktop,production
        ;;
    *) printf 'HDMA_UI must be 0 or 1.\n' >&2; exit 2 ;;
esac
destination="${1:-$root/dist/$app-$os-$arch}"
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
    # Go and its linker wrapper are native Windows programs, unlike this MSYS2
    # shell. Give them a drive-letter path for files they create themselves.
    HDMA_EXE_SUFFIX=".exe"
    native_destination="$(cygpath -m "$destination")"
fi
export HDMA_RELINK_DIR="$native_destination/relink"
export HDMA_RELINK_NAME="$app-relinked"
wrapper="$native_destination/hdma2truehd-link-cc$HDMA_EXE_SUFFIX"
# The pure-Go wrapper intercepts the final external link while Go's temporary
# objects still exist. Force relinking so cached build results cannot skip it.
CGO_ENABLED=0 go build -o "$wrapper" ./tools/link-wrapper
ldflags="-linkmode=external -extld=hdma2truehd-link-cc"
if [[ "$app" == hdma2truehd-ui && "$os" == windows ]]; then
    ldflags+=" -H windowsgui"
    if ! command -v windres >/dev/null 2>&1; then
        printf 'windres is required for the Windows application icon; install UCRT64 binutils.\n' >&2
        exit 1
    fi
    resource="$root/cmd/hdma2truehd-ui/resource_windows_${arch}.syso"
    if [[ -e "$resource" ]]; then
        printf 'Refusing to overwrite an existing resource object: %s\n' "$resource" >&2
        exit 1
    fi
    # Go embeds this COFF object in the PE executable. The captured application
    # objects also retain it for LGPL relinking after the source copy is removed.
    trap 'rm -f -- "$resource"' EXIT
    windres -I "$root/cmd/hdma2truehd-ui" -i "$root/cmd/hdma2truehd-ui/app.rc" \
        -O coff -o "$resource"
fi
PATH="$destination:$PATH" go build -a -trimpath \
    -tags "$tags" -ldflags="$ldflags" \
    -o "$destination/$app$HDMA_EXE_SUFFIX" "$package"
rm "$wrapper"
test -s "$HDMA_RELINK_DIR/relink.sh"
# A static binary alone is not the complete release: package licenses,
# corresponding FFmpeg sources/patches and the inputs needed to relink it.
mkdir -p "$destination/licenses" "$destination/source/patches/ffmpeg" "$destination/source/scripts"
cp patches/ffmpeg/COPYING.LGPLv2.1 "$destination/licenses/"
cp patches/ffmpeg/README.md "$destination/licenses/FFmpeg-NOTICE.md"
cp "$(go env GOROOT)/LICENSE" "$destination/licenses/Go-LICENSE"
cp "$(go env GOROOT)/PATENTS" "$destination/licenses/Go-PATENTS"
cp docs/RELINKING.md "$destination/RELINKING.md"
cp patches/ffmpeg/0001-truehd-7.1-and-short-streams.patch \
    patches/ffmpeg/0002-eac3-dependent-7.1.patch "$destination/source/patches/ffmpeg/"
cp scripts/build-ffmpeg.sh "$destination/source/scripts/"
if [[ "$app" == hdma2truehd-ui ]]; then
    cp go.mod go.sum "$destination/source/"
    cp scripts/build-ui.sh scripts/build-release.sh "$destination/source/scripts/"
    # Ship license notices for modules actually linked into the UI, not every
    # module appearing transitively in the dependency manifest.
    go list -tags "$tags" -deps -f '{{with .Module}}{{if not .Main}}{{.Path}}@{{.Version}}|{{.Dir}}{{end}}{{end}}' "$package" |
        sort -u > "$destination/source/modules.txt"
    while IFS='|' read -r module directory; do
        [[ -z "$module" ]] && continue
        if [[ "$os" == windows ]]; then directory="$(cygpath -u "$directory")"; fi
        notices="$destination/licenses/${module//\//_}"
        mkdir -p "$notices"
        found=false
        for notice in "$directory"/LICENSE* "$directory"/LICENCE* "$directory"/COPYING* "$directory"/NOTICE* "$directory"/PATENTS*; do
            if [[ -f "$notice" ]]; then cp "$notice" "$notices/"; found=true; fi
        done
        if [[ "$found" == false ]]; then
            printf 'WARNING: review license notices for %s (none found at module root).\n' "$module" >&2
        fi
    done < "$destination/source/modules.txt"
    if [[ "$os" == darwin ]]; then
        bundle="$destination/HDMA Converter.app/Contents"
        mkdir -p "$bundle/MacOS" "$bundle/Resources"
        cp cmd/hdma2truehd-ui/Info.plist "$bundle/"
        cp cmd/hdma2truehd-ui/assets/app.icns "$bundle/Resources/"
        cp "$destination/$app" "$bundle/MacOS/"
    fi
fi
cp "$root/.backend/ffmpeg-8.0.1.tar.xz" "$destination/source/"
# Include the exact configured sources and build scripts, not just a source URL.
tar --exclude='*.o' --exclude='*.a' --exclude='*.d' --exclude='ffmpeg' \
    --exclude='ffprobe' --exclude='ffmpeg.exe' --exclude='ffprobe.exe' \
    -czf "$destination/source/ffmpeg-8.0.1-configured.tar.gz" -C "$root/.backend" ffmpeg-8.0.1
go version > "$destination/source/toolchain.txt"
"$HDMA_CC" --version >> "$destination/source/toolchain.txt"
go env GOOS GOARCH CGO_ENABLED >> "$destination/source/toolchain.txt"
printf '\nRelease and re-linkable objects: %s\nTest: bash "%s/relink/relink.sh"\n' "$destination" "$destination"
