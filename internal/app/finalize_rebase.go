package app

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is the `finalize rebase` / `rebase-continue` / `rebase-abort`
// operations and the local-gate composition that follows a completed rebase. It
// owns the local rebase state machine's POLICY only: it reloads authoritative
// metadata, resolves the effective base and fetches its exact remote head,
// inspects the manifest-owned feature workspace, records an ownership-scoped
// rebase receipt BEFORE any Git rewrite, and drives the owned rebase primitives
// (Task 3) through begin, resolver-fed continue, and verified abort. The Git
// mechanics live in internal/gitcli; the receipt and its atomic write live in
// internal/workspace; the suite launch/observe/evidence live behind the gate
// seam. This layer wires them and maps closed outcomes; it holds no generic
// Git runner and no force-push escape hatch beyond the receipt-scoped lease the
// later publish consumes.
//
// Load-bearing properties:
//
//   - The receipt is written before the first Git mutation and keyed to the exact
//     object identities (orig head, orig remote head, base ref, base head) so a
//     response-lost success is recovered by proving the same rewrite completed —
//     never by rebasing a different head. A foreign/malformed rebase, a moved
//     base, a changed remote feature head, a dirty worktree, or an errored probe
//     is retained and blocked, never reset or adopted.
//   - A resolver report is an authored hint, not authority. Every reported path is
//     validated against the LIVE unmerged set before it is staged; a path outside
//     that set refuses. Report bodies are never echoed into a result (Global
//     Constraints: returned diagnostics redact report bodies) — only bounded,
//     safe tokens (disposition, counts, the attempt) travel out.
//   - The local suite is skipped ONLY when the rebase was a no-op and the PR body
//     carries green evidence for the EXACT current head AND the recorded command is
//     byte-equal to the currently resolved finalize.test_command (gateDecision).
//     Skipped (build-gate-off) build evidence never waives finalize; a differing or
//     empty command runs the suite. Any other evidence head runs the full suite;
//     the deferred results-only strict-ancestor
//     exemption is NOT implemented. A passed run produces evidence only through the
//     landed evidence-record path; a failed run is repair work; a run still live at
//     budget, or signaled/stopped/vanished/malformed/unavailable, is a halt — never
//     a fabricated red.

// Operation keys the rebase group records in its result envelopes.
const (
	OperationFinalizeRebase         = "finalize.rebase"
	OperationFinalizeRebaseContinue = "finalize.rebase-continue"
	OperationFinalizeRebaseAbort    = "finalize.rebase-abort"
)

// The closed set of overall rebase dispositions (spec "Rebase and local gate").
const (
	RebaseDispUnchanged  = "unchanged"  // feature already on the base; no rewrite
	RebaseDispRebased    = "rebased"    // the feature branch was rewritten onto the base
	RebaseDispConflicted = "conflicted" // stopped at unmerged paths awaiting a resolver
	RebaseDispContended  = "contended"  // a lost race the caller resolves by re-reading context
	RebaseDispBlocked    = "blocked"    // retained foreign/precondition/halt state; a human is needed
	RebaseDispFailed     = "failed"     // the rebase or the local gate failed; repair work
	RebaseDispWaiting    = "waiting"    // the local-gate slice ended; re-enter with the continuation
)

// The closed gate-composition sub-outcomes reported in GateReport.
const (
	gateComposeSkipped = "skipped" // no-op rebase + exact-head green PR evidence whose command is byte-equal to the resolved finalize.test_command
	gateComposeRan     = "ran"     // the full suite was launched and observed
)

// The stable machine reasons the rebase operations report. Message text is
// explanatory and must not be parsed.
const (
	// Identity / version refusals.
	ReasonRebaseNotImplemented = "not-implemented" // the change is not `implemented`
	ReasonRebaseVersionDrift   = "version-drift"   // the record version moved (contended)
	// Precondition refusals resolved before any Git mutation.
	ReasonRebaseWorkspaceProbe      = "workspace-probe-failed"      // Inspect returned a probe error
	ReasonRebaseWorkspaceNotReady   = "workspace-not-ready"         // not the clean, registered feature state
	ReasonRebaseWorkspaceDirty      = "workspace-dirty"             // registered but dirty/staged/untracked
	ReasonRebaseLocalHeadMismatch   = "local-head-mismatch"         // the workspace head is not the expected head
	ReasonRebaseRepoUnresolved      = "repository-unresolved"       // the GitHub repo identity did not resolve
	ReasonRebasePRProbeFailed       = "pr-probe-failed"             // the PR probe could not be established (retain)
	ReasonRebasePRNotOpen           = "pr-not-open"                 // not exactly one open PR for the feature head
	ReasonRebasePRHeadMismatch      = "pr-head-mismatch"            // the open PR names a different head
	ReasonRebasePRBaseMismatch      = "pr-base-mismatch"            // the open PR targets a base other than the effective base
	ReasonRebaseBaseFetchFailed     = "base-fetch-failed"           // fetching the base's remote head failed (retain)
	ReasonRebaseRemoteFeatureProbe  = "remote-feature-probe-failed" // probing the remote feature head failed (retain)
	ReasonRebaseRemoteFeatureAbsent = "remote-feature-absent"       // the remote feature ref is not present
	ReasonRebaseRemoteHeadMismatch  = "remote-head-mismatch"        // the remote feature head is not the expected head
	ReasonRebaseReceiptRead         = "receipt-read-failed"         // a corrupt/unreadable receipt (never a clean absence)
	ReasonRebaseReceiptWrite        = "receipt-write-failed"        // the receipt could not be written (Git untouched)
	// Git rebase state refusals / outcomes.
	ReasonRebaseForeignInProgress = "foreign-rebase-in-progress" // a rebase this call did not start is underway
	ReasonRebaseMovedBase         = "base-moved-under-receipt"   // a resumed rewrite's base head moved; not adopted
	ReasonRebaseGitFailed         = "git-rebase-failed"          // the owned rebase failed structurally
	ReasonRebaseConflicted        = "rebase-conflicted"          // stopped at conflicts; dispatch the resolver
	// Continue/abort refusals.
	ReasonRebaseNoReceipt         = "no-rebase-receipt"        // no owned attempt to continue/abort
	ReasonRebaseAttemptMismatch   = "attempt-token-mismatch"   // the supplied attempt is not the owned one
	ReasonRebaseReportChangeID    = "report-change-mismatch"   // the report names a different change
	ReasonRebaseReportDisposition = "report-not-resolved"      // continue given a non-resolved report
	ReasonRebaseReportPaths       = "report-path-not-unmerged" // a reported path is not a live unmerged path
	ReasonRebaseNoConflict        = "no-conflict-to-continue"  // continue with no rebase in progress
	ReasonRebaseAbortRestore      = "abort-restore-failed"     // abort did not restore the recorded orig head
	// Gate-composition outcomes.
	ReasonRebaseGateFailed  = "gate-failed"  // the local suite failed; repair work
	ReasonRebaseGateHalted  = "gate-halted"  // the run was not a decidable pass/fail; a human is needed
	ReasonRebaseGateWaiting = "gate-waiting" // the local-gate slice ended; re-enter with the continuation
	// Config refusals.
	ReasonRebaseGateOff = "gate-off" // finalize.gate is off; no rebase and no local retest
)

// FinalizeRebaseRequest is the closed request for `finalize rebase`. ID and
// Version pin the exact implemented record; Head is the expected local feature
// head the rebase begins from (the authorization was computed against it).
type FinalizeRebaseRequest struct {
	ID      int    `json:"id"`
	Version string `json:"version"`
	Head    string `json:"head"`
}

