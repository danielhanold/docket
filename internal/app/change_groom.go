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

// This file is the `change groom` planning operation: it grooms a proposed,
// needs-design change to build-ready by one of two authored outcomes — a full
// spec, or a trivial verdict — landing the change's source mutation and every
// affected v1-owned derived view (the change record's owned proposal sections,
// its typed fields, its artifact block; a new spec file for the spec outcome;
// the inline board) as one validated atomic transaction. Grooming is a
// non-allocating edit of an existing record, so it pins the submitted record
// version with an exact-blob entity expectation rather than an idempotency key,
// and it never touches claim metadata. It decides no lifecycle policy beyond the
// groom gate the spec fixes here (proposed, needs-design, not yet trivial).

// OperationChangeGroom is the operation key `change groom` records in its result
// envelope and its transaction trailer.
const OperationChangeGroom = "change.groom"

// GroomOutcome is the closed set of groom dispositions a request may carry.
type GroomOutcome string

const (
	// GroomSpec lands an authored design spec and links it from the change.
	GroomSpec GroomOutcome = "spec"
	// GroomTrivial marks the change trivial with an authored rationale, writing
	// no spec file.
	GroomTrivial GroomOutcome = "trivial"
)

// specsDir is the metadata-tree directory design specs live in. It is a fixed v1
// location (the Bash grooming skills write here); it is not configurable.
const specsDir = "docs/superpowers/specs"

// ChangeGroomRequest is the closed, caller-supplied request for one groom. Path
// and Version pin the exact submitted record; the relationship collections are
// the complete desired values (a nil collection is left unchanged, an explicit
// empty collection clears the field). Authored Markdown rides inside the string
// fields and is never interpolated into any shell command.
type ChangeGroomRequest struct {
	ChangeID int          `json:"change_id"`
	Path     string       `json:"path"`    // current canonical record path
	Version  string       `json:"version"` // exact full blob object id
	Outcome  GroomOutcome `json:"outcome"`

	SpecMarkdown string               `json:"spec_markdown,omitempty"` // required for the spec outcome
	Sections     []SectionEditRequest `json:"sections"`                // proposal-section edits

	DependsOn      []int `json:"depends_on"`
	Related        []int `json:"related"`
	DiscoveredFrom []int `json:"discovered_from"`
	ADRs           []int `json:"adrs"`
	StackedOn      *int  `json:"stacked_on"`
}

// SectionEditRequest is the wire shape of one owned-section edit. It is the
// request-layer analogue of render.SectionEdit; the operation validates it and
// converts it before handing it to render.ApplySectionEdits.
type SectionEditRequest struct {
	Heading  string `json:"heading"`
	Intent   string `json:"intent"` // preserve|replace|remove
	Markdown string `json:"markdown,omitempty"`
}

// ChangeGroomResult is the protocol-v1 document `change groom` returns. It
// embeds the envelope; the identity fields are populated on a successful apply,
// and Findings carries every refusal or validation diagnostic (marshalled as []
// never null).
type ChangeGroomResult struct {
	Envelope
	ID       int             `json:"id,omitempty"`
	SpecPath string          `json:"spec_path,omitempty"`
	Revision string          `json:"committed_revision,omitempty"`
	Findings []StatusFinding `json:"findings"`
}

// HumanText renders the one-line human summary of a groom outcome.
func (r ChangeGroomResult) HumanText() string {
	switch r.Result {
	case ResultApplied:
		if r.SpecPath != "" {
			return fmt.Sprintf("change %04d groomed (spec %s) — %s", r.ID, r.SpecPath, r.Revision)
		}
		return fmt.Sprintf("change %04d groomed (trivial) — %s", r.ID, r.Revision)
	default:
		return fmt.Sprintf("change groom: %s", r.Result)
	}
}

// newChangeGroomResult stamps the envelope and normalizes Findings to an empty
// slice so the array marshals as [] on every path.
func newChangeGroomResult(result Result, r ChangeGroomResult) ChangeGroomResult {
	r.Envelope = NewEnvelope(OperationChangeGroom, result)
	if r.Findings == nil {
		r.Findings = []StatusFinding{}
	}
	return r
}

