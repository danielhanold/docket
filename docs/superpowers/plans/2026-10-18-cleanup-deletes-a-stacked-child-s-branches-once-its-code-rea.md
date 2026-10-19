<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0567 — Clean up stacked children when their stack lands, and finalize a child after its parent](../../changes/archive/2026-10-19-0567-cleanup-deletes-a-stacked-child-s-branches-once-its-code-rea.md)**
<!-- docket:backlink:end -->

# Clean up stacked children when their stack lands, and finalize a child after its parent — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build`, task by task, under the `docket-build-task` contract. Steps use checkbox (`- [ ]`) syntax for tracking. Each task names its build tier.

**Goal:** When a stack root lands, finalize and the sweep remove every carried child's worktree folder, local branch, and remote branch right after the root's, once cleanup proves the child's work reached the integration branch. A child whose parent landed first can be finalized normally.

**Architecture:** Three changes, each in the layer that owns it. (1) `internal/workspace`: a `ready` or `cleaned` manifest stops comparing `BaseRef` when deciding ownership, so a finished checkout stays its change's own after the effective base moves. An `allocating` manifest still has to match. (2) `internal/domain` + `internal/app`: closeout's carry walk (`proveCarry`) and its per-descendant content proof (inside `closeoutIntegrationDestination`) each become one shared helper. `finalizeCleanupDone` calls both to prove a `done` carried child before it refuses `destination-mismatch`, then runs its normal legs, with the content proof standing in for the local leg's merge-chain containment check. (3) `internal/app` sweep: `sweepRunCloseout` cleans each carried id after the root, and the carried-descendant presence exception added by #562 is removed.

**Tech Stack:** Go (module `github.com/danielhanold/docket`), real-git integration tests (`-tags integration`), the CLI end-to-end matrix (`-tags e2e`), markdown skill/docs files, the embedded skill tree (`go generate ./internal/assets/`).

**Spec:** `docs/superpowers/specs/2026-10-18-cleanup-deletes-a-stacked-child-s-branches-once-its-code-rea-design.md` (on the metadata branch; a copy rides the feature branch). Read it first. Its *Design* §1–§5 are the requirements this plan implements.

## Global Constraints

- Fail closed. A probe that errors (GitHub PR read, fetch, ancestry, preservation) is **unknown**: the result is `pending`, everything is kept, and nothing is deleted. It never reads as "not carried" or "proven". (learning `probe-error-is-not-clean-absence`)
- One copy of each proof. Closeout and cleanup call the **same** relationship walk and the **same** content-proof helper. No second walk, no copied predicate. (learning `duplicated-gate-copies-the-whole-predicate`)
- When the carry can't be proven, the message never gives "delete it by hand" advice or a `git branch -D` / `git push … --delete` command, and the new reason is **not** an `isKeptBranchFinding` code.
- Records, rendered tables (*Stack carried*, *Stacked children*), and the sweep inventory never authorize a delete. Every destructive leg still runs through a fresh prepare and `FinalizeCleanup`'s live proofs.
- An unstacked change pays no new probe. A carried child's cleanup adds one `ProbeMerged` per stack ancestor up to the root, plus the content proof.
- Ordinary (unstacked) cleanup behavior is unchanged, including removing a clean worktree folder when the branch is then kept.
- Docs describe current behavior only, with no change or PR citations (ADR citations are fine). Cross-references in maintained source anchor on symbol names, never line numbers.
- New integration tests use an existing shard prefix (`TestIntegrationWorkspaceLifecycle…`, `TestIntegrationFinalizeCleanup…`, `TestIntegrationFinalizeArchive…`, `TestIntegrationSweep…`), or `tests/test_go_integration_contract.sh` fails. New end-to-end tests are named `TestE2E…` in `internal/app/finalize_e2e_test.go`.
- Run every Go test that re-verifies or mutates with `-count=1`. A mutation restore uses a backup copy (`cp f f.bak` … `mv -f f.bak f`), never `git checkout --`.
- ADR: Task 1's rule changes what identifies a workspace. implement-next records that ADR at Step 6 through `docket-adr`, so no build task writes ADR files.
- Whole-suite gate: `build.test_command` (`go run ./cmd/docket development test`), run by docket-build at the end. Read the budget report even when it's green. `tests/test_go_finalize_e2e.sh` (30s, parallel) and `tests/test_go_integration_app_cleanup.sh` (40s, parallel) both gain tests here.

## Review Focus

1. **A carried child cleaned right after its root deleted the root's branch.** The child's merge-result commit is now reachable only through objects already in the local store. The proof must still pass in the same run, and a missing object must read as `pending` (unknown), never as a delete. Task 4's happy path runs root cleanup before child cleanup to pin this.
2. **A `cleaned` tombstone whose base differs.** After Task 1 it reads as owned (`StateCleaned`, and cleanup returns already-clean) instead of foreign. The sweep must then report it as no work, not blocked. Task 1 pins inspect and cleanup on this state.
3. **A child cleaned twice in one full sweep.** The child sits in the pinned cleanup worklist (it was `stacked-merged`) and is also cleaned by the root's closeout suffix. It must get one cleanup outcome and one dispatch. Task 5 pins this both with and without the assessment seam.
4. **An ancestor link that is a results-artifact or has no PR reference.** The walk has no facts for that link, so the relationship fails (`destination-mismatch` refusal, everything kept). Task 2 pins it in the domain.
5. **Sweep traffic and request-budget tests.** Carried-child cleanups now run inside the closeout suffix. Any traffic test that drives a root carry changes its counts. Re-derive the counts from the new behavior; don't widen tolerances. Records with nothing left to clean must still cost nothing. The whole-suite gate surfaces these.

