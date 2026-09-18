# Radio Balkan 0.0.24 — Windows

Nativna Windows x64 aplikacija i installer bez Electrona, ugrađenog browser runtimea ili telemetry SDK-a.

## Produkcijski fokus u 0.0.24

- donji player sada izlaže stvarni **Stop** transport control; postojeći handler više nije skriven od korisnika
- `Prikaži sve →` uz Žanrove ima stvarni hit target i otvara postojeći genre selector
- Prev/Next, Play/Pause, volume, favorite, health provjere i replacement-source funkcije ostaju aktivne
- Portable i Setup i dalje drže Win32 prozor/message loop na istom OS threadu
- PresentationCore helper zadržava `READY` + `OK/ERR`, bounded ACK čekanja i helper reset prije MCI fallbacka
- CI i dalje stvarno izvršava lokalni `Play → Pause → Volume → Resume → Stop → Play → Stop` te 15-sekundni Portable runtime soak
- Go vet/test/build fail-fast i clean-worktree gateovi ostaju obavezni

## Build

```powershell
cd apps/windows
./build-release.ps1 -Version (Get-Content ../../VERSION).Trim()
```

Rezultat su `RadioBalkan-Portable-v0.0.24.exe`, `RadioBalkan-Setup-v0.0.24.exe` i SHA-256 manifest.
