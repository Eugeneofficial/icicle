param(
  [string]$Version
)

if (-not $Version) {
  $versionFile = Join-Path $PSScriptRoot '..\VERSION'
  $Version = (Get-Content -Raw $versionFile).Trim()
}

$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

go test ./...
go vet ./...
go build -o icicle.exe ./cmd/icicle

$dist = Join-Path (Get-Location) "dist"
if (!(Test-Path $dist)) { New-Item -ItemType Directory -Path $dist | Out-Null }

$pkg = Join-Path $dist "icicle-$Version-windows-x64"
if (Test-Path $pkg) { Remove-Item -Recurse -Force $pkg }
New-Item -ItemType Directory -Path $pkg | Out-Null

Copy-Item icicle.exe $pkg
Copy-Item README.md $pkg
Copy-Item LICENSE $pkg
Copy-Item CHANGELOG.md $pkg
Copy-Item VERSION $pkg
Copy-Item icicle.bat $pkg

$zip = "$pkg.zip"
if (Test-Path $zip) { Remove-Item -Force $zip }
Compress-Archive -Path (Join-Path $pkg '*') -DestinationPath $zip
Write-Host "Release package created: $zip"
