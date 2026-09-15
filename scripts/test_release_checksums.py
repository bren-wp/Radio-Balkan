#!/usr/bin/env python3
from __future__ import annotations

import hashlib
import tempfile
from pathlib import Path

from generate_release_checksums import write_manifest


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def main() -> None:
    with tempfile.TemporaryDirectory() as temp:
        root = Path(temp)
        portable = root / "RadioBalkan-Portable-v0.0.12.exe"
        browser = root / "RadioBalkan-Chrome-Extension-v0.0.12.zip"
        manifest = root / "RadioBalkan-v0.0.12-SHA256.txt"

        portable.write_bytes(b"portable-release-bytes")
        browser.write_bytes(b"browser-release-bytes")
        manifest.write_text("stale self-referential manifest\n", encoding="utf-8")

        generated = write_manifest(root, "0.0.12")
        assert generated == manifest
        text = generated.read_text(encoding="ascii")
        lines = text.splitlines()

        assert len(lines) == 2, lines
        assert generated.name not in text, "checksum manifest must never include itself"
        assert lines == sorted(lines, key=lambda line: line.split("  ", 1)[1].encode("utf-8"))
        assert f"{sha256(browser.read_bytes())}  {browser.name}" in lines
        assert f"{sha256(portable.read_bytes())}  {portable.name}" in lines
        assert not (root / (generated.name + ".tmp")).exists()

    print("Release checksum manifest regression test OK")


if __name__ == "__main__":
    main()
