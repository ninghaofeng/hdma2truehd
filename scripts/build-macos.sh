#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
# Keep this entry point thin so release checks cannot drift between platforms.
exec bash "$root/scripts/build-common.sh" macos "$@"