---

### Task 1: A finished workspace's identity excludes its base

**Build tier:** premium

**Files:**
- Modify: `internal/workspace/prepare.go`: `ownsManifest` and its doc comment.
- Test: `internal/workspace/prepare_test.go` (unit, untagged).
- Test: `internal/workspace/inspect_integration_test.go`. Add a new `TestIntegrationWorkspaceLifecycle…` test here or in a sibling `*_integration_test.go` in the same package.

**Interfaces:**
- Produces: `ownsManifest(m Manifest, repo gitcli.Repository, target Target, intendedPath string) bool` keeps its signature. The new rule: `m.BaseRef == target.BaseRef` is required **only when** `m.Phase == PhaseAllocating`. Every other identity field is still compared exactly: `CommonDir`, `ID`, `FeatureRef`, `ChangeID`, `Slug`, `Path`. Don't relax any of them toward a prefix or "non-empty" test (learning `identity-match-relaxed-to-prefix-is-vacuous`). A separate predicate is fine if you prefer one, as long as every existing `ownsManifest` caller (prepare, inspect, cleanup, publish, checkout) gets the new rule.

- [ ] **Step 1: Write the failing unit test** `TestOwnsManifestBaseRefByPhase` in `prepare_test.go`. Build one owned `Manifest` from a `Target` (the same fields `Prepare` writes), then table over:
  - `PhaseReady` with `BaseRef` set to a different valid branch ref (`refs/remotes/origin/main` vs `refs/remotes/origin/feat/parent`), expected **owned**
  - `PhaseCleaned` with a different `BaseRef`, expected **owned**
  - `PhaseAllocating` with a different `BaseRef`, expected **not owned**
  - `PhaseReady` with matching `BaseRef`, expected owned (control)
  - `PhaseReady` with a different `ChangeID`, `Slug`, `FeatureRef`, `Path`, or `CommonDir` (one row each), expected **not owned**

- [ ] **Step 2: Write the failing integration test** `TestIntegrationWorkspaceLifecycleOwnedAfterBaseChange`. Use `mainModeRepo`, `freshTarget`, `prepareOK`, `loadManifest`/`writeManifest`, and `metaDirOf`, the same rewrite idiom `TestIntegrationWorkspaceLifecycleInspectReadyAfterParentRebase` uses. After `prepareOK`, rewrite the manifest's `BaseRef` to `refs/remotes/origin/feat/parent` (a stacked child's old base). Then add subtests:
  - `inspect-ready`: `Inspect` returns `Kind == StateReady`. The returned `BaseCommit` is still the manifest's.
  - `prepare-existing`: `Prepare` returns `Disposition == PrepareExisting`.
  - `publish`: commit one file in the worktree, then `PublishHead` succeeds (not `KindInvalidState` "manifest identity does not match").
  - `cleanup-then-inspect-cleaned`: `Cleanup` returns `CleanupCleaned`, the folder is gone, and a following `Inspect` returns `StateCleaned` (not `StateForeign`). A second `Cleanup` returns `CleanupAlreadyClean`.
  - `allocating-stays-blocked`: also set `Phase = PhaseAllocating` (remove the worktree first so the state looks like an interrupted allocation). `Prepare` returns `PrepareBlocked`, and `Inspect` returns `StateForeign` with detail "manifest identity does not match".

- [ ] **Step 3: Run both tests and confirm they fail** for the ready/cleaned rows only:
  `go test -count=1 -run '^TestOwnsManifestBaseRefByPhase$' ./internal/workspace`
  `go test -tags integration -count=1 -run '^TestIntegrationWorkspaceLifecycleOwnedAfterBaseChange$' ./internal/workspace`

- [ ] **Step 4: Implement the phase rule in `ownsManifest`.** Rewrite its doc comment to say why: a finished checkout's effective base legitimately moves when a stack parent lands (ADR-0092). An interrupted allocation resumes from its recorded `BaseCommit`, so it must still match the base ref. `BaseRef`/`BaseCommit` stay recorded as history.

- [ ] **Step 5: Re-run both tests (PASS), then the package:** `go test -count=1 ./internal/workspace && go test -tags integration -count=1 ./internal/workspace`. The checkout matrix row `foreign-manifest-unowned` writes an **allocating** manifest with another base, so it must stay blocked. If any other existing test asserted that a ready/cleaned base mismatch is blocked, ask what it guards (learning `test-premise-deleted-not-regated`), and invert or delete it on that basis. Don't re-gate it.

- [ ] **Step 6: Mutation check.** Back up `prepare.go`, restore the unconditional `m.BaseRef == target.BaseRef`, and confirm both new tests go red. Then restore from the backup.

