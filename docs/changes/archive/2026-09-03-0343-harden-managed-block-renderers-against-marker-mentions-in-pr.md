---
id: 343
slug: harden-managed-block-renderers-against-marker-mentions-in-pr
title: 'Harden managed-block renderers against marker mentions in prose/code (fence-aware block finder)'
status: 'killed'
priority: medium
type: fix
created: 2026-08-24
updated: '2026-09-03'
depends_on: []
stacked_on:
related: []
discovered_from: [341]
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

The managed-block renderers locate a `docket:artifacts` / `docket:backlink` block by a **fixed-string first-match** on the marker text, with no fence/prose awareness. Any artifact whose *body prose or a fenced code example* contains the marker string is corrupted when the block is re-rendered: the renderer grabs the first textual occurrence — a fenced example, an indented bullet, a shell `START_MARKER=` assignment, a Go test literal — instead of a genuine top-of-file/managed block, then overwrites the surrounding authored content.

This bit change 0341's own one-time re-render sweeps: three files that *document or test the backlink feature itself* were corrupted and had to be manually restored and excluded — `docs/superpowers/specs/2026-07-23-artifact-backlinks-design.md`, `docs/superpowers/plans/2026-07-24-artifact-backlinks.md`, and `docs/superpowers/plans/2026-08-22-finalize-backlink-leg-corpus-scoping.md`. The sweeps' marker-presence gate could not protect them, because the gate's own grep matches the prose mention too. Manual exclusion does not protect **future** re-renders: 0341 now routes every lifecycle re-render through the Go binary, so an unattended `claim`/`attach`/`finalize` on one of these files would silently corrupt it.

A second, related non-defensiveness surfaced in the same area: `scripts/render-board.sh`'s `pr_cell` mangles a non-URL, non-bare-number `pr:` value (the `github.com/owner/repo#N` shorthand) into a broken link `[#owner/repo#N](https://github.com/.../pull/owner/repo#N)`, where `render-change-links.sh` defensively renders a non-URL `pr:` verbatim (no broken link). 0341 corrected the offending `pr:` values to the full-URL convention, but the board renderer should fail safe on a non-conforming value rather than emit a broken URL.

## What changes

Anchor the block finder in **both** the bash renderers (`render-artifact-backlink.sh`, `render-change-links.sh`) and the Go `internal/render` block locator on a *genuine* managed block — a column-0 marker line that is outside any fenced code region (and, for the backlink block, at the top of the file) — rather than the first textual hit anywhere in the file. Confirm the Go port's behavior against the bash renderers (0341 proved byte-parity on a clean input; this needs the prose-mention case added). Make `render-board.sh`'s `pr_cell` fail safe on a non-URL / non-bare-number `pr:` (render verbatim, never a broken link), matching `render-change-links.sh`. Add regression guards using the three real files above as fixtures — re-rendering them must be a byte-identical no-op.

## Out of scope

The Go link-context web-URL derivation (shipped in 0341); redesigning the block format or markers; the one-time corpus heal (done in 0341).

## Why killed

Backlog review 2026-09-02 (Bash→Go migration): already fixed in Go — internal/document/markers.go and internal/render/section.go are fence-aware; the board PR cell only links http-prefixed values.
