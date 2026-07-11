---
id: 54
slug: slim-finalize-change-skill
title: Slim docket-finalize-change — rewire close-out to the shared reference
status: done
priority: medium
created: 2026-07-10
updated: 2026-07-11
depends_on: [53]
related: [53, 55]
adrs: [2]
spec: docs/superpowers/specs/2026-07-10-slim-finalize-change-skill-design.md
plan: docs/superpowers/plans/2026-07-11-slim-finalize-change-skill.md
results: docs/results/2026-07-11-slim-finalize-change-skill-results.md
trivial: false
auto_groomable:
branch: feat/slim-finalize-change-skill
pr: https://github.com/danielhanold/docket/pull/66
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-10-slim-finalize-change-skill-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-10-slim-finalize-change-skill-design.md) |
| Plan | [2026-07-11-slim-finalize-change-skill.md](https://github.com/danielhanold/docket/blob/feat/slim-finalize-change-skill/docs/superpowers/plans/2026-07-11-slim-finalize-change-skill.md) |
| Results | [2026-07-11-slim-finalize-change-skill-results.md](https://github.com/danielhanold/docket/blob/feat/slim-finalize-change-skill/docs/results/2026-07-11-slim-finalize-change-skill-results.md) |
| PR | [#66](https://github.com/danielhanold/docket/pull/66) |
| ADRs | [ADR-0002](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0002-docket-mode-default-and-bootstrap.md) |
<!-- docket:artifacts:end -->

## Why

`docket-finalize-change` is the largest operating skill (~234 lines / ~3,500 words). Change #0053
creates `docket-convention/references/terminal-close-out.md` as the single source of the shared
archive→re-render→publish→cleanup→board sequence; finalize still restates it (with the
"identical — must not diverge" warning) plus verbose gate/selection prose. Categorized
**high optimization potential / medium-high risk** in #0053's spec — the merge gate is docket's
highest-blast-radius path.

## What changes

Behavior-neutral restructure per the spec (target 234 → ≤ ~140 lines / ≤ ~2,200 words):

- Rewire per-change step 3 and the Terminal-publish section to loud blocking pointers at
  `references/terminal-close-out.md`; delete finalize's single-source ownership claims and the
  "identical — must not diverge" note (the reference is the single source now). Finalize keeps
  only its own facts: UTC merge date, `--results`, abort-and-report posture.
- The rebase-retest gate stays **inline**, compressed ~95 → ~65 lines (it runs on every
  finalize — LEARNINGS #20): the config block, 6-step flow, two-agent split, sign-off rule,
  full abort-and-report set, and PR-comment durable-reason rule survive in meaning.
- The *Harvest learnings* step stays finalize-owned and intact (cited by name as single source
  by the convention and docket-status; not part of #0053's reference).
- Append a dated `## Update` to ADR-0002 (its "terminal-publish single-sourced in finalize"
  clause goes stale); `adrs: [2]` re-publishes it at merge.
- Apply the convention's Step-0 preamble compression (#0053 §3); cut provenance narration per
  #0053's decision 2.
- Re-anchor the doc sentinels in the seven test files that grep finalize prose, preserving each
  assertion's intent.

## Out of scope

- Any change to gate semantics, selection matrix behavior, sign-off, or close-out ordering.
- The other skills (#0053, #0055).

## Reconcile log

- 2026-07-11 — Reconciled at claim (design-ahead obligations discharged against landed #53).
  Verified on `origin/main`: (a) `skills/docket-convention/references/terminal-close-out.md`
  exists with a `## Failure posture — per caller` table carrying a `docket-finalize-change`
  **abort-and-report** row (spec assumption holds); (b) the convention heading is exactly
  `### Step-0 preamble (every operating skill)` — the ~3-line citation must use it verbatim;
  (c) finalize is still **234 lines / 3529 words** (matches the spec's baseline; size targets
  ≤~140L/≤~2200w stand); (d) all seven sentinel test files present; (e) the `2.5 **Harvest
  learnings.**` heading is intact and byte-stable; (f) ADR-0002 already carries `## Update —
  2026-06-19 (change 0025)`, so this change appends a new dated `## Update` (not an edit).
  #0053 landed as the spec anticipated; related #55 still `proposed` (no overlap). Scope
  unchanged; no drops or additions.
