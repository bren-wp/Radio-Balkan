'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const encoder = new TextEncoder();
let fetchHandler = async () => { throw new Error('unexpected fetch'); };
const context = {
  URL, AbortController, TextDecoder, setTimeout, clearTimeout,
  fetch(...args) { return fetchHandler(...args); }
};
context.globalThis = context;
vm.createContext(context);

const networkPath = path.join(__dirname, '..', 'extensions', 'shared', 'network.js');
const source = fs.readFileSync(networkPath, 'utf8');
vm.runInContext(`${source}\nthis.__RBNet = RBNet;`, context, { filename: networkPath });
const RBNet = context.__RBNet;

assert.equal(RBNet.safeHttp('https://example.com/live'), true, 'public HTTPS target must remain allowed');
assert.equal(RBNet.safeHttp('http://127.0.0.1/live'), false, 'IPv4 loopback must be rejected');
assert.equal(RBNet.safeHttp('http://10.0.0.4/live'), false, 'private IPv4 must be rejected');
assert.equal(RBNet.safeHttp('http://169.254.169.254/latest/meta-data/'), false, 'link-local metadata target must be rejected');
assert.equal(RBNet.safeHttp('https://user:pass@example.com/live'), false, 'credentialed URL must be rejected');
assert.equal(RBNet.safeHttp('http://[::]/live'), false, 'IPv6 unspecified target must be rejected');
assert.equal(RBNet.safeHttp('http://[::1]/live'), false, 'IPv6 loopback must be rejected');
assert.equal(RBNet.safeHttp('http://[::ffff:127.0.0.1]/live'), false, 'IPv4-mapped IPv6 loopback must be rejected');
assert.equal(RBNet.safeHttp('http://[::ffff:10.0.0.4]/live'), false, 'IPv4-mapped IPv6 private target must be rejected');
assert.equal(RBNet.safeHttp('http://[fc00::1]/live'), false, 'IPv6 unique-local target must be rejected');
assert.equal(RBNet.safeHttp('http://[fd12:3456::1]/live'), false, 'IPv6 unique-local fd00/8 target must be rejected');
assert.equal(RBNet.safeHttp('http://[fe90::1]/live'), false, 'full IPv6 link-local fe80/10 range must be rejected');
assert.equal(RBNet.safeHttp('http://[ff02::1]/live'), false, 'IPv6 multicast target must be rejected');
assert.equal(RBNet.safeHttp('http://localhost./live'), false, 'trailing-dot localhost must be rejected');
assert.equal(RBNet.safeHttp('https://metadata.google.internal./computeMetadata/v1/'), false, 'trailing-dot metadata host must be rejected');
assert.equal(RBNet.safeHttp('http://radio.local./live'), false, 'trailing-dot local-domain target must be rejected');

assert.equal(RBNet.allowedCountry('HR'), true, 'supported Balkan station must remain playable');
assert.equal(RBNet.allowedCountry('BG'), true, 'Bulgarian Balkan station must remain playable');
assert.equal(RBNet.allowedCountry('INT'), true, 'curated foreign group must be playable');
assert.equal(RBNet.allowedCountry('US'), false, 'arbitrary external country code must not bypass curated foreign selection');
assert.equal(RBNet.validStation({ countrycode: 'INT', url: 'https://example.com/live' }), true, 'curated foreign station with a safe stream must be accepted');
assert.equal(RBNet.validStation({ countrycode: 'US', url: 'https://example.com/live' }), false, 'unmapped foreign station must still be rejected');
assert.equal(RBNet.validStation({ countrycode: 'INT', url: 'http://localhost./live' }), false, 'foreign grouping must never weaken URL safety');

assert.equal(RBNet.safeRadioBrowserBase('de1.api.radio-browser.info'), 'https://de1.api.radio-browser.info', 'trusted Radio Browser host must be accepted');
assert.equal(RBNet.safeRadioBrowserBase('DE2.API.RADIO-BROWSER.INFO.'), 'https://de2.api.radio-browser.info', 'trusted host normalization must be deterministic');
assert.equal(RBNet.safeRadioBrowserBase('de2.api.radio-browser.info..'), 'https://de2.api.radio-browser.info', 'multiple trailing dots must canonicalize deterministically');
assert.equal(RBNet.safeRadioBrowserBase('localhost'), '', 'catalog discovery must reject localhost');
assert.equal(RBNet.safeRadioBrowserBase('127.0.0.1'), '', 'catalog discovery must reject loopback literals');
assert.equal(RBNet.safeRadioBrowserBase('example.com'), '', 'catalog discovery must reject unrelated public origins');
assert.equal(RBNet.safeRadioBrowserBase('de1.api.radio-browser.info.evil.example'), '', 'catalog discovery must reject suffix-confusion hosts');
assert.equal(RBNet.safeRadioBrowserBase('user@de1.api.radio-browser.info'), '', 'catalog discovery must reject credential-like input');

const catalogPath = path.join(__dirname, '..', 'extensions', 'shared', 'catalog.js');
const catalogSource = fs.readFileSync(catalogPath, 'utf8');
assert.match(catalogSource, /RBNet\.safeRadioBrowserBase/, 'catalog discovery must use the trusted Radio Browser origin validator');
assert.match(catalogSource, /redirect:\s*'error'/, 'catalog API fetches must reject redirects instead of following a server-controlled hop');

function responseJson(value, status = 200, declaredLength = '') {
  const bytes = encoder.encode(JSON.stringify(value));
  let sent = false;
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: { get(name) { return name.toLowerCase() === 'content-length' ? declaredLength : null; } },
    body: {
      getReader() {
        return {
          async read() {
            if (sent) return { done: true, value: undefined };
            sent = true;
            return { done: false, value: bytes };
          },
          async cancel() {},
          releaseLock() {}
        };
      }
    }
  };
}

