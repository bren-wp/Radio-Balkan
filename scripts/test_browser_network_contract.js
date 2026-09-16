'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const context = { URL };
context.globalThis = context;
vm.createContext(context);

const networkPath = path.join(__dirname, '..', 'extensions', 'shared', 'network.js');
const source = fs.readFileSync(networkPath, 'utf8');
vm.runInContext(`${source}\nthis.__RBNet = RBNet;`, context, { filename: networkPath });
const RBNet = context.__RBNet;

assert.equal(RBNet.safeHttp('https://example.com/live'), true, 'public HTTPS target must remain allowed');
assert.equal(RBNet.safeHttp('http://127.0.0.1/live'), false, 'IPv4 loopback must be rejected');
assert.equal(RBNet.safeHttp('http://10.0.0.4/live'), false, 'private IPv4 must be rejected');
assert.equal(RBNet.safeHttp('http://169.254.169.254/latest/meta-data/'), false, 'link-local metadata target must be rejected');
assert.equal(RBNet.safeHttp('https://user:pass@example.com/live'), false, 'credentialed URL must be rejected');
assert.equal(RBNet.safeHttp('http://[::]/live'), false, 'IPv6 unspecified target must be rejected');
assert.equal(RBNet.safeHttp('http://[::1]/live'), false, 'IPv6 loopback must be rejected');
assert.equal(RBNet.safeHttp('http://[::ffff:127.0.0.1]/live'), false, 'IPv4-mapped IPv6 loopback must be rejected');
assert.equal(RBNet.safeHttp('http://[::ffff:10.0.0.4]/live'), false, 'IPv4-mapped IPv6 private target must be rejected');
assert.equal(RBNet.safeHttp('http://localhost./live'), false, 'trailing-dot localhost must be rejected');
assert.equal(RBNet.safeHttp('https://metadata.google.internal./computeMetadata/v1/'), false, 'trailing-dot metadata host must be rejected');
assert.equal(RBNet.safeHttp('http://radio.local./live'), false, 'trailing-dot local-domain target must be rejected');

assert.equal(RBNet.safeRadioBrowserBase('de1.api.radio-browser.info'), 'https://de1.api.radio-browser.info', 'trusted Radio Browser host must be accepted');
assert.equal(RBNet.safeRadioBrowserBase('DE2.API.RADIO-BROWSER.INFO.'), 'https://de2.api.radio-browser.info', 'trusted host normalization must be deterministic');
assert.equal(RBNet.safeRadioBrowserBase('de2.api.radio-browser.info..'), 'https://de2.api.radio-browser.info', 'multiple trailing dots must canonicalize deterministically');
assert.equal(RBNet.safeRadioBrowserBase('localhost'), '', 'catalog discovery must reject localhost');
assert.equal(RBNet.safeRadioBrowserBase('127.0.0.1'), '', 'catalog discovery must reject loopback literals');
assert.equal(RBNet.safeRadioBrowserBase('example.com'), '', 'catalog discovery must reject unrelated public origins');
assert.equal(RBNet.safeRadioBrowserBase('de1.api.radio-browser.info.evil.example'), '', 'catalog discovery must reject suffix-confusion hosts');
assert.equal(RBNet.safeRadioBrowserBase('user@de1.api.radio-browser.info'), '', 'catalog discovery must reject credential-like input');

const catalogPath = path.join(__dirname, '..', 'extensions', 'shared', 'catalog.js');
const catalogSource = fs.readFileSync(catalogPath, 'utf8');
assert.match(catalogSource, /RBNet\.safeRadioBrowserBase/, 'catalog discovery must use the trusted Radio Browser origin validator');
assert.match(catalogSource, /redirect:\s*'error'/, 'catalog API fetches must reject redirects instead of following a server-controlled hop');

const popupPath = path.join(__dirname, '..', 'extensions', 'shared', 'popup.html');
const popupSource = fs.readFileSync(popupPath, 'utf8');
assert.match(popupSource, /id="playerLogo"[^>]*referrerpolicy="no-referrer"/, 'remote player artwork must not send a referrer');

console.log('Browser network safety regression tests OK');
