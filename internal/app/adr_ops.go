package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// This file is the `adr record` planning operation: it records one brand-new
// Accepted architecture decision and lands every affected v1-owned derived view
// (the new ADR record, the re-rendered ADR index, and — when a producing change
// is supplied — that change's appended adrs collection and its re-rendered
// artifact block) as one validated atomic transaction commit. Recording an ADR
// allocates a fresh id, so it carries a caller-supplied idempotency key like
// `change create`; a supplied producing change is pinned by an exact-blob entity
// expectation so a concurrently moved change contends rather than clobbers. The
// ADR index is rerendered on every ADR operation, unconditionally.

// OperationADRRecord is the operation key `adr record` records in its result
// envelope, its transaction trailer, and its idempotency digest.
const OperationADRRecord = "adr.record"

// ADRRecordRequest is the closed, caller-supplied request for one brand-new
// Accepted ADR. Authored Markdown rides inside the string fields and is never
// interpolated into any shell command. RelatesTo is the complete set of
// non-reciprocal related-ADR references; Change, when set, names the producing
// change the ADR is recorded against.
type ADRRecordRequest struct {
	RequestID    string `json:"request_id" docket:"required"`
	Title        string `json:"title" docket:"required"`
	Context      string `json:"context" docket:"required"`
	Decision     string `json:"decision" docket:"required"`
	Consequences string `json:"consequences" docket:"required"`
	Alternatives string `json:"alternatives" docket:"required"`
	RelatesTo    []int  `json:"relates_to"`

	Change *ADRProducingChange `json:"change,omitempty"`
}

// ADRProducingChange pins the change an ADR is recorded against: its id, its
// current canonical path, and the exact full blob object id the transaction
// expects that record to carry.
type ADRProducingChange struct {
	ID      int    `json:"id"`
	Path    string `json:"path"`
	Version string `json:"version"`
}

// ADRResult is the protocol-v1 document `adr record` returns. It embeds the
// envelope; the identity fields are populated on a successful apply or an
// idempotent replay, and Findings carries every refusal or validation
// diagnostic (marshalled as [] never null).
type ADRResult struct {
	Envelope
	ID       int             `json:"id,omitempty"`
	Path     string          `json:"path,omitempty"`
	Revision string          `json:"committed_revision,omitempty"`
	Replayed bool            `json:"replayed,omitempty"`
	Findings []StatusFinding `json:"findings"`
}

// HumanText renders the one-line human summary of an ADR outcome.
func (r ADRResult) HumanText() string {
	switch r.Result {
	case ResultApplied:
		verb := "recorded"
		if r.Replayed {
			verb = "already recorded"
		}
		return fmt.Sprintf("ADR %04d %s (%s) — %s", r.ID, verb, r.Path, r.Revision)
	default:
		return fmt.Sprintf("%s: %s", r.Operation, r.Result)
	}
}

// newADRResult stamps the envelope for opKey and normalizes Findings to an empty
// slice so the array marshals as [] on every path.
func newADRResult(opKey string, result Result, r ADRResult) ADRResult {
	r.Envelope = NewEnvelope(opKey, result)
	if r.Findings == nil {
		r.Findings = []StatusFinding{}
	}
	return r
}

// adrRecordReceipt is the canonical receipt persisted with an ADR-record commit
// and replayed verbatim on an idempotent re-run. Field order is alphabetical so
// json.Marshal emits the canonical, sorted-key compact form the engine's receipt
// validator requires.
type adrRecordReceipt struct {
	ID   int    `json:"id"`
	Op   string `json:"op"`
	Path string `json:"path"`
}

// adrRecordPayload is the request's semantic content — every input that governs
// the produced records — minus the caller-chosen RequestID and the producing
// change's concurrency Version pin (a pin, not semantic content). It is the
// digest payload.
type adrRecordPayload struct {
	Title        string  `json:"title"`
	Context      string  `json:"context"`
	Decision     string  `json:"decision"`
	Consequences string  `json:"consequences"`
	Alternatives string  `json:"alternatives"`
	RelatesTo    []int   `json:"relates_to"`
	ChangeID     *int    `json:"change_id"`
	ChangePath   *string `json:"change_path"`
}

func adrRecordSemanticPayload(req ADRRecordRequest) adrRecordPayload {
	p := adrRecordPayload{
		Title:        req.Title,
		Context:      req.Context,
		Decision:     req.Decision,
		Consequences: req.Consequences,
		Alternatives: req.Alternatives,
		RelatesTo:    req.RelatesTo,
	}
	if req.Change != nil {
		id := req.Change.ID
		p.ChangeID = &id
		pth := req.Change.Path
		p.ChangePath = &pth
	}
	return p
}

