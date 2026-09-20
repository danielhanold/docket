package app

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/gitcli"
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

// rawGateEpoch is the run epoch a raw app.GateLaunch carries: none. A raw launch
// never owns an implementation epoch, so it presents the empty epoch to the
// stale-run-epoch fence. Real epochs are threaded into workflow gates by Task 9;
// until a slot records a non-empty epoch this fence stays dormant (it compiles and
// stays green), then rejects a raw launch into a worktree an epoch owns.
const rawGateEpoch = ""

// GateLaunch launches a supervised run and maps its handle and post-launch
// state to a protocol result. When cwd sits inside a registered git worktree, the
// launch first admits through the worktree execution slot (change 0375): one
// canonical worktree carries at most one reserved-or-running top-level gate
// execution across scoped, scopeless, and raw launches, so a second raw launch
// into a busy worktree is REFUSED (worktree-busy / unresolved-execution) with a
// safe incumbent locator and no process spawned. Admission composes ONCE, here at
// the public boundary; the gate driver's own ProcessSeam.Launch holds its ticket
// from Tasks 3–5 and never re-reserves. A cwd outside any git worktree keeps the
// pre-admission contract exactly: no slot, no reservation token.
func GateLaunch(root, cwd string, argv []string) GateResult {
	svc, res, reason := gateService()
	if svc == nil {
		return GateResult{Envelope: NewEnvelope(OperationGateLaunch, res), Reason: reason}
	}

	worktreeRoot, repoIdentity, store, admit := resolveWorktreeAdmission(cwd)
	var token string
	if admit {
		if refusal, refused := rawStaleEpochRefusal(store, worktreeRoot); refused {
			return refusal
		}
		t, aerr := store.ReserveRawWorktreeExecution(repoIdentity, worktreeRoot, svc)
		if aerr != nil {
			r, reason := mapAdmissionFailure(aerr)
			return GateResult{Envelope: NewEnvelope(OperationGateLaunch, r), Reason: reason, Cause: admissionRefusalCause(aerr)}
		}
		token = t
	}

	out, err := svc.Launch(process.LaunchRequest{Root: root, Cwd: cwd, Argv: argv, ReservationToken: token})
	if err != nil {
		if admit {
			resolveRawLaunchFailure(svc, store, worktreeRoot, root, token)
		}
		res, reason := mapGateFailure(err)
		return GateResult{Envelope: NewEnvelope(OperationGateLaunch, res), Reason: reason}
	}
	if admit {
		if cerr := store.ConfirmWorktreeExecution(worktreeRoot, token, out.RunID, out.RunDir); cerr != nil {
			// The run is launched but its slot could not be confirmed: only a
			// proven teardown lets the slot release, else it fails closed to
			// unresolved so it never becomes a free slot over a live run.
			if rawStopIfOwned(svc, out.RunDir) {
				_ = store.ReleaseWorktreeExecution(worktreeRoot, token)
			} else {
				_ = store.MarkWorktreeExecutionUnresolved(worktreeRoot, token)
			}
			r, reason := mapAdmissionFailure(cerr)
			return GateResult{Envelope: NewEnvelope(OperationGateLaunch, r), Reason: reason}
		}
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

// resolveWorktreeAdmission resolves the registered worktree that contains cwd and
// opens the gate-admission store on its repository's Git common directory. ok is
// false when cwd sits outside any git worktree, or the worktree is unregistered or
// broken, so a raw gate.launch there keeps its pre-admission contract (no slot, no
// token). It reaches Git only through gitcli — never internal/process — and returns
// the CANONICAL containing worktree root (the admission key) and the common dir
// (the store root), the two dimensions the driver's scoped starts also key on.
func resolveWorktreeAdmission(cwd string) (worktreeRoot, repoIdentity string, store *gatedrive.Store, ok bool) {
	client, err := gitcli.NewClient()
	if err != nil {
		return "", "", nil, false
	}
	ctx := context.Background()
	wt, err := client.DiscoverWorktree(ctx, gitcli.DiscoverOptions{InvocationPath: cwd})
	if err != nil {
		return "", "", nil, false
	}
	repo, err := client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: cwd})
	if err != nil {
		return "", "", nil, false
	}
	return wt.Root, repo.CommonDir, gatedrive.OpenStore(repo.CommonDir), true
}