// changeGroomReceipt is the canonical receipt persisted with a groom commit.
// Field order is alphabetical so json.Marshal emits the canonical, sorted-key
// compact form the engine's receipt validator requires; SpecPath is omitted for
// the trivial outcome.
type changeGroomReceipt struct {
	ID       int    `json:"id"`
	Op       string `json:"op"`
	Outcome  string `json:"outcome"`
	SpecPath string `json:"spec_path,omitempty"`
}

// ChangeGroom validates the request, pins authoritative context, and drives one
// atomic transaction that grooms the change and — when inline is enabled —
// re-renders the board. Every failure that predates the transaction (bad request
// shape, an unparseable spec body, a github board surface) returns without an
// engine call.
func ChangeGroom(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeGroomRequest) ChangeGroomResult {
	// 1. Request-shape validation independent of configuration and repository
	//    state. A failure here never reaches the engine.
	if findings := validateChangeGroomShape(req); len(findings) > 0 {
		return newChangeGroomResult(ResultInvalidInput, ChangeGroomResult{Findings: findings})
	}

	// 2. Pin authoritative context: the metadata mode, branches, and resolved
	//    configuration the board fence consults.
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newChangeGroomResult(result, ChangeGroomResult{
			Findings: []StatusFinding{{Code: reason, Severity: string(domain.SeverityError), Message: err.Error()}},
		})
	}
	eff := pin.Config.Effective

	// 3. Board-surface fence: a github surface is an unsupported configuration,
	//    refused before any transaction; otherwise learn whether inline is on.
	inline, err := fenceBoardSurface(eff)
	if err != nil {
		if pe, ok := asPlanningError(err); ok {
			return newChangeGroomResult(pe.Result, ChangeGroomResult{
				Findings: []StatusFinding{{Code: pe.Reason, Severity: string(domain.SeverityError), Message: pe.Message}},
			})
		}
		return newChangeGroomResult(ResultInternalError, ChangeGroomResult{
			Findings: []StatusFinding{{Code: ReasonStatusInternalError, Severity: string(domain.SeverityError), Message: err.Error()}},
		})
	}

	// 4. Discover the repository identity the transaction writes against.
	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return newChangeGroomResult(result, ChangeGroomResult{
			Findings: []StatusFinding{{Code: reason, Severity: string(domain.SeverityError), Message: err.Error()}},
		})
	}

	op := changeGroomOp{
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
		TargetRef:  gitcli.RefName(branchRefPrefix + metadataBranchOf(pin)),
		Expected: []transaction.EntityExpectation{{
			Path:    gitcli.RepoPath(req.Path),
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(req.Version)},
		}},
		Loader:    newPlanningLoader(eff),
		Operation: op,
	})

	return changeGroomResultFromOutcome(res, execErr)
}

// changeGroomResultFromOutcome folds a transaction outcome into the result
// document. A refusal from this operation is always state-shaped (the groom
// gate, a taken spec path, or an evolution refusal), so the refusal maps onto
// invalid-state.
func changeGroomResultFromOutcome(res transaction.Result, execErr error) ChangeGroomResult {
	result, _ := mapOutcome(res, execErr, ResultInvalidState)

	out := ChangeGroomResult{Findings: findingsToStatus(res.Findings)}
	if result == ResultApplied {
		if rec, ok := decodeChangeGroomReceipt(res.Receipt); ok {
			out.ID = rec.ID
			out.SpecPath = rec.SpecPath
		}
		out.Revision = string(res.AppliedCommit)
	}
	r := newChangeGroomResult(result, out)
	r.Failure = failureStatus(res, execErr)
	return r
}

