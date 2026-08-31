---
id: 157
slug: roll-up-the-seven-build-ready-changes-into-one-branch
title: Roll up the seven build-ready changes into one branch
status: done
priority: medium
type: fix
created: 2026-07-28
updated: 2026-07-28
depends_on: []
related: [143, 144, 146, 148, 149, 152, 153]
discovered_from: []
adrs: [52]
spec: docs/superpowers/specs/2026-07-28-build-ready-fix-rollup-design.md
plan: docs/superpowers/plans/2026-07-28-build-ready-fix-rollup-plan.md
results:
trivial: false
auto_groomable: false
branch: feat/roll-up-the-seven-build-ready-changes-into-one-branch
pr: 136
blocked_by:
claimed_at: 
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-28-build-ready-fix-rollup-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-28-build-ready-fix-rollup-design.md) |
| Plan | [2026-07-28-build-ready-fix-rollup-plan.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-07-28-build-ready-fix-rollup-plan.md) |
| ADRs | [ADR-0052](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0052-config-key-resolution-boundary.md) |
<!-- docket:artifacts:end -->

## Why

Seven changes sat build-ready at once — 0143, 0144, 0146, 0148, 0149, 0152, 0153 — each groomed with
a full linked spec, each a small edit to `scripts/` or `tests/`. Draining them one at a time costs
seven claim/reconcile/plan/build/review/PR cycles that re-read the same files, and the per-change
overhead dominates the actual work. This change carries all seven on one branch, one plan, one
review, one PR.

The seven also cluster: 0148 and 0149 both edit `tests/test_docket_config.sh`, and 0152 and 0153 both
edit `scripts/lib/docket-runtime.sh`. Built separately, each pair costs a rebase and a stale-numbers
re-derivation; built together, the ordering is just a commit sequence.

The seven originals are killed as this change is created — they are not dependencies and must not
stay selectable by `docket-implement-next`, or the work lands twice. Their spec files stay on
`docket` and stay authoritative for their own design; this change's spec coordinates them.

## What changes

Seven units, in this build order (the ordering rationale and the per-unit specs are in the linked
spec — read the constituent spec before touching its files):

1. **0143** (`fix`) — hoist the emptiness guard above `render-board.sh`'s archive sort-feeder TAB
   join; guard both per-status tally loops against an empty subscript.
2. **0144** (`chore`) — capture `board-checks.sh`'s exit status in `docket-status.sh`'s
   `health_checks()` and emit a `health checks failed <exit>` report line.
3. **0146** (`fix`) — widen `tests/test_config_read_channel.sh`'s scanned token set to
   `{.docket.yml, .docket.local.yml, config.yml}` at both match sites; dated `## Update` on ADR-0052.
4. **0148** (`chore`) — delete the two unfalsifiable `-z "$DOCKET_BASH_PATH"` asserts, their seeds,
   and the now-dead `__poison__` clause.
5. **0149** (`chore`) — replace the prelude guard's absolute `exempt <= 5` ceiling with a
   proportional floor on `t_ok`, derived against the tree *after* 0148 lands.
6. **0152** (`refactor`) — route `ensure-docket-env.sh` through `docket-runtime.sh`'s validator, keep
   `docket.sh`'s POSIX prologue as a documented exception, and add the missing negative fixtures.
7. **0153** (`fix`) — depth-anchor `_docket_runtime_scan`'s leaf match to the block's shallowest
   structural child, with a named error for the rejected shape, keeping
   `ensure-global-config.sh`'s both-declarations guard armed.

Acceptance is every constituent's own bar, plus one green full-suite run at the end of the branch.

## Out of scope

- Re-designing any constituent. Each spec is settled; this change executes them.
- A reusable batching capability in docket (a batch mode for `docket-implement-next`, or grouping
  build-ready changes by type). Worth its own change; it would not land these seven.
- Change 0018 (`docs`, ADR for the pure-bash YAML stance) — build-ready but unrelated to this
  cluster.
- Weakening any test to get the branch green. If a unit cannot go green it is dropped from the branch
  and re-minted as its own change.

## Open questions

None. Scope, kill-the-originals, and the seven-change membership were settled with the human at
creation.

## Reconcile log

### 2026-07-28

No drift. The change was created today and reconciled the same day, so the snapshot it was drafted
against is still current. Verified:

- **All seven originals are killed and archived** — `2026-07-28-0143-…`, `-0144-`, `-0146-`, `-0148-`,
  `-0149-`, `-0152-`, `-0153-` all sit in `archive/`, and each kill's terminal record is published on
  `origin/main` (commits `483d3d3c`…`f264d7e6`). None is selectable by `docket-implement-next`, so the
  double-implementation risk the spec names is closed.
- **All seven constituent specs survive the kills** on `docket` in `docs/superpowers/specs/`, as the
  spec predicted (killing a change archives the change file, not its artifacts). They stay
  authoritative for their own designs.
- **No target file moved under us.** Nothing on `origin/main` has touched `render-board.sh`,
  `docket-status.sh`, `lib/docket-runtime.sh`, `ensure-docket-env.sh`, `ensure-global-config.sh`,
  `test_docket_config.sh`, `test_config_read_channel.sh`, or `test_render_board.sh` since the specs
  were authored; the newest code commits on those paths are change 0133's runtime-library work, which
  every constituent spec was already written against.
- **The code anchors each unit keys on are intact** — the two `-z "$DOCKET_BASH_PATH"` asserts
  (0148) at `test_docket_config.sh:1534,1550`, the `__poison__` seeds at `:1410,1566`, and the
  absolute `exempt <= 5` ceiling (0149) at `:2021`.

Scope, ordering, and the acceptance bar carry forward unchanged. One correction of emphasis for the
build: 0149's proportional floor must be derived against the tree **after** 0148's assert deletions
land on this branch — the numbers printed in 0149's own spec predate them.
