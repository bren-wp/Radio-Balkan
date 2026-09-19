# Radio Balkan browser ekstenzije 0.0.25

Produkcijski source za Chrome, Edge, Opera i Firefox.

## Produkcijski fokus u 0.0.25

- popup player izlaže **Previous / Stop / Play-Pause / Next**
- Previous/Next rade unutar aktivno filtriranog skupa; puni katalog je fallback samo kada nema vidljivih rezultata
- promjena filtera odmah sinkronizira enabled/disabled stanje adjacent kontrola
- compact 360 px prikaz skriva samo dekorativni artwork kako naziv stanice i transport ostanu čitljivi
- autoritativni cold state bez remote stanice čisti lokalni optimistic current item, pa se stanica koju korisnik nije odabrao ne prikazuje kao aktivna niti se može favorizirati iz playera
- testovi provjeravaju konkretan station payload za Previous/Next, Stop → fresh Play, cold-state cleanup i postojeće session/epoch/revision zaštite
- 12-sekundni `audio.play()` timeout, 15-sekundni stall recovery i bounded UUID refresh iz v0.0.24 ostaju aktivni
- nema novih browser dozvola, remote codea ni telemetry koda

## Build

```bash
cd extensions
python tools/build_extensions.py
```

Build generira Chrome, Edge, Opera i Firefox ZIP pakete i odbija version drift, branding/permission regresije te ključne playback, network-safety, UI-state i lifecycle regresije.
