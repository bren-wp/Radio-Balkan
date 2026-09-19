'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

let messageListener = null;
let playCalls = 0;
let fastTimeout = false;
const playPlans = [];
const refreshPlans = [];
let refreshCalls = 0;
const reports = [];
const instances = [];
const recentStorage = { rbRecent: [] };

class FakeAudio {
  constructor() {
    this.preload = '';
    this.src = '';
    this.paused = true;
    this.onerror = null;
    this.onended = null;
    this.onplaying = null;
    this.onpause = null;
    this.onwaiting = null;
    this.onstalled = null;
    instances.push(this);
  }

  async play() {
    playCalls += 1;
    const plan = playPlans.length ? playPlans.shift() : Promise.resolve();
    await plan;
    this.paused = false;
    if (typeof this.onplaying === 'function') this.onplaying();
  }

  pause() {
    const wasPlaying = !this.paused;
    this.paused = true;
    if (wasPlaying && typeof this.onpause === 'function') this.onpause();
  }

  removeAttribute(name) {
    if (name === 'src') this.src = '';
  }

  load() { }
}

const runtime = {
  onMessage: {
    addListener(listener) {
      messageListener = listener;
    }
  },
  async sendMessage(message) {
    reports.push(JSON.parse(JSON.stringify(message)));
    return { ok: true };
  }
};

const realSetTimeout = setTimeout;
const context = {
  console,
  Date,
  Math,
  Promise,
  setTimeout(fn, delay) {
    const effective = fastTimeout && (delay === 12_000 || delay === 15_000) ? 0 : delay;
    return realSetTimeout(fn, effective);
  },
  clearTimeout,
  globalThis: null,
  browser: {
    runtime,
    storage: {
      local: {
        async get(keys) {
          const out = {};
          for (const key of keys) out[key] = recentStorage[key];
          return out;
        },
        async set(values) { Object.assign(recentStorage, values); }
      }
    }
  },
  document: {
    createElement(tag) {
      assert.equal(tag, 'audio');
      return new FakeAudio();
    }
  },
  RBNet: {
    safeHttp(value) {
      return /^https?:\/\//.test(String(value || ''));
    },
    candidateUrls(station) {
      return Array.isArray(station?.streams) ? [...station.streams] : [];
    },
    async refreshCandidateUrls() {
      refreshCalls += 1;
      if (!refreshPlans.length) return [];
      return await refreshPlans.shift();
    }
  }
};
context.globalThis = context;
vm.createContext(context);

const stationA = { stationuuid: 'a', name: 'Radio A', streams: ['https://example.com/a'] };
const stationB = { stationuuid: 'b', name: 'Radio B', streams: ['https://example.com/b'] };
const stationC = { stationuuid: 'c', name: 'Radio C', streams: ['https://example.com/c1', 'https://example.com/c2'] };

async function flush() {
  for (let i = 0; i < 6; i += 1) await new Promise(resolve => setImmediate(resolve));
}

