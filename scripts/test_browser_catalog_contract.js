'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const storage = {};
const balkanCodes = new Set(['HR', 'BA', 'RS', 'SI', 'MK', 'AL', 'ME']);

function response(body, ok = true, status = 200, extraHeaders = {}) {
  const payload = JSON.stringify(body);
  const bytes = new TextEncoder().encode(payload);
  return {
    ok,
    status,
    headers: {
      get(name) {
        const key = String(name || '').toLowerCase();
        if (key === 'content-length' && Object.prototype.hasOwnProperty.call(extraHeaders, key)) return String(extraHeaders[key]);
        return null;
      }
    },
    body: {
      getReader() {
        let sent = false;
        return {
          async read() {
            if (sent) return { done: true, value: undefined };
            sent = true;
            return { done: false, value: bytes };
          },
          async cancel() { sent = true; },
          releaseLock() {}
        };
      }
    },
    async text() { return payload; }
  };
}

let oversizedCountry = '';
let networkOffline = false;
const attemptedDynamicHosts = new Set();

function station(index, code, lastcheckok = 1, url = `https://stream${index}.example.com/live`) {
  return {
    stationuuid: `station-${code}-${index}`,
    name: `Station ${code} ${index}`,
    url,
    url_resolved: url,
    homepage: `https://radio${index}.example.com`,
    favicon: '',
    tags: index % 2 ? 'pop,hits' : 'rock',
    country: code === 'US' ? 'United States' : code === 'GB' ? 'United Kingdom' : code,
    countrycode: code,
    state: '',
    language: 'English',
    codec: 'MP3',
    votes: 1000 - index,
    bitrate: 128,
    lastcheckok
  };
}

async function mockFetch(raw) {
  const url = String(raw);
  if (url === 'https://all.api.radio-browser.info/json/servers') {
    const rows = Array.from({ length: 20 }, (_, i) => ({ name: `dyn${i}.api.radio-browser.info` }));
    rows.push({ name: 'de1.api.radio-browser.info' });
    return response(rows);
  }
  if (url.includes('/json/stations/search?countrycode=')) {
    if (networkOffline) throw new Error('simulated offline catalog');
    const parsed = new URL(url);
    if (parsed.hostname.startsWith('dyn')) {
      attemptedDynamicHosts.add(parsed.hostname);
      throw new Error('simulated dynamic API failure');
    }
    const code = parsed.searchParams.get('countrycode');
    if (code === oversizedCountry) {
      return response([station(1, code)], true, 200, { 'content-length': 8 * 1024 * 1024 + 1 });
    }
    return response([station(1, code)]);
  }
  if (url.includes('/json/stations/search?') && url.includes('hidebroken=true') && url.includes('limit=100') &&
      (url.includes('tag=diaspora') || url.includes('name=balkan') || url.includes('name=ex%20yu') ||
       url.includes('language=croatian') || url.includes('language=serbian') || url.includes('language=bosnian') ||
       url.includes('language=macedonian') || url.includes('language=albanian') || url.includes('language=slovenian'))) {
    if (networkOffline) throw new Error('simulated offline diaspora catalog');
    const parsed = new URL(url);
    if (parsed.hostname.startsWith('dyn')) {
      attemptedDynamicHosts.add(parsed.hostname);
      throw new Error('simulated dynamic API failure');
    }
    const signature = Array.from(parsed.searchParams.entries()).find(([key]) => ['tag', 'name', 'language'].includes(key))?.join(':') || 'diaspora';
    const seed = Array.from(signature).reduce((sum, char) => sum + char.charCodeAt(0), 0);
    const diaspora = [];
    for (let i = 0; i < 28; i += 1) {
      const item = station(seed * 100 + i, i % 2 ? 'DE' : 'AT');
      item.name = `Diaspora ${signature} ${i}`;
      item.country = i % 2 ? 'Germany' : 'Austria';
      item.language = 'Croatian';
      diaspora.push(item);
    }
    diaspora.push(station(seed * 100 + 90, 'HR'));
    diaspora.push(station(seed * 100 + 91, 'CH', 0));
    diaspora.push(station(seed * 100 + 92, 'DE', 1, 'http://127.0.0.1/private'));
    return response(diaspora);
  }
  if (url.includes('/json/stations/search?hidebroken=true&order=votes&reverse=true&limit=1000')) {
    if (networkOffline) throw new Error('simulated offline catalog');
    const parsed = new URL(url);
    if (parsed.hostname.startsWith('dyn')) {
      attemptedDynamicHosts.add(parsed.hostname);
      throw new Error('simulated dynamic API failure');
    }
    const foreign = [];
    for (let i = 0; i < 150; i += 1) foreign.push(station(i, i % 2 ? 'US' : 'GB'));
    foreign.push(station(500, 'HR'));
    foreign.push(station(501, 'US', 0));
    foreign.push(station(502, 'DE', 1, 'http://localhost./private'));
    return response(foreign);
  }
  throw new Error(`Unexpected fetch: ${url}`);
}

