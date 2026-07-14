---
id: 32
slug: positive-off-state-empty-is-a-wiring-bug
title: A deliberate off-state is encoded positively — absence and emptiness are reserved for error
status: Accepted
date: 2026-07-14
supersedes: []
reverses: []
relates_to: [28, 30, 31]
change: 71
---

## Context

docket's board is a *derived view* over the change files; `board_surfaces` lists which surfaces to render, and `board_surfaces: []` means "this repo has no board."

Change 0059 encoded that off-state **negatively**: `docket-config.sh` mapped `[]` to an **empty** `BOARD_SURFACES`, and the sole gated board writer `board-refresh.sh` treated an empty `--surfaces` **value** as "the board is deliberately disabled" — a no-op, exit 0, `BOARD.md` left untouched. **This ADR reverses that decision.** (Change 0059 produced no ADR, so there is no ADR id to name in `reverses:`; the reversal is recorded here.)

The flaw is that `$BOARD_SURFACES` is a **shell** variable, and shell state does not survive between an LLM agent's Bash tool calls. By the time a skill's Board pass ran, the variable was typically **unset**, and the command sent `--surfaces ""`. At the script boundary, **a deliberately disabled repo and a mis-wired caller were byte-identical.** The writer printed "inline disabled — no-op" and exited 0; the caller saw an unchanged `BOARD.md`, read it as exactly the genuine no-op the prose licensed, committed nothing, and proceeded believing the board was current. **The board went stale with a success exit code the whole way down.** This was hit live while filing change 0070.

Change 0059 had in fact defended the *adjacent* hole: a caller that **forgets** the flag entirely exits 2, tracked precisely via `SURFACES_SET`. What it did not anticipate was a **present flag carrying an unresolved value** — which sailed straight into the legitimate-empty branch.

## Decision

**Invert the polarity. A deliberate off-state must be encoded POSITIVELY; absence and emptiness are reserved for error.**

- **`docket-config.sh` never emits an empty `BOARD_SURFACES`.** `board_surfaces: []` — and any layer combination whose tokens all get filtered out (e.g. a global `[github]` dropped by the machine-scope fence) — resolves to the **reserved token `none`**.
- **`board-refresh.sh`:** an **empty `--surfaces` value now exits 2** (previously: no-op, exit 0). The **missing-flag** exit-2 guard from 0059 stays separate and intact — two distinct guards for two distinct holes, neither collapsed into the other (ADR-0031). `none` takes over the deliberate-disable path: no-op, exit 0, `BOARD.md` never created or truncated. **`none` is reserved and exclusive** — combined with any other token it exits 2 rather than silently picking a winner. Unknown tokens are still warned-and-ignored (a typo must never abort a build).
- **`docket-status.sh`'s `board_pass` gets the same treatment:** an empty `BOARD_SURFACES` is a fatal exit 2 — it was the reference implementation of this very bug, reading unresolved config as "disabled." `none` maps to its existing `board off` report line, so a genuinely disabled repo's stdout is byte-identical to before.

**Empty therefore has exactly one meaning left, everywhere: *nobody resolved this* — a wiring bug.**

## Consequences

**1. The trigger was removed, not merely made loud.** All 8 Board-pass call sites across 6 skill/reference files collapsed into ONE facade call, `docket.sh docket-status --board-only`, whose orchestrator self-resolves its own config. **No surfaces value crosses the skill/script boundary anymore** — so the exit-2 sentinel now defends only script-to-script callers and future ones. The generalizable lesson: **encoding a value that an agent must correctly carry across tool calls is itself the hazard.** A loud failure on the unresolved value is the safety net; the durable fix was to stop passing the value at all. (This is the same guard-the-invocation-not-the-noun instinct as ADR-0030.)

**2. A report channel must be TOTAL.** Making the caller key on a stdout report line rather than an exit code reintroduces this same defect class one level up. The whole-branch review found exit-0 paths that emitted **no `board …` line at all** — an unknown/typo'd surface token, and an inline render failure — so a caller reading "no retryable line → terminal → the board landed" would again proceed on a silently stale board. Every surface path now emits a positive line, and the contract states plainly that **no line at all, or a non-zero exit, is a FAILURE**: must-land callers stop and surface it. **Silence is never evidence of success** — the same rule ADR-0028 states for the report channel, applied to its own totality.

**What it costs.** One reserved token in the config vocabulary (`none`), a value that can never mean a surface; a config resolver that must synthesize a token rather than pass a list through; and three scripts that now hard-fail on an input they previously tolerated (a caller passing `--surfaces ""` on purpose, if any existed, breaks loudly — which is the point).

**The durable rule this ADR records — it generalizes well past the board:**

> **A deliberate off-state must be encoded positively. Absence and emptiness are reserved for error.**

Had this rule been in force the day change 0059 shipped, it would have caught this bug in review.

Related changes: 0059 (the reversed decision — the gated board writer and its empty-means-disabled contract), 0070 (where the stale board was hit live), 0068 (the `docket.sh` facade the Board pass now routes through).
