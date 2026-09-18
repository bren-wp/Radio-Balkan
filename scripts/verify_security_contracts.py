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
        "ui.removeCallbacksAndMessages(null)",
        "source.size() > 16",
        "}, 8000);",
    )
    require(
        "apps/android/app/src/main/java/net/radiobalkan/app/RadioRepository.java",
        "StreamResolver.isSafeHttpForConnection(current)",
        "setInstanceFollowRedirects(false)",
        "isTrustedApiUrl(current)",
    )
    require(
        "apps/android/app/src/main/java/net/radiobalkan/app/RadioPlayerService.java",
        'INTERNAL_STATE_PERMISSION = "net.radiobalkan.app.permission.INTERNAL_STATE"',
        "sendBroadcast(i, INTERNAL_STATE_PERMISSION);",
        "StreamResolver.isSafeHttp(repaired.url)",
        "pendingPlayer == mp || player == mp",
        "setInstanceFollowRedirects(false)",
        "if (!updateForeground(false, \"Povezujem…\"))",
        "private boolean updateForeground(",
        "audioManager.requestAudioFocus(focusListener, AudioManager.STREAM_MUSIC, AudioManager.AUDIOFOCUS_GAIN)",
    )
    require(
        "apps/android/app/src/main/java/net/radiobalkan/app/StreamResolver.java",
        "MAX_REDIRECTS = 4",
        "setInstanceFollowRedirects(false)",
        "return isSafeHttp(next) ? probe(next, depth + 1) : null;",
        "isSafeHttpForConnection",
        "InetAddress.getAllByName",
        "allAddressesSafe",
    )
    require(
        "apps/android/app/src/main/java/net/radiobalkan/app/ImageLoader.java",
        "MAX_REDIRECTS = 4",
        "setInstanceFollowRedirects(false)",
        "String next = new URL(requested, location.trim()).toString();",
        "if (!StreamResolver.isSafeHttp(next)) return null;",
        "redirects >= MAX_REDIRECTS",
    )
    require(
        "apps/android/app/src/test/java/net/radiobalkan/app/StreamResolverTest.java",
        "rejectsPrivateCredentialedAndMetadataTargets",
        "https://user:pass@example.com/live",
        "http://169.254.169.254/latest/meta-data/",
        "resolvedAddressSetRejectsAnyPrivateOrLocalTarget",
    )
    forbid(
        "apps/android/app/src/main/java/net/radiobalkan/app/StreamResolver.java",
        "setInstanceFollowRedirects(true)",
    )
    forbid(
        "apps/android/app/src/main/java/net/radiobalkan/app/ImageLoader.java",
        "setInstanceFollowRedirects(true)",
    )

    require(
        "extensions/shared/catalog.js",
        "MAX_SERVER_RESPONSE_BYTES = 512 * 1024",
        "MAX_CATALOG_RESPONSE_BYTES = 8 * 1024 * 1024",
        "MAX_DISCOVERED_API_BASES = 4",
        "MAX_API_BASES = 8",
        "async function readJsonLimited(response, maxBytes)",
        "response.body?.getReader",
        "total > maxBytes",
        "readJsonLimited(response, MAX_SERVER_RESPONSE_BYTES)",
        "readJsonLimited(response, MAX_CATALOG_RESPONSE_BYTES)",
    )
    forbid(
        "extensions/shared/catalog.js",
        "await response.json()",
        "await response.text()",
    )
    require(
        "scripts/test_browser_catalog_contract.js",
        "oversized country response must be rejected",
        "oversized response must not be parsed into the catalog",
        "dynamic API discovery must be capped before stable fallbacks are tried",
        "forced refresh must surface a real network failure",
        "UI preferences must be sanitized and persisted",
    )
    require(
        "extensions/shared/network.js",
        "h === '::'",
        "h.startsWith('::ffff:')",
        "/^fe[89ab][0-9a-f]:/",
        "/^f[cd][0-9a-f]{2}:/",
        "/^ff[0-9a-f]{2}:/",
        "!url.username && !url.password",
    )
    require(
        "scripts/test_browser_network_contract.js",
        "IPv6 unspecified target must be rejected",
        "IPv4-mapped IPv6 loopback must be rejected",
        "IPv4-mapped IPv6 private target must be rejected",
        "IPv6 unique-local target must be rejected",
        "full IPv6 link-local fe80/10 range must be rejected",
        "IPv6 multicast target must be rejected",
    )
    require(
        "extensions/platform/chromium/service_worker.js",
        "'use strict';",
        "revision: 0, epoch",
        "let offscreenCreating = null;",
        "let offscreenClosing = null;",
        "let commandGeneration = 0;",
        "let currentSessionId = null;",
        "let lastOffscreenGeneration = -1;",
        "function newSessionId()",
        "function acceptOffscreenState(",
        "incomingGeneration < lastOffscreenGeneration",
        "requestToken !== commandGeneration",
        "async function waitForOffscreenClose()",
        "async function closeOffscreen()",
        "await waitForOffscreenClose();",
        "requestToken === commandGeneration && requestedSession === currentSessionId",
    )
    require(
        "extensions/platform/chromium/offscreen.js",
        "'use strict';",
        "let audio = null;",
        "let generation = 0;",
        "let sessionId = null;",
        "function stateEnvelope(",
        "function disposeAudio(",
        "function createAudio(token, expectedSession, candidate)",
        "document.createElement('audio')",
        "sessionId !== expectedSession",
        "instance.onerror = () => playbackFailed(token, expectedSession, instance);",
        "instance.onended = () => playbackFailed(token, expectedSession, instance);",
        "PLAY_START_TIMEOUT_MS = 12_000",
        "STALL_RECOVERY_TIMEOUT_MS = 15_000",
        "async function playWithTimeout(instance)",
        "instance.onwaiting = () => scheduleStallRecovery(token, expectedSession, instance);",
        "instance.onstalled = () => scheduleStallRecovery(token, expectedSession, instance);",
    )
    require(
        "extensions/platform/firefox/background-firefox.js",
        "'use strict';",
        "revision: 0, epoch",
        "let audio = null;",
        "let generation = 0;",
        "let currentSessionId = null;",
        "function newSessionId()",
        "function commitState(",
        "function createAudio(token, expectedSession, candidate)",
        "document.createElement('audio')",
        "currentSessionId !== expectedSession",
        "instance.onerror = () => playbackFailed(token, expectedSession, instance);",
        "instance.onended = () => playbackFailed(token, expectedSession, instance);",
        "PLAY_START_TIMEOUT_MS = 12_000",
        "STALL_RECOVERY_TIMEOUT_MS = 15_000",
        "async function playWithTimeout(instance)",
        "instance.onwaiting = () => scheduleStallRecovery(token, expectedSession, instance);",
        "instance.onstalled = () => scheduleStallRecovery(token, expectedSession, instance);",
        "if (msg.type === 'RB_STOP')",
        "currentSessionId = null;",
        "candidates = [];",
        "idx = 0;",
    )
    require(
        "extensions/shared/popup.js",
        "let commandGeneration = 0;",
        "let activeCommandToken = 0;",
        "let stateEpoch = '';",
        "let lastRevision = -1;",
        "let stateSyncPromise = null;",
        "const retiredEpochs = new Set();",
        "function rememberRetiredEpoch(epoch)",
        "function acceptStateEnvelope(value, allowEpochChange = false)",
        "retiredEpochs.has(epoch)",
        "if (!allowEpochChange) return false;",
        "function synchronizePlayerState()",
        "ext.runtime.sendMessage({ type: 'RB_GET_STATE' })",
        "void synchronizePlayerState();",
        "if (revision < lastRevision) return false;",
        "if (token !== commandGeneration) return;",
        "message?.type !== 'RB_STATE' || activeCommandToken",
        "$('playerState').textContent = playerStatus;",
    )
    require(
        "extensions/shared/popup.html",
        'id="playerState"',
        'aria-live="polite"',
    )
    require(
        "scripts/test_browser_state_contract.js",
        "lower revision from the current epoch must be ignored",
        "a foreign epoch must trigger authoritative RB_GET_STATE resync",
        "retired worker epoch cannot restore stale state",
        "stale command reply from a retired epoch cannot replace current station",
        "duplicate toggle clicks must be ignored while a command is in flight",
        "busy playback state must be announced accessibly",
        "player toggle must be re-enabled after command completion",
    )
    require(
        "scripts/test_chromium_player_contract.js",
        "stale stop must not close the offscreen document used by a newer play",
        "new play must wait until the previous offscreen close completes",
        "play issued during close must recover into active playback",
    )
    require(
        "scripts/test_chromium_offscreen_contract.js",
        "a hanging first candidate must fall back to the next stream",
        "stalled active audio must recover to a fallback candidate",
    )
    require(
        "scripts/test_firefox_player_contract.js",
        "stop must terminate the Firefox playback session",
        "toggle after stop must not revive the stopped session",
        "superseded play request must be marked stale",
        "slow old playback must not replace the newer station",
        "a hanging first candidate must recover to the next Firefox stream",
        "Firefox stalled audio must recover to the next candidate",
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
        "apps/windows/build-release.ps1",
        "[IO.File]::ReadAllText",
        '.Replace("`r`n", "`n")',
        "& gofmt -w $temp",
        "nije gofmt formatiran",
    )
    require(
        "apps/windows/portable/main.go",
        "procCopyMemory.Call(uintptr(unsafe.Pointer(&info)), lParam, unsafe.Sizeof(info))",
        "procCopyMemory.Call(lParam, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))",
    )
    forbid(
        "apps/windows/portable/main.go",
        "(*MINMAXINFO)(unsafe.Pointer(lParam))",
    )
    require(
        "apps/windows/portable/main.go",
        "if u.User != nil {",
        "DialContext: safeDialContext",
        "net.DefaultResolver.LookupIPAddr",
        "unsafeNetworkIP",
        "strings.TrimRight",
        "shutdownOnce",
        "func prepareShutdown()",
        "func writeFileDurable(",
        "return f.Sync()",
        "writeFileDurable(tmp, b, 0644)",
        "func scheduleStartupHealth(limit int)",
        "time.NewTimer(8 * time.Second)",
        "audioAck",
        "func waitAudioAckLocked() error",
        "audio engine nije odgovorio na vrijeme",
        "if shuttingDown() {",
        "func prepareShutdown()",
    )
    require(
        "scripts/test-windows-runtime.ps1",
        "Radio Balkan runtime smoke OK",
        "Radio Balkan exited during the",
        "PostMessage",
        "WM_CLOSE",
    )
    require(
        "apps/windows/setup/main.go",
        "func writeFileDurable(",
        "return f.Sync()",
        "writeFileDurable(tmp, appBytes, 0755)",
        "writeFileDurable(uTmp, uninstallerBytes, 0755)",
        "writeFileDurable(iconPath(), setupIconBytes, 0644)",
    )
    require(
        "apps/windows/setup/main_test.go",
        "TestNextInstallerFocusWraps",
        "TestInstallerCheckboxHitTargetsIncludeLabels",
        "TestWriteFileDurablePersistsInstallerPayload",
    )
    require(
        "apps/windows/portable/main_test.go",
        "TestSafeHTTPURLRejectsPrivateAndCredentialedTargets",
        "http://127.0.0.1/live",
        "http://10.0.0.4/live",
        "http://192.168.1.10/live",
        "https://user:pass@example.com/live",
        "TestValidateStateDropsUnsafeReplacementURLs",
        "TestWriteFileDurablePersistsCompleteContent",
        "TestAudioEngineAcknowledgesCommand",
    )

    require(
        "scripts/check_clean_worktree.py",
        '"git", "status", "--porcelain", "--untracked-files=all"',
        "Repository workspace is not clean after build:",
        "Repository workspace clean after build",
    )
    require(
        "scripts/generate_release_checksums.py",
        "path.name != manifest.name",
        "os.replace(temporary, manifest)",
        "No release assets found for checksum manifest",
    )
    require(
        "scripts/test_release_checksums.py",
        "stale self-referential manifest",
        "checksum manifest must never include itself",
        "Release checksum manifest regression test OK",
    )
    require(
        "scripts/bump_version.py",
        "new_semver <= previous_semver",
        "def apply_updates_transactionally(",
        "def write_atomic(",
        "Post-bump validation failed with exit code",
        "README.md next bump example",
        "docs/BUILD.md next bump example",
        'for relative in ("apps/windows/portable/main.go", "apps/windows/setup/main.go")',
        'parser.add_argument("--dry-run"',
    )
    require(
        "scripts/test_version_tools.py",
        "prepare must be read-only",
        "non-increasing version must be rejected",
        "failed validation must restore every original file",
        'var appVersion = "1.2.4"',
        "Version tooling regression tests OK",
    )
    require(
        "scripts/check_versions.py",
        "current GitHub release link mismatch",
        "next bump example must be exactly",
        "USER_AGENT must derive from VERSION",
        "portable source version mismatch",
        "setup source version mismatch",
    )
    require(
        ".github/workflows/ci.yml",
        "Verify browser build leaves repository clean",
        "Verify Windows build leaves repository clean",
        "Verify Android build leaves repository clean",
        "Test browser network safety contract",
        "Test Chromium player close/session contract",
        "Test Chromium offscreen timeout/stall contract",
        "Test Firefox player timeout/stall/session contract",
        "Soak-test Windows startup runtime",
        "scripts/test-windows-runtime.ps1",
        "python scripts/check_clean_worktree.py",
        "Test release checksum manifest",
        "python scripts/test_release_checksums.py",
        "Test version tooling transactions",
        "python scripts/test_version_tools.py",
        "Validate next version bump dry-run",
        '--dry-run',
    )
    require(
        ".github/workflows/publish.yml",
        "Verify Windows build leaves repository clean",
        "Verify browser build leaves repository clean",
        "Verify Android build leaves repository clean",
        "Generate and verify checksums",
        'python scripts/generate_release_checksums.py release-assets "$VERSION"',
        'sha256sum -c "RadioBalkan-v${VERSION}-SHA256.txt"',
    )
    require(
        ".github/workflows/screenshots.yml",
        "Verify screenshot build leaves repository clean",
        "python scripts/check_clean_worktree.py",
    )

    print("Security contracts OK")


if __name__ == "__main__":
    main()
