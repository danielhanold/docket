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
