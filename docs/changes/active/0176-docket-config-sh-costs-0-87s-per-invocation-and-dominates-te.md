---
id: 176
slug: docket-config-sh-costs-0-87s-per-invocation-and-dominates-te
title: docket-config.sh costs ~0.87s per invocation and dominates test_docket_config.sh
status: proposed
priority: medium
type: perf
created: 2026-07-31
updated: 2026-07-31
depends_on: []
related: []
discovered_from: [174]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 0174 templated `test_docket_config.sh`'s git fixture and measured what was actually left.
The answer reframes the file's cost: of its ~109s wall clock, fixture construction was only
~15s (121 `mkrepo` calls at ~0.127s each, now ~4s). The remaining **~105s is 121 real
`bash scripts/docket-config.sh` invocations** — the same invocation-bound class as change 0175,
but a different script and, after `sync-agents.sh`, the largest single remaining item in the suite.

This was not visible before 0174. The spec for 0174 attributed the file's whole 103s to fixture
rebuilding, estimating ~0.5-0.8s per fixture; the measured figure is ~0.127s, so the fixture
premise was roughly 7x optimistic and the real cost sat somewhere else the whole time. 0174's
templating was still correct and kept nearly all of the ~11s actually available — but the file
stays a ~109s outlier, and no open change covers why.

Change 0175 is explicitly scoped to `sync-agents.sh` (and names `test_render_board.sh` as the same
class at smaller scale). Neither it nor change 0150 covers the config resolver's per-invocation
cost, so this would otherwise fall through the gap between them.

## What changes

To be designed. The first question is where the ~0.87s per `docket-config.sh` run goes — it is a
shell YAML reader that shells out to git for branch and remote probes, so the candidates are
repeated git subprocess work, redundant config re-parsing, or the bootstrap probe. Whether the
right fix is a cheaper resolver (which pays out for every real docket operation, since every skill
runs `preflight`) or a test-level change (fewer real invocations) is exactly what a brainstorm
should settle — the same fork change 0175 names, and the two may share a design.

## Out of scope

- `sync-agents.sh` per-invocation cost — change 0175.
- Git fixture reuse — change 0174, already done.
- A parallel suite runner — change 0150.
