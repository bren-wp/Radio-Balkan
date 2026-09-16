# Izdavanja

## 0.0.13

Izdanje 0.0.13 završava hardening release/version automatizacije bez nepotrebnih promjena runtime ponašanja Windows, Android ili browser klijenata.

- `scripts/bump_version.py` sada zahtijeva strogo rastući numerički SemVer i odbija ponavljanje iste verzije ili downgrade prije bilo kakvog writea
- Android `versionCode` i dalje mora strogo rasti; zadana vrijednost automatski je prethodna + 1
- version bump radi preflight svih target datoteka i markera u memoriji prije izmjene sourcea, pa nedostajući ili duplicirani marker ne može ostaviti polovično ažuriran repository
- writeovi se izvode atomskim privremenim datotekama i `os.replace`, bez djelomično zapisanih verzijskih datoteka
- ako završni `scripts/check_versions.py` validator ne prođe, transactional rollback vraća sve prethodne sadržaje i uklanja `.version-tmp` ostatke
- dodan je `--dry-run` koji nad stvarnim repozitorijem provjerava cijeli budući bump bez pisanja datoteka
- README i `docs/BUILD.md` automatski dobivaju sljedeći patch primjer, pa dokumentacija više ne ostaje na upravo izdanoj verziji
- `scripts/check_versions.py` sada provjerava točan GitHub Release link, release heading, jedinstveni next-patch primjer u README/BUILD dokumentaciji, Android metadata/User-Agent vezu, browser verziju i version badge sadržaj
- dodan je izvršni `scripts/test_version_tools.py` koji provjerava normalan bump, Android versionCode, downgrade rejection, `--dry-run` logiku, rollback nakon simuliranog validator failurea i uklanjanje privremenih datoteka
- CI uz fixture regression test izvršava i real-repository next-version `--dry-run`, tako da promjena stvarnog README/BUILD/metadata formata odmah otkriva neusklađen version tooling
- security-contract verifier zaključava transactional version-tooling, dry-run i pripadajuće CI korake
- checksum generator iz 0.0.12 ostaje determinističan, ne uključuje vlastiti manifest i i dalje se prije objave neovisno verificira s `sha256sum -c`
- Windows, Android i browser buildovi i dalje moraju završiti s čistim repository workspaceom prije promocije i objave
- nisu uvedene nove browser dozvole, telemetry komponente, runtime ovisnosti ni user-facing rebranding postavke

Izdanje je fokusirano na predvidljiv i obnovljiv release proces: verzijski bump mora ili završiti potpuno i proći validaciju ili vratiti repository u početno stanje.

## 0.0.12

Izdanje 0.0.12 proširuje reproducibilnost iz Windows builda na cijeli cross-platform CI/release lanac i dodaje eksplicitnu verifikaciju release checksum manifesta prije objave artefakata.

- dodan je zajednički `scripts/check_clean_worktree.py` koji preko `git status --porcelain --untracked-files=all` odbija build ako promijeni tracked source ili ostavi neignorirani privremeni sadržaj
- browser CI nakon pakiranja svih ekstenzija mora završiti s čistim repository workspaceom
- Android CI nakon `testReleaseUnitTest`, `lintRelease` i `assembleRelease` također mora završiti s čistim repository workspaceom
- postojeća Windows clean-worktree provjera prebačena je na isti zajednički checker, pa sva tri platform builda koriste identičan kriterij
- Publish workflow primjenjuje isti clean-worktree invariant na Windows, browser i Android artefakt buildove prije uploada workflow artefakata
- Product screenshots workflow provjerava čist workspace odmah nakon Windows builda i prije očekivanih izmjena screenshot PNG datoteka
- Publish workflow nakon generiranja `RadioBalkan-v<verzija>-SHA256.txt` odmah izvršava `sha256sum -c`, pa se release ne objavljuje ako checksum manifest ne odgovara preuzetim artefaktima
- security-contract verifier zahtijeva prisutnost clean-build provjera u CI, Publish i Product screenshots workflowima te checksum validation u Publish workflowu
- putanje workflow triggera uključuju zajednički clean-worktree checker, pa izmjena tog security/reproducibility sloja pokreće odgovarajuću validaciju
- nisu uvedene nove runtime ovisnosti, browser dozvole, telemetry komponente ni promjene playback ponašanja

Izdanje je namjerno fokusirano na integritet distribucije: isti source mora proći build bez nuspojava, a objavljeni artefakti dobivaju verificirani SHA-256 manifest prije GitHub Release uploada.

## 0.0.11

Izdanje 0.0.11 učvršćuje reproducibilnost Windows builda i uklanja nuspojave koje su tijekom CI/screenshot builda mijenjale working tree i ostavljale privremeni installer payload.

