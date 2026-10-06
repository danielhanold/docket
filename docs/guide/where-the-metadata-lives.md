# Metadata branch: Where the metadata lives

By the end of this page you will know where docket keeps its planning records and why they sit
apart from your code: the two branches it uses, which record lives on which one, how your code
checkout stays put while a second checkout handles the backlog, and the commands that set a
repository up, check it, and repair it.

docket needs a durable, queryable record of planning state — changes, their statuses, decisions,
dependencies, and the **board** (the generated overview of every change and its state, never edited
by hand) — shared across agents, machines, and time, using **git as the only
storage** (no database, no service). It stores that state in one layout: a separate `docket` branch,
checked out at `.docket/`. The reasoning behind splitting metadata from code — the "why" under
everything on this page — is [Two branches and the metadata worktree](../concepts/two-branches.md).

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
  GitFlow) — stays code-only. It receives exactly what each pull request carries: a copy of the
  change's spec and the code. Nothing is pushed to it when a change closes out.

A change's feature branch is always cut from the integration branch — unless the change is a
**stacked change** (a change built on another change's unmerged branch rather than on the
integration branch), in which case it is cut from that parent's branch and targets it while the
parent is still open, then moves onto the integration branch once the parent is `done`. Either way,
the feature branch carries only the spec copy (its first commit) and code, and never modifies
planning records.

## Where each artifact lives

Each kind of record has one home:

| Record | Lives on | Reaches the integration branch |
|---|---|---|
| Change file (manifest + body) | metadata branch | never |
| Spec | metadata branch | a copy, as the feature branch's first commit, through the pull-request merge |
| ADR | metadata branch | never |
| Learnings | metadata branch | never |
| Board | metadata branch | never |
| Plan | metadata branch | never |
| Results | metadata branch | never |
| Build evidence | the change file's `## Build evidence` section | never |
| Code | feature branch | through the pull-request merge |
| `.docket.yml` | committed with your code | already there |

The split to notice: only the code and a copy of the spec ride a pull request onto the integration
branch. The plan, the results file, and the build evidence are written on the metadata branch as
the build runs, so recording a results checkpoint never moves the feature branch. Every planning
record stays on the metadata branch for good, including after the change closes out; the archive
lives there too. Links between files on the metadata branch are relative paths, so they keep
working after a record is archived; `docket repository repair` converts older absolute ones.

## `integration_branch` and GitFlow

The `integration_branch` key says where code lands and where feature branches are cut from:

- `auto` (the default, and what an absent key resolves to) follows the remote's default branch.
- `main` or `develop` is used verbatim.

That is what lets docket serve trunk-based (`main`) and **GitFlow** (`develop`) projects alike. One
caveat: `auto` follows the repo's *default* branch, so a GitFlow repository whose default branch is
`main` but whose real integration line is `develop` must set `integration_branch: develop`
explicitly. `integration_branch` is a shared coordination key — set it only in the committed
`.docket.yml`, per [Repo config](../install/config-layers.md); a machine layer cannot move it.

## The `.docket/` metadata worktree

Git checks out one branch per folder, so to write a file that belongs on the metadata branch while
your main folder sits on `main` or a feature branch, a skill needs a second folder parked on the
metadata branch — a **metadata worktree** (a second checkout of the repo at `.docket/`, parked on
the metadata branch, so backlog edits never touch your code checkout). Every workflow's startup
check (`docket repository prepare`) attaches the persistent worktree at `.docket/` and brings it up
to date with the remote before any read. **Your main working
tree never switches branches.**

`.docket/` is gitignored (alongside `.worktrees/`, which holds per-change feature worktrees), and
it deliberately sits at `.docket/` rather than under `.worktrees/` for three reasons: a change
could be titled "docket" and collide on `.worktrees/docket`; the metadata worktree is permanent
infrastructure while `.worktrees/` entries are ephemeral and get pruned; and keeping it out of
`.worktrees/` puts it outside the blast radius of any worktree-pruning cleanup.

## git-hook frameworks (pre-commit, husky, lefthook)

docket makes many small machine-generated bookkeeping commits — claims, board refreshes, status
writes, ADRs — on the metadata branch, and those commits **skip your repo's git hooks** by
construction. The `.docket/` worktree points `core.hooksPath` at an empty directory, and docket's
own commits are made with that same empty hooks directory, so a shared `pre-commit` hook never fires
against docket's bookkeeping, which lives on the orphan metadata branch with no hook config anyway.
Your **code** commits on feature branches are untouched — the team's hooks still run on everything
headed to a pull request. There is nothing to configure: `init`, `migrate`, and every workflow's
startup check apply the setting.

## Setting up, checking, and repairing a repository

All of these operate on the repository in your current directory (or the one `--repo-dir` names).

- **`docket repository init`** — for a repository that has never used docket. It creates the orphan
  `docket` branch, pushes it, and attaches the `.docket/` worktree. It also writes the managed
  `.gitignore` block (which ignores `.docket/`, `.worktrees/`, `.docket.local.yml`, and docket's
  other machine-local files) and a starting `.docket.yml` test policy, and leaves those edits
  unstaged for you to review and commit.
- **`docket repository migrate`** — only for a repository on the old single-branch layout, where
  the planning records still live on the integration branch. It prints its plan and asks for
  confirmation before changing anything (pass `--yes` to authorize it without the prompt). It then
  seeds the orphan `docket` branch with your changes, specs, and ADRs, and pushes one commit to the
  integration branch that removes the live planning surface (active changes and the board), keeps
  the archived records, and writes the managed `.gitignore` block and the `.docket.yml` test
  policy. It builds both commits directly from git objects, without a temporary worktree, and
  finishes by attaching `.docket/`. Re-running it after an interruption picks up where it stopped.
  A repository with nothing to migrate is told to run `init` instead.
- **`docket repository check`** — read-only. It reports the repository's health as findings: the
  metadata branch, the `.docket/` worktree, the committed `.gitignore` block, and the frontmatter
  of the change records.
- **`docket repository repair`** — previews the mechanical repairs `check` reports and applies them
  only once you confirm (or pass `--yes`).
- **`docket repository configure-tests`** — writes the build and finalize test commands into
  `.docket.yml` for a repository that is already set up, from suite discovery or from
  `--command "<cmd>"`, for you to review and commit.

The workflows never set a repository up for you. If one runs against a repository that is not set
up, its startup check stops and names the command to run: `init` for a fresh repository, `migrate`
for a single-branch one, including one whose migration was interrupted.
