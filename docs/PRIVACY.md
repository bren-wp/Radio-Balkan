# Privatnost

Radio Balkan je projektiran bez korisničkog računa, behavioral trackinga, oglasnih SDK-ova i telemetry sustava. Aplikacije kontaktiraju samo izvore potrebne za katalog, logotipe, provjeru dostupnosti i reprodukciju javnih radio streamova.

## Lokalno spremljeni podaci

- favoriti, nedavne stanice, odabrani izvori i postavke playera ostaju lokalno na uređaju
- browser ekstenzije lokalno spremaju katalog/cache te UI preference poput države, žanra i filtra Omiljenih
- Windows i Android mogu voditi ograničene lokalne dijagnostičke logove radi stabilnosti i otkrivanja grešaka
- projekt ne zahtijeva korisničke lozinke, račun, profil ni cloud sinkronizaciju

Lokalni cache i preference služe samo funkcionalnosti i performansama aplikacije. Ne postoji kod koji ih šalje Brendigu ili oglašivačima.

Browser ekstenzije koriste samo dozvole potrebne za player, katalog i lokalne preference. Android signing ključevi i release secrets nisu dio aplikacijskih korisničkih podataka i ne spremaju se u repozitorij.
