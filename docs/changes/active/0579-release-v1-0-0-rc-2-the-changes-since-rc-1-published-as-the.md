---
id: 579
slug: 'release-v1-0-0-rc-2-the-changes-since-rc-1-published-as-the'
title: 'Release v1.0.0-rc.2: the changes since rc.1, published as the build to test'
status: 'proposed'
priority: 'high'
type: 'chore'
deliverable: 'pr'
created: '2026-10-23'
updated: '2026-10-23'
depends_on: []
stacked_on:
related: [544, 512, 513, 514, 545, 555, 570, 573, 575]
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-10-23-release-v1-0-0-rc-2-the-changes-since-rc-1-published-as-the-design.md'
plan:
results:
auto_groomable:
hold:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-23-release-v1-0-0-rc-2-the-changes-since-rc-1-published-as-the-design.md](../../superpowers/specs/2026-10-23-release-v1-0-0-rc-2-the-changes-since-rc-1-published-as-the-design.md) |
<!-- docket:artifacts:end -->

## Why

`v1.0.0-rc.1` was published on 2026-10-08 (change 0544). Since then about 45 changes, 267 commits, have landed on `main`, and none of them is in a release. They include private repositories, `docket open` and export, holds, loops that carry on past a halt, closing a change without a PR, the Claude Code allow rules for finalize, and the retirement of `trivial` and of claim reclaim.

The human will test these by installing rc.2 from the GitHub release on their own machines and running real work through Claude Code and Cursor. That use is the test, so rc.2 is not human-tested before it is published.

## What changes

- **Release.** Package, verify and publish `v1.0.0-rc.2` from `main` with rc.1's protocol (0544). Like rc.1 it is a **full release, not a pre-release**, and becomes "Latest", replacing rc.1.
- **No test before publishing.** No harness test and no separate public-install check: the human's own install from the release is both. The candidate's full test suite gates it, as for rc.1.
- **Harness status, unchanged from rc.1:** Claude Code and Cursor supported; OpenCode shipped but untested; Codex unsupported.
- **Release notes:** install; upgrading (from rc.1 or an alpha, including a per-repository step to refresh shared repositories' dispatch instructions; from Bash `v0.9.x`); what changed since rc.1, grouped by theme; known gaps. Cursor's pinned models are now stated to work.
- **Install docs.** The `v1.0.0-rc.1` version strings in `README.md` and `docs/install/install.md` become `v1.0.0-rc.2`, on this change's branch.
- **Evidence** under `docs/release/v1.0.0-rc.2/`.

## Out of scope

- Human-testing any harness before publication, and the public-install check rc.1 ran.
- Product source changes inside the release freeze; a defect gets its own change and a new candidate.
- Full OpenCode support or an OpenCode section in the upgrade guide (0513); Codex.
- Notarization, Homebrew, Windows.
- Stable `v1.0.0`.
- Docs beyond the install-doc version strings.
