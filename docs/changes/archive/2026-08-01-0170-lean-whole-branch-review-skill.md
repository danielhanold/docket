---
id: 170
slug: lean-whole-branch-review-skill
title: Lean Docket-owned whole-branch review skill
status: done
priority: medium
type: feat
created: 2026-07-30
updated: 2026-08-01
depends_on: [167, 184]
related: [137]
discovered_from: [167]
adrs: [66]
spec: docs/superpowers/specs/2026-08-01-lean-whole-branch-review-skill-design.md
plan: docs/superpowers/plans/2026-08-01-lean-whole-branch-review-skill.md
results: docs/results/2026-08-01-lean-whole-branch-review-skill-results.md
trivial: false
auto_groomable:
branch: feat/lean-whole-branch-review-skill
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/149
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-01-lean-whole-branch-review-skill-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-01-lean-whole-branch-review-skill-design.md) |
| Plan | [2026-08-01-lean-whole-branch-review-skill.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-01-lean-whole-branch-review-skill.md) |
| Results | [2026-08-01-lean-whole-branch-review-skill-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-01-lean-whole-branch-review-skill-results.md) |
| ADRs | [ADR-0066](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md) |
<!-- docket:artifacts:end -->

## Why

Change 0167 removes SDD's per-task and final reviews but deliberately preserves Docket Step 6 as
one independent whole-branch review. The current default still delegates that boundary to
Superpowers, leaving its model, effort, recursion, and output cost outside Docket's explicit
control — and its "All tests passing?" checklist re-runs the ~10-minute full suite on the exact
branch state the build gate just tested. With finalize's post-rebase run, one change pays for
three full-suite executions (~30 minutes); only the build gate's and finalize's runs have
distinct justifications, and finalize's only when the base actually moved.

## What changes

Ship `docket-review` — a Docket-owned `skills.review` replacement — plus the suite-once
evidence chain it consumes:

- One bounded, **read-only** whole-branch reviewer, dispatched foreground from
  `docket-implement-next` Step 6 via one of **three pinned rung wrappers**
  (lean / standard / deep; Claude: sonnet-5/high, opus-5/medium, opus-5/high), selected
  deterministically as "one above the build" — from the highest profile the build routed or
  escalated to, with an optional diff-size bump. It returns severity-tiered findings
  (blocker / important / minor) and never fixes, never dispatches subagents, and **never runs
  the test suite**.
- The build gate stays the suite's sole implementation-phase home and records a
  **build-evidence block** (command, result, HEAD SHA, timestamp). The reviewer verifies it
  instead of re-running; Step 7 writes it durably into the PR body.
- Controller triage: blockers → one synthetic fix task through the existing
  `docket-build-task` contract, then one suite re-run refreshing the evidence; important/minor
  → PR body for merge-time judgment; follow-ups → existing auto-capture. No re-review round.
- `docket-finalize-change`'s local gate **skips** its post-rebase suite run only when the
  rebase was a no-op and the PR's evidence block is green at the exact branch HEAD; any doubt
  runs the suite. Net: one run when the review is clean and the base has not moved, two when
  either a blocker fix lands or the rebase actually moves the branch, three only when both do.
- Shipped default binding unchanged (`superpowers:requesting-code-review`); this repo dogfoods
  `docket-review` via `.docket.yml`. README documents the suite-placement rationale.

Design detail, diagram, schema, and the full ripple list live in the linked spec.

## Out of scope

- Per-task review.
- Implementing findings inside the reviewer.
- Changing the build profiles, rubric, or TDD contract from changes 0167/0184.
- A second review round after blocker fixes.

## Reconcile log

### 2026-08-01

Claimed and reconciled against merged `origin/main` (tip `794e7545`, 0184's terminal publish).

**Design holds — no scope change.** Both dependencies are satisfied: 0167 and 0184 are `done`
and archived, so the four-tier profile ladder this change reads its rung-selection input from
is live on main.

**Verified, not assumed:**

- The reviewer pin table was re-checked row-by-row against merged
  `agents/harness-defaults.yml`. All nine rows hold as groomed, including the invariant that
  **review-deep equals the build-max pin on every harness**.
- Wrapper roster on main is **thirteen** (`agents/docket-*.md`), so the designed 13 → 16
  transition is accurate.
- The generators need **no code change**: `sync-agents.sh` (which lives at the repo root, not
  under `scripts/`), `scripts/lib/harness-defaults.sh`, and `link-skills.sh` are all
  glob-driven, so three wrapper files and one skill directory are picked up automatically.

**Ripple list expanded.** The groom's list named the right surfaces but understated the guard
rails around them. Six hard gates were added to the spec, the two sharpest being
`test_dispatch_capability.sh`'s reverse-correspondence roster (an exact `-eq 5` site count
that three named rung dispatches will break) and `test_skill_size_budgets.sh` (four of the
files to edit sit at 3-4 lines of headroom, so budget raises ship in the same diff). The
full table and gate list live in the spec's *Reconciled ripple list*.

**Folded in rather than captured:** `skills/docket-convention/SKILL.md`'s
convention-injection sentence says "every wrapper except four" while naming five — an
off-by-one left behind by 0184. This change already edits that sentence, so it is corrected
here as in-scope drift rather than minted as separate work.
