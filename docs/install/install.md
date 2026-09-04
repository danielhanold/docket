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

## Adopting docket in a repository

The change data — `docs/changes/`, `docs/adrs/`, `docs/results/` — lives in each consuming project,
not in the docket repo itself. To adopt docket in an *existing* repo, run `docket repository
migrate` from inside that repo — a separate step from this machine install (see
[Where the metadata lives](../guide/where-the-metadata-lives.md)).
