(() => {
  'use strict';

  const $ = id => document.getElementById(id);
  const list = $('stations');
  const country = $('country');
  const panel = $('stationPanel');
  let selectedRow = null;

  if (!list || !country || !panel) return;

  function setPressed(button, pressed) {
    if (button) button.setAttribute('aria-pressed', String(!!pressed));
  }

  function updateQuickAreas() {
    const value = String(country.value || '').toUpperCase();
    setPressed($('quickAll'), !value);
    setPressed($('quickDiaspora'), value === 'DIA');
    setPressed($('quickForeign'), value === 'INT');
  }

  function selectArea(code) {
    const wanted = String(code || '').toUpperCase();
    const options = Array.from(country.options || []);
    country.value = options.some(option => option.value === wanted) ? wanted : '';
    country.dispatchEvent(new Event('change', { bubbles: true }));
    updateQuickAreas();
  }

  function rowText(row, selector) {
    return String(row?.querySelector?.(selector)?.textContent || '').trim();
  }

  function openDetails(row) {
    if (!row) return;
    selectedRow = row;
    const name = rowText(row, '.stationText strong') || 'Radio stanica';
    const meta = rowText(row, '.stationText span') || 'Radio uživo';
    const category = rowText(row, '.stationText em') || 'Radio uživo';

    $('stationPanelTitle').textContent = name;
    $('stationDetailMeta').textContent = meta;
    $('stationDescription').textContent = name + ' je radio stanica iz kataloga Radio Balkan. ' + meta + '. Kategorija: ' + category + '. Pokreni slušanje tipkom ispod; player će koristiti postojeće timeout, fallback i recovery zaštite.';
    $('stationFacts').textContent = [meta, category].filter(Boolean).join(' · ');

    const detailLogo = $('stationDetailLogo');
    const rowLogo = row.querySelector?.('.logoBox img');
    detailLogo.onerror = null;
    detailLogo.src = rowLogo?.src || 'icon48.png';
    detailLogo.onerror = () => {
      detailLogo.onerror = null;
      detailLogo.src = 'icon48.png';
    };

    panel.hidden = false;
    $('stationDetailPlay')?.focus?.();
  }

  function closeDetails() {
    const row = selectedRow;
    selectedRow = null;
    panel.hidden = true;
    row?.focus?.();
  }

  list.addEventListener('click', event => {
    if (event.target.closest('[data-more]') || event.target.closest('.stationPlay')) return;
    const row = event.target.closest('.station');
    if (!row) return;
    event.preventDefault();
    event.stopImmediatePropagation();
    openDetails(row);
  }, true);

  list.addEventListener('keydown', event => {
    if (event.key !== 'Enter' && event.key !== ' ') return;
    if (event.target.closest('button')) return;
    const row = event.target.closest('.station');
    if (!row) return;
    event.preventDefault();
    event.stopImmediatePropagation();
    openDetails(row);
  }, true);

  $('stationDetailClose')?.addEventListener('click', closeDetails);
  panel.addEventListener('click', event => {
    if (event.target === panel) closeDetails();
  });
  panel.addEventListener('keydown', event => {
    if (event.key !== 'Escape') return;
    event.preventDefault();
    closeDetails();
  });
  $('stationDetailPlay')?.addEventListener('click', () => {
    const play = selectedRow?.querySelector?.('.stationPlay');
    if (!play) return;
    closeDetails();
    play.click();
  });

  $('quickAll')?.addEventListener('click', () => selectArea(''));
  $('quickDiaspora')?.addEventListener('click', () => selectArea('DIA'));
  $('quickForeign')?.addEventListener('click', () => selectArea('INT'));
  country.addEventListener('change', updateQuickAreas);

  if (typeof MutationObserver === 'function') {
    new MutationObserver(updateQuickAreas).observe(country, { childList: true });
  }
  updateQuickAreas();
})();
