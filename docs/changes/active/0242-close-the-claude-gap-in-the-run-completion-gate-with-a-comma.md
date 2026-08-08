---
id: 242
slug: close-the-claude-gap-in-the-run-completion-gate-with-a-comma
title: Close the Claude gap in the run-completion gate with a caller-side verify in the dispatch rules
status: proposed
priority: high
type: feat
created: 2026-08-07
updated: 2026-08-08
depends_on: [237]
related: [212, 237]
discovered_from: [237]
adrs: []
spec: docs/superpowers/specs/2026-08-08-close-the-claude-gap-in-the-run-completion-gate-with-a-comma-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-08-08-close-the-claude-gap-in-the-run-completion-gate-with-a-comma-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-08-close-the-claude-gap-in-the-run-completion-gate-with-a-comma-design.md) |
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

The precise uncovered surface: for every CLI-driven harness the autonomous path runs through
`runner-dispatch.sh` and is gated; a Claude **interactive** session dispatches the skill as a
fork itself, and the parent session — the one context that regains control after the failing
agent has stopped — checks nothing. A `Stop`/`SubagentStop` hook was this stub's original
candidate and was groomed to a full draft, then rejected the same day as too heavy: user-level
registration intercepts every turn end and subagent completion machine-wide, and couples to
harness surface docket does not own. The draft is preserved in the spec's *Rejected* section as
the escalation path.

## What changes

The caller-side gate, carried by machinery docket already generates: the per-harness
`docket-implement-next` dispatch rule (written by `sync-agents.sh` into each harness's
agent-instructions file, read by the parent session) grows 0237's gate shape executed by the
parent — snapshot the in-progress set before dispatching (`verify-run --in-progress-ids`), diff
after the fork returns to attribute this run's claim, `verify-run <id>`, and on
`run-incomplete` one bounded re-dispatch with the unmet conjuncts, then stop-and-report loudly.
`run-halted` never re-dispatches. Same oracle, same discriminator, same cap as the runner-side
gate; every step a single transcript-visible facade command. Plus a one-sentence pointer in the
convention's *Composition* prose naming this as the mechanical form of the caller's
verify-the-child obligation. Full design in the linked spec.

## Out of scope

- Re-deriving 0237's verdict logic. This wires to the existing oracle or it is not worth doing.
- Any Claude Code hook (rejected — recorded in the spec), any `settings.json` or installer work.
- Any loop-specific mechanism or documentation; any headless-Claude runner adapter.
- Any change to `verify-run.sh` / `runner-dispatch.sh` / `board-checks.sh`; any metadata write
  by the gate; any new config knob.
