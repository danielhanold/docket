package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// cleanSweep builds an applied implementation-scope sweep whose entries are all
// clean dispositions, for the both-halves-compose path.
func cleanSweep() MaintenanceResult {
	return newMaintenanceResult(ResultApplied, MaintenanceResult{
		Scope: string(SweepScopeImplementation),
		Entries: []MaintenanceEntry{
			{ID: 12, Kind: "closeout", Disposition: SweepDispApplied},
			{ID: 13, Kind: "reclaim", Disposition: SweepDispSkipped, Reason: ReasonSweepReclaimAutoDisabled},
			{ID: 14, Kind: "closeout", Disposition: SweepDispNoOp},
		},
		DeferredHistoricalCleanups: 241,
	})
}

func TestPreflightCleanComposesBothHalves(t *testing.T) {
	sweep := cleanSweep()
	status := NewStatusResult(ResultApplied, StatusResult{
		Context: StatusContext{MetadataRevision: "abc123"},
		Summary: StatusSummary{TotalChanges: 3},
		Ready:   []int{15},
	})
	res := maintenancePreflight(context.Background(), preflightOps{
		sweep:  func(context.Context) MaintenanceResult { return sweep },
		status: func(context.Context) StatusResult { return status },
	})
	if res.Operation != OperationMaintenancePreflight || res.ProtocolVersion != 1 || res.Result != ResultApplied {
		t.Fatalf("envelope: %+v", res.Envelope)
	}
	if res.Preflight != PreflightClean {
		t.Fatalf("verdict: %q", res.Preflight)
	}
	if len(res.Sweep.Entries) != 3 || len(res.Sweep.ProblemEntries) != 0 || res.Sweep.DeferredHistoricalCleanups != 241 {
		t.Fatalf("sweep half: %+v", res.Sweep)
	}
	if res.Sweep.Scope != string(SweepScopeImplementation) {
		t.Fatalf("sweep scope: %q", res.Sweep.Scope)
	}
	if res.Status == nil || res.Status.Summary.TotalChanges != 3 || len(res.Status.Ready) != 1 {
		t.Fatalf("status half: %+v", res.Status)
	}
	if res.MetadataRevision != "abc123" {
		t.Fatalf("metadata revision: %q", res.MetadataRevision)
	}
	// The compact projection carries no records/changes keys at all.
	b, _ := json.Marshal(res)
	for _, forbidden := range []string{`"records"`, `"changes"`} {
		if strings.Contains(string(b), forbidden) {
			t.Fatalf("preflight envelope leaks %s: %s", forbidden, b)
		}
	}
	// Every array marshals [] rather than null.
	for _, arr := range []string{`"entries":[`, `"problem_entries":[`, `"findings":[`, `"ready":[`} {
		if !strings.Contains(string(b), arr) {
			t.Fatalf("array %s not marshaled as []: %s", arr, b)
		}
	}
}

func TestPreflightProblemPerDisposition(t *testing.T) {
	for _, disp := range []string{SweepDispBlocked, SweepDispFailed, SweepDispUnknown, SweepDispContended} {
		disp := disp
		t.Run(disp, func(t *testing.T) {
			sweep := newMaintenanceResult(ResultApplied, MaintenanceResult{
				Scope: string(SweepScopeImplementation),
				Entries: []MaintenanceEntry{
					{ID: 20, Kind: "closeout", Disposition: SweepDispApplied},
					{ID: 21, Kind: "closeout", Disposition: disp},
				},
			})
			status := NewStatusResult(ResultApplied, StatusResult{})
			res := maintenancePreflight(context.Background(), preflightOps{
				sweep:  func(context.Context) MaintenanceResult { return sweep },
				status: func(context.Context) StatusResult { return status },
			})
			if res.Preflight != PreflightProblem {
				t.Fatalf("disposition %q: verdict = %q, want problem", disp, res.Preflight)
			}
			if len(res.Sweep.ProblemEntries) != 1 || res.Sweep.ProblemEntries[0].ID != 21 || res.Sweep.ProblemEntries[0].Disposition != disp {
				t.Fatalf("disposition %q: problem_entries = %+v, want exactly the disp entry", disp, res.Sweep.ProblemEntries)
			}
		})
	}
}

