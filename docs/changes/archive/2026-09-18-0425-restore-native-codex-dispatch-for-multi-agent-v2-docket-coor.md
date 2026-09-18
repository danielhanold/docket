---
id: 425
slug: 'restore-native-codex-dispatch-for-multi-agent-v2-docket-coor'
title: 'Restore native Codex dispatch for Multi-Agent V2 Docket coordinators'
status: 'killed'
priority: 'critical'
type: 'fix'
created: '2026-09-11'
updated: '2026-09-18'
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
branch:
pr: 'https://github.com/danielhanold/docket/pull/303'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-14-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-14-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor-design.md) |
| Plan | [2026-09-14-native-codex-dispatch-0425.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-14-native-codex-dispatch-0425.md) |
| Results | [2026-09-14-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-09-14-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor-results.md) |
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

### 2026-09-17

2026-09-17: User approved step 2 only: reconcile PR303, rebase onto current main, verify source and obtain independent whole-production-diff review; no merge or424 launch. Adopted actual PR303 via change.repair-identity. Fast-forwarded local425 to published8deef57f6ff8764d06e786b4a27bda504c202e25, retained backup/425-before-main-rebase-20260917, and rebased onto main3ccf9fac511f370c200315674a1edf97766b0a5b. Rebased/published head7ea8e514515b829a05d9f814ad9f99a21e246c00. Regenerated conflicting asset manifests; combined capability-budget commentary without raising the limit. Preserved431/432 frozen artifacts and acceptance fixture. Full source suite passed58/58 files468 assertions exit0 wall301s; toolchain_test serial60s, no serial-confirmed breach; parallel/watch findings retained. Independent source review pending. This human-directed bootstrap summary is not an attributed native gate receipt, and no ownership records or runtime installations changed.

### 2026-09-17

2026-09-17: Independent whole-production-diff source review completed at7ea8e514515b829a05d9f814ad9f99a21e246c00. Two actionable findings: P1 native resolver WritePaths equality rejects authored-only assignments during mixed authored/generated conflicts introduced by current main; P2 runtime verifier rejects generator-emitted ../LAUNCH.md before selecting runtime entries. Require targeted producer/consumer and mixed-conflict entry regressions plus minimal fixes, subject to human repair approval. Full report and exact-head source verification are preserved at https://github.com/danielhanold/docket/pull/303#issuecomment-5718304844. Registered native review role declined missing assignment/payload; independent human-directed read-only source review was obtained instead, with no claim of native acceptance. Keep425 in-progress pending findings disposition; no merge, ownership change, runtime replacement or424 launch.

### 2026-09-17

User-approved review repairs published to PR #303 at 80e9d2f805febe7bc9907fcb56588a57160d4d40. Both independent source-review findings resolved: resolver entry excludes controller-owned generated conflicts only under existing Docket bundle eligibility; runtime pin verification ignores only the exact non-runtime ../LAUNCH.md manifest entry. No coordinator lifecycle, gate execution, ownership mutation or retry policy changes. Regression tests reproduced both failures before fixes; focused integration tests passed; deliberate eligibility-bypass and broad-path-exception mutations were rejected by tests, then restored. Configured source suite (go run ./cmd/docket development test) passed 58/58 files, 468 assertions, exit 0, on the unchanged source tree subsequently committed as this head. Log: /var/folders/9k/38zqdm6j2wn82qp7mcc415d00000gn/T/docket-425-fixes-suite.E4a5d9. Budget report reviewed: merge shard serial confirmation 45s; no serial-confirmed breach; parallel timing advisories persist, nativefixture watch 1/5, race_app_a and race_app_b confirmations deferred by runner slot policy. Independent read-only source re-review found no new findings and confirmed both original issues resolved. This is source verification, not native runtime acceptance or a Docket gate receipt. No merge, binary install, fresh session or 424 dogfood launch performed; subsequent steps remain approval-gated.

## Why killed

Abandoned by explicit human decision on 2026-09-18 in favor of [change 433](../active/0433-pilot-top-level-codex-coordinators-with-one-level-native-dis.md), starting afresh from main. The successor must leave Claude Code and Cursor and opencode behavior unchanged. Preserve predecessor branches, worktrees, dirty files, commits, original plans/results and private operational evidence; no deletion, merge into main, runtime restart, or wholesale cherry-pick is authorized.

See the [per-change retirement findings](../research/0433-fresh-main-retirement-findings.md), recorded before this transition.

The native nested-coordinator architecture accumulated substantial shared gate/persistence/skill changes and operational bookkeeping without establishing reliable unattended production execution in the later 424 dogfood. Source review fixes and green source/package/platform-smoke CI remain legitimate evidence, but live four-harness acceptance is not established. PR #303 was explicitly closed without merging. Preserve branch codex/restore-native-codex-dispatch-for-multi-agent-v2-docket-coor at 70de2fc251ea9720a84bfe3a02c0f79bf15ed21e and its worktree/runtime kit. PRs #309 and #310 merged only into this abandoned branch. [Original plan](https://github.com/danielhanold/docket/blob/70de2fc251ea9720a84bfe3a02c0f79bf15ed21e/docs/superpowers/plans/2026-09-14-native-codex-dispatch-0425.md) and [source results](https://github.com/danielhanold/docket/blob/70de2fc251ea9720a84bfe3a02c0f79bf15ed21e/docs/results/2026-09-14-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor-results.md) are retained on that exact commit, not claimed to exist on main.
