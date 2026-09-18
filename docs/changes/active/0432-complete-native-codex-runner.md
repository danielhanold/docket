---
id: 432
slug: 'complete-native-codex-runner'
title: 'Complete native Codex runner'
status: 'in-progress'
priority: 'critical'
type: 'fix'
created: '2026-09-16'
updated: '2026-09-17'
depends_on: []
stacked_on: 425
related: [425, 412]
discovered_from: [431]
adrs: []
spec: 'docs/superpowers/specs/2026-09-16-complete-native-codex-runner-design.md'
plan: 'docs/superpowers/plans/2026-09-17-complete-native-codex-runner.md'
results: 'docs/results/2026-09-17-complete-native-codex-runner-closeout-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/complete-native-codex-runner'
pr: 'https://github.com/danielhanold/docket/pull/310'
blocked_by:
reconciled: true
claimed_at: '2026-09-17T09:57:10Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-16-complete-native-codex-runner-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-16-complete-native-codex-runner-design.md) |
| Plan | [2026-09-17-complete-native-codex-runner.md](https://github.com/danielhanold/docket/blob/fix/complete-native-codex-runner/docs/superpowers/plans/2026-09-17-complete-native-codex-runner.md) |
| Results | [2026-09-17-complete-native-codex-runner-closeout-results.md](https://github.com/danielhanold/docket/blob/fix/complete-native-codex-runner/docs/results/2026-09-17-complete-native-codex-runner-closeout-results.md) |
| PR | [#310](https://github.com/danielhanold/docket/pull/310) |
<!-- docket:artifacts:end -->

## Why

Native Codex docket-implement-next still cannot complete a verified workflow despite the implemented 425 dispatch repairs and repeated 431 acceptance runs. The latest run passed the scoped build and published results but halted at final-certification continuation. Inspection of the saved responses shows that the handoff token was delivered and the continuation claim succeeded with fresh owner authority; the agents misinterpreted those responses as missing credentials. Repair receipt interpretation and continuation under the existing protocol, then prove the complete remaining workflow. Completion of Codex compatibility is a bounded goal distinct from shared machinery simplification and the supervisor proposed in 412.

## What changes

Finish native Codex dispatch, actual child completion observation, complete receipt transport, supported continuation, exact-HEAD certification, native review, results/evidence verification and PR completion under the existing Docket contract. Reproduce every observed failure at its actual CLI/native boundary and rehearse the complete remaining workflow before another acceptance attempt. Preserve 431 as the acceptance case, including its completed plan, worker, published results and counters.

Branch provenance (user-approved): fix/complete-native-codex-runner was branched from change 431's clean local and published remote tip ed80a72a33ce535ce8c3ab639b6090cb3ba9abc8 on 2026-09-16, not from main or merely the 425 repair branch. Source branch: chore/native-codex-acceptance-for-active-worker-validation. Both tips were verified equal before creation. The starting commit includes repair 95660e8fee6e08ba1f439a0283dba4f0364e7d0a, plan 95842609f2a0b1c9c5e8646e8cba8d8a105babd7, worker bbe70004a97f716dfa197d497985ef8930433169, and 431's published results. The branch was pre-created at the human's explicit request; preserve it when establishing future claim/workspace ownership, never replace it from main. Its intended PR base is 431's branch while that parent remains unmerged.

Track deferred shared simplification in docs/changes/research/shared-orchestration-simplification.md, and supervisor findings in docs/changes/research/0412-supervisor-findings.md linked to existing change 412. These are discovery notes, not prerequisites or implementation authorization.

Execution is human-directed: after spec approval, plan and repair on the existing branch with red/green boundary tests and full-suite verification. Do not use docket-implement-next to bootstrap its own repair. Spec publication stops before planning or implementation; any later 431 resume/native acceptance requires separate authorization after the repaired path has been rehearsed.

## Out of scope

No new acceptance fixture/change, no restart or deletion of completed 431 implementation, no reset of budgets or ownership records, no automatic PR merge, no shared orchestration redesign, and no implementation of 412. No weakening of validation or routing native roles through agent.enter, generic agents, shell runners or another harness. Fix a shared defect only when necessary to satisfy the existing Codex contract, with cross-harness regression coverage; surface contract changes separately.

## Reconcile log

### 2026-09-17 — authorized bookkeeping exception

The user explicitly authorized bookkeeping repair and equivalent workarounds without
changing ownership records or adding Docket features. The implementation is already
carried by 425 through merged PR #309. Documentation-only PR #310 uses the existing
432 branch and targets 425; stacked_on is therefore changed from 431 to 425.
The original branch provenance above remains historical evidence.

The committed original plan and current closeout results are linked directly, bypassing
managed attachment's missing workspace-adoption prerequisite. Status implemented here
means the completed documentation is published in an open draft PR awaiting review and
finalization; it is a human-authorized bootstrap bookkeeping exception, not a forged
successful mark-implemented gate receipt. No new full-suite or exact-HEAD certification
is claimed. No workspace manifest, ownership label, gate record, counter, or capability
was changed. The board and artifact links are updated with this bounded metadata repair.

Original bootstrap results remain preserved unchanged. The current results explain the
successful native acceptance and its human interventions. Research documents are on the
feature branch under docs/research; metadata originals remain until PR #310 merges.
Do not mark stacked-merged until the real PR merge is verified, or done before main
integration. No merge is authorized by this bookkeeping repair.

## Human-authorized retirement — 2026-09-18

The human explicitly approved a metadata-only archival exception because the installed change.kill transaction accepts only proposed/in-progress records. This temporary administrative status permits the supported archival renderer; it does not restart implementation, acquire a new claim, erase a merge, or authorize any run. Kill immediately as abandoned delivery, retaining all branches, worktrees and evidence.

PR #310 really merged into 425, not main, at 8deef57f6ff8764d06e786b4a27bda504c202e25; its feature head was bc78d999d32069e828ba401f26b203b46864dbac. The documentation-only closeout and repairs carried through 431 remain historical facts. Receipt/continuation interpretation and transport fixes enabled intervention-assisted acceptance but did not establish reliable later production orchestration. See the [retirement findings](../research/0433-fresh-main-retirement-findings.md) and successor 433, which starts afresh from main and must leave Claude Code, Cursor and OpenCode behavior unchanged. No merge into main is claimed or authorized.
