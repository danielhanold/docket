---
id: 561
slug: 'export-an-opened-artifact-as-a-clean-docket-free-markdown-fi'
title: 'Export an opened artifact as a clean, docket-free Markdown file'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-10-17'
updated: '2026-10-20'
depends_on: []
stacked_on:
related: [559, 542, 573, 532]
discovered_from: [559]
adrs: []
spec: 'docs/superpowers/specs/2026-10-20-export-an-opened-artifact-as-a-clean-docket-free-markdown-fi-design.md'
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
| Artifact | Link |
|---|---|
| Spec | [2026-10-20-export-an-opened-artifact-as-a-clean-docket-free-markdown-fi-design.md](../../superpowers/specs/2026-10-20-export-an-opened-artifact-as-a-clean-docket-free-markdown-fi-design.md) |
<!-- docket:artifacts:end -->

## Why

Results files and other docket artifacts — most often a spec — are worth sharing with teammates who do not all use docket, as a Markdown file sent by email or Teams. As stored, they carry docket machinery an outside reader cannot use: frontmatter, generated backlink and artifact blocks with their markers, the spec's change stamp, links into files the reader cannot open, and change references such as `change 0559`. Today the sender deletes these by hand.

## What changes

Add an `--export` flag to `docket open` for a change, its spec, its plan, and its results. It writes a clean Markdown copy — by default to `~/Downloads` on macOS or the home folder on Linux, with `--output-path` to override — named `<date>-<type>-<name>.md` (type `issue`, `spec`, `plan`, or `results`), and opens that copy. The copy:

- removes frontmatter, managed blocks and their markers, and the spec's change stamp; a change exported as an issue keeps only its proposal sections under an H1 of its title;
- removes every link and keeps its text;
- turns explicit change references (`change 0559`, `the 0111 guard`) into the change's title in quotes, and leaves bare `#N`, PR, and ADR references as written;
- comes out with one H1 and well-formed headings.

A self-check of the output warns — never blocks — about docket leftovers and lists mentions of docket; `--warnings all|leftovers|mentions` picks which are shown. Every conversion is deterministic, for a person running the command in a shell.

## Out of scope

The board. Windows (docket ships only macOS and Linux). Other export formats (.docx, PDF). Any change to how artifacts are stored. Rewriting prose beyond the change-reference conversions.
