<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0497 — Re-running finalize can start a second suite beside an orphaned one](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0497-re-running-finalize-can-start-a-second-suite-beside-an-orpha.md)**
<!-- docket:backlink:end -->
# Re-running finalize can start a second suite beside an orphaned one — Implementation Plan

> **For agentic workers:** this plan is executed by **docket-build**: one tier-routed worker per
> task, strictly sequential in the feature worktree, one commit per task, no per-task review, and a
> single full-suite gate at the end. Each task carries an explicit **Build tier:** line. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a gate drive halts because its supervisor died and part of its suite is still
running, the halt carries the informational `tree-survives:<drive>:<pgid>` finding, finalize's
halt message says the supervisor died and tells the human to wait for that process group before
re-running, and the build role relays the finding into its halt report — with no new halt, block,
refusal, or signal, and no change to any halt's outcome or cause.

**Architecture:** (1) `internal/gatedrive`: `driveSlice`'s death branch (`StateSignaled`,
`StateVanished`), after choosing `supervisor-died` or `uncertain-ownership`, calls the existing
read-only `ProcessSeam.ProbeLeftover` on the drive's run dir. Only a nil-error `LeftoverPresent`
answer stamps the finding. The slice result carries it, the persist CAS stores it as
`last_finding` (omitempty, no schema bump), and `recordedDoc` exposes it as `DriveDoc.Finding` on a
HALTED document only. The census and the halt share one spelling helper. (2) `internal/app`:
`GateDriveResult.HumanText` prints a `finding:` line; the finalize gate seam carries the driver's
raw cause and finding up as `LocalGateResult.HaltDriveCause` / `HaltLeftover`; the finding fills
`teardown_finding` when no retention token does; `finalize.rebase`'s halt message is composed by
one helper that names the supervisor death and the wait. Disposition, reason, result, and halt cause
are untouched. (3) Skill prose and the glossary relay it; the ADR-0134 Update note is the
coordinator's.

**Tech Stack:** Go (module `github.com/danielhanold/docket`, `go 1.26.0`), `go test` (with
`-tags integration` for the real-git app tests), the asset mirror generator
(`go generate ./internal/assets`), the Go suite runner (`go run ./cmd/docket development test`, run
by docket-build at the gate, never by a task).

**Spec:** `docs/superpowers/specs/2026-10-04-re-running-finalize-can-start-a-second-suite-beside-an-orpha-design.md`
(on the `docket` metadata branch; the synchronized copy is under `.docket/` in the primary
checkout). Executors read the spec beside this plan. Change file:
`docs/changes/active/0497-re-running-finalize-can-start-a-second-suite-beside-an-orpha.md`.

## Global Constraints

- **No new halt, block, refusal, or signal.** The probe is read-only and runs only inside the
  death branch, after the cause is chosen. A finding never changes a drive's `Outcome` (`HALTED`)
  or `Cause`, and never changes finalize's disposition (`blocked`), reason (`gate-halted`), result
  (`blocked`), or `halt_cause` (`unavailable`).
- **Only a clean leftover answer stamps the finding:** `lerr == nil && lo.Answer ==
  process.LeftoverPresent`. `none`, `unclear`, and any probe error (even one paired with a
  `LeftoverPresent` answer) stamp nothing (learning: probe-error-is-not-clean-absence).
- **The finding is exactly `tree-survives:<drive id>:<decimal pgid>`** — no path, argv, env value,
  or credential — spelled by one helper shared with the launch census.
- **No schema or protocol bump.** `last_finding` and `finding` are `omitempty`; an older record
  without `last_finding` decodes and re-reads unchanged. `driveSchemaVersion` stays 4 and
  `ProtocolVersion` stays 1.
- **The finalize message text is the contract** (copy verbatim; `<pgid>` is the decimal group id):
  - `supervisor-died`, no finding: `the local gate's supervisor died before the suite finished; re-run finalize to re-run the suite`
  - `supervisor-died`, finding: `the local gate's supervisor died before the suite finished; part of the suite is still running as process group <pgid> — wait until pgrep -lg <pgid> prints nothing, then re-run finalize`
  - `uncertain-ownership`, finding: `the local gate did not reach a decidable pass/fail; retained, no red fabricated; part of the suite is still running as process group <pgid> — wait until pgrep -lg <pgid> prints nothing, then re-run finalize`
  - every other halt: today's message, byte for byte (the generic `the local gate did not reach a decidable pass/fail; retained, no red fabricated`, or the `the local gate could not start: …` refusal message).
- **`teardown_finding` precedence:** a run-root retention token (`run-root-retained-unsettled`,
  `start-failed-run-root-retained`) wins; the leftover still reaches the message through
  `HaltLeftover`.
- **Unchanged:** gate admission, the worktree lock and holder note (ADR-0132); the launch census and
  its findings (ADR-0134); `run.start`, `run.verdict`, `run.cancel` output; finalize's run-root
  removal; the relaunch retirement (ADR-0135); `evidence.recertify`'s code and messages.
- **ADR work is out of scope for every build task.** The dated Update note on ADR-0134 is recorded by
  the coordinator through docket-adr after the build. No task creates or edits anything under
  `docs/adrs/`.
- **Frozen records are never edited:** `docs/superpowers/plans/**` (except this plan),
  `docs/superpowers/specs/**`, `docs/results/**`, `docs/changes/**`, Accepted ADR bodies.
- **Comment anchors** name a symbol or quote a clause, never a line number (ADR-0054).
- **Do not write the hyphenated run-id spelling** in new prose or comments; write "run id". In new
  skill prose, do not write the worker outcome tokens (`COMPLETE`, `BLOCKED`, `NEEDS_ESCALATION`).
- **Every test run that observes a change in outcome defeats the cache:** `go test -count=1 …`.
- **Mutation procedure:** `cp <file> <file>.bak` → apply the mutation → confirm it landed with a
  `grep -c` before and after → run the test with `-count=1` (no `(cached)`) → `mv -f <file>.bak
  <file>` → re-run green. Never restore with `git checkout --`. Record every mutation's reading
  (mutation, test, red/green) in the task's commit body; the results file cites them.
- **App integration tests keep their shard prefix:** a finalize-rebase test name starts with
  `TestIntegrationFinalizeRebaseOps` (`tests/test_go_integration_app_finalizerebaseops.sh`).
- **Tasks run focused tests only.** The whole suite runs once at docket-build's gate through
  `build.test_command`; read every `BUDGET WATCH:` and `SERIAL CONFIRMED OVER BUDGET:` line there.

## Review Focus

1. **A probe error paired with a `LeftoverPresent` answer.** The halt must trust the answer only
   when the error is nil, or a failed probe prints a wait instruction for a group it never proved.
   Pinned in Task 1 (`error` row of `leftoverAnswers`).
2. **A re-read of the terminal drive.** A second `Advance` (and the handoff/claim re-read) must
   return the same finding from the record and must not probe again. Pinned in Task 1
   (`TestDeathHaltFindingSurvivesReRead`).
3. **A finding on a non-HALTED record.** Only a HALTED document exposes `finding`; a record
   carrying `last_finding` beside any other outcome must not leak it. Pinned in Task 1
   (`TestDeathHaltFindingOnlyOnHalted`).
4. **Retention and leftover together.** When the halted run root is retained,
   `teardown_finding` keeps the retention token and the message still carries the wait clause.
   Pinned in Task 2 (`retained` row of `TestMapDriveOutcomeCarriesLeftoverFinding`).
5. **A malformed finding reaching finalize.** An empty drive, a missing or non-numeric pgid, a pgid
   `<= 1`, or another finding's token must fall back to the message without the wait clause, never
   a garbled `process group ` line. Pinned in Task 2 (malformed rows of
   `TestFinalizeGateHaltMessage`).

