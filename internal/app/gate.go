package app

import (
	"os"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/process"
)

// Gate operation names — the fixed protocol identifiers for the gate group.
const (
	OperationGateLaunch  = "gate.launch"
	OperationGateObserve = "gate.observe"
	OperationGateStop    = "gate.stop"
	OperationGateRecover = "gate.recover"
)

// GateState is the run-state vocabulary the process supervisor decides, carried
// verbatim into the protocol document. It mirrors process.State's spellings.
type GateState string

// RecoveryEntry is one per-slot recover verdict as the protocol exposes it — a
// stable client of process.RecoveryEntry, never that type, so the protocol
// contract is fixed independent of the process layer's internal spelling.
type RecoveryEntry struct {
	RunID       string `json:"run_id"`
	RunDir      string `json:"run_dir"`
	Disposition string `json:"disposition"`
	Reason      string `json:"reason,omitempty"`
}

// GateResult is the protocol document for gate.launch, gate.observe, and
// gate.stop. Optional handle and outcome fields are omitempty; ExitCode and
// Signal are pointers so a real zero is distinguishable from absence. Reason
// carries only bounded safe failure text — never argv, environment values, or
// child output.
type GateResult struct {
	Envelope
	RunID     string    `json:"run_id,omitempty"`
	RunDir    string    `json:"run_dir,omitempty"`
	State     GateState `json:"state,omitempty"`
	ExitCode  *int      `json:"exit_code,omitempty"`
	Signal    *int      `json:"signal,omitempty"`
	Cause     string    `json:"cause,omitempty"`
	StdoutLog string    `json:"stdout_log,omitempty"`
	StderrLog string    `json:"stderr_log,omitempty"`
	Reason    string    `json:"reason,omitempty"`
}

// GateRecoverResult is gate.recover's own protocol document. Recovery is a
// non-omitempty slice so it marshals as "recovery": [] on every path (the
// landed nil-collection convention); a single struct cannot both emit [] here
// and omit the field on the other three operations, so recover gets its own
// type. Reason carries bounded safe failure text on failure paths only.
type GateRecoverResult struct {
	Envelope
	Marked   int             `json:"marked"`
	Recovery []RecoveryEntry `json:"recovery"`
	Reason   string          `json:"reason,omitempty"`
}

// mapObservation is the operation-sensitive result table for gate.observe and
// post-launch states; pure, so the table is testable without a process.
func mapObservation(st process.State) Result {
	switch st {
	case process.StateRunning, process.StatePassed:
		return ResultApplied
	case process.StateFailed:
		return ResultGateFailed
	case process.StateSignaled, process.StateStopped, process.StateVanished:
		return ResultInterrupted
	default:
		return ResultInternalError
	}
}

// mapGateFailure maps a process operation error to a protocol result and its
// bounded safe reason. The reason is the failure's own "stage: reason" text,
// which the process layer keeps free of argv, environment values, and child
// output; an unclassified error is an internal error.
func mapGateFailure(err error) (Result, string) {
	if f, ok := process.AsFailure(err); ok {
		var r Result
		switch f.Class {
		case process.FailInvalidInput:
			r = ResultInvalidInput
		case process.FailInvalidState:
			r = ResultInvalidState
		case process.FailBlocked:
			r = ResultBlocked
		case process.FailExternal:
			r = ResultExternalFailed
		default:
			r = ResultInternalError
		}
		return r, f.Error()
	}
	return ResultInternalError, err.Error()
}

// gateService resolves the current executable and builds a process.Service. A
// non-nil (result, reason) signals a resolution failure the caller returns
// directly; the reason is a fixed safe string, never a host path.
func gateService() (*process.Service, Result, string) {
	exe, err := os.Executable()
	if err != nil {
		return nil, ResultExternalFailed, "resolve-executable: cannot determine executable path"
	}
	svc, err := process.NewService(exe)
	if err != nil {
		r, reason := mapGateFailure(err)
		return nil, r, reason
	}
	return svc, "", ""
}

// applyState carries a run state and its exact terminal facts into a
// GateResult: an exit-kind terminal sets ExitCode (including a real 0), a
// signal-kind terminal sets Signal.
func applyState(r *GateResult, st process.State, term *process.Terminal) {
	r.State = GateState(st)
	if term == nil {
		return
	}
	switch term.Kind {
	case "exit":
		c := term.ExitCode
		r.ExitCode = &c
	case "signal":
		s := term.Signal
		r.Signal = &s
	}
}

// GateLaunch launches a supervised run and maps its handle and post-launch
// state to a protocol result.
func GateLaunch(root, cwd string, argv []string) GateResult {
	svc, res, reason := gateService()
	if svc == nil {
		return GateResult{Envelope: NewEnvelope(OperationGateLaunch, res), Reason: reason}
	}
	out, err := svc.Launch(process.LaunchRequest{Root: root, Cwd: cwd, Argv: argv})
	if err != nil {
		res, reason := mapGateFailure(err)
		return GateResult{Envelope: NewEnvelope(OperationGateLaunch, res), Reason: reason}
	}
	r := GateResult{
		Envelope:  NewEnvelope(OperationGateLaunch, mapObservation(out.State)),
		RunID:     out.RunID,
		RunDir:    out.RunDir,
		StdoutLog: out.StdoutLog,
		StderrLog: out.StderrLog,
	}
	applyState(&r, out.State, out.Terminal)
	return r
}

