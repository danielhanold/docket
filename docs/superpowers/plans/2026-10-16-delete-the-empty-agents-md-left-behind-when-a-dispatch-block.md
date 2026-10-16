<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0537 — Delete the empty AGENTS.md left behind when a dispatch block is retired](../../changes/active/0537-delete-the-empty-agents-md-left-behind-when-a-dispatch-block.md)**
<!-- docket:backlink:end -->

# Delete the empty AGENTS.md left behind when a dispatch block is retired — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build`, task by task, each task under the
> `docket-build-task` contract (focused test, implementation, verification, self-review, one commit).
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When removing docket's managed block leaves a file empty or whitespace only, install and
uninstall delete the file instead of writing it back; `repository set-visibility private` stops
requiring `.git/dckt/AGENTS.md` when the declared harness list is empty.

**Architecture:** One rule in the installer's single managed-block removal step,
`(*Txn).removeManagedBlock` in `internal/install/txn.go`: after the cut, `len(bytes.TrimSpace(out)) == 0`
routes to `removeIfPresent(t.fs, step.Path)` instead of `writeThroughStaging`. Rollback and recovery
need no change (the step's `preFile` pre-image is restored whether the destination holds new bytes
or nothing). One predicate change in `planToPrivate` (`internal/app/repository_set_visibility.go`)
makes the `instructions` phase done when the declared harness list is empty.

**Tech Stack:** Go; `internal/install` unit tests; `internal/app` real-Git integration tests behind
the `integration` build tag, run through the shard scripts in `tests/`.

**Spec:** `docs/superpowers/specs/2026-10-16-delete-the-empty-agents-md-left-behind-when-a-dispatch-block-design.md`
(on the `docket` branch; read it beside this plan).

## Global Constraints

- The emptiness test is exactly `len(bytes.TrimSpace(out)) == 0` — the rule `stripDispatchBlock`
  (`internal/app/repository_set_visibility_private.go`) already applies. Sharing a helper with it is
  optional and the builder's call; they live in different packages.
- Deletion goes through `removeIfPresent(t.fs, step.Path)` (the FS seam), never `os.Remove` directly.
- Unchanged: the `preAbsent` no-op, the `preSymlink` refusal, parse/apply errors, ownership proof
  (an edited block still refuses), hook-entries removal, install output (`remove <path>` covers both
  outcomes — no new action or wording), ownership records, journal format. Directories are never
  removed.
- No ADR expected.
- Cross-references in maintained source anchor on symbol names, never line numbers (ADR-0054).
- Every mutation probe and manual re-verification uses `go test -count=1` (cached results are not
  evidence).
- The build gate is `build.test_command` (`go run ./cmd/docket development test`), whole suite; read
  its budget report (`BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:`) even when green.

## Review Focus

