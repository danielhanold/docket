<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0393 — Enter compositional Docket roles as coordinator-capable Codex root threads](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0393-enter-compositional-docket-roles-as-coordinator-capable-code.md)**
<!-- docket:backlink:end -->
# Coordinator-capable Codex root entry — dogfood continuation results

Change: #0393 · Branch: fix/enter-compositional-docket-roles-as-coordinator-capable-code · PR: #265 · Original feature head: 822a9424e171b423098c4381cc3eb6f1f1c7cc91 · Plan: docs/superpowers/plans/2026-09-01-coordinator-capable-codex-root-entry.md · ADRs: 36, 59, 60, 94, 103

## Executive status

Change 0393's core mechanism works: `docket agent enter` starts the real
`docket-implement-next` contract as a coordinator-capable Codex root, and that root can launch and
consume the real plan-writer, build, and review roles. The production workflow is not yet complete,
however. The dogfood run required an explicit retry through `docket agent enter`, and the outer run
gate could not attribute the successful foreground run after it returned.

Do not merge PR #265 from its current head. Continue the same change on its existing branch, rebase
it onto current `main`, preserve both the root-entry implementation and current main's gate-context
binding, and repeat the production-shaped dogfood test described below.

## Dogfood evidence

The human ran change 0364 as a dogfood candidate using the binary built from change 0393's feature
head:

- Installed binary: `v0.9.3-621-g822a9424`.
- The first attempt used generic native `spawn_agent` dispatch. It entered
  `docket-implement-next` as an ordinary child and halted at the nested-composition boundary.
- The retry explicitly invoked `docket agent enter` and returned `agent.enter: applied`.
- The root coordinator launched and consumed the real plan writer and the build/review workers.
- Change 0364 reached `run-complete` and `implemented`.
- Dogfood PR: #266.
- Dogfood feature head: `63e23120`.

This is stronger than the disposable certification committed earlier in this branch: it proves the
new root-entry adapter can carry an actual Docket change through the complete plan/build/review/PR
workflow.

## Remaining defects and unproven boundaries

### 1. The ordinary native launch path still fails

`@docket-implement-next` or generic native `spawn_agent` launch still starts the coordinator role as
a child. Change 0393 intentionally does not grant collaboration controls to that child, so it cannot
reliably launch `docket-plan-writer`.

The supported repair is routing a coordinator-marked role through `docket agent enter`. The dogfood
run proves the explicit command, but it does not prove the complete parent-facing route. A fresh
Codex session must receive an ordinary prose request, read the generated `AGENTS.md` policy, choose
`docket agent enter` without manual correction, and pass the request and gate dispatch context into
the new root. If the reported first attempt was produced by ordinary prose rather than an explicit
human choice of native spawn, it is direct evidence that this routing remains defective; otherwise
it is the expected negative control.

### 2. The change-0393 head loses successful-run gate attribution

The outer facade was armed before the corrected root-entry run, but its keyed verdict returned
`no-attributable-claim` after `docket agent enter` completed.

At head `822a9424`, the exact cause is lifecycle timing, not a shell exit-code failure and not direct
tracking of native harness dispatches:

1. The gate records the pre-dispatch set of `in-progress` claims.
2. `docket agent enter` waits in the foreground for the whole coordinator turn.
3. The successful coordinator marks its change `implemented` before the command returns.
4. The old keyed verdict searches only the then-current `in-progress` set, so the completed change
   has disappeared from its candidate population.

The resulting `gate-done ... no-attributable-claim` is conservative about retry and did not
invalidate change 0364 or authorize a duplicate run. It is nevertheless an invalid success receipt:
the facade cannot prove which change the dispatched coordinator completed, so change 0393 has not
preserved the run-gate attribution contract.

### 3. PR #265 must be reconciled before further certification

As of 2026-09-07, PR #265 remains open at `822a9424`, while prepared `main` is
`257d56111d67bfc75c6a266b54a8758a50da26d8`. GitHub reports the PR merge state as `DIRTY`. The next
agent must rebase onto current `main`, resolve the generated-surface conflicts by intent, regenerate
embedded assets, and test the rebased head. The earlier green suite and GitHub checks certify only
the pre-rebase head.

