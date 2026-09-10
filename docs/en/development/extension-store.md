---
description: Publish res-downloader plugins to the extension store using GitHub topics, versions, tags, and releases. Learn package structure and installation validation requirements.
---

# Publish Plugins to the Extension Store

This guide is for plugin authors and project maintainers. For installing, updating, or uninstalling plugins as a user, see [Plugin Management](../guide/plugin-management.md).

The extension store distributes plugins through GitHub repositories and releases, with a plugin index on the website. Publish your plugin in your own public repository. The store periodically looks for repositories that meet the publishing requirements and updates its index.

> The `res-downloader-ext` topic means the author has chosen to join the plugin ecosystem. It does not imply official review or a security endorsement.

## Publishing requirements and recommendations

To be listed in the extension store and available for installation, a plugin needs:

1. A public, non-archived GitHub repository that is not a fork, with the `res-downloader-ext` topic.
2. One plugin per repository, with `plugin.json` at the repository root.
3. All runtime files, including JavaScript, WASM, and page scripts, committed to the repository. Do not depend on uncommitted local build output.
4. A GitHub release that is neither a draft nor a prerelease, for example with tag `v1.2.0`.
5. A `plugin.json.version` of `1.2.0`, exactly matching the tag after removing an optional `v` prefix.

Before release, prepare at least one sanitized fixture and run `plugin lint` and `plugin replay` as recommended author checks. Fixtures help validate behavior but are not required for store listing.

We recommend committing a package at `dist/plugin.zip` to improve download speeds for users in mainland China. The store downloads this package first; if it is unavailable, it automatically downloads the GitHub source archive for the tag. You can publish without this optional package, and no separate release asset is required.

## Plugin sources and reserved IDs

**Official** and **Community** are source labels used by the app. Classification depends on the installation method:

- **Extension store**: the repository owner in the index determines the label. Repositories under `putyy` are marked **Official**; others are marked **Community**.
- **Local ZIP**: the label is based on `author.url` in `plugin.json`. A URL pointing to the `putyy` account on GitHub receives the **Official** label; other packages receive **Community**. Use the plugin repository URL for this field.

For local ZIPs, the source label comes from the author URL supplied in the package. It does not prove that the files came from that repository or passed official review.

Plugin IDs have the following restrictions. Changing letter case does not bypass them:

- `builtin.` is reserved for built-in app features. No plugin package may use it.
- `official.` is limited to bundled official plugins and store or local ZIP packages labeled **Official** under the rules above.
- Community plugins cannot use either prefix.

`plugin lint`, `plugin replay`, and `plugin pack` allow you to check, replay, and package `official.` plugins. These commands do not grant official status. During installation, the app also checks whether the plugin's source permits it to use that ID.

## Versions and tags

The store uses the latest stable release. `plugin.json.version` must exactly match its tag after removing an optional `v` prefix.

When publishing an update to plugin content, follow these steps in order:

1. Update the Manifest version.
2. Commit all runtime files. Include fixtures if you provide them.
3. Create a new tag and release.
4. Wait for the store index to refresh.

Do not change an existing tag to point to another commit. Commit WASM, `dist/plugin.zip`, and other build files before creating the tag. Files committed afterward will not be included in that version's downloads, because jsDelivr and GitHub source archives both use the commit referenced by the tag.

## Package structure and download acceleration

Store releases require `plugin.json` at the repository root. Recommended structure:

```text
repository-root/
├── plugin.json
├── main.js
├── fixtures/
├── tests/
├── decrypt.wasm
└── dist/
    └── plugin.zip
```

Run this command from the `res-downloader` source root to generate `<plugin-directory>/dist/plugin.zip`:

```bash
go run main.go plugin pack <plugin-directory>
```

Use `tests/` for the plugin's JavaScript tests, preferably named `*.test.js`.

When packaging:

- The packer excludes `.git/`, `dist/`, `tests/`, and the output file itself, but retains `fixtures/`.
- Place `plugin.json` at the root of `plugin.zip`.
- Rebuild the package for each release and commit `dist/plugin.zip` before creating the tag, so its runtime files and version match that tag.

Do not include account details, capture logs, cookies, Authorization headers, fixtures containing real user data, or unrelated large files.

## Installation validation

During installation, the app checks ZIP structure, extracted size, `plugin.json`, plugin ID, version, permissions, entry files, and runtime code. The package's `plugin.json` must match the content read by the store index from the same tag.

The installer ignores common macOS archive metadata and rejects symlinks, duplicate files, files whose extraction paths escape the plugin directory, oversized files, and invalid `plugin.json` files.

For local ZIP installations, the app displays a content digest (SHA-256) to help identify the package. It is calculated from the extracted file paths and contents, not from the ZIP file itself.

The store index does not provide a content digest, so the app only compares the local package's plugin ID and version with the store entry. Matching IDs and versions do not prove that the two packages contain identical files.

## Other distribution methods

Developers can share the source ZIP that GitHub automatically generates for the version's tag through cloud storage or other channels. When a user selects it under **Plugins → Install ZIP**, the app first shows:

- Plugin ID, name, author, version, and API version.
- Requested domains and capabilities.
- The local ZIP's content digest.
- Whether its plugin ID and version match a locally cached store entry.
- Whether it will replace an installed version.

Installation starts only after the user confirms. Updating or replacing an external plugin preserves its settings and the previous version, allowing the user to roll back after an update.

Updating a bundled JavaScript plugin also requires:

- A package labeled **Official** under the [source rules](#plugin-sources-and-reserved-ids). Community packages cannot replace bundled plugins.
- The same ID as the bundled plugin.

All updates and replacements must pass version and permission checks.

## Recommended release checks

- `plugin.json` is at the repository root, with a stable ID associated with the repository that follows the [reserved ID rules](#plugin-sources-and-reserved-ids).
- The version follows semantic versioning and matches the release tag.
- Permissions and domains are limited to what is actually needed.
- The README describes supported sites, main features, required settings, and known limitations.
- Fixtures, logs, and examples are sanitized.
- The final commit passes `go run main.go plugin lint <plugin-directory>` and all fixture replays.
- Installation checks succeed using GitHub's generated source ZIP for the tag.
- If you provide an acceleration package, `go run main.go plugin pack <plugin-directory>` succeeds, its `dist/plugin.zip` output is committed before tagging, and installation from that package is checked.
- The new version uses a new tag without moving or overwriting an old one.
