[CmdletBinding()]
param(
    [ValidateNotNullOrEmpty()]
    [string]$Binary,
    [switch]$Overwrite,
    [ValidateSet('truehd', 'eac3', 'both')]
    [string]$Format = 'truehd',
    [int]$Eac3Bitrate = 0
)

$ErrorActionPreference = 'Stop'
# Preserve the CLI's exit code explicitly rather than converting a nonzero
# native status into a PowerShell exception under newer PowerShell versions.
$PSNativeCommandUseErrorActionPreference = $false

$root = Split-Path -Parent $PSScriptRoot
if (-not $PSBoundParameters.ContainsKey('Binary')) {
    # Match the Bash script: only ordinary binaries in dist or one level below
    # it are candidates. Never silently pick a release by modification time.
    $dist = Join-Path $root 'dist'
    if (-not (Test-Path -LiteralPath $dist -PathType Container)) {
        throw "Release directory not found: $dist. Build a release first or specify -Binary PATH."
    }
    $candidates = @(
        Get-ChildItem -LiteralPath $dist -File -Filter 'hdma2truehd.exe'
        Get-ChildItem -LiteralPath $dist -Directory |
            ForEach-Object { Get-ChildItem -LiteralPath $_.FullName -File -Filter 'hdma2truehd.exe' }
    )
    if ($candidates.Count -eq 0) {
        throw "No hdma2truehd.exe release executable found in $dist. Build a release first or specify -Binary PATH."
    }
    if ($candidates.Count -gt 1) {
        $paths = ($candidates | ForEach-Object { "  $($_.FullName)" }) -join [Environment]::NewLine
        throw "Multiple release executables found:`n$paths`nSpecify -Binary PATH to select the release to test."
    }
    $Binary = $candidates[0].FullName
}

if (-not (Test-Path -LiteralPath $Binary -PathType Leaf)) {
    throw "Executable not found: $Binary. Build the program first or specify -Binary PATH."
}
$Binary = (Resolve-Path -LiteralPath $Binary).ProviderPath

$samples = Join-Path $root 'samples'
if (-not (Test-Path -LiteralPath $samples -PathType Container)) {
    throw "Samples directory not found: $samples."
}
$inputs = @(
    # Array syntax preserves Count for zero/one input; sorting stabilizes logs.
    Get-ChildItem -LiteralPath $samples -File |
        Where-Object { $_.Extension -in @('.dts', '.dtshd') } |
        Sort-Object Name |
        ForEach-Object { $_.FullName }
)
if ($inputs.Count -eq 0) {
    throw "No .dts or .dtshd sample files found in $samples."
}

$convertArgs = @('convert', '--format', $Format.ToLowerInvariant(), '--output-dir', $samples)
# Verification is intentionally not configurable here; every sample must pass
# full PCM comparison (TrueHD) or lossy decode/format/length validation (E-AC-3).
if ($Eac3Bitrate -ne 0) {
    $convertArgs += @('--eac3-bitrate', $Eac3Bitrate.ToString())
}
if ($Overwrite) {
    $convertArgs += '--overwrite'
}
$convertArgs += $inputs

Write-Host ("Converting {0} sample(s), format={1}, with full format-appropriate verification." -f $inputs.Count, $Format)
Write-Host "Using executable: $Binary"
& $Binary @convertArgs
exit $LASTEXITCODE
