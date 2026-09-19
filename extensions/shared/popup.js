(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  const ext = RB.ext;
  const PAGE = 48;
  const GENRE_LABELS = new Map([
    ['domaca', 'Domaća / regionalna'], ['pop', 'Pop & Rock'], ['folk', 'Narodna / Folk'],
    ['electronic', 'Elektronička'], ['jazz', 'Jazz'], ['classical', 'Klasična'],
    ['news', 'Vijesti & Talk'], ['hits', 'Hits / Top 40'], ['oldies', 'Oldies']
  ]);
  let all = [];
  let visible = [];
  let current = null;
  let playing = false;
  let stopped = true;
  let favs = {};
  let favoritesOnly = false;
  let searchTimer = 0;
  let preferenceTimer = 0;
  let uiPreferencesReady = false;
  let renderLimit = PAGE;
  let playerStatus = 'Spremno';
  let commandGeneration = 0;
  let activeCommandToken = 0;
  let stateEpoch = '';
  let lastRevision = -1;
  let stateSyncPromise = null;
  let adminMode = false;
  let adminFailures = 0;
  let adminLockedUntil = 0;
  let adminDetailsGeneration = 0;
  let detailStation = null;
  let detailReturnFocus = null;
  let viewMode = 'all';
  const retiredEpochs = new Set();
  const ADMIN_ITERATIONS = 120000;
  const ADMIN_SALT = Uint8Array.from('c6d79acaafb52bb8bac278313e84ccf7'.match(/../g).map(x => Number.parseInt(x, 16)));
  const ADMIN_DIGEST = '6d319ade7c2c0f333d1d520eaf582034a80cabbc4e2f0317527b68576b631f80';

  const search = $('search');
  const country = $('country');
  const genre = $('genre');
  const list = $('stations');
  const status = $('status');
  const refresh = $('refresh');


  async function adminCredentialsValid(username, password) {
    if (String(username || '').trim().toLowerCase() !== 'brendigo' || typeof password !== 'string') return false;
    if (!globalThis.crypto?.subtle || typeof TextEncoder !== 'function') return false;
    const material = await globalThis.crypto.subtle.importKey(
      'raw',
      new TextEncoder().encode(password),
      { name: 'PBKDF2' },
      false,
      ['deriveBits']
    );
    const digest = new Uint8Array(await globalThis.crypto.subtle.deriveBits({
      name: 'PBKDF2',
      salt: ADMIN_SALT,
      iterations: ADMIN_ITERATIONS,
      hash: 'SHA-256'
    }, material, 256));
    const expected = new Uint8Array(ADMIN_DIGEST.match(/../g).map(x => Number.parseInt(x, 16)));
    if (digest.length !== expected.length) return false;
    let different = 0;
    for (let i = 0; i < digest.length; i += 1) different |= digest[i] ^ expected[i];
    return different === 0;
  }

  function setAdminMessage(message, controls = false) {
    const target = controls ? $('adminControlMessage') : $('adminMessage');
    target.textContent = message || '';
  }

  function updateAdminUi() {
    $('adminStatusBadge').hidden = !adminMode;
    $('adminToggle').textContent = adminMode ? '♛' : '♙';
    $('adminToggle').setAttribute('aria-label', adminMode ? 'Admin prijavljen' : 'Admin prijava');
    $('adminToggle').title = adminMode ? 'Admin · brendigo' : 'Admin prijava';
    $('adminLoginView').hidden = adminMode;
    $('adminControls').hidden = !adminMode;
    $('adminRole').textContent = adminMode ? 'Prijavljen: brendigo' : 'Prijava za napredne kontrole';
    if (!adminMode) {
      $('adminPassword').value = '';
      $('adminSource').value = '';
      $('adminHomepage').value = '';
      $('adminStationName').textContent = 'Nije odabrano';
    } else {
      void refreshAdminDetails();
    }
  }

  function openAdminPanel() {
    $('adminPanel').hidden = false;
    updateAdminUi();
    if (adminMode) $('adminSource').focus();
    else $('adminUsername').focus();
  }

  function closeAdminPanel() {
    $('adminPanel').hidden = true;
    $('adminPassword').value = '';
    $('adminToggle').focus();
  }

  async function refreshAdminDetails() {
    if (!adminMode) return;
    const generation = ++adminDetailsGeneration;
    const station = current;
    $('adminStationName').textContent = station?.name || 'Nije odabrano';
    if (!station) {
      $('adminSource').value = '';
      $('adminHomepage').value = '';
      $('adminSaveSource').disabled = true;
      $('adminResetSource').disabled = true;
      $('adminOpenWeb').disabled = true;
      return;
    }
    const key = RB.key(station);
    const override = await RB.adminOverrideFor(key).catch(() => '');
    if (generation !== adminDetailsGeneration || !adminMode || current !== station) return;
    $('adminSource').value = override || station.url_resolved || station.url || '';
    $('adminHomepage').value = RB.safeHttp(station.homepage) ? station.homepage : '';
    $('adminSaveSource').disabled = false;
    $('adminResetSource').disabled = !override;
    $('adminOpenWeb').disabled = !RB.safeHttp(station.homepage);
  }

  function logoutAdmin() {
    adminMode = false;
    adminDetailsGeneration += 1;
    setAdminMessage('');
    updateAdminUi();
    closeAdminPanel();
    playerStatus = 'Administrator je odjavljen';
    updatePlayer();
  }

  function stationCountry(station) {
    if (station?.countrycode === RB.DIASPORA_CODE) return station.country ? `Dijaspora · ${station.country}` : 'Dijaspora';
    if (station?.countrycode === RB.FOREIGN_CODE) return station.country ? `Strano · ${station.country}` : 'Strano';
    return station?.country || station?.countrycode || '';
  }

  function stationMeta(station) {
    return [
      stationCountry(station),
      String(station.tags || '').split(',').map(x => x.trim()).find(x => x.length > 1 && x.length < 20),
      station.bitrate ? `${station.bitrate} kbps` : station.codec
    ].filter(Boolean).join(' · ');
  }

  function firstUsefulTag(station) {
    return String(station?.tags || '')
      .split(',')
      .map(value => value.trim())
      .find(value => value && value.toLowerCase() !== 'dijaspora' && value.length <= 32) || '';
  }

  function stationDescription(station) {
    const area = stationCountry(station) || 'područja Radio Balkan kataloga';
    const genre = firstUsefulTag(station);
    const language = String(station?.language || '').trim();
    let text = `${station.name} je radio stanica iz područja ${area}. Slušanje se pokreće kroz sigurni Radio Balkan player s ograničenim timeoutom te fallback i recovery postupkom kada je dostupan.`;
    if (genre) text += ` Na programu je istaknuta kategorija ${genre}.`;
    if (language) text += ` Jezik programa: ${language}.`;
    return text;
  }

  function stationPublicFacts(station) {
    const parts = [];
    const state = String(station?.state || '').trim();
    if (state && state.toLowerCase() !== String(station?.country || '').trim().toLowerCase()) parts.push(state);
    if (station?.codec) parts.push(String(station.codec).toUpperCase());
    if (Number(station?.bitrate) > 0) parts.push(`${station.bitrate} kbps`);
    const tags = String(station?.tags || '')
      .split(',')
      .map(value => value.trim())
      .filter(value => value && value.toLowerCase() !== 'dijaspora')
      .slice(0, 5);
    if (tags.length) parts.push(`Kategorije: ${tags.join(', ')}`);
    return parts.join(' · ') || 'Radio uživo';
  }

  function updateStationPageFavorite() {
    const button = $('stationPageFavorite');
    if (!button || !detailStation) return;
    const favorite = !!favs[RB.key(detailStation)];
    button.textContent = favorite ? '♥ Omiljena' : '♡ Dodaj u omiljene';
    button.setAttribute('aria-pressed', String(favorite));
  }

  function renderSimilarStations(station) {
    const target = $('stationSimilarList');
    if (!target) return;
    const genre = firstUsefulTag(station).toLowerCase();
    const sameArea = value => value.countrycode === station.countrycode ||
      (!!station.sourcecountrycode && value.sourcecountrycode === station.sourcecountrycode);
    const ranked = all
      .filter(value => RB.key(value) !== RB.key(station))
      .map(value => {
        let score = 0;
        if (sameArea(value)) score += 6;
        if (genre && String(value.tags || '').toLowerCase().includes(genre)) score += 4;
        score += Math.min(3, Math.max(0, Number(value.votes || 0) / 1000));
        return { value, score };
      })
      .filter(entry => entry.score >= 4)
      .sort((a, b) => b.score - a.score || Number(b.value.votes || 0) - Number(a.value.votes || 0))
      .slice(0, 5);
    const fragment = document.createDocumentFragment();
    for (const { value } of ranked) {
      const button = document.createElement('button');
      button.type = 'button';
      button.className = 'similarStation';
      button.dataset.stationKey = RB.key(value);
      const copy = document.createElement('span');
      const name = document.createElement('strong');
      name.textContent = value.name;
      const meta = document.createElement('span');
      meta.textContent = stationMeta(value);
      copy.append(name, meta);
      const arrow = document.createElement('b');
      arrow.textContent = '›';
      button.append(copy, arrow);
      fragment.append(button);
    }
    if (!ranked.length) {
      const empty = document.createElement('div');
      empty.className = 'empty';
      empty.textContent = 'Nema dovoljno sličnih stanica za ovaj prikaz.';
      fragment.append(empty);
    }
    target.replaceChildren(fragment);
  }

  function openStationPage(station, returnFocus = null) {
    if (!station) return;
    detailStation = station;
    detailReturnFocus = returnFocus;
    $('browsePage').hidden = true;
    $('stationPage').hidden = false;
    $('stationPageTitle').textContent = station.name;
    $('stationPageMeta').textContent = stationMeta(station) || 'Radio uživo';
    $('stationBreadcrumbArea').textContent = stationCountry(station) || 'Radio uživo';
    $('stationPageDescription').textContent = stationDescription(station);
    $('stationPageFacts').textContent = stationPublicFacts(station);
    const logo = $('stationPageLogo');
    logo.onerror = null;
    logo.src = station.logo && RB.safeHttp(station.logo) ? station.logo : 'icon48.png';
    logo.onerror = () => { logo.onerror = null; logo.src = 'icon48.png'; };
    updateStationPageFavorite();
    renderSimilarStations(station);
    $('stationBack').focus();
  }

  function closeStationPage() {
    if ($('stationPage').hidden) return;
    $('stationPage').hidden = true;
    $('browsePage').hidden = false;
    const focusTarget = detailReturnFocus;
    detailReturnFocus = null;
    detailStation = null;
    focusTarget?.focus?.();
  }

  function setQuickPressed(id, pressed) {
    const button = $(id);
    if (button) button.setAttribute('aria-pressed', String(!!pressed));
  }

  function updateQuickNavigation() {
    const value = String(country.value || '').toUpperCase();
    setQuickPressed('quickAll', viewMode === 'all' && !value && !genre?.value && !favoritesOnly);
    setQuickPressed('quickTop', viewMode === 'top');
    setQuickPressed('quickDiaspora', value === RB.DIASPORA_CODE);
    setQuickPressed('quickForeign', value === RB.FOREIGN_CODE);
  }

  function selectArea(code) {
    viewMode = 'all';
    favoritesOnly = false;
    updateFavoritesFilterButton();
    search.value = '';
    if (genre) genre.value = '';
    country.value = [...country.options].some(option => option.value === code) ? code : '';
    queueUiPreferencesSave();
    apply();
    updateQuickNavigation();
  }

  function selectTop() {
    viewMode = 'top';
    favoritesOnly = false;
    search.value = '';
    country.value = '';
    if (genre) genre.value = '';
    updateFavoritesFilterButton();
    queueUiPreferencesSave();
    apply();
    updateQuickNavigation();
  }

  function foldedStation(station) {
    return RB.fold(`${station.name} ${station.country} ${station.state} ${station.tags} ${station.language} ${station.codec}`);
  }

  function containsAny(value, words) {
    return words.some(word => value.includes(RB.fold(word)));
  }

  function matchesGenre(station, selected) {
    if (!selected) return true;
    const value = foldedStation(station);
    switch (selected) {
      case 'domaca': return station.countrycode !== RB.FOREIGN_CODE && containsAny(value, ['domaca','domaća','balkan','ex yu','ex-yu','regional','croatian','hrvatska']);
      case 'pop': return containsAny(value, ['pop','rock','indie','alternative']);
      case 'folk': return containsAny(value, ['folk','narodna','narodno','turbo folk','sevdah','sevdalinka','etno','krajiska','krajiska']);
      case 'electronic': return containsAny(value, ['electronic','dance','house','techno','edm','trance','club']);
      case 'jazz': return containsAny(value, ['jazz','blues','soul']);
      case 'classical': return containsAny(value, ['classical','klasicna','klasična','opera','symphony']);
      case 'news': return containsAny(value, ['news','talk','informativni','vijesti','speech','spoken']);
      case 'hits': return containsAny(value, ['hits','top 40','top40','charts','chart','current hits']);
      case 'oldies': return containsAny(value, ['oldies','retro','60s','70s','80s','90s','classic hits']);
      default: return value.includes(RB.fold(selected));
    }
  }

  function primaryCategory(station) {
    if (station?.countrycode === RB.DIASPORA_CODE) return 'Dijaspora';
    for (const [value, label] of GENRE_LABELS) if (matchesGenre(station, value)) return label;
    return station.countrycode === RB.FOREIGN_CODE ? 'Strana postaja' : 'Radio uživo';
  }

  function initials(name) {
    return RB.fold(name).slice(0, 2).toUpperCase() || 'RB';
  }

  function setBusy(busy, message = '') {
    refresh.disabled = !!busy;
    refresh.setAttribute('aria-busy', String(!!busy));
    list.setAttribute('aria-busy', String(!!busy));
    if (message) status.textContent = message;
  }

  function updateFavoritesFilterButton() {
    const button = $('favoritesOnly');
    button.textContent = favoritesOnly ? '♥ Prikaži sve' : '♡ Omiljene';
    button.setAttribute('aria-pressed', String(favoritesOnly));
  }

  function queueUiPreferencesSave() {
    if (!uiPreferencesReady) return;
    clearTimeout(preferenceTimer);
    preferenceTimer = setTimeout(() => {
      void RB.setUiPreferences({
        country: country.value,
        genre: genre?.value || '',
        favoritesOnly
      }).catch(() => {});
    }, 120);
  }

  function restoreUiPreferences(value) {
    if (uiPreferencesReady) return;
    const prefs = value && typeof value === 'object' ? value : {};
    const requestedCountry = String(prefs.country || '').toUpperCase();
    const requestedGenre = String(prefs.genre || '').toLowerCase();
    if (requestedCountry && [...country.options].some(option => option.value === requestedCountry)) country.value = requestedCountry;
    if (genre && GENRE_LABELS.has(requestedGenre)) genre.value = requestedGenre;
    favoritesOnly = !!prefs.favoritesOnly;
    updateFavoritesFilterButton();
    uiPreferencesReady = true;
  }

  function resetFilters() {
    clearTimeout(searchTimer);
    viewMode = 'all';
    search.value = '';
    country.value = '';
    if (genre) genre.value = '';
    favoritesOnly = false;
    updateFavoritesFilterButton();
    queueUiPreferencesSave();
    apply();
    search.focus();
  }

  function rememberRetiredEpoch(epoch) {
    if (!epoch) return;
    retiredEpochs.add(epoch);
    while (retiredEpochs.size > 8) retiredEpochs.delete(retiredEpochs.values().next().value);
  }

  function acceptStateEnvelope(value, allowEpochChange = false) {
    if (!value || typeof value !== 'object') return false;
    const epoch = String(value.epoch || '');
    const revision = Number(value.revision);
    if (!epoch || !Number.isInteger(revision) || revision < 0 || retiredEpochs.has(epoch)) return false;
    if (!stateEpoch) {
      stateEpoch = epoch;
      lastRevision = -1;
    } else if (epoch !== stateEpoch) {
      if (!allowEpochChange) return false;
      rememberRetiredEpoch(stateEpoch);
      stateEpoch = epoch;
      lastRevision = -1;
    }
    if (revision < lastRevision) return false;
    lastRevision = revision;
    return true;
  }

  function applyStateEnvelope(value, updateStatus = true, allowEpochChange = false) {
    if (!acceptStateEnvelope(value, allowEpochChange)) return false;
    const hasRemoteStation = !!value.station;
    if (hasRemoteStation) current = value.station;
    else if (!value.sessionId && !value.playing) current = null;
    playing = !!value.playing;
    const hasSession = !!value.sessionId;
    stopped = !playing && !hasSession;
    if (updateStatus) {
      if (playing) playerStatus = 'Sada svira';
      else if (hasRemoteStation && hasSession) playerStatus = 'Pauzirano';
      else if (hasRemoteStation) playerStatus = 'Zaustavljeno';
      else playerStatus = 'Spremno';
    }
    return true;
  }

  async function applyCommandResult(result, token) {
    if (token !== commandGeneration) return false;
    const incomingEpoch = String(result?.epoch || '');
    if (stateEpoch && incomingEpoch && incomingEpoch !== stateEpoch) {
      const confirmed = await ext.runtime.sendMessage({ type: 'RB_GET_STATE' }).catch(() => null);
      if (token !== commandGeneration) return false;
      return applyStateEnvelope(confirmed, false, true);
    }
    return applyStateEnvelope(result, false, !stateEpoch);
  }

  function synchronizePlayerState() {
    if (stateSyncPromise) return stateSyncPromise;
    const syncToken = commandGeneration;
    stateSyncPromise = Promise.resolve(ext.runtime.sendMessage({ type: 'RB_GET_STATE' }))
      .then(result => {
        if (syncToken !== commandGeneration || activeCommandToken) return false;
        if (!applyStateEnvelope(result, true, true)) return false;
        updatePlayer();
        syncStationPlaybackUi();
        return true;
      })
      .catch(() => false)
      .finally(() => { stateSyncPromise = null; });
    return stateSyncPromise;
  }

  function fillCountries() {
    const previous = country.value;
    const counts = new Map();
    for (const station of all) counts.set(station.countrycode, (counts.get(station.countrycode) || 0) + 1);
    country.replaceChildren();
    const allOption = document.createElement('option');
    allOption.value = '';
    allOption.textContent = `Sve postaje · ${all.length}`;
    country.append(allOption);
    for (const [code, name] of RB.COUNTRIES) {
      const count = counts.get(code) || 0;
      if (code === RB.FOREIGN_CODE && !count) continue;
      const option = document.createElement('option');
      option.value = code;
      option.textContent = `${name} · ${count}`;
      country.append(option);
    }
    country.value = RB.COUNTRIES.some(([code]) => code === previous) ? previous : '';
  }

  function apply() {
    const q = RB.fold(search.value);
    const selectedCountry = country.value;
    const selectedGenre = genre?.value || '';
    visible = all.filter(station =>
      (!selectedCountry || station.countrycode === selectedCountry) &&
      matchesGenre(station, selectedGenre) &&
      (!favoritesOnly || favs[RB.key(station)]) &&
      (!q || foldedStation(station).includes(q))
    );
    if (viewMode === 'top') {
      visible = visible.slice().sort((a, b) => Number(b.votes || 0) - Number(a.votes || 0) || a.name.localeCompare(b.name)).slice(0, 50);
    }
    renderLimit = PAGE;
    render();
    updatePlayer();
    updateQuickNavigation();
  }

  function stationCard(station) {
    const key = RB.key(station);
    const active = !!current && RB.key(current) === key;
    const row = document.createElement('article');
    row.className = `station${active ? ' active' : ''}`;
    row.dataset.key = key;
    row.tabIndex = 0;
    row.setAttribute('role', 'listitem');
    row.setAttribute('aria-current', active ? 'true' : 'false');
    row.setAttribute('aria-label', `${station.name}, ${stationMeta(station)}`);

    const logoBox = document.createElement('div');
    logoBox.className = 'logoBox';
    if (station.logo && RB.safeHttp(station.logo)) {
      const image = document.createElement('img');
      image.src = station.logo;
      image.alt = '';
      image.referrerPolicy = 'no-referrer';
      image.loading = 'lazy';
      image.decoding = 'async';
      image.width = 56;
      image.height = 56;
      const fallback = document.createElement('span');
      fallback.className = 'fallback';
      fallback.textContent = initials(station.name);
      fallback.hidden = true;
      image.addEventListener('error', () => { image.remove(); fallback.hidden = false; }, { once: true });
      logoBox.append(image, fallback);
    } else {
      const fallback = document.createElement('span');
      fallback.className = 'fallback';
      fallback.textContent = initials(station.name);
      logoBox.append(fallback);
    }

    const copy = document.createElement('div');
    copy.className = 'stationText';
    const name = document.createElement('strong');
    name.textContent = station.name;
    const meta = document.createElement('span');
    meta.textContent = stationMeta(station);
    const hint = document.createElement('em');
    hint.textContent = favs[key] ? '♥ Omiljena' : primaryCategory(station);
    copy.append(name, meta, hint);

    const play = document.createElement('button');
    play.className = 'stationPlay';
    play.type = 'button';
    play.dataset.play = key;
    play.title = active && playing ? 'Pauziraj' : 'Slušaj';
    play.setAttribute('aria-label', `${play.title} ${station.name}`);
    play.textContent = active && playing ? 'Ⅱ' : '▶';
    row.append(logoBox, copy, play);
    return row;
  }

  function syncStationPlaybackUi() {
    const nodes = typeof list.querySelectorAll === 'function' ? list.querySelectorAll('.station') : [];
    const currentKey = current ? RB.key(current) : '';
    for (const row of nodes) {
      const active = !!currentKey && row.dataset.key === currentKey;
      row.classList?.toggle('active', active);
      row.setAttribute?.('aria-current', active ? 'true' : 'false');
      const button = typeof row.querySelector === 'function' ? row.querySelector('.stationPlay') : null;
      if (!button) continue;
      const station = visible.find(item => RB.key(item) === row.dataset.key);
      const action = active && playing ? 'Pauziraj' : 'Slušaj';
      button.textContent = active && playing ? 'Ⅱ' : '▶';
      button.title = action;
      button.disabled = !!activeCommandToken && active;
      button.setAttribute('aria-busy', String(!!activeCommandToken && active));
      button.setAttribute('aria-label', station ? `${action} ${station.name}` : action);
    }
  }

  function render() {
    const shown = visible.slice(0, renderLimit);
    const fragment = document.createDocumentFragment();
    for (const station of shown) fragment.append(stationCard(station));
    if (!shown.length) {
      const message = favoritesOnly ? 'Još nema omiljenih stanica za ovaj prikaz.' : 'Nema stanica za odabranu kombinaciju filtera.';
      fragment.append(renderEmptyState(message, 'Poništi filtre', resetFilters));
    } else if (visible.length > renderLimit) {
      const more = document.createElement('button');
      more.type = 'button';
      more.className = 'moreStations';
      more.dataset.more = '1';
      more.textContent = `Prikaži još (${visible.length - renderLimit})`;
      fragment.append(more);
    }
    list.replaceChildren(fragment);
    const count = Math.min(renderLimit, visible.length);
    const activeArea = country.value ? country.options?.[country.selectedIndex]?.textContent?.split(' · ')[0] : '';
    const activeGenre = genre?.value ? GENRE_LABELS.get(genre.value) : '';
    const context = [activeArea, activeGenre].filter(Boolean).join(' · ');
    status.textContent = `${count} od ${visible.length} prikazano${context ? ` · ${context}` : ''}`;
    $('clearFilters').hidden = !(search.value || country.value || genre?.value || favoritesOnly);
  }

  function focusStationAt(index) {
    if (!Number.isInteger(index) || index < 0 || typeof list.querySelectorAll !== 'function') return;
    const buttons = list.querySelectorAll('.stationPlay');
    const target = buttons?.[index];
    if (target && typeof target.focus === 'function') target.focus();
  }

  function renderEmptyState(message, actionLabel = '', action = null) {
    const empty = document.createElement('div');
    empty.className = 'empty';
    empty.setAttribute('role', 'status');
    const copy = document.createElement('p');
    copy.textContent = message;
    empty.append(copy);
    if (actionLabel && typeof action === 'function') {
      const button = document.createElement('button');
      button.type = 'button';
      button.className = 'emptyAction';
      button.textContent = actionLabel;
      button.addEventListener('click', action, { once: true });
      empty.append(button);
    }
    return empty;
  }


  async function play(station) {
    if (!station) return;
    if (activeCommandToken && current && RB.key(current) === RB.key(station)) return;
    const token = ++commandGeneration;
    activeCommandToken = token;
    current = station;
    playing = false;
    playerStatus = 'Povezujem…';
    updatePlayer();
    syncStationPlaybackUi();
    try {
      const override = await RB.adminOverrideFor(RB.key(station)).catch(() => '');
      if (token !== commandGeneration) return;
      const playableStation = override ? { ...station, url_resolved: override } : station;
      const result = await ext.runtime.sendMessage({ type: 'RB_PLAY', station: playableStation });
      if (token !== commandGeneration) return;
      await applyCommandResult(result, token);
      if (token !== commandGeneration) return;
      if (result?.error && !playing) playerStatus = 'Nedostupno';
      else playerStatus = playing ? 'Sada svira' : 'Nedostupno';
    } catch {
      if (token !== commandGeneration) return;
      playerStatus = playing ? 'Sada svira' : 'Nedostupno';
    } finally {
      if (token === commandGeneration) {
        activeCommandToken = 0;
        updatePlayer();
        syncStationPlaybackUi();
      }
    }
  }

  function navigationStations() {
    if (visible.length) return visible;
    return all;
  }

  async function playAdjacent(delta) {
    if (activeCommandToken || !delta || !current) return;
    const source = navigationStations();
    if (source.length < 2) return;
    const currentKey = RB.key(current);
    let index = source.findIndex(station => RB.key(station) === currentKey);
    if (index < 0) index = delta > 0 ? -1 : 0;
    const nextIndex = (index + (delta > 0 ? 1 : -1) + source.length) % source.length;
    await play(source[nextIndex]);
  }

  async function toggle() {
    if (activeCommandToken) return;
    if (!current && visible.length) return play(visible[0]);
    if (!current) return;
    if (stopped) return play(current);
    const token = ++commandGeneration;
    activeCommandToken = token;
    playerStatus = playing ? 'Pauziram…' : 'Povezujem…';
    updatePlayer();
    try {
      const result = await ext.runtime.sendMessage({ type: 'RB_TOGGLE' });
      if (token !== commandGeneration) return;
      await applyCommandResult(result, token);
      if (token !== commandGeneration) return;
      if (result?.error && !playing) playerStatus = 'Nedostupno';
      else playerStatus = playing ? 'Sada svira' : 'Pauzirano';
    } catch {
      if (token !== commandGeneration) return;
      playerStatus = playing ? 'Sada svira' : 'Nedostupno';
    } finally {
      if (token === commandGeneration) {
        activeCommandToken = 0;
        updatePlayer();
        syncStationPlaybackUi();
      }
    }
  }

  async function stopPlayback() {
    if (activeCommandToken || !current) return;
    const token = ++commandGeneration;
    activeCommandToken = token;
    playerStatus = 'Zaustavljam…';
    updatePlayer();
    syncStationPlaybackUi();
    try {
      const result = await ext.runtime.sendMessage({ type: 'RB_STOP' });
      if (token !== commandGeneration) return;
      await applyCommandResult(result, token);
      if (token !== commandGeneration) return;
      playing = false;
      stopped = !result?.error;
      playerStatus = result?.error ? 'Zaustavljanje nije uspjelo' : 'Zaustavljeno';
    } catch {
      if (token !== commandGeneration) return;
      playerStatus = playing ? 'Sada svira' : 'Zaustavljanje nije uspjelo';
    } finally {
      if (token === commandGeneration) {
        activeCommandToken = 0;
        updatePlayer();
        syncStationPlaybackUi();
      }
    }
  }

  function updatePlayer() {
    const station = current;
    $('playerState').textContent = playerStatus;
    $('playerName').textContent = station ? station.name : 'Nije odabrano';
    $('playerMeta').textContent = station ? stationMeta(station) : 'Odaberi stanicu';
    $('heroName').textContent = station ? station.name : (visible[0]?.name || all[0]?.name || 'Radio Balkan');
    if (station) {
      $('heroMeta').textContent = stationMeta(station);
    } else {
      const foreign = all.filter(item => item.countrycode === RB.FOREIGN_CODE).length;
      const diaspora = all.filter(item => item.countrycode === RB.DIASPORA_CODE).length;
      const regional = all.length - foreign - diaspora;
      $('heroMeta').textContent = `${regional} regionalnih · ${diaspora} dijaspora · ${foreign} stranih postaja`;
    }
    const playerLogo = $('playerLogo');
    playerLogo.onerror = null;
    playerLogo.src = station?.logo && RB.safeHttp(station.logo) ? station.logo : 'icon48.png';
    playerLogo.onerror = () => { playerLogo.onerror = null; playerLogo.src = 'icon48.png'; };
    const commandBusy = !!activeCommandToken;
    $('playerToggle').textContent = playing ? 'Ⅱ' : '▶';
    $('playerToggle').disabled = !station || commandBusy;
    $('playerToggle').setAttribute('aria-busy', String(commandBusy));
    $('playerToggle').setAttribute('aria-label', commandBusy ? 'Radnja je u tijeku' : (playing ? 'Pauziraj' : 'Pokreni'));
    $('heroPlay').textContent = commandBusy ? 'Pričekaj…' : (playing && station ? 'Ⅱ Pauziraj' : '▶ Slušaj');
    $('heroPlay').disabled = commandBusy || (!station && !visible.length);
    $('heroPlay').setAttribute('aria-busy', String(commandBusy));
    const key = station ? RB.key(station) : '';
    const favorite = !!(key && favs[key]);
    $('playerFav').textContent = favorite ? '♥' : '♡';
    $('playerFav').disabled = !station || commandBusy;
    $('playerFav').setAttribute('aria-pressed', String(favorite));
    $('playerFav').setAttribute('aria-label', favorite ? 'Ukloni iz omiljenih' : 'Dodaj u omiljene');
    $('playerStop').disabled = !station || commandBusy || stopped;
    $('playerStop').setAttribute('aria-busy', String(commandBusy));
    $('playerStop').setAttribute('aria-label', commandBusy ? 'Radnja je u tijeku' : (stopped ? 'Reprodukcija je zaustavljena' : 'Zaustavi reprodukciju'));
    const navigationCount = navigationStations().length;
    const canNavigate = !!station && navigationCount > 1 && !commandBusy;
    $('playerPrev').disabled = !canNavigate;
    $('playerNext').disabled = !canNavigate;
    $('playerPrev').setAttribute('aria-busy', String(commandBusy));
    $('playerNext').setAttribute('aria-busy', String(commandBusy));
    if (adminMode && !$('adminPanel').hidden) void refreshAdminDetails();
  }

  list.addEventListener('click', event => {
    const more = event.target.closest('[data-more]');
    if (more) {
      const previousLimit = renderLimit;
      renderLimit = Math.min(visible.length, renderLimit + PAGE);
      render();
      if (renderLimit > previousLimit) focusStationAt(previousLimit);
      return;
    }
    const row = event.target.closest('.station');
    if (!row) return;
    const station = visible.find(item => RB.key(item) === row.dataset.key);
    if (!station) return;
    if (event.target.closest('.stationPlay')) {
      void play(station);
      return;
    }
    openStationPage(station, row);
  });

  list.addEventListener('keydown', event => {
    if (event.key !== 'Enter' && event.key !== ' ') return;
    if (event.target.closest('button')) return;
    const row = event.target.closest('.station');
    if (!row) return;
    event.preventDefault();
    const station = visible.find(item => RB.key(item) === row.dataset.key);
    if (station) openStationPage(station, row);
  });

  $('stationBack').addEventListener('click', closeStationPage);
  $('stationPagePlay').addEventListener('click', () => {
    if (detailStation) void play(detailStation);
  });
  $('stationPageFavorite').addEventListener('click', async () => {
    if (!detailStation) return;
    const key = RB.key(detailStation);
    try {
      favs = await RB.setFavorite(key, !favs[key]);
      updateStationPageFavorite();
      render();
      updatePlayer();
    } catch {
      playerStatus = 'Omiljene nisu spremljene';
      updatePlayer();
    }
  });
  $('stationSimilarList').addEventListener('click', event => {
    const button = event.target.closest('[data-station-key]');
    if (!button) return;
    const station = all.find(item => RB.key(item) === button.dataset.stationKey);
    if (station) openStationPage(station, button);
  });
  $('stationPage').addEventListener('keydown', event => {
    if (event.key !== 'Escape') return;
    event.preventDefault();
    closeStationPage();
  });

  $('quickAll').addEventListener('click', () => selectArea(''));
  $('quickTop').addEventListener('click', selectTop);
  $('quickCountries').addEventListener('click', () => country.focus());
  $('quickGenres').addEventListener('click', () => genre?.focus());
  $('quickDiaspora').addEventListener('click', () => selectArea(RB.DIASPORA_CODE));
  $('quickForeign').addEventListener('click', () => selectArea(RB.FOREIGN_CODE));

  search.addEventListener('input', () => {
    viewMode = 'all';
    clearTimeout(searchTimer);
    searchTimer = setTimeout(apply, 130);
  });
  country.addEventListener('change', () => { viewMode = 'all'; apply(); queueUiPreferencesSave(); });
  genre?.addEventListener('change', () => { viewMode = 'all'; apply(); queueUiPreferencesSave(); });
  $('adminToggle').addEventListener('click', openAdminPanel);
  $('adminClose').addEventListener('click', closeAdminPanel);
  $('adminPanel').addEventListener('click', event => { if (event.target === $('adminPanel')) closeAdminPanel(); });
  $('adminPanel').addEventListener('keydown', event => {
    if (event.key === 'Escape') {
      event.preventDefault();
      closeAdminPanel();
    }
  });
  async function attemptAdminLogin() {
    const loginButton = $('adminLogin');
    if (loginButton.disabled) return;
    const now = Date.now();
    if (now < adminLockedUntil) {
      setAdminMessage(`Previše pokušaja. Pokušaj ponovno za ${Math.max(1, Math.ceil((adminLockedUntil - now) / 1000))} s.`);
      return;
    }
    loginButton.disabled = true;
    loginButton.textContent = 'Provjeravam…';
    loginButton.setAttribute('aria-busy', 'true');
    setAdminMessage('Provjeravam…');
    try {
      const ok = await adminCredentialsValid($('adminUsername').value, $('adminPassword').value);
      $('adminPassword').value = '';
      if (ok) {
        adminMode = true;
        adminFailures = 0;
        adminLockedUntil = 0;
        setAdminMessage('');
        updateAdminUi();
        setAdminMessage('Admin kontrole su otključane.', true);
        return;
      }
      adminFailures += 1;
      if (adminFailures >= 5) {
        adminFailures = 0;
        adminLockedUntil = Date.now() + 30000;
        setAdminMessage('Previše neuspjelih pokušaja. Prijava je zaključana 30 s.');
      } else {
        setAdminMessage('Neispravno korisničko ime ili lozinka.');
      }
      $('adminPassword').focus();
    } catch {
      $('adminPassword').value = '';
      setAdminMessage('Prijava trenutačno nije dostupna.');
      $('adminPassword').focus();
    } finally {
      loginButton.disabled = false;
      loginButton.textContent = 'Prijavi se';
      loginButton.setAttribute('aria-busy', 'false');
    }
  }

  $('adminLogin').addEventListener('click', () => void attemptAdminLogin());
  for (const input of [$('adminUsername'), $('adminPassword')]) {
    input.addEventListener('keydown', event => {
      if (event.key !== 'Enter') return;
      event.preventDefault();
      void attemptAdminLogin();
    });
  }
  $('adminLogout').addEventListener('click', logoutAdmin);
  $('adminSaveSource').addEventListener('click', async () => {
    if (!adminMode || !current) return;
    const value = String($('adminSource').value || '').trim();
    if (value && !RB.safeHttp(value)) {
      setAdminMessage('Izvor mora biti sigurna javna http/https poveznica.', true);
      return;
    }
    try {
      await RB.setAdminOverride(RB.key(current), value);
      setAdminMessage(value ? 'Admin izvor je spremljen · ponovno povezujem…' : 'Vraćen je automatski izvor · ponovno povezujem…', true);
      const station = current;
      await refreshAdminDetails();
      if (station && current === station) await play(station);
    } catch {
      setAdminMessage('Izvor nije moguće spremiti.', true);
    }
  });
  $('adminResetSource').addEventListener('click', async () => {
    if (!adminMode || !current) return;
    try {
      await RB.setAdminOverride(RB.key(current), '');
      setAdminMessage('Vraćen je automatski izvor · ponovno povezujem…', true);
      const station = current;
      await refreshAdminDetails();
      if (station && current === station) await play(station);
    } catch {
      setAdminMessage('Izvor nije moguće vratiti.', true);
    }
  });
  $('adminOpenWeb').addEventListener('click', () => {
    if (!adminMode || !current || !RB.safeHttp(current.homepage)) return;
    const result = ext.tabs?.create?.({ url: current.homepage });
    if (result?.catch) result.catch(() => setAdminMessage('Web stranicu nije moguće otvoriti.', true));
  });

  refresh.addEventListener('click', () => void load(true));
  $('clearFilters').addEventListener('click', resetFilters);
  $('heroPlay').addEventListener('click', () => void toggle());
  $('playerToggle').addEventListener('click', () => void toggle());
  $('playerPrev').addEventListener('click', () => void playAdjacent(-1));
  $('playerStop').addEventListener('click', () => void stopPlayback());
  $('playerNext').addEventListener('click', () => void playAdjacent(1));
  $('favoritesOnly').addEventListener('click', () => {
    viewMode = 'all';
    favoritesOnly = !favoritesOnly;
    updateFavoritesFilterButton();
    queueUiPreferencesSave();
    apply();
  });
  $('playerFav').addEventListener('click', async () => {
    if (!current) return;
    const key = RB.key(current);
    try {
      favs = await RB.setFavorite(key, !favs[key]);
      apply();
      if (detailStation) updateStationPageFavorite();
    } catch {
      playerStatus = 'Omiljene nisu spremljene';
      updatePlayer();
    }
  });

  ext.runtime.onMessage.addListener(message => {
    if (message?.type !== 'RB_STATE' || activeCommandToken) return;
    const incomingEpoch = String(message.epoch || '');
    if (stateEpoch && incomingEpoch && incomingEpoch !== stateEpoch) {
      void synchronizePlayerState();
      return;
    }
    if (!applyStateEnvelope(message, true, !stateEpoch)) return;
    updatePlayer();
    syncStationPlaybackUi();
  });

  async function load(force = false) {
    const hadCatalog = all.length > 0;
    setBusy(true, force ? 'Osvježavam katalog…' : 'Učitavam provjerene radio postaje…');
    try {
      const preferencesPromise = uiPreferencesReady ? Promise.resolve(null) : RB.uiPreferences().catch(() => ({}));
      const loaded = await RB.load(force);
      all = loaded;
      favs = await RB.favorites();
      fillCountries();
      const preferences = await preferencesPromise;
      if (!uiPreferencesReady) restoreUiPreferences(preferences);
      if (!current && all.length) current = all[0];
      apply();
      const readToken = commandGeneration;
      const playerState = await ext.runtime.sendMessage({ type: 'RB_GET_STATE' }).catch(() => null);
      if (readToken === commandGeneration && applyStateEnvelope(playerState, true, true)) {
        updatePlayer();
        syncStationPlaybackUi();
      } else {
        updatePlayer();
      }
    } catch {
      if (hadCatalog) {
        status.textContent = 'Nije moguće osvježiti · prikazan je postojeći popis';
      } else {
        status.textContent = 'Katalog trenutačno nije dostupan';
        list.replaceChildren(renderEmptyState(
          'Provjeri internetsku vezu i pokušaj ponovno.',
          'Pokušaj ponovno',
          () => void load(true)
        ));
      }
    } finally {
      setBusy(false);
    }
  }

  updateAdminUi();
  updatePlayer();
  updateQuickNavigation();
  void load(false);
})();
