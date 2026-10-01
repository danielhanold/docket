<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0481 — Split the overloaded gate-drive halt tokens left by 0469](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-01-0481-split-the-overloaded-gate-drive-halt-tokens-left-by-0469.md)**
<!-- docket:backlink:end -->
# Split the overloaded gate-drive halt tokens left by 0469 — Implementation Plan

> **For agentic workers:** This plan is executed by docket's build role (`docket-build`), task by task under the `docket-build-task` contract. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the two conditions that 0469's renamed halt tokens mis-cover their own accurate tokens: takeover scope drift halts `scope-identity-mismatch`, and a lost drive-to-run link halts the new `run-link-lost`.

**Architecture:** Token-only change in `internal/gatedrive`. The takeover `scopeIdentityMatch` failure reuses the existing ownership kind `ErrScopeIdentityMismatch` as its halt cause (the same pattern `takeover.go` already uses for `ErrHandoffOutstanding` and `ErrFingerprintMismatch`). A new exported constant `CauseRunLinkLost` is returned by both worktree-slot branches of `resolveDriveRun`. Tests, the run-verdict pass-through fixture, and the glossary follow. When the drive halts and how it recovers do not change.

**Tech Stack:** Go (`go test`), Markdown glossary.

**Spec:** `docs/superpowers/specs/2026-10-01-split-the-overloaded-gate-drive-halt-tokens-left-by-0469-design.md` (on the `docket` branch; the synchronized copy is at `.docket/docs/superpowers/specs/…` in the primary checkout).

## Global Constraints

- Token-only change: never alter when the gate drive halts, what it launches, or how it recovers.
- Takeover scope drift halts with `string(ErrScopeIdentityMismatch)` (`"scope-identity-mismatch"`). No new vocabulary for it.
- A lost run link halts with exactly one new token: `CauseRunLinkLost = "run-link-lost"`, declared beside the other `Cause*` constants in `internal/gatedrive/drive.go`.
- Leave unchanged: the two correct `"worktree-changed"` sites in `driver.go` (pass-path fingerprint revalidation, `relaunchRefusal`), the two correct `"launch-unconfirmed"` sites (relaunch `Launch` error with an unresolved reservation, `haltReservedRelaunch` default), the `ErrLaunchUnconfirmed` ownership refusal kind, all skill prose, `mapDriveHaltCause` in `internal/app/finalize_rebase.go`, and the scoped branch of `resolveDriveRun` (`CauseRunRecordUnreadable`).
- Leave unchanged as historical data: `internal/repoguard/retired_vocabulary_test.go` rows 68/69 and their fixtures, the `admission_test.go` history-cause lists, `drive_test.go`'s `Cause: "worktree-changed"` serialization sample, and every archived plan/results file.
- The ADR-0129 dated `## Update` is NOT a feature-branch task (the parent records it on the `docket` branch). Do not edit any ADR file.
- Cross-references in maintained source anchor on symbol names or quoted clauses, never line numbers (AGENTS.md, ADR-0054).
- Every focused `go test` run uses `-count=1` (the Go result cache can serve a stale PASS for a mutated tree). Every mutation probe backs the file up to a scratch copy (`cp`), never restores with `git checkout --` (that restores to HEAD and destroys uncommitted work).
- Stage explicit paths only; never `git add -A`.
- The whole suite runs once at docket-build's gate; no task adds a separate full-suite step.

## Review Focus

1. **The four correct sites must keep their tokens.** Fingerprint drift still halts `worktree-changed`; an unresolved relaunch reservation still halts `launch-unconfirmed`. Pinned by the existing, unmodified `driver_test.go` cases (`wantCause: "worktree-changed"`, the "unresolved replacement halts closed" row, `TestRelaunchReservationNotRefundedOnUncertainty`). Each task's verify step runs them.
2. **The takeover fingerprint case must still halt `ErrFingerprintMismatch`**, not `scope-identity-mismatch`. The "fingerprint drift" row of `TestTakeoverFailClosedTable` already pins this; Task 1 runs the whole table.
3. **An absent slot is a lost link, not a refusal to read.** A scopeless drive with an `AdmissionToken` whose worktree has no slot at all must halt `run-link-lost` without launching and without consulting the run launch gate. Task 2 adds this case.
4. **The scoped branch of `resolveDriveRun` keeps `run-record-unreadable`.** `takeover_run_test.go` and `reconcile_test.go` already pin it; Task 2's verify step runs the package.
5. **Finalize classifies both tokens as unavailable.** Task 2 adds `CauseRunLinkLost` and `ErrScopeIdentityMismatch` rows to `TestMapDriveHaltCauseKeysOnGatedriveConstants`, so a later special case in `mapDriveHaltCause` is a visible decision.

