---
id: 450
slug: 'typed-change-unblock-operation-to-reverse-change-block'
title: 'Typed change.unblock operation to reverse change.block'
status: 'proposed'
priority: 'high'
type: 'chore'
created: '2026-09-24'
updated: '2026-09-24'
depends_on: []
stacked_on:
related: [444, 446]
discovered_from: []
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

`change.block` moves a change to `blocked` and records `blocked_by`, but nothing reverses it. When the blocker clears, the only way back is a hand edit of the frontmatter plus a plain git commit on `docket`. That skips the transaction engine's version check and replay, and it leaves BOARD.md stale until a human runs `docket repository migrate`. It also blocks the resume path: `run.gate-before --resume` and `change.resume-halted` both require `in-progress` and refuse a `blocked` change (`resume-unverified` and `not-halted`). Hit on change 444 on 2026-09-24, after its blocker, change 446, merged.

## What changes

- Add a `change.unblock` operation (`docket change unblock`) to the capability catalog. It is the inverse of `change.block`: it takes the change id and the exact record version, restores the status the change had before it was blocked, clears `blocked_by`, and commits through the transaction engine with a board re-render.
- Decide how the pre-block status is found: record it at block time or derive it from history. Refuse when the status can't be established rather than guessing.
- A change that was halted and then blocked must come back in a state `change.resume-halted` accepts, so the resume flow works without hand edits.
- Update the docket-convention and skill text that describes a blocked change so it names `change.unblock` as the way out.

## Out of scope

Automatically unblocking when a dependency lands (a sweep or watcher). Changing `change.block` semantics beyond anything needed to record the pre-block status. The `finalize.block` / `finalize.clear-block` pair.
