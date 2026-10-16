import hashlib
import io
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch
import zipfile

import upstream


class UpstreamTests(unittest.TestCase):
    def test_checksum_parser_rejects_duplicates_and_paths(self):
        checksum = "a" * 64
        self.assertEqual(upstream.parse_checksums(f"{checksum}  app.zip\n"), {"app.zip": checksum})
        for text in (f"{checksum}  ../app.zip", f"{checksum}  app.zip\n{checksum}  app.zip", "bad  app.zip"):
            with self.assertRaises(ValueError):
                upstream.parse_checksums(text)

    def test_download_rejects_wrong_content(self):
        with patch.object(upstream, "fetch", return_value=b"tampered"):
            with self.assertRaises(ValueError):
                upstream.verified({"url": "https://example.invalid/asset", "sha256": hashlib.sha256(b"expected").hexdigest()})

    def test_extracts_named_file_without_writing_archive_paths(self):
        with tempfile.TemporaryDirectory() as directory:
            archive = Path(directory) / "core.tar.gz"
            with tarfile.open(archive, "w:gz") as output:
                member = tarfile.TarInfo("folder/cli-proxy-api")
                member.size = 4
                output.addfile(member, io.BytesIO(b"core"))
                evil = tarfile.TarInfo("../../escape")
                evil.size = 3
                output.addfile(evil, io.BytesIO(b"bad"))
            self.assertEqual(upstream.extract_member(archive, "cli-proxy-api"), b"core")
            self.assertFalse((Path(directory) / "escape").exists())

    def test_rejects_duplicate_binary_and_symlink(self):
        with tempfile.TemporaryDirectory() as directory:
            archive = Path(directory) / "core.zip"
            with zipfile.ZipFile(archive, "w") as output:
                output.writestr("one/cli-proxy-api.exe", b"one")
                output.writestr("two/cli-proxy-api.exe", b"two")
            with self.assertRaises(ValueError):
                upstream.extract_member(archive, "cli-proxy-api.exe")
            archive = Path(directory) / "core.tar.gz"
            with tarfile.open(archive, "w:gz") as output:
                member = tarfile.TarInfo("cli-proxy-api")
                member.type, member.linkname = tarfile.SYMTYPE, "/bin/other"
                output.addfile(member)
            with self.assertRaises(ValueError):
                upstream.extract_member(archive, "cli-proxy-api")

    def test_api_and_published_checksums_must_agree(self):
        with self.assertRaises(ValueError):
            upstream.asset_record({"name": "core.zip", "digest": "sha256:" + "a" * 64}, "b" * 64)

    def test_windows_download_preserves_and_accepts_crlf_license(self):
        archive = io.BytesIO()
        with zipfile.ZipFile(archive, "w") as output:
            output.writestr("cli-proxy-api.exe", b"MZfixture")
            output.writestr("config.example.yaml", b"server:\n  port: 8317\n")
            output.writestr("LICENSE", b"MIT License\r\nfixture\r\n")
        content = {"archive": archive.getvalue(), "license": b"MIT License\nfixture\n", "panel": b"<html>panel</html>"}
        def record(key, name=""):
            return {"name": name, "url": key, "sha256": upstream.digest(content[key])}
        manifest = {"core": {"assets": {"windows-amd64": record("archive", "core.zip")}, "license": record("license")},
                    "panel": {"asset": record("panel"), "license": record("license")}}
        with tempfile.TemporaryDirectory() as directory, patch.object(upstream, "fetch", side_effect=lambda url: content[url]):
            output = Path(directory)
            upstream.download(manifest, "windows-amd64", output)
            self.assertEqual((output / "cli-proxy-api.exe").read_bytes(), b"MZfixture")
            self.assertEqual((output / "licenses/CLIProxyAPI-LICENSE.txt").read_bytes(), b"MIT License\r\nfixture\r\n")
            self.assertFalse((output / "core.zip").exists())


if __name__ == "__main__":
    unittest.main()
