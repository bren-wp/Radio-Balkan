# Radio Balkan browser ekstenzije 0.0.20

Produkcijski source za Chrome, Edge, Opera i Firefox. Sve varijante dijele isti popup UI, katalog i sigurnosnu validaciju; razlikuje se playback sloj potreban za Chromium odnosno Firefox.

## Produkcijski fokus u 0.0.20

- popup lokalno pamti državu, žanr i Omiljene između otvaranja
- `Očisti` vraća filtere jednim klikom
- station kartice podržavaju Enter/Space, vidljivi focus state i `aria-current`
- pressed, loading i reduced-motion stanja vizualno su dosljednija
- ručni refresh prikazuje stvarni mrežni neuspjeh; cache se više ne prikazuje kao da je refresh uspio
- Radio Browser JSON čita se streaming putem uz 512 KiB server-list i 8 MiB catalog limite
- dinamički API discovery ograničen je na četiri nova hosta + četiri stabilna fallbacka
- katalog ostaje paged/limited, a logotipi lazy-loadani
- Chromium koristi MV3 service worker + offscreen audio; Firefox zasebni playback sloj s istim session/generation/revision principima
- nema `unlimitedStorage`, remote codea, telemetry koda ni korisničkog rebrandinga
- canonical naziv i toolbar identitet ostaju zaključani na **Radio Balkan**

## Build

```bash
cd extensions
python tools/build_extensions.py
```

Build generira Chrome, Edge, Opera i Firefox ZIP pakete i odbija version drift, promjenu kanonskog brenda, zabranjene dozvole ili regresiju ključnih playback/security ugovora.
