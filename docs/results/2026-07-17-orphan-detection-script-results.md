<!-- results-template.md — close-out artifact for a change. -->
# Orphan detection script — results
Change: #0092 · Branch: feat/orphan-detection-script · PR: see change #0092 `pr:` · Plan: docs/superpowers/plans/2026-07-17-orphan-detection-script.md · ADRs: 0001, 0012 (cited, pre-existing; none produced)

## Verify (human)

<!-- No hard manual gate: behavior is covered by the hermetic suite plus the build-time real-history
     check recorded under Findings. The item below is an optional post-merge confirmation. -->
- [ ] Optional: on the first `docket-status` full pass after merge, confirm `merged-orphan` /
      `unknown-commit-ref` surface no spurious findings against real `origin/main` history (build-time
      check already showed zero false positives — see Findings).

## Findings

- **Real-history verification — zero false positives (the primary risk, retired).** The hermetic
  `tests/test_board_checks.sh` cannot exercise real integration-branch history (the
  `metadata-branch-invisible-to-suite` learning), so the whole-branch review verified it manually:
  running the branch's `board-checks.sh` over the actual `.docket` change set (19 active, 75 archived)
  against real `origin/main` (543 grammar-matching commit subjects) produced **zero** findings. The
  detection path was proven live (not a swallowed no-op) by deleting an archived record in a throwaway
  copy, which correctly fired `unknown-commit-ref 85` with evidence commit `250ff7c` and correct
  `10#0085`→85 normalization. The real tree was left pristine.
- **No ADRs produced.** Every design decision (home = `board-checks.sh`; two-form subjects-only
  grammar; full-history stateless window; class-2 deferred to #0083) was settled at brainstorm and is
  recorded in the spec's `## Assumptions`, citing the existing ADR-0001 and ADR-0012.
- **Plan deviation (review-endorsed simplification).** The plan's `ID_ARCHIVED` map was dropped as
  dead code — "archived" is the implicit `else` in the emission trichotomy (`ID_ACTIVE` → merged-orphan;
  no file → unknown-commit-ref; else → silent). Behavior-neutral; shellcheck SC2034 cleared; suite green.

## Follow-ups

All pre-scoped in the spec's `## Out of scope`; none block this PR:

- **Class-2 detector** (archived-but-unpublished terminal record — the #0043 case) stays **#0083's**
  human-gated, still-undecided call. Reconfirmed at reconcile that #0083 is `proposed` /
  auto-groom-blocked; deliberately not built here.
- **`--since` history window / persisted high-water mark** — additive follow-up if integration-branch
  history grows enough to matter (full-history scan is fine at current repo size).
- **Grammar widening** (bare `#NNNN`, merge-subject branch names) — deferred as the false-positive-prone
  options; the conservative floor shipped first.
- **Recall floor:** one id per grammar form per subject (leftmost `BASH_REMATCH`). Accepted spec floor;
  widening to multi-ref extraction is additive.
