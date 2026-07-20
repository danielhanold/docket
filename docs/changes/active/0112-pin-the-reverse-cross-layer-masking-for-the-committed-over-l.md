---
id: 112
slug: pin-the-reverse-cross-layer-masking-for-the-committed-over-l
title: Pin the reverse cross-layer masking for the committed-over-local rung pair
status: proposed
priority: medium
created: 2026-07-20
updated: 2026-07-20
depends_on: []
related: []
discovered_from: [106]
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

Change 0106 pinned the `finalize.test_command` `auto` sentinel's cross-layer masking with three
fixtures in section S of `tests/test_docket_config.sh`: two forward cases (`s4`, `s5`) and one
reverse case (`s6`). The whole-branch review of that change found the reverse direction is proven
for only **one** of the two lower rungs.

`s6` covers *(lower = global `auto`, higher = committed real command)*. The pair
*(lower = committed `auto`, higher = local `.docket.local.yml` real command)* is untested — a gap on
the **precedence** axis.

The concrete refactor that would slip through: a committed-rung-specific clear appended after the
resolution chain, e.g.

```sh
[ "$(yaml_get "$CFG" test_command)" = auto ] && FINALIZE_TEST_COMMAND=""
```

Traced against all five of 0106's asserts, every one stays **green**:

- `s4` — the chain yields `auto` from the local rung; the committed rung holds `make test`, so the
  extra clear does not fire; `:201` collapses as normal. Green.
- `s5` — the committed rung holds `auto`, so the extra clear fires on a value that is already `""`.
  Green.
- `s6` — the committed rung holds `make test`, so the extra clear does not fire. Green.

Yet a real repo with `.docket.local.yml` setting `test_command: make local-test` over a committed
`test_command: auto` would **silently lose its local command** — finalize would fall back to
auto-detection with the whole suite green. That is exactly the class 0106 exists to prevent, one
rung over.

Note that 0106's spec explicitly declined a *forward* case on the grounds that it "reuses the two
helpers `s4` and `s5` already cover and adds no distinct code path" — a claim about **helpers**.
That rationale does not reach this gap, which is about **precedence**, not helper coverage.

## What changes

Add one fixture to section S of `tests/test_docket_config.sh`, alongside `s4`/`s5`/`s6`, following
their established shape (per-fixture `$tmp/s7.xdg` root, a control assert, and the
`FINALIZE_TEST_COMMAND=__poison__` prelude before each `eval`):

- **`s7`** — committed `.docket.yml` sets `test_command: auto`; `.docket.local.yml` sets
  `test_command: make local-test`; assert the resolved export is `make local-test` (the higher
  layer's real command survives a lower layer's sentinel).

Mutation-test it with the committed-rung-specific clear shown above: `s7` must redden while all
five of 0106's asserts stay green. Read the `ok` count as part of the contract, per the repo's
`guards-are-code` rule.

## Out of scope

- Any change to the sentinel's semantics or the placement of the collapse — like 0106, this pins
  current behavior.
- Re-running 0106's own two mutations; they are already recorded in that change's results file.
