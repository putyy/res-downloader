---
name: app-release
description: Prepare and trigger a res-downloader application release through its tag-driven GitHub Actions workflow. Use for release readiness, version and tag checks, release commits, or pushing an application release tag. Do not use for plugin releases or local multi-platform packaging.
---

# Application Release

Prepare one exact, committed host-application revision and trigger the existing GitHub Actions release workflow by pushing its tag.

## Source of truth

- Inspect `AGENTS.md`, `CONTRIBUTING.md`, `build/config.yml`, and `.github/workflows/release.yml` before acting.
- Inspect called release workflows only when needed to verify current platform artifacts or metadata rules.
- The workflow is authoritative. Do not reproduce the macOS, Windows, or Linux release build locally.

## Release preflight

1. Determine the expected release branch, remote, version, and tag.
2. Validate the tag against the workflow's current accepted format. At present it accepts an optional `v` prefix and `X.Y.Z`, optionally followed by `-alpha.N`, `-beta.N`, or `-rc.N`.
3. Read `build/config.yml` and require `info.version` to be the numeric `X.Y.Z` base of the tag.
4. Inspect release-related documentation and metadata for consistency. Perform only applicable static repository checks; do not start the application, run browser validation, or build platform installers locally.
5. Check whether the proposed tag already exists locally or remotely. Never move, replace, or delete an existing release tag as part of the normal flow.
6. Compare the intended release commit with the previous application release tag. Group the delta by user-visible behavior and release risk, then derive a focused manual-verification checklist from the actual changes. Do not reuse a generic checklist when the release changes installers, startup, persistence, downloads, media tools, certificates, plugins, or platform integration.
7. Inspect the called platform workflows and record the exact artifact matrix for this release, including architecture-specific or dependency-bundled variants. Treat the workflows as the source of truth instead of hardcoding filenames that may drift.
8. Check the `plugins` entries in `build/bundled-plugins.json` and the preparation in `cmd/bundleplugins/`, `build/Taskfile.yml`, and the release workflows. Release preparation downloads each repository's latest default branch with `go run ./cmd/bundleplugins -download`; platform jobs consume the shared artifact through the platform Taskfiles and `common:bundle:plugins`. Plugin sources are fetched at release time and are not fixed by the host commit. A local checkout or generated bundle does not prove which plugin revision a release will include; use the preparation log's commit IDs when reporting actual release inputs. Keep generated plugin directories under `internal/plugin/bundled/` out of commits. Before direct Go compilation or host checks, prepare local sources with `go run ./cmd/bundleplugins`; do not download remote sources merely for static preflight.

## Commit-completeness gate

- Inspect staged, unstaged, and untracked files with Git status and diff summaries.
- If any file is uncommitted, list it and resolve its release scope before tagging. Complete already authorized release commits; ask only about unrelated or ambiguous changes. Never silently ignore untracked files or auto-commit unrelated work.
- Require a clean worktree before creating the tag.
- Show the current branch, HEAD hash and message, changes since the previous release tag, and the proposed version and tag. A clean worktree establishes mechanical completeness; obtain confirmation that the intended changes are included unless the user already confirmed that exact release scope.
- Check the configured upstream after refreshing remote state. The release commit should be reachable from the expected remote release branch. If the branch is ahead, push only within explicit authorization; if it is behind or diverged, stop and resolve that state first.
- If no expected upstream or remote exists, stop and ask the user which repository and branch are authoritative.

## Authorization and trigger

- Read-only inspection and static validation do not authorize mutation.
- Preserve explicit authorization already given for the same release scope and actions. Preparing a release alone does not authorize committing, pushing, or publishing. Follow the project's commit rules without asking again for an already requested commit.
- Before requesting missing approval, finish applicable preparation and present the concrete files, commit, branch, tag, remote, and commands relevant to that approval. One confirmation can cover the listed commit, branch push, and tag creation/push; do not request separate confirmations for actions already approved. Seek new approval when the release target or scope changes.
- Pushing the tag triggers `.github/workflows/release.yml`, which creates and publishes the GitHub Release. Do not also create a Release manually unless the user separately requests recovery from a documented workflow failure.
- If tag creation succeeds but push fails, report the remaining local tag and inspect local and remote state. Retry within existing authorization only when it cannot duplicate or overwrite release artifacts. Deleting or replacing tags, force-pushing, or manually dispatching a workflow requires separate authorization.

## Handoff

After a successful tag push, stop active release work. Report the branch, commit, version, tag, remote, static checks, expected artifact matrix, and the risk-based manual checklist derived from the release delta. Tell the user to wait for GitHub Actions and then download the Release artifacts themselves. Their manual verification must cover each applicable artifact and architecture, baseline installation, startup and upgrade behavior, plus every changed high-risk path identified during preflight. Do not claim the application release works merely because the tag was pushed or the workflow completed.
