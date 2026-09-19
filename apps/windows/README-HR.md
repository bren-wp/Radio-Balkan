# Radio Balkan 0.0.26 — Windows

Nativna Windows x64 aplikacija i installer bez Electrona, ugrađenog browser runtimea ili telemetry SDK-a.

## Produkcijski fokus u 0.0.26

- Previous/Next zahtijevaju valjanu aktualnu stanicu i barem dvije dostupne stanice u aktivnom skupu
- renderer i `playAdjacent` handler koriste isti `canNavigateStations(current, stationCount, filteredCount)` contract
- cold start bez odabrane stanice više nema adjacent hit-regione
- Go regression testovi pokrivaju no-current, single/multiple catalog i filtered scenarije
- postojeći Stop availability, početni Play fallback i volume 0%/100% disabled pravila ostaju aktivni
- PresentationCore `READY` + `OK/ERR`, ACK timeouti, MCI fallback i Stop → fresh reconnect ostaju aktivni
- CI i dalje izvršava fail-fast Go gateove, lokalni audio lifecycle i stvarni Portable runtime soak

## Build

```powershell
cd apps/windows
./build-release.ps1 -Version (Get-Content ../../VERSION).Trim()
```

Rezultat su `RadioBalkan-Portable-v0.0.26.exe`, `RadioBalkan-Setup-v0.0.26.exe` i SHA-256 manifest.
