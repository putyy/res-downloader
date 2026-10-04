'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const source = fs.readFileSync(path.join(__dirname, '..', 'operation_sdk.js'), 'utf8');

function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return {promise, resolve, reject};
}
function response(revision, status = 200) {
  return {ok: status === 200, status, json: () => Promise.resolve({ok: status === 200, data: {revision}, error: 'temporary failure'})};
}
function outcome(promise) {
  return promise.then(value => ({value}), error => ({error}));
}
async function pump() { for (let i = 0; i < 40; i++) await Promise.resolve(); }
function memoryStorage(initial = {}) {
  const values = new Map(Object.entries(initial)), writes = [], removals = [];
  return {
    values, writes, removals,
    getItem: key => values.has(key) ? values.get(key) : null,
    setItem(key, value) { writes.push({key, value: String(value)}); values.set(key, String(value)); },
    removeItem(key) { removals.push(key); values.delete(key); },
  };
}
function harness(fetch, options = {}) {
  let now = options.now ?? 0, nextTimer = 0;
  const timers = new Map(), calls = [], warnings = [];
  const storage = options.storage || memoryStorage();
  const reloads = [];
  function timer(fn, ms, interval) {
    const id = ++nextTimer;
    timers.set(id, {fn, at: now + ms, interval});
    return id;
  }
  const context = vm.createContext({
    Map, Promise, AbortController, TextEncoder,
    Date: class extends Date { static now() { return now; } },
    sessionStorage: storage,
    pluginId: options.pluginId || 'test.plugin', pluginVersion: options.pluginVersion || '1', scriptId: options.scriptId || 'page',
    bridgeBase: '/bridge/', document: {title: 'Example'},
    location: {href: options.href || 'https://example.test/watch?v=one', reload() { reloads.push(now); return options.reload?.(); }},
    listeners: [], console: {warn: (...args) => warnings.push(args)},
    setTimeout: (fn, ms) => timer(fn, ms, 0), clearTimeout: id => timers.delete(id),
    setInterval: (fn, ms) => timer(fn, ms, ms), clearInterval: id => timers.delete(id),
    nativeFetch: (url, options) => {
      const call = {action: url.replace('/bridge/operation-', ''), body: JSON.parse(options.body), options};
      calls.push(call);
      return Promise.resolve(fetch(call, calls));
    },
  });
  vm.runInContext(source, context, {filename: 'operation_sdk.js'});
  return {
    context, calls, storage, reloads, warnings,
    send: message => context.listeners[0](message),
    async advance(ms) {
      await pump();
      const target = now + ms;
      while (true) {
        const due = [...timers.entries()].filter(([, t]) => t.at <= target).sort((a, b) => a[1].at - b[1].at)[0];
        if (!due) break;
        const [id, entry] = due;
        now = entry.at;
        if (entry.interval) entry.at += entry.interval; else timers.delete(id);
        entry.fn();
        await pump();
      }
      now = target;
      await pump();
    },
  };
}
function invoke(revision, executionId = 'execution') {
  return {protocol: 1, type: 'operation-invoke', executionId, operationId: 'read', revision, input: {}, limit: 1};
}

test('an interrupted handler reports its original revision after setState advances', async () => {
  const h = harness(call => response(call.action === 'ready' ? call.body.context : 'ok'));
  await h.context.operationsAPI.setState({ready: true, context: 'old'});
  h.context.operationsAPI.handle('read', ctx => new Promise((resolve, reject) => {
    ctx.signal.addEventListener('abort', () => reject(new Error('stopped')), {once: true});
  }));
  h.send(invoke('old'));
  await pump();
  await h.context.operationsAPI.setState({ready: true, context: 'new'});
  await pump();
  const reports = h.calls.filter(call => call.action === 'report');
  assert.equal(reports.length, 1);
  assert.equal(reports[0].body.state, 'cancelled');
  assert.equal(reports[0].body.revision, 'old');
  assert.equal(h.context.operationRevision, 'new');
  assert.equal(h.context.operationControllers.size, 0);
});

