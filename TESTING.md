# Testing

## Automated

```powershell
go test ./...
go vet ./...
```

## Build Smoke

```powershell
go build -o icicle.exe ./cmd/icicle
.\icicle.exe help
.\icicle.exe tree --top 3 .
.\icicle.exe heavy --n 5 .
```

## GUI Smoke

1. Run `.\icicle.exe`
2. Click `Run Heavy` and `Run Tree`
3. Open heavy actions panel and test:
- Move
- Safe delete (Recycle Bin)
- Reveal in Explorer
4. Switch RU/EN and Light/Dark
5. Test watch on Downloads with dry-run

## Performance Checks (Large Folders)

- Enable `Fast scan`
- Set `Max files` between `120000` and `300000`
- Verify heavy list appears quickly and meta pill shows scanned/cached source

## Release Checks

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\release.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\portable.ps1
```
