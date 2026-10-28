[CmdletBinding()]
param(
    [string]$Msys2Root = $(if ($env:MSYS2_ROOT) { $env:MSYS2_ROOT } else { 'C:\msys64' }),
    [string]$OutputDir,
    [switch]$ReuseBackend,
    [switch]$BinaryOnly,
    [switch]$InstallDependencies
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

$common = (Join-Path $PSScriptRoot 'build-common.sh').Replace('\', '/')
$buildArgs = @('windows')
if ($BinaryOnly) {
    $buildArgs += '--binary-only'
}
if ($InstallDependencies) {
    $buildArgs += '--install-deps'
}
if ($ReuseBackend) {
    $buildArgs += '--reuse-backend'
}
if ($OutputDir) {
    $buildArgs += @('--output-dir', $OutputDir.Replace('\', '/'))
}

$names = @('MSYSTEM', 'MSYS2_PATH_TYPE', 'CHERE_INVOKING')
$previous = @{}
foreach ($name in $names) {
    $previous[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
try {
    $env:MSYSTEM = 'UCRT64'
    $env:MSYS2_PATH_TYPE = 'inherit'
    $env:CHERE_INVOKING = '1'
    # Pass paths as arguments, not interpolated shell commands.
    & $bash --login $common @buildArgs
    $buildExitCode = $LASTEXITCODE
}
finally {
    foreach ($name in $names) {
        [Environment]::SetEnvironmentVariable($name, $previous[$name], 'Process')
    }
}
if ($buildExitCode -ne 0) {
    throw "Windows build failed with exit code $buildExitCode."
}
