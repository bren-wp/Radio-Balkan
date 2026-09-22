//go:build windows

package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)


func newResolverTestClient(t *testing.T, server *httptest.Server) *http.Client {
	t.Helper()
	dialAddr := server.Listener.Addr().String()
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, dialAddr)
		},
	}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{
		Timeout:   3 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 5 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
}

func TestCheckStreamResolvesPlaylistThroughProductionPath(t *testing.T) {
	const publicHost = "93.184.216.34"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/station.m3u":
			w.Header().Set("Content-Type", "audio/x-mpegurl")
			_, _ = fmt.Fprintf(w, "#EXTM3U\n#EXTINF:-1,CI station\nhttp://%s/live.mp3\n", publicHost)
		case "/live.mp3":
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = w.Write([]byte("ID3-realistic-stream-bytes"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	app = App{
		http:      newResolverTestClient(t, server),
		ctx:       context.Background(),
		done:      make(chan struct{}),
		streamSem: make(chan struct{}, 2),
	}
	resolved, ok := checkStream("http://" + publicHost + "/station.m3u")
	if !ok {
		t.Fatal("production stream resolver rejected a valid M3U -> MP3 path")
	}
	want := "http://" + publicHost + "/live.mp3"
	if resolved != want {
		t.Fatalf("resolved playlist URL = %q; want %q", resolved, want)
	}
}

func TestCheckStreamFollowsHTTPRedirectThroughProductionPath(t *testing.T) {
	const publicHost = "93.184.216.34"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/listen":
			http.Redirect(w, r, "/stream.aac", http.StatusFound)
		case "/stream.aac":
			w.Header().Set("Content-Type", "audio/aac")
			_, _ = w.Write([]byte("AAC-realistic-stream-bytes"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	app = App{
		http:      newResolverTestClient(t, server),
		ctx:       context.Background(),
		done:      make(chan struct{}),
		streamSem: make(chan struct{}, 2),
	}
	resolved, ok := checkStream("http://" + publicHost + "/listen")
	if !ok {
		t.Fatal("production stream checker rejected a valid HTTP redirect -> AAC path")
	}
	want := "http://" + publicHost + "/stream.aac"
	if resolved != want {
		t.Fatalf("redirect final URL = %q; want %q", resolved, want)
	}
}

func TestAdminCredentialsAcceptOnlyConfiguredAdministrator(t *testing.T) {
	password := fmt.Sprintf("%s%d", "brendigo", 2025)
	if !adminCredentialsValid("brendigo", password) {
		t.Fatal("configured administrator credentials were rejected")
	}
	if !adminCredentialsValid(" BRENDIGO ", password) {
		t.Fatal("administrator username normalization regressed")
	}
	if adminCredentialsValid("user", password) {
		t.Fatal("non-admin username was accepted")
	}
	if adminCredentialsValid("brendigo", "wrong") {
		t.Fatal("wrong administrator password was accepted")
	}
}

func TestSourceChangeRestartDecision(t *testing.T) {
	if shouldRestartAfterSourceChange("", "station", true) {
		t.Fatal("missing current station must not restart")
	}
	if shouldRestartAfterSourceChange("station", "station", false) {
		t.Fatal("paused player must not restart after admin source change")
	}
	if shouldRestartAfterSourceChange("other", "station", true) {
		t.Fatal("unrelated playing station must not restart")
	}
	if !shouldRestartAfterSourceChange("station", "station", true) {
		t.Fatal("active changed station must restart onto the new source")
	}
}

func TestSafeHTTPURLRejectsPrivateAndCredentialedTargets(t *testing.T) {
	tests := []string{
		"",
		"file:///C:/Windows/System32/drivers/etc/hosts",
		"http://localhost:8080/live",
		"http://localhost./live",
		"http://radio.local./live",
		"http://127.0.0.1/live",
		"http://10.0.0.4/live",
		"http://172.16.1.2/live",
		"http://192.168.1.10/live",
		"http://169.254.169.254/latest/meta-data/",
		"http://100.64.0.1/live",
		"https://metadata.google.internal/computeMetadata/v1/",
		"https://metadata.google.internal./computeMetadata/v1/",
		"https://user:pass@example.com/live",
	}
	for _, raw := range tests {
		if safeHTTPURL(raw) {
			t.Fatalf("safeHTTPURL(%q) = true; want false", raw)
		}
	}
}

func TestSafeHTTPURLAcceptsPublicHTTPStreams(t *testing.T) {
	tests := []string{
		"https://example.com/live.mp3",
		"http://stream.example.org:8000/radio",
	}
	for _, raw := range tests {
		if !safeHTTPURL(raw) {
			t.Fatalf("safeHTTPURL(%q) = false; want true", raw)
		}
	}
}

func TestSidebarFallbackGeometryMatchesRenderedRows(t *testing.T) {
	rows := []int32{
		sidebarHomeY,
		sidebarTopY,
		sidebarCountriesY,
		sidebarGenresY,
		sidebarDiasporaY,
		sidebarForeignY,
		sidebarFavoritesY,
		sidebarRecentY,
		sidebarReplacedY,
		sidebarBrokenY,
	}
	for _, y := range rows {
		if !sidebarRowContains(y, y) || !sidebarRowContains(y+sidebarItemHeight, y) {
			t.Fatalf("sidebar row %d does not include its rendered bounds", y)
		}
		if sidebarRowContains(y-1, y) || sidebarRowContains(y+sidebarItemHeight+1, y) {
			t.Fatalf("sidebar row %d accepts coordinates outside rendered bounds", y)
		}
	}
	if sidebarFavoritesY != 393 || sidebarRecentY != 435 {
		t.Fatalf("library fallback rows drifted: favorites=%d recent=%d", sidebarFavoritesY, sidebarRecentY)
	}
}

func TestNavigationTabsIncludeInAppCountryAndGenrePages(t *testing.T) {
	for _, tab := range []string{"all", "croatia", "popular", "countries", "genres", "favorites", "recent"} {
		if !isValidTab(tab) {
			t.Fatalf("tab %q must be valid", tab)
		}
	}
	if isValidTab("dropdown") {
		t.Fatal("legacy dropdown pseudo-tab must not become a persisted navigation state")
	}
}

func TestRegionalRefreshPreservesSupplementalCatalogGroups(t *testing.T) {
	regional := []RadioStation{
		{StationUUID: "hr", Name: "HR", CountryCode: "HR", URL: "https://example.com/hr"},
		{StationUUID: "rs", Name: "RS", CountryCode: "RS", URL: "https://example.com/rs"},
	}
	previous := []RadioStation{
		{StationUUID: "dia", Name: "Diaspora", CountryCode: diasporaCatalogCode, SourceCountryCode: "DE", URL: "https://example.com/dia", Votes: 10},
		{StationUUID: "int", Name: "Foreign", CountryCode: foreignCatalogCode, SourceCountryCode: "US", URL: "https://example.com/int", Votes: 9},
	}
	got := preserveSupplementalStations(regional, previous)
	seen := map[string]bool{}
	for _, station := range got {
		seen[station.CountryCode] = true
	}
	for _, code := range []string{"HR", "RS", diasporaCatalogCode, foreignCatalogCode} {
		if !seen[code] {
			t.Fatalf("refreshed catalog lost %q group: %#v", code, got)
		}
	}
}

func TestRegionalFetchCodesExcludeApplicationGroups(t *testing.T) {
	codes := regionalFetchCodes()
	if len(codes) != 8 {
		t.Fatalf("regional fetch has %d country codes; want 8", len(codes))
	}
	seen := map[string]bool{}
	for _, code := range codes {
		if !isRegionalCatalogCode(code) {
			t.Fatalf("fetch contains non-regional code %q", code)
		}
		if isSupplementalCatalogCode(code) {
			t.Fatalf("fetch must never send application group %q as an ISO country code", code)
		}
		if seen[code] {
			t.Fatalf("duplicate regional fetch code %q", code)
		}
		seen[code] = true
	}
	for _, want := range []string{"HR", "BA", "RS", "SI", "MK", "AL", "ME", "BG"} {
		if !seen[want] {
			t.Fatalf("regional fetch is missing %q", want)
		}
	}
}

func TestRegionalCountryBrowseExcludesApplicationGroups(t *testing.T) {
	items := regionalCountryDefs()
	if len(items) != 8 {
		t.Fatalf("regional country page contains %d entries; want 8", len(items))
	}
	for _, item := range items {
		if !isRegionalCatalogCode(item.Code) {
			t.Fatalf("non-regional code %q leaked into country page", item.Code)
		}
		if isSupplementalCatalogCode(item.Code) {
			t.Fatalf("supplemental catalog code %q leaked into country page", item.Code)
		}
	}
}

func TestCompactDesktopWindowDefaultsAndRestoreCap(t *testing.T) {
	fresh := validateState(PersistedState{}, false)
	if fresh.WindowWidth != 1240 || fresh.WindowHeight != 760 {
		t.Fatalf("fresh window = %dx%d; want 1240x760", fresh.WindowWidth, fresh.WindowHeight)
	}

	large := validateState(PersistedState{Volume: 80, CountryCode: "HR", Tab: "all", WindowWidth: 2200, WindowHeight: 1200}, true)
	if large.WindowWidth != 1420 || large.WindowHeight != 860 {
		t.Fatalf("large restored window = %dx%d; want compact 1420x860 cap", large.WindowWidth, large.WindowHeight)
	}

	normal := validateState(PersistedState{Volume: 80, CountryCode: "HR", Tab: "all", WindowWidth: 1360, WindowHeight: 820}, true)
	if normal.WindowWidth != 1360 || normal.WindowHeight != 820 {
		t.Fatalf("normal restored window changed to %dx%d", normal.WindowWidth, normal.WindowHeight)
	}
}

func TestValidateStateDefaultsToCroatiaOnlyOnFirstLaunch(t *testing.T) {
	fresh := validateState(PersistedState{}, false)
	if fresh.CountryCode != "HR" {
		t.Fatalf("fresh country = %q; want HR", fresh.CountryCode)
	}

	savedAll := validateState(PersistedState{CountryCode: "", Volume: 80, Tab: "all"}, true)
	if savedAll.CountryCode != "" {
		t.Fatalf("saved all-country selection = %q; want empty", savedAll.CountryCode)
	}

	savedSerbia := validateState(PersistedState{CountryCode: "RS", Volume: 80, Tab: "all"}, true)
	if savedSerbia.CountryCode != "RS" {
		t.Fatalf("saved country = %q; want RS", savedSerbia.CountryCode)
	}

	for _, code := range []string{diasporaCatalogCode, foreignCatalogCode} {
		saved := validateState(PersistedState{CountryCode: code, Volume: 80, Tab: "all"}, true)
		if saved.CountryCode != code {
			t.Fatalf("saved supplemental country = %q; want %q", saved.CountryCode, code)
		}
	}
}

func TestValidateStateDropsUnsafeReplacementURLs(t *testing.T) {
	state := PersistedState{
		Favorites:    map[string]bool{"station": true},
		Replacements: map[string]string{"good": "https://example.com/live", "bad": "http://127.0.0.1/live"},
		Backups: map[string][]string{
			"station": {"https://example.com/a", "https://example.com/a", "http://localhost/b"},
		},
		Recent:      []string{"one", "two"},
		Volume:      80,
		CountryCode: "HR",
		Tab:         "all",
	}

	got := validateState(state, true)
	if got.Replacements["good"] == "" {
		t.Fatal("safe replacement was removed")
	}
	if _, ok := got.Replacements["bad"]; ok {
		t.Fatal("unsafe replacement URL was retained")
	}
	if len(got.Backups["station"]) != 1 || got.Backups["station"][0] != "https://example.com/a" {
		t.Fatalf("unexpected sanitized backups: %#v", got.Backups["station"])
	}
}

func TestUnsafeNetworkIPRejectsLocalAndSpecialRanges(t *testing.T) {
	rejected := []string{
		"0.0.0.0",
		"127.0.0.1",
		"10.0.0.1",
		"100.64.0.1",
		"169.254.169.254",
		"172.16.0.1",
		"192.168.1.1",
		"::",
		"::1",
		"fc00::1",
		"fe90::1",
		"ff02::1",
		"::ffff:127.0.0.1",
	}
	for _, raw := range rejected {
		if !unsafeNetworkIP(net.ParseIP(raw)) {
			t.Fatalf("unsafeNetworkIP(%q) = false; want true", raw)
		}
	}
	if unsafeNetworkIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public IPv4 address was rejected")
	}
}

func TestWriteFileDurablePersistsCompleteContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.tmp")
	want := []byte("{\"volume\":80,\"country_code\":\"HR\"}\n")
	if err := writeFileDurable(path, want, 0600); err != nil {
		t.Fatalf("writeFileDurable() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("durable file content = %q; want %q", got, want)
	}

	replacement := []byte("{\"volume\":25}\n")
	if err := writeFileDurable(path, replacement, 0600); err != nil {
		t.Fatalf("writeFileDurable() replacement error = %v", err)
	}
	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() after replacement error = %v", err)
	}
	if string(got) != string(replacement) {
		t.Fatalf("durable replacement content = %q; want %q", got, replacement)
	}
}

func TestTransportAvailabilityUsesVisibleStationCount(t *testing.T) {
	tests := []struct {
		name         string
		current      int
		stationCount int
		filtered     int
		want         bool
	}{
		{"empty", -1, 0, 0, false},
		{"no current station", -1, 5, 5, false},
		{"one station", 0, 1, 0, false},
		{"two stations", 0, 2, 0, true},
		{"single filtered result", 0, 5, 1, false},
		{"multiple filtered results", 0, 5, 2, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := canNavigateStations(tc.current, tc.stationCount, tc.filtered); got != tc.want {
				t.Fatalf("navigation availability = %v, want %v", got, tc.want)
			}
		})
	}

	if canStopPlayback(-1, false) {
		t.Fatal("stop must be disabled without a current station")
	}
	if canStopPlayback(0, true) {
		t.Fatal("stop must be disabled after terminal stop")
	}
	if !canStopPlayback(0, false) {
		t.Fatal("stop must be enabled for an active or paused current station")
	}

	if canAdjustVolume(0, -5) {
		t.Fatal("volume down must disable at 0 percent")
	}
	if !canAdjustVolume(5, -5) {
		t.Fatal("volume down must enable above 0 percent")
	}
	if canAdjustVolume(100, 5) {
		t.Fatal("volume up must disable at 100 percent")
	}
	if !canAdjustVolume(95, 5) {
		t.Fatal("volume up must enable below 100 percent")
	}
	if canAdjustVolume(50, 0) {
		t.Fatal("zero volume delta must not expose a control action")
	}
}

func TestDefaultPlaybackIndexLocked(t *testing.T) {
	previousStations := app.stations
	previousFiltered := app.filtered
	defer func() {
		app.stations = previousStations
		app.filtered = previousFiltered
	}()

	app.stations = nil
	app.filtered = nil
	if got := defaultPlaybackIndexLocked(); got != -1 {
		t.Fatalf("empty default index = %d, want -1", got)
	}

	app.stations = []RadioStation{{Name: "A"}, {Name: "B"}, {Name: "C"}}
	app.filtered = nil
	if got := defaultPlaybackIndexLocked(); got != 0 {
		t.Fatalf("unfiltered default index = %d, want 0", got)
	}

	app.filtered = []int{2, 1}
	if got := defaultPlaybackIndexLocked(); got != 2 {
		t.Fatalf("filtered default index = %d, want 2", got)
	}

	app.filtered = []int{99}
	if got := defaultPlaybackIndexLocked(); got != 0 {
		t.Fatalf("invalid filtered index fallback = %d, want 0", got)
	}
}

func TestPendingPlayMatchesOnlyLatestGeneration(t *testing.T) {
	app = App{playSeq: 12, pendingPlaySeq: 12}
	app.mu.RLock()
	if !pendingPlayCurrentLocked() {
		app.mu.RUnlock()
		t.Fatal("latest pending Play was not recognized")
	}
	app.mu.RUnlock()

	app.mu.Lock()
	app.playSeq = 13
	app.mu.Unlock()
	app.mu.RLock()
	if pendingPlayCurrentLocked() {
		app.mu.RUnlock()
		t.Fatal("superseded pending Play remained active")
	}
	app.mu.RUnlock()
}

func TestStationSwitchStopsOnlyDifferentActiveBackend(t *testing.T) {
	if !shouldStopAudioForStationSwitch("station-a", "station-b", audioBackendWPF) {
		t.Fatal("different station must stop the active WPF backend before opening the replacement")
	}
	if !shouldStopAudioForStationSwitch("station-a", "station-b", audioBackendMCI) {
		t.Fatal("different station must stop the active MCI backend before opening the replacement")
	}
	if shouldStopAudioForStationSwitch("station-a", "station-a", audioBackendWPF) {
		t.Fatal("same-station reconnect must not be treated as a station switch")
	}
	if shouldStopAudioForStationSwitch("station-a", "station-b", audioBackendNone) {
		t.Fatal("station switch without an active backend must not schedule a redundant Stop")
	}
}

func TestPlaybackToggleDecisionLifecycle(t *testing.T) {
	tests := []struct {
		name     string
		current  int
		playing  bool
		stopped  bool
		expected playbackToggleAction
	}{
		{name: "no station", current: -1, expected: playbackToggleNone},
		{name: "playing pauses", current: 0, playing: true, expected: playbackTogglePause},
		{name: "paused resumes", current: 0, expected: playbackToggleResume},
		{name: "stopped reconnects", current: 0, stopped: true, expected: playbackToggleReconnect},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := decidePlaybackToggle(tc.current, tc.playing, tc.stopped); got != tc.expected {
				t.Fatalf("decidePlaybackToggle(%d, %v, %v) = %d; want %d", tc.current, tc.playing, tc.stopped, got, tc.expected)
			}
		})
	}
}

