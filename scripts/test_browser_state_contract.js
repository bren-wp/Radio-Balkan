'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

class Element {
  constructor(id = '') {
    this.id = id;
    this.value = '';
    this.textContent = '';
    this.disabled = false;
    this.hidden = false;
    this.dataset = {};
    this.className = '';
    this.children = [];
    this.attributes = {};
    this.listeners = {};
    this.src = '';
    this.onerror = null;
    this.replaceChildrenCalls = 0;
    this.querySelectorAllResult = [];
    this.focused = false;
    this.selectedIndex = 0;
    this.options = [];
  }

  addEventListener(type, listener) {
    (this.listeners[type] ||= []).push(listener);
  }

  setAttribute(name, value) {
    this.attributes[name] = String(value);
  }

  append(...items) {
    this.children.push(...items);
    if (this.id === 'country') this.options.push(...items);
  }

  replaceChildren(...items) {
    this.replaceChildrenCalls += 1;
    this.children = [...items];
    if (this.id === 'country') this.options = [...items];
  }

  querySelectorAll() {
    return this.querySelectorAllResult;
  }

  focus() {
    this.focused = true;
  }

  remove() { }

  closest() {
    return null;
  }

  dispatch(type, event = {}) {
    for (const listener of this.listeners[type] || []) listener(event);
  }
}

function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

const ids = [
  'search', 'country', 'genre', 'stations', 'status', 'refresh', 'heroPlay',
  'playerToggle', 'playerPrev', 'playerStop', 'playerNext', 'favoritesOnly', 'clearFilters', 'playerFav', 'playerState',
  'playerName', 'playerMeta', 'heroName', 'heroMeta', 'playerLogo'
];
const elements = Object.fromEntries(ids.map(id => [id, new Element(id)]));

const stationA = {
  stationuuid: 'station-a',
  name: 'Radio A',
  country: 'Hrvatska',
  countrycode: 'HR',
  tags: 'pop',
  bitrate: 128,
  codec: 'MP3',
  logo: ''
};
const stationB = {
  stationuuid: 'station-b',
  name: 'Radio B',
  country: 'Srbija',
  countrycode: 'RS',
  tags: 'rock',
  bitrate: 192,
  codec: 'AAC',
  logo: ''
};
const extraStations = Array.from({ length: 58 }, (_, index) => ({
  stationuuid: `station-extra-${index}`,
  name: `Radio Extra ${index}`,
  country: 'Hrvatska',
  countrycode: 'HR',
  tags: index % 2 ? 'pop' : 'rock',
  bitrate: 128,
  codec: 'MP3',
  logo: ''
}));

let runtimeListener = null;
let getState = { epoch: 'epoch-a', revision: 1, station: stationA, playing: true, sessionId: 'session-a' };
let toggleResult = null;
let stopResult = null;
let getStateCalls = 0;
let toggleCalls = 0;
let stopCalls = 0;
let playCalls = 0;
let lastPlayedStation = null;
let favoriteStore = {};
const toggleQueue = [];
const stopQueue = [];

function copy(value) {
  return value == null ? value : JSON.parse(JSON.stringify(value));
}

const runtime = {
  onMessage: {
    addListener(listener) {
      runtimeListener = listener;
    }
  },
  async sendMessage(message) {
    if (message?.type === 'RB_GET_STATE') {
      getStateCalls += 1;
      return copy(getState);
    }
    if (message?.type === 'RB_TOGGLE') {
      toggleCalls += 1;
      if (toggleQueue.length) return await toggleQueue.shift().promise;
      return copy(toggleResult || getState);
    }
    if (message?.type === 'RB_STOP') {
      stopCalls += 1;
      if (stopQueue.length) return await stopQueue.shift().promise;
      return copy(stopResult || getState);
    }
    if (message?.type === 'RB_PLAY') {
      playCalls += 1;
      lastPlayedStation = copy(message.station);
      return copy(getState);
    }
    throw new Error(`Unexpected runtime message: ${message?.type}`);
  }
};

