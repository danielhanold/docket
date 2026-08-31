---
id: 174
slug: reuse-test-git-fixtures
title: Reuse test git fixtures instead of rebuilding them per assertion
status: done
priority: medium
type: chore
created: 2026-07-31
updated: 2026-07-31
depends_on: []
related: [150, 175]
discovered_from: [168]
adrs: []
spec: docs/superpowers/specs/2026-07-31-reuse-test-git-fixtures-design.md
plan: docs/superpowers/plans/2026-07-31-reuse-test-git-fixtures.md
results: docs/results/2026-07-31-reuse-test-git-fixtures-results.md
trivial: false
auto_groomable: false
branch: feat/reuse-test-git-fixtures
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/141
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-31-reuse-test-git-fixtures-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-31-reuse-test-git-fixtures-design.md) |
| Plan | [2026-07-31-reuse-test-git-fixtures.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-07-31-reuse-test-git-fixtures.md) |
| Results | [2026-07-31-reuse-test-git-fixtures-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-07-31-reuse-test-git-fixtures-results.md) |
<!-- docket:artifacts:end -->

## Why

The suite costs ~530s of wall clock, and four files spend most of theirs rebuilding a
byte-identical git repository once per assertion group: `test_docket_config.sh` calls its
`mkrepo` helper 122 times, `test_board_checks.sh` calls `new_repo` 37 times, `test_closeout.sh`
31 times, `test_docket_status.sh` calls `git_repo_setup` 30 times. Each call spawns ~8–10 git
processes to produce the same baseline. That is ~165s of the suite buying nothing.

This is a developer-loop tax rather than a correctness problem, but it is paid on every build,
by every profile worker, and by every review pass — and it is paid to construct fixtures whose
content never varies.

The measurements and the per-file breakdown are in the linked spec.

## What changes

Each of the four fixture-builder helpers keeps its name and signature exactly as they are, and
changes only its body: build the baseline repository once per file into a template directory,
then produce each fixture by copying that template and repointing the copy's `remote.origin.url`
at the copied origin.

Holding the signature fixed is the design: 220 call sites stay untouched, the diff is four helper
bodies, and the review reduces to one question — are the copied fixtures still independent of each
other and of the template? That question gets an explicit test, because fixture coupling surfaces
as order-dependent flakiness that a single green run would not catch.

## Out of scope

- `sync-agents.sh`'s ~5.5s-per-invocation cost and the three `test_sync_agents*` files (279s, 53%
  of the suite). Different root cause, and a change to shipped script behavior rather than a
  test-only refactor — tracked as change 0175.
- `test_render_board.sh` — same invocation-bound cause at smaller scale, despite its place in the
  top eight.
- A parallel suite runner. The largest single lever, but there is no suite runner today and
  introducing one is its own design; change 0150 already records the gap.
- Any change to what the tests assert. This makes existing assertions cheaper; it must not delete,
  weaken, or merge one.

## Open questions

Carried in the spec, and all four are checks to run before swapping a helper body rather than
things to discover from a red suite: whether any assertion depends on fixtures having distinct
baseline SHAs; whether `test_board_checks.sh`'s commit-ageing survives a fixed template date;
whether `remote set-head origin -a` must be re-run per copy; and whether the four helpers should
converge on one shared implementation or stay independent.

## Reconcile log

### 2026-07-31 — reconciled against current `origin/main`

Re-read against the spec, `related: [150, 175]`, `discovered_from: [168]`, ADRs 0058–0064, and the
current test sources. The change is unchanged in scope; it was drafted today and nothing has landed
since that touches it.

Verified rather than assumed:

- The four call-site counts in the spec are still exact against the working tree — `mkrepo` 122,
  `new_repo` (board_checks) 37, `new_repo` (closeout) 31, `git_repo_setup` 30.
- The three fixture layouts described in the spec match the current helper bodies, including the
  detail that `git_repo_setup` builds only `seed` + `origin.git` and leaves the working clone to its
  callers — so its copy path differs in shape from the other three.
- Neither unmerged branch collides with this change: `feat/codex-cli-validation-runbook` (#89) and
  `feat/cursor-profile-routed-build-support` (#140) touch none of the four test files.
- The two related changes remain unstarted (0150 `proposed`, 0175 `proposed`), so the out-of-scope
  boundaries the spec draws — cause (B) to 0175, cause (C) to 0150 — are still live and correct.

No scope adjustment. The four open questions stay open and are planning inputs, as the spec intends.
