---
id: 201
slug: skill-compression-round-three
title: Skill compression round three — targeted progressive disclosure on the Big 4 + regrowth-guard ratchet
status: in-progress
priority: medium
type: refactor
created: 2026-08-02
updated: 2026-08-03
depends_on: []
related: [53, 55, 85, 137, 167]
discovered_from: []
adrs: []
spec: docs/superpowers/specs/2026-08-02-skill-compression-round-three-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/skill-compression-round-three
claimed_at: 2026-08-03T03:03:22Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-02-skill-compression-round-three-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-02-skill-compression-round-three-design.md) |
<!-- docket:artifacts:end -->

## Why

Rounds one (0053/0055) and two (0085) of skill slimming both hit their targets and both
regrew: the regrowth-guard budgets have been raised five times since 0085 (0102, 0127,
0137, 0167 ×2), and the four largest skills now sit within 11–51 words of their caps.
`docket-convention` — loaded as blocking Step 0 on every docket run — is at 6,349 words,
well past the < 5k guidance for a frequently-loaded skill, and every excess token is paid
on nearly every docket operation. The budget test has degraded into a speed bump: each
change asserts its prose has "no other home" and raises the cap in-diff.

## What changes

One change applies targeted progressive disclosure to the Big 4 (design in the linked
spec):

- **Three new reference files**, each a cold path behind a loud blocking pointer at its
  trigger moment: `docket-finalize-change/references/gate-failure.md` (rebase-conflict +
  repair dispatch, sign-off, `## Finalize blocked` lifecycle),
  `docket-implement-next/references/edge-paths.md` (reconcile-kill, blocked transition,
  resume-with-id, PR-body mechanics), and
  `docket-convention/references/auto-capture.md` (the full auto-capture shared
  definition; a ~4-line summary + read trigger stays inline).
- **In-place tightening** of all four files: provenance → bare pointers, duplicated
  litanies → single owner + citation, enumerable prose → tables. 0137's
  dispatch-capability rule and 0167's halting dispositions are tightened in wording only
  — same content, location, and citation anchors (decided in brainstorm; not reversed).
- **Targets:** convention ≤ ~4,700 w, finalize ≤ ~2,900, implement-next ≤ ~2,900, build
  ≤ ~2,100 (~35% off the Big 4 combined).
- **Regrowth-guard hardening:** budgets ratchet down to post-slim actuals + margin; the
  raise procedure now requires naming the reference file the new prose was considered
  for and why it cannot live there.
- Verification per the proven 0085 recipe: anchor grep-gate, behavior-neutrality diff
  review, loud-pointer check, budget test green at the lower rows, status smoke run.

## Out of scope

- Skill semantics or workflow behavior; only relocation and rewording.
- Frontmatter `description:` lines, agent wrappers, `sync-agents.sh`, scripts, templates,
  `github-board-mirror.md`.
- The seven skills already ≤ 1.4k words, and convention's three existing references.
- Reversing 0137's or 0167's inline-placement decisions.

## Open questions

- Exact per-file budget numbers — fixed at build time from post-slim actuals via the
  existing rounding rule.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
