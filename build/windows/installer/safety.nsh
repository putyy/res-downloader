!ifndef RESD_INSTALLER_SAFETY
!define RESD_INSTALLER_SAFETY

!include "LogicLib.nsh"
!if "${REQUEST_EXECUTION_LEVEL}" != "admin"
    !error "res-downloader installer safety checks require the machine-wide install scope"
!endif
!ifdef WAILS_INSTALL_SCOPE
    !if "${WAILS_INSTALL_SCOPE}" == "user"
        !error "res-downloader installer safety checks require the machine-wide install scope"
    !endif
!endif
!define RESD_INSTALL_MARKER ".res-downloader-install.ini"
!define RESD_SYSTEM_WEBVIEW2_MARKER ".res-downloader-system-webview2"
; Machine-owned records must not come from a manifest in a user-writable folder.
!define RESD_INSTALL_STATE_KEY "Software\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}\Installer"
!define RESD_RUNTIME_FILES_KEY "${RESD_INSTALL_STATE_KEY}\RuntimeFiles\$INSTDIR"
Var resdRemovalFailed

; Both installer and uninstaller reject directory junctions/symlinks, including
; any ancestor. Missing paths are allowed; other attribute errors fail closed.
!macro resd.PathFunctions PREFIX
Function ${PREFIX}resd.PlainPath
    Exch $0
    Push $1
    Push $2
    Push $3
    StrCpy $1 $0 2 1
    ${If} $1 != ":\"
        StrCpy $0 ""
        Goto done
    ${EndIf}
    GetFullPathName $0 "$0"
    StrLen $2 $0
    ${If} $2 > 3
        StrCpy $1 $0 1 -1
        ${If} $1 == "\"
            StrCpy $0 $0 -1
        ${EndIf}
    ${EndIf}
    StrCpy $1 $0
    loop:
        System::Call 'kernel32::GetFileAttributesW(w r1) i .r2 ?e'
        Pop $3
        ${If} $2 == -1
            ${If} $3 != 2
            ${AndIf} $3 != 3
                StrCpy $0 ""
                Goto done
            ${EndIf}
        ${Else}
            IntOp $2 $2 & 0x400 ; FILE_ATTRIBUTE_REPARSE_POINT
            ${If} $2 != 0
                StrCpy $0 ""
                Goto done
            ${EndIf}
        ${EndIf}
        StrLen $2 $1
        ${If} $2 <= 3
            Goto done
        ${EndIf}
        ${GetParent} "$1" $1
        ; GetParent returns C: for C:\child; keep the drive root absolute.
        StrLen $2 $1
        ${If} $2 == 2
            StrCpy $1 "$1\"
        ${EndIf}
        Goto loop
    done:
    Pop $3
    Pop $2
    Pop $1
    Exch $0
FunctionEnd

Function ${PREFIX}resd.InstallDirectory
    Exch $0
    Push $1
    Push $2
    Push "$0"
    Call ${PREFIX}resd.PlainPath
    Pop $0
    StrLen $1 $0
    ${If} $1 <= 3
        Goto invalid
    ${EndIf}
    ; Leave room for the application's fixed file names in NSIS string buffers.
    ${If} $1 > 900
        Goto invalid
    ${EndIf}
    ; Never accept Windows or any of its descendants.
    StrLen $1 "$WINDIR"
    StrCpy $2 $0 $1
    ${If} $2 == "$WINDIR"
        StrCpy $2 $0 1 $1
        ${If} $2 == ""
        ${OrIf} $2 == "\"
            Goto invalid
        ${EndIf}
    ${EndIf}
    ${If} $0 == "$PROGRAMFILES"
    ${OrIf} $0 == "$PROGRAMFILES64"
    ${OrIf} $0 == "$COMMONFILES"
    ${OrIf} $0 == "$COMMONFILES64"
    ${OrIf} $0 == "$SYSDIR"
    ${OrIf} $0 == "$PROFILE"
    ${OrIf} $0 == "$APPDATA"
    ${OrIf} $0 == "$LOCALAPPDATA"
    ${OrIf} $0 == "$DESKTOP"
    ${OrIf} $0 == "$DOCUMENTS"
    ${OrIf} $0 == "$SMPROGRAMS"
    ${OrIf} $0 == "$STARTMENU"
    ${OrIf} $0 == "$SMSTARTUP"
    ${OrIf} $0 == "$TEMP"
        Goto invalid
    ${EndIf}
    ; Also reject the current user's shell roots, since the installer normally
    ; runs with the all-users shell context.
    SetShellVarContext current
    ${If} $0 == "$APPDATA"
    ${OrIf} $0 == "$LOCALAPPDATA"
    ${OrIf} $0 == "$DESKTOP"
    ${OrIf} $0 == "$DOCUMENTS"
    ${OrIf} $0 == "$SMPROGRAMS"
    ${OrIf} $0 == "$STARTMENU"
    ${OrIf} $0 == "$SMSTARTUP"
        SetShellVarContext all
        Goto invalid
    ${EndIf}
    SetShellVarContext all
    Goto done
    invalid:
        StrCpy $0 ""
    done:
    Pop $2
    Pop $1
    Exch $0
