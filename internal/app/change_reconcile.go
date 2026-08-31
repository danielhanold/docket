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
)

// This file is the `change reconcile` planning operation: it applies Claude's
// authored reconciliation of an in-progress change against current reality as
// one validated atomic transaction. Reconciliation is a STRUCTURED edit, never
// a whole-record replacement — it patches the owned proposal sections, appends
// one dated ## Reconcile log entry, updates the complete desired relationship
// values, optionally patches the still-mutable linked spec's sections, flips
// reconciled: true, and refreshes the claim (re-stamps claimed_at). Everything
// it does not name — unknown frontmatter, authored sections outside the patch,
// line endings, unrelated files — stays byte-identical.
//
// It is a non-allocating edit of an existing record, so it pins the submitted
// record version with an exact-blob entity expectation rather than an
// idempotency key. A record whose fresh status is no longer in-progress is an
// incompatible fresh state: the operation refuses and writes nothing, and the
// result maps that refusal onto `contended` — it never text-merges two authored
// decisions (a stale version is the engine's own CAS contention).

// OperationChangeReconcile is the operation key `change reconcile` records in
// its result envelope and its transaction trailer.
const OperationChangeReconcile = "change.reconcile"

// The closed set of reconcile dispositions a result may carry.
const (
	ReconcileDispositionApplied   = "applied"
	ReconcileDispositionContended = "contended"
	// ReconcileDispositionFailed: a transaction failure; the cause is in the
	// envelope's failure field.
	ReconcileDispositionFailed = "failed"
)

// reasonReconcileNotInProgress is the stable plan-refusal reason for a change
// whose fresh status is no longer in-progress. The result mapping folds it onto
// `contended`: an incompatible fresh state is never overwritten.
const reasonReconcileNotInProgress = "not-in-progress"

// maxAuthoredMarkdownBytes bounds each authored-Markdown input a reconcile
// request carries (each proposal section body, each spec section body, and the
// reconcile-log entry). Authored Markdown is size-bounded (Global Constraints);
// this is the first authored-input bound in the planning layer, so it is
// defined here rather than reused. It is generous for prose while refusing a
// request that could bloat a metadata record without bound.
const maxAuthoredMarkdownBytes = 65536

// reconcileLogHeading is the exact H2 heading of the appended reconcile log.
const reconcileLogHeading = "## Reconcile log"

// DesiredRelations carries the complete desired relationship values a reconcile
// may set. A nil DesiredRelations pointer leaves every relationship untouched;
// when present, each slice is the complete desired value (nil leaves that one
// field unchanged, an explicit empty slice clears it), mirroring the groom
// operation's relationship semantics. StackedOn is set only when non-nil.
type DesiredRelations struct {
	DependsOn      []int `json:"depends_on"`
	StackedOn      *int  `json:"stacked_on"`
	Related        []int `json:"related"`
	ADRs           []int `json:"adrs"`
	DiscoveredFrom []int `json:"discovered_from"`
}

// ChangeReconcileRequest is the closed, caller-supplied request for one
// reconcile. ID and Version pin the exact submitted record; Sections replaces
// owned proposal sections by canonical heading with full replacement text;
// SpecSections replaces still-mutable linked-spec sections the same way;
// Relations carries the complete desired relationship values; ReconcileLogEntry
// is the required authored, dated-entry body. Authored Markdown rides inside the
// string fields and is never interpolated into any shell command.
type ChangeReconcileRequest struct {
	ID                int               `json:"id"`
	Version           string            `json:"version"`
	Sections          map[string]string `json:"sections"`
	SpecSections      map[string]string `json:"spec_sections"`
	Relations         *DesiredRelations `json:"relations"`
	ReconcileLogEntry string            `json:"reconcile_log_entry"`
}

