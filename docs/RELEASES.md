# Izdavanja

## 0.0.25

Izdanje 0.0.25 nastavlja UI/UX i transport hardening nakon v0.0.24 te zatvara release-integrity problem pronađen nakon mergea PR-a #25. Nisu dodane nove dozvole, analytics, telemetry, korisnički račun niti novi backend.

- browser popup izlaže Previous / Stop / Play-Pause / Next kontrole i drži Prev/Next unutar aktivnog filtriranog skupa stanica
- promjena filtera odmah sinkronizira enabled/disabled stanje browser Prev/Next kontrola
- compact popup skriva samo dekorativni artwork kako naziv stanice i puni transport ne bi bili zgnječeni
- browser cold-state bez stvarne remote stanice čisti optimistični `current`, pa player više ne izgleda kao da je prva stanica već odabrana
- executable browser test provjerava konkretan Previous/Next station payload, Stop → fresh Play, cold-state cleanup i postojeće session/epoch/revision zaštite
- Windows Stop više nema hit-region kad nema aktivne/pausirane stanice ili je player terminalno zaustavljen
- Windows Prev/Next se onemogućuju kada vidljivi skup nema barem dvije stanice; availability pravila su izdvojena u testirane pure helper funkcije
- veliki Windows Play bez prethodnog odabira pokreće prvu filtriranu stanicu ili prvu stanicu iz kataloga umjesto no-op ponašanja
- Windows volume `−` na 0% i `+` na 100% sada su vizualno disabled i nemaju klikabilni hit-region
- Android transport ostaje disabled dok player state nije spreman, a neuspjeli odabir čisti zastarjeli player artwork
- automatski Publish workflow na `main` sada se pokreće samo promjenom `VERSION`; obične promjene u aplikacijama, ekstenzijama ili workflow helperima više ne pokušavaju ponovno objaviti već postojeći tag
- postojeća tag/SHA zaštita u Publish koraku i dalje odbija pokušaj objave istog taga s drugog commita
- CI prije bumpa prošao je versions, browser, Android i Windows uključujući stvarni Windows runtime soak
- verzija je sinkronizirana na `0.0.25`, uz Android `versionCode 25`

Ovo izdanje dodatno smanjuje no-op, stale-state i release-pipeline rizike. Dostupnost stvarnih radio streamova i dalje ovisi o third-party infrastrukturi, pa se ne tvrdi da svaka stanica mora uvijek biti dostupna niti da je moguće jamčiti apsolutno crash-free ponašanje na svakoj kombinaciji OS-a, uređaja, drivera i mreže.

## 0.0.24

Izdanje 0.0.24 fokusirano je na konkretan UI/UX polish i playback recovery probleme pronađene nakon v0.0.23, bez novih dozvola, analyticsa, telemetryja, korisničkog računa ili novog backenda.

- Windows donji player izlaže stvarni Stop gumb umjesto prethodno nedostupnog `hitPlayerStop` handlera
- Windows `Prikaži sve →` uz Žanrove dobiva stvarni hit target i otvara postojeći genre selector
- Android stalni player dobiva Prev / Play-Pause / Stop / Next, uz testiranu wrap-around adjacent navigaciju
- Android `Radio` donja navigacija više nije no-op, a reprodukcija iz Favorita ne prebacuje lažno selected stanje na Radio tab
- browser popup dobiva zaseban Stop control s disabled, busy i accessibility stanjem
- browser UI razlikuje `Pauzirano` od terminalnog `Zaustavljeno`; glavni Play nakon Stop-a šalje novi `RB_PLAY`
- Chromium worker izlaže aktualni `sessionId`, pa popup može pouzdano razlikovati aktivnu pauziranu sesiju od zaustavljene
- Chromium i Firefox nakon iscrpljenja lokalnih stream kandidata jednom po sesiji pokušavaju obnoviti URL po station UUID-u
- UUID recovery koristi samo fiksne Radio Browser API hostove, response limit od 512 KiB, 4 s per-request timeout i ukupni 9 s budžet
- regionalni refresh mora zadržati istu državu; `Strano / INT` refresh ne smije mapirati balkansku postaju u INT i poštuje izvorni country code kada postoji
- STOP ili nova session/generation vrijednost poništavaju zakašnjeli catalog refresh; regression test potvrđuje da stari refresh ne može ponovno pokrenuti audio
- uspješno obnovljeni stream ostaje aktivni URL trenutne browser player sesije
- postojeći Windows runtime soak, Go fail-fast gateovi, Android unit/lint/release build, browser stall/race testovi, security contracts, clean-worktree i checksum provjere ostaju aktivni
- verzija je sinkronizirana na `0.0.24`, uz Android `versionCode 24`

