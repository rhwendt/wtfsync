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

; Reads the user PATH into $0. $R7 is "ok", or "toolong" when PATH (plus our entry) doesn't fit
; in NSIS's ${NSIS_MAX_STRLEN}-character strings: ReadRegStr then returns "" or the edit would be
; cut short, and writing either back would wipe the user's other PATH entries. A missing PATH
; value also returns "", so `reg query` tells "missing" from "too long".
!macro ReadUserPath
  StrCpy $R7 ok
  ClearErrors
  ReadRegStr $0 HKCU "Environment" "Path"
  ${If} ${Errors}
    nsExec::Exec 'reg query HKCU\Environment /v Path'
    Pop $R8
    ${If} $R8 == 0
      StrCpy $R7 toolong
    ${EndIf}
  ${EndIf}
  StrLen $R8 $0
  StrLen $R6 $INSTDIR
  IntOp $R8 $R8 + $R6
  IntOp $R8 $R8 + 2 ; the ";" separators
  ${If} $R8 >= ${NSIS_MAX_STRLEN}
    StrCpy $R7 toolong
  ${EndIf}
!macroend

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

  !insertmacro ReadUserPath
  ${StrStr} $1 "$0;" "$INSTDIR;"
  ${If} $R7 == toolong
    ${If} ${SectionIsSelected} ${SecPath}
      MessageBox MB_ICONEXCLAMATION|MB_OK "Your PATH is too long for the installer to change safely, so it was left alone. To use the wtfsync command, add this folder to PATH yourself:$\r$\n$INSTDIR" /SD IDOK
    ${EndIf}
  ${ElseIf} ${SectionIsSelected} ${SecPath}
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
    StrCpy $3 $0
    !insertmacro ReadUserPath ; PATH → $0
    ${StrStr} $2 "$0;" "$3;"
    ${If} $R7 == ok
    ${AndIf} $2 == ""
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
  !insertmacro ReadUserPath
  ${UnStrStr} $1 "$0;" "$INSTDIR;"
  ${If} $R7 == toolong
    DetailPrint "PATH is too long to edit safely; remove $INSTDIR from it yourself."
  ${ElseIf} $1 != ""
    ${UnStrRep} $0 "$0;" "$INSTDIR;" ""
    StrCpy $0 $0 -1
    WriteRegExpandStr HKCU "Environment" "Path" "$0"
    SendMessage ${HWND_BROADCAST} ${WM_SETTINGCHANGE} 0 "STR:Environment" /TIMEOUT=5000
  ${EndIf}
  DeleteRegKey HKCU "${UNINSTKEY}"
SectionEnd
