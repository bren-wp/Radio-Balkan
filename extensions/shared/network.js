'use strict';

const RBNet = (() => {
  const ALLOWED = new Set(['HR', 'BA', 'RS', 'SI', 'MK', 'AL', 'ME']);

  function countryCode(value) {
    return String(value || '').trim().toUpperCase();
  }

  function allowedCountry(value) {
    return ALLOWED.has(countryCode(value));
  }

  function privateHost(hostname) {
    const h = String(hostname || '').toLowerCase().replace(/^\[|\]$/g, '');
    if (!h || h === 'localhost' || h.endsWith('.localhost') || h.endsWith('.local') ||
        h === 'metadata.google.internal' || h === 'instance-data.ec2.internal' || h === 'metadata.azure.internal') return true;
    if (h === '::1' || h === '0:0:0:0:0:0:0:1' || h.startsWith('fe80:') || /^f[cd][0-9a-f]:/.test(h)) return true;
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

  function candidateUrls(station) {
    if (!allowedCountry(station?.countrycode)) return [];
    return [...new Set([station?.url_resolved, station?.url]
      .map(value => String(value || '').trim())
      .filter(safeHttp))];
  }

  function validStation(station) {
    return !!station && allowedCountry(station.countrycode) && candidateUrls(station).length > 0;
  }

  return Object.freeze({ allowedCountry, safeHttp, candidateUrls, validStation });
})();