Ovi popravci povećavaju dostupnost i predvidljivost playera, ali Radio Balkan i dalje ne tvrdi da svaki third-party radio stream mora uvijek biti online ili da je moguće jamčiti apsolutno crash-free ponašanje na svakoj kombinaciji OS-a, uređaja, drivera, mreže i vanjske infrastrukture.

## 0.0.23

Izdanje 0.0.23 završava player-lifecycle i Win32 thread-affinity hardening nakon 0.0.22. Nisu dodane nove dozvole, analytics, telemetry, korisnički račun ni novi backend.

- Windows CI nad lokalnim validnim WAV zapisom stvarno izvršava `PLAY → PAUSE → VOLUME → RESUME → STOP → PLAY → STOP`
- Windows audio helper se nakon command timeouta ili stdin write greške potpuno odbacuje prije fallbacka, pa zakašnjeli ACK ne može kontaminirati sljedeću naredbu
- `PLAY` ima 5-sekundni ACK budžet, a kontrolne naredbe 2 sekunde; čekanja ostaju ograničena
- Windows Portable i Setup zaključavaju Win32 UI/message loop na isti OS thread
- Chromium i Firefox testovi pokrivaju Pause/Resume, Stop koji umirovljuje sesiju, odbijanje Toggle-a nakon Stop-a i svježi Play nakon Stop-a
- Android lifecycle testovi zaključavaju da eksplicitni Stop vodi na novi `PLAY`, ne na `RESUME` stare servisne sesije
- postojeći Windows runtime soak, Android unit/lint/release build, browser stall/race testovi, security contracts, clean-worktree i checksum gateovi ostaju aktivni
- verzija je sinkronizirana na `0.0.23`, uz Android `versionCode 23`

Ovo izdanje zatvara potvrđene player state/ACK i Win32 message-loop rizike pronađene izvršnim testovima. Ne tvrdi da svaki third-party radio stream, OEM Android implementacija ili browser/Windows konfiguracija može biti apsolutno bez greške.

## 0.0.22

Izdanje 0.0.22 fokusirano je na potvrđene startup, shutdown i playback rizike pronađene dubinskim runtime auditom. Nisu dodane nove dozvole, analytics, telemetry, korisnički račun ni novi backend.

- Windows CI sada pokreće stvarni Portable executable i zahtijeva da glavni prozor/proces ostanu živi kroz 15-sekundni startup soak, nakon čega se aplikacija mora uredno zatvoriti
- Windows automatski startup health posao odgođen je 8 sekundi i početno radi nad manjim prioritetnim skupom stanica, umjesto da konkurira prvom renderu i katalogu
- PresentationCore audio subprocess dobiva background warmup, `READY` startup handshake i `OK/ERR` potvrdu svake naredbe; startup i command čekanja ostaju ograničena, a MCI fallback ostaje dostupan
- Windows Stop → Play radi svježi reconnect umjesto resumea zatvorenog sourcea, a promjena glasnoće primjenjuje se i dok je playback pauziran
- Windows shutdown prvo signalizira background poslovima prekid, a završni state/audio cleanup ima ograničen vremenski budžet kako UI thread ne bi ostao beskonačno blokiran
- otkriven je i uklonjen false-green Windows build rizik: `build-release.ps1` sada eksplicitno ruši pipeline ako bilo koji Portable/Setup `go vet`, `go test` ili `go build` završi non-zero
- Android player faila zatvoreno ako foreground notification/promocija nije moguća; MediaSession, audio-focus request i notification-channel inicijalizacija izolirane su od platformskih/OEM iznimki
- Android automatski startup health scan odgođen je na 8 sekundi i ograničen na 16 prioritetnih stanica; postojeći `MediaPlayer.prepareAsync()` watchdog i source fallback ostaju aktivni
- Chromium i Firefox playeri ograničavaju početni `audio.play()` pokušaj na 12 sekundi i imaju 15-sekundni `waiting/stalled` recovery koji prelazi na sljedeći kandidat samo ako je session/generation još aktualan
- novi izvršni browser regression testovi pokrivaju hanging play promise i stalled stream, uz postojeće stale-session, close/play race i popup state ugovore
- CI-only Windows shutdown trace aktivira se samo eksplicitnom runtime-test environment varijablom i koristi postojeći lokalni dijagnostički log; ne uvodi mrežno slanje podataka
- verzija je sinkronizirana na `0.0.22`, uz Android `versionCode 22`