// ChangeReconcileResult is the protocol-v1 document `change reconcile` returns.
// It embeds the envelope; the identity fields are populated on a successful
// apply, Disposition carries the closed reconcile disposition, and Findings
// carries every refusal or validation diagnostic (marshalled as [] never null).
type ChangeReconcileResult struct {
	Envelope
	ID          int             `json:"id,omitempty"`
	Revision    string          `json:"committed_revision,omitempty"`
	Disposition string          `json:"disposition,omitempty"`
	Findings    []StatusFinding `json:"findings"`
}

// HumanText renders the one-line human summary of a reconcile outcome. It names
// identity and disposition only — no authored document body (redaction).
func (r ChangeReconcileResult) HumanText() string {
	switch r.Result {
	case ResultApplied:
		return fmt.Sprintf("change %04d reconciled — %s", r.ID, r.Revision)
	default:
		disp := r.Disposition
		if disp == "" {
			disp = string(r.Result)
		}
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, disp)
	}
}

// newChangeReconcileResult stamps the envelope and normalizes Findings to an
// empty slice so the array marshals as [] on every path.
func newChangeReconcileResult(result Result, r ChangeReconcileResult) ChangeReconcileResult {
	r.Envelope = NewEnvelope(OperationChangeReconcile, result)
	if r.Findings == nil {
		r.Findings = []StatusFinding{}
	}
	return r
}

// changeReconcileReceipt is the canonical receipt persisted with a reconcile
// commit. Field order is alphabetical so json.Marshal emits the canonical,
// sorted-key compact form the engine's receipt validator requires.
type changeReconcileReceipt struct {
	ID int    `json:"id"`
	Op string `json:"op"`
}

// ChangeReconcile validates the request, pins authoritative context, resolves
// the record's canonical path, and drives one atomic exact-version transaction
// that reconciles the change and — when inline is enabled — re-renders the
// board. Every failure that predates the transaction (bad request shape, a
// fenced board surface, a corpus-read failure) returns without an engine call.
func ChangeReconcile(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeReconcileRequest) ChangeReconcileResult {
	if findings := validateChangeReconcileShape(req); len(findings) > 0 {
		return newChangeReconcileResult(ResultInvalidInput, ChangeReconcileResult{Findings: findings})
	}

	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newChangeReconcileResult(result, ChangeReconcileResult{Findings: []StatusFinding{lifecycleFinding(reason, err.Error())}})
	}
	eff := pin.Config.Effective

	inline, err := fenceBoardSurface(eff)
	if err != nil {
		if pe, ok := asPlanningError(err); ok {
			return newChangeReconcileResult(pe.Result, ChangeReconcileResult{Findings: []StatusFinding{lifecycleFinding(pe.Reason, pe.Message)}})
		}
		return newChangeReconcileResult(ResultInternalError, ChangeReconcileResult{Findings: []StatusFinding{lifecycleFinding(ReasonStatusInternalError, err.Error())}})
	}

	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return newChangeReconcileResult(result, ChangeReconcileResult{Findings: []StatusFinding{lifecycleFinding(reason, err.Error())}})
	}

	// Resolve the record's current canonical path from one corpus pre-read; the
	// request carries only (id, version). Reconcile consults no branch facts — it
	// re-proves nothing about readiness — so the resolved facts are discarded.
	// This pre-read is a supporting observation; the authoritative record state is
	// re-read fresh inside the transaction.
	recPath, terr := resolveReconcileTarget(ctx, deps, pin, eff, req.ID)
	if terr != nil {
		return *terr
	}

	op := changeReconcileOp{
		opKey:      OperationChangeReconcile,
		req:        req,
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
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(req.Version)},
		}},
		Loader:    newPlanningLoader(eff),
		Operation: op,
	})

	return changeReconcileResultFromOutcome(res, execErr)
}