// validateChangeGroomShape runs the configuration-independent request checks:
// the pinned-entity fields, the outcome, the outcome-specific authored inputs,
// and every section edit's shape.
func validateChangeGroomShape(req ChangeGroomRequest) []StatusFinding {
	var findings []StatusFinding
	add := func(code, msg string) {
		findings = append(findings, StatusFinding{Code: code, Severity: string(domain.SeverityError), Message: msg})
	}

	if req.ChangeID <= 0 {
		add("invalid-change_id", "change_id must be a positive change id")
	}
	if strings.TrimSpace(req.Path) == "" {
		add("empty-path", "path must name the change's current canonical record path")
	}
	if strings.TrimSpace(req.Version) == "" {
		add("empty-version", "version must be the exact full blob object id of the submitted record")
	}

	switch req.Outcome {
	case GroomSpec:
		if strings.TrimSpace(req.SpecMarkdown) == "" {
			add("empty-spec_markdown", "spec_markdown must be non-empty for the spec outcome")
		} else if _, perr := document.Parse([]byte(req.SpecMarkdown)); perr != nil {
			add("invalid-spec_markdown", "spec_markdown must parse as a Markdown document: "+perr.Error())
		}
	case GroomTrivial:
		if !hasAuthoredRationale(req.Sections) {
			add("missing-rationale", "the trivial outcome requires a non-empty authored rationale among the section edits")
		}
	default:
		add("invalid-outcome", fmt.Sprintf("outcome %q must be one of spec, trivial", req.Outcome))
	}

	findings = append(findings, validateGroomSections(req.Sections)...)
	return findings
}

// hasAuthoredRationale reports whether the section edits carry at least one
// replace with a non-empty body — the trivial outcome's required rationale.
func hasAuthoredRationale(sections []SectionEditRequest) bool {
	for _, s := range sections {
		if s.Intent == string(render.SectionReplace) && strings.TrimSpace(s.Markdown) != "" {
			return true
		}
	}
	return false
}

// validateGroomSections checks each section edit against the owned-heading set
// and the intent grammar, and enforces the empty-Markdown rule for non-replace
// intents — the same rules render.ApplySectionEdits enforces, surfaced here as
// request-shape findings so a malformed request never reaches the engine.
func validateGroomSections(sections []SectionEditRequest) []StatusFinding {
	owned := make(map[string]bool, len(render.ChangeOwnedHeadings))
	for _, h := range render.ChangeOwnedHeadings {
		owned[h] = true
	}
	var findings []StatusFinding
	for _, s := range sections {
		if !owned[s.Heading] {
			findings = append(findings, StatusFinding{
				Code: "invalid-section-heading", Severity: string(domain.SeverityError),
				Message: fmt.Sprintf("section heading %q is not an owned change heading", s.Heading),
			})
			continue
		}
		switch render.SectionIntent(s.Intent) {
		case render.SectionPreserve, render.SectionRemove:
			if s.Markdown != "" {
				findings = append(findings, StatusFinding{
					Code: "invalid-section-markdown", Severity: string(domain.SeverityError),
					Message: fmt.Sprintf("intent %q for %q must carry empty markdown", s.Intent, s.Heading),
				})
			}
		case render.SectionReplace:
			// Markdown may be non-empty.
		default:
			findings = append(findings, StatusFinding{
				Code: "invalid-section-intent", Severity: string(domain.SeverityError),
				Message: fmt.Sprintf("section intent %q must be one of preserve, replace, remove", s.Intent),
			})
		}
	}
	return findings
}

// decodeChangeGroomReceipt decodes a persisted receipt into its identity fields.
func decodeChangeGroomReceipt(b []byte) (changeGroomReceipt, bool) {
	if len(b) == 0 {
		return changeGroomReceipt{}, false
	}
	var rec changeGroomReceipt
	if err := json.Unmarshal(b, &rec); err != nil {
		return changeGroomReceipt{}, false
	}
	return rec, true
}

// changeGroomOp is the SemanticOperation the engine drives per attempt. Every
// field is fixed before the transaction; the state-dependent work (the groom
// gate, section splicing, field patching, rendering) re-runs from the attempt's
// own fresh state.
type changeGroomOp struct {
	req        ChangeGroomRequest
	eff        config.Effective
	clock      transaction.Clock
	inline     bool
	link       render.LinkContext
	changesDir string
}

func (o changeGroomOp) Key() transaction.OperationKey { return OperationChangeGroom }

