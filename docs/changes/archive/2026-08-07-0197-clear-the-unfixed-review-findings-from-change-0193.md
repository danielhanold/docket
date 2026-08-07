---
id: 197
slug: clear-the-unfixed-review-findings-from-change-0193
title: Clear the unfixed review findings from change 0193
status: killed
priority: medium
type: chore
created: 2026-08-02
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [193]
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

PR #152 (change 0193) merged with five non-blocking review findings left unfixed by deliberate
merge-time judgment. They are small, real, and independent of the change's own correctness, so
they belong in their own pass rather than in a hotfix.

## What changes

- `README.md` — the agent-roster table still calls `docket-build` and `docket-review` "opt-in",
  contradicting the same file's `### docket-build` / `### docket-review` sections. Two cells.
  (The `docket-brainstorm` row's "opt-in" is correct and stays.)
- `skills/docket-convention/SKILL.md` — the config sketch header comment still reads "unset key =
  the superpowers default shown" directly above the two rows now showing docket-owned values.
- `tests/test_docket_review.sh` — the inverted absence assert `[ -z "$dy_skills" ]` is unanchored
  and passes green on any extraction failure. Add a live non-vacuity companion through the same
  extractor, as `tests/test_docket_build.sh` already does. This is the one with teeth.
- `tests/test_docket_config.sh` — the two added `!=` asserts are entailed by the `=` asserts above
  them and cannot redden independently; drop or re-aim them.
- The `README says how to opt back into SDD` guard is a whole-file substring already satisfied by
  incidental prose, so deleting the opt-out fence would leave it green. Anchor it.

## Out of scope

- The role-skill prose framing tracked separately as #0194.

## Open questions

- Whether the two entailed `!=` asserts are worth re-aiming at all, or simply removing.

## Why killed

Consolidated into #0257 at the 2026-08-07 backlog triage: the small review-finding clearance class (0193 residue) lands with #0204's rationale restorations as one sweep.
