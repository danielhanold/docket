---
id: 433
slug: 'pilot-top-level-codex-coordinators-with-one-level-native-dis'
title: 'Pilot top-level Codex coordinators with one-level native dispatch'
status: 'proposed'
priority: 'high'
type: 'refactor'
created: '2026-09-18'
updated: '2026-09-18'
depends_on: []
stacked_on:
related: [360, 412, 422, 423, 424, 425, 426, 430, 431, 432]
discovered_from: [424, 425, 430, 431, 432]
adrs: [16, 111, 115, 118, 119]
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0016](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0016-harness-first-agent-config.md), [ADR-0111](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0111-run-gate-attribution-binds-a-dispatch-to-its-successful-clai.md), [ADR-0115](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0115-outer-run-gate-retry-budget-is-a-counted-config-snapshotted.md), [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md), [ADR-0119](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0119-native-codex-dispatch-with-explicit-role-aware-feature-bindi.md) |
<!-- docket:artifacts:end -->

## Why

The user stopped the change 424 operational run after repeated intervention and proposes a structural experiment: let the existing top-level Codex session run docket-implement-next itself and directly dispatch leaf agents. Do not require a top-level controller to launch a second coordinator which then launches workers. Preserve the useful workflow and guarantees, but stop treating identical agent topology across harnesses as a requirement.

This is a proposed experiment, not an approved implementation spec. Keep it needs-brainstorm until runtime model verification, root lifecycle ownership, and the simplification boundary are settled with the human. On 2026-09-18 the human approved abandoning the 425 implementation stack and starting 433 afresh from main. There is no stack parent and no dependency on 425 or 424's partial implementation. The abandoned work is research evidence, not the implementation base. Do not resume any abandoned run.

### Fresh-main decision and predecessor findings

The [failed-approach retirement review](../research/0433-fresh-main-retirement-findings.md) summarizes each predecessor, preserved evidence, the retirement disposition, and the limits of the old acceptance results. All five directly superseded active changes were archived as killed on 2026-09-18; PR #303 was closed without merging. Historical POC 423 remains a completed experiment, not production certification. Independent shared work such as 412 and 422 is neither inherited nor automatically cancelled.

- [424](../archive/2026-09-18-0424-validate-codex-coordinator-models-against-a-versioned-capabi.md) — killed; findings and preserved evidence are recorded in its retirement rationale.
- [425](../archive/2026-09-18-0425-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor.md) — killed; findings and preserved evidence are recorded in its retirement rationale.
- [426](../archive/2026-09-18-0426-constrain-agent-enter-to-explicit-legacy-and-feature-worktre.md) — killed; findings and preserved evidence are recorded in its retirement rationale.
- [431](../archive/2026-09-18-0431-native-codex-acceptance-for-active-worker-validation.md) — killed; findings and preserved evidence are recorded in its retirement rationale.
- [432](../archive/2026-09-18-0432-complete-native-codex-runner.md) — killed; findings and preserved evidence are recorded in its retirement rationale.
- [430](../archive/2026-09-17-0430-native-codex-acceptance-for-durable-review-evidence.md) — already killed; earlier failed acceptance remains preserved.

431 and 432 were archived under the human-approved metadata-only exception. Their PRs #309 and #310 actually merged into 425, never main; this historical fact is preserved rather than rewritten as an unsuccessful merge. Branches, worktrees, dirty files, original plans/results and captures remain intact. No cleanup deletion was performed.

The non-negotiable acceptance criterion is: **leave Claude Code and Cursor and opencode behavior unchanged**. This covers their shared workflow instructions, generated assets, configuration and model selection, dispatch/worktree behavior, gate ownership and cancellation/resume semantics, persisted-state compatibility, review, evidence and publication—not merely their adapter source files.

### Findings from the stopped 424 run

The run used the isolated 425 candidate at clean source commit 70de2fc251ea9720a84bfe3a02c0f79bf15ed21e, version v0.0.0-candidate.425.70de2fc2, binary SHA-256 5f2a1cc874ee4bbc8c507b181fbee570e78ea834b0d87150a862e99c0e85dbde. The runtime kit was /Users/homer/dev/docket-0425-phase4.0zuR5P. The registered coordinator pin was gpt-5.6-terra/low; a registration pin is not independent proof of a session's actual model. No global pins or installation were changed. At the checkpoint, 425's PR #303 remained open at that candidate head.

