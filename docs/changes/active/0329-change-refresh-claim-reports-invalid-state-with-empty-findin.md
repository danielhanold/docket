---
id: 329
slug: 'change-refresh-claim-reports-invalid-state-with-empty-findin'
title: 'change refresh-claim reports invalid-state with empty findings — the transaction Failure is dropped on the DispositionFailed path'
status: 'in-progress'
priority: 'high'
type: 'fix'
created: '2026-08-19'
updated: '2026-08-19'
depends_on: []
stacked_on:
related: [315]
discovered_from: [316]
adrs: []
spec: docs/superpowers/specs/2026-08-19-failed-transaction-diagnostics-design.md
plan: 'docs/superpowers/plans/2026-08-19-failed-transaction-diagnostics.md'
results:
trivial: false
auto_groomable:
branch: 'feat/change-refresh-claim-reports-invalid-state-with-empty-findin'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-19T18:58:34Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | `docs/superpowers/specs/2026-08-19-failed-transaction-diagnostics-design.md` |
| Plan | `docs/superpowers/plans/2026-08-19-failed-transaction-diagnostics.md` |
<!-- docket:artifacts:end -->

## Why

An autonomous docket-implement-next run on change 0316 called `docket change refresh-claim` during its claim/reconcile phase and got back a deterministic, undiagnosable refusal: `result: invalid-state`, `disposition: invalid-state`, `findings: []`. Nothing in the result says why. The run had to abandon explicit refresh-claim entirely and lean on reconcile's own internal claim-refresh to keep the lease fresh, which happened to cover it — so the defect did not block that run, but it silently removed a lease-management operation the skills are supposed to be able to call.

The cause is a diagnostic-propagation gap, not a bad transition. On the `DispositionFailed` path the diagnosis lives in the typed `transaction.Failure` carried by the returned `error`, while `claimResultFromOutcome` builds its result exclusively from `transaction.Result`:

- `mapOutcome` (internal/app/planning.go) routes `DispositionFailed` through `mapFailure(err)`, which reads the Failure's `Kind` — so `KindInvalidState` correctly becomes `ResultInvalidState`. That much works.
- `claimResultFromOutcome` (internal/app/change_claim.go) then sets `Findings: findingsToStatus(res.Findings)`. On a *failed* transaction — as opposed to a *refused* one — `res.Findings` is empty, because findings are the refusal channel. The Failure's own message is never consulted.
- `claimDisposition` switches on `res.Disposition`. `DispositionFailed` matches no case, so it falls to `default:`, and with `replayed == false` it returns `string(result)` — restating "invalid-state" as the disposition rather than naming a cause.

So every field that could carry the reason is either empty or a tautology, and the operator sees a failure with no cause. The information exists in `execErr` and is thrown away at the boundary.

This is a general shape, not a refresh-claim quirk: `claimResultFromOutcome` serves `change claim` as well, and the `mapOutcome`/`mapFailure` pattern is shared across the app layer. Any operation that funnels a `DispositionFailed` into a result envelope built only from `transaction.Result` will redact its own failure the same way. Scope the fix by grepping the shared helpers, not by patching the one verb where it surfaced (AGENTS.md: never hand-list the sites of an operation you are gating — derive them from a whole-repo grep).

Separately, the trigger itself is still unexplained and worth pinning down as part of the fix: `domain.RefreshClaim` only guards `StatusInProgress`, and change 0316 *was* in-progress when the calls failed, so the Failure originated below the domain guard — plausibly the exact-version blob expectation or a loader/validation path. A repro is the first task; the run that found this could not investigate without mutating a live claim.

## What changes

Propagate the typed `transaction.Failure` into the result envelope on the `DispositionFailed` path so a failed operation names its own cause. Settled design (see spec): a new, **additive `failure` field** on the protocol-v1 result envelope — `{stage, kind, detail}`, present only on failed dispositions — produced by one shared app-layer helper; findings remain the refusal channel and are never overloaded. `claimDisposition` and every sibling mapper with the same `default:` fall-through get a real `DispositionFailed` arm instead of echoing the result string back as the disposition.

Scope is **every affected builder**, not just claim/refresh-claim: derive the full set from a grep over the shared helpers (`mapOutcome`, `mapFailure`, and the `*ResultFromOutcome` family), sort into surfacing vs redacting, and wire each redacting one to the shared helper.

