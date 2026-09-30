package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

// This file is the `change groom` planning operation: it grooms a proposed,
// needs-design change to build-ready by one of two authored outcomes — a full
// spec, or a trivial verdict — or, by the third outcome (revise), adjusts an
// already-groomed proposed change in place: a whole-body replace of its existing
// linked spec and/or owned proposal-section edits, never touching spec: or
// trivial:. The abstain outcome records an autonomous groom's abstain on a
// needs-design change — auto_groomable: false plus one dated
// "## Auto-groom blocked" entry — under the same groom gate; the re-enable outcome
// clears it — auto_groomable: true, the section removed — under that gate too,
// optionally with owned-section edits in the same commit. Every outcome
// lands the change's source mutation and every affected v1-owned derived view (the change record's owned proposal sections,
// its typed fields, its artifact block; a new spec file for the spec outcome, a
// replaced one for a spec-body revise; the inline board) as one validated
// atomic transaction. Grooming is a
// non-allocating edit of an existing record, so it pins the submitted record
// revision with an exact-blob entity expectation rather than an idempotency key
// (a spec-body revise also checks the linked spec file's blob id in Plan), and
// it never touches claim metadata. It decides no lifecycle policy beyond the
// groom gate the spec fixes here (proposed, needs-design, not yet trivial) and
// its exact complement, the revise gate (proposed, already spec'd or trivial).

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
	// GroomRevise adjusts an already-groomed proposed change: a whole-body
	// replace of its existing linked spec, owned proposal-section edits, or
	// both. It never writes spec: or trivial:, so a change can never flip
	// between spec'd and trivial through this outcome.
	GroomRevise GroomOutcome = "revise"
	// GroomAbstain records an autonomous groom's abstain on a needs-grooming
	// change: it sets auto_groomable: false and appends one dated entry to the
	// ## Auto-groom blocked section. It never writes spec: or trivial:, and it
	// accepts no section, spec, or relationship edits — an autonomous caller
	// cannot rewrite the proposal through it.
	GroomAbstain GroomOutcome = "abstain"
	// GroomReEnable re-enables a needs-grooming change for autonomous grooming:
	// it sets auto_groomable: true, removes the ## Auto-groom blocked section when
	// present, and applies any owned-section edits (typically the context the
	// abstain asked for) in the same commit. Human-typed or human-attended only.
	GroomReEnable GroomOutcome = "re-enable"
)

// reasonSpecRevisionMismatch is the Plan refusal for a stale spec_revision on a
// spec-body revise; changeGroomResultFromOutcome maps it onto contended.
const reasonSpecRevisionMismatch = "spec-revision-mismatch"

// specsDir is the metadata-tree directory design specs live in. It is a fixed v1
// location (the Bash grooming skills write here); it is not configurable.
const specsDir = "docs/superpowers/specs"

// autoGroomBlockedHeading is the presence-encoded abstain section: the abstain
// outcome appends to it, and the board's "auto-groom blocked — needs you" cell
// keys on its presence (domain.ReadyAutoGroomBlocked).
const autoGroomBlockedHeading = "## Auto-groom blocked"

// ChangeGroomRequest is the closed, caller-supplied request for one groom. Path
// and Revision pin the exact submitted record; the relationship collections are
// the complete desired values (a nil collection is left unchanged, an explicit
// empty collection clears the field). Authored Markdown rides inside the string
// fields and is never interpolated into any shell command.
type ChangeGroomRequest struct {
	ChangeID int          `json:"change_id" docket:"required"`
	Path     string       `json:"path" docket:"required"`     // current canonical record path
	Revision string       `json:"revision" docket:"required"` // exact full blob object id
	Outcome  GroomOutcome `json:"outcome" docket:"required"`

	SpecMarkdown string               `json:"spec_markdown,omitempty"` // required for the spec outcome
	Sections     []SectionEditRequest `json:"sections"`                // proposal-section edits

	// SpecRevision pins the change's existing linked spec file (the path the
	// record's spec: field names) by its exact full blob object id. A revise
	// carrying spec_markdown requires it and no other request accepts it: the
	// whole-body replace overwrites the spec file, so a concurrent spec edit
	// contends instead of being silently clobbered.
	SpecRevision string `json:"spec_revision,omitempty"`

	// BlockedNote is the authored body of one ## Auto-groom blocked entry. The
	// abstain outcome requires it and no other outcome accepts it; the operation
	// owns the heading and the dated "Recorded <date> (UTC)." lead line, so the
	// note must not carry a column-zero "## " heading or an unterminated fence.
	BlockedNote string `json:"blocked_note,omitempty"`

	// Title, when non-empty, retitles the change (change 0461). The spec,
	// trivial, revise, and re-enable outcomes accept it; abstain refuses it, since an
	// abstain cannot rewrite the proposal. Empty leaves the title unchanged. A
	// retitle renames nothing: the slug, record path, spec path, and branch stay put.
	Title string `json:"title,omitempty"`

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
	Outcome  string          `json:"outcome,omitempty"`
	SpecPath string          `json:"spec_path,omitempty"`
	Revision string          `json:"committed_revision,omitempty"`
	Findings []StatusFinding `json:"findings"`
}

