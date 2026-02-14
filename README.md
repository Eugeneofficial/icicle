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

- Native Windows desktop app (`Wails`) + fast CLI in one codebase
- Real-world performance mode for huge folders (`max files` + `workers`)
- Direct file actions in UI: open, reveal, auto-move, move-to, delete, undo
- Live watch mode with extension-based routing
- Safe-delete option (Recycle Bin)

## Product Highlights

- `tree`: directory size map + top files
- `heavy`: top-N largest files + export (CSV / JSON / Markdown)
- `watch`: real-time auto-sort for incoming files
- Drive dashboard with quick actions
- Tray reopen + in-app update flow
- RU/EN localization + Dark/Light themes
- Drive history timeline data collection
- Scheduled scan snapshots (background report generation)
- Smart cleanup presets: `games`, `media`, `dev-cache`
- Fast extension analytics for huge trees

## Quality & Reliability

- Fast-path scan mode for heavy/tree/ext with file limits + worker tuning
- Desktop busy-guard for heavy operations under high I/O load
- Update flow with rollback-safe backup (`.bak`) if swap/start fails
- CI checks: format, tests, race checks (core packages), Windows CLI/Desktop build
- Reproducible build flags in CI/release/update scripts (`-trimpath`, `-buildvcs=false`, deterministic `ldflags`)

## Quick Start

CLI:

```powershell
git clone https://github.com/Eugeneofficial/icicle.git
cd icicle
go build -o icicle.exe ./cmd/icicle
.\icicle.exe
```

Desktop (native):

```powershell
.\scripts\build_wails.bat
.\icicle-desktop.exe
```

Manual desktop build:

```powershell
go build -tags "wails,production" -o icicle-desktop.exe ./cmd/icicle-wails
```

## One-Click Install (Release)

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\install.ps1
```

## CLI Usage

```text
icicle watch [--dry-run] [path]
icicle heavy [--n 20] [path]
icicle tree [--n 20] [--w 24] [--top 5] [path]
```

Defaults:
- `watch` -> Windows `Downloads`
- `heavy/tree` -> Windows home folder

## Screens

<p align="center">
  <img src="docs/screen-dashboard-v2.svg" alt="dashboard dark" width="32%" />
  <img src="docs/screen-heavy-v2.svg" alt="heavy panel" width="32%" />
  <img src="docs/screen-light-v2.svg" alt="dashboard light" width="32%" />
</p>

## Benchmarks

See full benchmark notes: [BENCHMARKS.md](BENCHMARKS.md)

## Update & Release

Update local clone:

```powershell
.\update.bat
```

Create release package + checksum:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\release.ps1
```

Artifacts are produced in `dist/` (`.zip` + `.sha256`).

## Roadmap & Launch Docs

- Roadmap: [ROADMAP.md](ROADMAP.md)
- Changelog: [CHANGELOG.md](CHANGELOG.md)
- Release notes template: [RELEASE_NOTES_v2.0.0.md](RELEASE_NOTES_v2.0.0.md)
- Contributing: [CONTRIBUTING.md](CONTRIBUTING.md)
- Security policy: [SECURITY.md](SECURITY.md)
- Code signing plan: [docs/CODE_SIGNING.md](docs/CODE_SIGNING.md)

## GitHub Growth Setup

Recommended repository topics:
`windows`, `disk-cleanup`, `file-manager`, `golang`, `wails`, `desktop-app`, `cli`, `fsnotify`, `performance`, `storage`, `automation`

Enable Discussions in repository settings:
`Settings -> General -> Features -> Discussions`

## RU: Полная Версия

`icicle` — профессиональный инструмент для контроля, анализа и очистки дискового пространства в Windows. Он объединяет быстрый CLI и нативный Desktop GUI, позволяя быстро находить тяжёлые файлы, строить карту размеров, наводить порядок и автоматизировать рутину.

Ключевые возможности:

- `tree`: визуализация дерева размеров + топ файлов
- `heavy`: поиск тяжёлых файлов + экспорт (`CSV/JSON/Markdown`)
- `watch`: авто-сортировка новых файлов по расширениям
- действия по файлам из GUI: открыть, показать в проводнике, перенести, удалить, отменить перенос
- безопасное удаление в корзину (опционально)
- карточки дисков, быстрый режим сканирования, RU/EN, Dark/Light, трей и обновления

## License

MIT. See [LICENSE](LICENSE).