// ResolverReport is the versioned, bounded JSON envelope a conflict-resolver
// child returns. It is an authored HINT, decoded from a request file or stdin —
// never argv. Go verifies every mechanical claim against live Git before acting:
// the reported paths are validated against the live unmerged set, and the body
// prose (Summary, RecommendedAction) is redaction-only and never echoed into a
// result.
type ResolverReport struct {
	ChangeID          int      `json:"change_id"`
	Attempt           string   `json:"attempt"`
	Disposition       string   `json:"disposition"` // "resolved" | "stuck"
	Summary           string   `json:"summary"`
	TouchedPaths      []string `json:"touched_paths"`
	ConflictedPaths   []string `json:"conflicted_paths"`
	ObservedHead      string   `json:"observed_head"`
	ObservedBase      string   `json:"observed_base"`
	RecommendedAction string   `json:"recommended_action"`
}

// The closed resolver-report dispositions.
const (
	ResolverResolved = "resolved"
	ResolverStuck    = "stuck"
)

// GateReport is the local-gate composition detail a rebase result carries once a
// gate decision was made. Compose is "skipped" or "ran". Permit names the exact
// evidence head a skip rests on. Outcome/HaltCause/RunDir describe a run that
// executed; Evidence is the canonical build-evidence block a passed run produced.
type GateReport struct {
	Compose      string            `json:"compose"`
	Permit       string            `json:"permit,omitempty"`
	Outcome      string            `json:"outcome,omitempty"`
	HaltCause    string            `json:"halt_cause,omitempty"`
	RunDir       string            `json:"run_dir,omitempty"`
	Evidence     string            `json:"evidence,omitempty"`
	Continuation *GateContinuation `json:"continuation,omitempty"`
}

// FinalizeRebaseResult is the protocol-v1 document the three rebase operations
// return. It names identity, the closed disposition, the exact heads/base the
// rebase acted on, the conflicted paths (on a conflict), the owned attempt token
// (so the caller can continue/abort), and the gate report once composed. A
// refusal carries a stable reason and message; a shape refusal carries findings.
// It holds no authored bytes and no report prose.
type FinalizeRebaseResult struct {
	Envelope
	ID            int             `json:"id,omitempty"`
	Disposition   string          `json:"disposition,omitempty"`
	Head          string          `json:"head,omitempty"`
	OrigHead      string          `json:"orig_head,omitempty"`
	Base          string          `json:"base,omitempty"`
	BaseHead      string          `json:"base_head,omitempty"`
	Attempt       string          `json:"attempt,omitempty"`
	UnmergedPaths []string        `json:"unmerged_paths"`
	Gate          *GateReport     `json:"gate,omitempty"`
	Reason        string          `json:"reason,omitempty"`
	Message       string          `json:"message,omitempty"`
	Findings      []StatusFinding `json:"findings"`
}

