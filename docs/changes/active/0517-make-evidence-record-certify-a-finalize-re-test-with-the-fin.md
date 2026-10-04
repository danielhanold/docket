---
id: 517
slug: 'make-evidence-record-certify-a-finalize-re-test-with-the-fin'
title: 'Make evidence.record certify a finalize re-test with the finalize gate settings'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [502, 360, 520]
discovered_from: [502]
adrs: []
spec: 'docs/superpowers/specs/2026-10-04-make-evidence-record-certify-a-finalize-re-test-with-the-fin-design.md'
plan: 'docs/superpowers/plans/2026-10-04-make-evidence-record-certify-a-finalize-re-test-with-the-fin.md'
results: 'docs/results/2026-10-04-make-evidence-record-certify-a-finalize-re-test-with-the-fin-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/make-evidence-record-certify-a-finalize-re-test-with-the-fin'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-04T16:11:40Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-04-make-evidence-record-certify-a-finalize-re-test-with-the-fin-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-04-make-evidence-record-certify-a-finalize-re-test-with-the-fin-design.md) |
| Plan | [2026-10-04-make-evidence-record-certify-a-finalize-re-test-with-the-fin.md](https://github.com/danielhanold/docket/blob/fix/make-evidence-record-certify-a-finalize-re-test-with-the-fin/docs/superpowers/plans/2026-10-04-make-evidence-record-certify-a-finalize-re-test-with-the-fin.md) |
| Results | [2026-10-04-make-evidence-record-certify-a-finalize-re-test-with-the-fin-results.md](https://github.com/danielhanold/docket/blob/fix/make-evidence-record-certify-a-finalize-re-test-with-the-fin/docs/results/2026-10-04-make-evidence-record-certify-a-finalize-re-test-with-the-fin-results.md) |
<!-- docket:artifacts:end -->

## Why

After an integration repair, finalize re-tests the repaired head with `finalize.test_command`, and `finalize.rebase`'s built-in gate does the same after every rebase. Both turn the passing run into evidence through `evidence.record`, which reads only the `build.*` settings. So a passing finalize run can (a) produce *skipped* evidence when `build.gate` is `off`; (b) name `build.test_command` instead of the command that ran; or (c) be refused with `unconfigured-gate-command` when `build.gate` is local and `build.test_command` is unset, which the built-in gate turns into a halt after a green suite. The evidence then misstates what certified the head.

Change 0374 meant finalize's gate to validate against the finalize settings; the code never did. It went unseen because docket sets both commands to the same value. Found while building change 0502 (the re-test path); grooming found the built-in gate path.

## What changes

- `evidence.record` takes an optional `--owner build|finalize`. Omitted means build, exactly as today. With `finalize` it reads only `finalize.test_command`, never mints skipped evidence, and records the finalize command: the same owner split `gate drive start --owner` already uses.
- `finalize.rebase`'s built-in gate passes its own owner, so finalize gate passes are certified with the finalize settings and `evidence.recertify` stays build.
- The finalize skill's repaired-head re-test passes `--owner finalize`, and implement-next's description of what `evidence.record` reads is corrected.
- Tests use different build and finalize settings and cover (a)–(c) at both call sites, each mutation-tested.

## Out of scope

- Changing the evidence record format (no owner field in the record).
- `finalize.gate: off` behavior.
- How `docket schema` publishes the `evidence.*` flags (#520).
- The wider coordination-tax and evidence items bundled in #360 (session-scoped sync, accepting results-only deltas at `pr publish`, honoring the primary tree's `.docket.local.yml` from a feature worktree, auto-detecting test commands).

## Reconcile log

### 2026-10-04

Reconciled against origin/main fc719ac6d (0502 merged). `EvidenceRecord` still reads only `build.gate`/`build.test_command`, and `processFinalizeGate.mapTerminalDrive` still calls it with no owner; the seam already carries `owner` (finalize/build). Scope stands unchanged. #520 (in-progress) edits published request schemas and may touch `EvidenceRecordRequest`'s schema descriptor; this change adds an `owner` field, so whichever lands second rebases onto it.

