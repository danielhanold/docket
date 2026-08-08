---
id: 242
slug: close-the-claude-gap-in-the-run-completion-gate-with-a-comma
title: Close the Claude gap in the run-completion gate with a command-type Stop hook
status: proposed
priority: high
type: feat
created: 2026-08-07
updated: 2026-08-08
depends_on: [237]
related: [212, 237]
discovered_from: [237]
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

Change 0237 builds `docket.sh verify-run` — the missing consumer of the terminal-disposition
contract — and calls it from `runner-dispatch.sh`, the dispatch seam docket owns. That covers
`codex`, `cursor`, `opencode`, and every future adapter.

It does not cover **Claude**, because Claude Code dispatches subagents itself and
`runner-dispatch.sh` is not on that path. Claude runs stay covered only by `board-checks.sh`'s
`aborted-run` legs and their 2h/12h floors — exactly as today.

That gap is the whole point: all six observed instances of the half-run family (0109, 0194 ×2,
0206, 0231, 0235) happened under Claude. 0237 deliberately deferred this because a harness hook
covers exactly one harness and is the only candidate whose code docket does not own — but with the
oracle built, closing it becomes a small wiring job rather than a design problem.

Investigated and confirmed available during 0237's grooming (2026-08-07, Claude Code 2.1.x):
`Stop` and `SubagentStop` are live hook events, documented for "enforce completion standards".
A **command**-type hook receives `session_id`, `transcript_path`, `cwd`, and `hook_event_name` as
JSON on stdin, and **exit 2 blocks the stop and feeds stderr back to the agent** — so the block
signal doubles as the continue instruction, with the agent still alive to act on it. A
**prompt**-type hook is another model reading prose and would reintroduce the exact defect this
family is about; only the command type qualifies.

## What changes

To be settled at groom time. The shape is a command-type `Stop`/`SubagentStop` hook that shells
`docket.sh verify-run` and exits 2 on `run-incomplete`.

## Out of scope

- Re-deriving 0237's verdict logic. This wires to the existing oracle or it is not worth doing.
- Any prompt-type hook.

## Open questions

- **Where does it get registered?** User-level `settings.json` (docket's existing
  `ensure-docket-env.sh` seam, but fires on every turn end in every repo on the machine),
  per-repo gitignored `settings.local.json` (scoped, but per-machine drift), or a committed
  per-repo `.claude/settings.json` (travels with the repo, but registers a Claude-specific hook
  in harness-neutral config and runs on every teammate's turn end without their opt-in).
  The blast radius is the decision.
- **How does the hook know the stopping session owns an incomplete run?** 0237's snapshot-diff
  discriminator needs a before/after pair around a hand-off, which a Stop hook does not have.
  Candidates: the `session_id` on stdin matched against something the run stamps, or a
  presence-encoded run marker (with the `presence-encoded-state` removal obligation it brings).
  Neither is designed.
- **Livelock bound.** 0237's runner-side gate is capped at one re-dispatch. A Stop hook needs its
  own cap, or a hard error wedges the session with no recourse but editing `settings.json`.
- **Does this generalize?** If a second harness later exposes a comparable stop event, is there a
  shared shape, or is each harness its own adapter — the question `runner-dispatch.sh` already
  answered once for delegation.
