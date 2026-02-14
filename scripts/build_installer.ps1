param(
  [string]$Version = "2.3.0"
)

$ErrorActionPreference = "Stop"

Write-Host "Building Wails desktop binary..."
wails build -platform windows/amd64

$nsis = Get-Command makensis -ErrorAction SilentlyContinue
if (-not $nsis) {
  Write-Warning "makensis not found. Install NSIS to build optional installer."
  Write-Host "Portable binary is ready in build/bin/."
  exit 0
}

$script = Join-Path $PSScriptRoot "..\scripts\installer.nsi"
if (-not (Test-Path $script)) {
  Write-Warning "installer.nsi is missing. Create script and run again."
  exit 1
}

& $nsis.Source $script /DVERSION=$Version
Write-Host "Installer build completed."
