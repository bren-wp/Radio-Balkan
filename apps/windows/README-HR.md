# Radio Balkan 0.0.25 — Windows

Nativna Windows x64 aplikacija i installer bez Electrona, ugrađenog browser runtimea ili telemetry SDK-a.

## Produkcijski fokus u 0.0.25

- Previous/Next transport se onemogućuje kada vidljivi skup nema barem dvije stanice
- Stop je disabled bez aktualne stanice i nakon terminalnog Stop-a; disabled kontrole nemaju hit-region
- veliki donji Play bez prethodnog odabira pokreće prvu filtriranu stanicu ili prvi valjani katalog item
- volume `−` na 0% i `+` na 100% vizualno su disabled i nisu klikabilni
- transport availability pravila pokrivena su pure Go regression testovima
- postojeći PresentationCore `READY` + `OK/ERR`, ACK timeouti, MCI fallback i Stop → fresh reconnect ostaju aktivni
- CI i dalje izvršava lokalni `Play → Pause → Volume → Resume → Stop → Play → Stop`, fail-fast Go gateove i stvarni Portable runtime soak

## Build

```powershell
cd apps/windows
./build-release.ps1 -Version (Get-Content ../../VERSION).Trim()
```

Rezultat su `RadioBalkan-Portable-v0.0.25.exe`, `RadioBalkan-Setup-v0.0.25.exe` i SHA-256 manifest.
