'use strict';

let audio = null;
let station = null;
let candidates = [];
let index = 0;
let generation = 0;
let playing = false;
let sessionId = null;
let stallTimer = null;
let stallTarget = null;
let refreshAttempted = false;
const PLAY_START_TIMEOUT_MS = 12_000;
const STALL_RECOVERY_TIMEOUT_MS = 15_000;
const CONNECTION_ATTEMPT_BUDGET_MS = 36_000;

function urls(value) { return RBNet.candidateUrls(value); }

function stateEnvelope(extra = {}) {
  return {
    station,
    playing: !!playing && !!audio && !audio.paused,
    sessionId,
    generation,
    ...extra
  };
}

async function report() {
  try {
    await chrome.runtime.sendMessage({ type: 'RB_OFFSCREEN_STATE', ...stateEnvelope() });
  } catch { }
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
    const sessionMatches = sessionId === expectedSession;
    if (token !== generation || !sessionMatches || audio !== instance) return;
    playing = false;
    index += 1;
    disposeAudio(instance);
    void start(expectedSession);
  }, STALL_RECOVERY_TIMEOUT_MS);
}

async function playWithTimeout(instance, timeoutMs = PLAY_START_TIMEOUT_MS) {
  let timeout = null;
  const boundedTimeout = Math.max(1, Math.min(PLAY_START_TIMEOUT_MS, Number(timeoutMs) || PLAY_START_TIMEOUT_MS));
  try {
    await Promise.race([
      instance.play(),
      new Promise((_, reject) => {
        timeout = setTimeout(() => reject(new Error('playback start timeout')), boundedTimeout);
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
  playing = false;
}

function playbackFailed(token, expectedSession, instance) {
  if (token !== generation || sessionId !== expectedSession || audio !== instance || !playing) return;
  playing = false;
  index += 1;
  disposeAudio(instance);
  void start(expectedSession);
}

function bindAudioHandlers(instance, token, expectedSession) {
  instance.onerror = () => playbackFailed(token, expectedSession, instance);
  instance.onended = () => playbackFailed(token, expectedSession, instance);
  instance.onplaying = () => {
    clearStallTimer(instance);
    if (token !== generation || sessionId !== expectedSession || audio !== instance) return;
    playing = true;
    void report();
  };
  instance.onpause = () => {
    clearStallTimer(instance);
    if (token !== generation || sessionId !== expectedSession || audio !== instance || !playing) return;
    playing = false;
    void report();
  };
  instance.onwaiting = () => scheduleStallRecovery(token, expectedSession, instance);
  instance.onstalled = () => scheduleStallRecovery(token, expectedSession, instance);
}

function createAudio(token, expectedSession, candidate) {
  const instance = document.createElement('audio');
  instance.preload = 'none';
  instance.src = candidate;
  bindAudioHandlers(instance, token, expectedSession);
  audio = instance;
  return instance;
}

async function resumeCurrent(expectedSession) {
  const instance = audio;
  if (!instance || !instance.paused || sessionId !== expectedSession) return start(expectedSession);
  const token = ++generation;
  bindAudioHandlers(instance, token, expectedSession);
  try {
    await playWithTimeout(instance);
    if (token !== generation || sessionId !== expectedSession || audio !== instance) {
      return { ...stateEnvelope({ ok: false, stale: true }) };
    }
    playing = true;
    await report();
    return { ...stateEnvelope({ ok: true }) };
  } catch {
    if (token !== generation || sessionId !== expectedSession || audio !== instance) {
      return { ...stateEnvelope({ ok: false, stale: true }) };
    }
    disposeAudio(instance);
    playing = false;
    return start(expectedSession);
  }
}

async function start(expectedSession, deadline = Date.now() + CONNECTION_ATTEMPT_BUDGET_MS) {
  const token = ++generation;
  let expectedStation = station;
  while (token === generation && sessionId === expectedSession && Date.now() < deadline) {
    while (index < candidates.length && token === generation && sessionId === expectedSession && Date.now() < deadline) {
      const candidate = candidates[index];
      playing = false;
      disposeAudio();
      const instance = createAudio(token, expectedSession, candidate);
      try {
        await playWithTimeout(instance, deadline - Date.now());
        if (token !== generation || sessionId !== expectedSession || audio !== instance) {
          return { ok: false, stale: true, station: expectedStation, playing: false, sessionId: expectedSession, generation: token };
        }
        playing = true;
        await report();
        return { ok: true, station: expectedStation, playing: true, sessionId: expectedSession, generation: token };
      } catch {
        if (token !== generation || sessionId !== expectedSession || audio !== instance) {
          return { ok: false, stale: true, station: expectedStation, playing: false, sessionId: expectedSession, generation: token };
        }
        disposeAudio(instance);
        index += 1;
      }
    }

    if (refreshAttempted || Date.now() >= deadline) break;
    refreshAttempted = true;
    let refreshed = [];
    try { refreshed = await RBNet.refreshCandidateUrls(expectedStation, deadline - Date.now()); } catch { }
    if (token !== generation || sessionId !== expectedSession) {
      return { ok: false, stale: true, station: expectedStation, playing: false, sessionId: expectedSession, generation: token };
    }
    const before = candidates.length;
    for (const value of refreshed || []) {
      if (RBNet.safeHttp(value) && !candidates.includes(value)) candidates.push(value);
    }
    if (candidates.length <= before) break;
    expectedStation = { ...expectedStation, url_resolved: refreshed[0] };
    station = expectedStation;
  }

  if (token === generation && sessionId === expectedSession) {
    resetAudio();
    await report();
  }
  return { ok: false, station: expectedStation, playing: false, sessionId: expectedSession, generation: token };
}

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg?.target !== 'offscreen') return;

  if (msg.type === 'PLAY') {
    const requestedSession = String(msg.sessionId || '');
    const requestedStation = msg.station;
    generation += 1;
    resetAudio();
    sessionId = requestedSession || null;
    station = requestedStation;
    candidates = urls(station);
    index = 0;
    refreshAttempted = false;
    if (!sessionId || !candidates.length) {
      void report();
      sendResponse({ ok: false, station: requestedStation, playing: false, sessionId, generation });
      return;
    }
    start(requestedSession)
      .then(sendResponse)
      .catch(() => sendResponse({ ok: false, station: requestedStation, playing: false, sessionId: requestedSession, generation }));
    return true;
  }

  if (msg.type === 'TOGGLE') {
    const requestedSession = String(msg.sessionId || '');
    if (!requestedSession || requestedSession !== sessionId || !station || !candidates.length) {
      sendResponse({ ...stateEnvelope({ ok: false, stale: requestedSession !== sessionId }) });
      return;
    }
    if (audio && !audio.paused) {
      generation += 1;
      try { audio.pause(); } catch { }
      playing = false;
      void report();
      sendResponse({ ...stateEnvelope({ ok: true }) });
      return;
    }
    if (index >= candidates.length) index = 0;
    resumeCurrent(requestedSession)
      .then(sendResponse)
      .catch(() => sendResponse({ ...stateEnvelope({ ok: false }) }));
    return true;
  }

  if (msg.type === 'STOP') {
    const requestedSession = String(msg.sessionId || '');
    if (requestedSession && sessionId && requestedSession !== sessionId) {
      sendResponse({ ...stateEnvelope({ ok: false, stale: true }) });
      return;
    }
    generation += 1;
    resetAudio();
    const response = { ...stateEnvelope({ ok: true }) };
    sessionId = null;
    candidates = [];
    index = 0;
    refreshAttempted = false;
    void report();
    sendResponse(response);
    return;
  }

  if (msg.type === 'GET_STATE') {
    sendResponse({ ...stateEnvelope({ ok: true }) });
  }
});
