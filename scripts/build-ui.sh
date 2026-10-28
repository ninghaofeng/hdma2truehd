#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
# Run inside the native macOS toolchain or Windows MSYS2 UCRT64 shell, exactly
# like build-release.sh. Static FFmpeg must already have the v3 build marker.
exec bash "$root/scripts/build-release.sh" "$@" --target ui
