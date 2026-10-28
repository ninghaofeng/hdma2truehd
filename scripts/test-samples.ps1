[CmdletBinding()]
param(
    [ValidateNotNullOrEmpty()]
    [string]$Binary,
    [switch]$Overwrite
)

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false

$root = Split-Path -Parent $PSScriptRoot
if (-not $PSBoundParameters.ContainsKey('Binary')) {
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
    Get-ChildItem -LiteralPath $samples -File |
        Where-Object { $_.Extension -in @('.dts', '.dtshd') } |
        Sort-Object Name |
        ForEach-Object { $_.FullName }
)
if ($inputs.Count -eq 0) {
    throw "No .dts or .dtshd sample files found in $samples."
}

$convertArgs = @('convert', '--output-dir', $samples)
if ($Overwrite) {
    $convertArgs += '--overwrite'
}
$convertArgs += $inputs

Write-Host ("Converting {0} sample(s) with full PCM verification." -f $inputs.Count)
Write-Host "Using executable: $Binary"
& $Binary @convertArgs
exit $LASTEXITCODE
