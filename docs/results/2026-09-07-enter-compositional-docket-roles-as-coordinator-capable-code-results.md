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


## Inline continuation — 2026-09-08

The human explicitly authorized inline continuation because the workflow being repaired could
not safely bootstrap its own coordinator. The branch now contains integration base
`b801b8c6dd23f55b23d1edecc373039e04ffe96c`, including change 0407's claim-transaction binding.
The earlier observations above are historical; this section records the rebased continuation.

### Repairs and review dispositions

- Restored the agent command, capability registration and result schema lost during reconciliation.
  Required flags now have required, semantically named capability signatures.
- Made the generated Codex clause explicitly take precedence over general native-child wording.
  It preserves the user's request, dispatch context, resume/continuation ids and gate key, and
  requires the parent's keyed verdict after the foreground return.
- Regenerated embedded assets and this repository's committed managed dispatch block from their
  Go sources. A red-before-regeneration repository guard now detects stale committed policy.
  The dispatch word budget changed from 400 to 650 for the 636-word generated policy; no runtime
  budget changed. The policy remains smaller than the retired 1,156-word roster.
- Added installed-contract validation using the same Codex planner as installation. Missing or
  edited role TOML and stale skill preloads refuse before process launch. Mutation tests remove
  or edit those inputs and establish the distinct `role-contract-unavailable` result.
- Server requests needing interactive approval/input now fail explicitly instead of hanging.
  Root output selects the actual final-answer phase; explicit commentary cannot masquerade as
  the final result. Scripted tests cover both behaviors, including red-before-fix observations.
- Whole-branch inline review found and repaired the above issues. The initial full source suite
  passed 40 of 42 files; the two failing files reported capability-literal and bare temporary-dir
  guards in the new changes. Those guards were repaired, and the focused packages passed.
  Final publication is conditional on a new full source suite and evidence for the final head.

### Claim-binding regression and mutation

`TestIntegrationWorkflowRootEntryGateAttribution` crosses the real root-entry client with only
its model/server transport scripted. The request received at `turn/start` drives the existing
real-Git claim-to-implemented workflow, real evidence/publication adapters with a stateful local
GitHub fixture, and the real claim-proof scanner. The parent asks for its keyed verdict only
after root entry closes. Deliberately misleading output says sibling change 999 completed;
it cannot influence attribution.

The preserved-context case requires exactly `gate-done <key> run-complete 3`. The removed-context
case completes the same change but requires `gate-done <key> no-attributable-claim`. Both passed
on the reviewed branch. A separate mutation of production `turn/start` request forwarding
removed the context: the positive case failed specifically because its actual verdict was
`no-attributable-claim` instead of `run-complete 3`. Restoring the production bridge returned
the test to green.

### Live certification scope and binary provenance

The ordinary-prose test used an isolated Codex home with the user's configured model/effort pins,
the actual installed Docket roles, a real local Git origin, and a stateful local GitHub fixture.
No production backlog or GitHub PR was used for the synthetic sentinel change. A fresh generic
Codex parent received exactly `Please implement change 1.`; its request did not prescribe
root entry or explicitly select a child. The installed binary was built from the rebased
change-393 source corresponding to repair commit `615f5fdede07da193b54b95959a9a9e6a73bc092`
(SHA-256 `37f1dd641f22032955dc9351cc7ffd6515f0baa064a383c3a2340465c3d08d9d`).

The parent automatically selected the catalog-resolved root-entry operation and carried its
gate context into the actual claim. The following identities are diagnostic trace pointers;
the terminal keyed gate proof, not these ids or agent prose, establishes attribution:

- Fresh parent: `01a07ec4-145b-72a0-9434-3dfb9e94f9b8`.
- Entered root: `01a07ec4-aa94-7bd0-bcb4-4b62fcef6491`.
- Registered plan writer: `01a07ec9-8986-7320-b35d-fc2c71e51c2d`, a depth-1 child of that root.
  It returned `PLAN_PATH=docs/superpowers/plans/2026-09-07-write-root-entry-sentinel.md`;
  the verified plan commit is `d783b075c7b0653fcd91c1f6f8529dc05dc85c1b`.