test('ready deadline aborts an unfinished response body and frees the state chain', async () => {
  let attempts = 0, aborted = false;
  const h = harness(call => {
    assert.equal(call.action, 'ready');
    if (++attempts === 1) {
      call.options.signal.addEventListener('abort', () => { aborted = true; });
      return {ok: true, status: 200, json: () => new Promise(() => {})};
    }
    return response('revision');
  });
  const first = h.context.operationsAPI.setState({ready: true});
  assert.equal(h.context.operationsAPI.setState({ready: true}), first, 'identical announcements should share their in-flight update');
  const settled = outcome(first);
  await h.advance(5000);
  assert.equal(aborted, true, 'timeout must abort transport/body consumption');
  await h.advance(200);
  assert.equal((await settled).value.revision, 'revision');
  assert.equal(attempts, 2);
  assert.equal((await h.context.operationsAPI.setState({ready: true, context: 'next'})).revision, 'revision');
});

test('readiness retries stop after three attempts and a later update can recover', async () => {
  let healthy = false;
  const h = harness(() => response('recovered', healthy ? 200 : 503));
  const failed = outcome(h.context.operationsAPI.setState({ready: true}));
  await h.advance(400);
  assert.ok((await failed).error);
  assert.equal(h.calls.length, 3);
  await h.advance(10000);
  assert.equal(h.calls.length, 3, 'no background retry beyond the bounded attempt count');
  healthy = true;
  assert.equal((await h.context.operationsAPI.setState({ready: true})).revision, 'recovered');
});

test('superseded page readiness is aborted rather than replayed after navigation', async () => {
  const h = harness(call => call.body.context === 'old' ? new Promise(() => {}) : response('new'));
  const stale = outcome(h.context.operationsAPI.setState({ready: true, context: 'old'}));
  await pump();
  h.context.location.href = 'https://example.test/watch?v=two';
  const fresh = h.context.operationsAPI.setState({ready: true, context: 'new'});
  assert.equal(h.calls[0].options.signal.aborted, true);
  assert.equal((await fresh).revision, 'new');
  assert.ok((await stale).error);
  await h.advance(10000);
  assert.equal(h.calls.length, 2);
  assert.equal(h.context.operationPageURL, h.context.location.href);
});

test('SSE invocation waits for its pending ready response and only claims once', async () => {
  const pending = deferred();
  const h = harness(call => call.action === 'ready' && call.body.context === 'next' ? pending.promise : response('old'));
  await h.context.operationsAPI.setState({ready: true});
  let runs = 0;
  h.context.operationsAPI.handle('read', async () => { runs++; return {data: {}, count: 1}; });
  const ready = h.context.operationsAPI.setState({ready: true, context: 'next'});
  await pump();
  h.send(invoke('new'));
  h.send(invoke('new'));
  await pump();
  assert.equal(h.context.operationControllers.size, 1);
  assert.equal(h.calls.filter(c => c.action === 'claim').length, 0);
  assert.equal(runs, 0);
  pending.resolve(response('new'));
  await ready;
  await pump();
  assert.equal(h.calls.filter(c => c.action === 'claim').length, 1);
  assert.equal(runs, 1, 'the new revision must not abort its own waiting invocation');
  assert.equal(h.calls.filter(c => c.action === 'report' && c.body.state === 'succeeded').length, 1);
});

test('cancellation while awaiting readiness prevents claim and handler execution', async () => {
  const pending = deferred();
  const h = harness(call => call.action === 'ready' ? pending.promise : response('ready'));
  let runs = 0;
  h.context.operationsAPI.handle('read', async () => { runs++; return {data: {}}; });
  const ready = h.context.operationsAPI.setState({ready: true});
  await pump();
  h.send(invoke('ready'));
  h.send({protocol: 1, type: 'operation-cancel', executionId: 'execution'});
  await pump();
  assert.equal(h.calls.filter(c => c.action === 'claim').length, 0);
  pending.resolve(response('ready'));
  await ready;
  await pump();
  assert.equal(runs, 0);
  assert.equal(h.context.operationControllers.size, 0);
});

test('pagehide invalidates queued readiness and aborts its in-flight request', async () => {
  const h = harness(() => new Promise(() => {}));
  const ready = outcome(h.context.operationsAPI.setState({ready: true}));
  await pump();
  h.context.cancelOperationStateUpdates();
  assert.ok((await ready).error);
  assert.equal(h.calls[0].options.signal.aborted, true);
  await h.advance(20000);
  assert.equal(h.calls.length, 1);
});

test('permanent ready rejection is not retried and reports have no readiness timeout', async () => {
  const pending = deferred();
  const h = harness(call => call.action === 'ready' ? response('', 403) : pending.promise);
  const ready = await outcome(h.context.operationsAPI.setState({ready: true}));
  assert.ok(ready.error);
  await h.advance(1000);
  assert.equal(h.calls.length, 1);
  let completed = false;
  const report = h.context.operationRequest('report', {executionId: 'execution', state: 'succeeded'}).then(() => { completed = true; });
  await h.advance(20000);
  assert.equal(completed, false, 'ready timeout must not abort finalization reports');
  assert.equal(h.calls[1].options.signal, undefined);
  pending.resolve(response(''));
  await report;
});