- [ ] **Step 7: Commit** `fix(workspace): a finished checkout stays its change's own after its base moves`.

---

### Task 2: One carry walk for closeout and cleanup

**Build tier:** premium

**Files:**
- Modify: `internal/domain/stackcloseout.go`
- Test: `internal/domain/stackcloseout_test.go`

**Interfaces:**
- Produces:
  - `const carryNotDone = "not-done"`, a new refusal token (the start record is not `done`).
  - An unexported shared walk, for example `walkCarryChain(s Snapshot, start Change, facts map[ChangeID]PRFacts, linkStatus Status, isRoot func(parent Change) bool) (ChangeID, string)`. It holds the loop now inside `proveCarry`, in its current order: visited-set cycle check (`carryCycle`), `StackParent` lookup (`carryChainBroken`), killed parent (`carryKilledAncestor`), the current node's facts merged (`carryPRUnknown`), and its `BaseRef` equal to the parent's recorded branch (`carryDestinationMismatch`). Then `isRoot(parent)` returns `(parent.ID(), "")`. Otherwise `parent.Status() != linkStatus` returns `carryChainBroken`, and the walk ascends.
  - `proveCarry(s, root, id, facts)` keeps its signature and behavior. It keeps the `StatusStackedMerged` start check, then calls the walk with `linkStatus = StatusStackedMerged` and `isRoot = parent.ID() == root`.
  - `func ProveDoneCarry(s Snapshot, id ChangeID, facts map[ChangeID]PRFacts, integration string) (root ChangeID, token string)`. If the start is not found, it returns `carryChainBroken`. If the start is not `StatusDone`, it returns `carryNotDone`. Otherwise it runs the shared walk with `linkStatus = StatusDone` and `isRoot = parent.Status() == StatusDone && facts[parent.ID()]` is merged with `BaseRef == integration`. On success `root` is that ancestor's id and `token == ""`. On failure `root` is zero.

- [ ] **Step 1: Write the failing tests** in `stackcloseout_test.go`, using `stackSnapshot`, `stackSpec`, `parentEdge`, and `mergedInto`:
  - `TestProveDoneCarryTwoLevel`: 1 `done` `feat/1`, 2 `done` on 1 `feat/2`. Facts `{1: mergedInto("main"), 2: mergedInto("feat/1")}` give `(1, "")`.
  - `TestProveDoneCarryThreeLevel`: adds 3 `done` on 2 `feat/3` with `3: mergedInto("feat/2")`. `ProveDoneCarry(s, 3, …, "main")` gives `(1, "")`.
  - `TestProveDoneCarryRefusals`, a table:
    - the child merged into `feat/other`: `destination-mismatch`
    - the parent `killed`: `killed-ancestor`
    - the child's facts missing: `pr-unknown`
    - the parent `done` with no facts and no parent of its own: `chain-broken` (Review Focus 4)
    - the parent `stacked-merged` (the root not yet landed): `chain-broken`
    - the start `stacked-merged`: `not-done`
    - a cycle 2↔3 with neither merged into main: `cycle`
    - the start id absent: `chain-broken`
  - `TestProveDoneCarryUnstackedChildIsNotCarried`: a `done` change with no `stacked_on` returns `chain-broken`. The caller only consults the walk when the destination is not the integration branch.
  - Leave the existing `TestDeriveRootCloseoutSet*` tests unchanged. They pin `proveCarry`.

- [ ] **Step 2: Run and confirm the new tests fail to compile or fail:** `go test -count=1 -run 'ProveDoneCarry' ./internal/domain`

- [ ] **Step 3: Implement** the shared walk, re-express `proveCarry` through it, and add `ProveDoneCarry`. Doc comments say the relationship is the same walk closeout uses, applied to a population that is now `done`, and that content preservation is the caller's separate obligation.

- [ ] **Step 4: Run** `go test -count=1 ./internal/domain`. Expect PASS, including every existing `DeriveRootCloseoutSet`/`DeriveCarriedSet` test.

- [ ] **Step 5: Mutation check (one shared walk).** Back up the file. In the shared walk, delete the `carryDestinationMismatch` comparison. Confirm a `TestDeriveRootCloseoutSetRefusals` row **and** the `TestProveDoneCarryRefusals` destination row both go red. Restore from the backup.

- [ ] **Step 6: Commit** `refactor(domain): share the stack carry walk and prove a done child's carry`.

---

### Task 3: One content proof for closeout and cleanup

**Build tier:** premium

**Files:**
- Modify: `internal/app/finalize_preservation.go`: add the helper.
- Modify: `internal/app/finalize_closeout.go`: `closeoutIntegrationDestination`'s per-descendant loop calls the helper. `probeDescendantFacts` uses the extracted facts converter.
- Modify: `internal/app/finalize_context.go`: `FinalizeCleanupGit` gains `ProvePreserved`.
- Modify: `internal/app/finalize_cleanup_test.go`: `faultyCleanupGit` gains `ProvePreserved` with fault key `"preserve"`.
- Test: a new untagged `internal/app/finalize_carry_content_test.go`.

