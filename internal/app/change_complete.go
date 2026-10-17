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
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// This file is the `change complete` planning operation: the results-artifact
// ending (change 0559). It closes a proposed or in-progress change whose
// deliverable is results-artifact straight to done without a PR, relocating the
// record into the archive and landing every affected derived view (the
// record's owned lifecycle fields, its refreshed updated date, its re-rendered
// artifact block, the retargeted backlinks of its metadata-resident spec, plan,
// and results, the inline board) as one validated atomic transaction. It is
// modelled on `change kill`: the archive move is one MutationCreate at the
// archive path plus one MutationDelete of the active path in the same plan.
//
// The domain owns legality: domain.Complete decides whether the status,
// deliverable, PR, and results facts permit the transition and yields the
// field changes (status done, cleared claim stamp; the branch is kept so
// finalize cleanup can delete it). Every precondition — including
// build-readiness for a proposed change and the FINAL results verification
// shared with mark-implemented (resultsArtifactProblem) — is decided inside the
// transaction's Plan, on the exact tree the commit lands on. The only
// pre-transaction state read is the already-completed replay check.

// OperationChangeComplete is the operation key `change complete` records in its
// result envelope and its transaction trailer.
const OperationChangeComplete = "change.complete"

// The complete_dispositions vocabulary: the closed set a `change complete`
// result's Disposition draws from. A refusal carries no disposition; its
// reason is the first finding's code.
const (
	// CompleteDispCompleted: the change was archived as done.
	CompleteDispCompleted = "completed"
	// CompleteDispAlreadyCompleted: the change is already archived as a done
	// results-artifact change; nothing was written.
	CompleteDispAlreadyCompleted = "already-completed"
	// CompleteDispContended: the exact-revision transaction lost to a fresh
	// incompatible state; nothing was written.
	CompleteDispContended = "contended"
	// CompleteDispFailed: a transaction failure; the cause is in the envelope's
	// failure field.
	CompleteDispFailed = "failed"
)

// ChangeCompleteRequest is the closed, caller-supplied request for one
// completion. Path and Revision pin the exact submitted record.
type ChangeCompleteRequest struct {
	ChangeID int    `json:"change_id" docket:"required"`
	Path     string `json:"path" docket:"required"`
	Revision string `json:"revision" docket:"required"`
}

// ChangeCompleteResult is the protocol-v1 document `change complete` returns.
// ArchivePath carries the archived record's canonical path on a completion or
// an already-completed replay; Findings carries every refusal or validation
// diagnostic (marshalled as [] never null).
type ChangeCompleteResult struct {
	Envelope
	ID          int             `json:"id,omitempty"`
	Disposition string          `json:"disposition,omitempty" docket:"enum=complete_dispositions"`
	ArchivePath string          `json:"archive_path,omitempty"`
	Revision    string          `json:"committed_revision,omitempty"`
	Findings    []StatusFinding `json:"findings"`
}

// HumanText renders the one-line human summary of a completion outcome.
func (r ChangeCompleteResult) HumanText() string {
	switch {
	case r.Result == ResultApplied:
		return fmt.Sprintf("change %04d completed (archived %s) — %s", r.ID, r.ArchivePath, r.Revision)
	case r.Disposition == CompleteDispAlreadyCompleted:
		return fmt.Sprintf("change %04d already completed (%s)", r.ID, r.ArchivePath)
	default:
		return fmt.Sprintf("change complete: %s", r.Result)
	}
}

// newChangeCompleteResult stamps the envelope and normalizes Findings to an
// empty slice so the array marshals as [] on every path.
func newChangeCompleteResult(result Result, r ChangeCompleteResult) ChangeCompleteResult {
	r.Envelope = NewEnvelope(OperationChangeComplete, result)
	if r.Findings == nil {
		r.Findings = []StatusFinding{}
	}
	return r
}

// changeCompleteReceipt is the canonical receipt persisted with a completion
// commit. Field order is alphabetical so json.Marshal emits the canonical,
// sorted-key compact form the engine's receipt validator requires.
type changeCompleteReceipt struct {
	ArchivePath string `json:"archive_path"`
	ID          int    `json:"id"`
	Op          string `json:"op"`
}

