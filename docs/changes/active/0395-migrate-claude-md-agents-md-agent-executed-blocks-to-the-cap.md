---
id: 395
slug: 'migrate-claude-md-agents-md-agent-executed-blocks-to-the-cap'
title: 'Migrate CLAUDE.md/AGENTS.md agent-executed blocks to the capability-first idiom'
status: 'in-progress'
priority: 'medium'
type: 'refactor'
created: '2026-09-02'
updated: '2026-09-02'
depends_on: [394]
stacked_on:
related: [394]
discovered_from: []
adrs: [104]
spec: 'docs/superpowers/specs/2026-09-02-migrate-claude-md-agents-md-agent-executed-blocks-to-the-cap-design.md'
plan: 'docs/superpowers/plans/2026-09-02-migrate-claude-md-agents-md-agent-executed-blocks-to-the-cap.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/migrate-claude-md-agents-md-agent-executed-blocks-to-the-cap'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-02T15:35:12Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-02-migrate-claude-md-agents-md-agent-executed-blocks-to-the-cap-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-02-migrate-claude-md-agents-md-agent-executed-blocks-to-the-cap-design.md) |
| Plan | [2026-09-02-migrate-claude-md-agents-md-agent-executed-blocks-to-the-cap.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-02-migrate-claude-md-agents-md-agent-executed-blocks-to-the-cap.md) |
| ADRs | [ADR-0104](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0104-the-capability-catalog-is-the-authoritative-executable-cli-s.md) |
<!-- docket:artifacts:end -->

## Why

Change 0394 introduced an authoritative compact CLI capability catalog and migrated the maintained workflow surfaces (skills/, agents/, cursor-rules/, scripts/, README, .docket.example.yml) to fetch it and resolve executable spellings from it, but its Task-6 migration inventory and its repoguard enforcement guard were both scoped to those surfaces only. The two repo-root instruction files CLAUDE.md and AGENTS.md were left outside that scope, so their agent-executed blocks still hard-code CLI argv and the resulting asymmetry with the migrated Cursor mirror is unflagged by any guard. Two blocks are affected: the "Run gate" block (hard-codes `docket run gate-before` / `docket run gate-verdict` argv) and the "Rebuild the binary after a merge to main" block (hard-codes `docket development install` argv). These are exactly the agent-executed argv the catalog exists to make authoritative — this session itself executed the un-migrated Run gate block. Governed by ADR-0104 (the capability catalog is the authoritative executable CLI surface).

## What changes

Migrate the "Run gate" block to the run.gate-before / run.gate-verdict catalog idiom and the "Rebuild the binary after a merge to main" block to the development.install catalog idiom, in both CLAUDE.md and AGENTS.md, preserving each block's operational contract while removing hard-coded argv. Extend the repoguard enforcement guard's scope to cover CLAUDE.md and AGENTS.md so this parity cannot silently regress, reconciling the guard's exemption model with the repo-root instruction files (which are human-and-agent-facing prose, distinct from the generated/guarded workflow surfaces the catalog primarily targets). Regenerate any embedded/derived assets affected by the guard-scope extension and add coverage proving the guard reddens on a re-introduced hard-coded argv in these files.

## Out of scope

Introducing new catalog leaves or changing the catalog protocol; migrating any surface not named here; JSON-schema/request-shape discovery (owned by change 0360); rewriting historical changes, specs, plans, results, or Accepted ADR prose.

## Reconcile log

### 2026-09-02

2026-09-02 — Reconciled against current `main`/`docket`. Dependency 394 is `done` and ADR-0104 exists; the capability catalog exposes `run.gate-before`, `run.gate-verdict`, and `development.install` as the target semantic operations, so the intended idiom is live. No design change; two structural facts sharpen the Approach:

1. The **"Run gate"** block in `CLAUDE.md`/`AGENTS.md` is NOT hand-authored prose — it sits inside the machine-managed `<!-- docket:dispatch:start … -->` block that `docket development install` reconciles into every parent instruction file from the bundled `cursor-rules/run-gate.md` asset (via `harness.DispatchInterior`). That embedded asset is ALREADY migrated (it byte-matches the migrated `cursor-rules/run-gate.md`); the committed `CLAUDE.md`/`AGENTS.md` dispatch blocks are merely stale because no install ran since 0394 migrated the asset. So this block is migrated by **regenerating the managed dispatch block** (matching the embedded interior), not by a prose hand-edit. Only the **"Rebuild the binary after a merge to main"** block is hand-authored prose outside the markers and is edited directly to drop the hard-coded `docket development install --source …` argv in favour of the catalog-resolved `development.install` idiom.