**Interfaces:**
- Produces:
  - `type carryContentGit interface { IsAncestor(ctx context.Context, repo gitcli.Repository, ancestor, descendant gitcli.ObjectID) (bool, error); ProvePreserved(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, source, target gitcli.ObjectID) (gitcli.PreservationCheck, error) }`. `*gitcli.Client` and `FinalizeCleanupGit` both satisfy it.
  - `type carriedContent struct { Proven bool; Category string; Check gitcli.PreservationCheck }`. `Category` is `""` when proven, `CarryFindingMissingMerge` when the child merge id is not a valid full object id, and `CarryFindingUnpreserved` when the proof ran and came back unproven, with `Check` set.
  - `func proveCarriedContent(ctx context.Context, git carryContentGit, repo gitcli.Repository, childMerge string, rootMerge, integrationTip gitcli.ObjectID) (carriedContent, error)`. It is exactly closeout's per-descendant body: an invalid `childMerge` gives MissingMerge. If `IsAncestor(childMerge, integrationTip)` has a nil error and returns true, the result is proven. A false answer **or an error** falls through to `ProvePreserved(childMerge, rootMerge)`, keeping the existing comment that explains why. A `ProvePreserved` error is returned as an error. A proven outcome is proven; anything else is Unpreserved.
  - `func mergedPRFacts(number int, outcome githubcli.MergeOutcome, mf githubcli.MergedFacts) domain.PRFacts`: the conversion now inlined in `probeDescendantFacts` (merged gives `State: "merged"` with every field; otherwise `State: "closed"`).
  - `FinalizeCleanupGit` adds `ProvePreserved(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, source, target gitcli.ObjectID) (gitcli.PreservationCheck, error)`.

- [ ] **Step 1: Write the failing unit tests** in `finalize_carry_content_test.go` with a scripted fake `carryContentGit` that records calls:
  - `TestProveCarriedContentAncestryShortCircuits`: `IsAncestor` true makes the result proven, with zero `ProvePreserved` calls.
  - `TestProveCarriedContentAncestryErrorFallsThrough`: an `IsAncestor` error plus a proven `ProvePreserved` makes the result proven, with exactly one `ProvePreserved(childMerge, rootMerge)` call. The arguments are asserted: source is the child, target is the **root** merge, never the tip.
  - `TestProveCarriedContentUnpreserved`: an `IsAncestor` false plus `PreservationUnproven` with `Detail: gitcli.PreserveEntryDiffers` makes `Category == CarryFindingUnpreserved`, with `Check.Detail` carried.
  - `TestProveCarriedContentMissingMerge`: `childMerge = "abc"` gives `Category == CarryFindingMissingMerge`, with zero git calls.
  - `TestProveCarriedContentObservationError`: a `ProvePreserved` error is returned as an error, and the zero `carriedContent` is not proven.

- [ ] **Step 2: Run and confirm the tests fail to compile:** `go test -count=1 -run 'ProveCarriedContent' ./internal/app`

- [ ] **Step 3: Implement** the helper and the facts converter. Replace closeout's inline loop body with one `proveCarriedContent(ctx, deps.Planning.Client, cc.repo, mergeID, gitcli.ObjectID(facts.MergeCommit), rev.Commit)` call. Map its outcomes to exactly today's results: MissingMerge gives the `ReasonCloseoutChildUnproven` refusal with today's message, an error gives `ResultExternalFailed`/`CloseoutDispUnknown`/`ReasonCloseoutDestinationProbe`, and Unpreserved gives the `ReasonCloseoutChildUnproven` refusal naming `CarryFindingUnpreserved`, `check.Detail`, `pathsSuffix(check.Paths)`, and source → target. Add `ProvePreserved` to `FinalizeCleanupGit` and `faultyCleanupGit` (`fail == "preserve"` returns `errCleanupProbe`, otherwise delegate to `inner`).

- [ ] **Step 4: Run the closeout suites unchanged:**
  `go test -count=1 ./internal/app -run 'ProveCarriedContent|Closeout'`
  `go test -tags integration -count=1 -run '^TestIntegrationFinalizeArchive' ./internal/app`
  Expect PASS with no edits to existing closeout tests. This step is a pure extraction.

- [ ] **Step 5: Commit** `refactor(finalize): share the carried-descendant content proof`.

---

### Task 4: Cleanup proves a carried child, then cleans it normally

**Build tier:** premium

