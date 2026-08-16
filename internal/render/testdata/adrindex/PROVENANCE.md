# ADR index golden provenance

`index.golden` is a **frozen historical snapshot** of the live Bash ADR-index
renderer's output over the hand-built fixture ADR set under `adrs/`. It is the
drift guard for `internal/render`'s `ADRIndex` renderer, which reproduces the
same surface byte-for-byte.

## Generating command

Run from the repository root:

```
bash scripts/render-adr-index.sh --adrs-dir internal/render/testdata/adrindex/adrs > internal/render/testdata/adrindex/index.golden
```

- **Generated at commit:** `30afee5511c4d6f1bba46debe4b699c3a3d6d12b` (change 0312, task 5).
- **Generator:** `scripts/render-adr-index.sh` (change 0030), with
  `scripts/lib/docket-frontmatter.sh`.

## Historical-snapshot contract

This golden is a point-in-time snapshot; it does **not** track the Bash script.
The script dies in the 0316+ Bash→Go cutover, at which point `ADRIndex` becomes
the sole ADR-index renderer. Do **not** regenerate this golden from the script to
make a failing test pass: a diff here is a real change to the index surface and
must be re-blessed deliberately. The fixture `adrs/` corpus is data — exclude it
from repo-wide scans with a bounded path.

## What the fixture covers (frozen-corpus-covers-what-it-contains)

The five ADRs under `adrs/` exercise all three index groups and every row
annotation:

- **Active** (status not beginning `Superseded by`/`Reversed by`, not `Deprecated`):
  - id 1 `Accepted`, a **bare** row — no change, no supersedes/reverses/relates
    annotation;
  - id 2 `Accepted`, carrying **every** annotation at once — `← change #20`,
    `→ supersedes ADR-0003`, `→ reverses ADR-0004`, and a multi-id
    `· relates to ADR-0001, ADR-0005`.
- **Superseded / Reversed**:
  - id 3 `Superseded by ADR-0002`, with `← change #5` and `· relates to ADR-0001`;
  - id 4 `Reversed by ADR-0002`, a bare row.
- **Deprecated**:
  - id 5 `Deprecated`, a bare row.

The empty-group `_None._` rendering is not present in this fixture (every group is
non-empty); it is covered by a direct Go unit test in `adrindex_test.go`.
