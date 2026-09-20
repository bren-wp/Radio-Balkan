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
    require(errors, popup_html, 'id="discoveryPanel"', "Browser dedicated discovery panel")
    require(errors, popup_html, 'aria-controls="discoveryPanel"', "Browser dedicated discovery navigation")
    popup_js = read("extensions/shared/popup.js")
    popup_css = read("extensions/shared/popup.css")

    for needle in (
        'role="list"',
        'aria-pressed="false"',
        'id="playerDetails"',
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
        'id="stationPage"',
        'id="stationBack"',
        'id="stationPagePlay"',
        'id="stationSimilarList"',
        'id="quickTop"',
        'id="quickRecent"',
        'id="quickCountries"',
        'id="quickGenres"',
        'id="quickDiaspora"',
        'id="quickFolk"',
        'id="quickPop"',
        'id="browseTitle"',
        'id="browseHint"',
        'href="https://brendigo.com/"',
        'Built with',
        'disabled>▶</button>',
        'Nije odabrano',
    ):
        require(errors, popup_html, needle, "extensions/shared/popup.html")
    forbid(errors, popup_html, '>Ništa<', "extensions/shared/popup.html")
    forbid(errors, popup_html, 'id="stationPanel"', "extensions/shared/popup.html")
    forbid(errors, popup_html, 'role="dialog" aria-labelledby="station', "extensions/shared/popup.html")

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
        "$('adminStatusBadge').hidden = !adminMode;",
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
    require(errors, popup_js, "function openStationPage(station, returnFocus = null)", "extensions/shared/popup.js")
    require(errors, popup_js, "if (returnFocus && !detailReturnFocus) detailReturnFocus = returnFocus;", "extensions/shared/popup.js")
    require(errors, popup_js, "function closeStationPage()", "extensions/shared/popup.js")
    require(errors, popup_js, "if (event.target.closest('.stationPlay'))", "extensions/shared/popup.js")
    require(errors, popup_js, "openStationPage(station, row)", "extensions/shared/popup.js")
    require(errors, popup_js, "const PAGE = 72;", "extensions/shared/popup.js")
    require(errors, popup_js, "function areaLabel(code)", "extensions/shared/popup.js")
    require(errors, popup_js, "selectArea('HR')", "extensions/shared/popup.js")
    forbid(errors, popup_js, "if (!current && all.length) current = all[0];", "extensions/shared/popup.js")
    require(errors, popup_js, "function selectTop()", "extensions/shared/popup.js")
    require(errors, popup_js, "function selectRecent()", "extensions/shared/popup.js")
    require(errors, popup_js, "recentKey: RB.key(station)", "extensions/shared/popup.js")
    require(errors, popup_js, "recentKeys = await RB.recent()", "extensions/shared/popup.js")
    require(errors, popup_js, "viewMode === 'recent'", "extensions/shared/popup.js")
    forbid(errors, popup_js, "RB.addRecent(RB.key(station))", "extensions/shared/popup.js")
    require(errors, popup_js, "selectArea(RB.DIASPORA_CODE)", "extensions/shared/popup.js")
    require(errors, popup_js, "selectArea(RB.FOREIGN_CODE)", "extensions/shared/popup.js")
    require(errors, popup_css, ".builtWith", "extensions/shared/popup.css")
    require(errors, popup_css, ".stationPage", "extensions/shared/popup.css")
    require(errors, popup_css, ".browseIntro", "extensions/shared/popup.css")
    require(errors, popup_css, ".discoveryRail", "extensions/shared/popup.css")
    require(errors, popup_css, ".discoveryChip", "extensions/shared/popup.css")
    require(errors, popup_css, ".playerStationButton", "extensions/shared/popup.css")
    require(errors, popup_css, "grid-template-columns: repeat(3, minmax(0, 1fr))", "extensions/shared/popup.css")
    require(errors, popup_css, ".emptyAction", "extensions/shared/popup.css")
    require(errors, popup_css, "@media (max-width: 380px)", "extensions/shared/popup.css")
    require(errors, popup_css, ".playerStationButton > img { display: none; }", "extensions/shared/popup.css")
    require(errors, popup_css, "grid-template-columns: minmax(0, 1fr) 42px minmax(168px, auto)", "extensions/shared/popup.css")
    require(errors, popup_js, "$('playerDetails').addEventListener('click'", "extensions/shared/popup.js")
    require(errors, popup_js, "playerDetails.disabled = !station;", "extensions/shared/popup.js")

    chromium_worker = read("extensions/platform/chromium/service_worker.js")
    firefox_background = read("extensions/platform/firefox/background-firefox.js")
    for needle in (
        "async function recordRecentKey(rawKey)",
        "slice(0, 50)",
        "await recordRecentKey(msg.recentKey)",
    ):
        require(errors, chromium_worker, needle, "Chromium background recent history")
        require(errors, firefox_background, needle, "Firefox background recent history")

    state_store = read("apps/android/app/src/main/java/net/radiobalkan/app/StateStore.java")
    require(errors, state_store, 'DEFAULT_COUNTRY = "HR"', "Android StateStore")
    require(errors, state_store, 'prefs.getString("country", DEFAULT_COUNTRY)', "Android StateStore")

    android = read("apps/android/app/src/main/java/net/radiobalkan/app/MainActivity.java")
    adapter = read("apps/android/app/src/main/java/net/radiobalkan/app/StationAdapter.java")
    station_details = read("apps/android/app/src/main/java/net/radiobalkan/app/StationDetailsActivity.java")
    station_presentation = read("apps/android/app/src/main/java/net/radiobalkan/app/StationPresentation.java")
    manifest = read("apps/android/app/src/main/AndroidManifest.xml")
    for needle in (
        "StationPresentation.description(station)",
        "StationPresentation.publicDetails(station)",
        "EXTRA_SIMILAR_JSON",
        "addSimilarStations(root)",
        "navigateBack()",
        "openSimilar(candidate)",
        "PlaybackStarter.start(this, station, state)",
        '"Built with Brendigo"',
        '"https://brendigo.com/"',
        "finish();",
    ):
        require(errors, station_details, needle, "Android StationDetailsActivity")
    require(errors, manifest, 'android:name=".StationDetailsActivity"', "Android manifest")
    require(errors, manifest, 'android:exported="false"', "Android manifest")
    forbid(errors, station_details, "AlertDialog", "Android StationDetailsActivity")
    forbid(errors, station_details, "station.urlResolved", "Android StationDetailsActivity")
    forbid(errors, station_details, "station.homepage", "Android StationDetailsActivity")
    require(errors, station_presentation, "public static String description(RadioStation s)", "Android StationPresentation")
    require(errors, station_presentation, "public static String publicDetails(RadioStation s)", "Android StationPresentation")
    require(errors, station_presentation, "public static List<RadioStation> similarStations(", "Android StationPresentation")
    for needle in (
        '"Rezervni izvori"',
        '"Zemlje i žanrovi"',
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
        'Button sort = chip("Zemlje · Žanrovi", false)',
        "new RippleDrawable(",
        'navItem("⌂", "Početna", "all")',
        'tab = "all"; country = "HR"; genre = "";',
        'dp(168)',
        'navItem("★", "Top", "top")',
        'navItem("◷", "Nedavno", "recent")',
        'navItem("♡", "Omiljene", "favorites")',
        'navItem("⋯", "Više", "more")',
        "buildQuickAreas()",
        "buildInlineBrowsePanel()",
        "populateInlineBrowsePanel()",
        'chip("◎ Dijaspora", false)',
        'chip("◉ Strano", false)',
        'chip("♫ Narodna", false)',
        'chip("♪ Pop & Rock", false)',
        "applyQuickFilter(",
        "updateBrowseHeading()",
        "buildBrendigoFooter()",
        "@Override public void onDetails(RadioStation s)",
        "new Intent(this, StationDetailsActivity.class)",
        "StationPresentation.similarStations(source, s, 8)",
        "StationDetailsActivity.EXTRA_SIMILAR_JSON",
        "PlaybackStarter.start(this, s, state)",
        "catalogRefreshRunning.compareAndSet(false, true)",
        "updatePlaybackControls()",
        "openCurrentStationDetails()",
        "playerArtwork.setOnClickListener(v -> openCurrentStationDetails())",
        "info.setOnClickListener(v -> openCurrentStationDetails())",
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
        "updateAdminIndicator()",
        'text.setText(adminMode ? "Admin" : "Više");',
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
        'navItem("▥", "Radio", "radio")',
        'navItem("◇", "Otkrij", "discover")',
        'navItem("◎", "Dijaspora", "diaspora")',
        'navItem("▥", "Radio", "radio")',
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
        'row.root.setOnClickListener(v -> actions.onDetails(s))',
        "void onDetails(RadioStation s)",
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

    android_repository = read("apps/android/app/src/main/java/net/radiobalkan/app/RadioRepository.java")
    country_flag = read("apps/android/app/src/main/java/net/radiobalkan/app/CountryFlagDrawable.java")
    require(errors, android_repository, '{"BG", "Bugarska"}', "Android RadioRepository")
    require(errors, android_repository, '"tag=balkan"', "Android RadioRepository")
    require(errors, android_repository, '"name=radio%20diaspora"', "Android RadioRepository")
    require(errors, android_repository, '"language=bulgarian"', "Android RadioRepository")
    require(errors, android_repository, '"language=montenegrin"', "Android RadioRepository")
    require(errors, android_repository, '"name=jugoslav"', "Android RadioRepository")
    require(errors, android_repository, "MAX_FOREIGN = 240", "Android RadioRepository")
    require(errors, android_repository, "MAX_DIASPORA = 240", "Android RadioRepository")
    require(errors, android_repository, "FOREIGN_SCAN_LIMIT = 2000", "Android RadioRepository")
    require(errors, android_repository, "DIASPORA_QUERY_LIMIT = 160", "Android RadioRepository")
    require(errors, android_repository, 'new Thread(r, "radio-diaspora")', "Android RadioRepository")
    require(errors, android_repository, "CompletionService<List<RadioStation>> completion = new ExecutorCompletionService<>(diasporaPool)", "Android RadioRepository")
    require(errors, country_flag, 'case "BG":', "Android CountryFlagDrawable")

    windows_catalog = read("apps/windows/portable/foreign_catalog.go")
    for needle in (
        "foreignCatalogLimit      = 240",
        "diasporaCatalogLimit     = 240",
        "foreignCatalogScanLimit  = 2000",
        "diasporaQueryLimit       = 160",
        '{Field: "name", Value: "jugoslav"}',
        '{Field: "language", Value: "montenegrin"}',
    ):
        require(errors, windows_catalog, needle, "Windows supplemental catalog")

    windows = read("apps/windows/portable/main.go")
    require(errors, windows, 'st.CountryCode = "HR"', "Windows first-run country default")
    require(errors, windows, 'country == "HR"', "Windows home navigation")
    require(errors, windows, 'func homeGridColumns(width int32) int', "Windows responsive home cards")
    require(errors, windows, 'case width >= 1180:', "Windows responsive home cards")
    require(errors, windows, 'return 5', "Windows responsive home cards")
    require(errors, windows, 'case width >= 930:', "Windows responsive home cards")
    require(errors, windows, 'return 4', "Windows responsive home cards")
    require(errors, windows, 'return 3', "Windows responsive home cards")
    require(errors, windows, 'popular := popularStations(columns)', "Windows dense home catalog")
    require(errors, windows, 'balkan := discoveryStations(columns*2, excluded', "Windows dense home catalog")
    for needle in (
        "case WM_GETMINMAXINFO:",
        "info.PtMinTrackSize.X = 1100",
        "info.PtMinTrackSize.Y = 720",
        'drawSidebarLabel(hdc, "BIBLIOTEKA", y)',
        '"Top", tab == "popular"',
        '"Zemlje", tab == "countries"',
        '"Žanrovi", tab == "genres"',
        '"Dijaspora", country == diasporaCatalogCode',
        '"Strano", country == foreignCatalogCode',
        '"Omiljene", tab == "favorites"',
        '"Nedavno", tab == "recent"',
        '{"BG", "Bugarska"}',
        'case "BG":',
        "Kind: hitStationDetails",
        "Kind: hitStationDetails, Index: currentIdx",
        "Kind: hitStationBack",
        "Kind: hitBrendigo",
        'shellOpen("https://brendigo.com/")',
        "func drawStationDetailPage(hdc syscall.Handle, cr RECT)",
        "func openStationDetails(idx int)",
        "func closeStationDetails()",
        "func stationPublicDescription(s RadioStation) string",
        "func stationPublicFacts(s RadioStation) string",
        '"Popularno u Hrvatskoj"',
        '"Hrvatska"',
        '"Balkan"',
        '"Narodna / Folk"',
        '"Pop & Rock"',
        "func drawCountryBrowsePage(hdc syscall.Handle, cr RECT)",
        "func drawGenreBrowsePage(hdc syscall.Handle, cr RECT)",
        "func browseGridColumns(width int32) int",
        "st.WindowWidth = 1360",
        "st.WindowHeight = 820",
        'hitTab, "countries"',
        'action("Kopiraj", 54, hitLink)',
        'setStatus("Poveznica za reprodukciju je kopirana")',
        'Prazno polje vraća automatski odabir.',
        'Zapis o pogrešci spremljen je lokalno.',
        "func activateStation(idx int)",
        "func defaultPlaybackIndexLocked() int",
        "if defaultIndex >= 0 {",
        'playLabel = "Ⅱ  Pauziraj"',
        'playLabel = "▶  Nastavi"',
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
        '"Pretraga", false, hitTab, "searchfocus"',
        '"Pregledaj", false, hitTab, "browse"',
        "func showStationDetails(idx int)",
        "func countryCodeByName(name string) string",
        "func firstTag(tags string) string",
    ):
        forbid(errors, windows, needle, "Windows UI")

    # User-facing production surfaces must not accidentally expose common development placeholders.
    forbid(errors, windows, "func firstTag(tags string) string", "Windows dead-code cleanup")
    require(errors, windows, "firstPublicTag(s.Tags)", "Windows shared public-tag helper")

    user_surfaces = "\n".join((popup_html, popup_js, android, adapter, windows))
    if "radiobalkan.net" in user_surfaces.lower():
        errors.append("Production UI: reference-domain link must never be embedded in application surfaces")
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
