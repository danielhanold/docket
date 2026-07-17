# setup-auto-approve.sh — one-time, human-attended install for finalize's auto-approve

## Purpose

`setup-auto-approve.sh` is the one-time, **human-attended** setup procedure for change 0062's
headless-merge auto-approve path. It performs the two repo-level writes that
`finalize.auto_approve: true` depends on:

1. Installs `scripts/templates/docket-approve.yml` onto the integration branch as
   `.github/workflows/docket-approve.yml`, via a transient git worktree and a direct push (the
   same posture as `terminal-publish.sh`'s transient-worktree mechanics, minus the CAS retry loop —
   this is a single human-attended run, not a concurrent-safe background publish).
2. Flips the repo's Actions setting `can_approve_pull_request_reviews` to `true` via a `gh api`
   PUT, using **read-modify-write**: it first reads the current
   `default_workflow_permissions` value and re-sends it unchanged, so it never silently resets
   the repo's default workflow permission level.

It then prints a reminder: the script itself never edits the repo's committed `.docket.yml` — the
human must set `finalize.auto_approve: true` there themselves.

**This script is NEVER invoked by an autonomous skill.** It is run once, by a human with repo-admin
access, as part of opting a repo into headless-merge auto-approve. No docket skill (`docket-status`,
`docket-implement-next`, `docket-finalize-change`, `docket-adr`, ...) calls it, and it is not part
of any skill's Step-0 flow.

## Usage

```bash
setup-auto-approve.sh [--integration-branch B] [--remote R]
```

- `--integration-branch B` — the branch to install the workflow onto. Defaults to the branch
  `<remote>/HEAD` points at (resolved via `git remote set-head <remote> -a` then
  `git symbolic-ref refs/remotes/<remote>/HEAD`). If that cannot be resolved, the script dies
  asking for the flag explicitly.
- `--remote R` — defaults to `origin`.

Mock seams: `GIT="${GIT:-git}"`, `GH="${GH:-gh}"`.

## Behavior

### (1) Install the workflow onto the integration branch

1. Fetches `<remote>/<integration-branch>`.
2. Provisions a transient worktree at `<repo-root>/.setup-approve-wt` on a throwaway branch
   `setup-approve`, checked out from `<remote>/<integration-branch>`. Immediately before
   `git worktree add -B`, any leftover `setup-approve` worktree/branch from a prior interrupted
   run is force-removed (`git worktree remove --force`, `git branch -D`) so the fixed
   path/branch name never wedges a later run — `git worktree prune` alone cannot clear a
   still-present directory.
3. Best-effort disables the team's shared git hooks inside that worktree
   (`disable-worktree-hooks.sh --worktree`) — this is docket's own asset commit, not the team's
   code, so their hooks should not fire on it.
4. Copies `scripts/templates/docket-approve.yml` to
   `.github/workflows/docket-approve.yml` inside the worktree and stages it.
5. **Guarded commit:** if the staged content is byte-identical to what's already on the
   integration branch, no commit is made (`setup-auto-approve: workflow already up to date on
   <branch> (no commit needed)`) — this is the idempotency path. Otherwise it commits
   (`chore(docket): install docket-approve.yml auto-approve workflow`) and pushes `HEAD:<branch>`
   directly to `<remote>`.
6. **Workflow-OAuth-scope caveat:** pushing a new/changed file under `.github/workflows/` over
   HTTPS requires the token's OAuth grant to include the `workflow` scope; a plain `repo`-scoped
   HTTPS token is rejected by GitHub for this specific path. If the push fails and the captured
   stderr mentions `workflow`, the script surfaces a targeted hint (re-auth with
   `gh auth refresh -s workflow`, or push over SSH instead) rather than a bare git error. Any other
   push failure is reported with the raw git stderr.
7. Tears down the transient worktree and branch (`git worktree remove --force`,
   `git branch -D setup-approve`) whether the commit path or the up-to-date path was taken, and
   prunes worktree registrations. No leftover `.setup-approve-wt` worktree survives a
   successful (or a cleanly-failed) run.

### (2) Flip the repo Actions setting (read-modify-write)

1. Resolves `owner/repo` via `gh repo view`.
2. Reads the current `repos/<owner>/<repo>/actions/permissions/workflow` payload via `gh api`.
3. Extracts the existing `default_workflow_permissions` value (defaults to `read` if the field is
   absent from the response, matching GitHub's own default).
4. Sends a PUT to the same endpoint with `default_workflow_permissions=<preserved value>` and
   `can_approve_pull_request_reviews=true` — **never** a blind PUT that would silently reset
   `default_workflow_permissions` to GitHub's default. This is the read-modify-write guarantee: an
   existing `write` (or any non-default) permission level survives the flip untouched.
5. A read or write failure here dies with a message pointing at the required access (repo admin +
   a token with the `repo` scope for the read; org Actions policy may override the repo-level
   setting for the write).

### (3) Reminder

Prints a closing block naming exactly what changed and what the human must still do: set
`finalize.auto_approve: true` in the repo's **committed** `.docket.yml` (this script never touches
that file), plus the `gh api` command to verify the flipped setting.

## Exit codes

- `0` — the workflow file is present on the integration branch (freshly installed or already
  up to date) **and** the Actions setting was flipped successfully.
- Non-zero — any failure: bad arguments, missing template, not a git repo, unresolvable
  integration branch, fetch failure, worktree provision failure, copy/commit failure, push
  failure (including the workflow-scope case, which dies with the targeted hint above), unresolved
  `owner/repo`, or a `gh api` read/write failure.

## Invariants

- **Human-attended only, never autonomous.** No docket skill invokes this script; it is a
  standalone `docket.sh setup-auto-approve` run performed once by a human with repo-admin access.
- **Idempotent.** A second run with the same arguments makes no new commit when the workflow file
  is already byte-identical on the integration branch, and re-sends the same PUT (a no-op from
  GitHub's perspective) — running it twice is safe and leaves exactly one workflow file.
- **Read-modify-write, never blind-set.** The `can_approve_pull_request_reviews` PUT always
  preserves whatever `default_workflow_permissions` value it read; it never resets that field to a
  default the repo did not already have.
- **Never edits committed config.** The script does not write `.docket.yml`; it only prints a
  reminder naming the `finalize.auto_approve: true` knob the human must set themselves.
- **No leftover worktree.** The transient `.setup-approve-wt` worktree and its `setup-approve`
  branch are torn down on every exit path that reaches the teardown call; a crash before that
  point is recoverable on the next run because the leftover worktree/branch is force-removed
  immediately before `git worktree add -B` runs again, rather than erroring on it.
- **The workflow-scope failure is diagnosable, not a bare git error.** A push rejected because the
  token lacks the `workflow` OAuth scope surfaces a specific remediation hint instead of the raw
  git stderr.
