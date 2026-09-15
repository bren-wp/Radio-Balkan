# Sigurnost

Radio Balkan tretira URL-ove streamova i logotipa kao nepouzdani mrežni ulaz.

- Browser ekstenzije centralno blokiraju privatne, loopback, link-local, CGNAT i druge lokalne ciljeve.
- Android validira stream kandidate i ograničava veličinu mrežnih odgovora i slika.
- Windows ograničava katalog, cache i paralelne health provjere te koristi timeoute i recovery putanje.
- Nema telemetry SDK-a, oglasnih SDK-ova ni skrivenog praćenja.
- Tajne za potpisivanje Android izdanja ne smiju biti commitane; koriste se isključivo GitHub Actions Secrets.

Sigurnosni problem prijavi privatnim kanalom vlasniku repozitorija. Nemoj objavljivati aktivne exploite ili privatne ključeve u javnom issueu.
