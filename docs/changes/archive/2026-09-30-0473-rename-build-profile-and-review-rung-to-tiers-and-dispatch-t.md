---
id: 473
slug: 'rename-build-profile-and-review-rung-to-tiers-and-dispatch-t'
title: 'Rename build profile and review rung to tiers, and dispatch tiers to dispatch fallbacks'
status: 'done'
priority: 'medium'
type: 'refactor'
created: '2026-09-29'
updated: '2026-09-30'
depends_on: [468]
stacked_on:
related: [468, 469, 474]
discovered_from: []
adrs: [15, 86, 129]
spec: 'docs/superpowers/specs/2026-09-30-rename-build-profile-and-review-rung-to-tiers-and-dispatch-t-design.md'
plan: 'docs/superpowers/plans/2026-09-30-0473-rename-build-profile-and-review-rung-to-tiers-and-dispatch-t.md'
results: 'docs/results/2026-09-30-rename-build-profile-and-review-rung-to-tiers-and-dispatch-t-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/rename-build-profile-and-review-rung-to-tiers-and-dispatch-t'
pr: 'https://github.com/danielhanold/docket/pull/355'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-30-rename-build-profile-and-review-rung-to-tiers-and-dispatch-t-design.md](../../superpowers/specs/2026-09-30-rename-build-profile-and-review-rung-to-tiers-and-dispatch-t-design.md) |
| Plan | [2026-09-30-0473-rename-build-profile-and-review-rung-to-tiers-and-dispatch-t.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-09-30-0473-rename-build-profile-and-review-rung-to-tiers-and-dispatch-t.md) |
| Results | [2026-09-30-rename-build-profile-and-review-rung-to-tiers-and-dispatch-t-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-09-30-rename-build-profile-and-review-rung-to-tiers-and-dispatch-t-results.md) |
| ADRs | [ADR-0015](../../adrs/0015-harness-portable-agent-config.md), [ADR-0086](../../adrs/0086-in-context-gating-dispatch-carved-out-of-the-tier-taxonomy.md), [ADR-0129](../../adrs/0129-collision-free-docket-vocabulary.md) |
<!-- docket:artifacts:end -->

## Why

ADR-0129 (change 0468) settled a collision-free docket vocabulary. The strength of a worker is called a profile, a rung or a tier depending on the page, and what a workflow does when it cannot dispatch an agent is named by meaningless letters (Tier A / B / C) plus "the carve-out". This family applies ADR-0129 rows 46, 46a, 47, 47a and 48–52.

Grooming traced the code and corrected ADR-0129 in place (with the human's authorization): row 46a records three skill-to-skill line formats that carry the old words, row 47a stops calling finding severity a "tier", and row 51 now also retires the phrase "authorized-or-halt". There are no wire tokens and no Go identifiers to rename.

## What changes

Apply the spec's rename map across maintained source, in one PR:

- **build profile → build tier; review rung → review tier.** The tier names (economy / standard / premium / max, lean / standard / deep) and agent names are unchanged.
- **Skill labels hard-cut:** the plan override line `**Build profile:**` → `**Build tier:**`, the worker return line `PROFILE:` → `TIER:`, and the dispatch prompt labels `Profile:` / `Rung:` → `Tier:`.
- **Dispatch tiers A / B / C and the carve-out → dispatch fallbacks** `inline` / `abstain` / `auto-or-halt` / `no-fallback`; "authorized-or-halt" is retired in favour of `auto-or-halt`. The convention keeps ADR-0086's layout (three-row table, `no-fallback` paragraph after it).
- **Collisions:** severity becomes "severity-ranked" / "severity levels"; agent-layer.md's "no tier layer" becomes "no model-alias layer".
- Skills, agents, cursor dispatch rules, `harness-defaults.yml` comments, `.docket.example.yml`, docs and glossary entries, plus four Go test strings; the concept page is renamed to `docs/concepts/build-tiers-and-gate.md`. Embedded copies and harness goldens are regenerated.
- Word-neutral edits, no budget ceiling raised. Verified by the full suite and a closing whole-repo grep with a fixed list of frozen-path exclusions.
- **Lands after a drain:** no change mid-build or holding a committed-but-unbuilt plan, because an old `**Build profile:**` override would no longer be read.

## Out of scope

- Any alias, dual spelling or refusal text for the old words or labels: they are hard-cut (ADR-0129 Decision 2).
- Renaming agent names, tier names, config keys or frontmatter fields (ADR-0129 Decision 9).
- Editing point-in-time records: archived changes, results, specs, merged plans, and Accepted ADRs (including ADR-0015 and ADR-0086).
- A new guard or retired-vocabulary seal for these terms.
- Moving `no-fallback` into the convention's table (ADR-0086 rejected a fourth row).
- Rows owned by the other ADR-0129 families or by change 0469.

## Reconcile log

### 2026-09-30

Reconciled against origin/main c38ca3bed, the same commit grooming traced (2026-09-30); no main commits since, so the spec site list and budgets stand. Dependency 0468 is done. No other change is in-progress or implemented, so the drain precondition holds at claim time; merge must still wait for a drain. Scope unchanged.

