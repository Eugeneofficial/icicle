# icicle

<p align="center">
  <img src="https://raw.githubusercontent.com/Eugeneofficial/icicle/main/docs/hero-v5.svg" alt="icicle hero" width="100%" />
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

## EN

`icicle` is a production-grade Windows storage analyzer and cleanup tool.

- Fast `tree` / `heavy` / `extensions` scans on large disks
- Interactive `WizMap` treemap with drill-down, breadcrumbs, and keyboard navigation
- Hover details panel for `WizMap` (path + extension + size heat)
- Scheduled scans and scheduled cleanup from GUI
- Cleanup calendar mode: `interval` / `daily` / `weekly`
- Cleanup schedule presets per disk (`C:`, `D:`, `E:`...)
- Safe delete to Recycle Bin + queue + undo
- Include/ignore filters for scan pipelines
- Plugin-style routing rules (`ext`, `contains`, `prefix`, `regex`)
- Visual routing rules editor (no raw JSON required)
- Route tester panel with sample file simulation
- Encrypted profile export/import for portable setups
- Profile import conflict resolver: `merge` or `overwrite`
- Release pipeline uploads signed binaries/installer artifacts (when signing secrets are configured)

## RU

<p>icicle &#x2014; &#x43F;&#x440;&#x43E;&#x444;&#x435;&#x441;&#x441;&#x438;&#x43E;&#x43D;&#x430;&#x43B;&#x44C;&#x43D;&#x44B;&#x439; &#x438;&#x43D;&#x441;&#x442;&#x440;&#x443;&#x43C;&#x435;&#x43D;&#x442; &#x434;&#x43B;&#x44F; Windows: &#x431;&#x44B;&#x441;&#x442;&#x440;&#x44B;&#x439; &#x430;&#x43D;&#x430;&#x43B;&#x438;&#x437; &#x434;&#x438;&#x441;&#x43A;&#x430;, &#x43F;&#x43E;&#x438;&#x441;&#x43A; &#x442;&#x44F;&#x436;&#x451;&#x43B;&#x44B;&#x445; &#x444;&#x430;&#x439;&#x43B;&#x43E;&#x432;, &#x431;&#x435;&#x437;&#x43E;&#x43F;&#x430;&#x441;&#x43D;&#x430;&#x44F; &#x43E;&#x447;&#x438;&#x441;&#x442;&#x43A;&#x430; &#x438; &#x430;&#x432;&#x442;&#x43E;&#x43C;&#x430;&#x442;&#x438;&#x437;&#x430;&#x446;&#x438;&#x44F;.</p>
<p>&#x41F;&#x440;&#x43E;&#x435;&#x43A;&#x442; &#x43E;&#x431;&#x44A;&#x435;&#x434;&#x438;&#x43D;&#x44F;&#x435;&#x442; &#x43D;&#x430;&#x442;&#x438;&#x432;&#x43D;&#x44B;&#x439; Desktop GUI &#x438; &#x431;&#x44B;&#x441;&#x442;&#x440;&#x44B;&#x439; CLI, &#x447;&#x442;&#x43E;&#x431;&#x44B; &#x443;&#x434;&#x43E;&#x431;&#x43D;&#x43E; &#x440;&#x430;&#x431;&#x43E;&#x442;&#x430;&#x442;&#x44C; &#x438; &#x432;&#x440;&#x443;&#x447;&#x43D;&#x443;&#x44E;, &#x438; &#x43F;&#x43E; &#x440;&#x430;&#x441;&#x43F;&#x438;&#x441;&#x430;&#x43D;&#x438;&#x44E;.</p>

- &#x411;&#x44B;&#x441;&#x442;&#x440;&#x44B;&#x435; &#x43A;&#x43E;&#x43C;&#x430;&#x43D;&#x434;&#x44B; `tree`, `heavy`, `extensions`
- &#x418;&#x43D;&#x442;&#x435;&#x440;&#x430;&#x43A;&#x442;&#x438;&#x432;&#x43D;&#x430;&#x44F; &#x43A;&#x430;&#x440;&#x442;&#x430; &#x43C;&#x435;&#x441;&#x442;&#x430; `WizMap` (treemap)
- &#x41F;&#x43B;&#x430;&#x43D;&#x438;&#x440;&#x43E;&#x432;&#x449;&#x438;&#x43A; &#x441;&#x43A;&#x430;&#x43D;&#x43E;&#x432; &#x438; &#x43E;&#x447;&#x438;&#x441;&#x442;&#x43A;&#x438; &#x43F;&#x440;&#x44F;&#x43C;&#x43E; &#x432; GUI
- &#x411;&#x435;&#x437;&#x43E;&#x43F;&#x430;&#x441;&#x43D;&#x43E;&#x435; &#x443;&#x434;&#x430;&#x43B;&#x435;&#x43D;&#x438;&#x435; &#x432; &#x43A;&#x43E;&#x440;&#x437;&#x438;&#x43D;&#x443; + &#x43E;&#x447;&#x435;&#x440;&#x435;&#x434;&#x44C; &#x434;&#x435;&#x439;&#x441;&#x442;&#x432;&#x438;&#x439; + `undo`
- &#x424;&#x438;&#x43B;&#x44C;&#x442;&#x440;&#x44B; include/ignore &#x434;&#x43B;&#x44F; &#x442;&#x43E;&#x447;&#x43D;&#x43E;&#x433;&#x43E; &#x441;&#x43A;&#x430;&#x43D;&#x438;&#x440;&#x43E;&#x432;&#x430;&#x43D;&#x438;&#x44F;
- &#x413;&#x438;&#x431;&#x43A;&#x438;&#x435; &#x43F;&#x440;&#x430;&#x432;&#x438;&#x43B;&#x430; &#x43C;&#x430;&#x440;&#x448;&#x440;&#x443;&#x442;&#x438;&#x437;&#x430;&#x446;&#x438;&#x438; &#x444;&#x430;&#x439;&#x43B;&#x43E;&#x432;
- &#x428;&#x438;&#x444;&#x440;&#x43E;&#x432;&#x430;&#x43D;&#x43D;&#x44B;&#x439; &#x44D;&#x43A;&#x441;&#x43F;&#x43E;&#x440;&#x442;/&#x438;&#x43C;&#x43F;&#x43E;&#x440;&#x442; &#x43F;&#x440;&#x43E;&#x444;&#x438;&#x43B;&#x44F; (portable)
- RU/EN &#x43B;&#x43E;&#x43A;&#x430;&#x43B;&#x438;&#x437;&#x430;&#x446;&#x438;&#x44F;, dark/light &#x442;&#x435;&#x43C;&#x430;, &#x442;&#x440;&#x435;&#x439;, update flow

## Quick Start

```powershell
git clone https://github.com/Eugeneofficial/icicle.git
cd icicle
go build -o icicle.exe ./cmd/icicle
.\icicle.exe
```

```powershell
.\scripts\build_wails.bat
.\icicle-desktop.exe
```

## Optional Installer / Winget

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build_installer.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\package_winget.ps1 -Version 2.4.0
```

## License

MIT. See [LICENSE](LICENSE).
