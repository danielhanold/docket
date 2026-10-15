---
id: 552
slug: 'install-claude-code-allow-rules-so-finalize-can-publish-and'
title: 'Install Claude Code allow rules so finalize can publish and merge under auto mode'
status: 'implemented'
priority: 'medium'
type: 'fix'
created: '2026-10-15'
updated: '2026-10-15'
depends_on: []
stacked_on:
related: [27, 408]
discovered_from: []
adrs: [145, 150]
spec: 'docs/superpowers/specs/2026-10-15-install-claude-code-allow-rules-so-finalize-can-publish-and-design.md'
plan: 'docs/superpowers/plans/2026-10-15-install-claude-code-allow-rules-so-finalize-can-publish-and.md'
results: 'docs/results/2026-10-15-install-claude-code-allow-rules-so-finalize-can-publish-and-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/install-claude-code-allow-rules-so-finalize-can-publish-and'
pr: 'https://github.com/danielhanold/docket/pull/415'
blocked_by:
reconciled: true
claimed_at: '2026-10-15T19:47:16Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-15-install-claude-code-allow-rules-so-finalize-can-publish-and-design.md](../../superpowers/specs/2026-10-15-install-claude-code-allow-rules-so-finalize-can-publish-and-design.md) |
| Plan | [2026-10-15-install-claude-code-allow-rules-so-finalize-can-publish-and.md](../../superpowers/plans/2026-10-15-install-claude-code-allow-rules-so-finalize-can-publish-and.md) |
| Results | [2026-10-15-install-claude-code-allow-rules-so-finalize-can-publish-and-results.md](../../results/2026-10-15-install-claude-code-allow-rules-so-finalize-can-publish-and-results.md) |
| ADRs | [ADR-0145](../../adrs/0145-private-repository-parent-facing-rules-load-through-content.md), [ADR-0150](../../adrs/0150-docket-install-grants-claude-code-allow-rules-for-finalize-p.md) |
<!-- docket:artifacts:end -->

## Why

When implement-next has just run the full suite green and finalize runs right after, finalize correctly skips re-running the tests. Claude Code's auto-mode classifier reads the resulting `docket finalize publish` / `docket finalize merge` as a "CI bypass" and soft-denies them, so an otherwise clean finalize stalls on a human prompt. Adding `Bash(docket finalize publish:*)` and `Bash(docket finalize merge:*)` to `permissions.allow` in the user-level `~/.claude/settings.json` cleared the denial (observed on another machine, 2026-10). Today every user has to discover and hand-add those rules on every machine.

## What changes

`docket install` manages the two allow rules in `~/.claude/settings.json`, alongside the two SessionStart hooks it already owns there. The existing Claude settings target grows a second owned list (allow rules) next to its hook commands, because the install record allows one managed target per file. Every install converges the file: missing rules are added back, rules a newer docket no longer ships are retired, and an already-correct file stays byte-identical. Uninstall removes them with the hooks. On by default whenever the Claude harness is installed, no opt-out knob. A file docket cannot edit (symlink, unparseable) is skipped with a warning naming the rules to add by hand. A test pins each rule to the capability catalog's argv for `finalize.publish` / `finalize.merge`. Install docs list the rules.

## Out of scope

Per-repository grants (`.claude/settings.local.json`) — more surfaces, absent on fresh clones, and private repositories keep nothing in the root. Other harnesses (Cursor, Codex, OpenCode have no such classifier). The `dckt` alias spelling. Any other finalize subcommand or docket command. A config knob to disable the rules. A blocking probe of classifier behavior. Generalizing the hook-entries machinery into a generic JSON-array target.

## Reconcile log

### 2026-10-15

2026-10-15: Traced current code on main (9ace757c3). The spec still holds: `harness.ClaudeHookTarget` in internal/harness/trigger.go owns the two SessionStart hooks through one hook-entries target (internal/install/hook_entries.go), no allow-rule support exists yet, and finalize.publish / finalize.merge are cataloged as `docket finalize publish` / `docket finalize merge`. No related or archived change has done this work. ADR 145 already linked. Scope unchanged.

## Build evidence

<!-- docket:build-evidence:start -->
command:  go run ./cmd/docket development test
result:   green
head_sha: d11b1b9bd61e283773d4d66e64712a159a6d384a
ran_at:   2026-10-15T20:15:23Z
<!-- docket:build-evidence:end -->