// ChangeComplete validates the request, pins authoritative context, answers an
// already-completed change as a no-op replay, and otherwise drives one atomic,
// scoped transaction that completes the change. Every failure that predates
// the transaction (bad request shape, a github board surface) returns without
// an engine call.
func ChangeComplete(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeCompleteRequest) ChangeCompleteResult {
	if findings := validateLifecycleShape("change_id", req.ChangeID, req.Path, req.Revision); len(findings) > 0 {
		return newChangeCompleteResult(ResultInvalidInput, ChangeCompleteResult{Findings: findings})
	}

	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return completeErrorResult(result, reason, err.Error())
	}
	eff := pin.Config.Effective

	inline, err := resolveBoardSurface(eff)
	if err != nil {
		if pe, ok := asPlanningError(err); ok {
			return completeErrorResult(pe.Result, pe.Reason, pe.Message)
		}
		return completeErrorResult(ResultInternalError, ReasonStatusInternalError, err.Error())
	}

	// Replay: a change already archived as a done results-artifact change is a
	// no-op, answered without opening a transaction.
	replay, done := completeReplay(ctx, deps, pin, eff, req.ChangeID)
	if done {
		return replay
	}

	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return completeErrorResult(result, reason, err.Error())
	}

	op := changeCompleteOp{
		changeID:   req.ChangeID,
		path:       req.Path,
		eff:        eff,
		clock:      deps.Clock,
		inline:     inline,
		link:       linkContextOf(pin),
		changesDir: eff.ChangesDir.Value,
	}
	res, execErr := deps.Engine.Execute(ctx, transaction.Request{
		Repository: repo,
		Remote:     metadataRemote(pin.Layout),
		TargetRef:  metadataRef(pin.Layout),
		Expected: []transaction.EntityExpectation{{
			Path:     gitcli.RepoPath(req.Path),
			Revision: transaction.ExpectedRevision{Kind: transaction.RevisionBlob, ObjectID: gitcli.ObjectID(req.Revision)},
		}},
		Loader:    newPlanningLoader(eff),
		Scope:     changeScope(req.ChangeID, req.Path, false),
		Operation: op,
	})
	return changeCompleteResultFromOutcome(res, execErr)
}

// completeErrorResult builds a pre-transaction error result carrying one
// error-severity finding.
func completeErrorResult(result Result, reason, message string) ChangeCompleteResult {
	return newChangeCompleteResult(result, ChangeCompleteResult{
		Findings: []StatusFinding{{Code: reason, Severity: string(domain.SeverityError), Message: message}},
	})
}

// completeReplay reads the corpus once and reports whether id names a change
// already archived as a done results-artifact change; done is true when the
// returned result is final (the replay, or a corpus-read failure). Any other
// state — absent, ambiguous, still active — falls through to the transaction,
// whose Plan decides it on the tree it commits to.
func completeReplay(ctx context.Context, deps PlanningDeps, pin StatusPin, eff config.Effective, id int) (ChangeCompleteResult, bool) {
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return completeErrorResult(result, reason, err.Error()), true
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		return completeErrorResult(ResultInternalError, ReasonStatusInternalError, err.Error()), true
	}
	c, out := build.Snapshot.Change(domain.ChangeID(id))
	if out != domain.LookupFound || c.Status() != domain.StatusDone || c.Deliverable() != domain.DeliverableResultsArtifact {
		return ChangeCompleteResult{}, false
	}
	return newChangeCompleteResult(ResultNoOp, ChangeCompleteResult{
		ID: id, Disposition: CompleteDispAlreadyCompleted, ArchivePath: c.Path(),
	}), true
}

// changeCompleteResultFromOutcome folds a transaction outcome into the result
// document. A refusal is state-shaped (invalid-state) and carries no
// disposition: its reason is the first finding's code.
func changeCompleteResultFromOutcome(res transaction.Result, execErr error) ChangeCompleteResult {
	result, _ := mapOutcome(res, execErr, ResultInvalidState)
	out := ChangeCompleteResult{Findings: findingsToStatus(res.Findings)}
	switch {
	case result == ResultApplied:
		out.Disposition = CompleteDispCompleted
		if rec, ok := decodeChangeCompleteReceipt(res.Receipt); ok {
			out.ID = rec.ID
			out.ArchivePath = rec.ArchivePath
		}
		out.Revision = string(res.AppliedCommit)
	case result == ResultContended:
		out.Disposition = CompleteDispContended
	case res.Disposition == transaction.DispositionFailed || res.Disposition == "":
		out.Disposition = CompleteDispFailed
	}
	r := newChangeCompleteResult(result, out)
	r.Failure = failureStatus(res, execErr)
	return r
}

// decodeChangeCompleteReceipt decodes a persisted receipt into its identity
// fields.
func decodeChangeCompleteReceipt(b []byte) (changeCompleteReceipt, bool) {
	if len(b) == 0 {
		return changeCompleteReceipt{}, false
	}
	var rec changeCompleteReceipt
	if err := json.Unmarshal(b, &rec); err != nil {
		return changeCompleteReceipt{}, false
	}
	return rec, true
}

