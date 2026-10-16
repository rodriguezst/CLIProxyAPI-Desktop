"""Resolve official releases once, then verify every downloaded build input."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import tarfile
import urllib.parse
import urllib.request
import zipfile

CORE_REPO = "router-for-me/CLIProxyAPI"
PANEL_REPO = "router-for-me/Cli-Proxy-API-Management-Center"
TARGETS = {
    "windows-amd64": ("windows", "amd64", "zip"),
    "windows-arm64": ("windows", "aarch64", "zip"),
    "darwin-amd64": ("darwin", "amd64", "tar.gz"),
    "darwin-arm64": ("darwin", "aarch64", "tar.gz"),
}
MAX_DOWNLOAD = 512 * 1024 * 1024


def request(url):
    headers = {"User-Agent": "CLIProxyAPI-Desktop", "Accept": "application/vnd.github+json"}
    if urllib.parse.urlparse(url).hostname == "api.github.com":
        token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN")
        if token:
            headers["Authorization"] = f"Bearer {token}"
    return urllib.request.urlopen(urllib.request.Request(url, headers=headers), timeout=60)


def fetch(url, limit=MAX_DOWNLOAD):
    with request(url) as response:
        data = response.read(limit + 1)
    if len(data) > limit:
        raise ValueError("download exceeds size limit")
    return data


def digest(data):
    return hashlib.sha256(data).hexdigest()


def release(repo, tag):
    endpoint = "latest" if tag == "latest" else "tags/" + urllib.parse.quote(tag, safe="")
    data = json.loads(fetch(f"https://api.github.com/repos/{repo}/releases/{endpoint}"))
    if data.get("draft") or data.get("prerelease"):
        raise ValueError("only published stable upstream releases are supported")
    if not re.fullmatch(r"[A-Za-z0-9._-]+", data["tag_name"]):
        raise ValueError("unsupported upstream tag format")
    return data


def asset_record(asset, checksum=None):
    api_digest = asset.get("digest") or ""
    if api_digest.startswith("sha256:"):
        api_digest = api_digest.removeprefix("sha256:")
    else:
        api_digest = ""
    if checksum and api_digest and checksum != api_digest:
        raise ValueError(f"upstream checksum disagrees with API digest: {asset['name']}")
    checksum = checksum or api_digest
    if not checksum:
        checksum = digest(fetch(asset["browser_download_url"]))
    if not re.fullmatch(r"[0-9a-f]{64}", checksum):
        raise ValueError("invalid SHA-256 digest")
    return {"name": asset["name"], "url": asset["browser_download_url"], "sha256": checksum}


def license_record(repo, tag):
    url = f"https://raw.githubusercontent.com/{repo}/{tag}/LICENSE"
    return {"url": url, "sha256": digest(fetch(url, 1024 * 1024))}


def parse_checksums(text):
    result = {}
    for line in text.splitlines():
        if not line.strip():
            continue
        match = re.fullmatch(r"([0-9a-fA-F]{64})\s+\*?([^/\\\s]+)", line)
        if not match or match[2] in result:
            raise ValueError("invalid or duplicate entry in upstream checksums.txt")
        result[match[2]] = match[1].lower()
    return result


def resolve(core_tag="latest", panel_tag="latest", include_linux=False):
    core, panel = release(CORE_REPO, core_tag), release(PANEL_REPO, panel_tag)
    assets = {asset["name"]: asset for asset in core["assets"]}
    if "checksums.txt" not in assets:
        raise ValueError("upstream release is incomplete: checksums.txt is missing; retry later")
    checksum_record = asset_record(assets["checksums.txt"])
    checksum_data = verified(checksum_record)
    checksums = parse_checksums(checksum_data.decode("utf-8"))
    targets = dict(TARGETS)
    if include_linux:
        targets["linux-arm64"] = ("linux", "aarch64", "tar.gz")
        targets["linux-amd64"] = ("linux", "amd64", "tar.gz")
    selected = {}
    for target, (goos, arch, extension) in targets.items():
        name = f"CLIProxyAPI_{core['tag_name'].removeprefix('v')}_{goos}_{arch}.{extension}"
        if name not in assets or name not in checksums:
            raise ValueError(f"upstream release is incomplete: {name} or its checksum is missing; retry later")
        selected[target] = asset_record(assets[name], checksums[name])
    panel_asset = next((a for a in panel["assets"] if a["name"] == "management.html"), None)
    if panel_asset is None:
        raise ValueError("management release does not contain management.html")
    return {
        "schema": 1,
        "core": {"repository": CORE_REPO, "tag": core["tag_name"], "assets": selected,
                 "license": license_record(CORE_REPO, core["tag_name"])},
        "panel": {"repository": PANEL_REPO, "tag": panel["tag_name"],
                  "asset": asset_record(panel_asset), "license": license_record(PANEL_REPO, panel["tag_name"])},
    }


def verified(record):
    data = fetch(record["url"])
    if digest(data) != record["sha256"]:
        raise ValueError(f"SHA-256 verification failed for {record.get('name', 'license')}")
    return data


def extract_member(archive, name, limit=256 * 1024 * 1024):
    # Read only named regular files; never extract paths supplied by an archive.
    if archive.suffix == ".zip":
        with zipfile.ZipFile(archive) as container:
            matches = [m for m in container.infolist() if not m.is_dir() and Path(m.filename).name == name]
            if len(matches) != 1 or matches[0].file_size > limit:
                raise ValueError(f"expected one regular {name} in archive")
            if (matches[0].external_attr >> 16) & 0o170000 == 0o120000:
                raise ValueError("archive contains a symlink")
            return container.read(matches[0])
    with tarfile.open(archive, "r:gz") as container:
        matches = [m for m in container.getmembers() if Path(m.name).name == name]
        if len(matches) != 1 or not matches[0].isfile() or matches[0].size > limit:
            raise ValueError(f"expected one regular {name} in archive")
        with container.extractfile(matches[0]) as source:
            return source.read()


def download(manifest, target, output):
    output.mkdir(parents=True, exist_ok=True)
    record = manifest["core"]["assets"][target]
    archive = output / record["name"]
    archive.write_bytes(verified(record))
    try:
        executable = "cli-proxy-api.exe" if target.startswith("windows-") else "cli-proxy-api"
        (output / executable).write_bytes(extract_member(archive, executable))
        (output / executable).chmod(0o755)
        (output / "config.example.yaml").write_bytes(extract_member(archive, "config.example.yaml", 2 * 1024 * 1024))
        licenses = output / "licenses"
        licenses.mkdir(exist_ok=True)
        (licenses / "CLIProxyAPI-LICENSE.txt").write_bytes(extract_member(archive, "LICENSE", 1024 * 1024))
        # Also pin and verify the source license recorded in the manifest.
        if (licenses / "CLIProxyAPI-LICENSE.txt").read_bytes() != verified(manifest["core"]["license"]):
            raise ValueError("release archive license differs from the pinned source license")
        (licenses / "Management-Center-LICENSE.txt").write_bytes(verified(manifest["panel"]["license"]))
        (output / "management.html").write_bytes(verified(manifest["panel"]["asset"]))
        (output / "upstream.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    finally:
        archive.unlink(missing_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    resolver = sub.add_parser("resolve")
    resolver.add_argument("--core", default="latest")
    resolver.add_argument("--panel", default="latest")
    resolver.add_argument("--include-linux", action="store_true")
    resolver.add_argument("--output", type=Path, default=Path("build/upstream.json"))
    downloader = sub.add_parser("download")
    downloader.add_argument("--manifest", type=Path, required=True)
    downloader.add_argument("--target", required=True)
    downloader.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if args.command == "resolve":
        result = resolve(args.core, args.panel, args.include_linux)
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
        print(f"Resolved CLIProxyAPI {result['core']['tag']} and management UI {result['panel']['tag']}")
    else:
        download(json.loads(args.manifest.read_text(encoding="utf-8")), args.target, args.output)
        print(f"Verified official inputs for {args.target}")


if __name__ == "__main__":
    main()
