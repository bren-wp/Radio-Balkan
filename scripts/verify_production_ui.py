#!/usr/bin/env python3
from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def require(errors: list[str], text: str, needle: str, label: str) -> None:
    if needle not in text:
        errors.append(f"{label}: missing required production UI contract: {needle!r}")


def forbid(errors: list[str], text: str, needle: str, label: str) -> None:
    if needle in text:
        errors.append(f"{label}: deprecated/developer-facing UI text or behavior found: {needle!r}")


def method_body(text: str, signature: str) -> str:
    start = text.find(signature)
    if start < 0:
        return ""
    brace = text.find("{", start)
    if brace < 0:
        return ""
    depth = 0
    for index in range(brace, len(text)):
        char = text[index]
        if char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0:
                return text[brace + 1:index]
    return ""


def main() -> int:
    errors: list[str] = []

    popup_html = read("extensions/shared/popup.html")
    popup_js = read("extensions/shared/popup.js")
    popup_css = read("extensions/shared/popup.css")

    for needle in (
        'role="list"',
        'aria-pressed="false"',
        'id="playerToggle"',
        'id="playerPrev"',
        'id="playerStop"',
        'id="playerNext"',
        'id="adminToggle"',
        'id="adminPanel"',
        'role="dialog" aria-modal="true"',
        'id="adminPassword" type="password"',
        'id="adminSource"',
        'id="clearFilters"',
        'disabled>▶</button>',
        'Nije odabrano',
    ):
        require(errors, popup_html, needle, "extensions/shared/popup.html")
    forbid(errors, popup_html, '>Ništa<', "extensions/shared/popup.html")

    for needle in (
        "function syncStationPlaybackUi()",
        "refresh.setAttribute('aria-busy'",
        "list.setAttribute('aria-busy'",
        "const previous = country.value",
        "country.value = RB.COUNTRIES.some",
        "Nije moguće osvježiti · prikazan je postojeći popis",
        "function resetFilters()",
        "function queueUiPreferencesSave()",
        "row.tabIndex = 0",
        "list.addEventListener('keydown'",
        "playerFav').setAttribute('aria-pressed'",
        "if (activeCommandToken) return;",
        "playerToggle').setAttribute('aria-busy'",
        "playerStop').setAttribute('aria-busy'",
        "function stopPlayback()",
        "function playAdjacent(delta)",
        "navigationStations()",
        "render();\n    updatePlayer();",
        "ext.runtime.sendMessage({ type: 'RB_STOP' })",
        "async function adminCredentialsValid",
        "RB.adminOverrideFor(RB.key(station))",
        "RB.setAdminOverride(RB.key(current), value)",
        "if (event.key === 'Escape')",
        "await play(station)",
        "button.disabled = !!activeCommandToken && active",
        "function renderEmptyState(",
        "'Pokušaj ponovno'",
        "'Poništi filtre'",
    ):
        require(errors, popup_js, needle, "extensions/shared/popup.js")
    forbid(errors, popup_js, "$('playerName').textContent = station ? station.name : 'Ništa'", "extensions/shared/popup.js")
    require(errors, popup_css, "button:disabled", "extensions/shared/popup.css")
    require(errors, popup_css, "width: 44px", "extensions/shared/popup.css")
    require(errors, popup_css, ".station:focus-visible", "extensions/shared/popup.css")
    require(errors, popup_css, "button:active:not(:disabled)", "extensions/shared/popup.css")
    require(errors, popup_css, ".emptyAction", "extensions/shared/popup.css")
    require(errors, popup_css, "@media (max-width: 380px)", "extensions/shared/popup.css")
    require(errors, popup_css, ".player > img { display: none; }", "extensions/shared/popup.css")
    require(errors, popup_css, "grid-template-columns: minmax(0, 1fr) 42px minmax(168px, auto)", "extensions/shared/popup.css")

    android = read("apps/android/app/src/main/java/net/radiobalkan/app/MainActivity.java")
    adapter = read("apps/android/app/src/main/java/net/radiobalkan/app/StationAdapter.java")
    for needle in (
        '"Rezervni izvori"',
        '"Filtriraj stanice"',
        '"Poništi filtre"',
        '"Provjeri prikazane stanice"',
        'else if ("Provjeri prikazane stanice".equals(chosen) && requireAdmin()) checkVisibleStreams();',
        'else if (chosen.equals("Provjeri dostupnost") && requireAdmin()) checkOne(s);',
        '"Kopiraj poveznicu za reprodukciju"',
        '"Odaberi drugi izvor"',
        '"Vrati automatski odabir"',
        'StreamResolver.isSafeHttp(value)',
        '"Provjera nije uspjela · " + s.name',
        '"source-check-" + s.key()',
        'navItem("⋯", "Više", "more")',
        'Button sort = chip("Filtriraj ⌄", false)',
        "new RippleDrawable(",
        "navSelection = navSelectionForTab(tab)",
        "catalogRefreshRunning.compareAndSet(false, true)",
        "updatePlaybackControls()",
        "playerStop.setOnClickListener",
        "RadioPlayerService.ACTION_STOP",
        "playerPrev.setEnabled(false)",
        "playerStop.setEnabled(false)",
        "playerNext.setEnabled(false)",
        "} else if (playerArtwork != null) {",
        "playerArtwork.setImageResource(R.drawable.ic_radio_balkan)",
        "playerPrev.setOnClickListener",
        "playerNext.setOnClickListener",
        "showAdminLogin()",
        "if (adminMode) {",
        "if (!requireAdmin()) return;",
        "applyAdminSourceChange(s,",
        "PlaybackLifecycle.canNavigate(navigationCount, !currentKey.isEmpty())",
        "PlaybackLifecycle.adjacentIndex",
        "StreamResolver.isSafeHttp(s.homepage)",
        "ui.removeCallbacksAndMessages(null)",
        "setSearchVisible(",
        "@Override public void onBackPressed()",
        "hideSoftInputFromWindow",
    ):
        require(errors, android, needle, "Android MainActivity")
    for needle in (
        '"Alternativni izvori"',
        '"Kopiraj aktivni izvor"',
        '"Očisti automatske izvore"',
        '"Promijeni izvor"',
        'navItem("○", "Profil", "profile")',
    ):
        forbid(errors, android, needle, "Android MainActivity")

    on_create = method_body(android, "@Override protected void onCreate(Bundle savedInstanceState)")
    on_play = method_body(android, "@Override public void onPlay(RadioStation s)")
    start_playback = method_body(android, "private boolean startStationPlayback(RadioStation s, boolean requestPermission)")
    if not on_create:
        errors.append("Android MainActivity: onCreate body not found")
    elif "requestNotificationPermission();" in on_create:
        errors.append("Android MainActivity: notification permission must not be requested at cold start")
    if not on_play or "startStationPlayback(s, true);" not in on_play:
        errors.append("Android MainActivity: a real PLAY must use the contextual playback start path")
    if not start_playback or "if (requestPermission) requestNotificationPermission();" not in start_playback:
        errors.append("Android MainActivity: notification permission must be requested only by a real playback start")

    for needle in (
        'row.more.setContentDescription("Više opcija za " + s.name)',
        'row.more.setOnClickListener(v -> actions.onMore(s))',
        'root.setFocusable(true)',
        'root.addView(r.more',
        'dp(104)',
        '"Popularnost · " + value + " glasova"',
        '"Dostupno · zamjenski izvor"',
        '"Provjeravam dostupnost"',
        '"Trenutno nedostupno"',
        "private boolean adminMode;",
        "public void setAdminMode(boolean enabled)",
        "if (!adminMode) return popularity;",
        'row.root.setSelected(active)',
        '", trenutno odabrana"',
        "interactiveRounded(",

    ):
        require(errors, adapter, needle, "Android StationAdapter")

    setup = read("apps/windows/setup/main.go")
    for needle in (
        "case WM_KEYDOWN:",
        "handleKey(w)",
        "desktopHitRect",
        "runHitRect",
        "startupHitRect",
        "nextInstallerFocus",
        "activateInstallerControl",
        "focus == 4",
    ):
        require(errors, setup, needle, "Windows Setup UI")

    windows = read("apps/windows/portable/main.go")
    for needle in (
        "case WM_GETMINMAXINFO:",
        "info.PtMinTrackSize.X = 1100",
        "info.PtMinTrackSize.Y = 720",
        'drawSidebarLabel(hdc, "BRZI ODABIR", y)',
        '"Popularne", tab == "popular"',
        '"Jazz", genre == "jazz"',
        'RECT{mainR - 150, 528, mainR, 558}, Kind: hitGenreDropdown',
        'action("Kopiraj", 54, hitLink)',
        'setStatus("Poveznica za reprodukciju je kopirana")',
        'Prazno polje vraća automatski odabir.',
        'Zapis o pogrešci spremljen je lokalno.',
        "func activateStation(idx int)",
        "func defaultPlaybackIndexLocked() int",
        "if defaultIndex >= 0 {",
        'heroLabel = "Ⅱ  Pauziraj"',
        'heroLabel = "▶  Nastavi"',
        'playLabel = "Ⅱ"',
        "Kind: hitPlayerStop",
        "hitAdmin",
        "if adminModeEnabled()",
        "if !requireAdmin()",
        "restartCurrentStationIfPlaying",
        "canStopPlayback(currentIdx, stopped)",
        "canAdjustVolume(vol, -5)",
        "canAdjustVolume(vol, 5)",
        "canNavigate = canNavigateStations(currentIdx, len(app.stations), len(app.filtered))",
        "if canNavigate {",
        '"■"',
    ):
        require(errors, windows, needle, "Windows UI")
    for needle in (
        'drawSidebarLabel(hdc, "MOJE LISTE", y)',
        '"Jutarnji vibe", tab == "popular"',
        '"Chill večer", genre == "jazz"',
        'upiši AUTO',
        'strings.EqualFold(strings.TrimSpace(value), "AUTO")',
        'Lokalni dijagnostički zapis',
    ):
        forbid(errors, windows, needle, "Windows UI")

    # User-facing production surfaces must not accidentally expose common development placeholders.
    user_surfaces = "\n".join((popup_html, popup_js, android, adapter))
    for pattern in (r"\bTODO\b", r"\bFIXME\b", r"developer mode", r"debug mode", r"test mode"):
        if re.search(pattern, user_surfaces, re.IGNORECASE):
            errors.append(f"Production UI: developer placeholder matched {pattern!r}")

    if errors:
        print("\n".join(errors), file=sys.stderr)
        return 1
    print("Radio Balkan production UI/UX contracts OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
