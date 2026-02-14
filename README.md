# icicle

<p align="center">
  <img src="https://raw.githubusercontent.com/Eugeneofficial/icicle/main/docs/hero-v5.svg" alt="icicle v2.4 hero" width="100%" />
</p>

<p align="center">
  <strong>Premium Windows Disk Intelligence Suite</strong><br/>
  Native Desktop App + Fast CLI for heavy scans, interactive treemap, automation, and safe cleanup.
</p>

<p align="center">
  <a href="https://github.com/Eugeneofficial/icicle/actions/workflows/ci.yml"><img src="https://github.com/Eugeneofficial/icicle/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-16a34a.svg" alt="MIT"></a>
  <a href="go.mod"><img src="https://img.shields.io/badge/go-1.22+-00ADD8.svg" alt="Go"></a>
  <a href="https://github.com/Eugeneofficial/icicle/releases"><img src="https://img.shields.io/github/v/release/Eugeneofficial/icicle" alt="Release"></a>
  <a href="https://github.com/Eugeneofficial/icicle/stargazers"><img src="https://img.shields.io/github/stars/Eugeneofficial/icicle?style=social" alt="Stars"></a>
</p>

## EU

`icicle` is a production-grade Windows storage analyzer and cleanup tool.
It combines a native Wails desktop UI with a high-performance Go CLI.

Core value:
- Fast `tree` / `heavy` / `extensions` scans on large disks
- Interactive `WizMap` treemap with drill-down navigation
- Scheduled scans and scheduled cleanup from GUI
- Safe delete to Recycle Bin + queue + undo
- Advanced include/ignore filters for scan pipelines
- Plugin-style routing rules (`ext`, `contains`, `prefix`, `regex`)
- Encrypted profile export/import for portable setups

## RU 
`icicle` вЂ” СЌС‚Рѕ РїСЂРѕС„РµСЃСЃРёРѕРЅР°Р»СЊРЅС‹Р№ РёРЅСЃС‚СЂСѓРјРµРЅС‚ РґР»СЏ Windows, РєРѕС‚РѕСЂС‹Р№ РїРѕРјРѕРіР°РµС‚ Р±С‹СЃС‚СЂРѕ РЅР°С…РѕРґРёС‚СЊ, СЃРѕСЂС‚РёСЂРѕРІР°С‚СЊ Рё РѕС‡РёС‰Р°С‚СЊ С„Р°Р№Р»С‹ РЅР° Р±РѕР»СЊС€РёС… РґРёСЃРєР°С…. РџСЂРѕРµРєС‚ РѕР±СЉРµРґРёРЅСЏРµС‚ РЅР°С‚РёРІРЅС‹Р№ Desktop GUI Рё Р±С‹СЃС‚СЂС‹Р№ CLI, С‡С‚РѕР±С‹ СЂР°Р±РѕС‚Р°С‚СЊ РѕРґРёРЅР°РєРѕРІРѕ СѓРґРѕР±РЅРѕ Рё СЂСѓРєР°РјРё, Рё С‡РµСЂРµР· Р°РІС‚РѕРјР°С‚РёР·Р°С†РёСЋ.

Р§С‚Рѕ РІРЅСѓС‚СЂРё:
- Р±С‹СЃС‚СЂС‹Рµ РєРѕРјР°РЅРґС‹ `tree`, `heavy`, `extensions`
- РёРЅС‚РµСЂР°РєС‚РёРІРЅР°СЏ РєР°СЂС‚Р° РјРµСЃС‚Р° `WizMap` (РєР°Рє treemap)
- РїР»Р°РЅРёСЂРѕРІС‰РёРє СЃРєР°РЅРѕРІ Рё РїР»Р°РЅРёСЂРѕРІС‰РёРє РѕС‡РёСЃС‚РєРё РїСЂСЏРјРѕ РІ GUI
- Р±РµР·РѕРїР°СЃРЅРѕРµ СѓРґР°Р»РµРЅРёРµ РІ РєРѕСЂР·РёРЅСѓ + РѕС‡РµСЂРµРґСЊ РґРµР№СЃС‚РІРёР№ + `undo`
- С„РёР»СЊС‚СЂС‹ include/ignore РґР»СЏ С‚РѕС‡РЅРѕРіРѕ СЃРєР°РЅРёСЂРѕРІР°РЅРёСЏ
- РіРёР±РєРёРµ РїСЂР°РІРёР»Р° РјР°СЂС€СЂСѓС‚РёР·Р°С†РёРё С„Р°Р№Р»РѕРІ
- С€РёС„СЂРѕРІР°РЅРЅС‹Р№ СЌРєСЃРїРѕСЂС‚/РёРјРїРѕСЂС‚ РїСЂРѕС„РёР»СЏ (portable)
- RU/EN Р»РѕРєР°Р»РёР·Р°С†РёСЏ, dark/light С‚РµРјР°, С‚СЂРµР№, update flow

## Quick Start

CLI:

```powershell
git clone https://github.com/Eugeneofficial/icicle.git
cd icicle
go build -o icicle.exe ./cmd/icicle
.\icicle.exe
```

Desktop (Wails):

```powershell
.\scripts\build_wails.bat
.\icicle-desktop.exe
```

Manual desktop build:

```powershell
go build -tags "wails,production" -o icicle-desktop.exe ./cmd/icicle-wails
```

## Installer / Winget (Optional)

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build_installer.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\package_winget.ps1 -Version 2.4.0
```

## Screens

<p align="center">
  <img src="docs/screen-dashboard-v2.svg" alt="dashboard dark" width="32%" />
  <img src="docs/screen-heavy-v2.svg" alt="heavy panel" width="32%" />
  <img src="docs/screen-light-v2.svg" alt="dashboard light" width="32%" />
</p>

## Docs

- [ROADMAP.md](ROADMAP.md)
- [CHANGELOG.md](CHANGELOG.md)
- [BENCHMARKS.md](BENCHMARKS.md)
- [TESTING.md](TESTING.md)
- [CONTRIBUTING.md](CONTRIBUTING.md)
- [SECURITY.md](SECURITY.md)

## Topics

`windows`, `disk-cleanup`, `storage-analyzer`, `file-manager`, `golang`, `wails`, `desktop-app`, `cli`, `automation`, `performance`, `treemap`

## License

MIT. See [LICENSE](LICENSE).


