# Radio Balkan browser ekstenzije 0.0.22

Produkcijski source za Chrome, Edge, Opera i Firefox. Sve varijante dijele isti popup UI, katalog i sigurnosnu validaciju; razlikuje se playback sloj potreban za Chromium odnosno Firefox.

## Produkcijski fokus u 0.0.22

- Chromium offscreen i Firefox player ograničavaju početni `audio.play()` pokušaj na 12 sekundi; kandidat koji ostane pending više ne blokira player beskonačno
- `waiting` / `stalled` stanje ima 15-sekundni recovery koji, ako je session/generation još aktualan, oslobađa zaglavljeni audio i pokušava sljedeći siguran stream kandidat
- novi izvršni regression testovi simuliraju hanging play promise, stalled stream, stale session i close/play race ponašanje
- Chromium offscreen lifecycle i Firefox session ownership i dalje koriste generation/session zaštite kako stari async rezultat ne bi prepisao noviju reprodukciju
- popup single-flight play/pause guard, recovery CTA-ovi, spremljene UI preference i keyboard accessibility ostaju aktivni
- Radio Browser JSON response limiti, ograničen API discovery, privatni/lokalni network blocking i URL validacija ostaju nepromijenjeni
- nema novih browser dozvola, remote codea, telemetry koda ni korisničkog rebrandinga

## Build

```bash
cd extensions
python tools/build_extensions.py
```

Build generira Chrome, Edge, Opera i Firefox ZIP pakete i odbija version drift, promjenu kanonskog brenda, zabranjene dozvole ili regresiju ključnih playback/security ugovora.
