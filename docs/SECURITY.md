# Sigurnost

Radio Balkan tretira URL-ove streamova i logotipa kao nepouzdani mrežni ulaz.

- Browser ekstenzije centralno blokiraju privatne, loopback, link-local, CGNAT i druge lokalne ciljeve.
- Android validira stream kandidate i ograničava veličinu mrežnih odgovora i slika.
- Android player-state broadcast ostaje interni kanal: na novijim Android verzijama receiver je `RECEIVER_NOT_EXPORTED`, a na podržanim starijim verzijama kanal je zaštićen aplikacijskom `signature` permission dozvolom.
- Pre-Android 13 receiver registracija koristi isti `signature` permission. Usko scoped `UnspecifiedRegisterReceiverFlag` lint suppression postoji samo zato što Android Lint ne inferira zaštitu custom signature permissiona kroz stariji overload; nije zamjena za sigurnosnu kontrolu.
- Windows ograničava katalog, cache i paralelne health provjere te koristi timeoute i recovery putanje.
- Nema telemetry SDK-a, oglasnih SDK-ova ni skrivenog praćenja.
- Tajne za potpisivanje Android izdanja ne smiju biti commitane; koriste se isključivo GitHub Actions Secrets.

Sigurnosni problem prijavi privatnim kanalom vlasniku repozitorija. Nemoj objavljivati aktivne exploite ili privatne ključeve u javnom issueu.
