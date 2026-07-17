# Headless merge auto-approve — setup guide

Change 0062 lets `docket-finalize-change` merge a PR **without** `--admin` on a headless run, by
having a repo-controlled GitHub Actions workflow approve the PR itself before the merge — so
branch protection's required-review check is genuinely satisfied instead of bypassed. This is
opt-in, per repo: a one-time human-attended setup step plus a `.docket.yml` knob.

Read this before you opt in — it covers what the setup script changes, what finalize then does
differently at merge time, and the honest limitations (a bot review is not a substitute for a
human's, and this does not work on every repo).

## Why this exists

A solo maintainer can never approve their own PR — GitHub structurally disallows self-approval —
so branch protection's required review is unsatisfiable and merges have needed `--admin` to force
past it. `finalize.auto_approve` instead has a `github-actions[bot]` review satisfy that
requirement for real, the same pattern GitHub's own Dependabot auto-merge uses: a repo-controlled
workflow, running with the built-in `GITHUB_TOKEN`, approves the exact commit finalize is about to
merge, right after finalize's own rebase-retest gate has re-validated it. The merge that follows is
no longer "unreviewed" — it merges without `--admin`.

## Prerequisites

- **Repo admin access.** Both steps the setup script performs — pushing a new workflow file and
  flipping a repo-level Actions setting — need admin, not just write, access to the repo.
- **A `gh` CLI authenticated with a token that carries the classic `repo` scope** (or the
  fine-grained equivalents: Contents read/write for the workflow-file push, and Administration
  read/write for the Actions-permissions change). `gh auth status` shows what's currently active.
- **The `workflow` OAuth scope, if you push over HTTPS.** Installing `.github/workflows/docket-approve.yml`
  is itself a push to a path under `.github/workflows/`, and GitHub rejects that push from an
  HTTPS token that lacks the `workflow` scope, even if the token otherwise has full `repo` access.
  Two ways around it: re-authenticate with the scope added (`gh auth refresh -s workflow`), or use
  an SSH remote instead, which needs no extra scope for this push. The setup script recognizes this
  specific rejection and prints the same remediation rather than a bare git error.
- **"Allow GitHub Actions to create and approve pull requests," at the repo (or org) level.** This
  is the GitHub setting the setup script flips programmatically — you do not need to click it
  yourself in the repo's Settings → Actions page, but you do need enough access for the script's
  API call to succeed, and an **org-level** Actions policy that has this locked down can override
  the repo-level value regardless of what the script sets (see Limitations below).

## One-time setup: `docket.sh setup-auto-approve`

Run once, by a human, from a checkout of the repo you're opting in:

```bash
docket.sh setup-auto-approve
```

(Or directly: `bash scripts/setup-auto-approve.sh`, optionally with `--integration-branch <branch>`
if it can't resolve one from `origin/HEAD`, and `--remote <name>` if your remote isn't `origin`.)

It makes exactly two changes:

1. **Installs `scripts/templates/docket-approve.yml` onto your integration branch** as
   `.github/workflows/docket-approve.yml`, via a transient worktree and a direct push (the same
   posture as `terminal-publish.sh`'s writes to that branch). The workflow is a static template —
   every install is byte-identical — that, on `workflow_dispatch` with a `pr` number, checks the
   PR is open, not a draft, not from a fork, and on a docket `feat/*` branch, then runs
   `gh pr review --approve` with the built-in `GITHUB_TOKEN`. If the file is already up to date on
   that branch, no commit is made — this step is a no-op on a re-run.
2. **Flips `can_approve_pull_request_reviews` to `true`** on the repo's Actions permissions, via a
   `gh api -X PUT repos/{owner}/{repo}/actions/permissions/workflow` call. This is
   read-modify-write: the script first reads the repo's current `default_workflow_permissions`
   value and re-sends it unchanged, so an existing non-default permission level (e.g. `write`)
   survives the flip untouched — the call only ever adds the approve-permission, never resets
   anything else.

It never touches your committed `.docket.yml` — that's your decision, made in the next step — and
it prints a closing summary naming exactly what it changed.

**Verify it.** Read back the Actions setting directly:

```bash
gh api repos/<owner>/<repo>/actions/permissions/workflow
```

`can_approve_pull_request_reviews` should read `true`, and `default_workflow_permissions` should
match whatever it was before you ran the script. Confirm the workflow file landed with
`git log --oneline -- .github/workflows/docket-approve.yml` on your integration branch.

**Idempotent.** Re-running the script is safe: the workflow-file step no-ops when the content is
already current, and the permissions PUT re-sends the same values GitHub already has — running it
twice leaves the repo in the same state as running it once.

## Enabling the knob

With setup done, opt the repo in by setting, in the repo's committed `.docket.yml`:

```yaml
finalize:
  auto_approve: true
```

From then on, `docket-finalize-change`'s merge gate changes for a headless run whose PR is not
already `APPROVED`. After the rebase-retest gate finishes re-validating the branch and pushes it
(so the approval always covers the exact SHA being merged), finalize:

1. Dispatches `docket-approve.yml` (`gh workflow run … -f pr=<N>`).
2. Polls that run to completion.
3. Re-checks that `reviewDecision` is now `APPROVED`.
4. Merges the PR — **without** `--admin`.

If any of steps 1–3 fails — the dispatch is rejected, the run fails or times out, or the approval
never materializes — finalize **aborts and reports** rather than merging: the PR is left open, the
reason is surfaced (and recorded as a PR comment), and it **never** falls back to `--admin`. Falling
back would silently reintroduce the self-approval bypass this feature exists to retire. When
`auto_approve` is `false` (the default) or the PR is already approved by a human, this whole step
is a no-op — merge behavior is unchanged.

`finalize.auto_approve` is a coordination key (per the same rule as `terminal_publish` and the
other shared-state knobs): it is **per-repo-only** and warned-and-ignored if set in the global
config or a machine-local `.docket.local.yml` — set it only in the repo's committed `.docket.yml`.

## Limitations — read before you rely on this

- **CODEOWNERS-protected repos are not supported.** A `github-actions[bot]` review counts toward a
  plain "require approvals" branch-protection rule, but it does **not** satisfy a CODEOWNERS
  requirement — GitHub does not treat a bot as a code owner. If your integration branch requires
  CODEOWNERS review, this feature cannot clear that gate for you; a human still has to approve.
- **An org-level Actions policy can override the repo setting.** `can_approve_pull_request_reviews`
  is a repo-level toggle, but an organization can lock down Actions permissions org-wide in a way
  that takes precedence over what the setup script sets on any individual repo. If dispatched runs
  keep failing to produce an approval, check your org's Actions policy first.
- **A bot approval is not a human review — and `require_pr_approval: true` becomes bot-satisfiable
  once `auto_approve` is also `true`.** `finalize.require_pr_approval` was designed to prove a
  human authorized the merge; under `auto_approve`, the review it checks for can be the workflow's
  own approval, so the combination is legal but no longer proves what `require_pr_approval` was
  built to prove. Consult ADR-0042 (auto-approve consent model, relating to ADR-0011's finalize
  consent model) for the full reasoning before combining the two knobs. A repo that wants a real human in the loop should leave
  `auto_approve` at its default `false`.
- **The `terminal_publish: true` headless degradation still applies, independently.** Auto-approve
  only changes the *merge* step; it does nothing to finalize's separate direct-push-to-integration
  step for repos that also opt into `terminal_publish: true`. On a headless run, that push can
  still be denied by an agent permission classifier — a failure this feature does not touch.
  Finalize does not fail the run over it: archive, cleanup, and the board have already completed,
  and it surfaces a one-line follow-up asking you to run `docket.sh terminal-publish` manually. See
  `docket-finalize-change`'s `SKILL.md` ("Headless publish degradation") for the exact wording.
