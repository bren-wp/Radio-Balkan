#!/usr/bin/env python3
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SEMVER = re.compile(r"^(\d+)\.(\d+)\.(\d+)$")


def next_patch(value: str) -> str:
    match = SEMVER.fullmatch(value)
    if not match:
        raise ValueError(value)
    major, minor, patch = (int(part) for part in match.groups())
    return f"{major}.{minor}.{patch + 1}"


def require_regex(errors: list[str], relative: str, pattern: str, message: str) -> None:
    text = (ROOT / relative).read_text(encoding="utf-8")
    if not re.search(pattern, text):
        errors.append(f"{relative}: {message}")


def main() -> int:
    errors: list[str] = []
    version = (ROOT / "VERSION").read_text(encoding="utf-8").strip()
    if not SEMVER.fullmatch(version):
        print(f"VERSION: invalid semantic version {version!r}", file=sys.stderr)
        return 1
    following = next_patch(version)
    escaped = re.escape(version)

    checks = {
        "apps/windows/build-release.ps1": (rf'\$Version\s*=\s*"{escaped}"', "version mismatch"),
        "apps/windows/portable/main.go": (rf'var appVersion\s*=\s*"{escaped}"', "portable source version mismatch"),
        "apps/windows/setup/main.go": (rf'var appVersion\s*=\s*"{escaped}"', "setup source version mismatch"),
        "apps/android/app/build.gradle.kts": (rf'versionName\s*=\s*"{escaped}"', "versionName mismatch"),
        "apps/android/app/src/main/java/net/radiobalkan/app/AppInfo.java": (rf'VERSION\s*=\s*"{escaped}"', "AppInfo version mismatch"),
        "README.md": (rf'alt="version {escaped}"', "version badge alt mismatch"),
        "assets/badges/version.svg": (rf'version:\s*{escaped}', "version badge mismatch"),
    }
    for relative, (pattern, message) in checks.items():
        require_regex(errors, relative, pattern, message)

    windows_build = (ROOT / "apps/windows/build-release.ps1").read_text(encoding="utf-8")
    if windows_build.count("-X main.appVersion=$Version") < 2:
        errors.append("apps/windows/build-release.ps1: Windows appVersion linker injection missing")

    gradle_text = (ROOT / "apps/android/app/build.gradle.kts").read_text(encoding="utf-8")
    code_match = re.search(r"versionCode\s*=\s*(\d+)", gradle_text)
    if not code_match or int(code_match.group(1)) <= 0:
        errors.append("apps/android/app/build.gradle.kts: invalid Android versionCode")

    app_info = (ROOT / "apps/android/app/src/main/java/net/radiobalkan/app/AppInfo.java").read_text(encoding="utf-8")
    if 'USER_AGENT = "RadioBalkan-Android/" + VERSION' not in app_info:
        errors.append("apps/android/app/src/main/java/net/radiobalkan/app/AppInfo.java: USER_AGENT must derive from VERSION")

    extension_data = json.loads((ROOT / "extensions/version.json").read_text(encoding="utf-8"))
    if extension_data.get("version") != version:
        errors.append("extensions/version.json: version mismatch")

    for name in ("chrome", "edge", "opera", "firefox"):
        data = json.loads((ROOT / f"extensions/manifests/{name}.json").read_text(encoding="utf-8"))
        if data.get("name") != "Radio Balkan":
            errors.append(f"{name}: brand name changed")

    readme = (ROOT / "README.md").read_text(encoding="utf-8")
    if f"Gotovi v{version} artefakti" not in readme:
        errors.append("README.md: current release heading mismatch")
    if f"releases/tag/v{version}" not in readme:
        errors.append("README.md: current GitHub release link mismatch")
    readme_bumps = re.findall(r"python scripts/bump_version\.py (\d+\.\d+\.\d+)", readme)
    if readme_bumps != [following]:
        errors.append(f"README.md: next bump example must be exactly {following}, found {readme_bumps}")

    build_doc = (ROOT / "docs/BUILD.md").read_text(encoding="utf-8")
    build_bumps = re.findall(r"python scripts/bump_version\.py (\d+\.\d+\.\d+)", build_doc)
    if build_bumps != [following]:
        errors.append(f"docs/BUILD.md: next bump example must be exactly {following}, found {build_bumps}")

    badge = (ROOT / "assets/badges/version.svg").read_text(encoding="utf-8")
    if len(re.findall(rf"(?<!\d){escaped}(?!\d)", badge)) < 2:
        errors.append("assets/badges/version.svg: expected version in aria label and visible badge text")

    if errors:
        print("\n".join(errors), file=sys.stderr)
        return 1
    print(f"Radio Balkan version sync OK: {version}; next bump {following}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
