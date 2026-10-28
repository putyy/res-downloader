// Optional: send/capture/operation calls do not depend on this probe.
var pageBridgeProbePromise;
function probePageBridge() {
  if (pageBridgeProbePromise) return pageBridgeProbePromise;
  pageBridgeProbePromise = new Promise(function (resolve) {
    if (!bridgeBase || !nativeFetch) { resolve(false); return; }
    var controller = typeof AbortController === "function" ? new AbortController() : null;
    var timer = setTimeout(function () {
      resolve(false);
      if (controller) controller.abort();
    }, 1000);
    Promise.resolve().then(function () {
      return nativeFetch(bridgeBase + "probe", {
        method: "GET", credentials: "omit", referrerPolicy: "no-referrer",
        cache: "no-store", redirect: "error", signal: controller ? controller.signal : undefined
      });
    }).then(function (response) {
      return response.ok ? response.json() : null;
    }).then(function (result) {
      clearTimeout(timer);
      resolve(!!result && result.ok === true && result.bridge === "res-downloader" && result.apiVersion === 1);
    }).catch(function () { clearTimeout(timer); resolve(false); });
  });
  return pageBridgeProbePromise;
}
