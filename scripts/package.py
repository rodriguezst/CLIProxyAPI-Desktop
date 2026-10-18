"""Build native launchers, test the bundled server, and create desktop packages."""

import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import platform
import plistlib
import secrets
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import zipfile

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / "scripts"))
import upstream


def run(args, **kwargs):
    # Do not include command arguments in exceptions: signing commands contain secrets.
    result = subprocess.run([str(a) for a in args], **kwargs)
    if result.returncode:
        raise RuntimeError(f"{Path(str(args[0])).name} failed with exit code {result.returncode}")
    return result


def smoke(launcher):
    with tempfile.TemporaryDirectory(prefix="CPA smoke with spaces ") as directory:
        try:
            run([launcher, "--smoke-test", "--data-dir", directory], timeout=100)
        except (RuntimeError, subprocess.TimeoutExpired):
            log = Path(directory) / "logs" / "server.log"
            if log.exists():
                # The isolated test uses generated credentials and no provider accounts.
                print(log.read_text(encoding="utf-8", errors="replace")[-12000:])
                shutil.copy2(log, ROOT / "build" / "smoke-failure.log")
            raise
        # The server's listener must have been released by the helper.
        with socket.socket() as probe:
            probe.bind(("127.0.0.1", 8317))


def build(target, manifest, version):
    goos, arch = target.split("-", 1)
    native_os = {"Windows": "windows", "Darwin": "darwin", "Linux": "linux"}[platform.system()]
    native_arch = {"x86_64": "amd64", "AMD64": "amd64", "arm64": "arm64", "aarch64": "arm64", "ARM64": "arm64"}[platform.machine()]
    if (goos, arch) != (native_os, native_arch):
        raise ValueError("package.py must run on the target's native OS and architecture")
    work = ROOT / "build" / target
    if work.exists():
        shutil.rmtree(work)
    inputs = work / "inputs"
    upstream.download(manifest, target, inputs)
    app = work / "CLIProxyAPI Desktop.app" if goos == "darwin" else work / "CLIProxyAPI Desktop"
    binary = app / "Contents" / "MacOS" if goos == "darwin" else app
    resources = app / "Contents" / "Resources" if goos == "darwin" else app / "resources"
    binary.mkdir(parents=True)
    shutil.copytree(inputs, resources)
    suffix = ".exe" if goos == "windows" else ""
    if goos == "darwin":
        shutil.move(resources / "cli-proxy-api", binary / "cli-proxy-api")
    launcher = binary / ("CLIProxyAPI Desktop" + suffix)
    helper = binary / ("cli-proxy-supervisor" + suffix)
    env = dict(os.environ, GOOS=goos, GOARCH=arch, CGO_ENABLED="1" if goos == "darwin" else "0")
    if goos == "darwin":
        env["MACOSX_DEPLOYMENT_TARGET"] = "12.0"
    flags = f"-s -w -X main.version={version}"
    if goos == "windows":
        flags += " -H=windowsgui"
    run(["go", "build", "-trimpath", "-ldflags", flags, "-o", launcher, "./cmd/desktop"], cwd=ROOT, env=env)
    run(["go", "build", "-trimpath", "-ldflags", "-s -w", "-o", helper, "./cmd/supervisor"], cwd=ROOT, env=env)
    shutil.copy2(ROOT / "LICENSE", resources / "licenses" / "Desktop-LICENSE.txt")
    # Include exact third-party module license files, including transitive modules.
    result = run(["go", "list", "-m", "-json", "all"], cwd=ROOT, capture_output=True, text=True)
    decoder, tail = json.JSONDecoder(), result.stdout
    while tail.strip():
        module, offset = decoder.raw_decode(tail.lstrip())
        tail = tail.lstrip()[offset:]
        if module.get("Main") or not module.get("Dir"):
            continue
        for filename in ("LICENSE", "LICENSE.txt", "LICENSE.md", "COPYING", "NOTICE"):
            source = Path(module["Dir"]) / filename
            if source.is_file():
                name = module["Path"].replace("/", "_") + "-" + filename
                shutil.copy2(source, resources / "licenses" / name)
    (resources / "build-info.txt").write_text(
        f"CLIProxyAPI: {manifest['core']['tag']}\nManagement UI: {manifest['panel']['tag']}\nTarget: {target}\n", encoding="utf-8")
    if goos == "darwin":
        plist = plistlib.loads((ROOT / "packaging/macos/Info.plist").read_bytes())
        plist["CFBundleShortVersionString"] = version
        plist["CFBundleVersion"] = os.environ.get("GITHUB_RUN_NUMBER", "1")
        (app / "Contents/Info.plist").write_bytes(plistlib.dumps(plist))
        png = work / "icon.png"
        run(["go", "run", "./cmd/icon", "--out", png], cwd=ROOT)
        iconset = work / "AppIcon.iconset"
        iconset.mkdir()
        for size in (16, 32, 128, 256, 512):
            for scale in (1, 2):
                name = f"icon_{size}x{size}" + ("@2x" if scale == 2 else "") + ".png"
                run(["sips", "-z", size * scale, size * scale, png, "--out", iconset / name], stdout=subprocess.DEVNULL)
        run(["iconutil", "-c", "icns", iconset, "-o", resources / "AppIcon.icns"])
    elif goos == "windows":
        run(["go", "run", "./cmd/icon", "--out", app / "AppIcon.ico"], cwd=ROOT)
    return app, binary, launcher