// resolveReconcileTarget reads the metadata corpus once and resolves the target
// change's current canonical record path. An id that names no single record is
// refused here, before any engine call, with a typed unknown-change or
// ambiguous-change reason. Reconcile consults no branch facts — it re-proves
// nothing about readiness. This pre-read is a supporting observation; the
// authoritative record state is re-read fresh inside the transaction.
func resolveReconcileTarget(ctx context.Context, deps PlanningDeps, pin StatusPin, eff config.Effective, id int) (string, *ChangeReconcileResult) {
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := newChangeReconcileResult(result, ChangeReconcileResult{Findings: []StatusFinding{lifecycleFinding(reason, err.Error())}})
		return "", &r
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		r := newChangeReconcileResult(ResultInternalError, ChangeReconcileResult{Findings: []StatusFinding{lifecycleFinding(ReasonStatusInternalError, err.Error())}})
		return "", &r
	}
	snap := build.Snapshot

	c, out := snap.Change(domain.ChangeID(id))
	if out != domain.LookupFound {
		reason, result := "unknown-change", ResultInvalidInput
		msg := fmt.Sprintf("no change %04d is present in the corpus", id)
		if out == domain.LookupAmbiguous {
			reason, result = "ambiguous-change", ResultInvalidState
			msg = fmt.Sprintf("more than one record claims change id %04d; refusing to choose", id)
		}
		r := newChangeReconcileResult(result, ChangeReconcileResult{
			Disposition: reason,
			Findings:    []StatusFinding{lifecycleFinding(reason, msg)},
		})
		return "", &r
	}
	return c.Path(), nil
}

// changeReconcileResultFromOutcome folds a transaction outcome into the result
// document. A plan refusal carrying the incompatible-fresh-state reason is
// remapped from a state refusal onto `contended` — the operation never
// overwrites a record whose status moved out from under the authored request;
// every other refusal is state-shaped.
func changeReconcileResultFromOutcome(res transaction.Result, execErr error) ChangeReconcileResult {
	if res.Disposition == transaction.DispositionRefused && firstFindingCode(res.Findings) == reasonReconcileNotInProgress {
		return newChangeReconcileResult(ResultContended, ChangeReconcileResult{
			Findings:    findingsToStatus(res.Findings),
			Disposition: ReconcileDispositionContended,
		})
	}

	result, _ := mapOutcome(res, execErr, ResultInvalidState)
	out := ChangeReconcileResult{Findings: findingsToStatus(res.Findings)}
	if res.Disposition == transaction.DispositionFailed {
		out.Disposition = ReconcileDispositionFailed
	}
	switch result {
	case ResultApplied:
		if rec, ok := decodeChangeReconcileReceipt(res.Receipt); ok {
			out.ID = rec.ID
		}
		out.Revision = string(res.AppliedCommit)
		out.Disposition = ReconcileDispositionApplied
	case ResultContended:
		out.Disposition = ReconcileDispositionContended
	}
	r := newChangeReconcileResult(result, out)
	r.Failure = failureStatus(res, execErr)
	return r
}

// decodeChangeReconcileReceipt decodes a persisted reconcile receipt.
func decodeChangeReconcileReceipt(b []byte) (changeReconcileReceipt, bool) {
	if len(b) == 0 {
		return changeReconcileReceipt{}, false
	}
	var rec changeReconcileReceipt
	if err := json.Unmarshal(b, &rec); err != nil {
		return changeReconcileReceipt{}, false
	}
	return rec, true
}