// ADRRecordOp validates the request, pins authoritative context, and drives one
// atomic transaction that lands a canonical new Accepted ADR, the re-rendered ADR
// index, and — when a producing change is supplied — that change's appended adrs
// collection and re-rendered artifact block. Every failure that predates the
// transaction (bad request shape) returns without an engine call.
func ADRRecordOp(ctx context.Context, deps PlanningDeps, repoDir string, req ADRRecordRequest) ADRResult {
	if findings := validateADRRecordShape(req); len(findings) > 0 {
		return newADRResult(OperationADRRecord, ResultInvalidInput, ADRResult{Findings: findings})
	}

	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newADRResult(OperationADRRecord, result, ADRResult{Findings: []StatusFinding{adrFinding(reason, err.Error())}})
	}
	eff := pin.Config.Effective

	slug := slugifyTitle(req.Title)
	if !domain.ValidSlugToken(slug) {
		return newADRResult(OperationADRRecord, ResultInvalidInput, ADRResult{
			Findings: []StatusFinding{adrFinding("invalid-slug", fmt.Sprintf("title %q does not yield a valid slug", req.Title))},
		})
	}

	digest, err := canonicalDigest(OperationADRRecord, adrRecordSemanticPayload(req))
	if err != nil {
		return newADRResult(OperationADRRecord, ResultInternalError, ADRResult{Findings: []StatusFinding{adrFinding(ReasonStatusInternalError, err.Error())}})
	}

	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return newADRResult(OperationADRRecord, result, ADRResult{Findings: []StatusFinding{adrFinding(reason, err.Error())}})
	}

	op := adrRecordOp{
		req:     req,
		eff:     eff,
		slug:    slug,
		clock:   deps.Clock,
		link:    linkContextOf(pin),
		adrsDir: eff.ADRsDir.Value,
	}

	txReq := transaction.Request{
		Repository:  repo,
		Remote:      originRemote,
		TargetRef:   gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName),
		Idempotency: &transaction.IdempotencyKey{RequestID: req.RequestID, Digest: digest},
		Loader:      newPlanningLoader(eff),
		Operation:   op,
	}
	// A supplied producing change is pinned by an exact-blob entity expectation so
	// a concurrently moved change contends rather than being silently clobbered.
	if req.Change != nil {
		txReq.Expected = []transaction.EntityExpectation{{
			Path:    gitcli.RepoPath(req.Change.Path),
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(req.Change.Version)},
		}}
	}

	res, execErr := deps.Engine.Execute(ctx, txReq)

	// A refusal from record is request-shaped (a dangling relates_to reference, an
	// absent producing change), so the refusal maps onto invalid-input.
	result, replayed := mapOutcome(res, execErr, ResultInvalidInput)
	out := ADRResult{Findings: findingsToStatus(res.Findings)}
	if result == ResultApplied {
		if rec, ok := decodeADRRecordReceipt(res.Receipt); ok {
			out.ID = rec.ID
			out.Path = rec.Path
		}
		out.Revision = string(res.AppliedCommit)
		out.Replayed = replayed
	}
	r := newADRResult(OperationADRRecord, result, out)
	r.Failure = failureStatus(res, execErr)
	return r
}

// adrFinding builds one error-severity request-shape finding.
func adrFinding(code, msg string) StatusFinding {
	return StatusFinding{Code: code, Severity: string(domain.SeverityError), Message: msg}
}

// decodeADRRecordReceipt decodes a persisted or replayed receipt into its
// identity fields.
func decodeADRRecordReceipt(b []byte) (adrRecordReceipt, bool) {
	if len(b) == 0 {
		return adrRecordReceipt{}, false
	}
	var rec adrRecordReceipt
	if err := json.Unmarshal(b, &rec); err != nil {
		return adrRecordReceipt{}, false
	}
	return rec, true
}

// validateADRRecordShape runs the configuration-independent request checks: the
// idempotency id shape, the required authored fields, the relates_to collection
// shape, and — when a producing change is supplied — its pin shape.
func validateADRRecordShape(req ADRRecordRequest) []StatusFinding {
	var findings []StatusFinding
	if !validRequestID(req.RequestID) {
		findings = append(findings, adrFinding("invalid-request_id", "request_id must be 8–128 ASCII characters matching ^[A-Za-z0-9][A-Za-z0-9._-]*$"))
	}
	return append(findings, validateADRContent(req)...)
}

