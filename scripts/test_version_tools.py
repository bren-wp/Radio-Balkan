#!/usr/bin/env python3
from __future__ import annotations

import json
import tempfile
from pathlib import Path

from bump_version import VersionToolError, apply_updates_transactionally, next_patch, prepare_updates


def write(root: Path, relative: str, content: str) -> Path:
    path = root / relative
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    return path


def build_fixture(root: Path) -> None:
    write(root, "VERSION", "1.2.3\n")
    write(
        root,
        "apps/windows/build-release.ps1",
        'param(\n    [string]$Version = "1.2.3"\n)\n-X main.appVersion=$Version\n-X main.appVersion=$Version\n',
    )
    write(root, "apps/windows/portable/main.go", 'package main\nvar appVersion = "1.2.3"\n')
    write(root, "apps/windows/setup/main.go", 'package main\nvar appVersion = "1.2.3"\n')
    write(
        root,
        "apps/android/app/build.gradle.kts",
        'defaultConfig {\n    versionCode = 17\n    versionName = "1.2.3"\n}\n',
    )
    write(
        root,
        "apps/android/app/src/main/java/net/radiobalkan/app/AppInfo.java",
        'public final class AppInfo { public static final String VERSION = "1.2.3"; }\n',
    )
    write(root, "extensions/version.json", json.dumps({"version": "1.2.3"}, indent=2) + "\n")
    write(
        root,
        "README.md",
        'version 1.2.3\nGotovi v1.2.3: /releases/tag/v1.2.3\npython scripts/bump_version.py 1.2.4\n',
    )
    write(root, "assets/badges/version.svg", 'version: 1.2.3\n>1.2.3<\n')
    write(root, "docs/BUILD.md", 'python scripts/bump_version.py 1.2.4\n')


def snapshot(updates: dict[Path, str]) -> dict[Path, str]:
    return {path: path.read_text(encoding="utf-8") for path in updates}


def assert_no_temp_files(root: Path) -> None:
    leftovers = list(root.rglob("*.version-tmp"))
    assert not leftovers, leftovers


def main() -> None:
    assert next_patch("1.2.3") == "1.2.4"
    assert next_patch("1.9.9") == "1.9.10"

    with tempfile.TemporaryDirectory() as temp:
        root = Path(temp)
        build_fixture(root)

        updates, android_code = prepare_updates(root, "1.2.4")
        assert android_code == 18
        assert (root / "VERSION").read_text(encoding="utf-8") == "1.2.3\n", "prepare must be read-only"

        apply_updates_transactionally(updates, lambda: 0)
        assert (root / "VERSION").read_text(encoding="utf-8") == "1.2.4\n"
        assert '[string]$Version = "1.2.4"' in (root / "apps/windows/build-release.ps1").read_text(encoding="utf-8")
        assert 'var appVersion = "1.2.4"' in (root / "apps/windows/portable/main.go").read_text(encoding="utf-8")
        assert 'var appVersion = "1.2.4"' in (root / "apps/windows/setup/main.go").read_text(encoding="utf-8")
        gradle = (root / "apps/android/app/build.gradle.kts").read_text(encoding="utf-8")
        assert "versionCode = 18" in gradle
        assert 'versionName = "1.2.4"' in gradle
        assert 'VERSION = "1.2.4"' in (root / "apps/android/app/src/main/java/net/radiobalkan/app/AppInfo.java").read_text(encoding="utf-8")
        assert json.loads((root / "extensions/version.json").read_text(encoding="utf-8"))["version"] == "1.2.4"
        readme = (root / "README.md").read_text(encoding="utf-8")
        assert "version 1.2.4" in readme
        assert "Gotovi v1.2.4: /releases/tag/v1.2.4" in readme
        assert "python scripts/bump_version.py 1.2.5" in readme
        assert "python scripts/bump_version.py 1.2.5" in (root / "docs/BUILD.md").read_text(encoding="utf-8")
        assert_no_temp_files(root)

        for invalid in ("1.2.4", "1.2.3", "1.1.99"):
            try:
                prepare_updates(root, invalid)
            except VersionToolError:
                pass
            else:
                raise AssertionError(f"non-increasing version must be rejected: {invalid}")

        try:
            prepare_updates(root, "1.2.5", explicit_android_code=18)
        except VersionToolError:
            pass
        else:
            raise AssertionError("non-increasing Android versionCode must be rejected")

        rollback_updates, rollback_code = prepare_updates(root, "1.2.5", explicit_android_code=25)
        assert rollback_code == 25
        originals = snapshot(rollback_updates)
        try:
            apply_updates_transactionally(rollback_updates, lambda: 7)
        except VersionToolError:
            pass
        else:
            raise AssertionError("validator failure must abort the version transaction")
        assert snapshot(rollback_updates) == originals, "failed validation must restore every original file"
        assert_no_temp_files(root)

    print("Version tooling regression tests OK")


if __name__ == "__main__":
    main()