def sign_mac(app, binary):
    identity = os.environ.get("MACOS_SIGN_IDENTITY", "")
    cert = os.environ.get("MACOS_CERT_P12_B64", "")
    if bool(identity) != bool(cert):
        raise ValueError("MACOS_SIGN_IDENTITY and MACOS_CERT_P12_B64 must be configured together")
    if identity and not all(os.environ.get(key) for key in ("APPLE_ID", "APPLE_TEAM_ID", "APPLE_APP_PASSWORD")):
        raise ValueError("Developer ID signing requires all Apple notarization credentials")
    with tempfile.TemporaryDirectory() as temp:
        keychain = Path(temp) / "build.keychain-db"
        if identity:
            password = secrets.token_hex(24)
            p12 = Path(temp) / "sign.p12"
            p12.write_bytes(base64.b64decode(cert))
            run(["security", "create-keychain", "-p", password, keychain])
            run(["security", "set-keychain-settings", "-lut", "21600", keychain])
            run(["security", "unlock-keychain", "-p", password, keychain])
            run(["security", "import", p12, "-k", keychain, "-P", os.environ.get("MACOS_CERT_PASSWORD", ""), "-T", "/usr/bin/codesign"])
            run(["security", "set-key-partition-list", "-S", "apple-tool:,apple:", "-s", "-k", password, keychain], stdout=subprocess.DEVNULL)
        try:
            for item in (binary / "cli-proxy-api", binary / "cli-proxy-supervisor", binary / "CLIProxyAPI Desktop", app):
                args = ["codesign", "--force", "--sign", identity or "-"]
                if identity:
                    args += ["--keychain", keychain, "--options", "runtime", "--timestamp"]
                    if item.name == "cli-proxy-api":
                        args += ["--entitlements", ROOT / "packaging/macos/core-entitlements.plist"]
                run(args + [item])
            run(["codesign", "--verify", "--deep", "--strict", app])
        finally:
            if identity:
                run(["security", "delete-keychain", keychain])
    return bool(identity)


def notarize(path):
    credentials = [os.environ.get(key) for key in ("APPLE_ID", "APPLE_TEAM_ID", "APPLE_APP_PASSWORD")]
    if not all(credentials):
        raise ValueError("notarization requires APPLE_ID, APPLE_TEAM_ID and APPLE_APP_PASSWORD")
    run(["xcrun", "notarytool", "submit", path, "--apple-id", credentials[0], "--team-id", credentials[1], "--password", credentials[2], "--wait"])


