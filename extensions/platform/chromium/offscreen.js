'use strict';

const audio = document.getElementById('audio');
audio.preload = 'none';

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
      playing: !!playing && !audio.paused
    });
  } catch { }
}

function resetAudio() {
  audio.pause();
  audio.removeAttribute('src');
  audio.load();
  playing = false;
}

async function start() {
  const token = ++generation;
  while (index < candidates.length && token === generation) {
    const candidate = candidates[index];
    playing = false;
    audio.pause();
    audio.src = candidate;
    try {
      await audio.play();
      if (token !== generation) return false;
      playing = true;
      await report();
      return true;
    } catch {
      if (token !== generation) return false;
      index += 1;
    }
  }
  if (token === generation) {
    resetAudio();
    await report();
  }
  return false;
}

audio.addEventListener('error', () => {
  if (!playing) return;
  playing = false;
  index += 1;
  void start();
});

audio.addEventListener('playing', () => {
  playing = true;
  void report();
});

audio.addEventListener('pause', () => {
  if (!playing) return;
  playing = false;
  void report();
});

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
    if (!audio.paused) {
      generation += 1;
      audio.pause();
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
    sendResponse({ ok: true, station, playing: !!playing && !audio.paused });
  }
});
