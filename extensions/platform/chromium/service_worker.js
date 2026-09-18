'use strict';

importScripts('network.js');

const epoch = `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
let state = { station: null, playing: false, revision: 0, epoch };
let offscreenCreating = null;
let offscreenClosing = null;
let commandGeneration = 0;
let sessionCounter = 0;
let currentSessionId = null;
let lastOffscreenGeneration = -1;

function validStation(station) { return RBNet.validStation(station); }

function snapshot(extra = {}) {
  return { ...state, sessionId: currentSessionId, ...extra };
}

function broadcastState() {
  chrome.runtime.sendMessage({ type: 'RB_STATE', ...snapshot() }).catch(() => {});
}

function commitState(patch, notify = false) {
  if (Object.prototype.hasOwnProperty.call(patch, 'station')) state.station = patch.station;
  if (Object.prototype.hasOwnProperty.call(patch, 'playing')) state.playing = !!patch.playing;
  state.revision += 1;
  if (notify) broadcastState();
  return snapshot();
}

function newSessionId() {
  sessionCounter += 1;
  return `${epoch}:${sessionCounter}`;
}

function numericGeneration(value) {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed >= 0 ? parsed : null;
}

function acceptOffscreenState(value, { adopt = false, notify = false } = {}) {
  if (!value || typeof value !== 'object') return false;
  const incomingSession = String(value.sessionId || '');
  const incomingGeneration = numericGeneration(value.generation);
  if (!incomingSession || incomingGeneration === null) return false;

  if (!currentSessionId) {
    if (!adopt) return false;
    currentSessionId = incomingSession;
    lastOffscreenGeneration = -1;
  }
  if (incomingSession !== currentSessionId || incomingGeneration < lastOffscreenGeneration) return false;

  lastOffscreenGeneration = incomingGeneration;
  const patch = { playing: !!value.playing };
  if (value.station && validStation(value.station)) patch.station = value.station;
  commitState(patch, notify);
  return true;
}

async function hasOffscreen() {
  return !!(chrome.offscreen?.hasDocument && await chrome.offscreen.hasDocument());
}

async function waitForOffscreenClose() {
  if (offscreenClosing) await offscreenClosing;
}

async function ensureOffscreen() {
  if (!chrome.offscreen) throw new Error('Reprodukcija u pozadini nije podržana u ovom pregledniku');
  await waitForOffscreenClose();
  if (await hasOffscreen()) return;
  if (!offscreenCreating) {
    offscreenCreating = chrome.offscreen.createDocument({
      url: 'offscreen.html',
      reasons: ['AUDIO_PLAYBACK'],
      justification: 'Reprodukcija korisnički odabrane radio stanice dok je popup zatvoren.'
    }).finally(() => { offscreenCreating = null; });
  }
  await offscreenCreating;
}

async function closeOffscreen() {
  if (!offscreenClosing) {
    let closing;
    closing = Promise.resolve(chrome.offscreen.closeDocument())
      .catch(() => {})
      .finally(() => {
        if (offscreenClosing === closing) offscreenClosing = null;
      });
    offscreenClosing = closing;
  }
  await offscreenClosing;
}

async function offscreen(message) {
  return chrome.runtime.sendMessage({ target: 'offscreen', ...message });
}

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (!msg || msg.target === 'offscreen') return;
  (async () => {
    let requestToken = commandGeneration;
    try {
      if (msg.type === 'RB_PLAY') {
        if (!validStation(msg.station)) throw new Error('Stanica nije iz podržane regije ili nema siguran stream');
        requestToken = ++commandGeneration;
        const requestedSession = newSessionId();
        currentSessionId = requestedSession;
        lastOffscreenGeneration = -1;
        commitState({ station: msg.station, playing: false }, true);
        await ensureOffscreen();
        if (requestToken !== commandGeneration || requestedSession !== currentSessionId) return snapshot({ stale: true });
        const actual = await offscreen({ type: 'PLAY', station: msg.station, sessionId: requestedSession });
        if (requestToken !== commandGeneration || requestedSession !== currentSessionId) return snapshot({ stale: true });
        acceptOffscreenState(actual, { notify: true });
        if (!actual?.ok || !state.playing) return snapshot({ error: 'Stanica trenutačno nije dostupna' });
        return snapshot();
      }

      if (msg.type === 'RB_TOGGLE') {
        requestToken = ++commandGeneration;
        if (!validStation(state.station) || !currentSessionId) {
          commitState({ playing: false }, true);
          return snapshot();
        }
        const requestedSession = currentSessionId;
        await ensureOffscreen();
        if (requestToken !== commandGeneration || requestedSession !== currentSessionId) return snapshot({ stale: true });
        const actual = await offscreen({ type: 'TOGGLE', sessionId: requestedSession });
        if (requestToken !== commandGeneration || requestedSession !== currentSessionId) return snapshot({ stale: true });
        acceptOffscreenState(actual, { notify: true });
        if (!actual?.ok && !state.playing) return snapshot({ error: 'Reprodukcija trenutačno nije dostupna' });
        return snapshot();
      }

      if (msg.type === 'RB_STOP') {
        requestToken = ++commandGeneration;
        const requestedSession = currentSessionId;
        if (await hasOffscreen()) {
          const actual = await offscreen({ type: 'STOP', sessionId: requestedSession });
          if (requestToken === commandGeneration && requestedSession === currentSessionId) {
            acceptOffscreenState(actual, { notify: true });
            await closeOffscreen();
          }
        }
        if (requestToken === commandGeneration && requestedSession === currentSessionId) {
          currentSessionId = null;
          lastOffscreenGeneration = -1;
          commitState({ playing: false }, true);
        }
        return snapshot();
      }

      if (msg.type === 'RB_GET_STATE') {
        requestToken = commandGeneration;
        await waitForOffscreenClose();
        if (await hasOffscreen()) {
          try {
            const actual = await offscreen({ type: 'GET_STATE' });
            if (requestToken === commandGeneration) {
              acceptOffscreenState(actual, { adopt: !currentSessionId, notify: false });
            }
          } catch {
            if (requestToken === commandGeneration) commitState({ playing: false }, false);
          }
        } else if (requestToken === commandGeneration && state.playing) {
          commitState({ playing: false }, false);
        }
        return snapshot();
      }

      if (msg.type === 'RB_OFFSCREEN_STATE') {
        const adopted = acceptOffscreenState(msg, {
          adopt: commandGeneration === 0 && !currentSessionId,
          notify: true
        });
        return { ok: adopted, ...snapshot() };
      }

      return undefined;
    } catch (error) {
      if (requestToken === commandGeneration) commitState({ playing: false }, true);
      return snapshot({ error: String(error?.message || error) });
    }
  })().then(sendResponse);
  return true;
});
