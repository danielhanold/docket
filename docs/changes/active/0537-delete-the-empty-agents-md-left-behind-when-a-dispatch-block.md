---
id: 537
slug: 'delete-the-empty-agents-md-left-behind-when-a-dispatch-block'
title: 'Delete the empty AGENTS.md left behind when a dispatch block is retired'
status: 'in-progress'
priority: 'low'
type: 'fix'
created: '2026-10-06'
updated: '2026-10-16'
depends_on: []
stacked_on:
related: [533, 555]
discovered_from: [535]
adrs: []
spec: 'docs/superpowers/specs/2026-10-16-delete-the-empty-agents-md-left-behind-when-a-dispatch-block-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/delete-the-empty-agents-md-left-behind-when-a-dispatch-block'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-10-16T17:20:16Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-16-delete-the-empty-agents-md-left-behind-when-a-dispatch-block-design.md](../../superpowers/specs/2026-10-16-delete-the-empty-agents-md-left-behind-when-a-dispatch-block-design.md) |
<!-- docket:artifacts:end -->

## Why

When docket removes one of its managed blocks from a file it created, it writes the now-empty file back instead of deleting it. In a repository that went private, install retires the dispatch block an earlier install wrote into the working-tree AGENTS.md and leaves a zero-byte, untracked AGENTS.md that `git add -A` would commit. The same leftover can follow a shared-repository harness drop or a `docket uninstall` of the Codex pointer. Found during change 535's build.

## What changes

- When removing docket's managed block leaves a file empty or whitespace only, install and uninstall delete the file instead of writing it back. This is the rule `repository set-visibility private --remove-shared-files` already applies.
- A file with any other content keeps it, and only the block goes. A block someone edited still refuses the run, as today.
- The ownership records do not change.
- `repository set-visibility private` no longer needs the private instructions file when the repository declares no harness, so an already-private repository whose file was deleted still reports nothing to do.

## Out of scope

- Empty files earlier runs already left behind. Docket can no longer prove they are its own, so delete them by hand.
- The CLAUDE.md link left dangling when AGENTS.md stops being shared (#555).
- Hook files emptied of docket's entries, and directories left empty.
- Any change to what the private-mode triggers write.
