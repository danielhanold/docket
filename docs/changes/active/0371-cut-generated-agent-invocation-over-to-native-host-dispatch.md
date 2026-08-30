---
id: 371
slug: 'cut-generated-agent-invocation-over-to-native-host-dispatch'
title: 'Cut generated agent invocation over to native host dispatch'
status: 'in-progress'
priority: 'critical'
type: 'refactor'
created: '2026-08-30'
updated: '2026-08-30'
depends_on: [369]
stacked_on:
related: [311, 317, 318, 370, 366]
discovered_from: [369]
adrs: [36, 74]
spec: 'docs/superpowers/specs/2026-08-30-cut-generated-agent-invocation-over-to-native-host-dispatch-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/cut-generated-agent-invocation-over-to-native-host-dispatch'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-30T11:22:26Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-30-cut-generated-agent-invocation-over-to-native-host-dispatch-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-30-cut-generated-agent-invocation-over-to-native-host-dispatch-design.md) |
| ADRs | [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md) |
<!-- docket:artifacts:end -->

## Why

The Go v1 architecture deliberately relies on each supported harness's native named-agent dispatch rather than a Docket runner-dispatch verb. Maintained generated agents and dispatch instructions still contain Bash-era delegation assumptions, blocking the consumer cutover and facade deletion.

## What changes

- Make `internal/harness/dispatch.go` the single canonical native-dispatch policy: `dispatchPreamble` + `DispatchInterior` state that a parent invokes the exact registered same-name `docket-*` agent through the current host's native named-agent facility, forwards the request unchanged (ids and constraints included), keeps implement-next gate bracketing caller-side, and never falls back to `runner-dispatch`, a shell runner, another harness, a generic agent, or silent inline execution.
- Confirm the four `internal/harness/{claude,codex,cursor,opencode}` adapters and the parent-facing `internal/reposeed.Plan` `docket:dispatch` block all derive their routing text from that one policy (`DispatchInterior`) and render native dispatch, with deterministic, marker-safe, byte-stable output on a repeat render.
- Remove the two maintained `runner-dispatch` references from the shipped native-dispatch surface — the `runner:` cross-harness delegation section in `skills/docket-convention/references/agent-layer.md` and the `docket.sh runner-dispatch` references in `skills/docket-build/references/delegation-execution.md` — together with their embedded mirrors under `internal/assets/embedded/`, regenerating the embedded bundle deterministically (`cmd/genassets`) so the asset-drift gate stays green. Add no Go delegation/`runner-dispatch` op and no new harness.
- Strengthen tests: canonical-policy native-dispatch/exact-identity/request-forwarding coverage, per-host adapter fixtures asserting no `runner-dispatch` in maintained output and a visible missing-agent failure, fresh isolated four-host install coverage, and mutation tests that redden when `runner-dispatch` is restored, native dispatch is dropped from one host, gate-before/gate-verdict guidance is deleted, identity is weakened to generic inference, or missing registration falls back to inline/shell. Keep change 0369's stage-local guard (`tests/test_go_consumer_migration_guard.sh`) passing untouched.
- Disposition any runner-era ADR that normatively conflicts through supported ADR transactions (ADR-0036 machine-neutral dispatch and ADR-0074 gate tri-state are preserved, not rewritten); avoid a redundant new ADR when the accepted decisions already state the target architecture.
- Frozen Bash facade physical deletion (change 0370), retained lifecycle ops (change 0369, now merged), and the final global consumer seal (change 0372) stay out of scope.

## Out of scope

Lifecycle-operation migration owned by change 369; deferred auto-capture, learning-index, and terminal-publish retirement; physical Bash facade deletion; release and self-host acceptance; any new cross-harness runner or delegation verb.

## Design decisions

Host-native dispatch owns child creation, Docket's caller-side gate owns attribution and retry
authority, and the registered agent owns its workflow contract. Host status and child prose never
supersede the gate verdict. Missing native registration fails visibly with no shell, cross-harness,
generic-agent, or silent-inline fallback. Reconciliation halts if the existing shared generator seam
cannot support the four adapters as one bounded cutover.

## Reconcile log

### 2026-08-30

Reconciled against current `main` (0369 merged and archived; dependency satisfied). Verified the Go seam this change targets already exists: canonical policy in `internal/harness/dispatch.go` (`dispatchPreamble`, `DispatchInterior`, `RunGate`), the four `internal/harness/{claude,codex,cursor,opencode}` adapters, and the parent-facing generator `internal/reposeed/plan.go` (`docket:dispatch` managed block, `dispatchBlockName = "dispatch"`). Change 0334 already stripped the per-agent roster from the preamble and 0351 moved routing out of user-global files, so the policy is largely native-dispatch already; the remaining maintained `runner-dispatch` references are exactly two shipped skill files — `skills/docket-convention/references/agent-layer.md` (the `runner:` delegation section) and `skills/docket-build/references/delegation-execution.md` — plus their embedded mirrors under `internal/assets/embedded/`. The frozen Bash facade (`scripts/runner-dispatch.sh`, `scripts/runners/*`, `sync-agents.sh`) and the Bash `tests/test_runner_dispatch*.sh` surface are owned by change 0370 and stay untouched; archived changes/results/ADR bodies are point-in-time records and are not rewritten. Design holds unchanged — no fundamental invalidation; scope preserved, `## What changes` refreshed to name the concrete current-code targets. Relations unchanged (depends_on [369], related [311,317,318,370,366], discovered_from [369], adrs [36,74]).
