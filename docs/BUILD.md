# Build vodič

## Verzija izdanja

Kanonska verzija nalazi se u root `VERSION` datoteci. Za novo izdanje koristi jedan sinkronizirani bump, primjerice:

```bash
python scripts/bump_version.py 0.0.37
```

`bump_version.py` prvo radi preflight svih verzijskih markera bez pisanja u source. Nova SemVer vrijednost mora biti strogo veća od trenutačne, Android `versionCode` mora rasti, a README i ovaj BUILD vodič automatski dobivaju sljedeći patch primjer. Stvarni write koristi atomske replace operacije; ako završni `scripts/check_versions.py` validator padne, alat vraća sve originalne datoteke. Za provjeru bez promjena koristi `--dry-run`.

Skripta sinkronizira Windows build default, Portable/Setup source fallback `appVersion`, Android, browser metapodatke, README, BUILD primjer i version badge. Windows produkcijski Portable i Setup dodatno dobivaju isti `appVersion` kroz Go linker `-X main.appVersion=$Version`, pa i release i izravni/local source build koriste istu kanonsku verziju.

## Windows

Na Windowsu s aktualnim Go toolchainom:

```powershell
cd apps/windows
./build-release.ps1 -Version (Get-Content ../../VERSION).Trim()
```

Skripta radi read-only Go formatting provjeru na LF-normaliziranoj privremenoj kopiji, pa Windows CRLF checkout ne stvara lažni drift. Stvarni `gofmt` drift ruši build bez prepisivanja sourcea. Nakon toga izvršava `go vet`, `go test`, gradi Portable, privremeno ugrađuje isti binary u Setup, linkerom postavlja produkcijsku verziju, uklanja privremeni embedded payload u `finally` bloku i generira SHA-256. Od 0.0.22 svaki Portable/Setup `go vet`, `go test` i `go build` eksplicitno provjerava process exit code i odmah ruši build na grešci. Windows CI nakon builda pokreće i stvarni Portable runtime soak: prozor/proces mora ostati živ kroz startup/background fazu, a zatim CI-only in-process smoke način šalje `WM_CLOSE` vlastitom stvarnom Win32 prozoru i zahtijeva uredan završetak cijele shutdown putanje.

## Browser ekstenzije

```bash
cd extensions
python tools/build_extensions.py
```

Rezultat su četiri ZIP-a u `extensions/dist/`. Build dodatno provjerava JavaScript sintaksu, zaključani Radio Balkan branding, Chromium offscreen lifecycle, session-bound audio ownership te epoch/revision/command-ordering ugovore za Chromium i Firefox playere i popup UI. Regression suite u 0.0.24 dodatno izvršava Stop control lifecycle, Stop → fresh Play, bounded UUID stream refresh, country/URL safety, oversized-response fail-closed i Stop-during-refresh stale-session scenarije. U 0.0.25 dodatno se provjeravaju filter-aware Previous/Next payloadi, cold-state cleanup i transport enabled/disabled sinkronizacija. U 0.0.26 testovi dodatno zahtijevaju aktualnu stanicu za adjacent navigaciju na sva tri klijenta i potvrđuju trenutačno osvježavanje favorites-only rezultata nakon odfavoritiranja. U 0.0.27 security/UI contracti dodatno zaključavaju PBKDF2 admin verifier, logic-level RBAC za health/source kontrole, Enter/busy login lifecycle, vidljivi Admin status i post-logout odbijanje privilegirane browser source akcije.

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

Prije promocije na `main` GitHub CI provodi sinkronizaciju verzija, uključujući Windows source fallback metadata, `scripts/verify_security_contracts.py`, `scripts/verify_production_ui.py`, `scripts/test_release_checksums.py`, transactional/rollback test version alata, real-repository next-version `--dry-run`, izvršne browser popup, Chromium offscreen i Firefox player regression testove, browser build, Windows Portable/Setup testove uključujući durable installer-write i stvarni audio-engine lifecycle i ACK-timeout isolation, fail-fast Go vet/test/build, Windows startup/shutdown runtime soak, PowerShell AST provjeru tooling skripti i Android unit testove + release lint/build. Android JVM testovi dodatno zaključavaju Prev/Next wrap-around indeksiranje, a production UI contract zahtijeva stvarne Stop/Prev/Next wiring kontrole. `verify_production_ui.py` zaključava user-facing terminologiju, contextual Android permission flow, vidljive station opcije, browser accessibility/performance ugovore te minimalnu upotrebljivu Windows veličinu. Svaki platform build mora završiti s čistim repository workspaceom. Commitovi koji mijenjaju samo generirane `assets/screenshots/**` PNG datoteke preskaču puni multi-platform CI jer su sami screenshot capture koraci već provjereni u zasebnom workflowu.

Publish workflow ponovno gradi Windows, browser i Android artefakte, provjerava clean-worktree invariant, generira deterministički SHA-256 manifest koji ne može uključiti samoga sebe i odmah ga validira s `sha256sum -c` prije GitHub Release uploada. Automatski publish na `main` pokreće samo promjena `VERSION`; `workflow_dispatch` ostaje dostupan za eksplicitno ručno pokretanje, a postojeći tag/SHA guard odbija objavu iste verzije s drugog commita. Browser screenshot runtime nalazi se izvan `extensions/` i koristi se samo u privremenoj preview kopiji tijekom `Product screenshots` workflowa; produkcijski browser ZIP-ovi ga ne sadrže.