FunctionEnd

Function ${PREFIX}resd.RequirePlainPath
    Exch $0
    Push "$0"
    Call ${PREFIX}resd.PlainPath
    Pop $0
    ${If} $0 == ""
        MessageBox MB_ICONSTOP|MB_OK "Unsafe or inaccessible path. No directory links are allowed. Please reinstall into a regular application folder." /SD IDOK
        SetErrorLevel 2
        Abort
    ${EndIf}
    Pop $0
FunctionEnd
!macroend

!insertmacro resd.PathFunctions ""
!insertmacro resd.PathFunctions "un."

Function resd.ValidateInstallDirectory
    !insertmacro wails.setShellContext
    Push "$INSTDIR"
    Call resd.InstallDirectory
    Pop $0
    ${If} $0 == ""
        Goto invalid
    ${EndIf}
    StrCpy $INSTDIR $0
    ; Empty directories are safe for a new install. Existing installations must
    ; match the registry, including legacy installations without our marker.
    System::Call 'kernel32::GetFileAttributesW(w "$INSTDIR") i .r0 ?e'
    Pop $1
    ${If} $0 == -1
        ${If} $1 == 2
        ${OrIf} $1 == 3
            Return
        ${EndIf}
        Goto invalid
    ${EndIf}
    ClearErrors
    FindFirst $0 $1 "$INSTDIR\*"
    ${If} ${Errors}
        Goto invalid
    ${EndIf}
    loop:
        ${If} $1 == ""
            FindClose $0
            Return
        ${EndIf}
        ${If} $1 != "."
        ${AndIf} $1 != ".."
            FindClose $0
            SetRegView 64
            ReadRegStr $0 HKLM "${UNINST_KEY}" "UninstallString"
            ReadRegStr $1 HKLM "${UNINST_KEY}" "DisplayIcon"
            ${If} $0 == '$\"$INSTDIR\uninstall.exe$\"'
            ${AndIf} $1 == "$INSTDIR\${PRODUCT_EXECUTABLE}"
                Return
            ${EndIf}
            ReadRegStr $0 HKLM "${RESD_INSTALL_STATE_KEY}\Pending" "$INSTDIR"
            ${If} $0 == "1"
                Return
            ${EndIf}
            Goto invalid
        ${EndIf}
        FindNext $0 $1
        Goto loop
    invalid:
        MessageBox MB_ICONSTOP|MB_OK "Choose an empty application folder or the registered res-downloader installation folder. System folders and directory links are not allowed." /SD IDOK
        SetErrorLevel 2
        Abort
FunctionEnd

Function resd.BeginInstallation
    ; Called only after validating the destination, before writing any payload.
    ; Keep this separate from the Windows uninstall entry until installation
    ; succeeds. A failed first install can then be retried in the same directory.
    SetRegView 64
    ClearErrors
    WriteRegStr HKLM "${RESD_INSTALL_STATE_KEY}\Pending" "$INSTDIR" "1"
    ${If} ${Errors}
        MessageBox MB_ICONSTOP|MB_OK "Could not record the installation destination. No application files have been written." /SD IDOK
        SetErrorLevel 2
        Abort
    ${EndIf}
FunctionEnd