const reloadExecution = 'e'.repeat(32);
const reloadToken = 'a'.repeat(64);
const reloadKey = '__resDownloaderOperationReload:test.plugin:page';
function reloadTicket(overrides = {}) {
  return {executionId: reloadExecution, token: reloadToken, expiresAt: 60000, operationId: 'read', pluginVersion: '1', ...overrides};
}
function dataResponse(data, status = 200) {
  return {ok: status === 200, status, json: () => Promise.resolve({ok: status === 200, data, error: 'request rejected'})};
}
function terminalReports(h) {
  return h.calls.filter(call => call.action === 'report' && call.body.state !== 'running');
}
function reloadFetch(call) {
  if (call.action === 'reload') return dataResponse({token: reloadToken, expiresAt: 60000});
  return response('revision');
}

test('successful reload stores only its ticket and does not finish the old handler', async () => {
  const h = harness(reloadFetch);
  await h.context.operationsAPI.setState({ready: true});
  let entered = 0, returned = false;
  h.context.operationsAPI.handle('read', async context => {
    entered++;
    await context.reload();
    returned = true;
    return {data: {}};
  });
  h.send({...invoke('revision', reloadExecution), input: {privateValue: 'must-not-survive-reload'}, resource: {id: 'private-resource'}});
  await pump();
  assert.equal(entered, 1);
  assert.equal(returned, false, 'reload must unwind via the SDK sentinel');
  assert.equal(h.reloads.length, 1);
  assert.equal(h.calls.filter(call => call.action === 'reload').length, 1);
  assert.equal(terminalReports(h).length, 0);
  assert.equal(h.context.operationControllers.size, 0);
  const stored = JSON.parse(h.storage.getItem(reloadKey));
  assert.deepEqual(stored, reloadTicket());
  assert.deepEqual(Object.keys(stored).sort(), ['executionId', 'expiresAt', 'operationId', 'pluginVersion', 'token']);
  assert.equal(h.storage.getItem(reloadKey).includes('private'), false);
  await h.advance(30000);
  assert.equal(h.calls.filter(call => call.action === 'report').length, 0, 'old heartbeat must stop');
});

for (const lastGate of ['handler', 'ready', 'sse']) {
  test('reload is consumed once and waits for ' + lastGate + ' before resuming', async () => {
    const storage = memoryStorage({[reloadKey]: JSON.stringify(reloadTicket())});
    const h = harness(reloadFetch, {storage});
    const finish = deferred();
    let runs = 0, secondReloadRejected = false;
    const gates = {
      handler: async () => h.context.operationsAPI.handle('read', async context => {
        runs++;
        assert.equal(context.executionId, reloadExecution);
        assert.equal(context.reloadCount, 1);
        secondReloadRejected = !!(await outcome(context.reload())).error;
        await finish.promise;
        return {data: {done: true}};
      }),
      ready: async () => h.context.operationsAPI.setState({ready: true}),
      sse: async () => h.context.setOperationEventsConnected(true),
    };
    assert.equal(storage.getItem(reloadKey), null, 'consume storage before asynchronous work');
    assert.equal(h.context.operationsAPI.hasPendingReload(), true);
    for (const gate of Object.keys(gates).filter(gate => gate !== lastGate)) await gates[gate]();
    await pump();
    assert.equal(h.calls.filter(call => call.action === 'resume').length, 0);
    await gates[lastGate]();
    await pump();
    const resumes = h.calls.filter(call => call.action === 'resume');
    assert.equal(resumes.length, 1);
    assert.deepEqual(resumes[0].body, {executionId: reloadExecution, token: reloadToken, revision: 'revision'});
    assert.equal(runs, 0, 'resume response alone must not run the handler');
    const message = {...invoke('revision', reloadExecution), reloadCount: 1};
    h.send(message);
    h.send(message);
    await pump();
    assert.equal(runs, 1);
    assert.equal(h.calls.filter(call => call.action === 'claim').length, 1);
    assert.equal(secondReloadRejected, true);
    assert.equal(h.calls.filter(call => call.action === 'reload').length, 0, 'resumed invocation has no second reload allowance');
    assert.equal(h.context.operationsAPI.hasPendingReload(), false);
    finish.resolve();
    await pump();
    assert.equal(terminalReports(h).filter(call => call.body.state === 'succeeded').length, 1);
    h.context.setOperationEventsConnected(false);
    h.context.setOperationEventsConnected(true);
    await h.context.operationsAPI.setState({ready: true});
    await pump();
    assert.equal(h.calls.filter(call => call.action === 'resume').length, 1);
    const laterDocument = harness(reloadFetch, {storage});
    assert.equal(laterDocument.context.operationsAPI.hasPendingReload(), false, 'ticket cannot survive into another document');
  });
}

