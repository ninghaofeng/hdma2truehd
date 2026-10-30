#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
source "$root/scripts/build-env.sh"
build_target=both
# Preserve explicit legacy callers, but an ordinary invocation builds both.
case "${HDMA_UI:-}" in
    "") ;;
    0) build_target=cli ;;
    1) build_target=ui ;;
    *) printf 'HDMA_UI must be 0 or 1 when set.\n' >&2; exit 2 ;;
esac
binary_only=false
destination=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --target)
            if [[ $# -lt 2 ]]; then
                printf '%s\n' '--target requires both, cli or ui.' >&2
                exit 2
            fi
            build_target="$2"
            shift 2
            ;;
        --binary-only)
            binary_only=true
            shift
            ;;
        --help|-h)
            printf 'Usage: build-release.sh [--target both|cli|ui] [--binary-only] [DESTINATION]\n'
            printf 'Default: package CLI and UI together, using existing native FFmpeg libraries.\n'
            printf 'Binary-only: build applications in bin/ without tests or release materials.\n'
            exit 0
            ;;
        -*)
            printf 'Unknown build option: %s\n' "$1" >&2
            exit 2
            ;;
        *)
            if [[ -n "$destination" || -z "$1" ]]; then
                printf 'Specify exactly one nonempty destination directory.\n' >&2
                exit 2
            fi
            destination="$1"
            shift
            ;;
    esac
done
case "$build_target" in
    both) apps=(hdma2truehd hdma2truehd-ui) ;;
    cli) apps=(hdma2truehd) ;;
    ui) apps=(hdma2truehd-ui) ;;
    *) printf 'Invalid build target: %s (choose both, cli or ui).\n' "$build_target" >&2; exit 2 ;;
esac
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
check_release_toolchain
if [[ "$build_target" != cli && "$os" != darwin && "$os" != windows ]]; then
    printf 'The desktop UI supports macOS and Windows only; use --target cli on other platforms.\n' >&2
    exit 1
fi
resources=()
metadata=""
wrapper=""
cleanup() {
    for resource in ${resources[@]+"${resources[@]}"}; do rm -f -- "$resource"; done
    if [[ -n "$metadata" ]]; then rm -r -- "$metadata"; fi
    if [[ -n "$wrapper" ]]; then rm -f -- "$wrapper"; fi
}
trap cleanup EXIT
if [[ "$os" == windows ]]; then
    if ! command -v windres >/dev/null 2>&1; then
        printf 'windres is required for Windows version/icon resources; install the matching MSYS2 toolchain.\n' >&2
        exit 1
    fi
    for app in "${apps[@]}"; do
        resource_path="$root/cmd/$app/resource_windows_${arch}.syso"
        if [[ -e "$resource_path" || -L "$resource_path" ]]; then
            printf 'Refusing to overwrite an existing resource object: %s\n' "$resource_path" >&2
            exit 1
        fi
    done
fi
if [[ -z "$destination" ]]; then
    if [[ "$binary_only" == true ]]; then
        destination="$root/bin"
    elif [[ "$build_target" == ui ]]; then
        destination="$root/dist/hdma2truehd-ui-$os-$arch"
    else
        destination="$root/dist/hdma2truehd-$os-$arch"
    fi
fi
if [[ "$binary_only" == false && ( -e "$destination" || -L "$destination" ) ]]; then
    printf 'Refusing to overwrite release directory: %s\nChoose a new directory.\n' "$destination" >&2
    exit 1
fi
mkdir -p "$destination"
destination="$(cd "$destination" && pwd)"
export HDMA_CC="${CC:-cc}"
export HDMA_EXE_SUFFIX=""
metadata="$(mktemp -d "$destination/.metadata-XXXXXX")"
# Pure Go generation also works before the media backend is linked.
CGO_ENABLED=0 go run ./tools/build-metadata --output-dir "$metadata"
cp "$metadata/VERSION.txt" "$destination/"
native_destination="$destination"
if [[ "$os" == windows ]]; then
    # Go and its linker wrapper are native Windows programs, unlike this MSYS2
    # shell. Give them a drive-letter path for files they create themselves.
    HDMA_EXE_SUFFIX=".exe"
    native_destination="$(cygpath -m "$destination")"
fi
build_path="$PATH"
if [[ "$binary_only" == false ]]; then
    wrapper="$native_destination/hdma2truehd-link-cc$HDMA_EXE_SUFFIX"
    CGO_ENABLED=0 go build -o "$wrapper" ./tools/link-wrapper
    build_path="$destination:$PATH"
fi
if [[ "$os" == windows ]]; then
    # Go embeds this COFF object in the PE executable. The captured application
    # objects also retain it for LGPL relinking after the source copy is removed.
    for app in "${apps[@]}"; do
        resource="$root/cmd/$app/resource_windows_${arch}.syso"
        resources+=("$resource")
        input="$metadata/cli.rc"
        if [[ "$app" == hdma2truehd-ui ]]; then input="$root/cmd/$app/app.rc"; fi
        windres -I "$metadata" -I "$root/cmd/hdma2truehd-ui" -i "$input" -O coff -o "$resource"
    done
