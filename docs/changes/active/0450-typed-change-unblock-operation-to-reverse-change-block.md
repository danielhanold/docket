---
id: 450
slug: 'typed-change-unblock-operation-to-reverse-change-block'
title: 'Typed change.unblock operation to reverse change.block'
status: 'implemented'
priority: 'high'
type: 'chore'
created: '2026-09-24'
updated: '2026-09-25'
depends_on: []
stacked_on:
related: [444, 446]
discovered_from: [444]
adrs: [12]
spec: 'docs/superpowers/specs/2026-09-24-typed-change-unblock-operation-to-reverse-change-block-design.md'
plan: 'docs/superpowers/plans/2026-09-25-typed-change-unblock-operation-to-reverse-change-block.md'
results: 'docs/results/2026-09-25-typed-change-unblock-operation-to-reverse-change-block-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/typed-change-unblock-operation-to-reverse-change-block'
pr: 'https://github.com/danielhanold/docket/pull/334'
blocked_by:
reconciled: true
claimed_at: '2026-09-25T11:15:13Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-24-typed-change-unblock-operation-to-reverse-change-block-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-24-typed-change-unblock-operation-to-reverse-change-block-design.md) |
| Plan | [2026-09-25-typed-change-unblock-operation-to-reverse-change-block.md](https://github.com/danielhanold/docket/blob/chore/typed-change-unblock-operation-to-reverse-change-block/docs/superpowers/plans/2026-09-25-typed-change-unblock-operation-to-reverse-change-block.md) |
| Results | [2026-09-25-typed-change-unblock-operation-to-reverse-change-block-results.md](https://github.com/danielhanold/docket/blob/chore/typed-change-unblock-operation-to-reverse-change-block/docs/results/2026-09-25-typed-change-unblock-operation-to-reverse-change-block-results.md) |
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md) |
<!-- docket:artifacts:end -->

## Why

`change.block` moves a change to `blocked` and `change.defer` moves one to `deferred`, but neither has a typed inverse. When the blocker clears or a deferred change comes back, the only way out is a hand edit of the frontmatter plus a plain git commit on `docket`. That skips the transaction engine's version check and replay, and it leaves BOARD.md stale until a human runs `docket repository migrate`. The convention text still tells agents to do exactly that. Hit on change 444 on 2026-09-24: after its blocker, change 446, merged, 444 was hand-edited from `blocked` back to `in-progress` before `change.resume-halted` could run.

## What changes

- Add `change.unblock` (`docket change unblock`): `blocked` → `in-progress`, clearing `blocked_by`.
- Add `change.revive` (`docket change revive`): `deferred` → `proposed`.
- Both expose domain transitions that already exist (`domain.Unblock`, `domain.Revive`) through the existing lifecycle driver that `change.block` and `change.defer` use: exact-version pin, `updated:` refresh, artifact-block and board re-render in one metadata commit. A change in any other status is refused with nothing written.
- No pre-block status is recorded: `change.block` only accepts `in-progress`, so `in-progress` is always the right target. A halted-then-blocked change keeps its `## Run halted` section and resumes with `change.resume-halted` after unblock.
- Update the docket-convention lifecycle rules to name `change.unblock` and `change.revive` in place of the hand edit.

## Out of scope

Automatically unblocking when a dependency lands (a sweep or watcher) — `blocked_by` is free text, and machine-trackable dependencies already use `depends_on:`. Recovery for stack-kill descendants (`KillStackParent` is not wired to any operation, and its exits are re-scope, re-parent, or kill). The `finalize.block` / `finalize.clear-block` pair. Changing `change.block` or `domain.Revive` semantics, including clearing `branch:`/`claimed_at:` on revive.

## Reconcile log

### 2026-09-25

Reconciled against main 309b1e29 and docket da7d24a1. domain.Unblock and domain.Revive (internal/domain/actions.go) still exist unwired; executeChangeLifecycle in internal/app/change_lifecycle.go still drives only change.block/change.defer. Related 0444 and 0446 are done. Scope unchanged; spec stands as written.