for (const [name, raw] of [
  ['expired', JSON.stringify(reloadTicket({expiresAt: 0}))],
  ['version mismatch', JSON.stringify(reloadTicket({pluginVersion: 'old'}))],
  ['too long lived', JSON.stringify(reloadTicket({expiresAt: 65001}))],
  ['malformed JSON', '{'],
  ['invalid execution', JSON.stringify(reloadTicket({executionId: 'bad'}))],
  ['invalid token', JSON.stringify(reloadTicket({token: 'bad'}))],
  ['null', 'null'],
  ['oversized', 'x'.repeat(1025)],
]) {
  test(name + ' reload ticket is removed without a resume', async () => {
    const storage = memoryStorage({[reloadKey]: raw});
    const h = harness(reloadFetch, {storage});
    h.context.operationsAPI.handle('read', async () => assert.fail('invalid ticket invoked a handler'));
    h.context.setOperationEventsConnected(true);
    await h.context.operationsAPI.setState({ready: true});
    await pump();
    assert.equal(storage.getItem(reloadKey), null);
    assert.equal(h.context.operationsAPI.hasPendingReload(), false);
    assert.equal(h.calls.filter(call => call.action === 'resume').length, 0);
  });
}

test('a consumed ticket expires while waiting for prerequisites', async () => {
  const h = harness(reloadFetch, {now: 1000, storage: memoryStorage({[reloadKey]: JSON.stringify(reloadTicket({expiresAt: 2000}))})});
  h.context.operationsAPI.handle('read', async () => assert.fail('expired ticket invoked a handler'));
  await h.advance(1001);
  h.context.setOperationEventsConnected(true);
  await h.context.operationsAPI.setState({ready: true});
  assert.equal(h.calls.filter(call => call.action === 'resume').length, 0);
  assert.equal(h.context.operationsAPI.hasPendingReload(), false);
});

for (const failure of ['rejected', 'timeout']) {
  test('resume ' + failure + ' never retries or starts a new execution', async () => {
    const h = harness(call => call.action === 'resume'
      ? failure === 'timeout' ? new Promise(() => {}) : dataResponse(null, 503)
      : response('revision'), {storage: memoryStorage({[reloadKey]: JSON.stringify(reloadTicket())})});
    h.context.operationsAPI.handle('read', async () => assert.fail('failed resume ran a handler'));
    h.context.setOperationEventsConnected(true);
    await h.context.operationsAPI.setState({ready: true});
    await h.advance(5000);
    assert.equal(h.context.operationsAPI.hasPendingReload(), false);
    h.context.setOperationEventsConnected(true);
    await h.context.operationsAPI.setState({ready: true});
    await h.advance(60000);
    const calls = h.calls.filter(call => call.action !== 'ready');
    assert.equal(calls.length, 1);
    assert.equal(calls[0].action, 'resume');
    if (failure === 'timeout') assert.equal(calls[0].options.signal.aborted, true);
    assert.equal(h.reloads.length, 0);
  });
}

test('unavailable storage fails before requesting reload ownership', async () => {
  const storage = memoryStorage();
  storage.setItem = () => { throw new Error('storage disabled'); };
  const h = harness(reloadFetch, {storage});
  h.context.operationsAPI.handle('read', async context => context.reload());
  await h.context.operationsAPI.setState({ready: true});
  h.send(invoke('revision', reloadExecution));
  await pump();
  assert.equal(h.calls.filter(call => call.action === 'reload' || call.action === 'reload-abort').length, 0);
  assert.deepEqual(terminalReports(h).map(call => call.body.state), ['failed']);
  assert.equal(h.reloads.length, 0);
});

