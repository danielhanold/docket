---
id: 99
slug: finalize-marker-clearing-rule-wording
title: Re-phrase the `## Finalize blocked` clearing rule around what it actually guards
status: done
priority: low
created: 2026-07-19
updated: 2026-07-19
depends_on: []
related: [87, 98]
discovered_from: [87]
adrs: []
spec: docs/superpowers/specs/2026-07-19-finalize-marker-clearing-rule-wording-design.md
plan: docs/superpowers/plans/2026-07-19-finalize-marker-clearing-rule-wording.md
results: docs/results/2026-07-19-finalize-marker-clearing-rule-wording-results.md
trivial: false
auto_groomable: false
branch: feat/finalize-marker-clearing-rule-wording
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/107
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-19-finalize-marker-clearing-rule-wording-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-19-finalize-marker-clearing-rule-wording-design.md) |
| Plan | [2026-07-19-finalize-marker-clearing-rule-wording.md](https://github.com/danielhanold/docket/blob/feat/finalize-marker-clearing-rule-wording/docs/superpowers/plans/2026-07-19-finalize-marker-clearing-rule-wording.md) |
| Results | [2026-07-19-finalize-marker-clearing-rule-wording-results.md](https://github.com/danielhanold/docket/blob/feat/finalize-marker-clearing-rule-wording/docs/results/2026-07-19-finalize-marker-clearing-rule-wording-results.md) |
| PR | [#107](https://github.com/danielhanold/docket/pull/107) |
<!-- docket:artifacts:end -->

## Why

The `## Finalize blocked` clearing rule as written closes on an over-broad universal — *"State
encoded by an artifact's presence must be cleared by every transition out of that state."* Read
literally, that demands stripping the section on the way to `done`, which nothing does:
`archive-change.sh` only moves the file and sets frontmatter scalars, so on an out-of-band human
merge the section rides verbatim into `archive/`.

The rule's first two sentences are true and load-bearing; the closing universal is what invites a
reader to implement redundant clearing logic, or to conclude the rule is dead and drop it. What is
actually true is narrower: **every** reader of the marker is scoped to a pre-`done` change — the
board's `implemented`-only cell, the auto-detect selection skip on unmerged candidates, and the
`stale-finalize-blocked` health check (also `implemented`-only) — so archiving retires the marker's
meaning whether or not the section is physically present.

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

### 2026-07-19 — reconcile at claim

Re-read against `origin/main` at `a75995f`, the linked spec, related #87/#98, and the current code.

- **Design intact.** The three target texts are byte-for-byte as the spec quotes them:
  `skills/docket-finalize-change/SKILL.md` clearing bullet (now line 162 within the
  `### ## Finalize blocked` subsection), `skills/docket-convention/SKILL.md:171`, and the sole
  anchored sentinel `tests/test_finalize_disposition.sh:120`. No re-brainstorm needed.
- **New constraint folded in — a THIRD reader.** Change 0098 merged today (PR #106) and added
  `stale-finalize-blocked` to `scripts/board-checks.sh:140-158`, which reads the marker via
  `finalize_blocked()`. The spec's decision text enumerates "the marker's **two** readers"; that
  count is now stale. The check is gated on `[ "$status" = "implemented" ]`, so the *scoped truth*
  the change turns on is unchanged — it is strengthened, since a third independent reader also
  stops applying at `done`.
- **Consequent scope adjustment.** The replacement sentence must **not** hard-code an enumeration
  of readers, which 0098 has just demonstrated goes stale within a day. Phrase it as the property
  ("every reader is scoped to a pre-`done` change") rather than a list. Naming readers
  parenthetically is acceptable only as illustration, never as the load-bearing claim.
- **No other site needs the edit.** `README.md:184` already states the clearing rule in the
  narrow, correct form ("skipped by later *unscoped* runs until a successful finalize clears it
  automatically") and carries no over-broad universal — out of scope, unchanged.
  `scripts/lib/docket-frontmatter.sh:108` describes the marker as presence-encoded state without
  asserting a clearing obligation — correct as-is.
- **Out-of-scope confirmations re-verified against current code.** `scripts/archive-change.sh`
  still never removes a body section, so the out-of-band-merge path still carries the section into
  `archive/` verbatim — the premise of the whole change survives.
- No auto-capture (`auto_capture: false` for this repo): the third-reader drift is folded into
  this change's own scope above, not minted as follow-up work.