**Files:**
- Modify: `internal/app/finalize_cleanup.go`: `finalizeCleanupDone`, `finalizeCleanupLocalRef`, the new reason constant, the new helpers, and the file header comment's stacked-merged/carried sentences.
- Modify: `internal/app/finalize_closeout.go`: `probeDescendantFacts` uses `mergedPRFacts` (if Task 3 didn't already).
- Test: `internal/app/finalize_cleanup_integration_test.go`: the new carried-child fixture and tests, plus an update to `TestIntegrationFinalizeCleanupPRBacklinkRetryCarriedDescendant`.

**Interfaces:**
- Consumes: `domain.ProveDoneCarry` (Task 2), `proveCarriedContent`, `mergedPRFacts`, and `FinalizeCleanupGit.ProvePreserved` (Task 3). Task 1's ownership rule makes the child's workspace leg pass.
- Produces:
  - `const ReasonCleanupCarryUnproven = "carry-unproven"` (or a better name in the same house style). It is **not** added to `isKeptBranchFinding`.
  - `func probeCarryAncestorFacts(ctx context.Context, deps FinalizeDeps, ghRepo githubcli.Repository, snap domain.Snapshot, c domain.Change, integration string) (map[domain.ChangeID]domain.PRFacts, error)`. It walks `domain.StackAncestors(snap, c)` nearest first and probes each ancestor's PR (`deps.GitHub.ProbeMerged`, converted with `mergedPRFacts`). It stops right after the first ancestor merged into `integration`. An ancestor with no canonical PR reference contributes no facts. Any probe error is returned.
  - `func finalizeCleanupProveCarried(ctx context.Context, deps FinalizeDeps, git FinalizeCleanupGit, ghRepo githubcli.Repository, cc *closeoutContext, childFacts githubcli.MergedFacts) *CleanupOpResult`. It returns nil when relationship **and** content are proven. Otherwise it returns the refusal, mapped like this:

    | Condition | Result / disposition / reason |
    |---|---|
    | ancestor probe error | `ResultExternalFailed` / `pending` / `ReasonCleanupProbeUnknown`, quiet retryable message |
    | `ProveDoneCarry` token ≠ "" | today's `ResultBlocked` / `pending` / `ReasonCleanupDestination` refusal, today's message unchanged |
    | integration fetch, root ancestry, or `proveCarriedContent` error | `ResultExternalFailed` / `pending` / `ReasonCleanupAncestryProbe` |
    | root merge id not a valid full id, root merge not an ancestor of the fresh integration tip, or content Category ≠ "" | `ResultBlocked` / `pending` / `ReasonCleanupCarryUnproven`, plain message per the rule below |

    The `carry-unproven` message follows the house finding style (what happened, why it matters, the remedy). It names the change and the integration branch, says the work could not be found there through its stack, so the folder and branches were kept, says the branch may be the only copy of that work, and says cleanup retries on the next full sweep. It contains no `git branch -D`, no `--delete`, and no "by hand".
  - `finalizeCleanupLocalRef` gains a final parameter `containmentProven bool`. When it is true, the fetch + merge-commit ancestry block is skipped. The tip equal to `facts.HeadOID`, the worktree-detached check, and `DeleteLocalBranchChecked` all still run.

- [ ] **Step 1: Build the carried-child fixture** in `finalize_cleanup_integration_test.go`, next to `TestIntegrationFinalizeCleanupPRBacklinkRetryCarriedDescendant`, reusing `setupCloseoutFixture`, `carryCommit`, `fetchAllIntoInvocation`, `mergedFactsFor`, `closeoutRecord`, `fakeCloseoutGitHub`/`fakeCleanupGitHub`, and `cleanupDeps`. The fixture builds a real stack:
  - Root 5 (`feat/widget`, the existing fixture workspace).
  - Child 6 `gadget`: `stacked-merged`, `stacked_on: 5`, `branch: feat/gadget`, PR `#8`. It gets a **real prepared workspace** (`f.svc.Prepare` with `workspace.NewTarget(6, "gadget", domain.EffectiveBase{Kind: domain.BaseResolved, Branch: "feat/" + f.slug}, "feat/gadget")`), one commit, and `PublishHead`.
  - The child merge into `feat/widget` as a **rebase-style** merge: cherry-pick the child commit onto `feat/widget` in the writer clone and push. The child's merge result is that new commit.
  - The root merge into `main` as a **rebase-style** merge: cherry-pick the root branch's commits onto `main` and push. The root merge result is the new `main` tip, so neither merge result is an ancestor of `main` and the `ProvePreserved` arm is exercised.
  - An optional grandchild 7 (`feat/gizmo`, PR `#9`) stacked on 6 the same way, for the three-level case.
  - `FinalizeCloseout(root)` returns `root-archived` with `CarriedIDs` covering 6 (and 7).

  Name the helper, for example, `setupCarriedStackFixture(t, depth int) *carriedStackFixture`. Expose each member's id, branch, workspace path, merged head, and merge result, and a fake whose `merged` map covers PRs 7, 8, and 9.

- [ ] **Step 2: Write the failing tests** (`TestIntegrationFinalizeCleanupCarried…`):
  - `TestIntegrationFinalizeCleanupCarriedChildTwoLevel`: `FinalizeCleanup(5)` then `FinalizeCleanup(6)`. Root cleanup runs first and deletes `feat/widget` (Review Focus 1). Child result: `ResultApplied`/`cleaned`. Its workspace folder doesn't exist, `refs/heads/feat/gadget` is gone locally, and `feat/gadget` is gone on origin. `RemovedRefs` contains `local:feat/gadget` and `remote:feat/gadget`. A replay is a no-op/already-clean.
  - `TestIntegrationFinalizeCleanupCarriedChildThreeLevel`: depth 3. Clean 5, then 6, then 7. All three folders and all six branches are gone.
  - `TestIntegrationFinalizeCleanupCarriedChildFailClosed`, one subtest per row. Each asserts nothing was removed (child folder exists, local and remote `feat/gadget` present), `pending`, and the reason:
    - `content-unproven`: the child's facts point `MergeCommit` at a pushed commit carrying content absent from `main` (a `carryStackDroppedContent`-style sibling). Expect `ReasonCleanupCarryUnproven`. The message names `main`, and neither the message nor `HumanText()` contains `git branch -D`, `--delete`, or `by hand`. No finding satisfies `isKeptBranchFinding`.
    - `ancestor-probe-error`: a `fakeCleanupGitHub` variant that errors only for root PR `#7` after the child's probe succeeds. Expect `ResultExternalFailed` with `ReasonCleanupProbeUnknown`.
    - `preserve-probe-error`: `faultyCleanupGit{fail: "preserve"}` gives `ReasonCleanupAncestryProbe`.
    - `destination-matches-no-parent`: the child's facts `BaseRef: "feat/elsewhere"` give `ReasonCleanupDestination`.
    - `killed-link`: three-level, with 6 rewritten to `killed` on the metadata branch before cleaning 7. Expect `ReasonCleanupDestination`.
  - `TestIntegrationFinalizeCleanupCarriedChildBranchMoved`: after the root closes, commit once more on the child's local `feat/gadget`, then clean the child. The workspace is removed, and the local branch is kept with a `ReasonCleanupTipMoved` kept-branch finding (#562's named warning).
  - `TestIntegrationFinalizeCleanupCarriedChildOpenStackedPR`: two-level plus a live (in-progress) grandchild record whose PR is **open** with base `feat/gadget` in `openByHead`. Child cleanup keeps the remote `feat/gadget` and returns `children-retarget-required`.
  - Update `TestIntegrationFinalizeCleanupPRBacklinkRetryCarriedDescendant`. Its child has no workspace of its own, so after this change the relationship and content are proven and cleanup stops at the workspace leg (no manifest gives `workspace-blocked`). Keep what the test guards: the backlink repoint runs first, and nothing is removed while the root's local and remote branches stay intact. Replace the `ReasonCleanupDestination` expectations with `pending`, nothing removed, and the `pr-backlink-pending` finding on the failing retry. Rewrite its doc comment to match.

