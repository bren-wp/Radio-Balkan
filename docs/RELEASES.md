# Izdavanja

## 0.0.6

Izdanje 0.0.6 fokusirano je na stabilnost playera, sigurnije mrežne ulaze i strože regresijske provjere prije objave.

- ispravljen je browser playback state koji je koristio nedeklarirane runtime varijable i mogao pasti s `ReferenceError` već pri prvom pokretanju reprodukcije
- Chromium i Firefox playback sada koriste generacijske tokene kako stari asinkroni `audio.play()` pokušaj ne bi mogao promijeniti stanje novije odabrane stanice ili preskočiti prvi fallback stream
- Chromium offscreen dokument kreira se single-flight mehanizmom, čime su uklonjeni paralelni `createDocument()` race pokušaji
- browser packaging sada provjerava strict mode, eksplicitno player stanje, generacijski guard i Chromium offscreen creation contract prije izrade ZIP-ova
- Windows `safeHTTPURL` sada odbija credentialed URL-ove s `user:password@host` oblikom uz postojeću zaštitu od loopback, private, link-local, CGNAT i metadata ciljeva
- dodani su Windows regresijski testovi za sigurne javne streamove, privatne/credentialed ciljeve i sanaciju spremljenih replacement/backup URL-ova
- dodan je cross-platform `verify_security_contracts.py` koji u CI-ju čuva Android internal-broadcast izolaciju, browser player-state zaštite i Windows sigurnosne testove
- dodan je kanonski `scripts/bump_version.py` za sinkronizaciju Windows, Android, browser, README i version badge metapodataka
- `check_versions.py` sada provjerava i README/version badge kako dokumentacija ne bi ostala na staroj verziji
- CI sada otkazuje zastarjele validation runove na istoj grani, čime se smanjuje nepotrebno zauzeće GitHub runnera pri brzim uzastopnim commitovima
- product screenshot workflow automatski se pokreće samo na `main` i samo za UI-relevantne promjene, umjesto na svaki branch/doc push
- Android release signing sada odbija djelomično konfigurirane secrets umjesto tihog fallbacka, a privremeni keystore briše se nakon builda
- GitHub release workflow provjerava postojeći version tag i odbija prepisivanje istog taga artefaktima s drugog commita

## 0.0.5

GitHub-first izdanje objedinjeno je u jedan monorepo i pripremljeno za produkcijsku distribuciju kroz GitHub Actions i GitHub Releases.

- sva tri klijenta i sve browser varijante premještene su u jednu kanonsku bazu
- dodan centralni `VERSION` i automatska provjera verzijskog drifta
- uklonjeni dupli app-specifični workflowi; CI je centraliziran u korijenu repozitorija
- dokumentacija, privatnost, sigurnost i build proces usklađeni su s produkcijskim stanjem
- browser branding ostaje zaključan na Radio Balkan
- Android release prolazi kroz Java 17, Android 36, release lint, R8 i `assembleRelease`
- interni Android player-state broadcast izoliran je `RECEIVER_NOT_EXPORTED`/signature-permission zaštitom ovisno o verziji sustava
- Windows Portable i Setup grade se jednom kanonskom build skriptom koja provodi testove i provjere prije pakiranja
- Chrome, Edge, Opera i Firefox ekstenzije grade se iz zajedničkog sourcea bez duplicirane mrežne logike
- release artefakti, checksumovi i APK distribuiraju se kroz GitHub Releases, odvojeno od sourcea i dokumentacije
