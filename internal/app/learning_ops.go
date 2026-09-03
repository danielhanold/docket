package app

import (
	"bytes"
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

// This file is the `learning record` and `learning update` planning operations:
// the two manual-authoring mutations over the learnings ledger. `learning
// record` allocates a brand-new canonical finding under an idempotency key;
// `learning update` is a non-allocating edit of an existing finding pinned by
// its exact submitted blob version. Both refuse with unsupported-config at
// preflight when learnings.enabled is not true, and neither touches the
// learnings README/index: the plan file set is exactly the one finding record.
// Neither operation renders or reads the inline board — learnings are not a
// board surface — so no board fence runs here.

// OperationLearningRecord and OperationLearningUpdate are the operation keys the
// two learning mutations record in their result envelopes, transaction
// trailers, and (for record) idempotency digests.
const (
	OperationLearningRecord = "learning.record"
	OperationLearningUpdate = "learning.update"
)

// ReasonLearningsDisabled is the stable machine reason both operations report
// when learnings.enabled is not true — an unsupported configuration refused at
// preflight before any transaction.
const ReasonLearningsDisabled = "learnings-disabled"

// LearningRecordRequest is the closed, caller-supplied request for one brand-new
// learning finding. Slug is the caller-chosen canonical slug (shape-validated);
// Topics and Changes are the complete values; Apply and War story are the
// authored section bodies. Authored text rides inside the string fields and is
// never interpolated into any shell command.
type LearningRecordRequest struct {
	RequestID string   `json:"request_id" docket:"required"`
	Slug      string   `json:"slug" docket:"required"`
	Hook      string   `json:"hook" docket:"required"`
	Topics    []string `json:"topics"`
	Changes   []int    `json:"changes"`
	Apply     string   `json:"apply" docket:"required"`
	WarStory  string   `json:"war_story" docket:"required"`
}

// LearningUpdateRequest is the closed, caller-supplied request for one update.
// Path and Version pin the exact submitted record. Hook (when non-empty),
// Topics (when non-nil), and Changes (when non-nil) are the complete desired
// values; Sections carries the owned ## Apply / ## War story edits. A request
// whose planned bytes match the current record commits nothing (the engine's
// no-op).
type LearningUpdateRequest struct {
	Path     string               `json:"path" docket:"required"`
	Version  string               `json:"version" docket:"required"`
	Hook     string               `json:"hook"`
	Topics   []string             `json:"topics"`
	Changes  []int                `json:"changes"`
	Sections []SectionEditRequest `json:"sections"`
}

// LearningResult is the protocol-v1 document both learning operations return. It
// embeds the envelope; the identity fields are populated on a successful apply
// or an idempotent replay (record only), and Findings carries every refusal or
// validation diagnostic (marshalled as [] never null).
type LearningResult struct {
	Envelope
	Slug     string          `json:"slug,omitempty"`
	Path     string          `json:"path,omitempty"`
	Revision string          `json:"committed_revision,omitempty"`
	Replayed bool            `json:"replayed,omitempty"`
	Findings []StatusFinding `json:"findings"`
}

// HumanText renders the one-line human summary of a learning outcome.
func (r LearningResult) HumanText() string {
	switch r.Result {
	case ResultApplied:
		return fmt.Sprintf("%s %s (%s) — %s", r.Operation, r.Slug, r.Path, r.Revision)
	default:
		return fmt.Sprintf("%s: %s", r.Operation, r.Result)
	}
}

// newLearningResult stamps the envelope for opKey and normalizes Findings to an
// empty slice so the array marshals as [] on every path.
func newLearningResult(opKey string, result Result, r LearningResult) LearningResult {
	r.Envelope = NewEnvelope(opKey, result)
	if r.Findings == nil {
		r.Findings = []StatusFinding{}
	}
	return r
}

// learningReceipt is the canonical receipt persisted with a learning commit and
// replayed verbatim on an idempotent re-run (record). Field order is
// alphabetical so json.Marshal emits the canonical, sorted-key compact form the
// engine's receipt validator requires.
type learningReceipt struct {
	Op   string `json:"op"`
	Path string `json:"path"`
	Slug string `json:"slug"`
}

// learningRecordPayload is the request's semantic content — every input that
// governs the produced record — minus the caller-chosen RequestID. It is the
// digest payload.
type learningRecordPayload struct {
	Slug     string   `json:"slug"`
	Hook     string   `json:"hook"`
	Topics   []string `json:"topics"`
	Changes  []int    `json:"changes"`
	Apply    string   `json:"apply"`
	WarStory string   `json:"war_story"`
}

func learningRecordSemanticPayload(req LearningRecordRequest) learningRecordPayload {
	return learningRecordPayload{
		Slug:     req.Slug,
		Hook:     req.Hook,
		Topics:   req.Topics,
		Changes:  req.Changes,
		Apply:    req.Apply,
		WarStory: req.WarStory,
	}
}

// learningsDir is the ledger directory learning records live in, derived from
// the configured changes directory exactly as corpusPrefixes/classifyCorpusPath
// derive it.
func learningsDir(eff config.Effective) string {
	return path.Join(eff.ChangesDir.Value, "learnings")
}

// fenceLearningsEnabled refuses at preflight when learnings.enabled is not true:
// an unsupported configuration for either learning operation, refused before any
// transaction runs.
func fenceLearningsEnabled(eff config.Effective) error {
	if !eff.Learnings.Enabled.Value {
		return &planningError{
			Result:  ResultUnsupportedConfig,
			Reason:  ReasonLearningsDisabled,
			Message: "learnings.enabled is not true; learning record/update are unsupported in this configuration",
		}
	}
	return nil
}

// LearningRecordOp validates the request, pins authoritative context, refuses if
// learnings are disabled, and drives one atomic transaction that lands a canonical
// new learning finding. Every failure that predates the transaction (bad request
// shape, learnings disabled) returns without an engine call.
func LearningRecordOp(ctx context.Context, deps PlanningDeps, repoDir string, req LearningRecordRequest) LearningResult {
	if findings := validateLearningRecordShape(req); len(findings) > 0 {
		return newLearningResult(OperationLearningRecord, ResultInvalidInput, LearningResult{Findings: findings})
	}

	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newLearningResult(OperationLearningRecord, result, LearningResult{Findings: []StatusFinding{learningFinding(reason, err.Error())}})
	}
	eff := pin.Config.Effective

	if err := fenceLearningsEnabled(eff); err != nil {
		return learningPreflightRefusal(OperationLearningRecord, err)
	}

	digest, err := canonicalDigest(OperationLearningRecord, learningRecordSemanticPayload(req))
	if err != nil {
		return newLearningResult(OperationLearningRecord, ResultInternalError, LearningResult{Findings: []StatusFinding{learningFinding(ReasonStatusInternalError, err.Error())}})
	}

	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return newLearningResult(OperationLearningRecord, result, LearningResult{Findings: []StatusFinding{learningFinding(reason, err.Error())}})
	}

	op := learningRecordOp{
		req:          req,
		eff:          eff,
		clock:        deps.Clock,
		learningsDir: learningsDir(eff),
	}

	res, execErr := deps.Engine.Execute(ctx, transaction.Request{
		Repository:  repo,
		Remote:      originRemote,
		TargetRef:   gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName),
		Idempotency: &transaction.IdempotencyKey{RequestID: req.RequestID, Digest: digest},
		Loader:      newPlanningLoader(eff),
		Operation:   op,
	})

	// A refusal from record is request-shaped (a caller-chosen slug that
	// collides with an existing finding), so the refusal maps onto invalid-input.
	result, replayed := mapOutcome(res, execErr, ResultInvalidInput)
	out := LearningResult{Findings: findingsToStatus(res.Findings)}
	if result == ResultApplied {
		if rec, ok := decodeLearningReceipt(res.Receipt); ok {
			out.Slug = rec.Slug
			out.Path = rec.Path
		}
		out.Revision = string(res.AppliedCommit)
		out.Replayed = replayed
	}
	r := newLearningResult(OperationLearningRecord, result, out)
	r.Failure = failureStatus(res, execErr)
	return r
}

