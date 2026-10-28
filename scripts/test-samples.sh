#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
binary="$root/bin/hdma2truehd"
if [[ "${OS:-}" == Windows_NT ]]; then
    binary="${binary}.exe"
fi
overwrite=false

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
        --help|-h)
            printf 'Usage: bash scripts/test-samples.sh [--binary PATH] [--overwrite]\n'
            printf 'Convert samples/*.dts and samples/*.dtshd to .thd with full PCM verification.\n'
            printf 'Default executable: bin/hdma2truehd (.exe on Windows).\n'
            printf 'Existing outputs are preserved unless --overwrite is specified.\n'
            exit 0
            ;;
        *)
            printf 'Unknown test option: %s\n' "$1" >&2
            exit 2
            ;;
    esac
done

if [[ ! -f "$binary" || ! -x "$binary" ]]; then
    printf 'Executable not found or not executable: %s\nBuild the program first or specify --binary PATH.\n' "$binary" >&2
    exit 1
fi

shopt -s nullglob nocaseglob
inputs=()
for input in "$root"/samples/*.dts "$root"/samples/*.dtshd; do
    if [[ -f "$input" ]]; then
        inputs+=("$input")
    fi
done
if [[ ${#inputs[@]} -eq 0 ]]; then
    printf 'No .dts or .dtshd sample files found in %s/samples.\n' "$root" >&2
    exit 1
fi

options=(convert --output-dir "$root/samples")
if [[ "$overwrite" == true ]]; then
    options+=(--overwrite)
fi
printf 'Converting %d sample(s) with full PCM verification.\n' "${#inputs[@]}"
exec "$binary" "${options[@]}" "${inputs[@]}"
