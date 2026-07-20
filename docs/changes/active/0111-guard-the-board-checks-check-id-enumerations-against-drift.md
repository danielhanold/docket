---
id: 111
slug: guard-the-board-checks-check-id-enumerations-against-drift
title: Guard the board-checks check-id enumerations against drift
status: proposed
priority: medium
created: 2026-07-20
updated: 2026-07-20
depends_on: []
related: []
discovered_from: [104]
adrs: []
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

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

The `board-checks.sh` check-id vocabulary is written down in **three** places and nothing keeps them
in correspondence. Change 0104's reconcile found **both** documentation enumerations already
drifted, in opposite directions, each undetected since the change that introduced the gap:

- `scripts/board-checks.sh:11-12` (the header block) omits `malformed-id`.
- `scripts/docket-status.md:344` (the closed `check <check-id>` enumeration) omits
  `stale-finalize-blocked` — change 0098 shipped that check-id without registering it there.

0104 repairs both instances as a by-product of adding its own check-ids, but it adds no guard, so
the drift recurs on the next check-id anyone ships. The failure is quiet by construction: a missing
registration costs nothing at runtime, the suite stays green, and the enumeration that callers are
told is *closed* silently is not.

This is the same defect class as changes 0107 and 0108 (README config-snippet / config-fence drift
guards), one layer over: a documented vocabulary asserted to be complete, with no test tying it to
the code that emits it.

## What changes

A correspondence guard over the check-id vocabulary, anchored on the **emitting code** rather than
on any hand-maintained list — derive the emitted set from `board-checks.sh`'s `emit <check-id>` call
sites, then assert it matches the header block and both contract enumerations.

The correspondence is a **mirror, not a subset**, so per the `correspondence-guard-runs-one-way`
learning it needs both directions and mutation proof in both: a check-id emitted but undocumented
must redden, and a documented check-id nothing emits (a phantom / removed check) must redden too.

Registration surfaces to cover:

- `scripts/board-checks.sh`'s header `check-id ∈ {…}` block
- `scripts/board-checks.md`'s per-check sections
- `scripts/docket-status.md`'s `check <check-id> <change-id> <message>` report-line row

## Out of scope

- Changing the check-ids themselves, or the findings format.
- The `docket-status` report-line vocabulary beyond the `check` row.
- Repairing the two drift instances 0104 already fixes — this is the guard that keeps them fixed.

## Open questions

- Is `emit <id>` a reliable enough anchor to derive the emitted set, or does the guard need a
  declared array in `board-checks.sh` that both the emitter and the guard read (mirroring how 0104
  single-sources the status vocabulary into `lib/docket-frontmatter.sh`)?
- Should `malformed-id`'s "guard/carve-out, not counted among the named checks" framing in
  `board-checks.md` stay an exception, or does the guard force it into the enumeration proper?
