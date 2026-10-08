---
id: 543
slug: 'cursor-section-of-the-bash-upgrade-guide-proven-on-the-saved'
title: 'Cursor section of the Bash upgrade guide, proven on the saved cases'
status: 'in-progress'
priority: 'high'
type: 'docs'
created: '2026-10-08'
updated: '2026-10-08'
depends_on: []
stacked_on:
related: [511, 512, 513, 514]
discovered_from: [512]
adrs: [96]
spec: 'docs/superpowers/specs/2026-10-08-cursor-section-of-the-bash-upgrade-guide-proven-on-the-saved-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'docs/cursor-section-of-the-bash-upgrade-guide-proven-on-the-saved'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-08T16:15:28Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-08-cursor-section-of-the-bash-upgrade-guide-proven-on-the-saved-design.md](../../superpowers/specs/2026-10-08-cursor-section-of-the-bash-upgrade-guide-proven-on-the-saved-design.md) |
| ADRs | [ADR-0096](../../adrs/0096-legacy-reproduction-uses-a-frozen-embedded-floor.md) |
<!-- docket:artifacts:end -->

## Why

The Bash upgrade guide (`docs/release/upgrading-from-bash.md`) covers Claude Code only. It tells Cursor users that using docket from Cursor on an upgraded repository is not supported. alpha.2 (change 0512) proves and publishes Cursor support, so the guide needs the Cursor steps before the alpha.2 candidate is cut.

This was split out of 0512 at grooming. The release protocol builds no code inside its freeze window, so the guide and its test must merge first and run inside the candidate's source gate, the same way 0511 preceded alpha.1.

## What changes

- **The guide.** One combined path for Claude Code and Cursor in `docs/release/upgrading-from-bash.md`:
  - the install line names both harnesses (drop the one you don't use), on `v1.0.0-alpha.2`;
  - a marked Cursor takeover step removes Bash's `~/.cursor/skills` links, and on v0.9.3 also `docket-plan-writer.md` and Bash's user-level `~/.cursor/rules/docket-dispatch.mdc` (v0.9.2's rule is removed by the installer itself);
  - `configure-harnesses` with `claude,cursor`;
  - Cursor's leftovers, the sandbox note, and restarting Cursor.
- **The test.** `TestBashUpgrade` runs the guide once per saved case through both harnesses. It keeps every Claude Code assertion and adds Cursor's: the expected conflicts, no user-level rule left, a clean install check, no `~/.cursor` links into the old checkout, and the repository rule written.
- **Code bugs stay out.** If the test exposes something the binary does wrong, rather than something the guide can explain, the build halts and the bug gets its own fix change.

## Out of scope

- OpenCode (alpha.3, change 0513) and Codex.
- The release itself (change 0512).
- Retiring the saved cases (change 0514).

## Reconcile log

### 2026-10-08

2026-10-08: Reconciled against main 2847444ae. The guide (docs/release/upgrading-from-bash.md) and internal/bashupgrade are unchanged since grooming apart from the already-landed artifact-backlink-stale table row; no related change has shipped Cursor guide content. Scope stands as specified.