Izdanje uklanja više potvrđenih crash/playback i release-pipeline rizika, ali ne tvrdi apsolutno crash-free ponašanje na svakoj kombinaciji Windows/Android/OEM/browser implementacije, mreže i third-party radio streama.

## 0.0.21

Izdanje 0.0.21 nastavlja production polish nakon 0.0.20 i fokusira se na oporavak korisničkog sučelja, predvidljiv Android search/back lifecycle i pouzdanije Windows Setup write putanje. Nisu dodane nove dozvole, analytics, telemetry, korisnički račun ni backend.

- browser popup više ne završava u pasivnom empty-stateu: kombinacija filtera bez rezultata nudi izravni `Poništi filtre` CTA
- početni browser catalog/network failure nudi `Pokušaj ponovno`, koji pokreće kontrolirani forced refresh bez potrebe za zatvaranjem ili reloadanjem ekstenzije
- novi browser recovery gumbi koriste iste focus, pressed, disabled i premium vizualne konvencije kao ostatak popup sučelja
- Android search više se ne može sakriti dok nevidljivi query nastavlja filtrirati listu; zatvaranje searcha čisti query, uklanja pending debounce callback, skriva tipkovnicu i odmah osvježava prikaz
- Android Back dok je search otvoren prvo zatvara search stanje umjesto da odmah izlazi iz Activityja
- Windows Setup app executable, instalirani uninstaller i ikona zapisuju se preko durable writera koji radi puni write i `Sync()` prije rename/commit koraka
- installer i dalje provjerava SHA-256 instaliranog executabla nakon zamjene, pa durable write nadopunjuje postojeću integrity provjeru umjesto da je zamjenjuje
- dodan je Windows regression test za durable installer payload writer
- production UI verifier zaključava actionable browser recovery stateove i Android search/back ponašanje, a security contract verifier zaključava Setup durable-write putanju
- premium UX iz 0.0.20 ostaje aktivan: keyboard/focus/pressed/busy stanja, single-flight playback i refresh zaštite, Android health/popularity prikaz te Windows pause/resume kontrole
- README, Windows/Android/browser platform README-i, arhitektura, sigurnost, privatnost, build vodič i release dokumentacija usklađeni su s produkcijskim stanjem
- verzija je sinkronizirana na `0.0.21`, uz Android `versionCode 21`

Ovo izdanje smanjuje potvrđene UI i persistence rizike pronađene auditom. Ne tvrdi apsolutno crash-free ponašanje na svakoj kombinaciji OS-a, filesystema, OEM Android implementacije, preglednika, mreže i third-party radio streama.

## 0.0.20

Izdanje 0.0.20 fokusirano je na premium, jasniji i dostupniji UI/UX na svim klijentima, uz dodatno lifecycle i release-metadata učvršćivanje. Nisu dodane nove dozvole, analytics, telemetry, korisnički račun ni novi backend.

