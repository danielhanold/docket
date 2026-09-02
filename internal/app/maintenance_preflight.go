package app

import (
	"context"
	"fmt"
)

// OperationMaintenancePreflight is the operation key `maintenance preflight`
// records in its envelope: the implementation-scope sweep and the compact
// post-sweep read, sequenced in one process (change 0397). A thin composition
// over MaintenanceSweep and Status — no new sweep or status logic lives here.
const OperationMaintenancePreflight = "maintenance.preflight"

// The Go-computed preflight verdict vocabulary. `problem` when the sweep failed
// as a whole or any sweep entry is blocked, failed, unknown, or contended — the
// rule docket-implement-next's Step 0 previously stated in prose; `clean`
// otherwise (an intentional policy skip and a genuine noop are clean).
const (
	PreflightClean   = "clean"
	PreflightProblem = "problem"
)

// problemDispositions is the closed set of entry dispositions that make the
// preflight verdict `problem` and populate problem_entries.
var problemDispositions = map[string]bool{
	SweepDispBlocked:   true,
	SweepDispFailed:    true,
	SweepDispUnknown:   true,
	SweepDispContended: true,
}

// PreflightSweepHalf is the sweep half of the preflight envelope: the sweep's
// own envelope result and report fields, plus the problem_entries projection.
type PreflightSweepHalf struct {
	Result                     Result             `json:"result"`
	Scope                      string             `json:"scope"`
	Entries                    []MaintenanceEntry `json:"entries"`
	ProblemEntries             []MaintenanceEntry `json:"problem_entries"`
	DeferredHistoricalCleanups int                `json:"deferred_historical_cleanups"`
	Reason                     string             `json:"reason,omitempty"`
	Message                    string             `json:"message,omitempty"`
	Findings                   []StatusFinding    `json:"findings"`
}

// PreflightStatusHalf is the compact post-sweep read: no records, no changes.
type PreflightStatusHalf struct {
	Result   Result          `json:"result"`
	Summary  StatusSummary   `json:"summary"`
	Ready    []int           `json:"ready"`
	Findings []StatusFinding `json:"findings"`
}

// MaintenancePreflightResult is the protocol-v1 document `maintenance preflight`
// returns: one envelope carrying the Go-computed verdict, the sweep half with
// its problem_entries projection, the compact post-sweep read (absent when the
// sweep refused/errored or its post-sweep read failed), and the post-sweep
// metadata revision.
type MaintenancePreflightResult struct {
	Envelope
	Preflight        string               `json:"preflight"` // clean | problem
	Sweep            PreflightSweepHalf   `json:"sweep"`
	Status           *PreflightStatusHalf `json:"status,omitempty"` // absent when the sweep refused/errored or the read failed
	MetadataRevision string               `json:"metadata_revision,omitempty"`
	Reason           string               `json:"reason,omitempty"`
	Message          string               `json:"message,omitempty"`
}

// HumanText reuses the two composed renderers — the sweep line, then the compact
// status line when present — prefixed by the verdict, never an authored body.
func (r MaintenancePreflightResult) HumanText() string {
	sweep := MaintenanceResult{
		Entries:                    r.Sweep.Entries,
		Reason:                     r.Sweep.Reason,
		Message:                    r.Sweep.Message,
		Findings:                   r.Sweep.Findings,
		Scope:                      r.Sweep.Scope,
		DeferredHistoricalCleanups: r.Sweep.DeferredHistoricalCleanups,
	}
	sweep.Envelope = NewEnvelope(OperationMaintenanceSweep, r.Sweep.Result)
	out := fmt.Sprintf("%s: %s\n%s", r.Operation, r.Preflight, sweep.HumanText())
	if r.Status != nil {
		out += fmt.Sprintf("\nstatus: %d change(s), %d ready, %d finding(s)",
			r.Status.Summary.TotalChanges, len(r.Status.Ready), len(r.Status.Findings))
	}
	return out
}

