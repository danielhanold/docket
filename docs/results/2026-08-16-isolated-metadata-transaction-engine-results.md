<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0309 — Isolated metadata transaction engine](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-16-0309-isolated-metadata-transaction-engine.md)**
<!-- docket:backlink:end -->

# Isolated metadata transaction engine — results
Change: #309 · Branch: feat/isolated-metadata-transaction-engine · PR: <set at PR open> · Plan: docs/superpowers/plans/2026-08-15-isolated-metadata-transaction-engine.md · ADRs: 1, 34, 89 (cited, unchanged)

## Verify (human)

<!-- The automated suite (scripts/run-tests.sh) is fully green and is the primary receipt. No
     manual/interactive checks are required to accept this change. Two merge-gate awareness items: -->
- [ ] Awareness only (no action needed to merge): the full suite prints `OVER BUDGET:` for
      `test_docket_config`, `test_sync_agents`, `test_sync_agents_defaults`, `test_sync_agents_drift_docs`
      (and occasionally `test_harness_defaults_validator`). These are **pre-existing, machine-dependent**
      timings on files this change does not touch — the run still exits 0 (all tests pass). Change 0309's
      own package budgets were handled (see Follow-ups). The slack factor is calibrated to one machine
      (change 0229), so these breaches are expected noise on this host, not a regression.
- [ ] Awareness only: the real-git transaction tests register detached worktrees under
      `<common-dir>/docket/transactions/`. Verified on Git 2.55 / Darwin here; the plan calls for the
      Darwin+Linux matrix to confirm the same supported behavior — worth a glance if CI runs a different
      Git/OS than the dev host.

## Findings

Deep whole-branch review returned **0 blockers, 1 important, 3 minor**; all four were fixed in-branch
before the PR opened (dispositions in the PR body). Summary of the substantive one:

- **Important — cleanup leaked a git worktree registration on a transient list error.**
  `cleanupCandidate`/`worktreeRegistered` treated any `ListWorktrees` error as "not registered" and then
  removed the candidate directory directly, orphaning a `.git/worktrees/<name>` entry that
  `PruneAbandoned` could never reclaim. Fixed to distinguish a list error from a clean not-found and
  retain-on-uncertainty with a `cleanup-pending` warning (commit `5f9cacbe`, with a regression test).
- Minor: receipt-canonicality check was HTML-escaping-dependent (fixed via `SetEscapeHTML(false)`,
  `812016f0`); subpackage import-boundary now has a guard test, and the keyed no-op replay semantics are
  documented (`ba746acc`).

No new ADRs were produced — the engine's design decisions are captured in the linked spec; nothing
non-obvious arose during the build that the spec did not already record. ADR-0089's shared-worktree
retry posture is deliberately superseded by this design (as the spec's closing note states) but that
record is left untouched per the spec.

## Follow-ups

- **Test-runner shard (landed in this change, not deferred).** `internal/gitcli` already sat near the
  `test_go_race.sh` 60s hard ceiling; the transaction package's real-git fixtures pushed it over, so a
  sibling `tests/test_go_race_transaction.sh` was added running `go test -race
  ./internal/repository/transaction/`, with `test_go_race.sh` excluding that package via a
  `go list … | grep -v`-derived list and a completeness guard asserting the two files partition
  `go list ./...` exactly. `test_go_toolchain.sh` was re-budgeted 20→45s and the budgets guard's
  `EXPECTED_TOTAL` re-seeded 1965→2035. Measured margins are recorded in the plan's Task 10 notes.
- The pre-existing `OVER BUDGET` files above are unrelated to 0309 and are a candidate for a separate
  budget-curation pass on this host if they persist in CI (not blocking this change).
- Process note for the maintainer: this run's final build task (Task 10) authored its work but its
  dispatched worker yielded on a background full-suite run it could not be resumed from; the
  orchestrator certified the suite green and committed Task 10's files directly. All Task 10 content is
  present and the green gate stands.
