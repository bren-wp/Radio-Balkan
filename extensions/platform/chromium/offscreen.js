'use strict';

let audio = null;
let station = null;
let candidates = [];
let index = 0;
let generation = 0;
let playing = false;

function urls(value) { return RBNet.candidateUrls(value); }

async function report() {
  try {
    await chrome.runtime.sendMessage({
      type: 'RB_OFFSCREEN_STATE',
      station,
      playing: !!playing && !!audio && !audio.paused
    });
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

function playbackFailed(token, instance) {
  if (token !== generation || audio !== instance || !playing) return;
  playing = false;
  index += 1;
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
    playing = true;
    void report();
  };
  instance.onpause = () => {
    if (token !== generation || audio !== instance || !playing) return;
    playing = false;
    void report();
  };
  audio = instance;
  return instance;
}

async function start() {
  const token = ++generation;
  while (index < candidates.length && token === generation) {
    const candidate = candidates[index];
    playing = false;
    disposeAudio();
    const instance = createAudio(token, candidate);
    try {
      await instance.play();
      if (token !== generation || audio !== instance) return false;
      playing = true;
      await report();
      return true;
    } catch {
      if (token !== generation || audio !== instance) return false;
      disposeAudio(instance);
      index += 1;
    }
  }
  if (token === generation) {
    resetAudio();
    await report();
  }
  return false;
}

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg?.target !== 'offscreen') return;

  if (msg.type === 'PLAY') {
    generation += 1;
    resetAudio();
    station = msg.station;
    candidates = urls(station);
    index = 0;
    if (!candidates.length) {
      void report();
      sendResponse({ ok: false, station, playing: false });
      return;
    }
    start()
      .then(ok => sendResponse({ ok, station, playing: ok }))
      .catch(() => sendResponse({ ok: false, station, playing: false }));
    return true;
  }

  if (msg.type === 'TOGGLE') {
    if (!station || !candidates.length) {
      sendResponse({ ok: false, station, playing: false });
      return;
    }
    if (audio && !audio.paused) {
      generation += 1;
      try { audio.pause(); } catch { }
      playing = false;
      void report();
      sendResponse({ ok: true, station, playing: false });
      return;
    }
    if (index >= candidates.length) index = 0;
    start()
      .then(ok => sendResponse({ ok, station, playing: ok }))
      .catch(() => sendResponse({ ok: false, station, playing: false }));
    return true;
  }

  if (msg.type === 'STOP') {
    generation += 1;
    resetAudio();
    void report();
    sendResponse({ ok: true, station, playing: false });
    return;
  }

  if (msg.type === 'GET_STATE') {
    sendResponse({ ok: true, station, playing: !!playing && !!audio && !audio.paused });
  }
});