def sign_windows_files(executables):
    cert = os.environ.get("WINDOWS_SIGN_PFX_B64", "")
    if not cert:
        return False
    candidates = list(Path("C:/Program Files (x86)/Windows Kits/10/bin").glob("*/arm64/signtool.exe" if platform.machine().lower() == "arm64" else "*/x64/signtool.exe"))
    tool = shutil.which("signtool") or (str(sorted(candidates)[-1]) if candidates else None)
    if not tool:
        raise ValueError("Windows signing requires signtool from the Windows SDK")
    with tempfile.TemporaryDirectory() as temp:
        pfx = Path(temp) / "sign.pfx"
        pfx.write_bytes(base64.b64decode(cert))
        for executable in executables:
            run([tool, "sign", "/f", pfx, "/p", os.environ.get("WINDOWS_SIGN_PASSWORD", ""), "/fd", "SHA256", "/tr", "http://timestamp.digicert.com", "/td", "SHA256", executable])
    return True


def inno_compiler():
    configured = os.environ.get("ISCC")
    if configured:
        return Path(configured)
    install = ROOT / "build" / "tools" / "inno"
    compiler = install / "ISCC.exe"
    if not compiler.exists():
        install.parent.mkdir(parents=True, exist_ok=True)
        record = json.loads((ROOT / "packaging/windows/inno-setup.json").read_text())
        setup = install.parent / "inno-setup.exe"
        setup.write_bytes(upstream.verified(record))
        run([setup, "/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART", "/CURRENTUSER", f"/DIR={install}"])
        if not compiler.exists():
            raise RuntimeError("Inno Setup installation did not provide ISCC.exe")
    return compiler


def windows_installer(app, dist, name, target, version):
    run([inno_compiler(), f"/DSourceDir={app}", f"/DOutputDir={dist}",
         f"/DOutputName={name}_setup", f"/DAppVersion={version}",
         f"/DAppArch={target.split('-')[1]}", ROOT / "packaging/windows/desktop.iss"])
    installer = dist / (name + "_setup.exe")
    if not installer.exists():
        raise RuntimeError("Inno Setup did not create the Windows installer")
    return installer


def wait_for_windows_uninstall(install, timeout=30):
    # Inno Setup returns before its temporary clone deletes the original
    # uninstaller. Observe that cleanup without deleting any installed files.
    deadline = time.monotonic() + timeout
    while True:
        try:
            remaining = sorted(path.name for path in install.iterdir())
        except FileNotFoundError:
            return
        except PermissionError as error:
            detail = str(error)
        else:
            if not remaining:
                return
            detail = ", ".join(remaining[:10])
        if time.monotonic() >= deadline:
            raise RuntimeError(
                f"uninstaller returned success but cleanup did not finish within {timeout:g}s: {detail}")
        time.sleep(0.1)


def windows_installer_smoke(installer):
    # Only invoke a real installer on the disposable Actions host, never a
    # developer's machine where the same AppId may already be installed.
    if os.environ.get("GITHUB_ACTIONS") != "true":
        return False
    logs = ROOT / "build" / "installer-smoke"
    logs.mkdir(parents=True, exist_ok=True)
    # A locked temporary file must not mask an installer or smoke-test failure.
    # Successful uninstallation is checked separately before workspace cleanup.
    with tempfile.TemporaryDirectory(prefix="CPA installer smoke ", ignore_cleanup_errors=True) as temp:
        root = Path(temp)
        install = root / "Installed App"
        profile = root / "Local App Data"
        data = profile / "CLIProxyAPI"
        data.mkdir(parents=True)
        sentinel = data / "config.yaml"
        sentinel.write_text("# preserve existing user settings\n", encoding="utf-8")
        env = dict(os.environ, LOCALAPPDATA=str(profile))
        run([installer, "/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART",
             f"/DIR={install}", f"/LOG={logs / 'install.log'}"], env=env, timeout=120)
        launcher = install / "CLIProxyAPI Desktop.exe"
        if not launcher.is_file() or not (install / "resources/cli-proxy-api.exe").is_file():
            raise RuntimeError("installer did not deploy the application and bundled server")
        smoke(launcher)
        uninstallers = list(install.glob("unins*.exe"))
        if len(uninstallers) != 1:
            raise RuntimeError("installer did not provide an uninstaller")
        run([uninstallers[0], "/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART",
             f"/LOG={logs / 'uninstall.log'}"], env=env, timeout=120)
        wait_for_windows_uninstall(install)
        if sentinel.read_text(encoding="utf-8") != "# preserve existing user settings\n":
            raise RuntimeError("uninstall failed to remove program files or changed user data")
    return True


