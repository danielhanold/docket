package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is `change halt` and `change resume-halted`: the durable
// implementation-run halt state and its human-authorized recovery.
//
// A run halt and a finalize block are DIFFERENT states with different source
// and recovery, so they use different sections. `change halt` upserts one
// bounded authored report into a single "## Run halted" section on an
// in-progress change WITHOUT touching its branch, claim lease, workspace, or
// build evidence — the work is paused, not abandoned. `run verify` reports the
// closed `run-halted` verdict for such a change (run_verify.go), and `context
// implementation --id` surfaces the durable halt checkpoints
// (implementation_context.go).
//
// `change resume-halted` is human-authorized recovery of the SAME work, not a
// second claim. It requires the exact marked record and an explicit
// acknowledgement that the prior worker is quiescent, reprobes the owned
// branch/workspace (refusing when a live writer may still hold it), refreshes
// the claim, removes exactly the marker section, and preserves every valid
// checkpoint. It never discards, resets, adopts, or reassigns a workspace whose
// writer may still be live.

// The operation keys `change halt` / `change resume-halted` record in their
// result envelopes and transaction trailers.
const (
	OperationChangeHalt         = "change.halt"
	OperationChangeResumeHalted = "change.resume-halted"
)

// runHaltedSectionHeading is the full ATX H2 heading line the durable run-halted
// section carries. It is exactly domain.RunHaltedSection — the shape the domain
// parser detects as HasRunHalted and the claim action takes ownership of
// removing — so writing this heading is what those readers detect.
const runHaltedSectionHeading = domain.RunHaltedSection

// The closed set of `change halt` / `change resume-halted` dispositions.
const (
	// HaltDispHalted: a bounded halt report was recorded into the single section.
	HaltDispHalted = "halted"
	// HaltDispResumed: the claim was refreshed and the marker removed.
	HaltDispResumed = "resumed"
	// HaltDispContended: the exact-version transaction lost to a fresh
	// incompatible state.
	HaltDispContended = "contended"
	// HaltDispRefused: a retained precondition refusal.
	HaltDispRefused = "refused"
	// HaltDispFailed: a transaction failure; the cause is in the envelope's
	// failure field.
	HaltDispFailed = "failed"
)

// The stable machine reasons the halt operations report. Message text is
// explanatory and must not be parsed.
const (
	// ReasonHaltUnknownChange: an id names no record in the corpus.
	ReasonHaltUnknownChange = "unknown-change"
	// ReasonHaltAmbiguousID: an id is claimed by more than one record.
	ReasonHaltAmbiguousID = "ambiguous-change"
	// ReasonHaltNotInProgress: `change halt` requires an in-progress change; a
	// run that never started or already ended has no run to halt.
	ReasonHaltNotInProgress = "not-in-progress"
	// ReasonResumeNotAcknowledged: `resume-halted` was called without the
	// explicit --acknowledge-quiescent human acknowledgement.
	ReasonResumeNotAcknowledged = "quiescence-not-acknowledged"
	// ReasonResumeNotHalted: `resume-halted` names a change carrying no durable
	// run-halted marker; there is nothing to resume.
	ReasonResumeNotHalted = "not-halted"
	// ReasonResumeWorkspaceProbe: the owned workspace could not be reprobed.
	ReasonResumeWorkspaceProbe = "workspace-probe-failed"
	// ReasonResumeWorkspaceActive: the reprobe reports a workspace whose writer
	// may still be live; resume never adopts it.
	ReasonResumeWorkspaceActive = "workspace-writer-active"
)

// HaltRequest is the closed request for `change halt`. ID and Version pin the
// exact submitted record; Report is the authored bounded halt report recorded in
// the marker. The authored report rides inside the JSON and never reaches a
// shell or Git argument.
type HaltRequest struct {
	ID      int    `json:"id"`
	Version string `json:"version"`
	Report  string `json:"report"`
}

