# Radio Balkan browser ekstenzije 0.0.5

Produkcijski source za Chrome, Edge, Opera i Firefox. Sve varijante dijele isti UI, katalog i mrežnu validaciju; razlikuje se samo playback sloj potreban za pojedini preglednik.

## Produkcijska pravila

- canonical naziv i toolbar identitet zaključani su na **Radio Balkan**
- nema korisničke ili skrivene postavke za promjenu brenda
- build namjerno pada ako se promijeni naziv brenda ili doda options/settings rebranding UI
- nema `unlimitedStorage` dozvole
- nema vanjskih UI biblioteka, remote codea ni telemetry koda
- katalog je ograničen i prikazuje se u kontroliranim stranicama
- logotipi se učitavaju lijeno radi manjeg RAM/CPU opterećenja
- stream i logo URL-ovi prolaze jedan zajednički security validator umjesto duplicirane logike
- Chromium koristi MV3 service worker + offscreen audio; Firefox koristi zaseban playback sloj uz isti shared UI
- verzija se provjerava prema centralnom repozitorijskom `VERSION` izvoru

## Build

```bash
python tools/build_extensions.py
```

Skripta generira četiri produkcijska ZIP-a u `dist/` i prekida build kod neusklađene verzije, promijenjenog kanonskog brenda, branding/settings stranice ili zabranjene dozvole.
