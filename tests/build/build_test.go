package build_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Stub compilation, not orchestration: real Bash scripts still choose targets,
// package files, replay links, reject unsafe outputs and clean icon resources.
func fixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("Bash is required for build-script tests")
	}
	root := t.TempDir()
	write := func(name, contents string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"build-common.sh", "build-release.sh", "build-ui.sh", "build-ffmpeg.sh"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		write("scripts/"+name, string(data))
	}
	for _, name := range []string{
		"patches/ffmpeg/COPYING.LGPLv2.1", "patches/ffmpeg/README.md",
		"patches/ffmpeg/0001-truehd-7.1-and-short-streams.patch",
		"patches/ffmpeg/0002-eac3-dependent-7.1.patch",
		"docs/RELINKING.md", "go.mod", "go.sum", "goroot/LICENSE", "goroot/PATENTS",
		".backend/ffmpeg-8.0.1.tar.xz", "cmd/hdma2truehd-ui/Info.plist",
		"cmd/hdma2truehd-ui/assets/app.icns", "cmd/hdma2truehd-ui/app.rc",
	} {
		write(name, "fixture")
	}
	for _, lib := range []string{"avformat", "avcodec", "swresample", "avutil"} {
		write(".backend/ffmpeg-8.0.1/lib"+lib+"/lib"+lib+".a", "fixture")
	}
	write("tools/go", `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$BUILD_LOG"
case "$1" in
    env)
        case "$2" in
            CGO_ENABLED) echo 1 ;;
            GOOS) echo "${TEST_GOOS:-darwin}" ;;
            GOARCH|GOHOSTARCH) echo amd64 ;;
            GOROOT) echo "$FIXTURE_ROOT/goroot" ;;
        esac
        ;;
    build)
        output=""
        args="$*"
        if [[ "$args" == *./cmd/hdma2truehd-ui* && "${TEST_BUILD_FAIL:-}" == ui ]]; then
            exit 23
        fi
        while [[ $# -gt 0 ]]; do
            if [[ "$1" == -o ]]; then output="$2"; shift; fi
            shift
        done
        mkdir -p "$(dirname "$output")"
        printf '#!/usr/bin/env bash\nexit 0\n' > "$output"
        chmod +x "$output"
        if [[ "$args" == *-extld=hdma2truehd-link-cc* ]]; then
            mkdir -p "$HDMA_RELINK_DIR"
            basename "$output" > "$HDMA_RELINK_DIR/application.o"
            printf '#!/usr/bin/env bash\nset -e\ncd "$(dirname "$0")"\ncp "../%s" "../%s%s"\n' \
                "$(basename "$output")" "$HDMA_RELINK_NAME" "$HDMA_EXE_SUFFIX" > "$HDMA_RELINK_DIR/relink.sh"
        fi
        ;;
    version) echo "fixture Go" ;;
    test|vet|list) ;;
    *) exit 99 ;;
esac
`)
	write("tools/cc", "#!/usr/bin/env bash\necho 'fixture compiler'\n")
	write("tools/uname", "#!/usr/bin/env bash\necho Darwin\n")
	write("tools/otool", "#!/usr/bin/env bash\nprintf '%s:\\n\\t/usr/lib/libSystem.B.dylib\\n' \"$2\"\n")
	write("tools/objdump", "#!/usr/bin/env bash\nprintf '\\tDLL Name: KERNEL32.dll\\n'\n")
	write("tools/cygpath", "#!/usr/bin/env bash\nprintf '%s\\n' \"$2\"\n")
	write("tools/windres", `#!/usr/bin/env bash
set -e
while [[ "$1" != -o ]]; do shift; done
printf 'fixture COFF' > "$2"
exit "${TEST_WINDRES_EXIT:-0}"
`)
	t.Setenv("PATH", filepath.Join(root, "tools")+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FIXTURE_ROOT", filepath.ToSlash(root))
	t.Setenv("BUILD_LOG", filepath.ToSlash(filepath.Join(root, "build.log")))
	t.Setenv("CC", "cc")
	t.Setenv("HDMA_UI", "")
	t.Setenv("TEST_GOOS", "darwin")
	t.Setenv("TEST_WINDRES_EXIT", "0")
	t.Setenv("TEST_BUILD_FAIL", "")
	return root
}

