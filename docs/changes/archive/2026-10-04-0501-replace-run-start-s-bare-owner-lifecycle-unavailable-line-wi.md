---
id: 501
slug: 'replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi'
title: 'Replace run.start''s bare owner-lifecycle-unavailable line with a plain stop note'
status: 'done'
priority: 'medium'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [375]
discovered_from: []
adrs: [118]
spec: 'docs/superpowers/specs/2026-10-04-replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi-design.md'
plan: 'docs/superpowers/plans/2026-10-04-replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi.md'
results: 'docs/results/2026-10-04-replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi'
pr: 'https://github.com/danielhanold/docket/pull/375'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-04-replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi-design.md](../../superpowers/specs/2026-10-04-replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi-design.md) |
| Plan | [2026-10-04-replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-10-04-replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi.md) |
| Results | [2026-10-04-replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-10-04-replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi-results.md) |
| ADRs | [ADR-0118](../../adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md) |
<!-- docket:artifacts:end -->

## Why

Every successful `run.start` prints a bare `owner-lifecycle-unavailable` token on its own line under `run-started <key> <run-context>`. It looks like a failure code, but it is a fixed caveat: `startedRunResult` sets it on every start, so it says nothing about the run at hand. Coordinator agents then repeat it in every dispatch report ("run.start also printed owner-lifecycle-unavailable…"), and the human reads it as a bug.

The caveat itself is real and worth keeping (change 0375, ADR-0118): on the default dispatch routes docket cannot tell when the dispatching session ends, so a run stays active until someone runs `run.cancel`. The problem is how it is shown. It is also not true on every route: on the Codex route, `agent.enter` (which runs after `run.start`) sets up a death guardian, yet `run.start` still prints the token there because it cannot know which route follows.

## What changes

- `run.start`'s printed second line becomes a plain note that is true on every route and carries the run's own key in a ready-to-run cancel command: `note: closing this session may not stop this run. To stop it: docket run cancel --key <key> --reason <why>`.
- The JSON `owner_lifecycle` field keeps its current value for machine readers.
- The run-tracker rule (`cursor-rules/run-tracker.md`, the single source for the run-tracker section in AGENTS.md, CLAUDE.md, and the Cursor rule) says `run.start` prints a stop note and tells coordinators not to repeat it in dispatch reports. They bring up `run.cancel` only when the human asks how to stop a run, or a run actually needs stopping. The embedded copy and this repo's generated sections are regenerated.
- Tests pin the new printed line, check that it names the run's own key, and check that the bare token no longer appears in the printed text.

## Out of scope

- Making `run.start` aware of which dispatch route follows it, or having `agent.enter` report its death guardian.
- Changing or removing the JSON `owner_lifecycle` field or the `ReasonOwnerLifecycleUnavailable` constant.
- Rewording the rule's "there is no automatic Stop button" heading, which is also inexact on the Codex route.

## Reconcile log

### 2026-10-04

Re-read against current main (20bc0a36a). `startedRunResult` still stamps `ReasonOwnerLifecycleUnavailable` on every start and `RunStartResult.HumanText` still appends the bare token; the rule text in `cursor-rules/run-tracker.md` (and its embedded copy) still says "it reports the honest owner-lifecycle caveat". Test sites match the spec (plus `runtracker_start_resume_integration_test.go` references the token and must be checked). `docs/reference/glossary.md` names the caveat as a concept and stays accurate. No scope change.
