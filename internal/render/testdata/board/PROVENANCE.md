# Board golden provenance

`board.golden` is a **frozen historical snapshot** of the live Bash board
renderer's `--format markdown` output over the hand-built fixture corpus under
`corpus/`. It is the drift guard for `internal/render`'s `Board` renderer, which
reproduces the same surface byte-for-byte.

## Generating command

Run from the repository root:

```
bash scripts/render-board.sh --changes-dir internal/render/testdata/board/corpus > internal/render/testdata/board/board.golden
```

- **Generated at commit:** `fdd48200bd8ac78f7899b39c87c09ea8eccb2e69` (change 0312, task 4).
- **Generator:** `scripts/render-board.sh` (change 0022; status vocabulary
  change 0104; stacked-changes readiness change 0298), with
  `scripts/lib/docket-frontmatter.sh` and `scripts/lib/docket-stack.sh`.

## Historical-snapshot contract

This golden is a point-in-time snapshot; it does **not** track the Bash script.
The script dies in the 0316+ Bash→Go cutover, at which point `Board` becomes the
sole board renderer. Do **not** regenerate this golden from the script to make a
failing test pass: a diff here is a real change to the board surface and must be
re-blessed deliberately. The corpus is data — exclude it from repo-wide scans
with a bounded path.

## What the corpus covers (frozen-corpus-covers-what-it-contains)

The `corpus/` fixture exercises exactly these surfaces; anything not listed is
covered by direct Go unit tests in `board_test.go`, not by this golden.

Sections and rows:

- **In progress** — id 1, with a `spec:` (Spec cell) and a `branch:` (Branch cell).
- **Proposed** — three readiness bands:
  - id 2 `build-ready` (its `depends_on: [9]` points at a `done` change, so the
    dependency is satisfied);
  - id 3 `needs-brainstorm` (no spec, not trivial);
  - id 4 waiting `⏳ waiting on #3 — not yet built` (dep on a non-built change).
- **Blocked** — id 5, `blocked_by` free-text cell.
- **Deferred** — id 6.
- **Implemented — awaiting merge** — id 7, PR cell from a full PR URL, empty
  Readiness cell (no `## Finalize blocked` section).
- **Archive `<details>`** — id 9 (`done`) and id 10 (`killed`); summary emoji +
  `done + killed` label; Merged date from the archive filename prefix.

Mermaid graph:

- bare nodes (changes with no `depends_on`);
- a `depends_on` edge between two active changes (`0003 --> 0004`);
- a `done`-node style (`0009:::done`) plus the `classDef` line, driven by an
  active change (id 2) depending on the archived `done` change (id 9).

Bands and surfaces the corpus deliberately does **not** cover — unit-tested
directly instead:

- proposed `⏳ … — needs your merge` (dependency on an `implemented` change);
- proposed `auto-groom blocked — needs you`;
- proposed `⏳ waiting on #NNNN — stack base not built`;
- the **Stacked-merged** section, emoji, and `merged into #NNNN` Stack cell;
- numeric (not lexical) id ordering and the empty-priority / `untyped`-type
  cells;
- the archive `done`-collapse digest past the 15-most-recent window.
