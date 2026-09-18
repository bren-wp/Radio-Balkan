# Radio Balkan 0.0.22 — Android

Nativni Android klijent (`minSdk 26`, `targetSdk 36`) bez telemetry SDK-a i bez vlastitog backend servisa za reprodukciju. Javni radio stream reproducira se izravno s izvora postaje.

## Produkcijski fokus u 0.0.22

- foreground player više ne nastavlja rad u fatalnom polupokrenutom stanju ako Android odbije `startForeground()`/notification promociju; stanje se prijavi i servis se kontrolirano zaustavi
- MediaSession, audio-focus request i notification-channel inicijalizacija izolirane su tako da platformska/OEM iznimka ne ruši cijeli player servis
- automatski startup health scan odgođen je na 8 sekundi i ograničen na 16 prioritetnih stanica, čime prvi render i katalog više ne konkuriraju velikom broju stream probeova
- `MediaPlayer.prepareAsync()` zadržava watchdog timeout, release neodgovarajućeg playera i fallback na sljedeći kandidat
- pause/resume/stop, volume, audio focus, noisy-headset handling, MediaSession i foreground notification lifecycle ostaju povezani sa stvarnim playback stanjem
- Activity lifecycle i dalje čisti Handler queue, repository, workere i image loader; search/back cleanup iz 0.0.21 ostaje aktivan
- URL/DNS/redirect, image/API/ICY response limiti i bounded bitmap cache ostaju aktivni
- release ostaje R8-minificiran i resource-shrinkan; signed APK i dalje zahtijeva privatne signing secrets izvan repozitorija

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
