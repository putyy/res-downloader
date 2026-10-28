function onPageMessage(message, context, api) {
  if (!message || message.type !== "operation-result") return {ok: false};
  if (message.operationId !== "resolve" && message.operationId !== "text") return {ok: true};
  var data = message.data;
  if (!data || !/^note-[1-3]$/.test(data.id) || typeof data.title !== "string" || typeof data.text !== "string" || data.text.length > 4096) return {ok: false};
  var saved = api.capture.save(data.text);
  return {ok: true, resources: [{
    groupKey: "example:" + data.id, title: data.title, kind: "document.text", primaryType: "document",
    tracks: [{id: "text", role: "document", executor: "capture-file", captureKey: saved.captureKey,
      mime: "text/plain; charset=utf-8", extension: ".txt", size: saved.size}],
    requiredTracks: ["document"], capabilities: ["download"]
  }]};
}
