# docket

A change is a self-contained, tracked unit of planned work (≈ one PR). docket records each change as a single markdown file with a status lifecycle, and provides eight skills to create changes, groom stubs to build-ready (interactively or autonomously), work the next change to a PR, finalize a merged change, report the board, record architecture decisions (ADRs), and define the shared convention they all load — all coordinated through git, no CLI or database.

---

## What docket is

superpowers gives Claude excellent *execution*: brainstorm → spec → plan → TDD → code-review → merge, all in one invocation. What it does not give you is a tracked backlog or a "done" state that persists across invocations. Each session starts fresh.

OpenSpec / superspec solves that with a full lifecycle layer, but it requires a CLI dependency and a rigid markdown contract that not every project wants to adopt.

docket sits in between. It adds a thin lifecycle layer — plain markdown files in your repo, eight skills, no CLI — and delegates every execution step to superpowers by default (each invocation point is rebindable via `skills:` in `.docket.yml` — see below). The core unit is a **change** (one file, one PR's worth of work). Architecture decisions are captured separately as **ADRs** (an immutable ledger). The code is always the current-state truth; docket carries no living-spec layer and does not try to mirror the codebase in prose.

The eight skills cover the full loop: create, groom, implement, finalize, report, decide — plus the shared contract they all load as a pure-reference skill.

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

## Workflow engine: superpowers by default, pluggable per role

docket is a lifecycle wrapper around a workflow engine, and **superpowers is the default engine — recommended, but not a hard requirement.** Each of the five workflow invocation points is a pluggable **role**: an optional `skills:` map in `.docket.yml` rebinds any of them to a different skill (the name is passed to the Skill tool verbatim) or to the sentinel `auto` (no skill — the running agent performs the step inline at its own model).

| Role | Default skill | Invoked by |
|---|---|---|
| `brainstorm` | `superpowers:brainstorming` | `docket-new-change`, `docket-groom-next` — up-front design before the spec |
| `plan` | `superpowers:writing-plans` | `docket-implement-next` — the task plan from the spec |
| `build` | `superpowers:subagent-driven-development` | `docket-implement-next` — execute the plan with TDD |
| `review` | `superpowers:requesting-code-review` | `docket-implement-next` — whole-branch review before the PR |
| `finish` | `superpowers:finishing-a-development-branch` | `docket-implement-next`, `docket-finalize-change` — push the branch, open the PR |

Unset keys default to the superpowers skills above — an absent `skills:` map is byte-identical to superpowers-everywhere. And if a resolved skill cannot be invoked at runtime (superpowers not installed, a typo'd custom name), docket **degrades to that role's `auto` fallback with a prominent warning**, so a repo without superpowers works out of the box with zero config. The config shape — the `skills:` keys, the `auto` sentinel, and each role's fallback artifact — is documented once in docket-convention's **"Skill layer"**; consult it there rather than copying examples here.

When you want the default engine, installing superpowers is the consuming user's responsibility; docket does not bundle or install it.

---

## Install

Place the docket repo at `~/dev/docket` (the source of truth the symlinks point back to), then run:

```bash
bash ~/dev/docket/install.sh
```

That's the whole install. `install.sh` runs the three primitives in order — and is idempotent, so re-run it any time (after adding a harness, or after editing `~/.config/docket/config.yml`):

- **`link-skills.sh`** creates absolute symlinks from each present harness's global skill directory back to `~/dev/docket/skills/<name>`. It only writes into harness directories that already exist on your machine. Skills are symlinks, so editing one in the repo is picked up everywhere immediately.
- **`sync-agents.sh`** generates docket's model/effort-pinned subagent wrappers from layered config (built-in defaults ⊕ global `config.yml` ⊕ a repo's committed `.docket.yml` ⊕ that repo's `.docket.local.yml`) into each present harness's `agents/` directory, and for any repo that opts in (via an `agents:` block or an `agent_harnesses:` key, in either file) writes the full per-repo agent set as **machine-local, gitignored** files — never committed. Unlike the skill symlinks, these are generated **copies** (they bake in resolved model/effort), so re-run after editing any config layer — `install.sh` does this for you, or call `sync-agents.sh` directly. Run `sync-agents.sh --check` in CI to catch a missing/stale `.gitignore` block or an accidentally-tracked generated file.
- **`ensure-docket-env.sh`** exports `DOCKET_SCRIPTS_DIR` — the absolute path to docket's `scripts/` directory — into your shell profile (and Claude Code's user-level `settings.json` `env`), so every docket skill can reach its deterministic helper scripts from *any* repo, not just this clone. Re-running `install.sh` back-fills already-migrated repos. Without it the skills fail loud with the `run docket/install.sh` remedy rather than silently hand-working each operation.

(You can still run any primitive on its own — `install.sh` just saves you from remembering all three.) Migrating an *existing* repo to docket-mode is a separate step — `migrate-to-docket.sh`, run from inside that repo — not part of this machine install.

The change data — `docs/changes/`, `docs/adrs/`, `docs/results/` — lives per consuming project, not in the docket repo itself.

**Optional per-project configuration.** Add a `.docket.yml` to override defaults. It is committed on your repo's **default branch** (`origin/HEAD`) — every clone, agent, and device needs the same values, and the default branch is the one place a skill can find the file with zero prior config:

```yaml
# .docket.yml — committed on the repo's default branch; read by every docket skill at startup.
# Every key is optional; unset = the default shown. Commented keys are opt-in.
metadata_branch: docket      # docket (default) | main  — where planning commits land
integration_branch: auto     # auto (default → origin/HEAD, fallback main) | main | develop — where code lands
changes_dir: docs/changes    # default
adrs_dir: docs/adrs          # default
results_dir: docs/results    # default
auto_groom: false            # repo default for autonomous grooming; per-change auto_groomable overrides
board_surfaces: [inline]     # derived board views: inline (BOARD.md) and/or github; [] disables the board
# github_project: {owner: <o>, number: <n>}  # Projects v2 board; minted + written back on first github sync
finalize:                    # merge gate: rebase onto base + re-test before docket merges
  gate: local                # local (default) | ci | both | off
  # test_command:            # unset => finalize auto-detects the suite
  # require_pr_approval: false  # true => the no-arg finalize refuses to merge an unapproved PR
# agent_harnesses: [claude]  # harnesses the per-repo agent pass generates committed wrappers for
# agents:                    # per-skill subagent model/effort (see "Agent layer" in docket-convention)
# skills:                    # rebind the five workflow roles — brainstorm/plan/build/review/finish — to
#                            # any skill name or `auto` (see "Skill layer" in docket-convention)
```

With no `.docket.yml` at all, docket runs in its default **docket-mode** (`metadata_branch: docket`, `integration_branch: auto`). See the **docket-mode** section below for what that means and how to opt out.

`.docket.yml` is committed (not gitignored) because it governs cross-agent coordination.

## Global config (`~/.config/docket/config.yml`) and the machine-local layer

Every key resolves **per key**, across up to four layers, with precedence **repo-local > repo-committed > global > built-in**: a repo's own `.docket.local.yml` (this machine only) wins first, then that repo's committed `.docket.yml` (every clone), then the cross-repo global `config.yml` (this machine, every repo), then the built-in default. Map-valued keys (`skills:`, `agents:`) merge field-by-field with the same precedence.

Cross-repo defaults live in one optional user-level file: `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml`. It accepts the **same schema as `.docket.yml`**.

```yaml
# ~/.config/docket/config.yml — optional; applies to every repo on this machine.
# Same schema as .docket.yml; a repo's committed .docket.yml wins per key.
skills:                      # rebind workflow roles for all your repos
  build: auto
agents:                      # agent model/effort defaults (same agents: shape as .docket.yml)
  default:
    implement-next: { model: claude-opus-4-8, effort: xhigh }
auto_groom: false
finalize:
  gate: local
board_surfaces: [inline]     # the github token is per-repo-only and ignored here (see below)
agent_harnesses: [claude]    # scopes sync-agents.sh's user-level pass ONLY (overrides
                             # presence-on-disk detection); never the per-repo generation pass
```

**Coordination keys are per-repo-only.** Keys whose effect writes shared state — `metadata_branch`, `integration_branch`, `changes_dir`, `adrs_dir`, `results_dir`, `github_project`, and the `github` token of `board_surfaces` — are ignored with a loud warning when set globally **or** in a repo's `.docket.local.yml`: a machine-scoped value for these would silently split the backlog across machines or mint external GitHub objects. Set them in the repo's committed `.docket.yml` only.

**Misplacement fails loud.** A `~/.config/docket/.docket.yml` is never read — `docket-config.sh` warns and points at `config.yml`. A malformed/unreadable `config.yml` (or `.docket.local.yml`) warns and falls back to built-ins for that layer only — the repo, and its other layers, are still honored; a broken personal or machine file never bricks a repo.

**Migrating from `agents.yaml`.** The old single-purpose global file (`~/.config/docket/agents.yaml`) is migrated automatically: the next `sync-agents.sh` (or `install.sh`) run rewrites it under `agents:` in `config.yml` and renames the original to `agents.yaml.migrated`. Nothing reads the old file after migration.

### `.docket.local.yml` — the machine-local layer

`<repo>/.docket.local.yml` is an optional, **gitignored** sibling of the repo's committed `.docket.yml` — a machine-*and*-repo-scoped override that never leaves this clone: a personal model preference, a local `finalize.test_command`, or a way to try `agent_harnesses` before committing it for the team. It accepts exactly the same **global-able** key set as `config.yml` above — the coordination-key fence applies here too, verbatim: a fenced key set locally is loudly warned-and-ignored, never honored, never fatal.

```yaml
# <repo>/.docket.local.yml — optional, gitignored; overrides ONLY on this machine, for this repo.
# Accepts the same global-able keys as ~/.config/docket/config.yml. Fenced keys (metadata_branch,
# integration_branch, changes_dir, adrs_dir, results_dir, github_project, and board_surfaces'
# github token) are warned-and-ignored here too — set those in the committed .docket.yml instead.
skills:
  build: auto
agents:
  default:
    implement-next: { model: claude-opus-4-8, effort: xhigh }
agent_harnesses: [claude]     # can opt a tracking-only repo into per-repo agent generation on
                              # its own, without touching the committed .docket.yml
finalize:
  gate: local
auto_groom: false
board_surfaces: [inline]      # the github token is fenced here too — per-repo-only
```

Its own path (and every file `sync-agents.sh` generates) is kept out of git by a managed `# docket:generated` block the script owns in the repo's `.gitignore` — see **Tuning an agent's model & effort** below.

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

A repo that has been running in single-branch mode (everything on `main`) moves to docket-mode with a one-shot, idempotent script: **`migrate-to-docket.sh`** (it ships in this docket repo, alongside `link-skills.sh` and `sync-agents.sh`). The script operates on the git repo containing your **current directory** — so run it *from within the repo you want to migrate*, pointing at the script wherever docket is checked out:

```bash
cd <target-repo>
bash /path/to/docket/migrate-to-docket.sh
```

It prints the resolved target repo and prompts for confirmation before changing anything; pass `--yes` (or `-y`) to skip the prompt in automation. It then creates the orphan `docket` branch seeded from your current planning directories, prunes the live planning surface (`active/` changes, the changes `README.md`, `BOARD.md`) off the integration branch while keeping terminal records and build artifacts there, and adds `.docket/` + `.worktrees/` to `.gitignore`. Re-running it converges from any partial state.

Migration also grants one **local, per-repo** Claude Code permission: an allow-rule for docket's terminal-publish push to the integration branch (written to `.claude/settings.local.json`, which migration adds to `.gitignore`). This pre-authorizes the one push the permission classifier guards on every close-out, narrowly and only in this repo — force-pushes and pushes to other branches stay guarded. Because `settings.local.json` is gitignored and per-user, anyone who later **clones** an already-migrated repo can grant themselves the same rule by running the helper standalone:

```bash
bash /path/to/docket/scripts/ensure-claude-settings.sh
```

The skills will **not** migrate a repo for you. On first run against an un-migrated repo (metadata still on the integration branch, no `docket` branch yet), a **bootstrap guard** STOPs and points you at `migrate-to-docket.sh` rather than silently moving your data. The same guard detects a half-finished migration and points back to the script to complete it.

### `main`-mode: the single-branch opt-out

If you want everything on one branch — for example, a small repo, or a team that prefers all state in one place — pin both knobs:

```yaml
metadata_branch: main
integration_branch: main
```

This reproduces the original single-branch behavior **exactly**: no `docket` branch, no `.docket/` worktree, no terminal-publish copy. Planning commits land on the integration branch alongside your code, and the archive move there *is* the terminal record. Because docket-mode is now the default, an existing single-branch repo must pin `metadata_branch: main` to keep running as-is until it deliberately migrates — otherwise the bootstrap guard will stop and ask it to migrate.

---

## Tuning an agent's model & effort

Each **autonomous** docket skill runs as a model/effort-pinned subagent (`docket-implement-next`, `docket-auto-groom`, `docket-finalize-change`, `docket-status`, `docket-adr`; the two interactive skills, `docket-new-change` and `docket-groom-next`, stay inline and only surface an advisory recommendation). To change the model or effort one of them runs at:

**1. Edit a config layer.** Up to three layers override the built-in default, resolved per field (precedence: repo-local > repo-committed > global > built-in):

- **Global** — the `agents:` block in `~/.config/docket/config.yml` (user-level; applies to every repo on your machine; the legacy `agents.yaml` is auto-migrated into it — see **Global config** above).
- **Repo-committed** — the `agents:` block in a repo's committed `.docket.yml` (applies to that repo for every clone and agent).
- **Repo-local** — the `agents:` block in that repo's `.docket.local.yml` (this machine only; wins over the committed value for this clone — see **`.docket.local.yml`** above).

The config **shape** — the `agents:` keys and how `model:`/`effort:` are written — is documented once in docket-convention's **"Agent layer"**; consult it there rather than copying field examples here, so the shape has a single source of truth and stays current as it evolves.

**Changing only the model?** To override an agent's model while *dropping* its pinned effort — e.g. pointing it at a non-Claude harness model, where Claude's effort tiers do not apply — set `effort: auto`, which drops the effort line entirely so the agent inherits the model default. Omitting the `effort:` key instead *keeps* the built-in effort, so `auto` is the explicit way to drop it.

**2. Refresh the generated wrappers.** The resolved model/effort are baked into generated wrapper *copies* (not symlinks), so after editing any layer, regenerate them:

```bash
bash sync-agents.sh        # or re-run install.sh, which calls it for you
```

- A **global** edit rewrites user-level wrappers into every **present** harness root (`~/.<harness>/agents/`, e.g. `~/.claude/agents/`).
- A **repo-committed or repo-local** edit rewrites that repo's per-repo wrappers for each harness in its (local-then-committed) `agent_harnesses:` list (default `[claude]`; e.g. `[claude, cursor]` for a repo that also drives Cursor).

Note `sync-agents.sh` always writes **both** passes in one run — user-level wrappers into each targeted harness root AND (for opted-in repos) per-repo wrappers — project wins over global at generation time, per the four-layer precedence above.

**Generated per-repo agent files are machine-local — gitignored, never committed.** Unlike a repo's committed `.docket.yml`, `<repo>/.<harness>/agents/docket-*.md` (and, for Cursor, `docket-dispatch.mdc`) are regenerated on every machine from that machine's own resolved config; they carry no team intent of their own — the committed `agents:` block is the artifact that does. `sync-agents.sh` owns a marker-bounded `# docket:generated` block in the repo's `.gitignore`, covering every file it can generate for every harness (plus `.docket.local.yml` itself); it creates or repairs that block the moment a repo opts in — declares an `agents:` block or an `agent_harnesses:` key, in either file, or merely carries a `.docket.local.yml` — and prints a loud one-time notice to **commit it once**. After that the block is invisible plumbing.

**Migrating a pre-0051 repo.** Repos that predate this (change 0048 committed the per-repo files directly) get a one-time, automatic migration on the next `sync-agents.sh` run: it deletes the stale tracked copies from the working tree, writes the `.gitignore` block, regenerates the local set fresh, and prints the single remedy commit to run — `git rm -r --cached '.claude/agents/docket-*.md' … && git add .gitignore && git commit -m "…"` — so the repo converges in one commit per clone.

**3. Guard drift in CI.** `sync-agents.sh --check` is a three-part gate:

- The `.gitignore` `docket:generated` block is present and current, **and** no per-repo generated file is tracked by git — both are **CI-meaningful** (`rc != 0` fails the build; the second leg also catches a repo whose migration commit never happened).
- Generated content drifting from the resolved config is **advisory only** (`rc` unaffected) — every clone regenerates its own copy at build time, so a stale local file is a nudge to re-run `sync-agents.sh`, not a CI failure.

**Always the full set, plus a Cursor dispatch rule.** The per-repo layer writes the **full built-in agent set** for every harness in `agent_harnesses` (the `agents:` block only *overrides* model/effort — it never decides which agents exist). It is **opt-in**: a repo opts in by declaring an `agents:` block or an `agent_harnesses:` key, in **either** its committed `.docket.yml` or its local `.docket.local.yml`; a repo with neither key set in either file generates no per-repo wrappers and its `--check` stays a no-op. A repo listing `cursor` also gets a generated `.cursor/rules/docket-dispatch.mdc` that forces Cursor to dispatch docket agents instead of running them inline. `sync-agents.sh --check` covers both the generated agents and the dispatch rule.

**The clone-identical guarantee is retired.** Before this change, committing the generated per-repo files meant an autonomous change built on the exact same model on every clone, by construction. Generation is now all-local, so that guarantee is gone — a deliberate trade, not an oversight: never having to reconcile a machine-generated file in a PR diff, at the cost of no CI-enforced pinning of the generated copies. Team defaults for a repo still live in its committed `.docket.yml` `agents:` block, by convention.

---

## The eight skills

| Skill | Role |
|---|---|
| `docket-new-change` | Producer, interactive — turns an idea into a build-ready change via brainstorming; writes markdown only, never branches or code. |
| `docket-groom-next` | Interactive groomer — selects the next needs-brainstorm stub deterministically and designs it to build-ready with the human; abstained auto-groom stubs come first. |
| `docket-auto-groom` | Autonomous groomer — drains the auto-groomable needs-brainstorm queue with no human: default-biased self-brainstorm gated by an adversarial critic; emits specs/trivial verdicts or abstains back to the human queue; never kills or defers. |
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
