<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0143 — Empty id collapses the archive sort feeder's TAB-joined fields in render-board.sh](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-28-0143-empty-id-collapses-the-archive-sort-feeder-s-tab-joined-fiel.md)**
<!-- docket:backlink:end -->

# Empty fields collapse the archive sort feeder in render-board.sh — design

Change: 0143 · type `fix` · priority medium
Status: designed by `docket-auto-groom` (autonomous), 2026-07-28; revised after the adversarial
critic pass (one bounded round).

## Problem

`render-board.sh`'s archive block feeds its sort with a TAB-joined 4-tuple and reads it back with
`IFS=$'\t'`:

```
    for f in "${ARCFILES[@]}"; do
      base="$(basename "$f")"; d="${base:0:10}"; id="$(int_field "$f" id)"; st="$(field "$f" status)"
      printf '%s\t%s\t%s\t%s\n' "$d" "$id" "$st" "$f"
    done | sort -t$'\t' -k1,1r -k2,2nr
```

TAB is IFS *whitespace*, so `read` collapses runs of it and an **empty field is not preserved** —
every later field shifts left. The consuming loop's own guard (`[ -n "$id" ] || continue`) sits
*downstream* of the lossy join and therefore never sees the empty value: with an empty `id:` it
reads `id="done"`, `st=<path>`, `f=""`.

Reproduced against the current tip (3-file archive fixture: empty `id` + `status: done`; `id: 5` +
empty `status`; a well-formed `id: 6`):

```
| [0006](archive/2026-07-03-0006-ok.md) | Fine | 2026-07-03 |
sed: : No such file or directory
| [0005](archive/) |  | 2026-07-02 |
render-board.sh: line 125: printf: done: invalid number
sed: : No such file or directory
| [0000](archive/) |  | 2026-07-01 |
```

Three consequences, all confirmed by that run:

1. **Corrupt rows.** Shape `| [0000](archive/) |  | <date> |` — an archive link with an empty
   basename, an empty title, plus `printf: … invalid number` and `sed: : No such file or directory`
   on stderr.
2. **A widened recency window.** The corrupt row's `st` holds a path, not `done`, so it escapes the
   `done_seen` counter and stretches the `ARCHIVE_RECENT` verbatim window by one per corrupt file.
3. **An aborted tally.** Separately from the feeder, an empty `status:` makes the tally loops'
   associative-array assignment fail with `bad array subscript` — and the error **aborts the whole
   `for` loop**, so every file sorted after the offending one is silently dropped from the tally.
   This bites in **both** directions:
   - archive, `ARC_COUNT` (line 154): the fixture summary reads `Archive — done (1)` while two
     `done` files exist;
   - active, `SECTION` (line 141): an active file with an empty `status:` makes every later active
     change vanish from the board **and from the `--format digest` output** — `backlog proposed 1`
     instead of `2`, and an **empty `ready` line**, which is the machine-parsed queue
     `docket-implement-next` reads.

   In every case the script still **exits 0**, so a corrupt `BOARD.md` commits silently.

The active side's *field-shift* is genuinely unaffected — its `id` guard (line 139) precedes the
join, and it joins only two fields. Its **empty-subscript abort** is not; see design item 2.

## Relationship to change 0115 (settled)

