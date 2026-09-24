---
id: 422
slug: 'bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no'
title: 'Bind outer run-gate retry consumption to a dispatch epoch, not each observation'
status: 'blocked'
priority: 'medium'
type: 'chore'
created: '2026-09-10'
updated: '2026-09-24'
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
blocked_by: 'Halted at build Task 5 pending a human decision among 3 feasible paths for the AGENTS.md dispatch-budget overage (trim in-block coordinator prose, re-baseline dispatchBudget, or relocate guidance) — see the run-halted record on the change.'
reconciled: true
claimed_at: '2026-09-15T12:55:21Z'
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

The resume reached build Task 5 with the human-supplied revised approach — compress the four hand-authored `AGENTS.md` sections (Shell; Frontmatter and generated blocks; Guards and tests; Comments and cross-references) to free budget for the additive `--attempt`/`retry_attempt` coordinator prose. That approach cannot work as specified: those four sections lie OUTSIDE the block `TestDispatchBlockBudget` measures, so trimming them is budget-neutral. A fresh human decision is required.

### What was verified

- `TestDispatchBlockBudget` (`internal/repoguard/budgets_test.go`) counts ONLY the words between the `docket:dispatch:start` / `docket:dispatch:end` markers in `AGENTS.md` (currently lines 9–108). With Task 5's additive prose applied, that block is 1249 words against `dispatchBudget = 1137` — overage 112. Test reruns red at exactly that count.
- The four named sections begin at lines 133 (Shell), 150 (Frontmatter and generated blocks), 162 (Guards and tests), and 183 (Comments and cross-references) — all AFTER the end marker at line 109 (and after `Rebuild the binary after a merge to main` at line 111). They total ~640 words but contribute nothing to `dispatchBudget`; removing every one of them leaves the measured block at 1249 and the guard still red.
- The block interior is generated, not hand-authored: `harness.DispatchInterior` / `CodexDispatchInterior` (`internal/harness/dispatch.go`) compose `dispatchPreamble` + the `cursor-rules/run-gate.md` payload + `CodexRootEntryClause`, and the committed `AGENTS.md` block is held byte-identical to that generator (`internal/harness/native_dispatch_test.go`). So the only content that counts toward `dispatchBudget` is the generated coordinator instructions themselves — editable only via those `dispatch.go` constants and `cursor-rules/run-gate.md`, never by hand-trimming `AGENTS.md`.

### Why this is a fresh human decision, not autonomous work

The revised decision carries three constraints that cannot all hold: (1) do not re-baseline `dispatchBudget`/`dispatchOld`; (2) do not relocate the new guidance out of the always-loaded block; (3) do not shorten the load-bearing coordinator instructions (the stated reason for redirecting to the four sections). The baseline block was already at exactly 1137 (zero headroom), so any net additive words MUST be offset by removing words from INSIDE the marker block — i.e. from the generated coordinator instructions (violating 3), because the additive prose cannot net negative. There is no budget-relevant hand-authored prose available to trim. The decision rests on the premise that the four sections sit inside the managed dispatch block; they do not.

### Feasible paths (one required before resume)

1. Apply the decision's own tightening criterion ("tighten wording, cut redundancy, no loss of the load-bearing rule") to the IN-BLOCK generated coordinator prose — `cursor-rules/run-gate.md`, and if needed `dispatchPreamble` / `CodexRootEntryClause` in `internal/harness/dispatch.go` — trimming ~112 words to offset the additive `--attempt` prose. Honors constraints 1 and 2; relaxes 3. This is the "judgment call about which load-bearing instructions to shorten" the original halt flagged, applied to the coordinator prose the revised decision was trying to leave untouched. Best matches the decision's ship-without-re-baseline-or-relocate intent, but needs sign-off that shortening always-loaded coordinator instructions is acceptable.
2. Re-baseline `dispatchBudget` and `dispatchOld` past ~1249 with a dated 0422 rationale (original halt option 1). Relaxes constraint 1 — loosens the anti-regrowth ratchet the decision wished to preserve.
3. Relocate the new coordinator guidance into `skills/docket-implement-next/SKILL.md` + `docs/concepts/run-gate.md` only (original halt option 3). Relaxes constraint 2 — a coordinator reading only the always-loaded surface would not see the `--attempt` requirement.

### State left for inspection / resume

Tasks 1–4 remain committed and untouched on `chore/bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no` (tip a0327d35): the entire functional fix and its tests. Task 5's additive edits are still uncommitted in the feature worktree (`AGENTS.md`, `cursor-rules/run-gate.md`, `docs/concepts/run-gate.md`, `skills/docket-implement-next/SKILL.md`, and the regenerated `internal/assets/embedded/**` mirrors) — the exact additive prose the plan specified, ready for a human to inspect and adjust. No results artifact was authored and no PR was opened. The final full-suite build gate has not been run.
