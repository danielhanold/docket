package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
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

// This file is `change reclaim`: the proof-gated, exact-version return of one
// strictly-expired `in-progress` claim to `proposed`. Reclaim is destructive
// metadata surgery — it clears the branch and the claim stamp and marks the
// record unreconciled — so its destructive leg FAILS CLOSED on any probe it
// cannot answer. Every external observation has three outcomes: present, cleanly
// absent, and unknown; `unknown` never shares the "absent" branch, so a probe
// error is a retained skip, never a licence to reclaim (learning
// `probe-error-is-not-clean-absence`).
//
// The gate requires ALL of: a strictly-expired lease (missing, empty, malformed,
// future, exactly-at-boundary, and non-in-progress stamps are NOT expiry); the
// recorded AND conventional feature branch cleanly absent both locally and
// remotely; no owned live feature workspace (allocating/ready/dirty/ambiguous);
// and every Git and workspace probe succeeding. The workspace inspection is the
// ownership + live-gate/process-liveness probe: a live gate run keeps its owned
// workspace registered/dirty/allocating, exactly the states this refuses on —
// the same reading `change resume-halted` and `finalize clear-block` take of a
// workspace whose writer may still be live.
//
// Only when every conjunct holds does the exact-version transaction apply the
// landed `domain.Reclaim` action, append ONE dated `## Reclaim log` entry (the
// previous claim plus a proof summary), return the record to `proposed`, clear
// branch/claim, set `reconciled: false`, rerender the artifact block and inline
// board, validate the whole repository, commit exactly those paths, and push
// under the exact-ref lease. The lease and branch-fact re-evaluation happens on
// the transaction's own fresh state (decide-and-act-on-the-same-copy): the
// pre-transaction probes are the destructive gate, the in-transaction action is
// the authority. A record whose fresh status moved out from under the exact
// version maps to `contended`; every retained refusal is `skipped` and touches
// nothing (no cleanup, delete, reset, or marker removal).
//
// Explicit reclaim is available regardless of `reclaim.auto`: the auto policy
// governs only whether `maintenance sweep` attempts reclaim, never this command.

// OperationChangeReclaim is the operation key `change reclaim` records in its
// result envelope and its transaction trailer.
const OperationChangeReclaim = "change.reclaim"

// reclaimLogHeading is the exact H2 heading of the appended reclaim log.
const reclaimLogHeading = "## Reclaim log"

// The closed set of `change reclaim` dispositions a result may carry.
const (
	// ReclaimDispReclaimed: the claim was reclaimed to proposed.
	ReclaimDispReclaimed = "reclaimed"
	// ReclaimDispSkipped: a retained precondition refusal — the lease is not
	// strictly expired, a branch/workspace still holds the work, or a probe could
	// not be answered. Nothing was cleaned, deleted, reset, or removed.
	ReclaimDispSkipped = "skipped"
	// ReclaimDispContended: the exact-version transaction lost to a fresh
	// incompatible state (the record moved out from under the submitted version).
	ReclaimDispContended = "contended"
	// ReclaimDispFailed: a transaction failure; the cause is in the envelope's
	// failure field.
	ReclaimDispFailed = "failed"
)

// The stable machine reasons `change reclaim` reports. Message text is
// explanatory and must not be parsed.
const (
	// ReasonReclaimUnknownChange: an id names no record in the corpus.
	ReasonReclaimUnknownChange = "unknown-change"
	// ReasonReclaimAmbiguousID: an id is claimed by more than one record.
	ReasonReclaimAmbiguousID = "ambiguous-change"
	// ReasonReclaimBranchPresent: the recorded or conventional feature branch is
	// still present locally or remotely — unfinished work whoever left it.
	ReasonReclaimBranchPresent = "branch-still-present"
	// ReasonReclaimBranchProbe: a Git branch-existence probe could not be
	// answered; the destructive leg fails closed rather than assume absence.
	ReasonReclaimBranchProbe = "branch-probe-failed"
	// ReasonReclaimWorkspaceActive: an owned feature workspace is
	// allocating/ready/dirty/ambiguous — the work still exists.
	ReasonReclaimWorkspaceActive = "workspace-active"
	// ReasonReclaimWorkspaceProbe: the owned workspace could not be inspected.
	ReasonReclaimWorkspaceProbe = "workspace-probe-failed"
	// reclaimReasonIllegalSource is the domain's stable token for a non-in-progress
	// source status; the result mapper folds it onto `contended`.
	reclaimReasonIllegalSource = "illegal-source-status"
)

