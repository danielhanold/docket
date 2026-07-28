---
id: 143
slug: empty-id-collapses-the-archive-sort-feeder-s-tab-joined-fiel
title: Empty id collapses the archive sort feeder's TAB-joined fields in render-board.sh
status: proposed
priority: medium
type: fix
created: 2026-07-27
updated: 2026-07-28
depends_on: []
related: [115]
discovered_from: [115]
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

`render-board.sh`'s archive sort feeder joins fields with TAB and reads them back with `IFS=$'\t'`. Because TAB is IFS-whitespace, an empty `id` field collapses rather than being preserved as empty, shifting every later field left. That defeats the renderer's own id guard and emits a corrupt archive row of the shape `| [0000](archive/) |  | <date> |`.

The active side is unaffected — its guard runs before the join.

Found during change 0115 (extending the `board-row-dropped` invariant to `archive/` files) and deliberately left unfixed there: `render-board.sh` is the oracle that 0115's check is validated against, so fixing both in one commit would have destroyed the independence the backstop depends on.

## Scope

Fix the field-shift in the archive sort feeder so an empty `id` cannot collapse, and pin it with a test. Keep the fix separate from `board-checks.sh` so the check and its oracle stay independent.

## Auto-groom blocked

**2026-07-28** — autonomous grooming abstained. A draft spec was written and attacked by the
adversarial critic; the critic reproduced the bug against the real script and confirmed the core
diagnosis, but two decisions in the design turned out to be **cross-change ownership questions**
that the drain must not settle on its own.

### The undecidable decisions

**1. Who owns the header/table divergence — the renderer (0143) or the check (0115)?**
`render-board.sh`'s `ARC_COUNT` loop (line ~154) keys on `status` alone and never reads `id`, so an
archive file with an empty `id:` and `status: done` is **counted in the `Archive — done (N)` header**
even after the feeder drops its row. Measured on a 5-file fixture with the proposed fix applied:
header `done (5)` above **4** rows — the fix converts a corrupt row into a silent count mismatch.

Guarding the `ARC_COUNT` loop on `id` too would close it. But change 0115's spec
(`docs/superpowers/specs/2026-07-20-archive-side-row-dropped-invariant-design.md`) names *exactly
this state* as its case **(B)** — "`ARC_COUNT` keys on status alone, so the file **is** counted in
the summary, while … its row [is] drop[ped]. The summary promises a row that never renders" — i.e.
0115 **intends** this to be surfaced by `board-row-dropped` rather than repaired in the renderer.
Fixing it in 0143 would remove the state 0115's check exists to report. The two changes' division of
labor is genuinely contested and neither spec can resolve it unilaterally.

**2. Which of 0143 and 0115 must land first — and why?**
The stub asserts the fix must come *after* 0115 so the check keeps an unfixed oracle. The critic
showed the rationale is inverted: 0115's `renders_row` predicate hoists `[ -n "$ID" ] || return 1`
above the directory switch, and its own truth table asserts, for the archive arm, *empty id + `done`
→ not in a table, dropped*. That is **not** today's renderer (which emits the corrupt `[0000]` row) —
0115's archive predicate already models the **post-0143** renderer. So 0143 landing first would make
0115's predicate correct, and 0115 landing first ships a check whose archive arm mis-describes the
tip it was validated against. The ordering may still be right for merge hygiene, but the stated
reason is false and the real ordering constraint is unknown to the drain.

### What a human should supply

- A ruling on (1): does the empty-`id`-counted-in-header state get **fixed** in `render-board.sh`, or
  is it **preserved as a reportable finding** for 0115's `board-row-dropped`? The answer determines
  whether the `ARC_COUNT` loop is touched at all, and it rewrites 0115's case (B) if the answer is
  "fix".
- A ruling on (2): the intended merge order of 0143 and 0115, with the real reason, so the
  dependency (or its absence) is recorded honestly.

### Findings that survived and should be reused when this is re-armed

- The diagnosis is confirmed by reproduction: `IFS=$'\t'` treats TAB as IFS-whitespace, so an empty
  `id` collapses and shifts every later field; the guard `[ -n "$id" ]` sits downstream of the lossy
  join and sees `id="done"`. Output: `| [0000](archive/) |  | <date> |` plus
  `printf: done: invalid number` and `sed: : No such file or directory` on stderr.
- The active side is genuinely unaffected (its guard, line 139, precedes the join). Verified.
- `status` has the same collapse twin; an empty `status` additionally raises `ARC_COUNT: bad array
  subscript` and is silently uncounted.
- A third consequence: the corrupt row's `st` holds a path, not `done`, so it escapes `done_seen` and
  widens the `ARCHIVE_RECENT` verbatim window by one per corrupt file.
- **Interior** TABs in a frontmatter value (`status: done<TAB>X`) shift fields identically before and
  after any empty-field fix — a separate hazard, already handled in `board-checks.sh` by
  `sanitize()` (change 0104). Out of scope, worth its own stub.
- Test anchoring: assert the generic corrupt shape `](archive/) ` (archive link with empty basename),
  **not** the literal `[0000]` — the empty-`status` sibling renders `| [0005](archive/) |  | … |`, so
  a `[0000]`-anchored assert is a no-op for it.
- The delimiter must stay TAB: `sort -t$'\t'` appears at five sites in `render-board.sh`
  (lines 145, 202, 244, 352, 404), so a `\x1f` swap is a whole-file refactor.

### Recommendation

Keep it — the bug is real and reproduced. It is a small fix once the ownership question is settled.
Recommend settling (1) first, since it decides both the patch's shape and whether 0115's spec needs
an amendment.
