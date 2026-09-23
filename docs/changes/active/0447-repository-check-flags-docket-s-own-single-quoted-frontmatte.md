---
id: 447
slug: 'repository-check-flags-docket-s-own-single-quoted-frontmatte'
title: 'repository check flags docket''s own single-quoted frontmatter as needing manual review'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-09-23'
updated: '2026-09-23'
depends_on: []
stacked_on:
related: [352, 191, 266]
discovered_from: [446]
adrs: [71]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0071](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0071-writer-guarantees-yaml-validity-by-construction.md) |
<!-- docket:artifacts:end -->

## Why

`docket repository check` reports 801 `frontmatter-manual-review` errors, and the count grows by about 3 with every new change record. They're false positives against docket's own writer output, and no command clears them: every finding is `repairable: false`, and the remedy says to edit the frontmatter by hand. A hand edit can't help, though, because the writer re-quotes the value by design (ADR-0071).

The cause is in `planQuote` (`internal/reposetup/repair.go`, from change 0352's repair roster). The canonical writer single-quotes string fields such as `slug: 'my-slug'`, `title: '…'` and `type: 'fix'`. The checker then:

1. Tests the raw token. Its first byte, `'`, is a YAML indicator, so `unsafeScalarShape` returns true.
2. Calls `decodesToStringLiteral`, which requires the decoded value to equal the raw token. `'fix'` decodes to `fix`, which isn't equal, so the value is classed as "ambiguous".

So any correctly quoted string field reports as needing manual review. A newly created record like 0446 gets 3 findings (`slug`, `title`, `type`) with no hand edits at all. The noise buries real findings at `error` severity, and it has already been treated as ignorable "corpus noise".

## What changes

- Treat an already-quoted scalar (single- or double-quoted) that decodes cleanly to a string as safe. It's a well-formed string, not an ambiguous unsafe plain scalar.
- Keep reporting the real cases: plain (unquoted) tokens whose decoded value isn't the literal string, such as a bare `yes`/`no` or an unquoted `: `.
- Add a regression test showing that a record written by `change.create` produces zero `frontmatter-manual-review` findings. Mutation-check it by reverting the fix and confirming the test goes red.
- Confirm the corpus-wide count drops to only real manual-review cases, if there are any.

## Out of scope

- The other large findings families (`artifact-links-stale`, `drop-terminal-claimed-at`). Those are separate issues with their own repair paths.
- Changing the writer's quoting policy (ADR-0071).
- Hand-editing existing records.