// HumanText renders a one-line summary naming identity, disposition, and the
// gate compose only — never a report body.
func (r FinalizeRebaseResult) HumanText() string {
	if r.Result == ResultApplied || r.Result == ResultNoOp {
		s := fmt.Sprintf("%s: change %04d %s", r.Operation, r.ID, r.Disposition)
		if r.Gate != nil {
			s += " (gate " + r.Gate.Compose
			if r.Gate.Outcome != "" {
				s += ":" + r.Gate.Outcome
			}
			s += ")"
		}
		return s
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// newRebaseResult stamps the envelope for opKey and normalizes the collections so
// a nil never leaks into the protocol document.
func newRebaseResult(opKey string, result Result, out FinalizeRebaseResult) FinalizeRebaseResult {
	out.Envelope = NewEnvelope(opKey, result)
	if out.UnmergedPaths == nil {
		out.UnmergedPaths = []string{}
	}
	if out.Findings == nil {
		out.Findings = []StatusFinding{}
	}
	return out
}

// rebaseRefusal builds a refusing result carrying a stable reason, message, and
// disposition (no gate report, no heads).
func rebaseRefusal(opKey string, result Result, disposition, reason, message string, id int) FinalizeRebaseResult {
	return newRebaseResult(opKey, result, FinalizeRebaseResult{
		ID: id, Disposition: disposition, Reason: reason, Message: message,
	})
}

// ---------------------------------------------------------------------------
// Local-gate seam
// ---------------------------------------------------------------------------

// FinalizeGateOutcome is the closed disposition the gate seam maps a terminal (or
// budget-exhausted) observation to.
type FinalizeGateOutcome string

const (
	// FinalizeGatePassed: the run reached a green terminal; the seam produced the
	// immutable evidence block through the landed evidence-record path.
	FinalizeGatePassed FinalizeGateOutcome = "passed"
	// FinalizeGateFailed: the run reached a red terminal; the head must be repaired.
	FinalizeGateFailed FinalizeGateOutcome = "failed"
	// FinalizeGateHalted: the run was not a decidable pass/fail — still live at
	// budget, signaled, stopped, vanished, or its state was unobservable. Never a
	// fabricated red; a human is needed.
	FinalizeGateHalted FinalizeGateOutcome = "halted"
	// FinalizeGateWaiting: the driver's slice ended while the suite is still live.
	// It is NONTERMINAL — it mints no evidence and is not repair work. The caller
	// carries the opaque continuation and re-enters the same local-gate phase to
	// advance the SAME drive again, never re-launching the suite (spec "Finalize").
	FinalizeGateWaiting FinalizeGateOutcome = "waiting"
)

// GateContinuation is the opaque handle a WAITING slice hands back so the caller
// can re-enter the local gate and advance the SAME drive across slices — without
// re-launching the suite or repeating the already-completed rebase. It carries
// the drive id and owner generation only: never an argv, an environment value, a
// worktree diff, or a run dir. An empty DriveID means "no drive yet" — start one.
type GateContinuation struct {
	DriveID    string `json:"drive_id,omitempty"`
	Generation string `json:"generation,omitempty"`
}

// The closed halt causes a halted gate carries. They are the exact non-pass/fail
// observations the spec enumerates.
const (
	GateHaltRunningAtBudget = "running-at-budget"
	GateHaltSignaled        = "signaled"
	GateHaltStopped         = "stopped"
	GateHaltVanished        = "vanished"
	GateHaltMalformed       = "malformed"
	GateHaltUnavailable     = "unavailable"
)

// LocalGateRequest names the run the gate seam launches and observes.
type LocalGateRequest struct {
	RepoDir      string
	ID           int
	WorkspaceDir string
	Head         string
	// Continuation, when set (non-empty DriveID), resumes the drive a prior
	// WAITING slice returned — the seam advances that SAME drive rather than
	// launching a new suite. Empty on the first slice.
	Continuation GateContinuation
}

// LocalGateResult is the gate seam's closed return. On passed, Evidence carries
// the canonical build-evidence block; on halted, HaltCause names the exact
// non-pass/fail observation. RunDir is the supervised run's directory.
type LocalGateResult struct {
	Outcome   FinalizeGateOutcome
	HaltCause string
	Evidence  string
	RunDir    string
	// Continuation is populated on a WAITING outcome only: the opaque handle the
	// caller re-presents to advance the same drive on the next slice.
	Continuation GateContinuation
}

// FinalizeGate runs the resolved local suite in the feature workspace, observes
// it to a terminal within the observation budget, and maps the outcome. A
// returned error is an unrecoverable seam failure the caller treats as a halt
// (unavailable) — never a fabricated red.
type FinalizeGate interface {
	RunLocalGate(ctx context.Context, req LocalGateRequest) (LocalGateResult, error)
}

// ---------------------------------------------------------------------------
// gateDecision — pure local-gate skip policy
// ---------------------------------------------------------------------------

// gateDecision decides whether the local suite may be skipped after a completed
// rebase. It skips ONLY when the rebase was a no-op AND the PR body carries GREEN
// evidence for the EXACT current head AND the recorded command is byte-equal to
// the currently resolved finalize.test_command (with both non-empty); the permit
// names that head. Differing commands are different assertions even at the same
// SHA, so a green record certifying another command runs the full suite; an
// empty-vs-empty command is a vacuous match that must NOT skip; and skipped
// (build-gate-off) build evidence — which reaches here with evidenceGreen false —
// never waives finalize's local gate. Any other case — a real rewrite, a moved
// head, stale (different-head) evidence, or absent/non-green evidence — runs the
// full suite. There is no strict-ancestor exemption (spec: it is not
// implemented). The comparisons are full-length string equality; the caller
// normalizes both heads to lowercase.
func gateDecision(noop bool, evidenceHead, currentHead string, evidenceGreen bool,
	evidenceCommand, resolvedCommand string) (skip bool, permit string) {
	if noop && evidenceGreen && evidenceHead != "" && evidenceHead == currentHead &&
		evidenceCommand != "" && evidenceCommand == resolvedCommand {
		return true, evidenceHead
	}
	return false, ""
}

// ---------------------------------------------------------------------------
// rebase context
// ---------------------------------------------------------------------------

// rebaseContext bundles the authoritative facts a rebase operation resolves once
// from the pinned metadata and the live workspace: the discovered repository, the
// change and its exact record version, the validated workspace target, the
// read-only inspection, and the derived checkout and metadata directories.
type rebaseContext struct {
	repo    gitcli.Repository
	change  domain.Change
	version string
	base    domain.EffectiveBase
	target  workspace.Target
	insp    workspace.Inspection
	wsDir   string
	metaDir string
}

// loadRebaseContext resolves the shared rebase context or a typed refusal. It
// reuses the workspace context/target resolution (so the effective-base rule and
// the target validation are one copy), then inspects the workspace read-only.
func loadRebaseContext(ctx context.Context, deps FinalizeDeps, repoDir string, opKey string, id int) (*rebaseContext, *FinalizeRebaseResult) {
	wc, refusal := loadWorkspaceContext(ctx, deps.Planning, repoDir, id, opKey)
	if refusal != nil {
		r := translateWorkspaceRefusal(opKey, *refusal)
		return nil, &r
	}
	target, tRefusal := resolveWorkspaceTarget(opKey, wc)
	if tRefusal != nil {
		r := translateWorkspaceRefusal(opKey, *tRefusal)
		return nil, &r
	}
	insp, err := deps.Workspace.Inspect(ctx, workspace.InspectRequest{Repository: wc.repo, Target: target})
	if err != nil {
		r := rebaseRefusal(opKey, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseWorkspaceProbe, err.Error(), id)
		return nil, &r
	}
	return &rebaseContext{
		repo:    wc.repo,
		change:  wc.change,
		version: wc.version,
		base:    wc.base,
		target:  target,
		insp:    insp,
		wsDir:   insp.Path,
		metaDir: workspace.MetaDir(wc.repo.CommonDir, target.FeatureRef),
	}, nil
}

// translateWorkspaceRefusal maps a workspace-shaped refusal onto a rebase result,
// preserving the protocol Result/Reason/Message and deriving a rebase disposition
// from the Result class.
func translateWorkspaceRefusal(opKey string, w WorkspaceOpResult) FinalizeRebaseResult {
	disp := RebaseDispBlocked
	switch w.Result {
	case ResultContended:
		disp = RebaseDispContended
	case ResultInvalidInput:
		disp = ""
	}
	return rebaseRefusal(opKey, w.Result, disp, w.Reason, w.Message, w.ID)
}

// requireReadyWorkspace refuses unless the workspace is the clean, registered
// feature state at the expected head. A dirty workspace, a foreign/mismatched
// state, or a head drift is retained (blocked/contended) with the receipt never
// written and Git untouched.
func requireReadyWorkspace(opKey string, rc *rebaseContext, expectedHead string) *FinalizeRebaseResult {
	switch rc.insp.Kind {
	case workspace.StateReady:
		// clean and registered; fall through
	case workspace.StateDirty:
		r := rebaseRefusal(opKey, ResultBlocked, RebaseDispBlocked, ReasonRebaseWorkspaceDirty,
			"the feature workspace has uncommitted changes; a rebase requires a clean tree", int(rc.change.ID()))
		return &r
	default:
		r := rebaseRefusal(opKey, ResultBlocked, RebaseDispBlocked, ReasonRebaseWorkspaceNotReady,
			fmt.Sprintf("the feature workspace is %q, not the clean registered feature state", rc.insp.Kind), int(rc.change.ID()))
		return &r
	}
	if string(rc.insp.HeadCommit) != expectedHead {
		r := rebaseRefusal(opKey, ResultContended, RebaseDispContended, ReasonRebaseLocalHeadMismatch,
			"the workspace head moved under the authorization; re-read context finalize", int(rc.change.ID()))
		return &r
	}
	return nil
}

// ---------------------------------------------------------------------------
// finalize rebase (begin)
// ---------------------------------------------------------------------------

// FinalizeRebase begins the local rebase of an implemented change's feature
// branch onto the fresh remote head of its effective base, records the owned
// receipt before any Git mutation, and — on a completed rebase — composes the
// local gate. A response-lost success is recovered idempotently from the receipt,
// the owned refs, and live Git; a foreign/malformed rebase, a moved base, or an
// errored probe is retained and blocked, never reset or adopted.
func FinalizeRebase(ctx context.Context, deps FinalizeDeps, repoDir string, req FinalizeRebaseRequest) FinalizeRebaseResult {
	op := OperationFinalizeRebase
	if findings := validateRebaseShape(req); len(findings) > 0 {
		return newRebaseResult(op, ResultInvalidInput, FinalizeRebaseResult{ID: req.ID, Findings: findings})
	}

	// Capability preflight and the resolved gate mode, both before any Git effect.
	// A configuration that actively requests a deferred capability (ci/both gate
	// included) is unsupported-config; an explicit `off` gate is the opt-out — no
	// rebase and no local retest — so the operation is a truthful no-op.
	pin, err := deps.Planning.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return rebaseRefusal(op, result, RebaseDispBlocked, reason, err.Error(), req.ID)
	}
	if decision := config.PreflightMutation(&pin.Config); !decision.Allowed {
		return rebaseRefusal(op, ResultUnsupportedConfig, "", ReasonDeferredCapRequested,
			"configuration actively requests a deferred capability docket does not ship in this version ("+
				strings.Join(blockerPaths(decision.Blockers), ", ")+"); withdraw it before any mutation", req.ID)
	}
	if pin.Config.Effective.Finalize.Gate.Value == "off" {
		return newRebaseResult(op, ResultNoOp, FinalizeRebaseResult{
			ID: req.ID, Disposition: RebaseDispUnchanged, Reason: ReasonRebaseGateOff,
			Message: "finalize.gate is off; no rebase and no local retest",
		})
	}

	rc, refusal := loadRebaseContext(ctx, deps, repoDir, op, req.ID)
	if refusal != nil {
		return *refusal
	}
	id := int(rc.change.ID())

	// Identity and version: exactly the implemented record at the pinned version.
	if rc.change.Status() != domain.StatusImplemented {
		return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseNotImplemented,
			fmt.Sprintf("change %04d is %q, not implemented; there is nothing to rebase", id, rc.change.RawStatus()), id)
	}
	if rc.version != req.Version {
		return rebaseRefusal(op, ResultContended, RebaseDispContended, ReasonRebaseVersionDrift,
			"the change record moved since the submitted version; re-read context finalize", id)
	}

	// Verified open PR: exactly one for the feature head, naming the head and
	// targeting the effective base. A probe error is unknown — retain.
	pr, prRefusal := probeRebasePR(ctx, deps, repoDir, rc, req.Head)
	if prRefusal != nil {
		return *prRefusal
	}

	// Fetch the base's exact remote head and the remote feature head. Both are
	// remote probes whose errors are retained; an absent remote feature ref blocks.
	baseHead, remoteHead, headRefusal := probeRebaseHeads(ctx, deps, rc)
	if headRefusal != nil {
		return *headRefusal
	}

	ownedPrefix := ownedRefPrefixFor(id)

	// Response-loss recovery: an owned receipt already present means a rewrite began
	// under this repo/change. If it matches the intended rewrite and the base has
	// not moved, adopt its completed/mid-flight state instead of rebasing again.
	existing, present, rerr := deps.Workspace.ReadRebaseReceipt(ctx, rc.metaDir)
	if rerr != nil {
		return rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseReceiptRead, rerr.Error(), id)
	}
	if present {
		return recoverFromReceipt(ctx, deps, repoDir, rc, req, pr, baseHead, remoteHead, ownedPrefix, existing)
	}

	// Fresh attempt (no owned receipt). Guard against any foreign in-progress rebase
	// FIRST — a rebase this call did not start is retained as such, never demoted to
	// a mere dirty-tree refusal — and before writing a receipt, so a blocked
	// precondition leaves Git and the receipt untouched.
	state, err := deps.Planning.Client.RebaseState(ctx, rc.wsDir)
	if err != nil {
		return rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseWorkspaceProbe, err.Error(), id)
	}
	if state.Disposition != gitcli.RebaseUnchanged {
		return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseForeignInProgress,
			"a rebase this operation did not start is already in progress; retained, not adopted", id)
	}

	// Only now — never on the recovery path, whose local head is the rewritten
	// head, not the pre-rewrite expected head — require the clean, registered
	// feature workspace at exactly the expected local head, and the published
	// remote feature head at that same head. Local, remote, and PR feature head must
	// all agree before a rewrite begins.
	if r := requireReadyWorkspace(op, rc, req.Head); r != nil {
		return *r
	}
	if string(remoteHead) != req.Head {
		return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseRemoteHeadMismatch,
			"the remote feature head is not the expected head; local, remote, and PR heads must agree before a rebase", id)
	}

	attempt := newRebaseAttempt(deps, baseHead)
	receipt := workspace.RebaseReceipt{
		RepoIdentity:   rc.repo.CommonDir,
		ChangeID:       strconv.Itoa(id),
		OrigHead:       string(rc.insp.HeadCommit),
		OrigRemoteHead: string(remoteHead),
		BaseRef:        string(rc.target.BaseRef),
		BaseHead:       string(baseHead),
		Attempt:        attempt,
		CreatedUTC:     deps.Planning.Clock.Now().UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if err := deps.Workspace.WriteRebaseReceipt(ctx, rc.metaDir, receipt); err != nil {
		return rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseReceiptWrite, err.Error(), id)
	}

	// The receipt is durable; the first Git mutation may now run.
	status, err := deps.Planning.Client.BeginRebase(ctx, rc.wsDir, gitcli.ObjectID(req.Head), baseHead, ownedPrefix)
	if err != nil {
		return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseGitFailed, err.Error(), id)
	}
	return mapBegunRebase(ctx, deps, repoDir, op, rc, pr, receipt, status)
}

