---
id: 27
slug: claude-settings-publish-permission
title: Auto-grant docket's integration-branch push permission via a per-repo Claude settings rule
status: in-progress
priority: medium
created: 2026-06-19
updated: 2026-06-19
depends_on: [26]
related: [15, 22, 25, 26]
adrs: []
spec: docs/superpowers/specs/2026-06-19-claude-settings-publish-permission-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/claude-settings-publish-permission
pr:
blocked_by:
reconciled: false
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
eliminating a second config-resolution site. #26 is `implemented` (PR #38); #27 becomes
build-ready when #26 reaches `done`.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
