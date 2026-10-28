# Re-linking the statically linked FFmpeg libraries

The executable statically links modified FFmpeg libraries, licensed under
LGPL 2.1 or later. See `licenses/COPYING.LGPLv2.1` and
`licenses/FFmpeg-NOTICE.md`. It still uses operating-system libraries; "single
file" means no separately installed FFmpeg executable or FFmpeg shared library.

The `relink/` directory contains the application object files (including the Go
runtime and cgo bridge), four FFmpeg static libraries, and the exact external
link command captured for this build. You may replace these FFmpeg archives
with compatible modified versions and re-link the application for your own
use. Reverse engineering for debugging such modifications is not prohibited
by this distribution.

On the **same target OS and architecture**, with a compatible C compiler and
platform SDK installed:

```sh
bash relink/relink.sh
./hdma2truehd-relinked doctor
```

On Windows, run the script from an MSYS2 UCRT64 terminal and run
`./hdma2truehd-relinked.exe doctor`. Set `CC` to the compiler executable if
necessary. Platform SDK paths in the captured command may need adjustment
for your machine. Code signing, if required by your platform, must be performed
again after re-linking.

To modify FFmpeg, extract `source/ffmpeg-8.0.1-configured.tar.gz` into a working
directory, change the relevant sources, and run `make` with the same toolchain
and configuration. Replace `relink/libavcodec.a`, `libavformat.a`,
`libavutil.a`, and `libswresample.a` with the new archives. The configured
archive includes `ffbuild/config.log`, the configure command and the patched
source. The original source archive, patch, build script and toolchain details
are also included in `source/`. The source archive is provided, not just a
download link.

For changes that alter FFmpeg's public ABI or headers, rebuild the cgo bridge
and application from the project source rather than mixing incompatible
objects. The runtime build marker check is not a digital signature: keep
the compatible marker when experimenting with a compatible library, or change
the check when rebuilding the application. No Dolby certification or hardware
compatibility warranty is implied by a successful re-link or self-test.

Do not distribute only the executable while omitting the applicable notices,
corresponding FFmpeg source and re-linking materials. The generated release
directory is intended to travel as one archive. Review the applicable license
requirements for your actual distribution and any additional dependencies.
