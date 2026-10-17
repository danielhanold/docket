---
id: 561
slug: 'export-an-opened-artifact-as-a-clean-docket-free-markdown-fi'
title: 'Export an opened artifact as a clean, docket-free Markdown file'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-10-17'
updated: '2026-10-17'
depends_on: []
stacked_on:
related: [559]
discovered_from: [559]
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

Results files and other docket artifacts are often worth sharing with people outside the repository, for example as a Markdown file sent by email or Teams. As stored, they carry docket machinery a reader outside docket cannot use: frontmatter, generated backlink and artifact blocks with their markers, links into the metadata branch, and change and ADR references such as `#559` or `ADR-0141`.

## What changes

Add an export for every file artifact `docket open` can open: the board, a change, and its spec, plan, and results (a PR is a URL and has nothing to export). The export writes a clean Markdown copy that:

- removes everything docket-specific: frontmatter, managed and backlink blocks and their markers, docket-internal links, and change and ADR references;
- formats the remaining Markdown cleanly: consistent headings, lists, and tables, and no leftover runs of blank lines.

To settle when grooming: the command's shape (a flag on `docket open` or a command of its own), where the exported file goes, and what a removed docket reference leaves behind in the surrounding sentence.

## Out of scope

Other export formats (.docx, PDF). Any change to how artifacts are stored.
