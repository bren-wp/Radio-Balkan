# Radio Balkan 0.0.21 — Android

Nativni Android klijent (`minSdk 26`, `targetSdk 36`) bez telemetry SDK-a i bez vlastitog backend servisa za reprodukciju. Javni radio stream reproducira se izravno s izvora postaje.

## Produkcijski fokus u 0.0.21

- search overlay sada ima dosljedan lifecycle: ponovno pritiskanje gumba za pretragu ili Back zatvara search, skriva tipkovnicu, uklanja skriveni query i odmah vraća odgovarajući popis
- Back iz otvorene pretrage više ne izlazi iz Activityja prije nego što zatvori search stanje
- postojeći premium ripple/pressed feedback, selected chip state, `Poništi filtre`, health status i točna popularnost ostaju aktivni
- hero i player CTA i dalje prate Slušaj/Nastavi/Pauziraj stanje, a single-flight catalog refresh sprječava paralelne repository instance
- Activity pri destroyu i dalje prazni Handler queue, gasi repository/workere/image loader te ne ostavlja post-destroy UI posao
- player zadržava prepare watchdog, fallback izvore, audio focus, MediaSession i foreground-service lifecycle zaštite
- stream, logo, API i ICY mrežni putevi zadržavaju URL/DNS/redirect provjere i response limite
- bitmap cache je ograničen i reagira na Android memory-trim signale
- release ostaje R8-minificiran i resource-shrinkan

## Produkcijski build

Linux/macOS:

```bash
cd apps/android
./build-apk.sh
```

Windows PowerShell:

```powershell
cd apps/android
./build-apk.ps1
```

CI obavezno izvršava unit testove, Android Lint, release build i clean-worktree provjeru. Potpisani javni APK zahtijeva kompletno konfigurirane GitHub Actions signing secrets; privatni ključ i lozinke ne smiju biti u repozitoriju.