Change **0115** (`board-row-dropped` widened to `archive/`, merged 2026-07-28, PR #128) landed
`renders_row()` in `board-checks.sh`. Its archive arm is `docket_status_is_terminal "$rr_st"` — i.e.
**any** non-terminal status (empty or not) means "no row".

This design deliberately does **not** try to make the renderer agree with that predicate in full:

- The **empty**-`status` and empty-`id` cases are fixed here, and after the fix the renderer and
  `renders_row()` agree on them.
- The **non-empty but non-terminal** case (e.g. `status: proposed` sitting in `archive/` — the
  interrupted `archive-change.sh` state `board-checks.sh`'s own comment names as the live one) still
  renders a row after this fix. That divergence is **pre-existing, out of scope, and left to be
  reported by `board-row-dropped`**, which is exactly what 0115 is for. No claim is made here that
  0143 aligns the renderer with the merged oracle.

Two questions the earlier abstain could not settle, now settled by the human and recorded:

- **Ownership of the header/table divergence.** After the fix, an archive file with an empty `id:`
  and `status: done` is still counted by `ARC_COUNT` (which keys on `status` alone) while rendering
  no row — a header/table mismatch. That state is **deliberately preserved**: 0115's spec names it
  as case (B), the state its `board-row-dropped` check exists to *report*. 0143 does not add an `id`
  guard to the `ARC_COUNT` loop.
- **Ordering.** 0115 is `done`. No `depends_on` is recorded; `related: [115]` stands. The stub's
  original "0143 must land after 0115 so the check keeps an unfixed oracle" rationale was
  demonstrated to be inverted and is discarded, not carried forward.

## Design

### 1. Guard the archive feeder before the join, keep the delimiter

Hoist the emptiness guard **above** the join:

```
      base="$(basename "$f")"; d="${base:0:10}"; id="$(int_field "$f" id)"; st="$(field "$f" status)"
      [ -n "$id" ] && [ -n "$st" ] || continue
      printf '%s\t%s\t%s\t%s\n' "$d" "$id" "$st" "$f"
```

A file with no usable `id` or no `status` is dropped from the archive table rather than shifted into
a corrupt row. Verified against the fixture above: output collapses to the single well-formed row,
stderr is clean.

The downstream `[ -n "$id" ] || continue` inside the `while` loop **stays** as defense in depth.

### 2. Guard both tally loops against an empty subscript

Archive (`ARC_COUNT`, line 154) and active (`SECTION`, line 141) carry the identical defect, so both
are fixed here — a one-line guard each:

```
for f in "${ARCFILES[@]}"; do st="$(field "$f" status)"; [ -n "$st" ] || continue; ARC_COUNT["$st"]=…
```
```
  st="$(field "$f" status)"; [ -n "$st" ] || continue
  SECTION["$st"]+=…
```

Neither is the contested `id` guard, and neither changes *which* well-formed files are accounted
for: an empty `status` is neither a terminal status (so `ARC_COUNT` never legitimately counted it)
nor a member of `DOCKET_STATUSES_ACTIVE` (so `SECTION` bucketed it where nothing iterates). What
they fix is the **collateral abort** that today silently drops every *later* file from the tally.
Both drops are already reportable by `board-row-dropped`; the abort is not reportable by anything.

**Declared blast radius:** `ARC_COUNT` also feeds `--format digest`'s
`backlog <terminal-status> <n>` lines, and `SECTION` feeds the digest's `change`/`ready` lines. On a
tree containing an empty-`status` file the digest output therefore changes (in the fixture,
`backlog done 1` → `backlog done 2`, and the `ready` line stops being spuriously empty). That is the
point of the fix, and it is asserted, not merely tolerated.

### 3. Test (`tests/test_render_board.sh`)

A focused fixture, separate from the golden tree (which contains no empty fields and must stay
byte-identical). Archive: empty `id:` + `status: done`; valid `id:` + empty `status:`; one
well-formed `done`. Active: one well-formed `proposed` with a spec, plus one with an empty `status:`
sorted before it. Asserts:

- **No corrupt row**: no line of the render matches the ERE
  `^\| \[[0-9]{4}\]\(archive/\) \|` — a 4-digit padded id whose archive link has an empty basename.
  Anchored this way rather than on the bare substring `](archive/) `, which **also matches a
  legitimate, currently-shipping row**: the older-done collapse table emits
  `| [2026-07](archive/) | 62 done |`. The `YYYY-MM` key is not four digits, so the ERE excludes it
  regardless of fixture size — the assert must not depend on the fixture staying under
  `ARCHIVE_RECENT` (15). Anchored on the generic shape rather than the literal `[0000]`, since the
  empty-`status` sibling renders `| [0005](archive/) |  | … |`.
- **The well-formed archive row still renders**, with its real basename and title.
- **stderr is empty** for that render (catches the `printf`/`sed` noise and both `bad array
  subscript` aborts).
- **The archive header count is not truncated**: `Archive — done (2)` — both `done` files counted,
  one row rendered. The mismatch is asserted as *intended* behavior, so a later "fix" to the
  `ARC_COUNT` loop cannot land silently.
- **The digest is not truncated**: the `backlog`/`change`/`ready` lines account for the well-formed
  active change that today disappears behind the empty-`status` file.
- **Non-vacuity**: each assert is confirmed to fail against the pre-fix script; the test comment
  records the pre-fix output so the assert's key is pinned, not merely present.
- The existing **golden byte-compare and digest golden must stay green untouched** — verified during
  design by running the patched script against the full suite (`test_render_board.sh` 200/200,
  `test_board_checks.sh`, `test_board_refresh.sh`, `test_board_refresh_on_transition.sh`,
  `test_docket_status.sh` all green), and no live archive file carries an empty `id:`/`status:`, so
  the real `BOARD.md` is byte-unchanged.

Use `grep -E` via a POSIX grep when asserting the ERE — the interactive `grep` on this machine is
ugrep and accepts syntax `/usr/bin/grep` rejects.

## Out of scope

- **The non-empty, non-terminal archive status** (`status: proposed` in `archive/`). Pre-existing
  divergence from `renders_row()`; reported by `board-row-dropped`. Left alone.
- **Interior TABs in a frontmatter value** (`status: done<TAB>X`) shift fields identically before and
  after this fix. `board-checks.sh` already handles it via `sanitize()` (change 0104). A separate
  hazard, worth its own stub — recommended follow-up, not minted here (auto-groom is never a mint
  site).
- **Guarding `ARC_COUNT` on `id`** — deliberately left, see above.
- **Making `render-board.sh` exit non-zero on a malformed input file.** Real (a corrupt board commits
  at exit 0) but a contract change for every caller; recommended as its own stub.
- Any edit to `board-checks.sh`. The check and its oracle stay independent.
- Any delimiter change.

## Assumptions

Every decision below was defaulted autonomously; this is the deferred audit trail. Assumptions
1, 2, 3 and 7 were revised after the critic falsified their stated rationale (the decisions
themselves survived in 1, 2 and 3; 7's anchor was replaced).

1. **Fix shape: guard-before-join vs. a non-collapsing delimiter vs. `read -d`.**
   Chosen: guard before the join. Rejected: swapping the archive block to `\x1f` — this is a scoped
   three-edit change inside one block (`printf`, its `sort -t`, the `IFS=`), **not** the whole-file
   refactor an earlier draft claimed (the other four `sort -t$'\t'` sites consume the active-side
   `id<TAB>file` stream and are untouched) — but it changes the renderer's internal data format to
   fix a one-line omission, and a non-printing delimiter is harder to debug in a script whose output
   is byte-compared. Rejected: `read -d ''` with NUL-terminated records — same objection, and it
   additionally forces a matching `sort -z` (available on the macOS `sort` checked during design,
   but one more portability surface for a bug that needs none). The chosen guard mirrors the active
   side's existing precedent.

2. **Drop-or-render for a file with a valid `id` but an empty `status`.**
   Chosen: drop. Rationale: an empty `status` is not a renderable terminal status. Rejected:
   rendering the row anyway — the archive table is `| # | Title | Merged |`, with no status column,
   so `status` is carried in the sort tuple *only* for the `done_seen` partition; a rendered
   empty-`status` row would therefore escape `done_seen` and never collapse into the older-done
   digest, which is consequence 2 of the Problem section preserved rather than fixed. Rejected:
   substituting a sentinel status, which adds a decoding step and a "sentinel leaked into BOARD.md"
   failure mode. **Not** justified by alignment
   with `renders_row()`: that predicate is stricter than this fix, and the residual divergence is
   named in *Out of scope*.

3. **Whether to touch the tally loops at all.**
   Chosen: guard **both** the archive `ARC_COUNT` and the active `SECTION` loops against an empty
   subscript. Rejected: leaving them (a known loop-aborting error in the same file, which corrupts
   the very header and digest this change's tests must read; the active-side abort additionally
   empties the `ready` queue line). Rejected: fixing only the archive one, which the critic showed
   would leave the strictly worse-blast-radius twin thirteen lines above while the spec claimed "the
   active side is unaffected". Rejected: also guarding on `id` — that is the contested case (B)
   state 0115 exists to report, and removing it would silently delete a landed check's reason to
   exist.

4. **Recording the relationship to 0115.**
   Chosen: `related: [115]`, no `depends_on`. 0115 is `done`, so a dependency would be inert; and the
   stub's stated ordering rationale was demonstrated to be inverted, so it is dropped rather than
   re-encoded.

5. **Relationship to 0144.**
   Chosen: `related: [144]`. 0144 (`board-checks.sh` non-zero exit voids the health pass) touches the
   board/health surface but a **different file** — subject overlap, not file collision. No ordering
   constraint either way. The only other active changes naming `render-board.sh` are 0009 and 0010,
   both `deferred` with no spec; no further coupling to record.

6. **Test placement and the golden.**
   Chosen: a focused fixture in `tests/test_render_board.sh`, leaving the golden byte-compare
   untouched. The fix only skips files carrying an empty `id`/`status`, neither of which occurs in
   the golden tree or in the live repo, so the golden must stay byte-identical — verified
   empirically, and itself a regression signal.

7. **Assert anchoring.**
   Chosen: the ERE `^\| \[[0-9]{4}\]\(archive/\) \|`, plus an empty-stderr assert and the digest
   asserts. Rejected: the bare substring `](archive/) `, which the critic showed collides with the
   shipping older-done collapse row (`| [2026-07](archive/) | 62 done |`) and only passes while the
   fixture stays under `ARCHIVE_RECENT`. Rejected: the literal `[0000]` (a no-op for the
   empty-`status` sibling) and asserting on the stderr strings themselves (`printf: done: invalid
   number` is bash message text, not a contract).

## Not covered

- Whether any real repo has ever produced an empty `id:`/`status:` change file. The bug is
  reproduced from a synthetic fixture; the fix is a robustness guard on a derived view, and its cost
  is three lines.
- Interaction with the `github` board surface — `board_surfaces: inline` here, and the mirror reads
  the change files directly, not this feeder.
