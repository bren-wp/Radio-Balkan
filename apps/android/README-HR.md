# Radio Balkan 0.0.5 — Android

Nativni Android source za Radio Balkan (`minSdk 26`, `targetSdk 36`) bez telemetry SDK-a i bez vanjskog backend servisa za reprodukciju. Aplikacija reproducira javne radio streamove izravno s njihovih izvora.

## Produkcijska poboljšanja

- 15-sekundni MediaPlayer prepare watchdog sprječava beskonačno čekanje problematičnog streama
- health/source poslovi koriste zajednički ograničeni I/O pool
- automatski health scan ograničen je na 32 prioritetne stanice i jedan worker
- katalog i mrežni odgovori imaju stroge memorijske limite
- bitmap cache je dinamički ograničen, a slike se dekodiraju u ciljanu veličinu
- uklonjen nepotreban Wi-Fi lock i pripadajuća dozvola
- uklonjeni duplicirani country mapping i mrtvi artwork helper
- regex normalizacija koristi prekompajlirane uzorke
- release koristi R8 minifikaciju i resource shrinking
- `versionCode` i `versionName` usklađeni su s centralnim repozitorijskim `VERSION` izvorom
- GitHub Actions pokreće lint i release build; nema GitLab/vanjskog CI ovisnog toka

## Produkcijski build

Linux/macOS:

```bash
./build-apk.sh
```

Windows PowerShell:

```powershell
./build-apk.ps1
```

Za potpisani javni APK signing podatke postavi kao GitHub Actions secrets. Privatni ključ i lozinke ne smiju se spremati u repozitorij.
