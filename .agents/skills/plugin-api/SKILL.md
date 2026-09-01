---
name: plugin-api
description: Design and implement generic res-downloader host capabilities required by site plugins, keeping the Go protocol, runtime, validation, SDK schema, TypeScript declarations, examples, and documentation aligned. Use only when the user explicitly requests a host plugin-API enhancement. Do not use for site-private extraction logic.
---

# Plugin API

Add a reusable host capability when a plugin requirement cannot be met by the current public protocol. Keep site-specific behavior in plugins and keep the public API coherent across implementation, validation, SDK artifacts, examples, and documentation.

## Authorization and boundary

- Require an explicit user request to modify host capabilities. A `plugin-dev` report of a missing capability is evidence for a follow-up, not authorization to change the host.
- Read `docs/architecture.md` and `docs/plugins.md` completely, then inspect `docs/plugin-sdk/README.md` and the closest protocol implementation and examples.
- Demonstrate the capability gap before designing a new contract: identify the consumer plugin requirement, enumerate the closest existing protocol primitives, and explain why they cannot express the requirement safely. Implementation complexity or site-specific inconvenience alone is not a host capability gap.
- State the missing capability, affected plugin behavior, proposed generic contract, permissions, compatibility impact, and files likely to change before implementation.
- Do not embed target-site hostnames, selectors, signing constants, private endpoint shapes, or site-specific decryption logic in the host. The capability must be reusable and expose only the minimal primitive the plugin needs.
- Do not weaken access controls, plugin isolation, bounds, origin checks, permission checks, or credential handling to make one plugin work.

## Design invariants

- Treat Go models and runtime validation as the executable source of truth, while keeping all public SDK representations synchronized.
- Prefer an additive, least-privilege change. Add or extend an explicit capability when the operation has new security or resource implications.
- Validate untrusted plugin input at the host boundary, including types, sizes, counts, paths, URLs, domains, timeouts, and lifecycle cleanup as applicable.
- Preserve existing plugin behavior and API version compatibility. Do not increment `apiVersion`, introduce a breaking change, or silently reinterpret an existing field without presenting the compatibility decision to the user.
- Define understandable failure behavior and ensure unsupported or malformed input fails closed.

## Implementation coverage

Trace the feature end to end and update only applicable layers:

- protocol models and serialization under `internal/model/`;
- Manifest, capability, and candidate validation under `internal/plugin/`;
- JavaScript, declarative, page-script, download-plan, processor, or lifecycle runtime code;
- host integration needed to supply the generic primitive;
- focused protocol, validation, and runtime tests;
- `docs/plugin-sdk/plugin-v1.schema.json`;
- `docs/plugin-sdk/plugin-v1.d.ts`;
- `docs/plugins.md` and `docs/plugin-sdk/README.md`;
- the smallest representative example or sanitized fixture under `examples/plugins/`.

If a layer is not affected, do not modify it merely for symmetry. Never place real credentials, account data, private URLs, or captured user responses in tests or examples.

## Validation and handoff

Perform the applicable repository static checks for formatting, Go/API consistency, JSON Schema validity, TypeScript declaration consistency, permissions, examples, and documentation links. Do not start the desktop application, control a browser, replay live observations, install a plugin, or perform capture/download/playback acceptance automatically.

Report the public contract, permission model, compatibility decision, affected layers, static-check results, and known limitations. Provide a manual checklist that exercises the capability through a real plugin in the application, including denied-permission behavior, malformed or oversized input, lifecycle cleanup, and the relevant capture, download, processing, and playback path. Do not claim host or plugin runtime success until the user completes that verification.
