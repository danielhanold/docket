# Health check for a stale `## Finalize blocked` marker — results
Change: #0098 · Branch: feat/stale-finalize-marker-health-check · PR: <pending> · Plan: docs/superpowers/plans/2026-07-19-stale-finalize-marker-health-check.md · ADRs: none

## Verify (human)

No manual checks required — the new `stale-finalize-blocked` check is fully covered by
`tests/test_board_checks.sh` (RED→GREEN captured; whole suite green). CI green is the receipt.

## Findings

- **No ADR needed.** Every non-obvious decision was settled at spec time and recorded in the spec's
  assumptions A1–A5 (time-based not cause-re-probing; git commit-ts not the model-authored in-body
  date; fire on any marker past the horizon; hardcoded 72 h constant, no config knob). The build was
  faithful transcription — no new architectural decision arose.
- **Whole-branch review verdict: ready to merge (Yes).** No Critical/Important findings. Two Minor:
  (1) the marker-age `git log … -- "$f"` pathspec silently returns empty under a *relative*
  `--changes-dir` — addressed with a code comment noting the absolute-dir dependence (commit
  `65809c4`); real callers (docket-status) and the tests always pass an absolute dir. (2) the finding
  message opens with a literal `## ` token and its "marker set Nh ago" age is the file's last-commit
  age (== marker age while the file is quiescent, spec A2's accepted approximation) — left as-is,
  intentional and within spec.

## Follow-ups

- **docket-status SKILL.md health-checks enumeration drift.** `skills/docket-status/SKILL.md` still
  says "Five mechanical … checks" and lists only broken-spec / broken-plan-results / dep-cycle /
  stale-in-progress / merge-gate-stall. That count was already stale before this change
  (`merged-orphan` and `unknown-commit-ref` were never reflected) and is now stale by three with
  `stale-finalize-blocked` added. Deliberately out of scope here (SKILL.md is not in this change's
  file set, is published to `main`, and no test asserts on the count). A clean follow-up should fix
  the count and add the three missing bullets to match `scripts/board-checks.md`.
