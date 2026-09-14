<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0428 — Recover legacy gate history without blocking unrelated worktree admission](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0428-recover-legacy-gate-history-without-blocking-unrelated-workt.md)**
<!-- docket:backlink:end -->
# Recover legacy gate history without blocking unrelated worktree admission — Results

## Outcome

First-admission worktree gate starts no longer fail closed in a repository whose Git common
directory carries completed pre-0375 schema-2 gate-drive history. The admission inventory now
delegates to a shared legacy-history classifier that recognizes trustworthy completed (`PASSED`/
`FAILED`) drives *before* resolving a possibly-removed historical worktree path, reuses the existing
process recovery predicate for `HALTED` history, and never treats unknown or unprovable state as
safely inactive. A start that must still refuse carries the exact `inventory-legacy-drive-<id>`
locator (or the safe inventory-level `inventory-legacy-drives` when there is no valid id) and a
compact recovery summary, and the application refusal now names `docket gate history cleanup` as the
inspection/recovery route instead of the misleading "a prior execution in this worktree is
unresolved" message. The same classifier backs a new idempotent `docket gate history cleanup`
command (with `--drive-id` selector and `--dry-run`).

Behavioral shape delivered, task by task:

- A single process liveness/teardown predicate split into assess (`mark=false`, never writes) and
  apply (`mark=true`) modes — `process.Service.ClassifyRun` — so the gate layer reuses it rather
  than reimplementing liveness.
- A historical schema-2 reader (`loadHistoricalDrive`) for admission assessment only; the executable
  reader's schema policy (v3/v4) is unchanged, and schema 2 is never loaded for execution, migrated,
  or rewritten.
- A shared classifier (`classifyLegacyDrive`) with load-bearing terminal-before-path ordering and a
  probe-error-retains rule (a probe error or missing evidence retains; it never recovers).
- The admission inventory rewired through the classifier; `OwnershipError` carries a bounded
  `LegacyHistorySummary`; `admissionObservationProvesTeardown` deleted (a bare Observe `vanished` no
  longer certifies teardown), with the release-on-stop leg preserved via an extracted
  `stopProvesTeardown`.
- Start documents and application refusals carry the recovery summary; a typed refusal stage and
  safe locator flow through the app layer's result and human text.
- `Driver.CleanupHistory`, the `gate.history.cleanup` app operation with a redaction-bounded result
  document, and the `docket gate history cleanup` CLI leaf with capability registration.

No product-scope departures from the approved spec. Deviations were confined to keeping guards
non-decorative and are recorded under Findings.

## Verification performed

- Each of the ten plan tasks was built test-first through the native gate driver (task-owner
  drives), one commit per task; every task's focused package tests passed, and each new guard was
  mutation-tested (mutate with a backup copy, confirm the named assertion reddens, restore, re-confirm
  green): assess-mode-never-marks (Task 1), v2 required-field validation (Task 2), terminal-before-path
  ordering and probe-error-retains (Task 3), and locator id validation (Task 6).
- End-to-end acceptance (Task 10) exercised the real driver + real process supervisor for spec
  criterion 1 (a legacy v2 `PASSED`-over-removed-worktree start reaches launch with no prior cleanup
  and no second start; exactly one run dir; one suite attempt used), criterion 5 (two concurrent
  starts over a legacy-seeded store admit exactly one launch, the loser refused, and a concurrent
  `CleanupHistory` neither deadlocks nor mutates any seeded record — run under `-race`), and criterion
  7 negative space (`FAILED` and post-launch `HALTED` outcomes launch once and consult the
  legacy-recovery seam zero times; a busy-slot non-inventory refusal carries no stage/locator).
- The whole suite was run at the build gate from the source checkout via `go run ./cmd/docket
  development test`; see the build-evidence block in the PR body for the certified head and result.

## Findings and limitations

### Guard-mutation adjustment to the terminal-before-path acceptance case

The plan's terminal-before-path mutation case named the `missing-worktree` fixture, but a pure
step-swap does not redden that setup (an unresolvable path skips the path block, so the terminal
check still fires). The classifier task used a `PASSED` record bound to a resolvable-but-different
worktree to make the step-swap genuinely reddening, and preserved the durable-evidence-survives-
worktree-removal property as an added subtest. The guard is load-bearing and mutation-proven.

### Second caller of the deleted teardown helper

`admissionObservationProvesTeardown` had a second caller beyond the one the plan named
(`releaseAdmissionIfProven`'s HALTED release-on-stop leg from change 0375). It was deleted as the
plan mandated and that leg's exact proven-teardown state set was preserved by extracting
`stopProvesTeardown`.

### Cross-package fixture coupling in app-layer tests

The new `internal/app` tests for `gate.history.cleanup` and the end-to-end acceptance read the frozen
`internal/gatedrive/testdata/legacy-v2` fixtures via a relative path (`../gatedrive/testdata/legacy-v2`).
This is the first app-package test to read gatedrive fixtures cross-package. It is robust at the test
run cwd but is a deliberate coupling worth knowing about if either package's test layout moves.

### Catalog byte budget

Registering the new CLI leaf grew the capability catalog from 14733 to 14973 bytes against a 15360-byte
(15 KB) ceiling — roughly 387 bytes of headroom remain. No ceiling change was needed here, but the
next few cataloged commands will approach it.

## Follow-ups

### In-repo skills guidance mention of `docket gate history cleanup`

The Task 10 guidance greps found that the maintained `unresolved-execution` mentions in the in-repo
`skills/` texts (`docket-build-task`, `docket-build`, `docket-finalize-change`) all describe the
**live-slot** unresolved-execution refusal (a run that ended without proven teardown) — a different,
still-correct path from change 428's new **legacy-inventory** refusal. Editing them to name
`gate history cleanup` would conflate the two paths, so no edit was made. If surfacing
`docket gate history cleanup` alongside those live-slot descriptions is nonetheless wanted as an
additive cross-reference, that is a separate documentation follow-up outside this change's scope.
