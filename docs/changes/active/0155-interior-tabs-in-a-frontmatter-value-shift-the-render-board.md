---
id: 155
slug: interior-tabs-in-a-frontmatter-value-shift-the-render-board
title: Interior TABs in a frontmatter value shift the render-board sort feeder's fields
status: proposed
priority: medium
type: fix
created: 2026-07-28
updated: 2026-07-28
depends_on: []
related: []
discovered_from: [143]
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

`render-board.sh` joins its sort-feeder fields with TAB and reads them back with `IFS=$'\t'`.
Change 0143 addresses one half of that fragility — an EMPTY field collapsing, because TAB is
IFS-whitespace — but the reciprocal hazard is untouched: a frontmatter VALUE that itself contains
an interior TAB splits into extra fields on read-back, shifting every later field right.

The failure is the mirror image of 0143's: 0143 loses a field and shifts left, this loses nothing
and shifts right. Both defeat the renderer's downstream guards, and both are reachable by hand-editing
a manifest — `title:` is the obvious carrier, since it is free prose a human types.

Surfaced during 0143's design as explicitly out of scope: 0143 fixes the separator collapse for the
archive feeder's own fields, and widening it to sanitize arbitrary field CONTENT is a different
decision with a different blast radius.

## Scope

Decide whether interior control characters in a frontmatter value should be rejected at write time,
sanitized at read time, or made structurally harmless by 0143's separator change — then pin the
choice with a test. Note `mint-stub.sh` already rejects control characters in `--title`/`--slug`/`--type`
for exactly this class of reason; the gap is hand-edited manifests, which no script guards.

## Out of scope

- Change 0143's empty-field collapse, which is its own change and should land first.
- A general frontmatter schema validator.
