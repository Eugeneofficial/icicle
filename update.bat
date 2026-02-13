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
  del /f /q "icicle.exe" >nul 2>nul
)
echo [icicle-update] building icicle.exe...
go build -o icicle.exe ./cmd/icicle
if errorlevel 1 (
  echo [icicle-update] build failed.
  set "EXITCODE=1"
  goto :end
)
echo [icicle-update] done.

:end
popd >nul
if not "%EXITCODE%"=="0" (
  echo [icicle-update] failed with code %EXITCODE%.
)
exit /b %EXITCODE%
