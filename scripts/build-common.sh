#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
platform="${1:?Pass macos or windows}"
shift
reuse=false
binary_only=false
destination=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --reuse-backend)
            reuse=true
            shift
            ;;
        --binary-only)
            binary_only=true
            shift
            ;;
        --output-dir)
            if [[ $# -lt 2 || -z "$2" || "$2" == --* ]]; then
                printf '%s\n' '--output-dir requires a directory.' >&2
                exit 2
            fi
            destination="$2"
            shift 2
            ;;
        --help|-h)
            printf 'Usage: build-macos.sh [--binary-only] [--reuse-backend] [--output-dir DIR]\n'
            printf 'Windows: build-windows.ps1 [-Msys2Root PATH] [-BinaryOnly] [-ReuseBackend] [-OutputDir DIR]\n'
            printf 'Default: build libraries, test and package a complete release in dist/.\n'
            printf 'Binary-only: reuse existing static libraries and rebuild bin/hdma2truehd (no tests or release materials).\n'
            exit 0
            ;;
        *)
            printf 'Unknown build option: %s\n' "$1" >&2
            exit 2
            ;;
    esac
done

case "$platform" in
    macos)
        if [[ "$(uname -s)" != Darwin ]]; then
            printf 'The macOS build requires a macOS host.\n' >&2
            exit 1
        fi
        target=darwin
        export CC="${CC:-clang}"
        ;;
    windows)
        if [[ "${OS:-}" != Windows_NT || "${MSYSTEM:-}" != UCRT64 ]]; then
            printf 'The Windows build requires MSYS2 UCRT64. Use scripts/build-windows.ps1.\n' >&2
            exit 1
        fi
        target=windows
        export CC="${CC:-gcc}"
        ;;
    *)
        printf 'Unsupported build platform: %s\n' "$platform" >&2
        exit 2
        ;;
esac
tools=(go "$CC")
if [[ "$binary_only" == false ]]; then
    tools+=(bash make curl tar xz patch)
fi
if [[ "$platform" == macos ]]; then
    tools+=(otool)
else
    tools+=(objdump)
fi
for tool in "${tools[@]}"; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        printf 'Missing build tool: %s. See README.md for platform prerequisites.\n' "$tool" >&2
        exit 1
    fi
done
export CGO_ENABLED=1
if [[ "$(go env GOOS)" != "$target" || "$(go env GOARCH)" != "$(go env GOHOSTARCH)" ]]; then
    printf 'A native build is required; unset cross-compilation GOOS/GOARCH overrides.\n' >&2
    exit 1
fi
if [[ "$platform" == windows && "$(go env GOARCH)" != amd64 ]]; then
    printf 'The UCRT64 Windows build currently requires amd64 Go and GCC.\n' >&2
    exit 1
fi
suffix=""
if [[ "$platform" == windows ]]; then
    suffix=".exe"
fi

check_static_dependencies() {
    local imports
    if [[ "$platform" == macos ]]; then
        imports="$(otool -L "$1")"
        printf '%s\n' "$imports"
        if ! printf '%s\n' "$imports" | awk 'NR > 1 && $1 !~ /^\/usr\/lib\// && $1 !~ /^\/System\/Library\// { bad = 1 } END { exit bad }'; then
            printf 'Unexpected non-system dynamic library dependency: %s\n' "$1" >&2
            return 1
        fi
    else
        imports="$(objdump -p "$1")"
        printf '%s\n' "$imports" | grep 'DLL Name'
        if printf '%s\n' "$imports" | grep 'DLL Name' | grep -Ei 'avcodec|avformat|avutil|swresample|libgcc|libwinpthread|libstdc'; then
            printf 'Unexpected non-system DLL dependency: %s\n' "$1" >&2
            return 1
        fi
    fi
}

if [[ -z "$destination" ]]; then
    if [[ "$binary_only" == true ]]; then
        destination="$root/bin"
    else
        destination="$root/dist/hdma2truehd-$target-$(go env GOARCH)"
    fi
fi
if [[ "$binary_only" == false && -e "$destination" ]]; then
    printf 'Refusing to overwrite release directory: %s\nChoose another output directory.\n' "$destination" >&2
    exit 1
fi

if [[ "$reuse" == true || "$binary_only" == true ]]; then
    for lib in avformat avcodec swresample avutil; do
        if [[ ! -s "$root/.backend/ffmpeg-8.0.1/lib$lib/lib$lib.a" ]]; then
            printf 'Cannot reuse backend: missing lib%s.a. Run the full platform build without --reuse-backend or --binary-only first.\n' "$lib" >&2
            exit 1
        fi
    done
    if [[ "$binary_only" == false && ! -s "$root/.backend/ffmpeg-8.0.1.tar.xz" ]]; then
        printf 'Cannot package reused backend: original FFmpeg source archive is missing.\n' >&2
        exit 1
    fi
    printf 'Reusing existing FFmpeg static libraries.\n'
else
    bash scripts/build-ffmpeg.sh
fi

if [[ "$binary_only" == true ]]; then
    mkdir -p "$destination"
    go build -a -trimpath -ldflags="-linkmode=external" \
        -o "$destination/hdma2truehd$suffix" ./cmd/hdma2truehd
    check_static_dependencies "$destination/hdma2truehd$suffix"
    printf '\nStatic-library binary built: %s/hdma2truehd%s\nTests and release packaging were not run; use the full build before distribution.\n' "$destination" "$suffix"
    exit 0
fi

if [[ "$platform" == macos ]]; then
    go test -a -race ./...
else
    go test -a ./...
fi
go vet ./...
bash scripts/build-release.sh "$destination"
bash "$destination/relink/relink.sh"
"$destination/hdma2truehd$suffix" doctor
"$destination/hdma2truehd-relinked$suffix" doctor
check_static_dependencies "$destination/hdma2truehd$suffix"
check_static_dependencies "$destination/hdma2truehd-relinked$suffix"
printf '\nBuild and re-link verification completed: %s\n' "$destination"