- Windows `build-release.ps1` više ne izvršava `gofmt -w` nad produkcijskim sourceom; formatiranje se provjerava read-only preko `gofmt -d`, pa build ne prepisuje `portable/main.go` ni `setup/main.go`
- eventualni format diff prijavljuje se kao build warning bez promjene source datoteke; `go vet`, `go test` i produkcijski `go build` ostaju obavezni
- Portable binary potreban za `//go:embed RadioBalkan-Portable.exe` i dalje se kopira u setup direktorij samo prije setup builda, ali se sada bezuvjetno uklanja u `finally` bloku
- čišćenje embedded payload datoteke izvršava se i kada `go vet`, `go test` ili setup build završe greškom
- Windows CI nakon release builda izvršava `git status --porcelain --untracked-files=all` i ruši job ako build ostavi bilo kakav tracked ili neignorirani untracked sadržaj
- novi clean-workspace guard prošao je na stvarnom GitHub Windows runneru zajedno s Portable/Setup test/vet/build koracima
- screenshot-only commitovi ostaju izuzeti iz punog multi-platform CI-ja, dok Product screenshots workflow i dalje samostalno gradi stvarni Windows binary i snima production UI
- browser epoch/revision/session zaštite, popup i Firefox izvršni race regression testovi iz 0.0.10 ostaju aktivni i obavezni u svakom CI prolazu
- nisu dodane nove runtime ovisnosti, browser dozvole, telemetry komponente ni rebranding postavke

Promjena je namjerno ograničena na build/release higijenu: runtime ponašanje Windows, Android i browser klijenata nije mijenjano bez potvrđenog funkcionalnog razloga.

## 0.0.10

Izdanje 0.0.10 dodatno učvršćuje browser state ordering i usklađuje Firefox STOP semantiku s Chromium playerom, uz izvršne regresijske testove koji reproduciraju race scenarije umjesto da provjeravaju samo statičke fragmente sourcea.

- popup više ne prihvaća promjenu background `epoch` vrijednosti iz proizvoljne zakašnjele `RB_STATE` poruke; novi epoch mora biti potvrđen autoritativnim `RB_GET_STATE` resyncom
- napušteni worker epoch ulazi u ograničeni `retiredEpochs` set i više ne može vratiti zastarjelu stanicu ili playback stanje čak ni s većom revision vrijednošću
- `RB_GET_STATE` resync je single-flight, pa više paralelnih sumnjivih state poruka ne pokreće nepotrebne istodobne upite backgroundu
- command token i revision zaštite testirane su i scenarijem dva brza `RB_TOGGLE` zahtjeva čiji se Promise odgovori vraćaju obrnutim redoslijedom; novija korisnička akcija ostaje autoritativna
- Firefox `RB_STOP` sada stvarno završava playback session: briše `currentSessionId`, candidate listu i indeks, pa naknadni `RB_TOGGLE` ne može oživjeti prethodno zaustavljenu reprodukciju
- Firefox zadržava zadnju stanicu u prikazanom UI stateu nakon STOP-a, ali bez aktivne playback sesije, što je usklađeno s očekivanim ponašanjem Chromium playera
- dodan je izvršni `scripts/test_browser_state_contract.js` koji pokreće stvarni produkcijski `popup.js` u kontroliranom Node VM-u i provjerava revision rollback, worker restart/epoch resync, retired epoch i reversed-command race
- dodan je izvršni `scripts/test_firefox_player_contract.js` koji pokreće stvarni Firefox background player s kontroliranim audio objektima i provjerava STOP cleanup te preklapajuće `PLAY` zahtjeve
- browser CI obavezno izvršava oba regression testa prije pakiranja ekstenzija, a security verifier zahtijeva njihovu prisutnost i ključne regresijske scenarije
- browser packaging guard dodatno zaključava epoch resync i Firefox STOP-session cleanup contract
- commitovi koji mijenjaju samo generirane `assets/screenshots/**` datoteke više ne pokreću puni Windows/Android/browser CI, čime se uklanja nepotrebno ponovno trošenje runnera nakon uspješnog Product screenshots workflowa
- nisu dodane nove browser dozvole, telemetry kod ni rebranding postavke; kanonski naziv i toolbar identitet ostaju zaključani na **Radio Balkan**

Windows i Android runtime kod u ovom izdanju nije mijenjan bez potvrđenog razloga; oba klijenta i dalje prolaze puni produkcijski test/build pipeline prije promocije release commita.

## 0.0.9

Izdanje 0.0.9 zatvara preostale race probleme između browser playera, background sloja i popup UI-ja te stabilizira automatizirano snimanje produkcijskog sučelja.