// mapBegunRebase maps a completed BeginRebase/continue status to a result: a
// conflict surfaces its paths for the resolver; a completed rebase composes the
// local gate; a structural failure is retained. The attempt, base head, and orig
// head all derive from the owned receipt (rec) the caller just wrote or recovered
// — never re-read from rc.insp — so the fresh and recovery paths thread one value.
func mapBegunRebase(ctx context.Context, deps FinalizeDeps, repoDir, op string, rc *rebaseContext, pr githubcli.PullRequest, rec workspace.RebaseReceipt, status gitcli.RebaseStatus) FinalizeRebaseResult {
	id := int(rc.change.ID())
	switch status.Disposition {
	case gitcli.RebaseConflicted:
		return newRebaseResult(op, ResultApplied, FinalizeRebaseResult{
			ID: id, Disposition: RebaseDispConflicted, Head: string(status.HeadOID),
			OrigHead: rec.OrigHead, Base: rc.base.Branch, BaseHead: rec.BaseHead,
			Attempt: rec.Attempt, UnmergedPaths: status.UnmergedPaths, Reason: ReasonRebaseConflicted,
			Message: fmt.Sprintf("the rebase stopped at %d conflicted path(s); dispatch the resolver", len(status.UnmergedPaths)),
		})
	case gitcli.RebaseUnchanged, gitcli.RebaseRebased:
		noop := status.Disposition == gitcli.RebaseUnchanged
		return composeLocalGate(ctx, deps, repoDir, op, rc, pr, rec, status.HeadOID, noop)
	case gitcli.RebaseInProgressForeign:
		return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseForeignInProgress,
			"a foreign rebase is in progress; retained, not adopted", id)
	default: // RebaseFailed
		return rebaseRefusal(op, ResultBlocked, RebaseDispFailed, ReasonRebaseGitFailed,
			"the owned rebase failed structurally; the attempt is retained for abort", id)
	}
}

// recoverFromReceipt adopts an existing owned attempt when the receipt matches the
// intended rewrite and the base has not moved. It never rebases a different head:
// a completed rewrite (no in-progress rebase, head descends the base) returns the
// same outcome; a still-conflicted attempt returns its live conflicts; a moved
// base or a mismatched receipt is retained and blocked.
func recoverFromReceipt(ctx context.Context, deps FinalizeDeps, repoDir string, rc *rebaseContext, req FinalizeRebaseRequest, pr githubcli.PullRequest, baseHead, remoteHead gitcli.ObjectID, ownedPrefix string, rec workspace.RebaseReceipt) FinalizeRebaseResult {
	op := OperationFinalizeRebase
	id := int(rc.change.ID())

	// The receipt must describe THIS repo/change/base. A base that moved since the
	// receipt is never silently adopted or reset.
	if rec.RepoIdentity != rc.repo.CommonDir || rec.ChangeID != strconv.Itoa(id) || rec.BaseRef != string(rc.target.BaseRef) {
		return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseForeignInProgress,
			"the existing rebase receipt does not describe this change; retained, not adopted", id)
	}
	if rec.BaseHead != string(baseHead) {
		return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseMovedBase,
			"the effective base moved since the recorded attempt; retained, not adopted", id)
	}

	state, err := deps.Planning.Client.RebaseState(ctx, rc.wsDir)
	if err != nil {
		return rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseWorkspaceProbe, err.Error(), id)
	}
	switch state.Disposition {
	case gitcli.RebaseConflicted:
		// The owned attempt is still mid-conflict; surface the live conflicts.
		return newRebaseResult(op, ResultApplied, FinalizeRebaseResult{
			ID: id, Disposition: RebaseDispConflicted, Head: string(state.HeadOID),
			OrigHead: rec.OrigHead, Base: rc.base.Branch, BaseHead: string(baseHead),
			Attempt: rec.Attempt, UnmergedPaths: state.UnmergedPaths, Reason: ReasonRebaseConflicted,
			Message: fmt.Sprintf("the owned rebase is stopped at %d conflicted path(s); dispatch the resolver", len(state.UnmergedPaths)),
		})
	case gitcli.RebaseInProgressForeign:
		return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseForeignInProgress,
			"a foreign rebase is in progress over the owned attempt; retained, not adopted", id)
	}

	// No rebase in progress. Prove the completed state: the local head must descend
	// from the base head (the rewrite landed). Then the disposition is derivable
	// from whether the head is still the recorded orig (a no-op) or a rewrite.
	localHead := rc.insp.HeadCommit
	descends, err := deps.Planning.Client.IsAncestor(ctx, rc.repo, baseHead, localHead)
	if err != nil {
		return rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseWorkspaceProbe, err.Error(), id)
	}
	if !descends {
		// The receipt exists but the workspace does not carry a completed rewrite onto
		// this base: a partial/abandoned attempt. Retain rather than restart blindly.
		return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseForeignInProgress,
			"an owned attempt exists but the workspace head does not descend the base; retained for abort", id)
	}
	noop := string(localHead) == rec.OrigHead
	// The receipt's pair may be set (a prior WAITING to resume) or empty (a crash
	// before WAITING, or a cleared terminal); composeLocalGate derives the
	// continuation from rec, so both are correct as-is.
	return composeLocalGate(ctx, deps, repoDir, op, rc, pr, rec, localHead, noop)
}

