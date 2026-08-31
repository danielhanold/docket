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
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// This file is the `change block` and `change defer` planning operations: two
// non-allocating lifecycle transitions that land one change's owned frontmatter
// changes and every affected v1-owned derived view (the change record's typed
// lifecycle fields, its refreshed updated date, its re-rendered artifact block,
// and — for defer — its ## Why deferred authored section; plus the inline board)
// as one validated atomic transaction. The domain owns legality: domain.Block
// and domain.Defer decide whether the current status may take the transition and
// yield the exact FieldChanges to apply, so this layer decides no lifecycle
// policy of its own. Both operations edit an existing record, so each pins the
// submitted record version with an exact-blob entity expectation rather than an
// idempotency key. Neither inspects any process, branch, worktree, or PR state.

// OperationChangeBlock and OperationChangeDefer are the operation keys the two
// lifecycle transitions record in their result envelopes and transaction
// trailers.
const (
	OperationChangeBlock = "change.block"
	OperationChangeDefer = "change.defer"
)

// whyDeferredHeading is the owned authored section `change defer` replaces (or
// inserts when absent) with the caller's deferral rationale.
const whyDeferredHeading = "## Why deferred"

// ChangeBlockRequest is the closed, caller-supplied request for one block. Path
// and Version pin the exact submitted record; Reason is the non-empty
// blocked_by reason the domain records. Authored text rides inside the string
// fields and is never interpolated into any shell command.
type ChangeBlockRequest struct {
	ChangeID int    `json:"change_id"`
	Path     string `json:"path"`
	Version  string `json:"version"`
	Reason   string `json:"reason"`
}

// ChangeDeferRequest is the closed, caller-supplied request for one defer. Path
// and Version pin the exact submitted record; WhyDeferred is the non-empty
// authored ## Why deferred section body.
type ChangeDeferRequest struct {
	ChangeID    int    `json:"change_id"`
	Path        string `json:"path"`
	Version     string `json:"version"`
	WhyDeferred string `json:"why_deferred"`
}

// ChangeLifecycleResult is the protocol-v1 document `change block` and
// `change defer` return. It embeds the envelope; Status carries the resulting
// stored status on a successful apply, and Findings carries every refusal or
// validation diagnostic (marshalled as [] never null).
type ChangeLifecycleResult struct {
	Envelope
	ID       int             `json:"id,omitempty"`
	Status   string          `json:"status,omitempty"` // resulting stored status
	Revision string          `json:"committed_revision,omitempty"`
	Findings []StatusFinding `json:"findings"`
}

// HumanText renders the one-line human summary of a lifecycle outcome.
func (r ChangeLifecycleResult) HumanText() string {
	switch r.Result {
	case ResultApplied:
		return fmt.Sprintf("change %04d → %s — %s", r.ID, r.Status, r.Revision)
	default:
		return fmt.Sprintf("%s: %s", r.Operation, r.Result)
	}
}

// newChangeLifecycleResult stamps the envelope for opKey and normalizes Findings
// to an empty slice so the array marshals as [] on every path.
func newChangeLifecycleResult(opKey string, result Result, r ChangeLifecycleResult) ChangeLifecycleResult {
	r.Envelope = NewEnvelope(opKey, result)
	if r.Findings == nil {
		r.Findings = []StatusFinding{}
	}
	return r
}

// changeLifecycleReceipt is the canonical receipt persisted with a lifecycle
// commit. Field order is alphabetical so json.Marshal emits the canonical,
// sorted-key compact form the engine's receipt validator requires.
type changeLifecycleReceipt struct {
	ID     int    `json:"id"`
	Op     string `json:"op"`
	Status string `json:"status"`
}

