---
id: 580
slug: 'release-v1-0-0-stable-release-with-version-free-install-docs'
title: 'Release v1.0.0: stable release with version-free install docs'
status: 'proposed'
priority: 'high'
type: 'chore'
deliverable: 'pr'
created: '2026-10-27'
updated: '2026-10-27'
depends_on: []
stacked_on:
related: [579, 544, 513, 514]
discovered_from: []
adrs: []
spec:
plan:
results:
auto_groomable: false
hold:
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

`v1.0.0-rc.2` was published on 2026-10-23 (change 0579). Since then only rc.2's own evidence commits and the logo have landed on `main`, and the human has tested Claude Code and Cursor enough on the release candidates. It is time to ship stable `v1.0.0`.

The install docs pin a release version, so every release needs a doc edit, and that edit could only land after publishing. A stable release's notes link to the install guide, so the guide must be right the moment the release goes live.

## What changes

- **Version-free install docs, before the cut.** A small docs-only PR switches the install commands in `README.md`, `docs/install/install.md` and `docs/release/upgrading-from-bash.md` to GitHub's `releases/latest/download` form with no `--version`, and merges to `main` before the candidate is cut. No later release needs an install-doc edit.
- **Release.** Package, verify and publish `v1.0.0` from `main` with rc.2's protocol (0579): a full release that becomes "Latest", replacing rc.2. No harness test and no public-install check; the full test suite gates the candidate.
- **Release notes:** a one-paragraph overview of docket, a link to the install guide, a one-line harness status (Claude Code and Cursor supported, OpenCode untested, Codex unsupported), and a feature highlight list drawn from the docs index.
- **Evidence** under `docs/release/v1.0.0/`; the results record names #513 and #514 as now due.

## Out of scope

- Human-testing any harness before publication; a public-install check.
- Product source changes inside the release freeze; a defect gets its own change.
- Reviving or building #513 (full OpenCode support) or #514 (retiring the Bash upgrade test cases); Codex.
- Notarization, Homebrew, Windows.
- Docs beyond the install-command edits.