// ---------------------------------------------------------------------------
// finalize rebase-continue
// ---------------------------------------------------------------------------

// FinalizeRebaseContinue validates a resolver report against the live rebase
// state, stages exactly the reported (and verified) paths, continues the owned
// rebase non-interactively, and returns the next conflict or composes the local
// gate on completion. The attempt token must match the owned receipt; a report
// that names a change other than the receipt's, a non-resolved disposition, or a
// path outside the live unmerged set refuses without touching Git.
func FinalizeRebaseContinue(ctx context.Context, deps FinalizeDeps, repoDir string, id int, attempt string, report ResolverReport) FinalizeRebaseResult {
	op := OperationFinalizeRebaseContinue
	rc, refusal := loadRebaseContext(ctx, deps, repoDir, op, id)
	if refusal != nil {
		return *refusal
	}

	rec, refusal := requireOwnedAttempt(ctx, deps, op, rc, attempt)
	if refusal != nil {
		return *refusal
	}

	if report.ChangeID != id {
		return rebaseRefusal(op, ResultInvalidInput, "", ReasonRebaseReportChangeID,
			"the resolver report names a change other than the one being continued", id)
	}
	if report.Disposition != ResolverResolved {
		return rebaseRefusal(op, ResultInvalidInput, "", ReasonRebaseReportDisposition,
			"a non-resolved report cannot continue a rebase; route an ambiguous report through rebase-abort", id)
	}

	state, err := deps.Planning.Client.RebaseState(ctx, rc.wsDir)
	if err != nil {
		return rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseWorkspaceProbe, err.Error(), id)
	}
	if state.Disposition != gitcli.RebaseConflicted {
		return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseNoConflict,
			fmt.Sprintf("no resolvable conflict is in progress (state %q); nothing to continue", state.Disposition), id)
	}

	// Validate every reported path against the LIVE unmerged set before staging.
	live := make(map[string]bool, len(state.UnmergedPaths))
	for _, p := range state.UnmergedPaths {
		live[p] = true
	}
	stage := report.ConflictedPaths
	if len(stage) == 0 {
		return rebaseRefusal(op, ResultInvalidInput, "", ReasonRebaseReportPaths,
			"the resolver report names no resolved conflicted paths to stage", id)
	}
	for _, p := range stage {
		if !live[p] {
			return rebaseRefusal(op, ResultInvalidInput, "", ReasonRebaseReportPaths,
				"a reported path is not in the live unmerged set; refusing to stage it", id)
		}
	}

	status, err := deps.Planning.Client.StageAndContinueRebase(ctx, rc.wsDir, stage)
	if err != nil {
		return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseGitFailed, err.Error(), id)
	}

	// A continue that completes was never a no-op (a conflict implies a rewrite), so
	// the gate always runs. Probe the PR only for the result's identity fields.
	pr, _ := probeRebasePR(ctx, deps, repoDir, rc, string(rc.insp.HeadCommit))
	if pr.Number == 0 {
		pr = githubcli.PullRequest{}
	}
	return mapContinuedRebase(ctx, deps, repoDir, op, rc, pr, rec, status)
}

// mapContinuedRebase maps a StageAndContinueRebase status: another conflict
// surfaces its paths; a completed rebase composes the gate (never a no-op). The
// base head, attempt, and orig head derive from the owned receipt (rec).
func mapContinuedRebase(ctx context.Context, deps FinalizeDeps, repoDir, op string, rc *rebaseContext, pr githubcli.PullRequest, rec workspace.RebaseReceipt, status gitcli.RebaseStatus) FinalizeRebaseResult {
	id := int(rc.change.ID())
	switch status.Disposition {
	case gitcli.RebaseConflicted:
		return newRebaseResult(op, ResultApplied, FinalizeRebaseResult{
			ID: id, Disposition: RebaseDispConflicted, Head: string(status.HeadOID),
			Base: rc.base.Branch, BaseHead: rec.BaseHead, Attempt: rec.Attempt,
			UnmergedPaths: status.UnmergedPaths, Reason: ReasonRebaseConflicted,
			Message: fmt.Sprintf("the rebase stopped at %d further conflicted path(s); dispatch the resolver", len(status.UnmergedPaths)),
		})
	case gitcli.RebaseUnchanged, gitcli.RebaseRebased:
		// The rebase-continue path only runs on a mid-conflict receipt, so the pair is
		// empty by construction and composeLocalGate starts a fresh drive; a WAITING
		// slice is resumed by re-entering FinalizeRebase (which recovers from the
		// receipt), not this operation.
		return composeLocalGate(ctx, deps, repoDir, op, rc, pr, rec, status.HeadOID, false)
	default: // RebaseInProgressForeign / RebaseFailed
		return rebaseRefusal(op, ResultBlocked, RebaseDispFailed, ReasonRebaseGitFailed,
			"the owned rebase-continue did not reach a resolvable state; retained for abort", id)
	}
}

// ---------------------------------------------------------------------------
// finalize rebase-abort
// ---------------------------------------------------------------------------

// FinalizeRebaseAbort proves the owned attempt, aborts the rebase, verifies the
// worktree returned to the recorded original head, clears the receipt and owned
// refs, and returns a blocked disposition recommending the human-authored
// finalize-block. An ambiguous resolver report routes here; the report body is
// never echoed — only the bounded recommendation to record a finalize block.
func FinalizeRebaseAbort(ctx context.Context, deps FinalizeDeps, repoDir string, id int, attempt string, report ResolverReport) FinalizeRebaseResult {
	op := OperationFinalizeRebaseAbort
	rc, refusal := loadRebaseContext(ctx, deps, repoDir, op, id)
	if refusal != nil {
		return *refusal
	}
	rec, refusal := requireOwnedAttempt(ctx, deps, op, rc, attempt)
	if refusal != nil {
		return *refusal
	}
	if report.ChangeID != 0 && report.ChangeID != id {
		return rebaseRefusal(op, ResultInvalidInput, "", ReasonRebaseReportChangeID,
			"the resolver report names a change other than the one being aborted", id)
	}

	origHead := gitcli.ObjectID(rec.OrigHead)
	if err := deps.Planning.Client.AbortRebase(ctx, rc.wsDir, origHead); err != nil {
		return rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseAbortRestore, err.Error(), id)
	}

	// The rewrite is undone and the orig head restored. Clear the owned scratch: the
	// receipt and the two anchor refs. A cleanup failure here is non-fatal to the
	// abort proof — the head is already restored — so it is surfaced, not fatal.
	prefix := ownedRefPrefixFor(id)
	_ = deps.Planning.Client.DeleteOwnedRef(ctx, rc.repo, gitcli.RefName(prefix+"/orig"))
	_ = deps.Planning.Client.DeleteOwnedRef(ctx, rc.repo, gitcli.RefName(prefix+"/base"))
	_ = deps.Workspace.ClearRebaseReceipt(ctx, rc.metaDir)

	return newRebaseResult(op, ResultApplied, FinalizeRebaseResult{
		ID: id, Disposition: RebaseDispBlocked, Head: rec.OrigHead, OrigHead: rec.OrigHead,
		Attempt: rec.Attempt, Reason: ReasonRebaseConflicted,
		Message: "the rebase was aborted and the original head restored; record a finalize block for the human before merge",
	})
}