---

### Task 1: The death halt probes for a leftover and persists the finding

**Build tier:** premium

**Risk:** the three-way probe answer and the failure posture are the decision; collapsing an
error into "leftover", or letting the finding touch the cause, is exactly the defect class this
change must not ship.

**Files:**
- Modify: `internal/gatedrive/drive.go` (exported `CauseUncertainOwnership`,
  `FindingTreeSurvivesPrefix`, `treeSurvivesFinding`, `DriveDoc.Finding`,
  `driveRecord.LastFinding`; the "Other emitted causes" comment; the `CauseSupervisorDied` comment)
- Modify: `internal/gatedrive/driver.go` (`sliceResult.finding`, `driveSlice(id, rec)`, the death
  branch, new `leftoverFinding`, `driveAndPersist`, `recordedDoc`, the `ProcessSeam.ProbeLeftover`
  and `proveNoTreeSurvives` comments)
- Modify: `internal/gatedrive/store.go` (`NewReservedDrive` clears `LastFinding`)
- Modify: `internal/gatedrive/reconcile.go` (`supervisorGone` uses `treeSurvivesFinding`; drop the
  `strconv` import, its only use)
- Create: `internal/gatedrive/death_leftover_test.go`

**Interfaces:**
- Consumes: `process.Leftover`, `process.LeftoverPresent/None/Unclear`, `ProcessSeam.ProbeLeftover`;
  test helpers `fakeProc` (its `leftover` closure and `leftoverN`), `fakeClock`, `newTestDriver`,
  `seedRecord`, `seedDrive`, `deathAfterFirstSlice`, `obs`, `startRun`, `stableGit`,
  `sampleStart`, `recordFileName`.
- Produces (Task 2 relies on these exact names):
  ```go
  const CauseUncertainOwnership = "uncertain-ownership"
  const FindingTreeSurvivesPrefix = "tree-survives:"
  func treeSurvivesFinding(driveID string, pgid int) string // unexported
  // DriveDoc gains:
  Finding string `json:"finding,omitempty"` // HALTED documents only
  // driveRecord gains:
  LastFinding string `json:"last_finding,omitempty"`
  ```

- [ ] **Step 1: Write the failing tests**

Create `internal/gatedrive/death_leftover_test.go`:

```go
// The death halt's leftover check (change 0497): a drive whose supervisor died
// probes the dead supervisor's process group once, and only a clean leftover
// answer stamps the informational tree-survives:<drive>:<pgid> finding. The
// finding never changes the outcome or the cause.
package gatedrive

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
)

// leftoverAnswers is every answer ProbeLeftover can give the death halt: its
// three verdicts and a probe error. The error row deliberately pairs the error
// with a LeftoverPresent answer: the halt must trust the answer only when the
// error is nil.
var leftoverAnswers = []struct {
	name    string
	answer  process.Leftover
	err     error
	finding bool
}{
	{"leftover", process.Leftover{Answer: process.LeftoverPresent, PGID: 4242}, nil, true},
	{"none", process.Leftover{Answer: process.LeftoverNone, PGID: 4242}, nil, false},
	{"unclear", process.Leftover{Answer: process.LeftoverUnclear, PGID: 4242}, nil, false},
	{"error", process.Leftover{Answer: process.LeftoverPresent, PGID: 4242}, errors.New("gatedrive-test: probe failed"), false},
}

// deathStates is every death a slice can observe, with the cause the drive must
// halt with whatever the leftover probe answers.
var deathStates = []struct {
	name      string
	state     process.State
	stopErr   bool
	wantCause string
}{
	{"signaled", process.StateSignaled, false, CauseSupervisorDied},
	{"vanished", process.StateVanished, false, CauseSupervisorDied},
	{"signaled-unproven", process.StateSignaled, true, CauseUncertainOwnership},
}

// deathProc scripts a run that is already dead in state, whose death-probe stop
// succeeds as an already-terminal no-op (or fails when stopErr), and whose
// leftover probe answers (lo, loErr) and records each probed run dir.
func deathProc(state process.State, stopErr bool, lo process.Leftover, loErr error, probed *[]string) *fakeProc {
	return &fakeProc{
		observe: func(runDir string) (*process.Observation, error) { return obs(state, runDir), nil },
		stop: func(runDir, reason string) (*process.StopOutcome, error) {
			if stopErr {
				return nil, errors.New("gatedrive-test: stop cannot prove ownership")
			}
			return &process.StopOutcome{State: state, RunDir: runDir, Performed: false,
				Terminal: &process.Terminal{Kind: "signal", Signal: 9}}, nil
		},
		leftover: func(runDir string) (process.Leftover, error) {
			*probed = append(*probed, runDir)
			return lo, loErr
		},
	}
}

// TestDeathHaltFindingNeverChangesOutcomeOrCause is the failure-posture pin
// (change 0497): for every death state and every leftover answer the drive
// HALTs with the state's own cause, probes the recorded run dir exactly once,
// and launches nothing. Only a clean leftover answer stamps
// tree-survives:<drive>:<pgid>, on the document and in the record alike.
func TestDeathHaltFindingNeverChangesOutcomeOrCause(t *testing.T) {
	for _, st := range deathStates {
		for _, ans := range leftoverAnswers {
			t.Run(st.name+"/"+ans.name, func(t *testing.T) {
				var probed []string
				proc := deathProc(st.state, st.stopErr, ans.answer, ans.err, &probed)
				d, store := newTestDriver(t, &fakeClock{now: startRun().Add(time.Second)}, proc, stableGit())
				id, ownerGen := seedDrive(t, store, seedRecord(t))

				doc, err := d.Advance(id, ownerGen)
				if err != nil {
					t.Fatalf("Advance: %v", err)
				}
				if doc.Outcome != HALTED || doc.Cause != st.wantCause {
					t.Fatalf("death = %s/%q, want HALTED/%q whatever the probe answers", doc.Outcome, doc.Cause, st.wantCause)
				}
				if len(probed) != 1 || probed[0] != "/runs/run1" {
					t.Fatalf("the death halt must probe the recorded run dir once, probed %v", probed)
				}
				if proc.launchN != 0 {
					t.Fatalf("a death must never launch, launches = %d", proc.launchN)
				}
				want := ""
				if ans.finding {
					want = "tree-survives:" + id + ":4242"
				}
				if doc.Finding != want {
					t.Fatalf("document finding = %q, want %q", doc.Finding, want)
				}
				rec, err := store.Load(id)
				if err != nil {
					t.Fatalf("Load: %v", err)
				}
				if rec.LastOutcome != HALTED || rec.LastCause != st.wantCause || rec.LastFinding != want {
					t.Fatalf("record = %s/%q/%q, want HALTED/%q/%q", rec.LastOutcome, rec.LastCause, rec.LastFinding, st.wantCause, want)
				}
			})
		}
	}
}

// TestDeathHaltFindingAfterAWaitingSlice: the realistic path — Start's first
// slice WAITs (no probe), then the supervisor dies and the next Advance halts
// with the finding naming the started drive.
func TestDeathHaltFindingAfterAWaitingSlice(t *testing.T) {
	d, _, proc, started, dead := deathAfterFirstSlice(t, sampleStart(), process.StateVanished)
	if proc.leftoverN != 0 {
		t.Fatalf("a WAITING slice must not probe for a leftover, probes = %d", proc.leftoverN)
	}
	proc.leftover = func(string) (process.Leftover, error) {
		return process.Leftover{Answer: process.LeftoverPresent, PGID: 4242}, nil
	}
	*dead = true
	doc, err := d.Advance(started.DriveID, started.Generation)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc.Outcome != HALTED || doc.Cause != CauseSupervisorDied || doc.Finding != "tree-survives:"+started.DriveID+":4242" {
		t.Fatalf("death after WAITING = %s/%q/%q, want HALTED/%s/tree-survives:%s:4242",
			doc.Outcome, doc.Cause, doc.Finding, CauseSupervisorDied, started.DriveID)
	}
}

// TestDeathHaltFindingSurvivesReRead: re-advancing the terminal drive returns
// the recorded finding and never probes again.
func TestDeathHaltFindingSurvivesReRead(t *testing.T) {
	var probed []string
	proc := deathProc(process.StateSignaled, false, process.Leftover{Answer: process.LeftoverPresent, PGID: 4242}, nil, &probed)
	d, store := newTestDriver(t, &fakeClock{now: startRun().Add(time.Second)}, proc, stableGit())
	id, ownerGen := seedDrive(t, store, seedRecord(t))
	first, err := d.Advance(id, ownerGen)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	again, err := d.Advance(id, ownerGen)
	if err != nil {
		t.Fatalf("re-Advance: %v", err)
	}
	if again.Finding != first.Finding || again.Finding != "tree-survives:"+id+":4242" || again.Cause != CauseSupervisorDied {
		t.Fatalf("re-read = %q/%q, want the recorded %q/%s", again.Cause, again.Finding, first.Finding, CauseSupervisorDied)
	}
	if len(probed) != 1 {
		t.Fatalf("a terminal drive must not probe again, probes = %d", len(probed))
	}
}

// TestDeathHaltFindingOnlyOnHalted: a record carrying last_finding beside a
// non-HALTED outcome never exposes it.
func TestDeathHaltFindingOnlyOnHalted(t *testing.T) {
	proc := &fakeProc{}
	d, store := newTestDriver(t, &fakeClock{now: startRun().Add(time.Second)}, proc, stableGit())
	rec := seedRecord(t)
	rec.LastOutcome = FAILED
	rec.LastFinding = "tree-survives:x:4242"
	id, ownerGen := seedDrive(t, store, rec)
	doc, err := d.Advance(id, ownerGen)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc.Outcome != FAILED || doc.Finding != "" {
		t.Fatalf("FAILED document = %s/%q, want FAILED with no finding", doc.Outcome, doc.Finding)
	}
}

// TestHaltedRecordWithoutLastFindingDecodesUnchanged: a HALTED record an older
// binary wrote has no last_finding key; it loads and re-reads with no finding
// and its cause unchanged, and the key is spelled last_finding when present.
func TestHaltedRecordWithoutLastFindingDecodesUnchanged(t *testing.T) {
	proc := &fakeProc{}
	d, store := newTestDriver(t, &fakeClock{now: startRun().Add(time.Second)}, proc, stableGit())
	rec := seedRecord(t)
	rec.LastOutcome = HALTED
	rec.LastCause = CauseSupervisorDied
	id, ownerGen := seedDrive(t, store, rec)

	path := filepath.Join(store.root, id, recordFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	record := env["record"].(map[string]any)
	if _, ok := record["last_finding"]; ok {
		t.Fatalf("an empty finding must be omitted from the record: %v", record)
	}
	doc, err := d.Advance(id, ownerGen)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc.Outcome != HALTED || doc.Cause != CauseSupervisorDied || doc.Finding != "" || proc.leftoverN != 0 {
		t.Fatalf("old halted record = %s/%q/%q probes %d, want HALTED/%s, no finding, no probe",
			doc.Outcome, doc.Cause, doc.Finding, proc.leftoverN, CauseSupervisorDied)
	}

	// The persisted key is last_finding: a record carrying it re-reads it.
	record["last_finding"] = "tree-survives:" + id + ":77"
	if raw, err = json.Marshal(env); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	doc, err = d.Advance(id, ownerGen)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc.Finding != "tree-survives:"+id+":77" {
		t.Fatalf("last_finding must decode into the document, got %q", doc.Finding)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -count=1 -run 'TestDeathHalt|TestHaltedRecordWithoutLastFinding' ./internal/gatedrive/`