const context = {
  console,
  URL,
  AbortController,
  TextEncoder,
  TextDecoder,
  setTimeout,
  clearTimeout,
  fetch: mockFetch,
  browser: {
    storage: {
      local: {
        async get(keys) {
          const out = {};
          for (const key of keys) out[key] = storage[key];
          return out;
        },
        async set(values) { Object.assign(storage, values); }
      }
    },
    runtime: {}
  },
  RBNet: {
    safeHttp(raw) {
      try {
        const u = new URL(String(raw));
        const host = u.hostname.toLowerCase().replace(/\.+$/, '');
        if (!['http:', 'https:'].includes(u.protocol) || u.username || u.password) return false;
        return host !== 'localhost' && host !== '127.0.0.1' && !host.endsWith('.local');
      } catch { return false; }
    },
    safeRadioBrowserBase(name) {
      const host = String(name || '').toLowerCase().replace(/\.+$/, '');
      return host.endsWith('.api.radio-browser.info') ? `https://${host}` : '';
    }
  }
};
context.globalThis = context;
vm.createContext(context);

async function main() {
  const source = fs.readFileSync(path.join(__dirname, '..', 'extensions', 'shared', 'catalog.js'), 'utf8');
  vm.runInContext(source, context, { filename: 'catalog.js' });
  const RB = vm.runInContext('RB', context);

  assert.equal(RB.FOREIGN_CODE, 'INT');
  assert.equal(RB.DIASPORA_CODE, 'DIA');
  assert.ok(RB.COUNTRIES.some(([code, name]) => code === 'INT' && name === 'Strano'));
  assert.ok(RB.COUNTRIES.some(([code, name]) => code === 'DIA' && name === 'Dijaspora'));

  const catalog = await RB.load(true);
  const foreign = catalog.filter(item => item.countrycode === RB.FOREIGN_CODE);
  const diaspora = catalog.filter(item => item.countrycode === RB.DIASPORA_CODE);
  const regional = catalog.filter(item => balkanCodes.has(item.countrycode));

  assert.equal(foreign.length, 120, 'foreign catalog must be capped at 120 stations');
  assert.equal(diaspora.length, 120, 'diaspora catalog must be capped at 120 stations');
  assert.equal(regional.length, 7, 'regional catalog must retain all seven Balkan country batches');
  assert.ok(diaspora.every(item => item.lastcheckok === 1), 'diaspora catalog must keep only healthy Radio Browser entries');
  assert.ok(diaspora.every(item => !balkanCodes.has(item.sourcecountrycode)), 'diaspora catalog must represent stations hosted outside supported Balkan countries');
  assert.ok(diaspora.every(item => String(item.tags || '').toLowerCase().includes('dijaspora')), 'diaspora stations must keep their explicit classification');
  assert.ok(diaspora.every(item => /^https?:\/\//.test(item.url_resolved || item.url)), 'diaspora catalog must keep safe HTTP(S) streams');
  assert.ok(foreign.every(item => item.lastcheckok === 1), 'foreign catalog must keep only healthy Radio Browser entries');
  assert.ok(foreign.every(item => !balkanCodes.has(item.sourcecountrycode)), 'foreign catalog must exclude supported Balkan countries');
  assert.ok(foreign.every(item => /^https?:\/\//.test(item.url_resolved || item.url)), 'foreign catalog must keep safe HTTP(S) streams');
  for (let i = 1; i < foreign.length; i += 1) {
    assert.ok(foreign[i - 1].votes >= foreign[i].votes, 'foreign stations must remain sorted by popularity');
  }
  assert.ok(!foreign.some(item => String(item.url).includes('localhost')), 'unsafe local targets must be rejected');
  assert.equal(attemptedDynamicHosts.size, 4, 'dynamic API discovery must be capped before stable fallbacks are tried');

  delete storage.rbCatalog;
  delete storage.rbCatalogAt;
  oversizedCountry = 'HR';
  const limitedCatalog = await RB.load(true);
  const limitedRegional = limitedCatalog.filter(item => balkanCodes.has(item.countrycode));
  assert.equal(limitedRegional.length, 6, 'oversized country response must be rejected without poisoning other country batches');
  assert.ok(!limitedRegional.some(item => item.countrycode === 'HR'), 'oversized response must not be parsed into the catalog');
  oversizedCountry = '';

  await RB.setUiPreferences({ country: ' hr ', genre: 'POP', favoritesOnly: 1 });
  const savedPreferences = JSON.parse(JSON.stringify(await RB.uiPreferences()));
  assert.deepEqual(savedPreferences, { country: 'HR', genre: 'pop', favoritesOnly: true }, 'UI preferences must be sanitized and persisted');

  networkOffline = true;
  await assert.rejects(() => RB.load(true), /Radio Browser trenutačno nije dostupan/, 'forced refresh must surface a real network failure');
  const cachedWhileOffline = await RB.load(false);
  assert.ok(cachedWhileOffline.length > 0, 'normal load may still fall back to a fresh cached catalog while offline');
  networkOffline = false;

  console.log('Browser regional, diaspora, foreign, response-limit, refresh-state and UI preference contracts OK');
}

main().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
