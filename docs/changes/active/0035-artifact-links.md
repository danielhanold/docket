---
id: 35
slug: artifact-links
title: Artifact links — a generated link block at the top of every change
status: proposed
priority: medium
created: 2026-06-21
updated: 2026-06-21
depends_on: []
related: [1, 11, 22]
adrs: [7, 12]
spec: docs/superpowers/specs/2026-06-21-artifact-links-design.md
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Why

At every human review stop in the docket loop — spec review after grooming, the merge gate,
results review at close-out — the reviewer has to hand-find the artifacts the change refers
to. The change file names them only as bare frontmatter paths (`spec:`, `plan:`, `results:`,
`adrs:`, `pr:`), with no clickable links. Finding them is made worse because they do not live
on one branch: the spec and ADRs sit permanently on the `docket` metadata branch, while the
plan and results live on the feature branch during the build and on the integration branch
after merge (and the feature branch is deleted at finalize). A reviewer reading the change on
GitHub cannot click through to the very documents the review is about.

## What changes

Add a generated **`## Artifacts`** block at the top of every change body (first section, just
below the frontmatter, above `## Why`) that hyperlinks the spec, plan, results, ADRs, and PR
— each as an absolute GitHub blob URL pinned to the branch that artifact actually lives on,
so the link resolves wherever the change is viewed.

- A new deterministic renderer, `scripts/render-change-links.sh`, rebuilds a marker-bounded
  block from the change's frontmatter + resolved config. Frontmatter stays the single source
  of truth; the script is the sole writer of the block (no skill hand-edits it). This is the
  ADR-0012 script-vs-model boundary.
- Per-artifact ref: spec/ADRs → `docket` (stable); plan/results → the feature branch while
  building, re-pointed to the integration branch once the change is `done`; PR → its URL.
- Every skill that writes one of those fields calls the renderer afterward
  (`docket-new-change`, `docket-groom-next`, `docket-auto-groom`, `docket-implement-next`),
  and the `done` transition (`docket-finalize-change` + the `docket-status` sweep) re-renders
  so plan/results re-point after the feature branch is cleaned up.
- Rows appear only as each artifact is created (omit-until-set). Non-GitHub remotes degrade
  to bare code-formatted paths.
- `change-template.md` ships the empty marker block as the first body section.

Full design — block format, the per-artifact ref table, the renderer contract, all call
sites, edge cases (kill, trivial, offline), and the testing approach — is in the linked spec.

## Out of scope

- A one-time back-fill pass stamping the block onto existing active changes (it appears
  naturally on the next field write; a bulk pass is a separate follow-up if wanted).
- Linking the change-file-on-integration location (deliberately excluded).
- Changing BOARD.md's own link rendering.
- URL schemes beyond GitHub + bare-path fallback.

## Open questions

None outstanding — scope, link form, render mechanism, per-artifact ref lifecycle, and
placement are all resolved in the spec. Two presentation calls (table layout, omit-until-set
rows) are recorded there and open to revision at build time.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
