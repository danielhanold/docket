<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0307 — Domain snapshot, validation, graphs, and selection](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0307-domain-snapshot-validation-graphs-and-selection.md)**
<!-- docket:backlink:end -->

# Domain snapshot, validation, graphs, and selection — results
Change: #307 · Branch: feat/domain-snapshot-validation-graphs-and-selection · PR: recorded in the change manifest's `pr:` field · Plan: docs/superpowers/plans/2026-08-14-domain-snapshot-validation-graphs-and-selection.md · ADRs: 92, 93

## Verify (human)

- [ ] Accept the pre-existing suite advisory `OVER BUDGET: test_sync_agents_runners` — wall-clock
  budgets are machine-dependent and the overage predates this branch; confirm it needs no action
  before merge.

## Findings

- **Reference severity needed grading by structural role** (became **ADR-0093**). The first
  implementation raised an error for every dangling change/ADR reference. Review showed that only
  scheduling-bearing references (`depends_on`, `stacked_on`) can corrupt selection; associative
  references (`related`, `discovered_from`, ADR `relates_to`/`change`) now produce warnings so a
  pruned archive cannot block mutation.
- **ADR evolution's identity-reuse check fired on archive moves.** Comparing holders by path made a
  legitimate active→archive move look like ID reuse. Fixed by comparing surviving holders by
  content identity (`survivingHolders`): a move is not a reuse.
- **The frozen-corpus fixture collided with the repo-wide grep-portability scan.** A v0.9.2 corpus
  record legitimately contains `{0,600}` (an ERE bound BSD grep rejects), and Go slice literals
  like `{305}` false-positived as bounds. The scan now excludes
  `internal/repository/testdata/corpus/*` and constrains the Go-literal case, with
  mutation-verified boundedness controls so the exclusions cannot silently widen.
- **Filename identity is strict, not prefix-based.** A review blocker showed the slug-prefix
  relaxation admitted a vacuous empty-slug match; `identityMatchesFilename` is back to strict
  equality between the record's `id`/`slug` and its filename.
- **Claim needed an eligibility gate.** `Claim` now refuses an invalid slug and routes through
  `ClaimEligibility(status, claim, facts)` rather than assuming callers pre-checked.

## Follow-ups

- None minted. Auto-capture ran at both sites (reconcile, review) with zero admissible candidates;
  all review findings were fixed in-branch (10/10, dispositions in the PR body).
