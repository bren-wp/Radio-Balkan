const api = globalThis.browser || globalThis.chrome;
const audio = document.getElementById('audio');
audio.preload = 'none';
function urls(station) { return RBNet.candidateUrls(station); }

function report() { Promise.resolve(api.runtime.sendMessage({ type: 'RB_STATE', ...state })).catch(() => {}); }
function resetAudio() { audio.pause(); audio.removeAttribute('src'); audio.load(); state.playing = false; }
async function start() {
  const token = ++generation;
  while (idx < candidates.length && token === generation) {
    state.playing = false;
    audio.pause();
    audio.src = candidates[idx];
    try {
      await audio.play();
      if (token !== generation) return false;
      state.playing = true;
      report();
      return true;
    } catch { idx += 1; }
  }
  if (token === generation) { resetAudio(); report(); }
  return false;
}
audio.addEventListener('error', () => { if (state.playing) { state.playing = false; idx += 1; void start(); } });
audio.addEventListener('playing', () => { state.playing = true; report(); });
audio.addEventListener('pause', () => { if (state.playing) { state.playing = false; report(); } });
api.runtime.onMessage.addListener(async msg => {
  if (!msg) return;
  if (msg.type === 'RB_PLAY') {
    const list = urls(msg.station);
    if (!list.length) return { ...state, error: 'Stanica nije dostupna ili URL nije dopušten' };
    state.station = msg.station; candidates = list; idx = 0;
    const ok = await start();
    return ok ? state : { ...state, error: 'Stanica trenutačno nije dostupna' };
  }
  if (msg.type === 'RB_TOGGLE') {
    if (!state.station || !candidates.length) return state;
    if (!audio.paused) { generation += 1; audio.pause(); state.playing = false; report(); return state; }
    if (idx >= candidates.length) idx = 0;
    const ok = await start();
    return ok ? state : { ...state, error: 'Reprodukcija trenutačno nije dostupna' };
  }
  if (msg.type === 'RB_STOP') { generation += 1; resetAudio(); report(); return state; }
  if (msg.type === 'RB_GET_STATE') return { ...state, playing: !!state.playing && !audio.paused };
});
