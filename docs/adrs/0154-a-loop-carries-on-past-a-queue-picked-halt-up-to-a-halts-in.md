---
id: 154
slug: 'a-loop-carries-on-past-a-queue-picked-halt-up-to-a-halts-in'
title: 'A loop carries on past a queue-picked halt, up to a halts-in-a-row limit'
status: 'Accepted'
date: '2026-10-19'
supersedes: []
reverses: []
relates_to: [115, 148, 153]
change: 570
---

## Context

ADR-0153 recorded that every halted disposition stops the driver, so one change's halt ended an entire overnight drain. A halt that concerns only one change and a setup-wide halt (broken toolchain, missing auth, and so on) cannot be told apart cheaply: classifying every halt point would be costly and fragile.

## Decision

A tracked implement-next loop may carry on past a halt recorded on a change it picked from the queue.

- The run tracker keeps one per-repository "halts in a row" count, stored outside `run-tracker/` in the per-repo state folder, so every worktree and session shares it.
- The claim records how the change was chosen: `--selection queue|named`; absent means unknown.
- `run.verdict`, in keyed mode, adds one on a `run-halted` outcome for queue or unknown selections. Any `run-complete` (named or queue) resets it to 0, best-effort, after the run-done line is final. Every other outcome leaves it alone, including `no-attributable-claim`, so pre-claim setup failures never hand out a fresh limit. Each key's effect applies once and its decision is replayed.
- Under `run.max_consecutive_halts` (default 2, minimum 1, read at verdict time) a queue-picked halt prints the new line `run-next <key> run-halted <id>`: never re-dispatch that change; only a loop driver may start a fresh `run.start`. At the limit it prints today's `run-stop` plus a note asking the person to reset with `docket run reset-halts` (local-write), which the agent never runs without a yes.
- Named, resume, retry, continuation, unknown, observe, and finalize print exactly today's lines.
- Count faults never change a report line: they fall back to today's `run-stop`.
- No run-tracker or binding schema bump.
- Docket supports one implement loop and one finalize loop at a time; this is a documented boundary, not a gate.

This amends ADR-0153's context ("every halted stops the driver") and extends ADR-0115's run accounting.

## Consequences

Loops lose at most limit-1 extra changes to a setup failure before stopping. A stopped loop's count persists across sessions until a clean build or a reset. One-off unnamed runs count like loop runs. Two implement loops at once would share and corrupt the count; that configuration is unsupported.

## Alternatives considered

- Typed halt reasons / classifying each halt point: replaced by the marker plus a limit; the cost is a couple of halted changes before stopping.
- Resetting the count at loop start: docket cannot see a loop start, and a wrong agent judgment would disable the limit.
- Deriving the count from persisted verdicts: those are pruned after 7 days.
- A gate against concurrent same-stage loops: a documented boundary instead.
- Carrying finalize past blocks: a separate change (#572).