// validateADRContent runs the configuration-independent checks on an ADR's
// authored content and references — the required authored fields, the
// relates_to collection shape, and (when supplied) the producing change's pin
// shape. It deliberately does NOT check req.RequestID, so it is reusable for a
// supersede/reverse successor whose own request_id is ignored (the outer key
// governs).
func validateADRContent(req ADRRecordRequest) []StatusFinding {
	var findings []StatusFinding
	add := func(code, msg string) {
		findings = append(findings, adrFinding(code, msg))
	}

	for _, f := range []struct{ name, val string }{
		{"title", req.Title}, {"context", req.Context}, {"decision", req.Decision},
		{"consequences", req.Consequences}, {"alternatives", req.Alternatives},
	} {
		if strings.TrimSpace(f.val) == "" {
			add("empty-"+f.name, f.name+" must be non-empty")
		}
	}
	findings = append(findings, validateIDCollection("relates_to", req.RelatesTo, FCInvalidRelatesTo, FCDuplicateRelatesTo)...)

	if req.Change != nil {
		if req.Change.ID <= 0 {
			add("invalid-change-id", "change.id must be a positive change id")
		}
		if strings.TrimSpace(req.Change.Path) == "" {
			add("empty-change-path", "change.path must name the producing change's current canonical record path")
		}
		if strings.TrimSpace(req.Change.Version) == "" {
			add("empty-change-version", "change.version must be the exact full blob object id of the producing change")
		}
	}
	return findings
}

// adrRecordOp is the SemanticOperation the engine drives per attempt for a new
// ADR. Every field is fixed before the transaction; the id-dependent work
// (allocation, serialization, graph validation, index and artifact-block
// rendering) re-runs from the attempt's own fresh state.
type adrRecordOp struct {
	req     ADRRecordRequest
	eff     config.Effective
	slug    string
	clock   transaction.Clock
	link    render.LinkContext
	adrsDir string
}

func (o adrRecordOp) Key() transaction.OperationKey { return OperationADRRecord }

// Plan allocates the next ADR id from this attempt's snapshot, serializes the
// canonical Accepted record, appends the id to a supplied producing change's
// adrs collection and re-renders that change's artifact block, validates the
// complete candidate ADR graph, re-renders the ADR index, and assembles the
// closed plan: the new ADR create, the index (created or replaced), and — when
// present — the producing change replace.
func (o adrRecordOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot

	// Reject any relates_to reference that does not resolve against the attempt's
	// snapshot — a request-shaped refusal. The whole-corpus ValidateADRGraph grades
	// an unresolved relates_to a warning (a corpus legitimately leaves them open),
	// so a write that introduces one is checked explicitly here.
	if findings := validateADRReferences(o.req.RelatesTo, snap); len(findings) > 0 {
		return transaction.MutationPlan{}, transaction.OperationResult{Refused: true, Findings: findings}, nil
	}

	// Allocate max(existing)+1; a gap below the highest id is never backfilled.
	newID := domain.NextADRID(snap)
	adrRelPath := path.Join(o.adrsDir, fmt.Sprintf("%04d-%s.md", int(newID), o.slug))

	var producingID *domain.ChangeID
	if o.req.Change != nil {
		id := domain.ChangeID(o.req.Change.ID)
		producingID = &id
	}

	adrBytes, err := render.ADRRecord(render.NewADRRecord{
		ID:           newID,
		Slug:         o.slug,
		Title:        o.req.Title,
		Date:         o.clock.Now().UTC(),
		Change:       producingID,
		RelatesTo:    toADRIDs(o.req.RelatesTo),
		Context:      o.req.Context,
		Decision:     o.req.Decision,
		Consequences: o.req.Consequences,
		Alternatives: o.req.Alternatives,
	})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("adr record: serializing record: %w", err)
	}

	// When a producing change is supplied, patch its adrs collection (the complete
	// new value) and refresh its updated date — the artifact block is re-rendered
	// in a second pass, because rendering it needs the candidate snapshot.
	var changePath string
	var changeIntermediate []byte
	if o.req.Change != nil {
		changePath = o.req.Change.Path
		c, out := snap.Change(*producingID)
		if out != domain.LookupFound {
			return refuseADR("not-found", fmt.Sprintf("producing change %04d is not present in the current corpus", o.req.Change.ID))
		}
		src, ok := st.State.Sources[changePath]
		if !ok {
			return refuseADR("path-mismatch", fmt.Sprintf("no record source loaded at %q for change %04d", changePath, o.req.Change.ID))
		}

		changeIntermediate, err = appendChangeADR(c, src, o.clock, newID)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("adr record: patching producing change: %w", err)
		}
	}

	// The candidate snapshot reflects the after-state: every existing corpus
	// document, plus the new ADR, plus the producing change swapped in when
	// present. It validates the ADR graph, resolves the index rows, and resolves
	// the producing change's artifact-block ADR rows — byte-for-byte the state the
	// engine's after-load will validate.
	candidate, err := buildADRCandidate(o.eff, st.State.Documents, adrRelPath, adrBytes, changePath, changeIntermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}

	// Validate the complete candidate ADR graph; an error-severity finding (a
	// dangling relates_to, a self-reference) is a request-shaped refusal. Warnings
	// (unallocated id gaps) never refuse.
	if findings := adrGraphErrors(domain.ValidateADRGraph(candidate)); len(findings) > 0 {
		return transaction.MutationPlan{}, transaction.OperationResult{Refused: true, Findings: findings}, nil
	}

	indexPath := path.Join(o.adrsDir, "README.md")
	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(adrRelPath), Kind: transaction.MutationCreate, Bytes: adrBytes},
	}
	if err := includeADRIndex(ctx, st.Tree, candidate, indexPath, &files); err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("adr record: %w", err)
	}

	// Re-render the producing change's artifact block over the candidate and plan
	// its replace.
	if o.req.Change != nil {
		gc, gout := candidate.Change(*producingID)
		if gout != domain.LookupFound {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("adr record: producing change %04d absent from candidate snapshot", o.req.Change.ID)
		}
		body, err := render.ArtifactBlockContent(gc, candidate, o.link)
		if err != nil {
			return refuseADR("artifact-render-failed", err.Error())
		}
		doc2, err := document.Parse(changeIntermediate)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("adr record: reparsing patched change: %w", err)
		}
		var ps2 document.PatchSet
		ps2.ReplaceBlock("artifacts", body)
		changeFinal, err := doc2.Apply(ps2)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("adr record: writing producing change artifact block: %w", err)
		}
		files = append(files, transaction.FileMutation{
			Path: gitcli.RepoPath(changePath), Kind: transaction.MutationReplace, Bytes: changeFinal,
		})
	}

	receipt, err := json.Marshal(adrRecordReceipt{ID: int(newID), Op: OperationADRRecord, Path: adrRelPath})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("adr record: encoding receipt: %w", err)
	}

	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("ADR %04d recorded (%s)", int(newID), o.slug),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}