// validateChangeReconcileShape runs the configuration-independent request checks
// that never reach the engine: the pinned-entity fields, the owned-section
// fence over the named proposal sections, the required reconcile-log entry, and
// the authored-input size bound over every authored string.
func validateChangeReconcileShape(req ChangeReconcileRequest) []StatusFinding {
	findings := dropFindingCode(validateLifecycleShape(req.ID, "", req.Version), "empty-path")
	add := func(code, msg string) {
		findings = append(findings, lifecycleFinding(code, msg))
	}

	// Owned-section fence: a named proposal section must be an owned change
	// heading. The managed ## Artifacts block, the ## Reconcile log section, and
	// any unowned heading are refused here — reconcile is a structured edit of
	// owned proposal sections only.
	owned := make(map[string]bool, len(render.ChangeOwnedHeadings))
	for _, h := range render.ChangeOwnedHeadings {
		owned[h] = true
	}
	for heading, body := range req.Sections {
		if !owned[heading] {
			add("invalid-section-heading", fmt.Sprintf("section heading %q is not an owned proposal heading", heading))
		}
		boundAuthored(&findings, "section "+heading, body)
	}

	// Spec sections name still-mutable linked-spec headings; each must be a
	// top-level heading, and each body is bounded.
	for heading, body := range req.SpecSections {
		if !strings.HasPrefix(heading, "## ") {
			add("invalid-spec-section-heading", fmt.Sprintf("spec section heading %q must be a top-level '## ' heading", heading))
		}
		boundAuthored(&findings, "spec section "+heading, body)
	}

	if strings.TrimSpace(req.ReconcileLogEntry) == "" {
		add("empty-reconcile_log_entry", "reconcile_log_entry must be a non-empty authored dated-entry body")
	}
	boundAuthored(&findings, "reconcile_log_entry", req.ReconcileLogEntry)

	return findings
}

// boundAuthored appends an authored-input-too-large finding when body exceeds
// the authored-Markdown byte bound.
func boundAuthored(findings *[]StatusFinding, label, body string) {
	if len(body) > maxAuthoredMarkdownBytes {
		*findings = append(*findings, lifecycleFinding("authored-input-too-large",
			fmt.Sprintf("%s is %d bytes, over the %d-byte authored-input bound", label, len(body), maxAuthoredMarkdownBytes)))
	}
}

// changeReconcileOp is the SemanticOperation the engine drives per attempt.
// Every field is fixed before the transaction; the state-dependent work (the
// in-progress gate, section splicing, the reconcile-log append, field patching,
// the linked-spec patch, rendering) re-runs from the attempt's own fresh state.
type changeReconcileOp struct {
	opKey      string
	req        ChangeReconcileRequest
	eff        config.Effective
	clock      transaction.Clock
	inline     bool
	link       render.LinkContext
	changesDir string
}

func (o changeReconcileOp) Key() transaction.OperationKey { return transaction.OperationKey(o.opKey) }

