---
id: 495
slug: 'drop-startedrunresult-s-unreachable-empty-input-check'
title: 'Drop startedRunResult''s unreachable empty-input check'
status: 'in-progress'
priority: 'low'
type: 'refactor'
created: '2026-10-03'
updated: '2026-10-03'
depends_on: [491]
stacked_on:
related: []
discovered_from: [491]
adrs: []
spec: 'docs/superpowers/specs/2026-10-03-drop-startedrunresult-s-unreachable-empty-input-check-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/drop-startedrunresult-s-unreachable-empty-input-check'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-10-03T08:32:56Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-03-drop-startedrunresult-s-unreachable-empty-input-check-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-03-drop-startedrunresult-s-unreachable-empty-input-check-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0491's review fix F2 (commit `c999aea8a`) moved the empty-run-context check in `run.start` ahead of minting the run record, so an empty run context is now refused as `run-untracked scope-failed` before anything is minted. Both callers of `startedRunResult` (in `internal/app/runtracker_start.go`) run only after a successful mint, with a non-empty key and run context. That makes its `key == "" || runContext == ""` branch, which returns `run-untracked mint-failed`, unreachable. Its own comment already calls it "unreachable defense in depth". The branch stays only because two tests assert it directly. 0491's results file lists this as a follow-up ("`startedRunResult` keeps an unreachable fail-closed check").

## What changes

Remove the unreachable empty-input branch from `startedRunResult`. Rewrite its comment, and `HumanText`'s comment, so the two-token guarantee points at the callers' pre-mint `scope-failed` refusal. Remove the two test assertions that exercise the branch: the blank-input cases in `TestIntegrationRunStartStartedResultRequiresKeyAndContext` (`internal/app/runtracker_start_resume_integration_test.go`) and the empty-key assertion in `TestRunStartResultCarriesNoRunID` (`internal/app/runtracker_start_result_json_test.go`). Keep the started-case assertions, and rename the first test so its name says what it still checks. Grooming confirmed that `TestIntegrationRunStartEmptyRunContextMintsNothing` already covers the pre-mint refusal on all three scope-preparing paths, so no fail-closed coverage is lost.

## Out of scope

Any other change to `run.start` behavior, its refusal reasons, or the `run-started` report format. Reworking the run-start refusal ordering that 0491 settled.
