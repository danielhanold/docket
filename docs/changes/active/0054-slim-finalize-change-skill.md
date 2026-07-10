---
id: 54
slug: slim-finalize-change-skill
title: Slim docket-finalize-change — rewire close-out to the shared reference
status: proposed
priority: medium
created: 2026-07-10
updated: 2026-07-10
depends_on: [53]
related: [53, 55]
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

`docket-finalize-change` is the largest operating skill (~234 lines / ~3,500 words). Change #0053
creates `docket-convention/references/terminal-close-out.md` as the single source of the shared
archive→re-render→publish→cleanup→board sequence; finalize still restates it (with the
"identical — must not diverge" warning) plus verbose gate/selection prose. Categorized
**high optimization potential / medium-high risk** in #0053's spec — the merge gate is docket's
highest-blast-radius path.

## What changes

- Rewire per-change steps 3–5 and the Terminal-publish section to point at
  `references/terminal-close-out.md` (finalize's posture: abort-and-report).
- Compress the rebase-retest gate and Selection prose without weakening them: the gate flow, the
  two-agent split, the sign-off rule, and the full abort-and-report set must survive in meaning.
- Apply the convention's Step-0 preamble compression (#0053 §3).
- Cut provenance narration per #0053's decision 2.

## Out of scope

- Any change to gate semantics, selection matrix behavior, sign-off, or close-out ordering.
- The other skills (#0053, #0055).

## Open questions

- Whether the gate flow itself warrants its own reference file or stays inline (it runs on every
  finalize, so inline is the default lean).

## Reconcile log