// ChangeBlock validates the request, pins authoritative context, and drives one
// atomic transaction that blocks the change (recording the reason) and — when
// inline is enabled — re-renders the board. Every failure that predates the
// transaction (bad request shape, an empty reason, a github board surface)
// returns without an engine call.
func ChangeBlock(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeBlockRequest) ChangeLifecycleResult {
	findings := validateLifecycleShape(req.ChangeID, req.Path, req.Version)
	if strings.TrimSpace(req.Reason) == "" {
		findings = append(findings, lifecycleFinding("empty-reason", "reason must be non-empty for a block"))
	}
	if len(findings) > 0 {
		return newChangeLifecycleResult(OperationChangeBlock, ResultInvalidInput, ChangeLifecycleResult{Findings: findings})
	}

	reason := req.Reason
	action := func(c domain.Change) (domain.ActionResult, *domain.PolicyFailure) {
		return domain.Block(c, reason)
	}
	return executeChangeLifecycle(ctx, deps, repoDir, OperationChangeBlock, req.ChangeID, req.Path, req.Version, action, nil)
}

// ChangeDefer validates the request, pins authoritative context, and drives one
// atomic transaction that defers the change, replaces (or inserts) its
// ## Why deferred section, and — when inline is enabled — re-renders the board.
// Every failure that predates the transaction (bad request shape, an empty
// rationale, a github board surface) returns without an engine call.
func ChangeDefer(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeDeferRequest) ChangeLifecycleResult {
	findings := validateLifecycleShape(req.ChangeID, req.Path, req.Version)
	if strings.TrimSpace(req.WhyDeferred) == "" {
		findings = append(findings, lifecycleFinding("empty-why_deferred", "why_deferred must be a non-empty authored section body"))
	}
	if len(findings) > 0 {
		return newChangeLifecycleResult(OperationChangeDefer, ResultInvalidInput, ChangeLifecycleResult{Findings: findings})
	}

	action := func(c domain.Change) (domain.ActionResult, *domain.PolicyFailure) {
		return domain.Defer(c)
	}
	sections := []render.SectionEdit{
		{Heading: whyDeferredHeading, Intent: render.SectionReplace, Markdown: req.WhyDeferred},
	}
	return executeChangeLifecycle(ctx, deps, repoDir, OperationChangeDefer, req.ChangeID, req.Path, req.Version, action, sections)
}

// executeChangeLifecycle is the shared driver both transitions compose after
// their own request-shape validation: it pins context, fences the board
// surface, discovers the repository, and submits one exact-version transaction
// carrying the supplied domain action and section edits.
func executeChangeLifecycle(ctx context.Context, deps PlanningDeps, repoDir, opKey string,
	id int, recPath, version string,
	action func(domain.Change) (domain.ActionResult, *domain.PolicyFailure), sections []render.SectionEdit) ChangeLifecycleResult {

	// Pin authoritative context: the metadata mode, branches, and resolved
	// configuration the board fence consults.
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newChangeLifecycleResult(opKey, result, ChangeLifecycleResult{Findings: []StatusFinding{lifecycleFinding(reason, err.Error())}})
	}
	eff := pin.Config.Effective

	// Board-surface fence: a github surface is an unsupported configuration,
	// refused before any transaction; otherwise learn whether inline is on.
	inline, err := fenceBoardSurface(eff)
	if err != nil {
		if pe, ok := asPlanningError(err); ok {
			return newChangeLifecycleResult(opKey, pe.Result, ChangeLifecycleResult{Findings: []StatusFinding{lifecycleFinding(pe.Reason, pe.Message)}})
		}
		return newChangeLifecycleResult(opKey, ResultInternalError, ChangeLifecycleResult{Findings: []StatusFinding{lifecycleFinding(ReasonStatusInternalError, err.Error())}})
	}

	// Discover the repository identity the transaction writes against.
	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return newChangeLifecycleResult(opKey, result, ChangeLifecycleResult{Findings: []StatusFinding{lifecycleFinding(reason, err.Error())}})
	}

	op := changeLifecycleOp{
		opKey:      opKey,
		changeID:   id,
		path:       recPath,
		action:     action,
		sections:   sections,
		eff:        eff,
		clock:      deps.Clock,
		inline:     inline,
		link:       linkContextOf(pin),
		changesDir: eff.ChangesDir.Value,
	}

	res, execErr := deps.Engine.Execute(ctx, transaction.Request{
		Repository: repo,
		Remote:     originRemote,
		TargetRef:  gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName),
		Expected: []transaction.EntityExpectation{{
			Path:    gitcli.RepoPath(recPath),
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(version)},
		}},
		Loader:    newPlanningLoader(eff),
		Operation: op,
	})

	return lifecycleResultFromOutcome(opKey, res, execErr)
}

