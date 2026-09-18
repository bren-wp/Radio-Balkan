# Radio Balkan browser ekstenzije 0.0.24

Produkcijski source za Chrome, Edge, Opera i Firefox.

## Produkcijski fokus u 0.0.24

- popup player dobiva zaseban **Stop** control s disabled, busy i accessibility stanjima
- UI razlikuje aktivno `Pauzirano` stanje od terminalnog `Zaustavljeno`
- glavni Play nakon Stop-a šalje svježi `RB_PLAY`, pa ne pokušava Toggle umirovljene sesije
- Chromium worker izlaže aktualni `sessionId`; Firefox već vraća isti session signal kroz snapshot
- Chromium i Firefox nakon iscrpljenja postojećih kandidata jednom po sesiji mogu obnoviti stream URL po station UUID-u
- UUID refresh koristi fiksne Radio Browser API hostove, 512 KiB response limit, 4 s per-request timeout i 9 s ukupni recovery budžet
- regionalna/INT country pravila ostaju fail-closed, a privatni/lokalni literal URL-ovi i credentialed URL-ovi ostaju blokirani
- STOP ili nova session/generation vrijednost poništavaju zakašnjeli refresh tako da stari async rezultat ne može ponovno pokrenuti audio
- postojeći 12-sekundni `audio.play()` timeout i 15-sekundni `waiting/stalled` recovery ostaju session/generation-bound
- nema novih browser dozvola, remote codea ni telemetry koda

## Build

```bash
cd extensions
python tools/build_extensions.py
```

Build generira Chrome, Edge, Opera i Firefox ZIP pakete i odbija version drift, branding/permission regresije te ključne playback, network-safety i lifecycle regresije.