## Site inventory (derived from a whole-repo grep, sorted)

`grep -rn -e 'worktree-changed' -e 'launch-unconfirmed' --exclude-dir=.git .` at base `85bace7`:

- **Executable sites this plan changes:** `internal/gatedrive/takeover.go` (the `!scopeIdentityMatch(...)` halt), `internal/gatedrive/driver.go` `resolveDriveRun` (two returns).
- **Executable sites left alone (correct):** `driver.go` `halt(&res, "worktree-changed")` on the pass path, `relaunchRefusal`'s `return "worktree-changed"`, `halt(&res, "launch-unconfirmed")` after `ResolveReservation`, `haltReservedRelaunch`; `ownership.go` `ErrLaunchUnconfirmed`.
- **Tests this plan changes:** `takeover_test.go` (five "identity mismatch" rows), `driver_runfence_test.go` `TestRelaunchLostLinkageRefuses`, `internal/app/runtracker_verdict_integration_test.go` `TestIntegrationRunVerdictVerdictTakeoverHaltStops`, `internal/app/gate_drive_test.go` `TestMapDriveHaltCauseKeysOnGatedriveConstants` (rows added).
- **Prose this plan changes:** `docs/reference/glossary.md` (*Worktree changed / certified input changed* entry; new `run-link-lost` entry and index line).
- **Prose/data left alone:** skills and their embedded copies (they describe the admission refusal), comments quoting `worktree-busy / launch-unconfirmed`, repoguard retired-vocabulary fixtures, `admission_test.go` history lists, archived plans/results.

Re-run the grep at the start of Task 1. If it finds a site not listed above, classify it before editing and record it in the task report's notes.

---

### Task 1: Takeover scope drift halts `scope-identity-mismatch`

**Files:**
- Modify: `internal/gatedrive/takeover.go` (the halt after `if !scopeIdentityMatch(scope, rec.RepoIdentity, ...)` in `Takeover`)
- Modify: `internal/gatedrive/ownership.go` (doc comment on `ErrScopeIdentityMismatch`)
- Modify: `internal/gatedrive/takeover_test.go` (`TestTakeoverFailClosedTable`, the five rows named `identity mismatch branch|worktree|change|task|phase`)
- Modify: `internal/app/runtracker_verdict_integration_test.go` (`TestIntegrationRunVerdictVerdictTakeoverHaltStops`)
- Modify: `docs/reference/glossary.md` (`### Worktree changed / certified input changed`)

**Interfaces:**
- Consumes: `ErrScopeIdentityMismatch OwnershipErrorKind = "scope-identity-mismatch"` (exists in `ownership.go`).
- Produces: the takeover HALTED doc for scope drift carries `Cause == string(ErrScopeIdentityMismatch)`.

- [ ] **Step 1: Re-derive the site list.** Run `grep -rn -e 'worktree-changed' -e 'launch-unconfirmed' --exclude-dir=.git /Users/homer/dev/docket/.worktrees/split-the-overloaded-gate-drive-halt-tokens-left-by-0469` and compare it to the *Site inventory* above. Note any difference in the task report.

- [ ] **Step 2: Write the failing tests.** In `internal/gatedrive/takeover_test.go`, `TestTakeoverFailClosedTable`, change `want:` in each of the five rows named `"identity mismatch branch"`, `"identity mismatch worktree"`, `"identity mismatch change"`, `"identity mismatch task"`, `"identity mismatch phase"` from:

```go
			want: "worktree-changed",
```

to:

```go
			want: string(ErrScopeIdentityMismatch),
```

Do not touch the `"fingerprint drift"` row (it stays `string(ErrFingerprintMismatch)`).

In `internal/app/runtracker_verdict_integration_test.go`, `TestIntegrationRunVerdictVerdictTakeoverHaltStops`, change the fake takeover cause and its assertion so the pass-through fixture uses a value the driver can actually produce:

