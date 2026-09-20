//go:build windows

package main

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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

func TestAudioEngineCommandLifecycle(t *testing.T) {
	app = App{done: make(chan struct{})}
	defer audioShutdown()

	wavPath := filepath.Join(t.TempDir(), "lifecycle.wav")
	if err := os.WriteFile(wavPath, silentWAV(8000, 800), 0600); err != nil {
		t.Fatalf("write lifecycle WAV: %v", err)
	}
	fileURL := (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(wavPath)}).String()
	encoded := base64.StdEncoding.EncodeToString([]byte(fileURL))
	play := fmt.Sprintf("PLAY %s 0.25", encoded)

	commands := []string{play, "PAUSE", "VOLUME 0.60", "RESUME", "STOP", play, "STOP"}
	for i, command := range commands {
		if err := audioSend(command); err != nil {
			t.Fatalf("audio lifecycle command %d (%q) acknowledgement failed: %v", i+1, command, err)
		}
		if i == 0 || i == 5 {
			time.Sleep(75 * time.Millisecond)
		}
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


func TestHomeCatalogEnabledAtMinimumWindowHeight(t *testing.T) {
	if !homeCatalogEnabled(720, "all", "", "") {
		t.Fatal("dense home must remain enabled at the minimum supported window height")
	}
	if homeCatalogEnabled(699, "all", "", "") {
		t.Fatal("dense home must fall back below its safe height")
	}
	if homeCatalogEnabled(720, "popular", "", "") {
		t.Fatal("non-home tabs must use the library view")
	}
	if homeCatalogEnabled(720, "all", "radio", "") {
		t.Fatal("search results must use the library view")
	}
	if homeCatalogEnabled(720, "all", "", "rock") {
		t.Fatal("genre results must use the library view")
	}
}

func TestHomeGridColumnsStayWithinDenseThreeToFiveColumnContract(t *testing.T) {
	tests := []struct {
		width int32
		want  int
	}{
		{760, 3},
		{929, 3},
		{930, 4},
		{1179, 4},
		{1180, 5},
		{1500, 5},
	}
	for _, tc := range tests {
		if got := homeGridColumns(tc.width); got != tc.want {
			t.Fatalf("homeGridColumns(%d) = %d; want %d", tc.width, got, tc.want)
		}
	}
}
