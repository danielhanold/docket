---
id: 159
slug: docket-status-skill-md-s-normal-outcomes-list-omits-the-heal
title: docket-status SKILL.md's normal-outcomes list omits the 'health checks failed <exit>' line
status: proposed
priority: medium
type: docs
created: 2026-07-28
updated: 2026-07-28
depends_on: []
related: []
discovered_from: [157]
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

Change 0144 added a new report line to the `docket-status` orchestrator: `health checks failed
<exit>`, emitted when the health pass's `board-checks.sh` invocation exits non-zero. The line is
documented where the script contract lives — `scripts/docket-status.md` has both the narrative
(around the health-pass section) and a row in the report-line table.

But `skills/docket-status/SKILL.md` carries its own enumeration of what counts as a *normal*
stdout outcome, and it was not updated:

> `board off`, `pass ok`, findings, `sweep-failed`, `sweep-skipped`, `board *-failed`, and
> `judgment` lines on stdout are all normal outcomes, not errors

`health checks failed <exit>` belongs in that list. It is warn-only by design — the pass continues
to `pass ok` — so a reader working from the skill's enumeration alone can encounter an
undocumented line and mistake it for a hard error, which is exactly what the enumeration exists to
prevent.

This was found during change 0157 and deliberately left untouched: change 0145 owns
`skills/docket-status/SKILL.md`, and editing it from 0157's branch would have collided.

## What changes

Add `health checks failed <exit>` to the normal-outcomes enumeration in
`skills/docket-status/SKILL.md`, matching the posture already documented in
`scripts/docket-status.md` (warn-only, health-pass family, deliberately outside the `board `
family, cause stays on stderr).

Check at build time whether change 0145 has landed and whether it already covers this; if so, this
change is trivial or a no-op.

## Out of scope

- Any behavior change to the line itself, its wording, or its exit-code posture — 0144 settled all
  of that.
- Re-auditing the rest of the skill's enumeration against the script contract. If that audit is
  worth doing it is its own change.