func TestPlaybackControlsReturnImmediatelyWhenAudioBackendIsBusy(t *testing.T) {
	app = App{
		done:       make(chan struct{}),
		current:    0,
		currentKey: "station-1",
		playing:    true,
		stations: []RadioStation{{
			StationUUID: "station-1",
			Name:        "Station 1",
			CountryCode: "HR",
			URL:         "https://example.com/live.mp3",
		}},
		state: PersistedState{
			Favorites:    map[string]bool{},
			Replacements: map[string]string{},
			Backups:      map[string][]string{},
			Volume:       80,
		},
	}

	app.audioMu.Lock()
	start := time.Now()
	toggleCurrentPlayback()
	if elapsed := time.Since(start); elapsed > 150*time.Millisecond {
		app.audioMu.Unlock()
		t.Fatalf("pause control blocked the UI path for %s while audio backend was busy", elapsed)
	}
	app.mu.RLock()
	paused := !app.playing
	app.mu.RUnlock()
	if !paused {
		app.audioMu.Unlock()
		t.Fatal("pause control did not update visible playback state immediately")
	}

	app.mu.Lock()
	app.playing = true
	app.audioStopped = false
	app.mu.Unlock()
	start = time.Now()
	stopCurrentPlayback()
	if elapsed := time.Since(start); elapsed > 150*time.Millisecond {
		app.audioMu.Unlock()
		t.Fatalf("stop control blocked the UI path for %s while audio backend was busy", elapsed)
	}
	app.mu.RLock()
	stopped := app.audioStopped && !app.playing
	app.mu.RUnlock()
	app.audioMu.Unlock()
	if !stopped {
		t.Fatal("stop control did not update visible playback state immediately")
	}

	// Let the queued backend control goroutines observe the released mutex.
	time.Sleep(40 * time.Millisecond)
}