- browser popup lokalno pamti državu, žanr i Omiljene, dodaje jednim klikom `Očisti` filtre te station kartice podržavaju Enter/Space, visible focus state i `aria-current`
- browser pressed, busy i reduced-motion stanja vizualno su dosljednija, a ručni refresh više ne može cache prikazati kao uspješno novo mrežno osvježavanje
- browser regression testovi pokrivaju sanitized/saved UI preference i forced-refresh failure semantiku uz postojeće response-limit, API fan-out i player race testove
- popup player ima UI single-flight guard: isti ponovljeni play/pause klik ne šalje dodatnu runtime komandu, a aktivna kontrola dobiva disabled + `aria-busy` stanje
- Android UI dobiva ripple/pressed feedback, jasniji `Filtriraj` CTA, trenutno vidljiv selected chip state i `Poništi filtre`
- Android donja navigacija više ne prikazuje nepostojeći `Profil`; `Više` vodi na stvarne aplikacijske opcije, a modalne akcije ne ostavljaju lažno označen aktivni ekran
- Radio Browser `votes` na Androidu više se ne prikazuju uz people/listener semantiku, nego točno kao `Popularnost · N glasova`
- Android `RadioRepository.load()` sigurno podnosi executor/shutdown race i ne propušta `RejectedExecutionException` prema UI threadu
- spremljeni tab ponovno usklađuje bottom-navigation selected state, hero/player CTA prati stvarno Slušaj/Nastavi/Pauziraj stanje, a station red uz popularnost prikazuje health status
- ručni catalog refresh je single-flight; neuspjeli start player servisa više ne zapisuje lažni `currentKey`, replacement-source reset ostaje konzistentan, a vanjski homepage link mora proći `isSafeHttp` provjeru
- Windows Setup dobiva keyboard kontrolu preko Tab/strelica/Enter/Space/Escape, proširene checkbox label hit-targete i vidljivi fokus
- Windows hero i station-card play kontrole aktivne stanice rade kao pause/resume toggle i prikazuju `Pauziraj` / `Nastavi` umjesto ponovnog pokretanja playback pipelinea
- Windows Portable i Setup source fallback `appVersion` vrijednosti uvedene su u centralni transactional version-bump/check contract; release linker injection ostaje dodatni autoritativni sloj
- Windows build formatting guard sada normalizira line ending samo u privremenoj kopiji, odbija stvarni `gofmt` drift i više ne prijavljuje cijelu datoteku kao promijenjenu samo zbog Windows CRLF checkouta
- `WM_GETMINMAXINFO` više ne pretvara callback `lParam` izravno u `unsafe.Pointer`; struktura se kopira kroz postojeći `RtlMoveMemory` wrapper, čime je uklonjena vet opomena uz isto minimal-window ponašanje
- dodani su Windows Setup regression testovi za focus wrap i click-target geometriju
- production UI, security i version contract verifieri prošireni su na nove UX/lifecycle/version invariants
- README, platform README/changelogovi, arhitektura, sigurnost, privatnost, build i contribution dokumentacija usklađeni su s produkcijskim stanjem
- verzija je sinkronizirana na `0.0.20`, uz Android `versionCode 20`

Ovo izdanje smanjuje poznate UI/lifecycle/race rizike pronađene auditom, ali ne tvrdi apsolutno crash-free ponašanje na svakoj kombinaciji OS-a, drivera, OEM Android implementacije, preglednika, mreže i third-party radio streama.

## 0.0.19

Izdanje 0.0.19 fokusirano je na kontrolu memorije i mrežnog fan-outa u browser katalogu, pouzdanije spremanje Windows stanja te stroži Android lifecycle cleanup, bez novih dozvola, telemetry komponenti ili runtime dependencyja.

