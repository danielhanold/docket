# docket

A change is a self-contained, tracked unit of planned work (≈ one PR). docket records each change as a single markdown file with a status lifecycle, and provides five skills to create changes, work the next change to a PR, finalize a merged change, report the board, and record architecture decisions (ADRs) — all coordinated through git, no CLI or database.

---

## What docket is

[superpowers](https://github.com/anthropics/claude-code-skills) gives Claude excellent *execution*: brainstorm → spec → plan → TDD → code-review → merge, all in one invocation. What it does not give you is a tracked backlog or a "done" state that persists across invocations. Each session starts fresh.

[OpenSpec / superspec](https://github.com/anthropics/superspec) solves that with a full lifecycle layer, but it requires a CLI dependency and a rigid markdown contract that not every project wants to adopt.

docket sits in between. It adds a thin lifecycle layer — plain markdown files in your repo, five skills, no CLI — and delegates every execution step to superpowers wholesale. The core unit is a **change** (one file, one PR's worth of work). Architecture decisions are captured separately as **ADRs** (an immutable ledger). The code is always the current-state truth; docket carries no living-spec layer and does not try to mirror the codebase in prose.

The five skills cover the full loop: create, implement, finalize, inspect, decide.

---

## Prerequisite: superpowers

docket is a lifecycle wrapper around superpowers, not a replacement for it. superpowers must be installed and available in your harness before any docket skill will function. Installing superpowers is the consuming user's responsibility; docket declares it as a prerequisite but does not bundle or install it.

During implementation, `docket-implement-next` calls these superpowers skills directly:

- `superpowers:brainstorming` — (via `docket-new-change`) for up-front design
- `superpowers:writing-plans` — to build the task plan from the spec
- `superpowers:subagent-driven-development` — to execute the plan with TDD
- `superpowers:requesting-code-review` — for a whole-branch review before the PR
- `superpowers:finishing-a-development-branch` — to push the branch and open the PR

---

## Install

```bash
git clone https://github.com/your-org/docket ~/dev/docket
cd ~/dev/docket
bash link-skills.sh
```

`link-skills.sh` creates absolute symlinks from each present harness's global skill directory back to `~/dev/docket/skills/<name>`. It only writes into harness directories that already exist on your machine; it is idempotent and safe to re-run after adding a new harness. Skills are installed once and available in every project you open.

The change data — `docs/changes/`, `docs/adrs/` — lives per consuming project, not in the docket repo itself.

**Optional per-project configuration.** Add a committed `.docket.yml` at the repo root to override defaults:

```yaml
# .docket.yml — committed; read by every docket skill at startup
metadata_branch: main        # main (default) | docket
changes_dir: docs/changes    # default
adrs_dir: docs/adrs          # default
```

`.docket.yml` is committed (not gitignored) because it governs cross-agent coordination; every clone, agent, and device needs the same values.

---

## The reconcile superpower

This is the most valuable and least obvious part of docket. It warrants a careful read.

### The problem

A change is drafted against a *snapshot* of the world: the codebase, the ADR ledger, and the other in-flight changes as they stood on the brainstorm day. In an async backlog, the implementer may pick it up a week or a month later. By then:

- Another change shipped work that this one was planning to add.
- An ADR settled a question that the spec left open — in the opposite direction.
- A dependency changed the interface this spec assumed.

Most backlog-driven systems just build the ticket as written and let the implementer discover the mismatch mid-way. The classic result: implementing something already half-done elsewhere, or building something that a later architecture decision has invalidated.

### What docket does instead

`docket-implement-next` includes a **reconcile step** (Step 3) that runs at the *last responsible moment* — after the change is claimed (so it belongs to this invocation) but before the worktree and plan are created (so no work is thrown away).

The reconcile pass re-reads the change and its spec against:

- `related` and recently archived changes (to find work already done)
- cited and recent ADRs (to find new constraints)
- current code (to find interface drift)

It then rewrites the change body and spec to what is true now: drops work done elsewhere, adjusts scope, folds in new constraints. The change file gets a dated `## Reconcile log` entry, and `reconciled: true` is set as an audit record.

Two escape hatches exist for the non-trivial cases:

- If the change is now **entirely obsolete**, it is killed and the implementer loops back to select the next one.
- If the design is **fundamentally invalidated** in a way that requires re-thinking (not just scope-trimming), the implementer stops and escalates to you. Re-brainstorming requires a human and `docket-new-change`; the autonomous implementer cannot do it alone.

### The stance

**Plans rot. Refresh them just-in-time; never trust a stale backlog.**

The `reconciled` flag is the visible proof that a change was freshened against current reality before implementation began. On any resume of an `in-progress` change, if `reconciled` is still `false` (crash or interruption), the full reconcile pass runs again before continuing.

---

## How metadata is stored (transparency)

In the default mode, docket uses **`main` as a pseudo-database**. The live project state — backlog, board, decisions — is kept as files committed directly to `main`. Change files, `BOARD.md`, and ADRs all land on the integration branch alongside your code.

This is not how most projects treat `main`. Committing non-code bookkeeping straight to the integration branch is unconventional. Teams with strict branch-protection rules or a strong preference for code-only commits on `main` will find it noisy.

The cleaner conventional answer is `metadata_branch: docket` — a separate branch that holds all PM commits, keeping `main` code-only. That mode is supported in configuration, but it is **currently underdeveloped**. In v1, the cross-branch mechanics are not fully specified: spec files live on `docket` and must be read cross-tree during implementation, reconcile pushes land on `docket` but the feature branch still cuts from `origin/main`, and there is no automatic mirror or merge from `docket` into `main`. The `docket` mode works for motivated adopters who understand the rough edges, but it is not the recommended path yet.

docket defaults to `main` mode for visibility and simplicity, with full awareness of the trade-off. Expect the separate-branch mode to improve in future versions as the cross-branch mechanics are specified and tooled.

---

## The five skills

| Skill | Role |
|---|---|
| `docket-new-change` | Producer, interactive — turns an idea into a build-ready change via brainstorming; writes markdown only, never branches or code. |
| `docket-implement-next` | Autonomous implementer — picks the next build-ready change, reconciles it against current reality, builds to an open PR, and stops at the human merge gate. |
| `docket-finalize-change` | Human close-out — merges an approved PR, archives the change to `done`, cleans up the branch and worktree, and refreshes the board. |
| `docket-status` | Board and janitor — regenerates `BOARD.md`, sweeps merged PRs to `done`, and runs health checks for stale claims, broken links, and dependency stalls. |
| `docket-adr` | Immutable decision ledger — records architecture decisions, handles supersessions and reversals, and maintains the ADR index. |

---

## Status

**v1 — default `main` mode is the supported path.**

The `docket` separate-metadata-branch mode is a documented rough edge. Cross-branch mechanics (spec reads, reconcile push, board visibility) are not fully specified for v1. Until they are, `metadata_branch: main` is the recommended and tested configuration.

Markhaus is the first planned dogfood project; a migration plan exists to move its existing changes and ADRs into the docket format.