1. An earlier shell agent.enter path returned without successful workflow completion. Its registered coordinator later proved terminal, but its epoch remained active with no confirmed claim. run.cancel refused claim-unconfirmed; an owner-complete marker prevented automatic fencing. TestRunCancelRefusedWrongClaim passed, confirming this refusal was deliberate existing behavior. Native read_thread plus process inspection established quiescence; under explicit user reset approval the exact old directory was quarantined intact. That manual recovery was not a cancellation receipt.
2. Real native planning and worker dispatch did succeed. Nested children were available in this session. The failure was not a categorical inability of Codex to nest agents, nor proof that all Codex releases or models behave alike.
3. The coordinator repeatedly sent a terminal response after dispatching a still-live planner or worker, after individual tasks, while preparing assignments, and after recovered test results, despite persistence instructions. The top-level controller had to collect verdicts, continue or retry only when authorized, cancel, and obtain fresh supported admission. This was not unattended end-to-end acceptance.
4. Tool-result capture lost critical data: emitting only exec_command output discarded its exit_code; the outer tool's success was incorrectly substituted as exit 0 for a FAILED gate. The receipt checker correctly refused the contradiction. A worker also lost a yielded shell session identity. Durable stdout/stderr/actual-exit capture and retention of the original task handle repaired these individual incidents; the original operation was not rerun merely to reconstruct its status.
5. Assignment construction repeatedly omitted or confused required data: the exact planner artifact path, planner-versus-worker payload fields, provisional versus final resource roots, inherited devmode_test.go, and later CLI/harness fixture write grants. Validation refusals were useful, but too much exact bookkeeping remained model-authored.
6. A nativefixture success message always advised starting a fresh session; the coordinator treated that unconditional advisory as evidence of a failed session. Source inspection and an independently validated real child disproved that interpretation. Verified disk bytes alone still do not prove which resources a session loaded.
7. A continuation was misread as lacking credentials even though drive.generation was the documented single-use handoff token. Correct interpretation enabled claim/advance and exposed real test failures. A canonical inherited-state fingerprint already existed in the drive record, but the coordinator thought it needed to invent a hashing helper. Reusing the exact recorded snapshot passed validation; no authority or fingerprint was fabricated.
8. Completed descendants still occupied native thread capacity. Archiving the completed coordinator and descendants succeeded, but unloading propagated asynchronously; one immediate retry still failed, and a later verified capacity change allowed dispatch. Completion, archive, and released capacity were distinct states.
9. Historical cancelled/superseded run records caused resume-epoch-unreadable through ambiguous non-superseded matches. Only the exact historical chain was archived intact under reset approval. Other recovered returns produced takeover-ambiguous or identity-mismatch verdicts. Their complete causes were not established; do not label every refusal a product defect.
10. repository.prepare reported healthy/no-op while local metadata appeared behind the remote claim. A clean fast-forward and later authoritative context resolved the immediate mismatch; the underlying cause remains unproven.
11. Task 4 exposed genuine fixture/policy incompatibilities beyond orchestration: core install/fixture changes initially affected non-Codex selection and were corrected; remaining failures required Codex fixture pins in internal/cli/agent_test.go and minimum metadata in internal/harness/codex/codex_test.go. These are not evidence that safety checks should simply be deleted.
12. A narrow reset preserved exact Task 4 diff/status before restoring two paths. Later continuations preserved inherited dirty work. No controller should adopt a live worker's writes or reset a live writer to make the workflow advance.

The hypothesis is that eliminating an intermediate coordinator and reducing model-managed state will make Codex execution more reliable and understandable. There was no controlled comparison holding model, effort, workload, and harness constant against Claude Code or Cursor. The evidence supports testing this design, not claiming nesting alone caused every failure.

### Preserved checkpoint and evidence

424 stopped at Task 3 HEAD 434ebe2aac3f96d572a813368b4bcf030e41330c. Earlier commits: plan 4870144f; Task 1 9fac14eaff13f448e576cd8e244532507582a573; Task 2 1410893297c80f52febb9a0d5343fc1e46beb112. Tasks 1–3 have focused native-worker red/green, mutation, active-validation and acknowledgement evidence, not final whole-suite certification.

Task 4 remains uncommitted in /Users/homer/dev/docket/.worktrees/validate-codex-coordinator-models-against-a-versioned-capabi: cmd/nativefixture/main.go, internal/app/install.go and install_test.go, internal/cli/install.go, and internal/install/devmode.go, devmode_test.go, service.go, service_test.go. A corrected continuation assignment was prepared but never dispatched; its old scope is cancelled. Tasks 5–6, complete build/final-head gates, independent review, results publication, and the reviewed open PR were not reached. The last newly armed replacement was cancelled before dispatch; children were terminal and no relevant test/gate processes remained at the stop checkpoint. This record does not authorize reusing old capabilities.

