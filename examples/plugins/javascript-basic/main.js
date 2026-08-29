function onObservation(observation, api) {
  var payload = JSON.parse(observation.response.body);
  var settings = observation.settings || {};
  var minimumSize = settings.minimumSize || 0;
  if ((payload.size || 0) < minimumSize) return {decision: "continue"};

  api.emit({
    title: payload.title || "",
    kind: "media.video",
    tracks: [{
      id: "video-primary",
      role: "video",
      url: payload.url,
      mime: "video/mp4",
      extension: ".mp4",
      size: payload.size || 0
    }],
    requiredTracks: ["video"],
    capabilities: ["download", "preview", "open", "copy"],
    preview: {renderer: "video", mode: "proxy", mime: "video/mp4", trackId: "video-primary"},
    metadata: {"example.assetId": payload.id}
  });
  return {decision: "continue"};
}

function createDownloadPlan(input) {
  var track = input.resource.tracks[0];
  return {
    inputs: [{
      id: track.id,
      executor: "http-file",
      url: track.url,
      headers: track.headers || {},
      extension: track.extension || ""
    }],
    output: {input: track.id, extension: track.extension || ""}
  };
}