- browser više ne koristi neograničeni `response.json()` za Radio Browser API odgovore; JSON se dekodira iz streaming bodyja uz provjeru stvarno pročitanih bajtova prije pune alokacije odgovora
- discovery popis Radio Browser servera ograničen je na 512 KiB, a pojedini station-catalog odgovor na 8 MiB
- browser namjerno nema fallback koji bi prvo učitao cijeli response preko `response.text()` pa tek potom provjerio veličinu; ako streaming body nije dostupan, odgovor se odbija
- dinamički Radio Browser server discovery ograničen je na najviše četiri novootkrivena hosta uz četiri stabilna fallback hosta, odnosno najviše osam API baza po osvježavanju
- izvršni browser regression test simulira preveliki country response i velik dinamički server-list te potvrđuje da ostali country batchovi nastavljaju raditi, a API fan-out ostaje ograničen
- Windows `state.json.tmp` sada se zapisuje preko eksplicitnog durable writera koji radi puni write i `Sync()` prije backup/rename commit koraka
- Windows završni shutdown state/audio cleanup konsolidiran je u `sync.Once` sekvencu, tako da `WM_CLOSE` i `WM_DESTROY` više ne izvršavaju isti finalni persistence/audio posao dvaput
- debounce state-save callback provjerava shutdown stanje neposredno prije zapisa i ne pokreće zakašnjeli disk write nakon početka finalnog zatvaranja
- Windows regression test provjerava potpuni durable write te zamjenu postojećeg sadržaja
- Android `MainActivity.onDestroy()` uklanja sve pending Handler callbackove i poruke, uključujući odgođeni health scan i worker-to-UI callbackove, čime se smanjuje post-destroy rad i kratkotrajno zadržavanje Activity reference
- cross-platform security/lifecycle contracts zaključavaju response limite, zabranu neograničenih browser JSON fallbackova, API discovery cap, durable Windows writer, shutdown sekvencu i Android Handler cleanup
- regionalni katalog, kurirani `Strano / INT` katalog, browser playback session zaštite i mrežni hardening iz 0.0.18 ostaju nepromijenjeni
- verzija je sinkronizirana na `0.0.19`, uz Android `versionCode 19`; puni Windows/Android/browser/version CI obavezan je prije promocije na `main`

Ograničenja odgovora i broja API hostova štite klijenta od nepotrebnog RAM/CPU/network opterećenja uzrokovanog neispravnim ili neočekivano velikim katalogom, ali ne mogu jamčiti dostupnost third-party Radio Browser servisa ili pojedinih radio streamova. Durable file sync smanjuje rizik gubitka Windows postavki pri prekidu rada, ali aplikacija ne tvrdi apsolutnu otpornost na svaki hardverski ili filesystem kvar.

## 0.0.18

Izdanje 0.0.18 fokusirano je na mrežnu sigurnost i dosljedno blokiranje lokalnih, privatnih i metadata odredišta na Windows, Android i browser klijentima, uz regresijske testove koji zaključavaju novo ponašanje.

- browser URL validator ispravlja IPv6 provjeru za cijeli unique-local raspon `fc00::/7`, cijeli link-local raspon `fe80::/10` i multicast `ff00::/8`; postojeća zaštita od IPv4 loopback/private/link-local/CGNAT i IPv4-mapped IPv6 ciljeva ostaje aktivna
- browser network regression testovi sada eksplicitno pokrivaju `fc00::`, `fd00::`, `fe90::` i `ff02::` scenarije
- Windows URL validacija kanonizira hostname prije sigurnosne odluke, pa trailing-dot oblici poput `localhost.`, `radio.local.` i metadata hostova više ne mogu zaobići lokalne host provjere
- Windows HTTP transport koristi vlastiti `DialContext`: hostname se razrješava prije uspostave veze, cijeli DNS rezultat se odbija ako sadrži privatnu/lokalnu adresu, a TCP veza se uspostavlja na već provjerenu javnu IP adresu
- Windows testovi dodatno pokrivaju privatne, link-local, multicast, CGNAT, IPv4-mapped IPv6 i trailing-dot host scenarije
- Android prije Radio Browser API zahtjeva, stream probea, MediaPlayer pripreme, ICY metadata zahtjeva, homepage discoveryja i učitavanja logotipa radi DNS preflight i odbija rezultat koji sadrži privatnu ili lokalnu adresu
- Android JUnit regresija provjerava miješani public+private DNS rezultat, link-local metadata adresu i normalan javni skup adresa
- `verify_security_contracts.py` zaključava cross-platform mrežne zaštite kako se navedeni guardovi i testovi ne bi mogli nenamjerno ukloniti
- nisu dodane nove Android dozvole, browser broad permissions, analytics, telemetry, tracking SDK-ovi ni runtime dependencyji
- regionalni katalog i kurirani `Strano / INT` katalog ostaju nepromijenjeni po opsegu i prioritetu; ovo izdanje ne degradira postojeće playback/session/lifecycle zaštite iz 0.0.17
- verzija je sinkronizirana jednim release commitom na `0.0.18`, uz Android `versionCode 18`, a puni Windows/Android/browser/version CI ostaje obavezan prije mergea

