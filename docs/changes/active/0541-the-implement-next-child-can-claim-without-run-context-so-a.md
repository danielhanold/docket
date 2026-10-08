---
id: 541
slug: 'the-implement-next-child-can-claim-without-run-context-so-a'
title: 'The implement-next child can claim without --run-context, so a tracked run loses attribution'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-10-08'
updated: '2026-10-08'
depends_on: []
stacked_on:
related: [540, 345, 407]
discovered_from: [540]
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

A tracked implement-next run is attributed to the change it built only when the child passes the dispatch prompt's run-context token to `change.claim` as `--run-context`. The flag is optional (a claim without it is a supported untracked claim), and the only thing that makes the child pass it is one sentence in the implement-next skill. Children drop it. Evidence gathered while grooming change 540 on 2026-10-07: in a private test repository the parent ran `run start` and dispatched `docket-implement-next` with the run-context token in its prompt, but the child ran `docket change claim --id <id> --revision <revision> --json` with no `--run-context`, and the claim commit's result carries `gate_context_hash: ""`. The child built the change to a PR and `run verify` said run-complete, but the parent's keyed verdict could only say `run-done <key> no-attributable-claim`. The same shape happened in this repository: change 0366's claim on 2026-10-04 has an empty context hash, and its run ended no-attributable-claim. Every other claim since 2026-10-04 in this repository carries a context hash, so the drop is intermittent. Not the same gap as change 345: there the parent never arms a run; here the parent armed it correctly and the child omitted the token. A lost attribution means no retry authority, no continuation, and a run the tracker cannot fence or cancel (change 540 handles the symptom of the run staying active).

## What changes

Make it impossible for a tracked child to claim untracked by accident, without blocking legitimate untracked claims. Candidate directions for grooming: (1) `change.claim` requires an explicit choice, either `--run-context <token>` or an explicit untracked flag, so omission is a typed refusal rather than a silent untracked claim; (2) the implement-next context bundle (`context.implementation`) or another typed read carries the run context so the claim does not depend on the agent copying a token from its prompt; (3) the claim detects an armed, undispatched run for this repository and refuses or warns when the token is missing. Apply the same treatment to the other places the child must pass the token (`gate.drive.start --run-context`). Add a regression test for whichever path is chosen.

## Out of scope

Settling a run that already ended without attribution (change 540). The command-launched dispatch gap where the parent never arms a run (change 345). Inferring attribution from timestamps, launch shape, or child prose, which the run tracker forbids.