func TestAudioShutdownIsBoundedWhenBackendLockIsBusy(t *testing.T) {
	app = App{done: make(chan struct{})}
	app.audioMu.Lock()
	defer app.audioMu.Unlock()

	start := time.Now()
	done := make(chan struct{})
	go func() {
		audioShutdown()
		close(done)
	}()

	select {
	case <-done:
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("bounded audio shutdown took %s; want under 2s", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("audio shutdown blocked for more than 2s on a busy backend lock")
	}
}

func TestAudioAckTimeoutDiscardsStaleChannel(t *testing.T) {
	app = App{done: make(chan struct{}), audioAck: make(chan string)}

	app.audioMu.Lock()
	err := waitAudioAckLocked(15 * time.Millisecond)
	cleared := app.audioAck == nil && app.audioCmd == nil && app.audioIn == nil
	app.audioMu.Unlock()

	if err == nil {
		t.Fatal("audio ACK timeout unexpectedly succeeded")
	}
	if !cleared {
		t.Fatal("timed-out audio helper state was not discarded")
	}
}

func TestLatestPlayRequestCancelsStaleAckWait(t *testing.T) {
	app = App{
		done:         make(chan struct{}),
		audioAck:     make(chan string),
		playSeq:      41,
		audioBackend: audioBackendWPF,
	}

	go func() {
		time.Sleep(70 * time.Millisecond)
		app.mu.Lock()
		app.playSeq = 42
		app.mu.Unlock()
	}()

	start := time.Now()
	app.audioMu.Lock()
	err := waitAudioAckLockedForRequest(2*time.Second, 41)
	cleared := app.audioAck == nil && app.audioCmd == nil && app.audioIn == nil
	app.audioMu.Unlock()
	backend := currentAudioBackend()

	if !errors.Is(err, errPlayRequestSuperseded) {
		t.Fatalf("stale play wait returned %v; want errPlayRequestSuperseded", err)
	}
	if elapsed := time.Since(start); elapsed > 600*time.Millisecond {
		t.Fatalf("stale play request took %s to cancel; latest click should win promptly", elapsed)
	}
	if !cleared {
		t.Fatal("superseded PLAY did not discard the helper/ACK channel")
	}
	if backend != audioBackendNone {
		t.Fatalf("superseded WPF ACK wait left backend %v active; want none before the newer request takes ownership", backend)
	}
}

func TestSupersededPlayCannotClearNewerBackend(t *testing.T) {
	app = App{
		done:         make(chan struct{}),
		playSeq:      52,
		audioBackend: audioBackendWPF,
	}
	err := audioPlayRequest("http://93.184.216.34/live.mp3", 51)
	if !errors.Is(err, errPlayRequestSuperseded) {
		t.Fatalf("already-superseded Play returned %v; want errPlayRequestSuperseded", err)
	}
	if got := currentAudioBackend(); got != audioBackendWPF {
		t.Fatalf("superseded Play changed newer backend to %v; want WPF", got)
	}
}

func TestPlaybackWatchdogRecoveryPolicyTrustsActiveBackend(t *testing.T) {
	if playbackBackendNeedsRecovery(true, audioBackendWPF) {
		t.Fatal("healthy WPF playback must not be challenged by an HTTP probe watchdog")
	}
	if playbackBackendNeedsRecovery(true, audioBackendMCI) {
		t.Fatal("healthy MCI playback must not be challenged by an HTTP probe watchdog")
	}
	if !playbackBackendNeedsRecovery(true, audioBackendNone) {
		t.Fatal("playing state without an active backend should trigger controlled recovery")
	}
	if playbackBackendNeedsRecovery(false, audioBackendNone) {
		t.Fatal("inactive playback must not be restarted by the watchdog")
	}
}

func TestAudioRuntimeFailureEventParsing(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("network stream failed"))
	reqSeq, detail, ok := parseAudioRuntimeFailure("EVENT FAILED 77 " + encoded)
	if !ok {
		t.Fatal("runtime MediaFailed event was not recognized")
	}
	if reqSeq != 77 {
		t.Fatalf("runtime MediaFailed request = %d; want 77", reqSeq)
	}
	if detail != "network stream failed" {
		t.Fatalf("runtime MediaFailed detail = %q", detail)
	}
	if _, _, ok := parseAudioRuntimeFailure("EVENT FAILED invalid " + encoded); ok {
		t.Fatal("runtime failure with invalid playback generation was accepted")
	}
	if _, _, ok := parseAudioRuntimeFailure("OK"); ok {
		t.Fatal("normal command ACK was misclassified as an async runtime event")
	}
}

