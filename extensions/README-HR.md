# Radio Balkan browser ekstenzije 0.0.21

Produkcijski source za Chrome, Edge, Opera i Firefox. Sve varijante dijele isti popup UI, katalog i sigurnosnu validaciju; razlikuje se playback sloj potreban za Chromium odnosno Firefox.

## Produkcijski fokus u 0.0.21

- prazni rezultati filtera imaju jasan akcijski CTA `Poništi filtre`, umjesto pasivnog empty-state teksta
- početni network/catalog failure prikazuje `Pokušaj ponovno` i ponovno pokreće kontrolirani forced refresh bez zatvaranja popup-a
- recovery kontrole koriste iste focus/pressed/premium stilove kao ostatak popup UI-ja
- popup i dalje lokalno pamti državu, žanr i Omiljene; `Očisti` vraća filtere jednim klikom
- station kartice ostaju keyboard dostupne preko Enter/Space, s vidljivim focus stateom i `aria-current`
- play/pause komanda zadržava single-flight UI guard i disabled/`aria-busy` semantiku
- ručni refresh i dalje razlikuje stvarni mrežni neuspjeh od cache fallbacka
- Radio Browser JSON ostaje streaming-limitiran, a dinamički API discovery ostaje ograničen na najviše osam baza
- Chromium i Firefox zadržavaju session/generation/revision zaštite i cleanup putanje
- nema `unlimitedStorage`, remote codea, telemetry koda ni korisničkog rebrandinga

## Build

```bash
cd extensions
python tools/build_extensions.py
```

Build generira Chrome, Edge, Opera i Firefox ZIP pakete i odbija version drift, promjenu kanonskog brenda, zabranjene dozvole ili regresiju ključnih playback/security ugovora.
