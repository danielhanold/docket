---
id: 267
slug: correct-the-stale-field-quote-handling-claim-in-script-contr
title: 'Correct the stale field() quote-handling claim in script contracts'
status: proposed
priority: medium
type: docs
created: 2026-08-08
updated: 2026-08-08
depends_on: []
related: []
discovered_from: [244]
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

**Trigger** — surfaced during #0244's close-out harvest (PR #184), while migrating optional-key `field()` reads to the anchored tier. The build deliberately left it alone as out-of-scope.

**Opportunity** — `scripts/render-learnings-index.md` still states "`field()` returns the raw scalar with surrounding quotes intact". That has been wrong since change 0138: `field()` strips a matched quote pair, and `field_raw()` is the accessor that preserves them. It sits in the paragraph immediately after one #0244 rewrote, so a reader trusting the contract file next to the corrected text gets the opposite of the truth.

**Independent value** — the contract file is the authoritative spec for the script; the correction stands with #0244 reverted. Worth sweeping the other `scripts/*.md` contracts for the same stale claim while the context is loaded, since 0138 changed the behavior repo-wide.

**Boundary** — documentation only: correct the `field()`/`field_raw()` quote-handling sentence wherever it appears in `scripts/*.md`. No code, no accessor behavior, no new guard.

**Reason for deferral** — #0244's branch scope was the read-shape selection rule and its census guard; `hook` is not a read that change migrated, so editing this paragraph would have been scope creep on a branch already carrying 22 files.
