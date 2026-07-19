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
spec: docs/superpowers/specs/2026-07-19-finalize-marker-clearing-rule-wording-design.md
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-19-finalize-marker-clearing-rule-wording-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-19-finalize-marker-clearing-rule-wording-design.md) |
<!-- docket:artifacts:end -->

## Why

The `## Finalize blocked` clearing rule as written closes on an over-broad universal — *"State
encoded by an artifact's presence must be cleared by every transition out of that state."* Read
literally, that demands stripping the section on the way to `done`, which nothing does:
`archive-change.sh` only moves the file and sets frontmatter scalars, so on an out-of-band human
merge the section rides verbatim into `archive/`.

The rule's first two sentences are true and load-bearing; the closing universal is what invites a
reader to implement redundant clearing logic, or to conclude the rule is dead and drop it. What is
actually true is narrower: the marker's only readers — the board's `implemented`-only cell and the
auto-detect selection skip on unmerged candidates — both stop applying at `done`, so archiving
retires the marker's meaning whether or not the section is physically present.

## What changes

Re-phrase the closing sentence of the clearing-rule bullet in
`skills/docket-finalize-change/SKILL.md` around that scoped truth, keeping **"A successful finalize
removes the section"** intact — it is a real cleanup on the live path, and
`tests/test_finalize_disposition.sh:120` is anchored on that phrasing. Re-point the convention's
restatement (`skills/docket-convention/SKILL.md:171`) at the skill rather than restating the rule
independently.

**Wording only — no behavior change.** Design settled in the linked spec; see its
*Explicitly decided against* and *Reconciling with the `presence-encoded-state` learning* sections
before touching this.

## Out of scope

- Any change to when the marker is written or skipped.
- **Strip-on-archive, and a `## Finalize resolved` note at archive time** — both considered and
  **decided against** (maintainer, 2026-07-19), not deferred pending design. Rationale in the spec.
- `archive-change.sh` in any form.
- The stale-marker health check (change 0098).

## Open questions

_Both resolved during grooming — see the spec's `## Open questions — resolved`._

## Reconcile log
