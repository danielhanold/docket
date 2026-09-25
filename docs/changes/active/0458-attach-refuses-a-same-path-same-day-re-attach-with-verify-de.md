---
id: 458
slug: 'attach-refuses-a-same-path-same-day-re-attach-with-verify-de'
title: 'Attach refuses a same-path same-day re-attach with verify-delta invalid-state'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-09-25'
updated: '2026-09-25'
depends_on: []
stacked_on:
related: [315, 335, 450]
discovered_from: [450]
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

Re-attaching the same artifact path to a change on the same day is refused. `change.attach-results` (and `change.attach-plan`) returns `invalid-state` at stage `verify-delta` with "a declared path is not an actual change". This is the normal path when a results file is updated after review and re-attached at the new head, which is exactly what happened on change 0450's implement-next run on 2026-09-25.

The cause is in `changeAttachOp.Plan` (`internal/app/change_attach.go`). It always declares the change record as a replace. The record stores only the artifact path, not its contents, so re-attaching the same path on the same day re-renders byte-identical bytes: `results:`/`plan:` unchanged, `updated:` the same date, `## Artifacts` block unchanged. The transaction engine's two-way delta guard (`verifyActualDelta`, `internal/repository/transaction/commitverify.go`) then rejects a declared path that did not actually change. Idempotency keys on (id, path, blob-at-commit), so an edited file is a new request, not a replay, and doesn't short-circuit first.

The bug dates from change 0315 (`f4bbb76a`, 2026-08-17). Recent changes (0414, 0449, 0453) did not touch the record declaration. Change 0335 fixed the same class of bug for `BOARD.md` only (`includeBoard`'s declare-only-when-changed shape). The change record itself never got the same treatment. No test attaches the same path twice.

It is not rare. implement-next's results checkpoint rules say to attach on first creation and "reattach before completion" after later updates, and almost every run edits results after the review fix on the same day. Past implement-next agent transcripts show the identical refusal on these runs:

| Change | Run date |
|---|---|
| 0323 | 2026-09-16 |
| 0445 | 2026-09-24 |
| 0448 | 2026-09-24 |
| 0449 | 2026-09-24 |
| 0452 | 2026-09-24 |
| 0450 | 2026-09-25 |

It went unnoticed because a refusal writes no commit, so `origin/docket` shows one clean attach per change. Of 56 attach-results commits, only change 0431 has two, and its second attach succeeded only because it ran on the next UTC day, which moved `updated:` from `2026-09-16` to `2026-09-17`.

Each hit so far was harmless: the first attachment already set `results:` to the same path, and `change.mark-implemented` read the file at the tested head and passed. But every autonomous run meets a spurious `invalid-state` and has to reason its way past it, and a stricter caller could halt on it.

## What changes

- Attach declares the change record only when its re-rendered bytes differ from the committed source. When nothing changed, the plan is empty and the operation takes the engine's existing no-op path (`len(plan.Files) == 0` returns `DispositionNoOp`).
- Decide and document what a same-path re-attach reports to callers (`no-op` versus `applied`), so implement-next's checkpoint flow treats it as success.
- Add a regression test that attaches the same path twice on the same day, for both `attach-plan` and `attach-results`, with the inline board on. It should fail before the fix.
- Audit the other record-writing operations for the same unconditional-declaration shape, and fix or list any that can re-render a byte-identical record.

## Out of scope

- Changing the engine's two-way delta guard. It is correct: a plan must describe reality.
- Changing attach idempotency keying.
- The board declaration, already fixed by change 0335.
