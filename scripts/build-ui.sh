#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
# Use the native macOS toolchain or matching Windows MSYS2 UCRT64/CLANGARM64
# shell, like build-release.sh. Static FFmpeg must match the bridge's required
# patched-backend marker; this entry point does not build the backend itself.
exec bash "$root/scripts/build-release.sh" "$@" --target ui
