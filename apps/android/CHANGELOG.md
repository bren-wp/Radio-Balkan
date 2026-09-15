# Changelog — Android

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
