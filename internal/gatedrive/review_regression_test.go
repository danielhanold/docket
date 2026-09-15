package gatedrive

import (
	"testing"

	"github.com/danielhanold/docket/internal/process"
)

func TestChildInputValidationBindsEpochAndRevocation(t *testing.T) {
	for _, scenario := range []string{"omitted-epoch", "wrong-epoch", "cancelled-epoch"} {
		t.Run(scenario, func(t *testing.T) {
			d, store := newTestDriver(t, &fakeClock{now: startEpoch()}, &fakeProc{}, stableGit())
			req := sampleStart()
			scopeReq := scopeReqFor(req, "context")
			scopeReq.RunEpochID = "epoch"
			grant, err := store.PrepareScope(scopeReq)
			if err != nil {
				t.Fatal(err)
			}
			req.ScopeID, req.ChildCapability, req.GateContext, req.RunEpochID = grant.ScopeID, grant.ChildCapability, "context", "epoch"
			d.SetEpochRevokedResolver(func(string) (bool, error) { return false, nil })
			if err := d.ValidateChildInputs(req); err != nil {
				t.Fatalf("valid boundary refused: %v", err)
			}
			switch scenario {
			case "omitted-epoch":
				req.RunEpochID = ""
			case "wrong-epoch":
				req.RunEpochID = "other"
			case "cancelled-epoch":
				d.SetEpochRevokedResolver(func(string) (bool, error) { return true, nil })
			}
			if d.ValidateChildInputs(req) == nil {
				t.Fatal("invalid epoch accepted before child work")
			}
		})
	}
}

func TestChildInputValidationRejectsNoncurrentPredecessor(t *testing.T) {
	d, store := newTestDriver(t, &fakeClock{now: startEpoch()}, &fakeProc{observe: func(path string) (*process.Observation, error) { return obs(process.StatePassed, path), nil }}, stableGit())
	req := sampleStart()
	grant, err := store.PrepareScope(scopeReqFor(req, ""))
	if err != nil {
		t.Fatal(err)
	}
	req.ScopeID, req.ChildCapability = grant.ScopeID, grant.ChildCapability
	current, err := d.Start(req)
	if err != nil || current.Outcome != PASSED {
		t.Fatalf("start: %v %+v", err, current)
	}
	unrelated := seedRecord(t)
	unrelated.LastOutcome = PASSED
	otherID, otherGeneration := seedDrive(t, store, unrelated)
	for _, tc := range []struct {
		name, driveID, generation string
		valid                     bool
	}{
		{"current", current.DriveID, current.Generation, true},
		{"missing-id", "", current.Generation, false},
		{"stale-generation", current.DriveID, "stale", false},
		{"foreign", otherID, otherGeneration, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req.PredecessorDriveID, req.PredecessorOwnerGen = tc.driveID, tc.generation
			err := d.ValidateChildInputs(req)
			if tc.valid && err != nil {
				t.Fatalf("current predecessor rejected: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("invalid predecessor accepted before child work")
			}
		})
	}
}

func TestRecoveredInputValidationAcceptsCurrentPassingEvidence(t *testing.T) {
	git := stableGit()
	d, store := newTestDriver(t, &fakeClock{now: startEpoch()}, &fakeProc{observe: func(path string) (*process.Observation, error) { return obs(process.StatePassed, path), nil }}, git)
	req := sampleStart()
	scopeReq := scopeReqFor(req, "ctx")
	scopeReq.RunEpochID = "epoch"
	grant, err := store.PrepareScope(scopeReq)
	if err != nil {
		t.Fatal(err)
	}
	req.ScopeID, req.ChildCapability, req.GateContext, req.RunEpochID = grant.ScopeID, grant.ChildCapability, "ctx", "epoch"
	started, err := d.Start(req)
	if err != nil {
		t.Fatal(err)
	}
	taken, err := d.Takeover(grant.ScopeID, grant.ParentCapability, started.DriveID)
	if err != nil || taken.Outcome != PASSED {
		t.Fatalf("takeover: %v %+v", err, taken)
	}
	in := RecoveredInputs{ScopeID: grant.ScopeID, DriveID: taken.DriveID, OwnerGeneration: taken.Generation, ChangeID: req.ChangeID, TaskID: req.TaskID, Phase: req.Phase, GateContext: "ctx", RunEpochID: "epoch", RepoDir: req.RepoDir, Worktree: req.Worktree}
	if err := d.ValidateRecoveredInputs(in); err != nil {
		t.Fatalf("current passing recovered evidence rejected: %v", err)
	}
	for _, mutate := range []func(*RecoveredInputs){func(in *RecoveredInputs) { in.RepoDir = "/foreign" }, func(in *RecoveredInputs) { in.Worktree = "/foreign" }} {
		changed := in
		mutate(&changed)
		if d.ValidateRecoveredInputs(changed) == nil {
			t.Fatal("recovered evidence accepted for a foreign assignment root")
		}
	}
}

func TestRecoveredInputValidationRequiresCurrentPassingEvidence(t *testing.T) {
	for _, scenario := range []string{"failed-run", "fingerprint-drift", "cancelled-epoch"} {
		t.Run(scenario, func(t *testing.T) {
			git := stableGit()
			state := process.StatePassed
			if scenario == "failed-run" {
				state = process.StateFailed
			}
			d, store := newTestDriver(t, &fakeClock{now: startEpoch()}, &fakeProc{observe: func(path string) (*process.Observation, error) { return obs(state, path), nil }}, git)
			req := sampleStart()
			scopeReq := scopeReqFor(req, "ctx")
			scopeReq.RunEpochID = "epoch"
			grant, err := store.PrepareScope(scopeReq)
			if err != nil {
				t.Fatal(err)
			}
			req.ScopeID, req.ChildCapability, req.GateContext, req.RunEpochID = grant.ScopeID, grant.ChildCapability, "ctx", "epoch"
			started, err := d.Start(req)
			if err != nil {
				t.Fatal(err)
			}
			taken, err := d.Takeover(grant.ScopeID, grant.ParentCapability, started.DriveID)
			if err != nil || taken.Outcome == HALTED {
				t.Fatalf("takeover: %v %+v", err, taken)
			}
			in := RecoveredInputs{ScopeID: grant.ScopeID, DriveID: taken.DriveID, OwnerGeneration: taken.Generation, ChangeID: req.ChangeID, TaskID: req.TaskID, Phase: req.Phase, GateContext: "ctx", RunEpochID: "epoch", RepoDir: req.RepoDir, Worktree: req.Worktree}
			if scenario == "fingerprint-drift" {
				git.status = "unverified new edits"
			}
			if scenario == "cancelled-epoch" {
				d.SetEpochRevokedResolver(func(string) (bool, error) { return true, nil })
			}
			if d.ValidateRecoveredInputs(in) == nil {
				t.Fatal("accepted invalid recovered evidence for a no-test continuation")
			}
		})
	}
}
