---
id: 87
slug: headless-finalize-driver
title: Ship the headless finalize driver — 0062's capability has no consumer
status: proposed
priority: high
created: 2026-07-17
updated: 2026-07-17
depends_on: []
related: [62, 86]
adrs: [42]
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0042](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0042-auto-approve-consent-model.md) |
<!-- docket:artifacts:end -->

## Why

Change 0062 shipped the **capability** to finalize without `--admin` — `docket-approve.yml`, the
`finalize.auto_approve` knob, the gate's dispatch/poll/verify step — and validated the whole chain
end-to-end in a headless spike (CC 2.1.211, scratch PR #93, zero denials). ADR-0042's Scope then
says plainly:

> The driver/loop that would invoke finalize headless is separate work (0062 enables the
> capability; it does not ship the driver).

So **nothing actually invokes finalize hands-off today**. The capability has no consumer. A human
who wants "run finalize, walk away, come back to a merged PR" cannot get it: the interactive
session hits the classifier (see #86), and the headless path that provably works has no entry
point that a human can reach.

That is the gap that makes 0062 read as failed from the user's chair even though its mechanism is
sound — the value was always going to arrive with the driver, and the driver was punted. Building
it is what converts 0062 from a validated spike into something that pays off.

## What changes

- A driver that invokes `docket-finalize-change` headless (the `claude -p --permission-mode auto`
  shape the spike proved), so the auto-approve chain runs in the mode it was designed for.
- Decide the driver's trigger surface: an explicit human-run command, a `/loop`-style poller over
  `implemented` changes, a cron/scheduled agent, or a GitHub Actions runner. Each trades latency,
  cost, and blast radius differently.
- Decide its scope per run: one explicit id, or drain every eligible `implemented` change (the
  Selection matrix's blast-radius guard exists for exactly this and must be honored headless,
  where nobody can confirm a batch).
- Wire the abort-and-report reports somewhere a human actually reads — the PR comment channel
  exists; a run that stops needs to surface without a human tailing a log.

## Out of scope

- The attended merge path (#86) — separable, and higher-urgency since it is a live regression.
- Changing the auto-approve mechanism, the rebase-retest gate, or ADR-0042's consent model. This
  change consumes them; it does not revisit them.
- Parallel/multi-change drain semantics beyond the existing Selection matrix (see #8).

## Open questions

- Trigger surface — the real design question; everything else follows from it.
- Does the driver re-verify the classifier posture per run? ADR-0042 pins CC **2.1.211** and flags
  classifier behavior as version-dependent, so a future release could silently re-close the
  headless path. Does the driver detect that and degrade, or just abort-and-report?
- Interaction with `require_pr_approval: true` + `auto_approve: true` — ADR-0042 says the bot
  satisfies it. A driver draining unattended makes that combination's weakened semantics load
  bearing in a way no attended run ever did.
- Should the driver be docket-owned at all, or is it the consuming repo's job? 0062 punted this
  question along with the driver.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
