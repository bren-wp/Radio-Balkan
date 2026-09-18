# Radio Balkan 0.0.23 — Android

Nativni Android klijent (`minSdk 26`, `targetSdk 36`) bez telemetry SDK-a i bez vlastitog backend servisa za reprodukciju.

## Produkcijski fokus u 0.0.23

- eksplicitni Stop ne može završiti kao Resume stare servisne sesije; ista stanica nakon Stop-a koristi svježi `PLAY`
- Play/Pause/Resume odluke centralizirane su kroz `PlaybackLifecycle` i pokrivene JVM unit testovima
- foreground player i dalje faila zatvoreno ako foreground promocija nije moguća
- `MediaPlayer.prepareAsync()` watchdog, sigurni fallback kandidati, audio focus, MediaSession i noisy-headset lifecycle ostaju aktivni
- startup health scan ostaje odgođen i ograničen; URL/DNS/redirect i response limiti ostaju aktivni
- release ostaje R8-minificiran/resource-shrinkan; potpisani APK zahtijeva signing secrets izvan repozitorija

## Produkcijski build

```bash
cd apps/android
./build-apk.sh
```

CI izvršava unit testove, Android Lint, release build i clean-worktree provjeru.
