from pathlib import Path
import shutil
import tempfile
import unittest
from unittest.mock import patch

import package


class InstallerTests(unittest.TestCase):
    def test_installer_command_keeps_paths_and_architecture_as_arguments(self):
        with tempfile.TemporaryDirectory(prefix="installer paths ") as directory:
            root = Path(directory)
            app, dist = root / "app files", root / "dist files"
            dist.mkdir()
            expected = dist / "desktop_setup.exe"
            calls = []

            def compile_setup(args):
                calls.append([str(arg) for arg in args])
                expected.write_bytes(b"installer fixture")

            with patch.object(package, "inno_compiler", return_value=Path("ISCC.exe")), patch.object(package, "run", side_effect=compile_setup):
                self.assertEqual(package.windows_installer(app, dist, "desktop", "windows-arm64", "0.2.0"), expected)
            self.assertIn(f"/DSourceDir={app}", calls[0])
            self.assertIn("/DAppArch=arm64", calls[0])
            self.assertIn("/DAppVersion=0.2.0", calls[0])

    def test_installer_smoke_does_not_run_on_developer_host(self):
        with patch.dict(package.os.environ, {"GITHUB_ACTIONS": "false"}), patch.object(package, "run") as run:
            self.assertFalse(package.windows_installer_smoke(Path("installer.exe")))
            run.assert_not_called()

    def test_installer_smoke_waits_for_clone_and_checks_user_data(self):
        for change_config in (False, True):
            with self.subTest(change_config=change_config), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                installer = root / "setup.exe"
                install = None

                def installer_run(args, **kwargs):
                    nonlocal install
                    # Keep the logs outside the disposable installation workspace.
                    log = Path(next(arg[5:] for arg in args if str(arg).startswith("/LOG=")))
                    log.write_text("installer fixture log\n", encoding="utf-8")
                    if args[0] == installer:
                        install = Path(next(arg[5:] for arg in args if str(arg).startswith("/DIR=")))
                        (install / "resources").mkdir(parents=True)
                        for path in (install / "CLIProxyAPI Desktop.exe", install / "resources/cli-proxy-api.exe", install / "unins000.exe"):
                            path.write_bytes(b"executable fixture")
                    else:
                        self.assertEqual(args[0], install / "unins000.exe")
                        (install / "CLIProxyAPI Desktop.exe").unlink()
                        shutil.rmtree(install / "resources")
                        if change_config:
                            config = Path(kwargs["env"]["LOCALAPPDATA"]) / "CLIProxyAPI/config.yaml"
                            config.write_text("changed user settings\n", encoding="utf-8")
                        # Inno returns here while its clone still holds unins000.exe.

                def finish_uninstall(_):
                    (install / "unins000.exe").unlink()
                    install.rmdir()

                with patch.dict(package.os.environ, {"GITHUB_ACTIONS": "true"}), \
                        patch.object(package, "ROOT", root), \
                        patch.object(package, "run", side_effect=installer_run), \
                        patch.object(package, "smoke") as smoke, \
                        patch.object(package.time, "monotonic", side_effect=[0, 0]), \
                        patch.object(package.time, "sleep", side_effect=finish_uninstall) as sleep:
                    if change_config:
                        with self.assertRaisesRegex(RuntimeError, "changed user data"):
                            package.windows_installer_smoke(installer)
                    else:
                        self.assertTrue(package.windows_installer_smoke(installer))
                    smoke.assert_called_once_with(install / "CLIProxyAPI Desktop.exe")
                    sleep.assert_called_once_with(0.1)
                self.assertFalse(install.exists())
                for name in ("install.log", "uninstall.log"):
                    self.assertEqual((root / "build/installer-smoke" / name).read_text(), "installer fixture log\n")

    def test_uninstall_fails_if_program_files_remain(self):
        with tempfile.TemporaryDirectory() as directory:
            install = Path(directory)
            launcher = install / "CLIProxyAPI Desktop.exe"
            launcher.write_bytes(b"still installed")
            with patch.object(package.time, "monotonic", side_effect=[0, 0, 30]), \
                    patch.object(package.time, "sleep"):
                with self.assertRaisesRegex(RuntimeError, "cleanup did not finish within 30s: CLIProxyAPI Desktop.exe"):
                    package.wait_for_windows_uninstall(install)
            self.assertEqual(launcher.read_bytes(), b"still installed")

    def test_uninstall_retries_transient_directory_lock(self):
        with patch.object(Path, "iterdir", side_effect=[PermissionError("locked"), []]), \
                patch.object(package.time, "monotonic", side_effect=[0, 0]), \
                patch.object(package.time, "sleep") as sleep:
            package.wait_for_windows_uninstall(Path("Installed App"))
            sleep.assert_called_once_with(0.1)

    def test_locked_workspace_cleanup_does_not_mask_installer_failure(self):
        temporary_directory = tempfile.TemporaryDirectory
        unlink = package.os.unlink
        with temporary_directory() as directory:
            root = Path(directory)

            def workspace(**kwargs):
                return temporary_directory(dir=root, **kwargs)

            def fail_install(args, **kwargs):
                install = Path(next(arg[5:] for arg in args if str(arg).startswith("/DIR=")))
                install.mkdir()
                (install / "unins000.exe").write_bytes(b"locked executable")
                raise RuntimeError("original installer failure")

            def locked_unlink(path, *args, **kwargs):
                if Path(path).name == "unins000.exe":
                    raise PermissionError("uninstaller is still running")
                return unlink(path, *args, **kwargs)

            with patch.dict(package.os.environ, {"GITHUB_ACTIONS": "true"}), \
                    patch.object(package, "ROOT", root), \
                    patch.object(package, "run", side_effect=fail_install), \
                    patch.object(package.tempfile, "TemporaryDirectory", side_effect=workspace), \
                    patch.object(package.os, "unlink", side_effect=locked_unlink) as remove:
                with self.assertRaisesRegex(RuntimeError, "original installer failure"):
                    package.windows_installer_smoke(root / "setup.exe")
                self.assertTrue(remove.called)


if __name__ == "__main__":
    unittest.main()
