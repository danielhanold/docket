---
id: 542
slug: 'open-the-board-a-change-and-its-artifacts-from-the-terminal'
title: 'Open the board, a change, and its artifacts from the terminal'
status: 'in-progress'
priority: 'medium'
type: 'feat'
created: '2026-10-08'
updated: '2026-10-08'
depends_on: []
stacked_on:
related: [136, 531, 534]
discovered_from: []
adrs: [19, 141, 142]
spec: 'docs/superpowers/specs/2026-10-08-open-the-board-a-change-and-its-artifacts-from-the-terminal-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/open-the-board-a-change-and-its-artifacts-from-the-terminal'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-10-08T10:16:01Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-08-open-the-board-a-change-and-its-artifacts-from-the-terminal-design.md](../../superpowers/specs/2026-10-08-open-the-board-a-change-and-its-artifacts-from-the-terminal-design.md) |
| ADRs | [ADR-0019](../../adrs/0019-global-config-fence-classification.md), [ADR-0141](../../adrs/0141-build-artifacts-plan-results-evidence-live-on-the-metadata-b.md), [ADR-0142](../../adrs/0142-private-visibility-keeps-the-single-metadata-layout-and-vari.md) |
<!-- docket:artifacts:end -->

## Why

Getting from the terminal to the board, or to a change's spec, plan, results, or PR, takes several clicks today: open GitHub, switch to the `docket` branch, find the record under `active/` or `archive/`, then follow its Artifacts table. A private repository has no web page for any of it; the files sit in a checkout under `~/.local/share/dckt/…`, which is even harder to reach.

## What changes

- One new command, `docket open <what> [id]`, where `<what>` is `board`, `change`, `spec`, `plan`, `results`, or `pr`.
- With no id, inside a checkout whose branch belongs to a change (a `.worktrees/<slug>` worktree, or the main checkout on a feature branch), it opens that change's artifact.
- Shared repositories open GitHub pages in the browser by default. A new ordinary layered config key, `open.artifacts: github | local`, lets a user open the file from the local metadata checkout in their default Markdown app instead.
- Private repositories always open the local file. `pr` always opens the PR's URL, in both modes.
- `--print` prints the URL or path without opening anything; `--json` returns the protocol-v1 envelope.
- Missing artifacts, unknown ids, and unmatched branches fail with one clear line. A stale local checkout is synced first, and opened with a note if it can't be.

## Out of scope

- Web links for hosts other than GitHub (they fall back to the local file).
- Rendering Markdown to HTML or serving a local preview site.
- Opening ADRs, learnings, or the integration-branch copy of a spec.
- Shell completion for change ids.
