# Build vodič

## Verzija izdanja

Kanonska verzija nalazi se u root `VERSION` datoteci. Za novo izdanje koristi jedan sinkronizirani bump, primjerice:

```bash
python scripts/bump_version.py 0.0.8
```

Skripta sinkronizira Windows build default, Android, browser metapodatke, README i version badge te zatim pokreće `scripts/check_versions.py`. Windows produkcijski Portable i Setup dobivaju `appVersion` kroz Go linker `-X main.appVersion=$Version`, pa release bump ne prepisuje velike Win32 source datoteke samo radi verzijskog fallback stringa.

## Windows

Na Windowsu s aktualnim Go toolchainom:

```powershell
cd apps/windows
./build-release.ps1 -Version (Get-Content ../../VERSION).Trim()
```

Skripta radi `gofmt`, `go vet`, `go test`, gradi Portable, ugrađuje isti binary u Setup, linkerom postavlja produkcijsku verziju i generira SHA-256.

## Browser ekstenzije

```bash
cd extensions
python tools/build_extensions.py
```

Rezultat su četiri ZIP-a u `extensions/dist/`. Build dodatno provjerava JavaScript sintaksu, zaključani Radio Balkan branding i player-state/offscreen sigurnosne kontrakte.

## Android

Potrebni su JDK 17+, Android SDK 36 i kompatibilan Gradle/Android Gradle Plugin toolchain.

```bash
cd apps/android
./build-apk.sh
```

Produkcijski Android build izvršava `testReleaseUnitTest`, `lintRelease` i `assembleRelease`; release se ne smatra spremnim dok unit testovi ili Android Lint prijavljuju grešku. Za potpisani release koristi GitHub Actions Secrets / environment varijable opisane u `apps/android/README-HR.md`. Privatni ključevi ne pripadaju repozitoriju.

## CI provjere

Prije promocije na `main` GitHub CI provodi sinkronizaciju verzija, `scripts/verify_security_contracts.py`, browser build, Windows test/vet/build i Android unit testove + release lint/build.