; Each value name is a relative path generated from an installed fixed payload;
; its data is F (file) or D (directory). Records accumulate across fixed upgrades
; and are shared by both installer flavours and their uninstallers.
!macro resd.OwnedRuntimeFunctions PREFIX
Function ${PREFIX}resd.RemoveOwnedRuntime
    SetRegView 64
    loop:
        ClearErrors
        EnumRegValue $1 HKLM "${RESD_RUNTIME_FILES_KEY}" 0
        ${If} ${Errors}
            ; A missing key is normal for standard or legacy installations.
            ; Never erase a non-empty record if enumeration was unsuccessful.
            DeleteRegKey /ifempty HKLM "${RESD_RUNTIME_FILES_KEY}"
            Return
        ${EndIf}
        ReadRegStr $5 HKLM "${RESD_RUNTIME_FILES_KEY}" "$1"
        ${If} $5 != "F"
        ${AndIf} $5 != "D"
            Goto failed
        ${EndIf}
        StrLen $3 "$INSTDIR\WebView2Runtime\"
        StrLen $4 $1
        IntOp $3 $3 + $4
        IntOp $3 $3 + 1 ; string terminator
        ${If} $3 > ${NSIS_MAX_STRLEN}
            Goto failed
        ${EndIf}
        ; Even a damaged registry record must never turn Delete into a glob.
        StrCpy $3 0
        characters:
            StrCpy $4 $1 1 $3
            ${If} $4 == "*"
            ${OrIf} $4 == "?"
                Goto failed
            ${EndIf}
            ${If} $4 != ""
                IntOp $3 $3 + 1
                Goto characters
            ${EndIf}
        StrCpy $2 "$INSTDIR\WebView2Runtime\$1"
        GetFullPathName $3 "$2"
        ${If} $3 != "$2"
            Goto failed ; rejects absolute paths, . and .. components
        ${EndIf}
        Push "$2"
        Call ${PREFIX}resd.PlainPath
        Pop $3
        ${If} $3 == ""
            Goto failed
        ${EndIf}
        ${If} $5 == "F"
            ClearErrors
            Delete "$2"
            ${If} ${Errors}
                Goto failed
            ${EndIf}
            ${GetParent} "$2" $2
        ${EndIf}
        ; Empty owned directories can go; unknown files and their parents stay.
        parents:
            ${If} $2 == "$INSTDIR"
                Goto recorded
            ${EndIf}
            Push "$2"
            Call ${PREFIX}resd.PlainPath
            Pop $3
            ${If} $3 == ""
                Goto failed
            ${EndIf}
            RMDir "$2"
            ${GetParent} "$2" $2
            Goto parents
        recorded:
            ClearErrors
            DeleteRegValue HKLM "${RESD_RUNTIME_FILES_KEY}" "$1"
            ${If} ${Errors}
                Goto failed
            ${EndIf}
            Goto loop
    failed:
        MessageBox MB_ICONSTOP|MB_OK "Could not remove previously installed WebView2 Runtime files. Close res-downloader and retry. Remaining files and their installation records have been kept." /SD IDOK
        SetErrorLevel 2
        Abort
FunctionEnd
!macroend

!ifndef ARG_WEBVIEW2_FIXED_RUNTIME
    !insertmacro resd.OwnedRuntimeFunctions ""
!endif
!insertmacro resd.OwnedRuntimeFunctions "un."

Function un.resd.ValidateInstallation
    !insertmacro wails.setShellContext
    Push "$INSTDIR"
    Call un.resd.InstallDirectory
    Pop $0
    ${If} $0 == ""
        Goto invalid
    ${EndIf}
    StrCpy $INSTDIR $0
    SetRegView 64
    ReadRegStr $0 HKLM "${UNINST_KEY}" "InstallLocation"
    ${If} $0 != "$INSTDIR"
        Goto invalid
    ${EndIf}
    ReadRegStr $0 HKLM "${UNINST_KEY}" "UninstallString"
    ${If} $0 != '$\"$INSTDIR\uninstall.exe$\"'
        Goto invalid
    ${EndIf}
    Push "$INSTDIR\${RESD_INSTALL_MARKER}"
    Call un.resd.RequirePlainPath
    ReadINIStr $0 "$INSTDIR\${RESD_INSTALL_MARKER}" "Installation" "Product"
    ReadINIStr $1 "$INSTDIR\${RESD_INSTALL_MARKER}" "Installation" "Location"
    ReadINIStr $2 "$INSTDIR\${RESD_INSTALL_MARKER}" "Installation" "Schema"
    ${If} $0 != "${UNINST_KEY_NAME}"
    ${OrIf} $1 != "$INSTDIR"
    ${OrIf} $2 != "1"
        Goto invalid
    ${EndIf}
    Return
    invalid:
        MessageBox MB_ICONSTOP|MB_OK "The installation record is missing or does not match this folder. Uninstall has stopped without deleting files. Reinstall res-downloader to repair its installation record, then uninstall from Windows Settings." /SD IDOK
        SetErrorLevel 2
        Abort
FunctionEnd

; Only compile-time literal file names may be passed to these macros. No on-disk
; deletion manifest is trusted, and a changed link is never followed.
!macro resd.DeleteFile PATH
    Push "${PATH}"
    Call un.resd.PlainPath
    Pop $0
    ${If} $0 == ""
        StrCpy $resdRemovalFailed 1
    ${Else}
        ClearErrors
        Delete "${PATH}"
        ${If} ${Errors}
            StrCpy $resdRemovalFailed 1
        ${EndIf}
    ${EndIf}
!macroend

!macro resd.RemoveEmptyDirectory PATH
    Push "${PATH}"
    Call un.resd.PlainPath
    Pop $0
    ${If} $0 != ""
        RMDir "${PATH}"
    ${EndIf}
!macroend

Function un.resd.CheckRemoval
    ${If} $resdRemovalFailed != 0
        MessageBox MB_ICONSTOP|MB_OK "Some application files could not be removed. Close res-downloader and retry. Installation records have been kept so you can retry safely." /SD IDOK
        SetErrorLevel 2
        Abort
    ${EndIf}
FunctionEnd

!endif
