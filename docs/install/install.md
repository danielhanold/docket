# Installing docket

This page gets docket installed on your machine. docket installs once per machine and then works
in every repo you use it from, under whichever agent tool you use — a **harness** is the tool that
runs the agent: Claude Code, Cursor, Codex, or opencode. Product names appear freely across this
section because harness setup is exactly what it is about; the guide keeps them out of the way.

## What you need first

- **A harness.** docket's skills — a **skill** is a named, reusable instruction set an agent loads
  for one job — run inside a harness that has its own on-disk `skills/` and `agents/` directories
  for docket to write into. docket supports four: **Claude Code, Cursor, Codex, and opencode**.
- **`git` and the GitHub CLI (`gh`).** Every docket operation is a git operation, and the
  implementer opens pull requests with `gh`.
- **GNU coreutils `timeout`.** Build workers run each focused test under
  `timeout --kill-after=10s 10m`. It is standard on Linux; on macOS, run `brew install coreutils`
  (it may install as `gtimeout`, which workers also accept).
- **A GitHub remote** for the pull-request flow. docket pushes branches and opens PRs against your
  `origin`.
- **The superpowers plugin — recommended, not required.** Three of docket's five workflow steps
  default to superpowers skills (brainstorm, plan, and finish); docket's own skills carry build and
  review. Installing superpowers is your responsibility; docket neither bundles nor fetches it. If
  it is absent, each of those steps **runs inline at the agent's own model, with a prominent
  warning**, so docket still works out of the box with zero config. The full table is
  [Default workflow roles](../reference/skills-and-agents.md#default-workflow-roles).

## Install docket on your machine

Place the docket repo at `~/dev/docket` (the source of truth the symlinks point back to), then run:

```bash
bash ~/dev/docket/install.sh
```

That is the whole install. `install.sh` is a thin bootstrapper: it resolves this checkout and hands
the install to `docket development install --source <checkout>`, which does the real work as **one
journaled, all-or-nothing transaction** and is idempotent — re-run it any time (after adding a
harness, after editing a config file, and after every version update). A single run:

- **Builds a fresh binary and hands the install to that binary**, so the version that plans and
  writes your machine is the one you are installing — never the older binary that happened to be
  running. The recursion-guarded dispatch wrappers therefore land on the **first** run, not the
  second.
- **Links each present harness's global `skills/`** back to `~/dev/docket/skills/<name>` (symlinks,
  so editing a skill in the repo takes effect everywhere at once) and **reconciles that harness's
  global agent wrappers** — the model/effort-pinned subagent copies, resolved from your global
  config over docket's built-in defaults. Wrappers are user-level only; no repository carries its
  own copies.
- **Retires the old global parent-facing dispatch blocks** that earlier docket versions wrote into
  your personal `~/.claude/CLAUDE.md` and the other harnesses' global instruction files, while
  keeping the global skills and agent wrappers. Removal is **proof-gated** — the engine deletes a
  block only while it still matches docket's exact ownership marker, byte for byte. There is **no
  `--force`**: a block you edited, or one that no longer matches, is left untouched and the run
  reports it so you can remedy it and re-run.
- **Reconciles the current repository's parent-facing dispatch surfaces**, but only when that
  repository declares `agent_harnesses` in its `.docket.yml` or `.docket.local.yml`. Without that
  declaration the install touches no repository surface. See [Repo config](config-layers.md).

`install.sh` forwards two flags: **`--bin-dir <dir>`** chooses where the built binary is installed
(default `XDG_BIN_HOME` or `~/.local/bin`), and a repeatable **`--harness <name>`** limits the run
to the named harness(es) instead of every harness present on your machine. Run
`docket development install` directly when you also need **`--repo-dir <dir>`** to reconcile a
repository other than the one containing your current directory.

The install writes no configuration file. docket's built-in defaults already apply, so a
Claude-Code-only user can stop here. To pin a model or change a default on this machine, see
[Global config](global-config.md); to enable a harness for a repository, add it to
`agent_harnesses` in that repository's `.docket.yml` (see [Repo config](config-layers.md)).

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
not in the docket repo itself. Adopting docket in a repository is a separate step from this machine
install, run from inside that repository: a repository that has never used docket runs
`docket repository init`, and a repository still on the legacy single-branch layout runs
`docket repository migrate`. See [Where the metadata lives](../guide/where-the-metadata-lives.md). If the repository already has
a docket branch from the Bash version of docket, follow
[Upgrading from Bash docket](../release/upgrading-from-bash.md) instead.
