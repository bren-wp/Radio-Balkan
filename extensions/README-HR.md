# Radio Balkan browser ekstenzije 0.0.25

Produkcijski source za Chrome, Edge, Opera i Firefox.

## Produkcijski fokus u 0.0.25

Browser runtime u ovom izdanju namjerno zadržava hardening iz 0.0.24 bez širenja dozvola ili playback arhitekture:

- zaseban Stop control i dalje razlikuje `Pauzirano` od terminalnog `Zaustavljeno`
- glavni Play nakon Stop-a i dalje otvara svježu playback session
- Chromium/Firefox UUID stream refresh ostaje session/generation-bound i ograničen response/time budžetima
- stale refresh nakon Stop-a ili nove sesije ne može ponovno pokrenuti audio
- popup preference, accessibility, single-flight command i network-safety regression testovi ostaju obavezni
- nema novih browser dozvola, remote codea, analyticsa ni telemetryja

## Build

```bash
cd extensions
python tools/build_extensions.py
```

Build generira Chrome, Edge, Opera i Firefox ZIP pakete i odbija version drift, branding/permission regresije te playback/network-safety/lifecycle regresije.
