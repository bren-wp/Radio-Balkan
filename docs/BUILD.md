# Build vodič

## Verzija izdanja

Kanonska verzija nalazi se u root `VERSION` datoteci. Za novo izdanje koristi jedan sinkronizirani bump, primjerice:

```bash
python scripts/bump_version.py 0.0.13
```

Skripta sinkronizira Windows build default, Android, browser metapodatke, README i version badge te zatim pokreće `scripts/check_versions.py`. Windows produkcijski Portable i Setup dobivaju `appVersion` kroz Go linker `-X main.appVersion=$Version`, pa release bump ne prepisuje velike Win32 source datoteke samo radi verzijskog fallback stringa.

## Windows

Na Windowsu s aktualnim Go toolchainom:

```powershell
cd apps/windows
./build-release.ps1 -Version (Get-Content ../../VERSION).Trim()
```

Skripta radi read-only `gofmt -d` provjeru, `go vet`, `go test`, gradi Portable, privremeno ugrađuje isti binary u Setup, linkerom postavlja produkcijsku verziju, uklanja privremeni embedded payload u `finally` bloku i generira SHA-256. Build namjerno ne prepisuje Go source datoteke.

## Browser ekstenzije

```bash
cd extensions
python tools/build_extensions.py
```

Rezultat su četiri ZIP-a u `extensions/dist/`. Build dodatno provjerava JavaScript sintaksu, zaključani Radio Balkan branding, Chromium offscreen lifecycle, session-bound audio ownership te epoch/revision/command-ordering ugovore za Chromium i Firefox playere i popup UI.

## Android

Potrebni su JDK 17+, Android SDK 36 i kompatibilan Gradle/Android Gradle Plugin toolchain.

```bash
cd apps/android
./build-apk.sh
```

Produkcijski Android build izvršava `testReleaseUnitTest`, `lintRelease` i `assembleRelease`; release se ne smatra spremnim dok unit testovi ili Android Lint prijavljuju grešku. Za potpisani release koristi GitHub Actions Secrets / environment varijable opisane u `apps/android/README-HR.md`. Privatni ključevi ne pripadaju repozitoriju.

## Čist build workspace

Nakon builda koristi se zajednička provjera:

```bash
python scripts/check_clean_worktree.py
```

Provjera ruši pipeline ako build promijeni tracked source ili ostavi neignorirani privremeni sadržaj. Isti contract vrijedi za Windows, browser i Android CI buildove, Windows/browser/Android publish buildove te Windows build unutar Product screenshots workflowa prije nego screenshot koraci smiju mijenjati PNG datoteke.

## CI i release provjere

Prije promocije na `main` GitHub CI provodi sinkronizaciju verzija, `scripts/verify_security_contracts.py`, izvršne browser popup i Firefox player regression testove, browser build, Windows test/vet/build, PowerShell AST provjeru screenshot capture skripte i Android unit testove + release lint/build. Svaki platform build mora završiti s čistim repository workspaceom. Commitovi koji mijenjaju samo generirane `assets/screenshots/**` PNG datoteke preskaču puni multi-platform CI jer su sami screenshot capture koraci već provjereni u zasebnom workflowu.

Publish workflow ponovno gradi Windows, browser i Android artefakte, provjerava clean-worktree invariant, generira SHA-256 manifest i odmah ga validira s `sha256sum -c` prije GitHub Release uploada. Browser screenshot runtime nalazi se izvan `extensions/` i koristi se samo u privremenoj preview kopiji tijekom `Product screenshots` workflowa; produkcijski browser ZIP-ovi ga ne sadrže.
