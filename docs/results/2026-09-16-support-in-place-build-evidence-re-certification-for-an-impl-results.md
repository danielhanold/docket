<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0415 — Support in-place build-evidence re-certification for an implemented change](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-16-0415-support-in-place-build-evidence-re-certification-for-an-impl.md)**
<!-- docket:backlink:end -->
# Support in-place build-evidence re-certification for an implemented change — Results

## Outcome

Added a supported `docket evidence recertify --id <id> [--repo-dir <dir>]` command (operation
`evidence.recertify`, external-write). For an implemented change whose open PR received a
**published** follow-up commit, it reruns the configured **build** gate at the current feature
head, records and verifies canonical build evidence, and refreshes only the existing PR's
build-evidence block — leaving the change `implemented` throughout. It performs no docket metadata
mutation, no branch/commit/push/rebase/merge, no results rewrite, and no automatic code repair.

The implementation is an app-layer composition (`internal/app/evidence_recertify.go`) reusing
landed services:

- Identity/preconditions via `loadWorkspaceContext` / `resolveWorkspaceTarget` / `WorkspaceInspect`,
  consolidated into one reusable `recertifyProbe` predicate run both before the gate and again as
  the pre-publish recheck (per the `duplicated-gate-copies-the-whole-predicate` learning).
- The production local-gate seam `processFinalizeGate` was generalized with an `owner` field, and a
  new `NewBuildLocalGate` constructor reads **only** `build.test_command` through
  `NewBuildGateDriveService` (ADR-0102: build and finalize own independent gate/test commands). The
  finalize owner (and the zero-value default) still read `finalize.test_command` — finalize behavior
  is unchanged.
- Evidence via the build-owned `EvidenceRecord` path (`build.gate: off` mints a truthful `skipped`
  record and does not edit the PR block, since `evidence.Upsert` is green-only by design; an enabled
  local gate with no `build.test_command` refuses `unsupported-config`).
- The PR evidence-block edit via finalize publish's `evidence.Upsert` + `EnsurePullRequest`
  (`ExpectedHead`/`ExpectedVersion`), without any rebase receipt, branch push, rebase, or merge.

Exposed through a CLI leaf in `internal/cli/evidence.go`, deps wiring in `internal/cli/finalize.go`
(`newFinalizeDepsGated` / `newRecertifyDeps`), and a sorted schema binding in
`internal/app/schema_registry.go`. Documented in `docs/guide/proving-the-build.md` and
`docs/guide/reviewing-before-the-human.md`.

No material departures from the spec. Two behaviors were confirmed against the code and are called
out because they were implicit in the spec: `build.gate: off` completes as `skipped` without a PR
edit (green-only `evidence.Upsert`), and a recertify charges one attempt against the change-lifetime
`build.max_attempts` suite budget (key `repo + change + "build"`).

## Verification performed

- Full configured build suite (`go run ./cmd/docket development test`) driven to a PASSED terminal
  through the native build-owned gate driver: `SUITE files=49 passed=49 failed=0 asserts=411`.
  Build-evidence recorded and verified green at the certified head. (An earlier full-suite run went
  red on exactly one check — `gofmt` reported `internal/cli/install.go` unformatted; `go vet` and
  `go test ./...` were green. Fixed by an integration-repair task (`gofmt -w`) and re-certified.)
- TDD throughout: each build task established RED before GREEN; guards were mutation-tested
  (`-count=1` to defeat the Go result cache). Notably, the moved-head publication hazard is guarded
  three independent ways (the `recertifyProbe` remote-head check, its `pr.HeadCommit` check, and the
  post-gate `second.head == first.head` recheck); no subset-of-two mutation reddens the moved-head
  test — this is deliberate defense-in-depth, verified via mutation, not unprobed residual.
- A deep whole-branch review ran over the green branch (0 blockers, 1 important, 2 minor). The two
  minor findings were fixed in-branch (see below); the important finding is reported as follow-up.
- Test coverage confirmed for the refusal and disposition matrix: not-implemented, dirty workspace
  (pre- and post-gate), unpublished/unpushed follow-up, remote/PR head disagreement,
  closed/mismatched PR, invalid input, gate FAILED/HALTED/seam-error/WAITING-without-continuation,
  gate-off skipped, unconfigured build gate, foreign-command and wrong-head evidence rejection, and
  PR-edit unknown/contended/unrecognized dispositions — none of which publish successful evidence.
- The config-resolution nuance was verified: docket resolves `.docket.yml` from the origin
  default-branch tip, not the invocation working tree; the two config-overlay tests were corrected
  to publish config via the fixture's `writerAdvance` helper accordingly.

## Findings and limitations

### Post-gate cleanliness requires a clean-leaving build command

The pre-publish recheck reuses the whole `recertifyProbe` predicate, which requires a clean
(`StateReady`) workspace. A `build.test_command` that leaves an untracked, non-gitignored file in
the feature worktree during the gate flips the post-gate recheck to `workspace-dirty` and blocks
publishing an otherwise-green run (gitignored artifacts are unaffected). This mirrors the existing
gate-in-worktree model; the requirement is now documented in the recertify guide and pinned by a
test. It is deliberate — the whole-predicate reuse was not narrowed.

## Follow-ups

### Build-budget exhaustion during recertify surfaces as an opaque halt (review Finding 1, important)

Each recertify gate run charges one attempt against the change's shared `build.max_attempts` budget,
and every recoverable post-gate refusal (an `EnsureUnknown`/`EnsureContended` PR-edit outcome, or an
identity-drift-by-race) forces a fresh gate run — and another budget charge — on the next
invocation. On exhaustion, the `gate.drive.start` refusal carries `suite-attempts-exhausted`, but
`mapDriveOutcome` collapses it (with `out.Drive == nil`) to a generic `GateHaltUnavailable`, which
`runRecertifyGate` reports as the opaque `ReasonRecertifyGateHalted` — the operator sees neither the
real cause nor a remedy, and re-running fails identically. It fails closed (no bad evidence is ever
published), so this is an operability/diagnostic gap, not a correctness defect. A proper fix requires
threading the `suite-attempts-exhausted` reason through the **shared** `mapDriveOutcome` /
`FinalizeGate` seam (which also serves finalize, and is pre-existing to this change), and/or a
spec-level decision on whether a retry whose gate already passed should re-charge a full attempt —
both beyond this change's scope (the spec fixes "one suite attempt per invocation" and adds no retry
framework). Suggested next action: capture a change to (a) surface the exhaustion reason as a
distinct recertify outcome and (b) reconsider budget accounting for a passed-gate/failed-edit retry.