Local diagnostic narrative: /Users/homer/dev/docket-0424-restart.ZyKU6s/operational-evidence.MTtgDV/REPORT.md. Original request context: AUTONOMOUS-HANDOFF.md and START-HERE.md under /Users/homer/dev/docket-0424-restart.ZyKU6s. Captures and task4-reset-evidence remain in the preserved run-424-controller evidence. These are local provenance locators, not portable dependencies; this proposal carries the substantive findings. Do not publish private gate keys, epochs used as authority, child capabilities, or raw credential-bearing captures. Historical report sections describe earlier checkpoints and do not override the final stop.

The prior 96s/90s serial budget finding remains unwaived. Earlier parallel-green output is not proof of serial compliance. 431 and 432 contain previous intervention-assisted acceptance/repair history; do not erase that history or mistake a successful individual probe for a reliable workflow.

## What changes

### Proposed topology and role scope

For Codex, the active top-level session invokes the coordinator skill inline. docket-implement-next owns selection through reviewed open PR; docket-build runs inline under it and dispatches direct leaf workers. Planner, profile-selected build workers, independent reviewer, and any ADR author remain separate direct children. The root owns escalation and sequencing; children do not spawn another orchestration layer. Preserve role contracts, read-only review separation, and configured leaf profiles.

Audit other coordinator skills with the same need. docket-auto-groom has a critic child; docket-finalize-change has resolver/integration-repair children. Define the same top-level pattern where appropriate, without running either workflow merely to test registration. Audit resolved/custom skills for hidden nested dispatch; explicitly reject or document unsupported composition rather than silently restoring the old topology. Leave Claude Code, Cursor, and OpenCode behavior unchanged.

### Actual root model and effort, not self-attestation

Resolve the expected coordinator model/effort through existing config precedence, including global settings and valid repository overrides. Compare that expectation with authoritative runtime identity for the actual root session before claiming or mutating a change. A config file states intent; an agent saying its own model or seeing a role declaration does not verify reality.

Design explicit matched, mismatched, and unverified outcomes. Recommend stopping on a known mismatch. Decide with the human whether unavailable introspection is also a hard stop or permits an explicit, strongly warned pilot override. Never silently accept unknown, auto-change global pins, or substitute a child model as evidence of the root's model. Define revalidation for resumed sessions and model/effort changes. If native root identity cannot be verified, state that limitation as an experiment result instead of inventing proof.

### Root-owned lifecycle

Remove the artificial parent/coordinator boundary while preserving one authoritative run owner. Design supported admission, successful-claim attribution, cancellation before and after claim, child registration and terminal observation, continuation, interruption recovery, and completion verification for a root-owned run. Do not fake a coordinator dispatch receipt to satisfy the old state machine.

The removed parent-to-coordinator retry loop is not the same thing as suite-attempt budgets or bounded repair policy. Root termination is still not automatic child cancellation. Retain canonical child handles across yields, exact terminal exit/output, truthful stopped-versus-unobserved states, quiescence before recovery, and no overlapping replacement writers. Prevent stale cancelled records from creating ambiguous admission. Make slot cleanup explicit and observable where the native harness supports it.

### Simplification is a deliverable, not an incidental cleanup

Inventory actual Codex-specific checks, generated instructions, CLI/schema fields, tests and callers on main. Compare the retired stack only as research: machinery absent from main should normally stay absent, not be imported to be simplified. Classify existing checks as remove, replace/simplify, or retain with their invariants and negative-test coverage. Distinguish production requirements from isolated candidate/dogfood staging. Report the reduction in agent boundaries, state transitions, model-authored fields, and duplicate checks; do not promise an arbitrary deletion quota. No wholesale cherry-picking or merging of 425 or its descendants; any individually reused fix needs an independently demonstrated necessity, review and tests against main.

Candidates to REMOVE where they exist solely for the eliminated layer:

- Top-level dispatch of a second coordinator, coordinator-specific launch routing/description markers, duplicate bootstrap and resource loading.
- Parent/coordinator request, provenance, and authority transport needed only to cross that boundary.
- The coordinator-return/parent-verdict/redispatch choreography at that removed boundary, replaced by root lifecycle enforcement.
- Requirements that the coordinator model be usable as a nested child. The actual root must still possess the necessary dispatch capabilities and leaf assignments must remain valid.

Candidates to SIMPLIFY, not erase:

- Assignment/payload construction, exact artifact paths, role schemas, immutable resource snapshots, inherited-path/write-grant assembly, and canonical fingerprints. Prefer one supported deterministic preparation operation over requiring the model to assemble several mutually dependent documents.
- Gate invocation capture, receipt validation, continuation token mapping, and wait/observation plumbing. The model should receive an unambiguous typed result with the necessary handle, not manually reconstruct stdout/stderr/exit status.
- Duplicate candidate/binary/asset provenance checks where one verified immutable bundle and explicit child binding provides the same guarantee. Do not assume all staging checks are redundant in installed production.
- Legacy agent.enter/app-server paths and model-capability policy, reconciled with 426 and 424. Remove only genuinely unused paths with cross-harness regression evidence; do not wholesale delete feature-worktree protection.

RETAIN the guarantees: canonical repository/worktree/branch/head identity; child scope and write ownership; containment and no concurrent writers; live task tracking and explicit safe cancellation; bounded retries and single-use continuation; independent review; exact-head full-suite/evidence/results/PR verification; truthful unknown/failure distinctions. One-level dispatch does not itself give native worktree isolation. Existing checkers often correctly caught bad orchestration inputs; deleting them without replacing their invariant would hide the failures.

Review ADR-0119 and related lifecycle/config decisions for a new or superseding decision where needed; do not rewrite accepted historical ADRs. Reconcile overlap with 360/412's coordination cost/supervisor work, 426's legacy-entry cleanup, and 431/432's repair findings. This change tests a topology pivot, not another repair within the old topology.

### Pilot and acceptance evidence

Start from the freshly verified origin/main in a new isolated candidate/worktree, with 433's PR targeting main. The planning baseline at retirement is 3ccf9fac511f370c200315674a1edf97766b0a5b; re-resolve main at implementation rather than assuming this snapshot is current. Do not use the 425 branch, stopped 424 worktree, its partially written model registry, or the old candidate runtime as the implementation base. Human-approved design comes before implementation.

Exercise a small but complete real workflow: root coordinator, native planner, scoped native worker, native independent reviewer, complete configured build and final-head gates, results/evidence attachment, and a reviewed open PR. Include a planned real continuation and explicit cancellation/resume rehearsal. Observe all children to terminal state and verify the exact final head; leave the PR open and do not merge.

Prove the unchanged-behavior criterion for Claude Code, Cursor, and OpenCode against the exact candidate: inspect generated instructions/assets and config outputs, run cross-harness contract and integration tests, and obtain real workflow acceptance including interruption/resume. Exercise upgrade/rollback and old persisted-state compatibility if any shared format or reader changes. Green source/packaging/platform-smoke CI is not live harness acceptance. An unavailable harness is an explicitly outstanding acceptance item, never an inferred pass. Prefer Codex-local seams; any unavoidable shared edit must preserve all three harnesses' existing observable behavior, have negative/regression evidence, and receive explicit design scrutiny.

Test model match/mismatch/unverified behavior, wrong worktree and unauthorized writes, lost or inconsistent exit information, stale/duplicate handoffs, root interruption with a live child, cancellation before claim, replacement admission, and delayed native capacity release. Mutation-test guards rather than keeping vacuous assertions. Run the full source-resolved suite and read budget reports; do not weaken budgets or substitute focused tests for full certification.

Record every human/controller rescue, reset, missing capability, retained check, and model/effort provenance limitation. A completed spawn or plan alone is not acceptance; an intervention-assisted run may diagnose defects but does not establish unattended reliability. Define an explicit stopping criterion for repeated operational failure instead of accepting an unbounded reset loop. Compare the new topology against the documented baseline without claiming a controlled cross-harness result that was never measured.

### Decisions still requiring design approval

Authoritative root model/effort discovery and unknown policy; exact root lifecycle API and interruption behavior; which indirect skill compositions are supported; precise removal versus replacement inventory; and the minimal end-to-end fixture plus acceptance/rollback criteria. Capture the approved choices in a linked spec before making this change build-ready.

## Out of scope

The human authorized summarizing and killing the abandoned approach, not implementing 433, creating its worktree, resuming 424, or merging anything. Preserve predecessor branches, commits, dirty work, captures and cancelled records as historical evidence; no deletion or wholesale import is part of this retirement.

No global model-pin or stable-install changes, no workaround through another harness or a shell-hosted replacement coordinator, no fabricated identities/receipts/cancellation proof, no budget resets, no widening timing limits, and no bypass of scope or exact-head validation. No blanket removal of run gates, worktree safeguards, independent review, or cross-harness protections. Changes to Claude Code/Cursor/OpenCode behavior, the full supervisor envisioned by 412, unrelated timing cleanup, and wholesale shared-orchestration rewriting are outside this bounded pilot.
