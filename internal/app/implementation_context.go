package app

import (
	"context"
	"fmt"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/repository"
)

// This file is the read-only `context implementation` operation: it assembles
// the one authoritative bundle the implementation workflow reasons from. It
// pins context once, reads the metadata corpus once, and reports a single
// internally consistent snapshot — either a typed no-candidate/absence outcome
// or a bundle carrying the selected change, its spec, readiness and claim
// eligibility, the resolved effective base, the accepted-ADR and enabled-
// learning index views, and the supported workflow configuration. It decides no
// lifecycle policy of its own: selection, readiness, claim eligibility, and base
// resolution are the domain's; this layer only presents their results. It never
// mutates and never opens a transaction, so an unsupported/deferred capability
// surfaces as a warning rather than a refusal — a subsequent mutation runs its
// own preflight.

// OperationContextImplementation is the operation key the context bundle records
// in its result envelope.
const OperationContextImplementation = "context.implementation"

// The stable machine reasons the context operation reports for its typed
// non-bundle outcomes. Message text is explanatory and must not be parsed.
const (
	// ReasonContextNoCandidate is returned when policy selection finds no
	// build-ready change to implement (no --id supplied).
	ReasonContextNoCandidate = "no-candidate"
	// ReasonContextUnknownChange is returned when an explicit --id names no
	// record in the corpus.
	ReasonContextUnknownChange = "unknown-change"
	// ReasonContextAmbiguousID is returned when an explicit --id is claimed by
	// more than one record: the operation refuses to choose.
	ReasonContextAmbiguousID = "ambiguous-change"
	// ReasonContextMalformed is returned when the named change's identity is not
	// usable (a non-positive id or a slug outside the record-slug grammar).
	ReasonContextMalformed = "malformed-record"
	// ReasonContextMissingArtifact is returned when the change links a spec that
	// does not exist at the pinned revision: an absence is reported, never
	// silently omitted.
	ReasonContextMissingArtifact = "missing-artifact"
)

// ImplementationContextRequest is the closed request. ID==0 means "apply the
// landed deterministic selection policy"; a positive ID inspects that exact
// change, which supports an attributed retry.
type ImplementationContextRequest struct {
	ID int `json:"id"`
}

// ContextEntitySummary is the parsed, non-authored semantics of a record, so a
// caller can read a record's identity without decoding its source bytes.
type ContextEntitySummary struct {
	ID       int    `json:"id,omitempty"`
	Slug     string `json:"slug,omitempty"`
	Title    string `json:"title,omitempty"`
	Status   string `json:"status,omitempty"`
	Type     string `json:"type,omitempty"`
	Priority string `json:"priority,omitempty"`
}

// ContextEntity is one document the bundle carries: its canonical repo-relative
// path, exact loss-preserving source bytes (base64 in JSON, as a Go []byte),
// opaque entity version (blob object id), and — for a record — its parsed
// summary. A zero ContextEntity (empty Path) means the bundle carries no such
// document (e.g. a trivial change with no spec).
type ContextEntity struct {
	Path    string                `json:"path,omitempty"`
	Source  []byte                `json:"source,omitempty"`
	Version string                `json:"version,omitempty"`
	Summary *ContextEntitySummary `json:"summary,omitempty"`
}

// ContextChangeSummary is a related change's identity plus how it relates to the
// selected change.
type ContextChangeSummary struct {
	ID       int    `json:"id"`
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Relation string `json:"relation"` // depends-on | stacked-on | related
}

// ContextBase is the resolved effective-base outcome: its tagged kind, the base
// branch when resolved, and the ancestor change the resolution consulted when
// applicable.
type ContextBase struct {
	Kind         string `json:"kind"`
	Branch       string `json:"branch,omitempty"`
	SourceChange int    `json:"source_change,omitempty"`
}

