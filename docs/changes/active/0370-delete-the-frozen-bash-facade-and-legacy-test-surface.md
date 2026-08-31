---
id: 370
slug: 'delete-the-frozen-bash-facade-and-legacy-test-surface'
title: 'Delete the frozen Bash facade and legacy test surface'
status: 'in-progress'
priority: 'critical'
type: 'refactor'
created: '2026-08-29'
updated: '2026-08-31'
depends_on: [372, 377]
stacked_on:
related: [318, 322, 326, 361, 366, 367, 369, 371, 372, 377]
discovered_from: [318]
adrs: [14, 29, 30, 33, 36, 74, 99]
spec: 'docs/superpowers/specs/2026-08-29-delete-the-frozen-bash-facade-and-legacy-test-surface-design.md'
plan: 'docs/superpowers/plans/2026-08-30-delete-the-frozen-bash-facade-and-legacy-test-surface.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/delete-the-frozen-bash-facade-and-legacy-test-surface'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-31T09:49:32Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-29-delete-the-frozen-bash-facade-and-legacy-test-surface-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-29-delete-the-frozen-bash-facade-and-legacy-test-surface-design.md) |
| Plan | [2026-08-30-delete-the-frozen-bash-facade-and-legacy-test-surface.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-30-delete-the-frozen-bash-facade-and-legacy-test-surface.md) |
| ADRs | [ADR-0014](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0014-consuming-repo-script-resolution.md), [ADR-0029](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0029-docket-facade-routing-and-config-presentation.md), [ADR-0030](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0030-facade-wiring-guard-discriminates-on-invocation-prefix.md), [ADR-0033](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0033-cursor-auto-run-trust-at-facade.md), [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

Once retained lifecycle consumers use Go, registered-agent invocation uses native host dispatch, and
deferred feature paths are sealed, the legacy Bash facade and its helper/runtime tree become
duplicate production machinery. Keeping them would preserve two control planes, two test
architectures, and the configuration seams the v1 cutover is intended to retire.

## What changes

- Require 0377 to land the missing native Go operations and cut retained workflow consumers over before deletion begins.
- Reconcile against the merged 0369 -> 0371 -> 0372 -> 0377 cutover and fail closed unless every shape-derived facade/runtime candidate is classified and no maintained executable caller remains.
- Classify each substantive legacy assertion before deletion; move surviving behavior to mutation-sensitive Go coverage or the retained POSIX owner for repository-root `install.sh` or the release downloader.
- Delete the facade, production helpers/runtime, legacy runner, compatibility launchers, environment/configuration seams, and mechanism-only tests.
- Contract `docket development test` to Go plus exactly the two retained POSIX product suites while preserving source fidelity, isolation, completeness, interruption, aggregation, budgets, and ADR-0074 semantics.
- Add shape-derived, mutation-tested final absence guards and complete the facade-era ADR/index consequences through the ADR workflow.

## Out of scope

Retained typed-operation migration (0369), native dispatch migration (0371), deferred-feature retirement and the earlier consumer seal (0372), and the missing native facade-operation migration plus workflow cutover (0377); a replacement shim or shell control plane; unrelated configuration redesign; rewrites of historical records, archived specs/results, Accepted ADR history, or frozen v0.9.2 artifacts; release/tag/assets; and human fresh-host or rollback work (0366).

## Design decisions

Deletion follows coverage replacement, never precedes it. Unknown consumers and uncertain test
assertions block removal. Final guards classify executable shape and ownership rather than pinning
current spellings, counts, or filenames. A small missed caller consistent with the merged cutover
may be reconciled; a material migration redesign halts for regrooming.

## Reconcile log

### 2026-08-30

Reconciled against the merged 0369 -> 0371 -> 0372 consumer-cutover chain. Confirmed current reality on origin/main: changes 0369 (retained-consumer typed-Go migration), 0371 (native host dispatch), and 0372 (deferred-feature retirement + consumer seal) are all archived at status done, so the base contains the whole prerequisite chain and the spec's opening premise holds. The frozen surface is still physically present and unused by maintained consumers, exactly as the spec assumes: scripts/docket.sh, scripts/lib/docket-runtime.sh, and scripts/run-tests.sh all resolve on origin/main; scripts/ still carries ~49 shell scripts across ~88 tracked files; ~188 shell/bats tests remain under tests/; and ~204 maintained references to DOCKET_SCRIPTS_DIR / DOCKET_BASH_PATH / runtime.bash remain outside the immutable archive/ADR/spec history. The canonical runner is the Go-native docket development test (internal/cli/development_test_cmd.go + internal/suiterunner), which still executes the Go-plus-legacy corpus and must contract to Go plus exactly the two retained POSIX product suites (repository-root install.sh and the release downloader). No fundamental invalidation: the design's shape-derived, coverage-before-deletion, fail-closed-on-unknowns approach is intact and no maintained consumer has already been removed out from under it. Relations (depends_on [372]; related 318/322/326/361/366/367/369/371/372; adrs 14/29/30/33/36/74/99) remain accurate and are left untouched. Scope, goals, and acceptance criteria stand as written; concrete counts above are review context only, never architectural gates, per the spec. Proceeding to plan and build. NOTE for follow-up capture: this change deletes the very facade (docket.sh / DOCKET_SCRIPTS_DIR) that installed docket skills invoke at runtime via the Step-0 preamble; the installed skill copies and their harness wiring are outside this repo's tree and outside 0370's deletion surface, so their migration off the retired env vars (if any remains) is separate operator work to be captured deliberately if not already covered by 0371's native-dispatch cutover.

### 2026-08-30

Human-approved regroom in response to the Task 1 halt. Captured change 0377 as the critical predecessor that owns the missing native facade-operation verbs and retained workflow cutover. Change 0370 now depends on 0377, and its proposal/spec explicitly reserve that migration to 0377 while retaining 0370's deletion-only scope. This reconcile intentionally leaves the existing `## Run halted` record, plan reference, feature branch, and no-PR state intact; it does not resume or re-dispatch the run.

### 2026-08-31

### 2026-08-31

Resume reconcile after the Task-1 halt cleared. The predecessor change 0377 (migrate deferred Bash-facade workflow operations to native Go) has merged: both depends_on entries (0372 and 0377) are now archived at status done. Verified directly against current origin/main (tip d010aa54, 15 commits ahead of the feature branch's former base b853e8c0):

- The class-1 blocker that halted the prior run is resolved. 0377 completed the maintained-consumer cutover (`b9cfe4ea feat(0377): complete maintained-consumer cutover; drop DOCKET_SCRIPTS_DIR/DOCKET_BASH_PATH dependence`, plus the status/ADR/convention and implement-next/finalize/stack skill cutovers). `git grep` over `skills/` on origin/main finds no live facade invocation (`docket.sh preflight|board|render|adr|stack|docket-status`) and no `DOCKET_SCRIPTS_DIR` dependence: the retained workflow skills now reach every operation through native `docket` verbs (this run's own Step-0 used `docket repository prepare`). The spec's premise — 0372/0377 leave the facade frozen and unused by maintained consumers — now holds.
- The facade surface the change deletes is still physically present on origin/main exactly as the plan assumes: `scripts/docket.sh`, `scripts/run-tests.sh`, the `scripts/lib/` runtime tree (docket-runtime.sh, docket-preflight.sh, docket-stack.sh, …), and `scripts/check-test-source-hygiene.sh` all resolve. The runner seam the change itself retires is intact: `internal/cli/development_test_cmd.go:70` still reads `DOCKET_BASH_PATH` (Task 6's disposition target, not a cutover blocker).

Base-move note for the build: the feature branch (plan commit e3068d6) was cut from b853e8c0; current origin/main is d010aa54. The feature branch must be rebased onto current origin/main before building so the deletion runs against the post-0377 tree. The plan's Task 1 Step 1 pins the literal base SHA b853e8c0 — the build verifies the facade surface is present against the ACTUAL resolved base (origin/main d010aa54), not that literal SHA; the plan's own `moving-base` guidance and "counts are review context only" clause cover this. Surviving-tree counts shifted (~88 tracked under scripts/, ~173 non-fixture tests) but remain non-gating review context per the spec.

No fundamental invalidation: the design's shape-derived, coverage-before-deletion, fail-closed-on-unknowns approach is intact and no maintained consumer has been removed out from under it. Relations (depends_on 372/377; related; adrs 14/29/30/33/36/74/99) remain accurate and are left untouched. Proceeding to plan-verify and build.

## What blocks the change

The spec's foundational assumption — "0372 leaves the facade frozen, unused by maintained consumers" — and Acceptance criterion 2 ("shape-derived reconciliation classifies all candidates with no unknown/error state or maintained consumer") are contradicted by the merged base. The retained docket workflow skills are still LIVE executable consumers of a whole family of Bash-facade operations that have NO Go CLI verb, and 0370's plan builds none:

| Facade op (deleted when scripts/docket.sh + scripts/lib/ go) | Go verb? | Live skill invocations verified from git |
|---|---|---|
| `docket.sh preflight` (Step-0 bootstrap of EVERY docket skill: config-resolve + metadata-worktree sync) | none | skills/docket-convention/SKILL.md:75 and every operating skill's Step 0 |
| `docket.sh docket-status --board-only` (the Board pass) | none | 12 live skill-command references |
| `docket.sh render-change-links` (the `## Artifacts` block writer) | none | 5 live references |
| `docket.sh render-adr-index` | none | 2 live references (docket-adr) |
| `docket.sh stack-base` / stack-children / stack-closeout | none | stacked-changes reference |
| `docket.sh adr-checks` | none | 1 live reference (docket-adr) |
| `docket.sh docket-status` (plain) | none | docket-status skill |

Verification performed directly (not taken on the worker's word): `git grep -nE 'Use:\s*"(preflight|env|board-refresh|render-change-links|render-adr-index|adr-checks|stack-base|stack-children|stack-closeout|docket-status)"'` over internal/**/*.go and cmd/**/*.go returns EXIT 1 (no such cobra verb for any of them), while the canonical skills/**/SKILL.md files still invoke `"${DOCKET_SCRIPTS_DIR}"/docket.sh preflight` etc. as their live Step-0/Board/ADR commands.

## Why this is regrooming, not a reconcile adjustment

- The merged cutover chain deliberately deferred exactly these ops. Change 0369's reconcile log lists preflight, board-refresh, render-change-links, stack-base/children/closeout, and adr-checks as "Frozen / unmapped — no landed Go verb, left unchanged and reported (not this change)," with the abort boundary "Reconciliation halts if an in-scope caller needs a new operation or substantial bespoke adapter." Change 0372 states these "remain maintained callers until change 370 deletes the facade."
- But 0370's plan provides no Go homes: Task 7 only "corrects active prose" (it cannot retarget a skill to a Go verb that does not exist), and Task 8 deletes scripts/docket.sh + scripts/lib/ (which every op script sources) plus the op scripts. Deleting the facade therefore removes docket's own bootstrap (`preflight`) and its Board/artifacts/stack/ADR lifecycle from every skill with nothing to replace them.
- 0370's own "Out of scope" explicitly excludes "Retained consumer migration (0369)," so 0370 cannot legitimately absorb this by widening scope. Providing Go homes is a new subsystem's worth of work (a Go preflight/bootstrap for config-resolve + metadata-worktree sync, a Board-pass verb, a change-links renderer verb, stack verbs, ADR-index/adr-checks verbs) — a material lifecycle-migration gap, which the spec's "Failure and recovery" section routes to REGROOMING, not to in-run reconcile.

## Recommended human action

A predecessor change must migrate the deferred facade operations (preflight/bootstrap, board-refresh, render-change-links, render-adr-index, adr-checks, stack-base/children/closeout, docket-status) to Go CLI verbs and cut the skills over to them, BEFORE 0370 can delete the facade. Alternatively, re-scope/re-groom 0370 to include that migration (which contradicts its current Out-of-scope boundary and its dependency framing). Either way this is a design decision for a human/groomer.

The base itself is healthy: b853e8c0 is the merged 0369/0371/0372 tip, and scripts/docket.sh, scripts/lib/, and scripts/run-tests.sh are all present as expected. The block is the plan's premise against that base, not a moved base. No code was written; the reconcile log entry and this halt record are the only run artifacts on the metadata branch, and the feature branch carries only the plan commit.
