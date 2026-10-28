#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
binary=""
binary_name=hdma2truehd
if [[ "${OS:-}" == Windows_NT ]]; then
    binary_name="${binary_name}.exe"
fi
overwrite=false
format=truehd
eac3_bitrate=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --binary)
            if [[ $# -lt 2 || -z "$2" || "$2" == --* ]]; then
                printf '%s\n' '--binary requires an executable path.' >&2
                exit 2
            fi
            binary="$2"
            shift 2
            ;;
        --overwrite)
            overwrite=true
            shift
            ;;
        --format|--eac3-bitrate)
            if [[ $# -lt 2 || -z "$2" || "$2" == --* ]]; then
                printf '%s requires a value.\n' "$1" >&2
                exit 2
            fi
            if [[ "$1" == --format ]]; then
                format="$2"
            else
                eac3_bitrate="$2"
            fi
            shift 2
            ;;
        --help|-h)
            printf 'Usage: bash scripts/test-samples.sh [--binary PATH] [--overwrite] [--format truehd|eac3|both] [--eac3-bitrate KBPS]\n'
            printf 'Convert samples/*.dts and samples/*.dtshd; TrueHD uses full PCM comparison, lossy E-AC-3 uses strict decode/format/length validation.\n'
            printf 'Default: find one hdma2truehd executable in dist/ or its immediate release subdirectories (.exe on Windows).\n'
            printf 'If multiple releases exist, specify --binary PATH.\n'
            printf 'Existing outputs are preserved unless --overwrite is specified.\n'
            exit 0
            ;;
        *)
            printf 'Unknown test option: %s\n' "$1" >&2
            exit 2
            ;;
    esac
done

shopt -s nullglob
if [[ -z "$binary" ]]; then
    # Limit discovery to ordinary release executables, not recursive matches
    # or the relinked test binary. Multiple releases require an explicit choice.
    candidates=()
    for candidate in "$root/dist/$binary_name" "$root"/dist/*/"$binary_name"; do
        if [[ -f "$candidate" ]]; then
            candidates+=("$candidate")
        fi
    done
    if [[ ${#candidates[@]} -eq 0 ]]; then
        printf 'No %s release executable found in %s/dist. Build a release first or specify --binary PATH.\n' "$binary_name" "$root" >&2
        exit 1
    fi
    if [[ ${#candidates[@]} -gt 1 ]]; then
        printf 'Multiple release executables found:\n' >&2
        printf '  %s\n' "${candidates[@]}" >&2
        printf 'Specify --binary PATH to select the release to test.\n' >&2
        exit 1
    fi
    binary="${candidates[0]}"
fi

if [[ ! -f "$binary" || ! -x "$binary" ]]; then
    printf 'Executable not found or not executable: %s\nBuild the program first or specify --binary PATH.\n' "$binary" >&2
    exit 1
fi

shopt -s nullglob nocaseglob
inputs=()
# Only compressed inputs at the top level are fixtures. Never feed an existing
# encoded outputs back into conversion or recurse into large source collections.
for input in "$root"/samples/*.dts "$root"/samples/*.dtshd; do
    if [[ -f "$input" ]]; then
        inputs+=("$input")
    fi
done
if [[ ${#inputs[@]} -eq 0 ]]; then
    printf 'No .dts or .dtshd sample files found in %s/samples.\n' "$root" >&2
    exit 1
fi

options=(convert --format "$format" --output-dir "$root/samples")
if [[ -n "$eac3_bitrate" ]]; then
    options+=(--eac3-bitrate "$eac3_bitrate")
fi
if [[ "$overwrite" == true ]]; then
    options+=(--overwrite)
fi
printf 'Converting %d sample(s), format=%s, with full format-appropriate verification.\n' "${#inputs[@]}" "$format"
printf 'Using executable: %s\n' "$binary"
# Do not pass --skip-verify; preserve the converter's status for automation.
exec "$binary" "${options[@]}" "${inputs[@]}"