func TestStaleWPFRuntimeFailureCannotAffectNewerPlay(t *testing.T) {
	app = App{
		done:         make(chan struct{}),
		playSeq:      82,
		audioBackend: audioBackendWPF,
		playing:      true,
	}
	handleAudioBackendFailureForRequest(audioBackendWPF, 81, "late failure from old stream")
	app.mu.RLock()
	defer app.mu.RUnlock()
	if app.playSeq != 82 || app.audioBackend != audioBackendWPF || !app.playing {
		t.Fatalf("stale WPF event changed newer playback: seq=%d backend=%v playing=%v", app.playSeq, app.audioBackend, app.playing)
	}
}

func TestMCIModeHealthUsesBackendStateNotHTTPProbe(t *testing.T) {
	for _, mode := range []string{"playing", "PLAYING", "paused", "seeking"} {
		if !mciModeHealthy(mode) {
			t.Fatalf("MCI mode %q should be treated as healthy", mode)
		}
	}
	for _, mode := range []string{"", "stopped", "not ready", "closed"} {
		if mciModeHealthy(mode) {
			t.Fatalf("MCI mode %q should trigger controlled recovery", mode)
		}
	}
}

func TestTokenizedRuntimeFailureStillAppliesAfterPauseControlGenerationChanges(t *testing.T) {
	st := RadioStation{StationUUID: "paused-token", Name: "Paused Token"}
	key := stationKey(st)
	app = App{
		done:            make(chan struct{}),
		stations:        []RadioStation{st},
		current:         0,
		currentKey:      key,
		playing:         false,
		audioStopped:    false,
		audioBackend:    audioBackendWPF,
		playSeq:         55,
		audioControlSeq: 3,
	}
	handleAudioBackendFailureForRequest(audioBackendWPF, 55, "")
	app.mu.RLock()
	defer app.mu.RUnlock()
	if app.audioBackend != audioBackendNone {
		t.Fatalf("tokenized paused runtime failure left backend %v; want none", app.audioBackend)
	}
	if app.playSeq != 55 {
		t.Fatalf("runtime failure changed media generation to %d; want 55", app.playSeq)
	}
}

