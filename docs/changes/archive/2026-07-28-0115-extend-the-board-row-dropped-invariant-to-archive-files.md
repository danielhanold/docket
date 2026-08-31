---
id: 115
slug: extend-the-board-row-dropped-invariant-to-archive-files
title: Extend the board-row-dropped invariant to archive/ files
status: done
priority: medium
created: 2026-07-20
updated: 2026-07-28
depends_on: []
related: []
discovered_from: [104]
adrs: []
spec: docs/superpowers/specs/2026-07-20-archive-side-row-dropped-invariant-design.md
plan: docs/superpowers/plans/2026-07-27-archive-side-row-dropped-invariant.md
results:
trivial: false
auto_groomable: true
branch: feat/extend-the-board-row-dropped-invariant-to-archive-files
claimed_at: 
pr: 128
blocked_by:
reconciled: true
type: fix
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-20-archive-side-row-dropped-invariant-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-20-archive-side-row-dropped-invariant-design.md) |
| Plan | [2026-07-27-archive-side-row-dropped-invariant.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-07-27-archive-side-row-dropped-invariant.md) |
<!-- docket:artifacts:end -->

## Why

Change 0104 added `board-row-dropped`, a computed invariant catching an `active/` change file that
`render-board.sh` counts in the board's total but renders in no section. The check is deliberately
bounded to `active/`. The **symmetric archive-side violation is real and currently undetected**, and
0104's whole-branch review reproduced it:

An `archive/` file carrying a **non-terminal** status (e.g. `implemented`) is counted in `total` and
rendered nowhere. The archive block is gated on `ndone + nkilled > 0` and its summary count comes
from `ARC_COUNT`, which such a file never joins. Rendered against one healthy active change, the
board reads `**2 changes**` above a single row, with zero mentions of the dropped id — the exact
count-vs-tables disagreement change 0104 exists to eliminate.

A second flavor: when a real `done` file coexists, the misfiled row *does* print, but under a
`<summary>` whose count excludes it. Count and tables still disagree.

Grooming read the renderer and found a **second, distinct archive-side drop the stub did not
describe**: `ARC_COUNT` is keyed on status alone, with no id filter, while the archive table skips any
file whose `int_field id` is empty. So a *terminal* archive file with no usable id **is** counted in
the summary and its row never renders — the summary promises a row that does not exist. That case is
not a directory/status mismatch at all, which is what settles the design below.

**This is reachable by the same interrupted operation as the active-side case.** `archive-change.sh`
performs its `git mv` (step 2) *before* the status flip (step 3) and the commit (step 4), and each
of those can `die`. A failure in between leaves the file moved but not re-statused — precisely this
state. It is the mirror image of the `sweep-failed <id> archive <reason>` path that motivated
0104's active-side terminal-status trigger.

## What changes

**Widen `board-row-dropped` to cover both directories with one generalized predicate. No new
check-id.** `renders_row` takes the directory and selects the status set the renderer actually
iterates for it — `DOCKET_STATUSES_ACTIVE` for `active/`, `DOCKET_STATUSES_TERMINAL` for `archive/` —
above a hoisted, shared "id must be a usable integer" clause. The population site drops its
active-only guard. Suppression needs no new code: `malformed-id` and the `field-domain` `status` arm
already run over `archive/` files and are both genuine archive drop causes, so the two unsuppressed
archive triggers end up being exactly the two nothing enumerates — a legal status in the wrong
directory, and a file with no `id:` field at all.

Also in scope: the `board-checks.md` contract edit replacing its "covers `active/` only" paragraph —
which currently documents this very gap as follow-up work.

**Dropped at reconcile:** the spec's T9 correspondence assert. Change 0116 landed on 2026-07-22 and
single-sourced the renderer's terminal vocabulary, so the `done|killed` literals T9 was written to
pin no longer exist. See the reconcile log.

Full derivation, truth table, test plan, and the mutation set are in the spec.

## Out of scope

- Making `archive-change.sh` atomic. Ordering its `git mv` after the status flip, or making the
  sequence transactional, is a separate concern from *detecting* the resulting state. Worth its own
  change if the failure proves common.
- The `active/`-side invariant, which 0104 already ships.
- Repairing any offending file. Like 0104, this makes the failure visible; it does not decide what
  the file's canonical location or status should be.

## Open questions

Both settled at grooming; see the spec for the derivation.

