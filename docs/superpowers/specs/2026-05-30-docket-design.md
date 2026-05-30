# docket — design spec

**Date:** 2026-05-30
**Status:** Draft (awaiting review)
**Author:** Daniel Hanold
**Lineage:** OpenSpec (Fission-AI) · superpowers (obra) · superspec (danielhanold)

---

## 1. Purpose & positioning

**docket** is a portable, harness-neutral skill set that adds a *change-lifecycle and project-management layer* on top of the [superpowers](https://github.com/obra/superpowers) skills. It is the lightweight, OpenSpec-free cousin of [superspec](https://github.com/danielhanold/superspec): it keeps superspec's execution spine (which is just superpowers underneath) but drops OpenSpec's CLI, schema, and strict-markdown contract.

The problem it solves: a project accumulates planned work, but there is no durable, queryable record of *what is planned, what is in flight, what shipped, and what was abandoned* — and no clean way to run a "fill the backlog" agent and a "drain the backlog" agent in parallel. superpowers gives you excellent *execution* (brainstorm → plan → TDD → review → merge) but has no concept of a tracked backlog or a "done" state. OpenSpec/superspec give you that lifecycle but at the cost of a CLI dependency and a rigid markdown contract (mandatory `SHALL`/`MUST`, exactly-4-hashtag scenarios that fail silently).

docket takes the middle path: **a thin lifecycle layer expressed as plain files + a few skills, reusing superpowers wholesale for the actual work.**

### One-line definition

> A **change** is a self-contained, tracked unit of planned work (≈ one PR). docket records each change as a single markdown file with a status lifecycle, and provides three skills to propose changes, work the next change to a PR, and report the board — all coordinated through git, no CLI or database.

---

## 2. Lineage — what docket takes from each parent

| Concern | OpenSpec / superspec | superpowers | **docket** |
|---|---|---|---|
| Unit of planned work | `change` (a folder) | (none) | **`change`** — one markdown file |
| Design artifact | `design.md` / brainstorm | `docs/superpowers/specs/…` | **superpowers' spec, linked by path** (not copied) |
| Implementation plan | `tasks.md` + `plan.md` | `docs/superpowers/plans/…` | **superpowers' plan, linked by path** (not copied) |
| Execution | `/opsx:apply` → subagents | `subagent-driven-development` (TDD + review) | **superpowers, dispatched directly** |
| "Source of truth" / decisions | living `specs/` (mutable) | (none) | **ADRs** — an *immutable* decision ledger the change cites |
| Lifecycle / "done" | dir location + `archive/` + receipts | (none) | **`status` field + `active/`↔`archive/` move** |
| Validation contract | strict markdown (SHALL/MUST, `####`) | (none) | **none** — prose proposal, no silent-fail rules |
| Packaging | OpenSpec CLI + YAML schema | skills | **skills only** (no CLI, no schema) |

The key conceptual divergence from superspec: docket has **no living-spec layer**. superspec/OpenSpec continuously fold deltas into a mutable `specs/` tree. docket instead treats **the code as the current-state truth** and **ADRs as the immutable decision ledger** — append-only, dated, superseded-not-rewritten. A change *cites* ADRs and *produces* new ones; it never maintains a parallel behavior spec.

---

## 3. Locked decisions

These are the outcomes of the design brainstorm, each with its rationale. They function as the spec's mini-ADRs.

1. **Portable, authored as a harness-neutral skill set — not a Claude Code plugin.** The skill (`SKILL.md`) format runs natively across Claude Code, Codex, and Cursor; superpowers is a *declared prerequisite* present on every harness (installing it is out of scope for docket). Therefore docket needs no per-harness adapters and no capability-binding layer — it calls `superpowers:*` skills directly. A Claude Code plugin, if ever made, is only an optional distribution wrapper, not the substance.

2. **The unit is a `change`** (kept from OpenSpec/superspec vocabulary, for one shared mental model across the author's tools). The superpowers micro-step **plan** is therefore an *artifact inside/attached to* a change, never the unit itself — which resolves the overloading of the word "plan."

3. **`docs/changes/` is project-management only; superpowers output is never redirected.** `brainstorming` and `writing-plans` write to their native homes (`docs/superpowers/specs/`, `docs/superpowers/plans/`). The change file *links* to them by path. This keeps superpowers untouched and the change file thin.

4. **One file per change.** Manifest frontmatter + a full why/what/scope body. No per-change folder, no duplicated design/plan files. Reconcile notes live in a `## Reconcile log` section.

5. **Lifecycle is a `status` field + a single archive move.** Seven states; `active/` holds non-terminal, `archive/` holds terminal (`done`, `killed`). The happy-terminal state is **`done`** (a deliberate "definition of done"); the `archive/` directory keeps its name (it is the *location* for finished changes, both `done` and `killed`). The only physical file move happens once, on the terminal transition — so parallel agents never race on `git mv`. Finer state (blocked/deferred) is a field value, not a directory.

6. **Three skills:** `docket-propose` (producer), `docket-next` (implementer), `docket-status` (board + janitor).

7. **Two agents coordinate purely through committed manifests in git** — no locks, no database.

8. **Semi-autonomous, human gate at the PR.** The implementer runs the whole spine solo per change and stops at an open PR for human review/merge.

9. **Skills are self-contained — convention duplicated, not shared.** Each `SKILL.md` embeds the convention (directory layout + manifest schema + lifecycle) inline as a marker-delimited `## Convention` block, and `docket-propose` carries its own change template *inside its skill folder*. Nothing a skill needs lives outside its own directory, because skills are distributed by copying/symlinking that directory — an external `references/CONVENTION.md` would not travel with them. The duplication across the three skills is accepted; an optional `sync-convention.sh` propagates edits from one canonical block to keep the copies in step.

10. **A `link-skills.sh` convenience script** symlinks the three skill directories into the **global** agent-harness skill dirs (`~/.claude/skills/`, `~/.cursor/skills/`, `~/.kiro/skills/`, `~/.windsurf/skills/`, `~/.agents/skills/`) with absolute symlinks back to `~/dev/docket/skills/<name>`. So the source of truth stays in `~/dev/docket`, docket installs once, and the skills are available in every project without copying. Modeled on `~/dev/obsidian-wiki/link-skills.sh` (idempotent: only creates missing links, leaves existing ones alone).

---

## 4. Architecture — the skill set

```
docket/
  skills/
    docket-propose/
      SKILL.md             # producer: idea → proposed change (+ opt-in scan mode)
      change-template.md   # the change-file stub (travels with the skill)
    docket-next/
      SKILL.md             # implementer: pick → reconcile → build → PR → stop
    docket-status/
      SKILL.md             # board render + merge-sweep janitor + health checks
  link-skills.sh           # symlink the skill dirs into agent-harness skill dirs
  sync-convention.sh       # (optional) propagate the shared Convention block across skills
  README.md                # what docket is, install, the one prerequisite (superpowers)
  docs/                    # design spec etc. — repo-only, never copied into a harness
```

Each skill is a **self-contained** directory: it embeds the convention (directory layout + manifest schema + 7-state lifecycle) inline as a marker-delimited `## Convention` section, and `docket-propose` carries its own `change-template.md`. Nothing a skill needs lives outside its own folder — because skills are distributed by copying or symlinking their directory, and a shared external file (e.g. a top-level `references/CONVENTION.md`) would not travel with them. The convention is therefore duplicated across the three `SKILL.md` files; that duplication is accepted, and an optional `sync-convention.sh` propagates edits from a single canonical block to keep the copies in step.

**Distribution.** The skills are the portable, install-once unit (symlinked into the harness skill dirs by `link-skills.sh`, or copied); the change *data* (`docs/changes/`) lives per consuming project. So docket-the-tool is installed once and works in every repo; docket-the-backlog is local to each repo.

**Authoring guideline for portability:** keep docket's own tool surface to the lowest common denominator — read/write a file, run `git`/`gh`, invoke a skill — and let superpowers carry everything heavier. The more docket leans on superpowers for real work, the less any per-harness tool-name drift can affect it. (superpowers already ships internal tool-name mappings for non-CC harnesses.)

---

## 5. Data model

### Directory layout (in the consuming project)

```
docs/changes/
  active/
    0007-quicklook-interactions.md       # <id>-<slug>.md
    0008-onboarding-tour.md
  archive/
    2026-05-30-0004-quicklook-extension.md   # YYYY-MM-DD-<id>-<slug>.md (date = archival date)
  README.md                              # generated status board, spans active + archive
```

Base path is the one configurable knob; default `docs/changes/`.

### Manifest (frontmatter at the top of each change file)

```yaml
---
id: 7                     # integer; zero-padded to 4 digits in the filename
slug: quicklook-interactions
title: Quick Look interactions — external links + local images
status: proposed          # proposed | in-progress | blocked | deferred | implemented | done | killed
priority: 2               # 1 = highest
created: 2026-05-30
updated: 2026-05-30
depends_on: [4]           # change ids that must reach `done` before this can start
related: [4, 6]           # cross-links the reconcile pass reads
adrs: [24]                # ADRs this change cites or produces
spec:                     # path to the superpowers design doc; set when brainstormed
plan:                     # path to the superpowers plan; set when planned
branch:                   # set on claim
pr:                       # set when the PR is opened
blocked_by:               # free text; set only when status: blocked
reconciled: false         # set true after the just-in-time reconcile pass
---
```

### Body sections

- `## Why` — the motivation, as detailed as warranted (no length limit).
- `## What changes` — scope of the work.
- `## Out of scope` — explicit non-goals.
- `## Open questions` — unknowns to resolve during reconcile/design.
- `## Reconcile log` — dated entries appended by the implementer's reconcile pass.
- `## Why deferred` / `## Why killed` — added when entering those terminal/side states.

The change body is a *PM-altitude proposal* (intent + scope). The detailed design lives in the linked superpowers spec; the task breakdown in the linked superpowers plan. Different zoom levels, no duplication.

---

## 6. Lifecycle

```
                         ┌──────────────── deferred ──────────────┐
                         │ (conscious shelve; revive → proposed)   │
                         ▼                                          │
  proposed ──claim──▶ in-progress ──PR open──▶ implemented ──merge+sweep──▶ done
     │                    │                                                  (archive/)
     │                    └──blocker──▶ blocked ──clears──▶ in-progress
     │
     └────────────── killed (never to be built; → archive/) ─────────────────▶
```

| status | meaning | directory |
|---|---|---|
| `proposed` | drafted, awaiting work | `active/` |
| `in-progress` | claimed, being built | `active/` |
| `blocked` | external blocker (`blocked_by:`) | `active/` |
| `deferred` | consciously shelved, may revive | `active/` |
| `implemented` | built, PR open — **human merge gate** | `active/` |
| `done` | PR merged, filed away (happy terminal) | `archive/` |
| `killed` | abandoned, never built (sad terminal) | `archive/` |

**Rules.** `active/` holds every non-terminal status; `archive/` holds the two terminal outcomes. The single physical move (`active/ → archive/`, with a `YYYY-MM-DD-` date prefix) happens exactly once, on the terminal transition. Clearing a blocker or reviving a deferred change is a one-line frontmatter edit, no move. A change whose `depends_on` is unsatisfied is *implicitly* blocked — the implementer's selector skips it without a status change; reserve explicit `blocked` for external blockers the system can't infer.

---

## 7. The three skills

All three own only the lifecycle (change files + board) and dispatch superpowers for the work.

### 7.1 `docket-propose` — the producer

Turns an idea into a new change file. **Only ever creates new `proposed` ids**, so it structurally cannot collide with the implementer.

**Formalizer mode (default):**
1. **Allocate** — scan `active/` + `archive/` for the max `id`, increment; derive `slug` from the title.
2. **Recon** — scan neighbouring changes (active + recent archive) and the project's ADR index to pre-fill `related`, `depends_on`, `adrs`.
3. **Draft** — write `active/<id>-<slug>.md` from the skill's bundled `change-template.md`: frontmatter (`status: proposed`, dates, priority) + body.
4. **Design (optional)** — if the idea is fuzzy/large, dispatch `superpowers:brainstorming` → writes natively to `docs/superpowers/specs/`; record the path in `spec:`. Crisp/small → skip; the implementer can brainstorm at build time.
5. **Board + commit** — refresh `README.md`, commit. **Stops. Never implements.**

**Scan mode (opt-in, explicitly triggered):** survey TODOs, deferred changes, known gaps, and the ADR backlog, and propose several `proposed` changes in one pass. Same allocate/draft/commit machinery, batched. Kept opt-in so routine runs don't generate speculative noise.

### 7.2 `docket-next` — the implementer

Picks the next best change and drives it to a PR, then stops at the human gate.

0. **Sync & sweep** — `git pull`; invoke the `docket-status` merge-sweep so any `implemented` change whose PR has merged is swept to `archive/` (status → `done`) first (self-cleaning loop).
1. **Select** — among `active/` changes that are `proposed` with all `depends_on` satisfied, rank by `priority` → readiness → age; pick the top (or accept an explicit id). Skip `in-progress`/`blocked`/`deferred`.
2. **Claim** — re-read the manifest after the pull (avoid double-claim), set `status: in-progress` + `branch`, `updated`, commit. *That commit is the lock.* Spin up the worktree (`superpowers:using-git-worktrees`).
3. **Reconcile** ⭐ — re-read the change against `related` + recently-archived changes, cited + recent ADRs, and current code. Rewrite the body (and the linked `spec`, if any) to what is true *now*: drop work already done elsewhere, adjust scope, fold in new constraints. Append a dated `## Reconcile log` entry; set `reconciled: true`; commit. **If reconcile finds the change is now obsolete, transition it to `killed` (with `## Why killed`) instead of building** — and loop back to Select.
4. **Design** — if no `spec:` and there are real design questions → `superpowers:brainstorming` (native `specs/`), record path. Mechanical change → skip.
5. **Plan** — `superpowers:writing-plans` (native `plans/`), record path in `plan:`.
6. **Build** — `superpowers:subagent-driven-development` executes the plan task-by-task with TDD + per-task review.
7. **Review + ADRs** — `superpowers:requesting-code-review` (whole-branch); write ADRs for non-obvious decisions and append their numbers to `adrs:`.
8. **PR + stop** — `superpowers:finishing-a-development-branch` (PR mode) opens the PR; set `status: implemented`, `pr:`, commit. **Stops.** The change stays in `active/` as `implemented` until a human merges.

### 7.3 `docket-status` — the board & janitor

The queryable state plus housekeeping; run it to see "what's done, what's next, what's stuck."

- **Board** — scan `active/` + `archive/`, render `README.md` grouped by status, with id/title/priority/deps/branch/PR/spec/plan links.
- **Merge sweep** — for each `implemented` change, check via `gh` whether its `pr` merged → move the file to `archive/YYYY-MM-DD-<id>-<slug>.md`, set `status: done`, commit. Closes the loop after a human merge.
- **Health checks** — flag stale `in-progress` claims (branch gone / no commits in N days), `spec:`/`plan:` paths that no longer resolve (link rot), `blocked` changes whose `blocked_by` may have cleared, and `depends_on` cycles.

---

## 8. Two-agent coordination

The whole point: run a **producer agent** (`docket-propose`, filling the backlog) and an **implementer agent** (`docket-next` in a loop, draining it to PRs) in parallel.

- The producer only mints **new `proposed` ids** — it never touches in-flight changes, so it cannot collide with the implementer.
- The implementer **claims atomically**: `git pull` → re-read manifest → set `in-progress` → commit. Anything already non-`proposed` is skipped. Deterministic ordering (lowest eligible id) keeps two implementers, if ever run together, off the same change.
- Shared state is **the committed change files** — git is the coordination medium. No lock files, no database. Worktrees isolate the file changes during build.

**Where lifecycle commits land.** The change-file edits that form the lock and feed the board (claim, reconcile, status transitions) must be visible on a *shared* branch for coordination to work — if they rode the feature branch they would be invisible until merge, defeating the lock. Recommended: **lifecycle and board commits go to the project's integration branch** (e.g. `main`, or a dedicated `docket` branch), while the *code* for a change goes on its feature branch; the change's `pr:` field ties the two together. Projects with a strict no-commit-to-`main` policy point docket at a dedicated branch instead. This is flagged as an open item (§13) because it interacts with each project's branch policy.

---

## 9. Relationship to ADRs

ADRs are **out of scope for docket to manage** — they remain whatever the consuming project already uses (e.g. Markhaus's `docs/decisions/`): a project-wide, immutable, numbered **decision ledger**. docket only *references* them:

- a change **cites** ADRs (`adrs:`),
- the **reconcile** step **reads** relevant ADRs to refresh scope against decisions made since the change was drafted,
- implementing a change **produces** new ADRs for non-obvious decisions, appended to `adrs:`.

Clean split: **ADRs = durable "why, forever"; changes = scoped "what, now → done."** This is the precise role superspec's `design.md` plays *within* a change, promoted in docket to a first-class, project-wide, append-only log the changes draw on.

---

## 10. Error handling & edge cases

- **Claim race (two implementers):** `git pull` + re-read before the claim commit; skip non-`proposed`; lowest-eligible-id ordering. Worst case, two agents claim different changes — never the same one.
- **Reconcile finds the change obsolete:** transition to `killed` with `## Why killed`, archive, and pick the next change — don't build dead work.
- **Link rot:** `spec:`/`plan:` paths are validated by `docket-status` health checks; a broken link is surfaced, not silently ignored.
- **Board merge conflicts:** `README.md` is *generated*, so on conflict it is regenerated from the change files (source of truth), never hand-merged. Per-change files rarely conflict because each change is its own file.
- **Stale `in-progress`:** health checks flag a claim whose branch was deleted or has no recent commits, so it can be reset to `proposed`.
- **Archived files don't move their spec/plan:** the linked superpowers spec/plan stay in `docs/superpowers/` as frozen historical artifacts (like ADRs, they never move). Only the change file moves to `archive/`.

---

## 11. Portability model

- **Skills are the universal substrate.** The `SKILL.md` format runs natively on Claude Code, Codex, and Cursor; no per-harness trigger shim is needed.
- **superpowers is a declared prerequisite** on every harness; docket calls `superpowers:*` directly and uniformly. Installing superpowers is the consuming user's responsibility, not docket's.
- **Lowest-common-denominator tools** in docket's own steps (file read/write, `git`, `gh`, skill invocation); everything heavier is delegated to superpowers, which handles its own cross-harness tool mapping.
- The **convention itself** (each skill's embedded `## Convention` block + the file layout) is pure data + prose and is therefore portable by definition, independent of any harness.

---

## 12. Testing & rollout

Skills are not unit-tested like code; verification is behavioural and dogfood-driven.

- **Smoke path:** on a throwaway change, run `docket-propose` → `docket-next` → `docket-status` and confirm: file created in `active/` as `proposed`; claim flips it to `in-progress` with a branch; reconcile appends a log entry; build produces a branch + PR; status flips to `implemented`; after a (simulated) merge, the sweep moves it to `archive/` as `done`.
- **Manifest schema** is documented in each skill's embedded `## Convention` block so a change file is checkable by eye and by the `docket-status` health pass.
- **First real dogfood: Markhaus.** Migrate Markhaus's existing `docs/plans/` + `*-results.md` into `docs/changes/` — completed plans become `done` changes (with their results folded into the body), and any open work (e.g. the current `feat/quicklook-interactions` branch → change `0007`) becomes a real `in-progress`/`implemented` change. This proves the lifecycle and the PR gate end-to-end before docket is carried to other repos.

---

## 13. Out of scope for v1 / open items

- **Out of scope:** managing ADRs (the project owns those); a living-spec/behavior-contract layer (deliberately absent — code is current-state truth); an OpenSpec-style CLI or YAML schema; multi-repo coordination.
- **Open items to settle during implementation:** the branch model for lifecycle/board commits (integration branch vs dedicated `docket` branch vs feature branch) — affects the coordination guarantee; recommended approach noted in §8. Also: exact skill-name namespacing (`docket-propose` vs `docket:propose`); the board's rendered format; how `priority` is assigned/edited; whether `scan` mode reads a configurable list of "candidate sources."
- **Naming:** `docket` chosen over `speclite` / `changeflow` / `slate` / `dossier` — it captures the *queue you drain*, which is the heart of the two-agent loop.
