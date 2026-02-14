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
go build -tags "wails,production" -o icicle-desktop.exe ./cmd/icicle-wails

if ($env:ICICLE_SIGN_CERT_SHA1 -and (Get-Command signtool.exe -ErrorAction SilentlyContinue)) {
  Write-Host "Signing binaries with signtool..."
  signtool.exe sign /sha1 $env:ICICLE_SIGN_CERT_SHA1 /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 icicle.exe
  signtool.exe sign /sha1 $env:ICICLE_SIGN_CERT_SHA1 /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 icicle-desktop.exe
}

$dist = Join-Path (Get-Location) "dist"
if (!(Test-Path $dist)) { New-Item -ItemType Directory -Path $dist | Out-Null }

$pkg = Join-Path $dist "icicle-$Version-windows-x64"
if (Test-Path $pkg) { Remove-Item -Recurse -Force $pkg }
New-Item -ItemType Directory -Path $pkg | Out-Null

Copy-Item icicle.exe $pkg
Copy-Item icicle-desktop.exe $pkg
Copy-Item README.md $pkg
Copy-Item LICENSE $pkg
Copy-Item CHANGELOG.md $pkg
Copy-Item VERSION $pkg
Copy-Item icicle.bat $pkg
Copy-Item icicle-portable.bat $pkg
Copy-Item update.bat $pkg

$zip = "$pkg.zip"
if (Test-Path $zip) { Remove-Item -Force $zip }
Compress-Archive -Path (Join-Path $pkg '*') -DestinationPath $zip
Write-Host "Release package created: $zip"

$sha = (Get-FileHash -Algorithm SHA256 $zip).Hash.ToLower()
$shaFile = "$zip.sha256"
Set-Content -Path $shaFile -Value "$sha  $([System.IO.Path]::GetFileName($zip))"
Write-Host "SHA256 saved: $shaFile"
