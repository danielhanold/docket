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
plan: docs/superpowers/plans/2026-06-20-frontmatter-id-validation.md
results:
trivial: false
auto_groomable: true
branch: feat/frontmatter-id-validation
pr:
blocked_by:
reconciled: true
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

- **2026-06-20** — Spec authored earlier this same session; `origin/main` unchanged
  (`ad799b1`) since. Reconcile confirms the design holds and pins the **complete
  id-read inventory** (the spec's enumeration was illustrative + told the builder to
  `grep` every site — this makes the full list concrete):
  - `lib/docket-frontmatter.sh`: **3** reads — `resolve_deps` pass 1 (L43) + pass 2
    (L48), **and `readiness()` (L71)** (the spec named only the two `resolve_deps`
    passes — `readiness` is the third; harden it too).
  - `render-board.sh`: **3** — SECTION builder (L52), done-id list (L164), archive
    builder (L182). (Matches the spec.)
  - `render-adr-index.sh`: **1** — scan (L38).
  - `board-checks.sh`: **2** — main scan (L50) **and cycle-detection `cid` (L92)**
    (spec named the scan; L92 is the second — harden it too).
  - `adr-checks.sh`: **1** — scan (L35); the at-risk arithmetic is `[ "$id" -gt "$MAXID" ]` (L42).
  - `terminal-publish.sh`: `--id` is a CLI arg (L29), padded at L79 — fail-closed guard, not a frontmatter read.
  - **No scope change** — same `int_field` helper + by-role adoption; the plan must
    simply cover all the sites above, including the two the spec under-enumerated
    (`readiness` L71, `board-checks` L92).