// LearningUpdate validates the request, pins authoritative context, refuses if
// learnings are disabled, and drives one atomic transaction that edits the pinned
// finding. Every failure that predates the transaction (bad request shape,
// learnings disabled) returns without an engine call.
func LearningUpdate(ctx context.Context, deps PlanningDeps, repoDir string, req LearningUpdateRequest) LearningResult {
	if findings := validateLearningUpdateShape(req); len(findings) > 0 {
		return newLearningResult(OperationLearningUpdate, ResultInvalidInput, LearningResult{Findings: findings})
	}

	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newLearningResult(OperationLearningUpdate, result, LearningResult{Findings: []StatusFinding{learningFinding(reason, err.Error())}})
	}
	eff := pin.Config.Effective

	if err := fenceLearningsEnabled(eff); err != nil {
		return learningPreflightRefusal(OperationLearningUpdate, err)
	}

	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return newLearningResult(OperationLearningUpdate, result, LearningResult{Findings: []StatusFinding{learningFinding(reason, err.Error())}})
	}

	op := learningUpdateOp{
		req:   req,
		eff:   eff,
		clock: deps.Clock,
	}

	res, execErr := deps.Engine.Execute(ctx, transaction.Request{
		Repository: repo,
		Remote:     originRemote,
		TargetRef:  gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName),
		Expected: []transaction.EntityExpectation{{
			Path:    gitcli.RepoPath(req.Path),
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(req.Version)},
		}},
		Loader:    newPlanningLoader(eff),
		Operation: op,
	})

	// A refusal from update is state-shaped (an absent or mistyped record), so
	// the refusal maps onto invalid-state.
	result, _ := mapOutcome(res, execErr, ResultInvalidState)
	out := LearningResult{Findings: findingsToStatus(res.Findings)}
	if result == ResultApplied {
		if rec, ok := decodeLearningReceipt(res.Receipt); ok {
			out.Slug = rec.Slug
			out.Path = rec.Path
		}
		out.Revision = string(res.AppliedCommit)
	}
	r := newLearningResult(OperationLearningUpdate, result, out)
	r.Failure = failureStatus(res, execErr)
	return r
}

