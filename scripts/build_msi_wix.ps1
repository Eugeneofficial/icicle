param(
  [string]$Version = "2.7.0",
  [string]$SourceDir = "."
)

$ErrorActionPreference = 'Stop'
$repo = Join-Path $PSScriptRoot '..'
Set-Location $repo

$wix = Get-Command wix -ErrorAction SilentlyContinue
if (-not $wix) {
  Write-Host "WiX not found; trying choco install wixtoolset..."
  choco install wixtoolset -y --no-progress | Out-Null
  $wix = Get-Command wix -ErrorAction SilentlyContinue
}
if (-not $wix) {
  Write-Warning "WiX CLI still unavailable. Skipping MSI build."
  exit 0
}

New-Item -ItemType Directory -Path dist -Force | Out-Null
$src = Resolve-Path $SourceDir
$out = Join-Path (Resolve-Path dist) ("icicle-" + $Version + "-setup.msi")

& wix build scripts/installer.wix.wxs -d Version=$Version -d SourceDir=$src -o $out
if ($LASTEXITCODE -ne 0) {
  throw "WiX build failed"
}
Write-Host "MSI built: $out"