// requireOwnedAttempt reads the owned receipt and gates it on presence and an
// exact attempt-token match. A missing receipt or a token mismatch refuses; a
// corrupt receipt is an error, never a clean absence.
func requireOwnedAttempt(ctx context.Context, deps FinalizeDeps, op string, rc *rebaseContext, attempt string) (workspace.RebaseReceipt, *FinalizeRebaseResult) {
	id := int(rc.change.ID())
	rec, present, err := deps.Workspace.ReadRebaseReceipt(ctx, rc.metaDir)
	if err != nil {
		r := rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseReceiptRead, err.Error(), id)
		return workspace.RebaseReceipt{}, &r
	}
	if !present {
		r := rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseNoReceipt,
			"no owned rebase attempt is recorded for this change; nothing to continue or abort", id)
		return workspace.RebaseReceipt{}, &r
	}
	if rec.ChangeID != strconv.Itoa(id) || rec.Attempt != attempt {
		r := rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseAttemptMismatch,
			"the supplied attempt token does not match the owned rebase receipt", id)
		return workspace.RebaseReceipt{}, &r
	}
	return rec, nil
}

// ---------------------------------------------------------------------------
// gate composition
// ---------------------------------------------------------------------------

// composeLocalGate decides skip-or-run after a completed rebase and maps the gate
// outcome. It skips only on a no-op rebase with exact-head green PR evidence whose
// recorded command is byte-equal to the resolved finalize.test_command;
// otherwise it runs the full suite through the gate seam. A passed run carries the
// evidence block; a failed run is repair work (failed); a halt is retained
// (blocked) — never a fabricated red.
func composeLocalGate(ctx context.Context, deps FinalizeDeps, repoDir, op string, rc *rebaseContext, pr githubcli.PullRequest, rec workspace.RebaseReceipt, head gitcli.ObjectID, noop bool) FinalizeRebaseResult {
	id := int(rc.change.ID())
	// The attempt, base head, orig head, and gate continuation all derive from the
	// owned receipt this composition runs under — the callers already guarantee they
	// agree with the live rewrite (change 0396).
	attempt := rec.Attempt
	baseHead := gitcli.ObjectID(rec.BaseHead)
	origHead := gitcli.ObjectID(rec.OrigHead)
	cont := GateContinuation{DriveID: rec.GateDriveID, Generation: rec.GateOwnerGeneration}
	disposition := RebaseDispRebased
	if noop {
		disposition = RebaseDispUnchanged
	}
	currentHead := strings.ToLower(string(head))

	evidenceHead, evidenceCommand, evidenceGreen := prBodyEvidence(pr)
	resolvedCommand := resolvedFinalizeCommand(ctx, deps, repoDir)
	skip, permit := false, ""
	if cont.DriveID == "" {
		skip, permit = gateDecision(noop, evidenceHead, currentHead, evidenceGreen, evidenceCommand, resolvedCommand)
	}
	// A recorded live continuation means a drive is already running for this
	// attempt: it must be advanced to a terminal, never skipped past — a skip
	// here would strand the drive and wedge the receipt's pair (the pair is
	// presence-encoded state; every transition out must clear it).
	base := FinalizeRebaseResult{
		ID: id, Disposition: disposition, Head: string(head), OrigHead: string(origHead),
		Base: rc.base.Branch, BaseHead: string(baseHead), Attempt: attempt,
	}
	if skip {
		base.Gate = &GateReport{Compose: gateComposeSkipped, Permit: permit}
		result := ResultNoOp
		if !noop {
			result = ResultApplied
		}
		return newRebaseResult(op, result, base)
	}

	if deps.Gate == nil {
		return rebaseRefusal(op, ResultInternalError, RebaseDispBlocked, ReasonRebaseGateHalted,
			"no local-gate seam is wired; cannot run the suite", id)
	}
	gres, gerr := deps.Gate.RunLocalGate(ctx, LocalGateRequest{
		RepoDir: repoDir, ID: id, WorkspaceDir: rc.wsDir, Head: string(head), Continuation: cont,
	})
	if gerr != nil {
		base.Disposition = RebaseDispBlocked
		base.Gate = &GateReport{Compose: gateComposeRan, Outcome: string(FinalizeGateHalted), HaltCause: GateHaltUnavailable}
		base.Reason = ReasonRebaseGateHalted
		base.Message = "the local gate could not be established; retained, no red fabricated"
		out := newRebaseResult(op, ResultBlocked, base)
		clearGateContinuation(ctx, deps, rc, rec, &out)
		return out
	}
	base.Gate = &GateReport{Compose: gateComposeRan, Outcome: string(gres.Outcome), RunDir: gres.RunDir}
	switch gres.Outcome {
	case FinalizeGatePassed:
		base.Gate.Evidence = gres.Evidence
		out := newRebaseResult(op, ResultApplied, base)
		clearGateContinuation(ctx, deps, rc, rec, &out)
		return out
	case FinalizeGateWaiting:
		// Persist the continuation into the owned receipt so the WAITING re-entry is
		// the identical finalize.rebase invocation: the recovery path reads the pair
		// and advances the SAME drive (change 0396). The owner generation is
		// receipt-private — it never enters the document.
		c := gres.Continuation
		updated := rec
		updated.GateDriveID, updated.GateOwnerGeneration = c.DriveID, c.Generation
		if werr := deps.Workspace.WriteRebaseReceipt(ctx, rc.metaDir, updated); werr != nil {
			base.Disposition = RebaseDispBlocked
			base.Gate.RunDir = ""
			base.Reason = ReasonRebaseReceiptWrite
			base.Message = fmt.Sprintf(
				"the WAITING continuation could not be persisted to the rebase receipt (drive %s still running): %v",
				c.DriveID, werr)
			return newRebaseResult(op, ResultExternalFailed, base)
		}
		// A nonterminal slice: the suite is still running under the detached
		// supervisor. Surface the opaque continuation so the caller re-enters this
		// same phase and advances the SAME drive. Waiting mints no evidence and is
		// not repair work; no run dir is exposed on a nonterminal outcome.
		base.Disposition = RebaseDispWaiting
		base.Gate.RunDir = ""
		base.Gate.Continuation = &c
		base.Reason = ReasonRebaseGateWaiting
		base.Message = "the local-gate slice ended while the suite is still running; re-enter with the continuation to advance the same drive"
		return newRebaseResult(op, ResultApplied, base)
	case FinalizeGateFailed:
		base.Disposition = RebaseDispFailed
		base.Reason = ReasonRebaseGateFailed
		base.Message = "the local suite failed at the rebased head; this is repair work"
		out := newRebaseResult(op, ResultGateFailed, base)
		clearGateContinuation(ctx, deps, rc, rec, &out)
		return out
	default: // FinalizeGateHalted
		base.Disposition = RebaseDispBlocked
		base.Gate.HaltCause = gres.HaltCause
		base.Reason = ReasonRebaseGateHalted
		base.Message = "the local gate did not reach a decidable pass/fail; retained, no red fabricated"
		out := newRebaseResult(op, ResultBlocked, base)
		clearGateContinuation(ctx, deps, rc, rec, &out)
		return out
	}
}

// clearGateContinuation rewrites the receipt with the gate pair emptied after a
// terminal gate outcome, so a dead continuation never wedges the receipt: the
// driver's Advance on a terminal drive could never mint evidence again (its run
// root is removed at the terminal). Best-effort by design: the outcome is already
// mapped, so a clear failure is reported in the result message and does not change
// the disposition — the next re-run's Advance on the terminal drive halts and the
// clear is retried then.
func clearGateContinuation(ctx context.Context, deps FinalizeDeps, rc *rebaseContext, rec workspace.RebaseReceipt, res *FinalizeRebaseResult) {
	if rec.GateDriveID == "" && rec.GateOwnerGeneration == "" {
		return
	}
	updated := rec
	updated.GateDriveID, updated.GateOwnerGeneration = "", ""
	if err := deps.Workspace.WriteRebaseReceipt(ctx, rc.metaDir, updated); err != nil {
		res.Message = strings.TrimSpace(res.Message +
			" (clearing the gate continuation from the rebase receipt failed: " + err.Error() + ")")
	}
}