// learningPreflightRefusal folds a preflight *planningError into a result
// document, or reports an internal error for any other error.
func learningPreflightRefusal(opKey string, err error) LearningResult {
	if pe, ok := asPlanningError(err); ok {
		return newLearningResult(opKey, pe.Result, LearningResult{Findings: []StatusFinding{learningFinding(pe.Reason, pe.Message)}})
	}
	return newLearningResult(opKey, ResultInternalError, LearningResult{Findings: []StatusFinding{learningFinding(ReasonStatusInternalError, err.Error())}})
}

// learningFinding builds one error-severity request-shape finding.
func learningFinding(code, msg string) StatusFinding {
	return StatusFinding{Code: code, Severity: string(domain.SeverityError), Message: msg}
}

// decodeLearningReceipt decodes a persisted or replayed receipt into its
// identity fields.
func decodeLearningReceipt(b []byte) (learningReceipt, bool) {
	if len(b) == 0 {
		return learningReceipt{}, false
	}
	var rec learningReceipt
	if err := json.Unmarshal(b, &rec); err != nil {
		return learningReceipt{}, false
	}
	return rec, true
}

// validateLearningRecordShape runs the configuration-independent request checks:
// the idempotency id shape, the slug grammar, the required authored fields, and
// the collection shapes.
func validateLearningRecordShape(req LearningRecordRequest) []StatusFinding {
	var findings []StatusFinding
	add := func(code, msg string) {
		findings = append(findings, learningFinding(code, msg))
	}

	if !validRequestID(req.RequestID) {
		add("invalid-request-id", "request_id must be 8–128 ASCII characters matching ^[A-Za-z0-9][A-Za-z0-9._-]*$")
	}
	if !domain.ValidSlugToken(req.Slug) {
		add("invalid-slug", fmt.Sprintf("slug %q is not a valid record slug", req.Slug))
	}
	for _, f := range []struct{ name, val string }{
		{"hook", req.Hook}, {"apply", req.Apply}, {"war_story", req.WarStory},
	} {
		if strings.TrimSpace(f.val) == "" {
			add("empty-"+f.name, f.name+" must be non-empty")
		}
	}
	findings = append(findings, validateIDCollection("changes", req.Changes, FCInvalidChanges, FCDuplicateChanges)...)
	findings = append(findings, validateTopics(req.Topics)...)
	return findings
}

