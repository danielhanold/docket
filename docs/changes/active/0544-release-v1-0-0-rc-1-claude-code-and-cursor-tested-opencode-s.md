---
id: 544
slug: 'release-v1-0-0-rc-1-claude-code-and-cursor-tested-opencode-s'
title: 'Release v1.0.0-rc.1: Claude Code and Cursor tested, OpenCode shipped untested'
status: 'in-progress'
priority: 'high'
type: 'chore'
created: '2026-10-08'
updated: '2026-10-08'
depends_on: [512]
stacked_on:
related: [366, 511, 512, 513, 514, 543, 545]
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-10-08-release-v1-0-0-rc-1-claude-code-and-cursor-tested-opencode-s-design.md'
plan:
results:
trivial: false
auto_groomable: false
branch_prefix:
branch: 'chore/release-v1-0-0-rc-1-claude-code-and-cursor-tested-opencode-s'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-10-08T21:10:03Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-08-release-v1-0-0-rc-1-claude-code-and-cursor-tested-opencode-s-design.md](../../superpowers/specs/2026-10-08-release-v1-0-0-rc-1-claude-code-and-cursor-tested-opencode-s-design.md) |
<!-- docket:artifacts:end -->

## Why

The Go pre-releases have grown one harness at a time: Claude Code in `v1.0.0-alpha.1` (change 0366) and Cursor in `v1.0.0-alpha.2` (change 0512). The next step is a release candidate for `v1.0.0`.

On 2026-10-08 the human decided that OpenCode will not be human-tested before `v1.0.0`. Full OpenCode support moves to after `v1.0.0` (change 0513, deferred). OpenCode should still work, so the release candidate keeps shipping its files.

Claude Code and Cursor were each just proven end to end by hand, so the human decided the release candidate is not re-tested by hand: the candidate's full test suite covers it, and the release notes say so.

## What changes

- **Release.** Package, verify and publish `v1.0.0-rc.1` with the alpha.2 protocol (0512), minus the human harness test. rc.1 is a **full release, not a pre-release**, and becomes "Latest", replacing the Bash `v0.9.3`.
- **No human harness test.** Claude Code (alpha.1) and Cursor (alpha.2) count as tested; the full suite gates the candidate. Source changes that land before the cut ship too, and the notes list them.
- **Public install check** for both Claude Code and Cursor, plus proof that the installed macOS binary carries no quarantine flag and a valid signature.
- **Install docs.** `README.md`, `docs/install/install.md` and `docs/install/keeping-current.md` describe installing from the release, state the harness status, and warn against browser downloads. Edited on this change's branch; they reach `main` when its PR merges.
- **Harness status in the release notes and docs:**
  - Claude Code and Cursor: tested.
  - OpenCode: **untested**, not unsupported. The OpenCode files still ship and install as they do today.
  - Codex: the only officially **unsupported** harness.
- **Upgrade path.** The release notes link the Bash upgrade guide (0511, with the Cursor section from 0543). The guide has no OpenCode section yet; the notes say so.
- **macOS note.** The binaries are not notarized. The install script never triggers Gatekeeper; the notes and install docs warn that a browser-downloaded archive will be blocked.

## Out of scope

- Human-testing Claude Code, Cursor or OpenCode for this release.
- An OpenCode section in the upgrade guide, or full OpenCode support (change 0513, after `v1.0.0`).
- Removing or disabling OpenCode files from the release.
- Codex support.
- Notarizing or Developer ID signing the binaries; Homebrew; Windows.
- Stable `v1.0.0` itself.
- Docs beyond the install pages listed above.
- Product source changes of any kind inside the release freeze; a defect gets its own change and a new candidate.
