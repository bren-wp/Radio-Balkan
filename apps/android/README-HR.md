# Radio Balkan 0.0.25 — Android

Nativni Android klijent (`minSdk 26`, `targetSdk 36`) bez telemetry SDK-a i bez vlastitog backend servisa za reprodukciju.

## Produkcijski fokus u 0.0.25

- svaki station row dobiva izravni **♡ / ♥** Favorite gumb umjesto obaveznog otvaranja `⋮` izbornika
- favorite-set se adapteru predaje kao memorijski snapshot; nema SharedPreferences čitanja za svaki row bind
- Favorite kontrola ima dinamičan accessibility opis i UI se odmah osvježava nakon promjene
- na ekranima užim od 390 dp artwork i desne kontrole koriste kompaktnije dimenzije kako naziv, metadata i status ostanu čitljivi
- UUID stream recovery prihvaća samo ograničeni identifikator `[A-Za-z0-9._:-]` duljine do 128 znakova i odbija path/query/oversized input
- višestruki Radio Browser UUID pokušaji dijele ukupni 12-sekundni recovery budžet; connect/read timeout svakog pokušaja ostaje unutar preostalog budžeta
- postojeći Prev / Play-Pause / Stop / Next, foreground fail-closed, MediaPlayer watchdog, audio focus, MediaSession i URL/DNS/redirect zaštite ostaju aktivni
- nema novih dozvola, telemetryja ni trackinga

## Produkcijski build

```bash
cd apps/android
./build-apk.sh
```

CI izvršava `testReleaseUnitTest`, `lintRelease`, `assembleRelease` i clean-worktree provjeru.
