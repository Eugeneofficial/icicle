param(
  [string]$InstallDir = "$env:LOCALAPPDATA\icicle"
)

$ErrorActionPreference = 'Stop'
$repo = 'Eugeneofficial/icicle'
$api = "https://api.github.com/repos/$repo/releases/latest"

Write-Host "[icicle-install] fetching latest release metadata..."
$release = Invoke-RestMethod -Uri $api -Headers @{ 'User-Agent' = 'icicle-installer' }
$asset = $release.assets | Where-Object { $_.name -like '*windows-x64.zip' } | Select-Object -First 1
if (-not $asset) { throw 'No windows-x64 zip asset found in latest release.' }

$tmpZip = Join-Path $env:TEMP ("icicle-" + $release.tag_name + ".zip")
Write-Host "[icicle-install] downloading $($asset.name)..."
Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $tmpZip -UseBasicParsing

if (Test-Path $InstallDir) { Remove-Item -Recurse -Force $InstallDir }
New-Item -ItemType Directory -Path $InstallDir | Out-Null
Expand-Archive -Path $tmpZip -DestinationPath $InstallDir -Force

Write-Host "[icicle-install] installed to $InstallDir"
Write-Host "Run: $InstallDir\icicle-desktop.exe"