```go
	wdeps.Continuation = &fakeContinuationSeam{
		candidates:    []string{"d0opaque"},
		takeoverHalt:  true,
		takeoverCause: "scope-identity-mismatch",
	}
```

```go
	if res.Reason != "scope-identity-mismatch" {
		t.Errorf("reason = %q, want scope-identity-mismatch (driver cause passed through)", res.Reason)
	}
```

(This test uses a fake seam, so it passes as soon as it is edited; it is a fixture realignment, not a guard on `takeover.go`.)

- [ ] **Step 3: Run the takeover table to verify it fails.**

Run: `go test ./internal/gatedrive -run 'TestTakeoverFailClosedTable' -count=1`
Expected: FAIL in the five `identity mismatch …` subtests, each reporting cause `worktree-changed` where `scope-identity-mismatch` was wanted. Every other subtest passes.

- [ ] **Step 4: Change the emission site.** In `internal/gatedrive/takeover.go`, replace:

```go
	if !scopeIdentityMatch(scope, rec.RepoIdentity, rec.Branch, rec.WorktreePath, rec.ChangeID, rec.TaskID, rec.Phase) {
		return d.haltDoc(resolvedID, "", rec, "worktree-changed"), nil
	}
```

with:

```go
	if !scopeIdentityMatch(scope, rec.RepoIdentity, rec.Branch, rec.WorktreePath, rec.ChangeID, rec.TaskID, rec.Phase) {
		return d.haltDoc(resolvedID, "", rec, string(ErrScopeIdentityMismatch)), nil
	}
```

Keep the comment above it; it already describes this as an identity drift, not a worktree change.

- [ ] **Step 5: Broaden the kind's doc comment.** In `internal/gatedrive/ownership.go`, replace the `ErrScopeIdentityMismatch` comment:

```go
	// ErrScopeIdentityMismatch: a scope's identity (its bound change, or an
	// identity field a takeover re-verifies) no longer matches what the caller
	// presented — e.g. rebinding a scope to a different change. Fail closed.
```

with:

```go
	// ErrScopeIdentityMismatch: a scope's identity (its bound change, or an
	// identity field a takeover re-verifies) no longer matches what the caller
	// presented — e.g. rebinding a scope to a different change. Fail closed.
	// Takeover also emits it as a HALTED cause when the resolved drive's recorded
	// repo, branch, worktree, change, task, or phase is not the scope's: the
	// pairing drifted, while the worktree itself may be untouched (change 0481).
```

- [ ] **Step 6: Run the tests to verify they pass.**

Run: `go test ./internal/gatedrive -run 'TestTakeover' -count=1`
Expected: PASS (including the `"fingerprint drift"` row, still `ErrFingerprintMismatch`).

Run: `go test -tags integration ./internal/app -run 'TestIntegrationRunVerdictVerdictTakeoverHaltStops' -count=1`
Expected: PASS.

- [ ] **Step 7: Mutation-test the guard.** Back up and revert the site, confirm the table reddens, restore from the backup:

```bash
WT=/Users/homer/dev/docket/.worktrees/split-the-overloaded-gate-drive-halt-tokens-left-by-0469
BK="$(mktemp "${TMPDIR:-/tmp}/takeover-go.XXXXXX")"
cp "$WT/internal/gatedrive/takeover.go" "$BK"
# Hand-edit takeover.go: put "worktree-changed" back in the scopeIdentityMatch halt only.
go -C "$WT" test ./internal/gatedrive -run 'TestTakeoverFailClosedTable' -count=1   # expect FAIL in all five identity-mismatch rows
cp "$BK" "$WT/internal/gatedrive/takeover.go"
go -C "$WT" test ./internal/gatedrive -run 'TestTakeoverFailClosedTable' -count=1   # expect PASS
```

Before trusting the FAIL reading, confirm with `git -C "$WT" diff internal/gatedrive/takeover.go` that the mutation actually landed. Record both readings in the task report.

- [ ] **Step 8: Update the glossary entry.** In `docs/reference/glossary.md`, `### Worktree changed / certified input changed`, replace:

```markdown
The state a gate checked no longer matches the state now in front of it. A gate drive halts
`worktree-changed` when the worktree fingerprint (HEAD, index, status, live file bytes) moved
since the drive started, or when a takeover finds a drive whose recorded branch, worktree or
change is not the scope's. `evidence.recertify` refuses `certified-input-changed` when the PR
head or the build command moved after the gate passed.
```

