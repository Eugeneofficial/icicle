@echo off
setlocal

set "ROOT=%~dp0"
set "P_DATA=%ROOT%portable-data"
set "APPDATA=%P_DATA%\AppData\Roaming"
set "LOCALAPPDATA=%P_DATA%\AppData\Local"
set "TEMP=%P_DATA%\Temp"
set "TMP=%P_DATA%\Temp"

if not exist "%APPDATA%" mkdir "%APPDATA%" >nul 2>nul
if not exist "%LOCALAPPDATA%" mkdir "%LOCALAPPDATA%" >nul 2>nul
if not exist "%TEMP%" mkdir "%TEMP%" >nul 2>nul

call "%ROOT%icicle.bat" %*
exit /b %errorlevel%
