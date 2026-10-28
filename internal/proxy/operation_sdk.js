var operationHandlers = new Map(), operationControllers = new Map(), operationRevision = '', operationPageURL = '';
var operationReadyTimeout = 5000, operationReadyAttempts = 3, operationReadyRetryDelay = 200;
var operationStateUpdate = Promise.resolve(), operationStateGeneration = 0, operationStateController = null, operationStateSnapshot = '';
var operationPageReady = false, operationEventsConnected = false, operationReloadResuming = '', operationReloadResumeTimer;
var operationReloadSignal = {}, operationReloadStorageKey = typeof pluginId === 'string' && typeof scriptId === 'string' ? '__resDownloaderOperationReload:' + pluginId + ':' + scriptId : '';
var operationPendingReload = consumeOperationReload();
function removeOperationReload() {
  try { if (operationReloadStorageKey) sessionStorage.removeItem(operationReloadStorageKey); } catch (_) {}
}
function consumeOperationReload() {
  if (!operationReloadStorageKey) return null;
  try {
    var raw = sessionStorage.getItem(operationReloadStorageKey);
    if (!raw) return null;
    sessionStorage.removeItem(operationReloadStorageKey);
    if (raw.length > 1024) return null;
    var ticket = JSON.parse(raw);
    if (!ticket || !/^[a-f0-9]{32}$/.test(ticket.executionId) || !/^[a-f0-9]{32,128}$/.test(ticket.token) ||
        typeof ticket.operationId !== 'string' || !ticket.operationId || ticket.operationId.length > 64 ||
        ticket.pluginVersion !== pluginVersion || typeof ticket.expiresAt !== 'number' ||
        ticket.expiresAt <= Date.now() || ticket.expiresAt > Date.now() + 65000) return null;
    return ticket;
  } catch (_) { return null; }
}
function finishOperationReloadResume() {
  operationReloadResuming = '';
  clearTimeout(operationReloadResumeTimer);
}
function tryResumeOperationReload() {
  var ticket = operationPendingReload;
  if (!ticket) return;
  if (ticket.expiresAt <= Date.now()) { operationPendingReload = null; return; }
  if (!operationPageReady || !operationEventsConnected || !operationRevision ||
      operationPageURL !== location.href || !operationHandlers.has(ticket.operationId)) return;
  // Consume once, before the asynchronous ownership transfer. A failed or lost
  // response never creates a new execution or triggers another resume request.
  operationPendingReload = null;
  operationReloadResuming = ticket.executionId;
  operationReloadResumeTimer = setTimeout(finishOperationReloadResume, Math.max(0, ticket.expiresAt - Date.now()));
  operationRequest('resume', {executionId:ticket.executionId,token:ticket.token,revision:operationRevision}).catch(function() {
    finishOperationReloadResume();
    try { console.warn('[res-downloader] 页面刷新续接失败，请从软件重新执行'); } catch (_) {}
  });
}
function setOperationEventsConnected(connected) {
  operationEventsConnected = connected;
  if (connected) tryResumeOperationReload();
}
function operationAbortedError() { return new Error('page state changed or operation cancelled'); }
function operationRequest(action, data, stateSignal) {
  if (!bridgeBase || !nativeFetch) return Promise.reject(new Error('page bridge is unavailable'));
  if (stateSignal && stateSignal.aborted) return Promise.reject(operationAbortedError());
  var body = JSON.stringify(data);
  if (new TextEncoder().encode(body).length > 65536) return Promise.reject(new Error('operation message exceeds 64 KiB'));
  var options = {method:'POST', headers:{'Content-Type':'application/json'}, body:body, credentials:'same-origin', cache:'no-store'};
  var timer, abortState, controller, deadline;
  if (action === 'ready' || action === 'reload' || action === 'resume' || action === 'reload-abort') {
    controller = new AbortController(); options.signal = controller.signal;
    deadline = new Promise(function(_, reject) {
      abortState = function() { controller.abort(); reject(operationAbortedError()); };
      if (stateSignal) stateSignal.addEventListener('abort', abortState, {once:true});
      timer = setTimeout(function() { controller.abort(); reject(new Error('page readiness request timed out')); }, operationReadyTimeout);
    });
  }
  // The readiness deadline includes response.json(), not just response headers.
  // Racing also releases the state queue if a transport fails to settle on abort.
  var request = Promise.resolve().then(function() {
    if (controller && controller.signal.aborted) throw operationAbortedError();
    return nativeFetch(bridgeBase + 'operation-' + action, options);
  }).then(function(response) {
    return response.json().then(function(result) {
      if (!response.ok || !result.ok) {
        var error = new Error(result.error || 'operation request failed');
        error.retryable = response.status === 408 || response.status === 429 || response.status >= 500;
        throw error;
      }
      return result.data;
    });
  });
  if (!deadline) return request;
  return Promise.race([request, deadline]).finally(function() {
    clearTimeout(timer);
    if (stateSignal) stateSignal.removeEventListener('abort', abortState);
  });
}
function operationRetryDelay(signal) {
  return new Promise(function(resolve, reject) {
    if (signal.aborted) { reject(operationAbortedError()); return; }
    var timer = setTimeout(function() { signal.removeEventListener('abort', abort); resolve(); }, operationReadyRetryDelay);
    function abort() { clearTimeout(timer); signal.removeEventListener('abort', abort); reject(operationAbortedError()); }
    signal.addEventListener('abort', abort, {once:true});
  });
}
function cancelOperationStateUpdates() {
  operationPageReady = false;
  operationStateGeneration++;
  if (operationStateController) operationStateController.abort();
}
function waitForOperationReadiness(pending, signal) {
  return new Promise(function(resolve, reject) {
    if (signal.aborted) { reject(operationAbortedError()); return; }
    var timer = setTimeout(function() { finish(new Error('page readiness did not settle')); },
      operationReadyAttempts * operationReadyTimeout + (operationReadyAttempts - 1) * operationReadyRetryDelay + 1000);
    function abort() { finish(operationAbortedError()); }
    function finish(error) {
      clearTimeout(timer); signal.removeEventListener('abort', abort);
      if (error) reject(error); else resolve();
    }
    signal.addEventListener('abort', abort, {once:true});
    pending.then(function() { finish(); }, finish);
  });
}
var operationsAPI = Object.freeze({
  hasPendingReload: function() { return !!operationPendingReload || !!operationReloadResuming; },
  setState: function(state) {
    var snapshot = {title:document.title.slice(0,128),pageUrl:location.href,ready:state.ready===true,login:state.login||'unknown',context:state.context||''};
    var encoded = JSON.stringify(snapshot);
    // Repeated identical announcements must not abort a slow request forever.
    if (operationStateController && !operationStateController.signal.aborted && operationStateSnapshot === encoded) return operationStateUpdate;
    cancelOperationStateUpdates();
    var generation = operationStateGeneration, controller = new AbortController();
    operationStateController = controller; operationStateSnapshot = encoded;
    function requireCurrent() {
      if (controller.signal.aborted || generation !== operationStateGeneration || snapshot.pageUrl !== location.href) throw operationAbortedError();
    }
    operationStateUpdate = operationStateUpdate.catch(function() {}).then(async function() {
      for (var attempt = 0; attempt < operationReadyAttempts; attempt++) {
        requireCurrent();
        try {
          var result = await operationRequest('ready', snapshot, controller.signal);
          requireCurrent();
          if (operationRevision && operationRevision !== result.revision) operationControllers.forEach(function(active) { if (active.operationRevision !== result.revision) active.abort(); });
          operationRevision = result.revision; operationPageURL = snapshot.pageUrl; operationPageReady = snapshot.ready;
          tryResumeOperationReload();
          return result;
        } catch (error) {
          requireCurrent();
          if (error.retryable === false || attempt + 1 === operationReadyAttempts) throw error;
          await operationRetryDelay(controller.signal);
        }
      }
    }).finally(function() { if (operationStateController === controller) operationStateController = null; });
    return operationStateUpdate;
  },
  handle: function(id, handler) {
    if (typeof id !== 'string' || typeof handler !== 'function' || operationHandlers.has(id)) throw new Error('invalid or duplicate operation handler');
    operationHandlers.set(id, handler);
    tryResumeOperationReload();
    return function() { operationHandlers.delete(id); };
  }
});
listeners.push(function(message) {
  if (!message || message.protocol !== 1 || typeof message.executionId !== 'string') return;
  if (message.type === 'operation-cancel') { var active = operationControllers.get(message.executionId); if (active) active.abort(); return; }
  if (message.type !== 'operation-invoke' || operationControllers.has(message.executionId)) return;
  var handler = operationHandlers.get(message.operationId);
  if (!handler) return;
  // Register synchronously so duplicates and cancellation are handled while the
  // ready HTTP response is still in flight. Wait only for this captured promise.
  var controller = new AbortController(), pendingState = operationStateUpdate;
  controller.operationRevision = message.revision;
  operationControllers.set(message.executionId, controller);
  if (operationReloadResuming === message.executionId) finishOperationReloadResume();
  (async function() {
    var timer;
    try {
      await waitForOperationReadiness(pendingState, controller.signal);
      if (controller.signal.aborted || message.revision !== operationRevision || operationPageURL !== location.href) throw operationAbortedError();
      await operationRequest('claim', {executionId:message.executionId, revision:message.revision});
      if (controller.signal.aborted || message.revision !== operationRevision || operationPageURL !== location.href) throw operationAbortedError();
      var progress;
      var report = function(value) {
        if (controller.reloading) return Promise.reject(new Error('execution is waiting for page reload'));
        progress = value; return operationRequest('report', {executionId:message.executionId,revision:message.revision,state:'running',progress:progress});
      };
      var reload = async function() {
        if (controller.reloading || controller.signal.aborted || message.reloadCount > 0 ||
            message.revision !== operationRevision || operationPageURL !== location.href) throw operationAbortedError();
        // Verify storage before suspending ownership. Never save input, URLs or
        // a website response; only the short-lived host ticket crosses reload.
        try {
          if (!operationReloadStorageKey) throw new Error('unavailable');
          sessionStorage.setItem(operationReloadStorageKey, '{}');
          sessionStorage.removeItem(operationReloadStorageKey);
        } catch (_) { throw new Error('page reload storage unavailable'); }
        controller.reloading = true;
        clearInterval(timer);
        var ticket;
        try {
          ticket = await operationRequest('reload', {executionId:message.executionId,revision:message.revision});
          if (controller.signal.aborted || message.revision !== operationRevision || operationPageURL !== location.href) throw operationAbortedError();
          if (!ticket || typeof ticket.token !== 'string' || !/^[a-f0-9]{32,128}$/.test(ticket.token) ||
              typeof ticket.expiresAt !== 'number' || ticket.expiresAt <= Date.now()) throw new Error('invalid reload ticket');
          sessionStorage.setItem(operationReloadStorageKey, JSON.stringify({executionId:message.executionId,
            token:ticket.token,expiresAt:ticket.expiresAt,operationId:message.operationId,pluginVersion:pluginVersion}));
          location.reload();
        } catch (error) {
          removeOperationReload();
          // Preparation may have succeeded even if its response was lost. The
          // authenticated old owner can acknowledge that its handler stopped
          // without knowing the ticket; this can never resume an execution.
          await operationRequest('reload-abort', {executionId:message.executionId,revision:message.revision,
            token:ticket && typeof ticket.token === 'string' ? ticket.token : ''}).catch(function() {});
          controller.reloading = false;
          throw error;
        }
        throw operationReloadSignal;
      };
      timer = setInterval(function() { report(progress).catch(function() { if (!controller.reloading) controller.abort(); }); }, 15000);
      var result = await handler({executionId:message.executionId,input:message.input,cursor:message.cursor,limit:message.limit,resource:message.resource,signal:controller.signal,report:report,
        reloadCount:message.reloadCount === 1 ? 1 : 0,reload:reload});
      if (controller.reloading) return;
      await operationRequest('report', {executionId:message.executionId,revision:message.revision,state:controller.signal.aborted?'cancelled':'succeeded',data:result.data,count:result.count||0,hasMore:result.hasMore===true,nextCursor:result.nextCursor||'',truncated:result.truncated===true});
    } catch (error) {
      if (error !== operationReloadSignal && !controller.reloading) {
        await operationRequest('report', {executionId:message.executionId,revision:message.revision,state:controller.signal.aborted?'cancelled':'failed'}).catch(function() {});
      }
    } finally { clearInterval(timer); operationControllers.delete(message.executionId); }
  })();
});