// ResumeRequest is the closed request for `change resume-halted`. ID and Version
// pin the exact marked record; AcknowledgeQuiescent is the explicit human
// acknowledgement that the prior worker is quiescent — without it the operation
// refuses before any effect.
type ResumeRequest struct {
	ID                   int    `json:"id"`
	Version              string `json:"version"`
	AcknowledgeQuiescent bool   `json:"acknowledge_quiescent"`
}

// HaltResult is the protocol-v1 document the halt operations return. It names
// identity and the closed disposition; a refusal carries a stable reason and
// message. It leaks no authored report body (redaction).
type HaltResult struct {
	Envelope
	ID          int             `json:"id,omitempty"`
	Disposition string          `json:"disposition,omitempty"`
	Revision    string          `json:"committed_revision,omitempty"`
	Reason      string          `json:"reason,omitempty"`
	Message     string          `json:"message,omitempty"`
	Findings    []StatusFinding `json:"findings"`
}

// HumanText renders a one-line summary naming identity and disposition only —
// never the authored report body.
func (r HaltResult) HumanText() string {
	if r.Result == ResultApplied || r.Result == ResultNoOp {
		return fmt.Sprintf("%s: change %04d %s", r.Operation, r.ID, r.Disposition)
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// newHaltResult stamps the envelope for the named halt operation and normalizes
// the findings collection so a nil never leaks into the document.
func newHaltResult(op string, result Result, out HaltResult) HaltResult {
	out.Envelope = NewEnvelope(op, result)
	if out.Findings == nil {
		out.Findings = []StatusFinding{}
	}
	return out
}

// haltRefusal builds a refusing result carrying a stable reason, message, and
// disposition for the named halt operation.
func haltRefusal(op string, result Result, reason, message string, id int) HaltResult {
	return newHaltResult(op, result, HaltResult{
		ID: id, Disposition: HaltDispRefused, Reason: reason, Message: message,
	})
}

// haltReceipt is the canonical receipt persisted with a halt/resume commit.
// Field order is alphabetical for the engine's canonical-form validator.
type haltReceipt struct {
	ID int    `json:"id"`
	Op string `json:"op"`
}

// ChangeHalt records one bounded authored halt report into the single "## Run
// halted" section on an in-progress change, in one exact-version transaction.
// The change's branch, claim lease, workspace, and build evidence are untouched.
func ChangeHalt(ctx context.Context, deps PlanningDeps, repoDir string, req HaltRequest) HaltResult {
	if findings := validateHaltShape(req); len(findings) > 0 {
		return newHaltResult(OperationChangeHalt, ResultInvalidInput, HaltResult{ID: req.ID, Findings: findings})
	}
	pin, eff, inline, refusal := haltPinAndFence(ctx, OperationChangeHalt, deps, repoDir, req.ID)
	if refusal != nil {
		return *refusal
	}
	recPath, repo, refusal := haltResolveTargetAndRepo(ctx, OperationChangeHalt, deps, pin, eff, repoDir, req.ID)
	if refusal != nil {
		return *refusal
	}

	op := changeHaltOp{
		id:         req.ID,
		report:     req.Report,
		eff:        eff,
		clock:      deps.Clock,
		inline:     inline,
		changesDir: eff.ChangesDir.Value,
	}
	res, execErr := deps.Engine.Execute(ctx, transaction.Request{
		Repository: repo,
		Remote:     originRemote,
		TargetRef:  gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName),
		Expected: []transaction.EntityExpectation{{
			Path:    gitcli.RepoPath(recPath),
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(req.Version)},
		}},
		Loader:    newPlanningLoader(eff),
		Operation: op,
	})
	return haltResultFromOutcome(OperationChangeHalt, res, execErr, HaltDispHalted, ReasonHaltNotInProgress)
}