// validateADRReferences reports every relates_to reference that does not resolve
// against snap — a dangling ADR reference on the new record.
func validateADRReferences(relatesTo []int, snap domain.Snapshot) []domain.Finding {
	var findings []domain.Finding
	for _, id := range relatesTo {
		if _, out := snap.ADR(domain.ADRID(id)); out != domain.LookupFound {
			findings = append(findings, domain.Finding{
				Code:     domain.CodeADRDanglingReference,
				Severity: domain.SeverityError,
				Entity:   domain.EntityRef{Kind: domain.EntityADR},
				Field:    "relates_to",
				Detail:   map[string]string{"reference": fmt.Sprintf("%d", id)},
			})
		}
	}
	return findings
}

// upsertField patches name to v, choosing SetField when the field is already
// present and InsertField when it is absent — so an owned field can be written
// whether or not a Bash-era record happens to carry it.
func upsertField(ps *document.PatchSet, doc document.Document, name string, v document.Value) {
	if _, ok := doc.Field(name); ok {
		ps.SetField(name, v)
		return
	}
	ps.InsertField(name, v)
}

// adrGraphErrors filters an ADR-graph validation result down to its
// error-severity findings — the ones that refuse the operation. Warnings (an
// unallocated id gap below the highest existing id) never refuse.
func adrGraphErrors(findings []domain.Finding) []domain.Finding {
	var out []domain.Finding
	for _, f := range findings {
		if f.Severity == domain.SeverityError {
			out = append(out, f)
		}
	}
	return out
}

// refuseADR builds a refusing OperationResult carrying one ADR-scoped finding.
func refuseADR(code, msg string) (transaction.MutationPlan, transaction.OperationResult, error) {
	return transaction.MutationPlan{}, transaction.OperationResult{
		Refused: true,
		Findings: []domain.Finding{{
			Code:     code,
			Severity: domain.SeverityError,
			Entity:   domain.EntityRef{Kind: domain.EntityADR},
			Detail:   map[string]string{"message": msg},
		}},
	}, nil
}

