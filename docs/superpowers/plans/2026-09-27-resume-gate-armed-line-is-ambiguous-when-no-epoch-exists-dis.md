<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0463 — Resume gate-armed line is ambiguous when no epoch exists — dispatch context gets passed as --run-epoch](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-28-0463-resume-gate-armed-line-is-ambiguous-when-no-epoch-exists-dis.md)**
<!-- docket:backlink:end -->
# Resume Arm Always Binds a Run Epoch — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: this plan is executed by `docket-build` (the resolved
> build skill) task-by-task, each task routed to a build-profile worker under the `docket-build-task`
> contract, with one whole-suite gate at the end. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `run.gate-before --resume <id>` mint and bind a run epoch when the change has none, so
the `gate-armed <key> <epoch> <dispatch-context>` line always has three tokens, and give an unknown
`--run-epoch` the named refusal `unknown-run-epoch` instead of the catch-all `invalid-request`.

**Architecture:** In `RunGateBefore` (`internal/app/rungate_before.go`), step (6a) mints an epoch on
every arm that reaches it (fresh arm or epochless resume). A resume binds `ChangeID` + `Worktree` in
one `epochCAS`. A new `armedGateResult` constructor refuses to arm without an epoch, so `HumanText`
can drop its optional-epoch branch. A new focused file `internal/app/rungate_epoch_refusal.go`
classifies `EpochError`s into stable reason tokens and next-action messages. It is shared by
`mapDriveFailure` (gate drive start), a new resolvability pre-check in `GateDriveService.PrepareScope`,
and a new `agent.enter` preflight (`CheckRunEpochLinkage`) that runs before Codex is spawned.

**Tech Stack:** Go (`internal/app`, `internal/cli`, `internal/gatedrive`), `go test`, cobra CLI.

**Spec:** `docs/superpowers/specs/2026-09-27-resume-gate-armed-line-is-ambiguous-when-no-epoch-exists-dis-design.md`
(on the `docket` metadata branch; read it alongside this plan).

## Global Constraints

- The new refusal token is spelled exactly `unknown-run-epoch` (exported constant `ReasonUnknownRunEpoch`).
- Existing tokens are unchanged: `stale-run-epoch`, `run-cancelled`, `run-completed`, `invalid-request`.
- A reason is always a fixed vocabulary token, never argv, a path, or record content. A next-action message never echoes the presented `--run-epoch` value.
- `run gate-before` JSON result shape is unchanged (`key`, `epoch`, `dispatch_context`, … — same field names and tags).
- A mint or bind failure on the arm path returns `gate-unarmed mint-failed` (`ReasonGateMintFailed`), and no key is returned.
- Parent-facing prose (`AGENTS.md`, `cursor-rules/run-gate.md`, `internal/assets/embedded/tree/cursor-rules/run-gate.md`) already documents the three-token form and is **not edited**. Point-in-time records (archived changes, results, specs, published plans under `docs/superpowers/plans/`) are never edited.
- No per-change resume lock and no credential-hash detection of a dispatch context passed as `--run-epoch` (spec Out of scope).
- Code comments anchor on symbol names or verbatim-quoted clauses, never line numbers (ADR-0054, `TestCommentAnchorStyle`).
- Every new test is mutation-checked: strip the fix and watch it go red. Run mutation probes with `go test -count=1` (the cache otherwise serves pre-mutation verdicts). Restore from a backup copy (`cp f f.bak; <mutate>; <run>; mv -f f.bak f`), never `git checkout --`, which throws away your uncommitted edit.
- The build gate runs the whole suite through `build.test_command` (`go run ./cmd/docket development test`), not only the tests named here.

## Review Focus

1. **Worktree spelled through a symlink** (macOS `/var/...` vs `/private/var/...`). The resume inspect path and the start's `--repo-dir` may name one directory under two spellings, and the start must still be admitted: `epochOwnsWorktree` canonicalizes at compare time. Pinned in Task 6 (inspect with the raw temp path, start with the `EvalSymlinks` path).
2. **The epoch a resume mints goes through the normal cancel → resume cycle.** After it is cancelled, the next resume must reserve exactly one replacement and supersede it, like any other epoch. Pinned in Task 1.
3. **Parent swaps the epoch and dispatch-context fields.** A dispatch context presented as `--run-epoch` against a gate that has a real epoch is refused `unknown-run-epoch` and never admitted. Pinned in Task 6 (the misrouted leg).
4. **`run gate-before --resume --json` consumers.** The `epoch` field is populated on the epochless path and the rest of the JSON shape is unchanged. Pinned in Task 1.
5. **Human-mode (non-`--json`) refusals.** `gate drive prepare-scope` and `agent enter` render the reason and remedy without echoing the presented value. Pinned in Tasks 4 and 5.

---

## File Structure

- `internal/app/rungate_before.go` (modify): step (6a) mints on epochless resume and binds ChangeID+Worktree. Adds the new `armedGateResult`. `HumanText` always prints three tokens. Stale comments are corrected.
- `internal/app/rungate_epoch_refusal.go` (create): run-epoch refusal vocabulary. Contains `ReasonUnknownRunEpoch`, `ClassifyRunEpochError`, `RunEpochNextAction`, `runEpochLocator`, and `CheckRunEpochLinkage`.
- `internal/app/gate_drive.go` (modify): `mapDriveFailure`/`mapDriveResult` classify `EpochError`. `GateScopeResult.Message`. `GateDriveService.epochLocate` plus the `PrepareScope` pre-check, wired in `NewCommandlessGateDriveService`.
- `internal/cli/agent.go` (modify): `agent enter` preflight, plus classification of registration errors.
- `internal/cli/run.go` (modify): `gate-before` comment + cobra `Short`.
- Tests: `internal/app/rungate_before_resume_test.go`, `internal/app/rungate_before_test.go`, `internal/app/rungate_epoch_refusal_test.go` (create), `internal/app/gate_drive_test.go`, `internal/app/rungate_epochless_resume_e2e_test.go` (create), `internal/cli/gate_test.go`, `internal/cli/agent_test.go`.

---

### Task 1: Epochless resume mints and binds a run epoch

**Files:**
- Modify: `internal/app/rungate_before.go` (function `RunGateBefore` steps (4a), (6a), (7); function `armResumeReplacement` final return; field comment on `RunGateBeforeResult.Epoch`; `RunGateBefore` doc comment)
- Test: `internal/app/rungate_before_resume_test.go` (append)

**Interfaces:**
- Consumes: `MintEpochRecord(repoDir, gateKey, changeID string) (EpochRecord, error)`, `epochCAS(repoDir, gateKey string, mutate func(*EpochRecord) error) error`, `FindEpochByChange`, `LoadEpochRecord` (all existing, `rungate_epoch.go`).
- Produces: `func armedGateResult(key, epochID, dispatchContext string) RunGateBeforeResult` (package-private), which returns `gateUnarmed(ReasonGateMintFailed)` when `epochID == ""`. Tasks 2 and 6 rely on it: every armed result has a non-empty `Epoch`.

- [ ] **Step 1: Write the failing tests** (append to `internal/app/rungate_before_resume_test.go`; add `"encoding/json"` to its imports)

```go
// TestEpochlessResumeMintsBoundEpoch (change 0463): resuming an in-progress change
// that has NO prior run epoch (its first dispatch was never armed) mints one. The
// epoch is bound to the change and to the verified feature worktree, and the result
// carries its id, so the armed line is always `gate-armed <key> <epoch> <dispatch-context>`.
func TestEpochlessResumeMintsBoundEpoch(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	deps, wdeps := resumeEpochDeps(t)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	if _, _, found, err := FindEpochByChange(repoDir, "5"); err != nil || found {
		t.Fatalf("fixture must start epochless: found=%v err=%v", found, err)
	}

	res := RunGateBefore(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if !res.Armed || res.Key == "" {
		t.Fatalf("an epochless resume must arm: %q", res.HumanText())
	}
	if res.Epoch == "" {
		t.Fatalf("an epochless resume armed with no epoch: %q", res.HumanText())
	}
	ep, _, err := LoadEpochRecord(repoDir, res.Key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if ep.EpochID != res.Epoch {
		t.Errorf("result Epoch = %q, want the minted epoch id %q", res.Epoch, ep.EpochID)
	}
	if ep.ChangeID != "5" {
		t.Errorf("epoch ChangeID = %q, want \"5\" (bound to the resumed change)", ep.ChangeID)
	}
	if ep.State != EpochActive {
		t.Errorf("epoch state = %q, want active", ep.State)
	}
	if ep.Worktree != "/tmp/wt/epsilon" {
		t.Errorf("epoch Worktree = %q, want the verified feature worktree /tmp/wt/epsilon", ep.Worktree)
	}
	// JSON consumers see the epoch on this path too, and the shape is unchanged.
	buf, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"epoch":"` + res.Epoch + `"`, `"key":"` + res.Key + `"`, `"dispatch_context":"` + scopeGrantChild + `"`} {
		if !strings.Contains(string(buf), want) {
			t.Errorf("JSON result missing %s: %s", want, buf)
		}
	}
}

