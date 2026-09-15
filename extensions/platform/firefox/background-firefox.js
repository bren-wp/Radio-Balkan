'use strict';

const api = globalThis.browser || globalThis.chrome;
const state = { station: null, playing: false };
let audio = null;
let candidates = [];
let idx = 0;
let generation = 0;

function urls(station) { return RBNet.candidateUrls(station); }

function report() {
  Promise.resolve(api.runtime.sendMessage({ type: 'RB_STATE', ...state })).catch(() => {});
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
  state.playing = false;
}

function playbackFailed(token, instance) {
  if (token !== generation || audio !== instance || !state.playing) return;
  state.playing = false;
  idx += 1;
  disposeAudio(instance);
  void start();
}

function createAudio(token, candidate) {
  const instance = document.createElement('audio');
  instance.preload = 'none';
  instance.src = candidate;
  instance.onerror = () => playbackFailed(token, instance);
  instance.onended = () => playbackFailed(token, instance);
  instance.onplaying = () => {
    if (token !== generation || audio !== instance) return;
    state.playing = true;
    report();
  };
  instance.onpause = () => {
    if (token !== generation || audio !== instance || !state.playing) return;
    state.playing = false;
    report();
  };
  audio = instance;
  return instance;
}

async function start() {
  const token = ++generation;
  while (idx < candidates.length && token === generation) {
    const candidate = candidates[idx];
    state.playing = false;
    disposeAudio();
    const instance = createAudio(token, candidate);
    try {
      await instance.play();
      if (token !== generation || audio !== instance) return false;
      state.playing = true;
      report();
      return true;
    } catch {
      if (token !== generation || audio !== instance) return false;
      disposeAudio(instance);
      idx += 1;
    }
  }
  if (token === generation) {
    resetAudio();
    report();
  }
  return false;
}

api.runtime.onMessage.addListener(async msg => {
  if (!msg) return;

  if (msg.type === 'RB_PLAY') {
    const list = urls(msg.station);
    if (!list.length) return { ...state, error: 'Stanica nije dostupna ili URL nije dopušten' };
    generation += 1;
    resetAudio();
    state.station = msg.station;
    candidates = list;
    idx = 0;
    const ok = await start();
    return ok ? { ...state } : { ...state, error: 'Stanica trenutačno nije dostupna' };
  }

  if (msg.type === 'RB_TOGGLE') {
    if (!state.station || !candidates.length) return { ...state };
    if (audio && !audio.paused) {
      generation += 1;
      try { audio.pause(); } catch { }
      state.playing = false;
      report();
      return { ...state };
    }
    if (idx >= candidates.length) idx = 0;
    const ok = await start();
    return ok ? { ...state } : { ...state, error: 'Reprodukcija trenutačno nije dostupna' };
  }

  if (msg.type === 'RB_STOP') {
    generation += 1;
    resetAudio();
    report();
    return { ...state };
  }

  if (msg.type === 'RB_GET_STATE') {
    return { ...state, playing: !!state.playing && !!audio && !audio.paused };
  }
});
