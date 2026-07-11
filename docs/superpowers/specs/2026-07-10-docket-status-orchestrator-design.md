# docket-status orchestrator — one script call per status pass

**Date:** 2026-07-10
**Change:** #0058
**Status:** Approved design (brainstormed with Daniel; approach and fast mode confirmed)

## Problem

A `docket-status` run takes several minutes and costs ~$1 per run in Cursor. Measurement shows
the cost is dominated by **model round-trips**, not token bloat alone:

- Every Bash call is a full model turn that re-sends the whole context (docket-convention +
  docket-status skill + system prompt + conversation ≈ 20–30k input tokens per turn).
- A no-op run (nothing to sweep, board unchanged) is ~10–15 turns: skill load → config export →
  worktree sync → render-board → diff check → commit/push → board-checks → model-driven checks →
  report.
- Each merged change adds ~6–8 turns to the sweep (per-change `gh` query, archive, re-render,
  commit, push, publish, cleanup, harvest).
- ~10 turns × ~30k tokens ≈ 300k+ input tokens → ~$1 in Cursor; wall clock = turns × model
  latency.

Changes #0053–#0055 (in flight) attack the **tokens-per-turn** dimension by slimming the skill
bodies. This change attacks the orthogonal **turn-count** dimension. ADR-0012 already names the
motivation: *"a model turn re-sends the whole skill-and-convention context on every step."*
Every step of the status pipeline except two (harvest-learnings, `blocked_by:` free-text review)
is mechanical, and every sweep sub-step is already a deterministic script — the model is an
expensive glue layer invoking them one turn at a time.

## Decision summary

1. **Full orchestrator** — new `scripts/docket-status.sh` runs the entire deterministic status
   pipeline in ONE invocation and emits one compact structured report.
2. **`--board-only` fast mode** — sync + render + commit/push only, for the interactive
   "just show me the board" case.
3. **One new ADR** relating to ADR-0012: deterministic pipelines may author formulaic,
   template-generated commit messages and mutate state along an already-blessed script sequence;
   judgment-bearing prose stays model-authored.
4. **Skill rewrite** — `docket-status` SKILL.md becomes: Step-0 preamble → invoke orchestrator →
   surface report → judgment-only follow-ups. Depends on #0053's slimmed body landing first.

Expected effect: no-op run ≈ 2–3 turns (from 10–15); sweep run ≈ 4–5 turns plus harvest
(from ~20–35+). Cost scales down proportionally on any model/harness; wall clock drops from
minutes to roughly the script runtime plus 2–3 model turns.

## `scripts/docket-status.sh`

New orchestrator + co-located contract `scripts/docket-status.md`. Sequence (full pass):

1. **Config + bootstrap.** Internally `eval` `docket-config.sh --export` (fail-closed). Act on
   `BOOTSTRAP`: `PROCEED` continues; `STOP_MIGRATE` prints the migrate remedy and exits non-zero;
   `CREATE_ORPHAN` is NOT auto-taken — print the verdict and exit non-zero (bootstrap opt-in
   stays a model/human decision, per the convention).
2. **Metadata worktree.** Ensure the `.docket/` worktree (idempotent, state-specific create per
   the convention's Branch model) and sync it (`fetch` + `pull --rebase origin docket`).
   `main`-mode degrades to the primary tree, same as the skill prose today.
3. **Board pass.** For each surface in `board_surfaces`: `inline` → `render-board.sh` into
   `BOARD.md`; if changed, commit (templated message) and push with the pull-rebase retry loop —
   on a `BOARD.md` conflict, **regenerate, never 3-way merge** (the existing rule, now in code).
   `github` → `github-mirror.sh`, best-effort; `issue-minted` / `project-minted` lines are passed
   through to the report for the model to record (the write-back stays a metadata edit the model
   owns today; see Output contract).
4. **Sweep detection (batched).** Collect every `implemented` change; resolve merge state +
   `mergedAt` for all of them in **one batched `gh` call** (GraphQL aliases keyed on `pr:`
   numbers; `gh pr list --head` fallback for changes with `pr:` unset). Replaces today's
   per-change queries. `gh`/network failure ⇒ sweep detection reports `sweep-skipped` and the
   pass continues (best-effort, self-heals next run).
5. **Sweep execution.** Per merged change, chain the existing jointly-owned primitives, in the
   #0035-guarded order: `archive-change.sh --outcome done` → `render-change-links.sh` re-render
   as a separate follow-on commit, pushed → `terminal-publish.sh` → `cleanup-feature-branch.sh`.
   Per-change failure posture is the sweep's **log-and-continue**: a failed step abandons the
   remainder of that change's close-out (a failed re-render skips publish) and moves on. No
   mechanics are duplicated — the orchestrator only sequences the shared scripts, preserving
   ADR-0012's joint-ownership rule and the determinism invariant (change-file-only archive
   commits, UTC merge dates, no `now()`).
