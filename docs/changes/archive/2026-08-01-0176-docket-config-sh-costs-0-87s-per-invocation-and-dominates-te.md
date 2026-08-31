---
id: 176
slug: docket-config-sh-costs-0-87s-per-invocation-and-dominates-te
title: docket-config.sh costs ~0.87s per invocation and dominates test_docket_config.sh
status: done
priority: medium
type: perf
created: 2026-07-31
updated: 2026-08-01
depends_on: []
related: [174, 175, 179]
discovered_from: [174]
adrs: [62]
spec: docs/superpowers/specs/2026-07-31-docket-config-per-invocation-cost-design.md
plan: docs/superpowers/plans/2026-08-01-docket-config-per-invocation-cost.md
results: docs/results/2026-08-01-docket-config-per-invocation-cost-results.md
trivial: false
auto_groomable:
branch: feat/docket-config-sh-costs-0-87s-per-invocation-and-dominates-te
pr: https://github.com/danielhanold/docket/pull/145
blocked_by:
reconciled: true
claimed_at: 
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-31-docket-config-per-invocation-cost-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-31-docket-config-per-invocation-cost-design.md) |
| Plan | [2026-08-01-docket-config-per-invocation-cost.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-01-docket-config-per-invocation-cost.md) |
| Results | [2026-08-01-docket-config-per-invocation-cost-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-01-docket-config-per-invocation-cost-results.md) |
| ADRs | [ADR-0062](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0062-in-repo-shell-yaml-readers-no-external-parser.md) |
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

Optimize the shipped resolver itself; test speed improves as a consequence. A representative
local-origin profile found ~359 spawned commands per run, dominated by 235 `sed`, 77 `head`, and
18 `awk` invocations repeatedly scanning three tiny config layers, versus only eight Git calls.

Load each config layer once per resolver run and replace the general scalar and nested-block scans
with fork-free Bash readers over those immutable snapshots. Preserve the supported YAML subset,
precedence, output, warnings, diagnostics, Git freshness probes, and specialized `runtime.bash`
reader. Accept the change on a same-machine median speedup of at least 2× plus a standing
spawned-command ceiling; the existing resolver assertions remain the unchanged correctness oracle.

Design: [`docs/superpowers/specs/2026-07-31-docket-config-per-invocation-cost-design.md`](../../superpowers/specs/2026-07-31-docket-config-per-invocation-cost-design.md).

## Out of scope

- `sync-agents.sh` per-invocation cost — change 0175.
- Git fixture reuse — change 0174, already done.
- A parallel suite runner — change 0150.
- Shared config-reader extraction — change 0179.
- Caching or removing Git freshness and bootstrap probes.
- Any change to accepted config syntax, precedence, output, warnings, or diagnostics.

## Reconcile log

### 2026-08-01 — reconciled against current `origin/main`

Re-read the linked design against related changes 0174, 0175, and 0179; ADR-0062; recent
archived changes; and `origin/main` at `6fd61cc9`. Change 0174's reusable fixture is landed,
and change 0175's independent `sync-agents.sh` parser optimization is now done; neither changes
`scripts/docket-config.sh`'s file-backed `yaml_get` / `yaml_block_body` readers or their tests.
The current resolver still exhibits the fork-heavy reader shape described by the spec, while
change 0179 remains an ungroomed, dependency-gated follow-up and is not a shared-extractor
dependency. The specialized `runtime.bash` reader remains in `scripts/lib/docket-runtime.sh` and
is explicitly outside this refactor. No scope adjustment or additional follow-up is needed.
