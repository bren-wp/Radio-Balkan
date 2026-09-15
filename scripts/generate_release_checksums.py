#!/usr/bin/env python3
from __future__ import annotations

import hashlib
import os
import sys
from pathlib import Path

CHUNK_SIZE = 1024 * 1024


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(CHUNK_SIZE), b""):
            digest.update(chunk)
    return digest.hexdigest()


def write_manifest(directory: Path, version: str) -> Path:
    directory = directory.resolve()
    if not directory.is_dir():
        raise ValueError(f"Release asset directory does not exist: {directory}")

    version = version.strip()
    if not version or any(char in version for char in "\\/\r\n"):
        raise ValueError("Invalid release version")

    manifest = directory / f"RadioBalkan-v{version}-SHA256.txt"
    assets = sorted(
        (
            path
            for path in directory.iterdir()
            if path.is_file() and path.name != manifest.name and not path.name.endswith(".tmp")
        ),
        key=lambda path: path.name.encode("utf-8"),
    )
    if not assets:
        raise ValueError("No release assets found for checksum manifest")

    lines: list[str] = []
    for asset in assets:
        if any(char in asset.name for char in "\r\n"):
            raise ValueError(f"Unsupported release asset filename: {asset.name!r}")
        lines.append(f"{sha256_file(asset)}  {asset.name}\n")

    temporary = manifest.with_name(manifest.name + ".tmp")
    try:
        temporary.write_text("".join(lines), encoding="ascii", newline="\n")
        os.replace(temporary, manifest)
    finally:
        temporary.unlink(missing_ok=True)
    return manifest


def main(argv: list[str]) -> int:
    if len(argv) != 3:
        print("Usage: generate_release_checksums.py <asset-directory> <version>", file=sys.stderr)
        return 2
    try:
        manifest = write_manifest(Path(argv[1]), argv[2])
    except (OSError, ValueError) as exc:
        print(f"Checksum manifest generation failed: {exc}", file=sys.stderr)
        return 1
    print(manifest)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