// GateObserve reports a run's state through the read-only observe decision.
func GateObserve(runDir string) GateResult {
	svc, res, reason := gateService()
	if svc == nil {
		return GateResult{Envelope: NewEnvelope(OperationGateObserve, res), Reason: reason}
	}
	obs, err := svc.Observe(runDir)
	if err != nil {
		res, reason := mapGateFailure(err)
		return GateResult{Envelope: NewEnvelope(OperationGateObserve, res), Reason: reason}
	}
	r := GateResult{
		Envelope:  NewEnvelope(OperationGateObserve, mapObservation(obs.State)),
		RunID:     obs.RunID,
		RunDir:    obs.RunDir,
		Cause:     obs.Cause,
		StdoutLog: obs.StdoutLog,
		StderrLog: obs.StderrLog,
	}
	applyState(&r, obs.State, obs.Terminal)
	return r
}

// GateStop drives the ownership-gated stop and maps its verdict. A performed
// termination is applied; an already-terminal no-op carries the preserved
// state (consumers read state; the stop performed nothing).
func GateStop(runDir, reason string) GateResult {
	svc, res, freason := gateService()
	if svc == nil {
		return GateResult{Envelope: NewEnvelope(OperationGateStop, res), Reason: freason}
	}
	out, err := svc.Stop(runDir, reason)
	if err != nil {
		res, freason := mapGateFailure(err)
		return GateResult{Envelope: NewEnvelope(OperationGateStop, res), Reason: freason}
	}
	result := ResultApplied
	if !out.Performed {
		result = ResultNoOp
	}
	r := GateResult{
		Envelope: NewEnvelope(OperationGateStop, result),
		RunID:    out.RunID,
		RunDir:   out.RunDir,
	}
	applyState(&r, out.State, out.Terminal)
	return r
}

// GateRecover scans a root and maps the recover outcome. A newly marked run
// makes the pass applied; a clean scan is a no-op. The entry slice is
// normalized to non-nil on every path so "recovery" marshals as [].
func GateRecover(root string) GateRecoverResult {
	svc, res, reason := gateService()
	if svc == nil {
		return GateRecoverResult{
			Envelope: NewEnvelope(OperationGateRecover, res),
			Recovery: []RecoveryEntry{},
			Reason:   reason,
		}
	}
	out, err := svc.Recover(root)
	if err != nil {
		res, reason := mapGateFailure(err)
		return GateRecoverResult{
			Envelope: NewEnvelope(OperationGateRecover, res),
			Recovery: []RecoveryEntry{},
			Reason:   reason,
		}
	}
	result := ResultNoOp
	if out.Marked >= 1 {
		result = ResultApplied
	}
	entries := make([]RecoveryEntry, 0, len(out.Entries))
	for _, e := range out.Entries {
		entries = append(entries, RecoveryEntry{
			RunID:       e.RunID,
			RunDir:      e.RunDir,
			Disposition: e.Disposition,
			Reason:      e.Reason,
		})
	}
	return GateRecoverResult{
		Envelope: NewEnvelope(OperationGateRecover, result),
		Marked:   out.Marked,
		Recovery: entries,
	}
}

// HumanText renders GateResult as stable labeled lines in a fixed order,
// emitting only the fields that are set.
func (r GateResult) HumanText() string {
	var lines []string
	if r.State != "" {
		lines = append(lines, "state: "+string(r.State))
	}
	if r.RunID != "" {
		lines = append(lines, "run_id: "+r.RunID)
	}
	if r.RunDir != "" {
		lines = append(lines, "run_dir: "+r.RunDir)
	}
	if r.ExitCode != nil {
		lines = append(lines, "exit_code: "+strconv.Itoa(*r.ExitCode))
	}
	if r.Signal != nil {
		lines = append(lines, "signal: "+strconv.Itoa(*r.Signal))
	}
	if r.Cause != "" {
		lines = append(lines, "cause: "+r.Cause)
	}
	if r.StdoutLog != "" {
		lines = append(lines, "stdout_log: "+r.StdoutLog)
	}
	if r.StderrLog != "" {
		lines = append(lines, "stderr_log: "+r.StderrLog)
	}
	if r.Reason != "" {
		lines = append(lines, "reason: "+r.Reason)
	}
	return strings.Join(lines, "\n")
}

// HumanText renders GateRecoverResult as a marked count followed by one line
// per recovery entry, then a reason line when a failure supplied one.
func (r GateRecoverResult) HumanText() string {
	lines := []string{"marked: " + strconv.Itoa(r.Marked)}
	for _, e := range r.Recovery {
		lines = append(lines, "run: "+e.RunID+" "+e.Disposition)
	}
	if r.Reason != "" {
		lines = append(lines, "reason: "+r.Reason)
	}
	return strings.Join(lines, "\n")
}