// Plan gates the reconcile against the attempt's fresh snapshot, splices the
// owned proposal sections, appends the dated reconcile-log entry, patches the
// reconciled/claim/updated fields and the desired relationships, patches the
// linked spec's named sections, re-renders the artifact block, and assembles
// the closed plan: the reconciled record, the patched spec (when requested),
// and the re-rendered board when inline is enabled.
func (o changeReconcileOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot

	c, out := snap.Change(domain.ChangeID(o.req.ID))
	if out != domain.LookupFound {
		return refuseReconcile("not-found", fmt.Sprintf("change %04d is not present in the current corpus", o.req.ID))
	}

	// In-progress gate: a change whose fresh status has moved out from under the
	// authored request is an incompatible fresh state. The refusal reason is
	// remapped to `contended` by the result mapper; it is never overwritten.
	if c.Status() != domain.StatusInProgress {
		return refuseReconcile(reasonReconcileNotInProgress,
			fmt.Sprintf("change %04d is %q, not in-progress; reconcile writes nothing on incompatible fresh state", o.req.ID, c.Status()))
	}

	src, ok := st.State.Sources[c.Path()]
	if !ok {
		return refuseReconcile("path-mismatch",
			fmt.Sprintf("no record source loaded at %q for change %04d", c.Path(), o.req.ID))
	}

	// 1. Splice the owned proposal sections over the exact source bytes.
	edited, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, reconcileSectionEdits(o.req.Sections))
	if err != nil {
		return refuseReconcile("section-edit-failed", err.Error())
	}

	// 2. Append the dated reconcile-log entry, locating the section by a named
	//    terminator (learning section-slice-needs-a-named-terminator).
	edited, err = appendReconcileLog(edited, o.clock.Now().UTC().Format("2006-01-02"), o.req.ReconcileLogEntry)
	if err != nil {
		return refuseReconcile("reconcile-log-append-failed", err.Error())
	}

	// 3. Patch the typed fields: refresh the claim (claimed_at), flip reconciled,
	//    stamp updated, and write the complete desired relationships. domain.
	//    RefreshClaim yields the claimed_at FieldChange (the in-progress gate above
	//    guarantees it succeeds).
	refreshed, fail := domain.RefreshClaim(c, o.clock.Now())
	if fail != nil {
		return refuseLifecyclePolicy(fail)
	}
	doc1, err := document.Parse(edited)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change reconcile: reparsing edited record: %w", err)
	}
	var ps document.PatchSet
	for _, fc := range refreshed.Changed {
		upsertField(&ps, doc1, fc.Field, lifecycleFieldValue(fc.To))
	}
	upsertField(&ps, doc1, "reconciled", document.Bool(true))
	upsertField(&ps, doc1, "updated", document.String(o.clock.Now().UTC().Format("2006-01-02")))
	if rel := o.req.Relations; rel != nil {
		if rel.DependsOn != nil {
			ps.SetField("depends_on", intSeqValue(rel.DependsOn))
		}
		if rel.Related != nil {
			ps.SetField("related", intSeqValue(rel.Related))
		}
		if rel.DiscoveredFrom != nil {
			ps.SetField("discovered_from", intSeqValue(rel.DiscoveredFrom))
		}
		if rel.ADRs != nil {
			ps.SetField("adrs", intSeqValue(rel.ADRs))
		}
		if rel.StackedOn != nil {
			ps.SetField("stacked_on", document.Int(int64(*rel.StackedOn)))
		}
	}
	intermediate, err := doc1.Apply(ps)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change reconcile: patching record fields: %w", err)
	}

	// 4. Re-render the artifact block against the reconciled candidate snapshot.
	candidate, err := buildGroomCandidate(o.eff, st.State.Documents, c.Path(), intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	gc, gout := candidate.Change(domain.ChangeID(o.req.ID))
	if gout != domain.LookupFound {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change reconcile: reconciled record %04d absent from candidate snapshot", o.req.ID)
	}
	body, err := render.ArtifactBlockContent(gc, candidate, o.link)
	if err != nil {
		return refuseReconcile("artifact-render-failed", err.Error())
	}
	doc2, err := document.Parse(intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change reconcile: reparsing patched record: %w", err)
	}
	var ps2 document.PatchSet
	ps2.ReplaceBlock("artifacts", body)
	finalBytes, err := doc2.Apply(ps2)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change reconcile: writing artifact block: %w", err)
	}

	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: finalBytes},
	}

	// 5. Patch the still-mutable linked spec, when requested.
	if len(o.req.SpecSections) > 0 {
		specFile, refusal, ferr := o.planSpecPatch(ctx, st, c)
		if ferr != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, ferr
		}
		if refusal != nil {
			return transaction.MutationPlan{}, *refusal, nil
		}
		files = append(files, specFile)
	}

	// 6. Re-render the inline board when enabled.
	if o.inline {
		// Reconcile edits no board-visible field, so includeBoard's
		// declare-only-when-changed shape can render byte-identical to the
		// committed board and correctly declare no board mutation.
		boardPath := path.Join(o.changesDir, "BOARD.md")
		if err := includeBoard(ctx, st.Tree, boardPath, candidate, boardPresentation(o.eff), &files); err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change reconcile: %w", err)
		}
	}

	receipt, err := json.Marshal(changeReconcileReceipt{ID: o.req.ID, Op: o.opKey})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change reconcile: encoding receipt: %w", err)
	}

	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d reconciled", o.req.ID),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}

