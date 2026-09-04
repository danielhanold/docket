# Board golden provenance

`board.golden` is the **reviewed canonical render** of the hand-built fixture
corpus under `corpus/`, produced by `render.Board` under the **default
presentation** (`render.DefaultBoardPresentation` — the built-in section order,
`updated desc` on every section). It is the byte-drift guard for
`internal/render`'s `Board` renderer: `TestBoardGolden` re-renders the corpus and
requires the bytes to match.

As of **change 0367** the board renders six configurable presentation groups
(In progress, Built, Blocked, Groomed, Proposed, Deferred) with per-section
sorting, so the default output changed by design and this golden was **re-founded**
at that change. It is no longer the frozen Bash-era `render-board.sh` snapshot it
began as; the Bash script died in the 0316+ Bash→Go cutover, and `Board` is the
sole board renderer.

## Regenerating (re-blessing) the golden

There is deliberately **no committed regeneration hook** — a diff here is a real
change to the board surface and must be re-blessed deliberately, not auto-updated
to make a failing test pass. To re-derive after an intended surface change:

1. Render the corpus with `render.Board(render.BoardInput{Snapshot: <corpus>,
   Presentation: render.DefaultBoardPresentation()})` (see `boardCorpusSnapshot`
   in `board_test.go`) and write the bytes to `board.golden` — via a **throwaway**
   test hook you delete before committing, never a committed backdoor.
2. **Review the diff by hand** before accepting it: section order, headings,
   per-section sorted row order (`updated desc`, numeric id-desc ties),
   unified Built/Blocked cells, the counts line, and byte-identical mermaid and
   archive footer.
3. Update this file if the corpus coverage below changed.

## What the corpus covers (frozen-corpus-covers-what-it-contains)

The `corpus/` fixture exercises exactly these surfaces; anything not listed is
covered by direct Go unit tests in `board_test.go`, not by this golden. Sections
follow the default section order; rows sort `updated desc` within each section.

Sections and rows:

- **In progress** — two Readiness bands, sorted `updated desc`:
  - id 1 (updated 2026-08-10), with a `spec:` (Spec cell), a `branch:` (Branch
    cell), and no `## Run halted` section (empty Readiness cell);
  - id 8 (updated 2026-08-09), carrying the bare `## Run halted` body section:
    the `run halted — needs you` Readiness cell (change 0261).
- **Built** — id 7, an `implemented` change with no `## Finalize blocked`
  section: PR cell from a full PR URL and the `awaiting merge` State cell.
- **Blocked** — id 5, a lifecycle-`blocked` change: empty PR cell (no `pr:`) and
  the stored `blocked_by` free-text Reason cell.
- **Groomed** — id 2, a `proposed`, spec-backed, build-ready change (its
  `depends_on: [9]` points at a `done` change, so the dependency is satisfied):
  a `[spec](…)` Spec cell.
- **Proposed** — two readiness bands, sorted `updated desc`:
  - id 4 (updated 2026-08-04) waiting `⏳ waiting on #3 — not yet built` (dep on a
    non-built change);
  - id 3 (updated 2026-08-03) `needs-brainstorm` (no spec, not trivial).
- **Deferred** — id 6.
- **Archive `<details>`** — id 9 (`done`) and id 10 (`killed`); summary emoji +
  `done + killed` label; Merged date from the archive filename prefix.

Mermaid graph (outside section order/sorting — the presentation never reaches it):

- bare nodes (changes with no `depends_on`);
- a `depends_on` edge between two active changes (`0003 --> 0004`);
- a `done`-node style (`0009:::done`) plus the `classDef` line, driven by an
  active change (id 2) depending on the archived `done` change (id 9).

Bands and surfaces the corpus deliberately does **not** cover — unit-tested
directly instead:

- the dated `## Run halted — <date>` variant NOT lighting the Readiness cell
  (whole-line decode contract — `TestBoardRunHaltedWholeLineContract`);
- the classification precedence edges (finalize-blocked implemented → Blocked;
  spec-backed build-ready → Groomed vs trivial-without-spec → Proposed);
- the Blocked `finalize blocked — needs you` Reason cell;
- the Built `merged into #NNNN` State cell for a stacked-merged change;
- proposed `⏳ … — needs your merge` (dependency on an `implemented` change);
- proposed `auto-groom blocked — needs you`;
- proposed `⏳ waiting on #NNNN — stack base not built`;
- proposed `build-ready (trivial)` (trivial, spec-less build-ready);
- non-default section orders and per-section sort keys, same-date id-desc ties,
  and the unknown-date band;
- the empty-priority / `untyped`-type cells;
- the archive `done`-collapse digest past the 15-most-recent window.
