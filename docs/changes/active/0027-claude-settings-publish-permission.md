---
id: 27
slug: claude-settings-publish-permission
title: Auto-grant docket's integration-branch push permission via a per-repo Claude settings rule
status: implemented
priority: medium
created: 2026-06-19
updated: 2026-06-19
depends_on: [26]
related: [15, 22, 25, 26]
adrs: []
spec: docs/superpowers/specs/2026-06-19-claude-settings-publish-permission-design.md
plan: docs/superpowers/plans/2026-06-19-claude-settings-publish-permission.md
results: docs/results/2026-06-19-claude-settings-publish-permission-results.md
trivial: false
auto_groomable:
branch: feat/claude-settings-publish-permission
pr: https://github.com/danielhanold/docket/pull/39
blocked_by:
reconciled: true
---

## Why

Every docket terminal transition (`done`/`killed`) ends in the shared **terminal-publish**
procedure, whose final step pushes the change's terminal records onto the integration
branch with `git -C "$pub" push origin HEAD:<integration_branch>`. Claude Code's permission
classifier guards direct pushes to the repository's default branch, so in an interactive
session this push is refused unless the human approves it — on **every** close-out. (Seen
live finalizing change #23: the `origin/docket` archive push, the branch delete, and
`gh pr merge` all proceeded; only the publish push to `main` was blocked and needed manual
approval.)

This is exactly the kind of deterministic, well-understood action that should be
pre-authorized — but **narrowly**, and **only in repos that actually use docket**. A
machine-global grant would silently authorize pushes to `main` in every unrelated repo
Claude touches.

## What changes

Designed at brainstorm 2026-06-19 — see
[the spec](../../superpowers/specs/2026-06-19-claude-settings-publish-permission-design.md).
Decisions:

- **Per-repo, gitignored grant.** Write a single allow-rule
  `Bash(git -C * push origin HEAD:<integration_branch>)` into the repo's **local** Claude
  config `<repo>/.claude/settings.local.json` (`.permissions.allow`). The `*` absorbs the
  mktemp worktree path; the fixed tail authorizes **only** a push to the integration
  branch's HEAD — force-push and other-branch pushes stay guarded. The rule mirrors
  terminal-publish's existing command, so **no skill/convention edit is needed**.
- **New helper `scripts/ensure-claude-settings.sh`** — idempotent, standalone-runnable:
  resolves the repo root from `$PWD`, resolves `<integration_branch>` by consuming change
  **#26**'s `scripts/docket-config.sh --export` (no duplicated config parsing — the reason
  for the dependency), creates `<repo>/.claude/` + `settings.local.json` if absent, and
  merges the rule via `jq` (already a repo dependency) preserving all pre-existing keys.
- **`migrate-to-docket.sh` invokes the helper** as a setup step (next to its existing
  `.gitignore` extension), so migrating a repo to docket-mode grants the rule. Documented in
  its "next steps" plus the README, including that a fresh cloner of an already-migrated repo
  can run the helper **standalone** to grant themselves the rule (the gitignored/per-user
  gap).
- **Repo-level `.gitignore` of `.claude/settings.local.json`** (reconcile finding). The
  grant's "never committed onto collaborators" guarantee must not depend on each developer
  having a user-global ignore for `settings.local.json`. So `migrate-to-docket.sh`'s existing
  step-5 `.gitignore` extension also adds `.claude/settings.local.json` (committed once, for
  every clone). The **standalone helper itself stays git-write-free** — a fresh cloner already
  inherits the committed `.gitignore` entry, so the file it writes is ignored without the
  helper touching git.
- `tests/test_ensure_claude_settings.sh` — hermetic: create-when-absent, idempotent (no dup),
  preserve existing settings, `main` vs `develop` branch resolution, no git writes.

## Out of scope

- `install.sh` — machine-level setup (harness root `~/.claude`), no per-repo context; wrong
  owner for a per-repo rule.
- Any user-global (`~/.claude/…`) permission rule.
- Editing terminal-publish's invocation or the convention's terminal-publish procedure (a
  `cd && git push` rewrite was considered and rejected — it risks shifting the prompt onto a
  `cd` into the out-of-workspace mktemp dir, for no real safety gain).
- Authorizing pushes to `origin/<metadata_branch>` (docket branch) or branch deletes, and
  `gh pr merge` — none are blocked by the classifier (only the default branch is guarded).

## Open questions

Resolved at brainstorm — see the spec. Minor build-time choices left open: the exact `jq`
merge expression and whether the test seam is a `--integration-branch` flag or an env
override. None blocking.

**Dependency note.** `depends_on: [26]` is deliberate — the helper consumes #26's
`docket-config.sh` resolver for `<integration_branch>` rather than re-parsing `.docket.yml`,
eliminating a second config-resolution site. #26 is now `done` (PR #38 merged); the resolver
is live on `origin/main` (`scripts/docket-config.sh --export` → `INTEGRATION_BRANCH`, with
`--repo-dir DIR` as a ready-made hermetic test seam). #27 is build-ready.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-06-19 — reconcile before build

Brainstormed and built the same day; the world moved only by the dependency landing.

- **Dependency #26 reached `done`** (PR #38 merged into `origin/main` since brainstorm). The
  consumed resolver `scripts/docket-config.sh --export` is live and emits
  `INTEGRATION_BRANCH=main` (verified on `origin/main`). Build base `origin/main` is at
  `af27acd`, ahead of the brainstorm snapshot. Change is build-ready.
- **Rule shape confirmed against the real command.** `scripts/terminal-publish.sh:108` runs
  `git -C "$pub" push "$REMOTE" "HEAD:$INT_BRANCH"` with `REMOTE` defaulting to `origin` and
  `INT_BRANCH=main` → `git -C "$pub" push origin HEAD:main`. The planned allow-rule
  `Bash(git -C * push origin HEAD:main)` matches it verbatim; force-push / other-branch pushes
  stay guarded as designed.
- **New constraint folded in — repo-level gitignore.** `.claude/settings.local.json` is
  ignored on this machine only via the *user-global* excludesfile (`~/.config/git/ignore`),
  **not** the repo `.gitignore` (which lists only `.DS_Store`, `.docket/`, `.worktrees/`,
  `.superpowers/`). The spec's "gitignored by convention (already true in this repo)" is
  therefore a per-machine property, not a repo property — a collaborator without that global
  ignore could accidentally commit their local grant, defeating the change's "never committed
  onto collaborators" goal. Added to scope: `migrate-to-docket.sh`'s step-5 `.gitignore`
  extension also ignores `.claude/settings.local.json` (committed, machine-independent); the
  standalone helper stays git-write-free. Spec §3/§5/§7 updated to match.
- **Test seam simplification noted.** `docket-config.sh --repo-dir DIR` already exists as a
  clean hermetic seam (point it at a fixture repo with a `.docket.yml`), so the new helper may
  not need its own `--integration-branch` flag — a build-time choice, non-blocking.
- No scope dropped; design not invalidated. Proceeding to plan + build.
