# Radio Balkan 0.0.22 — Windows

Nativna Windows x64 aplikacija i installer bez Electrona, ugrađenog browser runtimea ili telemetry SDK-a. Portable i Setup grade se iz istog sourcea i istog Portable payload-a.

## Produkcijski fokus u 0.0.22

- Portable startup health provjera više ne kreće istodobno s prvim renderom/katalogom; odgođena je 8 sekundi i početno radi nad manjim, prioritetnim skupom stanica
- PresentationCore audio engine pokreće se u background warmupu, objavljuje `READY` prije prihvaćanja naredbi i za svaku naredbu vraća `OK/ERR`; naredbe imaju ograničene timeoutove, a postojeći MCI fallback ostaje aktivan
- Stop → Play sada radi svježi reconnect umjesto pokušaja resumea već zatvorenog sourcea
- promjena glasnoće šalje se aktivnom audio engineu i kada je reprodukcija pauzirana
- shutdown prvo prekida background rad, a završni state/audio cleanup ima vremenski limit kako UI thread ne bi ostao beskonačno blokiran pri zatvaranju
- CI pokreće stvarni Portable executable kroz 15-sekundni startup soak i zahtijeva da proces/prozor ostanu živi te da se aplikacija uredno zatvori
- `build-release.ps1` eksplicitno provjerava exit code svakog Portable/Setup `go vet`, `go test` i `go build` koraka; neuspjeli Go test više ne može završiti lažno-zelenim buildom
- postojeći durable Portable state zapis, durable Setup payload write, installer SHA-256 provjera, URL/DNS zaštite i keyboard installer UX ostaju aktivni

## Build

Na Windowsu s aktualnim Go toolchainom:

```powershell
cd apps/windows
./build-release.ps1 -Version (Get-Content ../../VERSION).Trim()
```

Rezultat su `RadioBalkan-Portable-v0.0.22.exe`, `RadioBalkan-Setup-v0.0.22.exe` i SHA-256 manifest. Produkcijski release artefakti ponovno se grade u GitHub Publish workflowu iz release commita.
