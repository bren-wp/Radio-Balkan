//go:build windows

package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestCatalogGroupsIncludeSupplementalGroupsExactlyOnce(t *testing.T) {
	initCatalogGroups()
	initCatalogGroups()

	if len(balkanCountries) == 0 || balkanCountries[0].Code != "" || balkanCountries[0].Name != "Sve postaje" {
		t.Fatalf("unexpected all-stations group: %#v", balkanCountries)
	}

	foreign, diaspora := 0, 0
	for _, item := range balkanCountries {
		switch item.Code {
		case foreignCatalogCode:
			foreign++
			if item.Name != "Strano" {
				t.Fatalf("foreign group label = %q; want Strano", item.Name)
			}
		case diasporaCatalogCode:
			diaspora++
			if item.Name != "Dijaspora" {
				t.Fatalf("diaspora group label = %q; want Dijaspora", item.Name)
			}
		}
	}
	if foreign != 1 || diaspora != 1 {
		t.Fatalf("supplemental group counts: foreign=%d diaspora=%d; want 1/1", foreign, diaspora)
	}
	if !isBalkanCode(foreignCatalogCode) {
		t.Fatal("INT application group must be accepted by existing selector/state validation")
	}
	if isRegionalCatalogCode(foreignCatalogCode) {
		t.Fatal("INT application group must never be treated as a real Balkan country")
	}
	if !isForeignCatalogCode(" int ") {
		t.Fatal("foreign group matching must be case/space insensitive")
	}
	if !isBalkanCode(diasporaCatalogCode) || isRegionalCatalogCode(diasporaCatalogCode) {
		t.Fatal("DIA must be accepted by the selector without becoming a real Balkan country")
	}
	if !isDiasporaCatalogCode(" dia ") {
		t.Fatal("diaspora group matching must be case/space insensitive")
	}
}

func TestNormalizeForeignCatalogFiltersSortsAndCaps(t *testing.T) {
	rows := make([]RadioStation, 0, foreignCatalogLimit+12)
	for i := 0; i < foreignCatalogLimit+5; i++ {
		rows = append(rows, RadioStation{
			StationUUID: fmt.Sprintf("foreign-%02d", i),
			Name:        fmt.Sprintf("Foreign %02d", i),
			URLResolved: fmt.Sprintf("https://stream%d.example.com/live.mp3", i),
			Country:     "United States",
			CountryCode: "US",
			Tags:        "pop,hits",
			Votes:       1000 - i,
			LastCheckOK: 1,
		})
	}

	rows = append(rows,
		RadioStation{StationUUID: "regional", Name: "Regional", URL: "https://regional.example.com/live", CountryCode: "HR", Votes: 5000, LastCheckOK: 1},
		RadioStation{StationUUID: "broken", Name: "Broken", URL: "https://broken.example.com/live", CountryCode: "GB", Votes: 6000, LastCheckOK: 0},
		RadioStation{StationUUID: "unsafe", Name: "Unsafe", URL: "http://127.0.0.1/live", CountryCode: "DE", Votes: 7000, LastCheckOK: 1},
		RadioStation{StationUUID: "blank-country", Name: "Blank", URL: "https://blank.example.com/live", CountryCode: "", Votes: 8000, LastCheckOK: 1},
	)

	got := normalizeForeignCatalog(rows)
	if len(got) != foreignCatalogLimit {
		t.Fatalf("foreign catalog length = %d; want %d", len(got), foreignCatalogLimit)
	}
	for i, station := range got {
		if station.CountryCode != foreignCatalogCode {
			t.Fatalf("station %d country code = %q; want %q", i, station.CountryCode, foreignCatalogCode)
		}
		if station.LastCheckOK != 1 {
			t.Fatalf("station %d retained a broken health state", i)
		}
		if !safeHTTPURL(station.URLResolved) && !safeHTTPURL(station.URL) {
			t.Fatalf("station %d retained an unsafe stream", i)
		}
		if !containsFoldedTag(station.Tags, "strano") {
			t.Fatalf("station %d missing Strano classification tag: %q", i, station.Tags)
		}
		if i > 0 && got[i-1].Votes < station.Votes {
			t.Fatalf("catalog is not sorted by votes at %d: %d < %d", i, got[i-1].Votes, station.Votes)
		}
	}
	for _, station := range got {
		if strings.EqualFold(station.StationUUID, "regional") || strings.EqualFold(station.StationUUID, "broken") ||
			strings.EqualFold(station.StationUUID, "unsafe") || strings.EqualFold(station.StationUUID, "blank-country") {
			t.Fatalf("filtered station leaked into foreign catalog: %s", station.StationUUID)
		}
	}
}