Expected: FAIL to compile — `undefined: CauseUncertainOwnership`, `doc.Finding undefined`,
`rec.LastFinding undefined`.

- [ ] **Step 3: Add the identities and fields in `internal/gatedrive/drive.go`**

Add `"strconv"` to the import block (`import ( "strconv"; "time" )`).

In the cause-constant block's lead comment, the sentence "Other emitted causes (owner-superseded,
fingerprint-error, uncertain-ownership, …) are not distinguished by any consumer" becomes "Other
emitted causes (owner-superseded, fingerprint-error, …) are not distinguished by any consumer".

In the `CauseSupervisorDied` comment, replace "It does not rule out a surviving process group: a
dead supervisor's suite can outlive it, which is the census's tree-survives finding (change 0492)."
with "It does not rule out a surviving process group: a dead supervisor's suite can outlive it,
which the census (change 0492) and the death HALT itself (change 0497) report as the
tree-survives finding."

Append to the same `const ( … )` block, after `CauseSupervisorDied`:

```go
	// CauseUncertainOwnership: the drive's run died without a verdict but the
	// supervisor could not be proven gone (the death probe's stop failed). Like
	// CauseSupervisorDied it HALTs and never relaunches. Finalize names it in its
	// halt message (change 0497), so it has an exported identity.
	CauseUncertainOwnership = "uncertain-ownership"
```

After the `const ( … )` block add:

```go
// FindingTreeSurvivesPrefix starts the informational tree-survives:<drive>:<pgid>
// finding: a dead supervisor's process group still has members
// (process.LeftoverPresent), so part of the suite is still running. The launch
// census reports it (change 0492) and a death HALT carries it (change 0497);
// neither signals the group or changes an outcome. A consumer keys on this
// constant, never on the literal spelling.
const FindingTreeSurvivesPrefix = "tree-survives:"

// treeSurvivesFinding is the one spelling of the finding both reporters emit.
func treeSurvivesFinding(driveID string, pgid int) string {
	return FindingTreeSurvivesPrefix + driveID + ":" + strconv.Itoa(pgid)
}
```

In `DriveDoc`, after the `RunRoot` field add:

```go
	// Finding is the informational tree-survives:<drive>:<pgid> finding a death
	// HALT stamps when the dead supervisor's process group still has members
	// (change 0497). It is set on a HALTED document only (omitempty) and never
	// changes Outcome or Cause. Like RunRoot it carries no credential.
	Finding string `json:"finding,omitempty"`
```

In `driveRecord`, directly after `LastCause` add:

```go
	// LastFinding is the informational finding persisted with a death HALT
	// (tree-survives:<drive>:<pgid>, change 0497); empty otherwise. A record an
	// older binary wrote has no last_finding key and decodes with it empty, so
	// there is no schema bump.
	LastFinding string `json:"last_finding,omitempty"`
```

- [ ] **Step 4: Probe at the death halt in `internal/gatedrive/driver.go`**

In `ProcessSeam`, the `ProbeLeftover` comment's last sentence "The launch census reports a
leftover as tree-survives and never signals on it; an unclear answer or an error is today's
behavior." becomes "The launch census and the death HALT report a leftover as tree-survives and
never signal on it; an unclear answer or an error is today's behavior."

`sliceResult` gains a field:

```go
type sliceResult struct {
	outcome   Outcome
	cause     string
	rawRunDir string // PASSED only
	finding   string // a death HALT's tree-survives finding only (change 0497)

	lastClock time.Time
}
```