First task is a **time-boxed reproduction** (not a ship gate): establish what makes `refresh-claim` fail deterministically against an in-progress change at a valid version — prime suspect is the engine's verify-delta invalid-state on a concurrently-dirtied shared `.docket` worktree. Build it as a hermetic fixture — do not reproduce against a live claimed change. Not reproduced within the box → record that and ship the propagation anyway.

Add regression tests asserting that a failed transaction surfaces a non-empty, non-tautological cause: the assert must fail if the Failure detail is dropped again. Per AGENTS.md, mutation-test the guard — strip the propagation, watch it redden — or it is decoration.

## Out of scope

Changing the claim/refresh-claim lease semantics themselves, the reclaim TTL policy, or reconcile's internal claim-refresh (which is working and covered the gap). Not a redesign of the protocol-v1 envelope or the finding vocabulary. Not the unrelated Step-5 gate failures on change 0316's branch.

## Reconcile log

### 2026-08-19

2026-08-19: Reconciled against current `main`. The spec's code sites are all present and unchanged: `mapOutcome`/`mapFailure` at internal/app/planning.go:250/274, `claimResultFromOutcome`/`claimDisposition` at internal/app/change_claim.go:354/374, and the `*ResultFromOutcome` builder family across change_claim/kill/groom/halt/reclaim/reconcile/lifecycle/implemented/attach/create + finalize_block. No `failure` envelope field or shared `failureStatus` helper exists yet, so the diagnostic-propagation gap the change targets is still live. Related change 315 (claim-to-implemented workflow) and the discovered-from change 316 (finalize-recovery) are both archived (done), so the workflow surface this fix touches has landed and is stable. Scope, design, and task shape hold as written — no adjustment needed. Build-time grep remains authoritative for the affected-builder set per the spec.

## Run halted

Halted 2026-08-19 by an autonomous docket-implement-next run at the Step-5 build gate.

**What the run completed.** The change is fully built on branch `feat/change-refresh-claim-reports-invalid-state-with-empty-findin` (five commits: plan `b247c87d`; repro spike `efd94991`; envelope field + `failureStatus` helper `cece78f5`; claim/refresh-claim wiring + real `failed` disposition `065881ee`; grep-derived sweep of every remaining builder + spike payoff `d9f5ff33`). Every task ran TDD with mutation-tested guards. The Task-1 spike **reproduced** the 0316 defect: a plain `refresh-claim` re-renders the inline board byte-identical while its plan declares that board path, so `verifyActualDelta` fails `verify-delta`/`invalid-state`, and the pre-fix mappers discarded that typed Failure into `invalid-state`/`invalid-state`/`[]`. The fix propagates it. In this very run, explicit `docket change refresh-claim` returned exactly that undiagnosable `invalid-state` — the bug this change fixes — so the lease was carried by reconcile's/attach's internal re-stamps instead.

**Why halted (needs a human).** The full-suite build gate (`scripts/run-tests.sh`, the fixed `finalize.test_command`) exits red on this machine, but the redness is **pre-existing infrastructure flakiness, not caused by this change**, proven three ways:
- The **base commit `5e5f7dfd` (origin/main), untouched by this change, fails the byte-identical 20-file set** under the full parallel suite (diff of the two failing sets: identical). test_adr_checks fails notok=10 in 1s on base too — a real, fast assertion failure, not a timeout.
- **All 20 failing files pass in isolation on the feature branch** (18 shell files: 2966 asserts, 0 failures; test_go_toolchain + test_go_race: green). The change's own package `internal/app` is green (`go test ./internal/app/ -count=1` ok).
- The failures are a **parallelism/resource-contention artifact**: at full `-j` on this box the race-detector Go tests blow their ceilings massively (test_go_race 60s→622s, test_sync_agents 50s→644s), starving the shell tests that shell out to git/`docket` into fast assertion failures. This is the "unrelated Step-5 gate failures on change 0316's branch" the Out-of-scope section already names.

Because only a green gate mints build evidence, and the gate command is fixed (no hand-rolled subset is permitted), the run cannot mint trustworthy evidence and cannot open the PR. There is **nothing in this change to repair** — a repair worker would rediscover the identical base-commit failures and halt.

**What a human must decide.** Run the fixed gate on capable infrastructure (CI, or a quiet box, or with `-j` tuned so the race-detector Go tests don't starve the suite) to obtain a genuine green, then resume `docket-implement-next` for change 0329 (branch already built; pass the id and note the build is complete through Step 5). Separately, the pre-existing full-`-j` suite flakiness on this machine is worth its own change — it makes the build gate unusable here regardless of the change under test.