// Plan gates the groom against the attempt's snapshot, splices the owned
// proposal sections, patches the typed fields, re-renders the artifact block,
// and assembles the closed plan: the groomed change record, the new spec file
// (spec outcome), and the re-rendered board when inline is enabled.
func (o changeGroomOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot

	c, out := snap.Change(domain.ChangeID(o.req.ChangeID))
	if out != domain.LookupFound {
		return refuseGroom("not-found", fmt.Sprintf("change %04d is not present in the current corpus", o.req.ChangeID))
	}
	// Groom gate: the change must be proposed, still need design (no spec, not
	// yet trivial). Grooming never inspects or sets claim metadata.
	if c.Status() != domain.StatusProposed || c.Spec().Value != "" || c.Trivial() {
		return refuseGroom("not-groomable",
			fmt.Sprintf("change %04d is not a proposed, needs-design change (status %q, spec %q, trivial %v)",
				o.req.ChangeID, c.Status(), c.Spec().Value, c.Trivial()))
	}

	src, ok := st.State.Sources[o.req.Path]
	if !ok {
		return refuseGroom("path-mismatch",
			fmt.Sprintf("no record source loaded at %q for change %04d", o.req.Path, o.req.ChangeID))
	}

	// Splice the owned proposal sections first, over the exact source bytes.
	edited, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, toSectionEdits(o.req.Sections))
	if err != nil {
		return refuseGroom("section-edit-failed", err.Error())
	}

	// First patch pass: the typed field edits (spec/trivial/updated and the
	// complete-desired relationship collections). The artifact block is filled in
	// a second pass, because rendering it needs the groomed snapshot.
	doc1, err := document.Parse(edited)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change groom: reparsing edited record: %w", err)
	}

	specPath := path.Join(specsDir, fmt.Sprintf("%s-%s-design.md", o.clock.Now().UTC().Format("2006-01-02"), c.Slug()))

	var ps document.PatchSet
	if o.req.Outcome == GroomSpec {
		// A pre-existing spec path would collide with the new file; refuse.
		exists, err := treeHasPath(ctx, st.Tree, specPath)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, err
		}
		if exists {
			return refuseGroom("spec-path-taken", fmt.Sprintf("spec path %q already exists", specPath))
		}
		ps.SetField("spec", document.String(specPath))
	}
	if o.req.Outcome == GroomTrivial {
		ps.SetField("trivial", document.Bool(true))
	}
	// upsertField (not bare SetField): the updated: field is inserted when a record
	// lacks it (a Bash-era or hand-authored record), so this op degrades like the
	// ADR ops, which upsert the same field, rather than internal-erroring with a
	// KindMissingPatchTarget.
	upsertField(&ps, doc1, "updated", document.String(o.clock.Now().UTC().Format("2006-01-02")))
	if o.req.DependsOn != nil {
		ps.SetField("depends_on", intSeqValue(o.req.DependsOn))
	}
	if o.req.Related != nil {
		ps.SetField("related", intSeqValue(o.req.Related))
	}
	if o.req.DiscoveredFrom != nil {
		ps.SetField("discovered_from", intSeqValue(o.req.DiscoveredFrom))
	}
	if o.req.ADRs != nil {
		ps.SetField("adrs", intSeqValue(o.req.ADRs))
	}
	if o.req.StackedOn != nil {
		ps.SetField("stacked_on", document.Int(int64(*o.req.StackedOn)))
	}

	intermediate, err := doc1.Apply(ps)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change groom: patching record fields: %w", err)
	}

	// The candidate snapshot is the before-state with this record replaced by its
	// groomed bytes: it resolves the artifact block's rows and drives the board.
	candidate, err := buildGroomCandidate(o.eff, st.State.Documents, o.req.Path, intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	gc, gout := candidate.Change(domain.ChangeID(o.req.ChangeID))
	if gout != domain.LookupFound {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change groom: groomed record %04d absent from candidate snapshot", o.req.ChangeID)
	}

	body, err := render.ArtifactBlockContent(gc, candidate, o.link)
	if err != nil {
		return refuseGroom("artifact-render-failed", err.Error())
	}
	doc2, err := document.Parse(intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change groom: reparsing patched record: %w", err)
	}
	var ps2 document.PatchSet
	// ReplaceBlock (not upsert) assumes the docket:artifacts block is present —
	// render.ChangeRecord always emits it for canonical v1 records, and the ADR ops
	// make the same assumption on the same corpus. A record without the block is
	// out of scope here (there is no v1 producer of one).
	ps2.ReplaceBlock("artifacts", body)
	finalBytes, err := doc2.Apply(ps2)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change groom: writing artifact block: %w", err)
	}

	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(o.req.Path), Kind: transaction.MutationReplace, Bytes: finalBytes},
	}

	if o.req.Outcome == GroomSpec {
		backlink, err := render.BacklinkContent(gc, o.link)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change groom: rendering spec backlink: %w", err)
		}
		specBytes := assembleSpecFile(backlink, o.req.SpecMarkdown)
		files = append(files, transaction.FileMutation{
			Path: gitcli.RepoPath(specPath), Kind: transaction.MutationCreate, Bytes: specBytes,
		})
	}

	if o.inline {
		boardBytes, err := render.Board(render.BoardInput{Snapshot: candidate})
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change groom: rendering board: %w", err)
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

	receiptSpecPath := ""
	if o.req.Outcome == GroomSpec {
		receiptSpecPath = specPath
	}
	receipt, err := json.Marshal(changeGroomReceipt{
		ID: o.req.ChangeID, Op: OperationChangeGroom, Outcome: string(o.req.Outcome), SpecPath: receiptSpecPath,
	})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change groom: encoding receipt: %w", err)
	}

	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d groomed (%s)", o.req.ChangeID, o.req.Outcome),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}

