# Changelog — browser ekstenzije


## 0.0.20

- popup lokalno pamti country, genre i favorites-only preference između otvaranja
- dodan `Očisti` filter control i jasniji filter-state UX
- station kartice dobile su keyboard Enter/Space aktivaciju, tabindex, `aria-current` i vidljivi focus state
- pressed/busy/reduced-motion CSS stanja dodatno su ispolirana
- aktivna play/pause komanda blokira isti duplicirani klik do završetka i izlaže disabled + `aria-busy` state; regression test zaključava single-flight ponašanje
- forced refresh više ne vraća cache kao lažno uspješan mrežni refresh; korisnik dobiva točan failure status
- dodani izvršni regression testovi za saved UI preferences i forced-refresh failure semantiku
- bounded streaming JSON response, API discovery cap, session/generation/revision player zaštite i locked Radio Balkan brand ostaju aktivni
- verzija manifestâ podignuta je na 0.0.20

## 0.0.5

- verzija svih manifestâ usklađena na 0.0.5 i centralno validirana
- Radio Balkan branding ostaje zaključan build guardom
- zajednička mrežna/SSRF validacija ostaje u jednom shared modulu
- nema `unlimitedStorage`, telemetry koda ni rebranding postavki
- zadržani manji katalog, paged rendering i lazy logo loading radi niže memorijske potrošnje
- GitHub-only CI gradi Chrome, Edge, Opera i Firefox iz istog shared sourcea