// validateLearningUpdateShape runs the pinned-entity request checks: a non-empty
// path and version, the owned-section edits, and the collection shapes.
func validateLearningUpdateShape(req LearningUpdateRequest) []StatusFinding {
	var findings []StatusFinding
	if strings.TrimSpace(req.Path) == "" {
		findings = append(findings, learningFinding("empty-path", "path must name the finding's current canonical record path"))
	}
	if strings.TrimSpace(req.Version) == "" {
		findings = append(findings, learningFinding("empty-version", "version must be the exact full blob object id of the submitted record"))
	}
	findings = append(findings, validateLearningSections(req.Sections)...)
	findings = append(findings, validateIDCollection("changes", req.Changes, FCInvalidChanges, FCDuplicateChanges)...)
	findings = append(findings, validateTopics(req.Topics)...)
	return findings
}

// validateTopics reports every blank topic entry — a topic must carry text.
func validateTopics(topics []string) []StatusFinding {
	var findings []StatusFinding
	for _, tpc := range topics {
		if strings.TrimSpace(tpc) == "" {
			findings = append(findings, learningFinding("invalid-topics", "topics must not contain a blank entry"))
		}
	}
	return findings
}

// validateLearningSections checks each section edit against the owned learning
// headings and the intent grammar, enforcing the empty-Markdown rule for
// non-replace intents — the same rules render.ApplySectionEdits enforces,
// surfaced here as request-shape findings so a malformed request never reaches
// the engine.
func validateLearningSections(sections []SectionEditRequest) []StatusFinding {
	owned := make(map[string]bool, len(render.LearningOwnedHeadings))
	for _, h := range render.LearningOwnedHeadings {
		owned[h] = true
	}
	var findings []StatusFinding
	for _, s := range sections {
		if !owned[s.Heading] {
			findings = append(findings, learningFinding("invalid-section-heading",
				fmt.Sprintf("section heading %q is not an owned learning heading", s.Heading)))
			continue
		}
		switch render.SectionIntent(s.Intent) {
		case render.SectionPreserve, render.SectionRemove:
			if s.Markdown != "" {
				findings = append(findings, learningFinding("invalid-section-markdown",
					fmt.Sprintf("intent %q for %q must carry empty markdown", s.Intent, s.Heading)))
			}
		case render.SectionReplace:
			// Markdown may be non-empty.
		default:
			findings = append(findings, learningFinding("invalid-section-intent",
				fmt.Sprintf("section intent %q must be one of preserve, replace, remove", s.Intent)))
		}
	}
	return findings
}

// stringSeqValue renders a string slice as a flow sequence of quoted scalars; an
// empty slice renders "[]", never null.
func stringSeqValue(ss []string) document.Value {
	items := make([]document.Value, len(ss))
	for i, s := range ss {
		items[i] = document.String(s)
	}
	return document.Seq(items...)
}

// learningRecordOp is the SemanticOperation the engine drives per attempt for a
// new finding. Every field is fixed before the transaction; the duplicate-slug
// gate and serialization re-run from the attempt's own fresh state.
type learningRecordOp struct {
	req          LearningRecordRequest
	eff          config.Effective
	clock        transaction.Clock
	learningsDir string
}

func (o learningRecordOp) Key() transaction.OperationKey { return OperationLearningRecord }

// Plan refuses when a finding with the requested slug already exists, then
// serializes one canonical finding and plans its single create — never the
// learnings index.
func (o learningRecordOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot

	if _, out := snap.Learning(o.req.Slug); out == domain.LookupFound {
		return refuseLearning("duplicate-slug", fmt.Sprintf("a learning finding with slug %q already exists", o.req.Slug))
	}

	recordBytes, err := render.LearningRecord(render.NewLearningRecord{
		Slug:     o.req.Slug,
		Hook:     o.req.Hook,
		Topics:   o.req.Topics,
		Changes:  toChangeIDs(o.req.Changes),
		Created:  o.clock.Now().UTC(),
		Apply:    o.req.Apply,
		WarStory: o.req.WarStory,
	})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("learning record: serializing record: %w", err)
	}

	recPath := path.Join(o.learningsDir, o.req.Slug+".md")
	receipt, err := json.Marshal(learningReceipt{Op: OperationLearningRecord, Path: recPath, Slug: o.req.Slug})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("learning record: encoding receipt: %w", err)
	}

	return transaction.MutationPlan{
		Files: []transaction.FileMutation{
			{Path: gitcli.RepoPath(recPath), Kind: transaction.MutationCreate, Bytes: recordBytes},
		},
		CommitSubject: fmt.Sprintf("learning %s recorded", o.req.Slug),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}

