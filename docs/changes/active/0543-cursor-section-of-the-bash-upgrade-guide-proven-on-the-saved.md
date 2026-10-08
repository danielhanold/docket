---
id: 543
slug: 'cursor-section-of-the-bash-upgrade-guide-proven-on-the-saved'
title: 'Cursor section of the Bash upgrade guide, proven on the saved cases'
status: 'proposed'
priority: 'high'
type: 'docs'
created: '2026-10-08'
updated: '2026-10-08'
depends_on: []
stacked_on:
related: [511, 512, 513, 514]
discovered_from: [512]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

The Bash upgrade guide (`docs/release/upgrading-from-bash.md`) covers Claude Code only. It tells Cursor users that using docket from Cursor on an upgraded repository is not supported. alpha.2 (change 0512) proves and publishes Cursor support, so the guide needs the Cursor steps before the alpha.2 candidate is cut.

This was split out of 0512 at grooming. The release protocol builds no code inside its freeze window, so the guide and its test must merge first and run inside the candidate's source gate, the same way 0511 preceded alpha.1.

## What changes

- **The guide.** Add the Cursor steps to `docs/release/upgrading-from-bash.md`:
  - taking over the old Cursor install under `~/.cursor` (skills, agents, and the v0.9.x `docket-dispatch.mdc` rule), including any ownership-conflict remedy;
  - the per-repository Cursor surface (`agent_harnesses` listing `cursor`, and the repository `.cursor/rules/docket-dispatch.mdc`);
  - Cursor's leftovers you can delete;
  - restarting Cursor;
  - the permissions note: docket runs outside Cursor's sandbox (link to `docs/install/cursor.md`).
  Remove the "not supported until their sections arrive" wording for Cursor; OpenCode keeps it.
- **The test.** Extend `TestBashUpgrade` (`internal/bashupgrade`) to assert the Cursor steps against the saved v0.9.2 and v0.9.3 cases, which already hold the Cursor files. It must end clean for Cursor the same way it does for Claude Code: a clean install check for the Cursor harness, nothing under `~/.cursor` pointing into the old checkout, and the guide's per-repository Cursor steps applied.
- **Code bugs stay out.** If the test exposes something the binary does wrong, rather than something the guide can explain, the build halts and the bug gets its own fix change.

## Out of scope

- OpenCode (alpha.3, change 0513) and Codex.
- The release itself (change 0512).
- Retiring the saved cases (change 0514).
