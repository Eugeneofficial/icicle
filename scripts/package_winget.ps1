param(
  [string]$Version = "2.3.0",
  [string]$Publisher = "Eugeneofficial",
  [string]$RepoUrl = "https://github.com/Eugeneofficial/icicle",
  [string]$InstallerUrl = "https://github.com/Eugeneofficial/icicle/releases/download/v2.3.0/icicle-desktop.exe"
)

$manifestDir = Join-Path $PSScriptRoot "..\winget"
New-Item -ItemType Directory -Force -Path $manifestDir | Out-Null

$base = @"
PackageIdentifier: Eugeneofficial.Icicle
PackageVersion: $Version
PackageLocale: en-US
Publisher: $Publisher
PublisherUrl: $RepoUrl
PackageName: icicle
License: MIT
ShortDescription: Desktop and CLI disk analyzer with automation and cleanup presets.
Installers:
  - Architecture: x64
    InstallerType: exe
    InstallerUrl: $InstallerUrl
    InstallerSha256: REPLACE_WITH_SHA256
ManifestType: installer
ManifestVersion: 1.6.0
"@
Set-Content -Path (Join-Path $manifestDir "Eugeneofficial.Icicle.installer.yaml") -Value $base -Encoding UTF8

$locale = @"
PackageIdentifier: Eugeneofficial.Icicle
PackageVersion: $Version
PackageLocale: en-US
Publisher: $Publisher
PackageName: icicle
ShortDescription: Fast disk insights, watch automation, cleanup scheduler, and interactive treemap.
ManifestType: defaultLocale
ManifestVersion: 1.6.0
"@
Set-Content -Path (Join-Path $manifestDir "Eugeneofficial.Icicle.locale.en-US.yaml") -Value $locale -Encoding UTF8

$version = @"
PackageIdentifier: Eugeneofficial.Icicle
PackageVersion: $Version
DefaultLocale: en-US
ManifestType: version
ManifestVersion: 1.6.0
"@
Set-Content -Path (Join-Path $manifestDir "Eugeneofficial.Icicle.yaml") -Value $version -Encoding UTF8

Write-Host "Winget manifest templates created in winget/."
Write-Host "Remember to replace InstallerSha256 before submitting." 
