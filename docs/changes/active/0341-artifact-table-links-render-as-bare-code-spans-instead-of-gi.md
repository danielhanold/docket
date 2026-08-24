---
id: 341
slug: 'artifact-table-links-render-as-bare-code-spans-instead-of-gi'
title: 'Artifact-table links render as bare code spans instead of GitHub links'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-08-24'
updated: '2026-08-24'
depends_on: []
stacked_on:
related: [35]
discovered_from: [339]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

The `## Artifacts` link block that `render-change-links.sh` stamps into every change file (and the same table shape in results files and PR bodies) renders the spec/plan/results paths as bare inline code spans (`` `docs/...` ``) rather than clickable links. A survey of recent active and archived changes shows this is systemic — nearly every change file is affected — so the paths are shown but none of the `a` tags work.

Root cause: the renderer emits a real markdown link `[name](https://github.com/OWNER/REPO/blob/<ref>/<path>)` only when its GitHub mode is on (`GITHUB=1`), which it derives from `git -C "$(dirname "$CHANGE_FILE")" remote get-url origin`. When that origin lookup returns empty at render time it silently falls back to `GITHUB=0` and emits code spans. Run against the change file in its real `.docket` worktree location the SAME renderer produces valid `https://github.com/...` links, so the committed code-span form proves the actual render call site (at mint / frontmatter-write time) invokes it from a context where the origin remote is not resolvable, and nothing re-renders it afterward. It is also inconsistent with the reciprocal backlink block (`render-artifact-backlink.sh`), whose own GitHub detection resolves correctly and DOES emit a proper link — the observed 'backlink works, artifact table does not' mismatch.

Discovered while generating artifacts for change 0339.

## What changes

Make the artifact-link renderer's GitHub-mode detection robust so it resolves the origin remote regardless of the caller's cwd or how the change-file path is passed (e.g. resolve the repo/worktree from the change file deterministically, or reuse the resolver the config/backlink path already uses, rather than `git -C dirname ... remote get-url`). Then add a one-time re-render sweep so existing change files pick up valid links. Confirm the fix against both the metadata-worktree render path and any mint-time path, and add a guard that fails if a rendered artifact row is a bare code span when origin is a GitHub remote.

## Out of scope

Redesigning the `## Artifacts` block format or its columns; changes to unrelated renderers (board, ADR index, learnings index) beyond the shared GitHub-detection fix; non-GitHub remote link styles.
