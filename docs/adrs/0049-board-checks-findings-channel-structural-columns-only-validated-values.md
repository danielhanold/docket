---
id: 49
slug: board-checks-findings-channel-structural-columns-only-validated-values
title: board-checks.sh findings channel — structural columns carry only script-derived or shape-validated values
status: Accepted
date: 2026-07-20
supersedes: []
reverses: []
relates_to: [12]
change: 104
---

## Context

`scripts/board-checks.sh` emits findings as `<check-id>\t<change-id>\t<message>`, and
`docket-status.sh` reads them back with `IFS=$'\t' read -r check_id change_id message`. The
pre-existing `malformed-id` check placed the *raw* `id:` frontmatter value in the change-id
column, so an interior TAB (`id: 4<TAB>EVIL`) shifted the message into the wrong field — the
health checker's own reporting channel was injectable by exactly the input class it exists to
catch. `field()` truncates at the first newline and strips trailing whitespace, but an interior
TAB survives.

Change 0104's spec prescribed the fix as a blanket rule: the change-id column "uses the
filename-derived padded id, falling back to `?`". Applied to every check, that would have changed
`broken-spec` / `dep-cycle` / `stale-in-progress` / `merge-gate-stall` / … from `2` to `0002` — a
silent break of the report format that roughly 15 existing test asserts depend on, and that the
spec never argued for. The spec's stated *rationale* was narrower: the column must never carry a
value that can shift a field.

## Decision

Three parts:

1. `emit` sanitizes every embedded value unconditionally — TAB and CR are rendered as visible
   `\t` / `\r` escapes — via bash parameter expansion, not `sed` (BSD `sed` does not interpret
   `\t` in a pattern).
2. The filename-derived padded id (with `?` fallback) is used exactly where a raw frontmatter
   value would otherwise appear: `malformed-id`, and the drop finding for a file with no usable
   id.
3. Every check that has an `int_field`-validated id keeps emitting it verbatim and unpadded. A
   validated id matches `^[0-9]+$` and provably cannot shift a field.

The general rule for future check authors: **structural columns carry only script-derived or
shape-validated values; untrusted text belongs in the message column**, which is the last field
of the caller's `read` and therefore harmless.

## Consequences

Untrusted values remain visible for diagnosis (the raw value moved into the message, not out of
the report). The report format is unchanged for every pre-existing check. The change-id column is
now mixed-format — unpadded `71` from frontmatter, padded `0070` from a filename — which is
consistent within a file but inconsistent across a report. Two id-less files whose filenames yield
no id both key on `?` and collapse into one finding.

The message column is explicitly untrusted text: a consumer that greps the findings blob for an
in-band marker must anchor on the check-id column, not substring-match the whole blob. This was a
live defect — a forged `[reclaimable]` marker in a change's `title:` reached `docket-status.sh`'s
reclaim gate — fixed in the same change (0104) by scoping that consumer.

This decision is an instance of ADR-0012's script-vs-model boundary applied to a channel's own
wire format: the script (not the model) owns the structural guarantee that its own report can't be
corrupted by the very input class it validates.