// buildADRCandidate rebuilds the complete snapshot the attempt would see after
// the ADR lands: every existing corpus document reclassified by its path, plus
// the new ADR at adrPath, with the producing change (when changePath is
// non-empty) swapped in at its path. The reclassification mirrors the planning
// loader so the candidate is the state the engine's after-load will validate.
func buildADRCandidate(eff config.Effective, docs map[string]document.Document, adrPath string, adrBytes []byte, changePath string, changeBytes []byte) (domain.Snapshot, error) {
	paths := make([]string, 0, len(docs))
	for p := range docs {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var changeDoc document.Document
	if changePath != "" {
		var err error
		changeDoc, err = document.Parse(changeBytes)
		if err != nil {
			return domain.Snapshot{}, fmt.Errorf("adr record: parsing patched change: %w", err)
		}
	}

	inputs := make([]repository.InputDocument, 0, len(docs)+1)
	for _, p := range paths {
		kind, loc, ok := classifyCorpusPath(eff, p)
		if !ok {
			continue
		}
		doc := docs[p]
		if changePath != "" && p == changePath {
			doc = changeDoc
		}
		inputs = append(inputs, repository.InputDocument{
			Kind: kind, Location: loc, Path: p, Document: doc,
		})
	}

	adrDoc, err := document.Parse(adrBytes)
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("adr record: parsing new ADR: %w", err)
	}
	inputs = append(inputs, repository.InputDocument{
		Kind: repository.KindADR, Location: repository.LocationLedger, Path: adrPath, Document: adrDoc,
	})

	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("adr record: building candidate snapshot: %w", err)
	}
	return build.Snapshot, nil
}

// appendChangeADR rebuilds a producing change's bytes with newID appended to its
// typed adrs collection and its updated date refreshed to the clock's UTC date —
// the first patch pass shared by adr record, supersede, and reverse. The
// artifact block is rendered in a second pass over the candidate snapshot,
// because rendering it needs the after-state.
func appendChangeADR(c domain.Change, src []byte, clock transaction.Clock, newID domain.ADRID) ([]byte, error) {
	newADRs := make([]int, 0, len(c.ADRs())+1)
	for _, a := range c.ADRs() {
		newADRs = append(newADRs, int(a))
	}
	newADRs = append(newADRs, int(newID))

	doc, err := document.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("parsing producing change: %w", err)
	}
	var ps document.PatchSet
	upsertField(&ps, doc, "adrs", intSeqValue(newADRs))
	upsertField(&ps, doc, "updated", document.String(clock.Now().UTC().Format("2006-01-02")))
	return doc.Apply(ps)
}

// This block is the `adr supersede` and `adr reverse` planning operations: both
// record one brand-new Accepted successor ADR AND flip an existing Accepted
// target's status, landing the new record, the re-rendered ADR index, the
// target's single status-field change, and — when a producing change is supplied
// — that change's appended adrs collection and re-rendered artifact block as one
// validated atomic transaction commit. The domain owns legality: domain.Supersede
// and domain.Reverse gate the target (it must be Accepted) and produce the exact
// flipped status. The successor allocates a fresh id, so each carries a
// caller-supplied idempotency key; the target and any producing change are pinned
// by exact-blob entity expectations so a concurrently moved record contends
// rather than being clobbered. The target's frozen body is preserved: only its
// status field changes.

// OperationADRSupersede and OperationADRReverse are the operation keys the two
// ADR-replacement transitions record in their result envelopes, transaction
// trailers, and idempotency digests.
const (
	OperationADRSupersede = "adr.supersede"
	OperationADRReverse   = "adr.reverse"
)

// adrNotAcceptedReason mirrors the domain's target-not-Accepted refusal reason
// (see domain.Supersede / domain.Reverse). A supersede/reverse refusal carrying
// this reason is state-shaped (the target is not in an Accepted state) and maps
// to invalid-state; every other refusal is request-shaped and maps to
// invalid-input.
const adrNotAcceptedReason = "adr-not-accepted"

// ADRTarget pins the existing Accepted ADR a supersede/reverse flips: its id, its
// current canonical path, and the exact full blob object id the transaction
// expects that record to carry.
type ADRTarget struct {
	ID      int    `json:"id"`
	Path    string `json:"path"`
	Version string `json:"version"`
}

// ADRReplaceRequest is the closed, caller-supplied request for one supersede or
// reverse. RequestID governs idempotency; Target pins the flipped ADR; Successor
// carries the brand-new ADR's authored content and references (its own RequestID
// is ignored — the outer key governs).
type ADRReplaceRequest struct {
	RequestID string           `json:"request_id" docket:"required"`
	Target    ADRTarget        `json:"target"`
	Successor ADRRecordRequest `json:"successor"`
}

// adrReplacePayload is the semantic content of a supersede/reverse — the verb,
// the target identity, and the successor's authored content — minus the
// caller-chosen RequestID and every concurrency Version pin. It is the digest
// payload.
type adrReplacePayload struct {
	Op         string           `json:"op"`
	TargetID   int              `json:"target_id"`
	TargetPath string           `json:"target_path"`
	Successor  adrRecordPayload `json:"successor"`
}

func adrReplaceSemanticPayload(opKey string, req ADRReplaceRequest) adrReplacePayload {
	return adrReplacePayload{
		Op:         opKey,
		TargetID:   req.Target.ID,
		TargetPath: req.Target.Path,
		Successor:  adrRecordSemanticPayload(req.Successor),
	}
}

