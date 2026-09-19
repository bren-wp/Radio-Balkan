# Radio Balkan 0.0.26 — Android

Nativni Android klijent (`minSdk 26`, `targetSdk 36`) bez telemetry SDK-a i bez vlastitog backend servisa za reprodukciju.

## Produkcijski fokus u 0.0.26

- `PlaybackLifecycle.canNavigate(size, hasCurrent)` centralizira adjacent availability
- Previous/Next ostaju disabled dok nema aktualne stanice, čak i ako katalog već sadrži više postaja
- JVM testovi zahtijevaju current station + najmanje dva kandidata
- postojeća wrap-around adjacent navigacija, explicit Stop → fresh Play i stale-artwork cleanup ostaju aktivni
- foreground fail-closed, `MediaPlayer.prepareAsync()` watchdog, sigurni fallback kandidati, audio focus, MediaSession i noisy-headset lifecycle ostaju aktivni
- nema novih dozvola, telemetryja ili trackinga

## Produkcijski build

```bash
cd apps/android
./build-apk.sh
```

CI izvršava `testReleaseUnitTest`, `lintRelease`, `assembleRelease` i clean-worktree provjeru.
