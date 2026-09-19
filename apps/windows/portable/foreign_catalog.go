//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"
)

const (
	foreignCatalogCode       = "INT"
	diasporaCatalogCode      = "DIA"
	foreignCatalogLimit      = 120
	diasporaCatalogLimit     = 120
	foreignCatalogScanLimit  = 1000
	diasporaQueryLimit       = 100
	supplementalCatalogLimit = foreignCatalogLimit + diasporaCatalogLimit
	regionalCatalogLimit     = 7240 - supplementalCatalogLimit
	foreignCatalogRefresh    = 30 * time.Minute
	foreignCatalogReadyPoll  = 120 * time.Millisecond
)

var regionalCatalogCodes = map[string]struct{}{
	"HR": {}, "BA": {}, "RS": {}, "SI": {}, "MK": {}, "AL": {}, "ME": {},
}

type diasporaCatalogQuery struct {
	Field string
	Value string
}

// initCatalogGroups extends the existing selector with two application-level groups.
// DIA and INT are not ISO country codes and are never sent to Radio Browser as
// country filters.
func initCatalogGroups() {
	if len(balkanCountries) > 0 && balkanCountries[0].Code == "" {
		balkanCountries[0].Name = "Sve postaje"
	}
	ensureCatalogGroup(diasporaCatalogCode, "Dijaspora")
	ensureCatalogGroup(foreignCatalogCode, "Strano")
}

func ensureCatalogGroup(code, name string) {
	for _, item := range balkanCountries {
		if strings.EqualFold(item.Code, code) {
			return
		}
	}
	balkanCountries = append(balkanCountries, CountryDef{Code: code, Name: name})
}

func init() {
	initCatalogGroups()
	if strings.HasSuffix(strings.ToLower(filepath.Base(os.Args[0])), ".test.exe") {
		return
	}
	go supplementalCatalogSupervisor()
}

func isRegionalCatalogCode(code string) bool {
	_, ok := regionalCatalogCodes[strings.ToUpper(strings.TrimSpace(code))]
	return ok
}

func isForeignCatalogCode(code string) bool {
	return strings.EqualFold(strings.TrimSpace(code), foreignCatalogCode)
}

func isDiasporaCatalogCode(code string) bool {
	return strings.EqualFold(strings.TrimSpace(code), diasporaCatalogCode)
}

func isSupplementalCatalogCode(code string) bool {
	return isForeignCatalogCode(code) || isDiasporaCatalogCode(code)
}

