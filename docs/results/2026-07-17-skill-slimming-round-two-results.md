# Second-round skill slimming — results
Change: #85 · Branch: feat/skill-slimming-round-two · PR: (see change file) · Plan: docs/superpowers/plans/2026-07-16-skill-slimming-round-two.md · ADRs: 12 (cited, none new)

## Verify (human)

- [ ] Skim `skills/docket-convention/SKILL.md` once end-to-end — the slim is behavior-neutral by
      test-anchor proof, but the convention is the file every docket run loads; a human read of the
      compressed Learnings/Agent-layer/Branch-model sections is the last defense against a meaning
      shift the sentinels can't see.
- [ ] Confirm the terminal-close-out step-5 posture fix reads correctly to you: the step 1–3
      Failure-posture table is unchanged; step 5 (Board) now defers to each caller's own skill body
      (`docket-implement-next`'s reconcile-kill board pass is best-effort; `docket-new-change`'s
      proposed-kill is must-land).

## Findings

- **Pre-existing posture mismatch (fixed in Task 5).** `terminal-close-out.md` step 5 lumped "the
  two kill callers" into abort-and-report, but `docket-implement-next`'s reconcile-kill runs its
  board pass best-effort per its own skill body — contradicting the file's own "Steps 4–5 follow
  the caller" rule. Fixed as a doc-consistency alignment (no behavior change; no test pinned the
  old wording). Predates this change.
- **Task 2 deviation (accepted at review).** The brief's literal single abort-and-report template
  for terminal-close-out step 5 would have made the `docket-status` merge sweep's instructions
  read as abort-and-report — a real behavior change. Both posture sentences were kept instead;
  independently re-verified by a reviewer tracing the sweep call chain.
- **Sentinel net works.** Four anchor regressions across the build (two mid-phrase line wraps in
  the convention, the adr `--enabled` 2-count, groom-next's `integration_branch` mention) were all
  caught by the existing suites within one gate run and repaired without weakening any test —
  `test-premise-deleted-not-regated` honored: KEEP-list phrases were restored inline, never
  re-anchored to green.
- **Landing sizes vs targets** (direction, not gate — `size-target-is-direction`): convention
  288 L / 4,640 w (target ≤ ~200 / ≤ ~2,600; residual is YAML blocks, tables, diagrams, and
  test-anchored contract sentences), finalize 145/2,453, status 107/2,175, implement-next
  108/2,228, adr 78/1,280, groom-next 70/1,349, new-change 55/1,209, auto-groom 60/1,124,
  brainstorm 76/629, agent-layer 152/1,671, terminal-close-out 133/1,125, learnings (new) 76/527.
  `tests/test_skill_size_budgets.sh` pins all 16 files at actual + ~10% (mutation-proven on both
  the size and completeness asserts).

## Follow-ups

- `scripts/docket-status.md` exit-codes section has two back-to-back "non-zero —" list items
  (cosmetic; noted at Task 1 review, contract tests green).
- No test locks `--help`/usage completeness for `--must-land` (the file convention only greps
  "usage"); fine today, worth folding into any future script-contract lint.

## Build-execution note

Tasks 1–2 were built by SDD subagents (prior orchestrator sessions); from Task 2's review onward
the change was executed inline in a single session at the user's explicit request (no build
subagents; per-task review + gates run inline). One just-dispatched Task-3 implementer agent was
stopped ~30 s in, before any edits, when the takeover happened.
