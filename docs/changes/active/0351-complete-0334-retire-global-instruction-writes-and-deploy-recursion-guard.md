---
id: 351
slug: complete-0334-retire-global-instruction-writes-and-deploy-recursion-guard
title: "Complete change 0334: stop writing global instruction files and actually deploy the recursion guard"
status: 'in-progress'
priority: critical
type: fix
created: 2026-08-26
updated: '2026-08-26'
depends_on: []
stacked_on:
related: [334, 294, 346]
discovered_from: [334]
adrs: []
spec: docs/superpowers/specs/2026-08-26-atomic-installer-handoff-and-repository-dispatch-seeding-design.md
plan: 'docs/superpowers/plans/2026-08-26-atomic-installer-handoff-and-repository-dispatch-seeding.md'
results:
trivial: false
auto_groomable:
branch: 'fix/complete-0334-retire-global-instruction-writes-and-deploy-recursion-guard'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-26T17:54:01Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-26-atomic-installer-handoff-and-repository-dispatch-seeding-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-26-atomic-installer-handoff-and-repository-dispatch-seeding-design.md) |
| Plan | [2026-08-26-atomic-installer-handoff-and-repository-dispatch-seeding.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-26-atomic-installer-handoff-and-repository-dispatch-seeding.md) |
<!-- docket:artifacts:end -->

## Why

Change 0334 merged the compact, non-recursive dispatch design, but a live development install did
not deliver its intended state. The Go harness planners still recreate parent-facing instructions
in personal global files, and the already-running executable renders wrappers after building the
new executable. A renderer-only change can therefore install the new binary with old wrappers on
the first run, leaving the recursion guard absent until a second install.

The global-write retirement and fresh-render defect share one installation transaction and remain
one change. Change 0346's stale-source-checkout problem is related but independent and is not folded
into this work.

## What changes

- Make the freshly built development binary the sole planner and mutator for one install invocation;
  the old binary only validates and builds the temporary candidate.
- Stop planning global parent-facing dispatch targets while retaining global skills and agent
  wrappers. Remove prior global blocks or rules only with exact Docket ownership proof; preserve and
  refuse on modified or malformed artifacts.
- Let `docket install` and `docket development install` discover the containing repository or accept
  `--repo-dir`. Reconcile only parent-facing surfaces selected by that repository's explicit
  `agent_harnesses`; no explicit selection means no repository write.
- Journal machine files, safe global cleanup, repository surfaces, and their isolated ownership
  records as one preflighted all-or-nothing operation.
- Verify the compact routing and installed recursion guard in fresh processes for all four harnesses.

## Out of scope

- Changing the compact dispatch rule or recursion-guard wording.
- Change 0346's source-update behavior and change 0349's finalize-resolver cap.
- Per-repository agent wrappers or skills.
- Full repository initialization, metadata-branch setup, Git commits, or legacy repository migration.

## Design decisions

The linked spec fixes the root cause with a fresh-binary handoff, scopes repository authority to an
explicit repo-layer `agent_harnesses`, isolates ownership per working tree, and extends the existing
journal across machine and repository targets. An explicit empty harness list retires unchanged
owned repository surfaces; an absent key touches nothing. Any target conflict aborts the entire run
before mutation, and synchronous failures roll back the complete operation.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-26

### 2026-08-26 — reconcile (docket-implement-next)

Verified against current reality before planning:

