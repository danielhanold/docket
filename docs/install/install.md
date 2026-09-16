# Installing docket

This page gets docket installed on your machine. docket installs once per machine and then works
in every repo you use it from, under whichever agent tool you use — a **harness** is the tool that
runs the agent: Claude Code, Cursor, Codex, or opencode. Product names appear freely across this
section because harness setup is exactly what it is about; the guide keeps them out of the way.

## What you need first

- **A harness.** docket's skills — a **skill** is a named, reusable instruction set an agent loads
  for one job — run inside a harness that has its own on-disk `skills/` and `agents/` directories
  for docket to write into. **Claude Code, Cursor, Codex, and opencode** are first-class; docket
  also writes into `.agents/`, `.kiro/`, and `.windsurf/` harness roots when they are present.
- **`git` and the GitHub CLI (`gh`).** Every docket operation is a git operation, and the
  implementer opens pull requests with `gh`.
- **A GitHub remote** for the pull-request flow. docket pushes branches and opens PRs against your
  `origin`.
- **The superpowers plugin — recommended, not required.** superpowers is docket's default execution
  engine (brainstorm, plan, build, review, finish). Installing it is your responsibility; docket
  neither bundles nor fetches it. If it is absent, each workflow step **degrades to running inline
  at the agent's own model, with a prominent warning** — so docket still works out of the box with
  zero config, just without superpowers' structured execution. See
  [Workflow roles](workflow-roles.md) to rebind any step.

## Install docket on your machine

Place the docket repo at `~/dev/docket` (the source of truth the symlinks point back to), then run:

```bash
bash ~/dev/docket/install.sh
```

That is the whole install. `install.sh` is a thin bootstrapper: it resolves this checkout and hands
the install to docket's Go engine (`docket development install`), which does the real work as **one
journaled, all-or-nothing transaction** and is idempotent — re-run it any time (after adding a
harness, after editing `~/.config/docket/config.yml`, and after every version update). A single run:

- **Builds a fresh binary and hands the install to that binary**, so the version that plans and
  writes your machine is the one you are installing — never the older binary that happened to be
  running. The recursion-guarded dispatch wrappers therefore land on the **first** run, not the
  second.
- **Links each present harness's global `skills/`** back to `~/dev/docket/skills/<name>` (symlinks,
  so editing a skill in the repo takes effect everywhere at once) and **reconciles that harness's
  global agent wrappers** — the model/effort-pinned subagent copies, resolved from your config
  layers over docket's shipped defaults. It also points `~/.config/docket/config.yml` at
  [`.docket.example.yml`](../../.docket.example.yml), docket's canonical reference for every key and
  its default.
- **Retires the old global parent-facing dispatch blocks** that earlier docket versions wrote into
  your personal `~/.claude/CLAUDE.md` and the other harnesses' global instruction files, while
  keeping the global skills and agent wrappers. Removal is **proof-gated** — the engine deletes a
  block only while it still matches docket's exact ownership marker, byte for byte. There is **no
  `--force`**: a block you edited, or one that no longer matches, is left untouched and the run
  reports it so you can remedy it and re-run.
- **Reconciles each repository's parent-facing dispatch surfaces** from that repository's *explicit*
  `agent_harnesses` opt-in (see [Global config](global-config.md)) — automatic and Go-owned, with no
  separate synchronization script to run.

Two flags scope a run: **`--repo-dir <path>`** targets a repository other than the one containing
your current directory, and a repeatable **`--harness <name>`** limits the run to the named
harness(es) instead of every harness present on your machine.

The installer also writes a minimal `~/.config/docket/config.yml` the first time it runs. docket's
ordinary defaults already apply, so a Claude-Code-only user can stop here; to enable another harness
or change a default, continue with [Global config](global-config.md).

> **Stale project-level Claude wrappers shadow the guard.** docket installs agent wrappers
> **machine-globally** (under `~/.claude/agents/`), never inside a repository. If a repo still
> carries its own `.claude/agents/docket-*.md` copies — as docket versions before the recursion
> guard left behind — Claude Code loads *those* project-level wrappers in preference to the guarded
> global ones, which re-enables recursive self-dispatch. Delete those project-level copies; docket
> will not touch them, because it never owned them.

> **Start a fresh harness process after any install that changed a wrapper or a parent surface.**
> Harnesses register their agents and read their instruction files **at process start**, so a
> changed dispatch wrapper or a retired dispatch block only takes effect in a newly started process
> — **clearing a conversation is not enough**.

## Uninstalling docket

`docket uninstall` removes the harness integrations docket recorded for you — the
global `skills/` symlinks and `agents/` wrappers it wrote — and nothing else. With
no flag it removes **every recorded harness**; a repeatable **`--harness <name>`**
limits the run to the harness(es) you name, and duplicate `--harness` values are
de-duplicated before anything is touched. **`--dry-run`** reports exactly what a
real run would remove and writes nothing.

Uninstall is **ownership-safe**. It removes a file only while that file still
matches the exact bytes docket recorded when it installed it, and a managed
dispatch block only while the block's interior still matches docket's ownership
marker. A file you edited, a block that drifted, or an integration docket never
owned is left untouched and reported so you can reconcile it and re-run — there is
**no `--force`**. Re-running after everything is already gone is a no-op, and
uninstalling a harness that was never recorded changes nothing.

Uninstall is deliberately narrow. After it finishes the CLI binary, your global
configuration, contributor checkouts, and each repository's docket setup all
remain in place — uninstall retires harness integrations, not docket itself.
Removing the last recorded harness leaves a valid *empty* installation on record:
`docket install check` then reports that docket is no longer fully installed, and
a later `docket install` repopulates the harnesses from that same empty state.

## Reclaiming old version trees

docket keeps each installed asset set as an immutable tree under its data
directory (`versions/<asset-set-id>/`), and an install or an uninstall can leave a
tree that nothing references any more. docket reclaims those trees for you: after
any **successful or no-op** `docket install`, `docket development install`, or
`docket uninstall`, it runs a best-effort collection pass that deletes only the
version trees it can prove are both **unreferenced** and **structurally verified**.
Each tree is quarantined before deletion, so an interruption never leaves a
half-deleted tree — a later run finishes the pending cleanup where it left off.

The collection report classifies every tree it examined as `collected`,
`referenced` (still in use, so kept), `unverified` (could not be proven, so kept),
or `failed` (an error while collecting, so kept). Anything docket cannot positively
prove is **retained**, never deleted. Do not reach into `versions/` yourself: the
layout is private, its trees are not a supported reference to point anything at,
and moving or editing one is exactly what turns a tree unverifiable.

A cleanup warning never undoes the install or uninstall it followed: the primary
operation is already recorded as successful, and only the version reclamation is
left pending. When automatic collection cannot finish, docket surfaces a
`collection-pending` warning that lists the paths and the exact retry command,
`docket install collect` — the only time you run collect by hand. Running `docket
install collect` (add `--dry-run` to preview it) completes the pass; unlike the
automatic sweep, an explicit collect exits non-zero while any tree is still
`unverified` or `failed`, because finishing the reclamation is that command's whole
job.

## Adopting docket in a repository

The change data — `docs/changes/`, `docs/adrs/`, `docs/results/` — lives in each consuming project,
not in the docket repo itself. To adopt docket in an *existing* repo, run `docket repository
migrate` from inside that repo — a separate step from this machine install (see
[Where the metadata lives](../guide/where-the-metadata-lives.md)).