6. **Health checks.** Run `board-checks.sh`; pass its TSV findings through verbatim.
7. **Integration sync.** `sync-integration-branch.sh` once at the end (best-effort, FF-only) —
   only when at least one change was swept, matching today's skill.

`--board-only` runs steps 1–3 and exits. Flags: `--board-only`, plus passthroughs the underlying
scripts need (`--repo` for board links; `--project`/`--auto-create-project` for the github
surface). Everything else comes from the config export.

### Output contract

Compact line-oriented report on stdout, designed to be the model's ONE tool result:

```
board    inline   changed|clean   pushed|push-failed
board    github   ok|skipped|failed
minted   issue    <change-id> <issue-number>        # pass-through from github-mirror.sh
swept    <id>     <merge-date>
sweep-failed <id> <step> <one-line reason>
sweep-skipped <reason>                              # batched gh detection unavailable
check    <check-id>  <change-id>  <message>         # board-checks.sh TSV, prefixed
harvest  <id>    <archived-path>                    # swept changes awaiting learnings harvest
judgment blocked <id> <blocked_by text>             # free-text blocker review for the model
```

Exit codes: `0` = pass completed (findings/warnings allowed); non-zero = hard error only
(config/bootstrap failure, metadata worktree unusable). Stderr carries diagnostics; stdout stays
machine-parseable.

### What stays in-model (ADR-0012 judgment rule, unchanged)

- **Harvest-learnings** per swept change (reads PR/plan/results, writes curated LEARNINGS.md
  entries) — driven off the report's `harvest` lines; still best-effort.
- **`blocked_by:` re-examination** — driven off the `judgment` lines.
- **`issue:` / `github_project` write-backs** after a fresh mint (metadata edits + push, normal
  discipline).
- The final human-facing summary.

## New ADR — templated commit messages in deterministic pipelines

Relates to ADR-0012 (does not supersede it — the script/judgment boundary stands). Decision:
a deterministic pipeline script may (a) author **formulaic commit messages from fixed templates**
(board refresh, archive, artifacts re-render) and (b) mutate state, provided the mutation follows
an already-blessed script sequence with the failure posture of its calling skill. Judgment-bearing
prose (harvest entries, kill reasons, PR bodies) stays model-authored. This legitimizes what
`archive-change.sh` already half-does (it commits and pushes; only the `--message` came from the
model) and removes the last reason the model had to sit between mechanical steps.

Message templates (fixed, deterministic — concurrent runs converge byte-identically where the
determinism invariant requires it): e.g. `docket(NNNN): done — archived (status done, <date>)`
for the archive (matching the existing convention the model uses today), `docket: board refresh`
for the board, `docket(NNNN): re-render artifacts block` for the follow-on.

## Skill rewrite — `docket-status` SKILL.md

Rewritten on top of #0053's slimmed body (hence `depends_on: [53]`):

- Step-0 preamble (convention pointer + config eval) — unchanged shape.
- **Mode choice:** the user asked only to *see* the backlog ⇒ `--board-only`; anything else
  (explicit refresh, implement-next step-0 safety net, post-merge cleanup) ⇒ full pass.
- Invoke `docket-status.sh`; trust exit code; surface the report.
- Judgment follow-ups from `harvest` / `judgment` / `minted` lines.
- The Board/Sweep/Health-check prose sections reduce to short descriptions pointing at
  `scripts/docket-status.md` as the executable source — same pattern #0053 used for
  `render-board.sh`'s Structure section.

Callers need no change: `docket-implement-next` step 0 and finalize's board refreshes dispatch
the same skill/agent; they simply get faster. The sweep-vs-finalize division is untouched —
finalize still owns the merge gate; the orchestrator's sweep only archives already-merged PRs.

## Testing

- Hermetic tests for `docket-status.sh` per the existing script-test pattern: no-op run
  (clean tree ⇒ `board inline clean`, exit 0), board-change run, `--board-only` skips sweep +
  checks, batched detection with mixed merged/unmerged PRs (mocked `gh`), per-change
  log-and-continue on a failing step (failed re-render must skip publish), `STOP_MIGRATE` and
  `CREATE_ORPHAN` exits, `main`-mode degradation.
- Determinism: two concurrent sweep runs converge (idempotent re-run is a byte-identical no-op).
- Post-rewrite smoke run of the skill end-to-end in this repo.

## Out of scope

- Model/effort re-pinning per harness (`agents:` config — a Cursor repo can independently pin
  `status` to a mini-tier model; note it in the README if anywhere).
- `docket-finalize-change`'s own flow (#0054 owns its slimming; the merge gate is untouched).
- Any convention/lifecycle semantics change; the board format; `render-board.sh` /
  `board-checks.sh` internals.
- Turn-count work on other skills (a follow-up can reuse the pattern if this proves out).

## Open questions

- Whether the batched detection uses one GraphQL query with aliases or falls back to N
  `gh pr view` calls inside the script (still one model turn either way) — decide at plan time
  by testing `gh api graphql` ergonomics.
