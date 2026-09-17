const RB = (() => {
  const API_FALLBACKS = [
    'https://de1.api.radio-browser.info',
    'https://de2.api.radio-browser.info',
    'https://at1.api.radio-browser.info',
    'https://nl1.api.radio-browser.info'
  ];
  const BALKAN_COUNTRIES = [
    ['HR','Hrvatska'], ['BA','Bosna i Hercegovina'], ['RS','Srbija'],
    ['SI','Slovenija'], ['MK','Sjeverna Makedonija'], ['AL','Albanija'], ['ME','Crna Gora']
  ];
  const FOREIGN_CODE = 'INT';
  const COUNTRIES = [...BALKAN_COUNTRIES, [FOREIGN_CODE, 'Strano']];
  const BALKAN_ALLOWED = new Set(BALKAN_COUNTRIES.map(x => x[0]));
  const ALLOWED = new Set(COUNTRIES.map(x => x[0]));
  const PAGE = 200;
  const MAX_PER_COUNTRY = 1600;
  const MAX_FOREIGN = 50;
  const FOREIGN_SCAN_LIMIT = 500;
  const MAX_CATALOG = 7050;
  const CACHE_MS = 12 * 60 * 60 * 1000;
  const ext = globalThis.browser || globalThis.chrome;

  const clean = value => String(value ?? '')
    .replace(/[\u0000-\u001f\u007f]+/g, ' ')
    .replace(/\s+/g, ' ').trim().slice(0, 2048);
  const fold = value => clean(value).toLocaleLowerCase().normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '').replace(/đ/g, 'd').replace(/[^\p{L}\p{N}]+/gu, '');

  const safeHttp = RBNet.safeHttp;
  const safeApiBase = RBNet.safeRadioBrowserBase;

  async function fetchWithTimeout(url, options = {}, timeoutMs = 9000) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);
    try {
      return await fetch(url, { ...options, signal: controller.signal });
    } finally {
      clearTimeout(timer);
    }
  }

  const host = raw => {
    try { return new URL(raw).hostname.toLowerCase().replace(/\.+$/, '').replace(/^www\./, ''); }
    catch { return ''; }
  };

  const logo = station => {
    if (safeHttp(station.favicon)) return station.favicon;
    try {
      const u = new URL(station.homepage);
      if (safeHttp(u.href)) {
        u.pathname = '/favicon.ico';
        u.search = '';
        u.hash = '';
        return u.href;
      }
    } catch { }
    return '';
  };

  const identity = station => {
    const name = fold(station.name);
    const country = clean(station.countrycode).toUpperCase();
    if (!name || !ALLOWED.has(country)) return '';
    const homepageHost = host(station.homepage);
    if (homepageHost) return `${country}|${name}|home:${homepageHost}`;
    try {
      const u = new URL(station.url_resolved || station.url);
      const streamHost = u.hostname.toLowerCase().replace(/\.+$/, '');
      return `${country}|${name}|stream:${streamHost}${u.pathname.toLowerCase().replace(/\/$/, '')}`;
    } catch { return `${country}|${name}`; }
  };

  const quality = station => (station.lastcheckok ? 1000000 : 0) + (Number(station.votes) || 0) +
    (logo(station) ? 20000 : 0) + (safeHttp(station.homepage) ? 10000 : 0) +
    (safeHttp(station.url_resolved) ? 5000 : 0) + Math.min(512, Math.max(0, Number(station.bitrate) || 0));

  function normalize(raw) {
    const s = {
      stationuuid: clean(raw?.stationuuid),
      name: clean(raw?.name),
      url: clean(raw?.url),
      url_resolved: clean(raw?.url_resolved),
      homepage: clean(raw?.homepage),
      favicon: clean(raw?.favicon),
      tags: clean(raw?.tags),
      country: clean(raw?.country),
      countrycode: clean(raw?.countrycode).toUpperCase(),
      sourcecountrycode: clean(raw?.sourcecountrycode).toUpperCase(),
      state: clean(raw?.state),
      language: clean(raw?.language),
      codec: clean(raw?.codec),
      votes: Math.max(0, Number(raw?.votes) || 0),
      bitrate: Math.max(0, Number(raw?.bitrate) || 0),
      lastcheckok: Number(raw?.lastcheckok) || 0
    };
    s.logo = logo(s);
    return s;
  }

  function merge(a, b) {
    let primary = quality(b) > quality(a) ? { ...b } : { ...a };
    const other = primary.stationuuid === b.stationuuid && primary.name === b.name ? a : b;
    for (const key of ['stationuuid','name','url','url_resolved','homepage','favicon','tags','country','countrycode','sourcecountrycode','state','language','codec']) {
      if (!clean(primary[key])) primary[key] = other[key] || '';
    }
    primary.votes = Math.max(+primary.votes || 0, +other.votes || 0);
    primary.bitrate = Math.max(+primary.bitrate || 0, +other.bitrate || 0);
    primary.lastcheckok = Math.max(+primary.lastcheckok || 0, +other.lastcheckok || 0);
    return normalize(primary);
  }

  function dedupe(list) {
    const out = [];
    const byUuid = new Map();
    const byIdentity = new Map();
    for (const raw of list || []) {
      const station = normalize(raw);
      if (!ALLOWED.has(station.countrycode) || !station.name || (!safeHttp(station.url) && !safeHttp(station.url_resolved))) continue;
      const id = identity(station);
      const uuid = station.stationuuid;
      let pos = uuid ? byUuid.get(uuid) : undefined;
      if (pos === undefined && id) pos = byIdentity.get(id);
      if (pos !== undefined) {
        out[pos] = merge(out[pos], station);
        const merged = out[pos];
        if (merged.stationuuid) byUuid.set(merged.stationuuid, pos);
        const mergedId = identity(merged);
        if (mergedId) byIdentity.set(mergedId, pos);
        continue;
      }
      if (out.length >= MAX_CATALOG) break;
      pos = out.length;
      out.push(station);
      if (uuid) byUuid.set(uuid, pos);
      if (id) byIdentity.set(id, pos);
    }
    return out;
  }

  async function bases() {
    try {
      const response = await fetchWithTimeout('https://all.api.radio-browser.info/json/servers', { cache: 'no-store', redirect: 'error' }, 6500);
      if (response.ok) {
        const body = await response.json();
        const dynamic = body.map(x => safeApiBase(x?.name)).filter(Boolean);
        if (dynamic.length) return [...new Set([...dynamic, ...API_FALLBACKS])];
      }
    } catch { }
    return API_FALLBACKS;
  }

  async function fetchCountry(base, code) {
    const rows = [];
    for (let offset = 0; offset < MAX_PER_COUNTRY; offset += PAGE) {
      const url = `${base}/json/stations/search?countrycode=${encodeURIComponent(code)}&hidebroken=true&order=votes&reverse=true&limit=${PAGE}&offset=${offset}`;
      const response = await fetchWithTimeout(url, { cache: 'no-store', redirect: 'error' }, 10000);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const page = await response.json();
      if (!Array.isArray(page) || !page.length) break;
      let accepted = 0;
      for (const raw of page) {
        const station = normalize(raw);
        const actual = station.countrycode || code;
        if (actual !== code) continue;
        station.countrycode = code;
        if (!station.country) station.country = BALKAN_COUNTRIES.find(x => x[0] === code)?.[1] || code;
        rows.push(station);
        accepted++;
      }
      if (page.length < PAGE || accepted === 0) break;
    }
    return dedupe(rows);
  }

  async function fetchCountryFromAny(code, serverList) {
    let lastError;
    for (const base of serverList) {
      try {
        const list = await fetchCountry(base, code);
        if (list.length) return list;
      } catch (error) { lastError = error; }
    }
    throw lastError || new Error(`Katalog ${code} nije dostupan`);
  }

  async function fetchForeignFromAny(serverList) {
    let lastError;
    for (const base of serverList) {
      try {
        const url = `${base}/json/stations/search?hidebroken=true&order=votes&reverse=true&limit=${FOREIGN_SCAN_LIMIT}`;
        const response = await fetchWithTimeout(url, { cache: 'no-store', redirect: 'error' }, 11000);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        const page = await response.json();
        if (!Array.isArray(page)) throw new Error('Neispravan strani katalog');
        const mapped = [];
        for (const raw of page) {
          const station = normalize(raw);
          const sourceCode = station.countrycode;
          if (!sourceCode || BALKAN_ALLOWED.has(sourceCode) || station.lastcheckok !== 1) continue;
          if (!station.name || (!safeHttp(station.url) && !safeHttp(station.url_resolved))) continue;
          station.sourcecountrycode = sourceCode;
          station.countrycode = FOREIGN_CODE;
          if (!station.country) station.country = 'Strana postaja';
          mapped.push(station);
        }
        const unique = dedupe(mapped)
          .sort((a, b) => b.votes - a.votes || quality(b) - quality(a) || a.name.localeCompare(b.name))
          .slice(0, MAX_FOREIGN);
        if (unique.length) return unique;
      } catch (error) { lastError = error; }
    }
    throw lastError || new Error('Strani katalog nije dostupan');
  }

  async function storageGet(keys) {
    const api = ext.storage.local;
    try {
      const result = api.get(keys);
      if (result && typeof result.then === 'function') return await result;
    } catch { }
    return await new Promise((resolve, reject) => {
      try { api.get(keys, value => ext.runtime?.lastError ? reject(new Error(ext.runtime.lastError.message)) : resolve(value || {})); }
      catch (error) { reject(error); }
    });
  }

  async function storageSet(values) {
    const api = ext.storage.local;
    try {
      const result = api.set(values);
      if (result && typeof result.then === 'function') {
        await result;
        return;
      }
    } catch { }
    await new Promise((resolve, reject) => {
      try { api.set(values, () => ext.runtime?.lastError ? reject(new Error(ext.runtime.lastError.message)) : resolve()); }
      catch (error) { reject(error); }
    });
  }

  function mergeMissingCountries(online, cached) {
    const onlineList = dedupe(online);
    const cacheList = dedupe(cached);
    const present = new Set(onlineList.map(x => x.countrycode));
    const merged = [...onlineList];
    for (const [code] of COUNTRIES) {
      if (!present.has(code)) merged.push(...cacheList.filter(x => x.countrycode === code));
    }
    const regional = dedupe(merged.filter(x => x.countrycode !== FOREIGN_CODE))
      .sort((a, b) => BALKAN_COUNTRIES.findIndex(x => x[0] === a.countrycode) - BALKAN_COUNTRIES.findIndex(x => x[0] === b.countrycode) || b.votes - a.votes || a.name.localeCompare(b.name))
      .slice(0, MAX_CATALOG - MAX_FOREIGN);
    const foreign = dedupe(merged.filter(x => x.countrycode === FOREIGN_CODE))
      .sort((a, b) => b.votes - a.votes || a.name.localeCompare(b.name))
      .slice(0, MAX_FOREIGN);
    return [...regional, ...foreign];
  }

  async function freshCatalog(cached) {
    const serverList = await bases();
    const results = new Map();
    let cursor = 0;
    async function worker() {
      while (true) {
        const index = cursor++;
        if (index >= BALKAN_COUNTRIES.length) return;
        const code = BALKAN_COUNTRIES[index][0];
        try { results.set(code, await fetchCountryFromAny(code, serverList)); }
        catch { results.set(code, []); }
      }
    }
    const foreignPromise = fetchForeignFromAny(serverList).catch(() => []);
    await Promise.all([worker(), worker()]);
    let online = [];
    for (const [code] of BALKAN_COUNTRIES) online.push(...(results.get(code) || []));
    online.push(...await foreignPromise);
    const merged = mergeMissingCountries(online, cached);
    const regionalCount = merged.filter(x => x.countrycode !== FOREIGN_CODE).length;
    if (regionalCount < 600 && cached.length > merged.length) return mergeMissingCountries([...merged, ...cached], cached);
    return merged;
  }

  async function load(force = false) {
    const cachedState = await storageGet(['rbCatalog','rbCatalogAt']);
    const cached = dedupe(Array.isArray(cachedState.rbCatalog) ? cachedState.rbCatalog : []);
    if (!force && cached.length && Date.now() - (+cachedState.rbCatalogAt || 0) < CACHE_MS) return cached;
    try {
      const list = await freshCatalog(cached);
      if (!list.length) throw new Error('Katalog nije dostupan');
      await storageSet({ rbCatalog: list, rbCatalogAt: Date.now() });
      return list;
    } catch (error) {
      if (cached.length) return cached;
      throw error;
    }
  }

  async function favorites() {
    const x = await storageGet(['rbFavorites']);
    return x.rbFavorites || {};
  }

  async function setFavorite(key, on) {
    const f = await favorites();
    if (on) f[key] = true;
    else delete f[key];
    await storageSet({ rbFavorites: f });
    return f;
  }

  function key(station) {
    return station.stationuuid || identity(station) || `${station.countrycode}|${fold(station.name)}|${station.url_resolved || station.url}`;
  }

  return {
    load, favorites, setFavorite, key, fold, safeHttp, ext,
    COUNTRIES, BALKAN_COUNTRIES, ALLOWED, BALKAN_ALLOWED, FOREIGN_CODE, MAX_FOREIGN
  };
})();