// ChangeResumeHalted is human-authorized recovery of a halted run. It requires
// the explicit acknowledgement, reprobes the owned workspace (refusing when a
// live writer may hold it), then refreshes the claim and removes the marker in
// one exact-version transaction, preserving every other byte.
func ChangeResumeHalted(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir string, req ResumeRequest) HaltResult {
	if findings := validateResumeShape(req); len(findings) > 0 {
		return newHaltResult(OperationChangeResumeHalted, ResultInvalidInput, HaltResult{ID: req.ID, Findings: findings})
	}
	// The explicit human acknowledgement gates every effect, before any pin.
	if !req.AcknowledgeQuiescent {
		return haltRefusal(OperationChangeResumeHalted, ResultBlocked, ReasonResumeNotAcknowledged,
			"resume-halted requires --acknowledge-quiescent: an explicit acknowledgement that the prior worker is quiescent", req.ID)
	}

	pin, eff, inline, refusal := haltPinAndFence(ctx, OperationChangeResumeHalted, deps, repoDir, req.ID)
	if refusal != nil {
		return *refusal
	}

	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return haltRefusal(OperationChangeResumeHalted, result, reason, err.Error(), req.ID)
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		return haltRefusal(OperationChangeResumeHalted, ResultInternalError, ReasonStatusInternalError, err.Error(), req.ID)
	}
	snap := build.Snapshot
	c, out := snap.Change(domain.ChangeID(req.ID))
	if refusal := haltLookupRefusal(OperationChangeResumeHalted, out, req.ID); refusal != nil {
		return *refusal
	}
	recPath := c.Path()

	// The record must carry the durable run-halted marker: there is no recovery
	// without a halt to resume from. HasRunHalted is the domain's shape-keyed
	// detection of the "## Run halted" body section.
	if !c.HasRunHalted() {
		return haltRefusal(OperationChangeResumeHalted, ResultInvalidState, ReasonResumeNotHalted,
			fmt.Sprintf("change %04d carries no durable run-halted marker; there is nothing to resume", req.ID), req.ID)
	}

	// Reprobe the owned workspace. A workspace whose writer may still be live is
	// never adopted, reset, or reassigned; resume refuses and leaves everything.
	insp := WorkspaceInspect(ctx, deps, wdeps, repoDir, WorkspaceIDRequest{ID: req.ID})
	if insp.Result != ResultApplied {
		return haltRefusal(OperationChangeResumeHalted, insp.Result, ReasonResumeWorkspaceProbe, insp.Message, req.ID)
	}
	if reason, msg := resumeQuiescenceRefusal(insp.State); reason != "" {
		return haltRefusal(OperationChangeResumeHalted, ResultBlocked, reason, msg, req.ID)
	}

	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return haltRefusal(OperationChangeResumeHalted, result, reason, err.Error(), req.ID)
	}

	op := resumeHaltedOp{
		id:         req.ID,
		eff:        eff,
		clock:      deps.Clock,
		inline:     inline,
		changesDir: eff.ChangesDir.Value,
	}
	res, execErr := deps.Engine.Execute(ctx, transaction.Request{
		Repository: repo,
		Remote:     originRemote,
		TargetRef:  gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName),
		Expected: []transaction.EntityExpectation{{
			Path:    gitcli.RepoPath(recPath),
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(req.Version)},
		}},
		Loader:    newPlanningLoader(eff),
		Operation: op,
	})
	return haltResultFromOutcome(OperationChangeResumeHalted, res, execErr, HaltDispResumed, ReasonResumeNotHalted)
}

// haltPinAndFence pins context, runs the deferred-capability preflight, and
// resolves the inline board-surface fence — the shared pre-transaction plumbing
// both halt operations run.
func haltPinAndFence(ctx context.Context, op string, deps PlanningDeps, repoDir string, id int) (StatusPin, config.Effective, bool, *HaltResult) {
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := haltRefusal(op, result, reason, err.Error(), id)
		return StatusPin{}, config.Effective{}, false, &r
	}
	if decision := config.PreflightMutation(&pin.Config); !decision.Allowed {
		r := newHaltResult(op, ResultUnsupportedConfig, HaltResult{
			ID: id, Reason: ReasonDeferredCapRequested,
			Message: "configuration actively requests a deferred capability docket does not ship in this version (" +
				strings.Join(blockerPaths(decision.Blockers), ", ") + "); withdraw it before any mutation",
		})
		return StatusPin{}, config.Effective{}, false, &r
	}
	eff := pin.Config.Effective
	inline, err := fenceBoardSurface(eff)
	if err != nil {
		var r HaltResult
		if pe, ok := asPlanningError(err); ok {
			r = haltRefusal(op, pe.Result, pe.Reason, pe.Message, id)
		} else {
			r = haltRefusal(op, ResultInternalError, ReasonStatusInternalError, err.Error(), id)
		}
		return StatusPin{}, config.Effective{}, false, &r
	}
	return pin, eff, inline, nil
}

