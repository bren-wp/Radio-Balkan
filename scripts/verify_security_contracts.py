#!/usr/bin/env python3
from __future__ import annotations

from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def require(path: str, *fragments: str) -> None:
    target = ROOT / path
    text = target.read_text(encoding="utf-8")
    missing = [fragment for fragment in fragments if fragment not in text]
    if missing:
        joined = ", ".join(repr(item) for item in missing)
        raise SystemExit(f"{path}: missing required security contract: {joined}")


def forbid(path: str, *fragments: str) -> None:
    target = ROOT / path
    text = target.read_text(encoding="utf-8")
    found = [fragment for fragment in fragments if fragment in text]
    if found:
        joined = ", ".join(repr(item) for item in found)
        raise SystemExit(f"{path}: forbidden security regression detected: {joined}")


def main() -> None:
    require(
        "apps/android/app/src/main/AndroidManifest.xml",
        'android:name="net.radiobalkan.app.permission.INTERNAL_STATE"',
        'android:protectionLevel="signature"',
        'android:name=".RadioPlayerService"',
        'android:exported="false"',
        'android:allowBackup="false"',
        'android:largeHeap="false"',
    )
    require(
        "apps/android/app/src/main/java/net/radiobalkan/app/MainActivity.java",
        '@android.annotation.SuppressLint("UnspecifiedRegisterReceiverFlag")',
        "RadioPlayerService.INTERNAL_STATE_PERMISSION",
        "Context.RECEIVER_NOT_EXPORTED",
    )
    require(
        "apps/android/app/src/main/java/net/radiobalkan/app/RadioPlayerService.java",
        'INTERNAL_STATE_PERMISSION = "net.radiobalkan.app.permission.INTERNAL_STATE"',
        "sendBroadcast(i, INTERNAL_STATE_PERMISSION);",
    )

    require(
        "extensions/platform/chromium/service_worker.js",
        "'use strict';",
        "let offscreenCreating = null;",
    )
    require(
        "extensions/platform/chromium/offscreen.js",
        "'use strict';",
        "let generation = 0;",
        "if (token !== generation) return false;",
    )
    require(
        "extensions/platform/firefox/background-firefox.js",
        "'use strict';",
        "let generation = 0;",
        "if (token !== generation) return false;",
    )

    require(
        "apps/windows/portable/main_test.go",
        "TestSafeHTTPURLRejectsPrivateAndCredentialedTargets",
        "http://127.0.0.1/live",
        "http://10.0.0.4/live",
        "http://192.168.1.10/live",
        "TestValidateStateDropsUnsafeReplacementURLs",
    )
    forbid(
        "extensions/platform/chromium/offscreen.js",
        "window.generation",
    )

    print("Security contracts OK")


if __name__ == "__main__":
    main()
