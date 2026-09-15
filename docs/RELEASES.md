# Izdavanja

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
