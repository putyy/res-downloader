Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them with the values from ProjectInfo.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
##
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the ProjectInfo file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "MyProject" # Default "{{.Name}}"
## !define INFO_COMPANYNAME    "MyCompany" # Default "{{.Info.CompanyName}}"
## !define INFO_PRODUCTNAME    "MyProduct" # Default "{{.Info.ProductName}}"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "{{.Info.ProductVersion}}"
## !define INFO_COPYRIGHT      "Copyright" # Default "{{.Info.Copyright}}"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
####
## Include the wails tools
####
!include "wails_tools.nsh"
!include "safety.nsh"

; Generate and embed the fixed runtime's exact payload paths using the project's
; existing Go toolchain. The generator never runs on user machines.
!ifdef ARG_WEBVIEW2_FIXED_RUNTIME
    !ifndef RESD_GO
        !define RESD_GO "go"
    !endif
    !tempfile RESD_RUNTIME_FILE_LIST
    !system '"${RESD_GO}" run ./runtimefiles "${ARG_WEBVIEW2_FIXED_RUNTIME}" "${RESD_RUNTIME_FILE_LIST}"' = 0
    !include /CHARSET=UTF8 "${RESD_RUNTIME_FILE_LIST}"
    !delfile "${RESD_RUNTIME_FILE_LIST}"
!endif

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!define MUI_PAGE_CUSTOMFUNCTION_LEAVE resd.ValidateInstallDirectory
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uinstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
!ifdef ARG_WEBVIEW2_FIXED_RUNTIME
    OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-fixed-webview2-installer.exe"
!else
    OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe"
!endif
InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}" # Default installing folder ($PROGRAMFILES is Program Files folder).
ShowInstDetails show # This will always show the installation details.

Function .onInit
   !insertmacro wails.checkArchitecture
FunctionEnd

Function un.onInit
    Call un.resd.ValidateInstallation
FunctionEnd

