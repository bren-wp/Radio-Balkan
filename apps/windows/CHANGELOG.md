# Changelog — Windows


## 0.0.20

- Windows Setup dobio keyboard navigaciju preko Tab/strelica, Enter/Space aktivaciju i Escape zatvaranje iz idle stanja
- installer checkbox opcije imaju proširene click-targete preko cijele labele i vidljivo fokus stanje
- dodani regression testovi za wrapanje keyboard fokusa i checkbox hit-targete
- Portable i Setup source `appVersion` fallback vrijednosti sada su dio centralnog transactional version bumpa i version-check contracta
- postojeći durable state write, idempotent shutdown cleanup, mrežni DNS/IP guardovi i installer SHA-256 provjera ostaju aktivni
- Portable i Setup ponovno prolaze Go test, vet, release build i clean-worktree provjeru

## 0.0.5

- uklonjeni nepotrebni ugrađeni JPG dizajnerski asseti i raster cache
- dodani proceduralni hero, genre i fallback artwork prikazi
- smanjen soft memory limit, logo cache i maksimalni katalog
- smanjen broj health-check workera i automatskih provjera
- smanjeni JSON/network response limiti
- očišćeni korisnički UX tekstovi od razvojnih oznaka
- centralizirana verzija i GitHub-only CI/release provjere
- Portable i Setup ostaju reproducibilno izgrađeni iz istog sourcea
