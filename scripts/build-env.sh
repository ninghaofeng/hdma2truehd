#!/usr/bin/env bash
# Sourced by all build entry points so FFmpeg and cgo share deployment targets.
if [[ "$(uname -s)" == Darwin ]]; then
    export MACOSX_DEPLOYMENT_TARGET=12.0
    case " ${CGO_CFLAGS:-} " in
        *" -mmacosx-version-min=12.0 "*) ;;
        *) export CGO_CFLAGS="${CGO_CFLAGS:--O2 -g} -mmacosx-version-min=12.0" ;;
    esac
    case " ${CGO_LDFLAGS:-} " in
        *" -mmacosx-version-min=12.0 "*) ;;
        *) export CGO_LDFLAGS="${CGO_LDFLAGS:--O2 -g} -mmacosx-version-min=12.0" ;;
    esac
fi

check_release_toolchain() {
    if [[ "$(go env GOOS)" == darwin ]]; then
        # A deployment flag cannot restore OS support removed from Go's runtime.
        case "$(go env GOVERSION)" in
            go1.25.*|go1.26.*) ;;
            *) printf 'macOS 12 builds require Go 1.25.x or 1.26.x (release CI uses 1.26.x); select a compatible GOTOOLCHAIN.\n' >&2; return 1 ;;
        esac
    fi
}

check_macos_deployment_target() {
    local minimum
    # Consume all load commands to avoid SIGPIPE under pipefail.
    minimum="$(otool -l "$1" | awk '/LC_BUILD_VERSION/ { build = 1 } build && $1 == "minos" { value = $2; build = 0 } /LC_VERSION_MIN_MACOSX/ { legacy = 1 } legacy && $1 == "version" { value = $2; legacy = 0 } END { if (value != "") print value }')"
    if [[ "$minimum" != 12 && "$minimum" != 12.0 && "$minimum" != 12.0.0 ]]; then
        printf 'Unexpected macOS deployment target %s in %s; rebuild FFmpeg with --clean-backend.\n' "${minimum:-missing}" "$1" >&2
        return 1
    fi
}
