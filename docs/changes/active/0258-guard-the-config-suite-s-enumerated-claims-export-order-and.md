---
id: 258
slug: guard-the-config-suite-s-enumerated-claims-export-order-and
title: 'Guard the config-suite''s enumerated claims: export order and rung pairs'
status: proposed
priority: medium
type: chore
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [123]
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

Consolidates #0123 and #0125 (2026-08-07 triage): the same meta-question — an enumerated claim in the config-suite's contract surface is prose-only; guard the claim or delete it — asked about two adjacent claims. One posture ruling, two applications.

Verified 2026-08-07:

- **Export-list order unguarded (#0123).** The fenced export list at `scripts/docket-config.md:344-374` (32/33 entries) is pinned only by per-key *presence* greps (`test_docket_config.sh:1248,1429,1581,1954`) and two runtime-only adjacency clusters (`:1643-1650`, `:1943`). No test reads the fence block and compares its **sequence** to `--export --format plain` output — a doc-side reorder stays green while R7 pins runtime order for a few pairs.
- **Rung-pair completeness prose-only (#0125).** Section S pins all six ordered rung pairs (s4–s9), but the "six pairs" claim lives in a header comment (`test_docket_config.sh:1707-1716`); no `rung_count`-style derivation exists, so a fourth config layer silently leaves six cells unpinned. The blocker named in the stub has resolved: 0114 landed ADR-0054 ("convert, do not close"), so source-shape anchors are a live option.

House bias, from ADR-0054 and the correspondence-guard learnings family: **guard the claim** rather than delete it — but each leg may independently conclude the claim should be re-specified (e.g. the doc list declared unordered) if guarding costs more than the claim is worth.

## What changes

- A machine check that the contract doc's fenced export list matches the resolver's emission (membership AND order, or membership with the doc explicitly re-specified as unordered).
- Derive the rung-pair count/enumeration from the resolver's layer set (or an enumerated corpus with a completeness anchor), replacing the prose-only "six pairs" claim.
- Both guards mutation-proved (reorder/removal must redden).

## Out of scope

- Adding config layers or keys.
- The population-floor/sharding rework of the same file — owned by the run-tests budget-regime change; coordinate at build time (same test file).
