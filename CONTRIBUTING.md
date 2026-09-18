# Doprinos projektu

1. Otvori zasebnu granu iz aktualnog `main`.
2. Ne mijenjaj kanonski naziv **Radio Balkan** niti dodaj rebranding postavke browser ekstenzijama.
3. Ne commitaj signing ključeve, tokene, `.env`, build cache ili generirane release binarije.
4. Za promjenu verzije koristi `python scripts/bump_version.py <nova-verzija>`; ne uređuj pojedinačne version markere ručno.
5. Prije pull requesta pokreni najmanje:

```bash
python scripts/check_versions.py
python scripts/verify_security_contracts.py
python scripts/verify_production_ui.py
python scripts/test_release_checksums.py
python scripts/test_version_tools.py
```

6. Pokreni test/build za svaku promijenjenu platformu. Browser promjene moraju proći izvršne popup/player regression testove; Windows promjene Go test/vet/build; Android promjene unit testove, lint i release build.
7. Build ne smije ostaviti tracked ili neignorirani privremeni sadržaj; provjeri `python scripts/check_clean_worktree.py`.
8. Pull request mora navesti utjecaj na stabilnost, RAM/CPU, lifecycle, dozvole, privatnost, accessibility/UI/UX i kompatibilnost.
9. Release kandidat se ne mergea dok puni Windows/Android/browser/version CI na točnom head SHA nije zelen.
