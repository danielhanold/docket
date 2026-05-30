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

> A **change** is a self-contained, tracked unit of planned work (≈ one PR). docket records each change as a single markdown file with a status lifecycle, and provides five skills to create changes, work the next change to a PR, finalize a merged change, report the board, and record architecture decisions (ADRs) — all coordinated through git, no CLI or database.

---

## 2. Lineage — what docket takes from each parent

| Concern | OpenSpec / superspec | superpowers | **docket** |
|---|---|---|---|
| Unit of planned work | `change` (a folder) | (none) | **`change`** — one markdown file |
| Design artifact | `design.md` / brainstorm | `docs/superpowers/specs/…` | **superpowers' spec, linked by path** (not copied) |
| Implementation plan | `tasks.md` + `plan.md` | `docs/superpowers/plans/…` | **superpowers' plan, linked by path** (not copied) |
| Execution | `/opsx:apply` → subagents | `subagent-driven-development` (TDD + review) | **superpowers, dispatched directly** |
| "Source of truth" / decisions | living `specs/` (mutable) | (none) | **ADRs** — an *immutable* decision ledger docket manages (`docs/adrs/`) and changes cite |
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

5. **Lifecycle is a `status` field + a single archive move.** Seven states; `active/` holds non-terminal, `archive/` holds terminal (`done`, `killed`). The happy-terminal state is **`done`** (a deliberate "definition of done"); the `archive/` directory keeps its name (it is the *location* for finished changes, both `done` and `killed`). The terminal move happens once per change, and is **idempotent and guarded** (re-pull, re-read `status` on `metadata_branch`, no-op if already `done`; the `YYYY-MM-DD` prefix is the **merge commit's** date — formatted in **UTC** so concurrent agents in any timezone derive the same calendar date — for `done`, and the **kill-commit's** date for `killed` — which has no merge but is written by a single agent under the claim lock — so concurrent paths agree on the filename **and on the change's `updated:`** (set to that same date, making concurrent archive commits — which move the change file only, board regenerated separately — byte-identical), never `now()`) — so the paths that can trigger it (`docket-finalize-change`, and the `docket-status` sweep — invoked directly or via `implement-next` step 0) never corrupt each other. Finer state (blocked/deferred) is a field value, not a directory.

6. **Five skills**, flat-prefixed with `docket-` (Claude Code invokes skills by flat name, so the prefix *is* the grouping — a `docket:`-style colon namespace would imply grouping that doesn't exist): `docket-new-change` (producer), `docket-implement-next` (implementer), `docket-finalize-change` (close a change to `done`), `docket-status` (board + janitor), `docket-adr` (decision ledger).

7. **Two agents coordinate purely through committed manifests in git** — no locks, no database.

8. **Semi-autonomous, human gate at the PR.** The implementer runs the whole spine solo per change and stops at an open PR for human review/merge.

9. **Skills are self-contained — convention duplicated, not shared.** Each `SKILL.md` embeds the convention (directory layout + manifest schema + lifecycle) inline as a marker-delimited `## Convention` block, and `docket-new-change` carries its own change template *inside its skill folder*. Nothing a skill needs lives outside its own directory, because skills are distributed by copying/symlinking that directory — an external `references/CONVENTION.md` would not travel with them. The duplication across the five skills is accepted; an optional `sync-convention.sh` propagates edits from one canonical block to keep the copies in step.

10. **A `link-skills.sh` convenience script** symlinks the five skill directories into the **global** skill dir of each agent harness present — for the harnesses §11 targets that's `~/.claude/skills/`, `~/.codex/skills/`, `~/.cursor/skills/`, plus the generic `~/.agents/skills/` (the reference script also covers `~/.kiro/` / `~/.windsurf/`); verify each harness's exact skills dir at build time. Absolute symlinks point back to `~/dev/docket/skills/<name>`, so the source of truth stays in `~/dev/docket`, docket installs once, and the skills are available in every project without copying. Modeled on `~/dev/obsidian-wiki/link-skills.sh` (idempotent: only creates missing links into dirs that already exist).

11. **ADRs are first-class in docket — not delegated to the project.** Architecture decisions live in `docs/adrs/` (sibling to `docs/changes/`), follow an immutable-ledger convention (numbered `NNNN-kebab-title.md`; an `Accepted` ADR changes only its `status:` line; a decision is reversed/superseded by a *new* ADR, never a rewrite), and are managed by the dedicated `docket-adr` skill. Changes `cite` and `produce` ADRs; ADRs outlive changes and are **never archived**. Markhaus proved the convention's value — but it was model-invented per project, so docket codifies it to stop it being reinvented (or missed) each time.

12. **Metadata lives on a configurable branch, default `main`.** Change files, board, and ADRs are PM metadata committed to `main` by default — decoupled from the code PR. A `metadata_branch` setting (default name `docket` when enabled) redirects that metadata to a dedicated branch for hard-protected-`main` repos, at a documented visibility cost (§8). **For v1, default `main` mode is the supported path; `docket` mode is a documented rough edge** (§8, §13) — its cross-branch write/read mechanics are not fully specified yet.

13. **Spec and plan split across the timeline.** The **spec** (design) is a *propose-time* artifact: `docket-new-change` produces it via `superpowers:brainstorming` and it lives **with the change on `metadata_branch`** (its `spec:` link always resolves; in default mode it's already on `main` for the build to read). The **plan** (task breakdown) is a *build-time* artifact: `docket-implement-next` produces it via `superpowers:writing-plans` on the **feature branch**, reaching `main` with the code at merge (Markhaus practice). Spec = the human's durable design; plan = the ephemeral mechanical breakdown (§8).

14. **The human is in the loop only at change creation; implementation is autonomous.** `docket-new-change` is **interactive** — it runs `superpowers:brainstorming` with the human and stops at the spec (it does *not* continue to `writing-plans`). `docket-implement-next` runs with **no human**: reconcile (non-interactive) → plan → build → review → PR. A change is **build-ready** — eligible for `docket-implement-next` — only when it has a `spec:` or is marked `trivial: true`. If reconcile finds the design *fundamentally* invalidated, the implementer **stops and escalates** rather than guessing.

---

## 4. Architecture — the skill set

```
docket/
  skills/
    docket-new-change/
      SKILL.md             # producer: idea → proposed change (+ opt-in scan mode)
      change-template.md   # the change-file stub (travels with the skill)
    docket-implement-next/
      SKILL.md             # implementer: pick → reconcile → build → PR → stop
    docket-finalize-change/
      SKILL.md             # close out a merged/approved change to done (human)
    docket-status/
      SKILL.md             # board render + merge-sweep janitor + health checks
    docket-adr/
      SKILL.md             # ADR ledger: create / supersede / maintain the index
      adr-template.md      # the ADR stub (travels with the skill)
  link-skills.sh           # symlink the skill dirs into agent-harness skill dirs
  sync-convention.sh       # (optional) propagate the shared Convention block across skills
  README.md                # what docket is, install, prerequisite (superpowers); must showcase the reconcile feature (§7.2) + the main-as-pseudo-database transparency note (§8)
  docs/                    # design spec etc. — repo-only, never copied into a harness
```

Each skill is a **self-contained** directory: it embeds the convention (directory layout + manifest schema + 7-state lifecycle) inline as a marker-delimited `## Convention` section, and the skills that need stubs carry their own (`docket-new-change` → `change-template.md`, `docket-adr` → `adr-template.md`). Nothing a skill needs lives outside its own folder — because skills are distributed by copying or symlinking their directory, and a shared external file (e.g. a top-level `references/CONVENTION.md`) would not travel with them. The convention is therefore duplicated across the five `SKILL.md` files; that duplication is accepted, and an optional `sync-convention.sh` propagates edits from a single canonical block to keep the copies in step.

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
    2026-05-30-0004-quicklook-extension.md   # YYYY-MM-DD-<id>-<slug>.md (date = merge-commit date for `done`, kill-commit date for `killed`)
  BOARD.md                               # generated status board (rich Markdown, see §7.4); spans active + archive
  README.md                              # small static blurb: explains the dir + links to BOARD.md (not generated)

docs/adrs/
  0024-quicklook-interaction-limits.md   # NNNN-kebab-title.md (immutable once Accepted)
  README.md                              # the ADR index (generated by docket-adr)
```

Base paths are configurable knobs; defaults `docs/changes/` and `docs/adrs/`. `active/`, `archive/`, `BOARD.md`, and the static `README.md` are always *relative to* `changes_dir`; `adrs_dir` is flat (ADRs are never archived — no `archive/` under it). A third knob, `metadata_branch` (default `main`), controls where lifecycle/board/ADR commits land — see §8. All three live in an optional committed `.docket.yml` (below); omit it and you get the defaults.

### Configuration — `.docket.yml` (optional, committed)

docket's knobs live in a single **committed** `.docket.yml` at the repo root — *not* a gitignored `.env`. The config (especially `metadata_branch`) governs **cross-agent coordination**, so it must be identical for every agent, clone, and device; a local/secret env file would let two agents disagree about where metadata lives and break the lock and board. The file is **optional** — absent means all defaults — and you create it only to override:

```yaml
# .docket.yml — committed; read by every docket skill at startup
metadata_branch: main        # default main; set to `docket` for a hard-protected main (§8)
changes_dir: docs/changes    # default
adrs_dir: docs/adrs          # default
```

It lives on **`main`** (like `.gitignore` / `.editorconfig`), *not* routed by `metadata_branch`, so every checkout and feature worktree can read it **before** it knows `metadata_branch` — resolving the bootstrap (you can't look on the `docket` branch for the setting that tells you to use the `docket` branch). It is set-once project config, separate from the per-change metadata churn. The skills are installed globally but read this **per-project** file at runtime: `docket`-the-tool is installed once; `docket`-the-config lives with each repo. (Future knobs — stale-claim threshold, `scan` candidate sources — would land here too.)

### Manifest (frontmatter at the top of each change file)

```yaml
---
id: 7                     # integer; zero-padded to 4 digits in the filename
slug: quicklook-interactions
title: Quick Look interactions — external links + local images
status: proposed          # proposed | in-progress | blocked | deferred | implemented | done | killed
priority: medium          # low | medium | high | critical  (default: medium)
created: 2026-05-30
updated: 2026-05-30
depends_on: [4]           # change ids that must reach `done` (PR merged) first; a dep still `implemented`/unmerged blocks as "needs your merge"
related: [4, 6]           # cross-links the reconcile pass reads
adrs: [24]                # ADRs this change cites or produces
spec:                     # superpowers design doc; set at brainstorm (propose) time, on metadata_branch
plan:                     # superpowers plan; set at build time, on the feature branch
trivial: false            # true = no spec needed (small mechanical change); still build-ready
branch:                   # planned feat/<slug> name, set on claim; the branch itself is created at build (step 4)
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

### ADR file (`docs/adrs/<NNNN>-<slug>.md`)

```yaml
---
id: 24
slug: quicklook-interaction-limits
title: Quick Look interaction limits under sandbox
status: Accepted          # Accepted | Superseded by ADR-NN | Reversed by ADR-NN | Deprecated
date: 2026-05-20
supersedes: []            # ADR ids this replaces (sets the old one's status)
reverses: []              # ADR ids this undoes
relates_to: [22]          # cross-links
change: 4                 # back-link: the change that produced this decision, if any
---

## Context   — the forces/problem that prompted the decision
## Decision  — what was chosen, and the rule a reader needs to know
## Consequences — what it enables, what it costs, what is given up
```

ADR frontmatter is machine-readable so `docket-adr` can generate the index and validate links (ADR `id`s follow the same rule as change ids — integer in frontmatter, zero-padded to 4 digits in the filename, e.g. `0024-…`) — a deliberate divergence from the classic prose-header ADR (e.g. Markhaus's `**Date:**` / `**Status:**` lines), chosen so the index and link-validation are robust rather than regex-parsed. The body follows the classic Context/Decision/Consequences shape. An `Accepted` ADR is immutable except its `status:` line; a non-reversing context change is appended as a dated `## Update` note, never an edit to the decision.

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
     └──── killed (obsolete — from proposed, or from in-progress via reconcile; → archive/) ────▶
```

| status | meaning | directory |
|---|---|---|
| `proposed` | drafted, awaiting work | `active/` |
| `in-progress` | claimed, being built | `active/` |
| `blocked` | external blocker (`blocked_by:`) | `active/` |
| `deferred` | consciously shelved, may revive | `active/` |
| `implemented` | built, PR open — **human merge gate** | `active/` |
| `done` | PR merged, filed away (happy terminal) | `archive/` |
| `killed` | abandoned — obsolete or never shipped (sad terminal) | `archive/` |

**Rules.** `active/` holds every non-terminal status; `archive/` holds the two terminal outcomes. The single physical move (`active/ → archive/`, prefixed with the **merge commit's** date for `done` / the **kill-commit's** date for `killed`) happens once on the terminal transition and is idempotent (decision #5). `deferred` may be entered from `proposed` or `in-progress` (with a `## Why deferred` note) and revived to `proposed`; clearing a blocker or reviving is a one-line frontmatter edit, no move. A change whose `depends_on` is unsatisfied is *implicitly* blocked — the selector skips it (no status change) and the board shows it as **waiting on #N**, never as plainly build-ready. A dependency is **satisfied when it reaches `done`**. Because `docket-implement-next` step 0 first sweeps any *merged* dependency to `done`, a dep that was merged-but-not-finalized becomes satisfied within the same run (for the clone that runs the sweep; other clones pick it up on their next pull). But if `#N` is still `implemented` (its PR open and **unmerged**), the dependent is gated behind a human action — the board flags this as **waiting on #N — needs your merge**, distinct from **waiting on #N — not yet built**, so the blocker is visible instead of a silent stall. Reserve explicit `blocked` for external blockers the system can't infer.

---

## 7. The five skills

`docket-new-change`, `docket-implement-next`, `docket-finalize-change`, and `docket-status` own the change lifecycle (create → build → close out → board); `docket-adr` owns the decision ledger. All dispatch superpowers for the heavy work.

### 7.1 `docket-new-change` — the producer (interactive)

**This is where the human is in the loop.** It turns an idea into a *build-ready* change by brainstorming the design up front, so `docket-implement-next` can later run with no interaction. **Only ever creates new `proposed` ids**, so it structurally cannot collide with the implementer. Writes markdown only — **no branch, no worktree, no code**.

**Brainstorm mode (default):**
1. **Allocate** — `git pull --rebase` on `metadata_branch`; scan the **`id:` frontmatter** of every change file in `active/` + `archive/` (the archive's date-prefixed filenames make frontmatter the reliable source), take max + 1; derive `slug` from the title. The id is finalized at the step-5 push (compare-and-swap): if that push is rejected because another `docket-new-change` minted the same id first, `re-pull → re-read max id → re-allocate`, **rename `active/<id>-<slug>.md` and fix any id-bearing links**, then re-push.
2. **Brainstorm** — run `superpowers:brainstorming` *with the human* to work through the design. This is the decision point. It **stops at the spec** — it does *not* continue to `writing-plans` (that's build-time). The spec is written natively to `docs/superpowers/specs/…` and committed to `metadata_branch`; record its path in `spec:`.
3. **Recon** — scan neighbouring changes (active + recent archive) and the ADR index to pre-fill `related`, `depends_on`, `adrs`.
4. **Draft the change** — write the thin `active/<id>-<slug>.md` from the bundled `change-template.md`: frontmatter (`status: proposed`, `spec:`, dates, `priority` — default `medium`) + a PM-altitude body (why/what/scope) distilled from the brainstorm. The design detail lives in the linked spec, not here.
5. **Board, commit & push** — refresh `BOARD.md`, commit the change + spec, and **push to the remote `metadata_branch`**. The pushed markdown is immediately reviewable on GitHub (e.g. when the change was authored from a phone) and visible to the autonomous implementer. **Stops. Never implements.**

**Trivial path:** for a small mechanical change with no real design questions, skip the brainstorm, set `trivial: true`, and write the change body directly — no spec, still build-ready (decision #14).

**Scan mode (opt-in, explicitly triggered):** survey TODOs, deferred changes, known gaps, and the ADR backlog, and emit several lightweight `proposed` **stubs** in one pass — *without* specs. Scan-stubs are **not build-ready** — the board calls this state **needs-brainstorm** (no spec and not `trivial`); they form an "ideas to brainstorm" backlog that a later `docket-new-change` brainstorm pass turns into build-ready changes. Kept opt-in so routine runs don't generate speculative noise.

### 7.2 `docket-implement-next` — the implementer (autonomous)

**Runs with no human interaction.** Picks the next build-ready change and drives it to a PR, then stops at the human merge gate.

0. **Sync & sweep** — `git pull --rebase`; invoke the `docket-status` merge-sweep so any `implemented` change whose PR has merged is swept to `archive/` (status → `done`) first (self-cleaning safety net, for changes not already closed via `docket-finalize-change`).
1. **Select** — among `active/` changes that are `proposed`, **build-ready** (have a `spec:` or `trivial: true`), and have all `depends_on` satisfied (a dependency is satisfied when it is `done`), rank by `priority` (`critical` > `high` > `medium` > `low`) → age (`created` date) → **lowest `id`** as the final deterministic tie-break (this is the ordering §8 relies on for no-collision); pick the top (or accept an explicit id). Skip `in-progress`/`blocked`/`deferred` and not-build-ready stubs.
2. **Claim** — re-read the manifest after the pull (avoid double-claim), set `status: in-progress` + `branch`, `updated`; commit and push on `metadata_branch`. *The lock is a **compare-and-swap**: re-read `status` under a freshly-pulled tree, claim, push; a non-fast-forward rejection means someone else moved — `pull --rebase`, **re-read, and retry the push until it lands** (under load it can be rejected repeatedly — keep rebasing + re-reading). The guard is that you never set `in-progress` on a change you re-read as non-`proposed` — not that any single push succeeds.* (No worktree yet — step 4.)
3. **Reconcile** ⭐ — re-read the change + its spec against `related` + recently-archived changes, cited + recent ADRs, and current code; refresh the change body and spec to what is true *now* (drop work done elsewhere, adjust scope, fold in new constraints), **non-interactively** (a `trivial` change has no spec — reconcile refreshes the body only); append a dated `## Reconcile log` entry; set `reconciled: true`; commit and push on `metadata_branch` (so the next step's worktree carries the refreshed spec in default mode). Two escape hatches: if the change is now **obsolete** → `killed` (`## Why killed`) and loop back to Select; if the design is **fundamentally invalidated** (not just scope-adjustable) → **stop and escalate to the human** — it can't re-brainstorm alone.
4. **Worktree + plan** — `git fetch origin`, confirm the step-3 reconcile push has landed on `origin/main` (default mode; if it hasn't — e.g. the push was rejected — `pull --rebase`, re-push, and re-fetch, looping until the reconcile commit is on `origin/main`, before continuing; in `docket` mode the spec lands on `origin/docket` and must be read cross-tree — a v1 rough edge, §8), then `git worktree add .worktrees/<slug> -b feat/<slug> origin/main` — the **freshly-fetched** `origin/main` carries the reconciled spec in default mode (never base on a *separate* metadata branch like `docket`; in default mode `metadata_branch` **is** `main`, the correct base — see §8). Run `superpowers:writing-plans` (writes `docs/superpowers/plans/` **on the feature branch**); record the path in `plan:`.
5. **Build** — `superpowers:subagent-driven-development` executes the plan task-by-task with TDD + per-task review.
6. **Review + ADRs** — `superpowers:requesting-code-review` (whole-branch); for any non-obvious decision, invoke `docket-adr` to record it (it assigns the number + updates the index) and append the returned number to the change's `adrs:`.
7. **PR + stop** — invoke `superpowers:finishing-a-development-branch`, **directed** to *push the feature branch and open a PR — do not merge, then stop*. Pre-specifying the outcome keeps it non-interactive (what the autonomous flow needs) while **reusing its push/PR mechanics** rather than reimplementing them — docket's "let superpowers carry the heavy work" principle. Then, **back in the main working tree**, set `status: implemented` + `pr:` and **commit + push on `metadata_branch`** — never in the feature worktree (metadata always lands on `metadata_branch`, §8; this is also what lets the sweep read `pr:`). **Stops.** The change stays in `active/` as `implemented` until a human **merges it, or approves `docket-finalize-change` to merge it for them** (§7.3).

#### The reconcile pass and the `reconciled` flag — docket's quiet superpower

> **The README must surface this prominently** — it is docket's most valuable, least obvious feature, not mere bookkeeping.

A change is drafted against a *snapshot* of the world — the codebase, the ADRs, and the other changes as they stood the day you brainstormed it. In an async backlog the implementer may not pick it up for weeks, by which point that snapshot is stale: other changes have shipped, new ADRs have landed, the code has moved. **Most backlog-driven systems build the ticket exactly as written, stale assumptions and all** — the classic failure mode where you implement something already half-done elsewhere, or that a later decision quietly invalidated.

docket's **reconcile** step (step 3) is the antidote. Just before building, it re-reads the change and its spec against the *current* world and rewrites them to what is true *now*: drops work already done elsewhere, narrows or widens scope, folds in constraints from ADRs written since — and if the change is now pointless it **kills** it, or if the design is fundamentally invalidated it **stops and asks you**. It runs at the **last responsible moment**, because reconciling any earlier would just go stale again.

The **`reconciled` flag** is the visible record that this refresh ran: `false` at birth (the change reflects its drafting-day snapshot), `true` after the pass (it reflects current reality). It is an **audit signal** — paired with the dated `## Reconcile log` entry, anyone can see *when* a change was last brought up to date and *what changed* — and a **resume-safety guard**: it is **not** a selection criterion (every change is born `false`, and selection build-readiness is `spec:`-or-`trivial`, §7.2 step 1); rather, on resume a claimed (`in-progress`) change still showing `reconciled: false` tells the implementer reconcile didn't finish, so it re-runs. The flag marks *that* reconcile ran, not that it is still *fresh* — so on **any** resume the implementer also re-runs reconcile if `origin/main` has advanced since (it is idempotent and non-interactive, and reconcile must reflect the *last responsible moment*). A one-line boolean that encodes docket's core stance: **plans rot; refresh them just-in-time, never trust a stale backlog.**

### 7.3 `docket-finalize-change` — close out a change (human)

The human's **closing bookend** (mirrors `docket-new-change`, the opening one). Run it when a change's PR is approved or merged, to complete the change to `done` *promptly* rather than waiting for the safety-net sweep. Given either an explicit change id, or **auto-detect**: auto-detect **finalizes every `implemented` change whose `pr:` is already merged** (safe, idempotent), and for any that are only *approved-and-mergeable* it **prompts before merging** (merging is a deliberate act). The steps below run per selected change:

1. **Check the PR** (`gh`). Already merged → straight to archive. Approved + mergeable but not merged → **merge it** (you invoking finalize *is* the merge decision — the gate is respected), then continue.
2. **Verify** the merge landed on `main` (optionally, tests green on the merged result).
3. **Archive (idempotent)** — `git pull --rebase`, re-read status; if already `done` (or already under `archive/`), no-op. Otherwise move `active/<id>-<slug>.md` → `archive/<merge-date>-<id>-<slug>.md` (the `YYYY-MM-DD` prefix **and** the change's `updated:` are both the **merge commit's** date in **UTC** — e.g. `gh`'s `mergedAt`, or `TZ=UTC git show -s --date=format-local:%Y-%m-%d` — not `now()`, so concurrent paths in any timezone produce a byte-identical result), set `status: done`; commit on `metadata_branch` and push (this archive commit moves the **change file only** — the `BOARD.md` regen is the separate step 5 commit, so concurrent archivers stay byte-identical; retry on a non-fast-forward race).
4. **Clean up** — remove the merged feature branch + worktree (provenance-guarded, like `finishing-a-development-branch`).
5. **Board** — regenerate `BOARD.md`.

It never touches the PR diff — the archive is a clean metadata commit on `metadata_branch`. **In `docket` mode finalize spans two branches** (a v1 rough edge — §8/§13): it merges the code into `main` (step 1) but commits the archive to `docket` (step 3), so the `done` state won't appear on `main` until the periodic `docket → main` sync (§8). `docket-status`'s bulk merge-sweep (and step 0 of `docket-implement-next`) remain a self-healing safety net for any change merged via the GitHub button without running this skill.

**Where `superpowers:finishing-a-development-branch` fits.** docket **delegates the git integration mechanics** to it rather than reimplementing them (its own "let superpowers carry the heavy work" principle). In the autonomous step 7 docket **directs** its choice — *push the branch, open a PR, do not merge, stop* — which keeps it non-interactive. When a human is present it can also be used interactively for a **non-standard closeout** (keep the branch as-is, discard it, or merge locally without a PR), where its merge/keep/discard chooser is exactly right. docket additionally borrows its **worktree provenance-guard** for cleanup.

### 7.4 `docket-status` — the board & janitor

The queryable state plus housekeeping; run it to see "what's done, what's next, what's stuck."

- **Board** — scan `active/` + `archive/` and **regenerate `BOARD.md` wholesale** (rich GitHub-Flavored Markdown): a one-line count summary; emoji-grouped sections per status with live counts (`## 🟢 In progress (2)`); per-group tables with clickable PR/spec links, a priority chip (`critical`/`high`/`medium`/`low`), and the **build-ready vs needs-brainstorm** split made visible (a change with an unsatisfied `depends_on` is shown as **⏳ waiting on #N — not yet built** — or, if `#N` is `implemented` with an unmerged PR, **⏳ waiting on #N — needs your merge** — not as build-ready); a **Mermaid dependency graph** built from `depends_on` edges (done nodes tinted); and a collapsible `<details>` Done section. It renders richly on **GitHub and in Markhaus** (Mermaid bundled, ADR-0011) and degrades gracefully in plain CommonMark viewers. Disciplines: **never hand-edited** (source of truth is the change files) and **no churny timestamp** (counts convey freshness). A small static `README.md` beside it explains the directory and links to `BOARD.md`.
- **Merge sweep** (the *bulk* safety net; `docket-finalize-change` closes a single change on demand — §7.3) — for each `implemented` change, check via `gh` whether its `pr` merged (falling back to `gh pr list --head feat/<slug>` if `pr:` is empty) → archive idempotently (re-pull, re-read `status` on `metadata_branch`; no-op if already `done`; move to `archive/<merge-date>-<id>-<slug>.md` using the **merge commit's** date in **UTC**), set `status: done`, commit (change-file-only; `BOARD.md` is regenerated by the Board pass, not bundled here), and remove the merged feature branch + worktree (provenance-guarded). Closes the loop after a human merge.
- **Health checks** — flag stale `in-progress` claims **past the build step** (branch gone / no commits in **N days, default 3** — a future `.docket.yml` knob; a just-claimed change has a planned `branch:` but no branch yet, which is *not* stale), a `spec:` that is **set but doesn't resolve against `metadata_branch`** (skipped for `trivial: true` changes, which have no spec), a `plan:` that doesn't resolve **on a `done` change** (link rot — ignored for `implemented` changes, whose plan legitimately still lives on the unmerged feature branch), a build-ready change blocked only by a dependency stuck at `implemented` (its PR **needs a human merge**), `blocked` changes whose `blocked_by` may have cleared, and `depends_on` cycles.

The board and these health checks share **one dependency-resolution pass** — for each change, resolve whether its `depends_on` targets are `done` vs `implemented`-with-unmerged-PR — so "waiting on #N" / "needs your merge" is computed once, not duplicated.

### 7.5 `docket-adr` — the decision ledger

Owns `docs/adrs/`. Invoked by `docket-implement-next` (step 6) when a decision is made, or directly by a human/agent any time a decision must be recorded or changed. Actions:

- **Create** — allocate the next ADR number (max+1 by scanning `docs/adrs/`), write `<NNNN>-<slug>.md` from the bundled `adr-template.md` (`status: Accepted`, optional `change:` back-link), add an index entry, commit, and **return the number** so the caller can cite it.
- **Supersede / reverse** — never edits an Accepted ADR's body. Writes a *new* ADR (`supersedes:`/`reverses:` the old), flips only the old ADR's `status:` to `Superseded by ADR-NN` / `Reversed by ADR-NN`, and annotates **both** entries in the index.
- **Update note** — for a non-reversing material change, append a dated `## Update` to the ADR (allowed) rather than touching the decision.
- **Index / validate** — (re)render `docs/adrs/README.md` grouped Active / Superseded-Reversed / Deprecated (each row e.g. `- [ADR-0024](0024-quicklook-interaction-limits.md) — Quick Look interaction limits (Accepted) ← change #4`); flag numbering gaps, dangling `supersedes`/`relates_to` links, and status inconsistencies.

---

## 8. Two-agent coordination

The whole point: run the **human-driven producer** (`docket-new-change`, brainstorming new build-ready changes) and the **autonomous implementer** (`docket-implement-next` in a loop, draining them to PRs) in parallel — the human's interactive time is all at change creation; draining is hands-off **for independent changes**. (Dependency *chains* still serialize on the human merge gate: a dependent can't start until the change it `depends_on` is merged — the board surfaces this as **waiting on #N — needs your merge**. So the drain is hands-off within independent work, but chained links each wait for a merge.)

- The producer only mints **new `proposed` ids** — it never touches in-flight changes, so it cannot collide with the implementer.
- The implementer **claims atomically** (compare-and-swap — see "The atomic claim" below): `git pull --rebase` → re-read → set `in-progress` → commit + push. Anything already non-`proposed` is skipped. The deterministic tie-break (`priority → age → lowest id`) keeps two implementers, if ever run together, off the same change.
- Shared state is **the committed change files** — git is the coordination medium. No lock files, no database. Worktrees isolate the file changes during build.
- **Every metadata commit is pushed to the remote `metadata_branch` immediately** (change creation, claim, reconcile, status transitions, board, ADRs). That is what makes the lock and board visible across sessions, lets a change authored on one device (e.g. a phone) be reviewed on GitHub, and lets the autonomous implementer pick up newly-proposed changes. The only exception is a single shared local clone **worked by one agent at a time** (sequential use needs no push; two *concurrent* agents always need separate clones — see the atomic claim below).

### Where metadata lives (the branch model)

docket separates two kinds of write. **Metadata** — the change file, the board (`BOARD.md`), and ADRs — is project-management state, not code. **Code** (plus the build-time plan) lives on a `feat/<slug>` branch and reaches `main` through a reviewed PR. The load-bearing invariant is that the feature branch **never *modifies* metadata**. In default mode the metadata is physically present in the feature worktree (it's cut from `origin/main`, which holds it — and that's *useful*: it's how the build reads the spec); the build only *adds* the plan + code and never edits the change file, board, or ADRs. The feature branch is cut from `origin/main` *after* claim + reconcile, so its change-file content **equals the merge base** and is never touched again. At PR merge the 3-way merge therefore takes `main`'s side for the change file **unconditionally** (feat == base on that path), whatever transitions `main` underwent meanwhile — **no conflict, no revert.** The change file's `branch:`/`pr:` fields link the metadata to the code. Operationally this falls out of worktrees — metadata commits happen in the main working tree, code/plan commits in the feature worktree. (This is how docket avoids superspec's "commit the change dir to the feature branch first" workaround: docket never *edits* the change file on the feature branch, so there is nothing to conflict at merge.)

For the two-agent lock to work, metadata commits must be visible on a branch both agents pull. docket exposes this as a configurable **`metadata_branch`** (set in the committed `.docket.yml`, §5):

- **Default — `main`.** Metadata commits land directly on `main` (in the main worktree), pushed immediately. Best visibility (board + ADRs always current on `main`; `See ADR-NN` code comments resolve), simplest (one branch). It reads "no direct commits to `main`" as its real intent — *no unreviewed **code** on `main`* — since a `status:` flip or a regenerated board is metadata with nothing to review.
- **Opt-in — a dedicated branch (default name `docket`).** Set `metadata_branch: docket` and all metadata churn commits there; agents pull that branch to coordinate; code still goes `feat → main` via PR. For repos with a hard-protected `main` that would *reject* a direct push.

  **Current drawbacks of the dedicated-branch mode** (documented so the trade-off is honest — these are *current* limitations, not fundamental):
  - **ADRs aren't on `main`** until the `docket` branch is merged, so `See ADR-NN` references in code dangle on `main` in the meantime — the sharpest cost, since the ADR ledger is meant to be permanent main-tree record (this also bites a reviewer following `See ADR-NN` from an open PR's diff against `main`).
  - **The board isn't on `main`** — you switch to the `docket` branch (or browse it on GitHub) to see the backlog.
  - **Sync overhead** — the `docket` branch must be merged into `main` periodically to surface ADRs/board there; nothing does that automatically yet.
  - **Under-specified mechanics (v1):** the cross-branch operations `docket` mode needs — the producer committing a spec to a `docket` branch it has no checkout for, the reconcile push landing on `docket`, and feeding `writing-plans` a spec absent from the `origin/main` worktree (`git show docket:…`) — are **not fully specified for v1**. Treat **default `main` mode as the supported v1 path**; `docket` mode is a documented rough edge to complete later (§13).

  A future docket version could mitigate these (auto-mirror metadata to `main`, or auto-merge the `docket` branch). Until then, the dedicated branch trades visibility for a clean `main`. (Under this mode the spec — being metadata — also lives on `docket`; see "Where the spec/plan live" below.)

> **The README must state this plainly (transparency).** In the default mode, docket uses `main` as a **pseudo-database**: the live project state — backlog, board, decisions — is kept as files committed *directly* to `main`. Be upfront that **this is not how most projects treat `main`** — committing non-code bookkeeping straight to the integration branch is unconventional, and some teams will find it noisy or at odds with branch protection. The cleaner, conventional answer for them is the separate `metadata_branch` (`docket`) mode — but that mode is **currently underdeveloped** (dangling ADR refs, manual sync, per the drawbacks above) and is expected to improve in future versions (auto-mirror / auto-merge to `main`). Until it matures, docket defaults to `main` for its visibility and simplicity, eyes open to the trade-off.

**The atomic claim** (either mode) is a **compare-and-swap**: `git pull --rebase` → **re-read `status`** → if still `proposed`, set `in-progress`, commit, and **push before building**; on a non-fast-forward rejection, **discard your pending local claim commit** (it edits the same `status:`/`branch:` lines and would conflict on replay), `pull --rebase`, **re-read** (mandatory); if still `proposed`, re-claim and push — looping until the push lands (it can be rejected repeatedly under load). The arbiter is the re-read (you abort if the change is no longer `proposed`), not that any single push succeeds. Because that arbiter is a re-read against the freshly-pulled remote, **don't run two agents against one shared local clone** (no remote to pull from between them); give each agent its own clone.

### Starting a change's feature branch

Whatever `metadata_branch` is, **a change's `feat/<slug>` branch is always cut from the tip of `origin/main`** — its code merges into `main` via PR, so basing it anywhere else pollutes the diff. The feature branch adds only the **plan + code** and **never *modifies* docket metadata** (in default mode the metadata is present in the worktree, inherited read-only from `origin/main`; it is simply never edited there — see "Where metadata lives" above). The skills **and the README must state this explicitly**, per mode:

- **`metadata_branch: main` (default).** `main` is both the metadata home and the feature-branch base. Flow: pull `main` → commit the claim on `main` → reconcile (refresh change + spec on `main`) → `git fetch` then `git worktree add .worktrees/<slug> -b feat/<slug> origin/main` → build → later status commits on `main` → PR → merge.
- **`metadata_branch: docket`.** Code still targets `main`, so the feature branch is **still cut from `origin/main`, never from `docket`.** This is the trap to call out loudly: `docket` holds metadata only and has diverged from `main`; branching a change off it bases your code on unrelated metadata commits and yields a junk PR. Here the implementer juggles three branches — `docket` (claim/status/board/ADR/spec commits, in a `docket` checkout), `main` (feature-branch base + merge target), and `feat/<slug>` (code + plan, in a worktree off `main`).

> **The one-line rule the skills/README encode:** *new change ⇒ `git worktree add .worktrees/<slug> -b feat/<slug> origin/main`* — in **both** modes. `metadata_branch` only redirects bookkeeping commits; it never changes where the code branch starts.

**Who creates the worktree, and when.** The **agent** creates it inside `docket-implement-next` (at the Worktree + plan step, *after* reconcile) — **never the human, and never `docket-new-change`**. The producer writes only a markdown change file (no branch, no worktree, no code), so `proposed` changes that may never be built don't litter the repo with worktrees. Worktrees are one-per-change-being-built, owned by the implementer.

Both skills are **invocation-branch-agnostic**: they `git fetch` and operate against `origin/main` (the feature base) and `metadata_branch` (bookkeeping) *explicitly*, so it does not matter which branch the human is on when invoking — basing the worktree on `origin/main` rather than the current local HEAD is exactly what makes this safe. (Convention, not requirement: invoke from the main checkout.)

The worktree **persists through the `implemented` / PR-open state** (so review feedback can add commits) and is removed by whichever archival path runs (`docket-finalize-change` or the merge-sweep) when the change is archived to `done` — same provenance-guard as `superpowers:finishing-a-development-branch` (only auto-remove worktrees under a known `.worktrees/`-style path).

### Where the spec/plan live

The **spec** and **plan** live on *different* branches, because they're created at different times by different actors:

- **The spec (design) is propose-time metadata.** `docket-new-change` produces it via brainstorming and commits it to `metadata_branch` alongside the change, so its `spec:` link **always resolves**. In default mode it's already on `main`, so the feature worktree (cut from `origin/main` *after* reconcile) carries the reconciled spec for the build to read.
- **The plan (task breakdown) is a build-time feature-branch artifact.** `docket-implement-next` writes it via `writing-plans` in the feature worktree; it merges to `main` with the code (Markhaus practice), and the build reads it from that same worktree — no cross-tree read. A change's `plan:` link therefore **resolves only after the PR merges**, so `docket-status`'s link-rot check ignores a missing `plan:` on an `implemented` change and flags it only once the change is `done`. (`spec:`, being metadata, is checked normally.)
- In `metadata_branch: docket` mode the spec sits on `docket`, **not** on `main`, so it is absent from the `origin/main`-based worktree. The build must read it explicitly from the metadata branch — `git show docket:docs/superpowers/specs/<file>` (or point `writing-plans` at a separate `docket` checkout). This cross-tree read is a known rough edge of the underdeveloped `docket` mode (§8 drawbacks); the plan + code still merge to `main`.

---

## 9. ADRs — the decision ledger

ADRs are a **first-class docket layer** (managed by `docket-adr`, §7.5; file schema in §5; convention in decision #11). They are the project-wide, immutable, numbered record of *why* — distinct from changes, which track *what* and are transient.

**How changes and ADRs relate:**
- a change **cites** ADRs (`adrs:`); each ADR optionally back-links the `change:` that produced it,
- the **reconcile** step **reads** relevant ADRs to refresh a change's scope against decisions made since it was drafted,
- implementing a change **produces** new ADRs (via `docket-adr`) for non-obvious decisions.

**Lifecycle contrast:** changes are *work* (`proposed → done`, then archived); ADRs are *decisions* (`Accepted`, then only ever `Superseded`/`Reversed`/`Deprecated` by a new ADR — **never archived, never rewritten, never moved**). When a change is archived, the ADRs it produced stay put in `docs/adrs/`.

Clean split: **ADRs = durable "why, forever"; changes = scoped "what, now → done."** This is the role superspec's `design.md` plays *within* a change, promoted in docket to a first-class, project-wide, append-only log — the convention Markhaus invented ad-hoc, now codified.

---

## 10. Error handling & edge cases

- **Claim race (two implementers):** mutual exclusion is a **compare-and-swap** — `git pull --rebase`, **re-read `status`** immediately before claiming, push; a non-fast-forward rejection forces `pull --rebase` + **re-read again** (mandatory) + retry, looping until the push lands (it can be rejected repeatedly under load); the arbiter is the re-read (abort if no longer `proposed`), not any single push; skip non-`proposed`; the deterministic priority→age→lowest-id ordering. Worst case, two agents claim different changes — never the same one. (Two agents must not share one local clone — §8.)
- **Concurrent archive (finalize vs sweep):** the `status` re-read on `metadata_branch` handles the *non-concurrent* case (no-op if already `done`). In a genuine race — both paths read `implemented` and both archive — safety rests on **determinism**: the archive commit is **change-file-only** (the `BOARD.md` regen is a *separate* commit), its filename uses the **merge-commit date in UTC** (kill-commit for `killed`), and it writes no nondeterministic field (`updated:` is that same date, not `now()`) — so both racers produce a **byte-identical add** that the losing push's `pull --rebase` resolves cleanly. (`BOARD.md`, regenerated separately, may differ between racers; that conflict is resolved by regeneration-from-source — see "Board merge conflicts" below — never hand-merge.) The push is retried on a non-fast-forward race.
- **Reconcile finds the change obsolete:** transition to `killed` with `## Why killed`, archive, and pick the next change — don't build dead work.
- **Link rot:** `spec:`/`plan:` paths are validated by `docket-status` health checks; a broken link is surfaced, not silently ignored.
- **Board merge conflicts:** `BOARD.md` is *generated*, so on conflict it is regenerated from the change files (source of truth), never hand-merged. Per-change files rarely conflict because each change is its own file.
- **Stale `in-progress`:** health checks flag a claim whose branch was deleted or has no recent commits, so it can be reset to `proposed`.
- **Archived files don't move their spec/plan:** the linked superpowers spec/plan stay in `docs/superpowers/` as frozen historical artifacts (like ADRs, they never move). Only the change file moves to `archive/`.
- **ADR immutability & numbering:** the next ADR number is max+1 by scanning `docs/adrs/`; concurrent writers use the same commit-as-claim discipline as change ids. `docket-adr` never rewrites an `Accepted` ADR — a change of mind is always a new, superseding ADR, so the ledger stays trustworthy.

---

## 11. Portability model

- **Skills are the universal substrate.** The `SKILL.md` format runs natively on Claude Code, Codex, and Cursor; no per-harness trigger shim is needed.
- **superpowers is a declared prerequisite** on every harness; docket calls `superpowers:*` directly and uniformly. Installing superpowers is the consuming user's responsibility, not docket's.
- **Lowest-common-denominator tools** in docket's own steps (file read/write, `git`, `gh`, skill invocation); everything heavier is delegated to superpowers, which handles its own cross-harness tool mapping.
- The **convention itself** (each skill's embedded `## Convention` block + the file layout) is pure data + prose and is therefore portable by definition, independent of any harness.

---

## 12. Testing & rollout

Skills are not unit-tested like code; verification is behavioural and dogfood-driven.

- **Smoke path:** on a throwaway change, run `docket-new-change` → `docket-implement-next` → `docket-status` and confirm: file created in `active/` as `proposed`; claim flips it to `in-progress` with a branch; reconcile appends a log entry; build produces a branch + PR; status flips to `implemented`; after a (simulated) merge, the sweep moves it to `archive/` as `done`.
- **Manifest schema** is documented in each skill's embedded `## Convention` block so a change file is checkable by eye and by the `docket-status` health pass.
- **First real dogfood: Markhaus.** Migrate Markhaus's existing `docs/plans/` + `*-results.md` into `docs/changes/` — completed plans become `done` changes (with their results folded into the body), and any open work (e.g. the current `feat/quicklook-interactions` branch → change `0007`) becomes a real `in-progress`/`implemented` change. Markhaus's existing `docs/decisions/` is also renamed to `docs/adrs/` and its ADRs carry over (prose headers converted to the ADR frontmatter), exercising `docket-adr`'s index render against a real multi-ADR ledger. This proves the lifecycle and the PR gate end-to-end before docket is carried to other repos.

---

## 13. Out of scope for v1 / open items

- **Out of scope:** a living-spec/behavior-contract layer (deliberately absent — code is current-state truth); an OpenSpec-style CLI or YAML schema; multi-repo coordination.
- **v1 rough edges (deferred, not blocking):** `docket` (separate-metadata-branch) mode — default `main` mode is the supported v1 path; `docket` mode's cross-branch write/read mechanics (§8) are documented but not fully specified for v1.
- **Open items to settle during implementation:** what the default `scan` candidate-source list contains (its *location* is settled — a `.docket.yml` knob, §5); and the **stale-`in-progress` threshold** (value settled at `3 days`; only its promotion to a `.docket.yml` knob is deferred — §7.4).
- **Naming:** `docket` chosen over `speclite` / `changeflow` / `slate` / `dossier` — it captures the *queue you drain*, which is the heart of the two-agent loop.