## Current-main repair candidate already available

Do not begin by inventing a second attribution protocol. Current `main` includes change 0407 and
ADR-0111, which replace time/cardinality inference with a claim-transaction binding:

- `run.gate-before` emits both a gate key and a dispatch-context token;
- the parent copies the dispatch context into the coordinator request;
- `docket-implement-next` passes it to `change.claim --gate-context`;
- the successful claim durably binds its exact change identity to the armed gate; and
- `run.gate-verdict` resolves the keyed verdict from that proof even after the change becomes
  `implemented`.

The implementation and its mutation/concurrency evidence are recorded in
`docs/results/2026-09-07-keyed-gate-verdict-misattributes-its-verdict-to-a-concurrent-results.md`.
This machinery likely repairs the dogfood attribution failure once rebased, but that conclusion is
not yet certified across change 0393's app-server root-entry boundary. The missing proof is that the
dispatch-context token reaches the root coordinator unchanged and is used by its claim.

## Required continuation

1. Rebase the existing change-0393 feature branch onto current `origin/main`. Preserve the
   `launch: root-coordinator` inventory posture, typed Codex role contract, direct
   `codex app-server --stdio` client, public `agent.enter` operation, generated Codex routing clause,
   and current main's gate-context/continuation contract.
2. Resolve generated files from their source generators and regenerate them; do not hand-merge
   embedded bytes or golden outputs.
3. Add a red regression for the combined path: arm `run.gate-before`, carry its dispatch context
   through the root-entry request, claim using that context, complete the change, wait for
   `agent.enter` to return, then obtain a keyed `run.gate-verdict`.
4. Require the exact terminal shape `gate-done <key> run-complete <change-id>`. A
   `no-attributable-claim`, sibling id, ambiguous claim, inferred id, or prose-derived id is a test
   failure.
5. Mutation-test the bridge by removing the dispatch context at one boundary. The combined test
   must redden specifically because no valid claim binding reaches the gate.
6. In a fresh Codex process, issue an ordinary prose implement-next request. Do not manually invoke
   `docket agent enter` and do not explicitly select the registered child. Confirm that the managed
   routing policy selects root entry, the real plan writer executes, and the parent gate reports the
   exact completed change id.
7. Run the resolved full build suite from the rebased feature source, perform whole-branch review,
   publish the exact reviewed head, replace the stale build evidence, and re-establish
   `run-complete` for change 0393 before merge.

## Acceptance checklist

- [ ] Ordinary prose routing selects `docket agent enter` for `docket-implement-next` in a fresh
      Codex process without human correction.
- [ ] The entered coordinator is a root thread and launches the real `docket-plan-writer`.
- [ ] The dispatch-context token emitted by the outer gate reaches `change.claim --gate-context`.
- [ ] After the foreground root returns, the same keyed verdict reports
      `gate-done <key> run-complete <exact-change-id>`.
- [ ] No gate decision is derived from the coordinator's prose, process exit code, timestamps, or
      candidate cardinality.
- [ ] Removing the dispatch-context bridge makes the combined regression fail.
- [ ] The rebased full suite and PR checks are green at the final published head.
- [ ] `docket run verify --id 393` reports `run-complete` with no unmet conjuncts at that head.

## Non-defects and preserved evidence

- Change 0364 and PR #266 are valid successful outputs of the explicit root-entry dogfood run; do
  not undo them merely because the outer attribution receipt was wrong.
- The direct app-server transport, root-turn terminal wait, real child composition, role-contract
  sharing, and ordinary-child rejection were all proven at the original change-0393 head and should
  be preserved through the rebase.
- The earlier root-entry certification remains valid for `822a9424`; append new rebased/dogfood
  evidence rather than rewriting what that point-in-time run observed.

## Findings summary

- **Blocker:** successful foreground root-entry runs are not attributable by the outer gate at the
  current PR head.
- **Important:** automatic ordinary-prose routing into root entry remains unproven and may have
  failed in the first dogfood attempt, depending on how that attempt was initiated.
- **Reconciliation opportunity:** current main's change-0407 claim binding is the first repair to
  test after rebase and should supersede the earlier suggestion to infer newly implemented records
  or parse the root coordinator's output.
