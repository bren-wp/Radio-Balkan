# Radio Balkan 0.0.25 — Android

Nativni Android klijent (`minSdk 26`, `targetSdk 36`) bez telemetry SDK-a i bez vlastitog backend servisa za reprodukciju.

## Produkcijski fokus u 0.0.25

- Prev / Play-Pause / Stop / Next ostaju inicijalno disabled dok player state nije spreman
- neuspjeli odabir ili povratak bez valjane aktualne stanice čisti stale player artwork
- postojeća adjacent wrap-around navigacija, explicit Stop → fresh Play i `PlaybackLifecycle` testovi ostaju aktivni
- foreground fail-closed, `MediaPlayer.prepareAsync()` watchdog, sigurni fallback kandidati, audio focus, MediaSession i noisy-headset lifecycle ostaju aktivni
- startup health scan ostaje odgođen i ograničen; URL/DNS/redirect i response limiti ostaju aktivni
- release ostaje R8-minificiran/resource-shrinkan; potpisani APK zahtijeva signing secrets izvan repozitorija
- nema novih dozvola, telemetryja ili trackinga

## Produkcijski build

```bash
cd apps/android
./build-apk.sh
```

CI izvršava `testReleaseUnitTest`, `lintRelease`, `assembleRelease` i clean-worktree provjeru.
