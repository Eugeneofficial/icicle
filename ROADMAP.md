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

### Quality Goals (Delivered)
- [x] Keep `tree/heavy` responsive on very large disks
- [x] Keep desktop UX fast under heavy I/O
- [x] Keep update path one-click and rollback-safe
- [x] Keep CI green and reproducible builds

## Next (v2.3)

- [ ] Full visual charts for drive history timeline
- [ ] Snapshot diff viewer (delta between two scans)
- [ ] Preset dry-run preview with risk labels
- [ ] Smarter duplicate actions (keep newest/oldest rules)
- [ ] Better watch diagnostics panel (per-folder health)

## Later (v2.4)

- [ ] Scheduled cleanup tasks from GUI
- [ ] Advanced ignore/include filters for heavy/tree/ext scans
- [ ] Portable encrypted profile export/import
- [ ] Optional installer + winget package
- [ ] Plugin-style custom routing rules

## Community Track

- [x] Issue templates for bugs/features/performance
- [x] Pull request template
- [ ] Public benchmark scenarios from real user datasets
- [ ] Monthly changelog with performance deltas
