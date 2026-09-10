---
description: Download the res-downloader Plugin SDK v1 Manifest JSON Schema and TypeScript declarations to enable editor assistance for plugin development.
---

# Plugin SDK v1

The public protocol helper files for plugin development are maintained in `docs/public/plugin-sdk/` and published unchanged during the build, shared by all documentation languages. They provide editor assistance and external tool integration; they are not used for runtime plugin loading or CLI validation.

## Files

- [Manifest Schema](/plugin-sdk/plugin-v1.schema.json): the JSON Schema for `plugin.json`, covering permissions, matching rules, page scripts, resource kinds, settings, declarative extractors, WASM processors, and resource actions.
- [`plugin-v1.d.ts`](/plugin-sdk/plugin-v1.d.ts): TypeScript declarations for the JavaScript plugin API, including Observation, runtime APIs, resources, page commands, refresh results, and download plans.

In an editor supporting JSON Schema, associate `plugin.json` with this schema URL:

```text
https://res.putyy.com/plugin-sdk/plugin-v1.schema.json
```

For a JavaScript project, download `plugin-v1.d.ts` into your development directory and enable type hints through editor configuration or `/// <reference path="./plugin-v1.d.ts" />`. The runtime still executes plain JavaScript; no TypeScript build is required.

The Go backend's protocol models and validation logic are authoritative when the app loads a plugin. Before submitting a plugin, run from the `res-downloader` source root:

```text
go run main.go plugin lint <plugin-directory>
go run main.go plugin replay <plugin-directory> <fixture-file>
```

See [Plugin Development](plugins.md) for the complete Manifest, permissions, hooks, resource model, and WASM ABI documentation.

When adding or changing the plugin protocol, update the Schema, type declarations, examples, fixtures, and developer guide together.

JSON Schema provides field assistance but cannot establish a package's distribution source. `plugin lint`, `plugin replay`, and `plugin pack` also do not grant official status; they allow authors to work with `official.` plugins during development. See [Plugin sources and reserved IDs](extension-store.md#plugin-sources-and-reserved-ids) for installation source classification and restrictions on the `builtin.` / `official.` prefixes.