// ContextADREntry is one accepted-ADR index entry.
type ContextADREntry struct {
	ID     int    `json:"id"`
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// ContextLearningEntry is one enabled-learning index entry.
type ContextLearningEntry struct {
	Slug string `json:"slug"`
	Hook string `json:"hook,omitempty"`
}

// ContextHalt reports the durable, human-needed checkpoint markers the selected
// change's record carries: a paused implementation run ("## Run halted", cleared
// by change resume-halted) and a blocked finalize attempt ("## Finalize
// blocked", cleared by finalize clear-block). Both are keyed on heading shape,
// never an interior spelling. The implementation workflow reads these to know a
// run must be resumed rather than re-dispatched.
type ContextHalt struct {
	RunHalted       bool `json:"run_halted"`
	FinalizeBlocked bool `json:"finalize_blocked"`
}

// ContextWorkflow is the supported workflow configuration the bundle reports.
type ContextWorkflow struct {
	RepoMode          string `json:"repo_mode"`
	IntegrationBranch string `json:"integration_branch"`
	TestCommand       string `json:"test_command,omitempty"`
	Remote            string `json:"remote"`
	FeatureBranch     string `json:"feature_branch"`
}

// ImplementationContext is the internally consistent bundle. Every fact derives
// from the one pinned snapshot; a caller never selects from one revision and
// claims against another.
type ImplementationContext struct {
	MetadataRef    string                 `json:"metadata_ref"`
	MetadataCommit string                 `json:"metadata_commit"`
	Change         ContextEntity          `json:"change"`
	Spec           ContextEntity          `json:"spec"`
	Related        []ContextChangeSummary `json:"related"`
	Readiness      string                 `json:"readiness"`
	ClaimEligible  bool                   `json:"claim_eligible"`
	ClaimRefusal   string                 `json:"claim_refusal,omitempty"`
	Halt           ContextHalt            `json:"halt"`
	EffectiveBase  ContextBase            `json:"effective_base"`
	ADRs           []ContextADREntry      `json:"adrs"`
	Learnings      []ContextLearningEntry `json:"learnings"`
	Workflow       ContextWorkflow        `json:"workflow"`
	Warnings       []string               `json:"warnings"`
}

// ImplementationContextResult is the protocol-v1 document the operation returns.
// It follows the read-only result shape: the bundle on a successful read, and a
// stable reason plus explanatory message on a typed absence/refusal.
type ImplementationContextResult struct {
	Envelope
	Context *ImplementationContext `json:"context,omitempty"`
	Reason  string                 `json:"reason,omitempty"`
	Message string                 `json:"message,omitempty"`
}

// HumanText renders the one-line human summary. It names identity, revision,
// and derived verdicts only — never an authored document body, honoring the
// redaction constraint on human-facing text.
func (r ImplementationContextResult) HumanText() string {
	if r.Result == ResultApplied && r.Context != nil {
		c := r.Context
		id, slug := 0, ""
		if c.Change.Summary != nil {
			id, slug = c.Change.Summary.ID, c.Change.Summary.Slug
		}
		return fmt.Sprintf("context: change %04d %s on %s@%s — readiness %s; claim-eligible=%t; base %s",
			id, slug, c.MetadataRef, shortCommit(c.MetadataCommit), c.Readiness, c.ClaimEligible, c.EffectiveBase.Kind)
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// shortCommit renders at most the first 12 characters of a commit id for human
// text; the full id rides in the JSON document.
func shortCommit(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

// newContextResult stamps the envelope and normalizes a bundle's collections.
func newContextResult(result Result, reason, message string, bundle *ImplementationContext) ImplementationContextResult {
	if bundle != nil {
		if bundle.Related == nil {
			bundle.Related = []ContextChangeSummary{}
		}
		if bundle.ADRs == nil {
			bundle.ADRs = []ContextADREntry{}
		}
		if bundle.Learnings == nil {
			bundle.Learnings = []ContextLearningEntry{}
		}
		if bundle.Warnings == nil {
			bundle.Warnings = []string{}
		}
	}
	return ImplementationContextResult{
		Envelope: NewEnvelope(OperationContextImplementation, result),
		Context:  bundle,
		Reason:   reason,
		Message:  message,
	}
}

// ContextImplementation assembles the authoritative implementation-context
// bundle. It is read-only: one pin, one corpus read, one snapshot; every fact
// is threaded from that snapshot.
func ContextImplementation(ctx context.Context, deps PlanningDeps, repoDir string, req ImplementationContextRequest) ImplementationContextResult {
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newContextResult(result, reason, err.Error(), nil)
	}
	eff := pin.Config.Effective

	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newContextResult(result, reason, err.Error(), nil)
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		// A build error means the CALL was malformed — a contract violation.
		return newContextResult(ResultInternalError, ReasonStatusInternalError, err.Error(), nil)
	}
	snap := build.Snapshot

	blobByPath := make(map[string]StatusBlob, len(blobs))
	for _, b := range blobs {
		blobByPath[b.Path] = b
	}

	// The context bundle resolves the effective base from the pinned snapshot
	// alone; remote-branch existence is not re-read here (the authoritative,
	// facts-backed resolution happens inside the claim transaction). A stacked
	// change whose base needs a live remote branch therefore reports an
	// unresolved-base readiness rather than a fabricated resolution.
	facts := domain.NewBranchFacts(nil)

	selected, refusal := selectContextChange(snap, facts, req)
	if refusal != nil {
		return *refusal
	}

	// Spec: read loss-preserving bytes when the change links one. A linked spec
	// that does not exist is a typed missing-artifact, never a silent omission.
	var specEntity ContextEntity
	if spec := selected.Spec(); spec.State == domain.FieldPresent && spec.Value != "" {
		art, err := deps.Reader.ReadArtifact(ctx, pin, sourceMetadata, spec.Value)
		if err != nil {
			result, reason := classifyStatusError(ctx, err)
			return newContextResult(result, reason, err.Error(), nil)
		}
		if !art.Found {
			return newContextResult(ResultInvalidState, ReasonContextMissingArtifact,
				fmt.Sprintf("change %04d links a spec that does not exist at the pinned revision: %s", int(selected.ID()), spec.Value), nil)
		}
		specEntity = ContextEntity{Path: spec.Value, Source: cloneBytes(art.Data), Version: art.Version}
	}

	changeBlob := blobByPath[selected.Path()]
	bundle := &ImplementationContext{
		MetadataRef:    metadataBranchOf(pin),
		MetadataCommit: metadataRevision(pin),
		Change: ContextEntity{
			Path:    selected.Path(),
			Source:  cloneBytes(changeBlob.Data),
			Version: changeBlob.Version,
			Summary: &ContextEntitySummary{
				ID:       int(selected.ID()),
				Slug:     selected.Slug(),
				Title:    selected.Title(),
				Status:   selected.RawStatus(),
				Type:     selected.Type(),
				Priority: selected.RawPriority(),
			},
		},
		Spec:          specEntity,
		Related:       relatedSummaries(snap, selected),
		Readiness:     string(domain.EvaluateReadiness(snap, selected, facts).Kind),
		ClaimEligible: true,
		Halt: ContextHalt{
			RunHalted:       selected.HasRunHalted(),
			FinalizeBlocked: selected.HasFinalizeBlocked(),
		},
		EffectiveBase: contextBase(domain.ResolveEffectiveBase(snap, selected, facts)),
		ADRs:          acceptedADRs(snap),
		Workflow: ContextWorkflow{
			RepoMode:          pin.Mode,
			IntegrationBranch: pin.IntegrationBranch,
			TestCommand:       eff.Finalize.TestCommand.Value,
			Remote:            string(originRemote),
			// The context is produced BEFORE the claim, so no branch is recorded yet;
			// this is the branch the imminent claim will mint and record, through the
			// one branch constructor (never a bare slug-derived name).
			FeatureBranch:     domain.MintBranch(selected.Type(), selected.BranchPrefix(), selected.Slug()),
		},
	}
	bundle.Learnings, bundle.Warnings = learningEntries(snap)

	return newContextResult(ResultApplied, "", "", bundle)
}

// selectContextChange resolves the change the bundle describes: the first
// build-ready candidate under the selection policy, or the exact requested id.
// It returns a non-nil result pointer for every typed non-bundle outcome, and a
// build-ready change with a nil pointer on success.
func selectContextChange(snap domain.Snapshot, facts domain.BranchFacts, req ImplementationContextRequest) (domain.Change, *ImplementationContextResult) {
	if req.ID <= 0 {
		queue := domain.SelectQueue(snap, facts, domain.SelectionFilter{})
		if len(queue) == 0 {
			r := newContextResult(ResultNoOp, ReasonContextNoCandidate,
				"no build-ready change is available to implement", nil)
			return domain.Change{}, &r
		}
		return queue[0], nil
	}

	c, out := snap.Change(domain.ChangeID(req.ID))
	switch out {
	case domain.LookupAbsent:
		r := newContextResult(ResultInvalidInput, ReasonContextUnknownChange,
			fmt.Sprintf("no change %04d is present in the corpus", req.ID), nil)
		return domain.Change{}, &r
	case domain.LookupAmbiguous:
		r := newContextResult(ResultInvalidState, ReasonContextAmbiguousID,
			fmt.Sprintf("more than one record claims change id %04d; refusing to choose", req.ID), nil)
		return domain.Change{}, &r
	}

	// The explicit change must be claimable exactly as the selection policy
	// requires. ClaimEligibility keys on readiness, so a non-build-ready change
	// carries its exact reason token: an unusable identity is malformed, and
	// every other non-ready outcome maps to not-ready-<kind> (including
	// stack-base-unresolved for an unresolved effective base).
	if fail := domain.ClaimEligibility(snap, c, facts); fail != nil {
		reason := fail.Reason
		if reason == "not-ready-"+string(domain.ReadyInvalid) {
			reason = ReasonContextMalformed
		}
		result := ResultInvalidState
		if fail.Kind == domain.FailInvalidInput {
			result = ResultInvalidInput
		}
		r := newContextResult(result, reason, fail.Error(), nil)
		return domain.Change{}, &r
	}
	return c, nil
}

// contextBase renders a resolved effective base as the bundle's tagged view.
func contextBase(base domain.EffectiveBase) ContextBase {
	out := ContextBase{Kind: string(base.Kind), Branch: base.Branch}
	if base.Cause != 0 {
		out.SourceChange = int(base.Cause)
	}
	return out
}

// relatedSummaries assembles the dependency, stack-parent, and related-change
// summaries for the selected change, in a fixed order (deps ascending, then the
// stack parent, then related ascending). Only references the snapshot resolves
// to a single record are summarized.
func relatedSummaries(snap domain.Snapshot, c domain.Change) []ContextChangeSummary {
	out := make([]ContextChangeSummary, 0)
	add := func(id domain.ChangeID, relation string) {
		other, lookup := snap.Change(id)
		if lookup != domain.LookupFound {
			return
		}
		out = append(out, ContextChangeSummary{
			ID:       int(other.ID()),
			Slug:     other.Slug(),
			Title:    other.Title(),
			Status:   other.RawStatus(),
			Relation: relation,
		})
	}
	for _, dep := range c.DependsOn() {
		add(dep, "depends-on")
	}
	if parent, lookup := domain.StackParent(snap, c); lookup == domain.LookupFound {
		add(parent.ID(), "stacked-on")
	}
	for _, rel := range c.Related() {
		add(rel, "related")
	}
	return out
}

// acceptedADRs is the accepted-ADR index view, ascending by ID.
func acceptedADRs(snap domain.Snapshot) []ContextADREntry {
	out := make([]ContextADREntry, 0)
	for _, a := range snap.ADRs() {
		if a.Status().Kind != domain.ADRAccepted {
			continue
		}
		out = append(out, ContextADREntry{
			ID:     int(a.ID()),
			Slug:   a.Slug(),
			Title:  a.Title(),
			Status: a.RawStatus(),
		})
	}
	return out
}

// learningEntries is the enabled-learning index view. With the learnings
// capability disabled it returns no entries and an explicit warning naming the
// capability, so a caller can tell "off" from "none matched".
func learningEntries(snap domain.Snapshot) ([]ContextLearningEntry, []string) {
	catalog := domain.LearningCandidates(snap)
	if catalog.Disabled {
		return []ContextLearningEntry{}, []string{
			"the learnings capability is disabled in policy; no learning index entries are surfaced",
		}
	}
	out := make([]ContextLearningEntry, 0, len(catalog.Findings))
	for _, l := range catalog.Findings {
		out = append(out, ContextLearningEntry{Slug: l.Slug(), Hook: l.Hook()})
	}
	return out, []string{}
}

// cloneBytes copies b so the bundle never aliases a reader's buffer.
func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	return append([]byte(nil), b...)
}
