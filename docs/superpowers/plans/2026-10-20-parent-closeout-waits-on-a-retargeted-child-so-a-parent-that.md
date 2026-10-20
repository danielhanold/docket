<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0571 — Close out a landed stack parent without waiting on children that never merged into it](../../changes/active/0571-parent-closeout-waits-on-a-retargeted-child-so-a-parent-that.md)**
<!-- docket:backlink:end -->

# Close out a landed stack parent without waiting on children that never merged into it — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build`, task by task, under the `docket-build-task` contract. Steps use checkbox (`- [ ]`) syntax for tracking. Each task names its build tier.

**Goal:** A stack parent that landed on the integration branch closes out to `done`, carrying exactly the children whose PRs merged into its branch (down the chain, `stacked-merged` or still `implemented`), once each one's work is proven on the integration branch. No other child blocks it. Closeout's `children-retarget-required` disposition goes away.

**Architecture:** One carry rule in `internal/domain/stackcloseout.go`. `DeriveCarriedSet` gains an `implemented` arm: an `implemented` child whose live PR merged into its parent's recorded branch is carried (proof `""`) and descended into; any other `implemented` child is skipped like an open child. `DeriveRootCloseoutSet` becomes a thin wrapper: resolve the root, keep today's `rootAncestryCycles` refusal, otherwise return `DeriveCarriedSet`. So rebase, publish, merge (through `proveCarriedOnHead`) and root closeout (through `closeoutIntegrationDestination`) share one walk. `proveCarry` and the `not-stacked-merged` token are deleted. In `internal/app`, `probeDescendantFacts` looks up only `implemented` and `stacked-merged` descendants, and the closeout disposition `children-retarget-required` is removed with its docs.

**Tech Stack:** Go (module `github.com/danielhanold/docket`), real-git integration tests (`-tags integration`), the CLI end-to-end matrix (`-tags e2e`), markdown skill and reference docs, the embedded skill tree (`go generate ./internal/assets/`).

**Spec:** `docs/superpowers/specs/2026-10-20-parent-closeout-waits-on-a-retargeted-child-so-a-parent-that-design.md` (on the metadata branch; a copy rides the feature branch). Read it first. Its *The rule*, *Before and after* table, *What else stays fail-closed*, *One definition*, *The `children-retarget-required` closeout disposition goes away*, and *Tests* sections are the requirements.

## Global Constraints

