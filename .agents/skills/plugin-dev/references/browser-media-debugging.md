# Browser media debugging

Read this when preview/download fails, capture contains noise, or navigation produces missing/stale works. Use only the browser and diagnostic actions currently authorized by the user.

## Establish the difference

- Where possible, compare the same work/file through failing and working routes. Trace metadata candidate → player request → emitted track → preview/download input. For an installed plugin, check its version/digest so old records or stale page injection are not mistaken for the current build.
- Compare method, status, URL/path/query differences, headers, redirects, expiry, and proxy path as relevant. A different CDN host or a 403 alone does not identify the cause. A TLS failure produces no HTTP evidence about the proposed fix.
- If permitted and useful, make a bounded HEAD or Range diagnostic request to an observed URL. Stop at headers or a small byte limit even if Range is ignored; do not download the full asset merely to diagnose access. Distinguish this from host application acceptance.
- After a user reports the same failure again, revisit the failed request and prior hypothesis before issuing another repair. Convert the observed difference into a sanitized regression case; passing a fixture invented from the implementation is not evidence of the live cause.

## Preserve the playable resource

- An API URL may require player-added parameters or refreshed session data. Neither maximum resolution, first CDN position, a filename/MIME, nor a `blob:` source proves that a candidate is a complete playable file.
- Prefer candidates supported by playback evidence. Preserve the full successful URL and necessary request context. When required data is missing, wait or represent the resource as incomplete instead of advertising a ready download.
- Correlate variants only with evidence of the same file/track. Preserve original signatures and parameters; do not fix mismatches by dropping arbitrary query fields, changing hosts, or copying credentials between works. Define site-specific matching in the plugin, not in this skill.
- Determine separate audio/video from the selected format and actual track data. Do not infer it from `blob:` or an unrelated audio list. Keep required tracks, download assembly, and advertised preview capability consistent.

## Test timing and host contracts

Choose cases relevant to the change rather than requiring every scenario for every plugin:

- Initial load and feed transitions; preloaded or stale mounted cards; requests before/after metadata; navigation during an in-flight page message. The emitted ID/title must describe the current work without needing another scroll.
- Actual host empty returns (`null`, `undefined`, empty arrays) and resource merge behavior. Check whether track IDs replace entries and whether required tracks/capabilities accumulate before changing a published resource's mode.
- Extraction and generic suppression separately. Assert `handled` where replay cannot; include business responses labeled binary, default ports, Range requests, and legitimate fallback traffic as relevant. MIME alone does not establish that a business response is downloadable media.

Keep fixtures and saved diagnostic output sanitized before writing them or printing them into logs. Preserve structural differences with fictional values, never live cookies, tokens, signed URLs, or unrelated account data.
