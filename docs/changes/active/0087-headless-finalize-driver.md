---
id: 87
slug: headless-finalize-driver
title: Headless finalize — the finalize-side disposition contract, mirroring 0088
status: proposed
priority: high
created: 2026-07-17
updated: 2026-07-18
depends_on: []
related: [8, 88, 95]
adrs: [43]
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
| ADRs | [ADR-0043](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0043-retire-bot-auto-approval-zero-approvals-branch-protection.md) |
<!-- docket:artifacts:end -->

## Why

**Nothing invokes `docket-finalize-change` hands-off today.** A human who wants "close out the
merge gate, walk away, come back to merged PRs" has no way to get it — verified 2026-07-18: no
driver surface exists anywhere in `scripts/` or `skills/`. That gap is the whole change, and it
survives every shift in the machinery underneath it.

Two things changed on 2026-07-18 that make this both **simpler** and **well-precedented**.

**The merge wall is gone (change 0095, ADR-0043).** This change was originally framed as "ship a
consumer for 0062's `auto_approve` capability" — dispatch the bot workflow, poll it, verify
`reviewDecision`, then merge. That subsystem was retired in full: the knob, the workflow, the
setup script, and the gate's step 6 are deleted, and ADR-0042 is reversed. Branch protection set
to **require a PR with zero approvals** now lets a plain `gh pr merge --rebase` land with no
`--admin`, no bot, and nothing for a permission classifier to deny. The hard part of the original
scope — an approval chain with its own failure modes — simply evaporated. What remains is
invocation.

**Change 0088 already solved the shape of this problem on the implement side.** Its answer was a
**driver-agnostic re-invocation contract** and deliberately **no loop primitive and no new entry
surface**: `docket-implement-next` ends every run declaring one of four dispositions
(`advanced` / `contended` / `drained` / `halted`), a driver keys on them (continue on the first
two, stop on the last two), and the built-in `/loop` is documented as the *recommended* driver
rather than something docket owns. 0088's own reconcile log partitions the space explicitly —
0088 serial self-continuation, #0008 concurrent fan-out, **#0087 finalize** — and its shipped
contract table names this change in the driver list. This change is the finalize-side half of
that partition, and it should mirror the design rather than invent a second vocabulary for the
same job.

## What changes

- **A terminal disposition contract on `docket-finalize-change`**, mirroring 0088's: every run
  ends declaring exactly one outcome a driver keys on, with a binary continue/stop decision. The
  starting proposal is `advanced` (merged + closed out) / `drained` (nothing eligible in scope) /
  `halted` (any abort-and-report), with `contended`'s finalize-side meaning an open question
  below. Prose on the skill, no scripts — as in 0088.
- **Id-set scoping** — generalize finalize's existing explicit-id argument to an allowlist
  (`docket-finalize-change 90,92,94`), deterministic order within the set, unset ⇒ every eligible
  `implemented` change. This matters more here than it did for implement-next: finalize's
  Selection matrix currently guards a multi-change batch with an **interactive prompt**, and a
  headless run cannot answer it. An explicit id set is the natural headless substitute for that
  confirmation — the human authorizes the batch by naming it.
- **Map every existing abort-and-report point to a disposition.** Finalize already has a
  well-enumerated abort set (ambiguous rebase conflict, no detectable suite, repair can't reach
  green in ≤2 attempts, red/absent CI, a rejected `--force-with-lease`, and any auto-authored
  repair under an autonomous run). These are already the right stop-and-surface semantics; this
  change names them as `halted` rather than adding new behavior.
- **Wire the stop reason somewhere a human reads.** Finalize already records abort reasons as a PR
  comment; confirm that channel covers the headless case, where nobody is tailing a log.
- **Document the drain pattern** alongside 0088's README section, framed the same way —
  recommended, confirm-in-your-harness — so both halves of the loop read as one system.

## Out of scope

- **Building a loop primitive, a `docket-drain` skill, or any new entry surface** — 0088's
  precedent is explicit and this change follows it. The driver is `/loop`, cron, a scheduled
  agent, or a human re-typing the command.
- **Concurrent/parallel fan-out** — #0008 owns that, and can build on this vocabulary the same
  way it can build on 0088's.
- **The rebase-retest gate, `require_pr_approval`, terminal-publish, or the consent model
  (ADR-0011 / ADR-0043).** This change consumes them; it does not revisit them.
- **Re-opening the retired `auto_approve` mechanism.** It is deleted and its ADR reversed; there
  is no bot chain left to drive.

## Open questions

- **Single finalize vs. drain — the partition disagrees with this file.** 0088's reconcile log
  calls #0087 "single headless finalize," while the scope above generalizes to an id set and a
  full eligible sweep. Settle which this change is. Note the asymmetry that motivates caution:
  implement-next's unit of work is reversible (a PR nobody merged), finalize's is **merging to the
  integration branch**, so an over-broad headless sweep costs more than an over-broad headless
  build.
- **What does `contended` mean for finalize?** There is no claim CAS here, but there is a real
  race: the `docket-status` sweep may archive a change between selection and close-out. That is
  an idempotent no-op the driver should continue past — which is exactly `contended`'s role. Is
  reusing the word right, or does finalize need its own fourth outcome (or only three)?
- **Does the driver re-verify classifier posture per run?** Still live, and it needs a **new
  home**: the CC 2.1.211 pin lived in ADR-0042, which ADR-0043 reversed. The exposure is smaller
  now (no dispatch to deny — only `gh pr merge` itself) but not zero, and the
  `harness-behavior-is-mode-and-version-scoped` learning is explicit that a headless observation
  is version-scoped. Detect and degrade, or just `halted`?
- **Shared dependency: the `/loop` composition spike is still unfiled.** 0088 shipped with its
  §6 live spike deferred and recommended filing a follow-up change; that change was never
  created, so `/loop` remains *recommended, confirm-in-your-harness* for both skills. If this
  change also leans on `/loop`, the two share one unverified assumption — file the spike, or
  scope this change to the contract only and let the driver question ride on it.
- **Should the contract be docket-owned at all, or the consuming repo's job?** Carried over
  unresolved; 0088's answer for the implement side was *docket owns the contract, not the
  driver*, which is probably the answer here too — confirm it is.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
