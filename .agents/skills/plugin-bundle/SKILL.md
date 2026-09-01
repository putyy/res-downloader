---
name: plugin-bundle
description: Synchronize an official res-downloader plugin source directory into the host application's bundled-plugin snapshot. Use when updating `internal/plugin/bundled/` from a finished official plugin. Do not use for plugin development, community plugins, or plugin publication.
---

# Plugin Bundle

Use the repository command to replace the host's bundled snapshot with the final source of one official plugin, then statically verify the exact synchronized result.

## Preconditions

- Read `docs/plugins.md` and inspect the source Manifest and the existing matching directory under `internal/plugin/bundled/`.
- Require a source directory outside `internal/plugin/bundled/` with an `official.*` plugin ID. Do not grant an official identity or bundle a community plugin.
- Require plugin development to be complete. This skill does not fix site behavior, alter the host API, publish a plugin, or package a Release.
- Before changing the host snapshot, require the source plugin to pass ordinary `plugin lint` and every documented sanitized fixture replay. Identify fixture files that are intentionally not replay inputs.
- Regenerate the source plugin's final `dist/plugin.zip` with `plugin pack`, verify it is non-empty, and make no further source changes before synchronization. The ZIP is a source-plugin release artifact and is normally excluded from the bundled host snapshot.
- Inspect the worktree and preserve unrelated changes. Determine both the target directory name and every existing bundled directory with the same plugin ID, because synchronization can replace them.
- If a replacement target contains unrelated or ambiguous local changes, stop and ask the user before overwriting it.

## Synchronize

From the res-downloader repository root, first validate and finish the source artifact:

```bash
go run main.go plugin lint <plugin-directory>
go run main.go plugin replay <plugin-directory> <fixture>
go run main.go plugin pack <plugin-directory>
```

Repeat replay for every documented replay fixture. Then run:

```bash
go run main.go plugin sync-bundled <plugin-directory>
```

Use this command instead of copying files manually. It validates the source, excludes development-only content according to repository rules, stages the copy, and replaces the bundled snapshot with the source directory name.

After it succeeds:

1. Resolve the resulting `internal/plugin/bundled/<source-directory-name>` path from command output and repository state.
2. Run `go run main.go plugin lint-bundled <result-directory>`.
3. Compare the source inputs and synchronized snapshot using the command's documented exclusion rules. Inspect Git diff for unexpected deletions, renames, secrets, logs, credentials, account data, or unrelated files.
4. Confirm the bundled snapshot excludes `dist/plugin.zip` and other development-only files according to the command rules; their absence from the host snapshot is expected.
5. Do not start the host, install or reload the plugin, capture traffic, download content, or claim live behavior is verified.

If synchronization or lint fails after modifying the snapshot, preserve diagnostic evidence and report the exact state. Do not manually delete, restore, or overwrite user changes without explicit authorization.

## Completion report

Report the plugin ID and version, source and bundled paths, source lint and per-fixture replay results, final `dist/plugin.zip` status, replaced directories, synchronization and `lint-bundled` results, files changed in the host, and any missing capability or documentation follow-up. Explicitly hand off installation/reload, capture, metadata, preview, download, processing, and output-playback verification to the user.