// preflightVerdict computes the verdict and the problem_entries projection from
// the sweep alone. A whole-sweep failure is a problem; otherwise the verdict is
// entry-driven, in sweep order. The projection marshals [] on every path.
func preflightVerdict(sweep MaintenanceResult) (string, []MaintenanceEntry) {
	problems := []MaintenanceEntry{}
	for _, e := range sweep.Entries {
		if problemDispositions[e.Disposition] {
			problems = append(problems, e)
		}
	}
	if sweep.Result != ResultApplied && sweep.Result != ResultNoOp {
		return PreflightProblem, problems
	}
	if len(problems) > 0 {
		return PreflightProblem, problems
	}
	return PreflightClean, problems
}

// preflightOps is the injection seam: the two sequenced operations, already
// bound to their scope and options. Production binds MaintenanceSweep at
// SweepScopeImplementation and Status with IncludeRecords off; tests inject
// canned results so the composition rules are proved without a repository.
type preflightOps struct {
	sweep  func(ctx context.Context) MaintenanceResult
	status func(ctx context.Context) StatusResult
}

// maintenancePreflight sequences the two injected operations and projects the
// one envelope. It is the composition under test.
func maintenancePreflight(ctx context.Context, ops preflightOps) MaintenancePreflightResult {
	sweep := ops.sweep(ctx)
	verdict, problems := preflightVerdict(sweep)
	entries := sweep.Entries
	if entries == nil {
		entries = []MaintenanceEntry{}
	}
	findings := sweep.Findings
	if findings == nil {
		findings = []StatusFinding{}
	}
	out := MaintenancePreflightResult{
		Preflight: verdict,
		Sweep: PreflightSweepHalf{
			Result:                     sweep.Result,
			Scope:                      sweep.Scope,
			Entries:                    entries,
			ProblemEntries:             problems,
			DeferredHistoricalCleanups: sweep.DeferredHistoricalCleanups,
			Reason:                     sweep.Reason,
			Message:                    sweep.Message,
			Findings:                   findings,
		},
	}
	if sweep.Result != ResultApplied && sweep.Result != ResultNoOp {
		// Whole-sweep refusal/error: no read is attempted — a parent must never
		// mistake a failed sweep for a failed read.
		out.Reason, out.Message = sweep.Reason, sweep.Message
		out.Envelope = NewEnvelope(OperationMaintenancePreflight, sweep.Result)
		return out
	}
	status := ops.status(ctx)
	if status.Result != ResultApplied {
		// Read failed after a successful sweep: the sweep half stays intact and
		// the envelope mirrors the read's failure spelling, so the parent never
		// advances on either half. The verdict stays the sweep's — a failed read
		// is signaled by result, not by faking a sweep problem.
		out.Reason, out.Message = status.Reason, status.Message
		out.Envelope = NewEnvelope(OperationMaintenancePreflight, status.Result)
		return out
	}
	ready := status.Ready
	if ready == nil {
		ready = []int{}
	}
	statusFindings := status.Findings
	if statusFindings == nil {
		statusFindings = []StatusFinding{}
	}
	out.Status = &PreflightStatusHalf{
		Result:   status.Result,
		Summary:  status.Summary,
		Ready:    ready,
		Findings: statusFindings,
	}
	out.MetadataRevision = status.Context.MetadataRevision
	out.Envelope = NewEnvelope(OperationMaintenancePreflight, sweep.Result)
	return out
}

// MaintenancePreflight is the production entry point the CLI wires: the
// implementation-scope sweep, then the compact post-sweep read over a fresh pin
// (no records, no changes), in one process.
func MaintenancePreflight(ctx context.Context, deps FinalizeDeps, reader StatusReader, repoDir string) MaintenancePreflightResult {
	return maintenancePreflight(ctx, preflightOps{
		sweep: func(ctx context.Context) MaintenanceResult {
			return MaintenanceSweep(ctx, deps, repoDir, SweepScopeImplementation)
		},
		status: func(ctx context.Context) StatusResult {
			return Status(ctx, reader, StatusOptions{RepoDir: repoDir, IncludeRecords: false})
		},
	})
}