// reclaimActiveWorkspaceStates is the set of inspected workspace states that
// prove the work still exists and so block a reclaim. A ready, dirty-owned,
// allocating, or path/registration/manifest-mismatched (ambiguous ownership)
// workspace is a live holder; a cleaned tombstone, a missing feature branch, or
// an absent/foreign/unowned manifest is not this change's live work and does not
// block. Keyed on the workspace layer's own state spellings.
var reclaimActiveWorkspaceStates = map[string]bool{
	string(workspace.StateReady):     true,
	string(workspace.StateDirty):     true,
	string(workspace.StateResumable): true,
	string(workspace.StateMismatch):  true,
}

// ChangeReclaimRequest is the closed, caller-supplied request for one reclaim.
// ID and Version pin the exact submitted record; the reclaim generates its own
// dated log entry, so there is no authored input.
type ChangeReclaimRequest struct {
	ID      int    `json:"id"`
	Version string `json:"version"`
}

// ChangeReclaimResult is the protocol-v1 document `change reclaim` returns. It
// names identity and the closed disposition; a refusal carries a stable reason
// and message. Findings marshals as [] on every path.
type ChangeReclaimResult struct {
	Envelope
	ID          int             `json:"id,omitempty"`
	Disposition string          `json:"disposition,omitempty"`
	Revision    string          `json:"committed_revision,omitempty"`
	Reason      string          `json:"reason,omitempty"`
	Message     string          `json:"message,omitempty"`
	Findings    []StatusFinding `json:"findings"`
}

