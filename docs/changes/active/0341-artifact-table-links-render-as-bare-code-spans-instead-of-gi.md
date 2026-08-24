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
spec: docs/superpowers/specs/2026-08-24-artifact-table-links-render-as-bare-code-spans-instead-of-gi-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-24-artifact-table-links-render-as-bare-code-spans-instead-of-gi-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-24-artifact-table-links-render-as-bare-code-spans-instead-of-gi-design.md) |
<!-- docket:artifacts:end -->

## Why

The generated `## Artifacts` link block on change files — and the reciprocal `docket:backlink` block on specs/plans/results, and the artifact/PR links in PR bodies — renders artifact paths as bare inline code spans (`` `docs/...` ``) instead of clickable `https://github.com/.../blob/...` links. Affected files are all recent: at survey time, 2 active and 9 archived change files (e.g. 0317, 0342, 0339, 0340, 0251, 0330, 0335–0338).

Root cause (code-proven, and it supersedes the stub's original cwd-based theory): rendering has been ported to the Go runtime, and the Go **app layer** constructs its link context (`render.LinkContext`) at every lifecycle call site — create, groom, claim, attach, implemented, reconcile, kill, reclaim, finalize close-out, ADR ops, PR publish — setting only the metadata branch and **never the repository web URL**. Nothing in the Go code derives that URL from the origin remote (the one `remote get-url origin` call it makes is used only to check a remote is configured, then discards the URL). The pure Go render layer faithfully emits a bare code span whenever the web URL is empty — so every block the `docket` **binary** renders comes out unlinked, unconditionally. The "good" files were last rendered by the legacy bash renderers (`render-change-links.sh` / `render-artifact-backlink.sh`), which still derive origin correctly and are what the grooming skills and the `docket.sh` facade invoke; the per-file "backlink links but artifact table doesn't" mismatch is just a coincidence of which runtime ran that file's *last* render, not a detection difference.

Discovered while generating artifacts for change 0339.

## What changes

Derive the repository web URL once in the Go app layer (a new remote-URL getter plus a pure GitHub URL parser matching the bash renderers' accepted forms) and thread it through a single shared link-context constructor, so all ~18 call sites are fixed at once and none can silently omit it again. This fixes the artifact table, the backlinks, and PR-body links uniformly. Then heal the already-broken files with a one-time re-render sweep — split across the two branches the artifacts live on: change files and specs on the metadata branch in one commit, and merged plan/results backlinks (which live on the integration branch) re-stamped via this change's own feature-branch PR. Finally, add a mutation-tested regression guard asserting that, given a GitHub origin, rendered artifact output carries blob URLs rather than code spans. Detailed design, alternatives, and the assumptions ledger are in the linked spec.

## Out of scope

Redesigning the `## Artifacts` block format or its columns; changes to the legacy bash renderers (already correct); other renderers (board, ADR index, learnings index) beyond sharing the same link-context fix; non-GitHub remote link styles (the bare-path fallback stays).
