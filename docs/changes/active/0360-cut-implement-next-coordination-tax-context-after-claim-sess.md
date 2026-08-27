---
id: 360
slug: 'cut-implement-next-coordination-tax-context-after-claim-sess'
title: 'Cut implement-next coordination tax (context after claim, session-scoped sync, evidence from PASSED drives)'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-08-27'
updated: '2026-08-27'
depends_on: []
stacked_on:
related: [315, 357, 313, 335, 324, 247, 294]
discovered_from: []
adrs: [3, 47, 20, 66, 92, 95]
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0003](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0003-convention-reference-loading.md), [ADR-0047](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0047-digest-only-read-tier-skips-preflight.md), [ADR-0020](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0020-generated-agent-artifacts-machine-local.md), [ADR-0066](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md), [ADR-0092](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md), [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md) |
<!-- docket:artifacts:end -->

## Why

Observed live 2026-08-27 in PropertyBrands/scp-qarch-deploy while `docket-implement-next` built change 0003 (stacked on 0004, Cursor harness, Go CLI). The Helm work was two commits and a clean review. Most of the session tokens were spent on protocol narration: repeated `docket.sh preflight`, opaque blob-version CAS, a mandatory rewrite-style reconcile, skill/CLI schema archaeology, and gate/evidence config that the product already knew.

The concurrency story is real — shared `.docket/` metadata worktree, claim leases, fail-closed bootstrap — but a single claimed run is forced to re-prove the world on every seam as if a concurrent writer had just moved it. After claim, the advertised source of truth died: `docket context implementation --id 3` returned `invalid-state` / `not-ready-not-proposed`, so the parent fell back to `git hash-object` plus opening the markdown. Reconcile still required a full replacement of `## Why` / `## What changes` / `## Out of scope` plus a dated log even though the spec blob, stack parent, chart `1.2.8`, and helm tests were unchanged.

CLI/skill drift made it worse. Implement-next still describes markdown `--input`; `docket change reconcile --help` does not print the JSON schema; unknown-field errors do not list allowed keys. The parent probed until `"## Why"` worked. Blocking "read this now" files (auto-capture, dummy mode, gate-loop, stacked-changes) loaded even when `AUTO_CAPTURE_ENABLED=false` and dummy mode was off.

Build-phase additions, same run:

- `finalize.test_command` unset; chart tests are `__main__` scripts, not pytest. `docket evidence record` failed with `unconfigured-gate-command` *after* a `PASSED` drive that already had a raw run dir. Unblock required a gitignored `.docket.local.yml` *and copying it into the feature worktree* because `--repo-dir` did not see the primary tree's local config (ADR-0020 exists; worktree visibility does not).
- Gate argv `-- -- bash` became `/bin/sh: --: invalid option` and returned `FAILED`. The driver treats `FAILED` as suite-red (repair ladder). This was exec error, not a red test.
- `bash -c 'set -euo pipefail; …'` word-split so `-c` ran `set`, which dumped the process environment into gate logs (credential hazard plus log blow-up). Tests still printed `ok`.
- `mark-implemented` lost CAS: a fresh `hash-object` was already stale after PR publish / claim refresh. Another preflight + retry.
- The 1500-line review bump measures `origin/<integration_branch>...HEAD`. For a stacked child that counted the parent stack (~2891 lines → standard→deep). The child delta vs effective base was ~408 lines.

ADR-0047 already exists so `--digest-only` can skip preflight for selection, but implement-next still re-preflights around every metadata mutation and long dispatch. Change 0315 shipped the claim-to-implemented workflow; change 0357 taught implementation context to load remote branch facts *before* claim — it still does not serve `in-progress`. Change 0294 (killed) tried to shrink always-loaded AGENTS.md footprint for a related token-cost reason.

This is not "make CAS optional." It is: keep the concurrency invariants, stop making one agent pay the multi-agent tax on every step, and stop discarding a green gate because config auto-detect did not guess `__main__` python scripts.

## What changes

PM-altitude: make a solo `docket-implement-next` run cheap without weakening shared-worktree CAS. Design (groom) must pick a split; the live defects below are the acceptance bar.

**Keep (do not weaken):** claim CAS, stacked-on base resolution, no metadata writes on the feature branch, one plan commit with `Docket-Plan-Path:`, attach-plan identity checks, suite evidence bound to a SHA.

**Context after claim.** `docket context implementation --id N` (or a sibling `context run --id N`) must still return the bundle once the change is `in-progress`: new entity version, paths, spec bytes, related, effective base, halt flags, claim lease. A typed no-candidate/not-proposed refusal is correct for *selection*; it is wrong as the only context verb after a successful claim.