// learningUpdateOp is the SemanticOperation the engine drives per attempt for an
// edit. Every field is fixed before the transaction; the source lookup, section
// splicing, field patching, and no-op comparison re-run from fresh state.
type learningUpdateOp struct {
	req   LearningUpdateRequest
	eff   config.Effective
	clock transaction.Clock
}

func (o learningUpdateOp) Key() transaction.OperationKey { return OperationLearningUpdate }

// Plan applies the section edits and the complete-desired field values over the
// exact source bytes, then bumps `updated` and plans one replace — unless the
// planned content is byte-identical to the source, in which case it returns an
// empty plan (the engine's no-op) so `updated` is not spuriously bumped. It
// never touches the learnings index.
func (o learningUpdateOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot

	src, ok := st.State.Sources[o.req.Path]
	if !ok {
		return refuseLearning("path-mismatch", fmt.Sprintf("no learning record source loaded at %q", o.req.Path))
	}
	slug := learningSlugAtPath(snap, o.req.Path)
	if slug == "" {
		return refuseLearning("not-a-learning", fmt.Sprintf("no learning finding is present at %q", o.req.Path))
	}

	// Splice the owned authored sections first, over the exact source bytes.
	edited, err := render.ApplySectionEdits(src, render.LearningOwnedHeadings, toSectionEdits(o.req.Sections))
	if err != nil {
		return refuseLearning("section-edit-failed", err.Error())
	}

	// Patch the complete-desired typed fields. `updated` is intentionally NOT
	// set here: it is bumped only after the no-op comparison confirms some
	// semantic input actually changed.
	doc1, err := document.Parse(edited)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("learning update: reparsing edited record: %w", err)
	}
	var ps document.PatchSet
	if strings.TrimSpace(o.req.Hook) != "" {
		ps.SetField("hook", document.String(o.req.Hook))
	}
	if o.req.Topics != nil {
		ps.SetField("topics", stringSeqValue(o.req.Topics))
	}
	if o.req.Changes != nil {
		ps.SetField("changes", intSeqValue(o.req.Changes))
	}
	intermediate, err := doc1.Apply(ps)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("learning update: patching record fields: %w", err)
	}

	receipt, err := json.Marshal(learningReceipt{Op: OperationLearningUpdate, Path: o.req.Path, Slug: slug})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("learning update: encoding receipt: %w", err)
	}

	// No semantic input differs from the current record: commit nothing (the
	// engine's no-op). An empty plan still carries a valid subject/receipt so
	// validatePlan passes; the engine short-circuits before materializing.
	if bytes.Equal(intermediate, src) {
		return transaction.MutationPlan{
			CommitSubject: fmt.Sprintf("learning %s unchanged", slug),
			Receipt:       receipt,
		}, transaction.OperationResult{}, nil
	}

	// Some input changed: bump `updated` and plan the replace.
	doc2, err := document.Parse(intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("learning update: reparsing patched record: %w", err)
	}
	var ps2 document.PatchSet
	ps2.SetField("updated", document.String(o.clock.Now().UTC().Format("2006-01-02")))
	finalBytes, err := doc2.Apply(ps2)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("learning update: stamping updated: %w", err)
	}

	return transaction.MutationPlan{
		Files: []transaction.FileMutation{
			{Path: gitcli.RepoPath(o.req.Path), Kind: transaction.MutationReplace, Bytes: finalBytes},
		},
		CommitSubject: fmt.Sprintf("learning %s updated", slug),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}

// learningSlugAtPath returns the slug of the finding whose canonical path is p,
// or "" when no learning is present at that path.
func learningSlugAtPath(snap domain.Snapshot, p string) string {
	for _, l := range snap.Learnings() {
		if l.Path() == p {
			return l.Slug()
		}
	}
	return ""
}

// refuseLearning builds a refusing OperationResult carrying one finding — a
// helper for the Plan closures' domain refusals.
func refuseLearning(code, msg string) (transaction.MutationPlan, transaction.OperationResult, error) {
	return transaction.MutationPlan{}, transaction.OperationResult{
		Refused: true,
		Findings: []domain.Finding{{
			Code:     code,
			Severity: domain.SeverityError,
			Entity:   domain.EntityRef{Kind: domain.EntityLearning},
			Detail:   map[string]string{"message": msg},
		}},
	}, nil
}