- Registered build worker: `01a07ed0-5b85-7792-b54f-bc2baf6fea4f` (`docket-build-economy`).
  It observed the missing-sentinel red baseline and committed the passing implementation at
  `cafed7660e75edcf0da2e51e140bd01aa93a9c2d`.
- Registered reviewer: `01a07ed9-0c66-7b03-a909-4a6717432963` (`docket-review-lean`), clean.

An earlier isolated attempt halted before claim because the synthetic repository explicitly
configured unsupported `skills.*: auto` bindings. Removing those fixture-only overrides allowed
the normal registered roles to run. That preclaim halt is not counted as completion evidence.

Subsequent contract-validation and final-message repairs were also exercised against a clean,
explicitly stamped candidate built from `d95b046d5b9e01872047ae0cb99a3a6982999391`, source tree
`01a11c6e3f2f1dcb22c108c004f422322d0bb26f`
(SHA-256 `181af9089c815bd33161334e02fac4e347b77735d9be84608b8abfbbeaffcadc`).
The build disabled automatic VCS stamping and set the verified full source commit explicitly,
because Go's automatic metadata identified the enclosing main checkout for this linked worktree.
A second isolated real root launch returned `applied`, and its output exactly matched the actual
session's final-answer message. Its transport-only marker expectation failed: the higher-priority
role contract ran repository preparation and correctly halted in the non-repository fixture.
This is launch/final-return evidence, not another successful implementation or full live rerun
of the later candidate. The final full suite separately certifies all code at the published head.


The claim transaction `f93121b61fb61f547592853f64951b22f6b9f59c` records change 1 and a
`gate_context_hash` matching the SHA-256 of the outer dispatch context. After the coordinator
published fixture PR 1 and marked the change implemented, its own `run.verify` returned
`run-complete` at `cafed7660e75edcf0da2e51e140bd01aa93a9c2d` with no unmet conjuncts.
The foreground command then returned, and the fresh parent's actual command output was:

```text
gate-done implement-next-20260908t020638z-97715-2227 run-complete 1
```

This satisfies the ordinary-prose routing, real planner, unchanged claim context, foreground
completion and exact parent-attribution boundaries that remained unproven in the earlier report.
The context-removal mutation supplies the negative control. The later repairs and all final
source changes remain subject to the full configured suite; its immutable exact-head record
belongs in PR #265's managed build-evidence block. Publication must also re-establish change
393's own `run-complete` receipt. The synthetic change's successful receipt does not substitute
for either final publication check.

## 2026-09-08 Task 13 verification continuation

Initial tested HEAD: `12e3d1d1a25a3bb45ab8fd999ba0fb5d550a62ce`.
`go fmt ./internal/...` left the clean tree unchanged and `git diff --check` was silent.
The focused command below passed, as did the asset check (67 entries,
`da3513307f640a2b9cf257046c04d292ad23d066dd112b76b360c8d0f7415402`):

```text
go test -count=1 ./internal/harness/... ./internal/codexentry ./internal/cli ./internal/reposeed ./internal/repoguard ./internal/assets
go run ./cmd/genassets -check
```

The first full source-gate run exposed a real integration-partition defect:
`internal/cli/agent_worktree_integration_test.go` lacked the required
`//go:build integration` line, so its `TestIntegration…` leaked into the default corpus.
The test was split into an ordinary helper/unit file and a tagged resolver regression; the new
CLI integration shard and runtime-budget row establish one tagged runner. The repaired checks
passed:

```text
go test -count=1 ./internal/cli
go test -count=3 -tags integration ./internal/cli -run '^TestIntegrationAgentEnterFeatureResolverObservesSelectedWorktreeConflict$'
bash tests/test_go_integration_contract.sh
```

The named guard-focused green controls passed: inventory closed-scope parsing, inventory-derived
scope-aware routing, feature-dispatch payload coverage, and the three-run resolver regression.
They retain the Task 7--11 red mutations: missing/unknown scope, caller cwd in place of the
resolved worktree, native feature routing, deleted owner payload line, and coordinator cwd in the
resolver conflict regression. No mutation was left in the tree.

