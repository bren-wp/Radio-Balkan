# Radio Balkan 0.0.20 — Windows

Nativna Windows x64 aplikacija i installer bez Electrona, ugrađenog browser runtimea ili telemetry SDK-a. Portable i Setup grade se iz istog sourcea i istog Portable payload-a.

## Produkcijski fokus u 0.0.20

- Setup je potpuno upotrebljiv tipkovnicom: Tab/strelice mijenjaju fokus, Enter/Space aktiviraju, Escape zatvara kada instalacija nije u tijeku
- checkbox opcije imaju veće click-targete koji uključuju i tekst labele, uz vidljivo fokus stanje
- source fallback verzija Portable i Setup aplikacije više nije zastarjeli hardcoded string; version tooling je sinkronizira s root `VERSION`
- release build i dalje linkerom postavlja `main.appVersion`, koristi `-trimpath`, uklanja VCS metadata i stripped simbole
- state zapis koristi durable temp-write + sync + backup/rename putanju, a finalni shutdown persistence/audio cleanup je idempotentan
- mrežni transport razrješava i validira javne IP adrese prije TCP spajanja te blokira privatne/lokalne/metadata ciljeve
- installer provjerava SHA-256 instaliranog executabla prije nastavka
- Portable i Setup prolaze strogi line-ending-neovisni `gofmt` gate, Go vet/test/build te clean-worktree provjeru u CI-ju
- `WM_GETMINMAXINFO` minimal-size handler koristi kontrolirani memory-copy put umjesto direktnog callback `uintptr → unsafe.Pointer` casta

## Build

Na Windowsu s aktualnim Go toolchainom:

```powershell
cd apps/windows
./build-release.ps1 -Version (Get-Content ../../VERSION).Trim()
```

Rezultat su `RadioBalkan-Portable-v0.0.20.exe`, `RadioBalkan-Setup-v0.0.20.exe` i SHA-256 manifest. Produkcijski release artefakti ponovno se grade u GitHub Publish workflowu iz release commita.
