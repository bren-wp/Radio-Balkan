//go:build windows

package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestCatalogGroupsIncludeForeignExactlyOnce(t *testing.T) {
	initCatalogGroups()
	initCatalogGroups()

	if len(balkanCountries) == 0 || balkanCountries[0].Code != "" || balkanCountries[0].Name != "Sve postaje" {
		t.Fatalf("unexpected all-stations group: %#v", balkanCountries)
	}

	foreign := 0
	for _, item := range balkanCountries {
		if item.Code == foreignCatalogCode {
			foreign++
			if item.Name != "Strano" {
				t.Fatalf("foreign group label = %q; want Strano", item.Name)
			}
		}
	}
	if foreign != 1 {
		t.Fatalf("foreign group count = %d; want 1", foreign)
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
}

func TestNormalizeForeignCatalogFiltersSortsAndCaps(t *testing.T) {
	rows := make([]RadioStation, 0, 72)
	for i := 0; i < 64; i++ {
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
