---
id: 468
slug: 'rename-colliding-docket-terms-and-retire-obsolete-glossary-e'
title: 'Rename colliding docket terms and retire obsolete glossary entries'
status: 'proposed'
priority: 'medium'
type: 'refactor'
created: '2026-09-28'
updated: '2026-09-29'
depends_on: []
stacked_on:
related: [402, 467, 469, 471, 472, 473, 474]
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-09-29-rename-colliding-docket-terms-and-retire-obsolete-glossary-e-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-29-rename-colliding-docket-terms-and-retire-obsolete-glossary-e-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-29-rename-colliding-docket-terms-and-retire-obsolete-glossary-e-design.md) |
<!-- docket:artifacts:end -->

## Why

A review of `docs/reference/glossary.md` (2026-09-28) found docket terms that collide, with each other or with unrelated concepts, so the same word means different things depending on the page or run log. "Gate" names about ten things, and the run gate is not a checkpoint at all. "Merge gate" names two different stops. "Fence" names two unrelated mechanisms. "Terminal" clashes with the shell terminal. `--version` on a change clashes with `docket version`. "Arm" and "re-arm" are unrelated. Worker strength is a profile, a rung, or a tier depending on the page. The glossary also still carries retired features.

Many of these names are wire tokens with hundreds of code sites (e.g. `run-epoch`: 594 Go occurrences in 66 files) and no alias mechanism, so one PR for the full rename would be too large. This change is the umbrella: it settles every name once, and four family changes carry out the wire renames.

## What changes

- Record an ADR, "Collision-free docket vocabulary": the naming rules (hard cut with no aliases; persisted names unchanged; "epoch" split by meaning into run / run id / run record; "gate" reserved for suite checkpoints; "final" for change-lifecycle end states; "fence" for the run fence only; config keys and agent names never renamed) and the full 66-row rename table, each row assigned to its owning change.
- Apply the prose-only renames (spec rows 60–65) across maintained docs, skills, agents, embedded copies, and CLAUDE.md/AGENTS.md: shared-setting guard, PR handoff, dropping the "merge gate" / "rebase-retest gate" / "test gate" aliases, final status / archived record / merged-PR sweep, and folding autonomous-eligible into auto-groomable.
- Move the four retired features (runner delegation, runner shim, `runtime.bash`, terminal publish) to an "Obsolete terms" section of the glossary.
- The wire renames are carried by the family changes 0471 (run tracker), 0472 (revision), 0473 (tiers), and 0474 (groom and lifecycle codes), each depending on this change.

## Out of scope

- The wire-token renames themselves (spec rows 1–59): owned by 0471–0474.
- The opaque-name renames in change 0469 (which must drop the rows 0468 took over: gate key, dispatch context, dispatch tiers).
- Any alias, deprecation-window, or dual-spelling machinery.
- A human-readable old→new mapping in the glossary; the mapping lives in the ADR and, once a family lands, in a code-level retired-vocabulary table.
- Editing point-in-time records (archived changes, results, specs, plans, Accepted ADRs).
