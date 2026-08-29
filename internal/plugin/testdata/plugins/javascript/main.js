function onObservation(observation, api) {
  var payload = JSON.parse(observation.response.body);
  api.emit({
    title: payload.title,
    kind: "media.video",
    tracks: [{id: "video-primary", role: "video", url: payload.url, mime: "video/mp4", extension: ".mp4"}],
    requiredTracks: ["video"],
    capabilities: ["download", "preview", "open", "copy"],
    preview: {renderer: "video", mode: "proxy", mime: "video/mp4", trackId: "video-primary"}
  });
  return {decision: "continue"};
}
