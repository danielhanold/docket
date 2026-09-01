---
id: 374
slug: 'separate-build-and-finalize-test-configuration'
title: 'Separate build and finalize test configuration'
status: 'in-progress'
priority: 'high'
type: 'refactor'
created: '2026-08-30'
updated: '2026-09-01'
depends_on: [370]
stacked_on:
related: [167, 316, 318, 352, 360, 370]
discovered_from: []
adrs: [63, 74, 95, 99]
spec: 'docs/superpowers/specs/2026-08-30-separate-build-and-finalize-test-configuration-design.md'
plan: 'docs/superpowers/plans/2026-08-31-separate-build-and-finalize-test-configuration.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/separate-build-and-finalize-test-configuration'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-01T02:28:25Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-30-separate-build-and-finalize-test-configuration-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-30-separate-build-and-finalize-test-configuration-design.md) |
| Plan | [2026-08-31-separate-build-and-finalize-test-configuration.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-31-separate-build-and-finalize-test-configuration.md) |
| ADRs | [ADR-0063](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0063-docket-owns-the-build-role-profile-routed-workers.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md), [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

Build and finalize currently share `finalize.test_command`, even though they certify different
workflow moments. A repository with no tests can disable finalize's gate but cannot disable the
build gate; setting the command to `true` is the only practical workaround and records a test pass
that never happened. The documented `auto` fallback is also stale: the Go gate treats it as an
empty, unavailable command, while build still refers to removed Bash-era discovery.

## What changes

- Give build and finalize independent gate and command settings, including an explicit build-off
  path with truthful skipped evidence.
- Remove runtime `auto`; local gates with no command halt with a typed setup remedy rather than
  guessing or entering repair.
- Add one deterministic test-discovery planner used by repository init, legacy migration, and a
  new `repository configure-tests` upgrade path for already-initialized repositories.
- Keep generated config human-gated: init and configure-tests leave pending unstaged edits;
  migration includes exact bytes in its existing confirmation preview.
- Split native gate/evidence/context ownership so build reads only build policy and finalize reads
  only finalize policy. Finalize may reuse green build evidence only when both head and command
  match; a skipped build never waives finalize.
- Replace the shared-command decision in ADR-0063 through the ADR workflow and update every
  maintained config, skill, bundled asset, self-hosting, documentation, and mutation-test touch
  point derived from the post-0370 tree.

## Out of scope

Runtime test discovery; silent installer/status/autonomous config edits; a universal test
orchestrator; worker-focused test selection; CI-only finalize policy, approval, merge, or repair
changes; and rewrites of historical changes, accepted ADR bodies, frozen plans/results, or legacy
fixtures.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-09-01

### 2026-09-01 — reconcile

Reconciled against current `main`/`docket` after dependency 0370 ("delete the frozen Bash facade and legacy test surface") reached `done`. Verified the spec's design premises still hold — no scope change, no obsolescence, design intact.

**Confirmed against current source (post-0370):**

- **Shared command still real.** `internal/config/schema.go` defines only `finalize.gate` and `finalize.test_command`; the sole `build.*` leaf is `build.checkpoint`. Both implementation-context (`internal/app/implementation_context.go`) and finalize-context (`internal/app/finalize_context.go`) read the single `eff.Finalize.TestCommand.Value`. Adding `build.gate`/`build.test_command` is greenfield.
- **`auto` → empty.** `internal/config/resolve.go` maps `finalize.test_command: auto` (`autoSentinel`) to the empty string; provenance preserved.
- **Gate halts on empty.** `internal/app/gate_drive.go` (`NewGateDriveService`/`Start`) sources the command only from `finalize.test_command` and refuses with `unresolved-command` when empty — no discovery/auto-detection at runtime.
- **Evidence is green-only.** `internal/evidence/record.go`+`codec.go` accept only `result: green`; there is no `skipped` value and no finalize-reuses-build predicate keyed on head+command match. A `skipped` result and the exact-head+exact-command reuse predicate are both new work.
- **No discovery / no configure-tests.** `internal/reposetup` init and migrate plans do no test discovery; there is no `repository configure-tests` command. (`internal/suiterunner/discover.go` is unrelated runtime suite discovery.)
- **0370 landed.** `scripts/run-tests.sh` is gone; `go run ./cmd/docket development test` (backed by `internal/suiterunner`) is the sole channel. The `docket-build` SKILL.md build-gate section (approx. lines 193–235) still describes the removed Bash-era `FINALIZE_TEST_COMMAND` auto-detection and per-file loop — materially stale, in-scope to rewrite.
- **ADR-0063 Decision 5** is the shared-command rule (full suite once, derived from `finalize.test_command`, "rather than a second, driftable test-command key") to be superseded by a new ADR carrying forward its unaffected build-role decisions. ADR-0074's tri-state verdict rule remains in force.

**Two documentation-surface notes surfaced for the build:** the root `.docket.yml` finalize block still carries a stale "scripts/run-tests.sh stays the frozen oracle (0369/0370)" comment (that file is deleted); this repo's own `.docket.yml` must gain an explicit `build.test_command` set to `go run ./cmd/docket development test` per the spec's rollout. Both are inside the change's stated touch points (#6 config presentation, #7 self-hosting).