// ADRSupersede records a successor ADR that supersedes an existing Accepted
// target, flipping the target to "Superseded by ADR-<successor>".
func ADRSupersede(ctx context.Context, deps PlanningDeps, repoDir string, req ADRReplaceRequest) ADRResult {
	return adrReplace(ctx, deps, repoDir, OperationADRSupersede, false, req)
}

// ADRReverse records a successor ADR that reverses an existing Accepted target,
// flipping the target to "Reversed by ADR-<successor>".
func ADRReverse(ctx context.Context, deps PlanningDeps, repoDir string, req ADRReplaceRequest) ADRResult {
	return adrReplace(ctx, deps, repoDir, OperationADRReverse, true, req)
}

// adrReplace is the shared driver both replacement transitions compose: it
// validates the request, pins authoritative context, and drives one atomic
// transaction carrying the successor record, the target's status flip, the
// re-rendered index, and any producing-change update. reverses selects the verb
// (domain.Reverse and the reverses edge) over supersede.
func adrReplace(ctx context.Context, deps PlanningDeps, repoDir, opKey string, reverses bool, req ADRReplaceRequest) ADRResult {
	if findings := validateADRReplaceShape(req); len(findings) > 0 {
		return newADRResult(opKey, ResultInvalidInput, ADRResult{Findings: findings})
	}

	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newADRResult(opKey, result, ADRResult{Findings: []StatusFinding{adrFinding(reason, err.Error())}})
	}
	eff := pin.Config.Effective

	slug := slugifyTitle(req.Successor.Title)
	if !domain.ValidSlugToken(slug) {
		return newADRResult(opKey, ResultInvalidInput, ADRResult{
			Findings: []StatusFinding{adrFinding("invalid-slug", fmt.Sprintf("successor title %q does not yield a valid slug", req.Successor.Title))},
		})
	}

	digest, err := canonicalDigest(opKey, adrReplaceSemanticPayload(opKey, req))
	if err != nil {
		return newADRResult(opKey, ResultInternalError, ADRResult{Findings: []StatusFinding{adrFinding(ReasonStatusInternalError, err.Error())}})
	}

	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return newADRResult(opKey, result, ADRResult{Findings: []StatusFinding{adrFinding(reason, err.Error())}})
	}

	op := adrReplaceOp{
		opKey:    opKey,
		req:      req,
		reverses: reverses,
		eff:      eff,
		slug:     slug,
		clock:    deps.Clock,
		link:     linkContextOf(pin),
		adrsDir:  eff.ADRsDir.Value,
	}

	// The target ADR is always pinned by an exact-blob entity expectation; a
	// supplied producing change adds a second one, so a concurrently moved record
	// contends rather than being silently clobbered.
	expected := []transaction.EntityExpectation{{
		Path:    gitcli.RepoPath(req.Target.Path),
		Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(req.Target.Version)},
	}}
	if req.Successor.Change != nil {
		expected = append(expected, transaction.EntityExpectation{
			Path:    gitcli.RepoPath(req.Successor.Change.Path),
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(req.Successor.Change.Version)},
		})
	}

	res, execErr := deps.Engine.Execute(ctx, transaction.Request{
		Repository:  repo,
		Remote:      originRemote,
		TargetRef:   gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName),
		Idempotency: &transaction.IdempotencyKey{RequestID: req.RequestID, Digest: digest},
		Loader:      newPlanningLoader(eff),
		Operation:   op,
		Expected:    expected,
	})

	// A refusal is state-shaped when it carries the domain not-Accepted reason (the
	// target-must-be-Accepted gate); every other refusal (a dangling successor
	// relates_to, an absent producing change) is request-shaped.
	result, replayed := mapOutcome(res, execErr, adrReplaceRefusalKind(res.Findings))
	out := ADRResult{Findings: findingsToStatus(res.Findings)}
	if result == ResultApplied {
		if rec, ok := decodeADRRecordReceipt(res.Receipt); ok {
			out.ID = rec.ID
			out.Path = rec.Path
		}
		out.Revision = string(res.AppliedCommit)
		out.Replayed = replayed
	}
	r := newADRResult(opKey, result, out)
	r.Failure = failureStatus(res, execErr)
	return r
}

// adrReplaceRefusalKind chooses the result a supersede/reverse refusal maps to:
// the target-not-Accepted gate is state-shaped → invalid-state; every other
// refusal is request-shaped → invalid-input.
func adrReplaceRefusalKind(findings []domain.Finding) Result {
	for _, f := range findings {
		if f.Code == adrNotAcceptedReason {
			return ResultInvalidState
		}
	}
	return ResultInvalidInput
}

