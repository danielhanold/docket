---
id: 242
slug: close-the-claude-gap-in-the-run-completion-gate-with-a-comma
title: Close the Claude gap in the run-completion gate with a caller-side verify in the dispatch rules
status: in-progress
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
branch: feat/close-the-claude-gap-in-the-run-completion-gate-with-a-comma
claimed_at: 2026-08-08T15:38:40Z
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

Two pieces, settled at the post-reconcile re-groom (2026-08-08):

**The surface** — the reconcile's finding was that no parent-facing generated file exists for
Claude (ADR-0024 solved routing natively, so none was ever needed). `sync-agents.sh` now
creates it: when `claude` is enabled, the parent-facing block targets `CLAUDE.md`'s `realpath`
if the file exists, else creates `CLAUDE.md` as a committed symlink to `AGENTS.md` (one
physical instructions file — this also finally delivers the promoted learnings to Claude
sessions, verified undelivered today), else seeds a fresh `CLAUDE.md`. Blocks are written once
per distinct physical file. Routing (`context: fork`) is untouched; a new parallel ADR records
the surface-vs-routing distinction and the symlink policy.

**The gate** — the shared dispatch-block template (assembled into the `AGENTS.md` block, the
cursor rule, and the Claude surface) grows 0237's gate shape executed by the parent: snapshot
the in-progress set before dispatching (`verify-run --in-progress-ids`), diff after the fork
returns to attribute this run's claim, `verify-run <id>`, and on `run-incomplete` one bounded
re-dispatch with the unmet conjuncts, then stop-and-report loudly. `run-halted` never
re-dispatches. Same oracle, discriminator, and cap as the runner-side gate; every step a single
transcript-visible facade command. Plus a one-sentence pointer in the convention's
*Composition* prose. Full design in the linked spec.

## Out of scope

- Re-deriving 0237's verdict logic. This wires to the existing oracle or it is not worth doing.
- Any Claude Code hook (rejected — recorded in the spec), any `settings.json` or installer work.
- Any loop-specific mechanism or documentation; any headless-Claude runner adapter.
- Any change to `verify-run.sh` / `runner-dispatch.sh` / `board-checks.sh`; any metadata write
  by the gate; any new config knob.

## Reconcile log

### 2026-08-08 — reconcile (docket-implement-next), halted

Reconciled against current `sync-agents.sh`, `cursor-rules/`, `scripts/verify-run.{sh,md}`,
`scripts/lib/docket-gitignore-block.sh`, and ADR-0024. `verify-run` is present and matches the
spec's assumptions (`--in-progress-ids`, `--with-claimed-at`, the four report lines,
callers-key-on-the-line). Dependency 0237 is `done`.

The reconcile located the dispatch-rule sources the spec deferred to build time:

- `cursor-rules/dispatch/docket-implement-next.md` → assembled by `assemble_dispatch_rule()` into
  `<root>/.cursor/rules/docket-dispatch.mdc`, written only for harnesses in
  `HARNESS_HAS_DISPATCH_RULES`, which is `DOCKET_GI_DISPATCH_HARNESSES` =
  `"cursor"` (`scripts/lib/docket-gitignore-block.sh:10`).
- `assemble_agents_md_dispatch()` → the managed `docket:dispatch` block in the repo-root
  `AGENTS.md`, written only when a harness in `AGENTS_MD_DISPATCH_HARNESSES` = `"codex opencode"`
  is targeted.

Neither reaches Claude, and no third generated surface does: `sync-agents.sh`'s claude path emits
**wrapper files only** (`.claude/agents/docket-*.md`), read by the subagent, not by the parent
session. `ensure-claude-settings.sh` writes a permission allow-rule, not instructions. This is not
an oversight to patch around — it is a recorded decision: ADR-0024 chose native per-skill
`context: fork` frontmatter for Claude Code explicitly as "**no generated file, no hook, no
CLAUDE.md routing**", and states `HARNESS_HAS_DISPATCH_RULES` stays **Cursor-only**.

See `## Run halted` for what this means for the design and what a human must decide.

## Run halted

Halted 2026-08-08 by `docket-implement-next` at Step 3 (reconcile), before any branch was cut.
Nothing was built; the claim is left intact with `claimed_at` refreshed.

**What stopped the run.** The spec's Decision 1 — the mechanism selection the whole design rests
on — is built on a premise that is false for the one harness this change exists to cover. It reads:
"The agent layer already generates, per harness, the dispatch rule that routes a directly-invoked
`docket-implement-next` to its pinned wrapper (`sync-agents.sh`, into each harness's
agent-instructions file). That rule — read by the *parent* session, not the fork — grows the gate."

There is no such file for Claude. Generated dispatch-rule surfaces exist for exactly two harness
sets — `cursor` (`.cursor/rules/docket-dispatch.mdc`) and `codex`/`opencode` (the `AGENTS.md`
`docket:dispatch` block) — and Claude is deliberately in neither, per ADR-0024 ("no generated file,
no hook, no CLAUDE.md routing"; `HARNESS_HAS_DISPATCH_RULES` stays Cursor-only). The evidence is in
the `## Reconcile log` entry above.

**Why this is not a scope adjustment.** Building the spec as written is not merely incomplete — it
is actively harmful. It would install the gate into precisely the three harnesses whose autonomous
runs `runner-dispatch.sh` already gates, leave the Claude path exactly as uncovered as it is today,
and mark the change `done` under a title that claims the Claude gap is closed. The change's `## Why`
rests on all six observed half-runs having occurred on the Claude path; a delivery vehicle that
cannot reach that path manufactures false coverage of the exact failure family docket built
`verify-run` to catch. (The rule would still carry real value on the cursor/codex/opencode
*interactive* paths — but that is a different, smaller change than this one.)

**What a human must decide.** Which parent-facing surface, if any, docket is willing to own for
Claude Code. The reconcile pass cannot default this, and it is a re-brainstorm rather than a
refinement, because every candidate reopens a decision already recorded:

1. **A managed `CLAUDE.md` block** (or adding `claude` to `AGENTS_MD_DISPATCH_HARNESSES`, if the
   installed Claude Code is confirmed to read `AGENTS.md`). Mechanically the closest fit and the
   only option that keeps the spec's shape — but it is a *new* generated surface, which the spec's
   §3 forswears ("adding no new mechanism"), and it contradicts ADR-0024's "no CLAUDE.md routing"
   in letter. That ADR decided the question for *dispatch routing*, not for a caller-side verify
   gate, so the contradiction may be only apparent — but distinguishing the two is a decision, and
   if the answer is yes it needs ADR-0024 amended or a new ADR alongside it.
2. **Escalate to the rejected Stop/SubagentStop hook**, already worked out in the spec's
   *Rejected* section and named there as the escalation path. Rejected the same day on weight, not
   on soundness; the finding here is new information bearing directly on that trade — the
   lighter-weight alternative it was rejected in favour of does not reach Claude at all.
3. **Re-scope 0242 to the three harnesses it can actually reach** and open a separate change for
   Claude. Honest and immediately buildable, but it renames the change and empties its `## Why`.
4. **Accept the gap** and rely on `board-checks.sh`'s `aborted-run` floors, killing 0242.

Recommended: re-brainstorm via `docket-new-change` / `superpowers:brainstorming` with option 1
against option 2 as the live trade, since 0237's oracle makes either cheap to wire once the surface
is settled.
