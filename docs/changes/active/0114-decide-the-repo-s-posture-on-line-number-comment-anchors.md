---
id: 114
slug: decide-the-repo-s-posture-on-line-number-comment-anchors
title: Decide the repo's posture on line-number comment anchors
status: proposed
priority: medium
created: 2026-07-20
updated: 2026-07-20
depends_on: []
related: []
discovered_from: [106]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 0106 exists because a code comment was the sole assertion of a behavioral property, and it
had already shipped that property backwards once (`a9da1e2`, caught only at the 0101 review). The
fix was a test fixture. But the replacement comment block 0106 landed in `scripts/docket-config.sh`
anchors its cross-references on hard line numbers (`:194`, `:201`) — reintroducing the same rot
vector one level up: any edit above those lines silently staled the pointer, with nothing to notice.

The 0106 whole-branch review raised this as a Minor finding and it was **considered and declined
in scope** for three stated reasons: it matches house style throughout the repo, it is comment-only,
and leaving `tests/test_docket_config.sh` byte-identical to its mutation-verified state was worth
more than the marginal improvement. The decline was explicitly scoped to that one block, with the
repo-wide version left open: "worth revisiting repo-wide rather than in this one block."

## What changes

Survey the repo for comments and doc prose that anchor cross-references on hard line numbers
(`<file>:<N>`, `lines N-M`) rather than on stable anchors (a function name, a unique clause, a
marker comment). Decide, once, whether the house style should change — and if so, convert the
sites and add a guard so new line-number anchors do not accrete.

Open at proposal time: whether a guard is even the right shape here, or whether the honest answer
is that line-number anchors are acceptable in a repo this size and the finding should be closed as
a no-op. The survey's result should decide that, not the stub.

## Out of scope

- Rewriting `tests/test_docket_config.sh` or the 0106 fixtures.
- Any behavioral change to `scripts/docket-config.sh`.

## Open questions

- How many sites are there? A survey of one or two argues for closing this; dozens argues for a sweep.
- Is there a stable-anchor idiom already in use elsewhere in the repo to standardize on?
