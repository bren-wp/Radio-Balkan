'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

class Element {
  constructor(id = '') {
    this.id = id;
    this.listeners = {};
    this.attributes = {};
    this.hidden = false;
    this.value = '';
    this.textContent = '';
    this.src = '';
    this.options = [];
    this.focused = false;
    this.clicks = 0;
  }
  addEventListener(type, listener) {
    (this.listeners[type] ||= []).push(listener);
  }
  dispatch(type, event = {}) {
    const payload = { target: this, preventDefault() {}, stopImmediatePropagation() {}, ...event };
    for (const listener of this.listeners[type] || []) listener(payload);
  }
  dispatchEvent(event) {
    this.dispatch(event.type, event);
    return true;
  }
  setAttribute(name, value) {
    this.attributes[name] = String(value);
  }
  focus() {
    this.focused = true;
  }
  click() {
    this.clicks += 1;
    this.dispatch('click');
  }
  closest() {
    return null;
  }
}

const ids = [
  'stations', 'country', 'stationPanel', 'stationPanelTitle', 'stationDetailMeta',
  'stationDescription', 'stationFacts', 'stationDetailLogo', 'stationDetailPlay',
  'stationDetailClose', 'quickAll', 'quickDiaspora', 'quickForeign'
];
const elements = Object.fromEntries(ids.map(id => [id, new Element(id)]));
elements.stationPanel.hidden = true;
elements.country.options = [{ value: '' }, { value: 'HR' }, { value: 'DIA' }, { value: 'INT' }];

let countryChanges = 0;
elements.country.addEventListener('change', () => { countryChanges += 1; });

const play = new Element('row-play');
const row = new Element('station-row');
const fields = {
  '.stationText strong': { textContent: 'Radio Dijaspora' },
  '.stationText span': { textContent: 'Dijaspora · Germany · 128 kbps' },
  '.stationText em': { textContent: 'Dijaspora' },
  '.logoBox img': { src: 'https://cdn.example.com/logo.png' },
  '.stationPlay': play
};
row.querySelector = selector => fields[selector] || null;

const rowTarget = {
  closest(selector) {
    if (selector === '[data-more]' || selector === '.stationPlay' || selector === 'button') return null;
    if (selector === '.station') return row;
    return null;
  }
};

class Event {
  constructor(type, init = {}) {
    this.type = type;
    Object.assign(this, init);
  }
}

class MutationObserver {
  constructor(callback) { this.callback = callback; }
  observe() {}
}

const document = {
  getElementById(id) {
    return elements[id] || null;
  }
};

const source = fs.readFileSync(path.join(__dirname, '..', 'extensions', 'shared', 'enhancements.js'), 'utf8');
vm.runInNewContext(source, { document, Event, MutationObserver, Array, String }, { filename: 'enhancements.js' });

assert.equal(elements.quickAll.attributes['aria-pressed'], 'true', 'all-stations quick filter must be selected initially');

let prevented = false;
let stopped = false;
elements.stations.dispatch('click', {
  target: rowTarget,
  preventDefault() { prevented = true; },
  stopImmediatePropagation() { stopped = true; }
});
assert.equal(prevented, true, 'station-card click must stop the legacy autoplay click path');
assert.equal(stopped, true, 'station-card click must not reach the legacy row autoplay handler');
assert.equal(elements.stationPanel.hidden, false, 'station-card click must open public station details');
assert.equal(elements.stationPanelTitle.textContent, 'Radio Dijaspora');
assert.match(elements.stationDescription.textContent, /Radio Balkan/);
assert.equal(elements.stationDescription.textContent.includes('https://'), false, 'public detail description must not expose a stream URL');
assert.equal(elements.stationDetailLogo.src, 'https://cdn.example.com/logo.png');
assert.equal(play.clicks, 0, 'opening details must not implicitly start playback');

elements.stationDetailPlay.click();
assert.equal(elements.stationPanel.hidden, true, 'detail play must close the detail panel');
assert.equal(play.clicks, 1, 'detail play must delegate to the existing station playback button');

elements.quickDiaspora.click();
assert.equal(elements.country.value, 'DIA');
assert.equal(elements.quickDiaspora.attributes['aria-pressed'], 'true');
assert.ok(countryChanges >= 1, 'diaspora quick filter must reuse the existing country-change contract');

elements.quickForeign.click();
assert.equal(elements.country.value, 'INT');
assert.equal(elements.quickForeign.attributes['aria-pressed'], 'true');

elements.quickAll.click();
assert.equal(elements.country.value, '');
assert.equal(elements.quickAll.attributes['aria-pressed'], 'true');

console.log('Browser station-detail and quick-area UI regression tests OK');
