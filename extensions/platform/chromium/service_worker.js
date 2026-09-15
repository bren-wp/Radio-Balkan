importScripts('network.js');

let state = { station: null, playing: false };

function validStation(station) { return RBNet.validStation(station); }

async function hasOffscreen() {
  return !!(chrome.offscreen?.hasDocument && await chrome.offscreen.hasDocument());
}
async function ensureOffscreen() {
  if (!chrome.offscreen) throw new Error('Reprodukcija u pozadini nije podržana u ovom pregledniku');
  if (await hasOffscreen()) return;
  await chrome.offscreen.createDocument({
    url: 'offscreen.html', reasons: ['AUDIO_PLAYBACK'],
    justification: 'Reprodukcija korisnički odabrane radio stanice dok je popup zatvoren.'
  });
}
async function offscreen(message) {
  const result = await chrome.runtime.sendMessage({ target: 'offscreen', ...message });
  if (result?.station && validStation(result.station)) state.station = result.station;
  state.playing = !!result?.playing;
  return result;
}
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (!msg || msg.target === 'offscreen') return;
  (async () => {
    try {
      if (msg.type === 'RB_PLAY') {
        if (!validStation(msg.station)) throw new Error('Stanica nije iz podržane regije ili nema siguran stream');
        await ensureOffscreen();
        state.station = msg.station;
        const actual = await offscreen({ type: 'PLAY', station: msg.station });
        if (!actual?.ok || !actual.playing) return { ...state, error: 'Stanica trenutačno nije dostupna' };
        return state;
      }
      if (msg.type === 'RB_TOGGLE') {
        if (!validStation(state.station)) { state.playing = false; return state; }
        await ensureOffscreen();
        const actual = await offscreen({ type: 'TOGGLE' });
        if (!actual?.ok && !actual?.playing) return { ...state, error: 'Reprodukcija trenutačno nije dostupna' };
        return state;
      }
      if (msg.type === 'RB_STOP') {
        if (await hasOffscreen()) {
          await offscreen({ type: 'STOP' });
          try { await chrome.offscreen.closeDocument(); } catch { }
        }
        state.playing = false;
        return state;
      }
      if (msg.type === 'RB_GET_STATE') {
        if (await hasOffscreen()) {
          try { await offscreen({ type: 'GET_STATE' }); } catch { state.playing = false; }
        } else state.playing = false;
        return state;
      }
      if (msg.type === 'RB_OFFSCREEN_STATE') {
        if (msg.station && validStation(msg.station)) state.station = msg.station;
        state.playing = !!msg.playing;
        chrome.runtime.sendMessage({ type: 'RB_STATE', ...state }).catch(() => {});
        return { ok: true };
      }
      return undefined;
    } catch (error) {
      state.playing = false;
      return { ...state, error: String(error?.message || error) };
    }
  })().then(sendResponse);
  return true;
});