2. Open Question #1 resolves clean: the repoguard `capabilitySurfaceCorpus` draws from `MaintainedFiles`, which already walks the whole tree, so `CLAUDE.md`/`AGENTS.md` at repo root are in `maintainedPop` — covering them needs only an added corpus case (repo-root basename match), no structural change to surface enumeration. The two files carry no `docket repository migrate|init|configure-tests` or `docket change create` exempt spellings, so the guard's exemption pins are unaffected; the only executable-position argv the guard-shape check would surface in these files are the two named blocks' spellings (`docket run gate-before`, `docket run gate-verdict`, `docket development install`) — no beyond-scope agent-executed argv (Open Question #2 resolves: `go run ./cmd/docket development test` at CLAUDE.md line ~62 is left-bounded by `/` and does not match the shape). Scope, goals, and acceptance criteria stand.

## Run halted

### 2026-09-02

2026-09-02 — Halted at the Step-5 build gate: environmental, not a defect in this branch.

The implementation is complete and committed on `refactor/migrate-claude-md-agents-md-agent-executed-blocks-to-the-cap` (base `origin/main` @ 9793ec22): plan 51fa117e, Task 1 b40ef53d (regenerate the AGENTS.md docket:dispatch block through `harness.DispatchInterior` to the catalog idiom + reconcile `dispatchBudget` 400→450 for the 419-word actual), Task 2 d065dd9f (migrate the hand-authored "Rebuild the binary" block to the `development.install` catalog idiom + extend `TestCapabilitySurface`'s corpus to AGENTS.md/CLAUDE.md with a population-presence assert, mutation-tested both directions). Task 3 verified `go generate ./internal/assets` is a no-op and the affected packages are green. CLAUDE.md is a symlink to AGENTS.md; only AGENTS.md was edited and the alias was verified after every step.

The full-suite build gate (`go run ./cmd/docket development test`) was driven through the native gate driver TWICE and returned FAILED both times — each time on a DIFFERENT, unrelated concurrency test under the whole-module `-race` job's parallel load, and each serial-confirmed GREEN 3/3 in isolation:

- Run 1: `--- FAIL: TestRecoverLeavesUnprovableGroupForInspection` (internal/process) — race-recovery timing. Serial `go test -race -count=1 -run … ./internal/process/` → ok 3/3.
- Run 2: `--- FAIL: TestPrepareConcurrentDistinctTargets` (internal/workspace) — `fatal: failed to read .git/worktrees/…/commondir: Result too large` (EFBIG) on concurrent git-worktree adds. Serial `go test -race -count=1 -run … ./internal/workspace/` → ok 3/3.

Neither package is touched by change 0395 (diff = AGENTS.md + internal/repoguard/{budgets_test.go,capability_surface_test.go}); `internal/repoguard` passed in BOTH parallel runs (ok ~44s). Per the repo's serial-confirm discipline a parallel-load breach that passes serially is not an authoritative breach, so there is no defect to repair — but the native gate only mints green build evidence from a `PASSED` drive, so no green evidence could be minted and the branch could not be certified for a PR.

Why halt rather than repair or re-drive: the FAILED→integration-repair path exists to fix a real cross-task failure; here there is none — both failures are pre-existing, non-deterministic, serial-confirmed-green parallel-load flakes in unrelated packages (the whole-module `-race` job contends with the concurrent `test_go_toolchain`/`test_go_integration_release` jobs, hitting an OS filesystem limit on concurrent worktree creation and race-detector timing slips). Manufacturing a repair task for them is out of scope for a docs+guard change and cannot yield a real fix; fabricating green evidence is forbidden; repeatedly re-driving to gamble on a lucky green is not sound.

Human action needed (a SEPARATE change, matching the documented `internal/app`-timeout environmental-blocker precedent): make the whole-module `-race` suite job robust under the runner's full parallelism — e.g. reduce race-job concurrency, or isolate/serialize the concurrent-worktree and gate-recovery tests so a machine-dependent parallel-load flake stops reddening the gate for unrelated diffs. Once the gate can run green, resume this change (`change resume-halted --acknowledge-quiescent`) — its own packages are already green and the branch is complete; only the automated gate certification is outstanding.
