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
related: [252]
discovered_from: [171, 233]
adrs: []
spec:
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
<!-- docket:artifacts:end -->

## Why

Consolidates #0171 and #0233 (2026-08-07 triage): the same prose-anchored-guard idiom, attacked from two sides — reflow fragility and catastrophic backtracking. One house pattern answers both.

Verified 2026-08-07:

- **Reflow-fragile line-scoped anchors (#0171).** 63 line-scoped `[^.]{0,N}` prose guards remain: `tests/test_docket_build.sh` (24), `tests/test_docket_review.sh` (18), `tests/test_gate_execution_posture.sh` (20), `tests/test_finalize_disposition.sh` (1). A cosmetic reflow of the guarded skill prose false-reddens them. A de-facto answer already exists but is triplicated, not shared: `flatten(){ tr -s '[:space:]' ' '; }` independently defined in `test_docket_review.sh:193`, `test_gate_execution_posture.sh:18`, `test_loop_continuation.sh:91`, plus a fourth variant `flatten_yaml()` (`test_docket_example_yml.sh:1222`).
- **Stacked-gap patterns hang (#0233), population grown.** Two stacked `[^x]{0,n}` gaps in one ERE backtrack catastrophically under ugrep — minutes of hang instead of a red assert. The class now matches **four** files (up from three when filed): `test_dispatch_capability.sh`, `test_docket_build.sh`, `test_docket_review.sh`, and `test_gate_execution_posture.sh` (new, shipped by 0223 — confirming the "grows with every proximity-shaped sentinel" claim). `test_grep_portability.sh` guards only `MAX_BOUND=255`; no stacked-gap leg exists.
- Known constraints the pattern must respect (documented in `test_docket_review.sh:519-531,:603,:651-656`): stacked `{0,n}` gaps backtrack; BSD grep caps repetition bounds at 255.

## What changes

- Hoist `flatten()` into `tests/lib/` as the house helper; state the pattern rule: flatten the haystack, single bounded gap, bounds < 255, no stacked `{0,n}` gaps.
- Convert the 63 line-scoped guards (and the 4 flatten copies) to the house pattern, mutation-proving each conversion (deletion of the guarded prose must still redden).
- Add the stacked-gap leg to `test_grep_portability.sh` — a static source guard over `tests/*.sh` detecting two `[^…]{0,n}` gaps in one pattern; the hang-vs-fail proof needs a `timeout`-shaped assert.

## Out of scope

- The SIGPIPE producer-pipe sweep (#0172 — separate change, same hygiene family).
- Guards over non-prose (code-shaped) anchors.