- [ ] **Step 3: Run and confirm they fail** (today every row refuses `destination-mismatch`):
  `go test -tags integration -count=1 -run '^TestIntegrationFinalizeCleanupCarried' ./internal/app`

- [ ] **Step 4: Implement.** In `finalizeCleanupDone`, move `featureBranch`/`git := cleanupGit(deps)` above the destination check. Replace the bare `facts.BaseRef != cc.integrationBranch` refusal with a `finalizeCleanupProveCarried` call, keeping the backlink findings appended to any refusal, and set `carried := true` on success. Pass `carried` into `finalizeCleanupLocalRef`. Leave the remote leg unchanged. Update the comments that call a carried descendant a permanent destination-mismatch (the leg-1 comment and the file header).

- [ ] **Step 5: Run** the new tests (PASS), then `go test -tags integration -count=1 -run '^TestIntegrationFinalizeCleanup' ./internal/app` and `go test -count=1 ./internal/app -run 'Cleanup'`.

- [ ] **Step 6: Mutation checks.** Each uses a backup copy and must turn the named tests red.
  - (a) Make `proveCarriedContent` return `carriedContent{}` without proving. The carried-child happy paths **and** a `TestIntegrationFinalizeArchive` root-carry test must both go red. This proves the helper is shared.
  - (b) Drop the root-ancestry check. The `content-unproven` row still reddens, or add a row (a root merge id not on `main`) that does.
  - (c) Pass `containmentProven: false` for carried children. The two-level happy path goes red.

  Restore each from its backup.

- [ ] **Step 7: Commit** `fix(cleanup): prove a carried stacked child's work reached the integration branch, then clean it`.

---

### Task 5: The sweep cleans carried children right after the root

**Build tier:** standard

**Files:**
- Modify: `internal/app/maintenance.go`: `sweepRunCloseout` and the item loop in `maintenanceSweep`.
- Test: `internal/app/maintenance_test.go`

**Interfaces:**
- Consumes: `CloseoutResult.CarriedIDs` and `sweepRunCleanup(ctx, ops, id)` (each prepares its own fresh observation).
- Produces: `sweepRunCloseout` appends, after the root's cleanup entry, one `sweepRunCleanup` entry per id in `res.CarriedIDs`, in order. That happens only on the path where the root's cleanup ran (closeout applied or no-op). A child's entry never changes the root's entry or another child's. In `maintenanceSweep`, an id the closeout suffix already cleaned in this invocation is **not** dispatched again from the cleanup worklist, and any pre-dispatch assessment entry for it (`Kind == sweepKindCleanup`, empty `Operation`) is dropped. The mechanism is your choice. One way: collect suffix-cleaned ids while running closeout items, skip them in the `sweepKindCleanup` case, and filter `assessEntries` before the final sort.