func run(t *testing.T, root, script string, success bool, args ...string) string {
	t.Helper()
	command := exec.Command("bash", append([]string{filepath.ToSlash(filepath.Join(root, "scripts", script))}, args...)...)
	output, err := command.CombinedOutput()
	if (err == nil) != success {
		t.Fatalf("%s %v: %v\n%s", script, args, err, output)
	}
	return string(output)
}

func exists(t *testing.T, root, name string, want bool) {
	t.Helper()
	_, err := os.Stat(filepath.Join(root, name))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if (err == nil) != want {
		t.Fatalf("%s: exists=%v, want %v", name, err == nil, want)
	}
}

func TestReleaseTargets(t *testing.T) {
	for _, target := range []string{"both", "cli", "ui", "legacy-ui"} {
		t.Run(target, func(t *testing.T) {
			root := fixture(t)
			destination := filepath.ToSlash(filepath.Join(root, "release with spaces"))
			args := []string{destination}
			switch target {
			case "cli", "ui":
				args = append(args, "--target", target)
			case "legacy-ui":
				t.Setenv("HDMA_UI", "1")
			}
			run(t, root, "build-release.sh", true, args...)
			cli, ui := target == "both" || target == "cli", target != "cli"
			exists(t, destination, "hdma2truehd", cli)
			exists(t, destination, "hdma2truehd-ui", ui)
			exists(t, destination, "HDMA Converter.app/Contents/Resources/app.icns", ui)
			exists(t, destination, "source/modules.txt", ui)
			exists(t, destination, "source/ffmpeg-8.0.1-configured.tar.gz", true)
			exists(t, destination, "hdma2truehd-link-cc", false)
			want := "hdma2truehd"
			if !cli {
				want = "hdma2truehd-ui"
			}
			object, err := os.ReadFile(filepath.Join(destination, "relink/application.o"))
			if err != nil || strings.TrimSpace(string(object)) != want {
				t.Fatalf("wrong primary relink object: %q, %v", object, err)
			}
			exists(t, destination, "relink-ui/relink.sh", target == "both")
			if target == "both" {
				object, err = os.ReadFile(filepath.Join(destination, "relink-ui/application.o"))
				if err != nil || strings.TrimSpace(string(object)) != "hdma2truehd-ui" {
					t.Fatalf("UI relink overwrote CLI objects: %q, %v", object, err)
				}
			}
			run(t, root, "build-release.sh", false, args...)
		})
	}
}

func TestBinaryOnlyTargets(t *testing.T) {
	for _, target := range []string{"both", "cli", "ui"} {
		t.Run(target, func(t *testing.T) {
			root := fixture(t)
			args := []string{"macos", "--binary-only"}
			if target != "both" {
				args = append(args, "--target", target)
			}
			run(t, root, "build-common.sh", true, args...)
			// A second development build must replace, not reject, its output.
			run(t, root, "build-common.sh", true, args...)
			exists(t, root, "bin/hdma2truehd", target != "ui")
			exists(t, root, "bin/hdma2truehd-ui", target != "cli")
			exists(t, root, "bin/HDMA Converter.app/Contents/Info.plist", target != "cli")
			exists(t, root, "bin/relink", false)
			exists(t, root, "bin/source", false)
			log, err := os.ReadFile(filepath.Join(root, "build.log"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(log), "test ") || strings.Contains(string(log), "list ") ||
				strings.Contains(string(log), "./tools/link-wrapper") {
				t.Fatalf("binary-only ran release steps:\n%s", log)
			}
		})
	}
}

func TestWindowsResourceCleanup(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			root := fixture(t)
			t.Setenv("TEST_GOOS", "windows")
			if fail {
				t.Setenv("TEST_WINDRES_EXIT", "17")
			}
			run(t, root, "build-release.sh", !fail, "--binary-only")
			exists(t, root, "cmd/hdma2truehd-ui/resource_windows_amd64.syso", false)
			exists(t, root, "bin/hdma2truehd.exe", !fail)
			exists(t, root, "bin/hdma2truehd-ui.exe", !fail)
			// An unrelated/preexisting resource object must never be replaced
			// or removed, including when a previous build was interrupted.
			resource := filepath.Join(root, "cmd/hdma2truehd-ui/resource_windows_amd64.syso")
			if err := os.WriteFile(resource, []byte("preserve"), 0600); err != nil {
				t.Fatal(err)
			}
			run(t, root, "build-release.sh", false, "--binary-only")
			data, err := os.ReadFile(resource)
			if err != nil || string(data) != "preserve" {
				t.Fatalf("preexisting resource changed: %q, %v", data, err)
			}
		})
	}
}

