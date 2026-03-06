# Changelog

## 5.0.0 - 2026-03-06
- Major desktop redesign across the Wails UI:
  - new design system and visual hierarchy,
  - rebuilt Dashboard / Analyze / Automation scenes,
  - stronger navigation, action grouping, and health surfaces,
  - improved responsive behavior for narrower windows.
- Frontend structure improved by extracting desktop CSS into `cmd/icicle-wails/frontend/app.css`.
- Heavy-file workspace upgraded into a clearer action-queue workflow with a dedicated control strip and stronger filtering layout.
- Dashboard upgraded into a two-zone operational view with drive intelligence and an adjacent diagnostic stack.
- Automation view reorganized by operating domain instead of a flat block of controls.
- Watcher safety improved:
  - waits for files to stabilize before auto-moving,
  - reduces risk of moving still-downloading files.
- Limited scan behavior hardened:
  - max-file limits are now enforced more strictly under concurrent scanning,
  - added regression coverage for limited heavy scans.
- CLI consistency pass:
  - `heavy` now rejects negative `-n`,
  - progress bar output is fixed-width and predictable.
- Expanded test coverage for watcher stability, heavy flag validation, UI bar rendering, tree flags, single-instance address generation, and scan-limit behavior.
- Build artifacts refreshed for the new desktop release (`icicle.exe`, `icicle-desktop.exe`).

## 3.1.1 - 2026-02-18
- Added favorite files workflow in heavy list (`☆/★`) with persisted favorites and `Fav only` mode.
- Added heavy extension quick chips for one-click filtering by dominant extensions.
- Added scan history dropdown with one-click rerun (`tree` / `heavy` / `wiz`).
- Added heavy auto-refresh mode with configurable interval.
- Added `Export selected TXT` for selected heavy paths.

## 3.1.0 - 2026-02-15
- Completed v3.0 release prep checklist (RU/EN wording pass + final visual QA targets).
- Added high-performance heavy table pipeline:
  - virtualized rendering with incremental chunk paint,
  - filter/sort cache and debounced inputs,
  - `Load more` pagination and live row counters,
  - quick actions: `Clear filters` and `Refresh`.
- Optimized queue execution by grouping batch jobs and deduplicating file paths.
- Added short-lived scan caches for `tree`, `heavy`, and WizMap requests.
- Added adaptive log polling behavior (watch-aware intervals, lower idle overhead).
- Added persistence for scan controls and UX preferences (theme/lang/filters/perf inputs).
- Improved quick-open and path flows:
  - cached defaults lookup,
  - path normalization before actions,
  - folder picker in Analyze quick path row.
- Added extra keyboard shortcuts for faster scanning workflows.
- Scanner hot-path optimizations in `internal/scan`:
  - faster extension parsing,
  - faster first path-segment extraction for tree/overview reducers.
- Extended scan tests to cover new helper fast-path behavior.
- README rewritten in modern release format (EN primary + full RU section).
- Roadmap updated with delivered `v3.1` and planned `v3.2` tracks.

## 2.0.0 - 2026-02-14
- Added native desktop app mode with Wails (`icicle-desktop.exe`).
- Reworked heavy-files UI block for denser, faster actions.
- Added fast heavy scan tuning (file cap + worker count).
- Added loading overlay animation for long scans.
- Added heavy export (CSV/JSON/Markdown) in desktop mode.
- Added tray integration for desktop mode with reopen.
- Added in-app updater support for desktop mode.
- Updated README, ROADMAP, and visual docs assets for v2.

## 1.0.1 - 2026-02-13
- Added `update.bat` for pull + rebuild update flow.
- Removed update buttons from GUI header.
- GUI now shows real app version in header badge.
- Improved Windows known-folder defaults for CLI/GUI.

## 1.0.0 - 2026-02-12
- Windows-first GUI + CLI release.
- Fast scanning for heavy files and tree stats.
- Folder watch with auto-sort rules.
- Drive dashboard with quick actions.
- Heavy panel with filters, sorting, bulk actions, exports.
- Snapshots, report export, empty-folder cleanup, extension and duplicate analysis.
- Tray integration with reopen/exit.
- GUI auto-update via GitHub Releases (check/install/restart).
- Konami easter egg.
- Release hardening for access-denied paths.
