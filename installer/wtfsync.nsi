; wtfsync installer: per-user (no admin), built on Linux with
;   makensis -NOCD -DVERSION=v1.2.3 -DBINDIR=dist -DOUTFILE=dist/wtfsync-setup.exe installer/wtfsync.nsi
; (-NOCD: paths are relative to the repo root, not this folder.)
; BINDIR must hold wtfsync.exe and wtfsync-tray.exe. /S installs silently (used by the tray's
; one-click update) and keeps the current "start at login" and PATH choices.

Unicode true
SetCompressor /SOLID lzma
RequestExecutionLevel user

!ifndef VERSION
  !define VERSION dev
!endif
!ifndef BINDIR
  !define BINDIR dist
!endif
!ifndef OUTFILE
  !define OUTFILE wtfsync-setup.exe
!endif

!define UNINSTKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\wtfsync"
!define RUNKEY "Software\Microsoft\Windows\CurrentVersion\Run"

Name "wtfsync ${VERSION}"
OutFile "${OUTFILE}"
InstallDir "$LOCALAPPDATA\Programs\wtfsync"
InstallDirRegKey HKCU "${UNINSTKEY}" "InstallLocation"

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "Sections.nsh"
!include "StrFunc.nsh"
!include "WinMessages.nsh"
${StrStr}
${StrRep}
${UnStrStr}
${UnStrRep}

!define MUI_COMPONENTSPAGE_NODESC
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_RUN "$INSTDIR\wtfsync-tray.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Start wtfsync now"
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

; Ask the running tray to quit, then wait up to 10 s; force it after that.
!macro StopTray
  IfFileExists "$INSTDIR\wtfsync-tray.exe" 0 +2
    ExecWait '"$INSTDIR\wtfsync-tray.exe" --quit'
  StrCpy $R9 0
  ${Do}
    nsExec::Exec 'cmd /c tasklist /FI "IMAGENAME eq wtfsync-tray.exe" /NH | find /I "wtfsync-tray.exe"'
    Pop $R8
    ${If} $R8 != 0
      ${Break}
    ${EndIf}
    ${If} $R9 >= 20
      nsExec::Exec 'taskkill /F /IM wtfsync-tray.exe'
      Pop $R8
      Sleep 500
      ${Break}
    ${EndIf}
    Sleep 500
    IntOp $R9 $R9 + 1
  ${Loop}
!macroend

Section "wtfsync" SecMain
  SectionIn RO
  !insertmacro StopTray
  SetOutPath "$INSTDIR"
  File "${BINDIR}/wtfsync.exe"
  File "${BINDIR}/wtfsync-tray.exe"
  WriteUninstaller "$INSTDIR\uninstall.exe"
  CreateShortcut "$SMPROGRAMS\wtfsync.lnk" "$INSTDIR\wtfsync-tray.exe"

  WriteRegStr HKCU "${UNINSTKEY}" "DisplayName" "wtfsync"
  WriteRegStr HKCU "${UNINSTKEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINSTKEY}" "Publisher" "wtfsync"
  WriteRegStr HKCU "${UNINSTKEY}" "DisplayIcon" "$INSTDIR\wtfsync-tray.exe"
  WriteRegStr HKCU "${UNINSTKEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTKEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKCU "${UNINSTKEY}" "QuietUninstallString" '"$INSTDIR\uninstall.exe" /S'
  WriteRegDWORD HKCU "${UNINSTKEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTKEY}" "NoRepair" 1
  WriteRegDWORD HKCU "${UNINSTKEY}" "EstimatedSize" 40000
SectionEnd

Section "Start wtfsync when Windows starts" SecAutostart
SectionEnd

Section "Add the wtfsync command to PATH" SecPath
SectionEnd

; Applies the two options (selected or not) after the files are in place.
Section "-apply"
  ${If} ${SectionIsSelected} ${SecAutostart}
    WriteRegStr HKCU "${RUNKEY}" "wtfsync" '"$INSTDIR\wtfsync-tray.exe"'
  ${Else}
    DeleteRegValue HKCU "${RUNKEY}" "wtfsync"
  ${EndIf}

  ReadRegStr $0 HKCU "Environment" "Path"
  ${StrStr} $1 "$0;" "$INSTDIR;"
  ${If} ${SectionIsSelected} ${SecPath}
    ${If} $1 == ""
      ${If} $0 == ""
        WriteRegExpandStr HKCU "Environment" "Path" "$INSTDIR"
      ${Else}
        WriteRegExpandStr HKCU "Environment" "Path" "$0;$INSTDIR"
      ${EndIf}
    ${EndIf}
  ${ElseIf} $1 != ""
    ${StrRep} $0 "$0;" "$INSTDIR;" ""
    StrCpy $0 $0 -1 ; drop the trailing ";" added for the search
    WriteRegExpandStr HKCU "Environment" "Path" "$0"
  ${EndIf}
  SendMessage ${HWND_BROADCAST} ${WM_SETTINGCHANGE} 0 "STR:Environment" /TIMEOUT=5000

  ${If} ${Silent}
    Exec '"$INSTDIR\wtfsync-tray.exe"' ; a silent update restarts the tray itself
  ${EndIf}
SectionEnd

; On an upgrade, start from the current choices instead of the defaults.
Function .onInit
  ReadRegStr $0 HKCU "${UNINSTKEY}" "InstallLocation"
  ${If} $0 != ""
    ReadRegStr $1 HKCU "${RUNKEY}" "wtfsync"
    ${If} $1 == ""
      !insertmacro UnselectSection ${SecAutostart}
    ${EndIf}
    ReadRegStr $1 HKCU "Environment" "Path"
    ${StrStr} $2 "$1;" "$0;"
    ${If} $2 == ""
      !insertmacro UnselectSection ${SecPath}
    ${EndIf}
  ${EndIf}
FunctionEnd

; Removes the programs, shortcut, startup entry and PATH entry. Keeps %APPDATA%\wtfsync
; (settings, sync state, backups) and the data in the cloud folder.
Section "Uninstall"
  !insertmacro StopTray
  Delete "$INSTDIR\wtfsync.exe"
  Delete "$INSTDIR\wtfsync-tray.exe"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\wtfsync.lnk"
  DeleteRegValue HKCU "${RUNKEY}" "wtfsync"
  ReadRegStr $0 HKCU "Environment" "Path"
  ${UnStrStr} $1 "$0;" "$INSTDIR;"
  ${If} $1 != ""
    ${UnStrRep} $0 "$0;" "$INSTDIR;" ""
    StrCpy $0 $0 -1
    WriteRegExpandStr HKCU "Environment" "Path" "$0"
    SendMessage ${HWND_BROADCAST} ${WM_SETTINGCHANGE} 0 "STR:Environment" /TIMEOUT=5000
  ${EndIf}
  DeleteRegKey HKCU "${UNINSTKEY}"
SectionEnd