Final-diff checks found no authored Claude/Cursor/OpenCode renderer change, no hand-edited embedded
asset (the generator check above is clean), no Accepted/archived historical edit, and no request
rewriting. Scope coverage is derived by `ParseInventory` and its guards, not a role-name population
count. The successor verification commit also contains this results-only evidence plus the
integration-partition repair. The final `go run ./cmd/docket development test` passed: 43/43
files, 387 assertions, exit 0. Screening diagnostics (not authoritative breaches) were:
`BUDGET WATCH` for finalize-e2e (113s), integration-app-rebase (148s), and
integration-app-workflow (120s); `PARALLEL-SENSITIVE` for go-race (268s; prior solo 83s)
and go-toolchain (205s; prior solo 58s). No `SERIAL CONFIRMED OVER BUDGET` line occurred.

## 2026-09-08 Task 13 rebase verification — final-review round 1

This round rebased old feature head `95a20a6f4c269fe5163d0c79b4ab071d4b6ea3ce`
(old merge base `b801b8c6dd23f55b23d1edecc373039e04ffe96c`) onto
`origin/main` `9e82cc47c8fc27579f6e32abd84a99e9e609b28e`. The exact source
head tested below is `18a18b8de053ac6fd3fd5bfd029573ebccdd8c49`, whose
merge base is that same `origin/main` commit. The results-only commit following
this section does not change the tested implementation.

The review's red condition was the integrated branch's missing `--json` capture
in the feature-worktree dispatch contract, which current main's
`TestGateDriveJSONCapture` rejects. Rebase resolution preserved both intents:
the `gate.drive.prepare-scope` call captures credentials from `--json`, while
the dispatch payload retains the canonical `Feature worktree:` line. Generated
embedded assets and manifest were then regenerated only through
`go run ./cmd/genassets` (67 entries,
`sha256:43cb21a47c767cbeccc1ed745bef6f2cf8acc83bdf5601352454730e8eb9d478`).

Green evidence at the tested source head:

```text
go fmt ./internal/...
git diff --check
go test -count=1 ./internal/harness/... ./internal/codexentry ./internal/cli ./internal/reposeed ./internal/repoguard ./internal/assets
go run ./cmd/genassets -check
go test -count=1 -tags=integration ./internal/app -run TestIntegrationWorkflowRootEntryGateAttribution
go test -count=1 ./internal/codexentry -run TestEnterMapsContractAndWaitsForRootCompletion
go test -count=1 ./internal/cli -run 'TestAgentEnterCLIUsesVerifiedFeatureWorktree|TestResolveAgentEntryCWDRejectsInvalidFeatureWorktrees'
go test -count=1 ./internal/repoguard -run 'TestGateDriveJSONCapture|TestFeatureDispatchPayloadsCarryCanonicalWorktree|TestSkillSizeBudgets|TestRuntimeBudgetsCorrespondence|TestCommittedCodexDispatchRoutesEveryScope'
go run ./cmd/docket development test
tests/test_go_finalize_e2e.sh
```

The configured full gate exited 0: 43/43 files passed, 387 assertions, 0
failures, wall 281s. It emitted no `SERIAL CONFIRMED OVER BUDGET` breach.
Screening-only diagnostics were `PARALLEL-SENSITIVE` for finalize-e2e (117s;
last solo 26s), go-race (281s; last solo 83s), and go-toolchain (216s; last
solo 58s); and `BUDGET WATCH` for integration-app-merge (77s, streak 2/5),
integration-app-rebase (156s, streak 5/5), integration-app-workflow (124s,
streak 3/5), and integration-gitcli-repo (77s, streak 2/5). The runner marked
finalize-e2e serial confirmation due and deferred rebase confirmation because
the confirmation slot was consumed; the direct serial `tests/test_go_finalize_e2e.sh`
check then passed all 6 assertions in 26.8s, clearing the only due confirmation
without a breach.
