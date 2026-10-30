#!/usr/bin/env bash
set -euo pipefail

# Both platform entry points use this orchestration; relative output paths are
# resolved from the repository, not from the caller's working directory.
root="$(cd "$(dirname "$0")/.." && pwd -P)"
cd "$root"
source "$root/scripts/build-env.sh"
platform="${1:?Pass macos or windows}"
shift
reuse=false
binary_only=false
install_deps=false
clean=false
clean_backend=false
destination=""
build_target=both
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
        --clean-backend)
            clean_backend=true
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
        --target)
            if [[ $# -lt 2 ]]; then
                printf '%s\n' '--target requires both, cli or ui.' >&2
                exit 2
            fi
            build_target="$2"
            shift 2
            ;;
        --help|-h)
            printf 'Usage: build-macos.sh [--target both|cli|ui] [--binary-only] [--reuse-backend|--clean-backend] [--clean] [--output-dir DIR]\n'
            printf 'Windows: build-windows.ps1 [-Target both|cli|ui] [-Msys2Root PATH] [-BinaryOnly] [-ReuseBackend|-CleanBackend] [-InstallDependencies] [-Clean] [-OutputDir DIR]\n'
            printf 'Default: build libraries, test and package CLI and UI together in dist/.\n'
            printf 'Binary-only: reuse existing static libraries and rebuild both applications in bin/ (no tests or release materials).\n'
            printf 'Windows --install-deps: install MSYS2 prerequisites with pacman --needed, then build (confirmation required).\n'
            printf 'Clean: remove only the selected release directory directly inside project dist/ before building; static libraries are preserved.\n'
            printf 'Clean-backend: delete only .backend/ffmpeg-8.0.1/ (including local source changes) and rebuild; keep the source archive and other backups. Cannot combine with reuse-backend or binary-only.\n'
            exit 0
            ;;
        *)
            printf 'Unknown build option: %s\n' "$1" >&2
            exit 2
            ;;
    esac
done

case "$build_target" in
    both) apps=(hdma2truehd hdma2truehd-ui) ;;
    cli) apps=(hdma2truehd) ;;
    ui) apps=(hdma2truehd-ui) ;;
    *) printf 'Invalid build target: %s (choose both, cli or ui).\n' "$build_target" >&2; exit 2 ;;
esac
# Reject contradictory modes before tool installation or filesystem cleanup.
if [[ "$clean" == true && "$binary_only" == true ]]; then
    printf '%s\n' '--clean is for full releases; --binary-only already replaces the executable without removing its output directory.' >&2
    exit 2
fi
if [[ "$clean_backend" == true && ( "$reuse" == true || "$binary_only" == true ) ]]; then
    printf '%s\n' '--clean-backend rebuilds FFmpeg and cannot be combined with --reuse-backend or --binary-only.' >&2
    exit 2
fi

case "$platform" in
    macos)
        if [[ "$install_deps" == true ]]; then
            printf '%s\n' '--install-deps is only supported for Windows MSYS2 builds.' >&2
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
        if [[ "${OS:-}" != Windows_NT || ( "${MSYSTEM:-}" != UCRT64 && "${MSYSTEM:-}" != CLANGARM64 ) ]]; then
            printf 'Windows requires MSYS2 UCRT64 (x64) or CLANGARM64 (ARM64). Use scripts/build-windows.ps1.\n' >&2
            exit 1
        fi
        target=windows
        if [[ "$MSYSTEM" == CLANGARM64 ]]; then
            export CC="${CC:-clang}"
        else
            export CC="${CC:-gcc}"
        fi
        ;;
    *)
        printf 'Unsupported build platform: %s\n' "$platform" >&2
        exit 2
        ;;
esac
if [[ "$install_deps" == true ]]; then
    # Installation is opt-in. Keep pacman's confirmation, avoid a system
    # upgrade, and recheck tools afterwards; Go/MSYS2 themselves are external.
    if ! command -v pacman >/dev/null 2>&1; then
        printf 'MSYS2 pacman not found. Check -Msys2Root points to a complete MSYS2 installation.\n' >&2
        exit 1
    fi
    packages=(mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-binutils)
    if [[ "$MSYSTEM" == CLANGARM64 ]]; then
        packages=(mingw-w64-clang-aarch64-clang mingw-w64-clang-aarch64-llvm-tools mingw-w64-clang-aarch64-lld mingw-w64-clang-aarch64-compiler-rt)
    fi
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
    tools+=(windres)
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
check_release_toolchain
export CGO_ENABLED=1
# Go and the C toolchain must target the same native platform/architecture.
# A pure-Go cross build cannot substitute for matching FFmpeg static archives.
if [[ "$(go env GOOS)" != "$target" || "$(go env GOARCH)" != "$(go env GOHOSTARCH)" ]]; then
    printf 'A native build is required; unset cross-compilation GOOS/GOARCH overrides.\n' >&2
    exit 1
