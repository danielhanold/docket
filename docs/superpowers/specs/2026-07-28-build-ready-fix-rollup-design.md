<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0157 — Roll up the seven build-ready changes into one branch](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0157-roll-up-the-seven-build-ready-changes-into-one-branch.md)**
<!-- docket:backlink:end -->

# Build-ready rollup — seven groomed changes in one branch

## Problem

Seven changes are build-ready and groomed, each with a full linked spec: 0143, 0144, 0146, 0148,
0149, 0152, 0153. Running `docket-implement-next` once per change costs seven claim/reconcile/plan/
build/review/PR cycles, each re-reading the same scripts and the same test suite. The per-change
overhead — not the edits — is what dominates, and the edits themselves are small and cluster into
three files-in-common groups.

This change is a **rollup**: one branch, one plan, one review, one PR carrying all seven bodies of
work. The seven originals are killed and this change carries their scope.

## Non-goal

This is not a reusable batching feature for docket. Nothing in `docket-implement-next`'s selection
or in the convention changes. It is a one-off change whose scope happens to be seven already-designed
units of work.

## The design is not re-authored here

Each constituent's design is already settled and stays authoritative in its own spec. This document
is a coordination spec: scope, ordering, collisions, and the acceptance bar. Read the constituent
spec before touching its files — do not re-derive the design from the change body.

| # | Type | Work | Constituent spec |
|---|---|---|---|
| 0143 | fix | Hoist the emptiness guard above `render-board.sh`'s archive sort-feeder join; guard both per-status tally loops against an empty subscript | `2026-07-28-archive-sort-feeder-empty-field-collapse-design.md` |
| 0144 | chore | Capture `board-checks.sh`'s exit status in `docket-status.sh`'s `health_checks()`; emit `health checks failed <exit>` | `2026-07-28-board-checks-exit-swallowed-design.md` |
| 0146 | fix | Widen `test_config_read_channel.sh`'s token set to `{.docket.yml, .docket.local.yml, config.yml}` at both match sites; dated `## Update` on ADR-0052 | `2026-07-28-config-read-channel-guard-widening-design.md` |
| 0148 | chore | Delete the two unfalsifiable `-z "$DOCKET_BASH_PATH"` asserts, their seeds, and the dead `__poison__` clause | `2026-07-28-unfalsifiable-runtime-asserts-design.md` |
| 0149 | chore | Replace the prelude guard's absolute `exempt <= 5` ceiling with a proportional floor on `t_ok` | `2026-07-28-prelude-guard-proportional-bound-design.md` |
| 0152 | refactor | Route `ensure-docket-env.sh` through `docket-runtime.sh`'s validator; keep `docket.sh`'s prologue as a documented exception; add the missing negative fixtures | `2026-07-28-consolidate-bash4-validator-copies-design.md` |
| 0153 | fix | Depth-anchor `_docket_runtime_scan`'s leaf to the block's shallowest structural child; named error for the rejected shape; keep `ensure-global-config.sh`'s both-declarations guard armed | `2026-07-28-runtime-bash-depth-anchor-design.md` |

All seven spec files live in `docs/superpowers/specs/` on `docket` and survive the kills — killing a
change archives the change file, it does not delete its artifacts.

## Ordering, and why it is not free

Three of the seven are independent; four form two ordered pairs that touch the same file. Build in
this order:

1. **0143** — `scripts/render-board.sh`, `tests/test_render_board.sh`. Isolated.
2. **0144** — `scripts/docket-status.sh`, `scripts/docket-status.md`, its tests. Isolated.
3. **0146** — `tests/test_config_read_channel.sh`, plus a dated `## Update` on ADR-0052. Isolated.
4. **0148 → 0149**, both in `tests/test_docket_config.sh`. 0148 removes asserts and its spec pins the
   post-change counts (assert count 381 → 375, `TOTALS sites=64 exempt=3 ok=61 viol=0` unchanged).
   0149's proportional floor is derived from `t_ok`, so it must be computed against the tree
   **after** 0148 lands, not against the numbers in 0149's spec. Reversing the pair makes 0149's
   floor stale on the same branch.
5. **0152 → 0153**, both in `scripts/lib/docket-runtime.sh`. Different functions
   (`docket_runtime_validate_bash` vs `_docket_runtime_scan`), and each spec explicitly disclaims the
   other's. But 0152 adds the first negative fixtures for `ensure-docket-env.sh` and
   `ensure-global-config.sh`, and 0153 changes what `ensure-global-config.sh`'s both-declarations
   guard counts. Land 0152 first so 0153 updates fixtures that exist rather than inventing them.

Within each unit, TDD as usual: the constituent spec's fixtures redden first.

## Acceptance

- Every constituent's own acceptance criteria met, per its spec. A rollup does not lower any bar.
- The full suite green in one run at the end of the branch — the point of batching is one integration
  signal, so a per-unit green that goes red later is not done.
- `/usr/bin/grep` used for any portability re-check: the PATH `grep` on this machine is ugrep and
  masks BSD ERE limits.
- One PR, one whole-branch review. The review covers all seven; a finding scoped to one unit does not
  block the others from merging, since they ship together anyway.

## Risks

- **A red suite is seven changes wide.** If the branch cannot go green, the fallback is to drop the
  offending unit from the branch and re-mint it as its own change, not to weaken a test.
- **Review surface.** Seven units in one diff is a bigger review than any of them alone. The ordering
  above is also the commit order, so the diff reads unit-by-unit.
- **Killing the originals is the commit point.** After the kills, the rollup is the only tracked
  record of this work; its `## What changes` must therefore name all seven explicitly.

## Rejected alternatives

- **`depends_on` the seven instead of killing them.** They stay selectable by
  `docket-implement-next`, which is exactly the double-implementation this avoids.
- **Roll up only the three `fix`-typed changes.** The two ordered pairs both straddle the type
  boundary (0148/0149 are `chore`; 0152 is `refactor`, 0153 is `fix`), so a fix-only rollup splits
  the couplings it exists to exploit.
- **Build a batch mode into docket.** A real option, and worth its own change later — but it is
  design work that does not land these seven fixes.