// HumanText renders a one-line summary naming identity and disposition only.
func (r ChangeReclaimResult) HumanText() string {
	if r.Result == ResultApplied || r.Result == ResultNoOp {
		return fmt.Sprintf("%s: change %04d %s — %s", r.Operation, r.ID, r.Disposition, r.Revision)
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// newChangeReclaimResult stamps the envelope and normalizes Findings so the
// array marshals as [] on every path.
func newChangeReclaimResult(result Result, out ChangeReclaimResult) ChangeReclaimResult {
	out.Envelope = NewEnvelope(OperationChangeReclaim, result)
	if out.Findings == nil {
		out.Findings = []StatusFinding{}
	}
	return out
}

// reclaimSkip builds a retained-refusal result: disposition skipped, a stable
// reason, and an explanatory message. A skip mutates nothing.
func reclaimSkip(result Result, reason, message string, id int) ChangeReclaimResult {
	return newChangeReclaimResult(result, ChangeReclaimResult{
		ID: id, Disposition: ReclaimDispSkipped, Reason: reason, Message: message,
	})
}

// changeReclaimReceipt is the canonical receipt persisted with a reclaim commit.
// Field order is alphabetical for the engine's canonical-form validator.
type changeReclaimReceipt struct {
	ID int    `json:"id"`
	Op string `json:"op"`
}

// ChangeReclaim proof-gates a reclaim and, when every conjunct holds, drives one
// atomic exact-version transaction that returns the record to proposed. The
// branch and workspace probes run first, before any effect: any present branch,
// any live workspace, or any unanswerable probe is a retained skip that mutates
// nothing. The authoritative lease and branch-fact decision re-runs inside the
// transaction on fresh state via the landed domain.Reclaim action.
func ChangeReclaim(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir string, req ChangeReclaimRequest) ChangeReclaimResult {
	if findings := validateReclaimShape(req); len(findings) > 0 {
		return newChangeReclaimResult(ResultInvalidInput, ChangeReclaimResult{ID: req.ID, Findings: findings})
	}

	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return reclaimSkip(result, reason, err.Error(), req.ID)
	}
	if decision := config.PreflightMutation(&pin.Config); !decision.Allowed {
		return newChangeReclaimResult(ResultUnsupportedConfig, ChangeReclaimResult{
			ID: req.ID, Reason: ReasonDeferredCapRequested,
			Message: "configuration actively requests a deferred capability docket does not ship in this version (" +
				strings.Join(blockerPaths(decision.Blockers), ", ") + "); withdraw it before any mutation",
		})
	}
	eff := pin.Config.Effective

	inline, err := fenceBoardSurface(eff)
	if err != nil {
		if pe, ok := asPlanningError(err); ok {
			return reclaimSkip(pe.Result, pe.Reason, pe.Message, req.ID)
		}
		return reclaimSkip(ResultInternalError, ReasonStatusInternalError, err.Error(), req.ID)
	}

	// Resolve the record's canonical path and read the recorded branch/slug the
	// branch probes consult. This pre-read is a supporting observation; the
	// authoritative lease + branch-fact decision re-runs fresh in the transaction.
	c, refusal := reclaimResolveChange(ctx, deps, pin, eff, req.ID)
	if refusal != nil {
		return *refusal
	}
	recPath := c.Path()

	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return reclaimSkip(result, reason, err.Error(), req.ID)
	}

	// Destructive gate — branch absence. Both the recorded and the conventional
	// feature branch must be cleanly absent locally AND remotely. A present branch
	// blocks; an unanswerable probe blocks (unknown never shares absent).
	if refusal := reclaimProveBranchesAbsent(ctx, deps, repo, c); refusal != nil {
		return *refusal
	}

	// Destructive gate — workspace/ownership/live-gate. The inspection's typed
	// error is a probe failure (skip); a live-holder state is active work (skip).
	if refusal := reclaimProveWorkspaceClear(ctx, deps, wdeps, repoDir, req.ID); refusal != nil {
		return *refusal
	}

	// Every conjunct held. The transaction re-decides on fresh state: it applies
	// domain.Reclaim (which re-evaluates the strict-expiry lease and the proven
	// branch facts), appends the reclaim log, and rerenders the derived views.
	op := reclaimOp{
		id:           req.ID,
		ttlHours:     eff.Reclaim.LeaseTTL.Value,
		facts:        domain.NewBranchFacts(nil), // branches proven absent above
		proofSummary: reclaimProofSummary(eff.Reclaim.LeaseTTL.Value),
		eff:          eff,
		clock:        deps.Clock,
		inline:       inline,
		link:         linkContextOf(pin),
		changesDir:   eff.ChangesDir.Value,
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
	return reclaimResultFromOutcome(res, execErr)
}

// reclaimResolveChange reads the metadata corpus once and resolves the target
// change. An id that names no single record is refused here, before any probe or
// engine call, with a typed unknown/ambiguous reason.
func reclaimResolveChange(ctx context.Context, deps PlanningDeps, pin StatusPin, eff config.Effective, id int) (domain.Change, *ChangeReclaimResult) {
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := reclaimSkip(result, reason, err.Error(), id)
		return domain.Change{}, &r
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		r := reclaimSkip(ResultInternalError, ReasonStatusInternalError, err.Error(), id)
		return domain.Change{}, &r
	}
	c, out := build.Snapshot.Change(domain.ChangeID(id))
	switch out {
	case domain.LookupFound:
		return c, nil
	case domain.LookupAmbiguous:
		r := newChangeReclaimResult(ResultInvalidState, ChangeReclaimResult{
			ID: id, Disposition: ReclaimDispSkipped, Reason: ReasonReclaimAmbiguousID,
			Message: fmt.Sprintf("more than one record claims change id %04d; refusing to choose", id),
		})
		return domain.Change{}, &r
	default:
		r := newChangeReclaimResult(ResultInvalidInput, ChangeReclaimResult{
			ID: id, Disposition: ReclaimDispSkipped, Reason: ReasonReclaimUnknownChange,
			Message: fmt.Sprintf("no change %04d is present in the corpus", id),
		})
		return domain.Change{}, &r
	}
}

