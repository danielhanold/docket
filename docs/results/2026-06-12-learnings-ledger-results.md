# Learnings ledger — results
Change: #6 · Branch: feat/learnings-ledger · PR: (set at open) · Plan: docs/superpowers/plans/2026-06-12-learnings-ledger.md · ADRs: 5

## Verify (human)

- [ ] The seeded ledger reads well: `git show origin/docket:docs/changes/LEARNINGS.md` — six
  entries distilled from the five archived results files (0001, 0002, 0005 ×2, 0012 ×2), newest
  first, each with an "Apply:" clause. Edit freely; it is curated prose, yours to prune.
- [ ] Live-harvest smoke test: when THIS change is finalized, `docket-finalize-change`'s new
  step 2.5 runs for the first time — confirm it probes the ledger for `(#6`, harvests zero or
  more entries from this PR's review + this results file, and commits the ledger separately
  from the archive commit.

## Findings

- **Spec deviation (documented in the plan):** spec §8 asked a test to assert `LEARNINGS.md`
  exists with its header contract — impossible as a repo test, since the ledger lives only on
  the `docket` branch and the suite runs against the integration-branch checkout. Covered
  instead by orchestrator verification at seed time (confirmed on `origin/docket`) and this
  results file.
- The close-out-only/single-writer/unpublished decisions became ADR-0005.
- First real consumption of the ledger happened during this build: the final whole-branch review
  was primed with the seeded lessons (paraphrase-hiding, non-vacuous assertions) and
  mutation-tested the new test's assertions as a result.

## Follow-ups

- The ledger's distill rule (~300-line soft cap) has no test by design — it is a judgment call
  the harvest makes; first exercised whenever the file grows past the cap.
