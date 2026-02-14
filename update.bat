@echo off
setlocal

set "ROOT=%~dp0"
set "EXITCODE=0"

pushd "%ROOT%" >nul

where git >nul 2>nul
if errorlevel 1 (
  echo [icicle-update] git is not installed.
  set "EXITCODE=1"
  goto :end
)

if not exist ".git" (
  echo [icicle-update] .git folder not found. Run this script from repo root.
  set "EXITCODE=1"
  goto :end
)

for /f %%i in ('git rev-parse --short HEAD 2^>nul') do set "OLD_HEAD=%%i"
echo [icicle-update] pulling latest changes...
git pull --ff-only
if errorlevel 1 (
  echo [icicle-update] git pull failed.
  set "EXITCODE=1"
  goto :end
)
for /f %%i in ('git rev-parse --short HEAD 2^>nul') do set "NEW_HEAD=%%i"
if /I "%OLD_HEAD%"=="%NEW_HEAD%" (
  echo [icicle-update] already up to date: %NEW_HEAD%
) else (
  echo [icicle-update] updated: %OLD_HEAD% ^> %NEW_HEAD%
)

where go >nul 2>nul
if errorlevel 1 (
  echo [icicle-update] go is not installed, skipping build.
  goto :end
)

if exist "icicle.exe" (
  copy /y "icicle.exe" "icicle.exe.bak" >nul 2>nul
)
echo [icicle-update] building icicle.exe...
go build -trimpath -buildvcs=false -ldflags "-s -w -buildid=" -o icicle.exe ./cmd/icicle
if errorlevel 1 (
  echo [icicle-update] build failed.
  if exist "icicle.exe.bak" copy /y "icicle.exe.bak" "icicle.exe" >nul 2>nul
  set "EXITCODE=1"
  goto :end
)
if exist "icicle.exe.bak" del /f /q "icicle.exe.bak" >nul 2>nul
if exist "cmd\icicle-wails\main_windows.go" (
  if exist "icicle-desktop.exe" copy /y "icicle-desktop.exe" "icicle-desktop.exe.bak" >nul 2>nul
  echo [icicle-update] building icicle-desktop.exe...
  go build -trimpath -buildvcs=false -ldflags "-s -w -buildid=" -tags "wails,production" -o icicle-desktop.exe ./cmd/icicle-wails
  if errorlevel 1 (
    echo [icicle-update] desktop build failed.
    if exist "icicle-desktop.exe.bak" copy /y "icicle-desktop.exe.bak" "icicle-desktop.exe" >nul 2>nul
    set "EXITCODE=1"
    goto :end
  )
  if exist "icicle-desktop.exe.bak" del /f /q "icicle-desktop.exe.bak" >nul 2>nul
)
echo [icicle-update] done.

:end
popd >nul
if not "%EXITCODE%"=="0" (
  echo [icicle-update] failed with code %EXITCODE%.
)
exit /b %EXITCODE%
