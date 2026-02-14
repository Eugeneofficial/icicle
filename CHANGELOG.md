# Changelog

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