// lifecycleResultFromOutcome folds a transaction outcome into the result
// document. A refusal from either transition is always state-shaped (an illegal
// source status, a not-found record), so the refusal maps onto invalid-state.
func lifecycleResultFromOutcome(opKey string, res transaction.Result, execErr error) ChangeLifecycleResult {
	result, _ := mapOutcome(res, execErr, ResultInvalidState)

	out := ChangeLifecycleResult{Findings: findingsToStatus(res.Findings)}
	if result == ResultApplied {
		if rec, ok := decodeChangeLifecycleReceipt(res.Receipt); ok {
			out.ID = rec.ID
			out.Status = rec.Status
		}
		out.Revision = string(res.AppliedCommit)
	}
	r := newChangeLifecycleResult(opKey, result, out)
	r.Failure = failureStatus(res, execErr)
	return r
}

// validateLifecycleShape runs the pinned-entity request checks common to both
// transitions: a positive change id and non-empty path and version.
func validateLifecycleShape(id int, recPath, version string) []StatusFinding {
	var findings []StatusFinding
	if id <= 0 {
		findings = append(findings, lifecycleFinding("invalid-change_id", "change_id must be a positive change id"))
	}
	if strings.TrimSpace(recPath) == "" {
		findings = append(findings, lifecycleFinding("empty-path", "path must name the change's current canonical record path"))
	}
	if strings.TrimSpace(version) == "" {
		findings = append(findings, lifecycleFinding("empty-version", "version must be the exact full blob object id of the submitted record"))
	}
	return findings
}

// lifecycleFinding builds one error-severity request-shape finding.
func lifecycleFinding(code, msg string) StatusFinding {
	return StatusFinding{Code: code, Severity: string(domain.SeverityError), Message: msg}
}

// decodeChangeLifecycleReceipt decodes a persisted receipt into its identity and
// resulting-status fields.
func decodeChangeLifecycleReceipt(b []byte) (changeLifecycleReceipt, bool) {
	if len(b) == 0 {
		return changeLifecycleReceipt{}, false
	}
	var rec changeLifecycleReceipt
	if err := json.Unmarshal(b, &rec); err != nil {
		return changeLifecycleReceipt{}, false
	}
	return rec, true
}

// changeLifecycleOp is the SemanticOperation the engine drives per attempt. Every
// field is fixed before the transaction; the state-dependent work (the domain
// legality gate, section splicing, field patching, rendering) re-runs from the
// attempt's own fresh state.
type changeLifecycleOp struct {
	opKey      string
	changeID   int
	path       string
	action     func(domain.Change) (domain.ActionResult, *domain.PolicyFailure)
	sections   []render.SectionEdit
	eff        config.Effective
	clock      transaction.Clock
	inline     bool
	link       render.LinkContext
	changesDir string
}

func (o changeLifecycleOp) Key() transaction.OperationKey { return transaction.OperationKey(o.opKey) }