with:

```markdown
The state a gate checked no longer matches the state now in front of it. A gate drive halts
`worktree-changed` when the worktree fingerprint (HEAD, index, status, live file bytes) moved
since the drive started. A takeover that finds a drive whose recorded branch, worktree, change,
task or phase is not the scope's halts `scope-identity-mismatch` instead, because nothing in the
worktree changed. `evidence.recertify` refuses `certified-input-changed` when the PR
head or the build command moved after the gate passed.
```

Leave the rest of the entry (the `pr-head-mismatch` sentence and the **Used for:** paragraph) unchanged.

- [ ] **Step 9: Commit.**

```bash
WT=/Users/homer/dev/docket/.worktrees/split-the-overloaded-gate-drive-halt-tokens-left-by-0469
git -C "$WT" add internal/gatedrive/takeover.go internal/gatedrive/ownership.go internal/gatedrive/takeover_test.go internal/app/runtracker_verdict_integration_test.go docs/reference/glossary.md && git -C "$WT" commit -m "refactor(gatedrive): halt takeover scope drift with scope-identity-mismatch (change 0481)"
```

---

### Task 2: A lost run link halts `run-link-lost`

**Files:**
- Modify: `internal/gatedrive/drive.go` (the `const (...)` block of `Cause*` tokens)
- Modify: `internal/gatedrive/driver.go` (`resolveDriveRun`: doc comment and the two worktree-slot returns)
- Modify: `internal/gatedrive/driver_runfence_test.go` (`TestRelaunchLostLinkageRefuses`)
- Modify: `internal/app/gate_drive_test.go` (`TestMapDriveHaltCauseKeysOnGatedriveConstants`)
- Modify: `docs/reference/glossary.md` (new `run-link-lost` entry in *Supervised gate runs*; index line)

**Interfaces:**
- Consumes: Task 1 leaves `ErrScopeIdentityMismatch` as a takeover halt cause (used only in the `mapDriveHaltCause` row below).
- Produces: `gatedrive.CauseRunLinkLost = "run-link-lost"` (untyped string constant, same block as `CauseRunRecordUnreadable`). `resolveDriveRun(rec driveRecord) (runID string, ok bool, cause string)` keeps its signature and returns `("", false, CauseRunLinkLost)` from both slot branches.

- [ ] **Step 1: Write the failing tests.** In `internal/gatedrive/driver_runfence_test.go`, replace the whole `TestRelaunchLostLinkageRefuses` function (doc comment included) with a two-case table that covers both slot branches of `resolveDriveRun`:

```go
// TestRelaunchLostLinkageRefuses proves a drive whose run linkage is LOST never
// demotes to a standalone relaunch: a scopeless drive with an AdmissionToken
// whose worktree slot is absent, or now carries a DIFFERENT reservation token,
// can no longer prove which run it belongs to, so its death-relaunch leg HALTs
// CauseRunLinkLost without launching and without consulting the run launch gate.
// One case per resolveDriveRun slot branch (load error, token mismatch). (change 0481)
func TestRelaunchLostLinkageRefuses(t *testing.T) {
	for _, tc := range []struct {
		name        string
		reserveSlot bool // false: the worktree has no slot at all (load error)
	}{
		{name: "slot reassigned to another reservation", reserveSlot: true},
		{name: "slot absent", reserveSlot: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := OpenStore(testsupport.TempDir(t))
			wt := sampleWorktree()
			proc := &fakeProc{
				observe: func(runDir string) (*process.Observation, error) {
					return obs(process.StateSignaled, runDir), nil
				},
			}
			slotToken := ""
			if tc.reserveSlot {
				// Mint a live worktree slot with its own reservation token.
				tok, _, rerr := store.reserveWorktreeExecution(admissionRecord{
					RepoIdentity: "/repo",
					WorktreeRoot: wt,
					Kind:         "scopeless",
				}, proc)
				if rerr != nil {
					t.Fatalf("reserve worktree slot: %v", rerr)
				}
				slotToken = tok
			} else if _, _, lerr := store.LoadWorktreeExecution(wt); lerr == nil {
				t.Fatal("precondition: the absent-slot case must have no slot to load")
			}

			rec := seedRecord(t)
			rec.WorktreePath = wt
			rec.AdmissionToken = "stale-admission-token" // NOT any slot's current token
			if rec.AdmissionToken == slotToken {
				t.Fatal("the drive's stale token must differ from the slot's live token")
			}
			id, ownerGen := seedDrive(t, store, rec)

			d := NewDriver(reopenStore(store), &fakeClock{now: startRun().Add(time.Second)}, proc, stableGit())
			d.slice = pollTick
			d.pollInterval = pollTick
			d.sleep = func(dur time.Duration) {}
			// A gate that fails the test if consulted: lost linkage must refuse BEFORE the gate.
			d.SetRunLaunchGate(func(_, _ string, _ func() error) error {
				t.Fatalf("lost linkage must refuse before the run launch gate is consulted")
				return nil
			})

			doc, err := d.Advance(id, ownerGen)
			if err != nil {
				t.Fatalf("Advance: %v", err)
			}
			if doc.Outcome != HALTED || doc.Cause != CauseRunLinkLost {
				t.Fatalf("lost linkage = %s/%q, want HALTED/%s", doc.Outcome, doc.Cause, CauseRunLinkLost)
			}
			if proc.launchN != 0 {
				t.Fatalf("lost linkage must launch nothing, proc.Launch called %d times", proc.launchN)
			}
		})
	}
}
```

