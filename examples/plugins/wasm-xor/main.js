function onObservation(observation) {
  var payload = JSON.parse(observation.response.body);
  return {
    decision: "continue",
    resources: [{
      title: payload.title || "Encrypted resource",
      kind: "media.video",
      tracks: [{
        id: "video-primary",
        role: "video",
        url: payload.url,
        mime: "video/mp4",
        extension: ".mp4",
        processors: [{
          type: "plugin-wasm",
          options: {processor: "xor", key: payload.key}
        }]
      }],
      requiredTracks: ["video"],
      capabilities: ["download", "preview", "open", "copy"],
      preview: {renderer: "video", mode: "range-proxy", mime: "video/mp4", trackId: "video-primary"}
    }]
  };
}

function createDownloadPlan(input) {
  var track = input.resource.tracks[0];
  return {
    inputs: [{
      id: track.id,
      executor: "http-file",
      url: track.url,
      headers: track.headers || {},
      extension: track.extension || "",
      processors: track.processors || []
    }],
    output: {input: track.id, extension: track.extension || ""}
  };
}
