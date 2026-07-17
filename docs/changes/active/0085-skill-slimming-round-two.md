---
id: 85
slug: skill-slimming-round-two
title: Second-round skill slimming — re-slim regrown skills + regrowth guard
status: in-progress
priority: medium
created: 2026-07-16
updated: 2026-07-16
depends_on: []
related: [53, 54, 55]
adrs: [12]
spec: docs/superpowers/specs/2026-07-16-skill-slimming-round-two-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/skill-slimming-round-two
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-16-skill-slimming-round-two-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-16-skill-slimming-round-two-design.md) |
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md) |
<!-- docket:artifacts:end -->

## Why

The 0053–0055 slimming round hit its targets on 2026-07-11, but the ~30 changes since
(learnings ledger, brainstorm consultant, terminal-publish knob, worktree hooks, finalize and
board-pass hardening) silently regrew the skill bodies. `docket-convention` — loaded as
blocking Step 0 on every docket run — is back to 329 lines / 5,453 words (target was ~190 /
~2,400), ~2.2× the < 5,000-token recommendation for frequently-loaded skills; finalize,
status, and implement-next are all above their post-slim targets. The must-land Board-pass
litany is now restated verbatim in six-plus places (three in docket-new-change alone) — the
same "identical — must not diverge" drift risk 0053 eliminated for close-out. Every excess
token is paid on nearly every docket operation, competing with the actual work's context.

## What changes

One change re-applies the proven 0053 recipe to the measured regrowth, plus two new levers
(design in the linked spec):

- **Board-pass `--must-land` flag** — the one approved mechanical shift: the bounded retry
  moves inside `scripts/docket-status.sh`; exit code encodes the outcome; callers collapse
  their ~10-line report-line litany to one line plus posture. Report-line vocabulary and
  flagless behavior unchanged.
- **docket-convention re-slim** to ≤ ~200 L / ≤ ~2,600 w: learnings ledger →
  `references/learnings.md` behind a loud blocking pointer; bootstrap-guard and agent-layer
  prose tightened; provenance narration cut to bare `(ADR-NNNN)` pointers.
- **All nine operating skills re-slimmed** to at-or-under their prior targets; Step-0
  sections re-compressed to the ~3-line convention citation; small-model explicitness
  preserved in docket-status; finalize gate/sign-off/abort sets survive verbatim in meaning.
- **References trimmed** (agent-layer, terminal-close-out): ≤ ~150 L, TOC if > 100 L.
- **Regrowth guard:** new `tests/test_skill_size_budgets.sh` asserting per-file max
  lines/words for every `skills/**/*.md`, budgets ~10% above post-slim actuals.
- Verification: anchor grep-gate, behavior-neutrality diff review (sole exception:
  `--must-land`, which gets its own script tests), sentinel re-anchoring, status smoke run.

## Out of scope

- Any semantics change beyond the `--must-land` flag.
- Frontmatter `description:` lines, template content, agent wrappers, `sync-agents.sh`.
- `github-board-mirror.md`; script behavior other than `docket-status.sh`.

## Open questions

- Exact per-file budget numbers — fixed at build time from post-slim actuals + ~10%.

## Reconcile log
