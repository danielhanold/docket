# docket

docket keeps a backlog of planned work as plain markdown files that live inside your repo, and ships a set of agent skills that work that backlog for you. Each unit of work is a **change** — one markdown file, roughly one pull request's worth of work, with a status that moves through a fixed lifecycle. You design changes interactively in your agent harness; an autonomous implementer skill drains them to open pull requests one at a time, and you stay in control at the merge gate — all coordinated through git, with no CLI to install and no database to run.

What you get:

- **A durable backlog that outlives the session.** Planned work is tracked in-repo as markdown, so a change you brainstorm today is still there, with its full context, when an agent picks it up next week.
- **Hands-off implementation.** An autonomous skill claims the next ready change, refreshes it against the current state of the code, builds it with test-driven development, and opens a PR — with no supervision in between.
- **You stay at the merge gate.** Agents never merge. Your review of the pull request is the one required human checkpoint on the way to `done`.
- **No new infrastructure.** No service, no database, no bespoke CLI — just markdown files, git, and skills any supported agent harness can run; Claude Code, Cursor, Codex, and opencode are first-class.
- **The right model for each step.** Every autonomous skill is pinned to its own model and effort, so a board refresh runs at a cheap tier while a build runs at a top one — in the same session, with no model choice from you. See [Tuning agent models & effort](#tuning-agent-models--effort).

---

## Table of contents

- [How it works](#how-it-works)
- [Why docket](#why-docket)
- [Install](#install)
- [Updating docket](#updating-docket)
- [Quickstart: the daily loop](#quickstart-the-daily-loop)
- [Configuration — `.docket.yml`, global config, and machine-local overrides](#configuration--docketyml-global-config-and-machine-local-overrides)
- [docket-mode: where metadata lives](#docket-mode-where-metadata-lives)
- [Tuning agent models & effort](#tuning-agent-models--effort)
- [Skills](#skills)
- [Learnings — the loop's memory](#learnings--the-loops-memory)
- [Customization](#customization)
- [Status](#status)
- [Migration](#migration)

---

## How it works

docket runs as two roles you can think of as a producer and an implementer. You are the producer; an autonomous skill is the implementer. They never talk to each other directly — they coordinate entirely through the change files committed in git, with no locks, no message queue, and no database.

| Role | Skill | Mode |
|---|---|---|
| **Producer** | `docket-new-change` | Interactive — you brainstorm ideas into a backlog of build-ready changes. |
| **Implementer** | `docket-implement-next` | Autonomous — drains that backlog to open PRs, unsupervised. |

A change is **build-ready** — ready for the implementer to pick up — when it is `proposed`, has a written spec (or is marked `trivial` for a small mechanical change), and all its dependencies have already merged. The producer only ever mints new `proposed` change ids (it scans the highest existing id and increments). The implementer claims the next build-ready change atomically — a compare-and-swap on the change's status field — so two implementer runs can never grab the same change, even running in parallel.

The payoff: your interactive time is concentrated at change creation. Draining is hands-off for changes that don't depend on each other. You stay in the loop at exactly two points — writing the change, and merging the PR.

### The change lifecycle

Each change is one markdown file whose front-matter carries a `status`. The status walks a fixed happy path:

```
proposed  →  in-progress  →  implemented  →  done
```

with three off-ramps: `blocked` (an external blocker is recorded), `deferred` (consciously shelved, may revive), and `killed` (abandoned — kept in the archive as a record). A `proposed` change that has not yet been designed enough to build (no spec, not marked trivial) sits in a **needs-brainstorm** state until it is groomed.

There is one detour on the way to `done`: `stacked-merged`. A change built on another change's unmerged branch merges into that parent rather than into your integration branch, so its code hasn't shipped yet — it parks at `stacked-merged` and is promoted to `done` only once the root of its stack lands.

There is also one edge running backward: `in-progress → proposed`. A claim carries a lease, and a change whose lease has expired with no feature branch to show for it — the crashed-before-push case — self-heals back to `proposed` rather than sitting stuck; see [Reclaiming stale claims](#reclaiming-stale-claims-reclaim).

The **board** — a generated `BOARD.md` — is the at-a-glance view of every change grouped by status; you regenerate it with `docket-status`.

One honest caveat: dependency chains serialize on the merge gate. A change that depends on another cannot start until that dependency's PR is merged; the board surfaces this as "waiting on #N — needs your merge." Unrelated changes drain freely in parallel around it.

---

## Why docket

If you already run coding agents in your repos, you have probably felt the gap docket fills.

**superpowers gives you excellent execution but no memory.** Its workflow — brainstorm → spec → plan → TDD → code review → merge — runs well, but all inside a single invocation. It has no notion of a tracked backlog or a "done" state that outlives the session. Every session starts from a blank slate.

**OpenSpec-style tools close that gap, but heavily.** They add a full living-spec lifecycle — at the cost of a CLI dependency and a rigid markdown contract that not every project wants to adopt.

**docket sits in between.** It adds a thin lifecycle layer — plain markdown files in your repo, a handful of skills, no CLI — and by default delegates every execution step back to superpowers (each step is individually rebindable; see [Configuration](#configuration--docketyml-global-config-and-machine-local-overrides)). The code stays the single source of current-state truth; docket keeps no living-spec layer and never tries to mirror the codebase in prose. Architecture decisions are recorded separately, as ADRs — an immutable ledger.

But the thin lifecycle is not the real reason to use docket. This is.

### The reconcile superpower

This is the most valuable and least obvious part of docket.

**The problem.** A change is drafted against a *snapshot* of the world — the codebase, the ADR ledger, and the other in-flight changes as they stood on the day you brainstormed it. In an async backlog, the implementer may not pick it up for a week or a month. By then:

- Another change may have shipped work this one was planning to add.
- An ADR may have settled an open question — in the opposite direction.
- A dependency may have changed the interface this spec assumed.

Most backlog-driven systems build the ticket as written and let the implementer discover the mismatch halfway through. The classic results: re-implementing something already done elsewhere, or building something a later architecture decision has invalidated.

**What docket does instead.** `docket-implement-next` includes a **reconcile step** that runs at the *last responsible moment* — after the change is claimed (so it belongs to this run) but before the change's worktree (an isolated working copy) and plan are created (so no build work is wasted if the scope shifts). The reconcile pass re-reads the change and its spec against:

- `related` and recently archived changes (to find work already done),
- cited and recent ADRs (to find new constraints),
- the current code (to find interface drift).

It then rewrites the change body and spec to what is true now: it drops work done elsewhere, adjusts scope, and folds in new constraints. The change file gets a dated `## Reconcile log` entry, and `reconciled: true` is recorded as an audit trail.

Two escape hatches handle the cases a rewrite can't:

- If the change is now **entirely obsolete**, it is killed and the implementer loops back to select the next one.
- If the design is **fundamentally invalidated** — it needs re-thinking, not just scope-trimming — the implementer stops and escalates to you. Re-brainstorming needs a human and `docket-new-change`; the autonomous implementer will not do it alone.

**The stance: plans rot. Refresh them just-in-time; never trust a stale backlog.** The `reconciled` flag is the visible proof that a change was freshened against reality before implementation began. If an `in-progress` change resumes after a crash or interruption with `reconciled` still `false`, the full reconcile pass runs again before any work continues. And if it never resumes at all — the implementer crashed before it even pushed a branch — that isn't a dead end either: see [Reclaiming stale claims](#reclaiming-stale-claims-reclaim).

---

## Install

docket installs once per machine and then works in every repo you use it from.

### Prerequisites

- **An agent harness.** docket's skills run inside an agent **harness** — an agent front-end with its own on-disk `skills/` and `agents/` directories that docket writes into. **Claude Code, Cursor, Codex, and opencode** are first-class supported harnesses; docket also writes into `.agents/`, `.kiro/`, and `.windsurf/` harness roots when they are present on your machine.
- **`git` and the GitHub CLI (`gh`).** Every docket operation is a git operation, and the implementer opens pull requests with `gh`.
- **A GitHub remote** for the pull-request flow. docket pushes branches and opens PRs against your `origin`.
- **The superpowers plugin — recommended, not required.** superpowers is docket's default execution engine (brainstorm, plan, build, review, finish). Installing it is the consuming user's responsibility; docket does not bundle or fetch it. If it is absent, each workflow step **degrades to running inline at the agent's own model, with a prominent warning** — so docket still works out of the box with zero config, just without superpowers' structured execution. See [Configuration](#configuration--docketyml-global-config-and-machine-local-overrides) to rebind any step.

### 1. Install docket on your machine

Place the docket repo at `~/dev/docket` (the source of truth the symlinks point back to), then run:

```bash
bash ~/dev/docket/install.sh
```

That is the whole install. `install.sh` is a thin bootstrapper: it resolves this checkout and hands
the install off to docket's Go engine (`docket development install`), which does the real work as
**one journaled, all-or-nothing transaction** and is idempotent — re-run it any time (after adding a
harness, after editing `~/.config/docket/config.yml`, and after every version update — see
[Updating docket](#updating-docket)). A single run:

- **Builds a fresh binary and hands the install off to that binary**, so the version that plans and writes your machine is the one you are installing — never the older binary that happened to be running. The recursion-guarded dispatch wrappers therefore land on the **first** run, not the second.
- **Links each present harness's global `skills/`** back to `~/dev/docket/skills/<name>` (symlinks, so editing a skill in the repo takes effect everywhere at once) and **reconciles that harness's global agent wrappers** — the model/effort-pinned subagent copies, resolved from your config layers over docket's shipped defaults. It also points `~/.config/docket/config.yml` at [`.docket.example.yml`](.docket.example.yml), docket's canonical reference for every key and its default (see step 2).
- **Retires the old global parent-facing dispatch blocks** that earlier docket versions wrote into your personal `~/.claude/CLAUDE.md` and the other harnesses' global instruction files, while keeping the global skills and agent wrappers. Removal is **proof-gated** — the engine deletes a block only while it still matches docket's exact ownership marker, byte for byte. There is **no `--force`**: a block you edited, or one that no longer matches, is left untouched and the run reports it so you can remedy it and re-run.
- **Reconciles each repository's parent-facing dispatch surfaces** from that repository's *explicit* `agent_harnesses` opt-in (see [step 2](#2-set-up-your-global-config) and below) — automatic and Go-owned, with no separate Bash synchronization script to run.

Two flags scope a run: **`--repo-dir <path>`** targets a repository other than the one containing your
current directory, and a repeatable **`--harness <name>`** limits the run to the named harness(es)
instead of every harness present on your machine.

### 2. Set up your global config

The installer writes a minimal `~/.config/docket/config.yml` the first time it runs and
non-destructively maintains its managed values. Docket's ordinary behavior defaults already apply.

The canonical reference for every key is [`.docket.example.yml`](.docket.example.yml) in this repo: every config key, active at its shipped default, with full documentation and a scope tag saying which layers may set it. Copy the keys you want to change into the layer you want them in.

- **To see docket's built-in per-skill model and effort:** they all live in one file, [`agents/harness-defaults.yml`](agents/harness-defaults.yml) — docket's shipped, harness-indexed default sidecar, not a file you edit. All four of the example's commented harness blocks — `agents.claude`, `agents.cursor`, `agents.codex`, and `agents.opencode` — mirror it in full, value for value. So you can read every shipped default and tune the ones you want in a single place.
- **Claude-only users can skip this entirely** — the defaults already apply.
- **To enable another harness (Cursor, Codex, opencode):** add it to `agent_harnesses` and re-run `install.sh`; the Go engine reconciles that harness's wrappers and dispatch surfaces for you. That is the whole step — leave the harness's `agents:` block commented, since it only restates the shipped defaults and uncommenting it would freeze today's values into your config forever. `agent_harnesses` is the **explicit opt-in** for a repository's parent-facing dispatch surfaces, and it has **three states**: *absent* leaves the shipped default (Claude only) in force and writes no other harness's repository surfaces; a *non-empty* list reconciles exactly the harnesses you name; and an *explicit empty* list (`agent_harnesses: []`) retires every docket-owned repository surface the repo previously had. An absent key touches nothing — only an explicit value reconciles or retires.

> **Stale project-level Claude wrappers shadow the guard.** Docket installs agent wrappers
> **machine-globally** (under `~/.claude/agents/`), never inside a repository. If a repo still carries
> its own `.claude/agents/docket-*.md` copies — as docket versions before the recursion-guard change
> left behind — Claude Code loads *those* project-level wrappers in preference to the guarded global
> ones, which re-enables recursive self-dispatch. The remedy is to **delete those project-level
> copies**; docket will not touch them, because it never owned them.

> **Start a fresh harness process after any install that changed a wrapper or a parent surface.**
> Harnesses register their agents and read their global and repository instruction files **at process
> start**, so a changed dispatch wrapper or a retired dispatch block only takes effect in a newly
> started process — **clearing a conversation is not enough**. This is what lets the recursion guard
> actually take hold: until the process restarts, the old, unguarded wrappers keep running.

See [Configuration](#configuration--docketyml-global-config-and-machine-local-overrides) for the layer model.

The change data — `docs/changes/`, `docs/adrs/`, `docs/results/` — lives in each consuming project, not in the docket repo itself. To adopt docket in an *existing* repo, run `migrate-to-docket.sh` from inside that repo — a separate step from this machine install (see [Migration](#migration)).

---

## Updating docket

**Every time you pull a new version — a `git pull` on `main` or a checked-out release tag — re-run `install.sh`.** It is the catch-all: it applies whatever the new version needs on your machine, and it is idempotent, so running it when nothing changed is a no-op.

```bash
cd ~/dev/docket
git fetch --tags && git pull        # or: git checkout v0.8.0
bash ~/dev/docket/install.sh        # always — not only when something looks broken
```

Pulling alone is **not** enough. Skills are symlinks, so those do update the moment you pull — but the rest of docket's on-disk footprint is generated or persisted, and only an install run refreshes it:

- **Agent wrappers are generated copies**, not symlinks — they bake in the resolved model and effort. A version that adds a subagent, renames one, or changes a pin lands only when the installer reconciles the wrappers.
- **New harness support**, and any harness you installed since last time, gets its `skills/` symlinks and `agents/` wrappers only on the next install run.
- **Managed global config** in `~/.config/docket/config.yml` is back-filled non-destructively by the same run.
- **Retired global dispatch blocks and reconciled repository surfaces** land on this run too — which is why the recursion-guarded wrappers you are pulling only take effect after it, in a freshly started harness process (clearing a conversation is not enough).

Re-running it is **in addition to** anything the release notes call for, never a substitute. A release may also carry a per-repo step — a `migrate-to-docket.sh` run, a `.docket.yml` key to add, a remedy commit to land — and those are listed in the notes for that version. Do the machine-level `install.sh` first, then the per-repo steps.

---

## Quickstart: the daily loop

Once docket is installed and your repo is in [docket-mode](#docket-mode-where-metadata-lives) (planning metadata on its own branch), a day of work is a handful of skill invocations you make (by name) in an agent-harness session — Claude Code, Cursor, Codex, or opencode — opened in that repo.

**1. Capture work — `docket-new-change`.** Describe an idea; docket brainstorms it with you into a build-ready change — a spec, dependencies noted — and commits the change file to the backlog. Run it whenever an idea lands; the backlog is durable, so you can capture now and implement later. (A pinned, high-tier consultant can author the spec instead of your session model — see [Consultant-authored brainstorm](#consultant-authored-brainstorm-opt-in).)

**2. (Optional) Groom stubs to build-ready — `docket-groom-next` / `docket-auto-groom`.** If you captured rough stubs rather than fully-designed changes, grooming turns the next `needs-brainstorm` stub into a build-ready change. `docket-groom-next` does it interactively with you; `docket-auto-groom` drains the auto-groomable stubs with no human, each design gated by an adversarial critic. (Both write markdown only — never code.)

**3. Drain the backlog — `docket-implement-next`.** This is the autonomous workhorse. It claims the next build-ready change, reconciles it against current reality (see [Why docket](#why-docket)), plans it, builds it with test-driven development, opens a pull request, and stops. It never merges. Run it as many times as you have ready changes; independent changes drain back-to-back with no input from you.

**4. Review and merge.** Read the PR it opened. This is your one required checkpoint — merge it yourself, or let `docket-finalize-change` merge it once you have approved it.

**5. Close out — `docket-finalize-change`.** After the PR is approved or merged, this archives the change to `done`, publishes its terminal records if the repo has opted in, cleans up the branch and worktree, and refreshes the board. (A periodic `docket-status` run also sweeps already-merged PRs to `done` and regenerates `BOARD.md`, so close-out still happens on its own if you skip this step.)

In short: **you** create and merge; **docket** grooms, implements, and closes out. `docket-status` keeps the board honest in between.

### Draining hands-free with `/loop`

`docket-implement-next` ends every run by declaring one of four **dispositions** — `advanced` (built a change → PR), `contended` (lost a claim race, nothing built), `drained` (nothing build-ready in scope), or `halted` (needs a human). A driver keys on these: **continue on `advanced`/`contended`, stop on `drained`/`halted`.** The contract is driver-agnostic — a human re-typing the command works as well as any loop runner.

The recommended driver is the built-in **`/loop`**, which forks a fresh implementer each iteration so the heavy build stays in the fork and the loop context stays small:

- `/loop docket-implement-next` — self-paced; drains the whole build-ready backlog, stopping on `drained`.
- `/loop docket-implement-next 90,92,94` — drains only that id set (deterministic order within it); a scoped change that is not build-ready — needs-brainstorm, already in progress, or waiting on an unmerged dependency — is skipped with its reason.

Budget and iteration caps are `/loop`'s own mechanism; docket does not reimplement them. The driver **never merges** — the human merge gate is untouched, so a dependency only clears between drains via a merge performed outside this loop: a human clicking Merge, or a finalize drain ([Closing out hands-free with `/loop`](#closing-out-hands-free-with-loop)). A scoped change waiting on an unmerged dependency is skipped this drain, not waited on. Confirm `/loop` composes cleanly with the forked skill in your own harness before relying on it unattended — harness behavior is version- and mode-scoped.

### Closing out hands-free with `/loop`

`docket-finalize-change` ends every run declaring one of the **same four dispositions** — `advanced` (merged one change and closed it out), `contended` (another writer got there first, nothing merged), `drained` (nothing eligible in scope), or `halted` (needs a human) — so a single driver keys on both halves of the loop without knowing which one it is running: **continue on `advanced`/`contended`, stop on `drained`/`halted`.**

- `/loop docket-finalize-change` — closes out every eligible `implemented` change, **one merge per iteration**, stopping on `drained`.
- `/loop docket-finalize-change 90,92,94` — bounds the run to that id set. **Naming the ids is the authorization** the *attended* multi-candidate prompt would otherwise have collected. Neither drain prompts (one merge per iteration is never a batch); what naming the ids adds is that it merges PRs `require_pr_approval` would otherwise hold, and retries a change already marked `## Finalize blocked`.

Unlike the implementer, **this driver does merge** — that is the whole point of it, and it is the one place docket itself merges. Every merge still passes the rebase-retest gate, so `finalize.gate` remains your correctness control; set it to `off` only if you trust each PR's own CI.

**Prerequisite:** an unattended merge only lands if your branch protection permits it — see [Hands-off finalize — what blocks it, and the recipe that works](#hands-off-finalize--what-blocks-it-and-the-recipe-that-works) for the require-a-PR-with-zero-approvals setting this depends on. Without it the drain stops at `halted` on the first merge.

Selection is ordered by *mergeability* rather than priority — `depends_on` order first (a hard constraint), then GitHub's `mergeable`, then the smallest diff, with priority → age → id as the tiebreak — so each drain lands as many changes as it can before anything stops it. A change whose gate fails is marked with a `## Finalize blocked` section (dated in its body), shows on the board as **finalize blocked — needs you**, and is skipped by later *unscoped* runs until a successful finalize clears it automatically — **name its id to retry it**. As with the implementer, confirm `/loop` composes cleanly in your own harness before relying on it unattended.

---

## Configuration — `.docket.yml`, global config, and machine-local overrides

docket works with zero configuration. Everything below is optional — reach for it when a default doesn't fit.

Configuration resolves **per key**, across up to four layers, with precedence **repo-local > repo-committed > global > built-in**:

1. **Repo-local** — a repo's `.docket.local.yml` (this machine only). Wins first.
2. **Repo-committed** — that repo's committed `.docket.yml` (every clone).
3. **Global** — the cross-repo `~/.config/docket/config.yml` (this machine, every repo).
4. **Built-in** — docket's defaults.

`runtime.bash` is the deliberate exception: it is machine identity, so resolution is
**repo-local > global** and a committed value is warned-and-ignored.

Map-valued keys (`skills:`, `agents:`) merge field-by-field with the same precedence, so a global default and a repo override can each set different fields of the same map.

### `.docket.yml` — per-repo settings

Add a `.docket.yml` to override defaults for one repo. It is committed on your repo's **default branch** (`origin/HEAD`), because every clone, agent, and device needs the same coordination values, and the default branch is the one place a skill can find the file with zero prior config. It is committed (not gitignored) for that reason.

Every key is optional; unset means the shipped default. Set only what you want to change:

```yaml
# .docket.yml — committed on the repo's default branch; read by every docket skill at startup.
metadata_branch: docket      # docket (default) | main  — where planning commits land
integration_branch: auto     # auto (default → origin/HEAD) | main | develop — where code lands
board_surfaces: [inline]     # derived board views: inline (BOARD.md) and/or github; [] disables the board
finalize:                    # merge gate: rebase onto base + re-test before docket merges
  gate: local                # local (default) | ci | both | off
```

**[`.docket.example.yml`](.docket.example.yml) is the canonical reference for every key** — each one
shipped active at its default, with full documentation and the layers it may be set in. Copy from
there rather than from this snippet; it is the surface the test suite keeps honest against the
resolver, and it is deliberately the only place that enumerates all of them.

With no `.docket.yml` at all, docket runs in its default **docket-mode** (`metadata_branch: docket`, `integration_branch: auto`). See [docket-mode](#docket-mode-where-metadata-lives) for what that means and how to opt out.

### Reclaiming stale claims (`reclaim`)

A `docket-implement-next` run that crashes or is killed before it ever pushes a branch leaves its change stuck at `in-progress` — but that particular case doesn't need a human to notice and fix it by hand. `reclaim` closes it automatically, for the one situation it can close *safely*: an **expired claim lease with no feature branch**. Every claim stamps `claimed_at:`; once `NOW - claimed_at` exceeds `reclaim.lease_ttl` (hours, default `72` — >= `docket-status`'s 3-day stale-in-progress window) *and* no `feat/<slug>` branch exists anywhere `docket-status` can see, the change is eligible to flip back to `proposed` — the new `in-progress → proposed` edge in the lifecycle.

- **Detection is always on.** `docket-status` flags every eligible change on each run, regardless of `reclaim.auto`.
- **Mutation is opt-in.** `reclaim.auto: false` (the default) — `docket-status` only recommends: `reclaim: <n> expired-lease change(s) can self-heal — run: docket change reclaim`. `reclaim.auto: true` — `docket-status` reclaims eligible changes itself on every pass.
- **Run it by hand anytime** with `docket change reclaim --id <n> --version <v>` (per change, at its exact recorded version), whether or not `reclaim.auto` is set.
- **A change with a branch is left to a human.** It might carry real, unpushed work, so reclaim never touches it — it stays flagged instead.
<!-- docket:config-fence: values -->
```yaml
reclaim:
  lease_ttl: 72   # hours; >= docket-status's 3-day stale-in-progress window
  auto: false     # true => docket-status self-heals eligible claims each pass
```

### Capturing discovered work (`auto_capture`) and typing it (`change_types`)

Agents constantly surface follow-up work mid-task: a reconcile pass notices an adjacent gap, a build
uncovers a latent bug, a close-out finding implies a next step. With a human in the room the model
asks. In an unattended run there is nobody to ask, so that work is mentioned in the run's final
report rather than acted on silently.

The `auto_capture` map (`enabled`, `types`) remains a parseable configuration key and its stored
values are never rewritten, but it activates nothing: **automatic change capture is deferred from
Go v1 — capture work deliberately with `docket change create`**. An enabled value is answered with a
capability-specific unsupported diagnostic before any leg mutates state, never a Bash fallback — so a
run **reports** the follow-up work it discovers and a human files it deliberately.

- **Parseable, inactive.** With `auto_capture` set (`enabled: true` or `false`) the resolver still
  reads it and never rewrites your stored values; no run mints a stub from it.
- **Where discovered work goes.** `docket-implement-next` (reconcile and review) and the close-out
  pass surface genuine follow-up work **in the run's final report**. Build-loop lessons go to
  [learnings](#learnings--the-loops-memory); drift inside the current change goes to its reconcile
  log.
- **The taxonomy still governs creation.** `change_types` and `auto_capture.types` remain the
  documented vocabulary a deliberately-created change draws from (see below): they type the work you
  file, they do not schedule its capture.

```yaml
change_types: [chore, docs, feat, fix, refactor, perf]   # the taxonomy — see below

auto_capture:
  enabled: true
  types: [feat, fix]                                     # a subset of change_types, or `all`
```

Work you file with `docket change create` appears on the board as ordinary `needs-brainstorm` work
and flows into `docket-groom-next`'s queue like anything else you filed by hand.

#### The taxonomy (`change_types`)

`change_types` is the vocabulary a change's `type:` may draw from. It governs what this machine
**creates**, never how shared history reads: a change carrying a type you do not configure still
renders on the board and still answers `--type` queries, because configuration is not allowed to
make someone else's work unreadable.

- **Default.** `[chore, docs, feat, fix, refactor, perf]` when no layer sets it.
- **Replaced, never merged.** The first layer that sets `change_types` wins *entirely*. Merging
  would make a built-in value unremovable — you could only ever add types, never drop one — so
  restating the whole list is how you remove `perf`, or add something like `spike`.
- **Grammar.** Each entry matches `[a-z][a-z0-9-]*`. Duplicates and an empty list are config
  errors. `all` and `untyped` are reserved: they are query pseudo-values, never stored types.
- **Global-able.** Set it per-repo, in `~/.config/docket/config.yml`, or in `.docket.local.yml`.

```yaml
# ~/.config/docket/config.yml — drop a built-in, add your own; the list REPLACES the default
change_types: [chore, docs, feat, fix, spike]
```

`auto_capture.types` must be drawn from the taxonomy, but it is validated against the
`change_types` **visible to the layer that set it**. So narrowing `change_types` on one machine
never invalidates a `types` list inherited from the committed config — the two keys resolve
through independent chains, and each layer only has to be internally consistent.

Every active board row carries a **Type** cell, and `docket-status` and `render-board` both take
report-only `--type` / `--priority` filters: `--type untyped` finds changes carrying no type at
all, and `--type all` (the default) selects everything. These narrow the **digest only** — never
the board, the merge sweep, archiving, harvesting, health checks, or any write.

#### Migrating to typed changes

Change 0127 made `auto_capture` a map. **The old scalar is a hard error with no compatibility
shim** — the resolver refuses to start and prints the replacement, carrying your own value:

```yaml
# before — no longer valid
# auto_capture: true

# after
auto_capture:
  enabled: true
  types: all
```

Then categorize the active backlog once. Archived changes are never reclassified; every creation
path writes a type from here on, so the untyped set can only shrink.

```bash
# 1. the exact inventory — --digest-only keeps this a WRITE-FREE read. A bare docket-status pass
#    commits and pushes BOARD.md, sweeps merged changes, archives, and harvests before it prints
#    the digest, which is not what you want from a command you are running to *look*.
docket.sh docket-status --digest-only --type untyped

# 2. an agent proposes a complete id -> type mapping; you approve it as one decision

# 3. the deterministic helper validates and applies it — all files or none, and idempotent
docket.sh backfill-change-types --changes-dir .docket/docs/changes --map 7=feat,8=feat,9=fix
```

### Speaking your language (`dummy_mode`)

Docket talks to you in the vocabulary of the repo it is running in. If that repo is a pile of bash
and you read YAML but not bash, or it is Terraform and you have never seen a state file, every
brainstorm question and every run report arrives one translation short — and you end up asking
"say that more simply" over and over. `dummy_mode` moves that translation to the source: you
describe the reader once, and docket's **human-facing** prose is written for that reader from then
on.

```yaml
dummy_mode:
  enabled: true
  persona: "Comfortable with git and YAML, cannot read bash. Explain scripts by outcome."
  surfaces: all   # or a subset — see the tokens below
```

`persona` is free text and must be **one quoted line**: a YAML block scalar (`>` or `|`) and a `#`
inside the value — quoted or not — are both hard config errors, because the config reader is
line-oriented and strips from the first `#`. A `#` *after* the closing quote is an ordinary
trailing comment and is fine. Leave it blank (or omit it) to get the shipped default — a mid-level
engineer who knows architecture, has working-level fluency in any one language, and is told every
project-internal term with a gloss.
`enabled` is `false` by default, and the key is global-able: set it per-repo, in
`~/.config/docket/config.yml`, or in `.docket.local.yml`. It is *primarily* a per-repo setting,
since the same person is an expert in one repo's domain and a novice in another's.

**Five surfaces are eligible**, in two modes:

| Token | Covers | Mode |
|---|---|---|
| `dialogue` | brainstorm and groom conversation, and any human-present prompt | **replace** |
| `reports` | the prose a skill prints back to you at the end of a run | **replace** |
| `results` | the results file a change writes on close-out | **additive** |
| `change-sections` | skill-authored sections in a change file (`## Run halted`, `## Finalize blocked`, …) | **additive** |
| `pr` | the pull-request body | **additive** |

**Replace** means the prose itself is written calibrated to your persona — there is no second,
technical copy, because the decisions and the artifacts behind it are unchanged. **Additive** means
the artifact keeps its full technical content and *gains* an authored `### In plain terms` block
alongside it, written at the same moment as its parent.

**Agent-facing artifacts are never simplified.** Plans, spec files, learnings findings, build
evidence, and script contracts keep full technical density, and an `### In plain terms` block is
never a decision input — reconcile, review, planning, and every build worker read the technical
content only. Simplifying what the loop reads would degrade the loop.

You can also ask for it **ad hoc**: say "enable dummy mode" in a session and the same rules apply
for the rest of that session, using the same configured persona, writing nothing to disk. The
reverse request turns it back off.

#### Persona gallery

Five worked examples, spanning different application types and languages.

**1. Shell/CLI tooling repo (docket itself) — PM-technical reader**

```yaml
dummy_mode:
  enabled: true
  persona: "Comfortable with git, GitHub PRs, and YAML. Cannot read bash or awk — explain script behavior by outcome, never by code. Does not know docket's internal vocabulary (worktree, CAS push, claim lease, orphan branch) — use each term only with a one-clause gloss."
```

Effect: a brainstorm question like "should the CAS retry re-run preflight before the re-push?"
becomes "when two sessions save at the same time, one loses — should the loser automatically
re-sync and retry, or stop and ask you?"

**2. Python data-pipeline repo — analyst reader**

```yaml
dummy_mode:
  enabled: true
  persona: "Data analyst: fluent in SQL and pandas, reads simple Python. No infrastructure background — Docker, Airflow scheduling internals, and IAM are unknown terms. Frame trade-offs in terms of data freshness, correctness, and cost."
  surfaces: [dialogue, reports]
```

**3. TypeScript/React web app — designer/founder reader**

```yaml
dummy_mode:
  enabled: true
  persona: "Non-engineer founder: thinks in user flows and screens, not components or state. Knows what an API is, not what REST vs GraphQL implies. Avoid all TypeScript jargon; describe changes by what the user sees and what could break for them."
```

**4. Terraform/infrastructure repo — application-developer reader**

```yaml
dummy_mode:
  enabled: true
  persona: "Backend application developer (Go, Postgres) new to infrastructure-as-code: plan vs apply, state files, and drift are new concepts. Comfortable with networking basics. Always spell out blast radius: what a change destroys or recreates."
  surfaces: [dialogue, results, pr]
```

**5. iOS/Swift app — backend-engineer reader**

```yaml
dummy_mode:
  enabled: true
  persona: "Backend engineer fluent in Java and REST APIs, new to mobile: SwiftUI, the app lifecycle, and App Store review constraints are unfamiliar. Map iOS concepts to server-side analogies where one exists; flag where the analogy breaks."
```

Compose your own along the axes those five use: **subject-matter gaps** (knows CI/CD, new to
release engineering), **language gaps** (reads Python, not bash), **tooling/vocabulary gaps**
(docket's own terms), and **framing preferences** (what dimensions trade-offs should be expressed
in).

### Workflow roles — the `skills:` map

docket is a lifecycle wrapper around a workflow engine, and superpowers is the default engine for three of the five roles — docket owns the `build` and `review` roles itself. Each of the **five workflow invocation points is a pluggable role**: an optional `skills:` map in any config layer rebinds a role to a different skill (the name is passed to the Skill tool verbatim) or to the sentinel `auto` (no skill — the running agent performs the step inline at its own model).

| Role | Default skill | Invoked by |
|---|---|---|
| `brainstorm` | `superpowers:brainstorming` | `docket-new-change`, `docket-groom-next` — up-front design before the spec |
| `plan` | `superpowers:writing-plans` | `docket-implement-next` — the task plan from the spec |
| `build` | `docket-build` | `docket-implement-next` — execute the plan task-by-task via profile-routed workers |
| `review` | `docket-review` | `docket-implement-next` — whole-branch review before the PR |
| `finish` | `superpowers:finishing-a-development-branch` | `docket-implement-next`, `docket-finalize-change` — push the branch, open the PR |

Unset keys default to the skills shown above: superpowers for `brainstorm`, `plan`, and `finish`, and docket's own `docket-build` and `docket-review` for the two roles docket has since taken ownership of. To run the superpowers engine at every point, set `build: superpowers:subagent-driven-development` and `review: superpowers:requesting-code-review` explicitly. And if a resolved skill cannot be invoked at runtime (superpowers not installed, or a typo'd custom name), docket **degrades to that role's `auto` fallback with a prominent warning**, so a repo without superpowers works out of the box. The config shape — the `skills:` keys, the `auto` sentinel, and each role's fallback artifact — is documented once in docket-convention's **"Skill layer"**; consult it there rather than copying examples here. One caveat worth knowing before you set it: **`build: auto` is dual-purpose** — it authorizes inline building *and* the in-branch fix workers `docket-implement-next` runs on review findings, which execute the build role's own contract. There is deliberately no separate `skills.fix` key; the rule lives in docket-convention's *Dispatch-capability resolution*, Tier C.

### Global config — `~/.config/docket/config.yml`

Cross-repo defaults live in one optional user-level file: `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml`. It accepts the **same schema as `.docket.yml`**; a repo's committed `.docket.yml` wins over it per key.

```yaml
# ~/.config/docket/config.yml — optional; applies to every repo on this machine.
# Same schema as .docket.yml; committed values normally win (runtime.bash is local-only).
runtime:
  bash: /opt/homebrew/bin/bash # managed by install.sh; an explicit valid value is preserved
skills:                      # rebind workflow roles for all your repos
  build: auto
agents:                      # agent model/effort defaults (same agents: shape as .docket.yml)
                             # Write model/effort values unquoted and space-free; `#` cannot
                             # appear inside the `{…}` flow map.
  default:
    implement-next: { model: claude-opus-5, effort: medium }
auto_groom: false
auto_capture:                # a MAP since change 0127 — a bare scalar is a hard config error; capture itself is deferred from Go v1
  enabled: false
  types: all
dummy_mode:                  # persona-calibrated human-facing prose; off by default
  enabled: false
  persona: ""
  surfaces: all
finalize:
  gate: local
board_surfaces: [inline]     # the github token is per-repo-only and ignored here (see below)
agent_harnesses: [claude]    # scopes sync-agents.sh's user-level pass ONLY (overrides
                             # presence-on-disk detection); never the per-repo generation pass
```

### `.docket.local.yml` — the machine-local layer

`<repo>/.docket.local.yml` is an optional, **gitignored** sibling of the repo's committed `.docket.yml` — a machine-*and*-repo-scoped override that never leaves this clone: a personal model preference, a local `finalize.test_command`, or a way to try `agent_harnesses` before committing it for the team. It accepts exactly the same **global-able** key set as `config.yml` above.

```yaml
# <repo>/.docket.local.yml — optional, gitignored; overrides ONLY on this machine, for this repo.
# Accepts the same global-able keys as ~/.config/docket/config.yml. Fenced keys (metadata_branch,
# integration_branch, changes_dir, adrs_dir, results_dir, github_project, terminal_publish, and
# board_surfaces' github token) are warned-and-ignored here too — set those in the committed
# .docket.yml instead.
runtime:
  bash: /usr/local/bin/bash   # optional override for this clone; never commit it
skills:
  build: auto
agents:                       # Write model/effort values unquoted and space-free; `#` cannot
                              # appear inside the `{…}` flow map.
  default:
    implement-next: { model: claude-opus-5, effort: medium }
agent_harnesses: [claude]     # can opt a tracking-only repo into per-repo agent generation on
                              # its own, without touching the committed .docket.yml
finalize:
  gate: local
auto_groom: false
auto_capture:                 # a MAP since change 0127 — a bare scalar is a hard config error; capture itself is deferred from Go v1
  enabled: false
  types: all
dummy_mode:                   # persona-calibrated human-facing prose; off by default
  enabled: false
  persona: ""
  surfaces: all
board_surfaces: [inline]      # the github token is fenced here too — per-repo-only
```

Its own path (and every file `sync-agents.sh` generates) is kept out of git by the managed docket `.gitignore` block (the `# docket:start` / `# docket:end` markers) the script owns — see **Tuning an agent's model & effort** below.

### Coordination keys are per-repo-only

Some keys write shared state, and a machine-scoped value for them would silently split the backlog across machines or mint external GitHub objects. These keys — `metadata_branch`, `integration_branch`, `changes_dir`, `adrs_dir`, `results_dir`, `github_project`, `terminal_publish`, and the `github` token of `board_surfaces` — are therefore **per-repo-only**: they are ignored with a loud warning when set globally **or** in a repo's `.docket.local.yml`. Set them in the repo's committed `.docket.yml` only.

### When a file is misplaced or malformed

- A `~/.config/docket/.docket.yml` is never read — `docket-config.sh` (the per-skill runtime resolver every docket skill consults at startup) warns and points you at `config.yml`.
- A malformed or unreadable `config.yml` (or `.docket.local.yml`) warns and falls back to built-ins **for that layer only** — the repo and its other layers are still honored, so a broken personal or machine file never bricks a repo.

### Migrating from `agents.yaml`

The old single-purpose global file (`~/.config/docket/agents.yaml`) is migrated automatically: the next `sync-agents.sh` (or `install.sh`) run rewrites it under `agents:` in `config.yml` and renames the original to `agents.yaml.migrated`. Nothing reads the old file after migration.

---

## docket-mode: where metadata lives

docket needs a durable, queryable source of truth for planning state — changes, statuses, ADRs, dependencies, the board — shared across agents, machines, and time, with **git as the only persistence mechanism** (no database, no service). docket-mode is how that state is stored, and it is the supported default.

### The two-branch model

Two branches divide the work:

- An orphan **`docket`** branch is the authoritative working surface for **all planning metadata**: change files (active + archive), `BOARD.md`, ADRs, and specs. It is a true orphan — sharing no history with your code, carrying no code — the same well-trodden pattern `gh-pages` uses. (There is no `git checkout --orphan` in the flow: `migrate-to-docket.sh` creates the branch with `git worktree add --orphan`, and a fresh repo's bootstrap builds it straight from git plumbing — `git mktree` + `git commit-tree` — with no working-tree checkout at all.) It is **always pushed**, so the whole backlog, board, specs, and ADRs are browsable and reviewable on the remote (GitHub) at all times. All planning churn — `proposed → in-progress → implemented`, board refreshes, reconcile edits, ADR writes — lands here and never touches your code history.
- Your **integration branch** (`main`, or `develop` under GitFlow) stays code-only, except for **published terminal records** in a repo that opts in (see below). It holds code, the build artifacts that arrive with each PR (plan + results), and — only when `terminal_publish: true` — a copy of the archived change + spec + accepted ADRs once a change closes out.

A change's `feat/<slug>` branch is always cut from `origin/<integration_branch>` — unless it declares `stacked_on:`, in which case it is cut from that parent change's unmerged branch and its PR targets the same (see `stacked-merged` above) — carries only plan + results + code, and never modifies docket metadata.

### Where each artifact lives

| Artifact | Lives on | How it reaches the integration branch | On integration after a terminal change? |
|---|---|---|---|
| Change file (manifest + body) | `docket` | Terminal-publish copy (on `done` or `killed`) | Only if `terminal_publish: true` (Yes, archived) |
| Spec (`docs/superpowers/specs/…`) | `docket` | Terminal-publish copy | Only if `terminal_publish: true` (Yes) |
| ADR (`docs/adrs/…`) | `docket` | Terminal-publish copy, gated on `Accepted` | Only if `terminal_publish: true` (Yes, the `Accepted` ADRs) |
| `BOARD.md` | `docket` | **Never** — it is the live planning view | No — view it on `docket` |
| Plan (`docs/superpowers/plans/…`) | feature branch | The PR merge | Yes (`done` only) |
| Results (`docs/results/…`, i.e. `results_dir`) | feature branch | The PR merge | Yes (`done` only) |
| Code | feature branch | The PR merge | Yes (`done` only) |
| `.docket.yml` | **default branch** (`origin/HEAD`) | n/a — lives on the default branch | Only if the default branch *is* the integration branch (trunk mode); under GitFlow it stays on `main`, not `develop` |

A repo that opts in with `terminal_publish: true` ends up with all five artifacts plus code on the integration branch — they simply arrive by **two paths**. Plan and results are build artifacts that live on the feature branch and ride in through the **PR merge**, regardless of the knob. The change file, spec, and accepted ADRs live on `docket`; in an opted-in repo, a terminal transition **copies** them across (it does not branch-merge `docket`, which would drag all the planning churn onto your code line), while in the default (opted-out) repo they simply stay on `docket`. `BOARD.md` is the one artifact that never leaves `docket`, opt-in or not.

### `integration_branch` and GitFlow

The `integration_branch` knob says where code lands and where feature branches are cut from:

- `auto` (the default, and what an absent key resolves to) follows the remote's default branch via `origin/HEAD`. If `origin/HEAD` can't be resolved, the per-skill runtime resolver (`docket-config.sh`) fails closed with a diagnostic rather than guessing a branch. (Only the one-time `migrate-to-docket.sh` bootstrap falls back to `main` in that case — and that runs once, before a repo is even migrated.)
- `main` or `develop` is used verbatim.

This makes docket work for trunk-based (`main`) and **GitFlow** (`develop`) projects alike. One caveat: `auto` follows the repo's *default* branch, so a GitFlow repo whose default branch is `main` but whose integration line is `develop` must set `integration_branch: develop` explicitly. Feature branches always cut from `origin/<integration_branch>` — the one exception being a change declaring `stacked_on:`, which cuts from its parent's branch instead.

### The `.docket/` metadata worktree

Git checks out one branch per folder. To write a file that lives on `docket` while your main folder sits on `main` or a feature branch, a skill needs a second folder parked on `docket` — a **worktree**. Each skill, in docket-mode, ensures a persistent worktree at **`.docket/`**: a second checkout folder parked on the `docket` branch, synced to `origin/docket` before any read. **Your main working tree never switches branches.**

`.docket/` is **gitignored** (alongside `.worktrees/`). It deliberately lives at `.docket/`, not under `.worktrees/`, for three reasons:

- **No slug collision:** feature worktrees are `.worktrees/<slug>`, and a slug is arbitrary text from a change title — a change slugged `docket` would collide on `.worktrees/docket`. `.docket/` makes that impossible.
- **Lifecycle:** `.worktrees/` holds ephemeral per-change trees that get pruned; the metadata worktree is permanent, singular infrastructure.
- **Cleanup blast radius:** worktree-pruning logic (and humans) run over `.worktrees/`; keeping the metadata tree at `.docket/` puts it outside that blast radius.

### Finalize → selective publish

On a **terminal transition** — a change reaching `done` (PR merged) or `killed` (abandoned) — the driving skill archives that change on `docket`. A repo that opts in with `terminal_publish: true` (see below) *also* copies the change's terminal records onto the integration branch in one dedicated commit: the archived change file, its spec (if any), and the **`Accepted`** ADRs from its manifest, sourced from `origin/docket`. That copy is selective — a **file copy**, never a branch merge — so none of the planning churn comes with it, and the **live board stays on `docket`** and is never published. The result for a repo that opts in: your code history reads as code plus a clean trail of closed-out changes, while the working backlog churns entirely on `docket`.

### Publishing terminal records to the integration branch (`terminal_publish`, opt-in)

By default docket keeps **all** metadata on the `docket` branch. When a change reaches a terminal
state its record — the archived change file, its spec, and its `Accepted` ADRs — stays there, and
the integration branch accumulates **only** code, plans, and results, every one of them through a
pull request.

Opt in by setting `terminal_publish: true` in the repo's committed `.docket.yml`:

```yaml
terminal_publish: true   # ALSO copy closed change files, specs, and ADRs onto the integration branch
```

Each terminal transition then adds one direct commit to the integration branch carrying that
change's record, and `docket-adr` publishes `Accepted` ADRs the same way — so the code history
reads as code plus a clean trail of closed-out changes and decisions, browsable without switching
branches.

**Opt in deliberately — `true` writes to your code line.** It pushes machine commits **directly**
to the integration branch, bypassing PRs: that fights branch protection on a protected or PR-only
branch, and an autonomous agent's push can be denied mid-run by a permission classifier. A publish
that fails can also gap **silently** — the record simply never arrives, with nothing flagging its
absence. Leave the key unset unless direct commits on the integration branch genuinely suit your
workflow.

The knob gates both publish shapes: the change close-out *and* `docket-adr`'s ADR publish. It is
**per-repo-only** (a machine-scoped value is warned-and-ignored), because the headless
`docket-status` merge sweep must see the same policy as every other agent. It is inert in
`main`-mode, and it is never retroactive — it neither removes records already published nor
back-fills ones it skipped.

### `main`-mode: the single-branch opt-out

If you want everything on one branch — for example, a small repo, or a team that prefers all state in one place — pin both knobs:

```yaml
metadata_branch: main
integration_branch: main
```

This reproduces the original single-branch behavior **exactly**: no `docket` branch, no `.docket/` worktree, no terminal-publish copy. Planning commits land on the integration branch alongside your code, and the archive move there *is* the terminal record. Because docket-mode is now the default, an existing single-branch repo must pin `metadata_branch: main` to keep running as-is until it deliberately migrates — otherwise the bootstrap guard will stop and ask it to migrate.

### git-hook frameworks (pre-commit, husky, lefthook)

docket makes many small machine-generated bookkeeping commits (claims, board refreshes, status
writes, ADRs) on its metadata branch. Those commits **skip your repo's git hooks** by construction —
the `.docket` metadata worktree (and docket's transient publish/migration worktrees) have
`core.hooksPath` pointed at an empty directory, so a shared `pre-commit` hook never fires against
docket's own commits (which live on the orphan `docket` branch with no hook config anyway). Your
**code** commits on feature branches are untouched — the team's hooks still run on everything headed
to a PR. Nothing to configure; it is applied and self-heals on every docket run.

---

## Tuning agent models & effort

**Why pin a model per agent.** Most harnesses invite one mental model: *one session, one model.* You choose a tier when you start, and everything you do that hour runs at it. That is how you end up paying top-tier prices to regenerate a board — and thinking at the cheap tier while designing a build. Both are the same mistake, in opposite directions: the model was matched to the **session** instead of to the **task**.

docket's unit of work is the **skill**, so the tier is a property of the skill, not of your session. A `docket-status` sweep is mechanical file bookkeeping — the cheap tier. A `docket-implement-next` build and the autonomous design pass that precedes it both reason hardest about the change itself, so both run the top-tier model — but the build spends more effort doing it, because turning a settled design into working code demands more deliberation than drafting the design did. Recording the decision it reached (`docket-adr`) runs the same top-tier model at the lowest effort — same reasoning power, dialed down for a mechanical write-up. They run in the **same session**, minutes apart, each at its own model and effort, and you never pick one.

A single afternoon's loop spans all three: design and build the change at the top tier, record its ADR at that same tier but lighter effort, sweep the merged PR at the cheap tier. The `agents:` block below is how you *express* that; the generated wrapper is how it is *enforced*; and `context: fork` (Claude Code) and the generated dispatch rule (Cursor) are how the pin survives even a direct `/docket-status` invocation. Tune the tiers to your budget — docket's built-in defaults are a starting point, not a contract.

Each **autonomous** docket skill runs as a model/effort-pinned subagent (`docket-implement-next`, `docket-auto-groom`, `docket-finalize-change`, `docket-status`, `docket-adr`; the two interactive skills, `docket-new-change` and `docket-groom-next`, stay inline and only surface an advisory recommendation). To change the model or effort one of them runs at:

**1. Edit a config layer.** Up to three layers override the built-in default, resolved per field (precedence: repo-local > repo-committed > global > built-in):

- **Global** — the `agents:` block in `~/.config/docket/config.yml` (user-level; applies to every repo on your machine; the legacy `agents.yaml` is auto-migrated into it — see [Configuration](#configuration--docketyml-global-config-and-machine-local-overrides) above).
- **Repo-committed** — the `agents:` block in a repo's committed `.docket.yml` (applies to that repo for every clone and agent).
- **Repo-local** — the `agents:` block in that repo's `.docket.local.yml` (this machine only; wins over the committed value for this clone).

The config **shape** — the `agents:` keys and how the model and effort are written — is documented once in docket-convention's **"Agent layer"**; consult it there rather than copying field examples here, so the shape has a single source of truth and stays current as it evolves.

**`model: inherit` on Claude Code.** `inherit` is a Claude Code frontmatter value, not a docket keyword: it tells Claude Code to run the subagent on the **parent conversation's** model, which is a different outcome from setting no model at all (Claude Code's own subagent default). Docket passes it through to a `.claude` wrapper verbatim. Cursor, Codex, and opencode have no equivalent, so on those harnesses `inherit` is treated as "no pin" and the wrapper is generated without a model, letting the harness apply its default.

**Changing only the model?** To override an agent's model while *dropping* its pinned effort — e.g. pointing it at another harness's model, where Claude Code's effort tiers do not apply — set `effort: auto`, which drops the effort line entirely so the agent inherits the model default. Omitting the `effort:` key instead *keeps* the built-in effort, so `auto` is the explicit way to drop it.

**Finding model IDs.** A `model:` value is passed to the harness verbatim — docket never validates it — so use exactly the IDs your harness reports:

| Harness | List available model IDs |
|---|---|
| Claude Code | `ant models list` ([reference](https://platform.claude.com/docs/en/api/cli/models/list)) |
| Cursor | `cursor-agent models` |
| Codex | `codex debug models \| jq -r '.models[] \| .slug'` |
| opencode | `opencode models openrouter` |

**2. Refresh the generated wrappers.** The resolved model and effort are baked into generated wrapper *copies* (not symlinks), so after editing any layer, regenerate them:

```bash
bash sync-agents.sh        # or re-run install.sh, whose Go engine reconciles them for you
```

- A **global** edit rewrites user-level wrappers into every **present** harness root (`~/.<harness>/agents/`, e.g. `~/.claude/agents/`, `~/.cursor/agents/`, `~/.codex/agents/`).
- A **repo-committed or repo-local** edit rewrites that repo's per-repo wrappers for each harness in its (local-then-committed) `agent_harnesses:` list (default `[claude]`; e.g. `[claude, cursor]` for a repo that also drives Cursor).

`sync-agents.sh` always writes **both** passes in one run — user-level wrappers into each targeted harness root AND (for opted-in repos) per-repo wrappers — and project wins over global at generation time, per the four-layer precedence above.

**Generated per-repo agent files are machine-local — gitignored, never committed.** Unlike a repo's committed `.docket.yml`, `<repo>/.<harness>/agents/docket-*.md` (and, for Cursor, `docket-dispatch.mdc`) are regenerated on every machine from that machine's own resolved config; they carry no team intent of their own — the committed `agents:` block is the artifact that does. A single marker-bounded `# docket` block in the repo's `.gitignore` covers every docket-owned path — the `.docket/` worktree, `.worktrees/`, `.claude/settings.local.json`, `.docket.local.yml`, and every generated agent file for every harness — not just the generated-agents subset. It is seeded by `migrate-to-docket.sh` (fresh migration) or `docket-config.sh --bootstrap` (fresh orphan-branch bootstrap), and self-healed by `sync-agents.sh` — which creates or repairs it the moment a repo opts in — declares an `agents:` block or an `agent_harnesses:` key, in either file, or merely carries a `.docket.local.yml` — and prints a loud one-time notice to **commit it once**. After that the block is invisible plumbing.

**3. Guard drift in CI.** `sync-agents.sh --check` is a four-part gate:

- The `.gitignore` `# docket` block is present and current, **and** no per-repo generated file is tracked by git — both are **CI-meaningful** (`rc != 0` fails the build; the second leg also catches a repo whose migration commit never happened).
- A committed `.docket.yml` using the legacy bare-agent-key `agents:` shape (agent keys sitting directly under `agents:` instead of nested under `agents: default:`) also fails — **CI-meaningful** (`rc != 0`) — naming the offending keys and the reshape to `agents.default.<agent>` in its message.
- Generated content drifting from the resolved config is **advisory only** (`rc` unaffected) — every clone regenerates its own copy at build time, so a stale local file is a nudge to re-run `sync-agents.sh`, not a CI failure.

**Always the full set, plus a Cursor dispatch rule.** The per-repo layer writes the **full built-in agent set** for every harness in `agent_harnesses` (the `agents:` block only *overrides* model/effort — it never decides which agents exist). It is **opt-in**: a repo opts in by declaring an `agents:` block or an `agent_harnesses:` key, in **either** its committed `.docket.yml` or its local `.docket.local.yml`; a repo with neither key set in either file generates no per-repo wrappers and its `--check` stays a no-op. A repo listing `cursor` also gets a generated `.cursor/rules/docket-dispatch.mdc` that forces Cursor to dispatch docket agents instead of running them inline. `sync-agents.sh --check` covers both the generated agents and the dispatch rule. The model and effort each of those wrappers carries are resolved per field from your config layers over docket's shipped [`agents/harness-defaults.yml`](agents/harness-defaults.yml); a harness/agent pair with no entry in any layer is generated **unpinned**, so that harness applies its own default rather than inheriting another harness's model ID. For **Codex** — its `.codex/agents/*.toml` wrappers plus the committed `AGENTS.md` dispatch block, and why a *global* `agent_harnesses` does not generate them per-repo (the repo must opt in) — see [docs/codex/setup.md](docs/codex/setup.md) — both entry paths, a prose request routed by the committed `AGENTS.md` dispatch block and a direct `@docket-…` invocation, are supported and reach the identical registered wrapper; to validate the whole loop live, work through the [Codex live-validation runbook](docs/codex/validation-runbook.md). For **opencode** — its `.opencode/agents/*.md` definitions plus the same committed `AGENTS.md` dispatch block, which the two harnesses share — see [docs/opencode/setup.md](docs/opencode/setup.md).

**Two mechanisms for one inline quirk.** Both Cursor and Claude Code run a *directly-invoked* skill — a human typing `/docket-status`, or the model auto-invoking it — inline at the session model, which silently defeats the wrapper's model/effort pin. They fix it differently: Cursor uses the generated `docket-dispatch.mdc` rule above; **Claude Code uses native `context: fork` + `agent: docket-<name>` frontmatter** committed in each forked skill's `SKILL.md`, which forks the invocation into the same pinned wrapper. That frontmatter is inert in every other harness (unknown keys are ignored), so one shared `SKILL.md` serves all of them, and it degrades to today's inline behavior on a Claude Code too old to know the field. **Fork-exclusion principle:** only skills that never need the human mid-run are forked — a forked subagent has no channel to the human (Claude Code withholds `AskUserQuestion`, `EnterPlanMode`, and similar from subagents). So the four headless-safe autonomous skills — `docket-status`, `docket-adr`, `docket-implement-next`, `docket-auto-groom` — carry the frontmatter; the two interactive brainstorm skills (`docket-new-change`, `docket-groom-next`) and `docket-finalize-change` (which retains real prompts — the multi-candidate batch confirmation and repair sign-off — so a headless drive is authorized by [naming ids](#closing-out-hands-free-with-loop) instead) do not.

**The two invocation paths.** Both mechanisms above land a directly-invoked skill on the *same* pinned wrapper, so the model and effort it runs at are identical either way. What differs is what **you** see while it runs:

| Path | How | You get | You give up |
|---|---|---|---|
| **Skill-invoke** | `/docket-status`, or the model auto-invoking the skill | The pinned run, forked — cheapest, no dispatch turn | Observability: it returns as `completed (forked execution)`, with no box to drill into in the TUI |
| **Agent-dispatch** | `@docket-status`, or a subagent dispatch naming the wrapper | The **identical** pinned run, drillable live in the TUI | One dispatch turn of overhead |

Reach for **agent-dispatch when you want to watch a long run** — a build you intend to babysit — and **skill-invoke for everything else**. A forked run is not lost, only unobservable in the TUI: Claude Code still writes its full transcript to `~/.claude/projects/<project-slug>/<session-id>/subagents/agent-<id>.jsonl`. Treat that path as an **observed internal, not an interface** — it was accurate on Claude Code 2.1.207, it may move, and docket depends on it for nothing. Cursor users are always on the drillable path: the generated dispatch rule routes a direct invocation through a real `Task` dispatch.

**Restart your session after changing an agent or a skill.** Skills and agents are **registered at process start**. After you run `sync-agents.sh`, or edit a skill's frontmatter, an already-open session keeps running the *old* definitions — so a freshly-added fork appears to do nothing, and a healthy pin looks broken. Restart the harness process (a new session — clearing the context is not enough) and re-invoke.

**The clone-identical guarantee is retired.** Before this change, committing the generated per-repo files meant an autonomous change built on the exact same model on every clone, by construction. Generation is now all-local, so that guarantee is gone — a deliberate trade, not an oversight: never having to reconcile a machine-generated file in a PR diff, at the cost of no CI-enforced pinning of the generated copies. Team defaults for a repo still live in its committed `.docket.yml` `agents:` block, by convention.

The same `agents:` entries can also carry a `runner:` key, which delegates that agent's whole
run to a *different* harness (e.g. OpenAI Codex) with its own subscription and models — see
[Runner delegation](#runner-delegation--running-docket-agents-on-another-harness) under
Customization.

---

## Skills

The operating loop — create, groom, implement, finalize, report, decide — plus the shared contract every skill loads, and the pluggable role skills docket ships. Every directory under `skills/` appears below.

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
| `docket-brainstorm` | Pluggable `brainstorm` role, opt-in — keeps the design dialogue inline with you, then dispatches a pinned consultant once to author the spec or hand back critique concerns. See [Consultant-authored brainstorm](#consultant-authored-brainstorm-opt-in). |
| `docket-build` | Pluggable `build` role, opt-in — turns a written plan into commits by routing each task to a named economy/standard/premium profile agent, with one bounded escalation per task, no per-task review, and a single full-suite gate. See [docket-build](#docket-build--the-lean-profile-routed-build). |
| `docket-build-task` | The per-task worker contract `docket-build` preloads into its profile agents — one plan task from focused test through verification, self-review, and one commit. Not invoked directly by a human. |
| `docket-review` | Pluggable `review` role, opt-in — one bounded, read-only whole-branch reviewer behind pinned rung wrappers: reads the branch diff and the build-evidence record, returns severity-tiered findings, and never fixes, dispatches, or runs the test suite. |

---

## Learnings — the loop's memory

The repo gets smarter as changes ship. Every change that reaches `done` distills its close-out
lessons into a curated **finding** (zero is normal, and kills are never harvested).

- **Findings + a rendered index.** One file per lesson or consolidated family under
  `docs/changes/learnings/` on the metadata branch, plus a generated `README.md` index.
- **Pay per relevance.** Groom, plan, and review load the index — a small hint surface — and pull
  only the findings that bear on the change at hand, instead of paying for the whole history on
  every run.
- **Human-gated promotion.** A rule that must fire unprompted graduates into `AGENTS.md`/`CLAUDE.md`,
  where it is always in context; the finding then stops taxing the retrieval surface. docket
  proposes the candidate — it never edits your always-in-context file, and never auto-merges its
  own memory.
- **Controls.** `learnings.enabled` turns the subsystem off wholesale (a read/write gate, never a
  purge); `learnings.cap` sets the active-finding count past which docket flags "needs curation".

Mechanics — the finding schema and the promotion states — live in the `docket-convention` skill's
*Learnings ledger* section, which is their single source; the harvest procedure itself is
`docket-finalize-change`'s Step 2.5.

---

## Customization

Opt-in alternatives to docket's defaults. Each one is off until you turn it on, in one of the layers described in [Configuration](#configuration--docketyml-global-config-and-machine-local-overrides).

### Consultant-authored brainstorm (opt-in)

By default, the `brainstorm` role (the up-front design step in `docket-new-change` and `docket-groom-next`) runs `superpowers:brainstorming` — the dialogue and the spec are both produced inline, at the session model. **`docket-brainstorm` is an opt-in alternative** that keeps the design conversation exactly where it is — with you, inline, at whatever model the session runs — but adds a pinned, high-tier design **consultant** that authors (or audits) the final spec once the dialogue has settled the design. The consultant fires once, at the end, either handing back an authored spec or critique concerns that send you back to the dialogue.

Two ways to opt in:

- **Per-invocation (verbal).** Just say so when you run the interactive skill, e.g. `/docket-new-change "… have a consultant write the spec"`. This wins for that one run regardless of any configured default.
- **Durable (config).** Bind the role in `.docket.yml` (or global `config.yml`), and every brainstorm — `docket-new-change` and `docket-groom-next` alike — goes through the consultant:

  ```yaml
  skills:
    brainstorm: docket-brainstorm
  ```

If the consultant can't be dispatched on this machine (agents not synced, harness without dispatch support, or any other per-machine gap), `docket-brainstorm` **degrades to running the whole flow inline at the session model, with a prominent warning** — no worse than not having opted in.

**Capture-then-groom: an entire brainstorm at a chosen model.** The consultant pins *authorship*; the dialogue and option generation still run at whatever model the session is on. To pin the *whole* brainstorm, no new machinery is needed: capture the idea as a stub with `docket-new-change` in whichever session it strikes you (skip straight past brainstorming — the stub lands at `needs-brainstorm`), then run `docket-groom-next` from a session set to the model you want. That session does the full design conversation at its own model, and can still opt into consultant authorship on top.

### docket-build — the lean, profile-routed build

The `build` role — the step in `docket-implement-next` that turns a written plan into commits — runs **`docket-build`**, docket's own build engine and the shipped default since change 0193. It dispatches one fresh worker per task and does no review of its own, leaving `skills.review` as docket's sole review gate: `T` nested runs on the clean path — one worker per task — plus one full-suite gate the controller runs itself as a Bash command rather than as a nested agent, against the `2T + 2` of `superpowers:subagent-driven-development`, the per-task implementer/reviewer pair it replaced. Each escalation adds at most one more nested run.

Nothing to select — it is the default. To opt back out to the superpowers engine, set it in any config layer:

```yaml
skills:
  build: superpowers:subagent-driven-development
```

Because `docket-build` is now the default, a repo upgrading docket must re-run `install.sh` and start a fresh session: the installer is what generates the four profile agents and links the two build skills onto this machine, and every harness registers agent definitions only at process start — until the harness hosting your session has, the first build stops on the harness rejecting a dispatch to a profile agent it does not yet know about, and reports that condition (with re-running `install.sh` in a fresh session as the remedy) rather than improvising a substitute.

Each task is routed to one of four named profile agents (`docket-build-economy`, `docket-build-standard`, `docket-build-premium`, `docket-build-max`), sharing one worker contract and differing only in model and effort. The classifier is deliberately asymmetric: `economy` must be *positively* established (fully specified, pattern-following, no consequential risk, no cross-file reasoning), while genuine uncertainty defaults to `standard` rather than the cheaper tier. `max` is deliberately rare — reachable only for unresolved architecture or an irreversible data change, by an explicit plan override, or by escalation from `premium` — so the tier meant for extreme cases does not normalize. A plan task can override the classifier outright with a `**Build profile:** economy` line on that task; an invalid value halts the build rather than silently falling back. The full routing rubric and worker protocol are the [`docket-build`](skills/docket-build/SKILL.md) skill's, not restated here.

Each task carries at most one automatic escalation — an `economy` worker retries once at `standard`, a `standard` worker once at `premium`, a `premium` worker once at `max` — never a second climb; a `max` worker that still can't finish halts the build for a human. After every task has committed, `docket-build` runs the whole suite exactly once, resolved from `finalize.test_command` (no second, driftable test-command key of its own) — green hands off to `skills.review`, red becomes one synthetic integration-repair task on the `premium -> max -> halt` ladder.

`build.checkpoint` (default `false`) governs whether a run persists a resume ledger: `false` keeps only the per-task commits as the durable record, while `true` writes a compact state file recording each task's profile, escalation, and commit so a resumed run can skip work already proven complete.

`gate_observation_budget` (default `30`, minutes, settable in any layer) bounds that gate run. The gate does not assume the suite fits inside one foreground call: it executes durably, records its outcome where a later look can read it, and the agent establishes completion from that record rather than from the completion signal of whatever command started it — which is why a run may go quiet for a while and why a stale "still running" report is not evidence that it crashed. Observation is bounded by this budget, and exhausting it with no terminal result **fails closed**: the build halts for a human rather than inferring either success or a red suite, since an unfinished run is not a failing one. It is docket's own policy value, deliberately independent of whatever foreground-call timeout your harness imposes. What each harness must be able to do to host such a gate — and the measured verdict for each one docket ships — is in [`gate-execution.md`](skills/docket-build/references/gate-execution.md).

`delegation_observation_budget` (default `60`, minutes, settable in any layer) is its sibling for **runner delegation**: it bounds how long docket will await a terminal result from a delegated runner child it launched detached. Where `gate_observation_budget` bounds awaiting a suite run started by an agent, this one bounds awaiting a whole agent run. Expiry with no result also fails closed — and additionally **kills the detached process group**, so no unwatched agent keeps working on the repo after its run was declared failed. `0` is legal and means "observe once, then fail closed"; anything that is not a non-negative integer is a config error, not a silent fallback.

**`docket-build` ships validated model IDs for Claude Code, Cursor, Codex, and opencode.** Every shipped default lives in [`agents/harness-defaults.yml`](agents/harness-defaults.yml), indexed by harness, and all four are complete — seventeen agents each, the four build profiles, the three review rungs, and the internal plan-writer among them — so any of the four harnesses gets profile-routed builds with no configuration at all. Codex takes a real reasoning-effort token per agent, so its rows carry a model/effort pair where Cursor's IDs encode their variant and use `auto`; opencode's rows carry a pair too, emitted as `reasoningEffort:` and forwarded to the provider as a model option. A harness docket does not yet map generates **unpinned**, letting that harness apply its own default rather than inherit an ID that means nothing there. To retune any pair, set the model yourself in a config layer — `.docket.example.yml` mirrors all four shipped blocks value for value. On Claude, `build-economy` ships Sonnet rather than Haiku deliberately: the worker contract is long and strict, and its failure modes halt the build instead of escalating. If you want a more cost-aggressive floor, set `build-economy` to `claude-haiku-4-5-20251001` in a config layer.

**Tune plan writing on its own dial.** `docket-plan-writer` is the internal agent `docket-implement-next` dispatches at step 4 to author the change's plan artifact — give it a stronger (or cheaper) model than the orchestrator by setting `agents.<harness>.plan-writer` in any config layer; `skills.plan` still selects the *method* (the plan skill), while this pin selects the *model* that runs it.

### docket-review — the bounded whole-branch reviewer

The `review` role — the step in `docket-implement-next` that reads the finished branch before the PR opens — runs **`docket-review`**, the shipped default since change 0193: one read-only reviewer contract behind three pinned rung wrappers — `docket-review-lean`, `docket-review-standard`, `docket-review-deep` — that share the contract and differ only in model and effort. The reviewer reads `git diff`, `git log`, and the tree; it never writes, never commits, never checks out, never dispatches a subagent, and never runs the test suite. It gets one shot at the rung it was dispatched at: there is no reviewer escalation ladder.

Nothing to select — it is the default. To opt back out, set it in any config layer:

```yaml
skills:
  review: superpowers:requesting-code-review
```

The rung is chosen **deterministically as one above the build** — not by model judgment. `docket-implement-next` takes the highest profile any task routed or escalated to and maps `economy` to lean, `standard` to standard, and `premium` or `max` to deep; a whole-branch diff of more than 1500 changed lines bumps the rung one step, capped at deep. The reason a cheap build gets a cheap review is that the build's own routing already answered how hard the work was, and the diff-size bump is the one signal independent of that self-assessment.

Findings come back severity-tiered, and since change 0218 they are **fixed on the branch** rather than recorded and re-minted. `docket-implement-next` runs a bounded **fix loop** after review returns and before the PR opens: each finding becomes a task through the same `docket-build-task` contract that wrote the code, committed into the diff the human was going to read anyway, so the merge gate does not move. The reviewer itself is unchanged — it returns the finding list and a one-line verdict, nothing else, and never fixes anything.

Two axes, deliberately kept apart. A fix's **character** picks the model profile, using the same [routing rubric](skills/docket-build/references/task-routing.md) `docket-build` applies to plan tasks — so a subtle one-line fix is not handed to a cheap model just for being labelled minor. Severity picks only the **failure posture**: a blocker that cannot be fixed halts the run, while an important or minor that cannot be fixed falls back to a line in the PR body. Fix routing **never reaches the `max` profile** at any severity: `max` means irreversible, and an irreversible act must not happen to a branch as an unplanned side-quest discovered at review time.

The loop is bounded. One full-suite run after the fixes land; if it goes red, the non-blocker fix commits are reverted and the suite runs once more — green proceeds with those findings recorded unfixed, still-red halts. Two suite runs, no second repair chain, and no re-review round. The branch can never end worse than the green build that entered the loop. Every finding's outcome reaches the PR body as a **disposition table** — fixed (with its SHA), deferred, reverted, or recorded.

Two config knobs shape the loop, both settable in any layer. `review.min_fix_severity` (default `minor`) is the lowest severity that enters it: `important` records minors instead of fixing them, and `blocker` restores the pre-0218 behavior as a compat escape hatch. `review.max_fix_tasks` (default `10`) caps how many non-blocker fix **tasks** one run dispatches — the task, not the finding, so a batch of minors sharing a routed profile spends one slot; overflow findings are deferred in the disposition table with the cap named as the reason, and `0` is legal, meaning "fix nothing but blockers". **Blockers are fixed regardless of both** — neither knob can disarm the one gate that must not be disarmed, so blockers never count against the cap either. One consequence worth stating: a review finding about the branch's own diff is no longer auto-captured as a stub at all. It is fixed or it is recorded; only genuinely distinct, beyond-the-branch work still mints one.

**Why the full suite runs in the build gate and not inside the reviewer.** Four reasons, and they compound:

- The suite answers the *build's* question — "does what I assembled actually work together?" — while review asks a different one: "is this good?" Putting the first question inside the second confuses two jobs.
- The repair machinery already lives on the build side. A suite inside a reviewer forbidden to fix anything would have to hand its failures back out and re-enter build machinery, recreating exactly the build → review → build loop this design exists to kill.
- Gate-first ordering is cheaper on failure. A red suite discovered *after* an expensive whole-branch read has already wasted the read; discovered before, it costs one build task.
- The evidence chain then follows naturally, because the thing that last mutated the branch is the thing that certifies it.

The mental model: **the suite is the boundary between build and review**, owned by the side that can fix a failure. It works like CI status checks on a pull request — the check runs on the branch as it lands, and the reviewer is the human-style reader who reads the diff and trusts the green check rather than re-running it.

That boundary is made durable by a **build-evidence** record: on green, `docket-build`'s gate emits the command it ran, the result, the head SHA, and a timestamp; the reviewer verifies the record is present, green, and pinned to the exact HEAD it is reviewing, and returns an `unverified-build-state` blocker if it is missing, malformed, or stale — running the suite itself is never the remedy. `docket-implement-next` writes that record into the PR body inside `docket:build-evidence` markers, and `docket-finalize-change` reads it there: when the pre-merge rebase is a no-op and the evidence is green, the post-rebase suite run is skipped and the skip is logged. Two permits satisfy the SHA condition: the record's `head_sha` **equals** the branch HEAD being merged, or `head_sha` is a **strict ancestor** of that HEAD and every path changed in `head_sha..HEAD` lies under the repo's configured `<results_dir>/` — the tree Step 6.5 commits post-gate, after the evidence is minted. That second permit is armed per repo only when a live guard test asserts no suite component reads that tree as a content source; where it cannot be established, finalize falls back to the equality-only predicate. Anything else — a non-ancestor SHA, a rebase that moved commits, any changed path outside that prefix — still runs the suite. Two caveats on that skip. Any commit landing after the suite run that produced the record moves branch HEAD past the recorded `head_sha`, so the equality check fails; the ancestor permit is what keeps the skip reachable across that gap, but only for results-file commits — any real code delta, a fix commit no later run re-pinned the record onto, fails the predicate and finalize simply runs the suite. And the block lives in a PR body, which is human-editable input to a merge-gating decision — anyone who can edit the description can assert a green record at the current SHA, which is nil exposure for a single maintainer and worth weighing in a repo that takes outside contributions. The arithmetic that falls out is **one full-suite run on the clean path — nothing to fix, a no-op rebase, the record pinned to HEAD or separated from it only by post-gate results commits — and at most four**, from three sources: the build gate always runs it once; the fix loop above adds one run after the fixes land, and a second only when that run goes red and the non-blocker fixes are reverted; finalize adds one more unless all three skip conditions hold. A fifth is reachable only on the `unverified-build-state` path, where the implementer re-runs the suite itself to rebuild the missing or stale record before any fix dispatches — that baseline run precedes the loop and is deliberately outside its two-run bound, so charging it to the gate cannot eat the post-revert re-run. The previous shape could run it in the build, again inside the review loop, and again at finalize, on every path.

The binding posture used to be deliberately asymmetric — both roles shipped opt-in while this repository dogfooded them through its own committed `.docket.yml`. Change 0193 ended that: `docket-build` and `docket-review` are the shipped cross-harness defaults, `.docket.example.yml` states them as such, and this repository now carries no `skills:` block at all, so it runs exactly what everyone else does. As with `docket-build`, inheriting the role on upgrade means re-running `install.sh` and starting a fresh session, since the three rung wrappers are generated by the installer and every harness registers agent definitions only at process start.

### Runner delegation — running docket agents on another harness

Docket agents normally run on the harness hosting your session. **Runner delegation** hands an
agent's *whole run* to a child harness with its own subscription, models, and skills — activated
per agent by an explicit `runner:` key, never inferred from model IDs. Three pairs ship today, all
with parent `claude` (Claude Code): children `codex` (OpenAI Codex CLI), `cursor` (Cursor CLI), and
`opencode`.

```yaml
# .docket.yml (or the global ~/.config/docket/config.yml — runner is a machine preference)
agents:
  claude:                       # the PARENT harness: when Claude Code hosts the session…
    status: { model: gpt-5.1-codex, effort: medium, runner: codex }   # …run docket-status on Codex
    adr:    { model: gpt-5.1, effort: high, runner: cursor }          # …or run docket-adr on Cursor
    # Delegate the four build workers to cheap OpenRouter models; leave review native.
    build-economy:  { runner: opencode, model: openrouter/deepseek/deepseek-v4-flash-0731, effort: medium }
    build-standard: { runner: opencode, model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
    build-premium:  { runner: opencode, model: openrouter/moonshotai/kimi-k3,              effort: medium }
    build-max:      { runner: opencode, model: openrouter/moonshotai/kimi-k3,              effort: high }
    # review-lean / review-standard / review-deep: no runner: → native Claude Code
runners:
  codex:
    sandbox: workspace-write    # workspace-write (default) | danger-full-access
    network: true               # default true — git push and gh need it
  opencode:
    permissions: auto-approve   # ask (default) REFUSES to delegate — see below
```

The OpenRouter IDs above are illustrative, not validated: docket keeps no vendor allowlist
(ADR-0015), so **no test in this repo can catch a wrong or unentitled ID**. Confirm them against
`opencode models openrouter` under your own credentials before relying on them — catalog presence is
not entitlement. Full verification steps: [docs/opencode/setup.md](docs/opencode/setup.md).

A delegated run is anchored at the repo's **main worktree** by default (ADR-0034, cwd-independent
by design) — correct for the metadata-scoped `status`/`adr` agents. A build worker must instead
stay in the feature worktree on its branch, so a delegated build worker runs in the tree named by
`--worktree`. That requirement is keyed on a **declared** fact, not a name shape: every built-in
agent source carries `worktree-scope: feature` or `worktree-scope: metadata`, the generated shims
bake the `--worktree` slot for the feature-scoped ones, and the dispatch facade refuses a
feature-scoped delegation that names no worktree (or that names the main worktree).

How it works: `sync-agents.sh` generates that agent's wrapper with a **shim body** — a
**launch-then-observe** instruction over a single dispatch seam, `docket.sh runner-dispatch`. The
launch call resolves the `runners.codex` knobs, starts `codex exec` (sandboxed, final-message relay
via `--output-last-message`) **detached**, and returns a dispatch key immediately; the wrapper then
makes bounded, short observe calls against that key until the run reports a terminal result. Two
invocations, still exactly one dispatch seam — no delegated run is bounded by one foreground
call's ceiling. Every
invocation path (skill fork, `@docket-status`, composition from another skill) inherits the
delegation unchanged. `model:` is passed to the child verbatim (ADR-0015); `effort:` maps to
Codex's `model_reasoning_effort` (docket's `max` → codex `xhigh`). The wrapper's own frontmatter
pin is a separate value — see the shim-pin rule below.

Rules and limits:

- **Only autonomous wrappers are delegatable** (the seventeen generated agents). Interactive skills
  stay inline — an exec primitive has no human channel.
- A delegated *orchestrator*'s own sub-dispatches run child-natively (for Codex:
  `spawn_agent`, via superpowers' Codex support). Per-agent model pins do **not** carry into
  those child-side dispatches (accepted limitation).
- `runner:` under a non-`claude` harness key is reserved and warned-and-ignored; an
  unregistered runner name fails generation loudly.
- **Generation is all-or-nothing.** Any bad `runner:` entry is detected before the first wrapper is
  written, and the run refuses rather than regenerating some wrappers and not others: one bad entry
  refreshes *zero* wrappers, and previously generated ones survive untouched. The diagnostic names
  every offender in one pass, so the fix is a single edit and a re-run. `sync-agents.sh --check`
  reports the same failure without writing anything.
- Delegation is never a policy bypass: do not delegate `docket-finalize-change` to sidestep
  merge-approval gates (see ADR-0043).
- **The wrapper's own `model:`/`effort:` are not the child's.** A generated shim is two agents in
  one file: a relay that Claude Code runs, and the child run it dispatches. The frontmatter pins the
  relay and must therefore name a model *Claude Code* can resolve — it defaults to `inherit` (the
  parent conversation's model) and is retuned per runner with `runners.<name>.shim_model` /
  `shim_effort`. The child's identity travels in the baked `--model` / `--effort` arguments, from
  the `model:` you set under `agents:`. Setting `shim_model` to a cheap model is a pure cost
  optimization: the relay only blocks on the child and passes its output back.
- **A delegated agent must carry an explicit `model:` in your config.** Docket never forwards its
  own shipped default to another harness — that ID means nothing to the child — so a model-less
  `runner:` is a loud generation-time error rather than a silent run on the child's own default,
  which on a pay-per-token backend surfaces on the bill instead of in the run (ADR-0067).
- **`effort:` is optional — and omitting it is not the same as opting out.** It follows the same
  never-forward-a-shipped-default rule, but with no matching error. Omitting the key means "no
  opinion" and still resolves from lower **user** config layers, whose value *is* forwarded;
  `effort: auto` is the explicit no-pin that suppresses the flag. With no flag baked, the child uses
  its own default for the chosen model — the parent's effort is never inherited.
- **Delegate leaves, not orchestrators.** A delegated run's own sub-dispatches run child-natively,
  so delegating `docket-implement-next` drags its review dispatch into the child too. Delegating
  the four `build-*` profile workers rather than the `docket-build` controller is the same rule:
  delegating the controller would move the routing decision into the child as well.

**Prerequisites (codex):** Codex CLI installed and authenticated (`codex login`); superpowers
installed in Codex; docket skills linked (`link-skills.sh`, automatic on install); and
`[features] multi_agent = true` in `~/.codex/config.toml` if you delegate an orchestrator
(SDD fan-out) rather than a leaf agent. Full adapter contract: `scripts/runners/codex.md`.

**Prerequisites (cursor):** `cursor-agent` installed and authenticated, and docket skills linked
into `~/.cursor/skills` (`link-skills.sh`, automatic on install). `cursor` delegates to
`cursor-agent -p`. Note that `cursor-agent` is unreliable and lags the Cursor IDE in features, so
the adapter's posture is a loud abort-and-report on any failure — it never falls back to running the
agent inline. Cursor has no effort flag: `effort:` is encoded inside the model value as
`--model <id>[effort=<e>]`, and with no model resolved it is dropped with a WARN. Full adapter
contract: `scripts/runners/cursor.md`. Validating the Cursor harness end to end (the CLI probe and
the human IDE checklist): [docs/cursor/validation.md](docs/cursor/validation.md).

**Prerequisites (opencode):** `opencode` installed (verified against 1.18.11) with a provider
authenticated (`opencode auth login`), and docket skills linked into `~/.agents/skills`
(`link-skills.sh`, automatic on install). `opencode` delegates to `opencode run`. **You must set
`runners.opencode.permissions: auto-approve`** — opencode prompts for approval before editing a
file or running a command, a delegated run has nothing to answer with, and the adapter therefore
refuses up front rather than hanging. That grant is deliberately a visible line in config, not
something you get by typing `runner: opencode`; pair it with opencode's own deny rules (whose
containing behavior docket has documented from its help text but not tested — verify before relying
on it). `effort:`
maps to `--variant` and passes through unmapped, including docket's `max` (unlike codex, where `max`
becomes `xhigh`); the shared omit-vs-`auto` rule above applies unchanged. Full adapter contract:
`scripts/runners/opencode.md`; the config recipe is in
[docs/opencode/setup.md](docs/opencode/setup.md).

### Running under Cursor Auto-run

Cursor users running the skills under Auto-run in Sandbox: see
[docs/cursor/permissions.md](docs/cursor/permissions.md) for the copyable `permissions.json` and
`sandbox.json` fragments, the trust tiers, what one allowlist entry authorizes, and troubleshooting.

### Hands-off finalize — what blocks it, and the recipe that works

**The Claude Code auto-mode classifier.** In interactive auto-mode, Claude Code's permission
classifier *soft-denies* capability-granting and merge-adjacent `gh` actions — notably
`gh workflow run`, and `gh pr merge` on an unreviewed PR (occasionally even a post-merge
`gh pr view`). A soft-deny is a model-side judgment, not a permission lookup: for the `gh`
actions named above, a `permissions.allow` entry **cannot** clear it — a claim scoped to those
actions as observed here, not a general property of every allow-rule. The behavior is also
scoped to the harness **mode** and **version** it was observed in — headless and interactive
diverge, on the same repo, on the same day — so treat any statement about it as an observation
with an expiry date, not a fact.

This is precisely why docket's earlier bot-approval design (change 0062, ADR-0042) failed: its
very first step was a `gh workflow run` dispatch, which is exactly what gets denied. That
subsystem is retired — see ADR-0043.

**Single-maintainer hands-off finalize (the recipe).** Configure branch protection on the
integration branch to **require a pull request** but require **zero** approvals
(`required_approving_review_count: 0`; leave `enforce_admins` off). A solo maintainer cannot
approve their own PR, so a nonzero requirement is structurally unsatisfiable — but with zero
required approvals, `docket-finalize-change` runs its rebase-retest gate and then merges via a
plain `gh pr merge --rebase`: **no `--admin`, no bot, and nothing for the classifier to deny.**
Changing the real state of the external system beats arguing with the guard.

**Repos that require approvals (human sign-off preserved).** With
`required_approving_review_count >= 1`, a human approves the PR on GitHub — a co-maintainer, or
the maintainer running finalize when they are an eligible reviewer. That makes
`reviewDecision: APPROVED` satisfy both branch protection and `require_pr_approval: true`, and
finalize merges with **no `--admin`**. The attended, explicit-id `--admin` path remains the
escape hatch when a sole maintainer deliberately forces past an unsatisfiable required review.

---

## Status

**docket-mode is the supported default.** Planning metadata lives on the orphan `docket` branch via the `.docket/` worktree; terminal records stay there too unless the repo opts in to publishing them onto the integration branch; trunk-based and GitFlow layouts are both supported. Existing single-branch repos move over with `migrate-to-docket.sh`, and the bootstrap guard refuses to run against an un-migrated repo rather than touching your data.

`main`-mode remains a simple, fully-supported opt-out: pin `metadata_branch: main` (and `integration_branch: main`) to keep everything on one branch with exactly the original single-branch behavior.

---

## Migration

Two one-time migrations, each relevant only when you bring an *existing* repo onto docket or carry one forward from an older docket layout. A brand-new repo needs neither.

### Migrating an existing repo to docket-mode

A repo that has been running in single-branch mode (everything on `main`) moves to docket-mode with a one-shot, idempotent script: **`migrate-to-docket.sh`** (it ships in this docket repo, alongside `link-skills.sh` and `sync-agents.sh`). The script operates on the git repo containing your **current directory** — so run it *from within the repo you want to migrate*, pointing at the script wherever docket is checked out:

```bash
cd <target-repo>
bash /path/to/docket/migrate-to-docket.sh
```

It prints the resolved target repo and prompts for confirmation before changing anything; pass `--yes` (or `-y`) to skip the prompt in automation. It then creates the orphan `docket` branch seeded from your current planning directories, prunes the live planning surface (`active/` changes, the changes `README.md`, `BOARD.md`) off the integration branch while keeping terminal records and build artifacts there, and adds `.docket/` + `.worktrees/` to `.gitignore`. Re-running it converges from any partial state.

Migration also grants one **local, per-repo** Claude Code permission: an allow-rule for a push to the integration branch (written to `.claude/settings.local.json`, which migration adds to `.gitignore`). This allow-rule is **historical compatibility** — it pre-authorized the terminal-record publish push the permission classifier once guarded on close-out, but **terminal publication is deferred from Go v1 — `docket finalize closeout` is the complete automated closeout boundary**, so `terminal_publish: true` no longer exercises it. The rule stays granted (narrowly, and only in this repo — force-pushes and pushes to other branches stay guarded) as a harmless remnant. Because `settings.local.json` is gitignored and per-user, anyone who later **clones** an already-migrated repo can grant themselves the same rule by running the helper standalone:

```bash
bash /path/to/docket/scripts/ensure-claude-settings.sh
```

The skills will **not** migrate a repo for you. On first run against an un-migrated repo (metadata still on the integration branch, no `docket` branch yet), a **bootstrap guard** STOPs and points you at `migrate-to-docket.sh` rather than silently moving your data. The same guard detects a half-finished migration and points back to the script to complete it.

### Migrating a pre-0051 repo

Repos that predate change 0051 (change 0048 committed the per-repo agent files directly) get a one-time, automatic migration on the next `sync-agents.sh` run: it deletes the stale tracked copies from the working tree, writes the `.gitignore` block, regenerates the local set fresh, and prints the single remedy commit to run — `git rm -r --cached '.claude/agents/docket-*.md' … && git add .gitignore && git commit -m "…"` — so the repo converges in one commit per clone.
