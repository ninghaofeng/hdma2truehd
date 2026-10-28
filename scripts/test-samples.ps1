[CmdletBinding()]
param(
    [ValidateNotNullOrEmpty()]
    [string]$Binary = (Join-Path $PSScriptRoot '../bin/hdma2truehd.exe'),
    [switch]$Overwrite
)

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false

if (-not (Test-Path -LiteralPath $Binary -PathType Leaf)) {
    throw "Executable not found: $Binary. Build the program first or specify -Binary PATH."
}
$Binary = (Resolve-Path -LiteralPath $Binary).ProviderPath

$samples = Join-Path (Split-Path -Parent $PSScriptRoot) 'samples'
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
& $Binary @convertArgs
exit $LASTEXITCODE
