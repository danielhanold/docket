---
id: 573
slug: 'retire-trivial-every-build-ready-change-has-a-spec'
title: 'Retire trivial: every build-ready change has a spec'
status: 'proposed'
priority: 'high'
type: 'refactor'
deliverable: 'pr'
created: '2026-10-19'
updated: '2026-10-19'
depends_on: []
stacked_on:
related: [566]
discovered_from: []
adrs: [6, 153]
spec: 'docs/superpowers/specs/2026-10-19-retire-trivial-every-build-ready-change-has-a-spec-design.md'
plan:
results:
trivial: false
auto_groomable:
hold:
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
| Spec | [2026-10-19-retire-trivial-every-build-ready-change-has-a-spec-design.md](../../superpowers/specs/2026-10-19-retire-trivial-every-build-ready-change-has-a-spec-design.md) |
| ADRs | [ADR-0006](../../adrs/0006-autonomous-grooming-bounds.md), [ADR-0153](../../adrs/0153-a-hold-keeps-a-groomed-change-out-of-autonomous-build-select.md) |
<!-- docket:artifacts:end -->

## Why

Grooming can end in a trivial verdict: no spec, `trivial: true`, and the groom's research and decisions squeezed into the change body as "the tightened body as the trivial rationale." Nothing makes those decisions or the rejected alternatives survive, and the body never reaches the PR or `main`. The exit also saves almost nothing: implement-next still reconciles, plans, builds, reviews, and writes results for a trivial change; only writing the design down is skipped. Every trivial verdict since about #325 came out of a groom, after the design conversation had already happened, and the human now always declines the suggestion because the research has nowhere reliable to go.

## What changes

Remove `trivial` everywhere so a change is build-ready only when it has a spec: the domain model and readiness rule, the `change.groom` trivial outcome, the board's trivial cell, `docket open`'s trivial reason, new records' `trivial:` line, `docket-new-change`'s trivial path, and auto-groom's trivial exit (auto-groom ends in spec or abstain). The interactive groom and capture skills tell the brainstorm its artifact is always a written spec, sized to the change. A new ADR supersedes ADR-0006. Skill, agent, and doc prose drop trivial outright, and test fixtures that used `trivial: true` as a shortcut get a spec instead. Archived records keep their `trivial: true` lines as inert data.

## Out of scope

Rewriting archived records or other repositories' records (pre-1.0.0; an unbuilt trivial change elsewhere drops back to needs-grooming). The broader trimming of superpowers brainstorming steps docket does not use (#566). Changing what a spec must contain, the plan-writer, or the reconcile pass.