for (const failure of ['cancelled', 'storage-write', 'reload-call']) {
  test('prepared reload ' + failure + ' abandons its ticket and acknowledges termination', async () => {
    const prepared = deferred(), storage = memoryStorage();
    const save = storage.setItem.bind(storage);
    if (failure === 'storage-write') storage.setItem = (key, value) => {
      if (value !== '{}') throw new Error('quota exhausted');
      save(key, value);
    };
    const h = harness(call => call.action === 'reload' ? prepared.promise : response('revision'), {
      storage,
      reload: () => { if (failure === 'reload-call') throw new Error('navigation refused'); },
    });
    h.context.operationsAPI.handle('read', async context => context.reload());
    await h.context.operationsAPI.setState({ready: true});
    h.send(invoke('revision', reloadExecution));
    await pump();
    assert.equal(h.calls.filter(call => call.action === 'reload').length, 1);
    if (failure === 'cancelled') h.send({protocol: 1, type: 'operation-cancel', executionId: reloadExecution});
    prepared.resolve(dataResponse({token: reloadToken, expiresAt: 60000}));
    await pump();
    const abandon = h.calls.filter(call => call.action === 'reload-abort');
    assert.equal(abandon.length, 1);
    assert.deepEqual(abandon[0].body, {executionId: reloadExecution, revision: 'revision', token: reloadToken});
    assert.deepEqual(terminalReports(h).map(call => call.body.state), [failure === 'cancelled' ? 'cancelled' : 'failed']);
    assert.ok(h.calls.indexOf(abandon[0]) < h.calls.indexOf(terminalReports(h)[0]), 'abandon must precede terminal acknowledgement');
    assert.equal(storage.getItem(reloadKey), null);
    assert.equal(h.reloads.length, failure === 'reload-call' ? 1 : 0, 'only the throwing navigation attempt may be called');
    assert.equal(h.context.operationControllers.size, 0);
    await h.advance(30000);
    assert.equal(h.calls.filter(call => call.action === 'reload').length, 1);
  });
}

test('an in-flight heartbeat rejection during preparation does not abort reload', async () => {
  const heartbeat = deferred(), beginReload = deferred(), prepared = deferred();
  const h = harness(call => {
    if (call.action === 'report' && call.body.state === 'running') return heartbeat.promise;
    if (call.action === 'reload') return prepared.promise;
    return response('revision');
  });
  let operation;
  h.context.operationsAPI.handle('read', async context => {
    operation = context;
    await beginReload.promise;
    await context.reload();
  });
  await h.context.operationsAPI.setState({ready: true});
  h.send(invoke('revision', reloadExecution));
  await pump();
  await h.advance(15000);
  assert.equal(h.calls.filter(call => call.action === 'report').length, 1);
  beginReload.resolve();
  await pump();
  assert.equal(h.calls.filter(call => call.action === 'reload').length, 1);
  heartbeat.reject(new Error('old ownership suspended'));
  await pump();
  assert.equal(operation.signal.aborted, false);
  prepared.resolve(dataResponse({token: reloadToken, expiresAt: 60000}));
  await pump();
  assert.equal(h.reloads.length, 1);
  assert.equal(terminalReports(h).length, 0);
  await h.advance(30000);
  assert.equal(h.calls.filter(call => call.action === 'report').length, 1, 'no new heartbeat after preparing reload');
});

for (const failure of ['timeout', 'rejected']) {
  test('prepare ' + failure + ' acknowledges a stopped owner even without its ticket', async () => {
    const h = harness(call => call.action === 'reload'
      ? failure === 'timeout' ? new Promise(() => {}) : dataResponse(null, 409)
      : response('revision'));
    h.context.operationsAPI.handle('read', async context => context.reload());
    await h.context.operationsAPI.setState({ready: true});
    h.send(invoke('revision', reloadExecution));
    await h.advance(5000);
    const preparations = h.calls.filter(call => call.action === 'reload');
    const abandon = h.calls.filter(call => call.action === 'reload-abort');
    assert.equal(preparations.length, 1);
    assert.equal(abandon.length, 1);
    assert.deepEqual(abandon[0].body, {executionId: reloadExecution, revision: 'revision', token: ''});
    assert.deepEqual(terminalReports(h).map(call => call.body.state), ['failed']);
    assert.ok(h.calls.indexOf(abandon[0]) < h.calls.indexOf(terminalReports(h)[0]));
    assert.equal(h.reloads.length, 0);
    assert.equal(h.storage.getItem(reloadKey), null);
    assert.equal(h.context.operationControllers.size, 0);
    if (failure === 'timeout') assert.equal(preparations[0].options.signal.aborted, true);
    await h.advance(30000);
    assert.equal(h.calls.filter(call => call.action === 'reload').length, 1);
  });
}
