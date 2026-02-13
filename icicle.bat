@echo off
setlocal

set "ROOT=%~dp0"
set "BIN=%ROOT%icicle.exe"
set "POST_UPDATE=%ROOT%scripts\post_update.bat"
set "LAUNCHER_CFG=%APPDATA%\icicle\launcher.env"
set "EXITCODE=0"
set "ARGS=%*"
set "UPDATED=0"
set "GIT_AUTO_UPDATE=1"

if "%~1"=="" (
  set "ARGS=gui"
)

if exist "%LAUNCHER_CFG%" (
  for /f "usebackq tokens=1,2 delims==" %%A in ("%LAUNCHER_CFG%") do (
    if /I "%%A"=="ICICLE_GIT_AUTO_UPDATE" set "GIT_AUTO_UPDATE=%%B"
  )
)

if /I "%ICICLE_NO_GIT_UPDATE%"=="1" (
  set "GIT_AUTO_UPDATE=0"
)

if "%GIT_AUTO_UPDATE%"=="1" (
  where git >nul 2>nul
  if not errorlevel 1 (
    pushd "%ROOT%" >nul
    if exist ".git" (
      for /f %%i in ('git rev-parse HEAD 2^>nul') do set "OLD_HEAD=%%i"
      git pull --ff-only
      if not errorlevel 1 (
        for /f %%i in ('git rev-parse HEAD 2^>nul') do set "NEW_HEAD=%%i"
        if not "%OLD_HEAD%"=="%NEW_HEAD%" (
          set "UPDATED=1"
          echo [icicle] updated from git: %OLD_HEAD% ^> %NEW_HEAD%
        )
      ) else (
        echo [icicle] git pull failed, continue with local version.
      )
    )
    popd >nul
  )
)

if "%UPDATED%"=="1" (
  if exist "%POST_UPDATE%" (
    call "%POST_UPDATE%"
  )
)

if exist "%BIN%" (
  "%BIN%" %ARGS%
  set "EXITCODE=%errorlevel%"
  goto :end
)

where go >nul 2>nul
if errorlevel 1 (
  echo [icicle] icicle.exe not found and Go is not installed.
  echo Build first: go build -o icicle.exe ./cmd/icicle
  set "EXITCODE=1"
  goto :end
)

pushd "%ROOT%" >nul
go build -o icicle.exe ./cmd/icicle
if errorlevel 1 (
  set "CODE=%errorlevel%"
  popd >nul
  set "EXITCODE=%CODE%"
  goto :end
)
icicle.exe %ARGS%
set "CODE=%errorlevel%"
popd >nul

set "EXITCODE=%CODE%"

:end
if not "%EXITCODE%"=="0" (
  echo [icicle] failed with code %EXITCODE%.
  pause
)
exit /b %EXITCODE%
