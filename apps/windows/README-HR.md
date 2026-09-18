# Radio Balkan 0.0.21 — Windows

Nativna Windows x64 aplikacija i installer bez Electrona, ugrađenog browser runtimea ili telemetry SDK-a. Portable i Setup grade se iz istog sourcea i istog Portable payload-a.

## Produkcijski fokus u 0.0.21

- Setup app payload, uninstaller i ikona zapisuju se preko durable writera koji radi puni write i `Sync()` prije rename/commit koraka
- installer i dalje prije nastavka provjerava SHA-256 stvarno instaliranog executabla, a ne samo ugrađeni payload
- Setup zadržava potpunu keyboard kontrolu: Tab/strelice mijenjaju fokus, Enter/Space aktiviraju, Escape zatvara kada instalacija nije u tijeku
- checkbox opcije imaju proširene click-targete koji uključuju tekst labele i vidljiv fokus
- Portable state zapis i dalje koristi durable temp-write + sync + backup/rename putanju, a finalni shutdown persistence/audio cleanup ostaje idempotentan
- station i hero play kontrole aktivne stanice ostaju pause/resume toggle s jasnim `Pauziraj` / `Nastavi` tekstom
- mrežni transport i dalje razrješava i validira javne IP adrese prije TCP spajanja te blokira privatne/lokalne/metadata ciljeve
- Portable i Setup prolaze line-ending-neovisni `gofmt` gate, Go vet/test/build te clean-worktree provjeru u CI-ju

## Build

Na Windowsu s aktualnim Go toolchainom:

```powershell
cd apps/windows
./build-release.ps1 -Version (Get-Content ../../VERSION).Trim()
```

Rezultat su `RadioBalkan-Portable-v0.0.21.exe`, `RadioBalkan-Setup-v0.0.21.exe` i SHA-256 manifest. Produkcijski release artefakti ponovno se grade u GitHub Publish workflowu iz release commita.
