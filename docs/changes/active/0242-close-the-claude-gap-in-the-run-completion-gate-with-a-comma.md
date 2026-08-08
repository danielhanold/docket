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

Investigated and confirmed available during 0237's grooming (2026-08-07, Claude Code 2.1.x):
`Stop` and `SubagentStop` are live hook events, documented for "enforce completion standards".
A **command**-type hook receives `session_id`, `transcript_path`, `cwd`, and `hook_event_name` as
JSON on stdin, and **exit 2 blocks the stop and feeds stderr back to the agent** — so the block
signal doubles as the continue instruction, with the agent still alive to act on it. A
**prompt**-type hook is another model reading prose and would reintroduce the exact defect this
family is about; only the command type qualifies.

## What changes

A Claude-specific adapter onto 0237's oracle: `scripts/claude-stop-hook.sh` (+ contract),
registered for both `Stop` and `SubagentStop` as a command-type hook in user-level
`~/.claude/settings.json` via the existing `ensure-docket-env.sh` install seam — one
registration per machine, covering every docket repo on it, self-gating to a fast exit 0
everywhere else. The hook attributes the stopping session to its run transcript-derivedly
(harness-written evidence, with a `claimed_at`-epoch fallback via
`verify-run --in-progress-ids --with-claimed-at`), shells `docket.sh verify-run <id>`, and on
`run-incomplete` exits 2 — blocking the stop and feeding the unmet conjuncts back to the
still-alive agent — at most once per session×change, then allows. Fail-open on any internal
error. A blocking build-time spike re-probes the hook protocol and transcript format at the
current Claude Code version. Full design in the linked spec.

## Out of scope

- Re-deriving 0237's verdict logic. This wires to the existing oracle or it is not worth doing.
- Any prompt-type hook.
- Any second harness's stop event or shared stop-gate abstraction; any change to
  `verify-run.sh` / `runner-dispatch.sh` / `board-checks.sh`; any metadata write by the hook;
  any new config knob.
