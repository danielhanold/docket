# Codex CLI live-validation runbook — design

**Change:** 0078 · **Date:** 2026-07-15 · **Status:** approved (brainstormed with Daniel)
**Depends on:** change 0077 (Codex TOML agent generation + AGENTS.md dispatch) merged.

## Context

Change 0077 makes docket *emit* what Codex documents it reads; nobody has confirmed the
full loop actually works in OpenAI Codex CLI: skills loading, docket's bash scripts
running under Codex's sandbox, TOML agents being listed, skill→subagent dispatch, the
model/effort pin holding, and metadata writes landing on `origin/docket`. The Cursor
rollout needed exactly this kind of live verification (change 0045's live check plus the
sandbox/permissions guide), and it surfaced real gaps each time. This change produces a
**guided checklist** Daniel executes interactively in Codex CLI — chosen over `codex
exec` automation (brittle to assert subagent behavior from outside; burns API credits per
run) — with findings recorded and gaps turned into follow-up stubs.

## Reconcile update (2026-07-16, at build time)

The design holds; these current-reality corrections were folded in after change 0077 (the
dependency) and change 0079 (runner delegation) both merged. The build must honor them when
finalizing exact commands.

- **Script paths.** `sync-agents.sh` and `link-skills.sh` live at the **repo root**, not
  `scripts/`. Neither has a co-located `.md` contract; the Codex-facing doc is
  **`docs/codex/setup.md`** (new, on `main`), whose "Verifying it works" + "Restart after
  (re)generating" sections this runbook is the **live-execution counterpart** to — the
  runbook should reference and extend setup.md, not duplicate it.
- **The repo currently opts OUT.** `.docket.yml` has `agent_harnesses` commented out and no
  `.docket.local.yml` exists, so no `.codex/agents/*.toml` and no `AGENTS.md` are on disk
  today. Phase 1's first step must be the opt-in: write `.docket.local.yml` with
  `agent_harnesses: [claude, codex]` (keeps committed config clean), THEN run `sync-agents.sh`.
  The generated artifacts (9-agent `.codex/agents/docket-*.toml` set, committed `AGENTS.md`
  dispatch block) only appear after that. `sync-agents.sh --check` is a vacuous exit-0 while
  opted out, so Phase 1 asserts artifact presence directly, not just a green `--check`.
- **Native path vs. 0079 runner delegation — do not conflate.** This runbook validates docket
  running **natively inside Codex CLI**: skills load, and their bash reaches scripts via the
  **`docket.sh` facade** (`"${DOCKET_SCRIPTS_DIR:?…}"/docket.sh <op>`, change 0068) under
  **Codex's own sandbox**. Change 0079's `runner-dispatch` / `scripts/runners/codex.sh` is the
  **opposite direction** (a Claude-Code *parent* offloading a whole agent run onto `codex exec`
  as a *child*) and is **explicitly out of scope** here — Phase 2's "scripts run under Codex's
  sandbox" is the native facade path, not `runner-dispatch`.
- **Phase 4 must feed the ADR-0036 deferral.** ADR-0036 deliberately deferred the **user-level
  `~/.codex/AGENTS.md` dispatch** decision (only the project-level `<repo>/AGENTS.md` block
  exists today) to this change's live validation. Phase 4 must record the definitive evidence:
  does Codex honor the project-level `AGENTS.md` dispatch block (automatic / prompted / refused),
  and is a user-level `~/.codex/AGENTS.md` needed for globally-scoped agents? A "user-level
  dispatch is needed" finding becomes a follow-up stub — it does **not** turn this change into an
  implementation change.
- **Phase 5 needs real Codex model slugs.** The built-in wrappers carry **Claude** model IDs
  (`sync-agents.sh` even warns they may be invalid for `codex`). To genuinely prove a honored
  pin, set a real Codex slug under `agents.codex.<agent>.model` (e.g. `gpt-5.1-codex`; discover
  slugs via `codex debug models`, per setup.md) plus an `effort:` — which the TOML emitter writes
  verbatim as `model_reasoning_effort` (no `max→xhigh` remap at this layer; that remap lives only
  in the runner adapter).
