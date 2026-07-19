---
id: 99
slug: finalize-marker-clearing-rule-wording
title: Re-phrase the `## Finalize blocked` clearing rule around what it actually guards
status: proposed
priority: low
created: 2026-07-19
updated: 2026-07-19
depends_on: []
related: [87, 98]
discovered_from: [87]
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

The `## Finalize blocked` clearing rule as written has near-zero observable effect. A successful
finalize archives the change, which hides the board cell anyway — so the rule describes a cleanup
whose result nothing can see. The transitions that genuinely needed clearing were the three fixed
during change 0087's review (re-mark replaces rather than appends; an out-of-band human merge
still archives; the `drained`/`halted` boundary).

Stating the rule as **"the marker never survives into `done`"** would describe the property that is
actually load-bearing, rather than a procedural step that is mostly redundant with archiving.

Worth doing because the current wording invites a reader to implement redundant clearing logic, or
to conclude the rule is dead and drop it — and one of those two readings is wrong.

## What changes

Re-phrase the clearing rule in `skills/docket-finalize-change/SKILL.md` (and the convention entry
if it restates it) around the invariant rather than the step. Re-check the marker sentinels: one or
more may be anchored on the old phrasing, and a re-word that silently un-anchors a guard is the
failure mode change 0087 just spent a review catching.

**Wording only — no behavior change intended.** If the re-phrasing turns out to require a behavior
change to be true, that is a finding worth surfacing rather than absorbing.

## Out of scope

- Any change to when the marker is written or skipped.
- The stale-marker health check (change 0098).

## Open questions

- Does the convention's marker entry restate the rule, or only point at the skill? Only the source
  should carry the phrasing.
- Are any existing sentinels anchored on the current wording's specific literals?

## Reconcile log
