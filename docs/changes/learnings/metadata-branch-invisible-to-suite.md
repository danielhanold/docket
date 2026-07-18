---
slug: metadata-branch-invisible-to-suite
hook: "A hermetic suite sees only its fixtures and the integration-branch checkout — verify metadata-branch artifacts and real-history behavior at build time, and record it in the results file."
topics: [testing, metadata-branch, docket]
changes: [6, 89, 92]
created: 2026-06-12
updated: 2026-07-18
promotion_state: retained
promoted_to:
---

## Apply
When a behavior's real input is repo state the suite cannot construct — a metadata-branch
artifact, or the integration branch's actual commit history — do not specify a test for it.
Verify it at build time against the real tree, prove the detection path fires (mutate a throwaway
copy, watch it redden), and record both in the results file's `## Findings`.

## War story
- 2026-06-12 (#6, PR #8) — The spec asked for a test asserting a metadata-branch file exists,
  but the suite runs against the integration-branch checkout where that file never lives.
- 2026-07-18 (#92, PR #98) — Orphan detection reads real `origin/main` commit subjects, which the
  hermetic fixture suite cannot supply; a green suite said nothing about the actual false-positive
  risk. The whole-branch review ran the checks over the live repo (19 active / 75 archived changes
  vs 543 matching subjects) for zero findings, then proved the path was not a swallowed no-op by
  deleting an archived record in a throwaway copy and watching `unknown-commit-ref` fire.
- 2026-07-18 (#89, PR #99) — Reclaim mutates `docket`-branch change files, which the
  integration-branch suite cannot see. Rather than skip coverage, the build gave the script `NOW`
  and `GIT` seams and drove `tests/test_reclaim_claims.sh` against a real temp git repo with a bare
  origin — hermetic, but exercising the actual mutate-and-push path. The migration question was
  answered the same way: a pre-migration change carrying no `claimed_at:` is never reclaimed
  (absence of a lease is not evidence of an expired one), verified end-to-end instead of asserted.