// rawStaleEpochRefusal enforces the run-epoch fence at the raw launch boundary: a
// worktree slot that links a run epoch this raw launch does not carry is refused
// stale-run-epoch (an incumbent workflow owns the worktree). A raw launch carries
// rawGateEpoch (none), and until Task 9 records epochs into slots this stays
// dormant. A missing or unreadable slot is not a refusal here: the reserve is the
// authority that fails closed on an unreadable record.
func rawStaleEpochRefusal(store *gatedrive.Store, worktreeRoot string) (GateResult, bool) {
	slot, _, err := store.LoadWorktreeExecution(worktreeRoot)
	if err != nil {
		return GateResult{}, false
	}
	if slot.RunEpochID != "" && slot.RunEpochID != rawGateEpoch {
		// Decide-and-act on the single LoadWorktreeExecution read above: the
		// refusal's Cause is projected from the SAME slot record the fence
		// decided on, never a second re-read that a later-changed slot could
		// falsify. This mirrors gatedrive's own incumbentSnapshot projector;
		// that projector is unexported, so the bounded fields are copied here.
		inc := &gatedrive.IncumbentSnapshot{
			Kind:       slot.Kind,
			State:      string(slot.State),
			DriveID:    slot.DriveID,
			RawRunID:   slot.RawRunID,
			RawRunDir:  slot.RawRunDir,
			EpochOwned: slot.RunEpochID != "",
		}
		return GateResult{
			Envelope: NewEnvelope(OperationGateLaunch, ResultBlocked),
			Reason:   "stale-run-epoch",
			Cause:    incumbentRefusalLocator(inc),
		}, true
	}
	return GateResult{}, false
}

// admissionRefusalCause derives the refusal's safe incumbent locator from the
// snapshot the ownership error itself carries — the exact record the refusal
// was decided on under the admission lock. It never re-reads the slot: a later
// changed slot must not be represented as this refusal's cause. A snapshot-free
// or non-ownership error yields "".
func admissionRefusalCause(err error) string {
	oe, ok := gatedrive.AsOwnershipError(err)
	if !ok {
		return ""
	}
	return incumbentRefusalLocator(oe.Incumbent)
}

// mapAdmissionFailure classifies a worktree-admission rejection into a protocol
// result and a bounded stable reason token. A typed ownership rejection surfaces
// its kind verbatim (worktree-busy / unresolved-execution) as a blocked refusal; a
// store error (unknown schema, corrupt record, IO) is an internal error. The kind
// is the whole reason, so no argv, env, path, or token can leak.
func mapAdmissionFailure(err error) (Result, string) {
	if oe, ok := gatedrive.AsOwnershipError(err); ok {
		return ResultBlocked, string(oe.Kind)
	}
	if se, ok := gatedrive.AsStoreError(err); ok {
		return ResultInternalError, string(se.Kind)
	}
	return ResultInternalError, "admission-failed"
}

// resolveRawLaunchFailure decides the worktree slot's fate after a raw launch
// returned an error, mirroring the driver's launch-failure leg: only a proven
// never-launched resolution releases the slot (no process exists); every other
// outcome (identified, unresolved, or an unprovable census) marks it unresolved so
// it fails closed until recovery. ResolveReservation is the sole proof that an
// error response means nothing was launched.
func resolveRawLaunchFailure(svc *process.Service, store *gatedrive.Store, worktreeRoot, runRoot, token string) {
	res, err := svc.ResolveReservation(runRoot, token)
	if err == nil && res != nil && res.Disposition == "never-launched" {
		_ = store.ReleaseWorktreeExecution(worktreeRoot, token)
		return
	}
	_ = store.MarkWorktreeExecutionUnresolved(worktreeRoot, token)
}

