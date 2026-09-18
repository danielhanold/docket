---
id: 431
slug: 'native-codex-acceptance-for-active-worker-validation'
title: 'Native Codex acceptance for active worker validation'
status: 'killed'
priority: 'low'
type: 'chore'
created: '2026-09-16'
updated: '2026-09-18'
depends_on: []
stacked_on: 425
related: [425, 430]
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-09-16-native-codex-acceptance-for-active-worker-validation-design.md'
plan: 'docs/superpowers/plans/2026-09-16-native-codex-acceptance-for-active-worker-validation-plan.md'
results: 'docs/results/2026-09-16-native-codex-acceptance-for-active-worker-validation-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch:
pr: 'https://github.com/danielhanold/docket/pull/309'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-16-native-codex-acceptance-for-active-worker-validation-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-16-native-codex-acceptance-for-active-worker-validation-design.md) |
| Plan | [2026-09-16-native-codex-acceptance-for-active-worker-validation-plan.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-16-native-codex-acceptance-for-active-worker-validation-plan.md) |
| Results | [2026-09-16-native-codex-acceptance-for-active-worker-validation-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-09-16-native-codex-acceptance-for-active-worker-validation-results.md) |
<!-- docket:artifacts:end -->

## Why

Exercise the repaired native worker active-input check and completion ordering after acceptance 430 halted. Preserve change 430 and its failed fixture as evidence.

## What changes

On the published change 425 base, create internal/nativeacceptance/value.go and value_test.go with tested Value and Double functions. Require native planner, scoped worker, reviewer, both full configured gates, durable review evidence, results attachment, and a real stacked PR in danielhanold/docket. Use the fresh acceptance-active-validation launch kit and its pinned candidate.

## Out of scope

Do not merge, resume or reset change 430, modify unrelated changes, or weaken scope/input validation.

## Reconcile log

### 2026-09-16

2026-09-16: Reconciled against the prepared acceptance checkout and the published 425 effective base. The requested two-file Go package remains absent from the base, the stacked base and PR base remain codex/restore-native-codex-dispatch-for-multi-agent-v2-docket-coor, and the scope remains valid.

## Human-authorized retirement — 2026-09-18

The human explicitly approved a metadata-only archival exception because the installed change.kill transaction accepts only proposed/in-progress records. This temporary administrative status permits the supported archival renderer; it does not restart implementation, acquire a new claim, erase a merge, or authorize any run. Kill immediately as abandoned delivery, retaining all branches, worktrees and evidence.

PR #309 really merged into 425, not main, at 8982b2873580f5677a0900abc94bfaef22403028; its feature head was f6695e5f8fbb198ba9458d40cfeb215a2af928bc. The native fixture's successful checks remain valid point-in-time evidence, with human interventions and its explicitly scoped timing exception. They did not establish unattended reliability for the later failed 424 production run. See the [retirement findings](../research/0433-fresh-main-retirement-findings.md) and successor 433, which starts afresh from main and must leave Claude Code, Cursor and OpenCode behavior unchanged. No merge into main is claimed or authorized.

## Closeout notes

### Verification

- Final gate passed at f6695e5f8fbb198ba9458d40cfeb215a2af928bc with the configured command `go run ./cmd/docket development test`.
- The direct full suite passed 56 files and 460 assertions.

### Late findings

- The suite's serial confirmation measured tests/test_go_integration_app_closeout.sh at 61s against its 60s threshold. The human explicitly authorized this one-second timing exception for change 431 closeout; the budget policy was not changed.

## Why killed

Abandoned by explicit human decision on 2026-09-18 in favor of [change 433](../active/0433-pilot-top-level-codex-coordinators-with-one-level-native-dis.md), starting afresh from main. The successor must leave Claude Code and Cursor and opencode behavior unchanged. Preserve predecessor branches, worktrees, dirty files, commits, original plans/results and private operational evidence; no deletion, merge into main, runtime restart, or wholesale cherry-pick is authorized.

See the [per-change retirement findings](../research/0433-fresh-main-retirement-findings.md), recorded before this transition.

The human explicitly approved a metadata-only archival exception for this previously stacked-merged record. PR #309 really merged into 425 (not main) at 8982b2873580f5677a0900abc94bfaef22403028, from feature head f6695e5f8fbb198ba9458d40cfeb215a2af928bc on chore/native-codex-acceptance-for-active-worker-validation. Its completed fixture, native review, recorded 56-file/460-assertion suite and scoped 61s/60s timing exception remain historical facts. Human-assisted acceptance is not proof of later unattended production reliability. This kill abandons delivery through 425; it does not undo or deny the GitHub merge event. [Frozen results](https://github.com/danielhanold/docket/blob/f6695e5f8fbb198ba9458d40cfeb215a2af928bc/docs/results/2026-09-16-native-codex-acceptance-for-active-worker-validation-results.md) remain at the retained feature head.
