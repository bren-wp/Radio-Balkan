'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}

let listener = null;
let playCalls = 0;
let fastTimeout = false;
const playPlans = [];
const refreshPlans = [];
let refreshCalls = 0;
const instances = [];
const reports = [];

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
    addListener(fn) { listener = fn; }
  },
  async sendMessage(message) {
    reports.push(JSON.parse(JSON.stringify(message)));
    return { ok: true };
  }
};

const realSetTimeout = setTimeout;
const context = {
  console,
  Promise,
  setTimeout(fn, delay) {
    const effective = fastTimeout && (delay === 12_000 || delay === 15_000) ? 0 : delay;
    return realSetTimeout(fn, effective);
  },
  clearTimeout,
  chrome: { runtime },
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

function dispatch(message) {
  return new Promise((resolve, reject) => {
    if (typeof listener !== 'function') return reject(new Error('offscreen listener was not registered'));
    let settled = false;
    const sendResponse = value => {
      if (settled) return;
      settled = true;
      resolve(value);
    };
    try {
      const result = listener({ target: 'offscreen', ...message }, {}, sendResponse);
      if (result !== true && !settled) resolve(result);
    } catch (error) {
      reject(error);
    }
  });
}

async function flush() {
  for (let i = 0; i < 8; i += 1) await new Promise(resolve => setImmediate(resolve));
}

async function main() {
  const playerPath = path.join(__dirname, '..', 'extensions', 'platform', 'chromium', 'offscreen.js');
  vm.runInContext(fs.readFileSync(playerPath, 'utf8'), context, { filename: playerPath });
  assert.equal(typeof listener, 'function', 'Chromium offscreen player must register a runtime listener');

  const first = await dispatch({
    type: 'PLAY',
    sessionId: 's1',
    station: { name: 'Radio A', streams: ['https://example.com/a'] }
  });
  assert.equal(first.ok, true);
  assert.equal(first.playing, true);

  const firstInstance = instances[instances.length - 1];
  const instanceCountBeforePause = instances.length;
  const paused = await dispatch({ type: 'TOGGLE', sessionId: 's1' });
  assert.equal(paused.ok, true);
  assert.equal(paused.playing, false, 'pause must leave Chromium playback paused');
  assert.equal(instances.length, instanceCountBeforePause, 'pause must not recreate the audio element');

  const playsBeforeResume = playCalls;
  const resumed = await dispatch({ type: 'TOGGLE', sessionId: 's1' });
  assert.equal(resumed.ok, true);
  assert.equal(resumed.playing, true, 'resume must restore Chromium playback');
  assert.equal(instances.length, instanceCountBeforePause, 'resume must reuse the paused audio element');
  assert.equal(instances[instances.length - 1], firstInstance, 'resume must preserve the active audio instance');
  assert.equal(playCalls, playsBeforeResume + 1, 'resume must call play exactly once on the paused audio');

  const stopped = await dispatch({ type: 'STOP', sessionId: 's1' });
  assert.equal(stopped.ok, true);
  assert.equal(stopped.playing, false, 'stop must clear Chromium playback');
  const playsBeforeStoppedToggle = playCalls;
  const afterStopToggle = await dispatch({ type: 'TOGGLE', sessionId: 's1' });
  assert.equal(afterStopToggle.ok, false, 'toggle after stop must reject the retired Chromium session');
  assert.equal(afterStopToggle.playing, false);
  assert.equal(afterStopToggle.sessionId, null, 'stop must retire the offscreen session');
  assert.equal(playCalls, playsBeforeStoppedToggle, 'toggle after stop must not create a playback attempt');

  const restarted = await dispatch({
    type: 'PLAY',
    sessionId: 's1-restart',
    station: { name: 'Radio A', streams: ['https://example.com/a'] }
  });
  assert.equal(restarted.ok, true);
  assert.equal(restarted.playing, true, 'play after stop must create fresh Chromium playback');
  assert.equal(restarted.sessionId, 's1-restart', 'replay after stop must use the newly requested Chromium session');

  const hanging = deferred();
  playPlans.push(hanging.promise, Promise.resolve());
  fastTimeout = true;
  const beforeTimeoutRecovery = playCalls;
  const recovered = await dispatch({
    type: 'PLAY',
    sessionId: 's2',
    station: { name: 'Radio B', streams: ['https://example.com/b1', 'https://example.com/b2'] }
  });
  assert.equal(recovered.ok, true, 'a hanging first candidate must fall back to the next stream');
  assert.equal(recovered.playing, true);
  assert.equal(playCalls, beforeTimeoutRecovery + 2, 'start timeout must advance exactly one candidate');

  playPlans.push(Promise.resolve(), Promise.resolve());
  const playing = await dispatch({
    type: 'PLAY',
    sessionId: 's3',
    station: { name: 'Radio C', streams: ['https://example.com/c1', 'https://example.com/c2'] }
  });
  assert.equal(playing.ok, true);
  const active = instances[instances.length - 1];
  const beforeStallRecovery = playCalls;
  assert.equal(typeof active.onstalled, 'function');
  active.onstalled();
  await new Promise(resolve => realSetTimeout(resolve, 20));
  await flush();
  const afterStall = await dispatch({ type: 'GET_STATE', sessionId: 's3' });
  assert.equal(afterStall.playing, true, 'stalled active audio must recover to a fallback candidate');
  assert.ok(playCalls > beforeStallRecovery, 'stall recovery must attempt another candidate');

  fastTimeout = false;

  const beforeRefreshRecovery = playCalls;
  const refreshBefore = refreshCalls;
  playPlans.push({ then(resolve, reject) { reject(new Error('stale stream')); } });
  refreshPlans.push(Promise.resolve(['https://example.com/d-fresh']));
  const refreshed = await dispatch({
    type: 'PLAY',
    sessionId: 's4',
    station: { stationuuid: 'd', name: 'Radio D', streams: ['https://example.com/d-stale'] }
  });
  assert.equal(refreshed.ok, true, 'candidate exhaustion must recover through a refreshed station URL');
  assert.equal(refreshed.playing, true);
  assert.equal(refreshed.station.url_resolved, 'https://example.com/d-fresh', 'refreshed Chromium stream must become the active session URL');
  assert.equal(refreshCalls, refreshBefore + 1, 'one playback session may refresh the catalog at most once');
  assert.equal(playCalls, beforeRefreshRecovery + 2, 'refresh recovery must retry only the newly discovered stream');

  const refreshGate = deferred();
  playPlans.push({ then(resolve, reject) { reject(new Error('stale stream')); } });
  refreshPlans.push(refreshGate.promise);
  const pendingRefreshPlay = dispatch({
    type: 'PLAY',
    sessionId: 's5',
    station: { stationuuid: 'e', name: 'Radio E', streams: ['https://example.com/e-stale'] }
  });
  await flush();
  const playsBeforeStopDuringRefresh = playCalls;
  const stoppedDuringRefresh = await dispatch({ type: 'STOP', sessionId: 's5' });
  refreshGate.resolve(['https://example.com/e-fresh']);
  const staleRefreshResult = await pendingRefreshPlay;
  await flush();
  assert.equal(stoppedDuringRefresh.playing, false, 'stop during catalog refresh must remain terminal');
  assert.equal(staleRefreshResult.stale, true, 'catalog refresh finishing after stop must be rejected as stale');
  assert.equal(playCalls, playsBeforeStopDuringRefresh, 'stale refresh must not start a new audio instance after stop');

  assert.ok(reports.some(message => message.type === 'RB_OFFSCREEN_STATE'), 'offscreen player must continue reporting state');
  console.log('Chromium offscreen lifecycle/timeout/stall regression tests OK');
}

main().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
