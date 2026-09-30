---
name: plugin-release
description: Prepare and publish res-downloader plugins from independent GitHub repositories, or configure their shared GitHub Actions release workflow. Use for release readiness, release versioning, tags, Releases, and extension-store publication. Use plugin-dev for local identity/version edits, scaffolding, and repackaging; use app-release for host application releases.
---

# Plugin Release

Keep the source commit, Manifest version, ZIP, tag, and GitHub Release consistent. Local version edits and repackaging belong to `plugin-dev`; they do not imply a release.

## Source and tools

- Read the relevant sections of `docs/zh/development/plugins.md` and `docs/zh/development/extension-store.md`, reusing context already read in this task. Inspect the plugin Manifest, package inputs, release history, and caller workflow.
- For automation work, inspect `.github/workflows/plugin-release.yml`, `internal/plugin/templates/plugin-release.yml`, `cmd/pluginctl/`, and the Go authoring implementation. Prefer extending these Go tools over adding Python or duplicating package validation in shell.
- Treat current source and deployed workflows as authoritative. A local file does not prove the shared workflow is available on GitHub.

## Repository and authorization boundaries

- Resolve the canonical plugin directory and compare it with `git -C <directory> rev-parse --show-toplevel`. They must match before plugin Git mutations; never accidentally commit or publish the host repository. If the plugin is not an independent repository, obtain the intended remote and authorization before initialization or repository creation.
- For batch work, inventory each requested plugin's branch, remote, version, changes, and staged files. Commit separately by repository when requested; skip clean repositories rather than creating empty commits. Stop for unrelated or ambiguous staged changes.
- Preserve authorization already given in the conversation for the same targets and actions. Present concrete commits, tags, remotes, and commands before asking for missing approval; one explicit confirmation can cover the listed batch. Do not require another confirmation for each step already approved.
- A request to prepare or configure releases does not authorize commits, pushes, tags, or publication. Follow the project's explicit commit/push rules. Repository creation, remote changes, topics, or visibility changes need their own authorization unless already included in the approved task.
- Never move or overwrite an existing release tag. After a partial failure, inspect actual local and remote state before acting. Retry within existing authorization only when it cannot duplicate or overwrite release artifacts; obtain approval for changed targets or destructive recovery.

## Release preflight

1. Determine each intended branch, commit, version, and tag. Reuse an unpublished version unless a bump is requested or required. Follow the workflow's accepted format; the shared workflow accepts stable `vMAJOR.MINOR.PATCH` tags matching `plugin.json.version`.
2. Verify the remote repository identity, tag availability, and that the release commit is reachable from the expected remote branch. For store discovery, check that the repository is public, unarchived, not a fork, and has `res-downloader-ext`; report unmet conditions without silently changing repository settings.
3. Check publisher metadata and reserved IDs, package inputs, and scope of changes since the previous release. Preserve user-specified IDs; do not infer official status merely from a naming convention.
4. Respect the session's verification limits. Under static-only rules, use lint, compilation, workflow lint, and ZIP checks. Fixture replay and tests execute code; do not run them unless explicitly permitted. Report skipped execution without treating it as failed static validation.
5. Use the host's Go tooling from its root:

   ```bash
   go run ./cmd/pluginctl lint <plugin-directory>
   go run ./cmd/pluginctl pack <plugin-directory>
   go run ./cmd/pluginctl verify-release <plugin-directory> <tag>
   ```

   Repack only when package inputs changed or the ZIP is stale. `verify-release` includes static plugin validation and compares the existing ZIP with a temporary package from the same packer. It does not execute plugin hooks. Do not regenerate the ZIP after committing/tagging; finalize all release inputs first.
6. Before tagging, require the plugin worktree to be clean and the final ZIP and runtime files committed. Show a concise release table with version, commit, tag, remote, relevant changes, and validation results. Reuse the user's confirmation of that exact release scope.

## Shared workflow configuration

- Keep publication logic in the host's reusable workflow and a small caller in each plugin repository. New callers come from the Go `plugin create` scaffold; preserve existing custom workflows unless replacement is requested.
- Keep the scaffold template and existing callers consistent with the reusable workflow's inputs and permissions. Use the built-in `GITHUB_TOKEN`; do not add personal credentials to files.
- Use the intended long-lived host branch or a deliberately pinned revision. Account for an upcoming merge into the default branch instead of permanently binding callers to a temporary development branch. Keep the shared-tools checkout reference aligned with the caller's workflow reference.
- Check that shared workflow/tools are deployed before callers are used. Explain any required deployment order in the handoff, not as temporary migration instructions in public plugin documentation.
- Keep `.github/` excluded through the Go packer's rules. Check package contents rather than assuming a workflow-only or documentation-only change requires repacking.
- Validate workflow syntax and Go builds statically when required. Do not trigger a remote workflow just to test configuration without authorization. Use `docs-sync` for concise bilingual release instructions.

## Tag and publication

Use the tag alone for the annotation and Release title, without adding `Release `:

```bash
git -C <plugin-directory> tag -a v1.0.1 <commit> -m "v1.0.1"
git -C <plugin-directory> push origin refs/tags/v1.0.1
```

This annotation is not a Git commit message. Commit messages still follow `CONTRIBUTING.md` and the actual changes.

- If the deployed caller publishes on tag push, the authorized push triggers publication. Do not also create a Release manually. Report whether the tag is pushed, the workflow is pending, or the Release is verified; do not conflate these states.
- Without a working release workflow, use an authorized GitHub CLI or API operation to create a non-draft, non-prerelease Release. Use an existing verified tag, its version as the title, and generated notes. If `gh` is unavailable, an authenticated API can perform the same approved action without exposing or persisting credentials.
- Keep `dist/plugin.zip` committed before tagging because the store reads it from that revision. Release attachments supplement that committed package; they do not replace it.
- Verify the published tag target and Release state. Store-index refresh is asynchronous.

## Handoff

Report release links, versions, commits/tags, checks, and any remaining work concisely. Distinguish local configuration from deployed automation and tag push from completed publication. List only relevant user-run checks for unverified behavior; do not repeat acceptance already supplied or copy a generic verification checklist into public documentation.
