# Radio Balkan browser ekstenzije 0.0.23

Produkcijski source za Chrome, Edge, Opera i Firefox.

## Produkcijski fokus u 0.0.23

- Chromium i Firefox regression testovi pokrivaju `Play → Pause → Resume → Stop → Play`
- Pause/Resume zadržava istu audio instancu; Stop umirovljuje sesiju i Toggle je ne smije ponovno oživjeti
- novi Play nakon Stop-a mora dobiti svježu session identifikaciju i ponovno pokrenuti playback
- postojeći 12-sekundni `audio.play()` timeout i 15-sekundni `waiting/stalled` recovery ostaju session/generation-bound
- stale async rezultat, close/play race i popup epoch/revision/command-ordering zaštite ostaju pod izvršnim testovima
- nema novih browser dozvola, remote codea ni telemetry koda

## Build

```bash
cd extensions
python tools/build_extensions.py
```

Build generira Chrome, Edge, Opera i Firefox ZIP pakete i odbija version drift, branding/permission regresije i ključne playback/security regresije.