const context = {
  console,
  setTimeout,
  clearTimeout,
  Promise,
  document: {
    getElementById(id) {
      if (!elements[id]) throw new Error(`Unknown element id: ${id}`);
      return elements[id];
    },
    createElement() {
      return new Element();
    },
    createDocumentFragment() {
      return new Element('fragment');
    }
  },
  RB: {
    ext: { runtime },
    COUNTRIES: [['HR', 'Hrvatska'], ['RS', 'Srbija'], ['INT', 'Strano']],
    FOREIGN_CODE: 'INT',
    fold(value) {
      return String(value || '').toLowerCase().normalize('NFD').replace(/[\u0300-\u036f]/g, '').replace(/[^a-z0-9]+/g, '');
    },
    key(station) {
      return station?.stationuuid || station?.name || '';
    },
    safeHttp() {
      return false;
    },
    async load() {
      return [copy(stationA), copy(stationB), ...copy(extraStations)];
    },
    async favorites() {
      return copy(favoriteStore);
    },
    async setFavorite(key, value) {
      if (value) favoriteStore[key] = true;
      else delete favoriteStore[key];
      return copy(favoriteStore);
    },
    async uiPreferences() {
      return {};
    },
    async setUiPreferences() {
      return {};
    }
  }
};
context.globalThis = context;
vm.createContext(context);

async function flush() {
  for (let i = 0; i < 8; i += 1) await new Promise(resolve => setImmediate(resolve));
}