// prBodyEvidence extracts the exact-head evidence facts from a PR body: the
// certified head, the recorded command, and whether the record is GREEN (result
// == evidence.ResultGreen). A skipped (build-gate-off) record parses but is not
// green and carries no command, so it can never waive finalize's local gate
// through gateDecision. A body with no block, or one that does not parse, is not
// green.
func prBodyEvidence(pr githubcli.PullRequest) (evidenceHead, evidenceCommand string, green bool) {
	if pr.Body == "" {
		return "", "", false
	}
	rec, err := evidence.Extract([]byte(pr.Body))
	if err != nil {
		return "", "", false
	}
	return rec.Head, rec.Command, rec.Result == evidence.ResultGreen
}

// resolvedFinalizeCommand re-reads the authoritative finalize.test_command the
// same way processFinalizeGate.buildDriveService pins it (deps.Planning.Reader
// PinContext → pin.Config.Effective.Finalize.TestCommand.Value), so gateDecision
// can require the PR-body evidence command to be byte-equal to the command
// finalize would run now. Command selection is an explicit domain boundary; no
// caller substitutes a command around authoritative configuration. A resolution
// failure yields "" — gateDecision then never skips (the empty resolved command
// fails the non-empty conjunct), so the suite runs, fail-closed.
func resolvedFinalizeCommand(ctx context.Context, deps FinalizeDeps, repoDir string) string {
	pin, err := deps.Planning.Reader.PinContext(ctx, repoDir)
	if err != nil {
		return ""
	}
	return pin.Config.Effective.Finalize.TestCommand.Value
}

// ---------------------------------------------------------------------------
// shared probes and helpers
// ---------------------------------------------------------------------------

// probeRebasePR resolves the GitHub repository and the single open PR for the
// feature head, requiring it to name the expected head and target the effective
// base. A repository or PR probe error is unknown — retain; a non-unique or
// mismatched PR blocks.
func probeRebasePR(ctx context.Context, deps FinalizeDeps, repoDir string, rc *rebaseContext, expectedHead string) (githubcli.PullRequest, *FinalizeRebaseResult) {
	id := int(rc.change.ID())
	repo, err := deps.GitHub.DiscoverRepository(ctx, repoDir)
	if err != nil {
		r := rebaseRefusal(OperationFinalizeRebase, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseRepoUnresolved, err.Error(), id)
		return githubcli.PullRequest{}, &r
	}
	featureBranch := strings.TrimPrefix(string(rc.target.FeatureRef), branchRefPrefix)
	prs, err := deps.GitHub.FindOpenPullRequestsByHead(ctx, repo, featureBranch)
	if err != nil {
		r := rebaseRefusal(OperationFinalizeRebase, ResultExternalFailed, RebaseDispBlocked, ReasonRebasePRProbeFailed, err.Error(), id)
		return githubcli.PullRequest{}, &r
	}
	if len(prs) != 1 {
		r := rebaseRefusal(OperationFinalizeRebase, ResultBlocked, RebaseDispBlocked, ReasonRebasePRNotOpen,
			fmt.Sprintf("%d open pull requests for the feature head; a rebase requires exactly one verified PR", len(prs)), id)
		return githubcli.PullRequest{}, &r
	}
	pr := prs[0]
	if pr.HeadCommit != expectedHead {
		r := rebaseRefusal(OperationFinalizeRebase, ResultBlocked, RebaseDispBlocked, ReasonRebasePRHeadMismatch,
			"the open PR names a head other than the expected feature head", id)
		return githubcli.PullRequest{}, &r
	}
	if pr.BaseBranch != rc.base.Branch {
		r := rebaseRefusal(OperationFinalizeRebase, ResultBlocked, RebaseDispBlocked, ReasonRebasePRBaseMismatch,
			"the open PR targets a base other than the resolved effective base; retarget children first", id)
		return githubcli.PullRequest{}, &r
	}
	return pr, nil
}

// probeRebaseHeads fetches the base's exact remote head and probes the remote
// feature head. Both errors are retained (unknown); an absent remote feature ref
// blocks (there is no published head to rewrite).
func probeRebaseHeads(ctx context.Context, deps FinalizeDeps, rc *rebaseContext) (baseHead, remoteHead gitcli.ObjectID, refusal *FinalizeRebaseResult) {
	id := int(rc.change.ID())
	rev, err := deps.Planning.Client.FetchBranch(ctx, rc.repo, originRemote, rc.target.BaseRef)
	if err != nil {
		r := rebaseRefusal(OperationFinalizeRebase, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseBaseFetchFailed, err.Error(), id)
		return "", "", &r
	}
	rref, err := deps.Planning.Client.ProbeRemoteBranch(ctx, rc.repo, originRemote, rc.target.FeatureRef)
	if err != nil {
		r := rebaseRefusal(OperationFinalizeRebase, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseRemoteFeatureProbe, err.Error(), id)
		return "", "", &r
	}
	if rref.State != gitcli.RemoteRefFound {
		r := rebaseRefusal(OperationFinalizeRebase, ResultBlocked, RebaseDispBlocked, ReasonRebaseRemoteFeatureAbsent,
			"the remote feature ref is absent; publish the feature head before rebasing", id)
		return "", "", &r
	}
	return rev.Commit, rref.Commit, nil
}

// ownedRefPrefixFor is the owned refs/docket/ scratch namespace for one change's
// rebase attempt. BeginRebase writes <prefix>/orig and <prefix>/base beneath it.
func ownedRefPrefixFor(id int) string {
	return "refs/docket/finalize/" + strconv.Itoa(id)
}

// newRebaseAttempt derives an opaque, non-empty attempt token from the injected
// clock and the base head, distinguishing one rewrite attempt from another.
func newRebaseAttempt(deps FinalizeDeps, baseHead gitcli.ObjectID) string {
	stamp := deps.Planning.Clock.Now().UTC().Format("20060102T150405Z")
	short := string(baseHead)
	if len(short) > 12 {
		short = short[:12]
	}
	return stamp + "-" + short
}

// validateRebaseShape runs the configuration-independent request checks for
// `finalize rebase`: a positive id, a non-empty pinned version, and a valid
// full-length object id for the expected head.
func validateRebaseShape(req FinalizeRebaseRequest) []StatusFinding {
	findings := dropFindingCode(validateLifecycleShape(req.ID, "", req.Version), "empty-path")
	if !validFullObjectID(req.Head) {
		findings = append(findings, lifecycleFinding("invalid-head",
			"head must be a full 40- or 64-character lowercase hex object id"))
	}
	return findings
}

// validFullObjectID mirrors the gitcli object-id shape (40 or 64 lowercase hex),
// so a malformed head is a shape refusal before any Git probe.
func validFullObjectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// production gate seam
// ---------------------------------------------------------------------------

// processFinalizeGate is the production FinalizeGate. It composes the shared
// native gate-drive service (internal/gatedrive via GateDriveService) and
// advances the resolved suite in ONE short slice per call — never the old
// 30-minute synchronous polling loop (retired with change 0342). A first call
// starts a fresh drive; a call carrying a continuation resumes the SAME drive.
// A WAITING slice is nonterminal and returns the opaque continuation; only a
// trusted PASSED terminal mints evidence, through the landed evidence-record
// operation; a FAILED terminal is repair work; every other uncertainty is a
// halt — never a fabricated red. Unit tests inject a fake FinalizeGate rather
// than this process-backed implementation.
type processFinalizeGate struct {
	planning PlanningDeps
	wdeps    WorkspaceDeps
}