func supplementalCatalogSupervisor() {
	for {
		if existing, _, _ := procFindWindow.Call(uintptr(unsafe.Pointer(u16(className))), 0); existing != 0 {
			break
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

	waitForPrimaryCatalog()
	if shuttingDown() {
		return
	}
	if err := syncSupplementalCatalog(); err != nil {
		logError("supplemental-catalog-startup", err)
	}

	ticker := time.NewTicker(foreignCatalogRefresh)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := syncSupplementalCatalog(); err != nil && !shuttingDown() {
				logError("supplemental-catalog-periodic", err)
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

func fetchDiasporaCatalog() ([]RadioStation, error) {
	queries := []diasporaCatalogQuery{
		{Field: "tag", Value: "diaspora"},
		{Field: "name", Value: "balkan"},
		{Field: "name", Value: "ex yu"},
		{Field: "language", Value: "croatian"},
		{Field: "language", Value: "serbian"},
		{Field: "language", Value: "bosnian"},
		{Field: "language", Value: "macedonian"},
		{Field: "language", Value: "albanian"},
		{Field: "language", Value: "slovenian"},
	}
	var last error
	for _, base := range apiBases() {
		rows, err := fetchDiasporaFromBase(base, queries)
		if err != nil {
			last = err
			continue
		}
		catalog := normalizeDiasporaCatalog(rows)
		if len(catalog) > 0 {
			return catalog, nil
		}
		last = errors.New("katalog dijaspore nije vratio upotrebljive postaje")
	}
	if last == nil {
		last = errors.New("katalog dijaspore nije dostupan")
	}
	return nil, last
}

func fetchDiasporaFromBase(base string, queries []diasporaCatalogQuery) ([]RadioStation, error) {
	if len(queries) == 0 {
		return nil, errors.New("nema upita za dijasporu")
	}
	jobs := make(chan diasporaCatalogQuery)
	results := make(chan []RadioStation, len(queries))
	errs := make(chan error, len(queries))
	workers := 3
	if workers > len(queries) {
		workers = len(queries)
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for query := range jobs {
				if shuttingDown() {
					return
				}
				endpoint := strings.TrimRight(base, "/") + "/json/stations/search?" +
					url.QueryEscape(query.Field) + "=" + url.QueryEscape(query.Value) +
					"&hidebroken=true&order=votes&reverse=true&limit=" + fmt.Sprint(diasporaQueryLimit)
				var rows []RadioStation
				if err := getJSON(endpoint, &rows); err != nil {
					errs <- err
					continue
				}
				results <- rows
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, query := range queries {
			select {
			case jobs <- query:
			case <-app.done:
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
		close(errs)
	}()

	var out []RadioStation
	for rows := range results {
		out = append(out, rows...)
	}
	if len(out) > 0 {
		return out, nil
	}
	var last error
	for err := range errs {
		last = err
	}
	if last == nil {
		last = errors.New("katalog dijaspore nije dostupan")
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
		station.SourceCountryCode = actualCode
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
	return sortAndCapSupplemental(out, foreignCatalogLimit)
}

func normalizeDiasporaCatalog(rows []RadioStation) []RadioStation {
	out := make([]RadioStation, 0, minInt(len(rows), diasporaCatalogLimit*4))
	for _, station := range rows {
		actualCode := strings.ToUpper(strings.TrimSpace(station.CountryCode))
		if actualCode == "" || isRegionalCatalogCode(actualCode) || station.LastCheckOK != 1 {
			continue
		}
		if !safeHTTPURL(station.URLResolved) && !safeHTTPURL(station.URL) {
			continue
		}
		station.SourceCountryCode = actualCode
		station.CountryCode = diasporaCatalogCode
		if strings.TrimSpace(station.Country) == "" {
			station.Country = "Dijaspora"
		}
		if !containsFoldedTag(station.Tags, "dijaspora") {
			if strings.TrimSpace(station.Tags) == "" {
				station.Tags = "dijaspora"
			} else {
				station.Tags = "dijaspora," + station.Tags
			}
		}
		out = append(out, station)
	}
	return sortAndCapSupplemental(out, diasporaCatalogLimit)
}

func sortAndCapSupplemental(rows []RadioStation, limit int) []RadioStation {
	rows = dedupeStations(rows)
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Votes == rows[j].Votes {
			return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
		}
		return rows[i].Votes > rows[j].Votes
	})
	if len(rows) > limit {
		rows = append([]RadioStation(nil), rows[:limit]...)
	}
	return rows
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

func keepCurrentInSupplemental(list []RadioStation, current RadioStation, currentKey string, limit int) []RadioStation {
	if currentKey == "" {
		return list
	}
	for _, station := range list {
		if stationKey(station) == currentKey {
			return list
		}
	}
	if len(list) >= limit && len(list) > 0 {
		list[len(list)-1] = current
		return list
	}
	return append(list, current)
}

func syncSupplementalCatalog() error {
	if shuttingDown() {
		return context.Canceled
	}

	type result struct {
		kind string
		list []RadioStation
		err  error
	}
	ch := make(chan result, 2)
	go func() {
		list, err := fetchForeignCatalog()
		ch <- result{kind: "foreign", list: list, err: err}
	}()
	go func() {
		list, err := fetchDiasporaCatalog()
		ch <- result{kind: "diaspora", list: list, err: err}
	}()

	var foreign, diaspora []RadioStation
	var foreignErr, diasporaErr error
	for i := 0; i < 2; i++ {
		select {
		case item := <-ch:
			if item.kind == "foreign" {
				foreign, foreignErr = item.list, item.err
			} else {
				diaspora, diasporaErr = item.list, item.err
			}
		case <-app.done:
			return context.Canceled
		}
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
	oldForeign := make([]RadioStation, 0, foreignCatalogLimit)
	oldDiaspora := make([]RadioStation, 0, diasporaCatalogLimit)
	for _, station := range old {
		switch {
		case isForeignCatalogCode(station.CountryCode):
			oldForeign = append(oldForeign, station)
		case isDiasporaCatalogCode(station.CountryCode):
			oldDiaspora = append(oldDiaspora, station)
		default:
			regional = append(regional, station)
		}
	}

	if foreignErr != nil || len(foreign) == 0 {
		foreign = oldForeign
	}
	if diasporaErr != nil || len(diaspora) == 0 {
		diaspora = oldDiaspora
	}
	if len(foreign) == 0 && len(diaspora) == 0 && foreignErr != nil && diasporaErr != nil {
		return fmt.Errorf("supplementalni katalog nije dostupan: strano=%v; dijaspora=%v", foreignErr, diasporaErr)
	}

	if len(regional) > regionalCatalogLimit {
		regional = append([]RadioStation(nil), regional[:regionalCatalogLimit]...)
		if playing && !isSupplementalCatalogCode(current.CountryCode) && currentKey != "" {
			found := false
			for _, station := range regional {
				if stationKey(station) == currentKey {
					found = true
					break
				}
			}
			if !found && len(regional) > 0 {
				regional[len(regional)-1] = current
			}
		}
	}

	if playing && currentKey != "" {
		if isDiasporaCatalogCode(current.CountryCode) {
			diaspora = keepCurrentInSupplemental(diaspora, current, currentKey, diasporaCatalogLimit)
		}
		if isForeignCatalogCode(current.CountryCode) {
			foreign = keepCurrentInSupplemental(foreign, current, currentKey, foreignCatalogLimit)
		}
	}

	combined := make([]RadioStation, 0, len(regional)+len(diaspora)+len(foreign))
	combined = append(combined, regional...)
	combined = append(combined, diaspora...)
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

// Kept for compatibility with existing tests and maintenance call sites.
func syncForeignCatalog() error {
	return syncSupplementalCatalog()
}
