#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from collections.abc import Callable
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SEMVER = re.compile(r"^(\d+)\.(\d+)\.(\d+)$")
BUMP_COMMAND = re.compile(r"python scripts/bump_version\.py \d+\.\d+\.\d+")


class VersionToolError(ValueError):
    pass


def parse_semver(value: str) -> tuple[int, int, int]:
    match = SEMVER.fullmatch(value.strip())
    if not match:
        raise VersionToolError("Version must use MAJOR.MINOR.PATCH numeric format without a leading v")
    return tuple(int(part) for part in match.groups())


def next_patch(value: str) -> str:
    major, minor, patch = parse_semver(value)
    return f"{major}.{minor}.{patch + 1}"


def replace_once_text(text: str, pattern: str, replacement: str, label: str, *, flags: int = 0) -> str:
    updated, count = re.subn(pattern, replacement, text, count=1, flags=flags)
    if count != 1:
        raise VersionToolError(f"{label}: expected exactly one version marker, found {count}")
    return updated


def replace_all_text(text: str, pattern: str, replacement: str, label: str, *, minimum: int = 1) -> str:
    updated, count = re.subn(pattern, replacement, text)
    if count < minimum:
        raise VersionToolError(f"{label}: expected at least {minimum} version markers, found {count}")
    return updated


def prepare_updates(root: Path, version: str, explicit_android_code: int | None = None) -> tuple[dict[Path, str], int]:
    root = root.resolve()
    version = version.strip()
    new_semver = parse_semver(version)

    version_file = root / "VERSION"
    previous_version = version_file.read_text(encoding="utf-8").strip()
    previous_semver = parse_semver(previous_version)
    if new_semver <= previous_semver:
        raise VersionToolError(f"New version {version} must be greater than current VERSION {previous_version}")

    android_gradle = root / "apps/android/app/build.gradle.kts"
    gradle_text = android_gradle.read_text(encoding="utf-8")
    match = re.search(r"versionCode\s*=\s*(\d+)", gradle_text)
    if not match:
        raise VersionToolError("apps/android/app/build.gradle.kts: Android versionCode marker was not found")
    current_android_code = int(match.group(1))
    android_code = explicit_android_code if explicit_android_code is not None else current_android_code + 1
    if android_code <= current_android_code:
        raise VersionToolError(f"Android versionCode must increase above {current_android_code}")

    updates: dict[Path, str] = {version_file: version + "\n"}

    windows = root / "apps/windows/build-release.ps1"
    windows_text = windows.read_text(encoding="utf-8")
    updates[windows] = replace_once_text(
        windows_text,
        r'\[string\]\$Version\s*=\s*"[^"]+"',
        f'[string]$Version = "{version}"',
        "apps/windows/build-release.ps1",
    )

    for relative in ("apps/windows/portable/main.go", "apps/windows/setup/main.go"):
        source = root / relative
        updates[source] = replace_once_text(
            source.read_text(encoding="utf-8"),
            r'var appVersion\s*=\s*"[^"]+"',
            f'var appVersion = "{version}"',
            relative,
        )

    gradle_text = replace_once_text(
        gradle_text,
        r"versionCode\s*=\s*\d+",
        f"versionCode = {android_code}",
        "apps/android/app/build.gradle.kts versionCode",
    )
    gradle_text = replace_once_text(
        gradle_text,
        r'versionName\s*=\s*"[^"]+"',
        f'versionName = "{version}"',
        "apps/android/app/build.gradle.kts versionName",
    )
    updates[android_gradle] = gradle_text

    app_info = root / "apps/android/app/src/main/java/net/radiobalkan/app/AppInfo.java"
    updates[app_info] = replace_once_text(
        app_info.read_text(encoding="utf-8"),
        r'VERSION\s*=\s*"[^"]+"',
        f'VERSION = "{version}"',
        "apps/android/app/src/main/java/net/radiobalkan/app/AppInfo.java",
    )

    extension_version = root / "extensions/version.json"
    extension_data = json.loads(extension_version.read_text(encoding="utf-8"))
    extension_data["version"] = version
    updates[extension_version] = json.dumps(extension_data, ensure_ascii=False, indent=2) + "\n"

    escaped_previous = re.escape(previous_version)
    readme = root / "README.md"
    readme_text = replace_all_text(
        readme.read_text(encoding="utf-8"),
        rf"(?<!\d){escaped_previous}(?!\d)",
        version,
        "README.md current version",
        minimum=2,
    )
    readme_text = replace_once_text(
        readme_text,
        BUMP_COMMAND.pattern,
        f"python scripts/bump_version.py {next_patch(version)}",
        "README.md next bump example",
    )
    updates[readme] = readme_text

    badge = root / "assets/badges/version.svg"
    updates[badge] = replace_all_text(
        badge.read_text(encoding="utf-8"),
        rf"(?<!\d){escaped_previous}(?!\d)",
        version,
        "assets/badges/version.svg",
        minimum=2,
    )

    build_doc = root / "docs/BUILD.md"
    updates[build_doc] = replace_once_text(
        build_doc.read_text(encoding="utf-8"),
        BUMP_COMMAND.pattern,
        f"python scripts/bump_version.py {next_patch(version)}",
        "docs/BUILD.md next bump example",
    )

    return updates, android_code


def write_atomic(path: Path, text: str) -> None:
    temporary = path.with_name(path.name + ".version-tmp")
    try:
        temporary.write_text(text, encoding="utf-8", newline="\n")
        os.replace(temporary, path)
    finally:
        temporary.unlink(missing_ok=True)


def apply_updates_transactionally(
    updates: dict[Path, str],
    validator: Callable[[], int] | None = None,
) -> None:
    originals = {path: path.read_text(encoding="utf-8") for path in updates}
    try:
        for path, text in updates.items():
            write_atomic(path, text)
        if validator is not None:
            result = validator()
            if result:
                raise VersionToolError(f"Post-bump validation failed with exit code {result}")
    except BaseException:
        for path, text in originals.items():
            write_atomic(path, text)
        raise


def run_version_check(root: Path) -> int:
    check = subprocess.run([sys.executable, str(root / "scripts/check_versions.py")], cwd=root, check=False)
    return check.returncode


def main() -> None:
    parser = argparse.ArgumentParser(description="Synchronize Radio Balkan version metadata across the monorepo.")
    parser.add_argument("version", help="semantic version without a leading v, for example 0.0.13")
    parser.add_argument("--android-code", type=int, help="explicit Android versionCode; defaults to current + 1")
    parser.add_argument("--dry-run", action="store_true", help="validate the bump and print the planned files without writing them")
    args = parser.parse_args()

    try:
        updates, android_code = prepare_updates(ROOT, args.version, args.android_code)
        if args.dry_run:
            print(f"Radio Balkan version bump is valid: {args.version} (Android versionCode {android_code})")
            for path in updates:
                print(path.relative_to(ROOT))
            return
        apply_updates_transactionally(updates, lambda: run_version_check(ROOT))
    except (OSError, json.JSONDecodeError, VersionToolError) as exc:
        raise SystemExit(str(exc)) from exc

    print(f"Radio Balkan version updated to {args.version} (Android versionCode {android_code})")


if __name__ == "__main__":
    main()
