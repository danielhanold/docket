<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0411 — Steer post-completion durable-write failures to rebase-continue, not abort](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-18-0411-steer-post-completion-durable-write-failures-to-rebase-conti.md)**
<!-- docket:backlink:end -->
# Steer post-completion durable-write failures to rebase-continue, not abort — Results

## Outcome

Reworded the three reservation-reconciliation write-failure diagnostics in
`internal/app/finalize_rebase.go` so an operator whose owned resolver continuation succeeded in Git
but failed only the durable reservation-reconciliation write is steered to recover via
`finalize.rebase-continue` (same change id, owned attempt, and original resolved report) rather than
defaulting to `finalize.rebase-abort`, which would discard the completed local rewrite. A shared
`reconcileRecoveryRemedy` const carries the recovery tail; each site keeps its
`ResultExternalFailed` / `RebaseDispBlocked` / `ReasonRebaseReceiptWrite` result and its
`withResolverCounts` receipt argument unchanged. Wording is site-accurate: the immediate
post-continue site does not assert the whole rebase finished ("another conflict may remain"); the
advanced-conflict recovery branch says "advanced to another conflict"; the completed-rebase recovery
branch says "the owned rebase completed."

A matching harness-neutral operator exception was added to `skills/docket-finalize-change/SKILL.md`
(inside the resolver loop) and fully explained in `references/gate-failure.md` (a new
"reconciliation-write exception" section plus an abort-set carve-out), guarded by section-bound,
whitespace-collapsed prose contracts in `internal/repoguard/prose_contracts_test.go` with the two
budget ceilings bumped to the new actuals. Embedded skill assets were regenerated from maintained
source. This is a diagnostics-and-documentation change: no rebase state machine, reservation
admission/budget/refund, ownership, gate/evidence, merge, or generated-bundle behavior changed;
`internal/app/finalize_reserve.go` remained an unchanged negative control.

## Verification performed

- TDD in `internal/app/finalize_rebase_test.go`: a `reconcileFailWorkspace` fault-injection wrapper
  and five tests (A–E) covering spec acceptance criteria 1–4 — a post-continue reconcile-write
  failure preserving reservation/started-marker/used count and gating the suite until reconciliation
  succeeds; the completed-rebase recovery branch's failed-then-retried write; the advanced-conflict
  branch (both the immediate post-continue failure and the recovery-after-advance failure), each
  asserting the message never claims whole-rebase completion; and the AC4 negative boundary proving
  a pre-Git marker-write failure carries no recovery remedy. Each assert pins the mechanism (remedy
  operation name, same-attempt/report reuse phrase, underlying error detail), not merely "it failed."
- Prose guards mutation-tested: deleting the new reference section reddens, swapping the remedy
  operation to `finalize.rebase-abort` reddens, and re-wrapping a guarded clause across a newline
  stays green (reflow tolerance via `collapseWS`). A review-fix added a non-vacuity population floor
  to `TestRebaseRecoveryDocContracts`, mutation-confirmed (emptying the contract table reddens on the
  floor).
- Embedded-asset drift guards accept the regenerated bundle; `internal/assets`, `internal/app`, and
  `internal/repoguard` focused suites pass.
- Full build gate `go run ./cmd/docket development test` driven through the native gate driver:
  PASSED (green) at the final head. Build-evidence recorded and verified.

## Findings and limitations

### Environmental parallel-load timeout in the full suite (not a defect in this change)

Two full-suite gate runs during this change went red with identical `panic: test timed out after
10m0s` failures in `internal/app` integration buckets (`app_change`, `app_closeout`, `app_rebase`,
`app_workflow`) plus the `test_go_race` / `test_go_toolchain` sub-checks that re-run the module.
This is the known environmental blocker: under the suite's `-j11` parallelism, `internal/app`
integration binaries contend for CPU and blow Go's default 10m per-package timeout when the machine
is loaded (suite wall ballooned to ~742s vs ~300s on the passing runs). It was serial-confirmed
clean: `go test -tags integration -p 1 -timeout 30m -run '^TestIntegrationFinalizeRebase'
./internal/app` passed in ~66s, and a subsequent parallel gate re-run passed green. No code defect;
no gofmt failure remained. The real remedy (raise the per-package timeout, shard, or reduce
parallelism) is out of scope here and is a separate infrastructure change.

## Follow-ups

### Pre-existing gofmt drift in `internal/githubcli/comment_integration_test.go` (fixed as collateral)

The whole-repo gofmt gate was red on two files: this change's own new test file (fixed) and
`internal/githubcli/comment_integration_test.go`, which was already gofmt-unformatted on base main
(`3ccf9fac`) and is not otherwise part of this change. It was `gofmt -w`-formatted as a minimal,
whitespace-only collateral fix to green the whole-repo gate (`git diff --ignore-all-space` empty).
No follow-up action is required for that file, but the drift indicates a gap in whatever landed it
unformatted; a human may wish to confirm no other pre-existing gofmt drift remains on main.
