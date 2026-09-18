//go:build windows

package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"
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

func TestAudioEngineCommandLifecycle(t *testing.T) {
	app = App{done: make(chan struct{})}
	defer audioShutdown()

	commands := []string{"VOLUME 0.25", "PAUSE", "RESUME", "STOP", "VOLUME 0.50"}
	for _, command := range commands {
		if err := audioSend(command); err != nil {
			t.Fatalf("audio engine command %q acknowledgement failed: %v", command, err)
		}
	}
}
