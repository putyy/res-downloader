function endsWith(value, suffix) {
  return value.slice(-suffix.length) === suffix;
}

function firstHeader(headers, name) {
  var expected = name.toLowerCase();
  for (var key in headers) {
    if (Object.prototype.hasOwnProperty.call(headers, key) && key.toLowerCase() === expected) {
      var values = headers[key];
      return values && values.length ? values[0] : "";
    }
  }
  return "";
}

function queryValue(rawUrl, name) {
  var match = new RegExp("[?&]" + name + "=([^&#]*)").exec(rawUrl);
  return match ? decodeURIComponent(match[1].replace(/\+/g, " ")) : "";
}

function emptyResponse() {
  return {
    statusCode: 200,
    headers: {"Content-Type": "text/plain; charset=utf-8"},
    body: "The content does not exist"
  };
}

function mediaGroupKey(item, rawUrl, isImage) {
  var identity = queryValue(rawUrl, "encfilekey");
  var identityType = "encfilekey";

  if (!identity) {
    var idFields = ["mediaId", "id", "objectId"];
    for (var index = 0; index < idFields.length; index++) {
      var value = item[idFields[index]];
      if ((typeof value === "string" || typeof value === "number") && String(value)) {
        identity = String(value);
        identityType = idFields[index];
        break;
      }
    }
  }

  if (!identity) {
    var baseUrl = rawUrl.split("#")[0].split("?")[0];
    if (!endsWith(baseUrl, "/stodownload")) {
      identity = baseUrl;
      identityType = "url";
    }
  }

  if (!identity) return "";
  var groupKey = "wechat:" + (isImage ? "image" : "video") + ":" + identityType + ":" + identity;
  return groupKey.length <= 512 ? groupKey : "";
}

function mediaResources(body, pageUrl) {
  var payload;
  try {
    payload = JSON.parse(body);
  } catch (error) {
    return [];
  }
  var media = payload.media;
  if (!Array.isArray(media)) return [];

  var resources = [];
  for (var index = 0; index < media.length; index++) {
    var item = media[index] || {};
    var rawUrl = typeof item.url === "string" ? item.url : "";
    if (!rawUrl) continue;
    if (typeof item.urlToken === "string") rawUrl += item.urlToken;

    var isImage = Number(item.mediaType) === 9;
    var formats = [];
    if (Array.isArray(item.spec)) {
      for (var specIndex = 0; specIndex < item.spec.length; specIndex++) {
        var format = item.spec[specIndex] && item.spec[specIndex].fileFormat;
        if (typeof format === "string") formats.push(format);
      }
    }

    var metadata = {"wechat.fileFormats": formats};
    var processors = [];
    if (typeof item.decodeKey === "string" && item.decodeKey) {
      processors.push({
        type: "plugin-wasm",
        options: {processor: "isaac64-prefix", seed: item.decodeKey}
      });
    }

    var trackId = isImage ? "image-primary" : "video-primary";
    var groupKey = mediaGroupKey(item, rawUrl, isImage);
    var actions = [];
    if (!isImage && processors.length) {
      actions.push({
        id: "decrypt-local-file",
        data: {options: {seed: item.decodeKey}}
      });
    }
    resources.push({
      groupKey: groupKey,
      title: typeof payload.description === "string" ? payload.description : "",
      coverUrl: typeof item.coverUrl === "string" ? item.coverUrl : "",
      kind: isImage ? "media.image" : "media.video",
      tracks: [{
        id: trackId,
        role: isImage ? "image" : "video",
        url: rawUrl,
        mime: isImage ? "image/png" : "video/mp4",
        extension: isImage ? ".png" : ".mp4",
        size: Number(item.fileSize) || 0,
        processors: processors
      }],
      requiredTracks: [isImage ? "image" : "video"],
      capabilities: ["download", "preview", "open", "copy"],
      preview: {
        renderer: isImage ? "image" : "video",
        mode: "range-proxy",
        mime: isImage ? "image/png" : "video/mp4",
        trackId: trackId
      },
      metadata: metadata,
      actions: actions,
      source: {pageUrl: pageUrl}
    });
  }
  return resources;
}