async function testRefresh() {
  const calls = [];
  fetchHandler = async url => {
    calls.push(String(url));
    return responseJson([{
      stationuuid: 'abc-123',
      countrycode: 'HR',
      url_resolved: 'https://fresh.example.com/live',
      url: 'http://127.0.0.1/private'
    }]);
  };
  const regional = await RBNet.refreshCandidateUrls({ stationuuid: 'abc-123', countrycode: 'HR' });
  assert.deepEqual(Array.from(regional), ['https://fresh.example.com/live'], 'UUID refresh must return only safe public streams');
  assert.match(calls[0], /\.api\.radio-browser\.info\/json\/stations\/byuuid\/abc-123$/, 'UUID refresh must use a fixed Radio Browser API host');

  fetchHandler = async () => responseJson([{
    stationuuid: 'abc-123',
    countrycode: 'RS',
    url_resolved: 'https://wrong-country.example.com/live'
  }]);
  const mismatched = await RBNet.refreshCandidateUrls({ stationuuid: 'abc-123', countrycode: 'HR' });
  assert.deepEqual(Array.from(mismatched), [], 'UUID refresh must reject a regional station returned under another country');

  fetchHandler = async () => responseJson([{
    stationuuid: 'bg-1',
    countrycode: 'BG',
    url_resolved: 'https://bulgaria.example.com/live'
  }]);
  const bulgaria = await RBNet.refreshCandidateUrls({ stationuuid: 'bg-1', countrycode: 'BG' });
  assert.deepEqual(Array.from(bulgaria), ['https://bulgaria.example.com/live'], 'Bulgarian regional station must pass exact-country refresh validation');

  fetchHandler = async () => responseJson([{
    stationuuid: 'world-1',
    countrycode: 'DE',
    url_resolved: 'https://world.example.com/live'
  }]);
  const foreign = await RBNet.refreshCandidateUrls({ stationuuid: 'world-1', countrycode: 'INT', sourcecountrycode: 'DE' });
  assert.deepEqual(Array.from(foreign), ['https://world.example.com/live'], 'curated foreign station may refresh only within its source country');

  assert.equal(RBNet.allowedCountry('DIA'), true, 'diaspora pseudo-country must be accepted by browser playback validation');
  assert.deepEqual(
    Array.from(RBNet.candidateUrls({ countrycode: 'DIA', url_resolved: 'https://diaspora.example.com/live' })),
    ['https://diaspora.example.com/live'],
    'diaspora stations must expose safe playback candidates'
  );
  const diaspora = await RBNet.refreshCandidateUrls({ stationuuid: 'world-1', countrycode: 'DIA', sourcecountrycode: 'DE' });
  assert.deepEqual(Array.from(diaspora), ['https://world.example.com/live'], 'diaspora station may refresh only within its saved source country');

  fetchHandler = async () => responseJson([{
    stationuuid: 'world-1',
    countrycode: 'HR',
    url_resolved: 'https://regional.example.com/live'
  }]);
  const foreignRegional = await RBNet.refreshCandidateUrls({ stationuuid: 'world-1', countrycode: 'INT', sourcecountrycode: 'DE' });
  assert.deepEqual(Array.from(foreignRegional), [], 'foreign refresh must never remap a Balkan station into the INT group');

  const diasporaRegional = await RBNet.refreshCandidateUrls({ stationuuid: 'world-1', countrycode: 'DIA', sourcecountrycode: 'DE' });
  assert.deepEqual(Array.from(diasporaRegional), [], 'diaspora refresh must never remap a Balkan station into the DIA group');

  fetchHandler = async () => responseJson([{
    stationuuid: 'world-1',
    countrycode: 'BG',
    url_resolved: 'https://bulgaria.example.com/live'
  }]);
  assert.deepEqual(
    Array.from(await RBNet.refreshCandidateUrls({ stationuuid: 'world-1', countrycode: 'INT', sourcecountrycode: 'DE' })),
    [],
    'foreign refresh must treat BG as Balkan and never remap it into INT'
  );
  assert.deepEqual(
    Array.from(await RBNet.refreshCandidateUrls({ stationuuid: 'world-1', countrycode: 'DIA', sourcecountrycode: 'DE' })),
    [],
    'diaspora refresh must treat BG as Balkan and never remap it into DIA'
  );

  let oversizedCalls = 0;
  fetchHandler = async () => {
    oversizedCalls += 1;
    return responseJson([], 200, String(600 * 1024));
  };
  const oversized = await RBNet.refreshCandidateUrls({ stationuuid: 'abc-123', countrycode: 'HR' });
  assert.deepEqual(Array.from(oversized), [], 'oversized refresh responses must fail closed');
  assert.equal(oversizedCalls, 4, 'oversized refresh must try only the fixed bounded fallback set');

  assert.deepEqual(Array.from(await RBNet.refreshCandidateUrls({ stationuuid: '../bad', countrycode: 'HR' })), [], 'invalid station UUID must never reach the API');
}

const popupPath = path.join(__dirname, '..', 'extensions', 'shared', 'popup.html');
const popupSource = fs.readFileSync(popupPath, 'utf8');
assert.match(popupSource, /id="playerLogo"[^>]*referrerpolicy="no-referrer"/, 'remote player artwork must not send a referrer');

testRefresh().then(() => {
  console.log('Browser network safety regression tests OK');
}).catch(error => {
  console.error(error);
  process.exitCode = 1;
});
