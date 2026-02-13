param(
  [string]$Version
)

$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

if (-not $Version) {
  $Version = (Get-Content -Raw .\VERSION).Trim()
}

go build -o icicle.exe ./cmd/icicle

$dist = Join-Path (Get-Location) "dist"
if (!(Test-Path $dist)) { New-Item -ItemType Directory -Path $dist | Out-Null }

$pkg = Join-Path $dist "icicle-$Version-windows-portable"
if (Test-Path $pkg) { Remove-Item -Recurse -Force $pkg }
New-Item -ItemType Directory -Path $pkg | Out-Null
New-Item -ItemType Directory -Path (Join-Path $pkg "portable-data") | Out-Null

Copy-Item icicle.exe $pkg
Copy-Item icicle.bat $pkg
Copy-Item icicle-portable.bat $pkg
Copy-Item update.bat $pkg
Copy-Item README.md $pkg
Copy-Item LICENSE $pkg
Copy-Item VERSION $pkg
Copy-Item CHANGELOG.md $pkg

$zip = "$pkg.zip"
if (Test-Path $zip) { Remove-Item -Force $zip }
Compress-Archive -Path (Join-Path $pkg '*') -DestinationPath $zip
Write-Host "Portable package created: $zip"
