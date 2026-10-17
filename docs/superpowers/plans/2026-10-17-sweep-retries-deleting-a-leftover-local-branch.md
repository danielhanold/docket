<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0562 — Sweep retries deleting a leftover local branch](../../changes/active/0562-sweep-retries-deleting-a-leftover-local-branch.md)**
<!-- docket:backlink:end -->

# Sweep Retries a Leftover Local Branch — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build` (one tier worker per task under the `docket-build-task` contract, one full-suite gate at the end). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The full maintenance sweep re-dispatches cleanup for a done change whose recorded local branch still exists, and a branch cleanup keeps on purpose is reported by name, with the git command to delete it, in the human output of both `finalize cleanup` and `maintenance sweep`.

**Architecture:** Add one shared local-branch inventory (`gitcli.Client.ListLocalHeads`, one `git for-each-ref` per sweep invocation) to `sweepSharedFacts`, and key the snapshot assessment's local-ref leg on branch presence, excluding records whose cleanup would refuse anyway (possible carried descendants, and a blocked or unresolved workspace leg). Cleanup builds a named, remedy-carrying warning for the four kept-on-purpose reasons; the sweep copies those warnings into the entry message and appends them to its human summary.

**Tech Stack:** Go (`internal/gitcli`, `internal/app`), Markdown docs, `go generate ./internal/assets/`.

**Spec:** `docs/superpowers/specs/2026-10-17-sweep-retries-deleting-a-leftover-local-branch-design.md` (on the `docket` branch; read it alongside this plan).

## Global Constraints

- A failed or partial local-branch listing is never "branch absent": it makes the local-ref leg unknown (same posture as `remoteHeadsErr`).
- No per-change git or GitHub probe is added to the assessment. The local-branch inventory is one local git process per sweep invocation, gathered only when there are done candidates (inside the existing `needsShared` gate).
- Every sweep dispatch still goes through a fresh prepare plus `FinalizeCleanup` with all its live proofs. The inventory screens for work; it never authorizes a delete.
- No cleanup safety proof changes (tip, ancestry, worktree-detached, exact lease). Reason codes and the `pending` disposition stay as they are.
- "Delete it by hand" advice appears only for the four kept-on-purpose reasons: `recorded-tip-moved`, `tip-not-in-merge-chain`, `branch-checked-out`, `remote-ref-moved`. Every probe/fetch failure, `local-ref-delete-failed`, `remote-lease-rejected`, and `children-retarget-required` keep today's wording.
- `<b>` in every message is the recorded short branch name (e.g. `feat/foo`). The remote is always `origin`.
- JSON entry fields of `MaintenanceEntry` are unchanged; only `Message` text grows.
- Implementation-scope sweep (preflight) is unchanged, including its deferred count.
- Killed and stacked-merged records get no new dispatch and no warning.
- Docs describe current behavior only, no change citations. Run `go generate ./internal/assets/` after editing any file under `skills/`.
- Comments anchor on symbol names, never line numbers.
- New `integration`-tagged test names must start with an existing shard prefix: `TestIntegrationRepo` (gitcli), `TestIntegrationFinalizeCleanup` or `TestIntegrationSweep` (app). Run integration tests with `-tags integration -count=1 -run '^<Prefix>'`; never run `./internal/app/` integration tests without `-run`.

## Reconcile findings the plan acts on

