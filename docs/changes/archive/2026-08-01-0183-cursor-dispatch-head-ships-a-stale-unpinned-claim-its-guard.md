---
id: 183
slug: cursor-dispatch-head-ships-a-stale-unpinned-claim-its-guard
title: Cursor dispatch head ships a stale unpinned claim; its guard retired itself
status: killed
priority: medium
type: fix
created: 2026-07-31
updated: 2026-08-01
depends_on: []
related: []
discovered_from: [169]
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

`cursor-rules/dispatch.head.md` still tells every Cursor repo that docket ships validated Cursor
model IDs "for the three build-profile workers only — Every other wrapper is generated **unpinned**
unless a config layer sets a model for it". Change 0168 completed the Cursor block: all twelve
wrappers carry a shipped pin. The claim is false, and this file is catted **verbatim** into every
consumer repo's generated `.cursor/rules/docket-dispatch.mdc`, so the false claim ships rather than
merely displaying.

The guard that should have caught it retired itself. `tests/test_cursor_dispatch_rule.sh` gates its
head asserts on `if [ "$n_cursor_pinned" -lt "$n_src" ]` — designed so that "a future change that
pins every cursor wrapper turns the `if` false and retires the guard, rather than leaving a stale
assert behind". Change 0168 made that `12 < 12`, so the arm stopped running and the prose it was
protecting drifted with nothing watching.

Change 0169 hit the identical pattern on the Codex side and fixed it by giving the premise guard an
**`else` arm** that asserts the post-change truth just as hard (see `tests/test_sync_agents_codex.sh`,
the `agentsmd:` assert pair). The same shape applies here: a guard that switches itself off must hand
off, not vanish.

Discovered during change 0169's planning and confirmed by its independent whole-branch review.
Deliberately left out of 0169's scope — Cursor support belongs to change 0168's lineage, and 0169's
`## Out of scope` names Cursor explicitly.

## What changes

- Correct the head prose in `cursor-rules/dispatch.head.md` to the post-0168 truth: all twelve
  Cursor wrappers carry a shipped pin, overridable per field from any config layer.
- Give `tests/test_cursor_dispatch_rule.sh`'s premise guard an `else` arm asserting the pinned-truth
  claim, mirroring the pattern change 0169 established, plus a population floor so an emptied cursor
  block cannot leave both arms vacuous.
- Sweep for the same class elsewhere: any `if <premise-still-true>` guard whose arm has already gone
  false and left prose unwatched.

## Out of scope

- Changing any shipped Cursor or Codex model/effort mapping.
- Revisiting ADR-0064's sidecar design.

## Open questions

- Is the self-retiring `if <premise>` guard shape worth a house rule (or a lint) rather than being
  fixed case by case? Two instances are now known; a third would argue for the general form.

## Why killed

Absorbed by change 0184 (PR #147), which fixed both halves of this stub in the same diff: `cursor-rules/dispatch.head.md`'s stale "three build-profile workers only … every other wrapper is generated unpinned" claim is corrected to the post-0168/0184 truth, and `tests/test_cursor_dispatch_rule.sh`'s self-retiring premise guard gained the `else` arm this stub specified — asserting the pinned-truth claim and raising the population floor 12 → 13.

The third bullet (sweep for other self-retiring guards) was discharged by inspection: scanning every `tests/*.sh` for conditional blocks containing asserts with no `else` arm yields two hits, both in `test_docket_status.sh` (lines 606, 2369). Neither is this class — both are environment-conditional (did the sandbox push succeed) and each is preceded by an unconditional positive-shape assert that cannot go vacuous.

The open question asked whether the shape warrants a house rule or lint, and set its own bar: "Two instances are now known; a third would argue for the general form." The sweep found no third, so the general form is not warranted and nothing remains to build.
