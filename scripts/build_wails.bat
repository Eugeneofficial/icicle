@echo off
setlocal
cd /d "%~dp0\.."

where go >nul 2>nul
if errorlevel 1 (
  echo [icicle] Go is not installed.
  exit /b 1
)

echo [icicle] building desktop (Wails, local assets)...
go build -tags "wails,production" -o icicle-desktop.exe ./cmd/icicle-wails
if errorlevel 1 exit /b 1

echo [icicle] done: icicle-desktop.exe
exit /b 0
