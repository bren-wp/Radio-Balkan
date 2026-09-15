(() => {
  'use strict';

  const stations = [
    { stationuuid: 'preview-hr-1', name: 'Radio Zagreb', url: 'https://example.com/hr-zagreb', url_resolved: 'https://example.com/hr-zagreb', homepage: '', favicon: '', tags: 'pop,hitovi', country: 'Hrvatska', countrycode: 'HR', state: 'Zagreb', language: 'hrvatski', codec: 'MP3', votes: 420, bitrate: 192, lastcheckok: 1 },
    { stationuuid: 'preview-ba-1', name: 'Sarajevo Radio', url: 'https://example.com/ba-sarajevo', url_resolved: 'https://example.com/ba-sarajevo', homepage: '', favicon: '', tags: 'zabavna,domaća', country: 'Bosna i Hercegovina', countrycode: 'BA', state: 'Sarajevo', language: 'bosanski', codec: 'AAC', votes: 360, bitrate: 128, lastcheckok: 1 },
    { stationuuid: 'preview-rs-1', name: 'Beograd FM', url: 'https://example.com/rs-beograd', url_resolved: 'https://example.com/rs-beograd', homepage: '', favicon: '', tags: 'pop,rock', country: 'Srbija', countrycode: 'RS', state: 'Beograd', language: 'srpski', codec: 'MP3', votes: 330, bitrate: 192, lastcheckok: 1 },
    { stationuuid: 'preview-si-1', name: 'Ljubljana Radio', url: 'https://example.com/si-ljubljana', url_resolved: 'https://example.com/si-ljubljana', homepage: '', favicon: '', tags: 'pop', country: 'Slovenija', countrycode: 'SI', state: 'Ljubljana', language: 'slovenski', codec: 'AAC', votes: 290, bitrate: 128, lastcheckok: 1 },
    { stationuuid: 'preview-mk-1', name: 'Skopje Hits', url: 'https://example.com/mk-skopje', url_resolved: 'https://example.com/mk-skopje', homepage: '', favicon: '', tags: 'hitovi', country: 'Sjeverna Makedonija', countrycode: 'MK', state: 'Skopje', language: 'makedonski', codec: 'MP3', votes: 260, bitrate: 160, lastcheckok: 1 },
    { stationuuid: 'preview-me-1', name: 'Podgorica FM', url: 'https://example.com/me-podgorica', url_resolved: 'https://example.com/me-podgorica', homepage: '', favicon: '', tags: 'regionalna', country: 'Crna Gora', countrycode: 'ME', state: 'Podgorica', language: 'crnogorski', codec: 'MP3', votes: 220, bitrate: 128, lastcheckok: 1 },
    { stationuuid: 'preview-al-1', name: 'Tirana Radio', url: 'https://example.com/al-tirana', url_resolved: 'https://example.com/al-tirana', homepage: '', favicon: '', tags: 'pop', country: 'Albanija', countrycode: 'AL', state: 'Tirana', language: 'shqip', codec: 'AAC', votes: 180, bitrate: 128, lastcheckok: 1 }
  ];

  const storage = {
    rbCatalog: stations,
    rbCatalogAt: Date.now(),
    rbFavorites: { 'preview-hr-1': true }
  };
  const listeners = [];
  let revision = 0;
  const epoch = 'screenshot-preview';
  let playerState = { station: null, playing: false };

  function select(keys) {
    if (keys == null) return { ...storage };
    if (typeof keys === 'string') return { [keys]: storage[keys] };
    if (Array.isArray(keys)) return Object.fromEntries(keys.map(key => [key, storage[key]]));
    if (typeof keys === 'object') {
      return Object.fromEntries(Object.entries(keys).map(([key, fallback]) => [key, storage[key] ?? fallback]));
    }
    return {};
  }

  function envelope(extra = {}) {
    return { ...playerState, epoch, revision, ...extra };
  }

  function storageGet(keys, callback) {
    const value = select(keys);
    if (typeof callback === 'function') {
      queueMicrotask(() => callback(value));
      return undefined;
    }
    return Promise.resolve(value);
  }

  function storageSet(values, callback) {
    Object.assign(storage, values || {});
    if (typeof callback === 'function') {
      queueMicrotask(callback);
      return undefined;
    }
    return Promise.resolve();
  }

  function sendMessage(message, callback) {
    let value;
    if (message?.type === 'RB_GET_STATE') {
      value = envelope();
    } else if (message?.type === 'RB_PLAY') {
      playerState = { station: message.station || null, playing: true };
      revision += 1;
      value = envelope();
    } else if (message?.type === 'RB_TOGGLE') {
      playerState = { ...playerState, playing: !playerState.playing };
      revision += 1;
      value = envelope();
    } else {
      value = envelope();
    }
    if (typeof callback === 'function') {
      queueMicrotask(() => callback(value));
      return undefined;
    }
    return Promise.resolve(value);
  }

  globalThis.chrome = {
    storage: { local: { get: storageGet, set: storageSet } },
    runtime: {
      lastError: null,
      sendMessage,
      onMessage: { addListener(listener) { listeners.push(listener); } }
    }
  };
})();
