# Privatnost

Radio Balkan je projektiran bez korisničkog računa, behavioral trackinga, oglasnih SDK-ova i telemetry sustava. Aplikacije kontaktiraju samo izvore potrebne za katalog, logotipe, provjeru dostupnosti i reprodukciju javnih radio streamova.

## Lokalno spremljeni podaci

- favoriti, nedavne stanice, odabrani izvori i postavke playera ostaju lokalno na uređaju
- browser ekstenzije lokalno spremaju katalog/cache te UI preference poput države, žanra i filtra Omiljenih
- Windows i Android mogu voditi ograničene lokalne dijagnostičke logove radi stabilnosti i otkrivanja grešaka
- projekt ne zahtijeva korisničke lozinke, račun, profil ni cloud sinkronizaciju

Lokalni cache i preference služe samo funkcionalnosti i performansama aplikacije. Ne postoji kod koji ih šalje Brendigu ili oglašivačima.

Search tekst u Android aplikaciji nije cloud-sinkroniziran niti se šalje kao korisnički profil; zatvaranje search prikaza čisti aktivni query. Browser recovery/empty-state kontrole iz 0.0.21 ne uvode nove spremljene podatke. Playback/startup hardening iz 0.0.22 također ne uvodi telemetry ni cloud profiliranje. Windows shutdown trace koji se koristi u regression testu aktivira se isključivo CI environment varijablom i zapisuje faze samo u postojeći lokalni dijagnostički log; ništa se ne šalje van uređaja.

Browser ekstenzije koriste samo dozvole potrebne za player, katalog i lokalne preference. Android signing ključevi i release secrets nisu dio aplikacijskih korisničkih podataka i ne spremaju se u repozitorij.
