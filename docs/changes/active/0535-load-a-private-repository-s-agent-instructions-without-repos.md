---
id: 535
slug: 'load-a-private-repository-s-agent-instructions-without-repos'
title: 'Load a private repository''s agent instructions without repository files'
status: 'in-progress'
priority: 'medium'
type: 'feat'
created: '2026-10-06'
updated: '2026-10-06'
depends_on: [531, 534]
stacked_on:
related: [532, 533, 334, 351]
discovered_from: [531]
adrs: [36, 78]
spec: 'docs/superpowers/specs/2026-10-06-load-a-private-repository-s-agent-instructions-without-repos-design.md'
plan: 'docs/superpowers/plans/2026-10-06-load-a-private-repository-s-agent-instructions-without-repos.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/load-a-private-repository-s-agent-instructions-without-repos'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-06T20:40:42Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-06-load-a-private-repository-s-agent-instructions-without-repos-design.md](../../superpowers/specs/2026-10-06-load-a-private-repository-s-agent-instructions-without-repos-design.md) |
| Plan | [2026-10-06-load-a-private-repository-s-agent-instructions-without-repos.md](../../superpowers/plans/2026-10-06-load-a-private-repository-s-agent-instructions-without-repos.md) |
| ADRs | [ADR-0036](../../adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0078](../../adrs/0078-parent-facing-gate-surface-for-claude-one-physical-instructions-file.md) |
<!-- docket:artifacts:end -->

## Why

Docket's dispatch and run-tracker rules reach the parent agent only through the repository's own CLAUDE.md or AGENTS.md. Those rules say: dispatch the named docket agent, and bracket every implement-next run with the run tracker. The user-level copy was retired because two copies drifted apart.

A private-visibility repository can't carry those files, so docket must not write them there. Without another delivery path, agents in a private repository would run docket workflows inline and untracked. Promoted lessons have the same problem: they normally graduate into the repository's AGENTS.md, which a restricted repository doesn't allow.

## What changes

- In private repositories, the dispatch block and promoted lessons live in a private instructions file under `.git/dckt/`. Docket writes nothing in the worktree: no AGENTS.md, CLAUDE.md, or rule file, and no `.git/info/exclude` entry.
- A new read-only `docket instructions` command prints that file in a private repository and nothing anywhere else. It can print the dispatch block or the lessons on their own, and can wrap its output in Claude Code's or Cursor's session-start hook format.
- `docket install` sets up delivery once per machine. No user-level surface contains rule text or the word "docket":
  - Claude Code: two user-level `SessionStart` hooks in `settings.json`, one for the dispatch block and one for the lessons, so each fits Claude Code's per-hook size cap.
  - Cursor: a user-level `sessionStart` hook in `~/.cursor/hooks.json`.
  - OpenCode: a user-level plugin that adds the file to the system prompt.
  - Codex: a static pointer block (markers `dckt:`) in its user-level AGENTS.md, best effort.
- The build re-checks each harness in a fresh session against the real binary. The grooming spike already proved the mechanisms with a stub.
- `docket install` in a private repository writes no instruction file into the repository root: no AGENTS.md, CLAUDE.md, or docket-named rule file (found during change 531's build, where install could still write them). It routes the content to the private instructions file instead.

## Out of scope

- Shared repositories, whose repository-level dispatch blocks are unchanged.
- What ships through PRs and commits: writing rules and the leak check (#532).
- Installing the `dckt` alias itself (#534).
- Moving the instructions when switching modes (#533).

## Reconcile log

### 2026-10-06

Reconciled 2026-10-06 against main d997c1210 (after #532's private-visibility PR commits landed). Dependencies #531 and #534 are done. The cited code still matches the spec: `GlobalDispatchTarget` adapters in `internal/harness/*` and `internal/install/service.go`, private layout under `.git/dckt/` (`internal/layout`, `internal/app/repository_init_private.go`), and `.git/info/exclude` handling in `internal/reposetup/exclude.go`. No `docket instructions` command or private instructions file exists yet. #533 is still proposed and depends on this. Scope unchanged.

Spike posture for an autonomous build: the per-harness fresh-session check runs non-interactively where a harness CLI allows it without touching the user's real configuration; any harness that cannot be exercised that way is recorded in the results file as an Important human verification item rather than halting the run. A harness that is exercised and fails still stops the build, as the spec says.

### 2026-10-06

Re-groomed with the human after the run halted at the delivery spike. The human had alternatives re-tested for Claude Code, Cursor, and OpenCode, and settled the design (spec *Delivery spike* and *Decisions*):

- **Claude Code:** two `SessionStart` hooks in `settings.json`, split into dispatch block and lessons, managed by install and uninstall. A plugin was ruled out because it does not lift the per-hook cap.
- **Cursor:** a user-level `sessionStart` hook, replacing the excluded per-repository rule file.
- **OpenCode:** a user-level system-prompt plugin, replacing the pointer.
- **Codex:** the pointer, unchanged, as best effort.

The spec's *Summary*, *Evidence*, *Decisions*, *Design*, *Acceptance criteria*, and *ADRs expected* sections were replaced, and *What changes* was updated. Plan Task 1 (the spike) is superseded by the spec's fresh-session acceptance. The plan is rewritten against the revised spec before the halted run resumes.