// rawStopIfOwned drives the ownership-gated stop of a raw run this launch orphaned
// (a post-launch confirm failure) and reports whether the teardown is PROVEN — a
// stop this process performed, or a run already terminal-by-teardown. It is the
// same proof gate the driver's releaseOrUnresolveWorktree uses.
func rawStopIfOwned(svc *process.Service, runDir string) bool {
	out, err := svc.Stop(runDir, "gatedrive: raw launch confirmation failed; stopping the orphaned run")
	if err != nil {
		return false
	}
	return out.Performed || rawTeardownProven(out.State)
}

// rawTeardownProven reports whether a run state proves its process group is gone,
// matching the gate driver's own stop-teardown check (stopProvesTeardown): a
// passed, failed, stopped, or vanished run is proven; a signalled run is NOT (its
// group may still hold descendants), and a running run is obviously not.
func rawTeardownProven(st process.State) bool {
	switch st {
	case process.StatePassed, process.StateFailed, process.StateStopped, process.StateVanished:
		return true
	default:
		return false
	}
}

// releaseRawSlotForStop releases the raw worktree execution slot a stopped run
// occupied, and ONLY that slot, on PROVEN teardown (change 0375: "GateStop's proven
// teardown releases a raw slot it owns"). It resolves the worktree from the run's
// own recorded launch cwd, opens the admission store, and releases only a Kind
// "raw" slot whose recorded raw run is exactly this run, using the slot's own
// persisted reservation token. Any mismatch — a driven slot, a different run, an
// unresolvable worktree, or unproven teardown — leaves the slot untouched, so a
// stop never wrongly frees a worktree that is still live.
func releaseRawSlotForStop(svc *process.Service, runDir string, out *process.StopOutcome) {
	if !(out.Performed || rawTeardownProven(out.State)) {
		return
	}
	obs, err := svc.Observe(runDir)
	if err != nil || obs.Cwd == "" {
		return
	}
	worktreeRoot, _, store, ok := resolveWorktreeAdmission(obs.Cwd)
	if !ok {
		return
	}
	slot, _, err := store.LoadWorktreeExecution(worktreeRoot)
	if err != nil {
		return
	}
	if slot.Kind != "raw" || slot.RawRunDir != runDir {
		return
	}
	if st := string(slot.State); st != "executing" && st != "stopping" {
		return
	}
	_ = store.ReleaseWorktreeExecution(worktreeRoot, slot.ReservationToken)
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
	// A proven teardown vacates the raw worktree execution slot this run held, so
	// the worktree readmits (change 0375). A run outside any worktree, a driven
	// slot, or an unproven teardown leaves the slot untouched — fail closed.
	releaseRawSlotForStop(svc, runDir, out)
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

// observeGateRun is the ownership/terminal probe `gate cleanup` (Task 14) reads
// a run's disposability from. It is the read-only process.Observe decision
// surfaced as the raw Observation plus a mapped protocol result: a validation
// error (an unownable or malformed run slot) is returned as (nil, result,
// reason) so the caller RETAINS the directory rather than treating an unprovable
// ownership as clean. internal/cli must not import internal/process, so this
// bridge lives in the app layer beside the other gate operations.
func observeGateRun(runDir string) (*process.Observation, Result, string) {
	svc, res, reason := gateService()
	if svc == nil {
		return nil, res, reason
	}
	obs, err := svc.Observe(runDir)
	if err != nil {
		res, reason := mapGateFailure(err)
		return nil, res, reason
	}
	return obs, ResultApplied, ""
}

// gateRunRemovable reports whether an observed gate run is safely disposable: a
// passed run carries durable exact-head green evidence (its own green terminal
// record), and a stopped run carries a persisted halt/finalize stop report.
// Failed, signalled, vanished, and running runs are retained so their
// diagnostics survive. It lives here because it reads process.State spellings.
func gateRunRemovable(obs *process.Observation) bool {
	switch obs.State {
	case process.StatePassed:
		return obs.Terminal != nil
	case process.StateStopped:
		return true
	default:
		return false
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