Section
    !insertmacro wails.setShellContext
    ; Repeat the check here for silent installs and command-line /D overrides.
    Call resd.ValidateInstallDirectory
    Push "$INSTDIR\${PRODUCT_EXECUTABLE}"
    Call resd.RequirePlainPath
    Push "$INSTDIR\uninstall.exe"
    Call resd.RequirePlainPath
    Push "$INSTDIR\${RESD_INSTALL_MARKER}"
    Call resd.RequirePlainPath
    Push "$INSTDIR\${RESD_SYSTEM_WEBVIEW2_MARKER}"
    Call resd.RequirePlainPath
    Call resd.BeginInstallation

    !ifdef ARG_WEBVIEW2_FIXED_RUNTIME
        !insertmacro resd.RuntimePreflight ""
        !insertmacro resd.RecordRuntime
        SetOutPath "$INSTDIR\WebView2Runtime"
        ClearErrors
        File /r "${ARG_WEBVIEW2_FIXED_RUNTIME}\*.*"
        ${If} ${Errors}
            Abort "Could not install WebView2 Runtime files. Existing data has been kept."
        ${EndIf}

        # Fixed Version 120+ needs AppContainer read and execute permissions on Windows 10.
        nsExec::ExecToLog '"$SYSDIR\icacls.exe" "$INSTDIR\WebView2Runtime" /grant "*S-1-15-2-2:(OI)(CI)(RX)" /T /C /L /Q'
        Pop $0
        ${If} $0 != 0
            MessageBox MB_ICONSTOP|MB_OK "Failed to configure WebView2 Runtime permissions ($0). Installation will stop."
            Abort
        ${EndIf}
        nsExec::ExecToLog '"$SYSDIR\icacls.exe" "$INSTDIR\WebView2Runtime" /grant "*S-1-15-2-1:(OI)(CI)(RX)" /T /C /L /Q'
        Pop $0
        ${If} $0 != 0
            MessageBox MB_ICONSTOP|MB_OK "Failed to configure restricted WebView2 Runtime permissions ($0). Installation will stop."
            Abort
        ${EndIf}
    !else
        !insertmacro wails.webview2runtime

        # Wails' WebView2 macro does not validate the Bootstrapper exit code.
        # Verify the machine-level runtime before installing the application.
        SetRegView 64
        ReadRegStr $0 HKLM "SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
        ${If} $0 == ""
            MessageBox MB_ICONSTOP|MB_OK "WebView2 Runtime installation failed. Check the network or use the Fixed WebView2 release package."
            Abort
        ${EndIf}
    !endif

    SetOutPath "$INSTDIR"

    ClearErrors
    !insertmacro wails.files
    ${If} ${Errors}
        Abort "Could not install application files. Close res-downloader and retry."
    ${EndIf}

    ; Select the runtime explicitly. Legacy untracked payloads may remain, but
    ; must not override the system runtime selected by the standard installer.
    ClearErrors
    !ifdef ARG_WEBVIEW2_FIXED_RUNTIME
        Delete "$INSTDIR\${RESD_SYSTEM_WEBVIEW2_MARKER}"
    !else
        FileOpen $0 "$INSTDIR\${RESD_SYSTEM_WEBVIEW2_MARKER}" w
        ${If} ${Errors}
            Abort "Could not select the system WebView2 Runtime. Close res-downloader and retry."
        ${EndIf}
        FileClose $0
    !endif
    ${If} ${Errors}
        Abort "Could not save the WebView2 Runtime selection. Close res-downloader and retry."
    ${EndIf}
    !ifndef ARG_WEBVIEW2_FIXED_RUNTIME
        Call resd.RemoveOwnedRuntime
    !endif

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    ClearErrors
    !insertmacro wails.writeUninstaller
    WriteINIStr "$INSTDIR\${RESD_INSTALL_MARKER}" "Installation" "Product" "${UNINST_KEY_NAME}"
    WriteINIStr "$INSTDIR\${RESD_INSTALL_MARKER}" "Installation" "Location" "$INSTDIR"
    WriteINIStr "$INSTDIR\${RESD_INSTALL_MARKER}" "Installation" "Schema" "1"
    WriteRegStr HKLM "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
    ${If} ${Errors}
        Abort "Could not save the installation record. Reinstall before attempting to uninstall."
    ${EndIf}
    DeleteRegValue HKLM "${RESD_INSTALL_STATE_KEY}\Pending" "$INSTDIR"
    DeleteRegKey /ifempty HKLM "${RESD_INSTALL_STATE_KEY}\Pending"
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext
    ; Validate again immediately before removal, including silent uninstall.
    Call un.resd.ValidateInstallation
    Push "$INSTDIR\${PRODUCT_EXECUTABLE}"
    Call un.resd.RequirePlainPath
    Push "$INSTDIR\uninstall.exe"
    Call un.resd.RequirePlainPath
    Push "$INSTDIR\${RESD_SYSTEM_WEBVIEW2_MARKER}"
    Call un.resd.RequirePlainPath
    !ifdef ARG_WEBVIEW2_FIXED_RUNTIME
        !insertmacro resd.RuntimePreflight "un."
    !endif

    ; Keep downloaded media, configuration, plugins and WebView2 profile data.
    ; Only package-owned files are removed; directories must already be empty.
    SetOutPath "$TEMP"
    StrCpy $resdRemovalFailed 0
    !insertmacro resd.DeleteFile "$INSTDIR\${PRODUCT_EXECUTABLE}"
    Call un.resd.CheckRemoval
    Call un.resd.RemoveOwnedRuntime
    !ifdef ARG_WEBVIEW2_FIXED_RUNTIME
        !insertmacro resd.RemoveRuntime
    !endif
    Call un.resd.CheckRemoval

    !insertmacro resd.DeleteFile "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    !insertmacro resd.DeleteFile "$DESKTOP\${INFO_PRODUCTNAME}.lnk"
    !insertmacro resd.DeleteFile "$INSTDIR\${RESD_SYSTEM_WEBVIEW2_MARKER}"
    Call un.resd.CheckRemoval

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro resd.DeleteFile "$INSTDIR\${RESD_INSTALL_MARKER}"
    Call un.resd.CheckRemoval
    !insertmacro resd.DeleteFile "$INSTDIR\uninstall.exe"
    ; Restore the marker if the uninstaller could not be removed, so retry works.
    ${If} $resdRemovalFailed != 0
        WriteINIStr "$INSTDIR\${RESD_INSTALL_MARKER}" "Installation" "Product" "${UNINST_KEY_NAME}"
        WriteINIStr "$INSTDIR\${RESD_INSTALL_MARKER}" "Installation" "Location" "$INSTDIR"
        WriteINIStr "$INSTDIR\${RESD_INSTALL_MARKER}" "Installation" "Schema" "1"
        Call un.resd.CheckRemoval
    ${EndIf}
    DeleteRegKey HKLM "${UNINST_KEY}"
    DeleteRegValue HKLM "${RESD_INSTALL_STATE_KEY}\Pending" "$INSTDIR"
    DeleteRegKey /ifempty HKLM "${RESD_INSTALL_STATE_KEY}\Pending"
    !insertmacro resd.RemoveEmptyDirectory "$INSTDIR"
SectionEnd