// haltResolveTargetAndRepo resolves the change's canonical record path from a
// corpus pre-read and discovers the Git repository. The authoritative
// in-progress gate re-runs inside the transaction on fresh state.
func haltResolveTargetAndRepo(ctx context.Context, op string, deps PlanningDeps, pin StatusPin, eff config.Effective, repoDir string, id int) (string, gitcli.Repository, *HaltResult) {
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := haltRefusal(op, result, reason, err.Error(), id)
		return "", gitcli.Repository{}, &r
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		r := haltRefusal(op, ResultInternalError, ReasonStatusInternalError, err.Error(), id)
		return "", gitcli.Repository{}, &r
	}
	c, out := build.Snapshot.Change(domain.ChangeID(id))
	if refusal := haltLookupRefusal(op, out, id); refusal != nil {
		return "", gitcli.Repository{}, refusal
	}
	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		r := haltRefusal(op, result, reason, err.Error(), id)
		return "", gitcli.Repository{}, &r
	}
	return c.Path(), repo, nil
}

// haltLookupRefusal folds a change lookup outcome into a typed halt refusal.
func haltLookupRefusal(op string, out domain.LookupOutcome, id int) *HaltResult {
	switch out {
	case domain.LookupAbsent:
		r := haltRefusal(op, ResultInvalidInput, ReasonHaltUnknownChange,
			fmt.Sprintf("no change %04d is present in the corpus", id), id)
		return &r
	case domain.LookupAmbiguous:
		r := haltRefusal(op, ResultInvalidState, ReasonHaltAmbiguousID,
			fmt.Sprintf("more than one record claims change id %04d; refusing to choose", id), id)
		return &r
	}
	return nil
}

// haltResultFromOutcome folds a halt/resume transaction outcome into the result
// document. A plan refusal carrying the incompatible-fresh-state reason is
// remapped onto `contended` — the operation never overwrites a record whose
// status moved out from under the authored request.
func haltResultFromOutcome(op string, res transaction.Result, execErr error, appliedDisp, contendedReason string) HaltResult {
	if res.Disposition == transaction.DispositionRefused && firstFindingCode(res.Findings) == contendedReason {
		return newHaltResult(op, ResultContended, HaltResult{
			Findings:    findingsToStatus(res.Findings),
			Disposition: HaltDispContended,
		})
	}
	result, _ := mapOutcome(res, execErr, ResultInvalidState)
	out := HaltResult{Findings: findingsToStatus(res.Findings)}
	if rec, ok := decodeHaltReceipt(res.Receipt); ok {
		out.ID = rec.ID
	}
	switch {
	case res.Disposition == transaction.DispositionFailed:
		out.Disposition = HaltDispFailed
	case result == ResultApplied:
		out.Disposition = appliedDisp
		out.Revision = string(res.AppliedCommit)
	case result == ResultNoOp:
		out.Disposition = appliedDisp
	case result == ResultContended:
		out.Disposition = HaltDispContended
	default:
		out.Disposition = HaltDispRefused
	}
	r := newHaltResult(op, result, out)
	r.Failure = failureStatus(res, execErr)
	return r
}