fi
if [[ "$platform" == windows ]]; then
    machine="$("$CC" -dumpmachine)"
    case "$MSYSTEM:$(go env GOARCH):$machine" in
        UCRT64:amd64:x86_64-*) ;;
        CLANGARM64:arm64:aarch64-*) ;;
        *) printf 'Go architecture, MSYS2 environment and C compiler disagree: %s / %s / %s.\n' "$(go env GOARCH)" "$MSYSTEM" "$machine" >&2; exit 1 ;;
    esac
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
        # Inspect the linked result too: cached FFmpeg objects may have been
        # built for a newer OS even when this invocation targets macOS 12.
        check_macos_deployment_target "$1"
        if ! printf '%s\n' "$imports" | awk 'NR > 1 && $1 !~ /^\/usr\/lib\// && $1 !~ /^\/System\/Library\// { bad = 1 } END { exit bad }'; then
            printf 'Unexpected non-system dynamic library dependency: %s\n' "$1" >&2
            return 1
        fi
    else
        imports="$(objdump -p "$1")"
        printf '%s\n' "$imports" | grep 'DLL Name'
        if printf '%s\n' "$imports" | grep 'DLL Name' | grep -Ei 'avcodec|avformat|avutil|swresample|libgcc|libwinpthread|libstdc|libc\+\+|libunwind|libclang_rt'; then
            printf 'Unexpected non-system DLL dependency: %s\n' "$1" >&2
            return 1
        fi
    fi
}

if [[ -z "$destination" ]]; then
    if [[ "$binary_only" == true ]]; then
        destination="$root/bin"
    else
        release_name=hdma2truehd
        if [[ "$build_target" == ui ]]; then release_name=hdma2truehd-ui; fi
        destination="$root/dist/$release_name-$target-$(go env GOARCH)"
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
    # Rebuild the backend before deleting an old release: a failed rebuild may
    # discard the old backend, but must leave the published release available.
    if [[ "$clean_backend" == true ]]; then
        backend_root="$root/.backend"
        backend_source="$backend_root/ffmpeg-8.0.1"
        # Never follow an aliased parent or source directory when deleting.
        # Only this fixed, versioned tree is disposable; backups/cache remain.
        for directory in "$backend_root" "$backend_source"; do
            if [[ -L "$directory" || ( -e "$directory" && ! -d "$directory" ) ]]; then
                printf 'Refusing to clean a symlink or non-directory backend path: %s\n' "$directory" >&2
                exit 1
            fi
            if [[ -d "$directory" && "$(cd "$directory" && pwd -P)" != "$directory" ]]; then
                printf 'Refusing to clean an aliased backend path: %s\n' "$directory" >&2
                exit 1
            fi
        done
        if [[ -d "$backend_source" ]]; then
            printf 'Cleaning FFmpeg source and static libraries (local changes will be lost): %s\n' "$backend_source"
            rm -r -- "$backend_source"
        fi
    fi
    bash scripts/build-ffmpeg.sh
fi

if [[ "$binary_only" == true ]]; then
    bash scripts/build-release.sh --binary-only --target "$build_target" "$destination"
    for app in "${apps[@]}"; do
        check_static_dependencies "$destination/$app$suffix"
    done
    printf '\nStatic-library applications built (%s): %s\nTests and release packaging were not run; use the full build before distribution.\n' "$build_target" "$destination"
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
if [[ "$build_target" != cli ]]; then
    go vet -tags desktop,production ./cmd/hdma2truehd-ui
fi
bash scripts/build-release.sh --target "$build_target" "$destination"
# A GUI launch would block on an interactive desktop. Verify both native links
# and dependency sets, but invoke doctor only on the command-line application.
for app in "${apps[@]}"; do
    relink=relink
    if [[ "$build_target" == both && "$app" == hdma2truehd-ui ]]; then relink=relink-ui; fi
    bash "$destination/$relink/relink.sh"
    if [[ "$app" == hdma2truehd ]]; then
        "$destination/$app$suffix" doctor
        "$destination/$app-relinked$suffix" doctor
    fi
    check_static_dependencies "$destination/$app$suffix"
    check_static_dependencies "$destination/$app-relinked$suffix"
done
# This release directory was freshly created by build-release.sh. Remove only
# diagnostics generated by its doctor checks, not arbitrary directory contents.
if [[ -L "$destination/logs" || ( -e "$destination/logs" && ! -d "$destination/logs" ) ]]; then
    printf 'Refusing to clean unexpected diagnostic path: %s/logs\n' "$destination" >&2
    exit 1
fi
if [[ -d "$destination/logs" ]]; then
    for log in "$destination"/logs/hdma2truehd-*.jsonl "$destination"/logs/hdma2truehd-*.jsonl.1; do
        if [[ ! -e "$log" && ! -L "$log" ]]; then continue; fi
        if [[ -L "$log" || ! -f "$log" ]]; then
            printf 'Refusing to clean unexpected diagnostic entry: %s\n' "$log" >&2
            exit 1
        fi
        rm -- "$log"
    done
    # Fail rather than deleting unrelated files if anything unexpected remains.
    rmdir -- "$destination/logs"
fi
printf '\nBuild and re-link verification completed: %s\n' "$destination"