// planSpecPatch reads the change's linked spec from the base tree and applies
// the requested section replacements over its exact bytes. It refuses (a
// returned *OperationResult) when the change has no linked spec or the spec is
// absent from the tree; a genuine tree/read fault is returned as a Go error.
func (o changeReconcileOp) planSpecPatch(ctx context.Context, st transaction.AttemptState, c domain.Change) (transaction.FileMutation, *transaction.OperationResult, error) {
	specPath := c.Spec().Value
	if strings.TrimSpace(specPath) == "" {
		refusal := refuseReconcileResult("no-linked-spec",
			fmt.Sprintf("change %04d has no linked spec to patch", o.req.ID))
		return transaction.FileMutation{}, &refusal, nil
	}
	results, err := st.Tree.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(specPath)})
	if err != nil {
		return transaction.FileMutation{}, nil, fmt.Errorf("change reconcile: reading linked spec %q: %w", specPath, err)
	}
	if len(results) != 1 || !results[0].Found {
		refusal := refuseReconcileResult("spec-not-found",
			fmt.Sprintf("linked spec %q is not present in the metadata tree", specPath))
		return transaction.FileMutation{}, &refusal, nil
	}

	specSrc := results[0].Blob.Bytes
	owned := make([]string, 0, len(o.req.SpecSections))
	for heading := range o.req.SpecSections {
		owned = append(owned, heading)
	}
	edited, err := render.ApplySectionEdits(specSrc, owned, reconcileSectionEdits(o.req.SpecSections))
	if err != nil {
		refusal := refuseReconcileResult("spec-section-edit-failed", err.Error())
		return transaction.FileMutation{}, &refusal, nil
	}
	return transaction.FileMutation{
		Path: gitcli.RepoPath(specPath), Kind: transaction.MutationReplace, Bytes: edited,
	}, nil, nil
}

// reconcileSectionEdits converts a heading→body replacement map into replace
// SectionEdits. Every reconcile section edit is a full-text replacement, so the
// intent is always SectionReplace.
func reconcileSectionEdits(sections map[string]string) []render.SectionEdit {
	edits := make([]render.SectionEdit, 0, len(sections))
	for heading, body := range sections {
		edits = append(edits, render.SectionEdit{
			Heading: heading, Intent: render.SectionReplace, Markdown: body,
		})
	}
	return edits
}

// appendReconcileLog appends one dated entry to the record's ## Reconcile log
// section, creating the section at EOF when absent. It preserves prior entries
// by replacing the section body with (old body + the new entry) through the
// loss-preserving render.ApplySectionEdits — which also enforces the section's
// uniqueness (a duplicate ## Reconcile log heading refuses the whole edit). The
// old body is sliced between the heading and its NAMED terminator (the next
// top-level heading, or EOF): a slice keyed on a generic terminator would run
// past the section (learning section-slice-needs-a-named-terminator), so the
// terminator is resolved from the same fence-aware heading scan the splice uses.
func appendReconcileLog(src []byte, date, entryBody string) ([]byte, error) {
	entry := "### " + date + "\n\n" + strings.TrimRight(entryBody, "\r\n")

	oldBody, present, err := reconcileLogBody(src)
	if err != nil {
		return nil, err
	}
	markdown := entry
	if present {
		if trimmed := strings.Trim(oldBody, "\r\n"); trimmed != "" {
			markdown = trimmed + "\n\n" + entry
		}
	}
	return render.ApplySectionEdits(src, []string{reconcileLogHeading},
		[]render.SectionEdit{{Heading: reconcileLogHeading, Intent: render.SectionReplace, Markdown: markdown}})
}

