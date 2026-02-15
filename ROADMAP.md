# Roadmap

## Released

### v2.1 (Delivered)
- [x] Background full-scan with progressive partial results in GUI
- [x] Batch file actions (multi-select, queue, undo stack)
- [x] Duplicate finder v2 (hash mode + quick name mode)
- [x] Better watch reliability on protected/system folders
- [x] Signed binaries + release checksums

### v2.2 (Delivered)
- [x] Storage history timeline per drive
- [x] Scheduled scans + report snapshots
- [x] Smart cleanup presets (Games, Media, Dev cache)
- [x] Faster extension analytics for huge trees
- [x] Better i18n consistency and glossary lock

### v2.3 (Delivered)
- [x] Full visual charts for drive history timeline
- [x] Snapshot diff viewer (delta between two scans)
- [x] Preset dry-run preview with risk labels
- [x] Smarter duplicate actions (keep newest/oldest rules)
- [x] Better watch diagnostics panel (per-folder health)

### v2.4 (Delivered)
- [x] Scheduled cleanup tasks from GUI
- [x] Advanced ignore/include filters for heavy/tree/ext scans
- [x] Portable encrypted profile export/import
- [x] Optional installer + winget package templates
- [x] Plugin-style custom routing rules

### v2.5 (Delivered)

- [x] Treemap zoom breadcrumbs + keyboard navigation
- [x] Cleanup scheduler calendar mode (daily/weekly)
- [x] Profile import conflict resolver (merge vs overwrite)
- [x] Rule tester panel with sample file simulation
- [x] Full release signing pipeline in CI

## Next (v2.6)

- [x] Treemap hover details panel (path + ext + size heat)
- [x] Cleanup schedule presets per disk (C:, D:, E:)
- [x] Routing rules visual editor (no JSON prompt)
- [x] Signed installer artifact (.msi/.exe) in release pipeline
- [x] Multi-language glossary QA automation

### v2.7 (Delivered)

- [x] Treemap compare mode between snapshots
- [x] Preset import/export packs per team
- [x] Routing rule conflict detector and priority solver
- [x] Native MSI pipeline alternative (WiX)
- [x] Localization regression tests on GUI strings

## Next (v2.8)

- [x] Delta heat overlay directly in live WizMap
- [x] Team preset registry (remote URL import)
- [x] Rule simulation against full scan result set
- [x] MSI + EXE notarization/report in release summary
- [x] Snapshot compare export (CSV/JSON/MD)
- [x] New file detection marker (`NEW`) in heavy list

## Community Track

- [x] Issue templates for bugs/features/performance
- [x] Pull request template
- [ ] Public benchmark scenarios from real user datasets
- [ ] Monthly changelog with performance deltas

## v3.0 Release Prep (Completed)

- [x] Full UI redesign pass (dashboard + command bar + denser file operations)
- [x] 10+ operator features for desktop workflow
- [x] Command palette and keyboard-first operation (`Ctrl+K`)
- [x] Heavy table pro filters/sort/bulk selection
- [x] Queue preset save/load
- [x] Quick destination registry
- [x] Live WizMap delta overlay from snapshot
- [x] Full-route simulation report on scan set
- [x] Snapshot compare export (CSV/JSON/MD)
- [x] Team preset registry import via URL
- [x] Release signing summary in CI
- [x] `NEW` file marker in heavy list
- [x] Final RU/EN wording polish for release notes
- [x] Final visual QA on 1366x768 and 4K

## v3.1 (Delivered)

- [x] Heavy-table virtualized rendering with incremental chunk paint
- [x] Heavy filter/sort cache + debounce for search/min-size controls
- [x] Heavy `Load more` pagination and row stats
- [x] Fast action bar (`Clear filters`, `Refresh`) in Analyze
- [x] Smart selection behavior on filtered rows
- [x] Short-lived cache for `tree`/`heavy` scan requests
- [x] Short-lived cache for WizMap requests
- [x] Adaptive log polling (watch-aware interval)
- [x] Queue execution optimization (batch grouping + dedup paths)
- [x] Input persistence for scan controls and UI preferences
- [x] Path normalization and safer fast actions
- [x] Reduced redundant backend calls via defaults/drive refresh caching
- [x] Extra keyboard shortcuts for scan flows
- [x] Scanner micro-optimizations in hot loops (path segment + extension parsing)
- [x] Extended scanner tests for helper fast-paths

## v3.2 (Next)

- [ ] Parallel reducer pipeline in scanner (lower lock contention)
- [ ] Persistent snapshot index for instant diff preloading
- [ ] Streaming heavy/tree updates from backend workers
- [ ] Drive-level cache invalidation strategy after file actions
- [ ] Public benchmark dataset pack + monthly perf delta report