func TestPreflightCleanDispositionsStayClean(t *testing.T) {
	sweep := newMaintenanceResult(ResultApplied, MaintenanceResult{
		Scope: string(SweepScopeImplementation),
		Entries: []MaintenanceEntry{
			{ID: 30, Kind: "closeout", Disposition: SweepDispApplied},
			{ID: 31, Kind: "closeout", Disposition: SweepDispNoOp},
			{ID: 32, Kind: "reclaim", Disposition: SweepDispSkipped, Reason: ReasonSweepReclaimAutoDisabled},
			{ID: 33, Kind: "cleanup", Disposition: SweepDispSkipped, Reason: ReasonSweepItemVanished},
		},
	})
	status := NewStatusResult(ResultApplied, StatusResult{})
	res := maintenancePreflight(context.Background(), preflightOps{
		sweep:  func(context.Context) MaintenanceResult { return sweep },
		status: func(context.Context) StatusResult { return status },
	})
	if res.Preflight != PreflightClean {
		t.Fatalf("verdict = %q, want clean", res.Preflight)
	}
	if len(res.Sweep.ProblemEntries) != 0 {
		t.Fatalf("problem_entries = %+v, want empty", res.Sweep.ProblemEntries)
	}
}

func TestPreflightSweepRefusalOmitsStatus(t *testing.T) {
	sweep := maintenanceRefusal(ResultInvalidState, "some-reason", "msg")
	res := maintenancePreflight(context.Background(), preflightOps{
		sweep: func(context.Context) MaintenanceResult { return sweep },
		status: func(context.Context) StatusResult {
			t.Fatal("status seam must not be called after a whole-sweep refusal")
			return StatusResult{}
		},
	})
	if res.Result != ResultInvalidState {
		t.Fatalf("result = %q, want invalid-state", res.Result)
	}
	if res.Reason != "some-reason" {
		t.Fatalf("reason = %q, want some-reason", res.Reason)
	}
	if res.Preflight != PreflightProblem {
		t.Fatalf("verdict = %q, want problem", res.Preflight)
	}
	if res.Status != nil {
		t.Fatalf("status half = %+v, want nil on refusal", res.Status)
	}
	if res.MetadataRevision != "" {
		t.Fatalf("metadata revision = %q, want empty on refusal", res.MetadataRevision)
	}
	b, _ := json.Marshal(res)
	if strings.Contains(string(b), `"status"`) {
		t.Fatalf("refusal envelope leaks status key: %s", b)
	}
}

func TestPreflightStatusFailureKeepsSweepHalf(t *testing.T) {
	sweep := cleanSweep()
	status := NewStatusResult(ResultExternalFailed, StatusResult{
		Reason:  "read-failed",
		Message: "origin unreachable",
	})
	res := maintenancePreflight(context.Background(), preflightOps{
		sweep:  func(context.Context) MaintenanceResult { return sweep },
		status: func(context.Context) StatusResult { return status },
	})
	if res.Result != ResultExternalFailed {
		t.Fatalf("result = %q, want external-failed (mirrored)", res.Result)
	}
	if len(res.Sweep.Entries) != 3 {
		t.Fatalf("sweep half not preserved: %+v", res.Sweep)
	}
	if res.Status != nil {
		t.Fatalf("status half = %+v, want nil on read failure", res.Status)
	}
	if res.Preflight != PreflightClean {
		t.Fatalf("verdict = %q, want clean (the sweep's verdict, not the read's)", res.Preflight)
	}
	if res.Reason != "read-failed" {
		t.Fatalf("reason = %q, want read-failed", res.Reason)
	}
}

func TestPreflightHumanText(t *testing.T) {
	sweep := cleanSweep()
	status := NewStatusResult(ResultApplied, StatusResult{
		Summary: StatusSummary{TotalChanges: 3},
		Ready:   []int{15},
	})
	res := maintenancePreflight(context.Background(), preflightOps{
		sweep:  func(context.Context) MaintenanceResult { return sweep },
		status: func(context.Context) StatusResult { return status },
	})
	human := res.HumanText()
	if !strings.Contains(human, PreflightClean) {
		t.Fatalf("HumanText missing verdict token: %q", human)
	}
	if !strings.Contains(human, "maintenance.sweep") {
		t.Fatalf("HumanText missing sweep one-liner: %q", human)
	}
	if !strings.Contains(human, "status:") {
		t.Fatalf("HumanText missing status one-liner: %q", human)
	}
}