// changeCompleteOp is the SemanticOperation the engine drives per attempt.
// Every field is fixed before the transaction; the state-dependent work (the
// domain legality gate, readiness, the results verification, field patching,
// relocation, rendering) re-runs from the attempt's own fresh state, reading
// the clock each time.
type changeCompleteOp struct {
	changeID   int
	path       string
	eff        config.Effective
	clock      transaction.Clock
	inline     bool
	link       render.LinkContext
	changesDir string
}

func (o changeCompleteOp) Key() transaction.OperationKey { return OperationChangeComplete }

// Plan gates the completion against the attempt's snapshot and tree, then
// applies the domain's owned field changes plus the refreshed updated date,
// drops an owned "## Run halted" section, relocates the record into the
// archive, re-renders its artifact block, retargets its metadata-resident
// artifacts' backlinks, and re-renders the inline board when enabled.
func (o changeCompleteOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot

	c, out := snap.Change(domain.ChangeID(o.changeID))
	if out != domain.LookupFound {
		return refuseLifecycle(FCNotFound, fmt.Sprintf("change %04d is not present in the current corpus", o.changeID))
	}

	result, fail := domain.Complete(c)
	if fail != nil {
		return refuseLifecyclePolicy(fail)
	}
	// A proposed change completes only when it could have been claimed: met
	// dependencies, a design outcome, an unambiguous id.
	if c.Status() == domain.StatusProposed {
		if fail := domain.ClaimEligibility(snap, c, domain.NewBranchFacts(nil)); fail != nil {
			return refuseLifecyclePolicy(fail)
		}
	}

	// The FINAL results verification, on the tree this commit lands on.
	resultsPath := strings.TrimSpace(c.Results().Value)
	data, found, err := readTreeBlob(ctx, st.Tree, resultsPath)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change complete: %w", err)
	}
	if reason, msg := resultsArtifactProblem(resultsPath, data, found, c); reason != "" {
		return refuseLifecycle(FindingCode(reason), msg)
	}

	src, ok := st.State.Sources[o.path]
	if !ok {
		return refuseLifecycle(FCPathMismatch,
			fmt.Sprintf("no record source loaded at %q for change %04d", o.path, o.changeID))
	}
	edited, err := applyClaimRemovals(src, result.OwnedRemovals)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change complete: removing owned sections: %w", err)
	}

	// First patch pass: the domain's owned FieldChanges plus the refreshed
	// updated date. The artifact block is filled in a second pass, because
	// rendering it needs the relocated candidate snapshot.
	now := o.clock.Now().UTC()
	doc1, err := document.Parse(edited)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change complete: reparsing record: %w", err)
	}
	var ps document.PatchSet
	for _, fc := range result.Changed {
		ps.SetField(fc.Field, lifecycleFieldValue(fc.To))
	}
	upsertField(&ps, doc1, "updated", document.String(now.Format("2006-01-02")))
	intermediate, err := doc1.Apply(ps)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change complete: patching record fields: %w", err)
	}

	archivePath := path.Join(o.changesDir, "archive",
		fmt.Sprintf("%s-%04d-%s.md", now.Format("2006-01-02"), o.changeID, c.Slug()))
	candidate, err := buildKillCandidate(o.eff, st.State.Documents, o.path, archivePath, intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	gc, gout := candidate.Change(domain.ChangeID(o.changeID))
	if gout != domain.LookupFound {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change complete: completed record %04d absent from candidate snapshot", o.changeID)
	}

	body, err := render.ArtifactBlockContent(gc, candidate, o.link)
	if err != nil {
		return refuseLifecycle(FCArtifactRenderFailed, err.Error())
	}
	doc2, err := document.Parse(intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change complete: reparsing patched record: %w", err)
	}
	var ps2 document.PatchSet
	ps2.ReplaceBlock("artifacts", body)
	finalBytes, err := doc2.Apply(ps2)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change complete: writing artifact block: %w", err)
	}

	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(archivePath), Kind: transaction.MutationCreate, Bytes: finalBytes},
		{Path: gitcli.RepoPath(o.path), Kind: transaction.MutationDelete},
	}
	files, err = retargetArtifactBacklinks(ctx, st.Tree, gc, files)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	if o.inline {
		boardPath := path.Join(o.changesDir, "BOARD.md")
		if err := includeBoard(ctx, st.Tree, boardPath, candidate, boardUnrenderable(st.State, o.changesDir), boardPresentation(o.eff), &files); err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change complete: %w", err)
		}
	}

	receipt, err := json.Marshal(changeCompleteReceipt{
		ArchivePath: archivePath, ID: o.changeID, Op: OperationChangeComplete,
	})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change complete: encoding receipt: %w", err)
	}
	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d completed (done, archived)", o.changeID),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}
