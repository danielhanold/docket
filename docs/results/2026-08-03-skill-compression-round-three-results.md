<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0201 — Skill compression round three — targeted progressive disclosure on the Big 4 + regrowth-guard ratchet](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0201-skill-compression-round-three.md)**
<!-- docket:backlink:end -->

# Skill compression round three — results
Change: #0201 · Branch: feat/skill-compression-round-three · PR: (opened at close-out) · Plan: docs/superpowers/plans/2026-08-03-skill-compression-round-three.md · ADRs: none

## Verify (human)

- [ ] Spot-read one extracted reference at its trigger moment (e.g. force a gate-failure path
  mentally through `docket-finalize-change/SKILL.md` → `references/gate-failure.md`) and confirm
  the pointer text is loud enough that an agent would actually read it before acting.
- [ ] Judge the two accepted target overshoots below — if either file should be pushed further,
  that is a conscious round-four decision, not a missed step here.

## Findings

- **Post-slim actuals vs spec targets** (targets were direction, per the spec's own rule):
  convention 5,773 w (target ~4,700), finalize 3,395 w (~2,900), implement-next 3,654 w (~2,900),
  build 2,348 w (~2,100). The Big-4 hot path went 17,008 → 15,170 words (−10.8%) with 1,617 words
  moved to three cold references (gate-failure 852, edge-paths 389, auto-capture 376). The
  convention re-confirmed the compressibility floor #85 documented: the residual is
  sentinel-pinned, normative contract text (the 0137 dispatch rule, wrapper counts, YAML contracts,
  lifecycle), and the file has grown four subsystems since the round-two floor of 4,640 w.
- **Review (deep rung, self-report): 2 minor, 0 blocker/important.** (1) The `## Finalize blocked`
  named-id override's anti-deadlock rationale was dropped rather than relocated — the rule
  survives in the SKILL.md stub, the *why* is now stated nowhere. (2) The convention's
  never-a-mint-site clause dropped the `auto_groom` × `auto_capture` backlog-growth-loop
  consequence, keeping only the invariant name. Both left for merge-time judgment per the
  no-auto-fix triage rule.
- The budget-test anchor net worked exactly as designed: two anchor-phrase misses (Task 2) and two
  reflow line-break breaks (Task 6: the `LEARNINGS.md` pointer-stub same-line exemption, the
  `is a **derived view**` literal) were caught by the focused sets and fixed in-task.
- Budgets ratcheted down for the first time since 0085 (convention 365/6400 → 345/5800, finalize
  193/4350 → 180/3450, implement-next 147/3950 → 145/3700, build 270/2450 → 265/2400) and the
  raise procedure now requires arguing the reference-file case in-diff.

## Follow-ups

- In-flight change 0113 also edits `skills/docket-implement-next/SKILL.md` and
  `tests/test_skill_size_budgets.sh` on its unmerged branch; whichever of 0113/0201 merges second
  rebases across a real (but intent-composable) conflict — the budget rows will need re-measuring
  against the merged file, never taken from either side (learnings:
  concurrent-edits-compose-at-rebase).