// HumanText renders the one-line human summary of a groom outcome.
func (r ChangeGroomResult) HumanText() string {
	switch r.Result {
	case ResultApplied:
		if r.Outcome == string(GroomAbstain) {
			return fmt.Sprintf("change %04d auto-groom abstained — %s", r.ID, r.Revision)
		}
		if r.Outcome == string(GroomReEnable) {
			return fmt.Sprintf("change %04d re-enabled for auto-groom — %s", r.ID, r.Revision)
		}
		if r.Outcome == string(GroomRevise) {
			return fmt.Sprintf("change %04d revised — %s", r.ID, r.Revision)
		}
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
	//    configuration the board-surface check consults.
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newChangeGroomResult(result, ChangeGroomResult{
			Findings: []StatusFinding{{Code: reason, Severity: string(domain.SeverityError), Message: err.Error()}},
		})
	}
	eff := pin.Config.Effective

	// 3. Board-surface check: a github surface is an unsupported configuration,
	//    refused before any transaction; otherwise learn whether inline is on.
	inline, err := resolveBoardSurface(eff)
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

	// The engine pins the record. A spec-body revise's spec_revision is checked
	// in Plan instead, against the blob at the path the record links — the
	// engine checks expectations before the record is read, and Plan runs on
	// the same fetched base, so the check is just as exact.
	res, execErr := deps.Engine.Execute(ctx, transaction.Request{
		Repository: repo,
		Remote:     originRemote,
		TargetRef:  gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName),
		Expected: []transaction.EntityExpectation{{
			Path:     gitcli.RepoPath(req.Path),
			Revision: transaction.ExpectedRevision{Kind: transaction.RevisionBlob, ObjectID: gitcli.ObjectID(req.Revision)},
		}},
		Loader:    newPlanningLoader(eff),
		Operation: op,
	})

	return changeGroomResultFromOutcome(res, execErr)
}