fi
packages=()
for app in "${apps[@]}"; do
    package="./cmd/$app"
    packages+=("$package")
    tags=""
    relink=relink
    ldflags="-linkmode=external"
    # Both Go mains produce names such as go.o. Separate capture directories
    # prevent object collisions while sources/licenses remain shared.
    if [[ "$app" == hdma2truehd-ui ]]; then
        tags=desktop,production
        if [[ "$build_target" == both ]]; then relink=relink-ui; fi
        if [[ "$os" == windows ]]; then ldflags+=" -H windowsgui"; fi
    fi
    if [[ "$binary_only" == false ]]; then
        export HDMA_RELINK_DIR="$native_destination/$relink"
        export HDMA_RELINK_NAME="$app-relinked"
        ldflags+=" -extld=hdma2truehd-link-cc"
    fi
    # External archives are not Go source dependencies. Force rebuilding so
    # replacing FFmpeg cannot accidentally reuse a previously cached link.
    PATH="$build_path" go build -a -trimpath \
        -tags "$tags" -ldflags="$ldflags" \
        -o "$destination/$app$HDMA_EXE_SUFFIX" "$package"
    if [[ "$os" == darwin ]]; then
        check_macos_deployment_target "$destination/$app"
    fi
    if [[ "$binary_only" == false ]]; then test -s "$HDMA_RELINK_DIR/relink.sh"; fi
done
if [[ "$build_target" != cli && "$os" == darwin ]]; then
    bundle="$destination/HDMA Converter.app/Contents"
    mkdir -p "$bundle/MacOS" "$bundle/Resources"
    cp "$metadata/Info.plist" "$bundle/"
    cp cmd/hdma2truehd-ui/assets/app.icns "$bundle/Resources/"
    cp "$destination/hdma2truehd-ui" "$bundle/MacOS/"
fi
if [[ "$binary_only" == true ]]; then
    # Local mode deliberately stops before source/license/relink packaging;
    # these executables alone are not a redistributable LGPL release kit.
    printf '\nApplications built (%s): %s\nNo tests or release materials were generated.\n' "$build_target" "$destination"
    exit 0
fi
# A static binary alone is not the complete release: package licenses,
# corresponding FFmpeg sources/patches and the inputs needed to relink it.
mkdir -p "$destination/licenses" "$destination/source/patches/ffmpeg" "$destination/source/scripts"
cp patches/ffmpeg/COPYING.LGPLv2.1 "$destination/licenses/"
cp patches/ffmpeg/README.md "$destination/licenses/FFmpeg-NOTICE.md"
cp "$(go env GOROOT)/LICENSE" "$destination/licenses/Go-LICENSE"
cp "$(go env GOROOT)/PATENTS" "$destination/licenses/Go-PATENTS"
cp docs/RELINKING.md "$destination/RELINKING.md"
cp patches/ffmpeg/0001-truehd-7.1-and-short-streams.patch \
    patches/ffmpeg/0002-eac3-dependent-7.1.patch \
    patches/ffmpeg/0003-stream-info-discard.patch \
    patches/ffmpeg/0004-matroska-audio-scope.patch "$destination/source/patches/ffmpeg/"
cp scripts/build-ffmpeg.sh "$destination/source/scripts/"
cp scripts/build-env.sh "$destination/source/scripts/"
if [[ "$build_target" != cli ]]; then
    cp go.mod go.sum "$destination/source/"
    cp scripts/build-ui.sh scripts/build-release.sh "$destination/source/scripts/"
    # Ship license notices for modules actually linked into the UI, not every
    # module appearing transitively in the dependency manifest.
    go list -tags desktop,production -deps -f '{{with .Module}}{{if not .Main}}{{.Path}}@{{.Version}}|{{.Dir}}{{end}}{{end}}' "${packages[@]}" |
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
fi
cp "$root/.backend/ffmpeg-8.0.1.tar.xz" "$destination/source/"
# Include the exact configured sources and build scripts, not just a source URL.
tar --exclude='*.o' --exclude='*.a' --exclude='*.d' --exclude='ffmpeg' \
    --exclude='ffprobe' --exclude='ffmpeg.exe' --exclude='ffprobe.exe' \
    -czf "$destination/source/ffmpeg-8.0.1-configured.tar.gz" -C "$root/.backend" ffmpeg-8.0.1
go version > "$destination/source/toolchain.txt"
"$HDMA_CC" --version >> "$destination/source/toolchain.txt"
go env GOOS GOARCH CGO_ENABLED >> "$destination/source/toolchain.txt"
printf '\nRelease and re-linkable objects (%s): %s\nTest: bash "%s/relink/relink.sh"\n' "$build_target" "$destination" "$destination"
if [[ "$build_target" == both ]]; then
    printf 'UI relink test: bash "%s/relink-ui/relink.sh"\n' "$destination"
fi
