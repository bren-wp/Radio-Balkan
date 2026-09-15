#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SEMVER = re.compile(r"^\d+\.\d+\.\d+$")


def replace_once(path: Path, pattern: str, replacement: str, *, flags: int = 0) -> None:
    text = path.read_text(encoding="utf-8")
    updated, count = re.subn(pattern, replacement, text, count=1, flags=flags)
    if count != 1:
        raise SystemExit(f"{path.relative_to(ROOT)}: expected exactly one version marker, found {count}")
    path.write_text(updated, encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser(description="Synchronize Radio Balkan version metadata across the monorepo.")
    parser.add_argument("version", help="semantic version without a leading v, for example 0.0.6")
    parser.add_argument("--android-code", type=int, help="explicit Android versionCode; defaults to current + 1")
    args = parser.parse_args()

    version = args.version.strip()
    if not SEMVER.fullmatch(version):
        raise SystemExit("Version must use MAJOR.MINOR.PATCH numeric format without a leading v")

    android_gradle = ROOT / "apps/android/app/build.gradle.kts"
    gradle_text = android_gradle.read_text(encoding="utf-8")
    match = re.search(r"versionCode\s*=\s*(\d+)", gradle_text)
    if not match:
        raise SystemExit("Android versionCode marker was not found")
    current_android_code = int(match.group(1))
    android_code = args.android_code if args.android_code is not None else current_android_code + 1
    if android_code <= current_android_code:
        raise SystemExit(f"Android versionCode must increase above {current_android_code}")

    (ROOT / "VERSION").write_text(version + "\n", encoding="utf-8")

    replace_once(
        ROOT / "apps/windows/portable/main.go",
        r'var appVersion\s*=\s*"[^"]+"',
        f'var appVersion = "{version}"',
    )
    replace_once(
        ROOT / "apps/windows/setup/main.go",
        r'var appVersion\s*=\s*"[^"]+"',
        f'var appVersion = "{version}"',
    )
    replace_once(
        ROOT / "apps/windows/build-release.ps1",
        r'\[string\]\$Version\s*=\s*"[^"]+"',
        f'[string]$Version = "{version}"',
    )

    replace_once(android_gradle, r"versionCode\s*=\s*\d+", f"versionCode = {android_code}")
    replace_once(android_gradle, r'versionName\s*=\s*"[^"]+"', f'versionName = "{version}"')
    replace_once(
        ROOT / "apps/android/app/src/main/java/net/radiobalkan/app/AppInfo.java",
        r'VERSION\s*=\s*"[^"]+"',
        f'VERSION = "{version}"',
    )

    extension_version = ROOT / "extensions/version.json"
    data = json.loads(extension_version.read_text(encoding="utf-8"))
    data["version"] = version
    extension_version.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

    check = subprocess.run([sys.executable, str(ROOT / "scripts/check_versions.py")], cwd=ROOT)
    if check.returncode:
        raise SystemExit(check.returncode)

    print(f"Radio Balkan version updated to {version} (Android versionCode {android_code})")


if __name__ == "__main__":
    main()
