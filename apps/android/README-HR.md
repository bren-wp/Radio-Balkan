# Radio Balkan 0.0.20 — Android

Nativni Android klijent (`minSdk 26`, `targetSdk 36`) bez telemetry SDK-a i bez vlastitog backend servisa za reprodukciju. Javni radio stream reproducira se izravno s izvora postaje.

## Produkcijski fokus u 0.0.20

- jasniji premium UI s ripple/pressed feedbackom na glavnim, navigation i station kontrolama
- donja navigacija više ne prikazuje nepostojeći korisnički Profil; `Više` otvara stvarne aplikacijske opcije
- filteri imaju jasni `Filtriraj` CTA, trenutno vidljiv selected state i `Poništi filtre`
- Radio Browser `votes` prikazuju se točno kao `Popularnost · N glasova`, a ne kao trenutačni broj slušatelja
- repository load je zaštićen od executor/shutdown racea i ne propušta `RejectedExecutionException` prema UI threadu
- Activity pri destroyu čisti cijeli Handler queue, a repository, image loader i player imaju eksplicitne shutdown putanje
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
