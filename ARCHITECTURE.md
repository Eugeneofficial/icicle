# Architecture

## Package Structure

```
cmd/icicle-wails/
├── app_windows.go              # Main App struct + all handlers (2800 lines)
│   ├── § Lifecycle            # startup, shutdown, Version, Defaults
│   ├── § Logging              # appendLog, ClearLog, WatchLog, pipe
│   ├── § Scan: Tree           # RunTree, RunTreeFast
│   ├── § Scan: Heavy          # RunHeavy, RunHeavyFast, FullScan+Cancel
│   ├── § Export               # ExportHeavy (CSV/JSON/MD)
│   ├── § Watch                # StartWatch, StopWatch, Diagnostics
│   ├── § Drives               # ListDrives, DriveHistory, OpenDrive
│   ├── § Navigation           # OpenPath, RevealPath, PickFolder, FolderHint
│   ├── § Saved Folders        # List/Save/Remove, load/save
│   ├── § File Operations      # Move, Delete, Batch, Undo
│   ├── § Empty Dirs           # Clean, Find, Delete
│   ├── § Cleanup Presets      # Scan, Apply, match helpers
│   ├── § Extension Stats      # ExtensionStats, ExtensionStatsFast
│   ├── § WizMap               # WizMap, WizMapTurbo, Delta
│   ├── § Duplicates           # Names, FinderV2, Keep
│   ├── § Scheduled Scan       # Start/Stop, Loop, Snapshots
│   ├── § Snapshots            # List, Diff, TreemapDiff, Export
│   └── § Utilities            # normalizePath, markNewHeavy, csv helpers
│
├── tray_windows.go             # System tray integration
├── update_windows.go           # Auto-update (Check/Apply)
├── routing_windows.go          # Routing rules engine + conflict detection
├── filters_windows.go          # Filtered scan wrappers (heavy/tree/ext)
├── cleanup_presets_windows.go  # Cleanup preset management + team packs
├── schedule_cleanup_profile_windows.go  # Scheduled cleanup + profiles
├── main_windows.go             # Wails entry point
│
├── frontend/
│   ├── index.html              # Single-page UI (vanilla JS)
│   └── app.css                 # Design system (Xiaomi-inspired dark theme)
│
└── handlers/
    └── handler.go              # Foundation for future handler extraction
```

## Internal Packages

```
internal/
├── scan/        # Core scanner: heavy, tree, extension stats, overview
├── commands/    # CLI commands: heavy, tree, watch, interactive
├── organize/    # Routing rules and file organization
├── ui/          # CLI theming: colors, bars, human-readable bytes
├── meta/        # Version info
└── singleinstance/  # Mutex-based single-instance guard
```

## Refactoring Plan

### Phase 1: Section Markers ✅ (Done)
- Added `§` section markers to `app_windows.go` for navigation
- Documented code organization in package doc comment

### Phase 2: Handler Extraction (Planned)
Extract each `§` section into its own file:

```
cmd/icicle-wails/
├── app_windows.go          # Lifecycle, logging, drives, navigation (~300 lines)
├── scan_handler.go         # Tree, Heavy, WizMap, ExtStats (~700 lines)
├── fileops_handler.go      # Move, Delete, Empty Dirs, Saved Folders (~350 lines)
├── snapshot_handler.go     # Snapshots, Diff, Export (~330 lines)
├── schedule_handler.go     # Scheduled Scan + Cleanup (~250 lines)
├── duplicate_handler.go    # Duplicate finder + keep logic (~180 lines)
├── cleanup_handler.go      # Cleanup presets + apply (~150 lines)
├── export_handler.go       # ExportHeavy + Snapshot compare export (~200 lines)
├── watch_handler.go        # Watch + Diagnostics (~100 lines)
└── handlers/               # Foundation package (shared types/utils)
    └── handler.go
```

### Phase 3: Type Cleanup
- Replace anonymous struct types with named types
- Extract shared interfaces
- Reduce `App` struct field count by grouping related state

## Key Design Decisions

1. **Single package `main`**: All Wails handlers live in one package for simplicity
2. **Environment over config**: Workers configured via `ICICLE_SCAN_WORKERS` env var (now passed as parameter)
3. **Concurrent scanning**: IO-bound workers default to `CPU*2`, min 8, max 32
4. **Safe deletions**: Default is recycle bin, not permanent delete
5. **Single instance**: Mutex-based guard prevents multiple app launches
