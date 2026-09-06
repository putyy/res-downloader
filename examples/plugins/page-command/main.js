function onObservation(observation, api) {
  var payload = JSON.parse(observation.response.body);
  api.emit({
    groupKey: "video:" + payload.id,
    title: payload.title || "",
    kind: "media.video",
    tracks: [{
      id: "video-primary",
      role: "video",
      url: payload.url,
      mime: "video/mp4",
      extension: ".mp4"
    }],
    requiredTracks: ["video"],
    capabilities: ["download"],
    actions: [{
      id: "inspect-page-resource",
      data: {assetId: String(payload.id)}
    }]
  });
  return {decision: "continue"};
}

function onPageMessage(message, context, api) {
  if (!message || message.type !== "page-command-result") {
    return {ok: false, error: "unsupported page message"};
  }
  if (typeof message.requestId !== "string" || typeof message.status !== "string") {
    return {ok: false, error: "invalid page command result"};
  }
  api.log("page command " + message.requestId + " returned " + message.status);
  return {ok: true, data: {accepted: true}};
}

function createDownloadPlan(input) {
  var track = input.resource.tracks[0];
  return {
    inputs: [{
      id: track.id,
      executor: "http-file",
      url: track.url,
      extension: track.extension || ""
    }],
    output: {input: track.id, extension: track.extension || ""}
  };
}
