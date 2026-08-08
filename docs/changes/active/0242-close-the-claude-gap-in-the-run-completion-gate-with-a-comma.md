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
claimed_at: 2026-08-08T16:07:36Z
pr:
blocked_by:
reconciled: true
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

See the `## Run halted` record (removed on re-claim; preserved in git at `7a28bd1f`) for what this
meant for the design and what the human was asked to decide.

### 2026-08-08 — reconcile (docket-implement-next), resumed

The halt above was answered by the human's post-reconcile re-groom (`09422ebb`): the spec's
Decision 2 now has docket **create** the missing Claude parent surface rather than assuming one,
and Decision 3 records why that leaves ADR-0024's routing decision untouched. The `## Run halted`
record was cleared as part of the re-claim, per the claim's ownership of that transition.

Re-verified against current `origin/main` (`487bfdc5`, unadvanced since the halted pass) — every
premise the re-groomed spec rests on holds:

- `sync-agents.sh:257` `HARNESS_HAS_DISPATCH_RULES="$DOCKET_GI_DISPATCH_HARNESSES"` (= `"cursor"`)
  and `:262` `AGENTS_MD_DISPATCH_HARNESSES="codex opencode"` — Claude is in neither, as the spec
  now states rather than assumes.
- The two assembly seams the gate text must reach are `assemble_dispatch_rule()` (`:1211`, static
  head `cursor-rules/dispatch.head.md` + per-agent fragments) and `assemble_agents_md_dispatch()`
  (`:1250`, an inline `HEAD` heredoc). The gate text is **single-sourced across neither today** —
  the spec's §2 single-source requirement is therefore net-new build work, not a refactor of an
  existing shared template. Called out here because it is the design's main structural risk.
- `scripts/verify-run.sh` exposes `--in-progress-ids` / `--with-claimed-at` and the four report
  lines (`run-complete`, `run-halted`, `run-incomplete <unmet…>`, `run-unclaimed`), exiting 0
  whenever a verdict was produced — the gate keys on the line, never the exit code.
- This repo is the spec's `AGENTS.md`-only combo: `AGENTS.md` present, no `CLAUDE.md`. Building
  this change will therefore create the committed symlink in docket's own root.
- Dependency 0237 is `done`; `runner-dispatch.sh` and `board-checks.sh` are untouched consumers.

No scope drift found; no adjacent follow-up work cleared the auto-capture gates. Proceeding to
plan.