In `internal/app/gate_drive_test.go`, `TestMapDriveHaltCauseKeysOnGatedriveConstants`, add two rows to `cases` just above the `"owner-superseded"` row:

```go
		// Change 0481's tokens are deliberately not distinguished: finalize reads
		// either as an unavailable gate.
		gatedrive.CauseRunLinkLost:                        GateHaltUnavailable,
		string(gatedrive.ErrScopeIdentityMismatch):        GateHaltUnavailable,
```

(`gofmt` will realign the map; run it.)

- [ ] **Step 2: Run the tests to verify they fail.**

Run: `go test ./internal/gatedrive -run 'TestRelaunchLostLinkageRefuses' -count=1`
Expected: build FAIL, `undefined: CauseRunLinkLost`.

- [ ] **Step 3: Declare the constant.** In `internal/gatedrive/drive.go`, inside the `const (...)` block, after `CauseRunRecordUnreadable`, add:

```go
	// CauseRunLinkLost: a scopeless run-linked drive can no longer prove which run
	// it belongs to — the worktree slot it was admitted through is absent or
	// unreadable, or now carries a different reservation token. resolveDriveRun
	// returns it for both slot branches, so the relaunch path refuses new
	// execution rather than demoting the drive to standalone. Distinct from the
	// launch-unconfirmed halt, which means a launch itself is in doubt. (change 0481)
	CauseRunLinkLost = "run-link-lost"
```

- [ ] **Step 4: Confirm the new test reaches the right branch before the fix.** With only the constant added (the `resolveDriveRun` returns still say `"launch-unconfirmed"`):

Run: `go test ./internal/gatedrive -run 'TestRelaunchLostLinkageRefuses' -count=1 -v`
Expected: FAIL in both subtests with `lost linkage = HALTED/"launch-unconfirmed", want HALTED/run-link-lost`. This proves both cases reach `resolveDriveRun`'s slot branches (a different cause, a launch, or a gate consult would point at a different path; if so, stop and investigate rather than editing the test to pass).

- [ ] **Step 5: Change the emission sites.** In `internal/gatedrive/driver.go`, `resolveDriveRun`, replace:

```go
	slot, _, err := d.store.LoadWorktreeExecution(rec.WorktreePath)
	if err != nil {
		return "", false, "launch-unconfirmed"
	}
	if slot.ReservationToken != rec.AdmissionToken {
		return "", false, "launch-unconfirmed"
	}
	return slot.RunID, true, ""
```

with:

```go
	slot, _, err := d.store.LoadWorktreeExecution(rec.WorktreePath)
	if err != nil {
		return "", false, CauseRunLinkLost
	}
	if slot.ReservationToken != rec.AdmissionToken {
		return "", false, CauseRunLinkLost
	}
	return slot.RunID, true, ""
```

In the same function's doc comment, replace the sentence:

```go
// match). ok=false with cause set means the linkage is LOST or inconsistent — the
// drive can no longer prove whether it is run-backed, so new execution is
// refused (never demoted to standalone).
```

with:

```go
// match). ok=false with cause set means the linkage is LOST or inconsistent — the
// drive can no longer prove whether it is run-backed, so new execution is
// refused (never demoted to standalone). A scoped drive whose scope cannot be read
// reports CauseRunRecordUnreadable; an absent, unreadable, or reassigned worktree
// slot reports CauseRunLinkLost.
```

Do not touch `haltReservedRelaunch`, `haltReservedRelaunchCause`, or the `halt(&res, "launch-unconfirmed")` after `ResolveReservation`.

- [ ] **Step 6: Run the tests to verify they pass.**

Run: `go test ./internal/gatedrive -count=1`
Expected: PASS for the whole package, including the unchanged `launch-unconfirmed` cases in `driver_test.go` ("unresolved replacement halts closed", `TestRelaunchReservationNotRefundedOnUncertainty`), the `worktree-changed` fingerprint case, and the `CauseRunRecordUnreadable` cases in `takeover_run_test.go` / `reconcile_test.go`.

Run: `go test ./internal/app -run 'TestMapDriveHaltCauseKeysOnGatedriveConstants' -count=1`
Expected: PASS.

- [ ] **Step 7: Mutation-test each site separately.** Each branch has its own subtest, so reverting one return must redden only its case:

```bash
WT=/Users/homer/dev/docket/.worktrees/split-the-overloaded-gate-drive-halt-tokens-left-by-0469
BK="$(mktemp "${TMPDIR:-/tmp}/driver-go.XXXXXX")"
cp "$WT/internal/gatedrive/driver.go" "$BK"
# (a) Hand-edit: the LoadWorktreeExecution-error return back to "launch-unconfirmed".
git -C "$WT" diff --stat internal/gatedrive/driver.go                                     # confirm the mutation landed
go -C "$WT" test ./internal/gatedrive -run 'TestRelaunchLostLinkageRefuses' -count=1 -v  # expect FAIL in "slot absent" only
cp "$BK" "$WT/internal/gatedrive/driver.go"
# (b) Hand-edit: the ReservationToken-mismatch return back to "launch-unconfirmed".
git -C "$WT" diff --stat internal/gatedrive/driver.go
go -C "$WT" test ./internal/gatedrive -run 'TestRelaunchLostLinkageRefuses' -count=1 -v  # expect FAIL in "slot reassigned to another reservation" only
cp "$BK" "$WT/internal/gatedrive/driver.go"
go -C "$WT" test ./internal/gatedrive -run 'TestRelaunchLostLinkageRefuses' -count=1     # expect PASS
```

If mutation (a) leaves "slot absent" green, the case is not reaching the load-error branch: investigate and fix the test, never record it as a limitation. Record all readings in the task report.

- [ ] **Step 8: Add the glossary entry and index line.** In `docs/reference/glossary.md`, in the `## Supervised gate runs` section, insert this entry directly after the `### \`worktree-busy\` / \`launch-unconfirmed\`` entry (after its **Used for:** paragraph, before the section's closing `---`), with one blank line on each side:

```markdown
### `run-link-lost`

A gate drive halts `run-link-lost` when it can no longer prove which run it belongs to: the
[worktree slot](#worktree-slot) it was admitted through is absent or unreadable, or now holds a
different reservation. Rather than relaunch as a standalone gate, the drive refuses.

**Used for:** telling an orphaned drive apart from `launch-unconfirmed`, where a launch itself is
in doubt. It is a halt, never a red suite. Cancel the run with `run.cancel` and start fresh.
```

In the `## Alphabetical index`, insert this line between `- [Run fence](#run-fence)` and `- [Run tracker](#run-tracker)`:

```markdown
- [run-link-lost](#run-link-lost)
```

- [ ] **Step 9: Format and commit.**

```bash
WT=/Users/homer/dev/docket/.worktrees/split-the-overloaded-gate-drive-halt-tokens-left-by-0469
gofmt -l "$WT/internal/gatedrive" "$WT/internal/app"   # expect no output; if any, gofmt -w those files
git -C "$WT" add internal/gatedrive/drive.go internal/gatedrive/driver.go internal/gatedrive/driver_runfence_test.go internal/app/gate_drive_test.go docs/reference/glossary.md && git -C "$WT" commit -m "refactor(gatedrive): halt a lost drive-to-run link with run-link-lost (change 0481)"
```