func TestNormalizeForeignCatalogDeduplicatesUUIDs(t *testing.T) {
	rows := []RadioStation{
		{StationUUID: "same", Name: "Example", URL: "https://a.example.com/live", CountryCode: "US", Votes: 10, LastCheckOK: 1},
		{StationUUID: "same", Name: "Example", URLResolved: "https://b.example.com/live", CountryCode: "GB", Votes: 25, LastCheckOK: 1},
	}
	got := normalizeForeignCatalog(rows)
	if len(got) != 1 {
		t.Fatalf("deduped catalog length = %d; want 1", len(got))
	}
	if got[0].Votes != 25 {
		t.Fatalf("dedupe did not retain higher-quality record: votes=%d", got[0].Votes)
	}
}

func TestNormalizeDiasporaCatalogFiltersSortsAndCaps(t *testing.T) {
	rows := make([]RadioStation, 0, diasporaCatalogLimit+8)
	for i := 0; i < diasporaCatalogLimit+5; i++ {
		rows = append(rows, RadioStation{
			StationUUID: fmt.Sprintf("diaspora-%03d", i),
			Name:        fmt.Sprintf("Balkan Diaspora %03d", i),
			URLResolved: fmt.Sprintf("https://diaspora%d.example.com/live", i),
			Country:     "Germany",
			CountryCode: "DE",
			Tags:        "balkan,hits",
			Votes:       2000 - i,
			LastCheckOK: 1,
		})
	}
	rows = append(rows,
		RadioStation{StationUUID: "regional-dia", Name: "Regional", URL: "https://regional-dia.example.com/live", CountryCode: "BA", Votes: 9000, LastCheckOK: 1},
		RadioStation{StationUUID: "broken-dia", Name: "Broken", URL: "https://broken-dia.example.com/live", CountryCode: "AT", Votes: 8000, LastCheckOK: 0},
		RadioStation{StationUUID: "unsafe-dia", Name: "Unsafe", URL: "http://127.0.0.1/live", CountryCode: "CH", Votes: 7000, LastCheckOK: 1},
	)

	got := normalizeDiasporaCatalog(rows)
	if len(got) != diasporaCatalogLimit {
		t.Fatalf("diaspora catalog length = %d; want %d", len(got), diasporaCatalogLimit)
	}
	for i, station := range got {
		if station.CountryCode != diasporaCatalogCode {
			t.Fatalf("station %d country code = %q; want %q", i, station.CountryCode, diasporaCatalogCode)
		}
		if !containsFoldedTag(station.Tags, "dijaspora") {
			t.Fatalf("station %d missing diaspora classification tag: %q", i, station.Tags)
		}
		if station.LastCheckOK != 1 || (!safeHTTPURL(station.URLResolved) && !safeHTTPURL(station.URL)) {
			t.Fatalf("station %d retained invalid availability or URL state", i)
		}
		if i > 0 && got[i-1].Votes < station.Votes {
			t.Fatalf("diaspora catalog is not sorted by votes at %d", i)
		}
	}
}

func TestSupplementalCatalogCountryMatching(t *testing.T) {
	if !matchesCatalogCountry(foreignCatalogCode, "DE", "DE") {
		t.Fatal("foreign recovery must accept its saved source country")
	}
	if !matchesCatalogCountry(diasporaCatalogCode, "AT", "AT") {
		t.Fatal("diaspora recovery must accept its saved source country")
	}
	if matchesCatalogCountry(diasporaCatalogCode, "AT", "HR") {
		t.Fatal("diaspora recovery must reject regional API rows")
	}
	if matchesCatalogCountry(foreignCatalogCode, "DE", "US") {
		t.Fatal("supplemental recovery with known source country must reject another country")
	}
	if !matchesCatalogCountry(diasporaCatalogCode, "", "CH") {
		t.Fatal("legacy diaspora cache without source metadata must still accept a non-regional UUID result")
	}
}

func TestDiasporaWinsDedupAndKeepsSourceCountry(t *testing.T) {
	base := RadioStation{
		StationUUID:       "same-uuid",
		Name:              "Radio Diaspora",
		URLResolved:       "https://diaspora.example/live",
		Country:           "Germany",
		CountryCode:       foreignCatalogCode,
		SourceCountryCode: "DE",
		Tags:              "hits",
		LastCheckOK:       1,
	}
	diaspora := base
	diaspora.CountryCode = diasporaCatalogCode
	diaspora.Tags = "hits,dijaspora"
	got := dedupeStations([]RadioStation{base, diaspora})
	if len(got) != 1 {
		t.Fatalf("dedupe length = %d; want 1", len(got))
	}
	if got[0].CountryCode != diasporaCatalogCode {
		t.Fatalf("deduped country code = %q; want DIA", got[0].CountryCode)
	}
	if got[0].SourceCountryCode != "DE" {
		t.Fatalf("deduped source country = %q; want DE", got[0].SourceCountryCode)
	}
	if stationFlagCode(got[0]) != "DE" {
		t.Fatalf("flag code = %q; want DE", stationFlagCode(got[0]))
	}
}
