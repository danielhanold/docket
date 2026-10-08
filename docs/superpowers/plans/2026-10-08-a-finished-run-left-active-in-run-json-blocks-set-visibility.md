<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0540 — A finished run left active in run.json blocks set-visibility and cannot be cancelled](../../changes/active/0540-a-finished-run-left-active-in-run-json-blocks-set-visibility.md)**
<!-- docket:backlink:end -->

# A keyed run-done verdict retires its run: implementation plan

> **For agentic workers:** this plan is executed by `docket-build`, one task at a time, under the
> `docket-build-task` contract. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** every keyed `run-done` verdict (`run-complete`, `no-attributable-claim`, `run-unclaimed`)
retires its run (`run.json` ends `completed`), and `set-visibility`'s live-run list names a remedy
that will actually work for each run.

**Architecture:** reuse the existing observation-only closeout (`runTrackerCompleteRun` →
`completeSuccessfulRun`) for all three `run-done` outcomes by passing the outcome token through,
instead of routing two of them to the record-only `persistRunVerdict`. Extract `run cancel`'s
ownership proof into one pure function, `runCancelOwner`, that both `runCancel` and `liveRunsUnder`
call, so the remedy text and cancel's real decision cannot drift.

**Tech Stack:** Go (`internal/app`), Go test with the `integration` build tag for tests that start
real git; the bash suite runner (`go run ./cmd/docket development test`).

**Spec:** `docs/superpowers/specs/2026-10-08-a-finished-run-left-active-in-run-json-blocks-set-visibility-design.md`
(on the `docket` metadata branch; it travels with this plan).

## Global Constraints

