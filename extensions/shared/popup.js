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
    current = station;
    status.textContent = `Povezujem · ${station.name}`;
    updatePlayer();
    render();
    try {
      const result = await ext.runtime.sendMessage({ type: 'RB_PLAY', station });
      if (result?.error) {
        playing = false;
        status.textContent = result.error;
      } else {
        playing = result?.playing !== false;
        status.textContent = playing ? 'Reprodukcija je pokrenuta' : 'Stanica trenutačno nije dostupna';
      }
    } catch {
      playing = false;
      status.textContent = 'Reprodukcija trenutačno nije dostupna';
    }
    updatePlayer();
    render();
  }

  async function toggle() {
    if (!current && visible.length) return play(visible[0]);
    if (!current) return;
    try {
      const result = await ext.runtime.sendMessage({ type: 'RB_TOGGLE' });
      if (result?.error) {
        playing = false;
        status.textContent = result.error;
      } else {
        playing = !!result?.playing;
        status.textContent = playing ? 'Reprodukcija je pokrenuta' : 'Pauzirano';
      }
    } catch {
      playing = false;
      status.textContent = 'Reprodukcija trenutačno nije dostupna';
    }
    updatePlayer();
    render();
  }

  function updatePlayer() {
    const station = current;
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
    if (message?.type !== 'RB_STATE') return;
    playing = !!message.playing;
    if (message.station) current = message.station;
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
      const playerState = await ext.runtime.sendMessage({ type: 'RB_GET_STATE' }).catch(() => null);
      if (playerState?.station) current = playerState.station;
      playing = !!playerState?.playing;
      updatePlayer();
      render();
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
