# Radio Balkan 0.0.5 — Windows

Nativna Windows x64 aplikacija i installer bez Electrona, browser runtimea ili telemetry SDK-a. Izdanje 0.0.5 fokusirano je na produkcijsku stabilnost, manji RAM/CPU otisak i čišći UI.

## Što je novo u 0.0.5

- uklonjeni ugrađeni JPG hero/genre fallbackovi i njihov raster cache
- fallback artwork, hero i genre vizuali crtaju se proceduralno bez dodatnog bitmap memorijskog troška
- Go soft memory limit postavljen na 192 MB
- logo cache ograničen na 48 stavki
- katalog ograničen na 7000 stanica uz strože mrežne response limite
- health scan koristi dva workera i manji automatski uzorak stanica
- početne i periodične health provjere dodatno su reducirane
- korisničko sučelje nema debug, QA ni developer tekstove
- Portable i Setup grade se s `-trimpath`, bez VCS metadata i sa stripped simbolima
- verzija se provjerava iz centralnog repozitorijskog `VERSION` izvora kroz GitHub CI

## Build

Na Windowsu s aktualnim Go toolchainom:

```powershell
./build-release.ps1 -Version 0.0.5
```

Rezultat su Portable i Setup x64 artefakti. Produkcijski release artefakti objavljuju se preko GitHub Releases workflowa iz istog commita.
