---
id: 537
slug: 'delete-the-empty-agents-md-left-behind-when-a-dispatch-block'
title: 'Delete the empty AGENTS.md left behind when a dispatch block is retired'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-06'
updated: '2026-10-06'
depends_on: []
stacked_on:
related: [533]
discovered_from: [535]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

When docket retires a dispatch block from a worktree's AGENTS.md (for example after a repository goes private), the file can be left behind empty and untracked. Docket cannot safely delete it today, because nothing records whether docket created the file or the user did. Found during change 535's build.

## What changes

- Record, in the installed-state record for a managed block, whether docket created the file.
- When retiring a block leaves the file empty and docket created it, delete the file.
- A file the user created or edited is never deleted.

## Out of scope

- Moving or removing instruction files when a repository switches visibility (#533 owns that).
- Any change to what the private-mode triggers write.
