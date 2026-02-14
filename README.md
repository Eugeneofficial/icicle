# icicle

<p align="center">
  <img src="docs/hero-v2.svg" alt="icicle hero" width="100%" />
</p>

<p align="center">
  <strong>Premium Windows disk intelligence toolkit.</strong><br/>
  Fast CLI + native desktop app for heavy-file analysis, folder automation, and safe cleanup.
</p>

<p align="center">
  <strong>RU:</strong> <code>icicle</code> — быстрый инструмент для анализа и очистки диска в Windows.<br/>
  CLI + нативный Desktop GUI для тяжёлых файлов, автосортировки и безопасной очистки.
</p>

<p align="center">
  <a href="https://github.com/Eugeneofficial/icicle/actions/workflows/ci.yml"><img src="https://github.com/Eugeneofficial/icicle/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-16a34a.svg" alt="MIT"></a>
  <a href="go.mod"><img src="https://img.shields.io/badge/go-1.22+-00ADD8.svg" alt="Go"></a>
  <a href="https://github.com/Eugeneofficial/icicle/releases"><img src="https://img.shields.io/github/v/release/Eugeneofficial/icicle" alt="Release"></a>
  <a href="https://github.com/Eugeneofficial/icicle/stargazers"><img src="https://img.shields.io/github/stars/Eugeneofficial/icicle?style=social" alt="Stars"></a>
</p>

## Why icicle

`icicle` is designed for people who need immediate control over disk usage without sacrificing speed or safety.

- Native Windows desktop app (`Wails`) and fast CLI in one project
- Large-folder performance mode with worker/file-cap tuning
- Heavy-file actions directly in UI: open, reveal, move, delete, undo
- Live watch mode for auto-sorting incoming files
- Safe-delete workflow (Recycle Bin option)
- RU/EN localization and dark/light themes

## Product Highlights

- `tree`: directory size map + top files
- `heavy`: top-N largest files with export (CSV / JSON / Markdown)
- `watch`: real-time folder monitoring with auto-routing by extension
- Drive dashboard with instant path actions
- Built-in updater flow for desktop releases
- Tray integration with quick reopen

## Architecture

- `cmd/icicle` — CLI + browser GUI entrypoint
- `cmd/icicle-wails` — native desktop app entrypoint
- `internal/scan` — high-performance traversal and stats
- `internal/organize` — file routing and move rules
- `internal/commands` — CLI command handlers
- `internal/gui` — legacy browser GUI backend (still supported)

## Quick Start

### 1) CLI build

```powershell
git clone https://github.com/Eugeneofficial/icicle.git
cd icicle
go build -o icicle.exe ./cmd/icicle
.\icicle.exe
```

### 2) Desktop build (native, no browser)

```powershell
.\scripts\build_wails.bat
.\icicle-desktop.exe
```

Manual desktop build:

```powershell
go build -tags "wails,production" -o icicle-desktop.exe ./cmd/icicle-wails
```

## CLI Usage

```text
icicle watch [--dry-run] [path]
icicle heavy [--n 20] [path]
icicle tree [--n 20] [--w 24] [--top 5] [path]
```

Default paths:

- `watch` -> Windows `Downloads`
- `heavy/tree` -> Windows home folder

Examples:

```powershell
.\icicle.exe tree C:\
.\icicle.exe heavy --n 30 D:\
.\icicle.exe watch --dry-run C:\Users\you\Downloads
```

## Desktop UX

<p align="center">
  <img src="docs/screen-dashboard-v2.svg" alt="icicle dashboard" width="49%" />
  <img src="docs/screen-heavy-v2.svg" alt="icicle heavy panel" width="49%" />
</p>

Desktop app includes:

- Compact heavy-file action panel
- Loader animation during scans
- Fast mode controls (`max files`, `workers`)
- Update check/apply workflow

## Performance Notes

- Fast mode intentionally returns partial heavy results for speed
- Full precision mode: disable fast mode or set file cap to `0`
- Worker tuning is available directly in desktop UI

## Update & Release

Update local repo and rebuild:

```powershell
.\update.bat
```

Create release artifacts:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\release.ps1
```

Artifacts are generated in `dist/`.

## Documentation

- Product roadmap: [ROADMAP.md](ROADMAP.md)
- Test notes: [TESTING.md](TESTING.md)
- Changelog: [CHANGELOG.md](CHANGELOG.md)
- Contributing guide: [CONTRIBUTING.md](CONTRIBUTING.md)
- Security policy: [SECURITY.md](SECURITY.md)

## RU: Полная Версия

`icicle` — профессиональный инструмент для контроля, анализа и очистки дискового пространства в Windows.  
Проект объединяет быстрый CLI и нативный Desktop GUI: можно быстро найти тяжёлые файлы, навести порядок в папках и автоматизировать рутинные операции.

### Основные возможности

- `tree`: визуализация дерева размеров + топ файлов
- `heavy`: список самых тяжёлых файлов с экспортом (`CSV/JSON/Markdown`)
- `watch`: авто-сортировка новых файлов по расширениям в реальном времени
- действия с файлами из GUI: открыть, показать в проводнике, перенести, авто-перенести, удалить, отменить перенос
- безопасное удаление через корзину (опционально)
- карточки дисков с быстрыми действиями
- быстрый режим сканирования для очень больших папок (`max files`, `workers`)
- локализация RU/EN, темы Dark/Light, трей и автообновление desktop-версии

### Быстрый старт (Windows)

CLI:

```powershell
go build -o icicle.exe ./cmd/icicle
.\icicle.exe
```

Desktop:

```powershell
.\scripts\build_wails.bat
.\icicle-desktop.exe
```

### Подсказка по производительности

- Для максимальной скорости включайте fast mode и задавайте лимит файлов.
- Для максимальной точности отключайте fast mode или ставьте лимит `0`.
- Для крупных дисков повышайте `workers` (обычно 16–32).

## License

MIT License. See [LICENSE](LICENSE).
