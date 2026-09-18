# Radio Balkan 0.0.24 — Android

Nativni Android klijent (`minSdk 26`, `targetSdk 36`) bez telemetry SDK-a i bez vlastitog backend servisa za reprodukciju.

## Produkcijski fokus u 0.0.24

- stalni player bar sada izlaže **Prev / Play-Pause / Stop / Next**
- Prev/Next koriste `PlaybackLifecycle.adjacentIndex` s testiranim wrap-around ponašanjem
- Stop šalje postojeći `ACTION_STOP`; nakon eksplicitnog Stop-a ista stanica i dalje koristi svježi `PLAY`, ne `RESUME`
- donji `Radio` više nije no-op nego vraća puni katalog i vrh liste
- pokretanje stanice iz Favorita više ne postavlja lažno odabrani Radio tab
- foreground fail-closed, `MediaPlayer.prepareAsync()` watchdog, fallback kandidati, audio focus, MediaSession i noisy-headset lifecycle ostaju aktivni
- startup health scan ostaje odgođen i ograničen; URL/DNS/redirect i response limiti ostaju aktivni
- nema novih dozvola, telemetryja ili trackinga

## Produkcijski build

```bash
cd apps/android
./build-apk.sh
```

CI izvršava `testReleaseUnitTest`, `lintRelease`, `assembleRelease` i clean-worktree provjeru.
