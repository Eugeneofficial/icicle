# Benchmarks

This page tracks practical performance for major flows.

## Method

- OS: Windows 11 (x64)
- CPU: all
- Storage: SSD/NVMe
- Build: `go build -tags "wails,production"`
- Command format:
  - `icicle heavy --n 20 <path>`
  - `icicle tree <path>`

## Sample Results (v2.0.0)

| Scenario | Mode | Result |
|---|---|---:|
| `heavy C:\` | fast (`maxFiles=220000`) | ~0.7-0.9s |
| `heavy C:\` | full | depends on full tree size |
| `tree C:\` | full | several seconds on large systems |

## Interpretation

- Fast mode is optimized for interactive UI loops.
- Full mode is optimized for accuracy.
- For huge folders, tune workers in desktop UI (`16-32` typical).

## Reproduce Locally

```powershell
$env:ICICLE_SCAN_WORKERS='24'
.\icicle.exe heavy --n 20 C:\
.\icicle.exe tree C:\
```

