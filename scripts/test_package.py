from pathlib import Path
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


if __name__ == "__main__":
    unittest.main()
