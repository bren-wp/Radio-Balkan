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
const playPlans = [];
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

const context = {
  console,
  Date,
  Math,
  Promise,
  globalThis: null,
  browser: { runtime },
  document: {
    createElement(tag) {
      assert.equal(tag, 'audio');
      return new FakeAudio();
    }
  },
  RBNet: {
    candidateUrls(station) {
      return Array.isArray(station?.streams) ? [...station.streams] : [];
    }
  }
};
context.globalThis = context;
vm.createContext(context);

const stationA = { stationuuid: 'a', name: 'Radio A', streams: ['https://example.com/a'] };
const stationB = { stationuuid: 'b', name: 'Radio B', streams: ['https://example.com/b'] };

async function flush() {
  for (let i = 0; i < 6; i += 1) await new Promise(resolve => setImmediate(resolve));
}

async function main() {
  const playerPath = path.join(__dirname, '..', 'extensions', 'platform', 'firefox', 'background-firefox.js');
  vm.runInContext(fs.readFileSync(playerPath, 'utf8'), context, { filename: playerPath });
  assert.equal(typeof messageListener, 'function', 'Firefox background must register a runtime listener');

  const firstPlay = await messageListener({ type: 'RB_PLAY', station: stationA });
  await flush();
  assert.equal(firstPlay.playing, true, 'play must enter the playing state');
  assert.ok(firstPlay.sessionId, 'play must create a session id');

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

  const slowPlay = deferred();
  playPlans.push(slowPlay.promise, Promise.resolve());
  const oldRequest = messageListener({ type: 'RB_PLAY', station: stationA });
  await flush();
  const newRequest = messageListener({ type: 'RB_PLAY', station: stationB });
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
  assert.ok(reports.some(message => message.type === 'RB_STATE'), 'player must report state updates to extension UI');

  console.log('Firefox player regression tests OK');
}

main().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