// TestRepeatEpochlessResumeRefusedActive (change 0463): after an epochless resume
// mints its epoch, a SECOND resume of the same change finds that epoch active and
// refuses resume-active-run with the safe locator, the same single-live-run
// protection every other epoch gets. It mints nothing and prepares no scope.
func TestRepeatEpochlessResumeRefusedActive(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	deps, wdeps := resumeEpochDeps(t)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	first := RunGateBefore(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if !first.Armed {
		t.Fatalf("first epochless resume must arm: %q", first.HumanText())
	}

	deps2, wdeps2 := resumeEpochDeps(t)
	sp2 := &fakeScopePrep{grant: sampleScopeGrant()}
	second := RunGateBefore(context.Background(), deps2, wdeps2, sp2.deps(), repoDir, "implement-next", 5)
	if second.Armed {
		t.Fatalf("a repeat resume over a live minted epoch must not arm: %q", second.HumanText())
	}
	if second.Reason != ReasonGateResumeActiveRun {
		t.Fatalf("Reason = %q, want %q", second.Reason, ReasonGateResumeActiveRun)
	}
	if !strings.Contains(second.Message, first.Epoch) || !strings.Contains(second.Message, first.Key) {
		t.Fatalf("locator must name epoch %q and key %q, got %q", first.Epoch, first.Key, second.Message)
	}
	if second.Key != "" || sp2.calls != 0 {
		t.Fatalf("an active refusal mints nothing: key=%q calls=%d", second.Key, sp2.calls)
	}
}

// TestEpochlessResumeEpochJoinsCancelCycle (change 0463, Review Focus 2): the epoch
// an epochless resume mints goes through the ordinary lifecycle. Once confirmed
// cancelled, the next resume supersedes it and reserves exactly one replacement,
// which carries its own fresh epoch.
func TestEpochlessResumeEpochJoinsCancelCycle(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	deps, wdeps := resumeEpochDeps(t)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	first := RunGateBefore(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if !first.Armed {
		t.Fatalf("first epochless resume must arm: %q", first.HumanText())
	}
	// A confirmed run.cancel leaves the epoch cancelled.
	if err := epochCAS(repoDir, first.Key, func(r *EpochRecord) error {
		r.State = EpochCancelled
		return nil
	}); err != nil {
		t.Fatalf("epochCAS cancel: %v", err)
	}

	deps2, wdeps2 := resumeEpochDeps(t)
	sp2 := &fakeScopePrep{grant: sampleScopeGrant()}
	repl := RunGateBefore(context.Background(), deps2, wdeps2, sp2.deps(), repoDir, "implement-next", 5)
	if !repl.Armed || repl.Epoch == "" || repl.Epoch == first.Epoch {
		t.Fatalf("the resume after cancel must arm one replacement with a fresh epoch: %q (first epoch %q)", repl.HumanText(), first.Epoch)
	}
	prior, _, err := LoadEpochRecord(repoDir, first.Key)
	if err != nil {
		t.Fatalf("LoadEpochRecord(prior): %v", err)
	}
	if prior.State != EpochSuperseded || prior.ReplacementReserved != repl.Key {
		t.Fatalf("prior epoch = (%q, reserved %q), want superseded reserving %q", prior.State, prior.ReplacementReserved, repl.Key)
	}
}

// TestConcurrentEpochlessResumeEpochsFailSafe (change 0463 decision 4, a documenting
// test): epoch locks are per gate key, so two epochless resumes that race past
// FindEpochByChange can each mint an active epoch for one change. The next resume
// must then fail closed as resume-epoch-unreadable and must never arm a third run.
func TestConcurrentEpochlessResumeEpochsFailSafe(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	seedPriorEpoch(t, repoDir, EpochActive)
	seedPriorEpoch(t, repoDir, EpochActive)
	deps, wdeps := resumeEpochDeps(t)
	sp := &fakeScopePrep{grant: sampleScopeGrant()}

	res := RunGateBefore(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if res.Armed {
		t.Fatalf("an ambiguous pair of live epochs must never arm: %q", res.HumanText())
	}
	if res.Reason != ReasonGateResumeEpochUnreadable {
		t.Fatalf("Reason = %q, want %q", res.Reason, ReasonGateResumeEpochUnreadable)
	}
	if res.Key != "" || sp.calls != 0 {
		t.Fatalf("a fail-closed refusal mints nothing: key=%q calls=%d", res.Key, sp.calls)
	}
}

// TestArmedGateResultRequiresEpoch (change 0463): the armed constructor refuses to
// arm without an epoch. That guarantee is what makes the positional three-token
// line unambiguous.
func TestArmedGateResultRequiresEpoch(t *testing.T) {
	if got := armedGateResult("k", "", "ctx"); got.Armed || got.Reason != ReasonGateMintFailed || got.Key != "" || got.DispatchContext != "" {
		t.Fatalf("an epochless armed result must fail closed as gate-unarmed mint-failed, got %+v", got)
	}
	got := armedGateResult("k", "e", "ctx")
	if !got.Armed || got.Result != ResultApplied || got.Key != "k" || got.Epoch != "e" || got.DispatchContext != "ctx" ||
		got.Target != gateBeforeStoredTarget || got.OwnerLifecycle != ReasonOwnerLifecycleUnavailable {
		t.Fatalf("armed result fields wrong: %+v", got)
	}
}
```

- [ ] **Step 2: Run the tests and confirm they fail**

Run: `go test -count=1 ./internal/app/ -run 'TestEpochlessResumeMintsBoundEpoch|TestRepeatEpochlessResumeRefusedActive|TestEpochlessResumeEpochJoinsCancelCycle|TestConcurrentEpochlessResumeEpochsFailSafe|TestArmedGateResultRequiresEpoch'`
Expected: build failure `undefined: armedGateResult`. With that test temporarily commented out, `TestEpochlessResumeMintsBoundEpoch` fails `armed with no epoch`, and `TestRepeatEpochlessResumeRefusedActive` fails because the second resume arms. `TestConcurrentEpochlessResumeEpochsFailSafe` already passes (it documents existing behavior).

- [ ] **Step 3: Implement**

In `internal/app/rungate_before.go`, add `armedGateResult` directly after `gateResumeObserve`:

```go
// armedGateResult builds the armed report for key. Every armed gate carries a run
// epoch (change 0463): parents read the `gate-armed <key> <epoch> <dispatch-context>`
// line positionally, and both tokens are 32-hex, so the line is unambiguous only
// when the epoch slot is always filled. An empty epoch therefore fails closed as
// gate-unarmed mint-failed. It never prints a two-token line whose dispatch context
// a parent would read as the epoch.
func armedGateResult(key, epochID, dispatchContext string) RunGateBeforeResult {
	if epochID == "" {
		return gateUnarmed(ReasonGateMintFailed)
	}
	return newRunGateBeforeResult(ResultApplied, RunGateBeforeResult{
		Armed:           true,
		Key:             key,
		Epoch:           epochID,
		Target:          gateBeforeStoredTarget,
		DispatchContext: dispatchContext,
		OwnerLifecycle:  ReasonOwnerLifecycleUnavailable,
	})
}
```

In `armResumeReplacement`, replace the final `return newRunGateBeforeResult(ResultApplied, RunGateBeforeResult{...})` literal (keep the comment above it) with:

```go
	return armedGateResult(key, epochRec.EpochID, grant.ChildCapability)
```

In `RunGateBefore`, replace the step (4a) comment's last sentence ("No prior epoch (a legacy/pre-epoch resume, or an unclaimed run that never bound one) falls through to the existing resume arm, which shares no epoch and reserves no replacement.") with:

```go
		// ... No prior epoch (a legacy/pre-epoch resume, or a first dispatch that was
		// never armed) falls through to the ordinary arm below, which mints and binds a
		// fresh epoch for the resumed change (step 6a, change 0463), so the armed line
		// always carries one.
```

Replace the whole step (6a) block, from its comment through the closing `}` of `if resumeID == 0 { ... }`, and the step (7) return, with:

```go
	// (6a) Every armed gate binds a run epoch beside the just-minted gate record,
	// keyed by the gate key (rungate_epoch.go). The epoch is the durable coordinator
	// fence that a later human cancellation flips and a resume supersedes. Its EpochID
	// travels onto each scoped start's worktree slot, so an omitted or stale epoch
	// cannot detach the worktree. Two arms reach this step: a FRESH arm, and a RESUME
	// whose change has no prior epoch (a legacy/pre-epoch run, or a first dispatch
	// that was never armed; change 0463). A resume that found a prior epoch never gets
	// here, because every found state returned above (a refusal, an observed
	// reservation, or armResumeReplacement, which mints its own).
	//
	// The epoch is minted unbound. A fresh arm binds ChangeID and Worktree later, at
	// claim confirmation (bindEpochChange / bindEpochWorktree). A resume has already
	// claimed, so it binds both NOW, in one epochCAS, the same way
	// armResumeReplacement binds its worktree. Why both are needed:
	//   - epochLaunchGate refuses an active epoch that has no Worktree, so an unbound
	//     resume epoch would be refused on first use.
	//   - With ChangeID bound, a later resume of the same change finds this epoch
	//     active and refuses resume-active-run.
	// Binding both in one CAS means a failed bind leaves an UNBOUND orphan (inert,
	// like a fresh arm's), never an orphan that names the change. A mint or bind
	// failure unarms fail-closed; the orphan gate record left behind is inert, because
	// no key is returned and nothing dispatches against it.
	epochRec, eerr := MintEpochRecord(repoDir, key, "")
	if eerr != nil {
		return gateUnarmed(ReasonGateMintFailed)
	}
	if resumeID != 0 {
		if werr := epochCAS(repoDir, key, func(rec *EpochRecord) error {
			rec.ChangeID = scopeChangeID
			rec.Worktree = worktree
			return nil
		}); werr != nil {
			return gateUnarmed(ReasonGateMintFailed)
		}
	}

	// (7) Report the armed gate with its dispatch context, its run epoch id, and the
	// honest owner-lifecycle caveat: the dispatched route has no automatic Stop, so a
	// Stop is the explicit `run.cancel` operation keyed by this epoch (change 0375
	// Task 13). armedGateResult refuses an empty epoch, so the line is always three
	// tokens (change 0463).
	return armedGateResult(key, epochRec.EpochID, grant.ChildCapability)
```

Remove the now-unused `var epochID string` declaration.

Update the `RunGateBeforeResult.Epoch` field comment: replace "Empty only on a legacy resume arm that shares no epoch; the resume-active locator already prints the epoch there." with "Never empty on an armed result (change 0463): armedGateResult refuses to arm without one, so the positional `gate-armed <key> <epoch> <dispatch-context>` line always has three tokens."

Update the `RunGateBefore` doc comment: replace "mints the durable record, and returns `gate-armed <key> <dispatch-context>`" with "mints the durable record and its run epoch, and returns `gate-armed <key> <epoch> <dispatch-context>`".

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test -count=1 ./internal/app/ -run 'TestEpochlessResume|TestRepeatEpochlessResumeRefusedActive|TestConcurrentEpochlessResumeEpochsFailSafe|TestArmedGateResultRequiresEpoch|TestGateBefore|TestResume|TestRepeatArm|TestMintSnapshotsRunMaxAttempts'`
Expected: PASS. The existing resume and fresh-arm tests stay green.

- [ ] **Step 5: Mutation-check** (backup-copy restore; `-count=1`)

- Restore `if resumeID == 0 {` around the mint (and the `epochID` var): `TestEpochlessResumeMintsBoundEpoch` reddens (the arm is refused `mint-failed` by the guard).
- Drop `rec.Worktree = worktree` from the resume CAS: `TestEpochlessResumeMintsBoundEpoch` reddens on Worktree.
- Drop `rec.ChangeID = scopeChangeID`: `TestRepeatEpochlessResumeRefusedActive` reddens (the second resume arms).
- Remove the `epochID == ""` guard in `armedGateResult`: `TestArmedGateResultRequiresEpoch` reddens.
- In `FindEpochByChange`, return the first live match instead of `ErrEpochAmbiguous`: `TestConcurrentEpochlessResumeEpochsFailSafe` reddens.

Restore each mutation from the backup and confirm green again.

- [ ] **Step 6: Commit**

```bash
git add internal/app/rungate_before.go internal/app/rungate_before_resume_test.go
git commit -m "fix(rungate): an epochless resume arm mints and binds a run epoch (change 0463)"
```

---

### Task 2: The armed line is always three tokens

**Files:**
- Modify: `internal/app/rungate_before.go` (`RunGateBeforeResult.HumanText` body and doc comment; file header comment block at the top of the file)
- Modify: `internal/cli/run.go` (the `gate-before` comment block and the cobra `Short`)
- Test: `internal/app/rungate_before_test.go` (append)

**Interfaces:**
- Consumes: `armedGateResult` (Task 1); test helpers `resumeEpochDeps`, `seedPriorEpoch` (`rungate_before_resume_test.go`), and `gateBeforeReader`, `gateBeforeCorpus`, `sampleScopeGrant`, `newGateRepo`, `newWorkingRepo`.
- Produces: `HumanText()` whose first line is always `gate-armed <key> <epoch> <dispatch-context>` (Task 6 parses it positionally).

- [ ] **Step 1: Write the failing test** (append to `internal/app/rungate_before_test.go`)

```go
// TestGateArmedLineIsAlwaysThreeTokens (change 0463): every armed result a real arm
// produces (fresh, epochless resume, cancelled-replacement resume) prints a first
// line of exactly four space-separated fields. Field 3 is the epoch and field 4 is
// the dispatch context, so a positional parser can never read the dispatch context
// as the epoch.
func TestGateArmedLineIsAlwaysThreeTokens(t *testing.T) {
	check := func(t *testing.T, res RunGateBeforeResult) {
		t.Helper()
		if !res.Armed {
			t.Fatalf("did not arm: %q", res.HumanText())
		}
		first := strings.SplitN(res.HumanText(), "\n", 2)[0]
		fields := strings.Fields(first)
		if len(fields) != 4 || fields[0] != "gate-armed" {
			t.Fatalf("armed line %q: want exactly `gate-armed <key> <epoch> <dispatch-context>`", first)
		}
		if fields[1] != res.Key || fields[2] != res.Epoch || fields[3] != res.DispatchContext {
			t.Fatalf("armed line %q: fields (%q,%q,%q), want (key %q, epoch %q, dispatch context %q)",
				first, fields[1], fields[2], fields[3], res.Key, res.Epoch, res.DispatchContext)
		}
		if res.Epoch == "" || res.Epoch == res.DispatchContext {
			t.Fatalf("epoch %q must be a distinct non-empty token from the dispatch context %q", res.Epoch, res.DispatchContext)
		}
	}
	t.Run("fresh arm", func(t *testing.T) {
		repo := newGateRepo(t)
		deps := PlanningDeps{Reader: gateBeforeReader(t, gateBeforeCorpus(), nil, nil), Clock: testClock()}
		sp := &fakeScopePrep{grant: sampleScopeGrant()}
		check(t, RunGateBefore(context.Background(), deps, WorkspaceDeps{}, sp.deps(), repo, "implement-next", 0))
	})
	t.Run("epochless resume", func(t *testing.T) {
		repoDir := newWorkingRepo(t, nil).invocation
		deps, wdeps := resumeEpochDeps(t)
		sp := &fakeScopePrep{grant: sampleScopeGrant()}
		check(t, RunGateBefore(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5))
	})
	t.Run("cancelled-replacement resume", func(t *testing.T) {
		repoDir := newWorkingRepo(t, nil).invocation
		seedPriorEpoch(t, repoDir, EpochCancelled)
		deps, wdeps := resumeEpochDeps(t)
		sp := &fakeScopePrep{grant: sampleScopeGrant()}
		check(t, RunGateBefore(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5))
	})
}
```

- [ ] **Step 2: Run it**

Run: `go test -count=1 ./internal/app/ -run TestGateArmedLineIsAlwaysThreeTokens`
Expected: PASS already on top of Task 1 (the invariant now holds). Its value is as a regression pin. Step 5 proves it reddens on the original defect.

- [ ] **Step 3: Implement**

`HumanText` (in `internal/app/rungate_before.go`): replace

```go
			line := "gate-armed " + r.Key
			if r.Epoch != "" {
				line += " " + r.Epoch
			}
			line += " " + r.DispatchContext
```

with

```go
			line := "gate-armed " + r.Key + " " + r.Epoch + " " + r.DispatchContext
```

Replace the `HumanText` doc comment's first sentence ("An armed gate prints `gate-armed <key> <epoch> <dispatch-context>` (the epoch is omitted only on a legacy resume arm that shares no epoch);") with: "An armed gate prints `gate-armed <key> <epoch> <dispatch-context>`. That is always three tokens, because every armed result carries an epoch (armedGateResult, change 0463), so a positional parser can never read the dispatch context as the epoch."

File header block at the top of `rungate_before.go`: replace "`gate-armed <key>` on success" with "`gate-armed <key> <epoch> <dispatch-context>` on success".

`internal/cli/run.go`, the `gate-before` comment: replace "prints `gate-armed\n\t// <key>` (or `gate-unarmed <reason>`)" with "prints `gate-armed\n\t// <key> <epoch> <dispatch-context>` (or `gate-unarmed <reason>`)". Set the cobra `Short` to:

```go
		Short: "Arm the run gate for a dispatched workflow and print gate-armed <key> <epoch> <dispatch-context>",
```

- [ ] **Step 4: Prose audit (derive the sites; never hand-list them)**

Run:
```bash
out=$(grep -rn -e 'gate-armed' . --include='*.go' --include='*.md' 2>/dev/null)
grep -v -e '^./docs/superpowers/' -e '^./docs/results/' -e '^./docs/changes/' -e '^./.git/' -e '_test.go:' <<<"$out"
```
Expected: every remaining prose site documents the three-token form, or is a `Disposition: "gate-armed"` record literal. That means `AGENTS.md`, `cursor-rules/run-gate.md`, and `internal/assets/embedded/tree/cursor-rules/run-gate.md` stay unchanged, and `internal/app/rungate_before.go` plus `internal/cli/run.go` are now fixed. If any other maintained site documents a two-token or optional-epoch form, fix it in this task and list it in the commit body.

Then run `go test -count=1 ./internal/app/ ./internal/cli/ -run 'TestGateBefore|TestGateArmedLine|TestCapability|TestCommentAnchorStyle'` and `go test -count=1 ./internal/repoguard/`.
Expected: PASS.

- [ ] **Step 5: Mutation-check** (backup-copy restore)

Reintroduce the original defect as a pair: restore `if resumeID == 0 {` around the Task 1 mint, remove the `epochID == ""` guard in `armedGateResult`, and restore the `if r.Epoch != ""` conditional in `HumanText`. The `epochless resume` subtest reddens with a 3-field line. Restore and confirm green.

- [ ] **Step 6: Commit**

```bash
git add internal/app/rungate_before.go internal/app/rungate_before_test.go internal/cli/run.go
git commit -m "fix(rungate): the gate-armed line always prints <key> <epoch> <dispatch-context> (change 0463)"
```

---

### Task 3: Named refusal for an unknown run epoch on gate drive start

**Files:**
- Create: `internal/app/rungate_epoch_refusal.go`
- Create: `internal/app/rungate_epoch_refusal_test.go`
- Modify: `internal/app/gate_drive.go` (`mapDriveFailure`, `mapDriveResult`)
- Test: `internal/app/gate_drive_test.go` (append), `internal/cli/gate_test.go` (append)

**Interfaces:**
- Consumes: `AsEpochError`, `EpochErrorKind` constants, `ErrStaleRunEpoch` (`*MutationFenceError`, `.Reason == "stale-run-epoch"`), `Result` constants.
- Produces (used by Tasks 4, 5, and 6):
  - `const ReasonUnknownRunEpoch = "unknown-run-epoch"`
  - `func ClassifyRunEpochError(err error) (Result, string, bool)`: `ok == false` when no `*EpochError` is in the chain.
  - `func RunEpochNextAction(reason string) string`: a message for `unknown-run-epoch` and `stale-run-epoch`, `""` otherwise.

- [ ] **Step 1: Write the failing tests**

`internal/app/rungate_epoch_refusal_test.go`:

```go
package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestClassifyRunEpochError (change 0463): every run-epoch registry failure maps to
// a stable protocol result and a fixed reason token. A not-found epoch is the named
// unknown-run-epoch. A presented value wrapped into the error chain never leaks into
// the reason.
func TestClassifyRunEpochError(t *testing.T) {
	const presented = "0790b760e26444866ef2e156ba383326"
	cases := []struct {
		kind   EpochErrorKind
		res    Result
		reason string
	}{
		{ErrEpochNotFound, ResultInvalidInput, "unknown-run-epoch"},
		{ErrEpochMismatch, ResultInvalidInput, "stale-run-epoch"},
		{ErrEpochAmbiguous, ResultInvalidInput, "epoch-ambiguous"},
		{ErrEpochNotActive, ResultInvalidInput, "epoch-not-active"},
		{ErrEpochOwnerAmbiguous, ResultInvalidInput, "epoch-owner-ambiguous"},
		{ErrEpochOwnerUnresolved, ResultInvalidInput, "epoch-owner-unresolved"},
		{ErrEpochCorrupt, ResultInternalError, "epoch-corrupt"},
		{ErrEpochIO, ResultInternalError, "epoch-io"},
	}
	for _, tc := range cases {
		err := fmt.Errorf("start refused for %s: %w", presented, epochErr(tc.kind, "find-dir-by-id", errors.New(presented)))
		res, reason, ok := ClassifyRunEpochError(err)
		if !ok || res != tc.res || reason != tc.reason {
			t.Errorf("%s: got (%s, %q, %v), want (%s, %q, true)", tc.kind, res, reason, ok, tc.res, tc.reason)
		}
		if strings.Contains(reason, presented) {
			t.Errorf("%s: reason leaked the presented value: %q", tc.kind, reason)
		}
	}
	if _, _, ok := ClassifyRunEpochError(errors.New("plain failure")); ok {
		t.Error("a non-epoch error must not classify")
	}
	if _, _, ok := ClassifyRunEpochError(ErrStaleRunEpoch); ok {
		t.Error("a mutation-fence error is not an epoch-registry error")
	}
}

// TestRunEpochNextAction (change 0463): unknown-run-epoch tells the caller which
// gate-armed field goes where. Its text is distinct from the stale-linkage remedy.
// Other reasons carry no invented message.
func TestRunEpochNextAction(t *testing.T) {
	unknown := RunEpochNextAction(ReasonUnknownRunEpoch)
	for _, want := range []string{"--run-epoch", "--gate-context", "gate-armed <key> <epoch> <dispatch-context>"} {
		if !strings.Contains(unknown, want) {
			t.Errorf("unknown-run-epoch message must mention %q, got %q", want, unknown)
		}
	}
	stale := RunEpochNextAction("stale-run-epoch")
	if stale == "" || stale == unknown {
		t.Errorf("stale-run-epoch needs its own message, got %q", stale)
	}
	if got := RunEpochNextAction("epoch-io"); got != "" {
		t.Errorf("an unmapped reason must yield no message, got %q", got)
	}
}
```

Append to `internal/app/gate_drive_test.go`:

```go
// TestMapDriveFailureEpochErrors (change 0463): an EpochError chained through the
// gate-drive seam (the epoch launch gate refusing an unknown --run-epoch) surfaces
// its named token, never the catch-all invalid-request. The service attaches the
// next-action message, and neither the reason nor the message echoes the value.
func TestMapDriveFailureEpochErrors(t *testing.T) {
	const presented = "0790b760e26444866ef2e156ba383326"
	wrapped := fmt.Errorf("refused %s: %w", presented, epochErr(ErrEpochNotFound, "find-dir-by-id", nil))
	res, reason := mapDriveFailure(wrapped)
	if res != ResultInvalidInput || reason != ReasonUnknownRunEpoch {
		t.Fatalf("mapDriveFailure = (%s, %q), want (invalid-input, unknown-run-epoch)", res, reason)
	}
	if res, reason := mapDriveFailure(epochErr(ErrEpochIO, "find-by-id", nil)); res != ResultInternalError || reason != "epoch-io" {
		t.Fatalf("an unreadable registry must be an internal error, got (%s, %q)", res, reason)
	}
	eng := &fakeDriveEngine{err: wrapped}
	got := newGateDriveService(eng, 0, "", "").Advance("d1", "owner")
	if got.Reason != ReasonUnknownRunEpoch {
		t.Fatalf("service reason = %q, want unknown-run-epoch", got.Reason)
	}
	if !strings.Contains(got.Message, "--gate-context") {
		t.Fatalf("service must attach the unknown-run-epoch next action, got %q", got.Message)
	}
	if strings.Contains(got.Message, presented) || strings.Contains(got.HumanText(), presented) {
		t.Fatalf("the presented value leaked: message=%q human=%q", got.Message, got.HumanText())
	}
}
```

Append to `internal/cli/gate_test.go`:

```go
// TestGateDriveStartUnknownRunEpochIsNamed (change 0463): the 0382 misuse, where a
// well-formed but unknown --run-epoch (a dispatch-context-shaped 32-hex token) goes
// through the REAL epoch launch gate, is refused invalid-input with the named
// unknown-run-epoch, never the catch-all invalid-request. The presented value is
// never echoed.
func TestGateDriveStartUnknownRunEpochIsNamed(t *testing.T) {
	wt := gateDriveConfiguredRepo(t, "metadata_branch: main\n")
	root := gateTempDir(t)
	const bogus = "0790b760e26444866ef2e156ba383326"
	out, _, _ := runCLI(t, "--json", "gate", "drive", "start",
		"--repo-dir", wt, "--run-root", root, "--owner", "task",
		"--change-id", "463", "--task-id", "task-3", "--phase", "build", "--branch", "fix/x",
		"--run-epoch", bogus, "--", "/bin/echo", "hi")
	doc := decodeOneJSON(t, out)
	if doc["result"] != "invalid-input" || doc["reason"] != "unknown-run-epoch" {
		t.Fatalf("unknown --run-epoch must refuse invalid-input/unknown-run-epoch, got %v", doc)
	}
	if _, ok := doc["drive"]; ok {
		t.Fatalf("a refused start must carry no drive document: %v", doc)
	}
	if msg, _ := doc["message"].(string); !strings.Contains(msg, "--gate-context") {
		t.Fatalf("refusal must carry the next action, got %q", msg)
	}
	if strings.Contains(out, bogus) {
		t.Fatalf("the presented --run-epoch value leaked into the output: %s", out)
	}
}
```

- [ ] **Step 2: Run the tests and confirm they fail**

Run: `go test -count=1 ./internal/app/ -run 'TestClassifyRunEpochError|TestRunEpochNextAction|TestMapDriveFailureEpochErrors'` and `go test -count=1 ./internal/cli/ -run TestGateDriveStartUnknownRunEpochIsNamed`
Expected: app build fails with `undefined: ClassifyRunEpochError / ReasonUnknownRunEpoch / RunEpochNextAction`. The CLI test fails with `reason:invalid-request`.

- [ ] **Step 3: Implement**

Create `internal/app/rungate_epoch_refusal.go`:

```go
package app

// This file is the run-epoch refusal vocabulary (change 0463). The run epoch is a
// public locator (ADR-0111) that a caller threads into --run-epoch flags (gate drive
// start, gate drive prepare-scope, agent.enter). When the presented value cannot be
// resolved, the caller must learn WHICH mistake it made through a stable token. A
// catch-all invalid-request makes a misrouted token (0382: the dispatch context
// passed as the epoch) indistinguishable from a malformed request. Tokens are a
// fixed vocabulary; nothing here echoes the presented value, a path, or record
// content.

// ReasonUnknownRunEpoch is the stable refusal token for a --run-epoch that names no
// run epoch in this repository.
const ReasonUnknownRunEpoch = "unknown-run-epoch"

// ClassifyRunEpochError maps a run-epoch registry failure (an *EpochError anywhere
// in err's chain) to a protocol result and a bounded reason token:
//   - not-found: unknown-run-epoch.
//   - mismatch: the existing stale-linkage token, stale-run-epoch.
//   - corrupt or unreadable: internal-error carrying the kind.
//   - any other readable-but-unusable registry state: invalid-input carrying the kind.
//
// ok is false when err carries no *EpochError, so callers fall through to their
// own classification.
func ClassifyRunEpochError(err error) (Result, string, bool) {
	ee, ok := AsEpochError(err)
	if !ok {
		return "", "", false
	}
	switch ee.Kind {
	case ErrEpochNotFound:
		return ResultInvalidInput, ReasonUnknownRunEpoch, true
	case ErrEpochMismatch:
		return ResultInvalidInput, ErrStaleRunEpoch.Reason, true
	case ErrEpochCorrupt, ErrEpochIO:
		return ResultInternalError, string(ee.Kind), true
	default:
		return ResultInvalidInput, string(ee.Kind), true
	}
}

// RunEpochNextAction maps a run-epoch refusal reason to a one-line, credential-free
// next action (the ownershipNextAction / fenceNextAction pattern). It never echoes
// the presented value. A reason with no specific remedy yields "", and callers then
// omit the message.
func RunEpochNextAction(reason string) string {
	switch reason {
	case ReasonUnknownRunEpoch:
		return "the --run-epoch value names no run epoch in this repository; pass the <epoch> field of the arm's " +
			"`gate-armed <key> <epoch> <dispatch-context>` line (the <dispatch-context> goes to --gate-context) — " +
			"never drop --run-epoch and retry"
	case ErrStaleRunEpoch.Reason:
		return "the --run-epoch value is not the run epoch this gate key carries; pass the <epoch> printed on the same gate-armed line as the key"
	default:
		return ""
	}
}
```

In `internal/app/gate_drive.go` `mapDriveFailure`, insert directly after the `AsMutationFenceError` block (before `gatedrive.AsStoreError`):

```go
	// A run-epoch registry failure (the epoch launch gate could not resolve the
	// presented --run-epoch) surfaces its named token rather than collapsing to the
	// generic invalid-request (change 0463): unknown-run-epoch for a not-found epoch,
	// the kind for any other registry fault.
	if res, reason, ok := ClassifyRunEpochError(err); ok {
		return res, reason
	}
```

In `mapDriveResult`, extend the `if oe, ok := gatedrive.AsOwnershipError(err); ok { ... } else if fe, ok := AsMutationFenceError(err); ok { ... }` chain with:

```go
		} else if _, reason, ok := ClassifyRunEpochError(err); ok {
			result.Message = RunEpochNextAction(reason)
		}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test -count=1 ./internal/app/ -run 'TestClassifyRunEpochError|TestRunEpochNextAction|TestMapDriveFailure|TestEpochLaunchGate'` and `go test -count=1 ./internal/cli/ -run 'TestGateDrive'`
Expected: PASS.

- [ ] **Step 5: Mutation-check** (backup-copy restore)

- Delete the `ClassifyRunEpochError` block from `mapDriveFailure`: `TestMapDriveFailureEpochErrors` and `TestGateDriveStartUnknownRunEpochIsNamed` redden (`invalid-request`).
- Delete the `mapDriveResult` branch: the message asserts redden.
- Map `ErrEpochNotFound` to `string(ee.Kind)`: `TestClassifyRunEpochError` reddens.

- [ ] **Step 6: Commit**

```bash
git add internal/app/rungate_epoch_refusal.go internal/app/rungate_epoch_refusal_test.go internal/app/gate_drive.go internal/app/gate_drive_test.go internal/cli/gate_test.go
git commit -m "fix(gate): an unknown --run-epoch refuses unknown-run-epoch, not invalid-request (change 0463)"
```

---

### Task 4: prepare-scope refuses an unresolvable --run-epoch

`gatedrive.Driver.PrepareScope` stores `RunEpochID` without resolving it, so today a bogus epoch is baked silently into the scope and only fails later, at start. This task adds a resolvability (not liveness) pre-check in the app seam.

**Files:**
- Modify: `internal/app/rungate_epoch_refusal.go` (add `runEpochLocator`; add imports `"path/filepath"`)
- Modify: `internal/app/gate_drive.go` (`GateScopeResult` + `HumanText`; `GateDriveService` field `epochLocate`; `PrepareScope`; `NewCommandlessGateDriveService`)
- Test: `internal/app/gate_drive_test.go`, `internal/cli/gate_test.go` (append)

**Interfaces:**
- Consumes: `findEpochDirByID(rungateRoot, epochID string) (string, EpochRecord, error)` (existing), `ClassifyRunEpochError`, `RunEpochNextAction`, `ReasonUnknownRunEpoch` (Task 3), `mapDriveFailure`.
- Produces: `func runEpochLocator(gitCommonDir string) func(string) error`. Field `GateDriveService.epochLocate func(epochID string) error`. Field `GateScopeResult.Message string` (`json:"message,omitempty"`).

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/gate_drive_test.go`:

```go
// TestPrepareScopeRefusesUnknownRunEpoch (change 0463): a presented --run-epoch that
// the registry cannot resolve is refused before any scope is minted, with the named
// token and the next action and without echoing the value. A scope with no
// --run-epoch never consults the locator (standalone scopes are unchanged).
func TestPrepareScopeRefusesUnknownRunEpoch(t *testing.T) {
	eng := &fakeDriveEngine{grant: gatedrive.ScopeGrant{ScopeID: "scope-1", ChildCapability: "c", ParentCapability: "p"}}
	svc := newGateDriveService(eng, 0, "", "")
	var asked string
	svc.epochLocate = func(id string) error {
		asked = id
		return epochErr(ErrEpochNotFound, "find-dir-by-id", nil)
	}
	got := svc.PrepareScope(gatedrive.ScopeRequest{ChangeID: "463", RunEpochID: "bogus-epoch-value"})
	if got.Result != ResultInvalidInput || got.Reason != ReasonUnknownRunEpoch {
		t.Fatalf("got (%s, %q), want (invalid-input, unknown-run-epoch)", got.Result, got.Reason)
	}
	if got.ScopeID != "" || got.ChildCapability != "" || got.ParentCapability != "" {
		t.Fatalf("a refused prepare-scope must carry no grant: %+v", got)
	}
	if eng.lastScopeReq.ChangeID != "" {
		t.Fatalf("an unresolvable epoch must mint no scope, engine saw %+v", eng.lastScopeReq)
	}
	if asked != "bogus-epoch-value" {
		t.Fatalf("locator asked %q, want the presented id", asked)
	}
	if !strings.Contains(got.Message, "--gate-context") || !strings.Contains(got.HumanText(), "unknown-run-epoch") {
		t.Fatalf("refusal must carry reason and next action: message=%q human=%q", got.Message, got.HumanText())
	}
	if strings.Contains(got.Message, "bogus-epoch-value") || strings.Contains(got.HumanText(), "bogus-epoch-value") {
		t.Fatalf("the presented value leaked: %q / %q", got.Message, got.HumanText())
	}

	asked = ""
	if ok := svc.PrepareScope(gatedrive.ScopeRequest{ChangeID: "463"}); ok.Result != ResultApplied || asked != "" {
		t.Fatalf("a scope without --run-epoch must skip the locator and apply: result=%s asked=%q", ok.Result, asked)
	}
}
```

Append to `internal/cli/gate_test.go`:

```go
// TestGateDrivePrepareScopeUnknownRunEpochIsNamed (change 0463): through the real
// wiring, prepare-scope with an unknown --run-epoch refuses unknown-run-epoch and
// mints no scope. In human mode it renders reason + remedy without the value. The
// cancelled-epoch prepare in TestGateDrivePrepareScopeRunEpochGatesTakeover must
// stay applied, because the pre-check is resolvability only, never liveness.
func TestGateDrivePrepareScopeUnknownRunEpochIsNamed(t *testing.T) {
	wt := gateDriveRepo(t)
	const bogus = "0790b760e26444866ef2e156ba383326"
	args := []string{"gate", "drive", "prepare-scope", "--repo-dir", wt, "--change-id", "463",
		"--task-id", "task-4", "--phase", "build", "--branch", "fix/x", "--worktree", wt, "--run-epoch", bogus}

	out, _, _ := runCLI(t, append([]string{"--json"}, args...)...)
	doc := decodeOneJSON(t, out)
	if doc["result"] != "invalid-input" || doc["reason"] != "unknown-run-epoch" {
		t.Fatalf("got %v, want invalid-input/unknown-run-epoch", doc)
	}
	if id, _ := doc["scope_id"].(string); id != "" {
		t.Fatalf("a refused prepare-scope minted scope %q", id)
	}
	if strings.Contains(out, bogus) {
		t.Fatalf("JSON output leaked the presented value: %s", out)
	}

	// A non-applied result may render on either stream; check both together.
	hOut, hErr, _ := runCLI(t, args...)
	human := hOut + hErr
	if !strings.Contains(human, "unknown-run-epoch") || !strings.Contains(human, "--gate-context") || strings.Contains(human, bogus) {
		t.Fatalf("human output must name reason + remedy and never the value, got %q", human)
	}
}
```

- [ ] **Step 2: Run the tests and confirm they fail**

Run: `go test -count=1 ./internal/app/ -run TestPrepareScopeRefusesUnknownRunEpoch` and `go test -count=1 ./internal/cli/ -run TestGateDrivePrepareScopeUnknownRunEpochIsNamed`
Expected: app build fails (`svc.epochLocate undefined`, `got.Message undefined`). The CLI test fails with `result:applied` (a scope was minted).

- [ ] **Step 3: Implement**

Append to `internal/app/rungate_epoch_refusal.go` (and add `import "path/filepath"`):

```go
// runEpochLocator builds the existence check prepare-scope runs on a presented
// --run-epoch. It resolves the id to exactly one epoch record under gitCommonDir's
// run-epoch registry through findEpochDirByID (the same locator the epoch launch
// gate uses) and returns its typed EpochError (not-found, ambiguous, IO) unchanged.
// It checks RESOLVABILITY only, never liveness: a scope may legitimately carry a
// cancelled epoch (the takeover revocation gate reads it later), and the launch gate
// still enforces liveness and worktree ownership at start.
func runEpochLocator(gitCommonDir string) func(string) error {
	rungateRoot := filepath.Join(gitCommonDir, "docket", "rungate")
	return func(epochID string) error {
		_, _, err := findEpochDirByID(rungateRoot, epochID)
		return err
	}
}
```

In `internal/app/gate_drive.go`:

1. `GateScopeResult`: add after `Reason`:
```go
	// Message is the one-line next action on a command failure (change 0463), e.g.
	// the unknown-run-epoch remedy. It never carries a capability or the presented
	// run-epoch value.
	Message string `json:"message,omitempty"`
```
   In `GateScopeResult.HumanText`, append after the reason line:
```go
	if r.Message != "" {
		lines = append(lines, "message: "+r.Message)
	}
```
2. `GateDriveService`: add the field after `maxAttempts`:
```go
	// epochLocate resolves a presented run-epoch id against the repository's run-epoch
	// registry before PrepareScope mints a scope (change 0463). A non-nil error is a
	// typed EpochError. Nil on the fake-engine test seam and on services that never
	// serve prepare-scope.
	epochLocate func(epochID string) error
```
3. `PrepareScope`: insert at the top of the method:
```go
	// A presented --run-epoch must resolve before it is baked into the scope (change
	// 0463): an unresolvable one refuses now with its named token, instead of
	// surfacing later at start as a refusal the caller cannot attribute.
	if req.RunEpochID != "" && s.epochLocate != nil {
		if lerr := s.epochLocate(req.RunEpochID); lerr != nil {
			res, reason := mapDriveFailure(lerr)
			return GateScopeResult{
				Envelope: NewEnvelope(OperationGateDrivePrepareScope, res),
				Reason:   reason,
				Message:  RunEpochNextAction(reason),
			}
		}
	}
```
4. `NewCommandlessGateDriveService`: replace `return newGateDriveService(engine, 0, "", ""), "", ""` with:
```go
	svc := newGateDriveService(engine, 0, "", "")
	// prepare-scope is served by this commandless service: resolve a presented
	// --run-epoch against the same registry before minting a scope (change 0463).
	svc.epochLocate = runEpochLocator(gitCommonDir)
	return svc, "", ""
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test -count=1 ./internal/app/ -run 'TestPrepareScope'` and `go test -count=1 ./internal/cli/ -run 'TestGateDrivePrepareScope|TestGateDriveScopeBoundStartRoundTrips|TestGateDriveTakeover'`
Expected: PASS, including the existing `TestGateDrivePrepareScopeRunEpochGatesTakeover` (cancelled epoch still prepares).

- [ ] **Step 5: Mutation-check** (backup-copy restore)

- Remove the pre-check block from `PrepareScope`: both new tests redden (a scope is minted).
- Remove the `svc.epochLocate = ...` wiring: only the CLI test reddens (this proves the production wiring, not just the seam).
- Make `runEpochLocator` also refuse non-active epochs: `TestGateDrivePrepareScopeRunEpochGatesTakeover` reddens. That confirms the existence-only boundary is pinned.

- [ ] **Step 6: Commit**

```bash
git add internal/app/rungate_epoch_refusal.go internal/app/gate_drive.go internal/app/gate_drive_test.go internal/cli/gate_test.go
git commit -m "fix(gate): prepare-scope refuses an unresolvable --run-epoch as unknown-run-epoch (change 0463)"
```

---

### Task 5: agent.enter preflights its run-epoch linkage

Traced: `agent enter --run-gate-key K --run-epoch E` registers the Codex thread through `RegisterEpochParticipant` (`epochCAS` on K) only after Codex's app-server has started and a thread exists. A missing epoch (`ErrEpochNotFound`) or a wrong id (`ErrEpochMismatch`) then surfaces as the generic `root-entry-failed` (`ResultExternalFailed`). This task adds a read-only preflight before anything is spawned (before the death guardian and before `client.Enter`), and classifies a registration-time `EpochError` the same way.

**Files:**
- Modify: `internal/app/rungate_epoch_refusal.go` (add `CheckRunEpochLinkage`)
- Modify: `internal/cli/agent.go` (inside the `if runGateKey != "" && runEpoch != ""` block, and the `client.Enter` error branch)
- Test: `internal/app/rungate_epoch_refusal_test.go`, `internal/cli/agent_test.go` (append)

**Interfaces:**
- Consumes: `LoadEpochRecord`, `AsGateStoreError`, `ErrGateNotFound`, `ErrGateMalformedKey`, `ClassifyRunEpochError`, `RunEpochNextAction` (Task 3).
- Produces: `func CheckRunEpochLinkage(repoDir, gateKey, epochID string) error`, which returns nil or always an `*EpochError`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/rungate_epoch_refusal_test.go` (add `"os"` and `"path/filepath"` to its imports):

```go
// TestCheckRunEpochLinkage (change 0463): the agent.enter preflight answers with a
// typed EpochError. Not-found covers both a gate key with no epoch and a gate key
// that does not exist (the pair names no epoch). Mismatch covers a different
// recorded id. A matching pair is nil.
func TestCheckRunEpochLinkage(t *testing.T) {
	repo := newGateRepo(t)
	bare := mintTestGateKey(t, repo)
	if err := CheckRunEpochLinkage(repo, bare, "0790b760e26444866ef2e156ba383326"); !isEpochKind(err, ErrEpochNotFound) {
		t.Fatalf("gate key without an epoch: got %v, want epoch-not-found", err)
	}

	withEpoch := mintTestGateKey(t, repo)
	ep, err := MintEpochRecord(repo, withEpoch, "463")
	if err != nil {
		t.Fatalf("MintEpochRecord: %v", err)
	}
	if err := CheckRunEpochLinkage(repo, withEpoch, ep.EpochID); err != nil {
		t.Fatalf("matching pair must pass, got %v", err)
	}
	if err := CheckRunEpochLinkage(repo, withEpoch, "0790b760e26444866ef2e156ba383326"); !isEpochKind(err, ErrEpochMismatch) {
		t.Fatalf("wrong epoch id: got %v, want epoch-mismatch", err)
	}

	gone := mintTestGateKey(t, repo)
	root, rerr := gateRoot(repo)
	if rerr != nil {
		t.Fatalf("gateRoot: %v", rerr)
	}
	if err := os.RemoveAll(filepath.Join(root, gone)); err != nil {
		t.Fatalf("remove gate dir: %v", err)
	}
	if err := CheckRunEpochLinkage(repo, gone, ep.EpochID); !isEpochKind(err, ErrEpochNotFound) {
		t.Fatalf("absent gate key: got %v, want epoch-not-found", err)
	}
}

func isEpochKind(err error, kind EpochErrorKind) bool {
	ee, ok := AsEpochError(err)
	return ok && ee.Kind == kind
}
```

Append to `internal/cli/agent_test.go`:

```go
// TestAgentEnterRefusesBadRunEpochLinkageBeforeLaunch (change 0463): an agent.enter
// whose --run-gate-key/--run-epoch pair names no run epoch, or names a different one,
// is refused with a named token BEFORE Codex is spawned. A stub codex that records
// any invocation proves nothing launched. The presented value never appears in the
// JSON or human output.
func TestAgentEnterRefusesBadRunEpochLinkageBeforeLaunch(t *testing.T) {
	seedAgentInstallation(t)
	repo := gateDriveRepo(t)
	bin := testsupport.TempDir(t)
	marker := filepath.Join(bin, "codex-invoked")
	stub := "#!/bin/sh\ntouch '" + strings.ReplaceAll(marker, "'", "'\\''") + "'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	const bogus = "0790b760e26444866ef2e156ba383326"
	mintKey := func() string {
		key, err := app.MintGateRecord(repo, app.GateRecord{Target: "docket-implement-next", AttemptLimit: 1, Retry: app.RetryUnused, Disposition: "gate-armed"})
		if err != nil {
			t.Fatalf("MintGateRecord: %v", err)
		}
		return key
	}
	bare := mintKey()
	withEpoch := mintKey()
	if _, err := app.MintEpochRecord(repo, withEpoch, "463"); err != nil {
		t.Fatalf("MintEpochRecord: %v", err)
	}

	for _, tc := range []struct {
		name, key, wantReason string
	}{
		{"gate key with no epoch", bare, "unknown-run-epoch"},
		{"epoch id not the key's", withEpoch, "stale-run-epoch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := []string{"agent", "enter", "--role", "docket-implement-next", "--request", "-", "--cwd", repo,
				"--approval-policy", "never", "--sandbox", "workspace-write", "--run-gate-key", tc.key, "--run-epoch", bogus}
			var out, stderr bytes.Buffer
			Run(append(base, "--json"), strings.NewReader("req"), &out, &stderr, devInfo(), hostFacts())
			var res app.AgentEnterResult
			if err := json.Unmarshal(out.Bytes(), &res); err != nil {
				t.Fatalf("decode %q: %v (stderr %q)", out.String(), err, stderr.String())
			}
			if res.Result != app.ResultInvalidInput || res.Reason != tc.wantReason {
				t.Fatalf("got (%s, %q), want (invalid-input, %q): %+v", res.Result, res.Reason, tc.wantReason, res)
			}
			if strings.Contains(out.String(), bogus) {
				t.Fatalf("JSON output leaked the presented value: %s", out.String())
			}
			var human, herr bytes.Buffer
			Run(base, strings.NewReader("req"), &human, &herr, devInfo(), hostFacts())
			if !strings.Contains(human.String()+herr.String(), "--run-epoch") || strings.Contains(human.String()+herr.String(), bogus) {
				t.Fatalf("human output must name the remedy and never the value: out=%q err=%q", human.String(), herr.String())
			}
			if _, err := os.Stat(marker); err == nil {
				t.Fatalf("codex was launched despite a bad run-epoch linkage")
			}
		})
	}
}
```

(If a git-repository `--cwd` changes how the role contract resolves in this fixture, mirror the setup of `TestAgentEnterCLIUsesEffectiveRepositoryRoleBeforeGlobal`. The assertions stay the same.)

- [ ] **Step 2: Run the tests and confirm they fail**

Run: `go test -count=1 ./internal/app/ -run TestCheckRunEpochLinkage` and `go test -count=1 ./internal/cli/ -run TestAgentEnterRefusesBadRunEpochLinkageBeforeLaunch`
Expected: app build fails (`undefined: CheckRunEpochLinkage`). The CLI test fails with `root-entry-failed`, and the marker exists (codex was launched).

- [ ] **Step 3: Implement**

Append to `internal/app/rungate_epoch_refusal.go`:

```go
// CheckRunEpochLinkage verifies, before agent.enter spawns anything, that the
// presented (--run-gate-key, --run-epoch) pair names a real run epoch (change 0463).
// It returns nil when the gate key's epoch record carries exactly epochID, and
// otherwise ALWAYS an *EpochError:
//   - a gate key with no directory, a malformed key, or no epoch record:
//     ErrEpochNotFound (the pair names no epoch);
//   - a different recorded id: ErrEpochMismatch;
//   - a corrupt record: keeps ErrEpochCorrupt;
//   - any other resolution fault: ErrEpochIO.
//
// It only reads. It never checks liveness, because participant registration still
// refuses a non-active epoch.
func CheckRunEpochLinkage(repoDir, gateKey, epochID string) error {
	rec, _, err := LoadEpochRecord(repoDir, gateKey)
	if err != nil {
		if _, ok := AsEpochError(err); ok {
			return err
		}
		if ge, ok := AsGateStoreError(err); ok && (ge.Kind == ErrGateNotFound || ge.Kind == ErrGateMalformedKey) {
			return epochErr(ErrEpochNotFound, "check-linkage", nil)
		}
		return epochErr(ErrEpochIO, "check-linkage", err)
	}
	if rec.EpochID != epochID {
		return epochErr(ErrEpochMismatch, "check-linkage", nil)
	}
	return nil
}
```

In `internal/cli/agent.go`, add a helper below `epochParticipantRegistrar`:

```go
// runEpochRefusal renders a typed run-epoch linkage failure as the agent.enter
// refusal (change 0463): the named reason token and a credential-free next action.
// It never includes the presented value.
func runEpochRefusal(role string, res app.Result, reason string) app.AgentEnterResult {
	msg := app.RunEpochNextAction(reason)
	if msg == "" {
		msg = "run-epoch linkage refused (" + reason + ")"
	}
	return app.AgentEnterResult{Envelope: app.NewEnvelope(app.OperationAgentEnter, res), Role: role, Reason: reason, Message: msg}
}
```

Make the preflight the first statement inside `if runGateKey != "" && runEpoch != "" {`, before `kind := "task"`:

```go
				// Preflight the linkage BEFORE anything is spawned (change 0463). An unknown
				// or mismatched epoch refuses with its named token, instead of surfacing as a
				// generic root-entry failure after Codex already started a thread.
				if lerr := app.CheckRunEpochLinkage(effectiveCWD, runGateKey, runEpoch); lerr != nil {
					res, reason, _ := app.ClassifyRunEpochError(lerr)
					setResult(runEpochRefusal(role, res, reason))
					return nil
				}
```

In the `out, err := client.Enter(...)` error branch, classify first:

```go
			if err != nil {
				// A registration-time epoch fault (e.g. the epoch was fenced after the
				// preflight) keeps its named token (change 0463).
				if res, reason, ok := app.ClassifyRunEpochError(err); ok {
					setResult(runEpochRefusal(role, res, reason))
					return nil
				}
				setResult(app.AgentEnterResult{Envelope: app.NewEnvelope(app.OperationAgentEnter, app.ResultExternalFailed), Role: role, Reason: "root-entry-failed", Message: err.Error()})
				return nil
			}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test -count=1 ./internal/app/ -run 'TestCheckRunEpochLinkage|TestClassifyRunEpochError'` and `go test -count=1 ./internal/cli/ -run 'TestAgentEnter'`
Expected: PASS. All existing `TestAgentEnter*` tests stay green (none pass the linkage flags).

- [ ] **Step 5: Mutation-check** (backup-copy restore)

- Remove the preflight block: the CLI test reddens (marker created; reason `root-entry-failed`).
- In `CheckRunEpochLinkage`, drop the `ErrGateNotFound` mapping (fall to `ErrEpochIO`): the `absent gate key` leg of `TestCheckRunEpochLinkage` reddens.
- Drop the `rec.EpochID != epochID` check: the mismatch legs redden in both tests.

- [ ] **Step 6: Commit**

```bash
git add internal/app/rungate_epoch_refusal.go internal/app/rungate_epoch_refusal_test.go internal/cli/agent.go internal/cli/agent_test.go
git commit -m "fix(agent): agent.enter preflights --run-gate-key/--run-epoch before launch (change 0463)"
```

---

### Task 6: End-to-end reproduction of the 0382 sequence

This test proves the whole chain. A change is claimed with no armed gate, so no epoch exists. `gate-before --resume` then prints three tokens. The positionally parsed `<epoch>` is admitted by the REAL epoch launch gate and lands on the worktree slot, and the misrouted dispatch context is refused `unknown-run-epoch`. The test admits through `Admit` and never launches, so no supervisor process is spawned.

**Files:**
- Create: `internal/app/rungate_epochless_resume_e2e_test.go`

**Interfaces:**
- Consumes: `RunGateBefore`, `armedGateResult` invariant (Tasks 1–2); `mapDriveFailure`, `ReasonUnknownRunEpoch` (Task 3); `NewTaskGateDriveService`, `GateDriveService.startRequest`, `svc.engine` (`driveEngine`: `Admit`, `AbandonAdmission`); `gatedrive.OpenStore(common).PrepareScope` / `.LoadWorktreeExecution`; test helpers `newWorkingRepo`, `workspaceDepsFor`, `inProgressChangeBlob`, `resumeInspectService`, `mainPin`, `buildEffWithMaxAttempts`, `gateGitCommonDir`.
- Produces: none (a test only).

- [ ] **Step 1: Write the test**

```go
package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/testsupport"
)

// TestEpochlessResumeEndToEnd0382 reproduces change 0382's resumed run (change 0463).
// The change was claimed by an UNARMED first dispatch, so no run epoch exists. The
// resume arm must print `gate-armed <key> <epoch> <dispatch-context>`. Parsed
// positionally (as AGENTS.md tells a parent), the <epoch> is admitted by the real
// epoch launch gate for the resumed worktree and recorded on its execution slot.
// The misrouted 0382 call (the dispatch context presented as the epoch) is refused
// with the named unknown-run-epoch. The resume inspect path uses the raw temp
// spelling and the start uses the symlink-resolved one (Review Focus 1).
func TestEpochlessResumeEndToEnd0382(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	worktree, err := filepath.EvalSymlinks(repoDir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	common, err := gateGitCommonDir(repoDir)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
	}
	if _, _, found, ferr := FindEpochByChange(repoDir, "5"); ferr != nil || found {
		t.Fatalf("the unarmed claim must leave no epoch: found=%v err=%v", found, ferr)
	}

	// Arm the resume through the REAL outer-scope store, as production composes it.
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(5, "epsilon", "v5", "")}}
	deps := workspaceDepsFor(t, reader)
	wdeps := WorkspaceDeps{Service: resumeInspectService(repoDir)}
	store := gatedrive.OpenStore(common)
	arm := RunGateBefore(context.Background(), deps, wdeps, GateScopeDeps{Prepare: store.PrepareScope}, repoDir, "implement-next", 5)
	if !arm.Armed {
		t.Fatalf("the epochless resume must arm: %q", arm.HumanText())
	}

	fields := strings.Fields(strings.SplitN(arm.HumanText(), "\n", 2)[0])
	if len(fields) != 4 || fields[0] != "gate-armed" {
		t.Fatalf("armed line %q must be `gate-armed <key> <epoch> <dispatch-context>`", arm.HumanText())
	}
	key, epoch, dispatchCtx := fields[1], fields[2], fields[3]
	if key != arm.Key || epoch != arm.Epoch || dispatchCtx != arm.DispatchContext {
		t.Fatalf("positional fields (%q,%q,%q) disagree with the result (%q,%q,%q)", key, epoch, dispatchCtx, arm.Key, arm.Epoch, arm.DispatchContext)
	}

	svc, res, reason := NewTaskGateDriveService(common, "/bin/true", buildEffWithMaxAttempts("go test ./...", 4), []string{"/bin/echo", "ok"})
	if svc == nil {
		t.Fatalf("task service: %s (%s)", res, reason)
	}
	req := GateDriveStartRequest{
		RepoDir: common, Worktree: worktree, ChangeID: "5", TaskID: "task-6", Phase: "build",
		RunRoot: testsupport.TempDir(t), Cwd: worktree, GateContext: dispatchCtx, RunEpochID: epoch,
	}

	// The misrouted 0382 call: the dispatch context presented as the run epoch.
	bad := req
	bad.RunEpochID = dispatchCtx
	if _, berr := svc.engine.Admit(svc.startRequest(bad)); berr == nil {
		t.Fatalf("the dispatch context must never admit as a run epoch")
	} else if r, why := mapDriveFailure(berr); r != ResultInvalidInput || why != ReasonUnknownRunEpoch {
		t.Fatalf("misrouted epoch refused as (%s, %q), want (invalid-input, unknown-run-epoch)", r, why)
	}

	// The correctly parsed epoch is admitted by the real launch gate.
	ticket, aerr := svc.engine.Admit(svc.startRequest(req))
	if aerr != nil {
		r, why := mapDriveFailure(aerr)
		t.Fatalf("the parsed epoch must admit for the resumed worktree, got (%s, %q): %v", r, why, aerr)
	}
	t.Cleanup(func() { _ = svc.engine.AbandonAdmission(ticket) })
	slot, _, lerr := store.LoadWorktreeExecution(worktree)
	if lerr != nil {
		t.Fatalf("LoadWorktreeExecution: %v", lerr)
	}
	if slot.RunEpochID != epoch {
		t.Fatalf("worktree slot RunEpochID = %q, want the armed epoch %q", slot.RunEpochID, epoch)
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test -count=1 ./internal/app/ -run TestEpochlessResumeEndToEnd0382 -v`
Expected: PASS on top of Tasks 1–3. If `Admit` refuses for a fixture reason unrelated to the epoch (for example, the fingerprint needs a clean checkout), fix the fixture, never the assertion. The worktree must be a git checkout with a HEAD, which `newWorkingRepo(...).invocation` provides.

- [ ] **Step 3: Mutation-check** (backup-copy restore)

- Reintroduce the original defect (restore `if resumeID == 0` around the Task 1 mint, remove the `armedGateResult` guard, and restore the `HumanText` epoch conditional): the test reddens on the 4-field assertion.
- Drop the resume `rec.Worktree = worktree` bind: the good `Admit` reddens (`stale-run-epoch`).
- Delete the `ClassifyRunEpochError` block from `mapDriveFailure`: the misrouted leg reddens (`invalid-request`).

- [ ] **Step 4: Commit**

```bash
git add internal/app/rungate_epochless_resume_e2e_test.go
git commit -m "test(rungate): end-to-end reproduction of the 0382 epochless resume (change 0463)"
```

---

## Self-Review

- **Spec coverage.** Decision 1 / Design §1: Task 1. Design §2 (HumanText, stale comments, CLI `Short`, prose grep): Tasks 1–2. Design §3 (`mapDriveFailure` `AsEpochError` case, the `unknown-run-epoch` token, next-action message, prepare-scope, agent.enter trace, token registration): Tasks 3–5. The token-registration grep over non-archival prose and skills found no enumerated gate-drive reason vocabulary outside Go (skills mention only `suite-attempts-exhausted`), so no prose registry edit is needed. Tests 1, 3, 7: Task 1. Test 2: Task 2. Test 4: Tasks 3–4. Test 5: Task 5. Test 6: Task 6. Decision 4 (race fail-safe): the Task 1 documenting test. Out-of-scope items are untouched.
- **Deliberate deviations, recorded.**
  - The resume epoch is minted unbound, and ChangeID+Worktree are bound in one `epochCAS`, rather than minted with ChangeID. The end state matches the spec, and a failed bind can no longer leave an orphan that names the change and blocks later resumes.
  - `agent.enter` maps a mismatch to the existing stale-linkage token `stale-run-epoch`, and maps an absent gate key to `unknown-run-epoch`, since the pair names no epoch.
  - prepare-scope checks resolvability only, keeping the existing cancelled-epoch prepare green.
- **Type consistency.** `armedGateResult(key, epochID, dispatchContext string)`, `ClassifyRunEpochError(err) (Result, string, bool)`, `RunEpochNextAction(reason string) string`, `ReasonUnknownRunEpoch`, `runEpochLocator(gitCommonDir string) func(string) error`, `GateDriveService.epochLocate`, `GateScopeResult.Message`, and `CheckRunEpochLinkage(repoDir, gateKey, epochID string) error` are used identically across Tasks 1–6.
