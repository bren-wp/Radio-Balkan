# Radio Balkan browser ekstenzije 0.0.26

Produkcijski source za Chrome, Edge, Opera i Firefox.

## Produkcijski fokus u 0.0.26

- Previous/Next zahtijevaju stvarnu `current` stanicu; cold start više ne može implicitno pokrenuti adjacent playback
- cold-state regression test provjerava disabled Prev/Next i potvrđuje da klik ne šalje `RB_PLAY`
- filter-aware adjacent navigacija iz v0.0.25 ostaje aktivna nakon stvarnog odabira stanice
- promjena favorite statusa ponovno primjenjuje filtre
- odfavoritiranje u favorites-only prikazu odmah uklanja stanicu iz vidljivog seta, bez dodatnog refresh događaja
- postojeći Stop → fresh Play, session/epoch/revision guardovi, 12-sekundni `audio.play()` timeout, stall recovery i bounded UUID refresh ostaju aktivni
- nema novih browser dozvola, remote codea ni telemetry koda

## Build

```bash
cd extensions
python tools/build_extensions.py
```

Build generira Chrome, Edge, Opera i Firefox ZIP pakete i odbija version drift, branding/permission regresije te ključne playback, network-safety, UI-state i lifecycle regresije.