`driveAndPersist`: change `res := d.driveSlice(rec)` to `res := d.driveSlice(id, rec)`, and in its
CAS closure, after `r.LastCause = res.cause`, add `r.LastFinding = res.finding`.

`driveSlice`: change the signature to `func (d *Driver) driveSlice(id string, rec driveRecord)
sliceResult` and add to its doc comment, after "rec is read-only;": "id names the drive in a death
HALT's finding;". Replace the whole `case process.StateSignaled, process.StateVanished:` arm with:

```go
		case process.StateSignaled, process.StateVanished:
			// A death without a verdict. A gate drive never relaunches (change
			// 0493): a proven death HALTs supervisor-died and a human re-runs the
			// workflow; a death whose tree cannot be proven gone HALTs
			// uncertain-ownership.
			cause := CauseSupervisorDied
			if gone, derr := d.proveNoTreeSurvives(runDir, observation); derr != nil || !gone {
				cause = CauseUncertainOwnership
			}
			// The cause is chosen. The leftover check only adds information: it
			// never changes the outcome or the cause (change 0497).
			res.finding = d.leftoverFinding(id, runDir)
			return halt(&res, cause)
```

Add, directly after `proveNoTreeSurvives`:

```go
// leftoverFinding asks, read-only, whether the dead supervisor's process group
// still has members, and returns the tree-survives:<drive>:<pgid> finding only on
// a clean leftover answer. None, unclear, and a probe error (whatever answer it
// carries) return "": a probe that cannot prove a leftover never prints a wait
// instruction (change 0497). It never signals.
func (d *Driver) leftoverFinding(id, runDir string) string {
	lo, err := d.proc.ProbeLeftover(runDir)
	if err != nil || lo.Answer != process.LeftoverPresent {
		return ""
	}
	return treeSurvivesFinding(id, lo.PGID)
}
```

In the `proveNoTreeSurvives` doc comment, replace "and that is the launch census's tree-survives
finding (change 0492)." with "and that is the tree-survives finding the census (change 0492) and
the death HALT's leftoverFinding (change 0497) report."

In `recordedDoc`, after the `if rec.LastOutcome == PASSED { … }` block add:

```go
	if rec.LastOutcome == HALTED {
		doc.Finding = rec.LastFinding
	}
```

- [ ] **Step 5: Clear it on a reserved drive and share the spelling with the census**

In `internal/gatedrive/store.go` `NewReservedDrive`, after `rec.LastCause = ""` add
`rec.LastFinding = ""`.

In `internal/gatedrive/reconcile.go` `supervisorGone`, change
`return true, "tree-survives:" + id + ":" + strconv.Itoa(lo.PGID)` to
`return true, treeSurvivesFinding(id, lo.PGID)`, then delete `"strconv"` from the import block
(it was its only use; `go vet` confirms).

- [ ] **Step 6: Run the tests to verify they pass**

Run:

```bash
gofmt -l internal/gatedrive/
go vet ./internal/gatedrive/
go test -count=1 -run 'TestDeathHalt|TestHaltedRecordWithoutLastFinding|TestDeathHaltsSupervisorDied|TestDeathUnprovenHaltsUncertainOwnership|TestVanishedProvenGoneWithoutStop|TestSignaledDeathConsumesTerminal|TestCensus|TestDriveDoc|TestNonPassedDocOmitsRawRunDir|TestDriveSchema' ./internal/gatedrive/
go test -count=1 ./internal/gatedrive/
```

