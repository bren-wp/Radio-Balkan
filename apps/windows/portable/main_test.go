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

func TestStationCardActionLayoutFitsSupportedWidths(t *testing.T) {
	for _, tc := range []struct {
		width  int32
		hasWeb bool
	}{
		{width: 390, hasWeb: true},
		{width: 390, hasWeb: false},
		{width: 440, hasWeb: true},
		{width: 520, hasWeb: true},
	} {
		if end := stationCardActionEnd(tc.width, tc.hasWeb); end > tc.width {
			t.Fatalf("station actions overflow card width %d: end=%d hasWeb=%v", tc.width, end, tc.hasWeb)
		}
	}
}

func TestTransportAvailability(t *testing.T) {
	tests := []struct {
		name                         string
		current, stationCount        int
		stopped                      bool
		play, stop, navigate         bool
	}{
		{name: "empty catalog", current: -1, stationCount: 0},
		{name: "no selection", current: -1, stationCount: 3, navigate: true},
		{name: "single selected playing", current: 0, stationCount: 1, play: true, stop: true},
		{name: "single selected stopped", current: 0, stationCount: 1, stopped: true, play: true},
		{name: "multiple selected playing", current: 1, stationCount: 3, play: true, stop: true, navigate: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			play, stop, navigate := transportAvailability(tc.current, tc.stationCount, tc.stopped)
			if play != tc.play || stop != tc.stop || navigate != tc.navigate {
				t.Fatalf("transportAvailability(%d, %d, %v) = (%v,%v,%v); want (%v,%v,%v)",
					tc.current, tc.stationCount, tc.stopped, play, stop, navigate, tc.play, tc.stop, tc.navigate)
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