func TestBackendFailureWhilePausedForcesReconnectOnNextPlay(t *testing.T) {
	st := RadioStation{StationUUID: "paused-station", Name: "Paused Station"}
	key := stationKey(st)
	app = App{
		stations:     []RadioStation{st},
		current:      0,
		currentKey:   key,
		playing:      false,
		audioStopped: false,
		audioBackend: audioBackendWPF,
	}
	handleAudioBackendFailure(audioBackendWPF, "")

	app.mu.RLock()
	backend := app.audioBackend
	playing := app.playing
	stopped := app.audioStopped
	app.mu.RUnlock()
	if backend != audioBackendNone {
		t.Fatalf("paused failed backend = %v; want audioBackendNone so Resume reconnects", backend)
	}
	if playing {
		t.Fatal("paused runtime failure unexpectedly marked playback active")
	}
	if stopped {
		t.Fatal("paused runtime failure must preserve resume/reconnect semantics")
	}
}

func TestStaleAsyncStopCannotClearNewPlaybackBackend(t *testing.T) {
	app = App{
		done:         make(chan struct{}),
		playSeq:      22,
		audioBackend: audioBackendWPF,
	}
	audioStopForRequest(21)
	if got := currentAudioBackend(); got != audioBackendWPF {
		t.Fatalf("stale Stop changed newer playback backend to %v; want WPF", got)
	}
}

func TestMCIControlRequiresActiveBackend(t *testing.T) {
	app = App{
		done:         make(chan struct{}),
		audioBackend: audioBackendNone,
	}
	if _, err := mciQueryExisting("status radio mode"); err == nil {
		t.Fatal("MCI command ran without MCI owning the audio backend")
	}
}

func TestWPFPlayReplacesActiveMCIBackend(t *testing.T) {
	if !shouldStopMCIForAudioCommand("PLAY Zm9v 0.50", audioBackendMCI) {
		t.Fatal("WPF PLAY must close an older MCI fallback stream first")
	}
	if shouldStopMCIForAudioCommand("VOLUME 0.50", audioBackendMCI) {
		t.Fatal("non-PLAY commands must not tear down MCI")
	}
	if shouldStopMCIForAudioCommand("PLAY Zm9v 0.50", audioBackendWPF) {
		t.Fatal("WPF-to-WPF PLAY should reuse the media host instead of forcing MCI cleanup")
	}
}

func TestStopCancelsPendingPlayWithoutCommittedCurrent(t *testing.T) {
	app = App{
		done:           make(chan struct{}),
		current:        -1,
		playing:        false,
		audioStopped:   false,
		audioBackend:   audioBackendNone,
		playSeq:        17,
		pendingPlaySeq: 17,
	}
	stopCurrentPlayback()
	app.mu.RLock()
	defer app.mu.RUnlock()
	if app.playSeq != 18 {
		t.Fatalf("Stop playSeq = %d; want 18 so pending Play becomes stale", app.playSeq)
	}
	if app.pendingPlaySeq != 0 {
		t.Fatalf("pending Play sequence = %d; want cleared after Stop", app.pendingPlaySeq)
	}
	if app.playing || !app.audioStopped {
		t.Fatalf("Stop pending state playing=%v stopped=%v; want false/true", app.playing, app.audioStopped)
	}
}

func TestStopCurrentPlaybackAdvancesRequestGeneration(t *testing.T) {
	st := RadioStation{StationUUID: "stop-seq", Name: "Stop Sequence"}
	app = App{
		done:         make(chan struct{}),
		stations:     []RadioStation{st},
		filtered:     []int{0},
		current:      0,
		currentKey:   stationKey(st),
		playing:      true,
		audioStopped: false,
		audioBackend: audioBackendNone,
		playSeq:      9,
	}
	stopCurrentPlayback()
	app.mu.RLock()
	defer app.mu.RUnlock()
	if app.playSeq != 10 {
		t.Fatalf("Stop playSeq = %d; want 10 so older async audio work becomes stale", app.playSeq)
	}
	if app.playing || !app.audioStopped {
		t.Fatalf("Stop state playing=%v stopped=%v; want false/true", app.playing, app.audioStopped)
	}
}

