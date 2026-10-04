---
id: 501
slug: 'replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi'
title: 'Replace run.start''s bare owner-lifecycle-unavailable line with a plain stop note'
status: 'proposed'
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
| Spec | [2026-10-04-replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-04-replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi-design.md) |
| ADRs | [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md) |
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