// validateADRReplaceShape runs the configuration-independent request checks: the
// outer idempotency id shape, the target's pin shape, and the successor's
// authored content and references (its own request_id is ignored).
func validateADRReplaceShape(req ADRReplaceRequest) []StatusFinding {
	var findings []StatusFinding
	if !validRequestID(req.RequestID) {
		findings = append(findings, adrFinding("invalid-request_id", "request_id must be 8–128 ASCII characters matching ^[A-Za-z0-9][A-Za-z0-9._-]*$"))
	}
	if req.Target.ID <= 0 {
		findings = append(findings, adrFinding("invalid-target-id", "target.id must be a positive ADR id"))
	}
	if strings.TrimSpace(req.Target.Path) == "" {
		findings = append(findings, adrFinding("empty-target-path", "target.path must name the target ADR's current canonical record path"))
	}
	if strings.TrimSpace(req.Target.Version) == "" {
		findings = append(findings, adrFinding("empty-target-version", "target.version must be the exact full blob object id of the Accepted target"))
	}
	return append(findings, validateADRContent(req.Successor)...)
}

// adrReplaceOp is the SemanticOperation the engine drives per attempt for a
// supersede or reverse. Every field is fixed before the transaction; the
// id-dependent work (allocation, the domain legality gate, serialization, the
// target's status flip, graph validation, index and artifact-block rendering)
// re-runs from the attempt's own fresh state.
type adrReplaceOp struct {
	opKey    string
	req      ADRReplaceRequest
	reverses bool
	eff      config.Effective
	slug     string
	clock    transaction.Clock
	link     render.LinkContext
	adrsDir  string
}

func (o adrReplaceOp) Key() transaction.OperationKey { return transaction.OperationKey(o.opKey) }

