---
id: 425
slug: 'restore-native-codex-dispatch-for-multi-agent-v2-docket-coor'
title: 'Restore native Codex dispatch for Multi-Agent V2 Docket coordinators'
status: 'in-progress'
priority: 'critical'
type: 'fix'
created: '2026-09-11'
updated: '2026-09-15'
depends_on: [423]
stacked_on:
related: [393, 407, 412, 426]
discovered_from: [423]
adrs: [114, 119]
spec: 'docs/superpowers/specs/2026-09-14-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor-design.md'
plan: 'docs/superpowers/plans/2026-09-14-native-codex-dispatch-0425.md'
results: 'docs/results/2026-09-14-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor-results.md'
trivial: false
auto_groomable:
branch_prefix: 'codex'
branch: 'codex/restore-native-codex-dispatch-for-multi-agent-v2-docket-coor'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-15T15:32:25Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-14-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-14-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor-design.md) |
| Plan | [2026-09-14-native-codex-dispatch-0425.md](https://github.com/danielhanold/docket/blob/codex/restore-native-codex-dispatch-for-multi-agent-v2-docket-coor/docs/superpowers/plans/2026-09-14-native-codex-dispatch-0425.md) |
| Results | [2026-09-14-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor-results.md](https://github.com/danielhanold/docket/blob/codex/restore-native-codex-dispatch-for-multi-agent-v2-docket-coor/docs/results/2026-09-14-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor-results.md) |
| ADRs | [ADR-0114](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0114-anchor-codex-feature-scoped-role-entry-to-the-owning-worktre.md), [ADR-0119](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0119-native-codex-dispatch-with-explicit-role-aware-feature-bindi.md) |
<!-- docket:artifacts:end -->

## Why

Completed POC 423 proved one continuous native ImplementNext → planner → standard worker run with a newly created feature worktree, explicit option 2 binding from inherited primary startup, real TDD, committed and attached results, configured suites at implementation and final results commits, and the original keyed terminal halt. Its accepted evidence and reusable fixture are on the metadata branch; it was manually closed without a production-code merge. Production should now replace automatic Codex agent.enter routing while retaining the verified worktree, input, gate and artifact contracts. The user explicitly chose to deliver this before 424 and configure dispatch-capable models manually.

## What changes

Make native named-agent dispatch the Codex route for coordinators and feature planner, build and review children. Depend only on completed 423; use operator-managed exact model/effort assignments through existing configuration, with no 424 registry or typed model-policy prerequisite. Replace startup-cwd equality with validated explicit feature targeting. Preserve complete planner resources, interpreter-safe entry, static assignments and actual private child capabilities, unchanged gate context/optional epoch, each agent's own catalog, correct nested gate-response parsing and first-response retention, scoped TDD/commit/acknowledgement, exact results-template discovery and separate implementation/final-checkpoint gates. Correct ownership-aware diagnostics for both feature-only plans and results using authoritative metadata revisions. Disclose mechanics through Codex-only adapter/skill/agent references and preserve other harnesses.

Record a successor ADR to 114. Build this bootstrap change as an ordinary human-directed coding task in an isolated Codex feature worktree, without invoking installed docket-implement-next or docket-build orchestration. Validate newly generated production assets in a fresh disposable native run and exercise native review; the accepted frozen POC is evidence, not a substitute for testing production assets. Produce a real reviewed source PR, results and final-head evidence. After merge and verified installation, native ImplementNext can build 424; 426 owns broader legacy retirement.

## Out of scope

Implementing 424's model registry, minimum-capability metadata, diagnostics/live audit or local assertions; automatic model/effort changes; native child startup-directory support or obscure host workarounds; automatic/fallback agent.enter, custom runners, relays, generic runtime substitutes or inline reconstruction; other-harness behavior changes; redesigning gate ownership or retry policy; 426 legacy removal; hard-isolation or parallel-certification claims; reopening completed 423 or changing its frozen evidence.

## Reconcile log

### 2026-09-14

User-authorized bootstrap preparation: create the isolated codex-prefixed 425 worktree; plan directly at a capable setting, implement directly at Sol/low, and perform full-suite inline repairs in a separate phase. No installed ImplementNext/build workflow is used to build 425. The reviewed spec is accepted as the baseline. A later 424 dogfood may stack on the tested, pushed 425 PR branch after grooming and explicit dependency-to-stack conversion; it is not launched by this preparation.
