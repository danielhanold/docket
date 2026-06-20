---
id: 32
slug: frontmatter-id-validation
title: Validate numeric id across the frontmatter script family
status: in-progress
priority: low
created: 2026-06-20
updated: 2026-06-20
depends_on: [30]
related: [30, 22, 23]
adrs: [12]
spec: docs/superpowers/specs/2026-06-20-frontmatter-id-validation-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: feat/frontmatter-id-validation
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

_Resolved at grooming (see spec) — behaviour split **by role**._

- **Fail-closed vs warn-and-skip?** → Per role: renderers + the shared `resolve_deps`
  scan **skip** the bad row (so one bad file never blanks the board/index);
  validators **flag** it; `terminal-publish.sh` (whose id is a CLI arg, not
  frontmatter) **fails closed** on a non-integer `--id`/`--adr`.
- **First-class check vs guard?** → A first-class warn-only `malformed-id` finding in
  `board-checks.sh` / `adr-checks.sh` — a validator that silently skipped a malformed
  id would hide the exact inconsistency it exists to report. Guard-only in renderers.
- **Subsumes existing ad-hoc id handling?** → No removal: the existing
  `[ -n "$id" ] || continue` guards remain (now fed by a shared `int_field` helper in
  `scripts/lib/docket-frontmatter.sh`); `pad`/arithmetic become guaranteed-integer
  downstream of the guard.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