Expected: `gofmt -l` and `go vet` print nothing; both test runs PASS (the existing death and census
tests keep their causes and findings — `fakeProc`'s default leftover answer is `none`).

- [ ] **Step 7: Mutation-test the two failure-posture properties**

Mutation 1 — remove the probe call:

```bash
f=internal/gatedrive/driver.go
cp "$f" "$f.bak"
grep -c 'res.finding = d.leftoverFinding(id, runDir)' "$f"    # expect 1
perl -ni -e 'print unless /res\.finding = d\.leftoverFinding\(id, runDir\)/' "$f"
grep -c 'res.finding = d.leftoverFinding(id, runDir)' "$f"    # expect 0
go test -count=1 -run 'TestDeathHaltFindingNeverChangesOutcomeOrCause' ./internal/gatedrive/
mv -f "$f.bak" "$f"
```

Expected: FAIL (every row: "the death halt must probe the recorded run dir once"; the `leftover`
rows also on the finding).

Mutation 2 — let a finding alter the cause:

```bash
f=internal/gatedrive/driver.go
cp "$f" "$f.bak"
grep -c 'if res.finding != "" { cause = CauseUncertainOwnership }' "$f"   # expect 0
perl -pi -e 's/^(\s*)(res\.finding = d\.leftoverFinding\(id, runDir\))$/$1$2\n$1if res.finding != "" { cause = CauseUncertainOwnership }/' "$f"
grep -c 'if res.finding != "" { cause = CauseUncertainOwnership }' "$f"   # expect 1
go test -count=1 -run 'TestDeathHaltFindingNeverChangesOutcomeOrCause' ./internal/gatedrive/
mv -f "$f.bak" "$f"
```

Expected: FAIL on `signaled/leftover` and `vanished/leftover` ("want HALTED/\"supervisor-died\"
whatever the probe answers").

Mutation 3 — trust the answer despite an error:

```bash
f=internal/gatedrive/driver.go
cp "$f" "$f.bak"
grep -c 'if err != nil || lo.Answer != process.LeftoverPresent {' "$f"   # expect 1
perl -pi -e 's/if err != nil \|\| lo\.Answer != process\.LeftoverPresent \{/if lo.Answer != process.LeftoverPresent {/' "$f"
grep -c 'if err != nil || lo.Answer != process.LeftoverPresent {' "$f"   # expect 0
go test -count=1 -run 'TestDeathHaltFindingNeverChangesOutcomeOrCause' ./internal/gatedrive/
mv -f "$f.bak" "$f"
```

Expected: FAIL on the three `…/error` rows (finding stamped from an errored probe).

Then re-run `go test -count=1 -run 'TestDeathHalt' ./internal/gatedrive/` — PASS, and
`git diff --stat` shows no `.bak` file.

- [ ] **Step 8: Commit**

```bash
git add internal/gatedrive/drive.go internal/gatedrive/driver.go internal/gatedrive/store.go internal/gatedrive/reconcile.go internal/gatedrive/death_leftover_test.go
git commit -m "feat(0497): a supervisor-death halt reports a leftover suite as tree-survives" -m "<mutation readings: M1 remove probe -> red; M2 finding alters cause -> red; M3 error trusted -> red>"
```

(Replace the second `-m` with the actual readings, naming each failing subtest.)

---

### Task 2: Finalize reports the leftover; gate-drive human text prints it

**Build tier:** standard

**Files:**
- Modify: `internal/app/gate_drive.go` (`GateDriveResult.HumanText`)
- Modify: `internal/app/finalize_rebase.go` (`LocalGateResult` fields and the `TeardownFinding`
  comment; `GateReport.TeardownFinding` comment; `mapDriveOutcome`; `mapTerminalDrive`; the
  `FinalizeGateHalted` branch of the gate composition; new `finalizeGateHaltGeneric`,
  `finalizeGateHaltMessage`, `leftoverPGID`; the `mapDriveHaltCause` comment)
- Modify: `internal/app/gate_drive_test.go` (unit tests, default build)
- Modify: `internal/app/finalize_rebase_ops_integration_test.go` (integration build)

**Interfaces:**
- Consumes (Task 1): `gatedrive.CauseSupervisorDied`, `gatedrive.CauseUncertainOwnership`,
  `gatedrive.FindingTreeSurvivesPrefix`, `gatedrive.DriveDoc.Finding`. Test helpers:
  `processFinalizeGate`, `runRootFixture`, `dirExists`, `runRootObserver`, `fakeGate`,
  `setupRebaseFixture`, `planRepoModes`, `fakeRebaseGitHub`, `retargetRepo`, `FinalizeRebase`.
- Produces:
  ```go
  // LocalGateResult gains:
  HaltDriveCause string // the driver's HALTED cause token
  HaltLeftover   string // the driver document's tree-survives finding
  const finalizeGateHaltGeneric = "the local gate did not reach a decidable pass/fail; retained, no red fabricated"
  func finalizeGateHaltMessage(gres LocalGateResult) string
  func leftoverPGID(finding string) (string, bool)
  ```

- [ ] **Step 1: Write the failing unit tests**

Append to `internal/app/gate_drive_test.go`:

```go
// --- change 0497: a supervisor-death halt's leftover finding ---------------

const testLeftover = "tree-survives:0123456789abcdef0123456789abcdef:4242"

const testWait = "part of the suite is still running as process group 4242 — wait until pgrep -lg 4242 prints nothing, then re-run finalize"

// TestFinalizeGateHaltMessage pins finalize.rebase's halt message: a supervisor
// death says so, a leftover adds the wait clause, a typed refusal keeps its own
// message, and every other halt keeps the generic message byte for byte. A
// malformed finding never produces a garbled wait clause.
func TestFinalizeGateHaltMessage(t *testing.T) {
	const died = "the local gate's supervisor died before the suite finished; "
	for _, tc := range []struct {
		name string
		gres LocalGateResult
		want string
	}{
		{"died", LocalGateResult{HaltDriveCause: gatedrive.CauseSupervisorDied},
			died + "re-run finalize to re-run the suite"},
		{"died-leftover", LocalGateResult{HaltDriveCause: gatedrive.CauseSupervisorDied, HaltLeftover: testLeftover},
			died + testWait},
		{"uncertain-leftover", LocalGateResult{HaltDriveCause: gatedrive.CauseUncertainOwnership, HaltLeftover: testLeftover},
			finalizeGateHaltGeneric + "; " + testWait},
		{"uncertain", LocalGateResult{HaltDriveCause: gatedrive.CauseUncertainOwnership}, finalizeGateHaltGeneric},
		{"deadline-leftover", LocalGateResult{HaltDriveCause: gatedrive.CauseDeadlineExpired, HaltLeftover: testLeftover}, finalizeGateHaltGeneric},
		{"unknown-observation", LocalGateResult{HaltDriveCause: gatedrive.CauseUnknownObservation}, finalizeGateHaltGeneric},
		{"no-drive", LocalGateResult{}, finalizeGateHaltGeneric},
		{"refusal-wins", LocalGateResult{HaltReason: "worktree-busy", HaltMessage: "wait for it",
			HaltDriveCause: gatedrive.CauseSupervisorDied, HaltLeftover: testLeftover},
			"the local gate could not start: worktree-busy — wait for it"},
		{"refusal-no-message", LocalGateResult{HaltReason: "worktree-busy"}, "the local gate could not start: worktree-busy"},
		{"malformed-empty-pgid", LocalGateResult{HaltDriveCause: gatedrive.CauseSupervisorDied, HaltLeftover: "tree-survives:d1:"},
			died + "re-run finalize to re-run the suite"},
		{"malformed-no-drive", LocalGateResult{HaltDriveCause: gatedrive.CauseSupervisorDied, HaltLeftover: "tree-survives::4242"},
			died + "re-run finalize to re-run the suite"},
		{"malformed-pgid-1", LocalGateResult{HaltDriveCause: gatedrive.CauseSupervisorDied, HaltLeftover: "tree-survives:d1:1"},
			died + "re-run finalize to re-run the suite"},
		{"malformed-not-numeric", LocalGateResult{HaltDriveCause: gatedrive.CauseSupervisorDied, HaltLeftover: "tree-survives:d1:abc"},
			died + "re-run finalize to re-run the suite"},
		{"other-finding", LocalGateResult{HaltDriveCause: gatedrive.CauseSupervisorDied, HaltLeftover: "run-terminal:d1"},
			died + "re-run finalize to re-run the suite"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := finalizeGateHaltMessage(tc.gres); got != tc.want {
				t.Fatalf("message = %q\nwant      %q", got, tc.want)
			}
		})
	}
	if finalizeGateHaltGeneric != "the local gate did not reach a decidable pass/fail; retained, no red fabricated" {
		t.Fatalf("the generic halt message changed: %q", finalizeGateHaltGeneric)
	}
}

// TestMapDriveOutcomeCarriesLeftoverFinding: a HALTED drive document's cause and
// finding travel up as HaltDriveCause/HaltLeftover, the finding fills
// TeardownFinding unless a retention token claims it, and the outcome and halt
// cause are the same with or without a finding.
func TestMapDriveOutcomeCarriesLeftoverFinding(t *testing.T) {
	for _, cause := range []string{gatedrive.CauseSupervisorDied, gatedrive.CauseUncertainOwnership} {
		for _, finding := range []string{"", testLeftover} {
			t.Run(cause+"/"+finding, func(t *testing.T) {
				g := &processFinalizeGate{}
				root := runRootFixture(t)
				doc := gatedrive.DriveDoc{Outcome: gatedrive.HALTED, Cause: cause, Finding: finding, RunRoot: root}
				res := g.mapDriveOutcome(context.Background(), LocalGateRequest{}, GateDriveResult{Drive: &doc})
				if res.Outcome != FinalizeGateHalted || res.HaltCause != GateHaltUnavailable {
					t.Fatalf("outcome/cause = %s/%s, want %s/%s whatever the finding", res.Outcome, res.HaltCause, FinalizeGateHalted, GateHaltUnavailable)
				}
				if res.HaltDriveCause != cause || res.HaltLeftover != finding || res.TeardownFinding != finding {
					t.Fatalf("drive cause/leftover/teardown = %q/%q/%q, want %q/%q/%q",
						res.HaltDriveCause, res.HaltLeftover, res.TeardownFinding, cause, finding, finding)
				}
				if dirExists(t, root) {
					t.Fatalf("an exited halted run root must still be removed")
				}
			})
		}
	}
	t.Run("retained", func(t *testing.T) {
		root := runRootFixture(t)
		runDir := filepath.Join(root, "0123456789abcdef0123456789abcdef")
		if err := os.MkdirAll(runDir, 0o700); err != nil {
			t.Fatal(err)
		}
		g := &processFinalizeGate{observer: runRootObserver{runDir: process.StateRunning}}
		doc := gatedrive.DriveDoc{Outcome: gatedrive.HALTED, Cause: gatedrive.CauseSupervisorDied, Finding: testLeftover, RunRoot: root}
		res := g.mapDriveOutcome(context.Background(), LocalGateRequest{}, GateDriveResult{Drive: &doc})
		if res.TeardownFinding != teardownFindingRunRootRetainedUnsettled || res.HaltLeftover != testLeftover {
			t.Fatalf("teardown/leftover = %q/%q, want the retention token and the leftover kept for the message",
				res.TeardownFinding, res.HaltLeftover)
		}
		if got := finalizeGateHaltMessage(res); !strings.HasSuffix(got, testWait) {
			t.Fatalf("a retained root must not drop the wait clause: %q", got)
		}
	})
	t.Run("failed", func(t *testing.T) {
		g := &processFinalizeGate{}
		doc := gatedrive.DriveDoc{Outcome: gatedrive.FAILED, RunRoot: runRootFixture(t)}
		res := g.mapDriveOutcome(context.Background(), LocalGateRequest{}, GateDriveResult{Drive: &doc})
		if res.HaltDriveCause != "" || res.HaltLeftover != "" || res.TeardownFinding != "" {
			t.Fatalf("a FAILED drive grew halt detail: %+v", res)
		}
	})
}

// TestGateDriveHumanTextPrintsFinding: the gate-drive human text prints a
// finding: line exactly when the document carries one.
func TestGateDriveHumanTextPrintsFinding(t *testing.T) {
	with := GateDriveResult{Drive: &gatedrive.DriveDoc{Outcome: gatedrive.HALTED, Cause: gatedrive.CauseSupervisorDied, Finding: testLeftover}}
	if got := with.HumanText(); !strings.Contains(got, "\nfinding: "+testLeftover) {
		t.Fatalf("human text lacks the finding line: %q", got)
	}
	without := GateDriveResult{Drive: &gatedrive.DriveDoc{Outcome: gatedrive.HALTED, Cause: gatedrive.CauseSupervisorDied}}
	if got := without.HumanText(); strings.Contains(got, "finding:") {
		t.Fatalf("human text printed a finding line with no finding: %q", got)
	}
}
```

- [ ] **Step 2: Write the failing integration test**

In `internal/app/finalize_rebase_ops_integration_test.go`, add
`"github.com/danielhanold/docket/internal/gatedrive"` to the imports, and add after
`TestIntegrationFinalizeRebaseOpsFinalizeRebaseGateHaltGenericUnchanged`:

```go
// TestIntegrationFinalizeRebaseOpsFinalizeRebaseGateHaltLeftoverMessage (change
// 0497): a halted local gate whose supervisor died says so, and a leftover suite
// adds the wait-for-the-group clause and rides teardown_finding — while the
// result, reason, disposition, and halt cause stay those of any drive halt.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseGateHaltLeftoverMessage(t *testing.T) {
	const finding = "tree-survives:0123456789abcdef0123456789abcdef:4242"
	const wait = "part of the suite is still running as process group 4242 — wait until pgrep -lg 4242 prints nothing, then re-run finalize"
	for _, tc := range []struct {
		name, cause, leftover, wantMsg string
	}{
		{"supervisor-died", gatedrive.CauseSupervisorDied, "",
			"the local gate's supervisor died before the suite finished; re-run finalize to re-run the suite"},
		{"supervisor-died-leftover", gatedrive.CauseSupervisorDied, finding,
			"the local gate's supervisor died before the suite finished; " + wait},
		{"uncertain-ownership-leftover", gatedrive.CauseUncertainOwnership, finding,
			"the local gate did not reach a decidable pass/fail; retained, no red fabricated; " + wait},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupRebaseFixture(t, planRepoModes()[0])
			gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
			gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGateHalted, HaltCause: GateHaltUnavailable,
				HaltDriveCause: tc.cause, HaltLeftover: tc.leftover, TeardownFinding: tc.leftover}}
			res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
				FinalizeRebaseRequest{ID: f.id, Revision: f.revision, Head: f.head})
			if res.Result != ResultBlocked || res.Reason != ReasonRebaseGateHalted || res.Disposition != RebaseDispBlocked {
				t.Fatalf("result/reason/disposition = %q/%q/%q, want blocked/%q/%q",
					res.Result, res.Reason, res.Disposition, ReasonRebaseGateHalted, RebaseDispBlocked)
			}
			if res.Gate == nil || res.Gate.Outcome != string(FinalizeGateHalted) || res.Gate.HaltCause != GateHaltUnavailable {
				t.Fatalf("gate report = %+v, want a halted/unavailable gate", res.Gate)
			}
			if res.Message != tc.wantMsg {
				t.Fatalf("message = %q\nwant      %q", res.Message, tc.wantMsg)
			}
			if res.Gate.TeardownFinding != tc.leftover {
				t.Fatalf("teardown_finding = %q, want %q", res.Gate.TeardownFinding, tc.leftover)
			}
			if res.Gate.Reason != "" || res.Gate.Message != "" || res.Gate.Stage != "" || res.Gate.Locator != "" {
				t.Fatalf("a drive halt grew refusal detail: %+v", res.Gate)
			}
		})
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run:

```bash
go test -count=1 -run 'TestFinalizeGateHaltMessage|TestMapDriveOutcomeCarriesLeftoverFinding|TestGateDriveHumanTextPrintsFinding' ./internal/app/
go vet -tags integration ./internal/app/
```

Expected: FAIL to compile — `undefined: finalizeGateHaltMessage`, `undefined:
finalizeGateHaltGeneric`, `unknown field HaltDriveCause`, `unknown field HaltLeftover`.

- [ ] **Step 4: Print the finding in `GateDriveResult.HumanText` (`internal/app/gate_drive.go`)**

Directly after the `if r.Drive.Cause != "" { … }` block add:

```go
		if r.Drive.Finding != "" {
			lines = append(lines, "finding: "+r.Drive.Finding)
		}
```

- [ ] **Step 5: Carry the cause and finding through the finalize gate (`internal/app/finalize_rebase.go`)**

In `LocalGateResult`, replace the `TeardownFinding` comment and add the two fields after
`HaltLocator`:

```go
	HaltLocator string
	// HaltDriveCause is the driver's own HALTED cause token behind the coarse
	// HaltCause (e.g. gatedrive.CauseSupervisorDied), and HaltLeftover the
	// driver document's tree-survives:<drive>:<pgid> finding (change 0497). Both
	// are set on a Halted outcome mapped from a drive document only. They choose
	// the halt message and never change Outcome or HaltCause.
	HaltDriveCause string
	HaltLeftover   string
```

and the `TeardownFinding` comment becomes:

```go
	// TeardownFinding is a bounded, credential-free token. It names why the
	// gate's private run root was RETAINED when its teardown evidence was not
	// settled (a terminal document carried no run root, or a failed Start left
	// launch evidence under the root); otherwise, for a halted drive whose dead
	// supervisor's process group still has members, it is that drive's
	// tree-survives finding (HaltLeftover, change 0497). It never changes Outcome.
	TeardownFinding string
```

In `GateReport`, the `TeardownFinding` comment becomes "TeardownFinding mirrors
LocalGateResult.TeardownFinding: a run-root retention token, or a halted drive's tree-survives
finding (change 0497)."

In `mapTerminalDrive`, the `default:` arm becomes:

```go
	default: // gatedrive.HALTED or an unrecognized outcome — fail closed, never red.
		return LocalGateResult{Outcome: FinalizeGateHalted, HaltCause: mapDriveHaltCause(doc.Cause),
			HaltDriveCause: doc.Cause, HaltLeftover: doc.Finding}
```

In `mapDriveOutcome`, replace the final `res.TeardownFinding = teardownFinding` with:

```go
	res.TeardownFinding = teardownFinding
	if res.TeardownFinding == "" {
		// Nothing was retained, so a halted drive's leftover finding is the
		// teardown news (change 0497). A retention token wins when both apply; the
		// leftover still reaches the halt message through HaltLeftover.
		res.TeardownFinding = res.HaltLeftover
	}
```

In the gate composition's `default: // FinalizeGateHalted` branch, replace these lines:

```go
		base.Reason = ReasonRebaseGateHalted
		base.Message = "the local gate did not reach a decidable pass/fail; retained, no red fabricated"
		// Upgrade the coarse message ONLY when a typed refusal supplied detail; a
		// genuinely detail-less halt keeps today's exact generic message.
		if gres.HaltReason != "" {
			base.Message = "the local gate could not start: " + gres.HaltReason
			if gres.HaltMessage != "" {
				base.Message += " — " + gres.HaltMessage
			}
		}
```

with:

```go
		base.Reason = ReasonRebaseGateHalted
		base.Message = finalizeGateHaltMessage(gres)
```

Add, directly before `mapDriveHaltCause`:

```go
// finalizeGateHaltGeneric is finalize.rebase's message for a halted local gate
// with nothing more specific to say.
const finalizeGateHaltGeneric = "the local gate did not reach a decidable pass/fail; retained, no red fabricated"

// finalizeGateHaltMessage composes finalize.rebase's message for a halted local
// gate. A typed refusal (HaltReason) keeps its "could not start" message. A drive
// whose supervisor died says so, and when the driver found part of the suite
// still running it tells the human to wait for that process group before
// re-running finalize (change 0497). Every other halt keeps the generic message
// byte for byte. Only the message varies: the caller sets the disposition,
// reason, result, and halt cause the same way for every halt.
func finalizeGateHaltMessage(gres LocalGateResult) string {
	if gres.HaltReason != "" {
		msg := "the local gate could not start: " + gres.HaltReason
		if gres.HaltMessage != "" {
			msg += " — " + gres.HaltMessage
		}
		return msg
	}
	pgid, leftover := leftoverPGID(gres.HaltLeftover)
	wait := "part of the suite is still running as process group " + pgid +
		" — wait until pgrep -lg " + pgid + " prints nothing, then re-run finalize"
	switch {
	case gres.HaltDriveCause == gatedrive.CauseSupervisorDied && leftover:
		return "the local gate's supervisor died before the suite finished; " + wait
	case gres.HaltDriveCause == gatedrive.CauseSupervisorDied:
		return "the local gate's supervisor died before the suite finished; re-run finalize to re-run the suite"
	case gres.HaltDriveCause == gatedrive.CauseUncertainOwnership && leftover:
		return finalizeGateHaltGeneric + "; " + wait
	default:
		return finalizeGateHaltGeneric
	}
}

// leftoverPGID extracts the process group from a tree-survives:<drive>:<pgid>
// finding. Anything else — empty, another finding, a missing drive, or a group
// id that is not a real group (<= 1) — reads as no leftover, so a malformed
// finding falls back to the message without the wait clause, never a garbled one.
func leftoverPGID(finding string) (string, bool) {
	rest, ok := strings.CutPrefix(finding, gatedrive.FindingTreeSurvivesPrefix)
	if !ok {
		return "", false
	}
	drive, pgid, ok := strings.Cut(rest, ":")
	if !ok || drive == "" {
		return "", false
	}
	n, err := strconv.Atoi(pgid)
	if err != nil || n <= 1 {
		return "", false
	}
	return strconv.Itoa(n), true
}
```

In the `mapDriveHaltCause` doc comment, after "is reported as unavailable — a human is needed, never
repair work." add "finalizeGateHaltMessage, not this mapping, distinguishes a supervisor death in
the message (change 0497)."

- [ ] **Step 6: Run the tests to verify they pass**

Run:

```bash
gofmt -l internal/app/
go vet ./internal/app/ && go vet -tags integration ./internal/app/
go test -count=1 -run 'TestFinalizeGateHaltMessage|TestMapDriveOutcome|TestGateDriveHumanText|TestMapDriveHaltCause|TestFinalizeCleanupReportsWithheldRunRoot|TestFailedStartRetainsRoot' ./internal/app/
go test -tags integration -count=1 -run '^TestIntegrationFinalizeRebaseOpsFinalizeRebaseGateHalt' ./internal/app/
```

Expected: `gofmt -l` and `go vet` print nothing; both test runs PASS, including the unchanged
`TestIntegrationFinalizeRebaseOpsFinalizeRebaseGateHaltGenericUnchanged` and
`…GateHaltCarriesAdmissionRefusal`.

- [ ] **Step 7: Mutation-test the teardown fallback and the message key**

Mutation 4 — drop the leftover fallback into `TeardownFinding`:

```bash
f=internal/app/finalize_rebase.go
cp "$f" "$f.bak"
grep -c 'res.TeardownFinding = res.HaltLeftover' "$f"    # expect 1
perl -ni -e 'print unless /res\.TeardownFinding = res\.HaltLeftover/' "$f"
grep -c 'res.TeardownFinding = res.HaltLeftover' "$f"    # expect 0
go test -count=1 -run 'TestMapDriveOutcomeCarriesLeftoverFinding' ./internal/app/
mv -f "$f.bak" "$f"
```

Expected: FAIL on the two `…/tree-survives:…` rows.

Mutation 5 — let a finding change the halt cause:

```bash
f=internal/app/finalize_rebase.go
cp "$f" "$f.bak"
grep -c 'if res.HaltLeftover != "" { res.HaltCause = GateHaltMalformed }' "$f"   # expect 0
perl -pi -e 's/^(\s*)(res\.TeardownFinding = teardownFinding)$/$1if res.HaltLeftover != "" { res.HaltCause = GateHaltMalformed }\n$1$2/' "$f"
grep -c 'if res.HaltLeftover != "" { res.HaltCause = GateHaltMalformed }' "$f"   # expect 1
go test -count=1 -run 'TestMapDriveOutcomeCarriesLeftoverFinding' ./internal/app/
mv -f "$f.bak" "$f"
```

Expected: FAIL on the two `…/tree-survives:…` rows ("want halted/unavailable whatever the
finding").

Re-run Step 6's two test commands — PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/app/gate_drive.go internal/app/finalize_rebase.go internal/app/gate_drive_test.go internal/app/finalize_rebase_ops_integration_test.go
git commit -m "feat(0497): finalize names a supervisor death and a leftover suite in its halt message" -m "<mutation readings: M4 teardown fallback removed -> red; M5 finding alters halt cause -> red>"
```

---

### Task 3: Skills, glossary, and mirror relay the finding

**Build tier:** standard

**Files:**
- Modify: `docs/reference/glossary.md` (the `tree-survives` entry)
- Modify: `skills/docket-finalize-change/references/gate-failure.md`
- Modify: `skills/docket-build/references/gate-caller-loop.md`
- Modify: `skills/docket-implement-next/SKILL.md` (Step 5)
- Modify: `internal/repoguard/budgets_test.go` (three budget rows)
- Regenerate: `internal/assets/embedded/manifest.json` and
  `internal/assets/embedded/tree/skills/…` mirrors of the three skill files

**Interfaces:**
- Consumes: the `finding` / `teardown_finding` / message behavior of Tasks 1–2.
- Produces: nothing executable.

- [ ] **Step 1: Re-derive the prose sites**

Run:

```bash
git grep -n -e "tree-survives" -e "uncertain-ownership" -e "supervisor-died" -e "teardown_finding" -- . \
  ':!docs/superpowers' ':!docs/results' ':!docs/changes' ':!docs/adrs' ':!internal/assets/embedded' ':!*_test.go'
```

Sort each hit into executable or prose. At plan time the maintained prose sites that describe who
reports `tree-survives` or what a supervisor-death halt tells the human were: the glossary entry
"### Cancel finding `tree-survives`", `gate-failure.md`'s sentence "A gate whose own supervisor
dies mid-run is never relaunched: …", and `gate-caller-loop.md`'s disposition bullets.
`docs/concepts/run-tracker.md` mentions `tree-survives` only as a cancel finding and does not list
the reporters, so it stays as it is (spec: "update its `tree-survives` mention only if it lists the
reporters"). The glossary's "Drive disposition" entry and `docket-build/SKILL.md`'s
"halts `supervisor-died`, never relaunched." stay too. Code comments were updated in Tasks 1–2. If
the grep finds another maintained site that lists the reporters, update it in the same register
and name it in the commit body. No page links the old heading anchor (`git grep -n
"cancel-finding-tree-survives"` prints nothing); re-run that grep and fix any hit.

- [ ] **Step 2: Rewrite the glossary entry**

In `docs/reference/glossary.md`, replace the whole entry from `### Cancel finding `tree-survives``
through its last paragraph (ending "the finding is the ordinary `run-terminal:<drive>`.") with:

````markdown
### Leftover-suite finding `tree-survives`

`tree-survives:<drive>:<pgid>` is an informational finding from `run.cancel`, the death guardian,
`run.verdict`'s success closeout, and a halted gate (build or finalize) whose supervisor died. The
drive's supervisor has exited (killed alone or crashed), but its process group `<pgid>` still has
members, so part of the suite (usually `go run` and the test runner) is still running. Cancel still
reports `cancelled`, the closeout verdict is unchanged, a halted gate keeps its halt and cause, and
the leftover suite finishes on its own. It is information, never a blocker: no skill or reviewer
escalates it into one.

A halted gate carries it in the drive document's `finding` and, for finalize, in the gate report's
`teardown_finding`; finalize's halt message names the process group. Before re-running the
workflow, wait until `pgrep -lg <pgid>` prints nothing, or the new gate starts a second suite
beside the leftover one.

Docket never signals the group: with the supervisor dead, nothing proves the group is still the
run's own. To stop it yourself, confirm its members first, then signal the group:

```sh
pgrep -lg <pgid>
kill -TERM -<pgid>
```

The check sees only the supervisor's own group, not test targets that lead their own groups. When
it cannot tell (for example, the dead supervisor is still an unreaped zombie), cancel and the
closeout report the ordinary `run-terminal:<drive>`, and a halted gate reports no finding.
````

- [ ] **Step 3: Add the finalize sentence**

In `skills/docket-finalize-change/references/gate-failure.md`, the paragraph under "## The finalize
gate shares the worktree's one lock" ends "…and the remedy is to re-run finalize, which re-runs
the suite." Append, in the same paragraph (re-wrap to the file's ~100-column width):

```markdown
When the halt message says part of the suite is still running as a process group, wait until
`pgrep -lg <pgid>` prints nothing before re-running finalize; that `tree-survives` finding is
information only and never a blocker.
```

- [ ] **Step 4: Add the build relay sentence**

In `skills/docket-build/references/gate-caller-loop.md`, under "## The disposition vocabulary and
what each earns", after the bullet "- **Only `PASSED` exposes the raw run dir**, so only a trusted
pass can feed the evidence operation." add:

```markdown
- **A `HALTED` document's `finding`, when present, is copied verbatim into the halt report** with
  the glossary's `tree-survives` remedy (wait until `pgrep -lg <pgid>` prints nothing before
  re-running); it is information only — it never changes the halt, never adds one, and never
  feeds repair.
```

- [ ] **Step 5: Keep the finding when the halt becomes `## Run halted`**

The build controller's halt report reaches the human only through implement-next's `change.halt`
report (Step 3's write, used "for any hard error that ends the run halted"); nothing there tells
the author to keep gate evidence verbatim, so a summarized report could drop the finding. In
`skills/docket-implement-next/SKILL.md`, at the end of the `### Step 5 — Build` paragraph (after
"…leaving the change `in-progress` with `claimed_at` refreshed and the halt reason recorded."),
append on the same line:

```markdown
 When the build role returns `halted`, the `## Run halted` report carries its halt report's gate evidence verbatim, a `tree-survives` finding included — information for the human, never a different disposition.
```

- [ ] **Step 6: Re-baseline the three budget rows and regenerate the mirror**

Measure:

```bash
for f in skills/docket-build/references/gate-caller-loop.md skills/docket-finalize-change/references/gate-failure.md skills/docket-implement-next/SKILL.md; do
  printf '%s %s lines %s words\n' "$f" "$(wc -l <"$f" | tr -d ' ')" "$(wc -w <"$f" | tr -d ' ')"
done
```

In `internal/repoguard/budgets_test.go`, set each row's two ceilings to the measured values and
prepend a note to its trailing comment, keeping the rest of the comment:

- `{"docket-build/references/gate-caller-loop.md", 134, 1510}` → `{…, <L>, <W>}, // 0497: +halted-gate tree-survives finding relay (134/1510 -> <L>/<W>); 0491: …`
- `{"docket-finalize-change/references/gate-failure.md", 146, 1901}` → `{…, <L>, <W>}, // 0497: +wait-for-the-leftover-group sentence (146/1901 -> <L>/<W>); 0493: …`
- `{"docket-implement-next/SKILL.md", 216, 8325}` → `{…, <L>, <W>}, // 0497: +Step 5 halted-build evidence kept verbatim in ## Run halted (216/8325 -> <L>/<W>); 0498 review: …`

Regenerate the embedded mirror:

```bash
go generate ./internal/assets
git status --porcelain -- internal/assets
```

Expected: the mirror copies of the three skill files and `internal/assets/embedded/manifest.json`
are modified; nothing else under `internal/assets` (the glossary is not mirrored).

- [ ] **Step 7: Run the guards**

Run:

```bash
go test -count=1 ./internal/assets/
go test -count=1 -run 'TestSkillSizeBudgets|TestCommentAnchorStyle|TestNoTaskOwnedDriveInstructions' ./internal/repoguard/
go test -count=1 ./internal/repoguard/
```

Expected: PASS. If a retired-vocabulary or other repoguard test reddens on the new prose, reword
the new sentence (never weaken the guard) and re-run.

- [ ] **Step 8: Commit**

```bash
git add docs/reference/glossary.md skills/docket-finalize-change/references/gate-failure.md skills/docket-build/references/gate-caller-loop.md skills/docket-implement-next/SKILL.md internal/repoguard/budgets_test.go internal/assets/embedded/manifest.json internal/assets/embedded/tree/skills/docket-finalize-change/references/gate-failure.md internal/assets/embedded/tree/skills/docket-build/references/gate-caller-loop.md internal/assets/embedded/tree/skills/docket-implement-next/SKILL.md
git status --porcelain
git commit -m "docs(0497): glossary and skills relay a halted gate's tree-survives finding"
```

`git status --porcelain` before the commit must list only the staged paths; if `go generate`
touched another mirrored file, stage it too and name it in the commit body.

---

## Spec coverage

| Spec item | Task |
|---|---|
| Decision 1: death branch probes after the cause is chosen; only `leftover` stamps; outcome and cause unchanged; `last_finding` omitempty, no schema bump; `recordedDoc` re-read; `DriveDoc.finding` on HALTED only; only the death branch probes | 1 |
| Decision 1: `HumanText` prints `finding:` | 2 |
| Decision 2: finding becomes `teardown_finding`; retention token wins; exact message texts; disposition/reason/result/halt cause unchanged; `evidence.recertify` unchanged | 2 |
| Decision 3: `docket-build` gate-caller-loop sentence; trace to `## Run halted` and one sentence there | 3 (Steps 4–5) |
| Failure posture: every probe answer × both death states, outcome/cause identical; finalize fields identical; two required mutations | 1 (Steps 1, 7), 2 (Steps 1–2, 7) |
| Prose: glossary retitle and halt reporters; `run-tracker.md` checked and left; `gate-failure.md` sentence; whole-repo grep | 3 |
| Tests: gatedrive matrix, re-read, old record, HumanText; app mapping and messages; whole suite at the gate | 1, 2; docket-build's gate |
| Decision record: ADR-0134 dated Update note | coordinator via docket-adr, not a build task |
| Out of scope: refusing/delaying a start, `run.start --resume` line, keeping the run root, signalling, recertify, raw `gate.launch`, 0492 gaps 2 and 4, test-leak hygiene | no task |