// reconcileLogBody returns the body of the single ## Reconcile log section — the
// bytes between its heading line and the NAMED terminator (the next top-level
// heading, or EOF) — and whether the section is present. It scans headings
// fence-aware, so a heading-shaped line inside a fenced code block is authored
// content, never a boundary. A duplicate heading yields no single body and is
// refused (the whole-record splice will refuse it too, but slicing must not pick
// one silently).
func reconcileLogBody(src []byte) (body string, present bool, err error) {
	heads := scanTopHeadings(src)
	idx := -1
	for i, h := range heads {
		if h.heading == reconcileLogHeading {
			if idx >= 0 {
				return "", false, fmt.Errorf("owned heading %q appears more than once; the reconcile-log slice has no single terminator", reconcileLogHeading)
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
		bodyEnd = heads[idx+1].start // NAMED terminator: the next top-level heading.
	}
	return string(src[bodyStart:bodyEnd]), true, nil
}

// topHeading is one located top-level ("## ") heading: its exact line text, the
// offset where its line begins, and the offset just past its line terminator.
type topHeading struct {
	heading string
	start   int
	lineEnd int
}

// scanTopHeadings returns the top-level "## " headings in src, in source order,
// skipping fenced code blocks so heading-shaped text inside a fence is authored
// content. It mirrors render.scanH2Headings' fence-aware boundary rules (which
// render.ApplySectionEdits applies to the same source), so the body slice this
// yields agrees with the splice that consumes it.
func scanTopHeadings(src []byte) []topHeading {
	var heads []topHeading
	fence := ""
	fenceChar := byte(0)
	start := 0
	for i := 0; i <= len(src); i++ {
		if i < len(src) && src[i] != '\n' {
			continue
		}
		// [start, i) is one physical line without its terminator; lineEnd is one
		// past the terminator (or EOF).
		textEnd := i
		if i > start && i-1 < len(src) && src[i-1] == '\r' {
			textEnd = i - 1
		}
		lineEnd := i + 1
		if i >= len(src) {
			lineEnd = len(src)
		}
		text := src[start:textEnd]
		if run, ok := fenceRunBytes(text); ok {
			switch {
			case fence == "":
				fence, fenceChar = run, run[0]
			case run[0] == fenceChar && len(run) >= len(fence) && bareFence(text, run):
				fence, fenceChar = "", 0
			}
		} else if fence == "" && strings.HasPrefix(string(text), "## ") {
			heads = append(heads, topHeading{heading: string(text), start: start, lineEnd: lineEnd})
		}
		start = i + 1
		if i >= len(src) {
			break
		}
	}
	return heads
}

// fenceRunBytes returns the leading fence delimiter run (three or more backticks
// or tildes, up to three leading spaces) of a code-fence line, and whether the
// line is one.
func fenceRunBytes(text []byte) (string, bool) {
	s := strings.TrimLeft(string(text), " ")
	if len(string(text))-len(s) > 3 {
		return "", false
	}
	if len(s) < 3 {
		return "", false
	}
	ch := s[0]
	if ch != '`' && ch != '~' {
		return "", false
	}
	n := 0
	for n < len(s) && s[n] == ch {
		n++
	}
	if n < 3 {
		return "", false
	}
	return s[:n], true
}

// bareFence reports whether a fence line carries nothing but its delimiter run
// and trailing whitespace — CommonMark's rule for a CLOSING fence.
func bareFence(text []byte, run string) bool {
	rest := strings.TrimLeft(string(text), " ")[len(run):]
	return strings.TrimSpace(rest) == ""
}

// refuseReconcile builds a refusing (plan, OperationResult) pair carrying one
// state-shaped finding.
func refuseReconcile(code, msg string) (transaction.MutationPlan, transaction.OperationResult, error) {
	return transaction.MutationPlan{}, refuseReconcileResult(code, msg), nil
}

// refuseReconcileResult builds a refusing OperationResult carrying one
// state-shaped finding.
func refuseReconcileResult(code, msg string) transaction.OperationResult {
	return transaction.OperationResult{
		Refused: true,
		Findings: []domain.Finding{{
			Code:     code,
			Severity: domain.SeverityError,
			Entity:   domain.EntityRef{Kind: domain.EntityChange},
			Detail:   map[string]string{"message": msg},
		}},
	}
}
