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
        "RadioPlayerService.EXTRA_STOPPED",
        "PlaybackLifecycle.uiCommand",
        "playbackStopped",
        "Context.RECEIVER_NOT_EXPORTED",
        "ui.removeCallbacksAndMessages(null)",
        "source.size() > 16",
        "}, 8000);",
    )
    require(
        "apps/android/app/src/main/java/net/radiobalkan/app/AdminAuth.java",
        'private static final String USERNAME = "brendigo";',
        'private static final String SALT = "RadioBalkanAdmin:v1:";',
        "MessageDigest.isEqual",
        "79cf893dcfdb18ecc6eba591896f896c5dd3eab95354d4e7e4503d13292fe9a0",
    )
    forbid(
        "apps/android/app/src/main/java/net/radiobalkan/app/AdminAuth.java",
        "brendigo" + "2025",
    )
    require(
        "apps/android/app/src/main/java/net/radiobalkan/app/MainActivity.java",
        "private boolean adminMode;",
        "private boolean requireAdmin()",
        "private void showAdminLogin()",
        "InputType.TYPE_TEXT_VARIATION_PASSWORD",
        'itemList.add("Admin prijava")',
        'itemList.add("Odjava administratora")',
        'if (adminMode) {',
        'chosen.equals("Web stranica") && requireAdmin()',
        'chosen.equals("Odaberi drugi izvor") && requireAdmin()',
        "if (!requireAdmin()) return;",
        "adminLockedUntilMs = System.currentTimeMillis() + 30_000L",
        'if ("replaced".equals(tab) || "broken".equals(tab))',
        "if (clickNow < adminLockedUntilMs)",
    )
    forbid(
        "apps/android/app/src/main/java/net/radiobalkan/app/MainActivity.java",
        "brendigo" + "2025",
    )
    require(
        "apps/android/app/src/test/java/net/radiobalkan/app/AdminAuthTest.java",
        "acceptsOnlyConfiguredAdministratorCredentials",
    )
    forbid(
        "apps/android/app/src/test/java/net/radiobalkan/app/AdminAuthTest.java",
        "brendigo" + "2025",
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
        'EXTRA_STOPPED = "stopped"',
        "private boolean explicitlyStopped = true;",
        "PlaybackLifecycle.canResume",
        "explicitlyStopped = true;",
        "explicitlyStopped = false;",
        "i.putExtra(EXTRA_STOPPED, stopped);",
    )
    require(
        "apps/android/app/src/main/java/net/radiobalkan/app/PlaybackLifecycle.java",
        "enum UiCommand { PLAY, PAUSE, RESUME }",
        "static UiCommand uiCommand",
        "static boolean canResume",
        "if (!sameStation || explicitlyStopped) return UiCommand.PLAY;",
        "return !explicitlyStopped && (hasPlayer || hasCandidates);",
    )
    require(
        "apps/android/app/src/test/java/net/radiobalkan/app/PlaybackLifecycleTest.java",
        "uiCommandDistinguishesPauseResumeAndRestart",
        "explicitStopCannotResumeAnOldServiceSession",
        "pausedOrRecoverablePlaybackCanResume",
        "stopThenPlayUsesFreshSessionInsteadOfResume",
        "navigationRequiresCurrentStationAndMultipleCandidates",
        "adjacentNavigationWrapsAndHandlesMissingSelection",
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
        "MAX_REFRESH_RESPONSE_BYTES = 512 * 1024",
        "REFRESH_TOTAL_TIMEOUT_MS = 9000",
        "RADIO_BROWSER_API_BASES",
        "async function refreshCandidateUrls(station)",
        "redirect: 'error'",
        "BALKAN.has(actualCountry)",
    )
    require(
        "scripts/test_browser_network_contract.js",
        "IPv6 unspecified target must be rejected",
        "IPv4-mapped IPv6 loopback must be rejected",
        "IPv4-mapped IPv6 private target must be rejected",
        "IPv6 unique-local target must be rejected",
        "full IPv6 link-local fe80/10 range must be rejected",
        "IPv6 multicast target must be rejected",
        "UUID refresh must return only safe public streams",
        "UUID refresh must reject a regional station returned under another country",
        "foreign refresh must never remap a Balkan station into the INT group",
        "oversized refresh responses must fail closed",
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
        "sessionId: currentSessionId",
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
        "function bindAudioHandlers(instance, token, expectedSession)",
        "function createAudio(token, expectedSession, candidate)",
        "async function resumeCurrent(expectedSession)",
        "document.createElement('audio')",
        "sessionId !== expectedSession",
        "instance.onerror = () => playbackFailed(token, expectedSession, instance);",
        "instance.onended = () => playbackFailed(token, expectedSession, instance);",
        "PLAY_START_TIMEOUT_MS = 12_000",
        "STALL_RECOVERY_TIMEOUT_MS = 15_000",
        "async function playWithTimeout(instance)",
        "instance.onwaiting = () => scheduleStallRecovery(token, expectedSession, instance);",
        "instance.onstalled = () => scheduleStallRecovery(token, expectedSession, instance);",
        "let refreshAttempted = false;",
        "RBNet.refreshCandidateUrls(expectedStation)",
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
        "function bindAudioHandlers(instance, token, expectedSession)",
        "function createAudio(token, expectedSession, candidate)",
        "async function resumeCurrent(expectedSession)",
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
        "let refreshAttempted = false;",
        "RBNet.refreshCandidateUrls(expectedStation)",
        "currentSessionId = null;",
        "candidates = [];",
        "idx = 0;",
    )
    require(
        "extensions/shared/popup.js",
        "let commandGeneration = 0;",
        "let activeCommandToken = 0;",
        "let stopped = true;",
        "let stateEpoch = '';",
        "let lastRevision = -1;",
        "let stateSyncPromise = null;",
        "const retiredEpochs = new Set();",
        "function rememberRetiredEpoch(epoch)",
        "function acceptStateEnvelope(value, allowEpochChange = false)",
        "retiredEpochs.has(epoch)",
        "if (!allowEpochChange) return false;",
        "function synchronizePlayerState()",
        "if (stopped) return play(current);",
        "else if (!value.sessionId && !value.playing) current = null;",
        "ext.runtime.sendMessage({ type: 'RB_GET_STATE' })",
        "void synchronizePlayerState();",
        "if (revision < lastRevision) return false;",
        "if (token !== commandGeneration) return;",
        "message?.type !== 'RB_STATE' || activeCommandToken",
        "$('playerState').textContent = playerStatus;",
        "async function stopPlayback()",
        "async function playAdjacent(delta)",
        "navigationStations()",
        "ext.runtime.sendMessage({ type: 'RB_STOP' })",
        "playerStop').setAttribute('aria-busy'",
        "const ADMIN_DIGEST = '79cf893dcfdb18ecc6eba591896f896c5dd3eab95354d4e7e4503d13292fe9a0';",
        "async function adminCredentialsValid",
        "crypto.subtle.digest('SHA-256'",
        "let adminMode = false;",
        "RB.adminOverrideFor(RB.key(station))",
        "RB.setAdminOverride(RB.key(current), value)",
        "adminLockedUntil = Date.now() + 30000",
    )
    forbid(
        "extensions/shared/popup.js",
        "brendigo" + "2025",
    )
    require(
        "extensions/shared/popup.html",
        'id="playerState"',
        'id="playerPrev"',
        'id="playerStop"',
        'id="playerNext"',
        'id="adminPanel"',
        'id="adminPassword" type="password"',
        'id="adminSource"',
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
        "duplicate stop clicks must be ignored while stop is in flight",
        "completed stop command must render an explicit stopped state",
        "player stop must stay disabled once playback is already stopped",
        "previous must disable when the active filter leaves only one visible station",
        "next from Radio B must select the following visible station",
        "previous from Radio B must select the preceding visible station",
        "next control must start the adjacent station",
        "main play control after stop must send a fresh RB_PLAY command",
        "authoritative cold state must clear the optimistic catalog selection",
        "cold adjacent controls must not start playback without a current station",
        "unfavoriting inside favorites view must immediately remove the station from the visible set",
        "advanced source controls must stay hidden before admin login",
        "saved admin source override must be applied without exposing it in the normal UI",
    )
    require(
        "scripts/test_chromium_player_contract.js",
        "stale stop must not close the offscreen document used by a newer play",
        "new play must wait until the previous offscreen close completes",
        "play issued during close must recover into active playback",
        "Chromium stopped state must expose a retired session",
    )
    require(
        "scripts/test_chromium_offscreen_contract.js",
        "pause must not recreate the audio element",
        "resume must reuse the paused audio element",
        "stop must retire the offscreen session",
        "a hanging first candidate must fall back to the next stream",
        "stalled active audio must recover to a fallback candidate",
        "play after stop must create fresh Chromium playback",
        "replay after stop must use the newly requested Chromium session",
        "candidate exhaustion must recover through a refreshed station URL",
        "catalog refresh finishing after stop must be rejected as stale",
        "refreshed Chromium stream must become the active session URL",
    )
    require(
        "scripts/test_firefox_player_contract.js",
        "stop must terminate the Firefox playback session",
        "toggle after stop must not revive the stopped session",
        "pause must not recreate the Firefox audio element",
        "resume must reuse the paused Firefox audio element",
        "superseded play request must be marked stale",
        "slow old playback must not replace the newer station",
        "a hanging first candidate must recover to the next Firefox stream",
        "Firefox stalled audio must recover to the next candidate",
        "play after stop must create fresh Firefox playback",
        "replay after stop must not reuse the retired Firefox session",
        "Firefox candidate exhaustion must recover through a refreshed station URL",
        "Firefox must reject a catalog refresh that finishes after stop",
        "refreshed Firefox stream must become the active session URL",
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
        "Windows Portable go vet nije uspio.",
        "Windows Portable go test nije uspio.",
        "Windows Portable go build nije uspio.",
        "Windows Setup go vet nije uspio.",
        "Windows Setup go test nije uspio.",
        "Windows Setup go build nije uspio.",
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
        "time.After(2500 * time.Millisecond)",
        "shutdown-timeout",
        "Do not block the UI thread acquiring app.mu during shutdown.",
        "func runtimeTestTrace(scope string)",
        "func scheduleCIRuntimeSmokeClose()",
        'runtimeTestTrace("ci-smoke-post-wm-close")',
        '"runtime-test-stacks"',
        "runtime.Stack(buf, true)",
        'runtimeTestTrace("wm-close-enter")',
        'runtimeTestTrace("wm-destroy-before-post-quit")',
        "func writeFileDurable(",
        "return f.Sync()",
        "writeFileDurable(tmp, b, 0644)",
        "func scheduleStartupHealth(limit int)",
        "time.NewTimer(8 * time.Second)",
        "audioAck",
        "func resetAudioEngineLocked()",
        "func audioCommandTimeout(line string) time.Duration",
        "return 5 * time.Second",
        "func waitAudioAckLocked(timeout time.Duration) error",
        "A late ACK must never be consumed by the next command.",
        "resetAudioEngineLocked()",
        "audio engine nije odgovorio na vrijeme",
        "READY",
        "time.After(15 * time.Second)",
        "func warmAudioEngine()",
        'safeGo("audio-warmup", warmAudioEngine)',
        "audio engine startup timeout",
        "audioStopped",
        "runtime.LockOSThread()",
        "defer runtime.UnlockOSThread()",
        "func decidePlaybackToggle(current int, playing, stopped bool) playbackToggleAction",
        "func defaultPlaybackIndexLocked() int",
        "playStation(defaultIndex)",
        "playbackToggleReconnect",
        "Kind: hitPlayerStop",
        "func canStopPlayback(current int, stopped bool) bool",
        "func canNavigateStations(current, stationCount, filteredCount int) bool",
        "canStopPlayback(currentStationIndexLocked(), app.audioStopped)",
        "playStationByKey(currentKey, current)",
        "app.audioStopped = true",
        "audioSetVolume(v)",
        "if shuttingDown() {",
        "func prepareShutdown()",
        "ES_PASSWORD",
        "func adminCredentialsValid(username, password string) bool",
        "subtle.ConstantTimeCompare",
        "func requireAdmin() bool",
        "func passwordDialog(",
        "func toggleAdminSession()",
        "if adminModeEnabled()",
        'if app.tab == "replaced" || app.tab == "broken"',
    )
    forbid(
        "apps/windows/portable/main.go",
        "brendigo" + "2025",
    )
    require(
        "apps/windows/portable/main_test.go",
        "TestAdminCredentialsAcceptOnlyConfiguredAdministrator",
    )
    forbid(
        "apps/windows/portable/main_test.go",
        "brendigo" + "2025",
    )
    require(
        "scripts/test-windows-runtime.ps1",
        "Radio Balkan runtime smoke OK",
        "Radio Balkan exited during the",
        "--ci-runtime-smoke",
        "RADIO_BALKAN_RUNTIME_TEST",
        "did not complete its CI self-close",
        "Recent runtime trace:",
        "Latest stalled stack:",
        "runtime-test-stacks",
        "Select-Object -Last 24",
    )
    forbid(
        "apps/windows/portable/main.go",
        "if playing {\n\t\taudioSetVolume(v)",
    )
    require(
        "apps/windows/setup/main.go",
        "runtime.LockOSThread()",
        "defer runtime.UnlockOSThread()",
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
        "TestPlaybackToggleDecisionLifecycle",
        "stopped reconnects",
        "TestAudioAckTimeoutDiscardsStaleChannel",
        "timed-out audio helper state was not discarded",
        "TestAudioEngineCommandLifecycle",
        "lifecycle.wav",
        "PLAY %s 0.25",
        "\"PAUSE\", \"VOLUME 0.60\", \"RESUME\", \"STOP\"",
        "silentWAV",
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
        "paths:",
        "- VERSION",
    )
    forbid(
        ".github/workflows/publish.yml",
        "- apps/**",
        "- extensions/**",
        "- scripts/check_clean_worktree.py",
        "- scripts/generate_release_checksums.py",
        "- .github/workflows/publish.yml",
    )
    require(
        ".github/workflows/screenshots.yml",
        "Verify screenshot build leaves repository clean",
        "python scripts/check_clean_worktree.py",
    )

    print("Security contracts OK")


if __name__ == "__main__":
    main()