**Session-scoped sync.** One `preflight` per phase (claim, reconcile, plan, build, pr), not before every read. Mutations that already pushed should return `{version, metadata_commit}` so the parent does not SHA-compare in prose. A cheap `docket sync --if-stale` (or equivalent) beats a full fetch+rebase narrative when tips already match. Do not reprint the 40-line `KEY=value` block on a no-op sync.

**Return the next entity version on every mutation** (`claim`, `reconcile`, `refresh-claim`, `attach-plan`, `pr publish`). Parents must not `hash-object` then lose a race at `mark-implemented`.

**Reconcile no-op.** If spec blob, related tips, and effective-base SHA are unchanged: stamp `reconciled: true` plus a one-line log `unchanged vs <sha>`. Do not ask the model to rewrite Why/What/Out of scope.

**Fold lease refresh into long ops** (`workspace prepare`, plan-writer, build). Stop making `refresh-claim` a parent-visible step around every dispatch.

**CLI schema.** `--help` / `--json-schema` for `change reconcile` (and create/halt/PR body) must print allowed keys. Unknown-field errors must list them. Skill prose must match the Go request (JSON with `## Why` headings, not a markdown `--input` flag that does not exist).

**Runtime card vs novel.** A ~50-line "this run" card (commands, versions, halt rules). Open reference X only on error or when the exported flag is on. Auto-capture / dummy-mode / gate-execution docs must not be "read on arrival" when those flags are false.

**Skip unconditional status sweep** when the allowlist is one claim-eligible id. Sweep is a janitor, not a prerequisite for building that id.

**Do not double-verify the plan.** Parent git checks and `attach-plan` currently repeat the same facts. One authority.

**Workers get paths, not the spec again.** Pass `PLAN_PATH` + `Task N` (and constraints). Do not re-paste the full plan task into every profile worker.

**Evidence from a PASSED drive.** `docket evidence record` must accept the command the drive actually ran. A `PASSED` disposition plus raw run dir plus matching head is enough. Unset `finalize.test_command` must not discard a green observation (`unconfigured-gate-command` after `PASSED` is a product bug). Auto-detect should recognize `.charts/*/ci/test_*.py` `__main__` scripts or equivalent, or fail with a remedy that does not require copying `.docket.local.yml` by hand.

**Honor `.docket.local.yml` from the primary tree** when `--repo-dir` is a feature worktree (gitignored files are not in the worktree). Copying local config by hand is not a workflow.

**Gate `FAILED` vs exec-error.** `/bin/sh: invalid option` and similar launch failures must be `HALTED` (or a new `exec-error` disposition), never `FAILED` that feeds the integration-repair ladder.

**Suite argv as one string or an argv JSON array.** `bash -c 'set -euo pipefail; …'` must not collapse to `bash -c set`. A word-split that runs `set` dumps the environment (secrets) into durable gate logs.

**Stacked 1500-line review bump** must use the resolved effective base, not `origin/<integration_branch>...HEAD`. Otherwise every stacked child looks huge and is over-reviewed.

**Optional driver.** A `docket implement --id N` (or documented equivalent) that owns claim, version tracking, workspace prepare, refresh-claim, attach-plan in Go so the agent sees `workspace=… version=… head=…` is in scope if cheaper than teaching every skill to stop re-deriving blob ids. Split across stacked changes if one PR is too large — this stub is the umbrella; grooming may mint children.

**Open questions for grooming (no spec yet):**
1. One umbrella change vs a stack (context-after-claim / evidence-from-PASSED / gate-argv / skill-runtime-card as separate PRs)?
2. Is a Go `docket implement` driver in v1, or only skill+CLI contract fixes?
3. Reconcile no-op: hash which exact artifacts (spec blob only vs change body vs related SHAs)?
4. Should `context implementation --id` grow an `--in-progress` mode, or a new `context run` verb so selection stays proposed-only?
5. How to bind "session" for session-scoped sync without a new coordination key (process? claim lease? request id)?

## Out of scope

- Weakening claim CAS, shared-worktree stage-by-path, or fail-closed bootstrap for concurrent agents.
- Making reconcile optional in the sense of skipping the *pass* — a no-op stamp is still a pass.
- Replacing helm/chart tests in consuming repos; this is docket's gate/evidence/config contract.
- Dummy-mode copy, auto-capture policy, or learnings harvest.
- Changing `finalize.gate: off` semantics in consuming repos.
- Re-litigating ADR-0001 (metadata branch), ADR-0008 (agent layer), or ADR-0063 (profile-routed build) except where this change cites them as constraints.
- Killing or reviving change 0294 (AGENTS.md footprint); cite it as related token-cost history only.
- Fixing scp-qarch-deploy's committed `.docket.yml` (unset `test_command`) except as a dogfood case for auto-detect / evidence-from-PASSED.
- Force-push, hook skip, or rewriting published PRs.