func TestPausePreservesMediaGenerationAndAdvancesControlGeneration(t *testing.T) {
	st := RadioStation{StationUUID: "pause-seq", Name: "Pause Sequence"}
	app = App{
		done:         make(chan struct{}),
		stations:     []RadioStation{st},
		filtered:     []int{0},
		current:      0,
		currentKey:   stationKey(st),
		playing:      true,
		audioStopped: false,
		audioBackend: audioBackendNone,
		playSeq:      31,
	}
	toggleCurrentPlayback()
	app.mu.RLock()
	defer app.mu.RUnlock()
	if app.playSeq != 31 {
		t.Fatalf("Pause changed media playSeq to %d; active WPF runtime events must retain token 31", app.playSeq)
	}
	if app.audioControlSeq != 1 {
		t.Fatalf("Pause audioControlSeq = %d; want 1", app.audioControlSeq)
	}
	if app.playing || app.audioStopped {
		t.Fatalf("Pause state playing=%v stopped=%v; want false/false", app.playing, app.audioStopped)
	}
}

func TestStalePauseAndResumeControlsCannotAffectNewerPlayback(t *testing.T) {
	app = App{
		done:            make(chan struct{}),
		playSeq:         42,
		audioControlSeq: 2,
		audioBackend:    audioBackendWPF,
	}
	if err := audioPauseForControl(41, 2); !errors.Is(err, errPlayRequestSuperseded) {
		t.Fatalf("Pause for stale media returned %v; want errPlayRequestSuperseded", err)
	}
	if err := audioPauseForControl(42, 1); !errors.Is(err, errAudioControlSuperseded) {
		t.Fatalf("stale Pause control returned %v; want errAudioControlSuperseded", err)
	}
	if got := currentAudioBackend(); got != audioBackendWPF {
		t.Fatalf("stale Pause changed newer backend to %v; want WPF", got)
	}
	if err := audioResumeForControl(42, 1); !errors.Is(err, errAudioControlSuperseded) {
		t.Fatalf("stale Resume control returned %v; want errAudioControlSuperseded", err)
	}
	if got := currentAudioBackend(); got != audioBackendWPF {
		t.Fatalf("stale Resume changed newer backend to %v; want WPF", got)
	}
}

func TestAudioEngineStartupWaitCancelsSupersededPlay(t *testing.T) {
	app = App{
		done:    make(chan struct{}),
		playSeq: 101,
	}
	ack := make(chan string)

	go func() {
		time.Sleep(70 * time.Millisecond)
		app.mu.Lock()
		app.playSeq = 102
		app.mu.Unlock()
	}()

	started := time.Now()
	_, _, err := waitAudioEngineStartupSignal(ack, 101, 2*time.Second)
	if !errors.Is(err, errPlayRequestSuperseded) {
		t.Fatalf("startup wait returned %v; want errPlayRequestSuperseded", err)
	}
	if elapsed := time.Since(started); elapsed > 600*time.Millisecond {
		t.Fatalf("superseded startup wait took %s; latest click should win promptly", elapsed)
	}
}

func TestAudioEngineStartupTimeoutIsBounded(t *testing.T) {
	if audioEngineStartupTimeout < 15*time.Second {
		t.Fatalf("audio engine startup timeout %s is too short for cold PresentationCore/Add-Type initialization", audioEngineStartupTimeout)
	}
	if audioEngineStartupTimeout > 30*time.Second {
		t.Fatalf("audio engine startup timeout %s is too long; startup must remain bounded", audioEngineStartupTimeout)
	}
}

func TestAudioPlayCommandIncludesRequestGeneration(t *testing.T) {
	got := audioPlayCommand(77, "Zm9v", 0.5)
	if got != "PLAY 77 Zm9v 0.50" {
		t.Fatalf("audio PLAY command = %q; want tokenized protocol", got)
	}
}

func TestAudioEngineCommandLifecycle(t *testing.T) {
	app = App{done: make(chan struct{})}
	defer audioShutdown()

	for i, command := range []string{"PING", "VOLUME 0.60", "STOP", "PING"} {
		if err := audioSend(command); err != nil {
			t.Fatalf("audio control command %d (%q) acknowledgement failed: %v", i+1, command, err)
		}
	}
}

func TestAudioEnginePlayReportsActualMediaOpenOutcome(t *testing.T) {
	app = App{done: make(chan struct{})}
	defer audioShutdown()

	wavPath := filepath.Join(t.TempDir(), "open-outcome.wav")
	if err := os.WriteFile(wavPath, silentWAV(8000, 800), 0600); err != nil {
		t.Fatalf("write playback WAV: %v", err)
	}
	fileURL := (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(wavPath)}).String()
	encoded := base64.StdEncoding.EncodeToString([]byte(fileURL))
	err := audioSend(audioPlayCommand(0, encoded, 0.00))
	if err != nil {
		if os.Getenv("CI") != "" && strings.Contains(strings.ToUpper(err.Error()), "0XC00D11BA") {
			t.Logf("headless CI has no usable Windows audio endpoint; structured MediaFailed outcome confirmed: %v", err)
			return
		}
		t.Fatalf("event-confirmed media open failed: %v", err)
	}
	if err := audioSendExisting("STOP"); err != nil {
		t.Fatalf("STOP after successful media open failed: %v", err)
	}
}

func silentWAV(sampleRate, samples int) []byte {
	dataSize := samples * 2
	out := make([]byte, 44+dataSize)
	copy(out[0:4], "RIFF")
	putLE32(out[4:8], uint32(36+dataSize))
	copy(out[8:12], "WAVE")
	copy(out[12:16], "fmt ")
	putLE32(out[16:20], 16)
	putLE16(out[20:22], 1)
	putLE16(out[22:24], 1)
	putLE32(out[24:28], uint32(sampleRate))
	putLE32(out[28:32], uint32(sampleRate*2))
	putLE16(out[32:34], 2)
	putLE16(out[34:36], 16)
	copy(out[36:40], "data")
	putLE32(out[40:44], uint32(dataSize))
	return out
}

func putLE16(dst []byte, v uint16) {
	dst[0] = byte(v)
	dst[1] = byte(v >> 8)
}