1. **Remove-then-write on one file keeps the user's mode.** `~/.codex/AGENTS.md` holding only a
   leftover block, chmod 0600: the cut deletes it, the pointer write recreates it, and the file ends
   0600 (taken from the write step's `preFile` pre-image). Pinned in Task 2's cut-and-write test.
2. **Crash after a deletion, finished by `Recover`.** Not just `Rollback`: a killed transaction
   (`applySteps` without rollback) whose deletion already ran must be restored byte-for-byte with
   mode by `Recover`. Pinned in Task 1's recovery test.
3. **Whitespace residue with CRLF and tabs.** A block written into a CRLF file, or surrounded by
   `" \t\r\n"`, leaves only whitespace; it must be deleted, not kept. Pinned in Task 1's table.
4. **A tracked AGENTS.md deleted by a harness drop is still a pending review path.** The deletion
   must show up in `PendingPaths` (git-status based), so the human commits it. Pinned in Task 4.
5. **Tests elsewhere that read a retired file and assumed it survives.** The whole-suite gate will
   find them; when one reddens because the file is now absent, update its expectation only if the
   file held nothing but the block (never weaken an assert about user bytes). Task 5 owns this.

---

### Task 1: The rule in `removeManagedBlock`, with transaction-level tests

**Files:**
- Modify: `internal/install/txn.go` — `removeManagedBlock` (body + doc comment), the comment above
  the `t.removeManagedBlock(step)` call in `applyStep`, the `rejectDuplicateDestinations` doc comment
- Test: `internal/install/txn_test.go`

**Interfaces:**
- Consumes: existing `removeIfPresent(fsops FSOps, path string) error`, `writeThroughStaging`,
  test helpers `newFixture`, `managedBlockSpan(interior string) string`, `managedFile`,
  `snapshotWorld`, `assertWorld`, `assertNoStaging`, `injectFS`, `failAtCall`, `recordCalls`,
  `DetectRecovery`, `Recover`, `(*Txn).applySteps`.
- Produces: no new exported API. Behaviour later tasks rely on: a managed-block removal whose
  residue is whitespace-only leaves the path absent (`os.Lstat` → not-exist) and its parent
  directory in place.

- [ ] **Step 1: Write the failing tests** in `internal/install/txn_test.go`, next to
  `TestTxnManagedBlockRemovalStripsOnlyTheBlock`:

  - `TestTxnManagedBlockRemovalDeletesAnEmptiedFile` — table over the file's full bytes, each with
    a removal `TargetRecord{Path: p, Kind: KindManagedBlock, BlockName: "dispatch", Role: "dispatch"}`
    applied via `BeginTxnWithRemovals(RealFS{}, f.roots, nil, []TargetRecord{removal})` + `Apply`:
    - `"only the block"`: `managedBlockSpan("old interior\n")`
    - `"trailing blank lines"`: `managedBlockSpan(...) + "\n\n"`
    - `"spaces and tabs around"`: `" \t\n" + managedBlockSpan(...) + "\t \n"`
    - `"CRLF residue"`: `"\r\n" + managedBlockSpan(...) + "\r\n"`
    Each case writes to `f.path("dispatch", "<case>.md")`, then asserts:
    ```go
    if _, err := os.Lstat(path); !os.IsNotExist(err) {
        t.Errorf("%s: an emptied file survived its block's removal (err=%v)", name, err)
    }
    if fi, err := os.Stat(filepath.Dir(path)); err != nil || !fi.IsDir() {
        t.Errorf("%s: the parent directory was disturbed (err=%v)", name, err)
    }
    assertNoStaging(t, f.targets)
    ```
  - `TestTxnManagedBlockRemovalDeletionRollsBack` — a copy of `TestTxnManagedBlockRemovalRollsBack`
    whose removal targets `f.path("dispatch", "only.md")`, seeded (before `snapshotWorld`) with
    `managedBlockSpan("old interior\n")` and `os.Chmod(path, 0o640)`; the later create stays
    `f.path("dispatch", "zzz.md")`. Same structure: clean run records ops, then every
    `failAtCall(n)` must end with `assertWorld(before, after)`, no staging, no journal. Add one
    guard in the clean run: `ops` must contain a `Remove` of `only.md` (proves the deletion branch
    ran, so the table is not vacuous).
  - `TestRecoveryAfterInterruptedManagedBlockDeletion` — following `TestRecoveryAfterInterrupt`:
    seed `only.md` as above (mode 0o640), plan the removal plus the `zzz.md` create, inject a
    failure on the `zzz.md` publish (`fail: func(op, p string) error` returning an error for
    `op == "Rename" && p == zzz`), call `txn.applySteps()` (expect error), assert `only.md` is
    absent (the interruption really happened after the deletion), then
    `DetectRecovery` → `Recover(RealFS{}, f.roots, id)` → `assertWorld(before, ...)` (bytes and
    mode 0o640 back), no staging, no journal.

- [ ] **Step 2: Run them to verify they fail**

  Run: `go test -count=1 ./internal/install/ -run 'TestTxnManagedBlockRemovalDeletesAnEmptiedFile|TestTxnManagedBlockRemovalDeletionRollsBack|TestRecoveryAfterInterruptedManagedBlockDeletion'`
  Expected: FAIL — `DeletesAnEmptiedFile` reports every case surviving; the rollback test fails its
  "contains a Remove" guard; the recovery test fails its "only.md is absent" precondition.

- [ ] **Step 3: Implement the rule in `(*Txn).removeManagedBlock`**

  After `out, err := doc.Apply(patch)` and its error check:
  ```go
  if len(bytes.TrimSpace(out)) == 0 {
      return removeIfPresent(t.fs, step.Path)
  }
  ```
  then the existing `writeThroughStaging(step, out, 0)`. Update the comments (describe current
  behaviour only, no change numbers):
  - `removeManagedBlock` doc: it cuts exactly the block's marker-to-marker lines and keeps every
    surrounding byte; when nothing but whitespace is left the file was only ever docket's block, so
    it is deleted rather than written back empty (the same rule `stripDispatchBlock` applies); the
    `preFile` pre-image restores either outcome on rollback.
  - `applyStep` comment above the call: replace "it is not a whole-file deletion" with: it keeps
    every surrounding byte, and deletes the file only when no non-whitespace byte is left.
  - `rejectDuplicateDestinations`: after "composes in any order planSteps picks", add that a
    removal may delete a file it leaves empty, and a later write on the same file then recreates it
    (the write reads the path as absent and takes the mode from its own pre-image), so the end
    state is the same in either order.
  - `TestTxnManagedBlockRemovalStripsOnlyTheBlock`'s comment ("it is a rewrite, not a deletion"):
    say the user's prose around the block is what keeps the file. Its asserts stay unchanged.

- [ ] **Step 4: Run the package to verify it passes**

  Run: `go test -count=1 ./internal/install/`
  Expected: PASS, including the unchanged `TestTxnManagedBlockRemovalStripsOnlyTheBlock`,
  `TestTxnManagedBlockRemovalRollsBack`, `TestTxnRefusesRemovalOfAMalformedMarkerFile`, and the
  `TestRetireManagedBlock*EditedInteriorConflicts` / uninstall conflict tests (spec tests 3 and 9).

- [ ] **Step 5: Mutation probes** (restore from a backup copy of `txn.go`, not `git checkout`)
  - Delete the `TrimSpace` branch → `DeletesAnEmptiedFile` (all cases), the rollback guard and the
    recovery precondition redden.
  - Make the branch unconditional (always `removeIfPresent`) → `TestTxnManagedBlockRemovalStripsOnlyTheBlock`
    reddens.
  Record both readings in the task's report.

- [ ] **Step 6: Commit**
  ```bash
  git add internal/install/txn.go internal/install/txn_test.go
  git commit -m "fix(install): delete a file left empty by a managed-block removal"
  ```

### Task 2: Caller-level coverage — cut and write on one file, and uninstall

**Files:**
- Test: `internal/install/retire_test.go`, `internal/install/uninstall_test.go`

**Interfaces:**
- Consumes: Task 1's behaviour; helpers `leftoverCodexWorld`, `leftoverCodexWorldAt`,
  `pointerBlockName`, `versionRoots`, `injectFS`, `newUninstallFixture`, `uninstallOptions`,
  `interiorDigest`, `managedBlockSpan` (defined in `txn_test.go`, same package),
  `document.Parse` / `doc.Block(name)` (`Block.Start`, `Block.End`, `Block.Interior` spans).
- Produces: nothing new.

- [ ] **Step 1: Write the tests**

  In `retire_test.go`:
  - `TestInstallRetiresLeftoverOnlyBlockAndWritesPointerInOneFile` — `opts, path, _ :=
    leftoverCodexWorld(t)`, then overwrite `path` with only the leftover block,
    `"<!-- docket:dispatch:start (managed by docket) -->\nleftover dispatch rules\n<!-- docket:dispatch:end -->\n"`
    (the interior the prior state records), and `os.Chmod(path, 0o600)`. `Install(opts)` must
    succeed. Assert: no `docket:dispatch` text; exactly one `pointerBlockName` block with interior
    `"pointer\n"`; the bytes outside it, `got[:block.Start.Start] + got[block.End.End:]`, are empty
    after `strings.TrimSpace`; `os.Stat(path).Mode().Perm() == 0o600`; actions on `path` are one
    `OpRemove` and one `OpUpdate` (output unchanged); the state holds exactly the pointer record
    for `path`.
  - `TestInstallRetiresLeftoverOnlyBlockAndWritesPointerInOneFileRollsBackBothSteps` — follows
    `…InOneFileRollsBackBothSteps` with the same block-only overwrite (capture it as `original`),
    and the injected failure on the `later` publish. The `injectFS.fail` callback also counts
    `op == "Remove" && p == path`. Expect `agentsRenames == 3` (pointer publish + two restores) and
    `agentsRemoves == 1` (the retirement deleted the file); after rollback the file equals
    `original`; `later` absent; no staging; no journal.

  In `uninstall_test.go`:
  - `TestUninstallManagedBlockRemovalDeletesAnEmptiedFile` — sibling of
    `TestUninstallSatisfiedAbsenceAndManagedBlockRemoval`: `block := f.paths["codex"]`, write
    `managedBlockSpan("owned\n")`, replace the codex record with
    `TargetRecord{Path: block, Kind: KindManagedBlock, BlockName: "dispatch", SHA256: interiorDigest([]byte("owned\n")), Role: "dispatch", Harness: "codex"}`,
    `WriteStateAtomic`, then `Uninstall(uninstallOptions(f))` must have `Err == nil`; assert
    `os.Lstat(block)` is not-exist and `filepath.Dir(block)` still exists. The existing prose test
    stays as is.

- [ ] **Step 2: Run them**

  Run: `go test -count=1 ./internal/install/ -run 'LeftoverOnlyBlock|TestUninstallManagedBlockRemovalDeletesAnEmptiedFile|TestInstallRetiresLeftoverBlockAndWritesPointerInOneFile|TestUninstallSatisfiedAbsence'`
  Expected: PASS (Task 1 already landed the rule; these pin it at the callers).

- [ ] **Step 3: Mutation probe** — delete Task 1's `TrimSpace` branch: the uninstall test and the
  rollback variant's `agentsRemoves`/`agentsRenames` counts redden; the forward cut-and-write test
  stays green by design (spec: its end state is the same either way). Restore from backup.

- [ ] **Step 4: Commit**
  ```bash
  git add internal/install/retire_test.go internal/install/uninstall_test.go
  git commit -m "test(install): pin file deletion for cut-and-write and uninstall"
  ```

### Task 3: `set-visibility private` needs no private file when the harness list is empty

**Files:**
- Modify: `internal/app/repository_set_visibility.go` — the `instructions` step's `done` in
  `planToPrivate`
- Test: `internal/app/repository_set_visibility_integration_test.go` (`//go:build integration`)

**Interfaces:**
- Consumes: `st.facts.SurfacesAuthorized`, `st.sc.cfg.AgentHarnesses.Value` (`[]string`, the
  resolved value behind `SurfacesAuthorized`), `st.privateInstructions`; test helpers
  `newPrivateInitRepo`, `newBornPrivateRepo`, `harnessFlag`, `runSetVisibility`, `phaseStatus`,
  `visibilityPhasePending`, and from `private_instructions_integration_test.go` (same package and
  tag) `setPrivateHarnesses`, `installSurfacesIn`; `layout.PrivateInstructionsPath`.
- Produces: nothing new.

- [ ] **Step 1: Write the failing test** `TestIntegrationRepoVisibilityPrivateInstructionsPhaseWithoutHarnesses`
  (prefix lands it in `tests/test_go_integration_app_repovisibility_private.sh`). Pin `HOME` and
  `XDG_CONFIG_HOME` to temp dirs first in each subtest (as `pinInstructionsUserRoots` does). With
  `primary := EvalSymlinks(r.invocation)`, `common := filepath.Join(primary, ".git")`,
  `ins := layout.PrivateInstructionsPath(common)`:
  - `"born with no harness"`: `newPrivateInitRepo(t, nil)` + `runInitWith(InitOptions{Private: true, Harnesses: harnessFlag("none")})`;
    remove `ins` if present (the case is "no file"). Preview `runSetVisibility(t, SetVisibilityOptions{Target: "private"})`
    and the `Authorized: true` run both return `ResultNoOp`.
  - `"harness list emptied after an install"`: `newBornPrivateRepo(t)` (codex), precondition `ins`
    exists; `setPrivateHarnesses(t, common, "[]")`; `installSurfacesIn(t, primary)`; assert `ins`
    is now absent (Task 1's deletion, end to end); preview and `Authorized: true` both `ResultNoOp`.
  - `"a declared harness still needs the file"`: `newBornPrivateRepo(t)`, `os.Remove(ins)`;
    preview must be `ConfirmationRequired()` with `phaseStatus(res, "instructions") == visibilityPhasePending`.

- [ ] **Step 2: Run it to verify it fails**

  Run: `go test -count=1 -tags integration ./internal/app/ -run '^TestIntegrationRepoVisibilityPrivateInstructionsPhaseWithoutHarnesses$'`
  Expected: FAIL — the first two subtests get confirmation-required (instructions pending) instead
  of no-op; the third passes.

- [ ] **Step 3: Implement** — the `instructions` step's `done` becomes
  `st.privateStateDir && !st.recordWorkingTree && (!st.facts.SurfacesAuthorized || len(st.sc.cfg.AgentHarnesses.Value) == 0 || st.privateInstructions)`
  (a named local such as `noHarnesses` is fine). One comment line: an empty declared list has
  nothing to install, so the phase needs no file; with any harness declared the file is still
  required (existence only).

- [ ] **Step 4: Run to verify it passes**, then the shard:
  `go test -count=1 -tags integration ./internal/app/ -run '^TestIntegrationRepoVisibilityPrivate'`
  Expected: PASS.

- [ ] **Step 5: Mutation probes** — revert the predicate: the two no-harness subtests redden. Make
  the phase done regardless of the file (replace the whole parenthesised clause with `true`): the
  declared-harness subtest reddens.
  Restore from backup.

- [ ] **Step 6: Commit**
  ```bash
  git add internal/app/repository_set_visibility.go internal/app/repository_set_visibility_integration_test.go
  git commit -m "fix(set-visibility): need no private instructions file without harnesses"
  ```

### Task 4: Repository-phase coverage — private retirement and shared harness drop

**Files:**
- Test: `internal/app/private_instructions_integration_test.go`,
  `internal/app/reposetup_harnesses_integration_test.go`

**Interfaces:**
- Consumes: Task 1's behaviour; helpers `newHarnessRepo`, `runConfigureHarnesses`,
  `ConfigureHarnessesOptions`, `harnessFlag`, `contains`, `mustReadFile`, `writeRepoFile`,
  `dispatchStartMarker`, `document.Parse`.
- Produces: nothing new.

- [ ] **Step 1: Tighten `TestIntegrationPrivateInstructionsRetiresBuggyWorktreeSurfaces`** — the
  AGENTS.md assert becomes `if _, err := os.Lstat(filepath.Join(primary, "AGENTS.md")); !os.IsNotExist(err)`
  → error "AGENTS.md survived (err=%v); it held only the dispatch block, so its retirement deletes it".
  Rewrite the comment above it ("its retirement cuts only the block and keeps the file") to: AGENTS.md
  is a managed block inside a file the user may also own; its retirement cuts only the block, and
  since the seeded file held nothing else the file is deleted.

- [ ] **Step 2: Write** `TestIntegrationRepoSetupConfigureHarnessesDropDeletesAnEmptiedAgentsMD`
  (prefix → `tests/test_go_integration_app_reposetup.sh`) with two subtests, each starting from
  `r := newHarnessRepo(t, "codex")`:
  - `"only the block"`: precondition — AGENTS.md parses and the bytes outside its `dispatch` block
    are whitespace-only (so the case is not vacuous). Run
    `runConfigureHarnesses(ConfigureHarnessesOptions{Harnesses: harnessFlag("none")})`, want
    `ResultApplied`; `os.Lstat(AGENTS.md)` is not-exist; `contains(res.PendingPaths, "AGENTS.md")`
    (the tracked deletion is a pending review path).
  - `"user text kept"`: append `"\n## My notes\n\nkeep me\n"` to AGENTS.md (capture the full file
    as `seeded`), drop to `none`; AGENTS.md equals `seeded` with the block's marker-to-marker bytes
    removed (compute the expectation from the parsed block spans of `seeded`, not by hand); it
    contains `"keep me"` and no `docket:dispatch`.

- [ ] **Step 3: Run**
  `go test -count=1 -tags integration ./internal/app/ -run '^TestIntegrationPrivateInstructionsRetiresBuggyWorktreeSurfaces$|^TestIntegrationRepoSetupConfigureHarnesses'`
  Expected: PASS (including the existing `…DropsAHarness`, whose CLAUDE.md assert already tolerates
  a deleted file).

- [ ] **Step 4: Mutation probe** — delete Task 1's `TrimSpace` branch: the tightened private assert
  and the `"only the block"` subtest redden; `"user text kept"` stays green. Restore from backup.

- [ ] **Step 5: Commit**
  ```bash
  git add internal/app/private_instructions_integration_test.go internal/app/reposetup_harnesses_integration_test.go
  git commit -m "test(app): pin AGENTS.md deletion on private retirement and harness drop"
  ```

### Task 5: Whole-suite gate

**Files:** none planned; only tests the gate surfaces (Review Focus 5).

- [ ] **Step 1:** Grep the test tree for asserts that read a retired managed-block file after
  retirement and expect it to exist:
  `grep -rn -e 'AGENTS.md' -e 'CLAUDE.md' -e 'PrivateInstructionsPath' --include='*_test.go' internal`
  — note candidates; the gate decides.
- [ ] **Step 2:** Run `go run ./cmd/docket development test` from the feature worktree.
  Expected: green. For any red test that read a file now deleted because it held only the block,
  update that expectation (absence) and its comment; never weaken an assert about user bytes.
- [ ] **Step 3:** Read the budget report; a `SERIAL CONFIRMED OVER BUDGET:` line for the
  `repovisibility_private`, `privateinstructions`, or `reposetup` shard is acted on (re-seed the
  budget from a serial run, as recent shard work did), screening lines are noted in results.
- [ ] **Step 4:** If Step 2 changed files, commit them:
  `git commit -m "test: expect files emptied by block retirement to be deleted"`.
