#!/usr/bin/env python3
from __future__ import annotations

import json
import shutil
import subprocess
import tempfile
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SHARED = ROOT / "shared"
PLATFORM = ROOT / "platform"
MANIFESTS = ROOT / "manifests"
DIST = ROOT / "dist"
VERSION = json.loads((ROOT / "version.json").read_text(encoding="utf-8"))["version"]
EXPECTED_BRAND = "Radio Balkan"
FORBIDDEN_PERMISSIONS = {"unlimitedStorage"}
BROWSERS = {
    "chrome": ("Chrome", "chromium"),
    "edge": ("Edge", "chromium"),
    "opera": ("Opera", "chromium"),
    "firefox": ("Firefox", "firefox"),
}


def validate_js(folder: Path) -> None:
    node = shutil.which("node")
    if not node:
        return
    for js in sorted(folder.glob("*.js")):
        result = subprocess.run([node, "--check", str(js)], capture_output=True, text=True)
        if result.returncode:
            raise SystemExit(f"JS syntax error in {js.name}:\n{result.stderr}")


def require_fragments(path: Path, fragments: tuple[str, ...]) -> None:
    text = path.read_text(encoding="utf-8")
    missing = [fragment for fragment in fragments if fragment not in text]
    if missing:
        joined = ", ".join(repr(item) for item in missing)
        raise SystemExit(f"{path.name}: missing player-state contract: {joined}")


def validate_player_contract(browser: str, target: Path) -> None:
    if browser == "firefox":
        require_fragments(
            target / "background-firefox.js",
            (
                "'use strict';",
                "const state = { station: null, playing: false };",
                "let candidates = [];",
                "let idx = 0;",
                "let generation = 0;",
                "if (token !== generation) return false;",
            ),
        )
        return

    require_fragments(
        target / "service_worker.js",
        (
            "'use strict';",
            "let offscreenCreating = null;",
            "if (!offscreenCreating)",
        ),
    )
    require_fragments(
        target / "offscreen.js",
        (
            "'use strict';",
            "let station = null;",
            "let candidates = [];",
            "let index = 0;",
            "let generation = 0;",
            "let playing = false;",
            "if (token !== generation) return false;",
        ),
    )


def materialize(browser: str, platform: str, target: Path) -> None:
    target.mkdir(parents=True, exist_ok=True)
    for item in SHARED.iterdir():
        if item.is_file():
            shutil.copy2(item, target / item.name)
    for item in (PLATFORM / platform).iterdir():
        if item.is_file():
            shutil.copy2(item, target / item.name)
    manifest = json.loads((MANIFESTS / f"{browser}.json").read_text(encoding="utf-8"))
    if manifest.get("name") != EXPECTED_BRAND:
        raise SystemExit(f"{browser}: brand name must remain {EXPECTED_BRAND!r}")
    action = manifest.get("action") or manifest.get("browser_action") or {}
    if action.get("default_title") != EXPECTED_BRAND:
        raise SystemExit(f"{browser}: toolbar brand must remain {EXPECTED_BRAND!r}")
    if "options_ui" in manifest or "options_page" in manifest:
        raise SystemExit(f"{browser}: production extension must not expose branding/settings pages")
    bad_permissions = FORBIDDEN_PERMISSIONS.intersection(manifest.get("permissions", []))
    if bad_permissions:
        raise SystemExit(f"{browser}: forbidden permissions: {', '.join(sorted(bad_permissions))}")
    manifest["version"] = VERSION
    (target / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    validate_js(target)
    if not (target / "popup.html").exists() or not (target / "catalog.js").exists() or not (target / "network.js").exists():
        raise SystemExit(f"{browser}: shared production UI missing")
    popup = (target / "popup.html").read_text(encoding="utf-8")
    if "<title>Radio Balkan</title>" not in popup or "<strong>Radio Balkan</strong>" not in popup:
        raise SystemExit(f"{browser}: locked Radio Balkan branding missing from popup")
    if browser == "firefox":
        required = ("background.html", "background-firefox.js")
    else:
        required = ("service_worker.js", "offscreen.html", "offscreen.js")
    missing = [name for name in required if not (target / name).exists()]
    if missing:
        raise SystemExit(f"{browser}: missing platform files: {', '.join(missing)}")
    validate_player_contract(browser, target)


def package_folder(folder: Path, target: Path) -> None:
    with zipfile.ZipFile(target, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for item in sorted(folder.iterdir()):
            if item.is_file():
                archive.write(item, item.name)
    with zipfile.ZipFile(target) as archive:
        bad = archive.testzip()
        if bad:
            raise SystemExit(f"Corrupt ZIP member: {bad}")


def main() -> None:
    if DIST.exists():
        shutil.rmtree(DIST)
    DIST.mkdir(parents=True)
    with tempfile.TemporaryDirectory(prefix="radiobalkan-ext-") as temp:
        temp_root = Path(temp)
        for browser, (display, platform) in BROWSERS.items():
            folder = temp_root / browser
            materialize(browser, platform, folder)
            target = DIST / f"RadioBalkan-{display}-Extension-v{VERSION}.zip"
            package_folder(folder, target)
            print(target)


if __name__ == "__main__":
    main()
