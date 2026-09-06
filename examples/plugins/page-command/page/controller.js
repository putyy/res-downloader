pageApi.onMessage(function (message) {
  if (!message || message.type !== "resource-action" || message.actionId !== "inspect-page-resource") return;

  var targetAssetId = message.data && message.data.assetId;
  var currentAssetId = document.documentElement.getAttribute("data-example-asset-id");
  var status = targetAssetId && targetAssetId === currentAssetId ? "matched" : "target-mismatch";

  pageApi.send({
    type: "page-command-result",
    requestId: message.requestId,
    status: status
  }).catch(function () {
    // The page may be closing or the plugin may have been reloaded.
  });
});
