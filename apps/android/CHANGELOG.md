# Changelog — Android


## 0.0.20

- UI kontrole dobile su dosljedniji ripple/pressed feedback bez uvođenja novog UI frameworka
- donji `Profil` koji nije predstavljao stvarni korisnički profil zamijenjen je opcijom `Više`
- modalne akcije više ne ostavljaju pogrešno označenu aktivnu bottom-navigation stavku
- browse/filter UI koristi jasni `Filtriraj` CTA, live selected chip state i `Poništi filtre`
- Radio Browser votes više se ne predstavljaju kao live listener count; prikazuju se kao `Popularnost · N glasova`
- repository load hvata executor/shutdown race i ne propušta `RejectedExecutionException` prema UI threadu
- spremljeni tab i bottom-navigation selected state usklađeni su nakon cold starta
- hero/player kontrole prikazuju stvarni Slušaj/Nastavi/Pauziraj state i dinamičan accessibility opis
- ručni refresh je single-flight; neuspjeli foreground-service start ne ostavlja lažni current station state
- station red uz popularnost prikazuje health status, a vanjski homepage link mora proći `isSafeHttp` provjeru
- povratak automatskog izvora više ne može ostaviti spremljeni ručni replacement u konfliktu s prikazanim aktivnim URL-om
- postojeći Handler, repository, image loader i player cleanup te URL/DNS/redirect hardening ostaju aktivni
- versionCode podignut na 20 i versionName na 0.0.20

## 0.0.7

- versionCode podignut na 7 i versionName na 0.0.7
- svaki HTTP redirect u stream resolveru ručno se validira prije praćenja; automatski redirecti su isključeni
- repaired stream URL mora proći `isSafeHttp` prije spremanja i reprodukcije
- ispravljen stale/dvostruki MediaPlayer callback koji je mogao preskočiti sljedeći fallback stream
- neuspjeli resume sada oslobađa pokvareni player i kontrolirano prelazi na sljedeći kandidat
- foreground notification, state broadcast i audio-focus error putanje dodatno su zaštićene od runtime/OEM iznimki
- ICY metadata zahtjevi više automatski ne slijede redirect
- dodani JUnit testovi za private, metadata, CGNAT, IPv6 i credentialed URL ciljeve
- `testReleaseUnitTest` sada je obvezan dio lokalnog release builda, CI-ja i GitHub Release pipelinea

## 0.0.5

- versionCode podignut na 5 i versionName na 0.0.5
- GitHub-only CI/release dokumentacija i centralna provjera verzije
- zadržan 15-sekundni prepare watchdog za problematične streamove
- zajednički ograničeni I/O pool za source i health poslove
- automatski health scan ograničen na 32 prioritetne stanice / jedan worker
- smanjeni bitmap, katalog i mrežni memorijski limiti
- uklonjeni nepotrebni Wi-Fi lock, duplicirani country mapping i mrtvi artwork helper
- release build ostaje minificiran i resource-shrinkan
