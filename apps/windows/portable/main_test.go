//go:build windows

package main

import "testing"

func TestSafeHTTPURLRejectsPrivateAndCredentialedTargets(t *testing.T) {
	tests := []string{
		"",
		"file:///C:/Windows/System32/drivers/etc/hosts",
		"http://localhost:8080/live",
		"http://127.0.0.1/live",
		"http://10.0.0.4/live",
		"http://172.16.1.2/live",
		"http://192.168.1.10/live",
		"http://169.254.169.254/latest/meta-data/",
		"http://100.64.0.1/live",
		"https://metadata.google.internal/computeMetadata/v1/",
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
