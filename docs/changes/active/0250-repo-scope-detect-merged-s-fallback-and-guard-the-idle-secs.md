---
id: 250
slug: repo-scope-detect-merged-s-fallback-and-guard-the-idle-secs
title: 'Repo-scope detect-merged''s fallback and guard the idle-secs duplication'
status: proposed
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [239]
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

Consolidates #0239 and #0241 (2026-08-07 triage): the two follow-ups harvested from 0219's close-out, both in `scripts/docket-status.sh`, both small and test-heavy.

Verified 2026-08-07:

- **`detect_merged`'s fallback ignores `--repo` (#0239) — a behavioral correctness bug.** `docket-status.sh:555`: `"$GH" pr list --head "feat/$slug" --state merged --json number,mergedAt` — no `--repo`, so under a `--repo`-scoped pass it queries whatever repo the cwd implies. The sibling graphql arm honors the flag, and the correct shape exists one function away at `:728` (`detect_orphan_pr`, with the explicit comment "`--repo \"$repo\"` is what SPENDS the resolution above"). `detect_merged` resolves `local repo="${REPO_FLAG:-}"` at `:515` and never spends it in the fallback arm.
- **No correspondence guard over the ADR-0072 duplication (#0241).** `ORPHAN_PR_IDLE_SECS=$(( 2 * 3600 ))` (docket-status.sh:577) and `ABORTED_RUN_IDLE_SECS=$(( 2 * 3600 ))` (board-checks.sh:189) are by-value duplicates ADR-0072 deliberately accepted (board-checks stays offline-runnable) — with prose comments at each site as the only link. `grep -rn ORPHAN_PR_IDLE_SECS tests/` returns nothing; a one-sided retune drifts silently.

## What changes

- Add `--repo "$repo"` (when resolved) to the `detect_merged` fallback; argv-recording stub assert per the 0219 fixture idiom.
- Add a correspondence guard asserting the two idle-secs values stay equal — via whatever minimal shared sentinel the two scripts can be compared against; explicitly NOT a shared-helper refactor (ADR-0072 rejected that).
- Set this change's `adrs: [72]` (the killed #0241 leaned on ADR-0072 but left its frontmatter `adrs:` empty).

## Out of scope

- Refactoring the duplicated predicate into a shared helper (ADR-0072 decision stands).
- Any other `detect_*` leg.
