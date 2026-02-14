!define APP_NAME "icicle"
!ifndef VERSION
  !define VERSION "2.3.0"
!endif
OutFile "..\\dist\\icicle-${VERSION}-setup.exe"
InstallDir "$PROGRAMFILES64\\icicle"
RequestExecutionLevel admin

Page directory
Page instfiles

Section "Install"
  SetOutPath "$INSTDIR"
  File "..\\build\\bin\\icicle-desktop.exe"
  CreateShortcut "$DESKTOP\\icicle.lnk" "$INSTDIR\\icicle-desktop.exe"
SectionEnd
