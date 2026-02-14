# icicle

<p align="center">
  <img src="docs/hero-v3.svg" alt="icicle hero" width="100%" />
</p>

<p align="center">
  <strong>Modern Windows disk intelligence suite.</strong><br/>
  Native desktop app + fast CLI for scanning, cleanup automation, safe file actions, and visual storage analytics.
</p>

<p align="center">
  <a href="https://github.com/Eugeneofficial/icicle/actions/workflows/ci.yml"><img src="https://github.com/Eugeneofficial/icicle/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-16a34a.svg" alt="MIT"></a>
  <a href="go.mod"><img src="https://img.shields.io/badge/go-1.22+-00ADD8.svg" alt="Go"></a>
  <a href="https://github.com/Eugeneofficial/icicle/releases"><img src="https://img.shields.io/github/v/release/Eugeneofficial/icicle" alt="Release"></a>
  <a href="https://github.com/Eugeneofficial/icicle/stargazers"><img src="https://img.shields.io/github/stars/Eugeneofficial/icicle?style=social" alt="Stars"></a>
</p>

## What It Does

- Tree and heavy-file scans tuned for huge disks
- Interactive WizMap (treemap) for space usage navigation
- Watch mode with auto-sort and routing rules
- Queue-based actions: move/delete with undo
- Safe delete via Recycle Bin
- Snapshot reports, diff viewer, and schedule automation
- Cleanup presets (Games / Media / Dev cache)
- Encrypted profile export/import (portable config)
- Advanced include/ignore filters for heavy/tree/ext scans
- RU/EN localization + dark/light theme

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

## Main Features

- `tree`: directory size map + top files
- `heavy`: top-N largest files + export (CSV / JSON / Markdown)
- `watch`: realtime folder watch with sorting
- `WizMap`: interactive treemap with drill-down and extension analytics
- Scheduled scans and scheduled cleanup tasks from GUI
- Plugin-style custom routing rules (ext/contains/prefix/regex)
- Encrypted profile backup/restore for portable setup

## Installer & Winget (Optional)

Build optional NSIS installer:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build_installer.ps1
```

Generate winget manifest templates:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\package_winget.ps1 -Version 2.3.0
```

Templates are generated in `winget/`.

## Screens

<p align="center">
  <img src="docs/screen-dashboard-v2.svg" alt="dashboard dark" width="32%" />
  <img src="docs/screen-heavy-v2.svg" alt="heavy panel" width="32%" />
  <img src="docs/screen-light-v2.svg" alt="dashboard light" width="32%" />
</p>

## Docs

- Roadmap: [ROADMAP.md](ROADMAP.md)
- Changelog: [CHANGELOG.md](CHANGELOG.md)
- Benchmarks: [BENCHMARKS.md](BENCHMARKS.md)
- Testing: [TESTING.md](TESTING.md)
- Contributing: [CONTRIBUTING.md](CONTRIBUTING.md)
- Security: [SECURITY.md](SECURITY.md)
- Code signing: [docs/CODE_SIGNING.md](docs/CODE_SIGNING.md)

## RU (Полная версия)

`icicle` — профессиональный Windows-инструмент для контроля свободного места, поиска тяжёлых файлов, безопасной очистки и автоматизации рутинных операций. Приложение объединяет быстрый CLI и нативный Desktop GUI (Wails), чтобы работать одинаково удобно и для power-user, и для обычного пользователя.

Ключевые возможности RU:

- интерактивная карта места (WizMap) с переходом по папкам
- быстрые сканы `tree` / `heavy` / `extensions` с фильтрами include/ignore
- авто-сортировка новых файлов через `watch`
- очередь действий по файлам: перенос, удаление, undo
- удаление в корзину как безопасный режим по умолчанию
- планировщик сканов и планировщик очистки прямо в GUI
- пресеты очистки (`games`, `media`, `dev-cache`) с оценкой риска
- шифрованный экспорт/импорт профиля (сохранённые папки + правила)
- настраиваемые plugin-style правила маршрутизации
- RU/EN, светлая/тёмная тема, системный трей, in-app update

## Repository Topics

`windows`, `disk-cleanup`, `storage-analyzer`, `file-manager`, `golang`, `wails`, `desktop-app`, `cli`, `automation`, `performance`, `treemap`

## License

MIT. See [LICENSE](LICENSE).
