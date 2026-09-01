---
name: plugin-release
description: Prepare, validate, and publish a res-downloader site plugin from its independent GitHub repository. Use for plugin version bumps, release readiness, repository setup, tags, GitHub Releases, or extension-store publication. Do not use for developing plugin behavior or releasing the host application.
---

# Plugin Release

Prepare a plugin release whose committed source, packaged ZIP, version, tag, GitHub repository, and extension-store metadata all describe the same immutable artifact.

## Source of truth

- Read `docs/plugins.md` and `docs/extension-store.md` from the res-downloader host repository before release work.
- Inspect the target plugin's Manifest, README, fixtures, runtime files, and existing release convention.
- Follow the current repository documentation when it conflicts with this skill.

## Establish the repository boundary

1. Resolve the plugin directory and its canonical path.
2. Run `git -C <plugin-directory> rev-parse --show-toplevel` and compare the canonical Git top-level with the canonical plugin directory.
3. Treat the plugin as an independent repository only when those paths are identical. A plugin directory merely contained in the res-downloader worktree is not an independent plugin repository.
4. If it is not independent, stop release mutation and ask the user for the intended GitHub repository URL. Require explicit authorization before running `git init`, adding or changing a remote, moving files, or creating a repository. Never publish the host repository as the plugin repository.
5. Confirm that `plugin.json` is at the independent repository root. The extension store does not publish a plugin nested below that root.

## Release preflight

- Determine the intended version and optional `v`-prefixed tag. Require the Manifest version to equal the tag after removing the optional `v` prefix.
- Check the GitHub repository and remote identity before any push. For extension-store discovery, verify it is public, not archived, not a fork, and has the `res-downloader-ext` topic; report any unmet condition.
- Confirm the plugin ID is stable and allowed for its publisher, permissions and domains are minimal, the README covers support and limitations, and fixtures and logs contain no credentials or private user data.
- Perform repository-local validation. Run `go run main.go plugin lint <plugin-directory>` and replay every documented sanitized fixture with `go run main.go plugin replay <plugin-directory> <fixture>` from the res-downloader source tree. Identify any fixture file that is intentionally not a replay input. Offline replay is required release evidence but does not replace live acceptance. Do not use live capture, application startup, installation, download, or playback as automated acceptance evidence.
- Package only after the final source changes with `go run main.go plugin pack <plugin-directory>`. Verify `dist/plugin.zip` exists, is non-empty, and contains the same root Manifest version. If packaged source changes, rerun lint, all affected fixture replays, and pack.
- Inspect staged, unstaged, and untracked files. The final worktree must be clean, and the latest `dist/plugin.zip` plus every release input must be committed before tagging. Never auto-commit unrelated changes.
- Show the user the final branch, commit hash and message, version, tag, remote, changed files since the previous release, and validation results. A clean worktree proves files are committed; the user must still confirm that the intended release is complete.

## Authorization gates

Read-only inspection does not require confirmation. Obtain explicit confirmation immediately before each state-changing group below, showing its exact target and command or equivalent action:

- initialize Git, create or change a remote, create a GitHub repository, or change repository topics or visibility;
- stage or commit files;
- create a tag or push a branch or tag;
- create, edit, publish, or upload assets to a GitHub Release.

An earlier request to “prepare a release” is not authorization to push or publish. Do not move, overwrite, or delete an existing tag. If a push or Release operation partially succeeds, report the resulting local and remote state and obtain new confirmation before retrying, repairing, or deleting anything.

## Publish sequence

1. Confirm the release commit is the intended commit and is reachable from the expected remote branch. If the branch is ahead, ask before pushing it; if it is behind or diverged, stop and resolve that state before tagging.
2. Verify the proposed tag does not already exist locally or remotely.
3. Ask for confirmation using the exact commit, tag, and remote, then create the tag and push it.
4. Ask separately before creating or publishing the GitHub Release. Use a new, non-draft, non-prerelease Release for extension-store discovery unless the user explicitly requests a release that is not intended for the store.
5. Verify the published Release and tag point to the expected commit and report that the extension-store index refresh is asynchronous.

## Completion report

Report the repository URL, plugin ID and version, release commit and tag, lint and per-fixture replay results, packaging results, `dist/plugin.zip` status, remote mutations actually performed, and any store-discovery requirements still pending. State clearly that live installation and behavior were not automatically verified, and ask the user to manually install both `dist/plugin.zip` and the GitHub tag source archive, then verify permissions, capture, metadata, preview, download, and output playback.
