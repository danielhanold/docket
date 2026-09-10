---
id: 375
slug: gate-drive-start-not-idempotent-spawns-concurrent-drive
title: '`docket gate drive start` is not idempotent — a re-run spawns a second concurrent drive'
status: proposed
priority: critical
type: fix
created: 2026-08-30
updated: 2026-09-10
depends_on: []
stacked_on:
related: [376]
discovered_from: [372]
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

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

During the change-0372 build, `docket gate drive start` was invoked a second time in the same
worktree — to recover the `drive_id`/`generation` (owner-gen) that the first call's human-readable
output had omitted (see #376). Rather than returning the existing drive or refusing, the second
`start` launched a **second concurrent drive** in the same worktree. The double-load then aggravated
the known `test_go_race` internal/app parallel-load flake, reddening a run that a clean single drive
passes. `start` acting as a spawn on every call — instead of a get-or-create / refuse-if-live — is a
foot-gun: the natural recovery action (re-run the command) is exactly the action that breaks the run.

### Manual-kill orphan — the second concurrent-driver source (folded in 2026-09-10)

Two more incidents (implement-next runs for changes 0420 and 0421) exposed a second, more damaging
path to the same "two drivers racing one worktree" state, and it is the same root defect: **nothing
supersedes a drive whose owner was killed rather than handed off.** The gate supervisor is launched
detached as a `setsid` session leader (`internal/process/launch.go` `Launch` / `sessionAttrs`:
"*the run survives this process's exit*"), by design so it outlives a foreground-call timeout. But
that same survival means a **human pressing stop in the TUI** kills the agent's process group while
the detached driver keeps advancing the change to terminal — commit, push, open PR, mark-implemented
— under the user's git identity. When the operator then starts a **second** implement-next to
"resume," two drivers advance the same worktree at once (observed as dueling `identity-mismatch`
gate HALTs and HEAD moving twice mid-gate in both runs).

Ownership is only relinquished by a cooperative `handoff` or an **event-authorized parent takeover
on a dispatch-return event** (ADR-0107) — explicitly "*never by a timer, heartbeat, log-activity
check, claim-age, or process-name liveness guess.*" A manual kill is neither a handoff nor a
dispatch-return, so the orphan's owner generation is never superseded and "stop the run, then
resume it" is unsafe by construction. ADR-0107's takeover covers a *dispatched child's* dropped
return, not a human-initiated stop of the top-level run; this change owns that gap.

## What changes

Make `docket gate drive start` safe to invoke more than once against the same worktree: a second
`start` while a drive is already live should either return the existing drive's identity
(idempotent get-or-create) or refuse loudly without launching anything — never silently start a
second concurrent drive. Design the precise contract (idempotent vs. refuse, and how a genuinely
stale/abandoned drive is superseded) during brainstorm.

Additionally, close the manual-kill orphan folded in above: a detached driver whose top-level owner
was killed (not handed off, not a dispatch-return takeover) must be detectable and safely superseded
so that a subsequent `--resume` — or a re-armed `gate-before --resume` + re-dispatch — becomes exactly
one authoritative driver, rather than a second one racing the still-live orphan. A human stop of the
run should leave a state that resumes cleanly.

## Out of scope

- The missing owner-gen in `start`'s human-readable output — tracked separately as #376 (the trigger
  that made the operator re-run `start` in the first place).
- Any change to the `test_go_race` / internal/app parallel-load flake itself.

## Open questions

- Idempotent get-or-create, or hard refuse-if-live? What does the caller actually need back?
- How is a genuinely dead/abandoned prior drive detected and superseded rather than blocking forever?
- Should concurrent-drive detection live in `start`, or in a worktree-level lock the drive acquires?
- Manual-kill orphan: how is a detached driver whose top-level owner was killed (no handoff, no
  dispatch-return) detected and superseded so a `--resume` is the single authoritative driver? Is
  this a worktree-level lock the resume path reclaims, a reaper, or an extension of the ADR-0107
  takeover authorization to a human-stop signal?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
