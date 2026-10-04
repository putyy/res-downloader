// Synthetic, sanitized catalog. No website endpoint is called or implied.
var catalog = [
  {id: "note-1", title: "Example one", text: "Example one\nA short UTF-8 note."},
  {id: "note-2", title: "Example two", text: "Example two\n第二条示例。"},
  {id: "note-3", title: "Example three", text: "Example three\nFinal sample."}
];
function item(value) {
  return {id: value.id, pluginId: pageApi.pluginId, kind: "document.text", title: value.title,
    pageUrl: "https://www.example.com/#" + value.id, capabilities: ["detail", "resolve"]};
}
function lookup(id, signal) {
  if (signal.aborted) throw new Error("cancelled");
  var value = catalog.find(function (entry) { return entry.id === id; });
  if (!value) throw new Error("unknown item");
  return value;
}
pageApi.operations.handle("search", async function (ctx) {
  if (ctx.signal.aborted) throw new Error("cancelled");
  var matches = catalog.filter(function (entry) { return entry.title.toLowerCase().includes(ctx.input.query.toLowerCase()); });
  var offset = ctx.cursor ? Number(ctx.cursor) : 0;
  if (!Number.isInteger(offset) || offset < 0 || offset > matches.length) throw new Error("invalid cursor");
  var selected = matches.slice(offset, offset + ctx.limit), next = offset + selected.length;
  return {data: {items: selected.map(item)}, count: selected.length, hasMore: next < matches.length,
    nextCursor: next < matches.length ? String(next) : "", truncated: false};
});
pageApi.operations.handle("detail", async function (ctx) {
  return {data: item(lookup(ctx.input.id, ctx.signal)), count: 1};
});
["resolve", "text"].forEach(function (id) {
  pageApi.operations.handle(id, async function (ctx) {
    var entry = lookup(ctx.input.id, ctx.signal);
    return {data: {id: entry.id, title: entry.title, text: entry.text}, count: 1};
  });
});
function ready() { return pageApi.operations.setState({ready: true, login: "unknown", context: "sample-catalog"}); }
if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", function () { void ready(); }, {once: true});
else void ready();
window.addEventListener("hashchange", function () { void ready(); });
