const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const code = () => fs.readFileSync('static/assets/stats.js', 'utf8');

function browser(options = {}) {
  const calls = [], timers = [], listeners = new Map();
  const storage = options.storage || new Map();
  let random = 0;
  const document = {
    currentScript: {dataset: {pageId: 'post:stable', siteTimezone: 'Asia/Shanghai'}},
    visibilityState: options.hidden ? 'hidden' : 'visible',
    referrer: options.referrer || '',
    addEventListener: (name, fn) => listeners.set(name, fn),
    removeEventListener: name => listeners.delete(name)
  };
  class Clock extends Date { constructor(...args) { super(...(args.length ? args : [options.now || '2026-10-22T16:01:00Z'])); } }
  const context = {
    document, URL, Blob, Uint8Array, Intl, Date: Clock,
    crypto: {getRandomValues: bytes => {bytes.fill(++random); return bytes;}},
    localStorage: {
      getItem: key => {if (options.noStorage) throw Error('denied'); return storage.get(key) || null;},
      setItem: (key, value) => {if (options.noStorage) throw Error('denied'); storage.set(key, value);}
    },
    navigator: {sendBeacon: (url, body) => {calls.push({url, body}); return options.queue !== false;}},
    setTimeout: fn => timers.push(fn)
  };
  vm.runInNewContext(code(), context);
  return {calls, timers, storage, visible() {document.visibilityState = 'visible'; listeners.get('visibilitychange')?.();}};
}
async function event(b, index = 0) {return JSON.parse(await b.calls[index].body.text());}

test('one visible load sends exactly the five fields and only navigation hostname', async () => {
  const b = browser({hidden: true, referrer: 'https://Search.Example:8443/private?q=secret#fragment'});
  assert.equal(b.calls.length, 0);
  b.visible(); b.visible();
  assert.equal(b.calls.length, 1);
  assert.equal(b.calls[0].url, '/api/events');
  assert.equal(b.calls[0].body.type, 'application/json');
  const e = await event(b);
  assert.deepEqual(Object.keys(e).sort(), ['event_id','page_id','referrer_host','v','visitor_id']);
  assert.equal(e.referrer_host, 'search.example');
  assert.equal(e.page_id, 'post:stable');
  assert.equal(e.v, 1);
  assert.match(e.event_id, /^[0-9a-f]{32}$/);
  assert.match(e.visitor_id, /^[0-9a-f]{32}$/);
  assert.notEqual(e.event_id, e.visitor_id);
});
test('queue failure retries at most once with identical event and visitor', async () => {
  const b = browser({queue: false, noStorage: true});
  assert.equal(b.calls.length, 1);
  assert.equal(b.timers.length, 1);
  b.timers.shift()();
  assert.equal(b.calls.length, 2);
  assert.equal(b.timers.length, 0);
  assert.deepEqual(await event(b), await event(b, 1));
});
test('visitor rotates on site midnight and denied storage still works', async () => {
  const a = browser();
  const saved = [...a.storage.values()][0];
  assert.equal(JSON.parse(saved).day, '2026-10-23');
  const b = browser({storage: a.storage});
  assert.equal((await event(a)).visitor_id, (await event(b)).visitor_id);
  const c = browser({storage: new Map([['blogo:visitor:v1', JSON.stringify({day:'2026-10-22', id:'a'.repeat(32)})]])});
  assert.notEqual((await event(c)).visitor_id, 'a'.repeat(32));
  assert.equal((await event(browser({noStorage:true, referrer:'invalid'}))).referrer_host, '');
});