// changeGroomResultFromOutcome folds a transaction outcome into the result
// document. A refusal from this operation is state-shaped (the groom gate, a
// taken spec path, or an evolution refusal), so it maps onto invalid-state —
// except a stale spec_revision, the spec file's analogue of a stale record
// revision, which maps onto contended like the engine's own pin mismatch.
func changeGroomResultFromOutcome(res transaction.Result, execErr error) ChangeGroomResult {
	result, _ := mapOutcome(res, execErr, ResultInvalidState)
	if res.Disposition == transaction.DispositionRefused {
		for _, f := range res.Findings {
			if f.Code == reasonSpecRevisionMismatch {
				result = ResultContended
			}
		}
	}

	out := ChangeGroomResult{Findings: findingsToStatus(res.Findings)}
	if result == ResultApplied {
		if rec, ok := decodeChangeGroomReceipt(res.Receipt); ok {
			out.ID = rec.ID
			out.Outcome = rec.Outcome
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
	addShape := func(code FindingCode, msg string) {
		findings = append(findings, StatusFinding{Code: string(code), Severity: string(domain.SeverityError), Message: msg})
	}

	if req.ChangeID <= 0 {
		addShape(FCInvalidChangeID, "change_id must be a positive change id")
	}
	if strings.TrimSpace(req.Path) == "" {
		addShape(FCEmptyPath, "path must name the change's current canonical record path")
	}
	if strings.TrimSpace(req.Revision) == "" {
		addShape(FCEmptyRevision, "revision must be the exact full blob object id of the submitted record")
	}

	switch req.Outcome {
	case GroomSpec:
		if strings.TrimSpace(req.SpecMarkdown) == "" {
			addShape(FCEmptySpecMarkdown, "spec_markdown must be non-empty for the spec outcome")
		} else if msg := specMarkdownShapeProblem(req.SpecMarkdown); msg != "" {
			addShape(FCInvalidSpecMarkdown, msg)
		}
	case GroomTrivial:
		if !hasAuthoredRationale(req.Sections) {
			addShape(FCMissingRationale, "the trivial outcome requires a non-empty authored rationale among the section edits")
		}
	case GroomRevise:
		if strings.TrimSpace(req.SpecMarkdown) != "" {
			if msg := specMarkdownShapeProblem(req.SpecMarkdown); msg != "" {
				addShape(FCInvalidSpecMarkdown, msg)
			}
		} else if !hasEffectiveSectionEdit(req.Sections) && req.Title == "" {
			addShape(FCEmptyRevise, "the revise outcome requires a non-empty spec_markdown, at least one replace/remove section edit, or a title")
		}
	case GroomAbstain:
		if strings.TrimSpace(req.BlockedNote) == "" {
			addShape(FCEmptyBlockedNote, "blocked_note must be non-empty for the abstain outcome")
		} else if err := render.ValidateSectionBody([]byte(req.BlockedNote)); err != nil {
			addShape(FCInvalidBlockedNote, blockedNoteBodyDiagnostic(err))
		}
		if strings.TrimSpace(req.SpecMarkdown) != "" {
			addShape(FCInvalidSpecMarkdown, "spec_markdown is not accepted by the abstain outcome")
		}
		if len(req.Sections) > 0 {
			addShape(FCInvalidSections, "sections are not accepted by the abstain outcome; an abstain cannot rewrite the proposal")
		}
		if req.Title != "" {
			addShape(FCInvalidTitle, "title is not accepted by the abstain outcome; an abstain cannot rewrite the proposal")
		}
		for _, rel := range []struct {
			name string
			set  bool
			code FindingCode
		}{
			{"depends_on", req.DependsOn != nil, FCInvalidDependsOn},
			{"related", req.Related != nil, FCInvalidRelated},
			{"discovered_from", req.DiscoveredFrom != nil, FCInvalidDiscoveredFrom},
			{"adrs", req.ADRs != nil, FCInvalidADRs},
			{"stacked_on", req.StackedOn != nil, FCInvalidStackedOn},
		} {
			if rel.set {
				addShape(rel.code, rel.name+" is not accepted by the abstain outcome")
			}
		}
	case GroomReEnable:
		if strings.TrimSpace(req.SpecMarkdown) != "" {
			addShape(FCInvalidSpecMarkdown, "spec_markdown is not accepted by the re-enable outcome")
		}
		for _, s := range req.Sections {
			if s.Heading == autoGroomBlockedHeading {
				addShape(FCInvalidSectionHeading, "the re-enable outcome removes \"## Auto-groom blocked\" itself; a section edit may not name it")
			}
		}
	default:
		addShape(FCInvalidOutcome, fmt.Sprintf("outcome %q must be one of spec, trivial, revise, abstain, re-enable", req.Outcome))
	}

	// A title retitles the change on every other outcome (change 0461); an empty
	// one means "unchanged". Any non-empty title must pass the shared rule, so a
	// whitespace-only one is empty-title, never a silent no-op.
	if req.Title != "" && req.Outcome != GroomAbstain {
		if code, msg := validateTitle(req.Title); code != "" {
			addShape(code, msg)
		}
	}

	// blocked_note is the abstain entry's body; nothing else reads it, so it is
	// refused anywhere else rather than silently ignored.
	if req.Outcome != GroomAbstain && req.BlockedNote != "" {
		addShape(FCInvalidBlockedNote, "blocked_note applies only to the abstain outcome")
	}
	boundAuthored(&findings, "blocked_note", req.BlockedNote)

	// A spec-body revise overwrites the linked spec file, so it must pin that
	// file's revision exactly like the record. Nothing else checks spec_revision,
	// so it is refused anywhere else rather than silently ignored.
	specRevise := req.Outcome == GroomRevise && strings.TrimSpace(req.SpecMarkdown) != ""
	hasSpecRevision := strings.TrimSpace(req.SpecRevision) != ""
	switch {
	case specRevise && !hasSpecRevision:
		addShape(FCEmptySpecRevision, "spec_revision must be the exact full blob object id of the linked spec when a revise carries spec_markdown")
	case !specRevise && hasSpecRevision:
		addShape(FCInvalidSpecRevision, "spec_revision applies only to a revise that carries spec_markdown")
	}

	findings = append(findings, validateGroomSections(req.Sections)...)
	return findings
}

// specMarkdownShapeProblem returns why an authored spec_markdown is unusable, or
// "" when it is usable. It must parse as a Markdown document, and it must be the
// spec body only: assembleSpecFile prepends the rendered docket:backlink block
// itself, so a body that already carries one (a spec file resubmitted as read)
// would commit a duplicate marker pair. It is refused, never silently stripped.
func specMarkdownShapeProblem(markdown string) string {
	doc, err := document.Parse([]byte(markdown))
	if err != nil {
		return "spec_markdown must parse as a Markdown document: " + err.Error()
	}
	if _, ok := doc.Block(backlinkBlockName); ok {
		return "spec_markdown must be the spec body without its docket:backlink block; the operation renders that block itself"
	}
	return ""
}

// blockedNoteBodyDiagnostic maps a section-body validation error onto an
// actionable, static diagnostic that never echoes the authored note.
func blockedNoteBodyDiagnostic(err error) string {
	if errors.Is(err, render.ErrSectionBodyUnterminatedFence) {
		return "blocked_note leaves a code fence unterminated; close the fence so the sections after \"## Auto-groom blocked\" stay visible"
	}
	return "blocked_note carries a column-zero \"## \" heading outside fenced code; the operation owns the \"## Auto-groom blocked\" heading — author body text, lists, or \"###\"-or-deeper subsections, and put heading examples inside closed code fences"
}

// autoGroomBlockedMarkdown is the ## Auto-groom blocked body after one more
// abstain: the prior entries (when the section exists) followed by one new
// entry — "Recorded <date> (UTC).", a blank line, then the note — so earlier
// entries are preserved, never replaced.
func autoGroomBlockedMarkdown(oldBody string, present bool, date, note string) string {
	entry := "Recorded " + date + " (UTC).\n\n" + strings.TrimRight(note, "\r\n")
	if present {
		if trimmed := strings.Trim(oldBody, "\r\n"); trimmed != "" {
			return trimmed + "\n\n" + entry
		}
	}
	return entry
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

// hasEffectiveSectionEdit reports whether the section edits carry at least one
// replace or remove — the revise outcome's minimum effective input. A
// preserve-only or empty list changes nothing and is refused as an empty
// revise; relationship-field patches alone do not qualify.
func hasEffectiveSectionEdit(sections []SectionEditRequest) bool {
	for _, s := range sections {
		switch render.SectionIntent(s.Intent) {
		case render.SectionReplace, render.SectionRemove:
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
				Code: string(FCInvalidSectionHeading), Severity: string(domain.SeverityError),
				Message: fmt.Sprintf("section heading %q is not an owned change heading", s.Heading),
			})
			continue
		}
		switch render.SectionIntent(s.Intent) {
		case render.SectionPreserve, render.SectionRemove:
			if s.Markdown != "" {
				findings = append(findings, StatusFinding{
					Code: string(FCInvalidSectionMarkdown), Severity: string(domain.SeverityError),
					Message: fmt.Sprintf("intent %q for %q must carry empty markdown", s.Intent, s.Heading),
				})
			}
		case render.SectionReplace:
			// Markdown may be non-empty.
		default:
			findings = append(findings, StatusFinding{
				Code: string(FCInvalidSectionIntent), Severity: string(domain.SeverityError),
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
// (spec outcome), the replaced existing spec file (a spec-body revise), or the
// linked spec's re-stamped backlink (a retitle), and the re-rendered board when
// inline is enabled.
func (o changeGroomOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot

	c, out := snap.Change(domain.ChangeID(o.req.ChangeID))
	if out != domain.LookupFound {
		return refuseGroom("not-found", fmt.Sprintf("change %04d is not present in the current corpus", o.req.ChangeID))
	}
	// Groom/revise gate. A proposed change is either needs-design (groomable)
	// or already-groomed (revisable) — the two gates are exact complements, so
	// no proposed change satisfies both and none satisfies neither. Neither
	// gate inspects or sets claim metadata.
	if o.req.Outcome == GroomRevise {
		if c.Status() != domain.StatusProposed || (c.Spec().Value == "" && !c.Trivial()) {
			return refuseGroom("not-revisable",
				fmt.Sprintf("change %04d is not an already-groomed proposed change (status %q, spec %q, trivial %v)",
					o.req.ChangeID, c.Status(), c.Spec().Value, c.Trivial()))
		}
	} else if c.Status() != domain.StatusProposed || c.Spec().Value != "" || c.Trivial() {
		return refuseGroom("not-groomable",
			fmt.Sprintf("change %04d is not a proposed, needs-design change (status %q, spec %q, trivial %v)",
				o.req.ChangeID, c.Status(), c.Spec().Value, c.Trivial()))
	}

	// Retitle gate (change 0461). A proposed change revived from a deferred
	// in-progress claim keeps its feature-branch artifacts; their backlinks embed
	// the title and are identity-checked (attach's backlink-mismatch,
	// mark-implemented's results-identity-broken), so renaming it here would
	// silently break the later build. A title equal to the current one is no
	// retitle and passes.
	if o.req.Title != "" && o.req.Title != c.Title() &&
		(c.Branch().Value != "" || c.Plan().Value != "" || c.Results().Value != "") {
		return refuseGroom(string(FCNotRetitleable),
			fmt.Sprintf("change %04d carries feature-branch artifacts (branch %q, plan %q, results %q) whose backlinks embed the title; it cannot be retitled",
				o.req.ChangeID, c.Branch().Value, c.Plan().Value, c.Results().Value))
	}

	// Revise spec-body decision, resolved before any mutation is assembled: a
	// non-empty SpecMarkdown replaces the change's EXISTING linked spec — never
	// a new path — and requires both the link and the file to exist.
	reviseSpec := o.req.Outcome == GroomRevise && strings.TrimSpace(o.req.SpecMarkdown) != ""
	var existingSpec []byte
	if reviseSpec {
		if c.Spec().Value == "" {
			return refuseGroom("spec-not-linked",
				fmt.Sprintf("change %04d has no linked spec to revise (spec_markdown was submitted against a trivial-only change)", o.req.ChangeID))
		}
		blob, blobID, exists, err := treeBlob(ctx, st.Tree, c.Spec().Value)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, err
		}
		existingSpec = blob
		if !exists {
			return refuseGroom("spec-file-missing",
				fmt.Sprintf("change %04d links spec %q but no such file exists on the tree", o.req.ChangeID, c.Spec().Value))
		}
		// The whole-body replace overwrites the spec file, so it is pinned like
		// the record: a stale spec_revision refuses (mapped to contended) rather
		// than clobbering a concurrent spec edit. The record's pin cannot catch
		// that race — a same-day spec-only revise leaves the record unchanged.
		if string(blobID) != o.req.SpecRevision {
			return refuseGroom(reasonSpecRevisionMismatch,
				fmt.Sprintf("spec %q moved since the submitted spec_revision; re-read it and retry", c.Spec().Value))
		}
	}

	src, ok := st.State.Sources[o.req.Path]
	if !ok {
		return refuseGroom("path-mismatch",
			fmt.Sprintf("no record source loaded at %q for change %04d", o.req.Path, o.req.ChangeID))
	}

	// Splice the owned proposal sections first, over the exact source bytes.
	edits := toSectionEdits(o.req.Sections)
	if o.req.Outcome == GroomAbstain {
		// Append one dated entry, preserving earlier ones. The body is sliced at
		// its NAMED terminator (the next top-level heading) by the same
		// fence-aware scan the splice uses; a duplicate heading refuses.
		oldBody, present, err := namedSectionBody(src, autoGroomBlockedHeading)
		if err != nil {
			return refuseGroom(string(FCSectionEditFailed), err.Error())
		}
		edits = append(edits, render.SectionEdit{
			Heading: autoGroomBlockedHeading, Intent: render.SectionReplace,
			Markdown: autoGroomBlockedMarkdown(oldBody, present, o.clock.Now().UTC().Format("2006-01-02"), o.req.BlockedNote),
		})
	}
	if o.req.Outcome == GroomReEnable {
		// Every transition out of the abstained state removes the marker whose
		// presence encodes it (the board keys on it). With no marker and the flag
		// already true there is nothing to re-enable.
		blocked := namedSectionPresent(src, autoGroomBlockedHeading)
		if ag := c.AutoGroomable(); !blocked && ag.State == domain.FieldPresent && ag.Value {
			return refuseGroom(string(FCNothingToReEnable),
				fmt.Sprintf("change %04d has no %s section and is already auto_groomable: true; there is nothing to re-enable", o.req.ChangeID, autoGroomBlockedHeading))
		}
		if blocked {
			edits = append(edits, render.SectionEdit{Heading: autoGroomBlockedHeading, Intent: render.SectionRemove})
		}
	}
	edited, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, edits)
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
	if o.req.Outcome == GroomAbstain {
		// upsertField: a record without the key (hand-authored or pre-template)
		// gets it inserted rather than failing on a missing patch target.
		upsertField(&ps, doc1, "auto_groomable", document.Bool(false))
	}
	if o.req.Outcome == GroomReEnable {
		upsertField(&ps, doc1, "auto_groomable", document.Bool(true))
	}
	// upsertField (not bare SetField): the updated: field is inserted when a record
	// lacks it (a Bash-era or hand-authored record), so this op degrades like the
	// ADR ops, which upsert the same field, rather than internal-erroring with a
	// KindMissingPatchTarget.
	upsertField(&ps, doc1, "updated", document.String(o.clock.Now().UTC().Format("2006-01-02")))
	if o.req.Title != "" {
		// Retitle (change 0461). The writer quotes the scalar, so ADR-0071 holds by
		// construction. upsertField tolerates a record lacking the key rather than
		// internal-erroring. The slug, record path, and spec path are never
		// touched: the candidate snapshot below carries the new title into the
		// artifact block and the board with no further code.
		upsertField(&ps, doc1, "title", document.String(o.req.Title))
	}
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

	// Declare only paths whose bytes actually change: the engine's delta verifier
	// rejects a declared path that is not an actual change, so a revise that
	// re-renders the record byte-identical (updated: already today, artifacts
	// already rendered, identical section text) must not declare it. An empty
	// plan is the engine's clean no-op path — the same skip includeBoard makes.
	var files []transaction.FileMutation
	if !bytes.Equal(finalBytes, src) {
		files = append(files, transaction.FileMutation{
			Path: gitcli.RepoPath(o.req.Path), Kind: transaction.MutationReplace, Bytes: finalBytes,
		})
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
	if reviseSpec {
		// Whole-body replace at the change's existing linked spec path; the
		// backlink block is re-rendered exactly as the spec outcome writes it.
		backlink, err := render.BacklinkContent(gc, o.link)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change groom: rendering spec backlink: %w", err)
		}
		// An identical spec body is not an actual change; skip the declaration.
		if specBytes := assembleSpecFile(backlink, o.req.SpecMarkdown); !bytes.Equal(specBytes, existingSpec) {
			files = append(files, transaction.FileMutation{
				Path: gitcli.RepoPath(c.Spec().Value), Kind: transaction.MutationReplace, Bytes: specBytes,
			})
		}
	}

	// Title re-stamp (change 0461). The spec outcome and a spec-body revise
	// already render the backlink from gc, so this runs only when the title
	// actually changed, the change links a spec, and nothing else writes that
	// spec in this plan — declaring the spec path at most once.
	if gc.Title() != c.Title() && c.Spec().Value != "" && o.req.Outcome != GroomSpec && !reviseSpec {
		updated, changed, code, msg, err := restampSpecBacklink(ctx, st.Tree, c.Spec().Value, gc, o.link)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, err
		}
		if code != "" {
			return refuseGroom(code, msg)
		}
		if changed {
			files = append(files, transaction.FileMutation{
				Path: gitcli.RepoPath(c.Spec().Value), Kind: transaction.MutationReplace, Bytes: updated,
			})
		}
	}

	if o.inline {
		boardPath := path.Join(o.changesDir, "BOARD.md")
		if err := includeBoard(ctx, st.Tree, boardPath, candidate, boardUnrenderable(st.State, o.changesDir), boardPresentation(o.eff), &files); err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change groom: %w", err)
		}
	}

	receiptSpecPath := ""
	if o.req.Outcome == GroomSpec {
		receiptSpecPath = specPath
	}
	if reviseSpec {
		// A revise that replaced the spec body names the existing linked path; a
		// sections-only revise leaves it empty, like the trivial outcome.
		receiptSpecPath = c.Spec().Value
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

// restampSpecBacklink rewrites only the docket:backlink block of the spec at
// specPath so it names gc's (new) title, over the spec's CURRENT bytes on the
// attempt's base tree (change 0461). It takes no spec_revision: nothing
// caller-authored is written, and a concurrent spec edit moves the base and
// contends the push instead of being clobbered. A missing file refuses
// spec-file-missing; a spec that does not parse (malformed markers) refuses
// spec-backlink-malformed; a spec without the block refuses
// spec-backlink-missing — a block is never silently inserted. changed reports
// whether the bytes differ, so an unchanged spec is never declared.
func restampSpecBacklink(ctx context.Context, tree transaction.Tree, specPath string, gc domain.Change, link render.LinkContext) (updated []byte, changed bool, refuseCode, refuseMsg string, err error) {
	blob, _, exists, err := treeBlob(ctx, tree, specPath)
	if err != nil {
		return nil, false, "", "", err
	}
	if !exists {
		return nil, false, "spec-file-missing",
			fmt.Sprintf("change %04d links spec %q but no such file exists on the tree", int(gc.ID()), specPath), nil
	}
	doc, perr := document.Parse(blob)
	if perr != nil {
		return nil, false, "spec-backlink-malformed",
			fmt.Sprintf("spec %q does not parse, so its docket:backlink block cannot be re-stamped: %v", specPath, perr), nil
	}
	if _, ok := doc.Block(backlinkBlockName); !ok {
		return nil, false, "spec-backlink-missing",
			fmt.Sprintf("spec %q has no docket:backlink block to re-stamp with the new title; revise the spec body (send spec_markdown with the current body plus spec_revision and title together) to rebuild the backlink block in one transaction", specPath), nil
	}
	block, err := render.BacklinkContent(gc, link)
	if err != nil {
		return nil, false, "", "", fmt.Errorf("change groom: rendering spec backlink: %w", err)
	}
	var ps document.PatchSet
	ps.ReplaceBlock(backlinkBlockName, backlinkInterior(block))
	out, aerr := doc.Apply(ps)
	if aerr != nil {
		return nil, false, "spec-backlink-malformed",
			fmt.Sprintf("rewriting the docket:backlink block in %q: %v", specPath, aerr), nil
	}
	return out, !bytes.Equal(out, blob), "", "", nil
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
	_, _, found, err := treeBlob(ctx, tree, path)
	return found, err
}

// treeBlob reads path's blob bytes and object id from the base tree, reporting
// whether it exists.
func treeBlob(ctx context.Context, tree transaction.Tree, path string) ([]byte, gitcli.ObjectID, bool, error) {
	results, err := tree.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(path)})
	if err != nil {
		return nil, "", false, fmt.Errorf("change groom: probing path %q: %w", path, err)
	}
	if len(results) != 1 || !results[0].Found {
		return nil, "", false, nil
	}
	return results[0].Blob.Bytes, results[0].Blob.ObjectID, true, nil
}
