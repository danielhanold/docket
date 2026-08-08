---
id: 78
slug: parent-facing-gate-surface-for-claude-one-physical-instructions-file
title: The parent-facing gate surface for Claude Code, and the one-physical-instructions-file symlink policy
status: Accepted
date: 2026-08-08
supersedes: []
reverses: []
relates_to: [24]
change: 242
---

## Context

Change 0237 built `docket.sh verify-run` — a pure git reader of `docket-implement-next`'s Step 7
postcondition — and called it from `runner-dispatch.sh`, covering every harness whose autonomous
runs are CLI-driven. **Claude Code was uncovered**: it forks skills natively via ADR-[[0024]]'s
`context: fork` frontmatter and never traverses `runner-dispatch.sh`. All six observed instances of
the half-run family (0109, 0194 ×2, 0206, 0231, 0235) occurred on that path.

The structural fact any fix must respect: a check placed **inside** the skill or its wrapper is
executed by the same agent that is failing, and an agent that stops at a step boundary also stops
before its own "now verify yourself" step. The check must run in the context that regains control
**after** the failing agent stopped — the **parent session**, at the moment the fork's report
returns. That requires a parent-facing, always-loaded instruction surface, and for Claude docket
generated none, deliberately: ADR-[[0024]] chose native per-skill `context: fork` frontmatter
explicitly as "no generated file, no hook, no CLAUDE.md routing".

## Decision

**1. Routing and surface are different questions; ADR-[[0024]] decided only the first.**
ADR-[[0024]] governs how an invocation **routes** to its pinned wrapper. This ADR governs what the
parent **does after** a routed run returns. `context: fork` frontmatter is untouched — it is the one
harness-enforced link in the chain, and it guarantees the parent/child separation the gate's
soundness rests on. In 2026-07 there was no oracle to consult, so 0024 never faced this question.
Hence **parallel**, not amending, superseding, or reversing.

**2. The surface is one physical instructions file.** Claude Code's documented always-loaded surface
is `CLAUDE.md`, and delivery targets the documented surface rather than betting that a given version
also reads `AGENTS.md` (the *harness-behavior-is-mode-and-version-scoped* finding). When `claude` is
an enabled harness, `sync-agents.sh` resolves `CLAUDE.md` to a **physical file**:

- an existing `CLAUDE.md` file **or symlink** resolves to its physical path;
- `CLAUDE.md` absent with `AGENTS.md` present → create `CLAUDE.md` as a **committed relative symlink
  to `AGENTS.md`**;
- neither present → seed a real `CLAUDE.md`.

The managed `docket:dispatch` block is then written **once per distinct physical file**
(resolved-path dedupe), so a symlinked pair never receives two diverging copies.

The symlink was chosen over a generated `@AGENTS.md`-import stub (import support is itself
harness-version-scoped) and over a standalone block-only `CLAUDE.md` (which would leave the promoted
learnings undelivered to Claude sessions and fork the gate text into a second physical file).

## Consequences

- **Claude sessions finally receive everything `AGENTS.md` carries** — including the promoted
  learnings tier, which was verified undelivered to Claude sessions before this change.
- **A committed symlink is a new repo-root artifact.** Fine on macOS/Linux git; a Windows checkout
  without symlink support would materialize it as a text file. Accepted for a solo-maintainer macOS
  project; recorded so a future contributor-facing repo can revisit.
- **The gate is prose a model follows**, and the half-run family exists precisely because prose
  degrades. It is differently positioned from the six failed levers on three axes: addressed to the
  **parent** (the non-failing agent, whose remaining job is short), **a handful of mechanical
  commands** rather than a multi-step behavioral contract, and **transcript-verifiable** (a degraded
  gate shows as a missing command, not a plausible summary). Residual risk is accepted explicitly;
  `board-checks.sh`'s `aborted-run` floors stand regardless.
- **A Claude Code `Stop`/`SubagentStop` hook remains rejected**, and is the recorded escalation path
  if the caller-side gate is observed degrading. It was rejected on weight: user-level registration
  intercepts every turn end and subagent completion machine-wide, including non-docket work, and
  couples docket to unowned, version-mobile harness surface.
- **Build-time finding worth recording:** docket's own repo opt-in lives in the gitignored,
  machine-local `.docket.local.yml`, so this repo's own dispatch surface could never have been
  generated or checked by CI. The change adds a suite assert comparing the committed block against a
  freshly assembled one.
