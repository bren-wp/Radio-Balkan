const RB = (() => {
  const API_FALLBACKS = [
    'https://de1.api.radio-browser.info',
    'https://de2.api.radio-browser.info',
    'https://at1.api.radio-browser.info',
    'https://nl1.api.radio-browser.info'
  ];
  const BALKAN_COUNTRIES = [
    ['HR','Hrvatska'], ['BA','Bosna i Hercegovina'], ['RS','Srbija'],
    ['SI','Slovenija'], ['MK','Sjeverna Makedonija'], ['AL','Albanija'], ['ME','Crna Gora'], ['BG','Bugarska']
  ];
  const FOREIGN_CODE = 'INT';
  const DIASPORA_CODE = 'DIA';
  const COUNTRIES = [...BALKAN_COUNTRIES, [DIASPORA_CODE, 'Dijaspora'], [FOREIGN_CODE, 'Strano']];
  const BALKAN_ALLOWED = new Set(BALKAN_COUNTRIES.map(x => x[0]));
  const ALLOWED = new Set(COUNTRIES.map(x => x[0]));
  const PAGE = 200;
  const MAX_PER_COUNTRY = 1600;
  const MAX_FOREIGN = 180;
  const MAX_DIASPORA = 180;
  const FOREIGN_SCAN_LIMIT = 1600;
  const DIASPORA_QUERY_LIMIT = 140;
  const MAX_CATALOG = 7360;
  const CACHE_MS = 12 * 60 * 60 * 1000;
  const MAX_RECENT = 50;
  const MAX_SERVER_RESPONSE_BYTES = 512 * 1024;
  const MAX_CATALOG_RESPONSE_BYTES = 8 * 1024 * 1024;
  const MAX_DISCOVERED_API_BASES = 4;
  const MAX_API_BASES = 8;
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

  async function readJsonLimited(response, maxBytes) {
    if (!response || !Number.isFinite(maxBytes) || maxBytes <= 0) throw new Error('Neispravan limit odgovora');
    const declaredRaw = response.headers?.get?.('content-length');
    const declared = Number(declaredRaw);
    if (declaredRaw && Number.isFinite(declared) && declared > maxBytes) {
      throw new Error('API odgovor je prevelik');
    }

    if (!response.body?.getReader) throw new Error('Streaming API odgovor nije dostupan');
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    const parts = [];
    let total = 0;
    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        total += value?.byteLength || 0;
        if (total > maxBytes) {
          try { await reader.cancel(); } catch { }
          throw new Error('API odgovor je prevelik');
        }
        parts.push(decoder.decode(value, { stream: true }));
      }
      parts.push(decoder.decode());
    } finally {
      try { reader.releaseLock?.(); } catch { }
    }
    return JSON.parse(parts.join(''));
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
    const catalogCode = clean(station.countrycode).toUpperCase();
    if (!name || !ALLOWED.has(catalogCode)) return '';
    const sourceCode = clean(station.sourcecountrycode).toUpperCase();
    const country = (catalogCode === DIASPORA_CODE || catalogCode === FOREIGN_CODE) && sourceCode ? sourceCode : catalogCode;
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
    if (a.countrycode === DIASPORA_CODE || b.countrycode === DIASPORA_CODE) {
      primary.countrycode = DIASPORA_CODE;
      primary.sourcecountrycode = a.sourcecountrycode || b.sourcecountrycode || '';
      if (!String(primary.tags || '').toLowerCase().includes('dijaspora')) {
        primary.tags = primary.tags ? `${primary.tags},dijaspora` : 'dijaspora';
      }
    }
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
        const body = await readJsonLimited(response, MAX_SERVER_RESPONSE_BYTES);
        if (!Array.isArray(body)) throw new Error('Neispravan popis API servera');
        const dynamic = [];
        for (const row of body) {
          const candidate = safeApiBase(row?.name);
          if (!candidate || API_FALLBACKS.includes(candidate) || dynamic.includes(candidate)) continue;
          dynamic.push(candidate);
          if (dynamic.length >= MAX_DISCOVERED_API_BASES) break;
        }
        return [...new Set([...dynamic, ...API_FALLBACKS])].slice(0, MAX_API_BASES);
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
      const page = await readJsonLimited(response, MAX_CATALOG_RESPONSE_BYTES);
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
        const page = await readJsonLimited(response, MAX_CATALOG_RESPONSE_BYTES);
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

  async function fetchDiasporaFromAny(serverList) {
    const queries = [
      ['tag', 'diaspora'], ['tag', 'balkan'], ['tag', 'exyu'],
      ['name', 'balkan'], ['name', 'ex yu'], ['name', 'radio diaspora'],
      ['language', 'croatian'], ['language', 'serbian'], ['language', 'bosnian'],
      ['language', 'macedonian'], ['language', 'albanian'], ['language', 'slovenian'],
      ['language', 'bulgarian']
    ];
    let lastError;
    for (const base of serverList) {
      try {
        const mapped = [];
        for (let offset = 0; offset < queries.length; offset += 3) {
          const batch = queries.slice(offset, offset + 3);
          const pages = await Promise.allSettled(batch.map(async ([field, value]) => {
            const requestUrl = `${base}/json/stations/search?${field}=${encodeURIComponent(value)}&hidebroken=true&order=votes&reverse=true&limit=${DIASPORA_QUERY_LIMIT}`;
            const response = await fetchWithTimeout(requestUrl, { cache: 'no-store', redirect: 'error' }, 9000);
            if (!response.ok) throw new Error(`HTTP ${response.status}`);
            const page = await readJsonLimited(response, 3 * 1024 * 1024);
            return Array.isArray(page) ? page : [];
          }));
          for (const result of pages) {
            if (result.status !== 'fulfilled') {
              lastError = result.reason instanceof Error ? result.reason : new Error('Diaspora upit nije uspio');
              continue;
            }
            for (const raw of result.value) {
              const station = normalize(raw);
              const sourceCode = station.countrycode;
              if (!sourceCode || BALKAN_ALLOWED.has(sourceCode) || station.lastcheckok !== 1) continue;
              if (!station.name || (!safeHttp(station.url) && !safeHttp(station.url_resolved))) continue;
              station.sourcecountrycode = sourceCode;
              station.countrycode = DIASPORA_CODE;
              station.tags = station.tags ? `${station.tags},dijaspora` : 'dijaspora';
              if (!station.country) station.country = 'Dijaspora';
              mapped.push(station);
            }
          }
        }
        const unique = dedupe(mapped)
          .sort((a, b) => b.votes - a.votes || quality(b) - quality(a) || a.name.localeCompare(b.name))
          .slice(0, MAX_DIASPORA);
        if (unique.length) return unique;
      } catch (error) { lastError = error; }
    }
    throw lastError || new Error('Katalog dijaspore nije dostupan');
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
    const regional = dedupe(merged.filter(x => BALKAN_ALLOWED.has(x.countrycode)))
      .sort((a, b) => BALKAN_COUNTRIES.findIndex(x => x[0] === a.countrycode) - BALKAN_COUNTRIES.findIndex(x => x[0] === b.countrycode) || b.votes - a.votes || a.name.localeCompare(b.name))
      .slice(0, MAX_CATALOG - MAX_FOREIGN - MAX_DIASPORA);
    const diaspora = dedupe(merged.filter(x => x.countrycode === DIASPORA_CODE))
      .sort((a, b) => b.votes - a.votes || a.name.localeCompare(b.name))
      .slice(0, MAX_DIASPORA);
    const foreign = dedupe(merged.filter(x => x.countrycode === FOREIGN_CODE))
      .sort((a, b) => b.votes - a.votes || a.name.localeCompare(b.name))
      .slice(0, MAX_FOREIGN);
    return [...regional, ...diaspora, ...foreign];
  }

  async function freshCatalog(cached) {
    const serverList = await bases();
    const results = new Map();
    let cursor = 0;
    let successfulBatches = 0;
    async function worker() {
      while (true) {
        const index = cursor++;
        if (index >= BALKAN_COUNTRIES.length) return;
        const code = BALKAN_COUNTRIES[index][0];
        try {
          results.set(code, await fetchCountryFromAny(code, serverList));
          successfulBatches += 1;
        } catch {
          results.set(code, []);
        }
      }
    }
    const foreignPromise = fetchForeignFromAny(serverList)
      .then(list => { successfulBatches += 1; return list; })
      .catch(() => []);
    const diasporaPromise = fetchDiasporaFromAny(serverList)
      .then(list => { successfulBatches += 1; return list; })
      .catch(() => []);
    await Promise.all([worker(), worker()]);
    let online = [];
    for (const [code] of BALKAN_COUNTRIES) online.push(...(results.get(code) || []));
    online.push(...await diasporaPromise);
    online.push(...await foreignPromise);
    if (successfulBatches === 0) throw new Error('Radio Browser trenutačno nije dostupan');
    const merged = mergeMissingCountries(online, cached);
    const regionalCount = merged.filter(x => BALKAN_ALLOWED.has(x.countrycode)).length;
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
      if (cached.length && !force) return cached;
      throw error;
    }
  }

  async function uiPreferences() {
    const x = await storageGet(['rbUiPrefs']);
    return x.rbUiPrefs && typeof x.rbUiPrefs === 'object' ? x.rbUiPrefs : {};
  }

  async function setUiPreferences(value) {
    const safe = value && typeof value === 'object' ? value : {};
    await storageSet({ rbUiPrefs: {
      country: clean(safe.country).toUpperCase().slice(0, 3),
      genre: clean(safe.genre).toLowerCase().slice(0, 32),
      favoritesOnly: !!safe.favoritesOnly
    }});
  }

  async function recent() {
    const x = await storageGet(['rbRecent']);
    const raw = Array.isArray(x.rbRecent) ? x.rbRecent : [];
    const unique = [];
    const seen = new Set();
    for (const value of raw) {
      const item = clean(value).slice(0, 512);
      if (!item || seen.has(item)) continue;
      seen.add(item);
      unique.push(item);
      if (unique.length >= MAX_RECENT) break;
    }
    return unique;
  }

  async function addRecent(stationKey) {
    stationKey = clean(stationKey).slice(0, 512);
    if (!stationKey) return await recent();
    const list = await recent();
    const next = [stationKey, ...list.filter(value => value !== stationKey)].slice(0, MAX_RECENT);
    await storageSet({ rbRecent: next });
    return next;
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

  async function adminOverrideFor(stationKey) {
    stationKey = clean(stationKey).slice(0, 512);
    if (!stationKey) return '';
    const x = await storageGet(['rbAdminOverrides']);
    const map = x.rbAdminOverrides && typeof x.rbAdminOverrides === 'object' ? x.rbAdminOverrides : {};
    const value = clean(map[stationKey]);
    return safeHttp(value) ? value : '';
  }

  async function setAdminOverride(stationKey, value) {
    stationKey = clean(stationKey).slice(0, 512);
    if (!stationKey) throw new Error('Stanica nije odabrana');
    value = clean(value);
    const x = await storageGet(['rbAdminOverrides']);
    const map = x.rbAdminOverrides && typeof x.rbAdminOverrides === 'object' ? { ...x.rbAdminOverrides } : {};
    if (!value) delete map[stationKey];
    else {
      if (!safeHttp(value)) throw new Error('Izvor mora biti sigurna javna http/https poveznica');
      map[stationKey] = value;
    }
    const keys = Object.keys(map);
    if (keys.length > 512) {
      for (const oldKey of keys.slice(0, keys.length - 512)) delete map[oldKey];
    }
    await storageSet({ rbAdminOverrides: map });
    return value;
  }

  function key(station) {
    return station.stationuuid || identity(station) || `${station.countrycode}|${fold(station.name)}|${station.url_resolved || station.url}`;
  }

  return {
    load, favorites, setFavorite, recent, addRecent, uiPreferences, setUiPreferences, adminOverrideFor, setAdminOverride, key, fold, safeHttp, ext,
    COUNTRIES, BALKAN_COUNTRIES, ALLOWED, BALKAN_ALLOWED, FOREIGN_CODE, DIASPORA_CODE, MAX_FOREIGN, MAX_DIASPORA
  };
})();
