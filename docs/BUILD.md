# Build vodič

## Windows

Na Windowsu s aktualnim Go toolchainom:

```powershell
cd apps/windows
./build-release.ps1 -Version 0.0.5
```

Skripta radi `gofmt`, `go vet`, `go test`, gradi Portable, ugrađuje isti binary u Setup i generira SHA-256.

## Browser ekstenzije

```bash
cd extensions
python tools/build_extensions.py
```

Rezultat su četiri ZIP-a u `extensions/dist/`.

## Android

Potrebni su JDK 17+, Android SDK 36 i kompatibilan Gradle/Android Gradle Plugin toolchain.

```bash
cd apps/android
./build-apk.sh
```

Za potpisani release koristi GitHub Actions Secrets / environment varijable opisane u `apps/android/README-HR.md`. Privatni ključevi ne pripadaju repozitoriju.