func putLE32(dst []byte, v uint32) {
	dst[0] = byte(v)
	dst[1] = byte(v >> 8)
	dst[2] = byte(v >> 16)
	dst[3] = byte(v >> 24)
}

func TestStationNameSearchPathsExcludeKnownBrokenCatalogRows(t *testing.T) {
	paths := stationNameSearchPaths("Radio Test", "HR", "")
	if len(paths) != 2 {
		t.Fatalf("search paths = %#v; want country-specific plus global fallback", paths)
	}
	for _, path := range paths {
		if !strings.Contains(path, "hidebroken=true") {
			t.Fatalf("recovery search must exclude known broken rows: %q", path)
		}
		if strings.Contains(path, "hidebroken=false") {
			t.Fatalf("recovery search re-enabled broken catalog rows: %q", path)
		}
		if !strings.Contains(path, "order=votes") || !strings.Contains(path, "reverse=true") {
			t.Fatalf("recovery search must prefer established catalog alternatives: %q", path)
		}
	}
	if !strings.Contains(paths[0], "countrycode=HR") {
		t.Fatalf("country-specific recovery path = %q; want HR filter", paths[0])
	}

	diaspora := stationNameSearchPaths("Radio Diaspora", diasporaCatalogCode, "DE")
	if len(diaspora) != 2 || !strings.Contains(diaspora[0], "countrycode=DE") {
		t.Fatalf("diaspora recovery paths = %#v; want source-country filter", diaspora)
	}
}

func TestPlaybackRecoveryCandidatesPreferFreshCatalogBeforePersistedBackups(t *testing.T) {
	station := RadioStation{
		StationUUID: "station-1",
		Name:        "Radio Test",
		CountryCode: "HR",
		URLResolved: "https://current.example/live.mp3",
		URL:         "https://current.example/listen",
	}
	refreshed := &RadioStation{
		StationUUID: "station-1",
		Name:        "Radio Test",
		CountryCode: "HR",
		URLResolved: "https://fresh.example/live.aac",
		URL:         "https://fresh.example/listen",
		LastCheckOK: 1,
	}
	alternatives := []RadioStation{
		{
			StationUUID: "station-1",
			Name:        "Radio Test",
			CountryCode: "HR",
			URLResolved: "https://alt.example/live.mp3",
			URL:         "https://alt.example/listen",
			LastCheckOK: 1,
		},
		{
			StationUUID: "other",
			Name:        "Different Radio",
			CountryCode: "HR",
			URLResolved: "https://wrong.example/live.mp3",
		},
	}
	backups := []string{
		"https://backup.example/live.mp3",
		"https://current.example/live.mp3",
	}
	got := playbackRecoveryCandidates(station, refreshed, alternatives, backups)
	want := []string{
		"https://current.example/live.mp3",
		"https://current.example/listen",
		"https://backup.example/live.mp3",
		"https://fresh.example/live.aac",
		"https://fresh.example/listen",
		"https://alt.example/live.mp3",
		"https://alt.example/listen",
	}
	if len(got) != len(want) {
		t.Fatalf("recovery candidates = %#v; want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candidate %d = %q; want %q (all=%#v)", i, got[i], want[i], got)
		}
	}
	if playbackRecoveryCandidateLimit < 5 {
		t.Fatalf("recovery candidate limit = %d; must allow multiple mirrors", playbackRecoveryCandidateLimit)
	}
}

func TestPlaybackRecoveryCandidatesRejectBrokenUUIDRefresh(t *testing.T) {
	station := RadioStation{
		StationUUID: "station-1",
		Name:        "Radio Test",
		CountryCode: "HR",
		URLResolved: "https://current.example/live.mp3",
	}
	brokenRefresh := &RadioStation{
		StationUUID: "station-1",
		Name:        "Radio Test",
		CountryCode: "HR",
		URLResolved: "https://broken-refresh.example/live.mp3",
		LastCheckOK: 0,
	}
	got := playbackRecoveryCandidates(
		station,
		brokenRefresh,
		nil,
		[]string{"https://backup.example/live.mp3"},
	)
	for _, candidate := range got {
		if candidate == brokenRefresh.URLResolved {
			t.Fatalf("known-broken UUID refresh leaked into playback recovery: %#v", got)
		}
	}
	if len(got) < 2 || got[1] != "https://backup.example/live.mp3" {
		t.Fatalf("persisted backup was not reserved near the front of recovery: %#v", got)
	}
}

func TestHomeCatalogEnabledAtMinimumWindowHeight(t *testing.T) {
	if !homeCatalogEnabled(600, "all", "", "", "HR") {
		t.Fatal("dense home must remain enabled within the client area of the minimum outer window")
	}
	if homeCatalogEnabled(599, "all", "", "", "HR") {
		t.Fatal("dense home must fall back below its safe client height")
	}
	if homeCatalogEnabled(720, "popular", "", "", "HR") {
		t.Fatal("non-home tabs must use the library view")
	}
	if homeCatalogEnabled(720, "croatia", "", "", "HR") {
		t.Fatal("full Croatia list must use the library view, not the landing page")
	}
	if homeCatalogEnabled(720, "all", "radio", "", "HR") {
		t.Fatal("search results must use the library view")
	}
	if homeCatalogEnabled(720, "all", "", "rock", "HR") {
		t.Fatal("genre results must use the library view")
	}
	for _, country := range []string{"", "BA", "RS", "DIA", "INT"} {
		if homeCatalogEnabled(720, "all", "", "", country) {
			t.Fatalf("Croatia home must not render for country %q", country)
		}
	}
}

func TestBrowseGridColumnsRemainResponsive(t *testing.T) {
	tests := []struct {
		width int32
		want  int
	}{
		{620, 2},
		{779, 2},
		{780, 3},
		{1079, 3},
		{1080, 4},
		{1500, 4},
	}
	for _, tc := range tests {
		if got := browseGridColumns(tc.width); got != tc.want {
			t.Fatalf("browseGridColumns(%d) = %d; want %d", tc.width, got, tc.want)
		}
	}
}