def package(target, manifest, version):
    if not re_version(version):
        raise ValueError("VERSION must have three numeric components")
    app, binary, launcher = build(target, manifest, version)
    signed = sign_mac(app, binary) if target.startswith("darwin-") else sign_windows_files(app.rglob("*.exe")) if target.startswith("windows-") else False
    smoke(launcher)
    dist = ROOT / "dist"
    dist.mkdir(exist_ok=True)
    name = f"CLIProxyAPI-Desktop_{version}_core-{manifest['core']['tag']}_ui-{manifest['panel']['tag']}_{target}"
    notarized = False
    artifacts = []
    installer_tested = False
    if target.startswith("darwin-"):
        if signed:
            archive = app.parent / "notarize.zip"
            run(["ditto", "-c", "-k", "--keepParent", app, archive])
            notarize(archive)
            run(["xcrun", "stapler", "staple", app])
        dmg_root = app.parent / "dmg-root"
        dmg_root.mkdir()
        shutil.copytree(app, dmg_root / app.name, symlinks=True)
        (dmg_root / "Applications").symlink_to("/Applications")
        artifact = dist / (name + ".dmg")
        run(["hdiutil", "create", "-volname", "CLIProxyAPI Desktop", "-srcfolder", dmg_root, "-ov", "-format", "UDZO", artifact])
        if signed:
            notarize(artifact)
            run(["xcrun", "stapler", "staple", artifact])
            notarized = True
    else:
        artifact = dist / (name + ".zip")
        with zipfile.ZipFile(artifact, "w", zipfile.ZIP_DEFLATED) as archive:
            for file in app.rglob("*"):
                if file.is_file():
                    archive.write(file, file.relative_to(app.parent))
    artifacts.append(artifact)
    if target.startswith("windows-"):
        installer = windows_installer(app, dist, name, target, version)
        sign_windows_files([installer])
        installer_tested = windows_installer_smoke(installer)
        artifacts.append(installer)
    report = {"wrapper": version, "target": target, "upstream": manifest, "signed": signed,
              "notarized": notarized, "installer_tested": installer_tested,
              "artifact": artifact.name, "sha256": hashlib.sha256(artifact.read_bytes()).hexdigest(),
              "artifacts": [{"name": a.name, "sha256": hashlib.sha256(a.read_bytes()).hexdigest()} for a in artifacts]}
    (dist / (name + ".json")).write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print(f"Created {', '.join(a.name for a in artifacts)}; signed={signed}, notarized={notarized}")


def re_version(value):
    import re
    return re.fullmatch(r"\d+\.\d+\.\d+", value)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--target", required=True)
    parser.add_argument("--manifest", type=Path, default=ROOT / "build/upstream.json")
    parser.add_argument("--version", default=(ROOT / "VERSION").read_text().strip())
    args = parser.parse_args()
    try:
        package(args.target, json.loads(args.manifest.read_text(encoding="utf-8")), args.version)
    except Exception as error:
        print(f"Packaging failed: {error}", file=sys.stderr)
        sys.exit(1)
