<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0313 — Workspaces, GitHub PR adapter, and build evidence](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0313-workspaces-github-pr-adapter-and-build-evidence.md)**
<!-- docket:backlink:end -->

# Workspaces, GitHub PR adapter, and build evidence — results
Change: #313 · Branch: feat/workspaces-github-pr-adapter-and-build-evidence · PR: (opened at close-out) · Plan: docs/superpowers/plans/2026-08-16-workspaces-github-pr-adapter-and-build-evidence.md · ADRs: 34, 66, 83, 92 (cited; none newly produced)

## Verify (human)

<!-- No genuinely-manual merge-gate check is required for this change. The live-GitHub boundary is
     deliberately deferred: the suite runs entirely against a protocol-faithful fake `gh` and local
     bare remotes, and the opt-in real-PR smoke test is 0317's release-gate scope (spec §"Live GitHub
     boundary"). -->
- [x] None required — no PR is created against a real repository by this change; the exact opt-in
      smoke request is documented for change 0317.

## Findings

- **Review (docket-review-deep): clean — 0 blocker, 0 important, 1 minor.** The deep reviewer verified
  the probe→act→verify idempotency (never a duplicate PR, never a forced remote overwrite, `unknown`
  load-bearing), the probe-error-vs-clean-absence distinction at every destructive/permissive consumer,
  ownership-proof gating before any workspace cleanup/publish, the loss-preserving evidence codec
  (no consume-to-EOF), the import/dependency boundaries, and no-shell / no-token-or-body leakage in
  `githubcli`, and confirmed the mutation-witness tests bite.
- **Minor finding — fixed in-branch (commit 1889c2d2).** Diagnostic truncation in
  `githubcli/failure.go:stderrExcerpt` and `workspace/inspect.go:boundedDetail` cut on a raw byte
  offset, which could split a multibyte UTF-8 rune and produce a subtly malformed diagnostic. Fixed by
  backing each cut off to a rune boundary (`trimToRuneBoundary`, `unicode/utf8`), with a focused test
  at each site; redaction, the byte limits, and the truncation-marker text are unchanged.

## Follow-ups

- **Pre-existing `OVER BUDGET` on six unrelated shell tests (not a 0313 regression).** The whole-suite
  gate passed green (121 files, 9402 asserts, exit 0) but flagged wall-clock `OVER BUDGET:` on
  `test_board_checks`, `test_docket_config`, `test_harness_defaults_validator`, `test_sync_agents`,
  `test_sync_agents_defaults`, `test_sync_agents_drift_docs` — none of which this change touches. The
  runner's own advisory notes the slack factor is calibrated to one machine (change 0229); this host
  reads ~5s above that calibration. Not gated (advisory only) and out of scope to fix by editing
  unrelated files; overlaps existing budget-re-seed work (changes 0251/0273).
- **Plan deviations, resolved against the authoritative spec (not scope changes):**
  - Task 1 — `git ls-remote --exact` does not exist (git 2.55); `ProbeRemoteBranch` was implemented
    per the plan's own Step-3 contract (byte-equal refname filtering). `PushCreateLease` deliberately
    does not call `IsAncestor`: for a genuine create the remote can never contain the local commit, so
    classification is purely the exact-remote probe (found==pushed → applied, found!=pushed →
    lease-lost, absent/unprobeable → failed); the fast-forward/ancestor branch is owned by Task 8's
    `PublishHead` via the expected-old lease.
  - Task 5/6 — the `allocating` manifest is published **after** the base fetch (its `validateManifest`
    requires a valid `BaseCommit`); the op lock is still acquired first and nothing is written until the
    all-absent inventory passes.
  - Task 12 — a real budget breach: the combined main race shard **with** the workspace matrix read 62s
    (over its 60s row) under `-race`, resolved by sharding out `tests/test_go_race_workspace.sh`
    (`go test -race ./internal/workspace/`, row 45) with a three-shard `go list`-derived completeness
    guard; `runtime-budgets.tsv` `EXPECTED_TOTAL` 2035 → 2080.
- **Live GitHub acceptance** remains 0317's release-program scope; this change ships the deterministic
  test seam and documents the opt-in smoke request only.
