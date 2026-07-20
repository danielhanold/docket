---
id: 104
slug: guard-tab-in-frontmatter-values-feeding-the-digest-s-tab-joi
title: Guard TAB in frontmatter values feeding the digest's TAB-joined sort rows
status: proposed
priority: medium
created: 2026-07-19
updated: 2026-07-19
depends_on: []
related: []
discovered_from: [94]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

The `ready` line emitted by `render-board.sh --format digest` is now **machine-parsed** — change
0094 made it the selection channel for `docket-implement-next`, with a documented
`^ready( [0-9]+)*$` grammar and an exit-status contract layered on top of it.

The sort rows behind that line are **TAB-joined**. A tab character inside `created:` therefore
shifts the field split and can emit a non-numeric token into the `ready` line, violating the
grammar the consumer now relies on. The `change`-line loop has the same shape via `slug`.

This is **pre-existing exposure**, deliberately left alone in 0094 — but 0094 raised the stakes by
making the line machine-parsed rather than human-read report output.

## What changes

Sanitize (or reject) TAB in the frontmatter values that feed TAB-joined sort rows — at minimum
`created:` and `slug:` — so a malformed value cannot produce an output line that violates the
documented grammar. Prefer a shape-keyed guard over an enumerated field list.

## Out of scope

Broader frontmatter validation unrelated to the digest's output grammar.

## Open questions

- Sanitize at read time (frontmatter helper) or at render time (the row builder)?
- Reject loudly vs. silently strip — a rejected change would need somewhere to surface.

## Auto-groom blocked

**2026-07-19** — `docket-auto-groom` designed this stub, failed its critic gate after the one
permitted revision round, and abstained. No spec emitted. The design work is summarized here so a
human is not starting over.

### The filed premise is falsified — verified, not argued

The `created:` → `ready`-line trigger **does not reproduce**. Change 0094 already closed it twice
over: `created:` is shape-checked before entering the sort row (`render-board.sh:180-183`; a `case`
glob matches the whole string, so a TAB'd value fails it and becomes the `9999-99-99` sentinel), and
`id` comes from `int_field` (`^[0-9]+$`). A 322-case fuzz (14 keys × 23 hostile values, each placed
first and last in frontmatter) produced **zero** violations of `^ready( [0-9]+)*$`. `field()` also
truncates at the first newline, so no value can fabricate an extra `ready` line.

**The `slug:` half is real.** A TAB — or, more likely, a plain SPACE — in `slug:` is emitted raw
into the space-joined `change` line: `change 4 proposed build-ready delta<TAB>EVIL`.

### What a human needs to decide

1. **Is the residual work worth a PR on its own?** With the `created:` half already done, the
   in-scope remainder is slug sanitation in one line of the digest. That is a much smaller change
   than what was filed, and it competes for priority with the discovered work below.
2. **Re-scope, narrow, or supersede?** The stub's `## Out of scope` line ("broader frontmatter
   validation unrelated to the digest's output grammar") excludes the three defects found below,
   which are individually more severe than what remains in scope. Re-scoping 0104 onto them, or
   killing 0104 in favor of a fresh change, are both defensible — and both are backlog-composition
   calls reserved to you.

### Where the design got to (adopt or discard)

- **Guard site.** `scripts/board-checks.sh` — the existing warn-only frontmatter-validation channel
  (findings are `<check-id>\t<change-id>\t<message>`, surfaced by `docket-status` through a generic
  passthrough at `docket-status.sh:623-630`, so a new check-id needs **zero** extra wiring). Rejected:
  changing `field()` (66 call sites across 13 scripts inherit the contract) and renderer-stderr
  warnings (outside the closed report-line vocabulary callers key on — nothing would ever parse them).
- **Posture.** Warn-only; never a non-zero exit from `render-board.sh`, which sits on the must-land
  Board pass. One bad change file must not be able to take the board out.
- **Unresolved after the revision round (why this abstained).** The shape key was specified as
  `*[[:cntrl:]]*`, which **excludes space (0x20)** — so it would not fire on the space case that is
  this change's own primary trigger. The proposed fallback (derive `slug` from the filename) has the
  same hole: `mint-stub.sh` guards `--slug` against control characters only, so `0004-delta EVIL.md`
  is reachable and the fallback reintroduces the defect. The critic's supplied fix — key on a
  *positive* slug grammar matching `slugify`'s own alphabet (`[a-z0-9-]`, `mint-stub.sh:88-91`),
  apply the identical check to the derived fallback, and terminate on the padded id — is untested and
  arrived after the revision budget was spent.
- **Registration points a builder must not miss:** `board-checks.sh` header block,
  `scripts/board-checks.md`, and the closed check-id enumeration at `scripts/docket-status.md:344`.
  Note the header block at `board-checks.sh:11-12` is already stale (it omits `malformed-id`).

### Discovered work — recommend filing as its own change, `high`, `discovered_from: [104]`

*A frontmatter value outside its field's domain silently deletes a change from every board surface.*
All three reproduce against the current renderer:

| | Trigger | Effect |
|---|---|---|
| B | `status: proposed  # awaiting X` | Change vanishes from the digest **and** `BOARD.md`. `SECTION["$st"]` buckets on the raw read; both renderers iterate a fixed seven-name list. No diagnostic exists on any channel. |
| C | `id: 4  # allocated by …` | Same total deletion. **Already detected** by `board-checks.sh`'s `malformed-id` check — so this half needs surfacing/eventing work, not a new check. |
| D | `title: T5 \| injected \| row` | Injects columns into the `BOARD.md` table row. |

For B and C, `render-board.sh:86` still counts the file in `total` while dropping its row, so the
board's count line and its tables silently disagree — a louder symptom than a missing row alone.

These were **not** minted as stubs: `docket-auto-groom` is never a mint site (the convention's
*Auto-capture* section), so they are reported here for you to file.
