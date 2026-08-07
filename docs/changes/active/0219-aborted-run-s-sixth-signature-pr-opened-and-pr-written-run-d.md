---
id: 219
slug: aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d
title: aborted-run's sixth signature: PR opened and pr: written, run dies before status: implemented
status: proposed
priority: high
type: fix
created: 2026-08-05
updated: 2026-08-05
depends_on: [211]
related: []
discovered_from: [211]
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

`aborted-run`'s three legs (0113's A and B, 0211's C) leave one abort signature undetected at
run-scale: the run opens the PR and writes `pr:`, then dies before writing `status: implemented`.

- Leg A sees no incoherence — `plan:` and `results:` are both recorded by then.
- Leg C is gated on `pr:` being EMPTY (deliberately: a recorded PR means the branch was delivered),
  so a written `pr:` makes leg C skip with zero git calls.
- Leg B catches it, but only at 12h — the same lag 0211 exists to close for the build-complete case.

This is the signature 0194's second stop actually produced (PR open, manifest unwritten), so it is
observed, not hypothetical.

Both 0211's spec and its change body name it explicitly as a deliberate follow-up rather than a
fold-in, for one reason: its evidence is a **manifest/GitHub comparison** (is the PR that `pr:`
names open, merged, or gone?), and `board-checks.sh` is git-only by contract — it shells no `gh`.
So the oracle for this signature has to live somewhere else, or the contract has to change. That is
a design question, not a fourth leg.

## What changes

Decide where a `pr:`-set, `status: in-progress` change gets its run-scale abort check, and build it:
either a new check outside `board-checks.sh` (the `docket-status` pass already runs `gh` elsewhere),
or an explicit, scoped relaxation of the git-only contract with the offline/rate-limit posture
settled. Then emit a finding naming the missing `status: implemented` write.

## Out of scope

- Retuning leg B's 12h horizon.
- Any status flip or claim release — the advisory posture holds.