// Plan allocates the next ADR id, gates the target through the domain verb (which
// requires it to be Accepted and yields the flipped status), serializes the
// canonical Accepted successor carrying the supersedes/reverses edge, patches the
// target's status field ONLY (preserving its frozen body), appends the id to a
// supplied producing change and re-renders its artifact block, validates the
// complete candidate ADR graph, re-renders the index, and assembles the closed
// plan: the successor create, the target replace, the index, and — when present —
// the producing change replace.
func (o adrReplaceOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot
	succ := o.req.Successor

	// Request-shaped: a dangling successor relates_to reference (the whole-corpus
	// graph validation grades an open relates_to a warning, so a write that
	// introduces one is checked explicitly here).
	if findings := validateADRReferences(succ.RelatesTo, snap); len(findings) > 0 {
		return transaction.MutationPlan{}, transaction.OperationResult{Refused: true, Findings: findings}, nil
	}

	// Allocate max(existing)+1; a gap below the highest id is never backfilled.
	newID := domain.NextADRID(snap)
	targetID := domain.ADRID(o.req.Target.ID)

	// Domain legality gate: the verb requires an Accepted target and yields the
	// flipped status. A not-Accepted target is a state-shaped refusal; an unknown
	// or ambiguous target is request-shaped — both carry the domain's stable reason
	// token as the finding code.
	var flip domain.ADRActionResult
	var fail *domain.PolicyFailure
	if o.reverses {
		flip, fail = domain.Reverse(snap, targetID, newID)
	} else {
		flip, fail = domain.Supersede(snap, targetID, newID)
	}
	if fail != nil {
		return refuseADRPolicy(fail)
	}

	// Patch the target's status field only — its frozen body is preserved, and
	// repository.ValidateEvolution's frozen-ADR check enforces that the diff is
	// confined to the status value span.
	oldSrc, ok := st.State.Sources[o.req.Target.Path]
	if !ok {
		return refuseADR("path-mismatch", fmt.Sprintf("no record source loaded at %q for target ADR %04d", o.req.Target.Path, o.req.Target.ID))
	}
	oldDoc, err := document.Parse(oldSrc)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("adr replace: parsing target: %w", err)
	}
	var psOld document.PatchSet
	upsertField(&psOld, oldDoc, "status", document.String(flip.NewStatus.String()))
	oldBytes, err := oldDoc.Apply(psOld)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("adr replace: patching target status: %w", err)
	}
	oldPatched, err := document.Parse(oldBytes)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("adr replace: reparsing patched target: %w", err)
	}

	adrRelPath := path.Join(o.adrsDir, fmt.Sprintf("%04d-%s.md", int(newID), o.slug))

	var producingID *domain.ChangeID
	if succ.Change != nil {
		id := domain.ChangeID(succ.Change.ID)
		producingID = &id
	}

	var supersedes, reverses []domain.ADRID
	if o.reverses {
		reverses = []domain.ADRID{targetID}
	} else {
		supersedes = []domain.ADRID{targetID}
	}

	adrBytes, err := render.ADRRecord(render.NewADRRecord{
		ID:           newID,
		Slug:         o.slug,
		Title:        succ.Title,
		Date:         o.clock.Now().UTC(),
		Change:       producingID,
		RelatesTo:    toADRIDs(succ.RelatesTo),
		Supersedes:   supersedes,
		Reverses:     reverses,
		Context:      succ.Context,
		Decision:     succ.Decision,
		Consequences: succ.Consequences,
		Alternatives: succ.Alternatives,
	})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("adr replace: serializing successor: %w", err)
	}

	// When a producing change is supplied, patch its adrs collection and refresh
	// its updated date — the artifact block is re-rendered in a second pass.
	var changePath string
	var changeIntermediate []byte
	if succ.Change != nil {
		changePath = succ.Change.Path
		c, out := snap.Change(*producingID)
		if out != domain.LookupFound {
			return refuseADR("not-found", fmt.Sprintf("producing change %04d is not present in the current corpus", succ.Change.ID))
		}
		src, ok := st.State.Sources[changePath]
		if !ok {
			return refuseADR("path-mismatch", fmt.Sprintf("no record source loaded at %q for change %04d", changePath, succ.Change.ID))
		}
		changeIntermediate, err = appendChangeADR(c, src, o.clock, newID)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("adr replace: patching producing change: %w", err)
		}
	}

	// The candidate snapshot reflects the after-state: every existing corpus
	// document, with the target's document swapped for its status-flipped bytes,
	// plus the new successor ADR, plus the producing change swapped in when present.
	docsCopy := make(map[string]document.Document, len(st.State.Documents))
	for p, d := range st.State.Documents {
		docsCopy[p] = d
	}
	docsCopy[o.req.Target.Path] = oldPatched

	candidate, err := buildADRCandidate(o.eff, docsCopy, adrRelPath, adrBytes, changePath, changeIntermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}

	// Validate the complete candidate ADR graph; an error-severity finding is a
	// request-shaped refusal (warnings never refuse).
	if findings := adrGraphErrors(domain.ValidateADRGraph(candidate)); len(findings) > 0 {
		return transaction.MutationPlan{}, transaction.OperationResult{Refused: true, Findings: findings}, nil
	}

	indexPath := path.Join(o.adrsDir, "README.md")
	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(adrRelPath), Kind: transaction.MutationCreate, Bytes: adrBytes},
		{Path: gitcli.RepoPath(o.req.Target.Path), Kind: transaction.MutationReplace, Bytes: oldBytes},
	}
	if err := includeADRIndex(ctx, st.Tree, candidate, indexPath, &files); err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("adr replace: %w", err)
	}

	// Re-render the producing change's artifact block over the candidate and plan
	// its replace.
	if succ.Change != nil {
		gc, gout := candidate.Change(*producingID)
		if gout != domain.LookupFound {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("adr replace: producing change %04d absent from candidate snapshot", succ.Change.ID)
		}
		body, err := render.ArtifactBlockContent(gc, candidate, o.link)
		if err != nil {
			return refuseADR("artifact-render-failed", err.Error())
		}
		doc2, err := document.Parse(changeIntermediate)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("adr replace: reparsing patched change: %w", err)
		}
		var ps2 document.PatchSet
		ps2.ReplaceBlock("artifacts", body)
		changeFinal, err := doc2.Apply(ps2)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("adr replace: writing producing change artifact block: %w", err)
		}
		files = append(files, transaction.FileMutation{
			Path: gitcli.RepoPath(changePath), Kind: transaction.MutationReplace, Bytes: changeFinal,
		})
	}

	verb := "superseded"
	if o.reverses {
		verb = "reversed"
	}
	receipt, err := json.Marshal(adrRecordReceipt{ID: int(newID), Op: o.opKey, Path: adrRelPath})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("adr replace: encoding receipt: %w", err)
	}

	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("ADR %04d recorded (%s), ADR %04d %s", int(newID), o.slug, int(targetID), verb),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}

// refuseADRPolicy builds a refusing OperationResult from a domain policy failure,
// carrying the domain's stable reason token as the finding code and its operands
// as detail — the target-not-Accepted (and unknown/ambiguous target) refusal path.
func refuseADRPolicy(fail *domain.PolicyFailure) (transaction.MutationPlan, transaction.OperationResult, error) {
	detail := map[string]string{"reason": fail.Reason}
	for k, v := range fail.Detail {
		detail[k] = v
	}
	return transaction.MutationPlan{}, transaction.OperationResult{
		Refused: true,
		Findings: []domain.Finding{{
			Code:     fail.Reason,
			Severity: domain.SeverityError,
			Entity:   domain.EntityRef{Kind: domain.EntityADR},
			Detail:   detail,
		}},
	}, nil
}