Windows transport pinning zatvara DNS-to-private promjenu između provjere i TCP diala za zahtjeve koji koriste zajednički Go HTTP transport. Android DNS preflight značajno smanjuje isti SSRF rizik, ali Android `MediaPlayer` i platformni `HttpURLConnection` ne nude ovom kodu isti stupanj socket-level IP pinninga; stoga se ne tvrdi apsolutna zaštita od svih DNS-rebinding/TOCTOU scenarija. Dostupnost stvarnih radio streamova i dalje ovisi o third-party infrastrukturi.


## 0.0.14

Izdanje 0.0.14 fokusirano je na produkcijski UI/UX, robusnije korisničke opcije i smanjenje nepotrebnog rada u sva tri klijenta, bez novih telemetry komponenti ili širih browser dozvola.

- browser popup više ne gradi ponovno cijeli prikaz stanica pri svakoj playback-state poruci; aktivna stanica i play/pause kontrola osvježavaju se ciljano, čime se smanjuje DOM/CPU churn
- odabrani country filter u browser ekstenziji ostaje sačuvan nakon osvježavanja kataloga, a neuspjeli mrežni refresh zadržava zadnji valjani prikaz umjesto praznog popisa
- browser player/favorite/refresh kontrole dobile su jasna disabled, `aria-pressed`, `aria-busy` i live-status stanja, veće touch targete i čitljiviju tipografiju
- izvršni browser regression test potvrđuje da playback-only state ne rerendera listu te da country filter preživljava refresh
- Android više ne traži notification permission odmah pri hladnom pokretanju aplikacije; dozvola se traži kontekstualno tek kada korisnik pokrene reprodukciju
- Android red stanice sada ima vidljiv gumb za dodatne opcije umjesto oslanjanja samo na skriveni long-press; artwork je kompaktniji, a kontrole imaju jasnije accessibility opise
- Android korisnički izbornici preimenovani su u jasne pojmove poput `Rezervni izvori`, `Filtriraj stanice`, `Kopiraj poveznicu za reprodukciju`, `Odaberi drugi izvor` i `Vrati automatski odabir`
- ručni Android izvor mora proći strožu `isSafeHttp` provjeru prije mrežnog testa; worker greške tijekom provjere stanice ili ručnog izvora vraćaju UI u upotrebljivo stanje i prikazuju korisničku poruku umjesto ostavljanja disabled kontrola
- Windows sada sprječava smanjivanje prozora ispod 1100×720, iste donje granice koju koristi spremljeno window-state stanje, pa header/search/filter kontrole više ne mogu ući u neupotrebljivo preklapanje
- Windows sidebar `MOJE LISTE` zamijenjen je točnim `BRZI ODABIR`, a lažni nazivi `Jutarnji vibe`/`Chill večer` zamijenjeni su stvarnim `Popularne`/`Jazz` filtrima
- Windows ručna promjena izvora više ne traži skriveni magic-string `AUTO`; prazno polje vraća automatski odabir, a copy/source statusi koriste jasniji korisnički jezik
- postojeći Windows HTTP redirect guard ostaje aktivan: svaki redirect prolazi `safeHTTPURL` provjeru prije praćenja i lanac je ograničen
- dodan je `scripts/verify_production_ui.py` koji u CI-ju zaključava user-facing tekst, accessibility, contextual permission, vidljive opcije, Windows minimum-size i zabranu povratka dev/test placeholdera
- Windows, Android i browser klijenti ponovno prolaze puni production build, security contracts, clean-worktree i release-integrity provjere prije objave

Izdanje smanjuje rizik od UI blokada i regresija, ali ne tvrdi da je moguće apsolutno jamčiti da se aplikacija nikada neće srušiti na svakoj kombinaciji uređaja, drivera, mreže i radio streama.

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