- **The rule:** a child is carried when its status is `stacked-merged` **or** `implemented` **and** its live PR facts show `State == "merged"` with `BaseRef` equal to its parent's recorded `branch:`. The walk descends through carried children only.
- **Still fail-closed, `stacked-merged` only:** a `stacked-merged` child with missing/unmerged facts is still surfaced with `pr-unknown`; with a different destination (or an absent parent `branch:`) with `destination-mismatch`. For an `implemented` child, any of those is just "not carried" — no token, not surfaced, not descended.
- An `implemented` child with no usable PR reference is not carried (it has no facts).
- A carried child whose work is not proven on the integration branch still refuses the parent with `blocked` / `descendant-carry-unproven` and **zero** descendant writes. The content proof (`proveCarriedContent`) runs for every carried child, `implemented` ones included. It reads the integration branch, never the parent's branch.
- A root whose own `stacked_on` ancestry cycles still refuses exactly as today (every descendant `cycle`).
- A GitHub lookup error is still `unknown` and retained. Only descendants whose status is `implemented` or `stacked-merged` are looked up.
- The archive transaction (`closeoutArchiveOp`, `domain.MarkDone`) needs no new shape — do not touch it.
- `walkCarryChain`, `ProveDoneCarry`, and cleanup's `CleanupDispChildrenRetargetRequired` (`children-retarget-required` as a **cleanup** disposition) are unchanged.
- Docs describe current behavior only, with no change or PR citations (ADR citations are fine). Cross-references in maintained source anchor on symbol names, never line numbers.
- New integration tests use an existing shard prefix: `TestIntegrationFinalizeArchive…` (closeout), `TestIntegrationFinalizeRebase…` (rebase and the carry helper), `TestIntegrationFinalizeMerge…`, `TestIntegrationFinalizeState…` (publish). End-to-end tests stay `TestE2E…` in `internal/app/finalize_e2e_test.go`.
- Run every Go test that re-verifies or mutates with `-count=1` (learning `cached-runner-serves-a-mutated-tree`). A mutation restore uses a backup copy (`cp f f.bak` … `mv -f f.bak f`), never `git checkout --` (learning `mutation-restore-needs-a-backup-copy`).
- When a test's premise is retired, ask what it guarded: invert a test whose mechanism still matters, delete one whose subject is gone (learning `test-premise-deleted-not-regated`).
- Whole-suite gate: `build.test_command` (`go run ./cmd/docket development test`), run by docket-build at the end. Read the budget report even when it's green (`tests/README.md`). Shards gaining tests: `tests/test_go_integration_app_archive.sh`/`…_closeout.sh` (whichever routes `TestIntegrationFinalizeArchive`), `…_merge.sh` (15s), `…_rebase.sh` (45s), the publish shard, and `tests/test_go_finalize_e2e.sh` (30s).
- **ADR (spec *Decision record*):** not a feature-branch task. ADRs live on the metadata branch, so `docket-implement-next` Step 6 dispatches `docket-adr` with:
  - **Title:** "A stack parent carries exactly the children whose PRs merged into it."
  - **Context:** root closeout required every descendant to be `stacked-merged`, so a landed parent stayed `implemented` forever behind a retargeted, unbuilt, killed, or already-done child, and behind an `implemented` child merged by hand into its branch (whose own closeout needs the parent's branch, which may be deleted). Merge, and the carry proof rebase/publish/merge run, already ignored those children.
  - **Decision:** a parent carries exactly the children whose PRs merged into its branch, transitively, whether their own closeout ran (`stacked-merged`) or not (`implemented`); each is proven in Git on the integration branch before any write; no other child blocks the parent's closeout. Rebase, publish, merge, and closeout share this one rule. The closeout disposition `children-retarget-required` is retired.
  - **Alternatives rejected:** keep requiring `stacked-merged` and let the sweep retry forever; let `implemented`-merged children wait for their own closeout (stuck once the parent's branch is deleted); carry `implemented` children whose PRs merged anywhere (they are not the parent's to carry).
  - **Related:** ADR-0092 (an `implemented` child still never goes `done` on its own merge into the parent; it reaches `done` only with the parent once its work is on the integration branch).

## Review Focus

1. **A grandchild beneath a child that is not carried.** Parent A, open child B, `stacked-merged` grandchild C merged into B: C must not be carried and must not block A. Task 1 pins it in the domain (`open child stops descent`).
2. **An `implemented` child merged into the parent's branch after the parent landed.** Its work is not on the integration branch, so A must refuse naming it (`blocked`, `descendant-carry-unproven`), zero writes. Task 2's `implemented-merged-child-content-dropped-refuses` pins it.
3. **The parent's remote branch is already deleted** (GitHub auto-delete). An `implemented`-merged child must still be carried, because the content proof reads the integration branch. Task 2's `implemented-merged-child-parent-branch-deleted` pins it.
4. **Lookup traffic.** Archived `done`/`killed` descendants and unbuilt children must cost zero GitHub probes; any test that counts `ProbeMerged` calls for a stack with such descendants changes. Re-derive counts from the new behavior; don't widen tolerances. Task 2 pins the count.
5. **Carried-id order on a branching stack.** `DeriveCarriedSet` is a pre-order depth-first walk while `StackDescendantsParentFirst` was the old order. Both are parent-first; `carried_ids` and archive target order may differ on a stack with two children that each carry grandchildren. If any test pins order on such a shape, re-derive it; parent-before-child is the only promise.

---

### Task 1: One carry rule in the domain

**Build tier:** premium

**Files:**
- Modify: `internal/domain/stackcloseout.go` — `DeriveCarriedSet`, `DeriveRootCloseoutSet`, delete `proveCarry` and the `carryNotStackedMerged` constant, rewrite the doc comments of `CarriedDescendant`, `DeriveRootCloseoutSet`, `DeriveCarriedSet`, and `walkCarryChain` (it is now the walk `ProveDoneCarry` uses for cleanup, not closeout's).
- Test: `internal/domain/stackcloseout_test.go` (untagged).

**Interfaces:**
- Produces (signatures unchanged): `DeriveCarriedSet(s Snapshot, parent ChangeID, facts map[ChangeID]PRFacts) ([]CarriedDescendant, *PolicyFailure)` and `DeriveRootCloseoutSet(s Snapshot, root ChangeID, facts map[ChangeID]PRFacts) ([]CarriedDescendant, *PolicyFailure)`.
- `DeriveRootCloseoutSet` keeps its `root-not-found` / `root-ambiguous` policy failures and its `rootAncestryCycles` arm (every `StackDescendantsParentFirst` id with proof `cycle`), and otherwise returns `DeriveCarriedSet(s, root, facts)` unchanged.
- Refusal tokens after this task: `destination-mismatch`, `pr-unknown`, `chain-broken`, `cycle` (from the carried set), plus `not-done` / `killed-ancestor` (from `ProveDoneCarry` only). `not-stacked-merged` no longer exists.

- [ ] **Step 1: Rewrite the refusal table and add the new cases.** In `TestDeriveRootCloseoutSetRefusals` delete three rows whose situations are now allowed: "descendant still in-progress is not-stacked-merged", "a gap mid-chain is chain-broken for the node below it", "a killed ancestor mid-chain is killed-ancestor". Keep the destination-mismatch, both pr-unknown, and cycle rows. Then add:
  - `TestDeriveRootCloseoutSetNonCarriedChildrenDoNotBlock` — table over the child (id 2, `parent: parentEdge(1)`) of root 1 (`StatusImplemented`, branch `feat/1`). For each row: `fail == nil`, `len(set) == 0`, `RootCloseoutProven(set) == true`. Rows:
    - status `StatusInProgress`, `StatusProposed`, `StatusBlocked`, `StatusDeferred`, `StatusKilled`, `StatusDone`, each with facts `{2: mergedInto("feat/1")}` (facts present must not matter);
    - `StatusImplemented` with no facts (no PR);
    - `StatusImplemented` with `{2: {State: "closed", Number: "8"}}` (open or closed-unmerged);
    - `StatusImplemented` with `{2: mergedInto("main")}` (retargeted and merged to main);
    - `StatusImplemented` with `{2: {State: "unknown", BaseRef: "feat/1"}}`.
  - `TestDeriveRootCloseoutSetOpenChildStopsDescent` — 1 (implemented, `feat/1`) ← 2 (`StatusInProgress`, `feat/2`) ← 3 (`StatusStackedMerged`), facts `{2: mergedInto("feat/1"), 3: mergedInto("feat/2")}`: set empty, proven. Repeat with 2 as `StatusKilled` and as `StatusImplemented` with `mergedInto("main")`.
  - `TestDeriveCarriedSetImplementedMergedChildIsCarried` — 1 ← 2 (`StatusImplemented`, `feat/2`), facts `{2: mergedInto("feat/1")}`: `idsOf(set) == [2]`, proof `""`, proven. Run the same assertions through `DeriveRootCloseoutSet`.
  - `TestDeriveCarriedSetChainThroughImplementedMiddle` — 1 ← 2 (`StatusImplemented`, `feat/2`) ← 3 (`StatusStackedMerged`, `feat/3`) ← 4 (`StatusImplemented`, `feat/4`), facts `{2: mergedInto("feat/1"), 3: mergedInto("feat/2"), 4: mergedInto("feat/3")}`: ids `[2 3 4]`, all proven.
  - `stacked-merged` children still refuse: keep the existing "merged into the wrong branch is destination-mismatch" and both "pr-unknown" rows in `TestDeriveRootCloseoutSetRefusals`, and `TestDeriveCarriedSetSurfacedRefusals` unchanged. No new test needed.
  - `TestDeriveRootCloseoutSetMatchesCarriedSet` — the drift guard: for every non-cyclic fixture above (build a slice of `{specs, facts}`), assert `DeriveRootCloseoutSet(s, 1, facts)` and `DeriveCarriedSet(s, 1, facts)` return equal slices (`slices.Equal` on `idsOf` and on proofs).
  - Update `TestDeriveCarriedSetKilledOrProposedChildContributesNothing` only if its comment mentions "non-stacked-merged"; its assertions still hold.

- [ ] **Step 2: Run the domain tests and confirm the new ones fail.**
  Run: `go test -count=1 ./internal/domain -run 'TestDeriveRootCloseoutSet|TestDeriveCarriedSet|TestRootCloseoutProven|TestProveDoneCarry'`
  Expected: FAIL in `…NonCarriedChildrenDoNotBlock` (`not-stacked-merged` / `chain-broken` / `killed-ancestor` proofs), `…ImplementedMergedChildIsCarried`, `…ChainThroughImplementedMiddle`, and `…MatchesCarriedSet`.

- [ ] **Step 3: Implement.** In `DeriveCarriedSet`'s child loop, replace the `c.Status() != StatusStackedMerged → continue` gate with a status switch:
  - `StatusStackedMerged`: today's code, unchanged (surface `pr-unknown` / `destination-mismatch`, descend only on `""`).
  - `StatusImplemented`: carried iff `facts[cid]` exists, `State == prStateMerged`, the parent's `Branch()` is `FieldPresent` and non-empty, and `BaseRef == branch.Value`. If carried, append `{ID: cid, Proof: ""}` and descend; otherwise `continue` (no entry, no descent).
  - default: `continue`.
  Then reduce `DeriveRootCloseoutSet` to: resolve root (same failures) → `rootAncestryCycles` arm (unchanged) → `return DeriveCarriedSet(s, root, facts)`. Delete `proveCarry` and `carryNotStackedMerged`. Rewrite the comments named in **Files** so they state the rule from the Global Constraints (no change citations).

- [ ] **Step 4: Run the domain tests.**
  Run: the Step 2 command, then `go vet ./internal/domain`.
  Expected: PASS. `TestProveDoneCarry*` unchanged and green.

- [ ] **Step 5: Mutation-test the new guards** (backup copy, `-count=1`, restore after each):
  - Drop the `StatusImplemented` arm (fall to `default`): `…ImplementedMergedChildIsCarried` and `…ChainThroughImplementedMiddle` must redden.
  - Relax the arm to carry any merged `implemented` child regardless of `BaseRef`: the `mergedInto("main")` rows must redden.
  - Restore the old rule — in `DeriveRootCloseoutSet`, after `DeriveCarriedSet`, append `{ID: id, Proof: "chain-broken"}` for every `StackDescendantsParentFirst(s, root)` id whose status is not `StatusStackedMerged`: `…NonCarriedChildrenDoNotBlock`, `…OpenChildStopsDescent`, and `…MatchesCarriedSet` must redden. Keep this mutation's text in the task report; Task 4 reuses it.
  Record which test reddened for each mutation in the task report.

- [ ] **Step 6: Commit.**
  ```bash
  git add internal/domain/stackcloseout.go internal/domain/stackcloseout_test.go
  git commit -m "fix(domain): a stack parent carries exactly the children whose PRs merged into it"
  ```
  Note: after this commit the app integration subtest `TestIntegrationFinalizeArchiveRootCarry/one-unproven-descendant-keeps-root-recoverable` expects the retired disposition and fails; Task 2 rewrites it. The package still builds.

---

### Task 2: Root closeout uses the shared rule; the closeout disposition goes away

**Build tier:** premium

**Files:**
- Modify: `internal/app/finalize_closeout.go` — delete `CloseoutDispChildrenRetargetRequired`; drop the `"not-stacked-merged"` arm of `closeoutUnprovenDisposition` and rewrite its comment (`pr-unknown` → `unknown`, anything else → `blocked`); filter `probeDescendantFacts`; rewrite the file header's item 3 ("Stack root carry") and the `probeDescendantFacts` comment to state the new rule.
- Modify: `internal/app/schema_vocab.go` — remove `CloseoutDispChildrenRetargetRequired` from `closeout_dispositions` (leave `cleanup_dispositions` alone).
- Modify: `docs/reference/glossary.md` — the `closeout_dispositions` row loses `children-retarget-required` (the `cleanup_dispositions` row keeps it).
- Test: `internal/app/finalize_closeout_integration_test.go` (`TestIntegrationFinalizeArchiveRootCarry`).

**Interfaces:**
- Consumes: Task 1's `DeriveRootCloseoutSet` (unchanged signature).
- Produces: `probeDescendantFacts(ctx, deps, ghRepo, snap, root)` keeps its signature; it skips any descendant whose `Status()` is neither `domain.StatusImplemented` nor `domain.StatusStackedMerged` before the PR-ref check. Its other caller, `proveCarriedOnHead`, inherits the filter.

- [ ] **Step 1: Write the failing integration subtests** in `TestIntegrationFinalizeArchiveRootCarry`. Generalize the local `seed` to `seedAt(t, descStatus, recPath string)` (keep `seed(t, s)` as `seedAt(t, s, groomPath(6, "gadget"))`); archived statuses use `"docs/changes/archive/2026-08-16-0006-gadget.md"`. Every subtest below asserts the parent result **and** the child's record location/status on origin.
  - Rewrite `one-unproven-descendant-keeps-root-recoverable` as `unmerged-implemented-child-does-not-block-root`: seed `implemented`, PR #8 unscripted (cleanly not merged). Expect `ResultApplied`, `CloseoutDispDoneArchived`, `len(res.CarriedIDs) == 0`, `docs/changes/archive/2026-08-18-0005-widget.md` present with `status: 'done'`, and `groomPath(6, "gadget")` still present and still `implemented`. (The zero-writes-on-unproven mechanism it used to guard stays covered by `descendant-dropped-refuses`.)
  - `non-carried-children-do-not-block-root` — table over `proposed`, `in-progress` (active path), `killed`, `done` (archive path); `blocked`/`deferred` may be omitted if the fixture record needs fields the helper does not write (the domain table covers them). Each: `done-archived`, child record unchanged, and **`gh.probes == 1`** (only the root's own reprobe — the lookup filter).
  - `implemented-child-merged-to-main-not-carried` — seed `implemented`, PR #8 scripted merged into `"main"` at a real commit. Expect `done-archived`, no carried ids, child still active and `implemented`, `gh.probes == 2`.
  - `implemented-merged-child-is-carried` — seed `implemented`; `childMerge := f.carryOntoRootFeature(...)`; `mergeIntoBase`; `fetchAllIntoInvocation`; `rootFake(f, mergeCommit, childMerge)`. Expect `assertBothArchived` (root-archived, carried `[6]`, both archived `done`).
  - `implemented-merged-child-parent-branch-deleted` — same as above, but after `mergeIntoBase` delete the root's remote branch (`git push -q origin --delete feat/widget` from the writer) before `fetchAllIntoInvocation`. Expect `assertBothArchived`.
  - `implemented-merged-child-content-dropped-refuses` — seed `implemented`; `childMerge := f.carryCommit(t, "gadget-mc", "main", …)` (exists, never on the root branch); `mergeIntoBase`; `fetchAllIntoInvocation`; `rootFake(f, mergeCommit, childMerge)`. Expect `ResultBlocked`, `CloseoutDispBlocked`, `ReasonCloseoutChildUnproven`, metadata ref tip unchanged, no archive files, both active records present.

- [ ] **Step 2: Run them and confirm the failures.**
  Run: `go test -count=1 -tags integration ./internal/app -run 'TestIntegrationFinalizeArchiveRootCarry'`
  Expected: FAIL in `non-carried-children-do-not-block-root` on `gh.probes` (the archived `done`/`killed` and unbuilt descendants are still looked up). The other new rows already pass on Task 1's rule; Step 5 proves they are not vacuous.

- [ ] **Step 3: Implement** the edits listed under **Files**. Then grep the whole repo for `CloseoutDispChildrenRetargetRequired` and `not-stacked-merged` (`grep -rn -e CloseoutDispChildrenRetargetRequired -e not-stacked-merged --include='*.go' .`) — only point-in-time plans/specs under `docs/superpowers/` may still mention the token.

- [ ] **Step 4: Run the closeout tests and the vocab guard.**
  Run: `go test -count=1 -tags integration ./internal/app -run 'TestIntegrationFinalizeArchive|TestIntegrationFinalizeCloseout'` and `go test -count=1 ./internal/app -run 'Vocab|Schema|Closeout'`
  Expected: PASS. Then `grep -rn -e 'probes' internal/app/*_test.go` for any other test that counts `ProbeMerged` calls on a stack with archived or unbuilt descendants, run it, and re-derive its count if it changed.

- [ ] **Step 5: Mutation-test** (backup copy, `-count=1`):
  - Remove the status filter from `probeDescendantFacts`: `non-carried-children-do-not-block-root` must redden on `gh.probes`.
  - In `closeoutIntegrationDestination`, skip `proveCarriedContent` for a descendant whose snapshot status is `implemented` (`continue` before the proof): `implemented-merged-child-content-dropped-refuses` must redden.

- [ ] **Step 6: Commit.**
  ```bash
  git add internal/app/finalize_closeout.go internal/app/schema_vocab.go docs/reference/glossary.md internal/app/finalize_closeout_integration_test.go
  git commit -m "fix(finalize): a landed parent's closeout no longer waits on children that never merged into it"
  ```

---

### Task 3: Rebase, publish, and merge prove an implemented-but-merged child

**Build tier:** standard

**Files:**
- Modify: `internal/app/finalize_preservation.go` — the file header comment: it lists "FinalizeRebase, FinalizePublish, FinalizeMerge, and stacked closeout" as `proveCarriedOnHead`'s users. Rewrite it to say `proveCarriedOnHead` serves rebase, publish, and merge; root closeout derives the same carried set (`domain.DeriveRootCloseoutSet` over `domain.DeriveCarriedSet`) and proves each child with `proveCarriedContent`; stacked closeout uses `requireStackedPreservation`. Adjust `ReasonCarryUnproven`'s comment the same way (it names "stacked-closeout").
- Modify (test helper): `internal/app/finalize_rebase_integration_test.go` — add `seedRebaseCarryChildAt(t *testing.T, f *rebaseFixture, status string)`; `seedRebaseCarryChild(t, f)` becomes `seedRebaseCarryChildAt(t, f, "stacked-merged")`.
- Test: `internal/app/finalize_preservation_integration_test.go`, `internal/app/finalize_merge_integration_test.go`, `internal/app/finalize_rebase_integration_test.go`, `internal/app/finalize_state_integration_test.go`.

**Interfaces:**
- Consumes: Task 1's widened `DeriveCarriedSet`; Task 2's filtered `probeDescendantFacts`. No production signature changes.

- [ ] **Step 1: Write the tests.**
  - `TestIntegrationFinalizeRebaseRecoveryCarryHelperImplementedChild` (preservation file; same fixture style as `…CarryHelperPreservation`, but `seedCarryChild(t, f, "implemented")`). Rows through `proveCarriedOnHead`:
    - `merged-into-parent-content-dropped` — `carryFake(f.head, "feat/widget", mcDropped)` against `tgtDropped`: unproven, one finding containing `CarryFindingUnpreserved`, `0006`, and `mcDropped`.
    - `merged-into-parent-preserved` — `carryFake(f.head, "feat/widget", mcAnc)` against `tgtAnc`: proven, no findings.
    - `merged-to-main-not-carried` — `carryFake(f.head, "main", mcDropped)` against `tgtDropped`: proven, no findings.
    - `open-pr-not-carried` — `&fakeCloseoutGitHub{repo: retargetRepo(), merged: map[int]closeoutProbe{}}` against `tgtDropped`: proven, no findings (an `implemented` child is never a relationship refusal).
  - In `TestIntegrationFinalizeMergeCarryPreservation` add:
    - `implemented-merged-child-lost-refuses-zero-merge-calls` — copy of `all-conditions-hold-but-carried-child-lost-refuses-zero-merge-calls` with `seedRebaseCarryChildAt(t, f.rebaseFixture, "implemented")`; same assertions (blocked / `ReasonCarryUnproven`, findings non-empty, `gh.mergeCalls == 0`).
    - `retargeted-child-merged-to-main-merges` — `seedRebaseCarryChildAt(…, "implemented")`, `mergeCommit := f.mergeFeatureIntoBase(t)`, child #8 scripted merged into `"main"` at the dropped commit; expect `ResultApplied` / `MergeDispMerged` / `gh.mergeCalls == 1`.
  - In `TestIntegrationFinalizeRebaseRecoveryCarryPreservation` add `pre-rewrite-refusal-implemented-merged-child`: copy of `pre-rewrite-refusal-leaves-receipt-workspace-remote-untouched` with `seedRebaseCarryChildAt(t, f, "implemented")`; same assertions.
  - In `TestIntegrationFinalizeStatePublishCarriedDescendant` add `implemented-merged-child-dropped-refuses-before-rewrite`: copy of the dropped-carry row with `seedRebaseCarryChildAt(t, f.rebaseFixture, "implemented")`; same assertions (blocked / `ReasonCarryUnproven`, `PublishDispBlocked`, zero rewrite calls, remote feature ref unchanged).

- [ ] **Step 2: Run them.**
  Run: `go test -count=1 -tags integration ./internal/app -run 'TestIntegrationFinalizeRebaseRecoveryCarry|TestIntegrationFinalizeMergeCarry|TestIntegrationFinalizeStatePublishCarried'`
  Expected: PASS (Tasks 1–2 already widened the rule). If a row passes trivially, prove it is not vacuous in Step 3.

- [ ] **Step 3: Mutation-test.** Drop Task 1's `StatusImplemented` arm (backup copy): the four `…implemented-merged…`/`merged-into-parent-content-dropped` rows must redden, and the `merged-to-main` / `open-pr` / `retargeted…merges` rows must stay green. Restore.

- [ ] **Step 4: Commit.**
  ```bash
  git add internal/app/finalize_preservation.go internal/app/finalize_preservation_integration_test.go internal/app/finalize_merge_integration_test.go internal/app/finalize_rebase_integration_test.go internal/app/finalize_state_integration_test.go
  git commit -m "test(finalize): rebase, publish, and merge prove an implemented child merged into the parent"
  ```

---

### Task 4: The parent-landed-first end-to-end test uses the real closeout

**Build tier:** standard

**Files:**
- Modify: `internal/app/finalize_e2e_test.go` — `TestE2EStackParentLandedFirst`; delete `archiveAsDoneOnOrigin` (its only caller is this test). Remove imports that become unused (`time` and `padID` are used elsewhere in the package/file — let the compiler decide).

**Interfaces:**
- Consumes: `stackCloseout(t *testing.T, s *e2eState, id int, want string) dkResult`.

- [ ] **Step 1: Edit the test.** Replace `archiveAsDoneOnOrigin(t, s, parentID, parentSlug)` with `stackCloseout(t, s, parentID, "done-archived")`, and replace the comment above it with: the parent lands first and closes out through the real closeout; its retargeted, still-open child was never merged into the parent's branch, so it is not carried and does not block. The rest of the story is unchanged.

- [ ] **Step 2: Run it.**
  Run: `go test -count=1 -tags e2e ./internal/app -run 'TestE2EStackParentLandedFirst'` (or `bash tests/test_go_finalize_e2e.sh` if the e2e binaries need the shard's setup).
  Expected: PASS.

- [ ] **Step 3: Mutation-test** with Task 1's "restore the old rule" mutation (backup copy): the e2e test must redden on the parent closeout disposition. Restore and re-run green.

- [ ] **Step 4: Commit.**
  ```bash
  git add internal/app/finalize_e2e_test.go
  git commit -m "test(e2e): close out a landed parent for real past its retargeted child"
  ```

---

### Task 5: Skills and docs describe the new carry rule

**Build tier:** standard

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` step 9 — delete the `children-retarget-required` bullet; in the `root-archived` bullet add that a descendant counts as carried only when its PR merged into its parent's branch (whether it reads `stacked-merged` or still `implemented`), and any other child is left alone and never blocks the root. Leave step 10's cleanup `children-retarget-required` mentions alone.
- Modify: `skills/docket-finalize-change/references/gate-failure.md` — the bullet becomes "an **open unauthorized child** on an autonomous run;".
- Modify: `skills/docket-convention/references/stacked-changes.md`:
  - *The `stacked-merged` state*: keep "the only consumer that takes a change out of it"; add that the same root close-out also takes an `implemented` child whose PR merged into its parent's branch straight to `done`.
  - *The stack close-out is idempotent*: "every carried `stacked-merged` descendant" → "every carried descendant"; the carried definition becomes: its PR merged into its parent's branch (it reads `stacked-merged`, or still `implemented` because its own close-out never ran), down an unbroken chain of such links from the root, **and** its work is proven in Git (keep the existing Git-proof sentences). Add: a child in any other state — an open or retargeted PR, not yet built, killed, already `done` — is not carried, never blocks the root, and is left alone; a child beneath it waits for its own parent. Routing sentence: `done-archived` (nothing carried), `root-archived` (every carried descendant proven), or `contended`/`blocked`/`unknown`.
- Modify: `skills/docket-implement-next/references/edge-paths.md` — keep "A `not-ready-stack-base-unresolved` refusal is never a closeout trigger and keeps its ordinary local refusal" verbatim; replace the rest of that sentence's reason with: the bounded closeout covers only the named change's own `depends_on` set, and an unresolved stack base clears through the parent's own finalize or the next sweep.
- Modify: `internal/repoguard/prose_contracts_test.go` — the comment inside the `change_0448_named_preflight_skip` edge-paths row gets the same true reason (no `children-retarget-required`).
- Modify: `internal/repoguard/budgets_test.go` — the 0448 history comment: append that this closeout reason was later replaced (the closeout disposition was retired) without editing the historical counts; re-pin the four ceilings below.
- Regenerate: `internal/assets/embedded/tree/` via `go generate ./internal/assets/`.

- [ ] **Step 1: Make the edits** above. Re-read each edited paragraph against the Global Constraints rule; no change/PR citations.

- [ ] **Step 2: Regenerate and measure.**
  Run: `go generate ./internal/assets/` then `wc -l -w skills/docket-finalize-change/SKILL.md skills/docket-finalize-change/references/gate-failure.md skills/docket-convention/references/stacked-changes.md skills/docket-implement-next/references/edge-paths.md`
  Current ceilings (lines/words): SKILL.md 243/6254, gate-failure.md 162/2111, stacked-changes.md 223/2217, edge-paths.md 109/1265. Set each `TestSkillSizeBudgets` row to the exact new counts with a leading note `0571: <what changed> (old -> new);` in the row's comment.

- [ ] **Step 3: Run the guards.**
  Run: `go test -count=1 ./internal/repoguard/... ./internal/assets/...`
  Expected: PASS — `TestSkillSizeBudgets`, `TestProseContracts` (the `change_0448_named_preflight_skip` present-string still matches), the embedded-tree drift guard.
  Then confirm no maintained doc still routes on the closeout disposition: `grep -rn -e 'children-retarget-required' skills docs/reference internal/assets/embedded` — every remaining hit must be a **cleanup** disposition.

- [ ] **Step 4: Commit.**
  ```bash
  git add skills internal/assets internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go
  git commit -m "docs(skills): a landed parent carries only the children merged into it"
  ```

---

### Final gate

- [ ] `docket-build` runs `build.test_command` (`go run ./cmd/docket development test`) once at the end. Read the budget report even on green; the closeout, merge, rebase, publish, and finalize-e2e shards all grew.
