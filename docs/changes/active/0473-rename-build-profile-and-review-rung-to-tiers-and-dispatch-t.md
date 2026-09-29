---
id: 473
slug: 'rename-build-profile-and-review-rung-to-tiers-and-dispatch-t'
title: 'Rename build profile and review rung to tiers, and dispatch tiers to dispatch fallbacks'
status: 'proposed'
priority: 'medium'
type: 'refactor'
created: '2026-09-29'
updated: '2026-09-29'
depends_on: [468]
stacked_on:
related: [468, 469]
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

Change 0468 settled a collision-free docket vocabulary. Worker strength is called a profile, a rung, or a tier depending on the page, and the dispatch tiers A/B/C carry letters with no meaning. This family applies 0468's rename table rows 46–52.

## What changes

Apply 0468 spec rows 46–52, following the spec's "Family changes" obligations (this family has no wire tokens):

- build profile → build tier; review rung → review tier (tier names economy/standard/premium/max and lean/standard/deep unchanged).
- Dispatch tiers A/B/C and the carve-out → dispatch fallbacks `inline` / `abstain` / `auto-or-halt` / `no-fallback`.
- Go identifiers carrying "rung"/"profile", tests, skills (incl. docket-convention's dispatch-capability table), agents, embedded copies, docs, and the glossary entries for these rows.
- Verified by a whole-repo grep showing no maintained-source "rung", "build profile", or "Tier A/B/C" remains.

## Out of scope

- Any alias, deprecation-window, or dual-spelling support: old spellings are hard-cut (0468 Decision 2).
- Renaming persisted storage names, config keys, agent names, or frontmatter fields (0468 Decisions 3, 9).
- Editing point-in-time records (archived changes, results, specs, plans, Accepted ADRs).
- Rows owned by the other 0468 family changes or by change 0469.