- [ ] **Step 1: Write the failing tests** with `recordingSweepOps`:
  - `TestSweepCloseoutCleansCarriedChildren`: the corpus from `TestSweepFindsMergedImplemented`, with 30 root-archived carrying `[31, 32]`. Expect cleanup dispatches for 30, 31, and 32, with each cleanup's `obs` a distinct observation (`obsFor`). Exactly one cleanup entry each for 31 and 32.
  - `TestSweepCarriedChildCleanupIsolated`: `ops.cleanup[31]` returns a pending (`ResultBlocked`) result. The entries for 30 and 32 are still applied, and 32 is still dispatched.
  - `TestSweepAlreadyClosedOutCarriesNoChildCleanup`: closeout 30 returns `CloseoutDispAlready` with no `CarriedIDs`, so only 30's cleanup runs from the suffix.
  - `TestSweepCarriedChildNotCleanedTwice`: the corpus has 30 `implemented` with a merged PR and 31 `stacked-merged` on 30 (it lands in the pinned cleanup worklist). Closeout 30 carries `[31]`. Assert 31 has exactly **one** cleanup dispatch and one cleanup entry. Repeat with `ops` wired to a stub `assessHistorical` that returns a `snapshot-retained` pre-dispatch entry for 31. The stale entry is gone, and the suffix's applied entry remains.
  - `TestSweepImplementationScopeCleansCarriedChildren`: the same as the first test under `SweepScopeImplementation`.

- [ ] **Step 2: Run and confirm they fail:** `go test -count=1 -run 'TestSweep(Closeout|CarriedChild|AlreadyClosedOut|ImplementationScopeCleans)' ./internal/app`

- [ ] **Step 3: Implement**, and update `sweepRunCloseout`'s doc comment to say the suffix also cleans every carried id.

- [ ] **Step 4: Run** the new tests and `go test -count=1 ./internal/app -run 'Sweep|Maintenance'`. Expect PASS.

- [ ] **Step 5: Commit** `fix(sweep): clean each carried child right after its stack root`.

---

### Task 6: The sweep treats a carried child's leftover branch like any other

**Build tier:** standard

**Files:**
- Modify: `internal/app/maintenance_assess.go`: delete `sweepMaybeCarriedDescendant` and its use in the `presence` expression. Drop the now-unused `integration` local in `sweepAssessHistorical`. Delete the `sweepSharedFacts.prFacts` field and its assignment in `maintenance.go` **only if** a whole-repo grep shows no other reader.
- Test: `internal/app/maintenance_assess_test.go`

**Interfaces:**
- Produces: `presence := wsClear && !reused`. Every other leg is unchanged.

- [ ] **Step 1: Replace the premise-deleted tests** (learning `test-premise-deleted-not-regated`):
  - Replace `TestAssessCarriedDescendantByPRFactsGetsNoPresenceDispatch` with `TestAssessCarriedChildLeftoverLocalBranchDispatches`. The same fixture and PR facts `BaseRef: "feat/parent"`, with only the local branch left, must now be **actionable** (41 in `actionable`).
  - Invert `TestAssessStackedOnRecordWithoutFactsGetsNoPresenceDispatch` into `TestAssessStackedOnRecordLeftoverBranchDispatches`. A `stacked_on: 5` done record with only its local branch is actionable.
  - Keep `TestAssessEmptyStackedOnStillRetriesLeftoverBranch`, `TestAssessLeftoverBranchBehindBlockedWorkspaceStaysBlocked`, and `TestAssessLeftoverBranchReusedByLiveRecordGetsNoPresenceDispatch` unchanged. They guard properties that still hold.

- [ ] **Step 2: Run and confirm the two new tests fail:** `go test -count=1 -run 'TestAssess(CarriedChildLeftover|StackedOnRecordLeftover)' ./internal/app`

- [ ] **Step 3: Implement** the removal, and reword the `presence` comment (drop "a possible carried descendant keeps its branch on purpose").

- [ ] **Step 4: Run** `go test -count=1 ./internal/app -run 'Assess|Sweep'` and `go vet ./internal/app`. Expect PASS.

- [ ] **Step 5: Commit** `fix(sweep): retry a carried child's leftover branch like any other done change`.

---

### Task 7: Finalize skill and docs describe the new cleanup

