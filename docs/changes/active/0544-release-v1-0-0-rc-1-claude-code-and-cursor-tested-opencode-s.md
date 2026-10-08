---
id: 544
slug: 'release-v1-0-0-rc-1-claude-code-and-cursor-tested-opencode-s'
title: 'Release v1.0.0-rc.1: Claude Code and Cursor tested, OpenCode shipped untested'
status: 'proposed'
priority: 'high'
type: 'chore'
created: '2026-10-08'
updated: '2026-10-08'
depends_on: [512]
stacked_on:
related: [366, 511, 512, 513, 514, 543]
discovered_from: []
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
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

The Go pre-releases have grown one harness at a time: Claude Code in `v1.0.0-alpha.1` (change 0366) and Cursor in `v1.0.0-alpha.2` (change 0512). The next step is a release candidate for `v1.0.0`.

On 2026-10-08 the human decided that OpenCode will not be human-tested before `v1.0.0`. Full OpenCode support moves to after `v1.0.0` (change 0513, deferred). OpenCode should still work, so the release candidate keeps shipping its files.

The release candidate is the first build that claims both Claude Code and Cursor together, so it needs its own acceptance run rather than relying on the two alphas.

## What changes

- **Release.** Package, verify and publish `v1.0.0-rc.1` with the lean pre-release protocol that 0366 and 0512 settled, plus anything alpha.2 teaches.
- **Human test.** Run the full lifecycle on Claude Code and on Cursor against the same candidate.
- **Harness status in the release notes and docs:**
  - Claude Code and Cursor: tested.
  - OpenCode: **untested**, not unsupported. The OpenCode files still ship and install as they do today.
  - Codex: the only officially **unsupported** harness.
- **Upgrade path.** The release notes link the Bash upgrade guide (0511, with the Cursor section from 0543). The guide has no OpenCode section yet; the notes say so.

## Out of scope

- Human-testing OpenCode, or an OpenCode section in the upgrade guide (change 0513, after `v1.0.0`).
- Removing or disabling OpenCode files from the release.
- Codex support.
- Stable `v1.0.0` itself.
- Source changes of any kind inside the release freeze; a defect gets its own change and a new candidate.
