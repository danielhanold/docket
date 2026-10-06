<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0535 — Load a private repository's agent instructions without repository files](../changes/active/0535-load-a-private-repository-s-agent-instructions-without-repos.md)**
<!-- docket:backlink:end -->

# Load a private repository's agent instructions without repository files — Results

**Human action:** Assessment pending: the build is green and the review fix pass is in progress.

## Outcome

In a private repository, docket now keeps the parent-facing dispatch rules and promoted lessons in `.git/dckt/AGENTS.md` and writes nothing into the working tree. A new read-only `docket instructions` command prints that file, or one section of it, from any worktree of a private repository, and prints nothing anywhere else. It can also wrap its output as Claude Code or Cursor session-start hook output. `docket install` adds content-free user-level triggers: two Claude Code `SessionStart` hooks, a Cursor `sessionStart` hook, an OpenCode plugin, and a Codex pointer block. All nine plan tasks are built; the full suite passed at the build gate.

## Verification performed

- Build gate: full suite green after one repair. The first run failed one pre-existing flaky test in `internal/process` (`TestRecoverLeavesUnprovableGroupForInspection`, a process-group kill race in its setup). It was fixed in-branch.
- Fresh-session acceptance (plan Task 9) passed for Claude Code 2.1.291, Cursor 2026.10.01, OpenCode 1.18.31 and Codex 0.154.0, all in print/exec mode against a temporary home.

## Known issues and follow-ups

Whole-branch review (deep tier) returned 8 findings: 0 blocker, 4 important, 4 minor. In-branch fixes are in progress:
1. (important) A scoped install drops the Codex clause from the private file.
2. (important) A symlinked `settings.json` or `hooks.json` stops every install.
3. (important) The install and uninstall docs don't mention the new triggers.
4. (important) An unparseable hooks file stops every install.
5. (minor) Hook commands that a later release changes are never retired.
6. (minor) Retirement leaves an empty `AGENTS.md` in the private worktree.
7. (minor) Surfaces an earlier install wrote and recorded from a linked worktree are never retired.
8. (minor) A JSON-mode `instructions` failure uses `Failure` and carries no reason code.