**Build tier:** standard

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md`: step 10.
- Modify: `skills/docket-convention/references/stacked-changes.md`: *The stack close-out is idempotent*.
- Modify: `skills/docket-convention/references/close-out.md`: the cleanup paragraph ending "except a stacked change merged into its parent's branch, …".
- Modify: `docs/reference/glossary.md`: *Sweep* (the same "except a stacked change merged into its parent's branch" clause), plus any other passage a grep finds saying cleanup keeps a carried child.
- Regenerate: `go generate ./internal/assets/` (the embedded skill tree).

- [ ] **Step 1: Find every passage to change** with one whole-repo grep. Sort the hits into maintained prose and tests that pin that prose (learning `restatement-accumulates-its-own-guards`):
  `grep -rn -e "merged into its parent's branch" -e "retain their workspace and branches until the root" -e "retained until its root" -e "carried descendant" skills docs internal tests`
  A test that pins removed wording gets repointed at the new wording. Never re-add old text to keep it green.

- [ ] **Step 2: Edit the prose** to describe current behavior only:
  - Step 10 adds one clause: when step 9 returned `root-archived`, run `finalize.cleanup` for the root, then once for each id in that result's `carried_ids`, and report each result the way the root's is reported. A child's result never changes the root's or another child's. A child's cleanup removes its folder and branches only after proving its work reached the integration branch through its stack. Otherwise it keeps everything and reports `pending` (`carry-unproven`).
  - Under *The stack close-out is idempotent*: once the root lands, each carried child's folder and branches are cleaned up right after the root's, by finalize and by the sweep, once cleanup proves the child's work reached the integration branch.
  - In close-out.md and the glossary *Sweep* entry, drop "a stacked change merged into its parent's branch" from the sweep's exceptions. Say that a carried child is retried like any other done change, and that a child whose carry can't be proven stays pending with no delete advice.

- [ ] **Step 3: Regenerate and verify:** `go generate ./internal/assets/ && go test -count=1 ./internal/assets/ ./internal/repoguard/`. Expect PASS.

- [ ] **Step 4: Commit** `docs: finalize and the sweep clean carried stacked children after their root`.

---

### Task 8: End-to-end proof through the CLI, including the parent-landed-first case

**Build tier:** premium

**Files:**
- Test: `internal/app/finalize_e2e_test.go` (`//go:build e2e`)
- Modify (only if the reproduction demands it): the finalize/workspace code a further base-mismatch blocker lives in.

**Interfaces:**
- Consumes: the e2e harness (`newImplEnv`, `env.implement`, `stackedBuildReadyChange`, `e2eState.dk`, `verStack`, `syncWorktree`, `featureBranchForHead`, `originHasBranch`, `rootEvidence`, `soleCandidate`). Tasks 1–6 must already be committed.

- [ ] **Step 1: Write the end-to-end tests:**
  - Extend `TestE2EStack`. After the root's closeout (`root-archived`, whose `carried_ids` includes the child), run `finalize cleanup --id <root>` and then `finalize cleanup --id <child>`, which is the step-10 order. Assert the child cleanup's `disposition == "cleaned"`, the child's worktree path (`child.wp`) no longer exists, `refs/heads/<childBranch>` is gone in the invocation clone, and `originHasBranch(childBranch)` is false. The default all-enabled policy merges by rebase (`TestE2EMergeSelectsRebaseShape`), so the child's merge result is not an ancestor of the integration tip, and this path exercises the `ProvePreserved` arm.
  - `TestE2EStackThreeLevelCleanedBySweep`: root, child on root, grandchild on child. Merge the grandchild into the child branch, then close it out (`stacked-merged`). Sync the child worktree, then merge it into the root branch and close it out (`stacked-merged`). Sync the root and run rebase, publish, and merge, **without** closeout. Then run `maintenance sweep`. The sweep's entries contain one closeout entry for the root carrying both descendants and exactly one cleanup entry each for the root, the child, and the grandchild, all applied. All three folders and all six branches are gone. A second `maintenance sweep` reports no cleanup work for the three ids.
  - `TestE2EStackLeftoverCleanedBySweep`: two levels. After root closeout, run only `finalize cleanup --id <root>`, so the child is `done` with its folder and both branches left over. Then one `maintenance sweep` cleans the child.
  - `TestE2EStackParentLandedFirst`: root A and child B on A, both implemented. Run `finalize retarget-children --id A` authorizing B (request shape per `internal/cli/finalize.go` and `finalize_retarget_test.go`), so B's PR now targets the integration branch. Finalize A through rebase, publish, merge, closeout (`done-archived`), and cleanup. Then finalize B through rebase (onto the integration branch), publish, merge, closeout (`done-archived`), and cleanup (`cleaned`). B's folder and both branches are gone.

- [ ] **Step 2: Run** `go test -tags e2e -count=1 -run '^TestE2EStack' ./internal/app`. Expect PASS. If `TestE2EStackParentLandedFirst` hits a blocker **caused by the same base mismatch** (a check comparing the manifest's recorded base with the current effective base), fix it here with a focused test beside the code. If it hits a blocker with any other cause, don't fix it and don't weaken the test past the workspace checkpoints. Report it in the task completion as a follow-up so the results file names it.

- [ ] **Step 3: Mutation check (spec *Ownership*).** Back up `internal/workspace/prepare.go`, restore the unconditional `m.BaseRef == target.BaseRef`, and confirm `TestE2EStackParentLandedFirst` and the child-cleanup assertions in `TestE2EStack` go red. Restore from the backup.

- [ ] **Step 4: Run the shards and read their budget lines:** `bash tests/test_go_finalize_e2e.sh` and `bash tests/test_go_integration_app_cleanup.sh`. If the e2e row breaches its 30s budget, fold `TestE2EStackLeftoverCleanedBySweep` into `TestE2EStack` (a sweep after root-only cleanup) rather than raising the budget, and say so in the completion.

- [ ] **Step 5: Commit** `test(e2e): carried stacked children are cleaned after their root; a child finalizes after its parent`.
