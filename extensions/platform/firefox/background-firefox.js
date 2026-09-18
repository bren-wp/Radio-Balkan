'use strict';

const api = globalThis.browser || globalThis.chrome;
const epoch = `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
const state = { station: null, playing: false, revision: 0, epoch };
let audio = null;
let candidates = [];
let idx = 0;
let generation = 0;
let sessionCounter = 0;
let currentSessionId = null;
let stallTimer = null;
let stallTarget = null;
const PLAY_START_TIMEOUT_MS = 12_000;
const STALL_RECOVERY_TIMEOUT_MS = 15_000;

function urls(station) { return RBNet.candidateUrls(station); }

function snapshot(extra = {}) {
  return { ...state, sessionId: currentSessionId, generation, ...extra };
}

function report() {
  Promise.resolve(api.runtime.sendMessage({ type: 'RB_STATE', ...snapshot() })).catch(() => {});
}

function commitState(patch, notify = true) {
  if (Object.prototype.hasOwnProperty.call(patch, 'station')) state.station = patch.station;
  if (Object.prototype.hasOwnProperty.call(patch, 'playing')) state.playing = !!patch.playing;
  state.revision += 1;
  if (notify) report();
  return snapshot();
}

function newSessionId() {
  sessionCounter += 1;
  return `${epoch}:${sessionCounter}`;
}

function clearStallTimer(target = null) {
  if (target && stallTarget && stallTarget !== target) return;
  if (stallTimer) clearTimeout(stallTimer);
  stallTimer = null;
  stallTarget = null;
}

function scheduleStallRecovery(token, expectedSession, instance) {
  clearStallTimer();
  stallTarget = instance;
  stallTimer = setTimeout(() => {
    stallTimer = null;
    stallTarget = null;
    const sessionMatches = currentSessionId === expectedSession;
    if (token !== generation || !sessionMatches || audio !== instance) return;
    state.playing = false;
    idx += 1;
    disposeAudio(instance);
    void start(expectedSession);
  }, STALL_RECOVERY_TIMEOUT_MS);
}

async function playWithTimeout(instance) {
  let timeout = null;
  try {
    await Promise.race([
      instance.play(),
      new Promise((_, reject) => {
        timeout = setTimeout(() => reject(new Error('playback start timeout')), PLAY_START_TIMEOUT_MS);
      })
    ]);
  } finally {
    if (timeout) clearTimeout(timeout);
  }
}

function disposeAudio(target = audio) {
  if (!target) return;
  clearStallTimer(target);
  if (audio === target) audio = null;
  target.onerror = null;
  target.onended = null;
  target.onplaying = null;
  target.onpause = null;
  target.onwaiting = null;
  target.onstalled = null;
  try { target.pause(); } catch { }
  try {
    target.removeAttribute('src');
    target.load();
  } catch { }
}

function resetAudio() {
  disposeAudio();
  state.playing = false;
}

function playbackFailed(token, expectedSession, instance) {
  if (token !== generation || currentSessionId !== expectedSession || audio !== instance || !state.playing) return;
  state.playing = false;
  idx += 1;
  disposeAudio(instance);
  void start(expectedSession);
}

function createAudio(token, expectedSession, candidate) {
  const instance = document.createElement('audio');
  instance.preload = 'none';
  instance.src = candidate;
  instance.onerror = () => playbackFailed(token, expectedSession, instance);
  instance.onended = () => playbackFailed(token, expectedSession, instance);
  instance.onplaying = () => {
    clearStallTimer(instance);
    if (token !== generation || currentSessionId !== expectedSession || audio !== instance) return;
    if (!state.playing) commitState({ playing: true });
  };
  instance.onpause = () => {
    clearStallTimer(instance);
    if (token !== generation || currentSessionId !== expectedSession || audio !== instance || !state.playing) return;
    commitState({ playing: false });
  };
  instance.onwaiting = () => scheduleStallRecovery(token, expectedSession, instance);
  instance.onstalled = () => scheduleStallRecovery(token, expectedSession, instance);
  audio = instance;
  return instance;
}

async function start(expectedSession) {
  const token = ++generation;
  while (idx < candidates.length && token === generation && currentSessionId === expectedSession) {
    const candidate = candidates[idx];
    state.playing = false;
    disposeAudio();
    const instance = createAudio(token, expectedSession, candidate);
    try {
      await playWithTimeout(instance);
      if (token !== generation || currentSessionId !== expectedSession || audio !== instance) {
        return { ok: false, stale: true };
      }
      if (!state.playing) commitState({ playing: true });
      return { ok: true };
    } catch {
      if (token !== generation || currentSessionId !== expectedSession || audio !== instance) {
        return { ok: false, stale: true };
      }
      disposeAudio(instance);
      idx += 1;
    }
  }
  if (token === generation && currentSessionId === expectedSession) {
    resetAudio();
    commitState({ playing: false });
  }
  return { ok: false };
}

api.runtime.onMessage.addListener(async msg => {
  if (!msg) return;

  if (msg.type === 'RB_PLAY') {
    const list = urls(msg.station);
    if (!list.length) return snapshot({ error: 'Stanica nije dostupna ili URL nije dopušten' });
    const requestedSession = newSessionId();
    currentSessionId = requestedSession;
    generation += 1;
    resetAudio();
    candidates = list;
    idx = 0;
    commitState({ station: msg.station, playing: false });
    const result = await start(requestedSession);
    if (currentSessionId !== requestedSession) return snapshot({ stale: true });
    return result.ok ? snapshot() : snapshot({ error: 'Stanica trenutačno nije dostupna' });
  }

  if (msg.type === 'RB_TOGGLE') {
    const requestedSession = currentSessionId;
    if (!requestedSession || !state.station || !candidates.length) return snapshot();
    if (audio && !audio.paused) {
      generation += 1;
      try { audio.pause(); } catch { }
      state.playing = false;
      commitState({ playing: false });
      return snapshot();
    }
    if (idx >= candidates.length) idx = 0;
    const result = await start(requestedSession);
    if (currentSessionId !== requestedSession) return snapshot({ stale: true });
    return result.ok ? snapshot() : snapshot({ error: 'Reprodukcija trenutačno nije dostupna' });
  }

  if (msg.type === 'RB_STOP') {
    generation += 1;
    resetAudio();
    currentSessionId = null;
    candidates = [];
    idx = 0;
    commitState({ playing: false });
    return snapshot();
  }

  if (msg.type === 'RB_GET_STATE') {
    const actualPlaying = !!state.playing && !!audio && !audio.paused;
    if (actualPlaying !== state.playing) commitState({ playing: actualPlaying });
    return snapshot();
  }
});
