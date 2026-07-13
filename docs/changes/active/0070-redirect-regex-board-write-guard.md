---
id: 70
slug: redirect-regex-board-write-guard
title: Harden the BOARD.md write guard — REDIRECT_RE misses real redirect forms
status: proposed
priority: medium
created: 2026-07-13
updated: 2026-07-13
depends_on: []
related: [59, 69]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`tests/test_render_board.sh` guards a failure mode docket has already been bitten by: `render-board.sh` emits to STDOUT, so an orchestrator that forgets to redirect it into `BOARD.md` silently no-ops while every surface still reports success. The guard is `REDIRECT_RE`, a regex asserting that `docket-status.sh` actually redirects the renderer's output into the board file.

`REDIRECT_RE` requires whitespace on **both** sides of the `>` operator. That makes it blind to at least two forms a future edit could plausibly introduce:

- `>"$1/BOARD.md"` — no space between the operator and the target
- `>>` — append rather than truncate

This is not a regression. The regex is byte-identical to `origin/main` and has always had this shape. What changed is its load-bearingness: before change 0069 there were overlapping guards on the orchestrator's board write, and 0069 left this regex as the **only** one. A false negative here now means a silent no-op board write ships undetected — precisely the failure this test exists to prevent.

## What changes

Make the guard actually cover the redirect forms the orchestrator could take, without giving up the false-positive resistance the current shape was chosen for.

`REDIRECT_RE` carries a ~15-line design comment enumerating the false-positive classes its narrow shape deliberately excludes. That comment is the real content of this change: naively widening the regex trades a false negative for a false positive, and a guard that fires on non-writes is a guard someone eventually disables. Whatever mechanism ships must re-derive that reasoning rather than delete it.

Scope is the guard and its rationale — not the renderer, not the orchestrator, both of which behave correctly today.

## Out of scope

- Changing `render-board.sh`'s STDOUT contract. Emitting to STDOUT is the design (change 0059's surface gate depends on it); the guard exists precisely because that contract puts the burden on callers.
- Reworking `docket-status.sh`'s board pass. The orchestrator's current redirect is correct — this is about the test's ability to *notice* if a future one isn't.
- Broader test-suite hardening. One guard, one change.

## Open questions

- **Widen or replace?** Two candidate shapes: (a) widen `REDIRECT_RE` to cover no-space and append forms while preserving its documented exclusions, or (b) drop source-text matching entirely and assert on the observed filesystem effect — run the orchestrator against a fixture and check that `BOARD.md` was actually written. (b) is immune to the whole class of "the regex didn't match the syntax someone used," but it costs a real fixture run and may be harder to keep hermetic.
- Which false-positive classes in the existing design comment are still real, and which were artifacts of an older orchestrator shape? The comment must be re-derived either way; grooming should establish whether its constraints still bind.
- Is `>>` (append) actually a form worth guarding, or should the guard reject it outright as a bug? An append to `BOARD.md` is almost certainly wrong — the board is a full regeneration, not an accumulation.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
