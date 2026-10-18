# CLIProxyAPI Desktop

A small Windows tray / macOS menu bar launcher that bundles official
[CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) release executables.
This repository is independent of the server: it does not fork, compile, patch,
or import CLIProxyAPI source.

## Application behavior

Launching the app starts its bundled server. The tray menu provides:

- Status and Start / Stop Service.
- Open Config Folder: Finder reveals `config.yaml`; Explorer opens its folder.
- Open Management UI: opens `/management.html` in the default browser.
- Reset Management Key: choose and confirm a new management login key.
- Start at Sign-in: enable or disable automatic tray and proxy startup for this user.
- Open Logs Folder, Show Last Error, About, and Quit.

Stop leaves the tray app running. Quit stops the owned server and exits. Stopping
uses the server's normal shutdown path, which interrupts active requests.
The app refuses to start on an occupied address and never adopts or terminates
an unrelated server. One launcher is allowed per data directory.

On Windows, run the setup EXE for a per-user installation with Start menu and
uninstall support. A portable ZIP is also provided: extract it completely and keep
the helper and `resources` directory alongside `CLIProxyAPI Desktop.exe`. On macOS,
copy the app from the DMG into Applications before launching it. The app has no
Dock window.

## Configuration and credentials

| Platform | Writable data directory |
| --- | --- |
| Windows | `%LOCALAPPDATA%\CLIProxyAPI\` |
| macOS | `~/Library/Application Support/CLIProxyAPI/` |

The first launch asks you to choose and confirm a management key. Canceling leaves
`config.yaml` uncreated. The app then creates a config from the bundled template,
binds to `127.0.0.1:8317`, replaces template API keys with a random client key, and
saves a bcrypt hash of your management key. Remember the key for the management
page's login screen. The plaintext key is never saved or cached in a system
keychain. `config.yaml` remains the source of truth when you change the key in
the file or management UI.

**Reset Management Key…** prompts for a new key and confirmation, then atomically
updates its hash in the current or legacy config layout while preserving other
settings and comments. A running server is stopped before the edit to avoid
watcher/API write races and restarted afterward; active requests are closed. A
stopped server stays stopped. Canceling the dialog changes nothing. Keys must
be non-empty and at most 72 UTF-8 bytes, matching bcrypt's input limit.

Upgrading from 0.1 preserves the existing config and password. Old Keychain or
Credential Manager entries are no longer accessed and can be removed manually.
Use Reset Management Key if you no longer know the existing password.

An existing config is preserved. Importing a config is currently manual: stop the
server, use Open Config Folder, replace the file, then start it again. Use that
config's management password in the browser. Both the current nested listener layout and legacy
top-level listener settings are recognized. Listener/TLS changes take effect
after Stop and Start. The running browser URL stays tied to the listener used at
startup. Custom TLS certificates must be trusted by the OS.

Credentials, logs, and the writable management page live below the data directory,
outside the installed application. Upgrading the app preserves configuration and
provider credentials. The app refreshes its managed page from the bundled version
at launch and disables the
upstream panel auto-updater in new configs. Removing the application does not
remove user data. The Windows uninstaller removes this profile's startup entry.

## Automatic startup

Enable **Start at Sign-in** in the tray menu. It is off by default and requires
no administrator privileges. macOS uses a per-user LaunchAgent; Windows uses a
per-user Run registry value. The tray and bundled proxy start after you sign in.
Quit remains effective for the current session; it does not revoke the next
sign-in's startup registration. Uncheck the option to disable future automatic
startup. OS background/startup settings can also restrict startup.

Registration is refreshed when you launch a moved or upgraded application. Copy
the macOS app into Applications or install/extract the Windows app into a
permanent location before enabling it. See [startup options](docs/autostart.md)
for the platform choices and their tradeoffs.

The launcher supplies explicit config/static/data paths and removes inherited
cloud-store/Home/management-password environment overrides. A `.env` deliberately
placed in the data directory is still loaded by upstream; keep this consistent
with a local file-backed instance. CLIProxyAPIHome operation, machine-wide
services that run before sign-in, and in-app executable updating are not included.

## Build and verification

Requires Go 1.26+ and Python 3.12+. Linux ARM64 is supported for shared development
and headless testing; native desktop packages use Windows/macOS runners.

```bash
gofmt -w .
go test ./...
go vet ./...
go build -o build/desktop ./cmd/desktop
go build -o build/cli-proxy-supervisor ./cmd/supervisor
python3 -m unittest discover -s scripts -p 'test_*.py'
```

To resolve exact upstream versions and build on a native host:

```bash
python3 scripts/upstream.py resolve --output build/upstream.json
# Choose the current host: windows-amd64, windows-arm64, darwin-amd64, darwin-arm64
python3 scripts/package.py --target darwin-arm64 --manifest build/upstream.json
```

`resolve --core v8.0.31 --panel v1.26.5` selects explicit stable releases. Each
manifest records exact release tags, URLs, and SHA-256 hashes. Incomplete upstream
releases fail resolution until all four assets and their checksums are published.
Every matrix job uses the same manifest. Archive contents are read as named
regular files without extracting untrusted paths. The official server, panel,
wrapper, and Go dependency licenses are included with the package.

To exercise the actual official Linux server on this development host:

```bash
python3 scripts/upstream.py resolve --include-linux --output build/upstream.json
python3 scripts/package.py --target linux-arm64 --manifest build/upstream.json
```

Linux output is for headless verification, not a supported tray distribution.
The packaged `--smoke-test --data-dir <temporary-directory>` mode supplies an
ephemeral key without displaying a dialog. It starts and stops the real
server twice, verifies `/management.html` and authenticated `/v8/management/config`,
resets the management key and checks the new key succeeds while the old key is
rejected, and checks listener release. Native
regression tests also prove delivery of the normal interrupt handler, restart,
early successful exits, and refusal of occupied ports. A desktop session is still
needed to manually verify tray rendering, password dialogs, folder reveal, and browser
launch on each OS.

Windows packaging downloads a checksum-pinned Inno Setup 6.4.3 compiler into
`build/tools` and produces a per-user setup EXE plus the portable ZIP. You may set
`ISCC` to an existing compiler executable. Disposable Windows Actions runners
also silently install the setup EXE, exercise the installed application, uninstall
it, and check that user configuration survives. Local packaging never executes
an installer automatically.

## GitHub Actions

The build workflow runs on pushes to `main`, pull requests, manual dispatch, and
a daily schedule. It resolves the latest stable server and management releases,
then builds and smoke-tests:

| Target | Runner | Artifact |
| --- | --- | --- |
| Windows x64 | `windows-latest` | Setup EXE + portable ZIP |
| Windows ARM64 | `windows-11-arm` | Setup EXE + portable ZIP |
| macOS Intel | `macos-15-intel` | DMG |
| macOS Apple Silicon | `macos-15` | DMG |

The first push needs no signing credentials: all six packages and four JSON build
reports are uploaded as workflow artifacts. Scheduled runs build updated upstream
versions as artifacts; they do not automatically publish releases. Native runners
must be available to the repository and GitHub Actions must be enabled.

Publish by pushing a `v<version>` tag matching `VERSION`, or manually run the
workflow with **publish** enabled. Manual publication creates a release tag that
includes the wrapper, server, and panel versions. Publication happens only after
all four native builds and their package tests succeed, and includes final package checksums and
per-platform signing status. No GitHub owner is hardcoded, so this repository can
be pushed to any account or organization.

### Optional signing secrets

| Secret | Purpose |
| --- | --- |
| `WINDOWS_SIGN_PFX_B64` | Base64-encoded Authenticode signing certificate/private key |
| `WINDOWS_SIGN_PASSWORD` | PFX password |
| `MACOS_CERT_P12_B64` | Base64-encoded Developer ID certificate/private key |
| `MACOS_CERT_PASSWORD` | P12 password |
| `MACOS_SIGN_IDENTITY` | Full Developer ID Application identity |
| `APPLE_ID` | Apple notarization account |
| `APPLE_TEAM_ID` | Apple team identifier |
| `APPLE_APP_PASSWORD` | App-specific password for notarization |

When macOS signing is configured, all Apple signing/notarization secrets are
required. Nested executables are signed before the outer bundle; the application
and DMG are notarized and stapled. Only the server receives the entitlement to
load user-installed dynamic plugins. Without these credentials, macOS builds are
ad-hoc signed for testing and require approval in Privacy & Security; use Developer
ID signed/notarized builds for normal distribution. Windows signing is likewise
optional; the build report distinguishes signed and unsigned artifacts.

## Add a remote and trigger the first build

Create an empty GitHub repository, then run from this repository's root:

```bash
git remote add origin git@github.com:YOUR-ACCOUNT/CLIProxyAPI-Desktop.git
git push -u origin main
```

Open the new repository's Actions tab and select **Build desktop apps**. For
later tagged releases, first update `VERSION`, commit, then create and push the
matching version tag. Do not add this repository as a submodule of CLIProxyAPI.

## Architecture

`cmd/desktop` initializes user data, locks the instance, and binds the tray to the
shared service controller. `cmd/supervisor` owns the official executable. Closing
the private parent pipe requests shutdown even after a launcher crash. On macOS
the server runs in a separate process group and receives SIGTERM. On Windows a
console-subsystem helper owns a hidden console, starts the server in a separate
console group, and sends CTRL_BREAK. A Windows job object contains the server
tree. A local process watchdog forces cleanup if normal shutdown does not finish;
the wrapper does not set timeouts on upstream proxy requests.

The upstream executable is treated as an external process. Compatibility work may
be needed when upstream changes flags, YAML paths, endpoints, or asset names, but
new server releases require no source merges or rebases.
