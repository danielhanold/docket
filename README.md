# docket

A change is a self-contained, tracked unit of planned work (≈ one PR). docket records each change as a single markdown file with a status lifecycle, and provides six skills to create changes, work the next change to a PR, finalize a merged change, report the board, record architecture decisions (ADRs), and define the shared convention they all load — all coordinated through git, no CLI or database.

---

## What docket is

superpowers gives Claude excellent *execution*: brainstorm → spec → plan → TDD → code-review → merge, all in one invocation. What it does not give you is a tracked backlog or a "done" state that persists across invocations. Each session starts fresh.

OpenSpec / superspec solves that with a full lifecycle layer, but it requires a CLI dependency and a rigid markdown contract that not every project wants to adopt.

docket sits in between. It adds a thin lifecycle layer — plain markdown files in your repo, six skills, no CLI — and delegates every execution step to superpowers wholesale. The core unit is a **change** (one file, one PR's worth of work). Architecture decisions are captured separately as **ADRs** (an immutable ledger). The code is always the current-state truth; docket carries no living-spec layer and does not try to mirror the codebase in prose.

The six skills cover the full loop: create, implement, finalize, report, decide — plus the shared contract they all load as a pure-reference skill.

---

## The producer / implementer loop

Two agents, run in parallel:

| Role | Skill | Mode |
|---|---|---|
| **Producer** | `docket-new-change` | Interactive — you brainstorm changes into a build-ready backlog. |
| **Implementer** | `docket-implement-next` | Autonomous — drains the backlog to open PRs, unsupervised. |

They coordinate purely through committed change files in git — no locks, no database. The producer only mints new `proposed` change ids; the implementer claims the next build-ready change atomically (compare-and-swap on the status field), so concurrent runs cannot collide.

The payoff: your interactive time is concentrated at change creation. Draining is hands-off for independent changes. The human stays in the loop at two points only — writing the change and merging the PR.

One honest caveat: dependency chains serialize on the merge gate. A change that depends on another cannot start until that dependency is merged; the board surfaces this as "waiting on #N — needs your merge." Unrelated changes drain freely in parallel around it.

---

## Prerequisite: superpowers

docket is a lifecycle wrapper around superpowers, not a replacement for it. superpowers must be installed and available in your harness before any docket skill will function. Installing superpowers is the consuming user's responsibility; docket declares it as a prerequisite but does not bundle or install it.

`docket-new-change` (the interactive producer) calls:

- `superpowers:brainstorming` — for up-front design before the spec is written

`docket-implement-next` (the autonomous build) calls:

- `superpowers:writing-plans` — to build the task plan from the spec
- `superpowers:subagent-driven-development` — to execute the plan with TDD
- `superpowers:requesting-code-review` — for a whole-branch review before the PR
- `superpowers:finishing-a-development-branch` — to push the branch and open the PR

---

## Install

Place the docket repo at `~/dev/docket` (the source of truth the symlinks point back to), then run:

```bash
bash ~/dev/docket/link-skills.sh
```

`link-skills.sh` creates absolute symlinks from each present harness's global skill directory back to `~/dev/docket/skills/<name>`. It only writes into harness directories that already exist on your machine; it is idempotent and safe to re-run after adding a new harness. Skills are installed once and available in every project you open.

The change data — `docs/changes/`, `docs/adrs/`, `docs/results/` — lives per consuming project, not in the docket repo itself.

**Optional per-project configuration.** Add a `.docket.yml` to override defaults. It is committed on your repo's **default branch** (`origin/HEAD`) — every clone, agent, and device needs the same values, and the default branch is the one place a skill can find the file with zero prior config:

```yaml
# .docket.yml — committed on the repo's default branch; read by every docket skill at startup
metadata_branch: docket      # docket (default) | main  — where planning commits land
integration_branch: auto     # auto (default → origin/HEAD, fallback main) | main | develop — where code lands
changes_dir: docs/changes    # default
adrs_dir: docs/adrs          # default
results_dir: docs/results    # default
```

With no `.docket.yml` at all, docket runs in its default **docket-mode** (`metadata_branch: docket`, `integration_branch: auto`). See the **docket-mode** section below for what that means and how to opt out.

`.docket.yml` is committed (not gitignored) because it governs cross-agent coordination.

---

## The reconcile superpower

This is the most valuable and least obvious part of docket.

### The problem

A change is drafted against a *snapshot* of the world: the codebase, the ADR ledger, and the other in-flight changes as they stood on the brainstorm day. In an async backlog, the implementer may pick it up a week or a month later. By then:

- Another change shipped work that this one was planning to add.
- An ADR settled a question that the spec left open — in the opposite direction.
- A dependency changed the interface this spec assumed.

Most backlog-driven systems just build the ticket as written and let the implementer discover the mismatch mid-way. The classic result: implementing something already half-done elsewhere, or building something that a later architecture decision has invalidated.

### What docket does instead

`docket-implement-next` includes a **reconcile step** (Step 3) that runs at the *last responsible moment* — after the change is claimed (so it belongs to this invocation) but before the worktree and plan are created — so no plan or build work is wasted if the scope changes.

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

## docket-mode: where metadata lives

docket needs a durable, queryable source of truth for planning state — changes, statuses, ADRs, dependencies, the board — shared across agents, machines, and time, with **git as the only persistence mechanism** (no database, no service). docket-mode is how that state is stored, and it is the supported default.

### The two-branch model

Two branches divide the work:

- An orphan **`docket`** branch is the authoritative working surface for **all planning metadata**: change files (active + archive), `BOARD.md`, ADRs, and specs. It is created with `git checkout --orphan docket` (the same well-trodden pattern as `gh-pages`), shares no history with your code, carries no code, and is **always pushed** so the whole backlog, board, specs, and ADRs are browsable and reviewable on the remote (GitHub) at all times. All planning churn — `proposed → in-progress → implemented`, board refreshes, reconcile edits, ADR writes — lands here and never touches your code history.
- Your **integration branch** (`main`, or `develop` under GitFlow) stays code-only, except for **published terminal records** (see below). It holds code, the build artifacts that arrive with each PR (plan + results), and a copy of the archived change + spec + accepted ADRs once a change closes out.

A change's `feat/<slug>` branch is always cut from `origin/<integration_branch>`, carries only plan + results + code, and never modifies docket metadata.

### Where each artifact lives

| Artifact | Lives on | How it reaches the integration branch | On integration after a terminal change? |
|---|---|---|---|
| Change file (manifest + body) | `docket` | Terminal-publish copy (on `done` or `killed`) | Yes (archived) |
| Spec (`docs/superpowers/specs/…`) | `docket` | Terminal-publish copy | Yes |
| ADR (`docs/adrs/…`) | `docket` | Terminal-publish copy, gated on `Accepted` | Yes (the `Accepted` ADRs) |
| `BOARD.md` | `docket` | **Never** — it is the live planning view | No — view it on `docket` |
| Plan (`docs/superpowers/plans/…`) | feature branch | The PR merge | Yes (`done` only) |
| Results (`docs/results/…`, i.e. `results_dir`) | feature branch | The PR merge | Yes (`done` only) |
| Code | feature branch | The PR merge | Yes (`done` only) |
| `.docket.yml` | **default branch** (`origin/HEAD`) | n/a — lives on the default branch | Only if the default branch *is* the integration branch (trunk mode); under GitFlow it stays on `main`, not `develop` |

The integration branch ends up with all five artifacts plus code — they simply arrive by **two paths**. Plan and results are build artifacts that live on the feature branch and ride in through the **PR merge**. The change file, spec, and accepted ADRs live on `docket` and would otherwise be stranded there, so a terminal transition **copies** them across (it does not branch-merge `docket`, which would drag all the planning churn onto your code line). `BOARD.md` is the one artifact that never leaves `docket`.

### `integration_branch` and GitFlow

The `integration_branch` knob says where code lands and where feature branches are cut from:

- `auto` (the default, and what an absent key resolves to) follows the remote's default branch via `origin/HEAD`, falling back to `main` if it can't be detected.
- `main` or `develop` is used verbatim.

This makes docket work for trunk-based (`main`) and **GitFlow** (`develop`) projects alike. One caveat: `auto` follows the repo's *default* branch, so a GitFlow repo whose default branch is `main` but whose integration line is `develop` must set `integration_branch: develop` explicitly. Feature branches always cut from `origin/<integration_branch>`.

### The `.docket/` metadata worktree

Git checks out one branch per folder. To write a file that lives on `docket` while your main folder sits on `main` or a feature branch, a skill needs a second folder parked on `docket` — a worktree. Each skill, in docket-mode, ensures a persistent worktree at **`.docket/`**: a second checkout folder parked on the `docket` branch, synced to `origin/docket` before any read. **Your main working tree never switches branches.**

`.docket/` is **gitignored** (alongside `.worktrees/`). It deliberately lives at `.docket/`, not under `.worktrees/`, for three reasons:

- **No slug collision:** feature worktrees are `.worktrees/<slug>`, and a slug is arbitrary text from a change title — a change slugged `docket` would collide on `.worktrees/docket`. `.docket/` makes that impossible.
- **Lifecycle:** `.worktrees/` holds ephemeral per-change trees that get pruned; the metadata worktree is permanent, singular infrastructure.
- **Cleanup blast radius:** worktree-pruning logic (and humans) run over `.worktrees/`; keeping the metadata tree at `.docket/` puts it outside that blast radius.

### Finalize → selective publish

On a **terminal transition** — a change reaching `done` (PR merged) or `killed` (abandoned) — the driving skill copies that change's terminal records onto the integration branch in one dedicated commit: the archived change file, its spec (if any), and the **`Accepted`** ADRs from its manifest, sourced from `origin/docket`. This is a selective **file copy**, never a branch merge, so none of the planning churn comes with it. The **live board stays on `docket`** and is never published. The result: your code history reads as code plus a clean trail of closed-out changes, while the working backlog churns entirely on `docket`.

### Migrating an existing repo

A repo that has been running in single-branch mode (everything on `main`) moves to docket-mode with a one-shot, idempotent script: **`migrate-to-docket.sh`** (it ships in this docket repo, alongside `link-skills.sh`). The script operates on the git repo containing your **current directory** — so run it *from within the repo you want to migrate*, pointing at the script wherever docket is checked out:

```bash
cd <target-repo>
bash /path/to/docket/migrate-to-docket.sh
```

It prints the resolved target repo and prompts for confirmation before changing anything; pass `--yes` (or `-y`) to skip the prompt in automation. It then creates the orphan `docket` branch seeded from your current planning directories, prunes the live planning surface (`active/` changes, the changes `README.md`, `BOARD.md`) off the integration branch while keeping terminal records and build artifacts there, and adds `.docket/` + `.worktrees/` to `.gitignore`. Re-running it converges from any partial state.

The skills will **not** migrate a repo for you. On first run against an un-migrated repo (metadata still on the integration branch, no `docket` branch yet), a **bootstrap guard** STOPs and points you at `migrate-to-docket.sh` rather than silently moving your data. The same guard detects a half-finished migration and points back to the script to complete it.

### `main`-mode: the single-branch opt-out

If you want everything on one branch — for example, a small repo, or a team that prefers all state in one place — pin both knobs:

```yaml
metadata_branch: main
integration_branch: main
```

This reproduces the original single-branch behavior **exactly**: no `docket` branch, no `.docket/` worktree, no terminal-publish copy. Planning commits land on the integration branch alongside your code, and the archive move there *is* the terminal record. Because docket-mode is now the default, an existing single-branch repo must pin `metadata_branch: main` to keep running as-is until it deliberately migrates — otherwise the bootstrap guard will stop and ask it to migrate.

---

## The six skills

| Skill | Role |
|---|---|
| `docket-new-change` | Producer, interactive — turns an idea into a build-ready change via brainstorming; writes markdown only, never branches or code. |
| `docket-implement-next` | Autonomous implementer — picks the next build-ready change, reconciles it against current reality, builds to an open PR, and stops at the human merge gate. |
| `docket-finalize-change` | Human close-out — merges an approved PR or closes out an already-merged one: archives the change to `done`, cleans up the branch and worktree, refreshes the board. |
| `docket-status` | Board and janitor — regenerates `BOARD.md`, sweeps merged PRs to `done`, and runs health checks for stale claims, broken links, and dependency stalls. |
| `docket-adr` | Immutable decision ledger — records architecture decisions, handles supersessions and reversals, and maintains the ADR index. |
| `docket-convention` | Shared contract, pure reference — single source of the docket convention (configuration, layout, manifest, lifecycle, build-readiness, bootstrap guard, branch model); every operating skill loads it as its blocking Step 0. |

---

## Status

**docket-mode is the supported default.** Planning metadata lives on the orphan `docket` branch via the `.docket/` worktree; terminal records are selectively published onto the integration branch; trunk-based and GitFlow layouts are both supported. Existing single-branch repos move over with `migrate-to-docket.sh`, and the bootstrap guard refuses to run against an un-migrated repo rather than touching your data.

`main`-mode remains as a simple, fully-supported opt-out: pin `metadata_branch: main` (and `integration_branch: main`) to keep everything on one branch with exactly the original single-branch behavior.

Markhaus is the first planned dogfood project; a migration plan exists to move its existing changes and ADRs into the docket format.
