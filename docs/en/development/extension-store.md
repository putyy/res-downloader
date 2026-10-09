---
description: Publish res-downloader plugins to the extension store using GitHub topics, versions, tags, and releases. Learn package structure and installation validation requirements.
---

# Extension Store

This guide is for plugin authors and project maintainers. For installing, updating, or uninstalling plugins as a user, see [Plugin Management](../guide/plugin-management.md).

The extension store distributes plugins through GitHub repositories and releases, with a plugin index on the website. Publish your plugin in your own public repository. The store periodically looks for repositories that meet the publishing requirements and updates its index.

> The `res-downloader-ext` topic means the author has chosen to join the plugin ecosystem. It does not imply official review or a security endorsement.

## Publishing requirements and recommendations

To be listed in the extension store and available for installation, a plugin needs:

1. A public, non-archived GitHub repository that is not a fork, with the `res-downloader-ext` topic.
2. One plugin per repository, with `plugin.json` at the repository root.
3. All runtime files, including JavaScript, WASM, and page scripts, committed to the repository. Do not depend on uncommitted local build output.
4. A GitHub release that is neither a draft nor a prerelease, for example with tag `v1.0.0`.
5. A `plugin.json.version` of `1.0.0`, exactly matching the tag after removing an optional `v` prefix.

Fixtures are not required for store listing, but sanitized data is recommended for validating behavior; see [Release checks](#recommended-release-checks).

## Plugin sources and reserved IDs

**Official** and **Community** are source labels used by the app. Classification depends on the installation method:

- **Extension store**: the repository owner in the index determines the label. Repositories under `putyy` are marked **Official**; others are marked **Community**.
- **Local ZIP**: the label is based on `author.url` in `plugin.json`. A URL pointing to the `putyy` account on GitHub receives the **Official** label; other packages receive **Community**. Use the plugin repository URL for this field.

For local ZIPs, the source label comes from the author URL supplied in the package. It does not prove that the files came from that repository or passed official review.

Reserved plugin ID prefixes are case-insensitive:

- `builtin.` is reserved for built-in app features. No plugin package may use it.
- `official.` is limited to bundled official plugins and store or local ZIP packages labeled **Official** under the rules above.

`plugin lint`, `plugin replay`, and `plugin pack` accept `official.` plugins but do not grant official status. Installation still validates the source and ID.

## Versions and tags

The store uses the latest stable release. Versions and tags must follow the [publishing requirements](#publishing-requirements-and-recommendations) above.

When publishing an update to plugin content, follow these steps in order:

1. Update the Manifest version.
2. Commit all runtime files and any fixtures. If providing `dist/plugin.zip`, rebuild and commit it too.
3. Create a new tag and release.
4. Wait for the store index to refresh.

Use a new tag for each version; do not move existing tags. Downloads use the commit referenced by the tag, so WASM, acceleration packages, or other build files committed afterward are not included in that version.

## Automated releases with GitHub Actions

Each official plugin's `.github/workflows/release.yml` calls the shared release workflow on the host repository's `master` branch.

After updating the version, rebuilding, and committing the source and `dist/plugin.zip` as above, push the matching `vMAJOR.MINOR.PATCH` tag. Actions checks the version and ZIP contents, then creates a stable release with the package attached.

## Package structure and download acceleration

A `dist/plugin.zip` acceleration package is recommended to improve download speeds in mainland China. The store tries it first, then falls back to the GitHub source ZIP for the tag. The package is optional; no separate release asset is required.

Recommended structure:

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

Place `plugin.json` at the root of `plugin.zip`. See [Plugin directories and loading](plugins.md#plugin-directories-and-loading) for test conventions and packaging exclusions. The package retains `fixtures/`.

Do not include account details, capture logs, cookies, Authorization headers, fixtures containing real user data, or unrelated large files.

## Installation validation

During installation, the app checks ZIP structure, extracted size, `plugin.json`, plugin ID, version, permissions, entry files, and runtime code. The package's `plugin.json` must match the content read by the store index from the same tag.

The installer ignores common macOS archive metadata and rejects symlinks, duplicate files, files whose extraction paths escape the plugin directory, oversized files, and invalid `plugin.json` files.

For local ZIP installations, the app displays a content digest (SHA-256) to help identify the package. It is calculated from the extracted file paths and contents, not from the ZIP file itself.

The store index does not provide a content digest, so the app only compares the local package's plugin ID and version with the store entry. Matching IDs and versions do not prove that the two packages contain identical files.

## Other distribution methods

Share the GitHub source ZIP for the tag through cloud storage or other channels. Users import it through **Plugins → Install ZIP**, then confirm the plugin information, permissions, content digest, and replacement notice.

Updating or replacing an external plugin preserves its settings and the previous version for rollback.

Updating a bundled JavaScript plugin also requires:

- A package labeled **Official** under the [source rules](#plugin-sources-and-reserved-ids). Community packages cannot replace bundled plugins.
- The same ID as the bundled plugin.

All updates and replacements must pass version and permission checks.

## Recommended release checks

- The repository follows the publishing rules, its plugin ID is stable, and the semantic version matches the tag; permissions and domains are limited to actual needs.
- The README describes supported sites, main features, required settings, and known limitations; fixtures, logs, and examples are sanitized.
- The final commit passes `plugin lint` and `plugin replay` for every fixture.
- Installation succeeds from the tag's GitHub source ZIP. If providing an acceleration package, also verify that its contents match the tag and that it installs correctly.
