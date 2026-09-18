# Sigurnost

Radio Balkan tretira URL-ove streamova i logotipa kao nepouzdani mrežni ulaz.

- Browser ekstenzije centralno blokiraju privatne, loopback, link-local, CGNAT i druge lokalne ciljeve te održavaju eksplicitno player stanje u strict-mode runtimeu. Radio Browser JSON odgovori čitaju se streaming putem s byte-limitima, a dinamički API discovery ima ograničen fan-out.
- Chromium offscreen player koristi single-flight kreiranje dokumenta i generacijske tokene kako zastarjeli asinkroni pokušaji ne bi mijenjali novije stanje reprodukcije. Popup dodatno ne šalje isti play/pause zahtjev ponovno dok je prethodna komanda aktivna.
- Android validira stream kandidate i ograničava veličinu mrežnih odgovora i slika. Katalogom dobiven vanjski homepage link prolazi `isSafeHttp` provjeru prije predaje sustavnom browseru. DNS preflight odbija privatne/lokalne rezultate prije aplikacijskih mrežnih veza, a repository executor shutdown race ne propagira runtime iznimku prema UI threadu.
- Android player-state broadcast ostaje interni kanal: na novijim Android verzijama receiver je `RECEIVER_NOT_EXPORTED`, a na podržanim starijim verzijama kanal je zaštićen aplikacijskom `signature` permission dozvolom.
- Pre-Android 13 receiver registracija koristi isti `signature` permission. Usko scoped `UnspecifiedRegisterReceiverFlag` lint suppression postoji samo zato što Android Lint ne inferira zaštitu custom signature permissiona kroz stariji overload; nije zamjena za sigurnosnu kontrolu.
- Windows blokira loopback, private, link-local, CGNAT i poznate metadata ciljeve te odbija URL-ove s ugrađenim korisničkim podacima (`user:password@host`).
- Windows ograničava katalog, cache i paralelne health provjere te koristi timeoute i recovery putanje. Portable state i Setup app/uninstaller/icon payloadi koriste durable temp/write-sync putanje prije commit/rename koraka.
- Cross-platform sigurnosni, lifecycle i production UI/UX ugovori te izvršni regression testovi izvršavaju se u GitHub CI-ju prije produkcijskog mergea. U 0.0.22 CI dodatno zaključava Chromium/Firefox hanging-play i stall recovery, Android foreground-service fail-closed ponašanje, odgođene startup health batchove, Windows audio `READY`/ACK ugovor, fail-fast Go build korake, bounded shutdown cleanup i stvarni 15-sekundni Portable runtime soak.
- Nema telemetry SDK-a, oglasnih SDK-ova ni skrivenog praćenja.
- Tajne za potpisivanje Android izdanja ne smiju biti commitane; koriste se isključivo GitHub Actions Secrets.

Sigurnosni problem prijavi privatnim kanalom vlasniku repozitorija. Nemoj objavljivati aktivne exploite ili privatne ključeve u javnom issueu.