// refuseGroom builds a refusing OperationResult carrying one state-shaped
// finding — a helper for the Plan closure's domain refusals.
func refuseGroom(code, msg string) (transaction.MutationPlan, transaction.OperationResult, error) {
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

// toSectionEdits converts the request-layer section edits into render.SectionEdit
// values. The request shape was validated before the transaction, so the intent
// strings are known-good here.
func toSectionEdits(reqs []SectionEditRequest) []render.SectionEdit {
	out := make([]render.SectionEdit, len(reqs))
	for i, r := range reqs {
		out[i] = render.SectionEdit{
			Heading:  r.Heading,
			Intent:   render.SectionIntent(r.Intent),
			Markdown: r.Markdown,
		}
	}
	return out
}

// intSeqValue renders an int slice as a flow sequence of integers; an empty
// slice renders "[]", never null (mirrors render's changeIDSeq).
func intSeqValue(ids []int) document.Value {
	items := make([]document.Value, len(ids))
	for i, id := range ids {
		items[i] = document.Int(int64(id))
	}
	return document.Seq(items...)
}

// assembleSpecFile lays out a new spec file: the backlink block, one blank line,
// then the authored markdown, terminated by exactly one trailing newline.
func assembleSpecFile(backlink, markdown string) []byte {
	var b strings.Builder
	b.WriteString(strings.TrimRight(backlink, "\n"))
	b.WriteString("\n\n")
	b.WriteString(strings.TrimRight(markdown, "\n"))
	b.WriteString("\n")
	return []byte(b.String())
}

// buildGroomCandidate rebuilds the complete snapshot the attempt would see after
// the groomed record lands: every existing corpus document reclassified by its
// path, with the groomed change's document swapped in at changePath. The
// reclassification mirrors the planning loader so the candidate is the state the
// engine's after-load will validate.
func buildGroomCandidate(eff config.Effective, docs map[string]document.Document, changePath string, changeBytes []byte) (domain.Snapshot, error) {
	paths := make([]string, 0, len(docs))
	for p := range docs {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	newDoc, err := document.Parse(changeBytes)
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("change groom: parsing groomed record: %w", err)
	}

	inputs := make([]repository.InputDocument, 0, len(docs))
	for _, p := range paths {
		kind, loc, ok := classifyCorpusPath(eff, p)
		if !ok {
			continue
		}
		doc := docs[p]
		if p == changePath {
			doc = newDoc
		}
		inputs = append(inputs, repository.InputDocument{
			Kind: kind, Location: loc, Path: p, Document: doc,
		})
	}

	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("change groom: building candidate snapshot: %w", err)
	}
	return build.Snapshot, nil
}

// treeHasPath reports whether path exists as a blob on the base tree.
func treeHasPath(ctx context.Context, tree transaction.Tree, path string) (bool, error) {
	results, err := tree.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(path)})
	if err != nil {
		return false, fmt.Errorf("change groom: probing path %q: %w", path, err)
	}
	return len(results) == 1 && results[0].Found, nil
}