// reclaimProveBranchesAbsent proves the recorded and conventional feature
// branches are cleanly absent both locally and remotely. It returns a retained
// skip on any present branch or any unanswerable probe, and nil only when every
// candidate branch is proven absent on both sides. Duplicate candidates (the
// recorded branch often equals the conventional one) are probed once.
func reclaimProveBranchesAbsent(ctx context.Context, deps PlanningDeps, repo gitcli.Repository, c domain.Change) *ChangeReclaimResult {
	seen := map[string]bool{}
	candidates := make([]string, 0, 2)
	if recorded := c.Branch(); recorded.State == domain.FieldPresent && recorded.Value != "" {
		candidates = append(candidates, recorded.Value)
	}
	candidates = append(candidates, domain.MintBranch(c.Type(), c.BranchPrefix(), c.Slug()))

	id := int(c.ID())
	for _, branch := range candidates {
		if branch == "" || seen[branch] {
			continue
		}
		seen[branch] = true
		ref := gitcli.RefName(branchRefPrefix + branch)

		// Local: a resolvable ref is present; a ref-unavailable failure is a clean
		// absence; any other failure is an unanswerable probe (fail closed).
		if _, err := deps.Client.ResolveRef(ctx, repo, ref); err == nil {
			return reclaimResultPtr(reclaimSkip(ResultBlocked, ReasonReclaimBranchPresent,
				fmt.Sprintf("local branch %q still exists; reclaim leaves unfinished work in place", branch), id))
		} else if f, ok := gitcli.AsFailure(err); !ok || f.Kind != gitcli.KindRefUnavailable {
			return reclaimResultPtr(reclaimSkip(ResultBlocked, ReasonReclaimBranchProbe,
				fmt.Sprintf("could not probe local branch %q; refusing to reclaim on an unknown probe", branch), id))
		}

		// Remote: three-outcome probe — found blocks, absent is clean, error fails
		// closed. ProbeRemoteBranch never reports absence for an errored probe.
		rref, err := deps.Client.ProbeRemoteBranch(ctx, repo, originRemote, ref)
		if err != nil {
			return reclaimResultPtr(reclaimSkip(ResultBlocked, ReasonReclaimBranchProbe,
				fmt.Sprintf("could not probe remote branch %q; refusing to reclaim on an unknown probe", branch), id))
		}
		if rref.State == gitcli.RemoteRefFound {
			return reclaimResultPtr(reclaimSkip(ResultBlocked, ReasonReclaimBranchPresent,
				fmt.Sprintf("remote branch %q still exists; reclaim leaves unfinished work in place", branch), id))
		}
	}
	return nil
}

// reclaimProveWorkspaceClear inspects the change's owned feature workspace and
// returns a retained skip when the inspection cannot be answered (probe failure)
// or reports a live-holder state (allocating/ready/dirty/ambiguous). The
// inspection is this reclaim's ownership and live-gate/process-liveness probe.
func reclaimProveWorkspaceClear(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir string, id int) *ChangeReclaimResult {
	insp := WorkspaceInspect(ctx, deps, wdeps, repoDir, WorkspaceIDRequest{ID: id})
	if insp.Result != ResultApplied {
		return reclaimResultPtr(reclaimSkip(ResultBlocked, ReasonReclaimWorkspaceProbe, insp.Message, id))
	}
	if reclaimActiveWorkspaceStates[insp.State] {
		return reclaimResultPtr(reclaimSkip(ResultBlocked, ReasonReclaimWorkspaceActive,
			fmt.Sprintf("an owned workspace is %q for change %04d; the work still exists", insp.State, id), id))
	}
	return nil
}

// reclaimProofSummary renders the fixed proof-summary clause the reclaim-log
// entry records: what the pre-transaction gate proved before the reclaim.
func reclaimProofSummary(ttlHours int) string {
	return fmt.Sprintf("lease strictly expired (TTL %dh); recorded and conventional feature branch cleanly absent locally and remotely; no owned live workspace or gate run.", ttlHours)
}

// reclaimResultFromOutcome folds a transaction outcome into the result document.
// A plan refusal carrying the illegal-source-status reason is remapped onto
// `contended` — the record's fresh status moved out from under the exact version.
// Every other in-transaction refusal (a lease re-evaluation that no longer
// strictly expires, a branch fact that reappeared) is a retained skip.
func reclaimResultFromOutcome(res transaction.Result, execErr error) ChangeReclaimResult {
	if res.Disposition == transaction.DispositionRefused && firstFindingCode(res.Findings) == reclaimReasonIllegalSource {
		return newChangeReclaimResult(ResultContended, ChangeReclaimResult{
			Disposition: ReclaimDispContended,
			Findings:    findingsToStatus(res.Findings),
		})
	}
	result, _ := mapOutcome(res, execErr, ResultBlocked)
	out := ChangeReclaimResult{Findings: findingsToStatus(res.Findings)}
	if rec, ok := decodeChangeReclaimReceipt(res.Receipt); ok {
		out.ID = rec.ID
	}
	switch {
	case res.Disposition == transaction.DispositionFailed:
		out.Disposition = ReclaimDispFailed
	case result == ResultApplied:
		out.Disposition = ReclaimDispReclaimed
		out.Revision = string(res.AppliedCommit)
	case result == ResultNoOp:
		out.Disposition = ReclaimDispReclaimed
	case result == ResultContended:
		out.Disposition = ReclaimDispContended
	default:
		out.Disposition = ReclaimDispSkipped
		out.Reason = firstFindingCode(res.Findings)
	}
	r := newChangeReclaimResult(result, out)
	r.Failure = failureStatus(res, execErr)
	return r
}

