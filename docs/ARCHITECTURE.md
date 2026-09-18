# Arhitektura

Radio Balkan je monorepo s tri runtime cilja i jednim GitHub-based release procesom.

## Windows

`apps/windows/portable` je nativna Go/Win32 aplikacija. Ne koristi Electron ni ugrađeni browser. Reprodukcija, katalog i UI rade unutar jednog laganog procesa s ograničenim cachevima i workerima. `apps/windows/setup` koristi isti Portable payload i vlastiti user-level installer. Setup ima keyboard navigaciju, vidljiv fokus i proširene hit-targete, a od 0.0.21 app payload, uninstaller i ikona prolaze durable write + sync prije rename/commit koraka.

Od 0.0.5 Windows UI ne ugrađuje JPG hero/genre fallbackove. Hero, genre kartice i fallback artwork crtaju se proceduralno, dok se mrežni logo stanice dekodira samo kada je potreban i ulazi u mali ograničeni cache. `apps/windows/setup` ugrađuje upravo isti Portable payload.

## Android

`apps/android` je nativni Java/Android klijent. Reprodukcija je izdvojena u foreground servis, a activity se može ponovno kreirati bez gubitka osnovnog playback stanja. Bitmap cache, mrežni responsei i background worker pool imaju definirane gornje granice. Activity, repository, image loader i player imaju eksplicitne shutdown/cleanup putanje; UI callback queue se prazni pri destroyu, repository odbija rad nakon gašenja bez propuštanja executor iznimke prema UI threadu, a ručni catalog refresh koristi single-flight guard. Search overlay ima eksplicitnu open/close putanju koja pri zatvaranju uklanja skriveni query i tipkovnicu prije povratka na osnovni prikaz.

## Browser ekstenzije

`extensions/shared` je jedini izvor za popup UI, katalog i mrežnu sigurnosnu validaciju. `extensions/platform` sadrži samo nužne razlike playback sloja za Chromium i Firefox. Manifesti su odvojeni, ali ih isti build proces validira i pakira. Popup sprema samo lokalne UI preference (država, žanr i Omiljene), podržava keyboard station navigaciju, razlikuje stvarni mrežni refresh od cache fallbacka i koristi UI command single-flight guard. Empty/error stateovi su akcijski: filter rezultat se može odmah poništiti, a početni catalog failure ponovno pokušati bez reloadanja ekstenzije.

## Jedinstveni release tok

Root `VERSION` je canonical verzija. `scripts/check_versions.py` sprječava drift između platformi, uključujući Windows Portable/Setup source fallback verzije, i provjerava browser brand. GitHub Actions provodi CI, generira stvarne UI screenshotove i objavljuje artefakte u GitHub Releases. Nema GitLab CI-a ni drugog paralelnog distribucijskog izvora.
