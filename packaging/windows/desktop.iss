; Build with /DSourceDir=... /DOutputDir=... /DOutputName=... /DAppVersion=... /DAppArch=...
[Setup]
AppId=CLIProxyAPI-Desktop
AppName=CLIProxyAPI Desktop
AppVersion={#AppVersion}
AppPublisher=CLIProxyAPI Desktop contributors
DefaultDirName={localappdata}\Programs\CLIProxyAPI Desktop
DefaultGroupName=CLIProxyAPI Desktop
PrivilegesRequired=lowest
WizardStyle=modern
DisableProgramGroupPage=yes
OutputDir={#OutputDir}
OutputBaseFilename={#OutputName}
SetupIconFile={#SourceDir}\AppIcon.ico
UninstallDisplayIcon={app}\CLIProxyAPI Desktop.exe
Compression=lzma2
SolidCompression=yes
AppMutex=Local\CLIProxyAPI-Desktop-Installer
CloseApplications=no
RestartApplications=no
#if AppArch == "arm64"
ArchitecturesAllowed=arm64
ArchitecturesInstallIn64BitMode=arm64
#else
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
#endif

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; Flags: unchecked

[Files]
Source: "{#SourceDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\CLIProxyAPI Desktop"; Filename: "{app}\CLIProxyAPI Desktop.exe"; WorkingDir: "{app}"; IconFilename: "{app}\AppIcon.ico"
Name: "{autodesktop}\CLIProxyAPI Desktop"; Filename: "{app}\CLIProxyAPI Desktop.exe"; WorkingDir: "{app}"; IconFilename: "{app}\AppIcon.ico"; Tasks: desktopicon

[Run]
Filename: "{app}\CLIProxyAPI Desktop.exe"; Description: "Launch CLIProxyAPI Desktop"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "{app}\CLIProxyAPI Desktop.exe"; Parameters: "--disable-autostart"; Flags: runhidden waituntilterminated; RunOnceId: "DisableSignInStartup"

; Only installation files are removed. User configuration, auth files, and logs
; live outside {app} and deliberately have no UninstallDelete entries.
