---
id: 250
slug: repo-scope-detect-merged-s-fallback-and-guard-the-idle-secs
title: 'Repo-scope detect-merged''s fallback and guard the idle-secs duplication'
status: done
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-08
depends_on: []
related: []
discovered_from: [239, 241]
adrs: [72]
spec: docs/superpowers/specs/2026-08-07-repo-scope-detect-merged-s-fallback-and-guard-the-idle-secs-design.md
plan: docs/superpowers/plans/2026-08-07-repo-scope-detect-merged-fallback-and-guard-idle-secs.md
results: docs/results/2026-08-07-repo-scope-detect-merged-s-fallback-and-guard-the-idle-secs-results.md
trivial: false
auto_groomable: true
branch: feat/repo-scope-detect-merged-s-fallback-and-guard-the-idle-secs
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/175
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-repo-scope-detect-merged-s-fallback-and-guard-the-idle-secs-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-repo-scope-detect-merged-s-fallback-and-guard-the-idle-secs-design.md) |
| Plan | [2026-08-07-repo-scope-detect-merged-fallback-and-guard-idle-secs.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-07-repo-scope-detect-merged-fallback-and-guard-idle-secs.md) |
| Results | [2026-08-07-repo-scope-detect-merged-s-fallback-and-guard-the-idle-secs-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-07-repo-scope-detect-merged-s-fallback-and-guard-the-idle-secs-results.md) |
| ADRs | [ADR-0072](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0072-leg-c-predicate-duplicated-by-value-across-two-scripts.md) |
<!-- docket:artifacts:end -->

## Why

Consolidates #0239 and #0241 (2026-08-07 triage): the two follow-ups harvested from 0219's close-out, both in `scripts/docket-status.sh`, both small and test-heavy.

Verified 2026-08-07:

- **`detect_merged`'s fallback ignores `--repo` (#0239) — a behavioral correctness bug.** `docket-status.sh:555`: `"$GH" pr list --head "feat/$slug" --state merged --json number,mergedAt` — no `--repo`, so under a `--repo`-scoped pass it queries whatever repo the cwd implies. The sibling graphql arm honors the flag, and the correct shape exists one function away at `:728` (`detect_orphan_pr`, with the explicit comment "`--repo \"$repo\"` is what SPENDS the resolution above"). `detect_merged` resolves `local repo="${REPO_FLAG:-}"` at `:515` and never spends it in the fallback arm.
- **No correspondence guard over the ADR-0072 duplication (#0241).** `ORPHAN_PR_IDLE_SECS=$(( 2 * 3600 ))` (docket-status.sh:577) and `ABORTED_RUN_IDLE_SECS=$(( 2 * 3600 ))` (board-checks.sh:189) are by-value duplicates ADR-0072 deliberately accepted (board-checks stays offline-runnable) — with prose comments at each site as the only link. `grep -rn ORPHAN_PR_IDLE_SECS tests/` returns nothing; a one-sided retune drifts silently.

## What changes

Design settled 2026-08-07 (auto-groom); detail in the linked spec.

- Add `--repo "$repo"` — unconditionally, since detect_merged's early returns guarantee `$repo` is resolved before the fallback runs — to the `gh pr list` fallback, mirroring `detect_orphan_pr`'s shape and call-site comment; new dedicated argv-recording GH stub asserts per the 0219 fixture idiom, including the REPO_FLAG end-to-end rerun.
- Add a correspondence guard in `tests/test_docket_status.sh`: textual extraction of the two `NAME=` assignment lines (exactly-one-match anchors), arithmetic evaluation in the test shell (no sourcing, no shared file, no third component — ADR-0072 stands), value-equality assert, plus an in-suite sed-mutation witness proving the guard reddens on a one-sided retune.
- Update `scripts/docket-status.md`, which quotes the fallback command verbatim without `--repo` (mandatory doc touch). No `## Update` note on ADR-0072 (optional, reversible via docket-adr later).
- `adrs: [72]` set (repairs the killed #0241's frontmatter omission).

## Out of scope

- Refactoring the duplicated predicate into a shared helper (ADR-0072 decision stands).
- Guarding the broader predicate *shape* (base handling, ref resolution) — narrowed to the idle-secs values at triage; shape stays prose-mitigated.
- Any other `detect_*` leg; no other `gh` call sites; no change to `scripts/board-checks.sh`.

## Reconcile log

### 2026-08-07 — reconciled at claim (no scope change)

Re-verified every premise against `origin/main` @ `483c5dad`:

- `scripts/docket-status.sh:555` still carries the unscoped `"$GH" pr list --head "feat/$slug" --state merged --json number,mergedAt` — the `--repo` omission is live. The sibling `detect_orphan_pr` shape at `:728` still passes `--repo "$repo"`, so the mirror target is unchanged.
- `ORPHAN_PR_IDLE_SECS=$(( 2 * 3600 ))` (docket-status.sh:577) and `ABORTED_RUN_IDLE_SECS=$(( 2 * 3600 ))` (board-checks.sh:189) are both still present, still equal, still by-value duplicates; `grep -c ORPHAN_PR_IDLE_SECS tests/test_docket_status.sh` is still `0` — no guard exists.
- `scripts/docket-status.md:161` still quotes the fallback without `--repo`, so the mandatory doc touch of assumption 5 still resolves true.

Nothing has been done elsewhere, no new ADR bears on the design, and no constraint has changed. Spec, scope, and out-of-scope list carry forward verbatim. `adrs: [72]` already set. No auto-capture candidates: everything this pass surfaced is inside the change's own scope.