async function main() {
  const playerPath = path.join(__dirname, '..', 'extensions', 'platform', 'firefox', 'background-firefox.js');
  vm.runInContext(fs.readFileSync(playerPath, 'utf8'), context, { filename: playerPath });
  assert.equal(typeof messageListener, 'function', 'Firefox background must register a runtime listener');

  const firstPlay = await messageListener({ type: 'RB_PLAY', station: stationA, recentKey: 'a' });
  await flush();
  assert.equal(firstPlay.playing, true, 'play must enter the playing state');
  assert.ok(firstPlay.sessionId, 'play must create a session id');
  assert.deepEqual(Array.from(recentStorage.rbRecent), ['a'], 'successful Firefox background playback must persist recent history');

  const firstInstance = instances[instances.length - 1];
  const instanceCountBeforePause = instances.length;
  const paused = await messageListener({ type: 'RB_TOGGLE' });
  await flush();
  assert.equal(paused.playing, false, 'pause must leave Firefox playback paused');
  assert.equal(instances.length, instanceCountBeforePause, 'pause must not recreate the Firefox audio element');

  const playsBeforeResume = playCalls;
  const resumed = await messageListener({ type: 'RB_TOGGLE' });
  await flush();
  assert.equal(resumed.playing, true, 'resume must restore Firefox playback');
  assert.equal(instances.length, instanceCountBeforePause, 'resume must reuse the paused Firefox audio element');
  assert.equal(instances[instances.length - 1], firstInstance, 'resume must preserve the active Firefox audio instance');
  assert.equal(playCalls, playsBeforeResume + 1, 'resume must call play exactly once on the paused Firefox audio');

  const playsBeforeStop = playCalls;
  const stopped = await messageListener({ type: 'RB_STOP' });
  await flush();
  assert.equal(stopped.playing, false, 'stop must clear playing state');
  assert.equal(stopped.sessionId, null, 'stop must terminate the Firefox playback session');

  const afterStopToggle = await messageListener({ type: 'RB_TOGGLE' });
  await flush();
  assert.equal(afterStopToggle.playing, false, 'toggle after stop must not revive the stopped session');
  assert.equal(afterStopToggle.sessionId, null, 'toggle after stop must remain outside a playback session');
  assert.equal(playCalls, playsBeforeStop, 'toggle after stop must not create a new audio playback attempt');

  const restarted = await messageListener({ type: 'RB_PLAY', station: stationA });
  await flush();
  assert.equal(restarted.playing, true, 'play after stop must create fresh Firefox playback');
  assert.ok(restarted.sessionId, 'replay after stop must have a Firefox session');
  assert.notEqual(restarted.sessionId, firstPlay.sessionId, 'replay after stop must not reuse the retired Firefox session');
  await messageListener({ type: 'RB_STOP' });
  await flush();

  const slowPlay = deferred();
  playPlans.push(slowPlay.promise, Promise.resolve());
  const oldRequest = messageListener({ type: 'RB_PLAY', station: stationA });
  await flush();
  const newRequest = messageListener({ type: 'RB_PLAY', station: stationB, recentKey: 'b' });
  const newResult = await newRequest;
  assert.equal(newResult.station.stationuuid, 'b', 'newer play request must own the active station');
  assert.equal(newResult.playing, true, 'newer play request must be playing');

  slowPlay.resolve();
  const oldResult = await oldRequest;
  await flush();
  assert.equal(oldResult.stale, true, 'superseded play request must be marked stale');

  const finalState = await messageListener({ type: 'RB_GET_STATE' });
  assert.equal(finalState.station.stationuuid, 'b', 'slow old playback must not replace the newer station');
  assert.equal(finalState.playing, true, 'newer station must remain playing after the stale promise resolves');
  assert.equal(recentStorage.rbRecent[0], 'b', 'stale Firefox playback completion must not overwrite the newest recent station');

  const hanging = deferred();
  playPlans.push(hanging.promise, Promise.resolve());
  fastTimeout = true;
  const beforeTimeoutRecovery = playCalls;
  const recovered = await messageListener({ type: 'RB_PLAY', station: stationC });
  assert.equal(recovered.station.stationuuid, 'c');
  assert.equal(recovered.playing, true, 'a hanging first candidate must recover to the next Firefox stream');
  assert.equal(playCalls, beforeTimeoutRecovery + 2, 'Firefox start timeout must advance exactly one candidate');

  playPlans.push(Promise.resolve(), Promise.resolve());
  const stalledStart = await messageListener({ type: 'RB_PLAY', station: stationC });
  assert.equal(stalledStart.playing, true);
  const active = instances[instances.length - 1];
  const beforeStallRecovery = playCalls;
  active.onstalled();
  await new Promise(resolve => realSetTimeout(resolve, 20));
  await flush();
  const afterStall = await messageListener({ type: 'RB_GET_STATE' });
  assert.equal(afterStall.playing, true, 'Firefox stalled audio must recover to the next candidate');
  assert.ok(playCalls > beforeStallRecovery, 'Firefox stall recovery must attempt another candidate');

  fastTimeout = false;
  assert.ok(reports.some(message => message.type === 'RB_STATE'), 'player must report state updates to extension UI');

  const beforeRefreshRecovery = playCalls;
  const refreshBefore = refreshCalls;
  playPlans.push({ then(resolve, reject) { reject(new Error('stale stream')); } });
  refreshPlans.push(Promise.resolve(['https://example.com/d-fresh']));
  const refreshed = await messageListener({
    type: 'RB_PLAY',
    station: { stationuuid: 'd', name: 'Radio D', countrycode: 'HR', streams: ['https://example.com/d-stale'] }
  });
  await flush();
  assert.equal(refreshed.playing, true, 'Firefox candidate exhaustion must recover through a refreshed station URL');
  assert.equal(refreshed.station.url_resolved, 'https://example.com/d-fresh', 'refreshed Firefox stream must become the active session URL');
  assert.equal(refreshCalls, refreshBefore + 1, 'Firefox may refresh the catalog at most once per playback session');
  assert.equal(playCalls, beforeRefreshRecovery + 2, 'Firefox refresh recovery must try only the new stream');

  const refreshGate = deferred();
  playPlans.push({ then(resolve, reject) { reject(new Error('stale stream')); } });
  refreshPlans.push(refreshGate.promise);
  const pendingRefreshPlay = messageListener({
    type: 'RB_PLAY',
    station: { stationuuid: 'e', name: 'Radio E', countrycode: 'HR', streams: ['https://example.com/e-stale'] }
  });
  await flush();
  const playsBeforeStopDuringRefresh = playCalls;
  const stoppedDuringRefresh = await messageListener({ type: 'RB_STOP' });
  refreshGate.resolve(['https://example.com/e-fresh']);
  const staleRefreshResult = await pendingRefreshPlay;
  await flush();
  assert.equal(stoppedDuringRefresh.playing, false, 'Firefox stop during catalog refresh must remain terminal');
  assert.equal(staleRefreshResult.stale, true, 'Firefox must reject a catalog refresh that finishes after stop');
  assert.equal(playCalls, playsBeforeStopDuringRefresh, 'Firefox stale refresh must not restart audio after stop');

  console.log('Firefox player lifecycle/timeout/stall/session regression tests OK');
}

main().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
