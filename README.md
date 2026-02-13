# icicle

`icicle` is a fast Windows-first CLI for disk hygiene.

Core commands:
- `icicle watch [path]`: watches a folder and auto-sorts new files by extension.
- `icicle heavy [path]`: shows top-N largest files.
- `icicle tree [path]`: prints a size tree with colored bars.

## Install

Requirements:
- Go 1.22+
- Windows 10/11

Build:

```powershell
go build -o icicle.exe ./cmd/icicle
```

Run:

```powershell
.\icicle.exe
.\icicle.exe gui
.\icicle.exe tree ~\Downloads
.\icicle.exe heavy --n 20 ~\
.\icicle.exe watch --dry-run ~\Downloads
```

If run without arguments, `icicle` opens a local GUI in your default browser.

## Usage

```text
icicle watch [--dry-run] [--no-color] [--no-emoji] [path]
icicle heavy [--n 20] [--no-color] [--no-emoji] [path]
icicle tree [--n 20] [--w 24] [--top 5] [--no-color] [--no-emoji] [path]
```

Defaults when `path` is omitted:
- `watch`: Windows Downloads folder (from User Shell Folders)
- `heavy`/`tree`: Windows Home folder

## Auto-sort map

`watch` moves new files into `~/Category` folders:
- Video: `.mp4`, `.mov`, `.mkv`, `.avi`, `.webm` -> `~/Videos`
- Archive: `.zip`, `.rar`, `.7z`, `.tar`, `.gz`, `.bz2`, `.xz` -> `~/Archives`
- Picture: `.jpg`, `.jpeg`, `.png`, `.gif`, `.webp`, `.bmp`, `.heic` -> `~/Pictures`
- Document: `.pdf`, `.doc`, `.docx`, `.txt`, `.md`, `.xls`, `.xlsx`, `.ppt`, `.pptx` -> `~/Documents`
- App: `.exe`, `.msi`, `.apk` -> `~/Apps`

Unknown extensions are skipped.

## Dev

```powershell
gofmt -w .
go test ./...
go vet ./...
go build ./cmd/icicle
```

## Release

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\release.ps1
```

## Update (Batch Script)

Use `update.bat` from repo root:

```powershell
.\update.bat
```

It does:
- `git pull --ff-only`
- rebuild `icicle.exe` (if Go is installed)

## Auto Update (Git Pull Launcher)

If you want update flow exactly via Git:

1. Keep app in a central Git repo (can include binaries like `icicle.exe`).
2. Users run `icicle.bat` (not `icicle.exe` directly).
3. On every start `icicle.bat` does:
   - `git pull --ff-only`
   - optional hook: `scripts/post_update.bat`
   - starts updated `icicle.exe`

Launcher setting is saved to `%APPDATA%\icicle\launcher.env`.

Notes:
- Disable pull check for one launch:

```powershell
$env:ICICLE_NO_GIT_UPDATE = "1"
.\icicle.bat
```

- If `icicle.exe` is missing, launcher auto-builds it from source (`go build`).

## Notes

- The watcher uses native file system notifications through `fsnotify`.
- Symlink directories are skipped to avoid loops.
- Name collisions are resolved with `name (N).ext`.