// decodeHaltReceipt decodes a persisted halt/resume receipt.
func decodeHaltReceipt(b []byte) (haltReceipt, bool) {
	if len(b) == 0 {
		return haltReceipt{}, false
	}
	var rec haltReceipt
	if err := json.Unmarshal(b, &rec); err != nil {
		return haltReceipt{}, false
	}
	return rec, true
}

// resumeQuiescenceRefusal maps a reprobed workspace state kind onto a resume
// refusal reason, or "" when the state is quiescent enough to resume. An
// allocating workspace is a partial allocation whose writer may still be live; a
// foreign or mismatched workspace has unclear ownership. Resume never adopts any
// of these. A ready, dirty-owned (the prior worker's uncommitted checkpoints),
// missing-branch, or cleaned workspace is quiescent and safe to resume.
func resumeQuiescenceRefusal(state string) (reason, message string) {
	switch state {
	case string(workspace.StateResumable):
		return ReasonResumeWorkspaceActive,
			"the owned workspace is mid-allocation; its writer may still be live, so resume adopts nothing"
	case string(workspace.StateForeign), string(workspace.StateMismatch):
		return ReasonResumeWorkspaceActive,
			"the owned workspace has foreign or mismatched ownership; resume never adopts a workspace whose writer may be live"
	default:
		return "", ""
	}
}

// validateHaltShape runs the configuration-independent request checks for
// `change halt`.
func validateHaltShape(req HaltRequest) []StatusFinding {
	findings := dropFindingCode(validateLifecycleShape("id", req.ID, "", req.Version), FCEmptyPath)
	if strings.TrimSpace(req.Report) == "" {
		findings = append(findings, lifecycleFinding(FCEmptyReport, "report must be a non-empty authored bounded halt report"))
	}
	boundAuthored(&findings, "report", req.Report)
	return findings
}

// validateResumeShape runs the configuration-independent request checks for
// `change resume-halted`.
func validateResumeShape(req ResumeRequest) []StatusFinding {
	return dropFindingCode(validateLifecycleShape("id", req.ID, "", req.Version), FCEmptyPath)
}

// changeHaltOp is the SemanticOperation the engine drives for `change halt`. It
// gates the in-progress status on fresh state, upserts the single "## Run
// halted" section with the bounded report, and rerenders the inline board — it
// touches no frontmatter field, so branch, claim, and every other datum stay
// byte-identical.
type changeHaltOp struct {
	id         int
	report     string
	eff        config.Effective
	clock      transaction.Clock
	inline     bool
	changesDir string
}

func (o changeHaltOp) Key() transaction.OperationKey {
	return transaction.OperationKey(OperationChangeHalt)
}

func (o changeHaltOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot
	c, out := snap.Change(domain.ChangeID(o.id))
	if out != domain.LookupFound {
		return refuseHalt("not-found", fmt.Sprintf("change %04d is not present in the current corpus", o.id))
	}
	// In-progress gate: a run halt records the paused state of an in-progress
	// run. A change that is not in-progress has no run to halt; the refusal reason
	// is remapped to `contended` when the fresh status moved out from under the
	// request.
	if c.Status() != domain.StatusInProgress {
		return refuseHalt(ReasonHaltNotInProgress,
			fmt.Sprintf("change %04d is %q, not in-progress; there is no run to halt", o.id, c.Status()))
	}
	src, ok := st.State.Sources[c.Path()]
	if !ok {
		return refuseHalt("path-mismatch", fmt.Sprintf("no record source loaded at %q for change %04d", c.Path(), o.id))
	}

	body := o.haltReportBody()
	edited, err := render.ApplySectionEdits(src, []string{runHaltedSectionHeading},
		[]render.SectionEdit{{Heading: runHaltedSectionHeading, Intent: render.SectionReplace, Markdown: body}})
	if err != nil {
		return refuseHalt("marker-edit-failed", err.Error())
	}
	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: edited},
	}
	files, err = planInlineBoard(ctx, st, snap, o.inline, o.changesDir, boardPresentation(o.eff), files)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	receipt, err := json.Marshal(haltReceipt{ID: o.id, Op: OperationChangeHalt})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change halt: encoding receipt: %w", err)
	}
	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d run halted", o.id),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}