func TestPlatformReleaseBoth(t *testing.T) {
	root := fixture(t)
	run(t, root, "build-common.sh", true, "macos", "--reuse-backend")
	release := "dist/hdma2truehd-darwin-amd64"
	for _, name := range []string{
		"hdma2truehd", "hdma2truehd-ui", "hdma2truehd-relinked", "hdma2truehd-ui-relinked",
		"relink/relink.sh", "relink-ui/relink.sh", "HDMA Converter.app/Contents/MacOS/hdma2truehd-ui",
	} {
		exists(t, root, release+"/"+name, true)
	}
	log, err := os.ReadFile(filepath.Join(root, "build.log"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(log), "test -a -race ./...") != 1 ||
		!strings.Contains(string(log), "vet -tags desktop,production ./cmd/hdma2truehd-ui") {
		t.Fatalf("missing shared test or UI validation:\n%s", log)
	}
	run(t, root, "build-common.sh", false, "macos", "--reuse-backend")
	run(t, root, "build-common.sh", false, "macos", "--reuse-backend", "--clean", "--output-dir", filepath.ToSlash(filepath.Join(root, "dist")))
	exists(t, root, release+"/hdma2truehd-ui", true)
	run(t, root, "build-common.sh", true, "macos", "--reuse-backend", "--clean")
	exists(t, root, release+"/relink-ui/relink.sh", true)
}

func TestFailedUIBuild(t *testing.T) {
	root := fixture(t)
	t.Setenv("TEST_GOOS", "windows")
	t.Setenv("TEST_BUILD_FAIL", "ui")
	destination := filepath.ToSlash(filepath.Join(root, "failed-release"))
	output := run(t, root, "build-release.sh", false, destination)
	exists(t, root, "cmd/hdma2truehd-ui/resource_windows_amd64.syso", false)
	exists(t, destination, "hdma2truehd-link-cc.exe", false)
	exists(t, destination, "source", false)
	if strings.Contains(output, "Release and re-linkable objects") {
		t.Fatalf("failed UI build reported success: %s", output)
	}
}

func TestBuildArguments(t *testing.T) {
	root := fixture(t)
	for _, script := range []string{"build-common.sh", "build-release.sh"} {
		for _, invalid := range [][]string{{"--target"}, {"--target", "invalid"}} {
			args := append([]string{}, invalid...)
			if script == "build-common.sh" {
				args = append([]string{"macos"}, args...)
			}
			run(t, root, script, false, args...)
		}
	}
	run(t, root, "build-common.sh", false, "macos", "--binary-only", "--clean")
	run(t, root, "build-common.sh", false, "macos", "--clean-backend", "--reuse-backend")
	run(t, root, "build-common.sh", false, "macos", "--clean-backend", "--binary-only")
	exists(t, root, ".backend/ffmpeg-8.0.1/libavcodec/libavcodec.a", true)
	exists(t, root, "dist", false)
	exists(t, root, "bin", false)
	run(t, root, "build-ui.sh", true)
	exists(t, root, "dist/hdma2truehd-ui-darwin-amd64/hdma2truehd-ui", true)
	exists(t, root, "dist/hdma2truehd-ui-darwin-amd64/hdma2truehd", false)
}

func stubBackendRebuild(t *testing.T, root string) {
	t.Helper()
	t.Setenv("TEST_BACKEND_EXIT", "0")
	script := `#!/usr/bin/env bash
set -euo pipefail
test ! -e .backend/ffmpeg-8.0.1
printf 'rebuild\n' >> backend.log
if [[ "$TEST_BACKEND_EXIT" != 0 ]]; then exit "$TEST_BACKEND_EXIT"; fi
for lib in avformat avcodec swresample avutil; do
    mkdir -p ".backend/ffmpeg-8.0.1/lib$lib"
    printf 'rebuilt' > ".backend/ffmpeg-8.0.1/lib$lib/lib$lib.a"
done
`
	if err := os.WriteFile(filepath.Join(root, "scripts/build-ffmpeg.sh"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
}

func TestCleanBackend(t *testing.T) {
	for _, platform := range []string{"macos", "windows"} {
		t.Run(platform, func(t *testing.T) {
			root := fixture(t)
			stubBackendRebuild(t, root)
			if platform == "windows" {
				t.Setenv("OS", "Windows_NT")
				t.Setenv("MSYSTEM", "UCRT64")
				t.Setenv("TEST_GOOS", "windows")
			}
			run(t, root, "build-common.sh", true, platform, "--reuse-backend")
			release := "dist/hdma2truehd-darwin-amd64"
			suffix := ""
			if platform == "windows" {
				release = "dist/hdma2truehd-windows-amd64"
				suffix = ".exe"
			}
			for _, name := range []string{
				".backend/ffmpeg-8.0.1/local-changes",
				".backend/ffmpeg-8.0.1-backup/keep",
				"samples/keep", "dist/other-release/keep", release + "/stale",
			} {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			// A conflicting output must be rejected before deleting the backend.
			run(t, root, "build-common.sh", false, platform, "--clean-backend")
			exists(t, root, ".backend/ffmpeg-8.0.1/local-changes", true)
			exists(t, root, "backend.log", false)
			run(t, root, "build-common.sh", true, platform, "--clean-backend", "--clean")
			exists(t, root, ".backend/ffmpeg-8.0.1/local-changes", false)
			exists(t, root, release+"/stale", false)
			for _, name := range []string{".backend/ffmpeg-8.0.1-backup/keep", "samples/keep", "dist/other-release/keep"} {
				exists(t, root, name, true)
			}
			archive, err := os.ReadFile(filepath.Join(root, ".backend/ffmpeg-8.0.1.tar.xz"))
			if err != nil || string(archive) != "fixture" {
				t.Fatalf("source archive changed: %q, %v", archive, err)
			}
			library, err := os.ReadFile(filepath.Join(root, ".backend/ffmpeg-8.0.1/libavcodec/libavcodec.a"))
			if err != nil || string(library) != "rebuilt" {
				t.Fatalf("backend was not rebuilt: %q, %v", library, err)
			}
			exists(t, root, release+"/hdma2truehd"+suffix, true)
			exists(t, root, release+"/hdma2truehd-ui"+suffix, true)
			exists(t, root, release+"/relink-ui/relink.sh", true)
		})
	}
}

func TestCleanBackendUnsafePaths(t *testing.T) {
	for _, name := range []string{"parent-link", "source-link", "broken-link", "parent-file", "source-file"} {
		t.Run(name, func(t *testing.T) {
			root := fixture(t)
			stubBackendRebuild(t, root)
			path := filepath.Join(root, ".backend")
			if !strings.HasPrefix(name, "parent") {
				path = filepath.Join(path, "ffmpeg-8.0.1")
			}
			saved := filepath.Join(root, "preserved-backend")
			if err := os.Rename(path, saved); err != nil {
				t.Fatal(err)
			}
			if strings.HasSuffix(name, "file") {
				if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				target := saved
				if name == "broken-link" {
					target = filepath.Join(root, "missing")
				}
				if err := os.Symlink(target, path); err != nil {
					t.Skipf("symlink creation unavailable: %v", err)
				}
			}
			run(t, root, "build-common.sh", false, "macos", "--clean-backend")
			exists(t, root, "preserved-backend", true)
			exists(t, root, "backend.log", false)
			if _, err := os.Lstat(path); err != nil {
				t.Fatalf("unsafe backend path was removed: %v", err)
			}
		})
	}
}

func TestCleanBackendMissingOrFailed(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "rebuild-failed"}[fail], func(t *testing.T) {
			root := fixture(t)
			stubBackendRebuild(t, root)
			if fail {
				t.Setenv("TEST_BACKEND_EXIT", "17")
				// The old release must survive a failed backend rebuild.
				run(t, root, "build-common.sh", true, "macos", "--reuse-backend")
			} else if err := os.Rename(filepath.Join(root, ".backend/ffmpeg-8.0.1"),
				filepath.Join(root, ".backend/ffmpeg-8.0.1-backup")); err != nil {
				t.Fatal(err)
			}
			run(t, root, "build-common.sh", !fail, "macos", "--clean-backend", "--clean")
			exists(t, root, ".backend/ffmpeg-8.0.1", !fail)
			exists(t, root, "dist/hdma2truehd-darwin-amd64/hdma2truehd", true)
			exists(t, root, ".backend/ffmpeg-8.0.1.tar.xz", true)
		})
	}
}

func TestFFmpegArchiveReuse(t *testing.T) {
	for _, mode := range []string{"cached", "missing", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			root := fixture(t)
			backend := filepath.Join(root, ".backend")
			if err := os.Rename(filepath.Join(backend, "ffmpeg-8.0.1"),
				filepath.Join(backend, "ffmpeg-8.0.1-backup")); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(backend, "ffmpeg-8.0.1.tar.xz")
			if mode == "missing" {
				if err := os.Rename(archive, filepath.Join(backend, "saved-archive")); err != nil {
					t.Fatal(err)
				}
			}
			hash := "05ee0b03119b45c0bdb4df654b96802e909e0a752f72e4fe3794f487229e5a41"
			if mode == "corrupt" {
				hash = "incorrect"
			}
			t.Setenv("TEST_ARCHIVE_HASH", hash)
			for name, script := range map[string]string{
				"sha256sum": `#!/usr/bin/env bash
printf '%s  %s\n' "$TEST_ARCHIVE_HASH" "$1"
`,
				"curl": `#!/usr/bin/env bash
set -euo pipefail
printf 'download\n' >> "$FIXTURE_ROOT/download.log"
while [[ "$1" != --output ]]; do shift; done
printf 'downloaded' > "$2"
`,
				"tar": `#!/usr/bin/env bash
printf 'extract\n' >> "$FIXTURE_ROOT/extract.log"
exit 37
`,
			} {
				if err := os.WriteFile(filepath.Join(root, "tools", name), []byte(script), 0755); err != nil {
					t.Fatal(err)
				}
			}
			// Stop at extraction: this tests download/hash ordering without
			// needing network access or compiling another copy of FFmpeg.
			output := run(t, root, "build-ffmpeg.sh", false, filepath.ToSlash(backend))
			exists(t, root, "download.log", mode == "missing")
			exists(t, root, "extract.log", mode != "corrupt")
			if mode == "corrupt" && !strings.Contains(output, "checksum mismatch") {
				t.Fatalf("corrupt cache was not explicitly rejected: %s", output)
			}
			data, err := os.ReadFile(archive)
			want := "fixture"
			if mode == "missing" {
				want = "downloaded"
			}
			if err != nil || string(data) != want {
				t.Fatalf("archive contents %q, want %q: %v", data, want, err)
			}
		})
	}
}