// Plan gates the transition against the attempt's snapshot via the domain
// action, applies the domain's owned FieldChanges plus the refreshed updated
// date, splices the ## Why deferred section (defer only), re-renders the
// artifact block, and assembles the closed plan: the mutated change record and
// the re-rendered board when inline is enabled.
func (o changeLifecycleOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot

	c, out := snap.Change(domain.ChangeID(o.changeID))
	if out != domain.LookupFound {
		return refuseLifecycle("not-found", fmt.Sprintf("change %04d is not present in the current corpus", o.changeID))
	}

	// Domain legality gate: the action decides whether the current status may take
	// the transition and yields the exact owned FieldChanges. An illegal source
	// status is a state-shaped refusal carrying the domain's stable reason token.
	result, fail := o.action(c)
	if fail != nil {
		return refuseLifecyclePolicy(fail)
	}

	src, ok := st.State.Sources[o.path]
	if !ok {
		return refuseLifecycle("path-mismatch",
			fmt.Sprintf("no record source loaded at %q for change %04d", o.path, o.changeID))
	}

	// Splice the owned authored sections first, over the exact source bytes.
	edited, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, o.sections)
	if err != nil {
		return refuseLifecycle("section-edit-failed", err.Error())
	}

	// First patch pass: the domain's owned lifecycle FieldChanges plus the
	// refreshed updated date. The artifact block is filled in a second pass,
	// because rendering it needs the mutated snapshot.
	doc1, err := document.Parse(edited)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change lifecycle: reparsing edited record: %w", err)
	}

	var ps document.PatchSet
	for _, fc := range result.Changed {
		ps.SetField(fc.Field, lifecycleFieldValue(fc.To))
	}
	// upsertField (not bare SetField): the updated: field is inserted when a record
	// lacks it (a Bash-era or hand-authored record), so this op degrades like the
	// ADR ops, which upsert the same field, rather than internal-erroring with a
	// KindMissingPatchTarget.
	upsertField(&ps, doc1, "updated", document.String(o.clock.Now().UTC().Format("2006-01-02")))

	intermediate, err := doc1.Apply(ps)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change lifecycle: patching record fields: %w", err)
	}

	// The candidate snapshot is the before-state with this record replaced by its
	// mutated bytes: it resolves the artifact block's rows and drives the board.
	candidate, err := buildGroomCandidate(o.eff, st.State.Documents, o.path, intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	gc, gout := candidate.Change(domain.ChangeID(o.changeID))
	if gout != domain.LookupFound {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change lifecycle: mutated record %04d absent from candidate snapshot", o.changeID)
	}

	body, err := render.ArtifactBlockContent(gc, candidate, o.link)
	if err != nil {
		return refuseLifecycle("artifact-render-failed", err.Error())
	}
	doc2, err := document.Parse(intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change lifecycle: reparsing patched record: %w", err)
	}
	var ps2 document.PatchSet
	// ReplaceBlock (not upsert) assumes the docket:artifacts block is present —
	// render.ChangeRecord always emits it for canonical v1 records, and the ADR ops
	// make the same assumption on the same corpus. A record without the block is
	// out of scope here (there is no v1 producer of one).
	ps2.ReplaceBlock("artifacts", body)
	finalBytes, err := doc2.Apply(ps2)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change lifecycle: writing artifact block: %w", err)
	}

	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(o.path), Kind: transaction.MutationReplace, Bytes: finalBytes},
	}

	if o.inline {
		boardPath := path.Join(o.changesDir, "BOARD.md")
		if err := includeBoard(ctx, st.Tree, boardPath, candidate, &files); err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change lifecycle: %w", err)
		}
	}

	status := string(result.Change.Status())
	receipt, err := json.Marshal(changeLifecycleReceipt{
		ID: o.changeID, Op: o.opKey, Status: status,
	})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change lifecycle: encoding receipt: %w", err)
	}

	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d → %s", o.changeID, status),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}

// lifecycleFieldValue renders one FieldChange's target value as a document
// value: a cleared field (empty target) becomes the bare null form, any other
// value a single-quoted string. Block and defer only ever set string-valued
// owned fields (status, blocked_by).
func lifecycleFieldValue(to string) document.Value {
	if to == "" {
		return document.Null()
	}
	return document.String(to)
}

// refuseLifecycle builds a refusing OperationResult carrying one state-shaped
// finding — a helper for the Plan closure's internal-consistency refusals.
func refuseLifecycle(code, msg string) (transaction.MutationPlan, transaction.OperationResult, error) {
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

// refuseLifecyclePolicy builds a refusing OperationResult from a domain policy
// failure, carrying the domain's stable reason token as the finding code and its
// operands as detail — the illegal-source-status refusal path.
func refuseLifecyclePolicy(fail *domain.PolicyFailure) (transaction.MutationPlan, transaction.OperationResult, error) {
	detail := map[string]string{"reason": fail.Reason, "from": string(fail.State)}
	for k, v := range fail.Detail {
		detail[k] = v
	}
	return transaction.MutationPlan{}, transaction.OperationResult{
		Refused: true,
		Findings: []domain.Finding{{
			Code:     fail.Reason,
			Severity: domain.SeverityError,
			Entity:   domain.EntityRef{Kind: domain.EntityChange},
			Detail:   detail,
		}},
	}, nil
}
