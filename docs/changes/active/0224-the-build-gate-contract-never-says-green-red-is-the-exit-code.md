---
id: 224
slug: the-build-gate-contract-never-says-green-red-is-the-exit-code
title: The build gate contract never says green/red is the exit code, so an output-shape match passes as a gate
status: proposed
priority: high
type: docs
created: 2026-08-06
updated: 2026-08-06
depends_on: []
related: [190, 223, 227]
discovered_from: [203]
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

`skills/docket-build/SKILL.md` § *The build gate* defines what green and red **mean** — green mints
the build-evidence record, red enters the repair path — but never says what **determines** which one
a run is. In practice that is the test's exit code, and nothing in the contract says so.

On the change 0203 run (2026-08-06) the implementer's first gate attempt keyed pass/fail on a
`tail -1` string match against `"PASS"` instead of the exit code. It reported
`### RED rc=0` for nearly every test file in the suite — a gate that was simultaneously wrong in
both directions and visibly self-contradictory (`RED` next to `rc=0`). The agent caught it itself
and rekeyed on `rc`, but only because the output was absurd enough to notice.

The failure mode that matters is the quiet one. A shape-matching gate that happens to agree with the
exit code passes review and mints a **valid-looking build-evidence record** certifying a branch
nobody actually verified. Because the record is what lets the review step and finalize skip their own
suite runs, a false green propagates: `docket-implement-next` Step 6 validates the record's presence
and `head_sha`, not the reasoning that produced it.

This is squarely the repo's own house rule from `AGENTS.md` — key a guard on **shape, never an
enumerated list of spellings** — turned on the gate itself, and the reason unguarded prose is
treated as decoration.

## What changes

- State **exit-code keying** normatively in `skills/docket-build/SKILL.md` § *The build gate*: a
  run is green if and only if the resolved suite command exits zero; output text is diagnostic, never
  the verdict.
- Add a **guard test** that fails if the contract stops saying it — the same treatment docket's other
  prose rules get. Per `AGENTS.md`, mutation-test the guard (strip the clause, watch it redden) or it
  is decoration.
- Confirm the rule reads correctly for a suite run as a **loop over per-file commands** (the shape
  this repo actually uses), not only for a single aggregate command — the per-file loop is where the
  0203 mistake became possible.

## Out of scope

- The execution posture / timeout problem — that is change 0223.
- Suite runtime — that is change 0227 (supersedes the killed 0225).
- Changing what green and red *do* (evidence record, repair ladder); only what decides them.

## Open questions

- Does the guard belong in an existing contract-prose test or a new one?
- Should the rule also bind the **repair** path's re-run and `finalize`'s post-rebase run, or is the
  build gate the only site?
- Is there a cheap assertion that catches a shape-matching gate at runtime, or is contract prose plus
  a docs guard the whole of it?

## Reconcile log
