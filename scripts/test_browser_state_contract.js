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
  }

  addEventListener(type, listener) {
    (this.listeners[type] ||= []).push(listener);
  }

  setAttribute(name, value) {
    this.attributes[name] = String(value);
  }

  append(...items) {
    this.children.push(...items);
  }

  replaceChildren(...items) {
    this.children = [...items];
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
  'search', 'country', 'stations', 'status', 'refresh', 'heroPlay',
  'playerToggle', 'favoritesOnly', 'playerFav', 'playerState',
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

let runtimeListener = null;
let getState = { epoch: 'epoch-a', revision: 1, station: stationA, playing: true };
let toggleResult = null;
let getStateCalls = 0;
const toggleQueue = [];

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
      if (toggleQueue.length) return await toggleQueue.shift().promise;
      return copy(toggleResult || getState);
    }
    if (message?.type === 'RB_PLAY') {
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
    COUNTRIES: [['HR', 'Hrvatska'], ['RS', 'Srbija']],
    fold(value) {
      return String(value || '').toLowerCase().replace(/[^a-z0-9]+/g, '');
    },
    key(station) {
      return station?.stationuuid || station?.name || '';
    },
    safeHttp() {
      return false;
    },
    async load() {
      return [copy(stationA), copy(stationB)];
    },
    async favorites() {
      return {};
    },
    async setFavorite() {
      return {};
    }
  }
};
context.globalThis = context;
vm.createContext(context);

async function flush() {
  for (let i = 0; i < 8; i += 1) {
    await new Promise(resolve => setImmediate(resolve));
  }
}

async function main() {
  const popupPath = path.join(__dirname, '..', 'extensions', 'shared', 'popup.js');
  const source = fs.readFileSync(popupPath, 'utf8');
  vm.runInContext(source, context, { filename: popupPath });
  await flush();

  assert.equal(typeof runtimeListener, 'function', 'popup must register the runtime state listener');
  assert.equal(elements.playerName.textContent, 'Radio A', 'initial GET_STATE must select the active station');
  assert.equal(elements.playerState.textContent, 'Sada svira', 'initial playing state must be rendered');

  runtimeListener({ type: 'RB_STATE', epoch: 'epoch-a', revision: 0, station: stationB, playing: false });
  await flush();
  assert.equal(elements.playerName.textContent, 'Radio A', 'lower revision from the current epoch must be ignored');
  assert.equal(elements.playerState.textContent, 'Sada svira');

  getState = { epoch: 'epoch-b', revision: 1, station: stationB, playing: true };
  const beforeEpochSync = getStateCalls;
  runtimeListener({ type: 'RB_STATE', epoch: 'epoch-b', revision: 0, station: stationB, playing: false });
  await flush();
  assert.ok(getStateCalls > beforeEpochSync, 'a foreign epoch must trigger authoritative RB_GET_STATE resync');
  assert.equal(elements.playerName.textContent, 'Radio B', 'confirmed worker epoch must replace the previous state');
  assert.equal(elements.playerState.textContent, 'Sada svira');

  getState = { epoch: 'epoch-b', revision: 2, station: stationB, playing: true };
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
  getState = { epoch: 'epoch-b', revision: 3, station: stationB, playing: true };
  elements.playerToggle.dispatch('click');
  await flush();
  assert.equal(elements.playerName.textContent, 'Radio B', 'stale command reply from a retired epoch cannot replace current station');
  assert.equal(elements.playerState.textContent, 'Sada svira', 'authoritative resync must win over stale command status');

  toggleResult = null;
  const olderToggle = deferred();
  const newerToggle = deferred();
  toggleQueue.push(olderToggle, newerToggle);
  elements.playerToggle.dispatch('click');
  elements.playerToggle.dispatch('click');
  newerToggle.resolve({ epoch: 'epoch-b', revision: 4, station: stationB, playing: false });
  await flush();
  assert.equal(elements.playerState.textContent, 'Pauzirano', 'newer command result must update the player');
  olderToggle.resolve({ epoch: 'epoch-b', revision: 5, station: stationB, playing: true });
  await flush();
  assert.equal(elements.playerState.textContent, 'Pauzirano', 'older promise reply cannot override a newer command even with a higher revision');
  assert.equal(elements.playerName.textContent, 'Radio B');

  console.log('Browser popup state regression tests OK');
}

main().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
