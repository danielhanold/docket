---
id: 115
slug: extend-the-board-row-dropped-invariant-to-archive-files
title: Extend the board-row-dropped invariant to archive/ files
status: proposed
priority: medium
created: 2026-07-20
updated: 2026-07-20
depends_on: []
related: []
discovered_from: [104]
adrs: []
spec: docs/superpowers/specs/2026-07-20-archive-side-row-dropped-invariant-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
type: fix
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-20-archive-side-row-dropped-invariant-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-20-archive-side-row-dropped-invariant-design.md) |
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

Also in scope: a correspondence assert pinning the renderer's hard-coded `done|killed` literals to
`DOCKET_STATUSES_TERMINAL`, and the `board-checks.md` contract edit replacing its "covers `active/`
only" paragraph — which currently documents this very gap as follow-up work.

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