// decodeChangeReclaimReceipt decodes a persisted reclaim receipt.
func decodeChangeReclaimReceipt(b []byte) (changeReclaimReceipt, bool) {
	if len(b) == 0 {
		return changeReclaimReceipt{}, false
	}
	var rec changeReclaimReceipt
	if err := json.Unmarshal(b, &rec); err != nil {
		return changeReclaimReceipt{}, false
	}
	return rec, true
}

// validateReclaimShape runs the configuration-independent request checks that
// never reach the engine: the pinned-entity fields (id and version).
func validateReclaimShape(req ChangeReclaimRequest) []StatusFinding {
	return dropFindingCode(validateLifecycleShape("id", req.ID, "", req.Version), "empty-path")
}

// reclaimOp is the SemanticOperation the engine drives per attempt. Every field
// is fixed before the transaction; the state-dependent work (the domain
// lease/branch gate, the reclaim-log append, field patching, rendering) re-runs
// from the attempt's own fresh state.
type reclaimOp struct {
	id           int
	ttlHours     int
	facts        domain.BranchFacts
	proofSummary string
	eff          config.Effective
	clock        transaction.Clock
	inline       bool
	link         render.LinkContext
	changesDir   string
}

func (o reclaimOp) Key() transaction.OperationKey {
	return transaction.OperationKey(OperationChangeReclaim)
}

// Plan re-decides the reclaim on the attempt's fresh snapshot via domain.Reclaim
// (the authoritative strict-expiry lease + branch-fact gate), appends the dated
// reclaim-log entry recording the previous claim and the proof summary, applies
// the domain's owned FieldChanges (status→proposed, cleared branch/claim,
// reconciled:false) plus the refreshed updated date, rerenders the artifact
// block, and assembles the closed plan with the inline board when enabled.
func (o reclaimOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot
	c, out := snap.Change(domain.ChangeID(o.id))
	if out != domain.LookupFound {
		return refuseReclaim("not-found", fmt.Sprintf("change %04d is not present in the current corpus", o.id))
	}

	// Capture the previous claim from the SAME fresh copy the mutation clears, so
	// the reclaim log records exactly what was taken back.
	prevClaim := c.ClaimedAt()
	prevBranch := c.Branch()

	// Authoritative gate: strict-expiry lease + proven branch absence. A
	// non-in-progress source is an illegal-source-status refusal the result mapper
	// folds onto `contended`; a non-strictly-expired lease is a retained skip.
	result, fail := domain.Reclaim(c, o.clock.Now(), o.ttlHours, o.facts)
	if fail != nil {
		return refuseLifecyclePolicy(fail)
	}

	src, ok := st.State.Sources[c.Path()]
	if !ok {
		return refuseReclaim("path-mismatch", fmt.Sprintf("no record source loaded at %q for change %04d", c.Path(), o.id))
	}

	// 1. Append the dated reclaim-log entry over the exact source bytes.
	edited, err := appendReclaimLog(src, o.clock.Now().UTC().Format("2006-01-02"), o.reclaimLogEntry(prevClaim, prevBranch))
	if err != nil {
		return refuseReclaim("reclaim-log-append-failed", err.Error())
	}

	// 2. Patch the domain's owned FieldChanges plus the refreshed updated date. A
	//    bool field (reconciled) is patched as a typed bool so its parsed type is
	//    preserved; cleared fields (branch, claimed_at) become the bare null form.
	doc1, err := document.Parse(edited)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change reclaim: reparsing edited record: %w", err)
	}
	var ps document.PatchSet
	for _, fc := range result.Changed {
		if fc.Field == "reconciled" {
			ps.SetField(fc.Field, document.Bool(fc.To == "true"))
			continue
		}
		ps.SetField(fc.Field, lifecycleFieldValue(fc.To))
	}
	upsertField(&ps, doc1, "updated", document.String(o.clock.Now().UTC().Format("2006-01-02")))
	intermediate, err := doc1.Apply(ps)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change reclaim: patching record fields: %w", err)
	}

	// 3. Re-render the artifact block against the reclaimed candidate snapshot.
	candidate, err := buildGroomCandidate(o.eff, st.State.Documents, c.Path(), intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	gc, gout := candidate.Change(domain.ChangeID(o.id))
	if gout != domain.LookupFound {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change reclaim: reclaimed record %04d absent from candidate snapshot", o.id)
	}
	body, err := render.ArtifactBlockContent(gc, candidate, o.link)
	if err != nil {
		return refuseReclaim("artifact-render-failed", err.Error())
	}
	doc2, err := document.Parse(intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change reclaim: reparsing patched record: %w", err)
	}
	var ps2 document.PatchSet
	ps2.ReplaceBlock("artifacts", body)
	finalBytes, err := doc2.Apply(ps2)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change reclaim: writing artifact block: %w", err)
	}

	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: finalBytes},
	}

	// Reclaim moves the record's status (board-visible), so the board renders from
	// the reclaimed CANDIDATE snapshot, not the before-state — a stale before-state
	// render would recommit the pre-reclaim board.
	if o.inline {
		boardPath := path.Join(o.changesDir, "BOARD.md")
		if err := includeBoard(ctx, st.Tree, boardPath, candidate, boardPresentation(o.eff), &files); err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change reclaim: %w", err)
		}
	}

	receipt, err := json.Marshal(changeReclaimReceipt{ID: o.id, Op: OperationChangeReclaim})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change reclaim: encoding receipt: %w", err)
	}
	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d reclaimed", o.id),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}

