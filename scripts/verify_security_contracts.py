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
        "StreamResolver.isSafeHttp(repaired.url)",
        "pendingPlayer == mp || player == mp",
        "setInstanceFollowRedirects(false)",
    )
    require(
        "apps/android/app/src/main/java/net/radiobalkan/app/StreamResolver.java",
        "MAX_REDIRECTS = 4",
        "setInstanceFollowRedirects(false)",
        "return isSafeHttp(next) ? probe(next, depth + 1) : null;",
    )
    require(
        "apps/android/app/src/test/java/net/radiobalkan/app/StreamResolverTest.java",
        "rejectsPrivateCredentialedAndMetadataTargets",
        "https://user:pass@example.com/live",
        "http://169.254.169.254/latest/meta-data/",
    )
    forbid(
        "apps/android/app/src/main/java/net/radiobalkan/app/StreamResolver.java",
        "setInstanceFollowRedirects(true)",
    )

    require(
        "extensions/platform/chromium/service_worker.js",
        "'use strict';",
        "let offscreenCreating = null;",
    )
    require(
        "extensions/platform/chromium/offscreen.js",
        "'use strict';",
        "let audio = null;",
        "let generation = 0;",
        "function disposeAudio(",
        "function createAudio(token, candidate)",
        "document.createElement('audio')",
        "if (token !== generation || audio !== instance) return false;",
        "instance.onerror = () => playbackFailed(token, instance);",
        "instance.onended = () => playbackFailed(token, instance);",
    )
    require(
        "extensions/platform/firefox/background-firefox.js",
        "'use strict';",
        "let audio = null;",
        "let generation = 0;",
        "function disposeAudio(",
        "function createAudio(token, candidate)",
        "document.createElement('audio')",
        "if (token !== generation || audio !== instance) return false;",
        "instance.onerror = () => playbackFailed(token, instance);",
        "instance.onended = () => playbackFailed(token, instance);",
    )
    forbid(
        "extensions/platform/chromium/offscreen.js",
        "window.generation",
        "document.getElementById('audio')",
    )
    forbid(
        "extensions/platform/firefox/background-firefox.js",
        "document.getElementById('audio')",
    )
    forbid(
        "extensions/platform/chromium/offscreen.html",
        '<audio id="audio">',
    )
    forbid(
        "extensions/platform/firefox/background.html",
        '<audio id="audio">',
    )

    require(
        "apps/windows/portable/main.go",
        "if u.User != nil {",
    )
    require(
        "apps/windows/portable/main_test.go",
        "TestSafeHTTPURLRejectsPrivateAndCredentialedTargets",
        "http://127.0.0.1/live",
        "http://10.0.0.4/live",
        "http://192.168.1.10/live",
        "https://user:pass@example.com/live",
        "TestValidateStateDropsUnsafeReplacementURLs",
    )

    print("Security contracts OK")


if __name__ == "__main__":
    main()