// haltReportBody renders the single run-halted section body: a dated sub-heading
// and the bounded authored report.
func (o changeHaltOp) haltReportBody() string {
	date := o.clock.Now().UTC().Format("2006-01-02")
	return "### " + date + "\n\n" + strings.TrimRight(o.report, "\r\n") + "\n"
}

// resumeHaltedOp is the SemanticOperation the engine drives for `change
// resume-halted`. It gates the in-progress status on fresh state, refreshes the
// claim lease (re-stamps claimed_at), removes exactly the "## Run halted"
// section, and rerenders the inline board. Every other byte — branch, plan,
// results, checkpoints — is preserved.
type resumeHaltedOp struct {
	id         int
	eff        config.Effective
	clock      transaction.Clock
	inline     bool
	changesDir string
}

func (o resumeHaltedOp) Key() transaction.OperationKey {
	return transaction.OperationKey(OperationChangeResumeHalted)
}

func (o resumeHaltedOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot
	c, out := snap.Change(domain.ChangeID(o.id))
	if out != domain.LookupFound {
		return refuseHalt("not-found", fmt.Sprintf("change %04d is not present in the current corpus", o.id))
	}
	// A resume refreshes an in-progress claim; RefreshClaim itself requires
	// in-progress, so the gate is remapped to `contended` on a moved fresh status.
	if c.Status() != domain.StatusInProgress {
		return refuseHalt(ReasonResumeNotHalted,
			fmt.Sprintf("change %04d is %q, not in-progress; there is no halted run to resume", o.id, c.Status()))
	}
	src, ok := st.State.Sources[c.Path()]
	if !ok {
		return refuseHalt("path-mismatch", fmt.Sprintf("no record source loaded at %q for change %04d", c.Path(), o.id))
	}
	if !c.HasRunHalted() {
		return refuseHalt(ReasonResumeNotHalted,
			fmt.Sprintf("change %04d carries no run-halted marker in the fresh state; there is nothing to resume", o.id))
	}

	// Remove exactly the marker section over the exact source bytes.
	edited, err := render.ApplySectionEdits(src, []string{runHaltedSectionHeading},
		[]render.SectionEdit{{Heading: runHaltedSectionHeading, Intent: render.SectionRemove}})
	if err != nil {
		return refuseHalt("marker-remove-failed", err.Error())
	}

	// Refresh the claim lease (re-stamp claimed_at). The in-progress gate above
	// guarantees RefreshClaim succeeds.
	refreshed, fail := domain.RefreshClaim(c, o.clock.Now())
	if fail != nil {
		return refuseLifecyclePolicy(fail)
	}
	doc, err := document.Parse(edited)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change resume-halted: reparsing edited record: %w", err)
	}
	var ps document.PatchSet
	for _, fc := range refreshed.Changed {
		upsertField(&ps, doc, fc.Field, lifecycleFieldValue(fc.To))
	}
	finalBytes, err := doc.Apply(ps)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change resume-halted: patching claim field: %w", err)
	}

	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: finalBytes},
	}
	files, err = planInlineBoard(ctx, st, snap, o.inline, o.changesDir, boardPresentation(o.eff), files)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	receipt, err := json.Marshal(haltReceipt{ID: o.id, Op: OperationChangeResumeHalted})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change resume-halted: encoding receipt: %w", err)
	}
	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d run resumed", o.id),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}

// refuseHalt builds a refusing (plan, OperationResult) pair carrying one
// state-shaped finding for the halt operations' Plan closures.
func refuseHalt(code, msg string) (transaction.MutationPlan, transaction.OperationResult, error) {
	return transaction.MutationPlan{}, transaction.OperationResult{
		Refused: true,
		Findings: []domain.Finding{{
			Code:     code,
			Severity: domain.SeverityError,
			Entity:   domain.EntityRef{Kind: domain.EntityChange},
			Detail:   map[string]string{"message": msg},
		}},
	}, nil
}
