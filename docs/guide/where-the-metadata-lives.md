# Metadata branch: Where the metadata lives

By the end of this page you will know where docket keeps its planning records and why they sit
apart from your code: the two branches it uses, which record lives on which one, how your code
checkout stays put while a second checkout handles the backlog, and how to opt a repository in to
copying closed-out records onto your code branch — or opt out of the two-branch layout entirely.
You will also know the one-time commands that bring an existing repository onto docket.

docket needs a durable, queryable record of planning state — changes, their statuses, decisions,
dependencies, and the **board** (the generated overview of every change and its state, never edited
by hand) — shared across agents, machines, and time, using **git as the only
storage** (no database, no service). The default way it stores that state is a two-branch layout,
and it is the supported default. The reasoning behind splitting metadata from code — the "why"
under everything on this page — is [Two branches and the metadata worktree](../concepts/two-branches.md).

## The two-branch model

Two branches divide the work, and neither touches the other's history:

- An orphan **metadata branch** (the `docket` git branch where the backlog, specs, and decisions
  are stored, separate from the code) is the authoritative surface for **all** planning
  records: every **change** (one unit of planned work, roughly one pull request, tracked as one
  markdown file), the board, every **spec** (the design document a change links to, written
  before building), and
  every **ADR** (an architecture decision record: one file per decision, immutable once accepted).
  It is a true orphan — it shares no history with your code and carries no code, the same pattern a
  `gh-pages` branch uses — and it is **always pushed**, so the whole backlog is browsable and
  reviewable on the remote at all times. Every bit of planning churn lands here and never touches
  your code history.
- Your **integration branch** — the branch code lands on, usually `main` (or `develop` under
  GitFlow) — stays code-only. It holds your code, the build artifacts that arrive with each pull
  request (the plan and results files), and, only in a repository that opts in, a copy of the
  closed-out records once a change finishes.

A change's feature branch is always cut from the integration branch — unless the change is a
**stacked change** (a change built on another change's unmerged branch rather than on the
integration branch), in which case it is cut from that parent's branch and targets the same. Either
way, the feature branch carries only plan, results, and code, and never modifies planning records.

## Where each artifact lives

Each kind of record has one home, and reaches your code branch (if at all) by one specific path:

| Record | Lives on | How it reaches the integration branch |
|---|---|---|
| Change file (manifest + body) | metadata branch | terminal-publish copy, only if opted in |
| Spec | metadata branch | terminal-publish copy, only if opted in |
| ADR | metadata branch | terminal-publish copy, gated on `Accepted`, only if opted in |
| Board | metadata branch | **never** — it is the live planning view |
| Plan | feature branch | the pull-request merge |
| Results | feature branch | the pull-request merge |
| Code | feature branch | the pull-request merge |
| `.docket.yml` | the repo's default branch | already there — read from the default branch |

The split to notice: plan, results, and code are **build artifacts** that live on the feature
branch and ride onto the integration branch through the ordinary pull-request merge, opt-in or not.
The change file, spec, and `Accepted` ADRs live on the metadata branch; a repository that opts in
**copies** them across on close-out (a file copy, never a branch merge, so no planning churn comes
with them), while a repository that does not simply leaves them on the metadata branch. The board
is the one record that never leaves the metadata branch, either way.

## `integration_branch` and GitFlow

The `integration_branch` key says where code lands and where feature branches are cut from:

- `auto` (the default, and what an absent key resolves to) follows the remote's default branch via
  `origin/HEAD`. If that cannot be resolved, the resolver fails closed with a diagnostic rather
  than guessing.
- `main` or `develop` is used verbatim.

That is what lets docket serve trunk-based (`main`) and **GitFlow** (`develop`) projects alike. One
caveat: `auto` follows the repo's *default* branch, so a GitFlow repository whose default branch is
`main` but whose real integration line is `develop` must set `integration_branch: develop`
explicitly. `integration_branch` is a shared coordination key — set it only in the committed
`.docket.yml`, per [Repo config](../install/config-layers.md).

## The `.docket/` metadata worktree

Git checks out one branch per folder, so to write a file that belongs on the metadata branch while
your main folder sits on `main` or a feature branch, a skill needs a second folder parked on the
metadata branch — a **metadata worktree** (a second checkout of the repo at `.docket/`, parked on
the metadata branch, so backlog edits never touch your code checkout). Each skill ensures a
persistent worktree at `.docket/` and syncs it to the remote before any read. **Your main working
tree never switches branches.**

`.docket/` is gitignored (alongside `.worktrees/`, which holds per-change feature worktrees), and
it deliberately sits at `.docket/` rather than under `.worktrees/` for three reasons: a change
could be titled "docket" and collide on `.worktrees/docket`; the metadata worktree is permanent
infrastructure while `.worktrees/` entries are ephemeral and get pruned; and keeping it out of
`.worktrees/` puts it outside the blast radius of any worktree-pruning cleanup.