- No new run state, no run-tracker schema change, no new record field, no change to the run lock or generation scheme.
- Report lines are unchanged on success: `run-done <key> no-attributable-claim`, `run-done <key> run-unclaimed <id>`, `run-done <key> run-complete <id>`.
- A blocked closeout reports `run-stop <key> run-tracker-unavailable completion-unaccounted` (or the engine's other bounded reason tokens), leaves the run `completing`, and consumes no retry. Repeating the same keyed verdict replays it.
- Halted, stop, retry, and continue verdicts keep today's behavior: `run.json` stays `active`.
- A `cancelling`, `cancelled`, or `superseded` run is never marked `completed`.
- Unattributed observe mode (`run verdict --unattributed`) stays read-only and never reaches the closeout.
- No run beside the record (keyless or legacy): mirror the report line and write nothing else, exactly as today.
- `run cancel`'s ownership proofs (ADR-0128) do not change; this change only extracts them.
- Every new assert must go red when the code it guards is removed. Mutation probes use `go test -count=1` (the Go test cache can serve a stale pass).
- Untagged test files in `internal/app` must never start real git (`nogit_guard_test.go`); any test that calls `RunVerdict`, `newRunTrackerRepo`, or another git-backed helper goes in an `//go:build integration` file.
- Cross-references in comments anchor on a symbol name, never a line number.
- No ADR task: the coordinator records the ADR that extends ADR-0124 Rule 1 at review.

## Review Focus

1. **A run whose `record.json` is missing or unreadable.** Neither `run cancel` nor `run verdict` can load it, so naming either command would print a remedy that fails as run. Expect the by-hand remedy (Task 2 pins it).
2. **A run whose claim binding is corrupt.** Cancel refuses `claim-unreadable` and the verdict stops `binding-unreadable` without retiring the run, so `docket run verdict <key>` would be a false remedy. Expect the by-hand remedy (Task 2 pins it).
3. **A run with a confirmed binding but no parent capability** (`authority-unavailable`). Cancel refuses, but the verdict does not need the capability and will retire the run. Expect `docket run verdict <key>` (Task 2 pins it).
4. **A verdict on a stranded run that already holds `terminal: true`** (the exact incident machines carry today). Re-running the keyed verdict must close it out, not short-circuit on the stored disposition (Task 3's incident test starts from that state).
5. **An explicitly cancelled run whose change went back to `proposed`.** It used to report `run-done … run-unclaimed`. It now reports `run-stop … run-tracker-unavailable run-cancelled` and stays cancelled, per the spec's never-relabel rule. Both lines forbid re-dispatch (Task 3's never-relabel test pins the state).

---

## File map

| File | Change |
|---|---|
| `internal/app/runtracker_cancel.go` | Add `runCancelOwner`; `runCancel` calls it. |
| `internal/app/runtracker_cancel_owner_test.go` (new, untagged) | Table test for `runCancelOwner`. |
| `internal/app/runtracker_store.go` | Add `decodeRunTrackerRecord`; `LoadRunTrackerRecord` calls it. |
| `internal/app/runtracker_live.go` | Remedy chosen by `liveRunCancelAuthority`; add `runVerdictCommand`. |
| `internal/app/runtracker_live_test.go` (untagged) | Fixture writers for `record.json`/`claim-binding.json`; remedy-matrix tests. |
| `internal/app/runtracker_verdict.go` | `runTrackerCompleteRun` takes the outcome; `run-unclaimed` and `no-attributable-claim` route through it; header comments. |
| `internal/app/runtracker_complete.go` | Header comment: driven by every keyed `run-done`. |
| `internal/app/runtracker_verdict_integration_test.go` (integration tag) | No-claim fixture and the regression tests. |
| `docs/concepts/run-tracker.md` | One sentence on the verdict retiring the run. |

---

### Task 1: Extract run cancel's ownership proof into `runCancelOwner`

Behavior-neutral refactor. `runCancel` today checks, in order: `ParentCap` empty → `authority-unavailable`; binding load error → `claim-unreadable`; confirmed binding → owner is the binding's change; no binding file and `rec.resumeAttributed()` → owner is `rec.AttributedID`; otherwise `claim-unconfirmed`; then the owner must match the run's `ChangeID` when that is set, else `claim-mismatch`. Move that whole decision (all five refusals, not just the binding switch) into one pure function so Task 2's live-run scan asks exactly the question cancel asks.

**Files:**
- Modify: `internal/app/runtracker_cancel.go` (the step-(2) block of `runCancel`, from `if rec.ParentCap == ""` through the `claim-mismatch` check)
- Create: `internal/app/runtracker_cancel_owner_test.go`

**Interfaces:**
- Produces: `func runCancelOwner(rec RunTrackerRecord, binding RunTrackerClaimBinding, hasBinding bool, bindingErr error, runChangeID string) (ownerID int, refusal string)`. `refusal == ""` means cancel would accept; otherwise it is the exact token `runCancel` reports (`authority-unavailable`, `claim-unreadable`, `claim-unconfirmed`, `claim-mismatch`).

- [ ] **Step 1: Write the failing test**

Create `internal/app/runtracker_cancel_owner_test.go` (no build tag; it starts no git):

```go
package app

import (
	"errors"
	"testing"
)

// TestRunCancelOwnerMatchesCancelProofs pins run.cancel's ownership proof
// (ADR-0128 Decision 1) as one pure decision, shared by runCancel and the
// live-run scan (liveRunCancelAuthority) so the two cannot drift (change 0540).
func TestRunCancelOwnerMatchesCancelProofs(t *testing.T) {
	confirmed := RunTrackerClaimBinding{Schema: bindingSchemaVersion, ChangeID: 7, RequestID: "r", Confirmed: true}
	reserved := RunTrackerClaimBinding{Schema: bindingSchemaVersion, ChangeID: 7, RequestID: "r"}
	withCap := RunTrackerRecord{ParentCap: "cap"}
	resume := RunTrackerRecord{ParentCap: "cap", AttributedID: 7}
	none := RunTrackerClaimBinding{}
	cases := []struct {
		name        string
		rec         RunTrackerRecord
		binding     RunTrackerClaimBinding
		has         bool
		berr        error
		runChange   string
		wantID      int
		wantRefusal string
	}{
		{"confirmed binding", withCap, confirmed, true, nil, "7", 7, ""},
		{"confirmed binding, run not yet bound", withCap, confirmed, true, nil, "", 7, ""},
		{"resume-verified record", resume, none, false, nil, "7", 7, ""},
		{"no parent capability", RunTrackerRecord{}, confirmed, true, nil, "7", 0, "authority-unavailable"},
		{"capability checked before binding", RunTrackerRecord{}, none, false, errors.New("corrupt"), "7", 0, "authority-unavailable"},
		{"unreadable binding", withCap, none, false, errors.New("corrupt"), "7", 0, "claim-unreadable"},
		{"unconfirmed reservation", withCap, reserved, true, nil, "7", 0, "claim-unconfirmed"},
		{"reservation beside a resume shape", resume, reserved, true, nil, "7", 0, "claim-unconfirmed"},
		{"no binding and no claim", withCap, none, false, nil, "", 0, "claim-unconfirmed"},
		{"owner differs from the run", withCap, confirmed, true, nil, "8", 0, "claim-mismatch"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, refusal := runCancelOwner(c.rec, c.binding, c.has, c.berr, c.runChange)
			if id != c.wantID || refusal != c.wantRefusal {
				t.Fatalf("runCancelOwner = (%d, %q), want (%d, %q)", id, refusal, c.wantID, c.wantRefusal)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd <feature worktree> && go test -count=1 -run '^TestRunCancelOwnerMatchesCancelProofs$' ./internal/app/`
Expected: FAIL to compile, `undefined: runCancelOwner`.

- [ ] **Step 3: Implement `runCancelOwner` and call it from `runCancel`**

Add to `internal/app/runtracker_cancel.go`, directly after `runCancel`:

```go
// runCancelOwner is run.cancel's ownership proof (ADR-0128 Decision 1) as one
// pure decision: the record must carry a parent-held authority, the claim binding
// must be readable, and the run must be owned by a CONFIRMED claim binding or by
// the resume-verified shape (no binding file at all and rec.resumeAttributed()).
// When the run already names its change, the owner must be that change. It returns
// the owning change id, or the exact refusal token runCancel reports. runCancel
// and the repository-wide live-run scan (liveRunCancelAuthority) both call it, so
// a printed `docket run cancel` remedy names cancel only when cancel would accept
// (change 0540).
func runCancelOwner(rec RunTrackerRecord, binding RunTrackerClaimBinding, hasBinding bool, bindingErr error, runChangeID string) (ownerID int, refusal string) {
	if rec.ParentCap == "" {
		return 0, "authority-unavailable"
	}
	if bindingErr != nil {
		return 0, "claim-unreadable"
	}
	switch {
	case hasBinding && binding.Confirmed:
		ownerID = binding.ChangeID
	case !hasBinding && rec.resumeAttributed():
		// Resume-verified authority (change 0463): `run start --resume` pre-binds
		// AttributedID through WorkspaceInspect identity and never gets a claim binding
		// (change.claim requires a proposed change). It is the same shape
		// resolveRunTrackerOwnership accepts as ownership. Only a record with NO binding
		// file qualifies: a reservation that exists but is unconfirmed still refuses.
		ownerID = rec.AttributedID
	default:
		return 0, "claim-unconfirmed"
	}
	if runChangeID != "" && strconv.Itoa(ownerID) != runChangeID {
		return 0, "claim-mismatch"
	}
	return ownerID, ""
}
```

In `runCancel`, replace the step-(2) block (the `ParentCap` check, the `LoadRunTrackerClaimBinding` call, the `ownerID` switch, and the `claim-mismatch` check) with:

```go
	// (2) Validate the remaining authority conditions through the shared ownership
	// proof (runCancelOwner): a parent-held authority, and a CONFIRMED claim binding
	// for the run's change or the resume-verified proof (ADR-0128 Decision 1).
	binding, hasBinding, berr := LoadRunTrackerClaimBinding(repoDir, key)
	if _, refusal := runCancelOwner(rec, binding, hasBinding, berr, ep.ChangeID); refusal != "" {
		return cancelRefused(refusal)
	}
```

If `ownerID` was used later in `runCancel`, keep it: `ownerID, refusal := runCancelOwner(...)`. Check with `grep -n ownerID internal/app/runtracker_cancel.go` after the edit; the build fails on an unused variable either way, so let the compiler decide.

- [ ] **Step 4: Run the tests to verify they pass, including cancel's own shard**

Run: `go test -count=1 -run '^TestRunCancelOwnerMatchesCancelProofs$' ./internal/app/`
Expected: PASS.

Run: `go test -tags integration -count=1 -run '^TestIntegrationRunCancel' ./internal/app/`
Expected: PASS. This shard includes `TestIntegrationRunCancelRunCancelRefusedWrongClaim` and `TestIntegrationRunCancelRunCancelResumeAuthorityFailsClosed`, which pin the refusals end to end.

Mutation probe: in `runCancelOwner`, change `case hasBinding && binding.Confirmed:` to `case hasBinding:`, re-run the unit test with `-count=1`, and confirm both `unconfirmed reservation` and `reservation beside a resume shape` go red. Restore from a backup copy (`cp` the file before editing), not `git checkout`.

- [ ] **Step 5: Commit**

```bash
git add internal/app/runtracker_cancel.go internal/app/runtracker_cancel_owner_test.go
git commit -m "refactor(run-tracker): extract run cancel's ownership proof into runCancelOwner"
```

---

### Task 2: The live-run scan names a remedy that works

`liveRunsUnder` (`internal/app/runtracker_live.go`) today hard-codes `runCancelCommand(key)` for `active`/`completing` and "re-run the same cancel" for `cancelling`, even when cancel will refuse. Choose the remedy from the run's own key directory with `runCancelOwner`:

| `run.json` state | `record.json` + binding readable, cancel accepts | readable, cancel refuses | `record.json` missing/unreadable, or binding unreadable |
|---|---|---|---|
| `active`, `completing` | `docket run cancel --key <key> --reason <why>` | `docket run verdict <key>` | inspect or remove `<dir>` by hand |
| `cancelling` | re-run the same cancel until it reports cancelled | by hand | by hand |
| unknown state | by hand | by hand | by hand |
| unreadable `run.json` | by hand | by hand | by hand |

The third column extends the spec's "unknown or unreadable → by hand" row to the two other files the remedies depend on: with `record.json` missing or unreadable, both `run cancel` and `run verdict` fail to load it, and with a corrupt binding, cancel refuses `claim-unreadable` and the verdict stops `binding-unreadable` without retiring the run. Printing either command would be a remedy that fails as run (learning `printed-remedy-state-validity`).

`record.json` is read through a new `decodeRunTrackerRecord`, which holds every read-boundary check `LoadRunTrackerRecord` makes except the repository-identity check. The scan has only a state directory, which already belongs to the repository. `LoadRunTrackerRecord` calls the same helper, so the two cannot drift. The only ordering change is that a record that is both from the wrong repository and internally corrupt now reports `corrupt-record` rather than `wrong-repo`.

**Files:**
- Modify: `internal/app/runtracker_store.go` (`LoadRunTrackerRecord`; add `decodeRunTrackerRecord` next to it)
- Modify: `internal/app/runtracker_live.go`
- Test: `internal/app/runtracker_live_test.go`

**Interfaces:**
- Consumes: `runCancelOwner` (Task 1); `readRunTrackerClaimBinding(dir, op string) (RunTrackerClaimBinding, bool, error)` (existing).
- Produces:
  - `func decodeRunTrackerRecord(buf []byte, op string) (RunTrackerRecord, error)`
  - `func runVerdictCommand(key string) string` returning `"docket run verdict " + key`
  - `type liveRunAuthority int` with constants `liveRunAuthorityUnknown`, `liveRunCancelAccepts`, `liveRunCancelRefuses`
  - `func liveRunCancelAuthority(dir, runChangeID string) liveRunAuthority`

- [ ] **Step 1: Write the failing tests**

In `internal/app/runtracker_live_test.go`, add these fixture writers below `writeRunFile`:

```go
// writeTrackerRecordFixture writes a schema-current record.json into key's
// directory under stateDir (no git: Repo is left empty, which only the
// repository-identity check reads).
func writeTrackerRecordFixture(t *testing.T, stateDir, key string, rec RunTrackerRecord) {
	t.Helper()
	rec.Schema = runTrackerSchemaVersion
	buf, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	writeKeyFile(t, stateDir, key, runTrackerRecordFileName, buf)
}

// writeBindingFixture writes claim-binding.json into key's directory.
func writeBindingFixture(t *testing.T, stateDir, key string, b RunTrackerClaimBinding) {
	t.Helper()
	b.Schema = bindingSchemaVersion
	buf, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	writeKeyFile(t, stateDir, key, runTrackerClaimBindingName, buf)
}

func writeKeyFile(t *testing.T, stateDir, key, name string, buf []byte) {
	t.Helper()
	dir := filepath.Join(stateDir, runTrackerDirName, key)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), buf, 0o600); err != nil {
		t.Fatal(err)
	}
}

// cancellableRun gives key the authority run cancel accepts: a parent
// capability and a confirmed binding for change 7 (writeRunFixture's change).
func cancellableRun(t *testing.T, stateDir, key string) {
	t.Helper()
	writeTrackerRecordFixture(t, stateDir, key, RunTrackerRecord{ParentCap: "cap", AttemptLimit: 2})
	writeBindingFixture(t, stateDir, key, RunTrackerClaimBinding{ChangeID: 7, RequestID: "r", Confirmed: true})
}
```

Update `TestLiveRunsUnderReportsLiveAndUnreadable` so its live runs carry cancel authority (the expectations stay as they are today). After the `writeRunFixture` calls, add:

```go
	cancellableRun(t, stateDir, "k-active")
	cancellableRun(t, stateDir, "k-completing")
	cancellableRun(t, stateDir, "k-cancelling")
```

Add the remedy-matrix test:

```go
// TestLiveRunsUnderRemedyFollowsCancelAuthority: each live run's remedy is a
// command that will act on it in its current state (change 0540). Cancel is named
// only when runCancelOwner accepts; a readable run cancel would refuse gets the
// keyed verdict, which retires it; anything neither command can load is by hand.
func TestLiveRunsUnderRemedyFollowsCancelAuthority(t *testing.T) {
	stateDir := testsupport.TempDir(t)
	cancel := func(k string) string { return "docket run cancel --key " + k + " --reason <why>" }
	verdict := func(k string) string { return "docket run verdict " + k }
	byHand := func(dir string) string { return "inspect or remove " + dir + " by hand" }

	// The incident: a record with no parent capability and no claim (no binding file).
	writeRunFixture(t, stateDir, "a-noclaim", RunActive)
	writeTrackerRecordFixture(t, stateDir, "a-noclaim", RunTrackerRecord{ChildContextHash: "h", AttemptLimit: 2, Terminal: true})

	// A capability but only an unconfirmed reservation.
	writeRunFixture(t, stateDir, "b-reserved", RunActive)
	writeTrackerRecordFixture(t, stateDir, "b-reserved", RunTrackerRecord{ParentCap: "cap", AttemptLimit: 2})
	writeBindingFixture(t, stateDir, "b-reserved", RunTrackerClaimBinding{ChangeID: 7, RequestID: "r"})

	// A confirmed binding but no parent capability: cancel refuses, the verdict does not need it.
	writeRunFixture(t, stateDir, "c-nocap", RunCompleting)
	writeTrackerRecordFixture(t, stateDir, "c-nocap", RunTrackerRecord{AttemptLimit: 2})
	writeBindingFixture(t, stateDir, "c-nocap", RunTrackerClaimBinding{ChangeID: 7, RequestID: "r", Confirmed: true})

	// Cancellable: confirmed binding and capability.
	writeRunFixture(t, stateDir, "d-owned", RunActive)
	cancellableRun(t, stateDir, "d-owned")

	// No record.json at all: neither cancel nor verdict can load the run.
	eDir := writeRunFixture(t, stateDir, "e-norecord", RunActive)

	// Unreadable record.json.
	fDir := writeRunFixture(t, stateDir, "f-badrecord", RunActive)
	writeKeyFile(t, stateDir, "f-badrecord", runTrackerRecordFileName, []byte("{not json"))

	// Corrupt binding: cancel refuses claim-unreadable, verdict stops binding-unreadable.
	gDir := writeRunFixture(t, stateDir, "g-badbinding", RunActive)
	writeTrackerRecordFixture(t, stateDir, "g-badbinding", RunTrackerRecord{ParentCap: "cap", AttemptLimit: 2})
	writeKeyFile(t, stateDir, "g-badbinding", runTrackerClaimBindingName, []byte("{not json"))

	// Cancelling with no claim (the death guardian fenced it): no command settles it.
	hDir := writeRunFixture(t, stateDir, "h-cancelling-noclaim", RunCancelling)
	writeTrackerRecordFixture(t, stateDir, "h-cancelling-noclaim", RunTrackerRecord{ParentCap: "cap", AttemptLimit: 2})

	// Cancelling and cancellable: re-run the cancel.
	writeRunFixture(t, stateDir, "i-cancelling-owned", RunCancelling)
	cancellableRun(t, stateDir, "i-cancelling-owned")

	// Owner differs from the run's change: cancel refuses claim-mismatch; the verdict still acts.
	writeRunFixture(t, stateDir, "j-mismatch", RunActive)
	writeTrackerRecordFixture(t, stateDir, "j-mismatch", RunTrackerRecord{ParentCap: "cap", AttemptLimit: 2})
	writeBindingFixture(t, stateDir, "j-mismatch", RunTrackerClaimBinding{ChangeID: 8, RequestID: "r", Confirmed: true})

	got, err := liveRunsUnder(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	remedies := map[string]string{}
	for _, l := range got {
		remedies[l.Key] = l.Remedy
	}
	want := map[string]string{
		"a-noclaim":            verdict("a-noclaim"),
		"b-reserved":           verdict("b-reserved"),
		"c-nocap":              verdict("c-nocap"),
		"d-owned":              cancel("d-owned"),
		"e-norecord":           byHand(eDir),
		"f-badrecord":          byHand(fDir),
		"g-badbinding":         byHand(gDir),
		"h-cancelling-noclaim": byHand(hDir),
		"i-cancelling-owned":   "re-run the same " + cancel("i-cancelling-owned") + " until it reports cancelled",
		"j-mismatch":           verdict("j-mismatch"),
	}
	if !reflect.DeepEqual(remedies, want) {
		t.Fatalf("remedies =\n%#v\nwant\n%#v", remedies, want)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -count=1 -run '^TestLiveRunsUnder' ./internal/app/`
Expected: `TestLiveRunsUnderRemedyFollowsCancelAuthority` FAILS (`a-noclaim`, `b-reserved`, `c-nocap`, `j-mismatch` show the cancel command; `e`/`f`/`g` show the cancel command; `h` shows "re-run the same cancel"). `TestLiveRunsUnderReportsLiveAndUnreadable` still PASSES (its expectations are unchanged).

- [ ] **Step 3: Implement**

In `internal/app/runtracker_store.go`, add above `LoadRunTrackerRecord`:

```go
// decodeRunTrackerRecord parses record.json bytes and applies every read-boundary
// check LoadRunTrackerRecord makes EXCEPT the repository-identity check (Repo vs the
// canonical common dir): an unparseable record, an unknown schema, a partial
// continuation triple, or a partial claim-binding mirror pair is a corrupt record.
// LoadRunTrackerRecord adds the identity check; the repository-wide live-run scan
// (liveRunCancelAuthority) reads a state folder that already selects the repository.
func decodeRunTrackerRecord(buf []byte, op string) (RunTrackerRecord, error) {
	var rec RunTrackerRecord
	if err := json.Unmarshal(buf, &rec); err != nil {
		return RunTrackerRecord{}, runTrackerErr(ErrRunTrackerCorruptRecord, op, err)
	}
	if rec.Schema != runTrackerSchemaVersion {
		return RunTrackerRecord{}, runTrackerErr(ErrRunTrackerCorruptRecord, op,
			fmt.Errorf("schema version %d, want %d", rec.Schema, runTrackerSchemaVersion))
	}
	// A partial continuation triple is a corrupt record: fail closed on read so a
	// half-written continuation is never handed to the verdict path.
	if !runTrackerContinuationTripleOK(rec) {
		return RunTrackerRecord{}, runTrackerErr(ErrRunTrackerCorruptRecord, op,
			errors.New("partial continuation triple"))
	}
	// A partial claim-binding mirror pair is a corrupt record: fail closed on read
	// so a half-written mirror is never handed to the verdict path.
	if !runTrackerBoundPairOK(rec) {
		return RunTrackerRecord{}, runTrackerErr(ErrRunTrackerCorruptRecord, op,
			errors.New("partial claim-binding mirror pair"))
	}
	return rec, nil
}
```

In `LoadRunTrackerRecord`, replace everything from `var rec RunTrackerRecord` through the `runTrackerBoundPairOK` block with:

```go
	rec, err := decodeRunTrackerRecord(buf, "load")
	if err != nil {
		return RunTrackerRecord{}, err
	}
	if rec.Repo != common {
		return RunTrackerRecord{}, runTrackerErr(ErrRunTrackerWrongRepo, "load", nil)
	}
```

Keep the retry-marker reflection and `return rec, nil` that follow.

In `internal/app/runtracker_live.go`:

1. Update the `liveRunsUnder` doc comment's last paragraph to say the remedy follows `liveRunCancelAuthority`, and replace the state switch with:

```go
		switch rec.State {
		case RunCompleted, RunCancelled, RunSuperseded:
			continue
		case RunActive, RunCompleting:
			switch liveRunCancelAuthority(dir, rec.ChangeID) {
			case liveRunCancelAccepts:
				loc.Remedy = runCancelCommand(key)
			case liveRunCancelRefuses:
				// Cancel refuses a run it cannot prove owned (ADR-0128); the keyed
				// verdict re-resolves ownership and retires a run that claimed nothing
				// (change 0540).
				loc.Remedy = runVerdictCommand(key)
			default:
				loc.Remedy = byHandRemedy(dir)
			}
		case RunCancelling:
			if liveRunCancelAuthority(dir, rec.ChangeID) == liveRunCancelAccepts {
				loc.Remedy = "re-run the same " + runCancelCommand(key) + " until it reports cancelled"
			} else {
				// Fenced cancelling with no claim to cancel under: no command settles it.
				loc.Remedy = byHandRemedy(dir)
			}
		default:
			// run cancel refuses a state it does not know, so only a person can settle it.
			loc.Remedy = byHandRemedy(dir)
		}
```

2. Add below `runCancelCommand`:

```go
// runVerdictCommand is the ready-to-run keyed verdict for key: for a run cancel
// would refuse, it re-resolves ownership and, on any run-done, retires the run.
func runVerdictCommand(key string) string {
	return "docket run verdict " + key
}

// liveRunAuthority is what run cancel would decide about one live run, read from
// its own key directory.
type liveRunAuthority int

const (
	// liveRunAuthorityUnknown: record.json is missing or unreadable, or the claim
	// binding is unreadable. Neither run cancel nor run verdict can act on the run.
	liveRunAuthorityUnknown liveRunAuthority = iota
	// liveRunCancelAccepts: runCancelOwner accepts — run cancel would act.
	liveRunCancelAccepts
	// liveRunCancelRefuses: the run is readable but runCancelOwner refuses.
	liveRunCancelRefuses
)

// liveRunCancelAuthority asks runCancelOwner — the exact ownership proof runCancel
// applies — about the run in dir, whose run.json names runChangeID. It reads the
// files only and starts no git.
func liveRunCancelAuthority(dir, runChangeID string) liveRunAuthority {
	buf, err := os.ReadFile(filepath.Join(dir, runTrackerRecordFileName))
	if err != nil {
		return liveRunAuthorityUnknown
	}
	rec, err := decodeRunTrackerRecord(buf, "live-runs")
	if err != nil {
		return liveRunAuthorityUnknown
	}
	binding, hasBinding, berr := readRunTrackerClaimBinding(dir, "live-runs")
	if berr != nil {
		// Cancel refuses claim-unreadable and the verdict stops binding-unreadable
		// without retiring the run: neither command settles it.
		return liveRunAuthorityUnknown
	}
	if _, refusal := runCancelOwner(rec, binding, hasBinding, nil, runChangeID); refusal != "" {
		return liveRunCancelRefuses
	}
	return liveRunCancelAccepts
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -count=1 -run '^TestLiveRunsUnder' ./internal/app/`
Expected: PASS (all three existing tests and the new one).

Run: `go test -tags integration -count=1 -run '^TestIntegrationRunRecord|^TestIntegrationRunCancel' ./internal/app/`
Expected: PASS (the store shard covers `LoadRunTrackerRecord`'s refusals).

Run: `go test -count=1 -run 'Visibility' ./internal/app/`
Expected: PASS. The set-visibility tests consume `liveRunsUnder` through `visibilityLiveRunLines`. If one asserts on the old cancel-remedy text for a fixture that has no `record.json`, update that fixture to carry cancel authority (use `cancellableRun`), or change the expected text to the by-hand remedy if the fixture really has no record. Never weaken the assert to a substring.

Mutation probes (each with `-count=1`, restoring from a backup copy):
- In `liveRunCancelAuthority`, delete the `if berr != nil` block (the predicate then refuses `claim-unreadable` → verdict remedy). `g-badbinding` must go red.
- Make `liveRunCancelAuthority` return `liveRunCancelRefuses` when `os.ReadFile` fails. `e-norecord` must go red.
- In the `RunCancelling` case, always use the re-run remedy. `h-cancelling-noclaim` must go red.

- [ ] **Step 5: Commit**

```bash
git add internal/app/runtracker_store.go internal/app/runtracker_live.go internal/app/runtracker_live_test.go
git commit -m "fix(run-tracker): name a live run's remedy from run cancel's own ownership proof"
```

---

### Task 3: Every keyed run-done verdict retires its run

Today `RunVerdict` (`internal/app/runtracker_verdict.go`) sends only `VerdictRunComplete` through `runTrackerCompleteRun`. `VerdictRunUnclaimed` and `runTrackerOwnershipDone` (`no-attributable-claim`) call `persistRunVerdict`, which writes `record.json` only, so `run.json` stays `active` forever. Generalize `runTrackerCompleteRun` to carry the outcome and send all three through it.

Consequences to keep in mind (all intended by the spec):
- Success lines are byte-identical to today's.
- A blocked closeout on a no-claim or unclaimed run reports `run-stop <key> run-tracker-unavailable completion-unaccounted` with `CompletionFindings`, leaves the run `completing`, and spends no retry.
- A run already `cancelling`/`cancelled` now reports `run-stop … run-tracker-unavailable run-cancelled` instead of `run-done`, and `superseded` reports `run-superseded`. The run is never relabelled.
- No `run.json` beside the record: the same mirrored line as today.
- `runTrackerOwnershipDone` gains a `wdeps` parameter so it can build the seams with `verdictSeams(repoDir, wdeps)`.

**Files:**
- Modify: `internal/app/runtracker_verdict.go` (`RunVerdict` switch, `runTrackerCompleteRun`, `runTrackerOwnershipDone` and its two call sites in `resolveRunTrackerOwnership`, file-header COMPLETION paragraph, `ReasonRunReportUnpersisted` comment)
- Modify: `internal/app/runtracker_complete.go` (file-header comment, `completeSuccessfulRun` doc comment)
- Modify: `docs/concepts/run-tracker.md` ("The verdict" bullet)
- Test: `internal/app/runtracker_verdict_integration_test.go`

**Interfaces:**
- Consumes: `runVerdictCommand` and `liveRunsUnder` (Task 2), used by the incident test.
- Produces:
  - `func runTrackerCompleteRun(repoDir, key string, rec RunTrackerRecord, outcome string, id int, seams cancelSeams) RunVerdictResult`, where `outcome` is one of `VerdictRunComplete`, `VerdictRunUnclaimed`, `RunOutcomeNoAttributableClaim`, and `id` is `0` for `no-attributable-claim`.
  - `func runTrackerOwnershipDone(wdeps WorkspaceDeps, repoDir, key string, rec RunTrackerRecord) *RunVerdictResult`

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/runtracker_verdict_integration_test.go` (it already imports `context`, `gatedrive`, and `testsupport`; add nothing else unless the compiler asks):

```go
// --- every keyed run-done retires its run (change 0540) ----------------------
//
// Before 0540 only run-complete drove the closeout; no-attributable-claim and
// run-unclaimed wrote record.json alone and left run.json active forever, so the
// repository-wide live-run scan refused set-visibility and run cancel refused
// claim-unconfirmed. These tests pin the closeout on every run-done.

// noClaimRunFixture is a keyed run that claimed nothing — no binding file and no
// committed proof under its context hash "ha" — with an active run.json beside its
// record and permissive, faked closeout seams.
type noClaimRunFixture struct {
	repo, key      string
	wdeps          WorkspaceDeps
	observer       *fakeProcessObserver
	launchObserver *fakeLaunchObserver
}

func newNoClaimRunFixture(t *testing.T) noClaimRunFixture {
	t.Helper()
	repo := newRunTrackerRepo(t)
	key := runTrackerMintStarted(t, repo, nil, 1, "ha")
	if _, err := MintRunRecord(repo, key, ""); err != nil {
		t.Fatalf("MintRunRecord: %v", err)
	}
	observer := &fakeProcessObserver{defaultProven: true}
	launchObserver := &fakeLaunchObserver{report: gatedrive.RunLaunchReport{Accounted: true}}
	wdeps := WorkspaceDeps{
		// A sibling's proof under a DIFFERENT context hash: filtered out, so zero match.
		ClaimProofs: &fakeProofScanner{proofs: []ClaimProof{{RequestID: "x", ChangeID: 9, RunContextHash: "OTHER"}}},
		CancelSeams: func(string) cancelSeams {
			return cancelSeams{observer: observer, launchObserver: launchObserver}
		},
	}
	return noClaimRunFixture{repo: repo, key: key, wdeps: wdeps, observer: observer, launchObserver: launchObserver}
}

// TestIntegrationRunVerdictNoAttributableClaimRetiresStrandedRun reproduces the
// #540 incident: record.json already terminal from an earlier verdict, run.json
// still active, no claim. The live-run scan lists it with the keyed-verdict remedy;
// re-running that verdict reports the unchanged line, closes the run out through
// the census for its own context hash, and the scan no longer lists it.
func TestIntegrationRunVerdictNoAttributableClaimRetiresStrandedRun(t *testing.T) {
	fx := newNoClaimRunFixture(t)
	rec, err := LoadRunTrackerRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord: %v", err)
	}
	rec.Terminal = true
	rec.Disposition = "run-done " + fx.key + " no-attributable-claim"
	must(t, SaveRunTrackerRecord(fx.repo, fx.key, rec))
	stateDir, err := runTrackerStateDir(fx.repo)
	if err != nil {
		t.Fatalf("runTrackerStateDir: %v", err)
	}
	before, err := liveRunsUnder(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || before[0].Key != fx.key || before[0].Remedy != runVerdictCommand(fx.key) {
		t.Fatalf("stranded run before the verdict = %#v, want one locator with remedy %q", before, runVerdictCommand(fx.key))
	}

	res := RunVerdict(context.Background(), PlanningDeps{}, fx.wdeps, GitHubDeps{}, fx.repo, fx.key)
	if got, want := res.HumanText(), "run-done "+fx.key+" no-attributable-claim"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleted {
		t.Fatalf("run state = %q, want completed", st)
	}
	if calls := fx.launchObserver.calls; len(calls) != 1 || calls[0] != "ha" {
		t.Fatalf("census calls = %v, want [ha] (the closeout ran for the run's context hash)", calls)
	}
	after, err := liveRunsUnder(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 0 {
		t.Fatalf("live runs after the verdict = %#v, want none", after)
	}
	if runTrackerRetryMarkerExists(t, fx.repo, fx.key) {
		t.Error("retiring a no-claim run must never spend the retry")
	}
}

// TestIntegrationRunVerdictUnconfirmedReservationRetiresRun: the other
// no-attributable-claim leg — a reservation whose claim never committed — also
// closes the run out, and leaves the reservation refused (never confirmed).
func TestIntegrationRunVerdictUnconfirmedReservationRetiresRun(t *testing.T) {
	fx := newNoClaimRunFixture(t)
	must(t, ReserveRunTrackerClaim(fx.repo, fx.key, 3, "claim-3-v"))
	fx.wdeps.ClaimProofs = &fakeProofScanner{proofs: nil}

	res := RunVerdict(context.Background(), PlanningDeps{}, fx.wdeps, GitHubDeps{}, fx.repo, fx.key)
	if got, want := res.HumanText(), "run-done "+fx.key+" no-attributable-claim"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleted {
		t.Fatalf("run state = %q, want completed", st)
	}
	b, ok, err := LoadRunTrackerClaimBinding(fx.repo, fx.key)
	if err != nil || !ok || b.Confirmed {
		t.Fatalf("binding = %+v ok=%v err=%v, want the unconfirmed reservation intact", b, ok, err)
	}
}

// TestIntegrationRunVerdictRunUnclaimedRetiresRun: a confirmed claim whose change
// went back to proposed reports the unchanged run-unclaimed line and closes the
// run out.
func TestIntegrationRunVerdictRunUnclaimedRetiresRun(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := runTrackerMintStarted(t, repo, nil, 1, "ha")
	must(t, ReserveRunTrackerClaim(repo, key, 3, "claim-3-v"))
	must(t, ConfirmRunTrackerClaim(repo, key, 3, "claim-3-v", "r1", ""))
	if _, err := MintRunRecord(repo, key, "3"); err != nil {
		t.Fatalf("MintRunRecord: %v", err)
	}
	launchObserver := &fakeLaunchObserver{report: gatedrive.RunLaunchReport{Accounted: true}}
	wdeps := WorkspaceDeps{
		ClaimProofs: &fakeProofScanner{proofs: []ClaimProof{
			{RequestID: "claim-3-v", ChangeID: 3, RunContextHash: "ha", Revision: "r1"},
		}},
		CancelSeams: func(string) cancelSeams {
			return cancelSeams{observer: &fakeProcessObserver{defaultProven: true}, launchObserver: launchObserver}
		},
	}
	deps := runTrackerLightDeps(t, []StatusBlob{runTrackerProposedBlob(3, rvSlug)})

	res := RunVerdict(context.Background(), deps, wdeps, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "run-done "+key+" run-unclaimed 3"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if st := loadRunState(t, repo, key); st != RunCompleted {
		t.Fatalf("run state = %q, want completed", st)
	}
	if len(launchObserver.calls) != 1 {
		t.Fatalf("census calls = %v, want one (the closeout ran)", launchObserver.calls)
	}
}

// TestIntegrationRunVerdictNoClaimBlockedCloseoutReplays: an unproven obligation
// blocks the closeout — run-stop completion-unaccounted with findings, the run
// stays completing, no retry spent. Once it settles, repeating the same keyed
// verdict replays the closeout to the unchanged run-done line and completed.
func TestIntegrationRunVerdictNoClaimBlockedCloseoutReplays(t *testing.T) {
	fx := newNoClaimRunFixture(t)
	must(t, RegisterRunParticipant(fx.repo, fx.key, RunParticipant{Kind: "raw-run", NativeHandle: "exec-live"}))
	fx.observer.defaultProven = false

	res := RunVerdict(context.Background(), PlanningDeps{}, fx.wdeps, GitHubDeps{}, fx.repo, fx.key)
	if res.Decision != RunDecisionStop || res.Outcome != RunOutcomeUnavailable || res.Reason != ReasonRunCompletionUnaccounted {
		t.Fatalf("decision/outcome/reason = %q/%q/%q, want run-stop/run-tracker-unavailable/completion-unaccounted",
			res.Decision, res.Outcome, res.Reason)
	}
	if len(res.CompletionFindings) == 0 {
		t.Fatal("a blocked closeout carried no findings to settle")
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleting {
		t.Fatalf("run state = %q, want completing (the fence holds)", st)
	}
	if runTrackerRetryMarkerExists(t, fx.repo, fx.key) {
		t.Error("a blocked closeout must never spend the retry")
	}

	fx.observer.defaultProven = true
	res2 := RunVerdict(context.Background(), PlanningDeps{}, fx.wdeps, GitHubDeps{}, fx.repo, fx.key)
	if got, want := res2.HumanText(), "run-done "+fx.key+" no-attributable-claim"; got != want {
		t.Fatalf("replay HumanText = %q, want %q", got, want)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleted {
		t.Fatalf("run state after replay = %q, want completed", st)
	}
}

// TestIntegrationRunVerdictHaltedLeavesRunActive: run-halted is a stop, not a
// run-done. The run stays active (so halt → cancel → resume still works) and the
// closeout never runs.
func TestIntegrationRunVerdictHaltedLeavesRunActive(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := runTrackerMintAttributedLimit(t, repo, 3, 2)
	if _, err := MintRunRecord(repo, key, "3"); err != nil {
		t.Fatalf("MintRunRecord: %v", err)
	}
	launchObserver := &fakeLaunchObserver{report: gatedrive.RunLaunchReport{Accounted: true}}
	wdeps := WorkspaceDeps{CancelSeams: func(string) cancelSeams {
		return cancelSeams{observer: &fakeProcessObserver{defaultProven: true}, launchObserver: launchObserver}
	}}
	deps := runTrackerLightDeps(t, []StatusBlob{runTrackerHaltedInProgressBlob(3, rvSlug)})

	res := RunVerdict(context.Background(), deps, wdeps, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "run-stop "+key+" run-halted 3"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if st := loadRunState(t, repo, key); st != RunActive {
		t.Fatalf("run state = %q, want active (a halted run waits for cancel or continue)", st)
	}
	if len(launchObserver.calls) != 0 {
		t.Fatalf("a halt ran the closeout census: %v", launchObserver.calls)
	}
}

// TestIntegrationRunVerdictNoClaimNeverRelabelsCancellingOrSuperseded: a run
// already fenced cancelling (or superseded) is never marked completed by a
// no-attributable-claim verdict; the closeout's own refusal is reported.
func TestIntegrationRunVerdictNoClaimNeverRelabelsCancellingOrSuperseded(t *testing.T) {
	for _, tc := range []struct {
		state  runState
		reason string
	}{
		{RunCancelling, ReasonRunCancelled},
		{RunSuperseded, ReasonRunSuperseded},
	} {
		t.Run(string(tc.state), func(t *testing.T) {
			fx := newNoClaimRunFixture(t)
			forceRunState(t, fx.repo, fx.key, tc.state)
			res := RunVerdict(context.Background(), PlanningDeps{}, fx.wdeps, GitHubDeps{}, fx.repo, fx.key)
			if res.Decision != RunDecisionStop || res.Reason != tc.reason {
				t.Fatalf("decision/reason = %q/%q, want run-stop/%s", res.Decision, res.Reason, tc.reason)
			}
			if st := loadRunState(t, fx.repo, fx.key); st != tc.state {
				t.Fatalf("run state = %q, want %q (never relabelled)", st, tc.state)
			}
		})
	}
}

// TestIntegrationRunVerdictObserveLeavesNoClaimRunActive: unattributed observe
// mode over the incident fixture holds no key, so it never reaches the closeout —
// the run stays active and byte-identical.
func TestIntegrationRunVerdictObserveLeavesNoClaimRunActive(t *testing.T) {
	fx := newNoClaimRunFixture(t)
	_, genBefore, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("load run before: %v", err)
	}
	res := RunVerdictObserve(context.Background(), runTrackerLightDeps(t, nil), fx.wdeps, GitHubDeps{}, fx.repo, nil)
	if got, want := res.HumanText(), "run-observe no-current-run"; got != want {
		t.Fatalf("observe HumanText = %q, want %q", got, want)
	}
	ep, genAfter, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("load run after: %v", err)
	}
	if ep.State != RunActive || genAfter != genBefore {
		t.Fatalf("observe mode wrote the run: state %q, generation %q -> %q", ep.State, genBefore, genAfter)
	}
	if len(fx.launchObserver.calls) != 0 {
		t.Fatalf("observe mode ran the closeout census: %v", fx.launchObserver.calls)
	}
}
```

Before relying on `"run-stop "+key+" run-halted 3"`, confirm the halt line's shape against `TestIntegrationRunVerdictVerdictHaltPrecedenceOverBudget` and `HumanText` (the `VerdictRunHalted` case appends the id). If `runTrackerMintAttributedLimit`'s record (resume-attributed: `AttributedID` set, no `BoundRequestID`) hits a different ownership path than expected, read `resolveRunTrackerOwnership` and adjust the fixture, not the expected line.

Also strengthen the existing keyless test so the no-run path is pinned. At the end of `TestIntegrationRunVerdictVerdictNoBindingNoProofIsNoAttributableClaim`, add:

```go
	if _, _, err := LoadRunRecord(repo, key); !isRunKind(err, ErrRunNotFound) {
		t.Fatalf("no run beside the record: the verdict must not fabricate one: %v", err)
	}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags integration -count=1 -run '^TestIntegrationRunVerdict(NoAttributableClaimRetiresStrandedRun|UnconfirmedReservationRetiresRun|RunUnclaimedRetiresRun|NoClaimBlockedCloseoutReplays|HaltedLeavesRunActive|NoClaimNeverRelabelsCancellingOrSuperseded|ObserveLeavesNoClaimRunActive)$' ./internal/app/`
Expected: `NoAttributableClaimRetiresStrandedRun`, `UnconfirmedReservationRetiresRun`, `RunUnclaimedRetiresRun`, `NoClaimBlockedCloseoutReplays`, and `NoClaimNeverRelabelsCancellingOrSuperseded` FAIL (state stays `active`, no census call, `run-done` instead of `run-stop`). `HaltedLeavesRunActive` and `ObserveLeavesNoClaimRunActive` PASS already. They are regression guards for behavior that must not move.

- [ ] **Step 3: Implement**

In `internal/app/runtracker_verdict.go`:

1. `RunVerdict` switch. Replace the `VerdictRunComplete` and `VerdictRunUnclaimed` cases with one case:

```go
	case VerdictRunComplete, VerdictRunUnclaimed:
		// Every keyed run-done drives the run's ownership closeout (change 0441 for
		// run-complete, extended to every run-done by change 0540) so run.json leaves
		// active and the run stops counting as live. The seam bundle is injectable
		// (unit tests fake the observers); production composes
		// productionCancelSeams(repoDir). RunVerify's verdict is reported as fact —
		// runTrackerCompleteRun never re-derives it.
		return runTrackerCompleteRun(repoDir, key, rec, v.Verdict, id, verdictSeams(repoDir, wdeps))
```

2. `runTrackerCompleteRun`. Change the signature to `func runTrackerCompleteRun(repoDir, key string, rec RunTrackerRecord, outcome string, id int, seams cancelSeams) RunVerdictResult`. Replace each of the two `runVerdictLine(key, RunDecisionDone, VerdictRunComplete, id, true, …)` calls with `runVerdictLine(key, RunDecisionDone, outcome, id, true, …)`. Rewrite its doc comment's first sentence to: "runTrackerCompleteRun maps a keyed run-done verdict — run-complete, run-unclaimed, or no-attributable-claim (outcome; id is 0 for no-attributable-claim) — onto the run's ownership closeout (changes 0441, 0540)." In point (3), replace "replays to run-done run-complete" with "replays to the same run-done line".

3. `runTrackerOwnershipDone`. Replace it with:

```go
// runTrackerOwnershipDone handles a dispatch that provably claimed nothing: it
// reports run-done no-attributable-claim and retires the run through the same
// closeout as every keyed run-done (change 0540), so a run that never claimed is
// never left active for the live-run scan to refuse on. A blocked closeout
// reports run-stop run-tracker-unavailable with the closeout's reason; no run beside
// the record keeps the prior mirror-only behavior.
func runTrackerOwnershipDone(wdeps WorkspaceDeps, repoDir, key string, rec RunTrackerRecord) *RunVerdictResult {
	res := runTrackerCompleteRun(repoDir, key, rec, RunOutcomeNoAttributableClaim, 0, verdictSeams(repoDir, wdeps))
	return &res
}
```

and update both call sites in `resolveRunTrackerOwnership` to `return runTrackerOwnershipDone(wdeps, repoDir, key, *rec)`.

4. Comments. In the file-header COMPLETION paragraph, change "On a keyed run-complete it additionally drives the successful-run ownership closeout" to "On every keyed run-done — run-complete, run-unclaimed, and no-attributable-claim (change 0540) — it additionally drives the run ownership closeout", and change "A BLOCKED or lost closeout maps to" so it does not imply run-complete only. In the `ReasonRunReportUnpersisted` comment, replace "replays to run-done run-complete" with "replays to the same run-done line". In the `ReasonRunCompletionUnaccounted` block heading comment "(change 0441)… when the keyed run-complete verdict cannot report success", say "a keyed run-done verdict".

In `internal/app/runtracker_complete.go`, change the header sentence "It is driven ONLY from the attributed, keyed RunVerdict path on a verified run-complete (Task 8 wires the caller)" to "It is driven ONLY from the attributed, keyed RunVerdict path, on every run-done verdict (run-complete since change 0441; run-unclaimed and no-attributable-claim since change 0540)". In `completeSuccessfulRun`'s doc comment, change "has already resolved the confirmed claim binding and the run-complete verdict" to "has already resolved ownership and a run-done verdict". Leave the "successful-run" name: renaming it is out of scope.

5. Find stale wording repo-wide instead of trusting this list: `grep -rn "run-complete" internal/app/runtracker_verdict.go internal/app/runtracker_complete.go` and fix any remaining comment that says only run-complete reaches the closeout.

In `docs/concepts/run-tracker.md`, append to "The verdict" bullet (after "holding no key and writing nothing."):

```markdown
  A keyed `run-done` line also retires the run, so it no longer counts as
  live; if something the run started is still settling, the verdict reports
  `run-stop … completion-unaccounted` instead, and repeating the same verdict
  once it settles retires the run.
```

Describe current behavior only. No change numbers in the doc.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go build ./... && go vet -tags integration ./internal/app/`
Expected: clean.

Run: `go test -tags integration -count=1 -run '^TestIntegrationRunVerdict' ./internal/app/`
Expected: PASS, the whole verdict shard, including the run-complete closeout tests (`…RunCompleteClosesOutRunOwnership`, `…RunCompleteWithoutRunUnchanged`, `…RunCompleteBlockedCloseoutStopsWithoutSuccess`, `…RunCompleteCancelledRunNeverReportsSuccess`, `…RunCompleteReportPersistFailureIsReported`).

Run: `go test -tags integration -count=1 -run '^TestIntegrationRunStart|^TestIntegrationRunCompletion|^TestIntegrationRunFence|^TestIntegrationRunCancel' ./internal/app/`
Expected: PASS. `TestIntegrationRunStartResumeAfterCancelReservesOnce` and `TestIntegrationRunStartResumeRefusesActiveRunWithLocator` keep pinning halt → cancel → resume.

Run: `go test -tags integration -count=1 ./cmd/docket/ ./internal/cli/` and `go test -tags integration -count=1 -run 'NoAttributable|Unclaimed|RootEntry' ./internal/app/`
Expected: PASS. `change_integration_test.go` and `root_entry_integration_test.go` mention these outcomes. If one now sees `run-stop … run-cancelled` where it expected `run-done`, read why. That is the never-relabel rule only when the fixture's run is already cancelling/cancelled/superseded. Update the expectation and say so in the commit message. Otherwise treat it as a defect.

Mutation probes (each with `-count=1`, restoring from a backup copy):
- Revert `runTrackerOwnershipDone` to the old `persistRunVerdict(… RunOutcomeNoAttributableClaim …)` body. `NoAttributableClaimRetiresStrandedRun`, `UnconfirmedReservationRetiresRun`, `NoClaimBlockedCloseoutReplays`, and `NoClaimNeverRelabelsCancellingOrSuperseded` must go red.
- Split `VerdictRunUnclaimed` back into its own `persistRunVerdict` case. `RunUnclaimedRetiresRun` must go red.
- In `runTrackerCompleteRun`, hard-code `VerdictRunComplete` in the success line. `NoAttributableClaimRetiresStrandedRun` and `RunUnclaimedRetiresRun` must go red on the HumanText assert.

- [ ] **Step 5: Commit**

```bash
git add internal/app/runtracker_verdict.go internal/app/runtracker_complete.go internal/app/runtracker_verdict_integration_test.go docs/concepts/run-tracker.md
git commit -m "fix(run-tracker): retire the run on every keyed run-done verdict"
```

---

### Task 4: Whole-suite gate and budget read

No code. Proves the branch as a whole.

**Files:** none (read-only verification).

- [ ] **Step 1: Run the whole suite through the configured build command**

Read `build.test_command` from the effective config (`.docket.yml` currently says `go run ./cmd/docket development test`). Run it from the feature worktree root.
Expected: green. A red test outside the files above is a hypothesis, not a verdict. Re-run it on the unmodified base before attributing it to this branch.

- [ ] **Step 2: Read the budget report**

Check the output for `BUDGET WATCH:`, `PARALLEL-SENSITIVE:`, or `SERIAL CONFIRMED OVER BUDGET:` lines naming `tests/test_go_integration_app_runverdict.sh` (budget 20s, parallel) or `tests/test_go_integration_app_runcancel.sh` (15s). This change adds seven real-git verdict tests. Record the runverdict shard's measured time against its 20s ceiling in the build evidence as a number, never as "did not trip the budget check". A `SERIAL CONFIRMED OVER BUDGET` line on these shards is a finding to act on: shard the new tests behind a new prefix rather than raising the ceiling silently.

- [ ] **Step 3: Audit the branch against its own thesis**

Run: `git diff <pre-build HEAD>..HEAD -- internal/app | grep -n -e 'persistRunVerdict(' -e 'runCancelCommand('`
Every remaining `persistRunVerdict` call that emits `RunDecisionDone` is a defect: a `run-done` that bypasses the closeout. Every remaining bare `runCancelCommand(key)` remedy must sit behind `liveRunCancelAccepts`.

---

## Self-review against the spec

- §1 per-verdict table: `run-complete` unchanged (existing closeout tests, Task 3 Step 4); `no-attributable-claim` → closeout (Task 3, both ownership legs); `run-unclaimed` → closeout (Task 3); halted/stop unchanged (Task 3 halted test); retry/continue untouched (verdict shard stays green).
- §1 bullets: outcome carried through (Task 3 Step 3.2); success lines unchanged (HumanText asserts); blocked closeout + replay, no retry (Task 3 blocked test); never relabel (Task 3 cancelling/superseded test); no run beside the record (strengthened keyless test); observe mode read-only (Task 3 observe test); no schema/state change (Global Constraints).
- §2 remedy matrix: every cell pinned in Task 2's matrix test; the predicate is extracted (Task 1), not copied.
- §3 ADR: done by the coordinator at review (no task, per dispatch).
- §4 no migration: the stranded-run test is exactly "re-run the keyed verdict on a terminal record with an active run". The results file (coordinator-owned) carries the human instructions.
- Tests 1–8 of the spec map to: 1 → `NoAttributableClaimRetiresStrandedRun`; 2 → `UnconfirmedReservationRetiresRun`; 3 → `RunUnclaimedRetiresRun`; 4 → `NoClaimBlockedCloseoutReplays`; 5 → `HaltedLeavesRunActive` (plus the existing resume-after-cancel tests); 6 → `NoClaimNeverRelabelsCancellingOrSuperseded`; 7 → `TestLiveRunsUnderRemedyFollowsCancelAuthority`; 8 → `ObserveLeavesNoClaimRunActive`.
