//go:build windows

package main

import "testing"

func TestStartupHomeStateAlwaysOpensHomeAndPreservesPreferences(t *testing.T) {
	original := PersistedState{
		Favorites:         map[string]bool{"station-1": true},
		Replacements:      map[string]string{"station-1": "https://example.com/live"},
		Backups:           map[string][]string{"station-1": {"https://example.com/backup"}},
		Recent:            []string{"station-1"},
		Volume:            37,
		HealthIntervalMin: 45,
		CountryCode:       foreignCatalogCode,
		Genre:             "rock",
		Tab:               "favorites",
		WindowWidth:       1360,
		WindowHeight:      820,
	}

	got := startupHomeState(validateState(original, true))
	if got.Tab != "all" || got.CountryCode != "HR" || got.Genre != "" {
		t.Fatalf("startup navigation = tab %q country %q genre %q; want home/all + HR + empty genre", got.Tab, got.CountryCode, got.Genre)
	}
	if !got.Favorites["station-1"] || got.Volume != 37 || got.HealthIntervalMin != 45 {
		t.Fatalf("startup reset changed durable preferences: favorites=%#v volume=%d health=%d", got.Favorites, got.Volume, got.HealthIntervalMin)
	}
	if got.Replacements["station-1"] != "https://example.com/live" || len(got.Backups["station-1"]) != 1 {
		t.Fatalf("startup reset changed source preferences: replacements=%#v backups=%#v", got.Replacements, got.Backups)
	}
	if got.WindowWidth != 1360 || got.WindowHeight != 820 {
		t.Fatalf("startup reset changed window size to %dx%d", got.WindowWidth, got.WindowHeight)
	}
}
