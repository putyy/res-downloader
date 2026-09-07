async function inspectResource(message) {
  if (!message || message.protocol !== 1 || message.type !== "resource-action" || message.actionId !== "inspect-page-resource") return;
  var targetAssetId = message.data && message.data.assetId;
  var currentAssetId = document.documentElement.getAttribute("data-example-asset-id");
  if (!targetAssetId || targetAssetId !== currentAssetId) {
    await pageApi.commands.report(message.requestId, {state: "rejected"});
    return;
  }
  var claim = await pageApi.commands.claim(message.requestId);
  if (!claim.accepted) return; // Another matching tab already owns this execution.
  try {
    await pageApi.commands.report(message.requestId, {state: "running", progress: 0, message: "Checking page resource"});
    var reply = await pageApi.send({type: "page-command-result", requestId: message.requestId, status: "matched"});
    if (!reply.ok) throw new Error("Plugin could not process the result");
    await pageApi.commands.report(message.requestId, {state: "completed", progress: 100, message: "Page resource matched"});
  } catch (error) {
    await pageApi.commands.report(message.requestId, {state: "failed", message: "Page resource check failed"});
  }
}

pageApi.onMessage(function (message) {
  inspectResource(message).catch(function () { /* The page closed or the command expired. */ });
});