- ~~Does the `ARCHIVE_RECENT` window or the per-month digest collapse legitimately drop a well-formed
  row, producing false positives?~~ **No — by construction.** Collapse *redirects* a row: a collapsed
  `done` file is still in the summary count and still represented in the "Older done (collapsed)"
  table. The predicate is written against *accounting*, not against verbatim row emission, so collapse
  is invisible to it. A regression test pins this so the cheaper row-emission formulation cannot creep
  back in.
- ~~Fold into `board-row-dropped` as a widened scope, or a new check-id?~~ **Widened scope.** The
  invariant is singular — one `total`, one set of tables — so splitting by directory yields two
  half-invariants and duplicates the suppression machinery; directionality lives in the message
  instead. It also adds no id to the two check-id enumerations change 0111 is concurrently hardening.
  The stub's "wrong directory for its status" framing is rejected as the *trigger*: the no-usable-id
  archive case is in the *correct* directory for its status, so that predicate would have missed it —
  the same blind-spot shape ADR-0050 was written about. It survives as the message's remedy hint.

## Reconcile log

### 2026-07-27 — reconciled at claim (docket-implement-next)

Re-read against `origin/main` @ `0da1c0aa`, the spec, and changes 0104 / 0111 / 0116 / 0127. The
design's decision, predicate, suppression analysis, and message text all survive unchanged. Four
adjustments:

1. **Change 0116 has landed** (archived 2026-07-22), and the spec anticipated exactly this: "If 0116
   has landed by build time, re-read `render-board.sh` and correct the comment rather than shipping
   it stale." It has, so the comment ships to the new reality. `render-board.sh` now reads
   `DOCKET_STATUSES_TERMINAL` directly at the archive block gate and summary count, and calls
   `docket_status_is_terminal` at the count-line and mermaid arms. **Zero** hard-coded `done|killed`
   set literals remain. The spec's central caveat — that the archive arm is a mirror "by convention,
   not by construction," with `DOCKET_STATUSES_TERMINAL` having no renderer readers — is therefore
   **obsolete**, and the `renders_row` comment must NOT ship it. Both arms are now backed the same
   way; the asymmetry the spec spends a section on is gone.
2. **T9 is dropped, not deferred.** T9 asked for a correspondence assert that tokenizes the
   renderer's `done|killed` `case` arms and asserts set-equality with `DOCKET_STATUSES_TERMINAL`.
   Those arms no longer exist — 0116 replaced them with reads of that very array, which is the
   guarantee T9 was a proxy for, delivered structurally rather than by test. Writing T9 now would
   mean asserting against literals that are gone. The two surviving `"done"` comparisons in the
   renderer (the mermaid done-node filter and the `ARCHIVE_RECENT` collapse partition) are
   single-status semantics — `killed` never collapses — not restatements of the terminal set, so
   neither is a correspondence target.
3. **The widen is smaller than drafted.** 0104's `renders_row` no longer loops an array inline; it
   delegates to the 0116 helper `docket_status_is_active`. The archive arm is therefore a
   `docket_status_is_terminal` branch, not new plumbing. Every line number in the spec is stale
   (0111, 0116, and 0127 all edited these files) — anchors get re-derived against the tip at build
   time rather than copied from the spec.
4. **Assumption 8 re-swept at build time, as the spec instructs.** This repo's `archive/` is clean:
   111 files, every one terminal-status, every one carrying a valid integer id. Nothing goes red on
   landing, and `--strict` gains no pre-existing failure.

Still-valid checks: assumption 6's guard holds — 0116 kept `DOCKET_STATUSES_ACTIVE` and
`DOCKET_STATUSES_TERMINAL` as distinct arrays and did not route the renderer to a third list, which
is what the two things it "must not do" forbade. 0111's check-id hardening now pins the enumerations
to `BOARD_CHECK_IDS` in the shared lib; adding no check-id keeps this change clear of it entirely.
0127 added a `type` arm to `field-domain` that does not mark `EXPLAINED` — consistent with the
spec's suppression analysis (only `status` suppresses), so no conflict.

Scope, dependencies (none), and the out-of-scope list are unchanged. `## Assumptions` item 9(ii)
(the `dir_kind` glob anchoring) is taken in passing, since the build edits that exact line; 9(i)
(`DROPPED`/`EXPLAINED` keyed by `cid` rather than path) stays out of scope and is captured as
follow-up.
