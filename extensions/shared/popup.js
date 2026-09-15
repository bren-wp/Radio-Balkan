(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  const ext = RB.ext;
  const PAGE = 48;
  let all = [];
  let visible = [];
  let current = null;
  let playing = false;
  let favs = {};
  let favoritesOnly = false;
  let searchTimer = 0;
  let renderLimit = PAGE;
  let playerStatus = 'Spremno';
  let commandGeneration = 0;
  let activeCommandToken = 0;
  let stateEpoch = '';
  let lastRevision = -1;
  let stateSyncPromise = null;
  const retiredEpochs = new Set();

  const search = $('search');
  const country = $('country');
  const list = $('stations');
  const status = $('status');

  function stationMeta(station) {
    return [
      station.country || station.countrycode,
      String(station.tags || '').split(',').map(x => x.trim()).find(x => x.length > 1 && x.length < 20),
      station.bitrate ? `${station.bitrate} kbps` : station.codec
    ].filter(Boolean).join(' · ');
  }

  function initials(name) {
    return RB.fold(name).slice(0, 2).toUpperCase() || 'RB';
  }

  function rememberRetiredEpoch(epoch) {
    if (!epoch) return;
    retiredEpochs.add(epoch);
    while (retiredEpochs.size > 8) {
      retiredEpochs.delete(retiredEpochs.values().next().value);
    }
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
    playing = !!value.playing;
    if (updateStatus) {
      if (playing) playerStatus = 'Sada svira';
      else if (hasRemoteStation) playerStatus = 'Pauzirano';
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
        render();
        return true;
      })
      .catch(() => false)
      .finally(() => { stateSyncPromise = null; });
    return stateSyncPromise;
  }

  function fillCountries() {
    const counts = new Map();
    for (const station of all) counts.set(station.countrycode, (counts.get(station.countrycode) || 0) + 1);
    country.replaceChildren();
    const allOption = document.createElement('option');
    allOption.value = '';
    allOption.textContent = `Sve zemlje · ${all.length}`;
    country.append(allOption);
    for (const [code, name] of RB.COUNTRIES) {
      const option = document.createElement('option');
      option.value = code;
      option.textContent = `${name} · ${counts.get(code) || 0}`;
      country.append(option);
    }
  }

  function apply() {
    const q = RB.fold(search.value);
    const selectedCountry = country.value;
    visible = all.filter(station =>
      (!selectedCountry || station.countrycode === selectedCountry) &&
      (!favoritesOnly || favs[RB.key(station)]) &&
      (!q || RB.fold(`${station.name} ${station.country} ${station.countrycode} ${station.state} ${station.tags} ${station.language}`).includes(q))
    );
    renderLimit = PAGE;
    render();
  }

  function stationCard(station) {
    const key = RB.key(station);
    const active = !!current && RB.key(current) === key;
    const row = document.createElement('article');
    row.className = `station${active ? ' active' : ''}`;
    row.dataset.key = key;
    row.setAttribute('role', 'group');
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
      image.width = 58;
      image.height = 58;
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
    hint.textContent = favs[key] ? '♥ Omiljena' : 'Radio uživo';
    copy.append(name, meta, hint);

    const play = document.createElement('button');
    play.className = 'stationPlay';
    play.type = 'button';
    play.dataset.play = key;
    play.title = active && playing ? 'Pauziraj' : 'Slušaj';
    play.setAttribute('aria-label', play.title);
    play.textContent = active && playing ? 'Ⅱ' : '▶';
    row.append(logoBox, copy, play);
    return row;
  }

  function render() {
    const shown = visible.slice(0, renderLimit);
    const fragment = document.createDocumentFragment();
    for (const station of shown) fragment.append(stationCard(station));
    if (!shown.length) {
      const empty = document.createElement('div');
      empty.className = 'empty';
      empty.textContent = favoritesOnly ? 'Još nema omiljenih stanica.' : 'Nema stanica za ovaj prikaz.';
      fragment.append(empty);
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
    status.textContent = `${count} od ${visible.length} prikazano · ${all.length} ukupno`;
  }

  async function play(station) {
    if (!station) return;
    const token = ++commandGeneration;
    activeCommandToken = token;
    current = station;
    playing = false;
    playerStatus = 'Povezujem…';
    updatePlayer();
    render();
    try {
      const result = await ext.runtime.sendMessage({ type: 'RB_PLAY', station });
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
        render();
      }
    }
  }

  async function toggle() {
    if (!current && visible.length) return play(visible[0]);
    if (!current) return;
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
        render();
      }
    }
  }

  function updatePlayer() {
    const station = current;
    $('playerState').textContent = playerStatus;
    $('playerName').textContent = station ? station.name : 'Ništa';
    $('playerMeta').textContent = station ? stationMeta(station) : 'Odaberi stanicu';
    $('heroName').textContent = station ? station.name : (all[0]?.name || 'Radio Balkan');
    $('heroMeta').textContent = station ? stationMeta(station) : `${all.length || '600+'} stanica iz Hrvatske i regije`;
    const playerLogo = $('playerLogo');
    playerLogo.onerror = null;
    playerLogo.src = station?.logo && RB.safeHttp(station.logo) ? station.logo : 'icon48.png';
    playerLogo.onerror = () => { playerLogo.onerror = null; playerLogo.src = 'icon48.png'; };
    $('playerToggle').textContent = playing ? 'Ⅱ' : '▶';
    $('playerToggle').setAttribute('aria-label', playing ? 'Pauziraj' : 'Pokreni');
    $('heroPlay').textContent = playing && station ? 'Ⅱ Pauziraj' : '▶ Slušaj';
    const key = station ? RB.key(station) : '';
    $('playerFav').textContent = key && favs[key] ? '♥' : '♡';
  }

  list.addEventListener('click', event => {
    const more = event.target.closest('[data-more]');
    if (more) {
      renderLimit = Math.min(visible.length, renderLimit + PAGE);
      render();
      return;
    }
    const row = event.target.closest('.station');
    if (!row) return;
    const station = visible.find(item => RB.key(item) === row.dataset.key);
    void play(station);
  });

  search.addEventListener('input', () => {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(apply, 130);
  });
  country.addEventListener('change', apply);
  $('refresh').addEventListener('click', () => void load(true));
  $('heroPlay').addEventListener('click', () => void toggle());
  $('playerToggle').addEventListener('click', () => void toggle());
  $('favoritesOnly').addEventListener('click', () => {
    favoritesOnly = !favoritesOnly;
    const button = $('favoritesOnly');
    button.textContent = favoritesOnly ? '♥ Sve stanice' : '♡ Omiljene';
    button.setAttribute('aria-pressed', String(favoritesOnly));
    apply();
  });
  $('playerFav').addEventListener('click', async () => {
    if (!current) return;
    const key = RB.key(current);
    favs = await RB.setFavorite(key, !favs[key]);
    updatePlayer();
    render();
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
    render();
  });

  async function load(force = false) {
    status.textContent = force ? 'Osvježavam katalog…' : 'Učitavam provjerene radio postaje…';
    $('refresh').disabled = true;
    try {
      all = await RB.load(force);
      favs = await RB.favorites();
      fillCountries();
      if (!current && all.length) current = all[0];
      apply();
      const readToken = commandGeneration;
      const playerState = await ext.runtime.sendMessage({ type: 'RB_GET_STATE' }).catch(() => null);
      if (readToken === commandGeneration && applyStateEnvelope(playerState, true, true)) {
        updatePlayer();
        render();
      } else {
        updatePlayer();
      }
    } catch {
      status.textContent = 'Katalog trenutačno nije dostupan';
      const empty = document.createElement('div');
      empty.className = 'empty';
      empty.textContent = 'Provjeri internetsku vezu i pokušaj ponovno.';
      list.replaceChildren(empty);
    } finally {
      $('refresh').disabled = false;
    }
  }

  void load(false);
})();
