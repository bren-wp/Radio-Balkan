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

const stationA = { stationuuid: 'a', name: 'Radio A', countrycode: 'HR', url: 'https://example.com/a' };
const stationB = { stationuuid: 'b', name: 'Radio B', countrycode: 'RS', url: 'https://example.com/b' };
const stationC = { stationuuid: 'c', name: 'Radio C', countrycode: 'BA', url: 'https://example.com/c' };

let listener = null;
let documentExists = false;
let createCalls = 0;
let closeCalls = 0;
let closeGate = null;
let delayedStop = null;
let offscreenGeneration = 0;
let offscreenSession = null;
let offscreenStation = null;
let offscreenPlaying = false;

const runtime = {
  onMessage: {
    addListener(fn) { listener = fn; }
  },
  async sendMessage(message) {
    if (message?.target !== 'offscreen') return null;
    if (message.type === 'PLAY') {
      offscreenSession = message.sessionId;
      offscreenStation = message.station;
      offscreenPlaying = true;
      offscreenGeneration += 1;
      return { ok: true, station: offscreenStation, playing: true, sessionId: offscreenSession, generation: offscreenGeneration };
    }
    if (message.type === 'STOP') {
      if (delayedStop) return delayedStop.promise;
      offscreenPlaying = false;
      offscreenGeneration += 1;
      return { ok: true, station: offscreenStation, playing: false, sessionId: message.sessionId, generation: offscreenGeneration };
    }
    if (message.type === 'TOGGLE') {
      offscreenPlaying = !offscreenPlaying;
      offscreenGeneration += 1;
      return { ok: true, station: offscreenStation, playing: offscreenPlaying, sessionId: message.sessionId, generation: offscreenGeneration };
    }
    if (message.type === 'GET_STATE') {
      return { ok: true, station: offscreenStation, playing: offscreenPlaying, sessionId: offscreenSession, generation: offscreenGeneration };
    }
    throw new Error(`Unexpected offscreen message ${message.type}`);
  }
};

const context = {
  console,
  URL,
  Promise,
  Date,
  Math,
  chrome: {
    runtime,
    offscreen: {
      async hasDocument() { return documentExists; },
      async createDocument() {
        createCalls += 1;
        documentExists = true;
      },
      async closeDocument() {
        closeCalls += 1;
        const gate = closeGate;
        if (gate) await gate.promise;
        documentExists = false;
        offscreenSession = null;
        offscreenStation = null;
        offscreenPlaying = false;
      }
    }
  }
};
context.globalThis = context;
vm.createContext(context);
context.importScripts = (...files) => {
  for (const file of files) {
    const target = path.join(__dirname, '..', 'extensions', 'shared', file);
    vm.runInContext(fs.readFileSync(target, 'utf8'), context, { filename: target });
  }
};

const workerPath = path.join(__dirname, '..', 'extensions', 'platform', 'chromium', 'service_worker.js');
vm.runInContext(fs.readFileSync(workerPath, 'utf8'), context, { filename: workerPath });

function dispatch(message) {
  return new Promise((resolve, reject) => {
    if (typeof listener !== 'function') return reject(new Error('service worker listener was not registered'));
    let settled = false;
    const sendResponse = value => {
      if (settled) return;
      settled = true;
      resolve(value);
    };
    try {
      const result = listener(message, {}, sendResponse);
      if (result !== true && !settled) {
        settled = true;
        resolve(result);
      }
    } catch (error) {
      reject(error);
    }
  });
}

async function flush() {
  for (let i = 0; i < 5; i += 1) await new Promise(resolve => setImmediate(resolve));
}

async function main() {
  assert.equal(typeof listener, 'function', 'Chromium service worker must register a runtime listener');

  const first = await dispatch({ type: 'RB_PLAY', station: stationA });
  assert.equal(first.station.name, 'Radio A');
  assert.equal(first.playing, true);
  assert.equal(createCalls, 1, 'first playback must create one offscreen document');

  delayedStop = deferred();
  const oldSession = offscreenSession;
  const staleStop = dispatch({ type: 'RB_STOP' });
  await flush();
  const newerPlay = await dispatch({ type: 'RB_PLAY', station: stationB });
  assert.equal(newerPlay.station.name, 'Radio B');
  assert.equal(newerPlay.playing, true);
  delayedStop.resolve({ ok: true, station: stationA, playing: false, sessionId: oldSession, generation: offscreenGeneration + 1 });
  await staleStop;
  await flush();
  delayedStop = null;
  assert.equal(closeCalls, 0, 'stale stop must not close the offscreen document used by a newer play');
  const afterStaleStop = await dispatch({ type: 'RB_GET_STATE' });
  assert.equal(afterStaleStop.station.name, 'Radio B', 'stale stop must not replace the newer station');
  assert.equal(afterStaleStop.playing, true, 'stale stop must not stop newer playback');

  closeGate = deferred();
  const freshStop = dispatch({ type: 'RB_STOP' });
  await flush();
  assert.equal(closeCalls, 1, 'fresh stop must begin closing the current offscreen document');
  const playDuringClose = dispatch({ type: 'RB_PLAY', station: stationC });
  await flush();
  assert.equal(createCalls, 1, 'new play must wait until the previous offscreen close completes');
  closeGate.resolve();
  const stopped = await freshStop;
  const newest = await playDuringClose;
  closeGate = null;
  assert.equal(stopped.stale, undefined, 'completed stop response may return the current snapshot without reviving old playback');
  assert.equal(createCalls, 2, 'new play must recreate offscreen only after close completion');
  assert.equal(newest.station.name, 'Radio C');
  assert.equal(newest.playing, true, 'play issued during close must recover into active playback');
  const finalState = await dispatch({ type: 'RB_GET_STATE' });
  assert.equal(finalState.station.name, 'Radio C');
  assert.equal(finalState.playing, true);

  console.log('Chromium player close/session race regression tests OK');
}

main().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
