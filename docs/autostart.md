# Sign-in startup options

The implemented option starts the existing tray app after the current user signs
in. The tray retains ownership of the server, its data paths, and its normal
Start/Stop/Quit behavior. Startup is opt-in and per-user.

| Platform mechanism | Behavior | Decision |
| --- | --- | --- |
| macOS user LaunchAgent | Starts in the user's graphical login session; supports macOS 12 | Implemented |
| macOS SMAppService login item | Modern registration and integration with Login Items; requires macOS 13+ and an additional native integration | Possible future replacement |
| Windows HKCU Run value | Starts at user sign-in without elevation | Implemented |
| Windows Scheduled Task at logon | Can add delays or conditions, but adds task configuration and maintenance | Not needed for the current tray app |
| macOS LaunchDaemon / Windows service | Runs before sign-in, needs administrative installation and separate service ownership/control | Outside the selected sign-in behavior |

On macOS, the app writes a plist under `~/Library/LaunchAgents/` with an absolute
executable path, explicit data directory, `RunAtLoad=true`, `KeepAlive=false`, and
an Aqua session restriction. It is read at the next login. Toggling the option
does not load or unload the current tray process, and Quit does not trigger an
automatic relaunch. Output goes to `logs/desktop.log` in the profile directory.

On Windows, the app writes a quoted command to
`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`. The command invokes the tray
with `--login --data-dir ...`. A profile-specific identifier prevents different
data directories from overwriting one another. Registration is refreshed on app
launch so a deliberate app move or upgrade can update its executable path.

The `--login` launch mode quietly exits if the same profile is already running,
including when macOS session restoration also reopens the app. Windows uninstall
runs `--disable-autostart` before removing application files and preserves user
configuration and provider credentials. Disable startup in any custom profiles
before uninstalling; the uninstaller addresses the default profile.

An OS-disabled startup/background entry may remain registered; OS settings take
precedence. The tray checkbox reports whether this app's registration exists.

References:

- [Apple: Creating Launch Daemons and Agents](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/CreatingLaunchdJobs.html)
- [Apple: SMAppService](https://developer.apple.com/documentation/servicemanagement/smappservice)
- [Microsoft: Run and RunOnce Registry Keys](https://learn.microsoft.com/en-us/windows/win32/setupapi/run-and-runonce-registry-keys)
