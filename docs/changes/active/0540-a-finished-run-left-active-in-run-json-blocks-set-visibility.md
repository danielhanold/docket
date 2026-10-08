---
id: 540
slug: 'a-finished-run-left-active-in-run-json-blocks-set-visibility'
title: 'A finished run left active in run.json blocks set-visibility and cannot be cancelled'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-10-08'
updated: '2026-10-08'
depends_on: []
stacked_on:
related: []
discovered_from: []
adrs: []
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
<!-- docket:artifacts:end -->

## Why

A run whose tracker record is already terminal can still leave its run.json at state "active". Observed on v1.0.0-alpha.1-214-g1ace73350: record.json held terminal:true with disposition `run-done <key> no-attributable-claim` and attributed_id 0, while run.json stayed `state: "active"` from the moment the run started. The repository-wide liveness check reads run.json, so `docket repository set-visibility shared` refused with `invalid-state (needs-review): a run or gate is still live` and named `docket run cancel` as the remedy. That remedy then refused with `claim-unconfirmed`. `docket run verdict <key>` reported the run as done without moving run.json, and `docket run verdict --unattributed <id>` returned `run-observe run-incomplete` without closing it. No documented command could settle the run; the only way out was to hand-edit run.json to `completed`. The run's attribution had also failed (attributed_id 0) although the change it ran for was later marked implemented, which is the same family of gap as change 345.

## What changes

Make a finished run settleable and keep the two tracker files consistent. Candidate directions, to be chosen when this is groomed: (1) when run.verdict reaches a terminal disposition it also moves run.json out of active (completed), so record.json and run.json cannot disagree; (2) the live-run scan used by set-visibility and other repository-wide preconditions treats a run with a terminal record.json as not live; (3) run.cancel can settle a run that has no attributable claim, or a run whose record.json is already terminal, instead of refusing with claim-unconfirmed. At least one path must let a human clear such a run with documented commands, and the refusal text must not name a remedy that is guaranteed to refuse. Add a regression test that reproduces the disagreement (terminal record.json, active run.json, attributed_id 0) and asserts set-visibility is not blocked or the run can be cancelled.

## Out of scope

Fixing why attribution failed in the first place (attributed_id 0 for a change that went on to be implemented); that belongs with the attribution gap work. Changing run.cancel semantics for genuinely live runs. Redesigning the run-tracker state machine or its lock and generation scheme.
