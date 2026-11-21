---
id: 594
slug: 'export-as-pdf-alongside-markdown-docket-export-format'
title: 'Export as PDF alongside Markdown: docket export --format'
status: 'proposed'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-11-21'
updated: '2026-11-21'
depends_on: []
stacked_on:
related: [589, 561]
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-11-21-export-as-pdf-alongside-markdown-docket-export-format-design.md'
plan:
results:
auto_groomable:
hold:
branch_prefix:
branch:
pr:
ticket_provider:
ticket_id:
ticket_url:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-11-21-export-as-pdf-alongside-markdown-docket-export-format-design.md](../../superpowers/specs/2026-11-21-export-as-pdf-alongside-markdown-docket-export-format-design.md) |
<!-- docket:artifacts:end -->

## Why

Exported changes, specs, plans, and results files get shared with colleagues on private-repository projects, and colleagues read a PDF more readily than a Markdown file. Today `docket export` writes only Markdown, and docket has no way to produce a PDF.

## What changes

- `docket export` gains a `--format` flag choosing which files an export writes: `markdown`, `pdf`, or both. The default is both.
- A new pure-Go PDF renderer turns the export's clean Markdown into a plain, readable PDF, embedding its own fonts so dashes, arrows, symbols, and most emoji render.
- The PDF holds exactly what the Markdown copy holds. When the export includes a PDF, the PDF opens; otherwise the Markdown does. If the PDF fails, the Markdown is still written and the command reports an error.
- The JSON result lists every file written. The `docket-export` agent skill keeps exporting Markdown only.

## Acceptance criteria

- `docket export spec <id>` with no `--format` writes both a `.md` and a `.pdf` with the same name and opens the PDF.
- `--format markdown` writes and opens only the Markdown; `--format pdf` writes and opens only the PDF.
- Every document in the frozen export corpus renders to a PDF without error.
- A PDF failure leaves the Markdown copy written and exits with an error.

## Out of scope

- Removing docket mentions, `#N` numbers, or ADR numbers from the PDF: it matches the Markdown copy exactly, in private repositories too.
- PDF output from the `docket-export` agent skill.
- Colour emoji, syntax highlighting, clickable links, a table of contents, and CJK text.
- A configuration key for the default format.
