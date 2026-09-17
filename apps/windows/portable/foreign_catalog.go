//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unsafe"
)

const (
	foreignCatalogCode      = "INT"
	foreignCatalogLimit     = 50
	foreignCatalogScanLimit = 500
	foreignCatalogRefresh   = 30 * time.Minute
	foreignCatalogReadyPoll = 120 * time.Millisecond
)

var regionalCatalogCodes = map[string]struct{}{
	"HR": {}, "BA": {}, "RS": {}, "SI": {}, "MK": {}, "AL": {}, "ME": {},
}

// initCatalogGroups extends the existing country selector with one application-level
// group. INT is deliberately not an ISO country code: it represents a curated global
// set and is never sent to Radio Browser as a real country filter by this module.
func initCatalogGroups() {
	if len(balkanCountries) > 0 && balkanCountries[0].Code == "" {
		balkanCountries[0].Name = "Sve postaje"
	}
	for _, item := range balkanCountries {
		if strings.EqualFold(item.Code, foreignCatalogCode) {
			return
		}
	}
	balkanCountries = append(balkanCountries, CountryDef{Code: foreignCatalogCode, Name: "Strano"})
}

func init() {
	initCatalogGroups()
	// Go test binaries do not create the production window. Avoid a dormant polling
	// goroutine there while still installing the catalog-group definitions for tests.
	if strings.HasSuffix(strings.ToLower(filepath.Base(os.Args[0])), ".test.exe") {
		return
	}
	go foreignCatalogSupervisor()
}

func isRegionalCatalogCode(code string) bool {
	_, ok := regionalCatalogCodes[strings.ToUpper(strings.TrimSpace(code))]
	return ok
}

func isForeignCatalogCode(code string) bool {
	return strings.EqualFold(strings.TrimSpace(code), foreignCatalogCode)
}

func foreignCatalogSupervisor() {
	// main assigns the App value before creating the native window. Waiting for the
	// window therefore avoids touching App while that one-time assignment happens.
	for {
		if existing, _, _ := procFindWindow.Call(uintptr(unsafe.Pointer(u16(className))), 0); existing != 0 {
			break
		}
		timer := time.NewTimer(foreignCatalogReadyPoll)
		<-timer.C
	}

	waitForPrimaryCatalog()
	if shuttingDown() {
		return
	}
	if err := syncForeignCatalog(); err != nil {
		logError("foreign-catalog-startup", err)
	}

	ticker := time.NewTicker(foreignCatalogRefresh)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := syncForeignCatalog(); err != nil && !shuttingDown() {
				logError("foreign-catalog-periodic", err)
			}
		case <-app.done:
			return
		}
	}
}

func waitForPrimaryCatalog() {
	for {
		if shuttingDown() {
			return
		}
		app.mu.RLock()
		ready := app.hwnd != 0 && !app.loading && !app.refreshRunning
		status := strings.ToLower(strings.TrimSpace(app.status))
		app.mu.RUnlock()
		if ready && !strings.Contains(status, "osvježavam") && !strings.Contains(status, "učitavanje") {
			return
		}
		timer := time.NewTimer(foreignCatalogReadyPoll)
		select {
		case <-timer.C:
		case <-app.done:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
	}
}

func fetchForeignCatalog() ([]RadioStation, error) {
	var last error
	for _, base := range apiBases() {
		select {
		case <-app.done:
			return nil, context.Canceled
		default:
		}
		endpoint := strings.TrimRight(base, "/") + "/json/stations/search?hidebroken=true&order=votes&reverse=true&limit=" + fmt.Sprint(foreignCatalogScanLimit)
		var rows []RadioStation
		if err := getJSON(endpoint, &rows); err != nil {
			last = err
			continue
		}
		catalog := normalizeForeignCatalog(rows)
		if len(catalog) > 0 {
			return catalog, nil
		}
		last = errors.New("globalni katalog nije vratio upotrebljive postaje")
	}
	if last == nil {
		last = errors.New("globalni katalog nije dostupan")
	}
	return nil, last
}

func normalizeForeignCatalog(rows []RadioStation) []RadioStation {
	out := make([]RadioStation, 0, minInt(len(rows), foreignCatalogScanLimit))
	for _, station := range rows {
		actualCode := strings.ToUpper(strings.TrimSpace(station.CountryCode))
		if actualCode == "" || isRegionalCatalogCode(actualCode) || station.LastCheckOK != 1 {
			continue
		}
		if !safeHTTPURL(station.URLResolved) && !safeHTTPURL(station.URL) {
			continue
		}
		station.CountryCode = foreignCatalogCode
		if strings.TrimSpace(station.Country) == "" {
			station.Country = "Strana postaja"
		}
		if !containsFoldedTag(station.Tags, "strano") {
			if strings.TrimSpace(station.Tags) == "" {
				station.Tags = "strano"
			} else {
				station.Tags = "strano," + station.Tags
			}
		}
		out = append(out, station)
	}
	out = dedupeStations(out)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Votes == out[j].Votes {
			return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
		}
		return out[i].Votes > out[j].Votes
	})
	if len(out) > foreignCatalogLimit {
		out = append([]RadioStation(nil), out[:foreignCatalogLimit]...)
	}
	return out
}

func containsFoldedTag(tags, needle string) bool {
	needle = foldText(needle)
	for _, tag := range strings.Split(tags, ",") {
		if foldText(strings.TrimSpace(tag)) == needle {
			return true
		}
	}
	return false
}

func syncForeignCatalog() error {
	if shuttingDown() {
		return context.Canceled
	}
	foreign, err := fetchForeignCatalog()
	if err != nil {
		return err
	}
	if len(foreign) == 0 {
		return errors.New("nema provjerenih stranih postaja")
	}

	app.mu.RLock()
	old := append([]RadioStation(nil), app.stations...)
	currentKey := app.currentKey
	currentIndex := currentStationIndexLocked()
	playing := app.playing
	var current RadioStation
	if currentIndex >= 0 && currentIndex < len(app.stations) {
		current = app.stations[currentIndex]
	}
	app.mu.RUnlock()

	regional := make([]RadioStation, 0, len(old))
	for _, station := range old {
		if !isForeignCatalogCode(station.CountryCode) {
			regional = append(regional, station)
		}
	}

	// Do not remove the station that is actively playing merely because its votes
	// moved it outside today's top 50. Replace the last foreign slot with it instead.
	if playing && isForeignCatalogCode(current.CountryCode) && currentKey != "" {
		found := false
		for _, station := range foreign {
			if stationKey(station) == currentKey {
				found = true
				break
			}
		}
		if !found {
			if len(foreign) >= foreignCatalogLimit {
				foreign[len(foreign)-1] = current
			} else {
				foreign = append(foreign, current)
			}
		}
	}

	combined := make([]RadioStation, 0, len(regional)+len(foreign))
	combined = append(combined, regional...)
	combined = append(combined, foreign...)
	combined = dedupeStations(combined)
	prepareStations(combined)
	mergeRuntimeStationState(old, combined)

	app.mu.Lock()
	app.stations = combined
	if currentKey != "" {
		if idx := findStationIndexLocked(currentKey, -1); idx >= 0 {
			app.current = idx
			app.currentKey = currentKey
			app.playing = playing
		}
	}
	app.mu.Unlock()

	saveCache(combined)
	postGenres()
	rebuildFilter()
	postUI()
	return nil
}
