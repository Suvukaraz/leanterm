Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows you to use this project.nsi manually
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
## The following information is taken from the wails_tools.nsh file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "my-project" # Default "LeanTerm"
## !define INFO_COMPANYNAME    "My Company" # Default "My Company"
## !define INFO_PRODUCTNAME    "My Product Name" # Default "LeanTerm"
## !define INFO_PRODUCTVERSION "1.2.0"     # Default "0.1.0"
## !define INFO_COPYRIGHT      "(c) Now, My Company" # Default "© 2026, My Company"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
## !define WAILS_INSTALL_SCOPE     "user"             # Default "machine" - set to "user" for per-user install ($LOCALAPPDATA) without UAC prompt
####
## Include the wails tools
####
!include "wails_tools.nsh"

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
!define MUI_COMPONENTSPAGE_SMALLDESC
!insertmacro MUI_PAGE_COMPONENTS # Optional parts (LeanEdit).
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uninstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
!ifndef OUTFILE
    !define OUTFILE "..\..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe"
!endif
OutFile "${OUTFILE}" # Name of the installer's file.
!if "${WAILS_INSTALL_SCOPE}" == "user"
    InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
!else
    InstallDir "$PROGRAMFILES64\${INFO_PRODUCTNAME}"
!endif
ShowInstDetails show # This will always show the installation details.

Function .onInit
   !insertmacro wails.checkArchitecture
FunctionEnd

Section "LeanTerm" SecLeanTerm
    SectionIn RO # always installed
    !insertmacro wails.setShellContext

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR
    
    !insertmacro wails.files
    File "..\leanedit.ico"

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols
    
    !insertmacro wails.writeUninstaller
SectionEnd

####
## LeanEdit: LeanTerm's text editor on its own (LeanTerm.exe --leanedit). It runs
## independently of LeanTerm, so it gets its own shortcuts and is offered in
## Explorer's "Open with" for text files.
####
!define LEANEDIT_PROGID "LeanTerm.LeanEdit"
!define LEANEDIT_CMD '"$INSTDIR\${PRODUCT_EXECUTABLE}" --leanedit'

!macro leanedit.openWith EXT
    WriteRegStr SHCTX "Software\Classes\${EXT}\OpenWithProgids" "${LEANEDIT_PROGID}" ""
!macroend
!macro leanedit.removeOpenWith EXT
    DeleteRegValue SHCTX "Software\Classes\${EXT}\OpenWithProgids" "${LEANEDIT_PROGID}"
!macroend
# Text file types offered "Open with LeanEdit" (the list is used for install and uninstall).
!macro leanedit.forEachExtension MACRO
    !insertmacro ${MACRO} ".txt"
    !insertmacro ${MACRO} ".log"
    !insertmacro ${MACRO} ".md"
    !insertmacro ${MACRO} ".ini"
    !insertmacro ${MACRO} ".cfg"
    !insertmacro ${MACRO} ".conf"
    !insertmacro ${MACRO} ".config"
    !insertmacro ${MACRO} ".json"
    !insertmacro ${MACRO} ".xml"
    !insertmacro ${MACRO} ".yaml"
    !insertmacro ${MACRO} ".yml"
    !insertmacro ${MACRO} ".toml"
    !insertmacro ${MACRO} ".csv"
    !insertmacro ${MACRO} ".properties"
    !insertmacro ${MACRO} ".env"
    !insertmacro ${MACRO} ".sql"
    !insertmacro ${MACRO} ".sh"
    !insertmacro ${MACRO} ".ps1"
    !insertmacro ${MACRO} ".bat"
    !insertmacro ${MACRO} ".cmd"
    !insertmacro ${MACRO} ".py"
    !insertmacro ${MACRO} ".js"
    !insertmacro ${MACRO} ".ts"
    !insertmacro ${MACRO} ".go"
    !insertmacro ${MACRO} ".c"
    !insertmacro ${MACRO} ".h"
    !insertmacro ${MACRO} ".cpp"
    !insertmacro ${MACRO} ".cs"
    !insertmacro ${MACRO} ".java"
    !insertmacro ${MACRO} ".css"
    !insertmacro ${MACRO} ".html"
    !insertmacro ${MACRO} ".htm"
!macroend

Section "LeanEdit (text editor)" SecLeanEdit
    !insertmacro wails.setShellContext

    CreateShortcut "$SMPROGRAMS\LeanEdit.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}" "--leanedit" "$INSTDIR\leanedit.ico" 0 "" "" "LeanEdit text editor"
    CreateShortcut "$DESKTOP\LeanEdit.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}" "--leanedit" "$INSTDIR\leanedit.ico" 0 "" "" "LeanEdit text editor"

    # ProgID used by "Open with": name and icon of LeanEdit, opens the file in it.
    WriteRegStr SHCTX "Software\Classes\${LEANEDIT_PROGID}" "" "Text Document (LeanEdit)"
    WriteRegStr SHCTX "Software\Classes\${LEANEDIT_PROGID}" "FriendlyTypeName" "Text Document (LeanEdit)"
    WriteRegStr SHCTX "Software\Classes\${LEANEDIT_PROGID}\DefaultIcon" "" "$INSTDIR\leanedit.ico"
    WriteRegStr SHCTX "Software\Classes\${LEANEDIT_PROGID}\Application" "ApplicationName" "LeanEdit"
    WriteRegStr SHCTX "Software\Classes\${LEANEDIT_PROGID}\Application" "ApplicationDescription" "LeanEdit text editor (LeanTerm)"
    WriteRegStr SHCTX "Software\Classes\${LEANEDIT_PROGID}\Application" "ApplicationIcon" "$INSTDIR\leanedit.ico"
    WriteRegStr SHCTX "Software\Classes\${LEANEDIT_PROGID}\shell\open" "FriendlyAppName" "LeanEdit"
    WriteRegStr SHCTX "Software\Classes\${LEANEDIT_PROGID}\shell\open\command" "" '${LEANEDIT_CMD} "%1"'
    !insertmacro leanedit.forEachExtension leanedit.openWith
    # Any other file Windows considers text (perceived type "text").
    WriteRegStr SHCTX "Software\Classes\SystemFileAssociations\text\OpenWithProgids" "${LEANEDIT_PROGID}" ""

    # Tell Explorer about the new "Open with" entries.
    System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, p 0, p 0)'
SectionEnd

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
    !insertmacro MUI_DESCRIPTION_TEXT ${SecLeanTerm} "LeanTerm connection manager (required)."
    !insertmacro MUI_DESCRIPTION_TEXT ${SecLeanEdit} "LeanEdit, LeanTerm's text editor on its own: Start menu and desktop shortcut and $\"Open with$\" for text files."
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "uninstall" 
    !insertmacro wails.setShellContext

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath

    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    # LeanEdit (nothing to do when it was not selected)
    Delete "$SMPROGRAMS\LeanEdit.lnk"
    Delete "$DESKTOP\LeanEdit.lnk"
    !insertmacro leanedit.forEachExtension leanedit.removeOpenWith
    DeleteRegValue SHCTX "Software\Classes\SystemFileAssociations\text\OpenWithProgids" "${LEANEDIT_PROGID}"
    DeleteRegKey SHCTX "Software\Classes\${LEANEDIT_PROGID}"
    System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, p 0, p 0)'

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
SectionEnd
