'use strict';

let audio = null;
let station = null;
let candidates = [];
let index = 0;
let generation = 0;
let playing = false;
let sessionId = null;

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

function disposeAudio(target = audio) {
  if (!target) return;
  if (audio === target) audio = null;
  target.onerror = null;
  target.onended = null;
  target.onplaying = null;
  target.onpause = null;
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

function createAudio(token, expectedSession, candidate) {
  const instance = document.createElement('audio');
  instance.preload = 'none';
  instance.src = candidate;
  instance.onerror = () => playbackFailed(token, expectedSession, instance);
  instance.onended = () => playbackFailed(token, expectedSession, instance);
  instance.onplaying = () => {
    if (token !== generation || sessionId !== expectedSession || audio !== instance) return;
    playing = true;
    void report();
  };
  instance.onpause = () => {
    if (token !== generation || sessionId !== expectedSession || audio !== instance || !playing) return;
    playing = false;
    void report();
  };
  audio = instance;
  return instance;
}

async function start(expectedSession) {
  const token = ++generation;
  const expectedStation = station;
  while (index < candidates.length && token === generation && sessionId === expectedSession) {
    const candidate = candidates[index];
    playing = false;
    disposeAudio();
    const instance = createAudio(token, expectedSession, candidate);
    try {
      await instance.play();
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
    start(requestedSession)
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
    void report();
    sendResponse({ ...stateEnvelope({ ok: true }) });
    return;
  }

  if (msg.type === 'GET_STATE') {
    sendResponse({ ...stateEnvelope({ ok: true }) });
  }
});