// reclaimLogEntry renders the single reclaim-log entry body: the previous claim
// stamp and branch that were cleared, plus the pre-transaction proof summary.
func (o reclaimOp) reclaimLogEntry(prevClaim domain.OptionalTime, prevBranch domain.OptionalString) string {
	claim := "(none)"
	if prevClaim.State != domain.FieldAbsent && prevClaim.Raw != "" {
		claim = prevClaim.Raw
	}
	branch := "(none)"
	if prevBranch.State == domain.FieldPresent && prevBranch.Value != "" {
		branch = prevBranch.Value
	}
	var b strings.Builder
	b.WriteString("Reclaimed an expired claim to proposed.\n\n")
	b.WriteString("- Previous claim: " + claim + " (branch " + branch + ").\n")
	b.WriteString("- Proof: " + o.proofSummary + "\n")
	return b.String()
}

// appendReclaimLog appends one dated entry to the record's ## Reclaim log
// section, creating the section at EOF when absent. It preserves prior entries by
// replacing the section body with (old body + the new entry) through the
// loss-preserving render.ApplySectionEdits — which also enforces the section's
// uniqueness (a duplicate heading refuses the whole edit). The old body is sliced
// between the heading and its NAMED terminator (the next top-level heading, or
// EOF), reusing the same fence-aware heading scan the splice uses.
func appendReclaimLog(src []byte, date, entryBody string) ([]byte, error) {
	entry := "### " + date + "\n\n" + strings.TrimRight(entryBody, "\r\n")

	oldBody, present, err := reclaimLogBody(src)
	if err != nil {
		return nil, err
	}
	markdown := entry
	if present {
		if trimmed := strings.Trim(oldBody, "\r\n"); trimmed != "" {
			markdown = trimmed + "\n\n" + entry
		}
	}
	return render.ApplySectionEdits(src, []string{reclaimLogHeading},
		[]render.SectionEdit{{Heading: reclaimLogHeading, Intent: render.SectionReplace, Markdown: markdown}})
}

// reclaimLogBody returns the body of the single ## Reclaim log section and
// whether it is present, scanning headings fence-aware (a heading-shaped line in
// a fenced block is authored content). A duplicate heading yields no single body
// and is an error (the whole-record splice refuses it too, but slicing must not
// pick one silently). It reuses scanTopHeadings from the reconcile operation.
func reclaimLogBody(src []byte) (body string, present bool, err error) {
	heads := scanTopHeadings(src)
	idx := -1
	for i, h := range heads {
		if h.heading == reclaimLogHeading {
			if idx >= 0 {
				return "", false, fmt.Errorf("owned heading %q appears more than once; the reclaim-log slice has no single terminator", reclaimLogHeading)
			}
			idx = i
		}
	}
	if idx < 0 {
		return "", false, nil
	}
	bodyStart := heads[idx].lineEnd
	bodyEnd := len(src)
	if idx+1 < len(heads) {
		bodyEnd = heads[idx+1].start
	}
	return string(src[bodyStart:bodyEnd]), true, nil
}

// refuseReclaim builds a refusing (plan, OperationResult) pair carrying one
// state-shaped finding for the reclaim Plan closure.
func refuseReclaim(code, msg string) (transaction.MutationPlan, transaction.OperationResult, error) {
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

// reclaimResultPtr returns a pointer to a ChangeReclaimResult — the retained-refusal return
// shape the destructive-gate helpers hand back.
func reclaimResultPtr(r ChangeReclaimResult) *ChangeReclaimResult { return &r }