- **Restart after regenerating.** Codex registers agents at process start; Phases 3–5 require
  restarting the Codex session after each `sync-agents.sh` run (already documented in setup.md).

## Deliverables

1. **The runbook** — the polished, executable checklist (this spec defines its phases
   and pass criteria; the build finalizes exact commands/prompts once 0077's generated
   artifacts exist to copy from). Lives with this change as its primary artifact.
2. **A results doc** at close-out (`docs/results/…-codex-validation-results.md`):
   per-step pass/fail, observed behavior, environment versions (Codex CLI version,
   model used, date).
3. **Follow-up stubs** — one `proposed` docket change per gap found, linked from the
   results doc.

## Runbook phases

Ordered so each phase is only meaningful if the previous one passed. Every step names
an exact command or prompt and an observable expected outcome with a pass/fail box.

### Phase 1 — Setup (outside Codex)

Create a fixture repo (or reuse the docket sandbox fixture pattern from `tests/`):
`git init` + remote, run `install.sh` and `migrate-to-docket.sh`, set
`agent_harnesses: [claude, codex]` (via `.docket.local.yml` to keep the fixture's
committed config clean), run `link-skills.sh` and `sync-agents.sh`.
**Expect:** `.codex/agents/docket-*.toml` on disk (full built-in set, valid TOML,
expected model/effort lines), the AGENTS.md dispatch block present, `~/.codex/skills`
containing the docket skill links, `sync-agents.sh --check` green.

### Phase 2 — Skills load and scripts run

In Codex CLI inside the fixture: ask Codex to list available skills, then invoke
`docket-status`.
**Expect:** docket skills appear; the convention loads; `docket.sh preflight` and the
status/board scripts execute under Codex's sandbox (this is the bash-compat smoke test —
Cursor needed a sandbox/permissions guide here, Codex likely has an analogous
approval/sandbox surface to document).

### Phase 3 — Agents load

Ask Codex to list available agents (or inspect via its `/agent` surface).
**Expect:** the `docket-*` agents from the generated TOML files are visible by name.

### Phase 4 — Dispatch honored

Directly invoke a docket skill that has a pinned wrapper (e.g. `docket-status`).
**Expect:** Codex delegates to the matching `docket-status` agent per the AGENTS.md
block rather than running the skill inline — the Cursor inline-quirk test replayed for
Codex. Record whether delegation is automatic, prompted, or refused.

### Phase 5 — Pin honored

Pin a distinctive `model` + `model_reasoning_effort` for one agent via the `agents:`
config (an OpenAI model ID, e.g. a gpt-5.x variant), re-run `sync-agents.sh`, dispatch
that agent, and have it report its own model identity.
**Expect:** the spawned agent runs the configured model/effort (the change-0045-style
live verification, which for Cursor proved arbitrary non-Claude IDs are honored).

### Phase 6 — End-to-end metadata write

From inside Codex, run a trivial `docket-new-change` stub to completion.
**Expect:** the change file commits land on `origin/docket`, the Board pass reports
`board inline changed pushed`, and the board renders the new stub. This proves the
whole producer loop — preflight, worktree sync, must-land push, board render — works
under a non-Claude harness end to end.

## Pass criteria & reporting

The validation "passes" when phases 1–3 and 6 fully pass and phases 4–5 have a
definitive observed answer (even if that answer is "Codex does not honor X") — the goal
is confirmed knowledge, not necessarily green across the board. Every failure or
surprise becomes a follow-up stub; none block writing the results doc.

## Out of scope

- Fixing anything the runbook finds (follow-up changes do that).
- Automating the runbook under `codex exec` / CI.
- Validating harnesses other than Codex (kiro/windsurf remain unvalidated tokens).
- Autonomous-loop soak testing under Codex (a later concern once basics are confirmed).

## Risks

- Codex CLI behavior and docs may drift between now and execution; the build step
  re-checks the live docs when finalizing exact commands.
- Steps run against real OpenAI billing; the runbook keeps each phase small (single
  skill invocations, one trivial stub).
