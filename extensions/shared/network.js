'use strict';

const RBNet = (() => {
  const FOREIGN_CODE = 'INT';
  const ALLOWED = new Set(['HR', 'BA', 'RS', 'SI', 'MK', 'AL', 'ME', FOREIGN_CODE]);
  const RADIO_BROWSER_SUFFIX = '.api.radio-browser.info';
  const BALKAN = new Set(['HR', 'BA', 'RS', 'SI', 'MK', 'AL', 'ME']);
  const RADIO_BROWSER_API_BASES = Object.freeze([
    'https://de1.api.radio-browser.info',
    'https://de2.api.radio-browser.info',
    'https://at1.api.radio-browser.info',
    'https://nl1.api.radio-browser.info'
  ]);
  const MAX_REFRESH_RESPONSE_BYTES = 512 * 1024;
  const REFRESH_TIMEOUT_MS = 7000;

  function countryCode(value) {
    return String(value || '').trim().toUpperCase();
  }

  function allowedCountry(value) {
    return ALLOWED.has(countryCode(value));
  }

  function canonicalHost(hostname) {
    return String(hostname || '').toLowerCase().replace(/^\[|\]$/g, '').replace(/\.+$/, '');
  }

  function privateHost(hostname) {
    const h = canonicalHost(hostname);
    if (!h || h === 'localhost' || h.endsWith('.localhost') || h.endsWith('.local') ||
        h === 'metadata.google.internal' || h === 'instance-data.ec2.internal' || h === 'metadata.azure.internal') return true;
    if (h === '::' || h === '::1' || h === '0:0:0:0:0:0:0:1' || h.startsWith('::ffff:') ||
        /^fe[89ab][0-9a-f]:/.test(h) || /^f[cd][0-9a-f]{2}:/.test(h) || /^ff[0-9a-f]{2}:/.test(h)) return true;
    const match = h.match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/);
    if (!match) return false;
    const octets = match.slice(1).map(Number);
    if (octets.some(n => n < 0 || n > 255)) return true;
    const [a, b] = octets;
    return a === 0 || a === 10 || a === 127 || a >= 224 ||
      (a === 100 && b >= 64 && b <= 127) ||
      (a === 169 && b === 254) ||
      (a === 172 && b >= 16 && b <= 31) ||
      (a === 192 && b === 168);
  }

  function safeHttp(raw) {
    const value = String(raw || '').trim();
    if (!value || value.length > 4096) return false;
    try {
      const url = new URL(value);
      return (url.protocol === 'http:' || url.protocol === 'https:') &&
        !!url.hostname && !url.username && !url.password && !privateHost(url.hostname);
    } catch {
      return false;
    }
  }

  function safeRadioBrowserBase(raw) {
    const value = String(raw || '').trim();
    if (!value || value.length > 253 || /[\s/@?#]/.test(value)) return '';
    const host = canonicalHost(value);
    if (!host.endsWith(RADIO_BROWSER_SUFFIX) || host === RADIO_BROWSER_SUFFIX.slice(1)) return '';
    const candidate = `https://${host}`;
    return safeHttp(`${candidate}/`) ? candidate : '';
  }

  function candidateUrls(station) {
    if (!allowedCountry(station?.countrycode)) return [];
    return [...new Set([station?.url_resolved, station?.url]
      .map(value => String(value || '').trim())
      .filter(safeHttp))];
  }

  function validStation(station) {
    return !!station && allowedCountry(station.countrycode) && candidateUrls(station).length > 0;
  }

  async function readJsonLimited(response, maxBytes) {
    const declaredRaw = response?.headers?.get?.('content-length');
    const declared = Number(declaredRaw);
    if (declaredRaw && Number.isFinite(declared) && declared > maxBytes) throw new Error('API odgovor je prevelik');
    if (!response?.body?.getReader) throw new Error('Streaming API odgovor nije dostupan');
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

  async function fetchRefreshRows(base, uuid) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), REFRESH_TIMEOUT_MS);
    try {
      const response = await fetch(
        `${base}/json/stations/byuuid/${encodeURIComponent(uuid)}`,
        { cache: 'no-store', redirect: 'error', signal: controller.signal }
      );
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const rows = await readJsonLimited(response, MAX_REFRESH_RESPONSE_BYTES);
      return Array.isArray(rows) ? rows : [];
    } finally {
      clearTimeout(timer);
    }
  }

  async function refreshCandidateUrls(station) {
    const expectedCountry = countryCode(station?.countrycode);
    const sourceCountry = countryCode(station?.sourcecountrycode);
    const uuid = String(station?.stationuuid || '').trim();
    if (!allowedCountry(expectedCountry) || !uuid || uuid.length > 128 || !/^[a-z0-9._:-]+$/i.test(uuid)) return [];

    for (const base of RADIO_BROWSER_API_BASES) {
      try {
        const rows = await fetchRefreshRows(base, uuid);
        const out = [];
        for (const row of rows) {
          const actualCountry = countryCode(row?.countrycode);
          if (expectedCountry === FOREIGN_CODE) {
            if (!actualCountry || BALKAN.has(actualCountry) || (sourceCountry && actualCountry !== sourceCountry)) continue;
          } else if (actualCountry !== expectedCountry) {
            continue;
          }
          for (const raw of [row?.url_resolved, row?.url]) {
            const value = String(raw || '').trim();
            if (safeHttp(value) && !out.includes(value)) out.push(value);
            if (out.length >= 4) break;
          }
          if (out.length >= 4) break;
        }
        if (out.length) return out;
      } catch { }
    }
    return [];
  }

  return Object.freeze({
    FOREIGN_CODE, allowedCountry, safeHttp, safeRadioBrowserBase, candidateUrls,
    validStation, refreshCandidateUrls
  });
})();
