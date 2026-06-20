---
id: 32
slug: frontmatter-id-validation
title: Validate numeric id across the frontmatter script family
status: proposed
priority: low
created: 2026-06-20
updated: 2026-06-20
depends_on: [30]
related: [30, 22, 23]
adrs: [12]
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Why

Every deterministic script in the family — `render-board.sh`, `board-checks.sh`,
`terminal-publish.sh`, and (from change 0030) `render-adr-index.sh` /
`adr-checks.sh` — reads the `id:` frontmatter field and trusts it to be a
well-formed integer: it becomes a `declare -A` key and feeds integer arithmetic
(`MAXID`, the numbering-gap loop, `[ "$id" -gt … ]`). A malformed `id:` (empty,
non-numeric, padded oddly) would produce a junk array key or an arithmetic error
under `set -u`. In practice ids are always integers (allocated max+1, encoded in
the filename), so this never bites — but it is an unguarded, **codebase-wide**
assumption. Change 0030's review flagged it as a shared latent assumption, not a
0030 regression; hardening it belongs in one place, applied uniformly.

## What changes

Add a single shared `id`-validation helper to `scripts/lib/docket-frontmatter.sh`
(e.g. a guarded accessor that fails closed or skips with a clear diagnostic on a
non-integer `id:`) and adopt it across all family scripts so they behave
identically. The point is **uniformity**: hardening only some scripts would make
them stricter than their siblings and break the "consistent with the existing
pattern" property — so this must cover the whole family in one change.

## Out of scope

- Changing how ids are allocated or formatted.
- Any per-script behavioral change beyond rejecting/skipping a malformed `id:`.

## Open questions

- Fail-closed (exit non-zero) vs. warn-and-skip the malformed file? Likely
  per-script: renderers skip the bad row, validators emit a finding.
- Should a `malformed-id` finding be added to `board-checks.sh` / `adr-checks.sh`
  as a first-class check, or just a defensive guard?
- Does this subsume any existing ad-hoc id handling worth removing?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
