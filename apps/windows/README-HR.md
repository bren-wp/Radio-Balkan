# Radio Balkan 0.0.25 — Windows

Nativna Windows x64 aplikacija i installer bez Electrona, ugrađenog browser runtimea ili telemetry SDK-a.

## Produkcijski fokus u 0.0.25

- Player transport sada je **state-aware**: Play, Stop, Prev i Next nemaju aktivnu hit-zonu kada odgovarajuća akcija nije valjana.
- Prev/Next zahtijevaju stvarno odabranu stanicu i više ne mogu neočekivano pokrenuti prvu/sljedeću postaju iz praznog player stanja.
- Stop je neaktivan nakon terminalnog zaustavljanja ili bez odabrane stanice.
- Globalni health-check i refresh katalog gumbi prikazuju busy/disabled stanje dok njihov posao traje.
- Station `Web` akcija prikazuje se samo za sigurnu javnu HTTP/HTTPS homepage adresu.
- Station-card action row koristi kompaktne metrike na užim karticama; regression test potvrđuje da ostaje unutar kartice od 390 px nadalje.
- Postojeći Play/Pause/Resume/Stop/replay lifecycle, ACK timeout isolation, thread-affinity i stvarni Portable runtime soak ostaju aktivni.

## Build

```powershell
cd apps/windows
./build-release.ps1 -Version (Get-Content ../../VERSION).Trim()
```

Rezultat su `RadioBalkan-Portable-v0.0.25.exe`, `RadioBalkan-Setup-v0.0.25.exe` i SHA-256 manifest.
