const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");
const source = fs.readFileSync(path.join(__dirname, "page_bridge_probe.js"), "utf8");
const marker = {ok: true, bridge: "res-downloader", apiVersion: 1};

function page(fetch) {
  const calls = [], timers = new Map();
  let next = 0;
  const context = {Promise, AbortController, bridgeBase: "/.well-known/res-downloader/page-bridge/sample/token/",
    nativeFetch(url, options) { calls.push({url, options}); return fetch(url, options); },
    setTimeout(fn) { const id = ++next; timers.set(id, fn); return id; }, clearTimeout(id) { timers.delete(id); }};
  vm.runInNewContext(source, context);
  return {context, calls, timers};
}

test("probe uses one bodyless request and caches its result", async () => {
  const p = page(() => Promise.resolve({ok: true, json: () => Promise.resolve(marker)}));
  const first = p.context.probePageBridge();
  assert.equal(p.context.probePageBridge(), first);
  assert.equal(await first, true);
  assert.equal(await p.context.probePageBridge(), true);
  assert.equal(p.calls.length, 1);
  const {url, options} = p.calls[0];
  assert.ok(url.endsWith("/probe"));
  assert.equal(options.method, "GET");
  assert.equal(options.body, undefined);
  assert.equal(options.credentials, "omit");
  assert.equal(options.referrerPolicy, "no-referrer");
  assert.equal(options.redirect, "error");
  assert.equal(p.timers.size, 0);
});

test("wrong markers, errors and unsupported bridges cache false", async () => {
  for (const fetch of [
    () => Promise.resolve({ok: false}),
    () => Promise.resolve({ok: true, json: () => Promise.resolve({...marker, bridge: "other"})}),
    () => Promise.resolve({ok: true, json: () => Promise.resolve({...marker, apiVersion: 2})}),
    () => Promise.resolve({ok: true, json: () => Promise.reject(new Error("not JSON"))}),
    () => Promise.reject(new Error("offline")),
    () => { throw new Error("fetch failed"); }
  ]) {
    const p = page(fetch);
    assert.equal(await p.context.probePageBridge(), false);
    assert.equal(await p.context.probePageBridge(), false);
    assert.equal(p.calls.length, 1);
  }
  const p = page(() => { throw new Error("must not fetch"); });
  p.context.bridgeBase = "";
  assert.equal(await p.context.probePageBridge(), false);
  assert.equal(p.calls.length, 0);
});

test("a timeout remains false even if a response arrives later", async () => {
  let release;
  const p = page(() => new Promise((resolve) => { release = resolve; }));
  const probe = p.context.probePageBridge();
  await Promise.resolve();
  for (const fn of p.timers.values()) fn();
  assert.equal(await probe, false);
  assert.equal(p.calls[0].options.signal.aborted, true);
  release({ok: true, json: () => Promise.resolve(marker)});
  assert.equal(await p.context.probePageBridge(), false);
  assert.equal(p.calls.length, 1);
});