- Chromium offscreen player sada uz svaki state/result prenosi playback `sessionId` i monotoni `generation`, pa zakašnjeli rezultat stare sesije više ne može upravljati novom reprodukcijom
- Chromium service worker uvodi vlastiti `epoch`, `revision` i `commandGeneration`, prati aktivni session i ignorira state s drugog sessiona ili starijom generation vrijednošću
- Firefox background player koristi isti princip epoch/revision/session identiteta i instance ownershipa kroz cijeli playback lifecycle
- popup UI prati background `epoch` i monotoni `revision`, odbacuje zakašnjele `RB_PLAY`/`RB_TOGGLE` Promise rezultate i ne dopušta početnom `RB_GET_STATE` odgovoru da prepiše noviju korisničku akciju
- playback status odvojen je od kataloškog statusa: `Povezujem`, `Pauziram`, `Sada svira`, `Pauzirano` i `Nedostupno` više se ne brišu pri ponovnom renderiranju broja stanica
- browser packaging i cross-platform security verifier sada zahtijevaju session/revision/command ordering ugovore te dedicated `playerState` UI
- Windows screenshot capture više ne može neograničeno blokirati runner u native `PrintWindow` pozivu; capture se izvršava u zasebnom child procesu s tvrdim timeoutom od 30 sekundi i gašenjem cijelog process treeja
- browser screenshot koristi stvarni produkcijski popup HTML/CSS/JavaScript iz privremene kopije, uz CI-only lokalni runtime adapter koji se nikada ne uključuje u produkcijske ZIP-ove
- screenshot job ima kraći globalni timeout, determinističan lokalni katalog i eksplicitno čišćenje privremenog browser profila i preview direktorija
- CI dodatno provjerava sintaksu screenshot runtime JavaScripta i PowerShell capture skripte te dijeli concurrency ključ između push/PR provjera istog brancha kako ne bi nepotrebno trošio dvostruke runnere

Browser URL validator i dalje blokira izravne private/loopback/link-local/CGNAT/metadata i credentialed URL-ove. Media redirect ponašanje ostaje pod kontrolom samog preglednika bez uvođenja širih `webRequest` dozvola.

## 0.0.8

Izdanje 0.0.8 učvršćuje browser playback lifecycle za Chrome, Edge, Opera i Firefox bez dodavanja novih širokih browser dozvola.

- Chromium offscreen player i Firefox background player više ne recikliraju jedan globalni `HTMLAudioElement` kroz više playback sesija
- svaki kandidat streama dobiva vlastiti audio objekt vezan uz trenutačni generation token i konkretnu player instancu
- zakašnjeli `error`, `ended`, `playing` ili `pause` događaj stare stanice više ne može povećati indeks kandidata, promijeniti playing state ili pokrenuti fallback za noviju stanicu
- pri promjeni streama stari audio objekt odvaja event handlere, zaustavlja reprodukciju, uklanja `src` i više nije aktivna player instanca
- uklonjeni su statički `<audio id="audio">` elementi iz Chromium offscreen i Firefox background HTML-a jer više nisu potrebni
- browser packaging guard sada zahtijeva session-bound audio ownership, dinamičko kreiranje audio objekta i instance-aware `error`/`ended` handlere
- cross-platform security verifier odbija povratak na `document.getElementById('audio')` i statički globalni audio element
- zaključani Radio Balkan naziv, toolbar identitet i zabrana user-facing rebrand postavki ostaju nepromijenjeni

URL validator i dalje blokira izravne private/loopback/link-local/CGNAT/metadata i credentialed URL-ove. Redirect ponašanje samog media elementa ostaje pod kontrolom preglednika; izdanje 0.0.8 namjerno ne uvodi dodatne široke `webRequest`/browser permissione samo radi inspekcije media redirecta.

## 0.0.7

Izdanje 0.0.7 dodatno učvršćuje Android mrežni i playback sloj te uvodi izvršne regresijske testove u svaki release build.

- Android stream resolver više ne dopušta automatsko HTTP preusmjeravanje; svaki redirect hop ručno se razrješava i prolazi `isSafeHttp` provjeru prije novog mrežnog zahtjeva
- redirect lanac ograničen je na četiri preusmjeravanja, a privatni, loopback, link-local, CGNAT, metadata i credentialed URL-ovi ostaju blokirani
- homepage discovery i playlist fallback sada primjenjuju isti safe-URL contract; repaired stream više ne može zaobići strožu provjeru preko običnog `isHttp`
- ispravljen je Android MediaPlayer stale/double-callback race zbog kojeg je zakašnjeli callback mogao povećati `currentCandidate` i preskočiti valjani fallback stream
- neuspjeli `resume()` sada oslobađa neispravni player i kontrolirano prelazi na sljedeći kandidat umjesto ponavljanja rada nad pokvarenim playerom
- foreground notification i interni state broadcast pozivi izolirani su od OEM/runtime iznimki, a greška pri traženju audio fokusa više se ne tretira kao odobren fokus
- ICY metadata request više automatski ne slijedi redirect
- dodani su Android JUnit testovi za private, link-local, metadata, CGNAT, IPv6 i credentialed URL ciljeve te normalne javne HTTP/HTTPS streamove
- Android CI i release pipeline sada obavezno izvršavaju `testReleaseUnitTest` prije `lintRelease` i `assembleRelease`
- security-contract verifier čuva ručno redirect praćenje, safe repair putanju, player ownership guard i prisutnost Android URL regresijskih testova
- Windows produkcijska verzija sada se tretira kao linker-injected vrijednost iz `build-release.ps1`; version bump više ne prepisuje velike Win32 source datoteke samo radi fallback verzijskog stringa

Zaštita URL-ova smanjuje rizik od lokalnih i metadata ciljeva, ali ne predstavlja opće jamstvo protiv svih DNS-rebinding ili mrežnih TOCTOU scenarija.

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
