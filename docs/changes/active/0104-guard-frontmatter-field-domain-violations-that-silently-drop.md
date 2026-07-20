---
id: 104
slug: guard-frontmatter-field-domain-violations-that-silently-drop
title: Guard frontmatter field-domain violations that silently drop board rows
status: proposed
priority: high
created: 2026-07-19
updated: 2026-07-20
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

**A frontmatter value that is well-formed text but outside its field's domain silently deletes the
change from every board surface.** No diagnostic fires on any channel. Reproduced against the
current renderer:

| | Trigger | Effect |
|---|---|---|
| A | `status: proposed  # awaiting X` | The change vanishes from the digest, the `ready` queue, and `BOARD.md`. `SECTION["$st"]` buckets on the raw read; both renderers iterate a fixed seven-name list, so an unrecognized bucket is never emitted. **No detection exists.** |
| B | `id: 4  # allocated by …` | Same total deletion. Already detected by `board-checks.sh`'s `malformed-id` (`board-checks.sh:74`) — this half needs surfacing/eventing, not a new check. |
| C | `title: T5 \| injected \| row` | Injects extra columns into the `BOARD.md` table row. |

For A and B, `render-board.sh:86` still counts the file in `total` while dropping its row, so the
board's count line and its tables **silently disagree** — a louder symptom than a missing row alone.
A three-change fixture with one poisoned `status:` renders `**3 changes** — 🟡 2 proposed` above two
rows, and the digest's `ready` queue drops the id entirely.

That last part is what makes this more than cosmetic: since change 0094 the `ready` line is the
**machine-parsed selection channel** for `docket-implement-next`. A change can therefore be quietly
removed from the autonomous build queue by a stray inline comment, with the board still reporting a
healthy-looking count.

## What changes

Detect field-domain violations and surface them, without letting one bad file take the board out:

- **Guard site:** `scripts/board-checks.sh`, the existing warn-only frontmatter-validation channel.
  Findings are `<check-id>\t<change-id>\t<message>`, surfaced by `docket-status` through a generic
  passthrough (`docket-status.sh:623-630`), so a new check-id needs **zero** extra wiring.
- **Posture:** warn-only. `render-board.sh` must never exit non-zero — it sits on the must-land
  Board pass.
- **Cover the count/rows disagreement** (A and B): a file counted in `total` but rendered in no
  section is itself a detectable invariant violation, and a cheaper key than enumerating field
  domains one by one.
- **Surface the already-detected half** (B) rather than adding a second overlapping check.

Folded in from this change's original scope — a **TAB or SPACE in `slug:`** is emitted raw into the
space-joined `change` line (`change 4 proposed build-ready delta<TAB>EVIL`). Key it on a *positive*
slug grammar matching `slugify`'s own alphabet (`[a-z0-9-]`, `mint-stub.sh:88-91`), apply the
identical check to any filename-derived fallback, and terminate on the padded id.

**Registration points a builder must not miss:** the `board-checks.sh` header block,
`scripts/board-checks.md`, and the closed check-id enumeration at `scripts/docket-status.md:344`.
The header block at `board-checks.sh:11-12` is already stale — it omits `malformed-id`.

## Out of scope

- Rejecting or rewriting the offending change files. This change makes the failure **visible**; it
  does not decide what a malformed file's canonical value should be.
- Broader frontmatter schema validation beyond what board rendering actually consumes.

## Open questions

- Does the count-vs-rows invariant subsume the per-field domain checks, or is it a backstop
  alongside them?
- Where does a warn-only finding need to be loud enough to stop an autonomous build — does
  `docket-implement-next` gate on it, or only report it?

## History

**2026-07-20 — re-scoped by hand.** This change was filed as "Guard TAB in frontmatter values
feeding the digest's TAB-joined sort rows". `docket-auto-groom` verified that premise **falsified**:
the `created:` → `ready`-line trigger does not reproduce, because 0094 already closed it twice over
(`created:` is shape-checked at `render-board.sh:180-183` and fails to the `9999-99-99` sentinel;
`id` comes from `int_field`). A 322-case fuzz produced zero violations of `^ready( [0-9]+)*$`, and
`field()` truncates at the first newline so no value can fabricate an extra `ready` line.

The `slug:` half was real but small, and the groomer's probe surfaced the far more severe defect
this change now targets. Re-scoping onto it — rather than killing 0104 and minting fresh — keeps the
verified findings and the settled guard-site design attached to the work they inform.

Rejected guard sites, recorded so they are not re-litigated: changing `field()` (66 call sites
across 13 scripts inherit the contract) and renderer-stderr warnings (outside the closed report-line
vocabulary callers key on — nothing would ever parse them).
