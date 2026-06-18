---
id: 23
slug: script-sweep-and-health-checks
title: Decide and apply scripting vs model-driven for the merge sweep and health checks
status: proposed
priority: medium
created: 2026-06-18
updated: 2026-06-18
depends_on: [22]
related: [11, 18]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Why

`docket-status` has three passes. Change 0022 scripts the first one (the
`inline` board render) because it is pure, judgment-free transformation. The
other two — the **merge sweep** and the **health checks** — also run as
model-driven prose today, and the same token-cost and determinism arguments
*may* apply, but they are not as clean-cut:

- The **merge sweep** is a terminal-transition driver: per merged PR it archives
  the change on `metadata_branch`, runs the **terminal-publish** copy onto the
  integration branch, removes the feature branch + worktree, and best-effort
  **harvests learnings**. Its idempotent archive add is mechanical, but the
  side effects (terminal-publish, harvest) are entangled with
  `docket-finalize-change` and may be better left agent-driven.
- The **health checks** are mostly mechanical probes (broken `spec:`/`plan:`/
  `results:` links, `depends_on` cycles, stale-branch age, human-merge-gate
  stalls, board/source drift) — but at least one (`blocked_by:` re-examination
  to see if an external blocker cleared) is genuinely judgment-bearing.

So this is a **decide-then-apply** change: with 0022's rendering script and its
shared dependency-resolution core in place, determine *per pass* whether it
should be scripted or stay model-driven, and implement that decision. Splitting
it from 0022 keeps the clean, high-value rendering extraction unblocked while we
deliberate the messier two passes.

## What changes

To be decided at brainstorm. The likely shape:

- **Health checks:** move the purely-mechanical probes (broken-link resolution,
  dependency cycles, stale-branch age, human-merge-gate stalls, inline
  board/source drift) into a script — either `scripts/board-checks.sh` or an
  extension of 0022's `render-board.sh` — reusing the shared
  dependency-resolution core. Keep the judgment-bearing `blocked_by:`
  re-examination model-driven.
- **Merge sweep:** likely script the mechanical core (the `gh` is-merged probe
  and the idempotent, byte-deterministic archive add) while keeping the
  terminal-publish copy and the learnings harvest agent-driven, since those are
  shared with `docket-finalize-change` and must not diverge from it.
- Record the per-pass decision as an **ADR** if it sets a reusable boundary
  (e.g. "mechanical-and-side-effect-free ⇒ script; judgment or shared
  terminal-transition side effects ⇒ agent-prose") — this generalizes the
  0007/0011 stance.
- Tests for whatever is scripted, matching the existing suite pattern.

## Out of scope

- The `inline` board render — owned by change 0022 (the dependency).
- Changing *what* the health checks flag or the sweep's terminal-publish /
  harvest contract — this change moves the work between model and script, it
  does not change the behavior.
- The `github` surface — already scripted.

## Open questions

- **Which health checks are mechanical enough to script** vs need model
  judgment? `blocked_by:` re-examination is the clear model-side case; is any
  other?
- **Can the sweep's idempotent archive add be a script** while terminal-publish
  + harvest stay agent-driven, without the two halves drifting from
  `docket-finalize-change`'s identical per-change archive?
- **Reuse of 0022's shared dependency-resolution core** — same helper, same
  parsing approach (and the same `yq`-vs-bash question from 0018).
- Does the inline **board/source-drift** check survive once 0022 makes inline
  rendering deterministic, or collapse into a "writer skipped the refresh"
  check?

## Reconcile log