// finalizeLocalGatePhase names the workflow phase a finalize local-gate drive
// certifies. It is recorded on the drive record only.
const finalizeLocalGatePhase = "finalize-local-gate"

// NewFinalizeGate builds the production local-gate seam over the planning and
// workspace dependencies the CLI already assembled. The gate reads the resolved
// finalize.test_command and observation budget from authoritative config; it
// never takes an agent-supplied command.
func NewFinalizeGate(planning PlanningDeps, wdeps WorkspaceDeps) FinalizeGate {
	return &processFinalizeGate{planning: planning, wdeps: wdeps}
}

// RunLocalGate advances the resolved suite by one driver slice. With no
// continuation it starts a fresh drive over the authoritative-config command and
// budget; with a continuation it resumes that drive. It maps the typed driver
// outcome onto the finalize gate vocabulary: WAITING carries the continuation for
// re-entry, PASSED mints evidence from the exact terminal raw run dir, FAILED is
// repair work, and every other outcome (a HALTED drive or a command failure) is a
// halt — never a fabricated red.
func (g *processFinalizeGate) RunLocalGate(ctx context.Context, req LocalGateRequest) (LocalGateResult, error) {
	svc, ok := g.buildDriveService(ctx, req.RepoDir)
	if !ok {
		return LocalGateResult{Outcome: FinalizeGateHalted, HaltCause: GateHaltUnavailable}, nil
	}

	var out GateDriveResult
	if req.Continuation.DriveID != "" {
		out = svc.Advance(req.Continuation.DriveID, req.Continuation.Generation)
	} else {
		runRoot, err := os.MkdirTemp("", "docket-finalize-gate-*")
		if err != nil {
			return LocalGateResult{Outcome: FinalizeGateHalted, HaltCause: GateHaltUnavailable}, nil
		}
		out = svc.Start(GateDriveStartRequest{
			RepoDir:             req.WorkspaceDir,
			Worktree:            req.WorkspaceDir,
			ChangeID:            strconv.Itoa(req.ID),
			Phase:               finalizeLocalGatePhase,
			Cwd:                 req.WorkspaceDir,
			RunRoot:             runRoot,
			IdempotentSuiteGate: true,
		})
		// A Start command failure never persisted a drive, so the just-minted run
		// root is orphaned (mapDriveOutcome cannot recover it — no drive document).
		// Remove it here so a failed Start does not leak the temp dir. A drive that
		// DID persist owns the root; its removal is at the terminal in mapDriveOutcome.
		if out.Drive == nil {
			removeGateRunRoot(runRoot)
		}
	}
	return g.mapDriveOutcome(ctx, req, out), nil
}

// buildDriveService composes the in-process gate-drive seam for one slice: it
// resolves the authoritative finalize.test_command and observation budget from
// config, roots the durable drive store at the repository's Git common dir, and
// binds the detached-supervisor re-exec target. Any resolution failure fails
// closed (ok=false) so the caller returns a halt, never a fabricated red.
func (g *processFinalizeGate) buildDriveService(ctx context.Context, repoDir string) (*GateDriveService, bool) {
	pin, err := g.planning.Reader.PinContext(ctx, repoDir)
	if err != nil || pin.Config.Effective.Finalize.TestCommand.Value == "" {
		return nil, false
	}
	repo, err := g.planning.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		return nil, false
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, false
	}
	// Finalize's gate is finalize-owned: it reads ONLY finalize.test_command
	// (the guard above pins the same key), never build's.
	svc, _, _ := NewFinalizeGateDriveService(repo.CommonDir, exe, pin.Config.Effective)
	if svc == nil {
		return nil, false
	}
	return svc, true
}

// mapDriveOutcome maps one driver document onto the finalize gate result. A
// command failure (no drive document) is a halt. WAITING surfaces the
// continuation; PASSED records evidence from the exact terminal raw run dir and
// halts (unavailable) if evidence cannot be minted; FAILED is repair work; a
// HALTED drive maps its typed cause into the finalize halt vocabulary.
func (g *processFinalizeGate) mapDriveOutcome(ctx context.Context, req LocalGateRequest, out GateDriveResult) LocalGateResult {
	if out.Drive == nil {
		return LocalGateResult{Outcome: FinalizeGateHalted, HaltCause: GateHaltUnavailable}
	}
	doc := out.Drive
	if doc.Outcome == gatedrive.WAITING {
		// Nonterminal: the run is still live and may relaunch under the run root, so
		// the root MUST be retained. No cleanup here.
		return LocalGateResult{
			Outcome:      FinalizeGateWaiting,
			Continuation: GateContinuation{DriveID: doc.DriveID, Generation: doc.Generation},
		}
	}
	// Every remaining outcome is terminal (PASSED/FAILED/HALTED): the drive is done
	// with its run root, so remove it once the outcome is mapped — for PASSED that
	// means AFTER evidence is minted from the raw run dir below, which the deferred
	// removal guarantees (defer runs after the return value is evaluated). doc.RunRoot
	// is exposed on terminal documents only, so this recovers the root the ORIGINAL
	// Start minted even when the terminal is reached on a later Advance slice. The one
	// exception is a PASSED run whose evidence could not be minted: that maps to a
	// halt and the raw run dir is the human's only diagnostic, so the root is retained
	// (removal is gated on evidence actually being minted).
	removeRoot := true
	defer func() {
		if removeRoot {
			removeGateRunRoot(doc.RunRoot)
		}
	}()
	switch doc.Outcome {
	case gatedrive.PASSED:
		evd := EvidenceRecord(ctx, g.planning, g.wdeps, req.RepoDir,
			EvidenceRecordRequest{ID: req.ID, RunDir: doc.RawRunDir, Head: req.Head})
		if evd.Result != ResultApplied || evd.Block == "" {
			removeRoot = false
			return LocalGateResult{Outcome: FinalizeGateHalted, HaltCause: GateHaltUnavailable, RunDir: doc.RawRunDir}
		}
		return LocalGateResult{Outcome: FinalizeGatePassed, Evidence: evd.Block, RunDir: doc.RawRunDir}
	case gatedrive.FAILED:
		return LocalGateResult{Outcome: FinalizeGateFailed, RunDir: doc.RawRunDir}
	default: // gatedrive.HALTED or an unrecognized outcome — fail closed, never red.
		return LocalGateResult{Outcome: FinalizeGateHalted, HaltCause: mapDriveHaltCause(doc.Cause)}
	}
}

// removeGateRunRoot removes a finalize gate's private run-root temp dir at a
// terminal outcome, so the per-drive dir minted in RunLocalGate does not
// accumulate under TMPDIR across finalize retries/rebases. It is best-effort: a
// removal failure leaves at most one dir behind and never affects the gate
// outcome. An empty root (a halt document that exposes none) is a no-op.
func removeGateRunRoot(runRoot string) {
	if runRoot == "" {
		return
	}
	_ = os.RemoveAll(runRoot)
}

// mapDriveHaltCause maps a driver HALTED cause token onto the closed finalize
// halt vocabulary. A deadline expiry is the running-at-budget analog; every other
// fail-closed cause (identity drift, uncertain ownership, malformed/unreadable
// state, an unadmitted death) is reported as unavailable — a human is needed. It
// never fabricates a decidable pass/fail.
func mapDriveHaltCause(cause string) string {
	switch {
	case strings.HasPrefix(cause, gatedrive.CauseDeadlineExpired):
		return GateHaltRunningAtBudget
	case cause == gatedrive.CauseSchemaMismatch ||
		cause == gatedrive.CauseObservationUnreadable ||
		cause == gatedrive.CauseUnknownObservation:
		return GateHaltMalformed
	default:
		return GateHaltUnavailable
	}
}

// Compile-time seam assertion: the production gate satisfies FinalizeGate.
var _ FinalizeGate = (*processFinalizeGate)(nil)