function injectWechatHooks(body) {
  var mediaPattern = /get\s*media\(\)\{/g;
  var commentPattern = /async\s*finderGetCommentDetail\((\w+)\)\s*\{return(.*?)\s*}\s*async/g;

  var next = body.replace(mediaPattern, [
    "get media(){",
    "if(this.objectDesc){",
    "fetch(\"https://wxapp.tc.qq.com/res-downloader/wechat?type=1\",{",
    "method:\"POST\",mode:\"no-cors\",body:JSON.stringify(this.objectDesc)",
    "});",
    "};"
  ].join(""));

  return next.replace(commentPattern, [
    "async finderGetCommentDetail($1){",
    "var res=await$2;",
    "if(res&&res.data&&res.data.object&&res.data.object.objectDesc){",
    "fetch(\"https://wxapp.tc.qq.com/res-downloader/wechat?type=2\",{",
    "method:\"POST\",mode:\"no-cors\",body:JSON.stringify(res.data.object.objectDesc)",
    "});",
    "}",
    "return res;",
    "}async"
  ].join(""));
}

function onObservation(observation, api) {
  var request = observation.request;
  var response = observation.response;
  var settings = observation.settings || {};
  var version = api.pluginVersion;

  if (observation.stage === "request") {
    var type = queryValue(request.url, "type");
    var fullIntercept = settings.fullIntercept !== false;
    var wanted = (fullIntercept && type === "1") || (!fullIntercept && type === "2");
    var resources = [];
    if (wanted && request.body) resources = mediaResources(request.body, request.url);
    return {
      decision: "stop",
      resources: resources,
      syntheticResponse: emptyResponse()
    };
  }

  if (!response || (response.statusCode !== 200 && response.statusCode !== 206)) {
    return {decision: "continue"};
  }

  var host = request.host;
  var path = request.path;
  var contentType = (response.contentType || "").toLowerCase();
  if (endsWith(host, "finder.video.qq.com") && contentType.indexOf("video/") === 0) {
    var origin = firstHeader(request.headers, "Origin");
    return {decision: origin.indexOf("mp.weixin.qq.com") >= 0 ? "continue" : "stop"};
  }

  var body = response.body || "";
  var changed = false;
  var suffix = ".js?v=" + version + "\"";
  if (endsWith(host, "channels.weixin.qq.com") &&
      (path.indexOf("/web/pages/feed") >= 0 || path.indexOf("/web/pages/home") >= 0)) {
    var pageBody = body.split(".js\"").join(suffix);
    changed = changed || pageBody !== body;
    body = pageBody;
  }

  if (endsWith(host, "res.wx.qq.com")) {
    if (endsWith(request.url, ".js?v=" + version)) {
      var dependencyBody = body.split(".js\"").join(suffix);
      changed = changed || dependencyBody !== body;
      body = dependencyBody;
    }
    if (path.indexOf("web/web-finder/res/js/virtual_svg-icons-register.publish") >= 0) {
      var injected = injectWechatHooks(body);
      changed = changed || injected !== body;
      body = injected;
    }
  }

  return changed ? {decision: "continue", patch: {body: body}} : {decision: "continue"};
}

function createDownloadPlan(input) {
  var resource = input.resource;
  var options = input.options || {};
  var settings = options.settings || {};
  var qualityModes = {"default": 0, "ultra": 1, "high": 2, "medium": 3, "low": 4};
  var quality = qualityModes[settings.quality] || 0;
  var primaryTrack = resource.tracks && resource.tracks.length ? resource.tracks[0] : {};
  var rawUrl = primaryTrack.url;
  var metadata = resource.metadata || {};
  var formats = metadata["wechat.fileFormats"] || [];

  if (quality === 1 && rawUrl.indexOf("encfilekey=") >= 0 && rawUrl.indexOf("token=") >= 0) {
    var encfilekey = queryValue(rawUrl, "encfilekey");
    var token = queryValue(rawUrl, "token");
    rawUrl = rawUrl.split("?")[0] + "?encfilekey=" + encodeURIComponent(encfilekey) + "&token=" + encodeURIComponent(token);
  } else if (quality > 1 && formats.length) {
    var choices = [formats[0], formats[Math.floor(formats.length / 2)], formats[formats.length - 1]];
    var choiceIndex = quality - 2;
    if (choiceIndex >= 0 && choiceIndex < choices.length) {
      rawUrl += (rawUrl.indexOf("?") >= 0 ? "&" : "?") + "X-snsvideoflag=" + encodeURIComponent(choices[choiceIndex]);
    }
  }

  return {
    inputs: [{
      id: primaryTrack.id || "primary",
      executor: "http-file",
      url: rawUrl,
      headers: primaryTrack.headers || {},
      extension: primaryTrack.extension || "",
      processors: primaryTrack.processors || []
    }],
    output: {
      input: primaryTrack.id || "primary",
      extension: primaryTrack.extension || ""
    }
  };
}