- Parent change 0334 is `done` (merged); this change correctly completes its unshipped installer state. `discovered_from: [334]` and `related: [334, 294, 346]` remain accurate (294 is `killed`, 346 is `proposed` and correctly kept independent — not folded in, per the spec's non-goals).
- Confirmed both defects still live in source: all four harness planners (e.g. `internal/harness/claude/claude.go`) still emit the `docket:dispatch` managed-block target under the harness's global `root`, and `internal/install/devmode.go` still has the currently-running binary plan and render the install (no fresh-binary candidate handoff). No `--repo-dir` flag, no repository `agent_harnesses` surface reconciliation, and no per-working-tree `<git-dir>/docket/install.json` ownership record exist yet.
- Scope, out-of-scope, and design decisions in the spec are current; change 0349 (finalize-resolver cap) remains out of scope. No relation, section, or spec edits required — proceeding to plan against the change and spec as written.

## Live repro & priority rationale (2026-08-26)

Bumped to **critical** — this is a live regression, not cleanup. `docket-implement-next` (whether dispatched or run via `/docket-implement-next`) self-dispatches recursively ~3 levels deep until the nested-subagent limit is reached, so no non-trivial change can be built through the dispatch/slash path right now.

Verified mechanism:

- The wrapper Claude Code actually loads in a repo is the **project-level** `.claude/agents/docket-*.md`. On this machine all 17 were the pre-0334 **no-guard** copies (untracked, dated Aug 18), shadowing the guarded user-level `~/.claude/agents/` copies. Without the guard, a running `docket-implement-next` reads the `docket:dispatch` block (in both the global `~/.claude/CLAUDE.md` and the project `CLAUDE.md`) and re-dispatches itself — the recursion.
- Local unblock applied while this change is pending: deleted the stale project-level `.claude/agents/docket-*.md` so the guarded user-level wrappers take effect (to be confirmed in a fresh session).

Open question for implementation: `## Out of scope` excludes per-repository agent wrappers and guard-wording changes, and the spec keeps wrappers global — but the copies that shadow the guard are **project-level** wrappers. Confirm 351's install/cleanup actually removes or updates project-level `.claude/agents/` wrappers (or documents deleting them); otherwise the guard stays shadowed in-repo even after 351 deploys it globally.

## Run halted

### 2026-08-27

### 2026-08-26

**Disposition:** halted (build gate cannot be certified green — environmental host oversubscription + a pre-existing gate-driver test flake; NOT a change-0351 code defect).
**When:** docket-implement-next Step 6 (build gate / evidence), after the full plan was built and committed.
**Phase reached:** All 12 plan tasks implemented and committed on `fix/complete-0334-retire-global-instruction-writes-and-deploy-recursion-guard`; the two real regressions the first gate surfaced are fixed and committed (`df4ef188`). The build-evidence record could not be minted because the full suite will not reach `failed=0` on this host.

### What was built (complete)

Twelve task commits on the feature branch (base `6e74df71`), plan attached (`plan:` set on the metadata branch):

- Task 1 `9ea73295` — `document.RemoveBlock` patch op
- Task 2 `912a03fd` — managed-block removal as a journaled transaction step
- Task 3 `0f3dda66` — retire global dispatch surfaces with ownership proof; adapters stop planning them
- Task 4 `c79aa7ea` — `agent_harnesses` typed repo-surface list with provenance
- Task 5 `d3916d78` — `gitcli.DiscoverWorktree`
- Task 6 `2aec6a07` — `internal/reposeed` parent-facing repository surface planner
- Task 7 `f4869a13` — per-worktree ownership record
- Task 8 `90f551d7` — single journaled transaction across machine, retirement, repository, both ownership records
- Task 9 `415155f3` — fresh-binary candidate handoff owns all development-install mutation (the recursion fix)
- Task 10 `39359127` — `--repo-dir`, explicit repository opt-in, scope-visible install reporting
- Task 11 `75e14bca` — docs: repository dispatch opt-in, retirement safety, fresh-process requirement
- Repair `df4ef188` — gofmt `internal/reposeed/plan_test.go` + trim `agent-layer.md` under the skill size budget (the two real first-gate failures)

Feature HEAD at halt: `df4ef188`. Worktree clean.

### Why the gate could not be certified green

The first full-suite drive was RED on two real, in-scope regressions from this branch, both now fixed and re-verified green in-suite: `test_go_toolchain` (a gofmt-unformatted `internal/reposeed/plan_test.go`) and `test_skill_size_budgets` (`skills/docket-convention/references/agent-layer.md` over its 205-line / 2350-word budget after the Task 11 edit — now 193 lines / 2341 words, substance preserved). These were committed in `df4ef188`.

After that fix, seven independent full-suite drives (two by me as controller, plus a premium and a max repair worker's runs) all reach `files=128 passed=127 failed=1` (or worse under peak load), and the sole remaining failure is a DIFFERENT environmental/flaky victim each run, never in a package this change touches:

- `internal/app` — `panic: test timed out after 10m0s` (the go-test 600s ceiling) mid-`os.WriteFile` in git-fixture tests (`TestFinalizeMergeDeniedCarriesMethod`, `TestClaimToImplementedWorkflow`, `TestFinalizeRebasePreconditions`). The same package PASSES at 53s–545s when the host is less loaded — load-sensitivity, not a hang or deadlock.
- `internal/gatedrive` (under `go test -race`) — `TestIntegrationDriverSlicesAcrossLiveChildThenPasses` fails on a `t.TempDir()` `RemoveAll` cleanup race: "directory not empty" because a detached gate child is still writing into the run dir at teardown. A pre-existing gate-driver test-cleanup flake, in a package change 0351 does not modify.

Root cause is host oversubscription: 16–41 concurrent `scripts/run-tests.sh` processes from OTHER worktrees' autonomous loops held 1-minute load averages of 30–92 for the entire build. Failure count scales directly with load (1 failure at load ~13, 5 failures at load ~92). This is exactly the machine-dependent parallel wall-clock breach the repo's own CLAUDE.md warns is confirmed serially, not a suite failure to repair.

Per the docket-build contract I must not weaken a test, bump the 600s go-test timeout, or fabricate a green build-evidence record, and the repair ladder (premium → max) is exhausted with both real regressions fixed. So the gate cannot be certified and no PR was opened.

### What a human needs to do

Re-run the suite gate once on this worktree when the host is not oversubscribed by sibling loops:

    /Users/homer/dev/docket/.worktrees/complete-0334-retire-global-instruction-writes-and-deploy-recursion-guard

Drive `scripts/run-tests.sh` via `docket gate drive` (or run it directly). Expected: `failed=0` with NO further code change — every failure observed was an environmental wall-clock timeout or the pre-existing `internal/gatedrive` TempDir cleanup flake. If `internal/gatedrive`'s `TestIntegrationDriverSlicesAcrossLiveChildThenPasses` still flakes on an idle host, that is a separate, pre-existing gate-driver test-cleanup bug (a live child racing `t.TempDir()` teardown) worth its own change — it is not introduced by 0351.

Then resume this run via:

    docket change resume-halted --id 351 --acknowledge-quiescent

On the resumed run, the branch is already fully built at `df4ef188`; only the build-gate → evidence → review → PR steps remain.