1. **A blocked workspace must not gain a presence dispatch.** `TestIntegrationFinalizeCleanupResultsArtifactNeverPrepared/leftover-branch-stays-blocked` (from #559) pins: a completed change with no manifest but a leftover local branch assesses `blocked/snapshot-blocked`. Inspect classifies that as `StateForeign`, and both cleanup paths return `pending/workspace-blocked` before ever reaching the ref legs. A presence trigger there would dispatch a cleanup that cannot touch the branch, every sweep. So the presence trigger applies only when the workspace leg is clear (neither blocked nor unresolved). This is the same rule the spec gives for carried descendants: no new dispatch for a record whose cleanup will refuse before the branch leg. The #559 test must stay green unchanged.
2. **`stacked_on:` with no value parses as `FieldEmpty`, not `FieldAbsent`.** Every real record carries an empty `stacked_on:`. The carried-descendant fallback must test `State == domain.FieldPresent` only, or it excludes every record. (A malformed `stacked_on` never reaches the legs: its base does not resolve, so it is `snapshot-invalid` first.)
3. **The destination predicate must match cleanup's exactly** (learning *duplicated-gate-copies-the-whole-predicate*). Cleanup compares `facts.BaseRef` against `pin.IntegrationBranch`, falling back to `pin.DefaultBranch` (in `loadCloseoutContext`). Factor that into one helper and use it in both places.
4. **`CleanupOpResult.HumanText` prints nothing but the reason for a pending result** (`Result == ResultBlocked` takes the second branch). The kept-branch lines must be appended on both branches.

## Review Focus

1. **Empty `stacked_on:` on an ordinary done record** — expect the presence trigger to fire (dispatch). Pinned in Task 3 (`TestAssessEmptyStackedOnStillRetriesLeftoverBranch`).
2. **A leftover branch next to a foreign/never-prepared workspace** — expect today's `snapshot-blocked`, no dispatch. Pinned in Task 3 (`TestAssessLeftoverBranchBehindBlockedWorkspaceStaysBlocked`) and by the existing #559 integration test.
3. **A branch name now recorded by a live change** — expect no delete command in any kept-branch warning, local or remote; a *final* record sharing the name must not suppress the advice. Pinned in Task 4 (`TestKeptBranchWarningReusedName`).
4. **A pending cleanup whose first finding is not the kept-branch one** (`pr-backlink-pending` first, or a local-plus-remote pair) — expect every kept-branch message in the sweep entry and summary. Pinned in Task 5.
5. **`for-each-ref` output for a repository with no branches** — expect an empty non-nil map, never an error or nil. Pinned in Task 1.

---

### Task 1: `gitcli.Client.ListLocalHeads`

**Files:**
- Create: `internal/gitcli/localheads.go`
- Create: `internal/gitcli/localheads_integration_test.go` (`//go:build integration`)

**Interfaces:**
- Produces: `func (c *Client) ListLocalHeads(ctx context.Context, repo Repository) (map[RefName]ObjectID, error)` — every `refs/heads/*` ref with its full object id; empty non-nil map for no branches; on any failure `nil` plus a `*Failure` (never a partial map). Operation label `listLocalHeadsOp Operation = "list-local-heads"`.

- [ ] **Step 1: Write the failing tests** in `localheads_integration_test.go`, mirroring `remoteheads_integration_test.go`:
  - `TestIntegrationRepoListLocalHeadsCompleteInventory`: real repo (`newMainModeRepos`, `mustDiscover`), create local branches `feature/x` and `feature/x-2` in `r.Invocation`; result equals exactly the set from an oracle `git for-each-ref --format=%(refname) %(objectname) refs/heads/` read via `gitOut` (compare with `reflect.DeepEqual`); assert no `refs/remotes/` or `refs/tags/` key appears (create a tag first).
  - `TestIntegrationRepoListLocalHeadsNoBranchesIsEmptyMap`: `git init` an empty repo (no commits) as in `TestIntegrationListRemoteHeadsEmptyOriginIsEmptyMapNotError`; `got != nil && len(got) == 0`, `err == nil`.
  - `TestIntegrationRepoListLocalHeadsMalformedLineIsFailure`: `helperClient(t, "script", "GITCLI_HELPER_STDOUT=deadbeef\trefs/heads/main\n")` → `got == nil`, `assertKind(t, err, KindInvalidOutput)`. Add cases for a non-`refs/heads/` name (`<40 hex>\trefs/tags/v1`) and a line with no TAB, both `KindInvalidOutput`.
  - `TestIntegrationRepoListLocalHeadsDuplicateIsFailure`: same ref twice → `got == nil`, `AsFailure(err)` ok.
  - `TestIntegrationRepoListLocalHeadsCommandFailure`: `helperClient(t, "exit", "GITCLI_HELPER_EXIT=128")` → `got == nil`, `assertKind(t, err, KindCommandFailed)`.

- [ ] **Step 2: Run to verify they fail**
  Run: `go test -tags integration -count=1 -run '^TestIntegrationRepoListLocalHeads' ./internal/gitcli/`
  Expected: build failure, `c.ListLocalHeads undefined`.

- [ ] **Step 3: Implement `ListLocalHeads`** in `localheads.go`. Run `git for-each-ref --format=%(objectname)%09%(refname) refs/heads/` with `dir: repo.PrimaryWorktree`, `network: false`. Parse each `stdoutLines` line with `splitLsRemoteLine`, then `validateObjectID`, require the `refs/heads/` prefix, `validateRefName`, refuse duplicates — the same strict build-or-fail loop as `ListRemoteHeads`. Non-zero exit → `KindCommandFailed` with `stderrExcerpt`. Doc comment states the three-outcome contract (complete map, proven-empty map, error — never partial).

- [ ] **Step 4: Run to verify they pass** (same command). Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/gitcli/localheads.go internal/gitcli/localheads_integration_test.go
git commit -m "feat(gitcli): list every local branch head in one read"
```

---

### Task 2: Gather the local-branch inventory and the batched PR facts

**Files:**
- Modify: `internal/app/maintenance_assess.go` (`sweepSharedFacts`)
- Modify: `internal/app/maintenance.go` (`gatherSweepSharedFacts`)
- Modify: `internal/app/finalize_closeout.go` (`loadCloseoutContext`)
- Test: `internal/app/maintenance_assess_test.go`

**Interfaces:**
- Consumes: `ListLocalHeads` (Task 1).
- Produces:
  - `sweepSharedFacts` gains `localHeads map[gitcli.RefName]gitcli.ObjectID`, `localHeadsErr error` (set ⇒ local-branch presence is unprovable), and `prFacts map[int]domain.PRFacts` (the batched read's `Facts`, set only when that read ran).
  - `func closeoutIntegrationBranch(pin StatusPin) string` — `pin.IntegrationBranch`, falling back to `pin.DefaultBranch`. `loadCloseoutContext` uses it in place of its inline fallback (behavior unchanged).

- [ ] **Step 1: Write the failing tests** in `maintenance_assess_test.go`:
  - `TestGatherSweepSharedFactsNoGitClientMakesLocalHeadsUnknown`: deps with nil `Planning.Client` and one done candidate → `shared.localHeadsErr != nil && shared.localHeads == nil` (alongside the existing `remoteHeadsErr`/`worktreesErr`).
  - `TestGatherSweepSharedFactsKeepsBatchedPRFacts`: as `TestGatherSweepSharedFactsMarksUnreadPRBodiesUnknown`, with `Facts: {n41: {BaseRef: "feat/parent", State: "merged"}}` → `shared.prFacts[n41].BaseRef == "feat/parent"`, and `_, ok := shared.prFacts[n42]; !ok`.
  - `TestCloseoutIntegrationBranchFallsBackToDefault`: `StatusPin{IntegrationBranch: "", DefaultBranch: "trunk"}` → `"trunk"`; `{IntegrationBranch: "main", DefaultBranch: "trunk"}` → `"main"`.

- [ ] **Step 2: Run to verify they fail**
  Run: `go test ./internal/app/ -count=1 -run 'TestGatherSweepSharedFacts|TestCloseoutIntegrationBranch'`
  Expected: build failure (unknown fields / undefined function).

- [ ] **Step 3: Implement.** In `gatherSweepSharedFacts`: keep `res.Facts` from the same `ProbePRSet` call (`shared.prFacts`), never a second call; set `shared.localHeadsErr` alongside the other two in the "no git client or repository" branch; after `ListWorktrees`, call `deps.Planning.Client.ListLocalHeads(ctx, repo)` once, storing the map or the error. Update the `sweepSharedFacts` and `gatherSweepSharedFacts` doc comments to name the third inventory.

- [ ] **Step 4: Run to verify they pass** (same command, plus `-run 'TestAssess|TestGather|TestSweepAssess'`). Expected: PASS, existing assess tests still green.

- [ ] **Step 5: Commit**
```bash
git add internal/app/maintenance_assess.go internal/app/maintenance.go internal/app/finalize_closeout.go internal/app/maintenance_assess_test.go
git commit -m "feat(sweep): gather one local-branch inventory per full sweep"
```

---

### Task 3: Local-ref leg keyed on branch presence

**Files:**
- Modify: `internal/app/maintenance_assess.go` (`sweepAssessHistorical`, `sweepAssessWorkspaceLeg`, `sweepAssessLocalRefLeg`, new `sweepMaybeCarriedDescendant`)
- Test: `internal/app/maintenance_assess_test.go`

**Interfaces:**
- Consumes: `sweepSharedFacts.localHeads/localHeadsErr/prFacts`, `closeoutIntegrationBranch` (Task 2).
- Produces:
  - `sweepAssessWorkspaceLeg(...) (clear bool)` — true only when the leg recorded neither a blocker nor an unknown (no service or inspect error → false; `StateForeign`/`StateAbsent` blocker → false; completed proven-absent, `StateCleaned`, or any work state → true).
  - `sweepAssessLocalRefLeg(shared sweepSharedFacts, featureRef gitcli.RefName, presence bool, a *sweepLegAssessment)`.
  - `func sweepMaybeCarriedDescendant(shared sweepSharedFacts, c domain.Change, integration string) bool` — if the record's PR number is in `shared.prFacts` with a non-empty `BaseRef`, returns `BaseRef != integration`; otherwise returns `c.StackedOn().State == domain.FieldPresent`.

Rules for `sweepAssessLocalRefLeg` (in order):
1. Today's worktree trigger, unchanged: `worktreesErr` set → `markUnknown(sweepLegLocalRef, <today's text>)`; else a worktree on `featureRef` → `markWork()` and return.
2. `presence == false` → return (today's behavior; `localHeadsErr` is not consulted).
3. `localHeadsErr` set → `markUnknown(sweepLegLocalRef, "the local branch list could not be read; whether the local branch still exists is unprovable")`, return.
4. `featureRef` is a key of `localHeads` (exact key lookup, never a prefix match) → `markWork()`.

In `sweepAssessHistorical`: compute `integration := closeoutIntegrationBranch(pin)`; `wsClear := sweepAssessWorkspaceLeg(...)`; `presence := wsClear && (completed || !sweepMaybeCarriedDescendant(shared, c, integration))`. Update the file header and leg doc comments to say the local-ref leg reads the shared local-branch inventory.

- [ ] **Step 1: Write the failing tests** (use `newAssessFixture`, `cleanWS`, `prShared`; add a helper `withLocal(s sweepSharedFacts, refs ...gitcli.RefName) sweepSharedFacts` that sets a non-nil `localHeads` with a fixed 40-hex id per ref):
  - `TestAssessLeftoverLocalBranchIsActionable`: done record 41 `archived`, clean tombstone, empty remote heads, `localHeads` holds `featureRefFor("archived")`, no worktrees → `actionableHas(actionable, 41)`. **Mutation:** delete rule 4 → this test reddens.
  - `TestAssessAbsentLocalBranchIsNoWork`: `localHeads` non-nil, holds only `refs/heads/main` → `snapshot-no-work`.
  - `TestAssessSimilarBranchNameDoesNotMatch`: `localHeads` holds `featureRefFor("archived-2")` only → `snapshot-no-work`.
  - `TestAssessFailedLocalBranchListIsUnknown`: `localHeadsErr: errors.New("for-each-ref failed")`, everything else clean → `assertEntry(..., SweepDispUnknown, ReasonSweepSnapshotUnknown)` and `strings.Contains(e.Message, sweepLegLocalRef)`.
  - `TestAssessCarriedDescendantByPRFactsGetsNoPresenceDispatch`: `prShared` with a correct backlink body for 41 (as `TestAssessCorrectPRBacklinkIsNoWork`) plus `prFacts[n] = {BaseRef: "feat/parent"}`, branch present → `snapshot-no-work`. Control in the same test: `BaseRef: closeoutIntegrationBranch(f.pin)` → actionable.
  - `TestAssessStackedOnRecordWithoutFactsGetsNoPresenceDispatch`: corpus = parent `finalizeBlob(5, "parent", "done", ...)` + child `finalizeBlob(41, "archived", "done", "high", prRefFor(41), "stacked_on: 5\n")`; shared has no PR facts (`prBodiesGathered: false`), branch present → `snapshot-no-work`. Also: same record with `worktrees` holding its feature ref → still actionable (today's trigger unchanged).
  - `TestAssessEmptyStackedOnStillRetriesLeftoverBranch`: child with `extra = "stacked_on:\n"`, no facts, branch present → actionable.
  - `TestAssessLeftoverBranchBehindBlockedWorkspaceStaysBlocked`: `wsState(41, workspace.StateForeign)`, branch present → `assertEntry(..., SweepDispBlocked, ReasonSweepSnapshotBlocked)`.
  - In `TestSweepAssessCompletedResultsArtifact`, add subtest `leftover-local-branch-is-actionable`: `raDoneBlob(41, "research", true, "")`, `cleanWS()`, `withLocal(prShared(map[int]string{}), featureRefFor("research"))` → actionable.

- [ ] **Step 2: Run to verify they fail**
  Run: `go test ./internal/app/ -count=1 -run 'TestAssess|TestSweepAssessCompletedResultsArtifact'`
  Expected: the presence tests FAIL (not actionable); compile errors if helpers are referenced before Step 3.

- [ ] **Step 3: Implement** the three functions and the wiring above.

- [ ] **Step 4: Run to verify they pass** (same command), then the mutation in Step 1: remove rule 4, rerun with `-count=1`, confirm `TestAssessLeftoverLocalBranchIsActionable` and the RA subtest FAIL, restore from a backup copy (not `git checkout --`), rerun green.

- [ ] **Step 5: Commit**
```bash
git add internal/app/maintenance_assess.go internal/app/maintenance_assess_test.go
git commit -m "fix(sweep): retry cleanup when a done change's local branch still exists"
```

---

### Task 4: Named warning for a branch cleanup keeps on purpose

**Files:**
- Modify: `internal/app/finalize_cleanup.go` (`finalizeCleanupLocalRef`, `finalizeCleanupLocalRefNoPR`, `finalizeCleanupRemoteRef`, `CleanupOpResult.HumanText`; new helpers)
- Test: `internal/app/finalize_cleanup_test.go`

**Interfaces:**
- Produces:
  - `func isKeptBranchFinding(f StatusFinding) bool` — true exactly for codes `ReasonCleanupTipMoved`, `ReasonCleanupUnreachable`, `ReasonCleanupCheckedOut`, `ReasonCleanupRemoteMoved` (Task 5 uses it).
  - `func keptBranchWarning(cc *closeoutContext, code, branch string) StatusFinding` — `cleanupWarning(code, msg)` with the message below.
  - `func branchInUseBy(snap domain.Snapshot, self domain.ChangeID, branch string) (domain.ChangeID, bool)` — lowest-id record other than `self` whose `Status().Final()` is false and whose `Branch()` is `FieldPresent` with `Value == branch`.

Exact message copy (`<b>` = branch, `<integration>` = `cc.integrationBranch`):

| code | message |
|---|---|
| `recorded-tip-moved` | `local branch <b> was kept: it moved off the merged commit. Review it, then delete it by hand (git branch -D <b>).` |
| `tip-not-in-merge-chain` | `local branch <b> was kept: it has commits that are not on <integration>. Review it, then delete it by hand (git branch -D <b>).` |
| `branch-checked-out` | `local branch <b> was kept: it is checked out in another worktree. Review that worktree, then delete the branch by hand (git branch -D <b>).` |
| `remote-ref-moved` | `remote branch <b> was kept: it moved after the merge. Review it, then delete it by hand (git push origin --delete <b>).` |
| any of the four, branch in use by #N | `local branch <b> was kept: change #N is now using it.` (`remote branch …` for `remote-ref-moved`) |

Replace the `cleanupWarning(...)` call at each of these sites with `keptBranchWarning(cc, <code>, featureBranch)`: tip-moved (PR local), unreachable (PR local and no-PR local), checked-out (PR local and no-PR local), remote-moved (PR remote). Leave every other `cleanupWarning` untouched. `HumanText`: on both the applied/no-op branch and the refusal branch, append `"; warning: " + f.Message` for each finding where `isKeptBranchFinding(f)` (the existing workspace-remnant line stays as is).

- [ ] **Step 1: Write the failing tests** in `finalize_cleanup_test.go`:
  - `TestKeptBranchWarningsNameBranchAndCommand` (table over the four reasons), produced through the real legs:
    - no-PR: `finalizeCleanupResultsArtifact` with `happyRAGit` + `ancestor=false` → finding code `tip-not-in-merge-chain`, message contains `local branch feat/research was kept`, `not on main`, `(git branch -D feat/research)`; with `worktrees` holding the feature ref → `branch-checked-out` message contains `checked out in another worktree` and `git branch -D feat/research`.
    - PR local: call `finalizeCleanupLocalRef(ctx, FinalizeDeps{}, git, raContext(t, true), featureRefFor("research"), "feat/research", githubcli.MergedFacts{HeadOID: <40×"c">, MergeCommit: <40×"d">})` with `happyRAGit` (tip `a…`) → `recorded-tip-moved`, message contains `moved off the merged commit` and `git branch -D feat/research`.
    - PR remote: a fake `keptRemoteGit` (embed `*raGit`, override `ProbeRemoteBranch` to return `gitcli.RemoteRef{State: gitcli.RemoteRefFound, Commit: <40×"e">}`) through `finalizeCleanupRemoteRef(..., facts{HeadOID: 40×"a"})` → `remote-ref-moved`, message contains `remote branch feat/research was kept` and `git push origin --delete feat/research`.
  - `TestQuietCleanupReasonsCarryNoDeleteAdvice`: drive `ref-probe-failed` (`resolveErr`), `ancestry-probe-failed` (`fetchErr`), `worktree-list-probe-failed` (`listErr`) through `finalizeCleanupResultsArtifact`, and `local-ref-delete-failed` (a `raGit` variant whose `DeleteLocalBranchChecked` errors); assert each message contains neither `git branch -D` nor `--delete`, and `isKeptBranchFinding` is false for each, and for `remote-lease-rejected`, `remote-ref-probe-failed`, `child-pr-probe-failed`, `workspace-remnant`.
  - `TestKeptBranchWarningReusedName`: corpus = done RA record 41 `research` plus `finalizeBlob(50, "other", "in-progress", ...)` with `branch: feat/other` replaced by `branch: feat/research`; build `cc` like `raContext`. Unreachable → message contains `change #50 is now using it` and not `git branch -D`. Remote form via `keptBranchWarning(cc, ReasonCleanupRemoteMoved, "feat/research")` → `remote branch feat/research was kept: change #50 is now using it.`. Control: record 50 `done` instead → advice present.
  - `TestCleanupHumanTextNamesKeptBranch`: take the pending result of the unreachable case above (producer path, `Result == ResultBlocked`) → `HumanText()` contains `tip-not-in-merge-chain` and `git branch -D feat/research`. Second case: an applied result carrying a kept finding (build with `newCleanupResult(..., ResultApplied, {Disposition: CleanupDispChildrenRetargetRequired, Findings: [keptBranchWarning(...)]})`) also shows it.

- [ ] **Step 2: Run to verify they fail**
  Run: `go test ./internal/app/ -count=1 -run 'TestKeptBranch|TestQuietCleanup|TestCleanupHumanText|TestFinalizeCleanupResultsArtifact'`
  Expected: FAIL / build errors for the new helpers.

- [ ] **Step 3: Implement** the helpers, the six call-site swaps, and `HumanText`.

- [ ] **Step 4: Run to verify they pass** (same command). Expected: PASS, existing results-artifact tests still green.

- [ ] **Step 5: Commit**
```bash
git add internal/app/finalize_cleanup.go internal/app/finalize_cleanup_test.go
git commit -m "fix(cleanup): name a branch kept on purpose and how to delete it"
```

---

### Task 5: Carry kept-branch warnings into the sweep entry and summary

**Files:**
- Modify: `internal/app/maintenance.go` (`MaintenanceEntry`, `sweepRunCleanup`, `MaintenanceResult.HumanText`)
- Test: `internal/app/maintenance_test.go`

**Interfaces:**
- Consumes: `isKeptBranchFinding` (Task 4).
- Produces: `MaintenanceEntry` gains unexported `keptBranches []string` (`json:"-"`, so the JSON shape is unchanged), filled only by `sweepRunCleanup`.

Behavior: in `sweepRunCleanup`, after the workspace-remnant loop, for each finding with `isKeptBranchFinding(f)`: append `f.Message` to `entry.Message` joined with `"; "` (or set it when empty), set `entry.Reason = f.Code` only when `entry.Reason` is empty, and append `f.Message` to `entry.keptBranches`. `MaintenanceResult.HumanText` appends, after everything it prints today, one line per kept-branch message: `"\nwarning: change #<id>: <message>"`, in entry order.

- [ ] **Step 1: Write the failing tests** in `maintenance_test.go`, modeled on `TestSweepCleanupSurfacesWorkspaceRemnant` (same corpus/prober/`recordingSweepOps` setup):
  - `TestSweepCleanupCarriesKeptBranchWarnings`: cleanup for 30 returns `finalizeCleanupResult(30, CleanupDispPending, nil, findings, "cleanup is partially complete; the retained legs are independently retryable")` with findings `[cleanupWarning(ReasonPRBacklinkPending, "…"), cleanupWarning(ReasonCleanupTipMoved, "local branch feat/merged was kept: … (git branch -D feat/merged)."), cleanupWarning(ReasonCleanupRemoteMoved, "remote branch feat/merged was kept: … (git push origin --delete feat/merged).")]`. Assert: `e.Reason == ReasonPRBacklinkPending` (unchanged); `e.Message` contains both `git branch -D feat/merged` and `git push origin --delete feat/merged`; `res.HumanText()` contains `warning: change #30: local branch feat/merged was kept` and `warning: change #30: remote branch feat/merged was kept`; `json.Marshal(e)` has no `kept` key.
  - `TestSweepCleanupQuietReasonAddsNoWarningLine`: findings `[cleanupWarning(ReasonCleanupLocalDelete, "the checked local ref delete did not land; the ref is retained")]` → `HumanText()` contains no `"\nwarning:"`.
  - Existing `TestSweepHumanTextNamesScopeAndDeferred` stays green unchanged (no kept warnings ⇒ identical output).

- [ ] **Step 2: Run to verify they fail**
  Run: `go test ./internal/app/ -count=1 -run 'TestSweepCleanup|TestSweepHumanText'`
  Expected: FAIL (message lacks the warnings).

- [ ] **Step 3: Implement** as described; extend the `MaintenanceResult.HumanText` doc comment to mention the kept-branch lines.

- [ ] **Step 4: Run to verify they pass** (same command). Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/app/maintenance.go internal/app/maintenance_test.go
git commit -m "fix(sweep): show a kept branch's warning in the sweep output"
```

---

### Task 6: End-to-end proof and request budget

**Files:**
- Modify: `internal/app/finalize_cleanup_test.go` (`faultyCleanupGit`: add `fail == "delete"` mode)
- Modify: `internal/app/finalize_cleanup_integration_test.go`
- Modify: `internal/app/maintenance_traffic_integration_test.go`

**Interfaces:**
- Consumes: everything above. `faultyCleanupGit.DeleteLocalBranchChecked` returns `errCleanupProbe` when `fail == "delete"`, else delegates.

- [ ] **Step 1: Write the tests**
  - `TestIntegrationFinalizeCleanupLeftoverLocalBranchSweepRetry` (PR path, `setupCloseoutFixture` + `archiveClosed` + `mergedCleanupFake`):
    - subtest `retried`: first `FinalizeCleanup` with `&faultyCleanupGit{inner: f.deps.Client, fail: "delete"}` → `Reason`/finding `local-ref-delete-failed`, `f.localBranchPresent(t)` true, `f.remoteBranchPresent(t)` false. `sweepAssessOne(t, realNode{dir: f.repo.invocation, deps: f.deps}, f.cleanupDeps(gh, f.deps.Client, f.svc), f.svc, f.id)` → actionable. Second `FinalizeCleanup` with the real client → `CleanupDispCleaned`, local branch gone.
    - subtest `kept-after-post-merge-commit`: after the same failing first run, move the branch: `tip := runGit(t, f.repo.invocation, "rev-parse", "refs/heads/feat/"+f.slug)`, `c := runGit(t, f.repo.invocation, "commit-tree", tip+"^{tree}", "-p", tip, "-m", "after merge")`, `runGit(..., "branch", "-f", "feat/"+f.slug, c)`. Assessment → actionable; `FinalizeCleanup` → `Disposition == CleanupDispPending`, branch still present, `res.HumanText()` contains `local branch feat/<slug> was kept` and `git branch -D feat/<slug>`.
  - `TestIntegrationFinalizeCleanupLeftoverBranchFullSweep` (results-artifact path, the same fixture as `TestIntegrationFinalizeCleanupResultsArtifactRealGit`: `prepareLeakWorkspace` + `completeLeakWorkspace`): first `FinalizeCleanup` with `deps.CleanupGit = &faultyCleanupGit{inner: lw.node.deps.Client, fail: "delete"}` → pending, branch kept, worktree removed. Then `MaintenanceSweep(ctx, sweepDeps, lw.node.dir, SweepScopeFull)` where `sweepDeps` = the returned deps with `CleanupGit: nil` and `PRBatch: &fakeSweepBatchReader{}` → an entry `{ID: lw.id, Kind: sweepKindCleanup, Disposition: SweepDispApplied}` and `for-each-ref` shows the branch gone. Subtest `kept`: instead of the plain retry, first move the branch onto a new child commit as above, then sweep → entry pending with `Reason == ReasonCleanupUnreachable`, branch present, `res.HumanText()` contains `warning: change #<id>: local branch feat/research was kept` and `git branch -D feat/research`. If `MaintenanceSweep` cannot run over this fixture, stop and report it as BLOCKED with the failure; do not substitute a lower-level call.
  - Traffic (`maintenance_traffic_integration_test.go`): add `localHeads int // for-each-ref refs/heads/ — the one shared local-branch inventory` to `remoteCounts`, filled by `countMatching(gl, "for-each-ref", "refs/heads/")`. In `TestIntegrationSweepDiscoveryTrafficAccounting` assert it is exactly 1; the scaling test then keeps it constant automatically (`got != base`) — add a sanity check `base.localHeads == 1`; in `TestIntegrationSweepImplementationScopeInspectsNoDeferredResources` assert 0 for implementation scope and 1 for full.

- [ ] **Step 2: Run** (red first where the behavior is new — the PR `retried` assessment and the full-sweep retry fail on a tree without Tasks 3–5; on this branch they pass):
  Run: `go test -tags integration -count=1 -run '^TestIntegrationFinalizeCleanup' ./internal/app/` and `go test -tags integration -count=1 -run '^TestIntegrationSweep' ./internal/app/`
  Expected: PASS, including the unchanged `TestIntegrationFinalizeCleanupResultsArtifactNeverPrepared`.

- [ ] **Step 3: Mutation check:** temporarily make `sweepAssessLocalRefLeg` skip rule 4; rerun the `^TestIntegrationFinalizeCleanupLeftover` tests with `-count=1` → they FAIL; restore from a backup copy; rerun green.

- [ ] **Step 4: Commit**
```bash
git add internal/app/finalize_cleanup_test.go internal/app/finalize_cleanup_integration_test.go internal/app/maintenance_traffic_integration_test.go
git commit -m "test(sweep): prove a leftover branch is retried end to end within budget"
```

---

### Task 7: Docs

**Files:**
- Modify: `skills/docket-convention/references/close-out.md` (step 3 "Clean up the feature branch + worktree")
- Modify: `docs/reference/glossary.md` (`### Sweep`)
- Regenerate: `internal/assets/` via `go generate ./internal/assets/`

- [ ] **Step 1: Edit close-out.md.** Replace "the sweep retries a leftover worktree only." with a sentence saying the full sweep retries cleanup for any `done` change whose worktree or local branch is still there. After "Any resource whose ownership it cannot prove is **retained**, not force-removed.", add: when cleanup keeps a branch because deleting it could lose work (it moved after the merge, it has commits not on the integration branch, or another worktree has it checked out), the `pending` result names the branch and gives the command to delete it by hand after review (`git branch -D <branch>`, or `git push origin --delete <branch>` for the remote copy); if a live change now uses that branch name, it says so instead of suggesting a delete. The warning shows in the human output of `finalize cleanup` and of the sweep, and repeats on every full sweep until the branch is gone.
- [ ] **Step 2: Edit glossary.md** `### Sweep`: "It also retries cleanup for every `done` change whose worktree or local branch is left over, including a completed `results-artifact` change." Keep the scope sentence.
- [ ] **Step 3: Regenerate and check:** `go generate ./internal/assets/` then `go test ./internal/assets/ -count=1`. Expected: PASS, generated assets updated.
- [ ] **Step 4: Commit**
```bash
git add skills/docket-convention/references/close-out.md docs/reference/glossary.md internal/assets
git commit -m "docs: the sweep retries a leftover local branch and names a kept one"
```

---

### Final gate

The build role runs the whole suite through the configured `build.test_command` (`go run ./cmd/docket development test`) once, at the end, and reads the budget report (`BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` lines) even on a green run.
