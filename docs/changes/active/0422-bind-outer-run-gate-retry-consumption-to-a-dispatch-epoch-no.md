---
id: 422
slug: 'bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no'
title: 'Bind outer run-gate retry consumption to a dispatch epoch, not each observation'
status: 'in-progress'
priority: 'medium'
type: 'chore'
created: '2026-09-10'
updated: '2026-09-15'
depends_on: []
stacked_on:
related: [421, 425, 426, 427]
discovered_from: [421]
adrs: [111, 115, 118]
spec: 'docs/superpowers/specs/2026-09-15-bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no-design.md'
plan: 'docs/superpowers/plans/2026-09-15-bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-15T11:45:52Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-15-bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-15-bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no-design.md) |
| Plan | [2026-09-15-bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no.md](https://github.com/danielhanold/docket/blob/chore/bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no/docs/superpowers/plans/2026-09-15-bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no.md) |
| ADRs | [ADR-0111](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0111-run-gate-attribution-binds-a-dispatch-to-its-successful-clai.md), [ADR-0115](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0115-outer-run-gate-retry-budget-is-a-counted-config-snapshotted.md), [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md) |
<!-- docket:artifacts:end -->

## Why

Change 0421 made outer-run attempt limits configurable. At limits of three or more, repeatedly checking the same unfinished attempt can spend successive retry allowances even when no new dispatch occurred. The store already grants each numbered attempt only once; the verdict path causes the over-count by deriving the observed attempt number from the number of grants. The approved design fixes that input while retaining the existing ownership and dispatch-observation contracts.

## What changes

- Pass an explicit observed attempt number through the keyed verdict path, defaulting omitted input to attempt 1. Repeated checks of that attempt reuse its existing retry marker.
- Reuse the current per-attempt reservation mechanism, require evidence of the preceding grant for later attempts, and return the next attempt number only with a newly granted retry.
- Have the coordinator associate the returned number with the authorized dispatch and retain it across observations and continuations. Ownership proof, the configured attempt limit, report tokens and existing durable formats remain unchanged.
- Keep a reservation spent when response delivery or launch is uncertain; stop instead of guessing, refunding or relaunching.
- Preserve the existing trust boundary: the coordinator identifies which dispatch finished. Independent child-entry or launch certification is outside this fix. Add regressions for repeated and staggered concurrent observations and the required caller wiring.

## Out of scope

New attempt ledgers, random attempt identities, child-admission handshakes, new CLI operations, schema migrations, cancellation-reader or lock redesign, launch tracking/recovery, claim/workspace resume redesign, and exactly-once launch guarantees. Build and finalize budgets, configuration keys/defaults, report-token vocabulary, and change 0427's recovery-worktree fix remain outside this change.

## Reconcile log

### 2026-09-15

2026-09-15: Reconciled against current source. The defect is confirmed present: RunGateVerdict (internal/app/rungate_verdict.go) derives the observed attempt as `attempt := 1 + usedBefore` where usedBefore = GateRetryUsage, so repeated verdicts of one unfinished attempt at AttemptLimit >= 3 walk successive markers and spend future allowances. The existing per-attempt CAS (ConsumeGateRetry + gateRetryMarkerFor in internal/app/rungate_store.go, schema v4) is the mechanism to reuse. CLI verdict lives in internal/cli/run.go (attributed mode, one positional key); the additive result field goes on RunGateVerdictResult. No dependency or stacked base is added. Related parent-instruction changes 0425 (in-progress) and 0426 (proposed) are unmerged, so the feature branch cut from origin/main sees neither — no base conflict. Change 0421 is done; ADR-0115 gets the successor decision recorded during implementation, preserving its accepted body. Scope, out-of-scope, and acceptance criteria remain accurate; no body edits required.

## Run halted

### 2026-09-15

The autonomous implement-next run halted at build Task 5 (parent-facing instruction propagation). Tasks 1-4 — the entire functional fix and its tests — are complete and committed on the feature branch; the blocker is a guard-integrity policy decision on where the new coordinator instruction may live. A human must decide the path before this change can reach a PR.

### What is done (committed on `chore/bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no`)

- Task 1 (467621ff): `GateRetryGrantExists` predecessor-evidence probe in `internal/app/rungate_store.go`, read-only over the existing markers (legacy bare marker == attempt 1), with tests.
- Task 2 (701372ce): `RunGateVerdict` now consumes the caller-supplied attempt ordinal (new `attempt int` parameter, default 1 at the CLI), gated on the predecessor grant marker for attempt > 1, with the additive `retry_attempt` result field (n+1 on a newly won grant, absent otherwise). Both required mutations (count-derived numbering; dropped predecessor check) were proven to land and redden their named tests. ~55 call sites updated.
- Task 3 (68dac899): concurrency race regression strengthened to the supplied-number contract, including a staggered late reader, at limit 4 (so old count-derived numbering would have double-granted); passes under `-race`.
- Task 4 (a0327d35): `docket run gate-verdict --attempt <n>` CLI flag with positive-integer and mode validation (rejects `< 1`, non-integer, and `--attempt` combined with `--unattributed`); catalog signature and schema `retry_attempt` surfaced; one production-guard expectation re-baselined per its own remedy.

The final full-suite build gate has NOT been run — the build halted before it.

### The blocker (build Task 5 — contradictory requirements, human policy decision)

Task 5 propagates the corrected coordinator rule into the parent-facing instructions. The run-gate coordinator prose is not a standalone file: in `AGENTS.md` it lives inside the managed `docket:dispatch` block, emitted by `harness.CodexDispatchInterior` composing the dispatch prose with the embedded run-gate payload sourced from `cursor-rules/run-gate.md`, and held byte-identical by `TestCommittedCodexDispatchMatchesGenerator`. So editing the run-gate coordinator instruction at all necessarily grows the always-loaded `AGENTS.md` dispatch block.

`TestDispatchBlockBudget` (`internal/repoguard/budgets_test.go`) is a deliberate anti-regrowth ratchet on that always-loaded block: `dispatchBudget = 1137` (the block was already at exactly 1137 at HEAD — zero headroom) plus the structural invariant `dispatchBudget < dispatchOld`, where `dispatchOld = 1156` is a fixed recorded pre-0334 roster measurement the block must never regrow toward. The plan's additive-only wording adds ~112 words, taking the block to ~1249. Making the guard green would require raising both `dispatchBudget` and `dispatchOld` past 1249 — which defeats the ratchet whose entire purpose is to keep the always-loaded block shrinking, a guard-integrity violation the repo AGENTS.md explicitly forbids. The other escapes are equally blocked autonomously: the task's additive rule ("keep every existing token") bars compressing other dispatch-block prose (the guard's own "slim it" remedy, a substantive edit of load-bearing coordinator instructions), and "never widen a ceiling beyond the measured value" bars raising `dispatchOld` (a historical measurement, not a current one). The plan's budget-pin remedy anticipated only the skill line/word pins (0416/0410 precedent), not this dispatch-block anti-regrowth invariant.

### Options for the human (one required before resume)

1. Consciously authorize the always-loaded dispatch block to exceed its historical roster: re-baseline BOTH `dispatchBudget` and `dispatchOld` in `internal/repoguard/budgets_test.go` with a dated rationale naming change 0422 — accepting the anti-regrowth ratchet is being loosened.
2. Compress existing dispatch-block prose to fit the new sentences within a re-baselined budget that stays strictly below `dispatchOld = 1156` (only ~18 words of net growth are possible without touching `dispatchOld`) — i.e. trim ~94 words of existing always-loaded coordinator guidance, a judgment call about which load-bearing instructions to shorten.
3. Relocate the new coordinator guidance out of the always-loaded dispatch block — carry it only in `skills/docket-implement-next/SKILL.md` and `docs/concepts/run-gate.md`, leaving the `cursor-rules/run-gate.md` / `AGENTS.md` run-gate block unchanged. This is a design change to the spec's intent (the spec directs the rule into the parent-facing generated instructions) and weakens what a dispatched coordinator reading only the always-loaded surface receives.

### State left for inspection / resume

The feature worktree carries Task 5's uncommitted edits (`AGENTS.md`, `cursor-rules/run-gate.md`, `docs/concepts/run-gate.md`, `skills/docket-implement-next/SKILL.md`, and the regenerated `internal/assets/embedded/**` mirrors) — the exact additive prose the plan specified, uncommitted, so a human can inspect, adjust, and choose an option. A stray 14-byte `Fatalf` file (a shell accident, not task output) was removed. Tasks 1-4 are committed and untouched. No results artifact was authored and no PR was opened — the run stopped here.
