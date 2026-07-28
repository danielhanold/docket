---
id: 149
slug: make-the-prelude-guard-s-exemption-bound-proportional-and-cl
title: Make the prelude guard's exemption bound proportional, and close the partial-rename gap
status: proposed
priority: medium
type: chore
created: 2026-07-28
updated: 2026-07-28
depends_on: []
related: [123, 125, 147, 148, 151]
discovered_from: [126]
adrs: []
spec: docs/superpowers/specs/2026-07-28-prelude-guard-proportional-bound-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-28-prelude-guard-proportional-bound-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-28-prelude-guard-proportional-bound-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0126's correspondence guard in `tests/test_docket_config.sh` defends itself against a
degenerate key set with `assert '[ "$t_exempt" -le 5 ]'` — a fixed ceiling against a real value of
3 (`TOTALS sites=64 exempt=3 ok=61 viol=0`). The whole-branch review accepted it as non-blocking and
flagged two residual weaknesses at the merge gate.

**The bound is absolute, not proportional.** Two sites of headroom that do not grow as the file
does, so several legitimately-exempt fixtures landing together would trip it. The ageing failure
mode is a loud false red, not a silent pass.

**A suspected partial-rename gap — which testing refuted.** The review reasoned that renaming part
of the export key set would raise `exempt` only slightly, slip under 5, and leave those sites
silently unguarded, and filed that as the more interesting half. Measured: renaming one emitted key
reddens four ordinary asserts while `TOTALS` stays byte-identical, and renaming five aborts the run
under `set -u` before section (T) ever executes. A rename is already caught, twice over, by the
ordinary fixtures and by `set -u`. The hole is not there.

## What changes

Replace the absolute ceiling with a **proportional floor on `ok`** — the count of sites the guard
actually proved something about — extracting `t_ok` from the existing `TOTALS` line. This trades one
site of immediate slack for slack that scales with the file. Record the negative finding in the
comment that replaces the retired assert's, so the ceiling is not reinstated by someone re-deriving
the original worry.

Deliberately **do not** build a partial-rename detector. The reverse key-coverage pass was drafted
and refuted on evidence: wrong count under the guard's own read-shape, blind to the empty key set,
and in standing conflict with the assert deletions changes 0148/0151 propose.

Design and measurements settled in the linked spec.

## Out of scope

- The guard's clearing-window semantics and its mirror-vs-subset ruling, both settled in 0126.
- The `t_keycount` vacuity floor, the `t_sites` population floor, the independent grep cross-check,
  and the self-block bound — all untouched.
- Extending the `TOTALS` line: `t_viol`'s extractor is end-anchored, so appending a field would
  silently empty it.
- Anchoring the export key set against a second independent source — change 0123 owns that.
