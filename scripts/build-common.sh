#!/usr/bin/env bash
set -euo pipefail

# Both platform entry points use this orchestration; relative output paths are
# resolved from the repository, not from the caller's working directory.
root="$(cd "$(dirname "$0")/.." && pwd -P)"
cd "$root"
platform="${1:?Pass macos or windows}"
shift
reuse=false
binary_only=false
install_deps=false
clean=false
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
        --install-deps)
            install_deps=true
            shift
            ;;
        --clean)
            clean=true
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
            printf 'Usage: build-macos.sh [--binary-only] [--reuse-backend] [--clean] [--output-dir DIR]\n'
            printf 'Windows: build-windows.ps1 [-Msys2Root PATH] [-BinaryOnly] [-ReuseBackend] [-InstallDependencies] [-Clean] [-OutputDir DIR]\n'
            printf 'Default: build libraries, test and package a complete release in dist/.\n'
            printf 'Binary-only: reuse existing static libraries and rebuild bin/hdma2truehd (no tests or release materials).\n'
            printf 'Windows --install-deps: install MSYS2 prerequisites with pacman --needed, then build (confirmation required).\n'
            printf 'Clean: remove only the selected release directory directly inside project dist/ before building; static libraries are preserved.\n'
            exit 0
            ;;
        *)
            printf 'Unknown build option: %s\n' "$1" >&2
            exit 2
            ;;
    esac
done

if [[ "$clean" == true && "$binary_only" == true ]]; then
    printf '%s\n' '--clean is for full releases; --binary-only already replaces the executable without removing its output directory.' >&2
    exit 2
fi

case "$platform" in
    macos)
        if [[ "$install_deps" == true ]]; then
            # Installation is opt-in. Do not upgrade the whole MSYS2 system or install
            # Go here; keep pacman's confirmation and recheck actual tools afterwards.
            printf '%s\n' '--install-deps is only supported for Windows MSYS2 UCRT64 builds.' >&2
            exit 2
        fi
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
if [[ "$install_deps" == true ]]; then
    if ! command -v pacman >/dev/null 2>&1; then
        printf 'MSYS2 pacman not found. Check -Msys2Root points to a complete MSYS2 installation.\n' >&2
        exit 1
    fi
    packages=(mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-binutils)
    if [[ "$binary_only" == false ]]; then
        packages=(make diffutils patch curl tar xz "${packages[@]}")
    fi
    printf 'Installing MSYS2 build prerequisites; pacman will request confirmation if needed.\n'
    pacman -S --needed "${packages[@]}"
fi
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
        if [[ "$platform" == windows ]]; then
            printf 'Re-run build-windows.ps1 with -InstallDependencies, keeping your other build options.\nGo must be installed separately and available on PATH.\n' >&2
        fi
        exit 1
    fi
done
export CGO_ENABLED=1
# Go and the C toolchain must target the same native platform/architecture.
# A pure-Go cross build cannot substitute for matching FFmpeg static archives.
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
    # "Single executable" still permits OS libraries/frameworks, but never a
    # runtime dependency on separately installed FFmpeg or GCC support DLLs.
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
if [[ "$clean" == true ]]; then
    # Constrain deletion to one real, direct child of the physical dist path.
    # Resolve the parent separately so "."/".." and symlink aliases cannot
    # broaden the cleanup target to dist itself or another project directory.
    if [[ -L "$root/dist" ]]; then
        printf 'Refusing to clean through a symlinked dist directory.\n' >&2
        exit 1
    fi
    mkdir -p "$root/dist"
    release_root="$(cd "$root/dist" && pwd -P)"
    output_parent="$(dirname "$destination")"
    output_name="$(basename "$destination")"
    if [[ "$output_name" == "." || "$output_name" == ".." ||
          ! -d "$output_parent" || "$(cd "$output_parent" && pwd -P)" != "$release_root" ]]; then
        printf 'Refusing to clean %s: choose a release directory directly inside %s.\n' "$destination" "$release_root" >&2
        exit 1
    fi
    destination="$release_root/$output_name"
    if [[ -L "$destination" || ( -e "$destination" && ! -d "$destination" ) ]]; then
        printf 'Refusing to clean a symlink or non-directory output: %s\n' "$destination" >&2
        exit 1
    fi
elif [[ "$binary_only" == false && ( -e "$destination" || -L "$destination" ) ]]; then
    printf 'Refusing to overwrite release directory: %s\nChoose another output directory, or use --clean (-Clean on Windows) for a release directly inside dist/.\n' "$destination" >&2
    exit 1
fi

if [[ "$reuse" == true || "$binary_only" == true ]]; then
    # A local quick build needs only archives. A redistributable release also
    # needs the original source archive for the bundled licensing materials.
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
    # External .a replacements are not ordinary Go source dependencies.
    # Force rebuilding/relinking rather than trusting a previously cached link.
    go build -a -trimpath -ldflags="-linkmode=external" \
        -o "$destination/hdma2truehd$suffix" ./cmd/hdma2truehd
    check_static_dependencies "$destination/hdma2truehd$suffix"
    printf '\nStatic-library binary built: %s/hdma2truehd%s\nTests and release packaging were not run; use the full build before distribution.\n' "$destination" "$suffix"
    exit 0
fi

if [[ "$clean" == true && -d "$destination" ]]; then
    # Tool/backend checks completed before removing the old release. A failure
    # after this point cannot restore it; --clean is deliberately explicit.
    printf 'Cleaning release directory: %s\n' "$destination"
    rm -r -- "$destination"
fi

if [[ "$platform" == macos ]]; then
    go test -a -race ./...
else
    go test -a ./...
fi
go vet ./...
# A release is checked twice: the originally linked program and the binary
# rebuilt solely from the captured objects/libraries must both pass doctor.
bash scripts/build-release.sh "$destination"
bash "$destination/relink/relink.sh"
"$destination/hdma2truehd$suffix" doctor
"$destination/hdma2truehd-relinked$suffix" doctor
check_static_dependencies "$destination/hdma2truehd$suffix"
check_static_dependencies "$destination/hdma2truehd-relinked$suffix"
printf '\nBuild and re-link verification completed: %s\n' "$destination"
