# Project Collaboration Rules

## Communication and Execution

- Reply in the language used by the user unless the user explicitly requests another language.
- Perform static validation only. If a change requires runtime, browser, or manual verification, list the verification steps in the handoff and clearly state that the user must perform them.
- Before changing code or documentation, present the proposed changes and obtain explicit user confirmation, unless the user explicitly asks to proceed without confirmation.
- Read `docs/architecture.md` when project architecture context is needed.
- Read `docs/plugins.md` for plugin-related work.

## Git and Commits

- When the user explicitly requests a commit, stage and commit the changes related to the current task.
- Do not run `git push` unless the user explicitly requests it.
- Prefer a commit message supplied by the user. Otherwise, generate a concise message from the actual changes and follow `CONTRIBUTING.md` without requesting additional confirmation.
- After committing, report the commit hash, commit message, changed files, and statistics for user review.
- If the staging area contains clearly unrelated changes or changes with ambiguous scope, stop and ask the user before committing.

## Frontend Design

- Frontend implementations must be polished and visually appealing.
- Avoid stereotypical AI-style blue or purple gradients.
- Prefer Naive UI and Tailwind CSS for page layout and styling.

## Prohibited Content

- Never record passwords in `AGENTS.md`.
