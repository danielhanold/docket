---
id: 468
slug: 'rename-colliding-docket-terms-and-retire-obsolete-glossary-e'
title: 'Rename colliding docket terms and retire obsolete glossary entries'
status: 'proposed'
priority: 'medium'
type: 'refactor'
created: '2026-09-28'
updated: '2026-09-28'
depends_on: []
stacked_on:
related: [402, 467]
discovered_from: []
adrs: []
spec:
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
<!-- docket:artifacts:end -->

## Why

A review of `docs/reference/glossary.md` (2026-09-28) found several docket terms that collide with each other or with unrelated concepts, so the same word means different things depending on the page. The worst is "gate": it names about ten different things, and the run gate is not a test checkpoint at all but bookkeeping for a launched run. "Merge gate" names two different stops. "Fence" names two unrelated mechanisms (the glossary itself has to warn about this). "Terminal" clashes with the shell terminal, and `--version` on a change clashes with the `docket version` command. These collisions make the docs and run logs harder to read for humans and agents alike. The glossary also still carries retired or blocked terms that take up space and suggest features that no longer exist.

## What changes

Rename the colliding terms (proposed names are hypotheses to settle at grooming):

| Current | Problem | Suggested |
|---|---|---|
| **run epoch** (`--run-epoch`) | "Epoch" suggests time, but it's just the run's id | **run id** (`--run-id`) |
| **Run gate** / `run.gate-before` / `run.gate-verdict` / `gate-armed` | Not a test gate; it's bookkeeping for a launched run. "Gate" already names ~10 other things | **run tracker**; `run.start`, `run.verdict`, `run-started` |
| **Human merge gate** vs **merge gate** (alias of finalize gate) | The same phrase names two different stops | Human merge gate → **PR handoff**. Finalize gate → **pre-merge retest**. Drop the "merge gate" and "rebase-retest gate" aliases |
| **Suite gate / test gate / build gate** | Three names, two concepts | Keep **build gate** and **suite run**; drop "test gate" |
| **Fence** (coordination key) vs **epoch fence** | Two unrelated mechanisms share one word | Coordination fence → **shared-setting guard**. Epoch fence → **cancel lock** |
| **Terminal** record / sweep / publish | Clashes with the shell terminal | **archived record**, **merged-PR sweep**; terminal statuses → **final statuses** |
| **Change version** (`--version <blob>`) | Reads like a semantic version; clashes with `docket version` | **revision** (`--revision`); entity version → **record revision** |
| **Arm** (run gate) vs **re-arm** (auto-groom) | One verb for two unrelated mechanisms | Arm → **start** (see run tracker). Re-arm → **re-enable auto-groom** |
| **Build profile** / **review rung** / **dispatch tier** | Three words for "which strength of worker" | **build tier** and **review tier** (dispatch tiers are renamed in the companion opaque-names change) |

Retire rather than rename: move **Runner / delegation**, **runner shim / `runners` block**, **`runtime.bash`**, and **terminal publish** to an "Obsolete terms" appendix, and merge **auto-groomable / autonomous-eligible** into one term.

Where a renamed term is also a wire token (operation id, CLI flag, closed-vocabulary token, verdict line), keep the old spelling as an accepted alias for a deprecation window so skills, agents, and scripts that match on it keep working. Update the glossary, guide, concept pages, skills, agent wrappers, and CLAUDE.md/AGENTS.md prose consistently.

## Out of scope

- The opaque-name renames (admission slot, owner generation, conjuncts, dummy mode, disposition, etc.); those are tracked in the companion change.
- Rewriting frozen build records, archived changes, specs, or Accepted ADRs; point-in-time records keep the terms that were true when written.
- Removing the old wire-token spellings; alias removal is a later change after the deprecation window.
