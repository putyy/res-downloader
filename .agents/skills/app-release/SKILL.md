---
name: app-release
description: Prepare and trigger a res-downloader application release through its tag-driven GitHub Actions workflow. Use for release readiness, version and tag checks, release commits, or pushing an application release tag. Do not use for plugin releases or local multi-platform packaging.
---

# Application Release

Prepare one exact, committed host-application revision and trigger the existing GitHub Actions release workflow by pushing its tag.

## Source of truth

- Inspect `AGENTS.md`, `CONTRIBUTING.md`, `wails.json`, and `.github/workflows/release.yml` before acting.
- Inspect called release workflows only when needed to verify current platform artifacts or metadata rules.
- The workflow is authoritative. Do not reproduce the macOS, Windows, or Linux release build locally.

## Release preflight

1. Determine the expected release branch, remote, version, and tag.
2. Validate the tag against the workflow's current accepted format. At present it accepts an optional `v` prefix and `X.Y.Z`, optionally followed by `-alpha.N`, `-beta.N`, or `-rc.N`.
3. Read `wails.json` and require `info.productVersion` to be the numeric `X.Y.Z` base of the tag.
4. Inspect release-related documentation and metadata for consistency. Perform only applicable static repository checks; do not start the application, run browser validation, or build platform installers locally.
5. Check whether the proposed tag already exists locally or remotely. Never move, replace, or delete an existing release tag as part of the normal flow.
6. Compare the intended release commit with the previous application release tag. Group the delta by user-visible behavior and release risk, then derive a focused manual-verification checklist from the actual changes. Do not reuse a generic checklist when the release changes installers, startup, persistence, downloads, media tools, certificates, plugins, or platform integration.
7. Inspect the called platform workflows and record the exact artifact matrix for this release, including architecture-specific or dependency-bundled variants. Treat the workflows as the source of truth instead of hardcoding filenames that may drift.

## Commit-completeness gate

- Inspect staged, unstaged, and untracked files with Git status and diff summaries.
- If any file is uncommitted, list it and stop before tagging. Ask the user whether it belongs in this release, is unrelated, or means the release should be cancelled. Never silently ignore untracked files and never auto-commit unrelated work.
- Require a clean worktree before creating the tag.
- Show the current branch, HEAD hash and message, changes since the previous release tag, and the proposed version and tag. A clean worktree establishes mechanical completeness; ask the user to confirm semantic completeness—that the intended latest changes are actually included.
- Check the configured upstream after refreshing remote state. The release commit should be reachable from the expected remote release branch. If the branch is ahead, request confirmation before pushing it; if it is behind or diverged, stop and resolve that state first.
- If no expected upstream or remote exists, stop and ask the user which repository and branch are authoritative.

## Authorization and trigger

- Read-only inspection and static validation do not authorize mutation.
- Require explicit confirmation immediately before staging or committing release changes, showing the exact files and proposed message.
- Require explicit confirmation before pushing a release branch, showing the branch, commit, and remote.
- Immediately before creating and pushing the release tag, present the exact tag, target commit, remote, and commands, and ask for explicit confirmation. One confirmation may cover creating and pushing that exact tag when both actions are clearly listed.
- Pushing the tag triggers `.github/workflows/release.yml`, which creates and publishes the GitHub Release. Do not also create a Release manually unless the user separately requests recovery from a documented workflow failure.
- If tag creation succeeds but push fails, report the remaining local tag. Do not retry, delete, recreate, force-push, or dispatch the workflow without new confirmation.

## Handoff

After a successful tag push, stop active release work. Report the branch, commit, version, tag, remote, static checks, expected artifact matrix, and the risk-based manual checklist derived from the release delta. Tell the user to wait for GitHub Actions and then download the Release artifacts themselves. Their manual verification must cover each applicable artifact and architecture, baseline installation, startup and upgrade behavior, plus every changed high-risk path identified during preflight. Do not claim the application release works merely because the tag was pushed or the workflow completed.
