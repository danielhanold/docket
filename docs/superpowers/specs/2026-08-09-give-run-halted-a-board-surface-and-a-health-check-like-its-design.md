<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0261 — Give '## Run halted' a board surface and a health check, like its two family members](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0261-give-run-halted-a-board-surface-and-a-health-check-like-its.md)**
<!-- docket:backlink:end -->

# Give `## Run halted` a board surface and a health check — design

Change: 0261. Discovered from 0237 (merged 2026-08-08), which introduced the `## Run halted`
presence-encoded body section and its `verify-run` reader. This change gives the section the same
derived-view surfaces its two family members already have.

## Design

### 1. Board cell — `run halted — needs you`

`scripts/render-board.sh` gains an `in_progress_cell()` in the exact shape of the existing
`implemented_cell()`: it prints `run halted — needs you` when the change file carries the
`## Run halted` section, else empty. The **In progress** table gains a trailing `Readiness`
column (header + row format updated together), mirroring how the `implemented` table carries its
`finalize blocked — needs you` cell. Cell vocabulary follows the family: lowercase marker name +
`— needs you`.

The **digest projection must move with the cell**: `render-board.sh`'s `digest_readiness()`
documents the invariant that the digest can never disagree with the board's Readiness cell (its
implemented arm exists because change 0087 added `finalize_blocked` readiness). Add an
in-progress arm emitting a `run-halted` token when the section is present, else `-`, in the exact
shape of the implemented arm. The golden in `tests/test_render_board.sh` pinning the current
six-column in-progress header/row is regenerated in the same commit.

### 2. Shared predicate — `run_halted()` in lib/docket-frontmatter.sh

One helper beside `finalize_blocked()`:

```bash
run_halted(){ # run_halted FILE  (only meaningful for an in-progress change)
  has_section "$1" "## Run halted"
}
```

Both render-board.sh and board-checks.sh call it — the whole predicate lives in one place
(learning: *duplicated-gate-copies-the-whole-predicate*). `has_section`'s whole-line `-x` match
is exactly what 0237's bare, undated heading contract requires.

### 3. Health check — `stale-run-halted` in board-checks.sh

A new check-id in the shape of `stale-finalize-blocked`, gated on `status: in-progress` and
`run_halted "$f"`, firing when the change file's last-commit timestamp (git `%ct`, same
tamper-proof source as the sibling) has outlived the same staleness horizon family
(`RUN_HALTED_STALE_SECS`, defaulted to the same value as `FINALIZE_BLOCKED_STALE_SECS`; a
separate constant so the two horizons can diverge later without a shared-constant hunt). Message
shape mirrors the sibling:

> `## Run halted marker set <N>h ago — read the section and resolve it; the marker clears only
> when docket-implement-next re-claims <id>, or it will sit on the board`

Advisory only: never mutates the file, never auto-clears the marker — removal stays owned by
`docket-implement-next` Step 2, per 0237.

### 4. The four-surface pin

Adding `stale-run-halted` means editing, in one change, all the places the `BOARD_CHECK_IDS`
contract names: the array in `lib/docket-frontmatter.sh`, `board-checks.sh`'s `--help` header
enumeration, a per-check section in `scripts/board-checks.md`, and the `check` report-line row in
`scripts/docket-status.md`. `tests/test_board_checks.sh` reads the runtime array, so a missed
surface goes red mechanically.

### 5. Tests

- Board renderer: an in-progress fixture carrying the bare `## Run halted` line renders the cell;
  one without it renders an empty cell; a dated heading variant (`## Run halted — 2026-…`) does
  NOT match (guards the `-x` whole-line contract).
- board-checks: marker + old commit timestamp fires `stale-run-halted`; marker + fresh timestamp
  is silent; no marker is silent; a non-in-progress status with the marker is silent.
- Mutation-test the new gate per the repo guard rule: strip the status gate and the horizon gate
  and watch the asserts redden.

## Out of scope (per the stub's boundary)

`verify-run`'s verdict vocabulary, the dispatch seam's exit contract, every existing
`aborted-run` leg and floor, and the marker's write/remove lifecycle (0237 owns it).

## Assumptions

1. **New check-id vs reusing `aborted-run`.** Chosen: new id `stale-run-halted`. Rejected:
   folding into `aborted-run` (the 0176 precedent for same-conclusion evidence) — a halt is a
   *deliberate* stop with a distinct remedy ("read the section; never re-dispatch") where
   `aborted-run` hedges ("verify it is not still building"); one id would force one remedy
   vocabulary over two opposite instructions. The stub itself asks for "a check-id for it".
2. **Staleness-gated vs immediate-presence check.** Chosen: staleness-gated, mirroring
   `stale-finalize-blocked` — the closest sibling (needs-you board cell + check on the same
   marker). The new board cell already gives immediate visibility, and `verify-run` reports
   `run-halted` at the dispatch seam, so an immediate-presence check would restate two existing
   surfaces at every status pass. Rejected: immediate presence (the `publish-deferred` shape) —
   that marker has NO board cell, so presence-firing is its only surface; here it would be the
   third.
3. **Cell placement — new trailing column on the in-progress table, with the digest arm.**
   Chosen: a `Readiness` column, the exact shape the `implemented` table uses for
   `finalize blocked — needs you`, PLUS the matching `digest_readiness()` in-progress arm (the
   digest must never disagree with the board's Readiness cell — the documented one-owner
   invariant). Rejected: suffixing the Branch cell (overloads a `code`-formatted field with
   prose) and a separate board section (no sibling precedent; a halted run is still
   `in-progress`).
4. **Check gated on `status: in-progress`.** The marker is written on an in-progress change and
   removed on re-claim; every other status either predates the marker or retires its meaning
   (the presence-encoded-state learning's reader-enumeration discharge — an archived or
   implemented file carrying a leftover section has no reader this check would misinform, and
   verify-run's postcondition already outranks a stale section). Rejected: no status gate (the
   `publish-deferred` posture) — that exists because its marker lives on archived files; this one
   does not.
5. **Separate horizon constant, same default value.** `RUN_HALTED_STALE_SECS` initialized to
   `FINALIZE_BLOCKED_STALE_SECS`'s default rather than aliasing it — the whole-predicate
   duplication learning cuts the other way for *constants with independent futures*.
6. **Coupling** — change 0222 (proposed, build-ready) rewrites idiom sites inside
   `board-checks.sh`; whichever lands second reconciles by intent at rebase (both additive).
   The groom writes `related: [222]` into 0261's manifest (forward link only). No `depends_on`:
   0237 is already `done`.
7. **Known, deliberately-untouched double-fire.** `aborted-run` leg B fires on any in-progress
   claim older than its 12h floor, so a halted change will also carry an `aborted-run` finding
   with its hedged (here misleading) remedy before `stale-run-halted`'s 72h horizon. Suppressing
   that is outside the stub's boundary ("leaves alone every existing `aborted-run` leg and
   floor") and must NOT be fixed opportunistically in this change.
