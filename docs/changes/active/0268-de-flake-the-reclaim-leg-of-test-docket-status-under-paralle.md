---
id: 268
slug: de-flake-the-reclaim-leg-of-test-docket-status-under-paralle
title: 'De-flake the reclaim leg of test_docket_status under parallel contention'
status: proposed
priority: medium
type: fix
created: 2026-08-08
updated: 2026-08-09
depends_on: []
related: [252]
discovered_from: [245]
adrs: []
spec: docs/superpowers/specs/2026-08-09-de-flake-reclaim-leg-test-docket-status-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-de-flake-reclaim-leg-test-docket-status-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-de-flake-reclaim-leg-test-docket-status-design.md) |
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced while running change 0245's full-suite build gate. The first gate run came back `EXIT=1` on a single assert in `tests/test_docket_status.sh`: `NOT OK - reclaim(auto off): prints the state-valid remedy naming docket.sh reclaim-claims`. Run standalone the suite passes; re-run in parallel it passed. Change 0245 touches none of `tests/test_docket_status.sh`, `scripts/docket-status.sh`, or `scripts/reclaim-claims.sh`, so this is a pre-existing flake under parallel contention, not a regression.

**Diagnosis (settled at groom, 2026-08-09; details in the linked spec)** — the flake is neither of the originally suspected causes. The failing assert is the only one in the leg — and the only remaining instance in `tests/` — written as a `printf "%s\n" "$(cat …)" | grep -qF …` pipeline under the file's `set -uo pipefail` prologue: `grep -q` exits on first match, the producer takes SIGPIPE under load, and the 141 becomes an intermittent NOT OK. This is the repo's promoted pipefail family (AGENTS.md Shell rule; learnings `pipefail`). Both stub suspects are exonerated: the tested path reads no clock (`board-checks.sh` is mocked; `reclaim_pass` greps a captured blob), and every fixture path lives under the file's own per-process `mktemp -d`. A flaky assert in the suite that gates every merge is worse than a missing one: it trains readers to re-run until green, which is exactly how a real red gets waved through.

**Independent value** — stands entirely with change 0245 reverted; the flake predates that branch and will keep firing on unrelated branches until the pipeline assert is rewritten.

**Boundary** — rewrite the one assert to the leg's sibling idiom (`grep -qF "docket.sh reclaim-claims" "$tmp/reclaim-off-out.txt"`), mutation-test the rewritten guard, and demonstrate stability with 10 consecutive green full parallel `scripts/run-tests.sh` runs. It stops there: no production-code change (the diagnosis proves the flake lives in the test's assert), no re-audit of other suites (a whole-repo grep already shows zero other hits of the shape), no `tests/lib/` fixture work — change 0252 owns that; recorded as `related:` only, a file-collision caution with no dependency either way.

**Reason for deferral** — change 0245's branch is scoped to `sync-agents.sh` wrapper generation and its own new suites. Debugging an unrelated suite's parallel-contention flake would expand that branch into a second, independently reviewable concern, and its fix belongs with someone reading the reclaim leg's own design rather than riding a refactor branch.

## Carry-forward from #0247 (2026-08-11)

Change 0247 landed on this surface and spent its budget headroom. Before adding to
`scripts/docket-status.sh`, `tests/test_docket_status.sh`, or `skills/docket-status/SKILL.md`, read
these two numbers as measured at 0247's close-out:

- `tests/test_docket_status.sh` — roughly **3s of margin** against its 45s row in
  `tests/runtime-budgets.tsv`. This change's own acceptance bar is **10 consecutive green full
  parallel runs**, which is exactly the contended measurement the margin is compared against.
- `skills/docket-status/SKILL.md` — **22 words** of headroom against its size budget.

The next edit to either trips a budget. The remedy is already settled and should not be re-derived:
apply **change 0137's rounding rule** (next multiple of 5 plus a 5s margin, computed from the worst
*standalone serial* reading, never the contended run-of-the-day number) and carry **change 0201's
in-diff argument** for the word budget. **#0118 is queued against the same surface**, so whichever of
the two lands second inherits whatever margin the first leaves. See the learnings finding
`budget-headroom-is-spent-before-it-is-breached`.
