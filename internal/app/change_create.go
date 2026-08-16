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
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// This file is the `change create` planning operation: it validates a typed
// creation request up front, pins authoritative context through the 0310 read
// seam, and hands a fresh-state SemanticOperation closure to the 0309 engine.
// The closure allocates the next change id from the attempt's own snapshot,
// serializes one canonical proposed change record, fills its artifact block,
// re-renders the inline board when that surface is enabled, and commits the
// closed file set as one atomic transaction under an exact-lease push. It
// decides no lifecycle policy of its own — the domain owns transitions — and it
// never rewrites an existing record: the only mutations it plans are the new
// record's create and, for inline boards, the board file.

// OperationChangeCreate is the operation key `change create` records in its
// result envelope, its transaction trailer, and its idempotency digest.
const OperationChangeCreate = "change.create"

// ChangeCreateRequest is the closed, caller-supplied request for one change
// creation. Authored Markdown rides inside the string fields and is never
// interpolated into any shell command. The relationship collections are the
// complete desired values for the new record.
type ChangeCreateRequest struct {
	RequestID string `json:"request_id"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Priority  string `json:"priority"`

	Why         string `json:"why"`
	WhatChanges string `json:"what_changes"`
	OutOfScope  string `json:"out_of_scope"`

	DependsOn      []int `json:"depends_on"`
	StackedOn      *int  `json:"stacked_on"`
	Related        []int `json:"related"`
	DiscoveredFrom []int `json:"discovered_from"`
	ADRs           []int `json:"adrs"`
}

// ChangeCreateResult is the protocol-v1 document `change create` returns. It
// embeds the envelope; the identity fields are populated on a successful apply
// or an idempotent replay, and Findings carries every refusal or validation
// diagnostic (marshalled as [] never null).
type ChangeCreateResult struct {
	Envelope
	ID       int             `json:"id,omitempty"`
	Slug     string          `json:"slug,omitempty"`
	Path     string          `json:"path,omitempty"`
	Revision string          `json:"committed_revision,omitempty"`
	Replayed bool            `json:"replayed,omitempty"`
	Findings []StatusFinding `json:"findings"`
}

// HumanText renders the one-line human summary of a change-create outcome.
func (r ChangeCreateResult) HumanText() string {
	switch r.Result {
	case ResultApplied:
		verb := "created"
		if r.Replayed {
			verb = "already created"
		}
		return fmt.Sprintf("change %04d %s (%s) — %s", r.ID, verb, r.Slug, r.Revision)
	default:
		return fmt.Sprintf("change create: %s", r.Result)
	}
}

// newChangeCreateResult stamps the envelope and normalizes Findings to an empty
// slice so the array marshals as [] on every path.
func newChangeCreateResult(result Result, r ChangeCreateResult) ChangeCreateResult {
	r.Envelope = NewEnvelope(OperationChangeCreate, result)
	if r.Findings == nil {
		r.Findings = []StatusFinding{}
	}
	return r
}

// changeCreateReceipt is the canonical receipt persisted with a change-create
// commit and replayed verbatim on an idempotent re-run. Field order is
// alphabetical so json.Marshal emits the canonical, sorted-key compact form the
// engine's receipt validator requires.
type changeCreateReceipt struct {
	ID   int    `json:"id"`
	Op   string `json:"op"`
	Path string `json:"path"`
	Slug string `json:"slug"`
}

// changeCreatePayload is the request's semantic content — every input that
// governs the produced record — minus the caller-chosen RequestID. It is the
// digest payload: two requests with the same id must carry the same content, so
// RequestID itself is excluded and only what the record depends on is hashed.
type changeCreatePayload struct {
	Title          string `json:"title"`
	Type           string `json:"type"`
	Priority       string `json:"priority"`
	Why            string `json:"why"`
	WhatChanges    string `json:"what_changes"`
	OutOfScope     string `json:"out_of_scope"`
	DependsOn      []int  `json:"depends_on"`
	StackedOn      *int   `json:"stacked_on"`
	Related        []int  `json:"related"`
	DiscoveredFrom []int  `json:"discovered_from"`
	ADRs           []int  `json:"adrs"`
}

func changeCreateSemanticPayload(req ChangeCreateRequest) changeCreatePayload {
	return changeCreatePayload{
		Title:          req.Title,
		Type:           req.Type,
		Priority:       req.Priority,
		Why:            req.Why,
		WhatChanges:    req.WhatChanges,
		OutOfScope:     req.OutOfScope,
		DependsOn:      req.DependsOn,
		StackedOn:      req.StackedOn,
		Related:        req.Related,
		DiscoveredFrom: req.DiscoveredFrom,
		ADRs:           req.ADRs,
	}
}

// ChangeCreate validates the request, pins authoritative context, and drives one
// atomic transaction that lands the new change record and — when the inline
// board surface is enabled — the re-rendered board. Every failure that predates
// the transaction (bad request shape, unknown type/priority, a github board
// surface) returns without an engine call.
func ChangeCreate(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeCreateRequest) ChangeCreateResult {
	// 1. Request-shape validation independent of configuration. A failure here
	//    never reaches the engine.
	if findings := validateChangeCreateShape(req); len(findings) > 0 {
		return newChangeCreateResult(ResultInvalidInput, ChangeCreateResult{Findings: findings})
	}

	// 2. Pin authoritative context: the metadata mode, branches, and resolved
	//    configuration the closed-value checks and the board fence consult.
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newChangeCreateResult(result, ChangeCreateResult{
			Findings: []StatusFinding{{Code: reason, Severity: string(domain.SeverityError), Message: err.Error()}},
		})
	}
	eff := pin.Config.Effective

	// 3. Closed-value validation against the resolved configuration: the change
	//    type must be configured (only configuration knows the closed set).
	if findings := validateChangeCreateConfig(eff, req); len(findings) > 0 {
		return newChangeCreateResult(ResultInvalidInput, ChangeCreateResult{Findings: findings})
	}

	// 4. Board-surface fence: a github surface is an unsupported configuration,
	//    refused before any transaction; otherwise learn whether inline is on.
	inline, err := fenceBoardSurface(eff)
	if err != nil {
		if pe, ok := asPlanningError(err); ok {
			return newChangeCreateResult(pe.Result, ChangeCreateResult{
				Findings: []StatusFinding{{Code: pe.Reason, Severity: string(domain.SeverityError), Message: pe.Message}},
			})
		}
		return newChangeCreateResult(ResultInternalError, ChangeCreateResult{
			Findings: []StatusFinding{{Code: ReasonStatusInternalError, Severity: string(domain.SeverityError), Message: err.Error()}},
		})
	}

	// 5. The slug is derived from the title alone (configuration-independent), so
	//    it is fixed once here rather than re-derived per attempt.
	slug := slugifyTitle(req.Title)
	if !domain.ValidSlugToken(slug) {
		return newChangeCreateResult(ResultInvalidInput, ChangeCreateResult{
			Findings: []StatusFinding{{
				Code: "invalid-slug", Severity: string(domain.SeverityError),
				Message: fmt.Sprintf("title %q does not yield a valid slug", req.Title),
			}},
		})
	}

	// 6. The idempotency digest binds the operation and the request's semantic
	//    content (never the caller-chosen request id).
	digest, err := canonicalDigest(OperationChangeCreate, changeCreateSemanticPayload(req))
	if err != nil {
		return newChangeCreateResult(ResultInternalError, ChangeCreateResult{
			Findings: []StatusFinding{{Code: ReasonStatusInternalError, Severity: string(domain.SeverityError), Message: err.Error()}},
		})
	}

	// 7. Discover the repository identity the transaction writes against.
	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return newChangeCreateResult(result, ChangeCreateResult{
			Findings: []StatusFinding{{Code: reason, Severity: string(domain.SeverityError), Message: err.Error()}},
		})
	}

	op := changeCreateOp{
		req:        req,
		eff:        eff,
		slug:       slug,
		clock:      deps.Clock,
		inline:     inline,
		link:       render.LinkContext{MetadataBranch: metadataBranchOf(pin)},
		changesDir: eff.ChangesDir.Value,
	}

	res, execErr := deps.Engine.Execute(ctx, transaction.Request{
		Repository:  repo,
		Remote:      originRemote,
		TargetRef:   gitcli.RefName(branchRefPrefix + metadataBranchOf(pin)),
		Idempotency: &transaction.IdempotencyKey{RequestID: req.RequestID, Digest: digest},
		Loader:      newPlanningLoader(eff),
		Operation:   op,
	})

	return changeCreateResultFromOutcome(res, execErr)
}

// changeCreateResultFromOutcome folds a transaction outcome into the result
// document. A refusal from this operation is always request-shaped (a dangling
// relationship reference), so the refusal maps onto invalid-input.
func changeCreateResultFromOutcome(res transaction.Result, execErr error) ChangeCreateResult {
	result, replayed := mapOutcome(res, execErr, ResultInvalidInput)

	out := ChangeCreateResult{Findings: findingsToStatus(res.Findings)}
	switch result {
	case ResultApplied:
		if rec, ok := decodeChangeCreateReceipt(res.Receipt); ok {
			out.ID = rec.ID
			out.Slug = rec.Slug
			out.Path = rec.Path
		}
		out.Revision = string(res.AppliedCommit)
		out.Replayed = replayed
	}
	return newChangeCreateResult(result, out)
}

// validateChangeCreateShape runs the configuration-independent request checks:
// the idempotency id shape, the required text fields, and the relationship
// collections (positive ids, no duplicate within a collection).
func validateChangeCreateShape(req ChangeCreateRequest) []StatusFinding {
	var findings []StatusFinding
	add := func(code, msg string) {
		findings = append(findings, StatusFinding{Code: code, Severity: string(domain.SeverityError), Message: msg})
	}

	if !validRequestID(req.RequestID) {
		add("invalid-request-id", "request_id must be 8–128 ASCII characters matching ^[A-Za-z0-9][A-Za-z0-9._-]*$")
	}
	for _, f := range []struct{ name, val string }{
		{"title", req.Title}, {"why", req.Why},
		{"what_changes", req.WhatChanges}, {"out_of_scope", req.OutOfScope},
	} {
		if strings.TrimSpace(f.val) == "" {
			add("empty-"+f.name, f.name+" must be non-empty")
		}
	}
	for _, coll := range []struct {
		name string
		ids  []int
	}{
		{"depends_on", req.DependsOn}, {"related", req.Related},
		{"discovered_from", req.DiscoveredFrom}, {"adrs", req.ADRs},
	} {
		findings = append(findings, validateIDCollection(coll.name, coll.ids)...)
	}
	if req.StackedOn != nil && *req.StackedOn <= 0 {
		add("invalid-stacked_on", "stacked_on must be a positive change id")
	}
	return findings
}

// validateIDCollection reports positive-id and no-duplicate violations for one
// relationship collection.
func validateIDCollection(name string, ids []int) []StatusFinding {
	var findings []StatusFinding
	seen := make(map[int]bool, len(ids))
	for _, id := range ids {
		if id <= 0 {
			findings = append(findings, StatusFinding{
				Code: "invalid-" + name, Severity: string(domain.SeverityError),
				Message: fmt.Sprintf("%s contains a non-positive id %d", name, id),
			})
			continue
		}
		if seen[id] {
			findings = append(findings, StatusFinding{
				Code: "duplicate-" + name, Severity: string(domain.SeverityError),
				Message: fmt.Sprintf("%s lists id %d more than once", name, id),
			})
			continue
		}
		seen[id] = true
	}
	return findings
}

// validateChangeCreateConfig runs the closed-value checks that need the resolved
// configuration: the change type must be one of the configured change_types, and
// the priority must be a closed domain spelling.
func validateChangeCreateConfig(eff config.Effective, req ChangeCreateRequest) []StatusFinding {
	var findings []StatusFinding
	if !containsString(eff.ChangeTypes.Value, req.Type) {
		findings = append(findings, StatusFinding{
			Code: "unknown-type", Severity: string(domain.SeverityError),
			Message: fmt.Sprintf("unknown change type %q; configured change_types are %s",
				req.Type, strings.Join(eff.ChangeTypes.Value, ", ")),
		})
	}
	if _, ok := domain.ParsePriority(req.Priority); !ok {
		findings = append(findings, StatusFinding{
			Code: "unknown-priority", Severity: string(domain.SeverityError),
			Message: fmt.Sprintf("unknown priority %q; valid values are critical, high, medium, low", req.Priority),
		})
	}
	return findings
}

// validRequestID enforces the idempotency id grammar: 8–128 ASCII characters,
// beginning with an alphanumeric, over [A-Za-z0-9._-].
func validRequestID(id string) bool {
	if len(id) < 8 || len(id) > 128 {
		return false
	}
	if c := id[0]; !isAlphanumeric(c) {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !(isAlphanumeric(c) || c == '.' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func isAlphanumeric(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

// slugifyTitle mirrors mint-stub.sh's slugify: lowercase, every run of
// non-alphanumerics becomes a single hyphen, leading/trailing hyphens are
// trimmed, the result is capped at 60 bytes, then trailing hyphens are trimmed
// again (the cap can reopen a hyphen the first trim never saw).
func slugifyTitle(title string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevHyphen = false
			continue
		}
		if !prevHyphen {
			b.WriteByte('-')
			prevHyphen = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > 60 {
		s = s[:60]
	}
	return strings.TrimRight(s, "-")
}

// metadataBranchOf returns the branch the metadata records live on: the metadata
// branch in docket mode, and the default branch in main mode.
func metadataBranchOf(pin StatusPin) string {
	if pin.Mode == metadataModeMain {
		return pin.DefaultBranch
	}
	return pin.MetadataBranch
}

// findingsToStatus converts a slice of domain findings into the DTO shape,
// always returning a non-nil slice so the result's Findings marshals as [].
func findingsToStatus(findings []domain.Finding) []StatusFinding {
	out := make([]StatusFinding, 0, len(findings))
	for _, f := range findings {
		out = append(out, validationFinding(f))
	}
	return out
}

// decodeChangeCreateReceipt decodes a persisted or replayed receipt into its
// identity fields.
func decodeChangeCreateReceipt(b []byte) (changeCreateReceipt, bool) {
	if len(b) == 0 {
		return changeCreateReceipt{}, false
	}
	var rec changeCreateReceipt
	if err := json.Unmarshal(b, &rec); err != nil {
		return changeCreateReceipt{}, false
	}
	return rec, true
}

// changeCreateOp is the SemanticOperation the engine drives per attempt. Every
// field is fixed before the transaction; the id-dependent work (allocation,
// serialization, rendering) re-runs from the attempt's own fresh state.
type changeCreateOp struct {
	req        ChangeCreateRequest
	eff        config.Effective
	slug       string
	clock      transaction.Clock
	inline     bool
	link       render.LinkContext
	changesDir string
}

func (o changeCreateOp) Key() transaction.OperationKey { return OperationChangeCreate }

// Plan allocates the next id from this attempt's snapshot, validates the
// request's relationship references against it, serializes the canonical record,
// fills its artifact block, and assembles the closed plan: the new record's
// create plus, when inline is enabled, the re-rendered board.
func (o changeCreateOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot

	// Allocate max(active ∪ archive id) + 1; gaps are never filled.
	newID := nextChangeID(snap)

	// Reject any relationship reference that does not resolve against the
	// attempt's snapshot — a request-shaped refusal.
	if findings := validateChangeReferences(o.req, snap); len(findings) > 0 {
		return transaction.MutationPlan{}, transaction.OperationResult{Refused: true, Findings: findings}, nil
	}

	recordBytes, err := render.ChangeRecord(render.NewChangeRecord{
		ID:             newID,
		Slug:           o.slug,
		Title:          o.req.Title,
		Type:           o.req.Type,
		Priority:       o.req.Priority,
		Created:        o.clock.Now().UTC(),
		DependsOn:      toChangeIDs(o.req.DependsOn),
		StackedOn:      toChangeIDPtr(o.req.StackedOn),
		Related:        toChangeIDs(o.req.Related),
		DiscoveredFrom: toChangeIDs(o.req.DiscoveredFrom),
		ADRs:           toADRIDs(o.req.ADRs),
		Why:            o.req.Why,
		WhatChanges:    o.req.WhatChanges,
		OutOfScope:     o.req.OutOfScope,
	})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change create: serializing record: %w", err)
	}

	activePath := path.Join(o.changesDir, "active", fmt.Sprintf("%04d-%s.md", int(newID), o.slug))

	// A candidate snapshot — the before-state plus the new record — is needed to
	// render the board (which must reflect the new change) and to resolve the new
	// record's ADR references to their paths. It is built only when something
	// consumes it.
	var candidate domain.Snapshot
	if o.inline || len(o.req.ADRs) > 0 {
		candidate, err = buildCandidateSnapshot(o.eff, st.State.Documents, recordBytes, activePath)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, err
		}
	}

	// Fill the artifact block. A freshly created change has no spec/plan/results,
	// so the only possible row is ADRs; when there are none the serializer's empty
	// block is already correct.
	finalBytes := recordBytes
	if len(o.req.ADRs) > 0 {
		c, out := candidate.Change(newID)
		if out != domain.LookupFound {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change create: new record %04d absent from candidate snapshot", int(newID))
		}
		body, err := render.ArtifactBlockContent(c, candidate, o.link)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change create: rendering artifact block: %w", err)
		}
		if body != "" {
			doc, perr := document.Parse(recordBytes)
			if perr != nil {
				return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change create: reparsing record: %w", perr)
			}
			var ps document.PatchSet
			ps.ReplaceBlock("artifacts", body)
			finalBytes, err = doc.Apply(ps)
			if err != nil {
				return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change create: writing artifact block: %w", err)
			}
		}
	}

	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(activePath), Kind: transaction.MutationCreate, Bytes: finalBytes},
	}

	if o.inline {
		boardBytes, err := render.Board(render.BoardInput{Snapshot: candidate})
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change create: rendering board: %w", err)
		}
		boardPath := path.Join(o.changesDir, "BOARD.md")
		kind, err := boardMutationKind(ctx, st.Tree, boardPath)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, err
		}
		files = append(files, transaction.FileMutation{
			Path: gitcli.RepoPath(boardPath), Kind: kind, Bytes: boardBytes,
		})
	}

	receipt, err := json.Marshal(changeCreateReceipt{
		ID: int(newID), Op: OperationChangeCreate, Path: activePath, Slug: o.slug,
	})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change create: encoding receipt: %w", err)
	}

	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d created (%s)", int(newID), o.slug),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}

// nextChangeID returns max(id over every change, active or archived) + 1, so an
// id is never reused and a gap left by an archived or deleted change is never
// backfilled.
func nextChangeID(snap domain.Snapshot) domain.ChangeID {
	var highest domain.ChangeID
	for _, c := range snap.Changes() {
		if c.ID() > highest {
			highest = c.ID()
		}
	}
	return highest + 1
}

// validateChangeReferences reports every relationship reference that does not
// resolve against snap: a dangling change id in depends_on/related/
// discovered_from/stacked_on, or a dangling ADR id in adrs.
func validateChangeReferences(req ChangeCreateRequest, snap domain.Snapshot) []domain.Finding {
	var findings []domain.Finding
	checkChange := func(field string, ids []int) {
		for _, id := range ids {
			if _, out := snap.Change(domain.ChangeID(id)); out != domain.LookupFound {
				findings = append(findings, domain.Finding{
					Code:     "dangling-reference",
					Severity: domain.SeverityError,
					Entity:   domain.EntityRef{Kind: domain.EntityChange},
					Field:    field,
					Detail:   map[string]string{"reference": fmt.Sprintf("%d", id)},
				})
			}
		}
	}
	checkChange("depends_on", req.DependsOn)
	checkChange("related", req.Related)
	checkChange("discovered_from", req.DiscoveredFrom)
	if req.StackedOn != nil {
		checkChange("stacked_on", []int{*req.StackedOn})
	}
	for _, id := range req.ADRs {
		if _, out := snap.ADR(domain.ADRID(id)); out != domain.LookupFound {
			findings = append(findings, domain.Finding{
				Code:     "dangling-reference",
				Severity: domain.SeverityError,
				Entity:   domain.EntityRef{Kind: domain.EntityChange},
				Field:    "adrs",
				Detail:   map[string]string{"reference": fmt.Sprintf("%d", id)},
			})
		}
	}
	return findings
}

// buildCandidateSnapshot rebuilds the complete snapshot the attempt would see
// after the new record lands: every existing corpus document reclassified by its
// path, plus the new change record. The reclassification mirrors the planning
// loader — the same directory-prefix rule — so the candidate is byte-for-byte
// the state the engine's after-load will validate.
func buildCandidateSnapshot(eff config.Effective, docs map[string]document.Document, recordBytes []byte, newPath string) (domain.Snapshot, error) {
	paths := make([]string, 0, len(docs))
	for p := range docs {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	inputs := make([]repository.InputDocument, 0, len(docs)+1)
	for _, p := range paths {
		kind, loc, ok := classifyCorpusPath(eff, p)
		if !ok {
			continue
		}
		inputs = append(inputs, repository.InputDocument{
			Kind: kind, Location: loc, Path: p, Document: docs[p],
		})
	}

	newDoc, err := document.Parse(recordBytes)
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("change create: parsing new record: %w", err)
	}
	inputs = append(inputs, repository.InputDocument{
		Kind: repository.KindChange, Location: repository.LocationActive, Path: newPath, Document: newDoc,
	})

	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("change create: building candidate snapshot: %w", err)
	}
	return build.Snapshot, nil
}

// boardMutationKind reports whether the board file already exists on the base
// tree — a replace — or is being created for the first time.
func boardMutationKind(ctx context.Context, tree transaction.Tree, boardPath string) (transaction.MutationKind, error) {
	results, err := tree.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(boardPath)})
	if err != nil {
		return "", fmt.Errorf("change create: probing board path: %w", err)
	}
	if len(results) == 1 && results[0].Found {
		return transaction.MutationReplace, nil
	}
	return transaction.MutationCreate, nil
}

func toChangeIDs(ids []int) []domain.ChangeID {
	if ids == nil {
		return nil
	}
	out := make([]domain.ChangeID, len(ids))
	for i, id := range ids {
		out[i] = domain.ChangeID(id)
	}
	return out
}

func toADRIDs(ids []int) []domain.ADRID {
	if ids == nil {
		return nil
	}
	out := make([]domain.ADRID, len(ids))
	for i, id := range ids {
		out[i] = domain.ADRID(id)
	}
	return out
}

func toChangeIDPtr(id *int) *domain.ChangeID {
	if id == nil {
		return nil
	}
	c := domain.ChangeID(*id)
	return &c
}
