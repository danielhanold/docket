---
id: 253
slug: settle-and-enforce-the-prose-anchored-guard-house-pattern
title: 'Settle and enforce the prose-anchored guard house pattern'
status: proposed
priority: medium
type: chore
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: [252, 172, 260]
discovered_from: [171, 233]
adrs: []
spec: docs/superpowers/specs/2026-08-07-settle-and-enforce-the-prose-anchored-guard-house-pattern-design.md
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
| Spec | [2026-08-07-settle-and-enforce-the-prose-anchored-guard-house-pattern-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-settle-and-enforce-the-prose-anchored-guard-house-pattern-design.md) |
<!-- docket:artifacts:end -->

## Why

Consolidates #0171 and #0233 (2026-08-07 triage): the same prose-anchored-guard idiom, attacked from two sides — reflow fragility and catastrophic backtracking. One house pattern answers both.

Verified 2026-08-07:

- **Reflow-fragile line-scoped anchors (#0171).** 63 line-scoped `[^.]{0,N}` prose guards remain: `tests/test_docket_build.sh` (24), `tests/test_docket_review.sh` (18), `tests/test_gate_execution_posture.sh` (20), `tests/test_finalize_disposition.sh` (1). A cosmetic reflow of the guarded skill prose false-reddens them. A de-facto answer already exists but is triplicated, not shared: `flatten(){ tr -s '[:space:]' ' '; }` independently defined in `test_docket_review.sh:193`, `test_gate_execution_posture.sh:18`, `test_loop_continuation.sh:91`, plus a fourth variant `flatten_yaml()` (`test_docket_example_yml.sh:1222`).
- **Stacked-gap patterns hang (#0233), population grown.** Two stacked `[^x]{0,n}` gaps in one ERE backtrack catastrophically under ugrep — minutes of hang instead of a red assert. The class now matches **four** files (up from three when filed): `test_dispatch_capability.sh`, `test_docket_build.sh`, `test_docket_review.sh`, and `test_gate_execution_posture.sh` (new, shipped by 0223 — confirming the "grows with every proximity-shaped sentinel" claim). `test_grep_portability.sh` guards only `MAX_BOUND=255`; no stacked-gap leg exists.
- Known constraints the pattern must respect (documented in `test_docket_review.sh:519-531,:603,:651-656`): stacked `{0,n}` gaps backtrack; BSD grep caps repetition bounds at 255.

## What changes

Settled by the linked design spec (groomed 2026-08-07, critic-gated):

- Hoist `flatten()` into a sourced `tests/lib/prose_guard.sh` (the three identical copies; `flatten_yaml` stays local as a documented-distinct contract) whose header states the house rule: flatten the haystack, at most one gap per alternation branch (mirrored `A[gap]B|B[gap]A` alternations are safe), bounds ≤ 255, negated-class gaps widened where tables could bridge, deliberate line-anchored structure guards stay with a why-comment.
- Convert the reflow-fragile line-scoped guards to the house pattern — site list re-derived at build time (the filed 63/4-file tally is a floor-check) — mutation-proving each conversion in both directions (deletion reddens; a rewrap stays green).
- Rewrite the sequential stacked-gap population (~50+ pattern-strings across ~a dozen test files, dot-gap and unbounded stacks included) to single-gap shapes, then add the stacked-gap leg to `test_grep_portability.sh`: a static per-pattern-string scan with paren-depth-aware top-level-`|` detection, visible `# stacked-gap-ok` exemptions, and five mutation controls including the grouped-`|` defeat shape. The hang-vs-fail proof runs watchdog-wrapped at build verification and is recorded in results, not committed as a runtime assert.

## Out of scope

- The SIGPIPE producer-pipe sweep (#0172 — separate change, same hygiene family; textual collision on `test_docket_build.sh`/`test_docket_review.sh`, hence `related:`).
- Guards over non-prose (code-shaped) anchors; loosening any guard's bite.