async function main() {
  const popupPath = path.join(__dirname, '..', 'extensions', 'shared', 'popup.js');
  const source = fs.readFileSync(popupPath, 'utf8');
  vm.runInContext(source, context, { filename: popupPath });
  await flush();

  assert.equal(typeof runtimeListener, 'function', 'popup must register the runtime state listener');
  assert.equal(elements.playerName.textContent, 'Radio A', 'initial GET_STATE must select the active station');
  assert.equal(elements.playerState.textContent, 'Sada svira', 'initial playing state must be rendered');
  assert.equal(elements.playerToggle.disabled, false, 'player toggle must be enabled after a station is available');
  assert.equal(elements.playerPrev.disabled, false, 'previous control must be enabled when multiple stations are available');
  assert.equal(elements.playerNext.disabled, false, 'next control must be enabled when multiple stations are available');
  assert.equal(elements.playerStop.disabled, false, 'player stop must be enabled after a station is available');
  assert.equal(elements.playerFav.disabled, false, 'favorite control must be enabled after a station is available');
  assert.equal(elements.playerFav.attributes['aria-pressed'], 'false', 'favorite state must be announced accessibly');

  elements.genre.value = 'jazz';
  elements.genre.dispatch('change');
  assert.ok(elements.stations.replaceChildrenCalls > 0, 'genre change must re-render the station result set');
  elements.genre.value = '';
  elements.genre.dispatch('change');

  const focusTargets = Array.from({ length: 60 }, () => new Element());
  elements.stations.querySelectorAllResult = focusTargets;
  elements.stations.dispatch('click', {
    target: {
      closest(selector) {
        return selector === '[data-more]' ? { dataset: { more: '1' } } : null;
      }
    }
  });
  assert.equal(focusTargets[48].focused, true, 'expanding the station list must focus the first newly revealed play control');
  elements.stations.querySelectorAllResult = [];

  elements.country.value = 'RS';
  elements.refresh.dispatch('click');
  await flush();
  assert.equal(elements.country.value, 'RS', 'selected country filter must survive a catalog refresh');
  assert.equal(elements.playerPrev.disabled, true, 'previous must disable when the active filter leaves only one visible station, even if the current station is outside that filter');
  assert.equal(elements.playerNext.disabled, true, 'next must disable when the active filter leaves only one visible station, even if the current station is outside that filter');
  elements.country.value = '';
  elements.country.dispatch('change');
  assert.equal(elements.playerPrev.disabled, false, 'previous must re-enable when multiple stations are visible again');
  assert.equal(elements.playerNext.disabled, false, 'next must re-enable when multiple stations are visible again');

  const listRepaintsBeforeState = elements.stations.replaceChildrenCalls;
  runtimeListener({ type: 'RB_STATE', epoch: 'epoch-a', revision: 2, station: stationA, playing: false, sessionId: 'session-a' });
  await flush();
  assert.equal(elements.playerState.textContent, 'Pauzirano');
  assert.equal(elements.stations.replaceChildrenCalls, listRepaintsBeforeState, 'playback-only state updates must not rebuild the full station list');

  runtimeListener({ type: 'RB_STATE', epoch: 'epoch-a', revision: 0, station: stationB, playing: false });
  await flush();
  assert.equal(elements.playerName.textContent, 'Radio A', 'lower revision from the current epoch must be ignored');
  assert.equal(elements.playerState.textContent, 'Pauzirano');

  getState = { epoch: 'epoch-b', revision: 1, station: stationB, playing: true, sessionId: 'session-b' };
  const beforeEpochSync = getStateCalls;
  runtimeListener({ type: 'RB_STATE', epoch: 'epoch-b', revision: 0, station: stationB, playing: false });
  await flush();
  assert.ok(getStateCalls > beforeEpochSync, 'a foreign epoch must trigger authoritative RB_GET_STATE resync');
  assert.equal(elements.playerName.textContent, 'Radio B', 'confirmed worker epoch must replace the previous state');
  assert.equal(elements.playerState.textContent, 'Sada svira');

  getState = { epoch: 'epoch-b', revision: 2, station: stationB, playing: true, sessionId: 'session-b' };
  const beforeRetiredEpoch = getStateCalls;
  runtimeListener({ type: 'RB_STATE', epoch: 'epoch-a', revision: 99, station: stationA, playing: false });
  await flush();
  assert.ok(getStateCalls > beforeRetiredEpoch, 'retired worker epoch must be resynchronized, never trusted directly');
  assert.equal(elements.playerName.textContent, 'Radio B', 'retired worker epoch cannot restore stale state');
  assert.equal(elements.playerState.textContent, 'Sada svira');

  runtimeListener({ type: 'RB_STATE', epoch: 'epoch-b', revision: 1, station: stationA, playing: false });
  await flush();
  assert.equal(elements.playerName.textContent, 'Radio B', 'stale revision in current epoch cannot roll state back');

  toggleResult = { epoch: 'epoch-a', revision: 100, station: stationA, playing: false, error: 'stale worker result' };
  getState = { epoch: 'epoch-b', revision: 3, station: stationB, playing: true, sessionId: 'session-b' };
  elements.playerToggle.dispatch('click');
  await flush();
  assert.equal(elements.playerName.textContent, 'Radio B', 'stale command reply from a retired epoch cannot replace current station');
  assert.equal(elements.playerState.textContent, 'Sada svira', 'authoritative resync must win over stale command status');

  toggleResult = null;
  const pendingToggle = deferred();
  toggleQueue.push(pendingToggle);
  const callsBeforeBusyToggle = toggleCalls;
  elements.playerToggle.dispatch('click');
  elements.playerToggle.dispatch('click');
  assert.equal(toggleCalls, callsBeforeBusyToggle + 1, 'duplicate toggle clicks must be ignored while a command is in flight');
  assert.equal(elements.playerToggle.disabled, true, 'player toggle must be disabled while a command is in flight');
  assert.equal(elements.playerToggle.attributes['aria-busy'], 'true', 'busy playback state must be announced accessibly');
  pendingToggle.resolve({ epoch: 'epoch-b', revision: 4, station: stationB, playing: false, sessionId: 'session-b' });
  await flush();
  assert.equal(elements.playerState.textContent, 'Pauzirano', 'completed toggle command must update the player');
  assert.equal(elements.playerToggle.disabled, false, 'player toggle must be re-enabled after command completion');
  assert.equal(elements.playerToggle.attributes['aria-busy'], 'false', 'busy state must clear after command completion');
  assert.equal(elements.playerName.textContent, 'Radio B');

  stopResult = null;
  const pendingStop = deferred();
  stopQueue.push(pendingStop);
  const callsBeforeBusyStop = stopCalls;
  elements.playerStop.dispatch('click');
  elements.playerStop.dispatch('click');
  assert.equal(stopCalls, callsBeforeBusyStop + 1, 'duplicate stop clicks must be ignored while stop is in flight');
  assert.equal(elements.playerStop.disabled, true, 'player stop must be disabled while stop is in flight');
  assert.equal(elements.playerStop.attributes['aria-busy'], 'true', 'stop busy state must be announced accessibly');
  pendingStop.resolve({ epoch: 'epoch-b', revision: 5, station: stationB, playing: false, sessionId: null });
  await flush();
  assert.equal(elements.playerState.textContent, 'Zaustavljeno', 'completed stop command must render an explicit stopped state');
  assert.equal(elements.playerStop.disabled, true, 'player stop must stay disabled once playback is already stopped');
  assert.equal(elements.playerStop.attributes['aria-busy'], 'false', 'stop busy state must clear after completion');
  assert.equal(elements.playerStop.attributes['aria-label'], 'Reprodukcija je zaustavljena', 'stopped control must expose its terminal state accessibly');

  const playCallsBeforeStoppedReplay = playCalls;
  getState = { epoch: 'epoch-b', revision: 6, station: stationB, playing: true, sessionId: 'session-b2' };
  elements.playerToggle.dispatch('click');
  await flush();
  assert.equal(playCalls, playCallsBeforeStoppedReplay + 1, 'main play control after stop must send a fresh RB_PLAY command');
  assert.equal(elements.playerState.textContent, 'Sada svira', 'play after stop must create a fresh playback session from the main control');

  const adjacentCallsBefore = playCalls;
  getState = { epoch: 'epoch-b', revision: 7, station: stationB, playing: true, sessionId: 'session-b-next' };
  elements.playerNext.dispatch('click');
  await flush();
  assert.equal(playCalls, adjacentCallsBefore + 1, 'next control must start the adjacent station');
  assert.equal(lastPlayedStation?.stationuuid, 'station-extra-0', 'next from Radio B must select the following visible station');
  const afterNextCalls = playCalls;
  getState = { epoch: 'epoch-b', revision: 8, station: stationB, playing: true, sessionId: 'session-b-prev' };
  elements.playerPrev.dispatch('click');
  await flush();
  assert.equal(playCalls, afterNextCalls + 1, 'previous control must start the previous visible station');
  assert.equal(lastPlayedStation?.stationuuid, 'station-a', 'previous from Radio B must select the preceding visible station');
  assert.equal(elements.playerPrev.disabled, false, 'previous control must remain available after adjacent playback');

  elements.playerFav.dispatch('click');
  await flush();
  assert.equal(elements.playerFav.attributes['aria-pressed'], 'true', 'player favorite must reflect the saved favorite state');
  elements.favoritesOnly.dispatch('click');
  await flush();
  assert.match(elements.status.textContent, /^1 od 1 prikazano/, 'favorites filter must contain the newly favorited current station');
  elements.playerFav.dispatch('click');
  await flush();
  assert.match(elements.status.textContent, /^0 od 0 prikazano/, 'unfavoriting inside favorites view must immediately remove the station from the visible set');
  elements.favoritesOnly.dispatch('click');
  await flush();
  assert.match(elements.status.textContent, /^48 od 60 prikazano/, 'leaving favorites-only view must restore the full visible catalog page');

  runtimeListener({ type: 'RB_STATE', epoch: 'epoch-b', revision: 9, station: null, playing: false, sessionId: null });
  await flush();
  assert.equal(elements.playerName.textContent, 'Nije odabrano', 'authoritative cold state must clear the optimistic catalog selection');
  assert.equal(elements.playerToggle.disabled, true, 'cold player state must not expose a fake selected-station toggle');
  assert.equal(elements.playerFav.disabled, true, 'cold player state must not favorite a station the user never selected');
  assert.equal(elements.playerPrev.disabled, true, 'cold player state must not expose previous navigation without a current station');
  assert.equal(elements.playerNext.disabled, true, 'cold player state must not expose next navigation without a current station');
  const coldAdjacentCalls = playCalls;
  elements.playerNext.dispatch('click');
  elements.playerPrev.dispatch('click');
  await flush();
  assert.equal(playCalls, coldAdjacentCalls, 'cold adjacent controls must not start playback without a current station');
  assert.equal(elements.heroPlay.disabled, false, 'hero play must remain available and may start the first visible station');

  console.log('Browser popup state and UI regression tests OK');
}

main().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