## Publishing terminal records to your code branch (`terminal_publish`, opt-in)

By default docket keeps **all** records on the metadata branch, and your integration branch
accumulates only code, plans, and results — every one of them through a pull request. When a change
reaches a terminal state (its pull request merged, or the change abandoned), its record stays on
the metadata branch. The mechanics of that close-out copy — what gets copied and why the live board
never does — are [Landing changes safely](./landing-changes.md#selective-publish-on-close-out).

Setting `terminal_publish: true` in the repository's committed `.docket.yml` opts in: each terminal
transition then also adds one direct commit to the integration branch carrying that change's record
— the archived change file, its spec, and its `Accepted` ADRs — and ADR writes publish `Accepted`
decisions the same way. Your code history then reads as code plus a clean, browsable trail of
closed-out changes and decisions.

Opt in **deliberately**, because `true` writes to your code line:

- It pushes machine commits **directly** to the integration branch, bypassing pull requests. On a
  protected or PR-only branch that fights branch protection, and an autonomous agent's direct push
  can be denied mid-run by a permission classifier.
- A failed publish can gap **silently** — the record simply never arrives, with nothing flagging
  its absence.

Leave the key unset unless direct commits on your integration branch genuinely suit your workflow.
It is a shared coordination key (a machine-scoped value is warned-and-ignored), because the
headless merge sweep must see the same policy as every other agent. It is inert in single-branch
mode (below), and it is never retroactive — it neither removes records already published nor
back-fills ones it skipped.

## Single-branch mode: the opt-out

If you want everything on one branch — a small repo, or a team that prefers all state in one place
— pin both branch keys in the committed `.docket.yml`:

```yaml
metadata_branch: main
integration_branch: main
```

This reproduces the original single-branch behavior **exactly**: no metadata branch, no `.docket/`
worktree, no terminal-publish copy. Planning commits land on the integration branch alongside your
code, and the archive move there *is* the terminal record. Because the two-branch layout is the
default, an existing single-branch repository must pin `metadata_branch: main` to keep running
as-is until it deliberately migrates — otherwise the bootstrap guard stops and asks it to migrate.

## git-hook frameworks (pre-commit, husky, lefthook)

docket makes many small machine-generated bookkeeping commits — claims, board refreshes, status
writes, ADRs — on the metadata branch, and those commits **skip your repo's git hooks** by
construction. The `.docket/` worktree (and docket's transient publish and migration worktrees)
point `core.hooksPath` at an empty directory, so a shared `pre-commit` hook never fires against
docket's own commits, which live on the orphan metadata branch with no hook config anyway. Your
**code** commits on feature branches are untouched — the team's hooks still run on everything
headed to a pull request. There is nothing to configure; it is applied and self-heals on every
docket run.

## Migrating an existing repo onto docket

Two one-time migrations exist, each relevant only when you bring an *existing* repository onto
docket or carry one forward from an older docket layout. A brand-new repo needs neither.

### Moving a single-branch repo to the two-branch layout

A repository that has been running with everything on one branch moves to the two-branch layout
with a one-shot, idempotent command that operates on the git repo containing your **current
directory** — so run it from inside the repo you want to migrate:

```bash
cd <target-repo>
docket repository migrate
```

It prints the resolved target repo and prompts for confirmation before changing anything (pass
`--yes` to skip the prompt in automation). It then creates the orphan metadata branch seeded from
your current planning directories, prunes the live planning surface (active changes, the changes
index, the board) off the integration branch while keeping terminal records and build artifacts
there, and adds `.docket/` and `.worktrees/` to `.gitignore`. Re-running it converges from any
partial state.

Migration also grants one local, per-repo permission — an allow-rule for a push to the integration
branch, written to `.claude/settings.local.json` (which migration adds to `.gitignore`). This rule
is a **historical remnant**: it pre-authorized a close-out publish push that an earlier control
plane guarded, and it stays granted narrowly and harmlessly in this repo only. A fresh clone
simply will not carry it, and does not need to.

The skills will **not** migrate a repo for you. On first run against an un-migrated repository — its
metadata still on the integration branch, no metadata branch yet — a **bootstrap guard** stops and
points you at `docket repository migrate` rather than silently moving your data, and the same guard
detects a half-finished migration and points back to it to finish the move.

### Carrying a pre-0051 repo forward

A repository that predates docket change 0051 (which committed the per-repo agent files directly)
gets a one-time, automatic migration on its next install run: it deletes the stale tracked copies
from the working tree, writes the managed `.gitignore` block, regenerates the local set fresh, and
prints the single remedy commit to run, so the repository converges in one commit per clone.
