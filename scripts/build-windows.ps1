[CmdletBinding()]
param(
    [string]$Msys2Root = $(if ($env:MSYS2_ROOT) { $env:MSYS2_ROOT } else { 'C:\msys64' }),
    [string]$OutputDir,
    [ValidateSet('both', 'cli', 'ui')]
    [string]$Target = 'both',
    [switch]$ReuseBackend,
    [switch]$CleanBackend,
    [switch]$BinaryOnly,
    [switch]$InstallDependencies,
    [switch]$Clean
)

$ErrorActionPreference = 'Stop'
if ($env:OS -ne 'Windows_NT') {
    throw 'This script requires Windows. On macOS, run bash scripts/build-macos.sh.'
}

$bash = Join-Path $Msys2Root 'usr\bin\bash.exe'
if (-not (Test-Path -LiteralPath $bash -PathType Leaf)) {
    throw "MSYS2 bash not found: $bash. Install MSYS2 or specify -Msys2Root."
}
if (-not (Get-Command go.exe -ErrorAction SilentlyContinue)) {
    throw 'Go is not on PATH. Install Go 1.24 or newer before building.'
}
$goArch = (& go.exe env GOARCH).Trim()
if ($LASTEXITCODE -ne 0) { throw 'Cannot determine Go architecture.' }
switch ($goArch) {
    'amd64' { $msystem = 'UCRT64' }
    'arm64' { $msystem = 'CLANGARM64' }
    default { throw "Unsupported Windows Go architecture: $goArch. Use native x64 or ARM64 Go." }
}

$common = (Join-Path $PSScriptRoot 'build-common.sh').Replace('\', '/')
# Translate switches into arguments for the shared orchestration; do not
# duplicate dependency installation, cleanup or release logic in PowerShell.
$buildArgs = @('windows', '--target', $Target.ToLowerInvariant())
if ($BinaryOnly) {
    $buildArgs += '--binary-only'
}
if ($InstallDependencies) {
    $buildArgs += '--install-deps'
}
if ($ReuseBackend) {
    $buildArgs += '--reuse-backend'
}
if ($CleanBackend) {
    # Keep backend deletion separate from release cleanup. The common script
    # validates the fixed deletion path and rejects conflicting reuse modes.
    $buildArgs += '--clean-backend'
}
if ($Clean) {
    $buildArgs += '--clean'
}
if ($OutputDir) {
    $buildArgs += @('--output-dir', $OutputDir.Replace('\', '/'))
}

$names = @('MSYSTEM', 'MSYS2_PATH_TYPE', 'CHERE_INVOKING')
# Scope the MSYS2 environment to this invocation. Inheriting PATH makes the
# user's native Go visible, while MSYSTEM selects its matching C toolchain.
$previous = @{}
foreach ($name in $names) {
    $previous[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
try {
    $env:MSYSTEM = $msystem
    $env:MSYS2_PATH_TYPE = 'inherit'
    $env:CHERE_INVOKING = '1'
    # Pass paths as arguments, not interpolated shell commands.
    & $bash --login $common @buildArgs
    $buildExitCode = $LASTEXITCODE
}
finally {
    # Restore even after an invocation failure so the caller's shell is not
    # left configured for MSYS2 by a failed build.
    foreach ($name in $names) {
        [Environment]::SetEnvironmentVariable($name, $previous[$name], 'Process')
    }
}
if ($buildExitCode -ne 0) {
    throw "Windows build failed with exit code $buildExitCode."
}