func TestHomeGridColumnsStayWithinDenseThreeToSixColumnContract(t *testing.T) {
	tests := []struct {
		width int32
		want  int
	}{
		{760, 3},
		{839, 3},
		{840, 4},
		{1119, 4},
		{1120, 5},
		{1379, 5},
		{1380, 6},
		{1700, 6},
	}
	for _, tc := range tests {
		if got := homeGridColumns(tc.width); got != tc.want {
			t.Fatalf("homeGridColumns(%d) = %d; want %d", tc.width, got, tc.want)
		}
	}
}

func TestCroatiaListIsDistinctFromHomeLanding(t *testing.T) {
	if !isValidTab("croatia") {
		t.Fatal("croatia list tab must be a valid internal navigation state")
	}
	if homeCatalogEnabled(760, "croatia", "", "", "HR") {
		t.Fatal("croatia list must not re-enter the home landing page")
	}
	if !homeCatalogEnabled(760, "all", "", "", "HR") {
		t.Fatal("home landing page contract changed unexpectedly")
	}
}

func TestSelectableCatalogCodesIncludeSupplementalGroups(t *testing.T) {
	for _, code := range []string{"", "HR", "BA", diasporaCatalogCode, foreignCatalogCode} {
		if !isSelectableCatalogCode(code) {
			t.Fatalf("catalog code %q should be selectable", code)
		}
	}
	for _, code := range []string{"ZZ", "LOCAL", "127"} {
		if isSelectableCatalogCode(code) {
			t.Fatalf("unsupported catalog code %q unexpectedly selectable", code)
		}
	}
}

func TestBuildBrowseCountsMatchesCountryAndGenreSemantics(t *testing.T) {
	stations := []RadioStation{
		{CountryCode: "HR", Tags: "folk,pop", TagsIndex: foldText("folk,pop")},
		{CountryCode: "BA", Tags: "jazz,news", TagsIndex: foldText("jazz,news")},
		{CountryCode: "HR", Tags: "oldies", TagsIndex: foldText("oldies")},
	}
	countries, genres := buildBrowseCounts(stations)
	if countries[""] != 3 || countries["HR"] != 2 || countries["BA"] != 1 {
		t.Fatalf("country counts = %#v; want total=3 HR=2 BA=1", countries)
	}
	if genres[""] != 3 {
		t.Fatalf("all-genre count = %d; want 3", genres[""])
	}
	for genre, want := range map[string]int{"folk": 1, "pop": 1, "jazz": 1, "news": 1, "oldies": 1} {
		if got := genres[genre]; got != want {
			t.Fatalf("genre %q count = %d; want %d (all=%#v)", genre, got, want, genres)
		}
	}
}

func TestHomeDiscoverySnapshotScalesToFullCatalogWithoutDuplicates(t *testing.T) {
	codes := []string{"HR", "BA", "RS", "SI", "MK", "AL", "ME", "BG"}
	stations := make([]RadioStation, 0, 6000)
	for i := 0; i < 6000; i++ {
		code := codes[i%len(codes)]
		tags := "pop,regional"
		if i%3 == 0 {
			tags = "folk,narodna"
		}
		stations = append(stations, RadioStation{
			StationUUID: fmt.Sprintf("scale-%05d", i),
			Name:        fmt.Sprintf("Scale Radio %05d", i),
			CountryCode: code,
			Tags:        tags,
			TagsIndex:   foldText(tags),
			Votes:       100000 - i,
		})
	}
	start := time.Now()
	snapshot := buildHomeDiscoverySnapshot(stations, 6, 77)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("6000-station home discovery took %s; paint-safe snapshot must stay bounded", elapsed)
	}
	if !snapshot.Valid || snapshot.Revision != 77 || snapshot.Columns != 6 {
		t.Fatalf("unexpected snapshot metadata: %+v", snapshot)
	}
	if snapshot.CroatiaCount != 750 {
		t.Fatalf("Croatia count = %d; want 750", snapshot.CroatiaCount)
	}
	groups := [][]int{
		snapshot.Popular, snapshot.Croatia, snapshot.Bosnia, snapshot.Serbia,
		snapshot.Balkan, snapshot.Folk, snapshot.PopRock,
	}
	seen := map[int]bool{}
	for groupIndex, group := range groups {
		for _, idx := range group {
			if idx < 0 || idx >= len(stations) {
				t.Fatalf("group %d contains invalid station index %d", groupIndex, idx)
			}
			if seen[idx] {
				t.Fatalf("station index %d is duplicated across home discovery groups", idx)
			}
			seen[idx] = true
		}
	}
	if len(snapshot.Popular) != 6 || len(snapshot.Croatia) != 12 || len(snapshot.Bosnia) != 6 || len(snapshot.Serbia) != 6 || len(snapshot.Balkan) != 6 {
		t.Fatalf("unexpected primary group sizes: popular=%d HR=%d BA=%d RS=%d Balkan=%d",
			len(snapshot.Popular), len(snapshot.Croatia), len(snapshot.Bosnia), len(snapshot.Serbia), len(snapshot.Balkan))
	}
}

func TestCompactPlayerTransportDoesNotOverlapFavorite(t *testing.T) {
	cx := playerTransportCenter(1024)
	if left := cx - 116; left <= 432 {
		t.Fatalf("compact previous-track hit starts at %d; must be right of favorite x=432", left)
	}
	if got := playerTransportCenter(1240); got != 620 {
		t.Fatalf("normal-width transport center = %d; want 620", got)
	}
}

func TestGenreBrowseScrollsAtCompactMinimumHeight(t *testing.T) {
	compactContentWidth := int32(1024) - sidebarWidth - mainPad*2
	if got := browsePageMaxScroll(680, compactContentWidth, "genres"); got <= 0 {
		t.Fatalf("compact genre browse max scroll = %d; want positive", got)
	}
	if got := browsePageMaxScroll(680, compactContentWidth, "countries"); got != 0 {
		t.Fatalf("compact country browse max scroll = %d; want 0", got)
	}
	normalContentWidth := int32(1240) - sidebarWidth - mainPad*2
	if got := browsePageMaxScroll(820, normalContentWidth, "genres"); got != 0 {
		t.Fatalf("normal genre browse max scroll = %d; want 0", got)
	}
}
