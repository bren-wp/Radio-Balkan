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

console.log('Browser network safety regression tests OK');
