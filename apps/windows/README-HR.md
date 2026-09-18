# Radio Balkan 0.0.23 — Windows

Nativna Windows x64 aplikacija i installer bez Electrona, ugrađenog browser runtimea ili telemetry SDK-a.

## Produkcijski fokus u 0.0.23

- Portable i Setup drže Win32 prozor i message loop na istom OS threadu.
- PresentationCore helper koristi `READY` + `OK/ERR`; `PLAY` ima 5 s ACK budžet, kontrolne naredbe 2 s.
- Timeout ili write failure odbacuju helper proces i ACK kanal prije fallbacka, pa zakašnjeli `OK` ne može potvrditi sljedeću naredbu.
- CI stvarno izvršava `Play → Pause → Volume → Resume → Stop → Play → Stop` nad lokalnim WAV zapisom.
- Stop → Play radi svježi reconnect; MCI ostaje fallback, a 15-sekundni startup/runtime soak i fail-fast Go gateovi ostaju obavezni.

## Build

```powershell
cd apps/windows
./build-release.ps1 -Version (Get-Content ../../VERSION).Trim()
```

Rezultat su `RadioBalkan-Portable-v0.0.23.exe`, `RadioBalkan-Setup-v0.0.23.exe` i SHA-256 manifest.
