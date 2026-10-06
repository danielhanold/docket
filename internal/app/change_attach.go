package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// This file is the `change attach-plan` and `change attach-results` operations:
// two exact-revision metadata transitions that carry an authored artifact (a
// plan, or a results record), WRITE it on the metadata branch, and link it to
// its change, re-rendering every affected v1-owned derived view (the change
// record's plan:/results: field, its refreshed updated date, its artifact
// block, and the inline board) in one atomic transaction (changeAttachMetadata).
// The feature branch is never read or written: a results checkpoint is a
// metadata commit, so it never moves the tested feature head.
//
// Both kinds share one profile, the way change.groom writes a spec: any
// submitted docket:backlink block is replaced by the freshly rendered one, the
// body must be non-empty, bounded, and parse cleanly, and the path must sit
// inside the kind's planning root (the plans directory, or the configured
// results root). The content rule is kind-specific: a plan carries no
// placeholder-only slot (change 0414); results pass checkpoint-phase content
// validation (the FINAL contract binds at mark-implemented). Inside the
// transaction a field that already names a different path refuses, and a file
// already at the path that does not point home to THIS change is never
// overwritten. A later attach at the linked path replaces the content.
//
// Idempotency is keyed on the PROMISED state — the (id, path, artifact bytes)
// triple — so a lost-response retry replays the original applied receipt rather
// than attaching whatever now occupies the path (learning idempotency-keying).

// The operation keys the two attach transitions record in their envelopes.
const (
	OperationChangeAttachPlan    = "change.attach-plan"
	OperationChangeAttachResults = "change.attach-results"
)

// The two artifact kinds an attach operation links. Kind selects the owned
// frontmatter field, the allowed planning root, and the content rule.
const (
	attachKindPlan    = "plan"
	attachKindResults = "results"
)

// backlinkBlockName and backlinkBlockAnnotation are the docket:backlink managed
// block's marker identity, shared by every writer of an artifact's or PR body's
// backlink (attach, groom, kill, close-out, PR publish, the spec copy). They
// mirror the marker spelling render.BacklinkContent and
// render.ArtifactBacklinkContent emit exactly, so an inserted block
// round-trips through render on the next write and the write is idempotent.
const (
	backlinkBlockName       = "backlink"
	backlinkBlockAnnotation = "generated — do not hand-edit"
)

// plansPlanningRoot is the allowed repository-relative directory a plan artifact
// lives under. It is the superpowers plan-writer convention (agents/
// docket-plan-writer.md), not a configured field; the results root IS configured
// (config.Effective.ResultsDir) and is read from the resolved workflow config.
const plansPlanningRoot = "docs/superpowers/plans"

// Stable machine reasons the attach operations report for their typed refusals.
// Message text is explanatory and must not be parsed. A refused call writes
// nothing: most refusals predate the transaction, and the in-transaction ones
// (artifact-path-mismatch, artifact-path-occupied) refuse before any commit.
const (
	// ReasonAttachAbsolutePath: the artifact path is absolute; paths crossing the
	// CLI are canonical repository-relative (Global Constraints).
	ReasonAttachAbsolutePath = "absolute-path"
	// ReasonAttachPathEscape: the artifact path escapes the repository root with a
	// `..` traversal.
	ReasonAttachPathEscape = "path-escape"
	// ReasonAttachPathOutsideRoot: the artifact path is well-formed but sits
	// outside the allowed planning directory for its kind.
	ReasonAttachPathOutsideRoot = "path-outside-planning-root"
	// ReasonAttachUnknownChange / -AmbiguousID: the id names no record, or more
	// than one; the operation never chooses.
	ReasonAttachUnknownChange = "unknown-change"
	ReasonAttachAmbiguousID   = "ambiguous-change"
	// ReasonAttachUnbalancedBacklink: the submitted Markdown's managed backlink
	// markers are malformed (dangling/out-of-order/nested) — the body will not
	// parse, so it refuses before anything is read.
	ReasonAttachUnbalancedBacklink = "unbalanced-backlink"
	// ReasonAttachEmptyMarkdown: the submitted artifact Markdown is empty or
	// whitespace-only.
	ReasonAttachEmptyMarkdown = "empty-markdown"
	// ReasonAttachMarkdownTooLarge: the submitted artifact Markdown exceeds the
	// authored-input bound (maxAuthoredMarkdownBytes).
	ReasonAttachMarkdownTooLarge = "authored-input-too-large"
	// ReasonAttachPathMismatch: the change's owned field already names a
	// different artifact path; an attach never silently re-points the link.
	ReasonAttachPathMismatch = "artifact-path-mismatch"
	// ReasonAttachPathOccupied: a file already sits at the artifact path on the
	// metadata branch and its backlink does not point home to this change, so
	// the attach refuses rather than overwrite another change's artifact.
	ReasonAttachPathOccupied = "artifact-path-occupied"
	// ReasonAttachPlaceholderToken: a plan slot — a section body or the
	// pre-heading preamble — contains only a bare placeholder token (an
	// unfilled slot; change 0414). A token mentioned inside substantive
	// content never refuses.
	ReasonAttachPlaceholderToken = "placeholder-token"
	// ReasonAttachResultsContent: a results artifact fails checkpoint-phase content
	// validation (raw template scaffolding, a missing title, or a malformed
	// document). The FINAL content contract binds at mark-implemented, not here.
	ReasonAttachResultsContent = "results-content-invalid"
)

// ChangeAttachRequest is the closed request for one attach. ID and Revision pin
// the change record (exact submitted blob); Path is the canonical repo-relative
// artifact path; Markdown is the authored artifact body the operation writes.
type ChangeAttachRequest struct {
	ID       int    `json:"id" docket:"required"`
	Revision string `json:"revision" docket:"required"`
	Path     string `json:"path" docket:"required"`
	// Markdown is the authored artifact body, read from --markdown at the CLI
	// boundary (a non-JSON file input, ADR-0138), never a JSON key.
	Markdown []byte `json:"-"`
}

// attachDigestPayload is the idempotency digest payload: the promised state a
// retry must match — the change id, the artifact path, Blob (the sha256 of the
// stored artifact bytes), and Revision, the submitted record revision. Keying on
// the content means a retry after the content changed is NOT a replay; keying on
// the revision means re-attaching earlier content at a later record revision is a
// new request rather than a replay of the earlier receipt (the engine scans the
// whole metadata ancestry for a matching request).
type attachDigestPayload struct {
	Blob     string `json:"blob"`
	ID       int    `json:"id"`
	Path     string `json:"path"`
	Revision string `json:"revision"`
}

// ChangeAttachResult is the protocol-v1 document both attach operations return.
// It names identity, the linked artifact kind and path, and the committed
// revision on success; a refusal carries a stable reason and message. It never
// carries the artifact's authored bytes (redaction).
type ChangeAttachResult struct {
	Envelope
	ID       int             `json:"id,omitempty"`
	Kind     string          `json:"kind,omitempty"`
	Path     string          `json:"path,omitempty"`
	Revision string          `json:"committed_revision,omitempty"`
	Reason   string          `json:"reason,omitempty"`
	Message  string          `json:"message,omitempty"`
	Findings []StatusFinding `json:"findings"`
}

// HumanText renders the one-line human summary. It names identity, kind, and the
// artifact path only — never an authored document body.
func (r ChangeAttachResult) HumanText() string {
	switch r.Result {
	case ResultApplied:
		return fmt.Sprintf("change %04d %s attached: %s — %s", r.ID, r.Kind, r.Path, r.Revision)
	default:
		if r.Reason != "" {
			return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
		}
		return fmt.Sprintf("%s: %s", r.Operation, r.Result)
	}
}

// newAttachResult stamps the envelope for opKey and normalizes Findings to [].
func newAttachResult(opKey string, result Result, r ChangeAttachResult) ChangeAttachResult {
	r.Envelope = NewEnvelope(opKey, result)
	if r.Findings == nil {
		r.Findings = []StatusFinding{}
	}
	return r
}

// attachRefusal builds a refusing result carrying a stable reason and message.
func attachRefusal(opKey string, result Result, kind, reason, message string) ChangeAttachResult {
	return newAttachResult(opKey, result, ChangeAttachResult{Kind: kind, Reason: reason, Message: message})
}

// ChangeAttachPlan writes the authored plan on the metadata branch with its
// backlink and links it to the change, in one metadata transaction.
func ChangeAttachPlan(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeAttachRequest) ChangeAttachResult {
	return changeAttachMetadata(ctx, deps, repoDir, req, attachKindPlan)
}

// ChangeAttachResults writes an authored results record (required at completion
// since change 0410) on the metadata branch with its backlink and links it to
// the change, in one metadata transaction. Each checkpoint is checked against
// the checkpoint-phase content contract; the feature head is never consulted.
func ChangeAttachResults(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeAttachRequest) ChangeAttachResult {
	return changeAttachMetadata(ctx, deps, repoDir, req, attachKindResults)
}

// changeAttachMetadata is the attach driver for both kinds: it validates the request
// and the authored body, pins context, assembles the stored artifact (the
// freshly rendered backlink prepended to the body), then drives one atomic,
// idempotency-keyed exact-revision transaction that writes the artifact file and
// links it. Every refusal before the transaction writes nothing; the
// state-dependent refusals (a field naming another path, an occupied path) are
// decided inside the transaction against the attempt's own fresh tree.
func changeAttachMetadata(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeAttachRequest, kind string) ChangeAttachResult {
	opKey := attachOpKey(kind)

	// (1) Request shape: the pinned-entity scalars, then the authored body —
	// non-empty, bounded, parsing cleanly (balanced managed markers), and
	// passing the kind's content rule. All of it is decided before anything is
	// read.
	if findings := validateLifecycleShape("id", req.ID, req.Path, req.Revision); len(findings) > 0 {
		return newAttachResult(opKey, ResultInvalidInput, ChangeAttachResult{Kind: kind, Findings: findings})
	}
	if r := checkAttachMarkdown(opKey, kind, req.Markdown); r != nil {
		return *r
	}

	// (2) Pin context, check the board surface, discover the repository.
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return attachRefusal(opKey, result, kind, reason, err.Error())
	}
	eff := pin.Config.Effective
	inline, err := resolveBoardSurface(eff)
	if err != nil {
		if pe, ok := asPlanningError(err); ok {
			return attachRefusal(opKey, pe.Result, kind, pe.Reason, pe.Message)
		}
		return attachRefusal(opKey, ResultInternalError, kind, ReasonStatusInternalError, err.Error())
	}
	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return attachRefusal(opKey, result, kind, reason, err.Error())
	}

	// (3) Resolve the change record (path, backlink target) from one corpus read.
	ac, refusal := resolveAttachChange(ctx, deps, pin, eff, req.ID, opKey, kind)
	if refusal != nil {
		return *refusal
	}

	// (4) Canonical-path containment inside the kind's allowed planning root.
	if r := verifyAttachPath(opKey, kind, req.Path, eff); r != nil {
		return *r
	}

	// (5) An owned field that already names a different path refuses: the link
	// is never silently re-pointed (the Plan closure re-checks on fresh state).
	if linked := attachFieldValue(ac.change, kind); linked != "" && linked != req.Path {
		return attachRefusal(opKey, ResultInvalidState, kind, ReasonAttachPathMismatch, pathMismatchMessage(kind, linked, req.Path))
	}

	// (6) Assemble the stored artifact: the submitted body with any backlink
	// block it carries replaced by the freshly rendered one for this change.
	backlink := render.ArtifactBacklinkContent(ac.change, req.Path)
	artifact, err := metadataArtifactBytes(req.Markdown, backlink)
	if err != nil {
		// Unreachable after checkAttachMarkdown parsed the same body; kept so a
		// future reorder can never store a malformed artifact.
		return attachRefusal(opKey, ResultInvalidInput, kind, ReasonAttachUnbalancedBacklink,
			fmt.Sprintf("the submitted Markdown has a malformed managed-block population: %v", err))
	}

	// (7) Open the exact-revision, idempotency-keyed transaction. The promised
	// state is (id, path, stored bytes, submitted revision), so a lost-response
	// retry of the same body at the same revision replays, while a different
	// body, or earlier bytes resubmitted at a later revision, is a new request.
	sum := sha256.Sum256(artifact)
	contentID := hex.EncodeToString(sum[:])
	// The request id carries a hash of the submitted revision, never the raw
	// string: the revision is only checked non-empty here (the CAS decides the
	// rest), and the request id must stay within the engine's id charset.
	revSum := sha256.Sum256([]byte(req.Revision))
	revisionID := hex.EncodeToString(revSum[:])
	digest, derr := canonicalDigest(opKey, attachDigestPayload{Blob: contentID, ID: req.ID, Path: req.Path, Revision: req.Revision})
	if derr != nil {
		return attachRefusal(opKey, ResultInternalError, kind, ReasonStatusInternalError, derr.Error())
	}
	op := changeAttachOp{
		opKey:         opKey,
		kind:          kind,
		changeID:      req.ID,
		artifact:      req.Path,
		artifactBytes: artifact,
		eff:           eff,
		clock:         deps.Clock,
		inline:        inline,
		link:          linkContextOf(pin),
		changesDir:    eff.ChangesDir.Value,
	}
	res, execErr := deps.Engine.Execute(ctx, transaction.Request{
		Repository: repo,
		Remote:     metadataRemote(pin.Layout),
		TargetRef:  metadataRef(pin.Layout),
		Expected: []transaction.EntityExpectation{{
			Path:     gitcli.RepoPath(ac.recPath),
			Revision: transaction.ExpectedRevision{Kind: transaction.RevisionBlob, ObjectID: gitcli.ObjectID(req.Revision)},
		}},
		Idempotency: &transaction.IdempotencyKey{
			RequestID: fmt.Sprintf("attach-%s-%d-%s-%s", kind, req.ID, revisionID[:16], contentID[:16]),
			Digest:    digest,
		},
		Loader:    newPlanningLoader(eff),
		Scope:     changeScope(req.ID, ac.recPath, false),
		Operation: op,
	})
	return attachResultFromOutcome(opKey, kind, req.Path, res, execErr)
}

// checkAttachMarkdown applies the authored-body request checks both kinds
// share — non-empty, within the authored-input bound, a clean parse (balanced
// managed markers) — then the kind's content rule: a plan has no slot whose
// whole body is a bare placeholder token (change 0414); results pass
// checkpoint-phase content validation (raw template scaffolding, a missing
// title, or a malformed document refuses; the FINAL content contract binds at
// mark-implemented, so a checkpoint can never claim it). Managed blocks are not
// author content, so both rules read the submitted body exactly as they read
// the stored artifact. It returns nil when the body is usable.
func checkAttachMarkdown(opKey, kind string, markdown []byte) *ChangeAttachResult {
	refuse := func(result Result, reason, msg string) *ChangeAttachResult {
		r := attachRefusal(opKey, result, kind, reason, msg)
		return &r
	}
	if len(bytes.TrimSpace(markdown)) == 0 {
		return refuse(ResultInvalidInput, ReasonAttachEmptyMarkdown,
			fmt.Sprintf("the %s Markdown is empty; send the authored artifact body", kind))
	}
	if len(markdown) > maxAuthoredMarkdownBytes {
		return refuse(ResultInvalidInput, ReasonAttachMarkdownTooLarge,
			fmt.Sprintf("the %s Markdown is %d bytes, over the %d-byte authored-input bound", kind, len(markdown), maxAuthoredMarkdownBytes))
	}
	if _, err := document.Parse(markdown); err != nil {
		return refuse(ResultInvalidInput, ReasonAttachUnbalancedBacklink,
			fmt.Sprintf("the submitted Markdown has a malformed managed-block population: %v", err))
	}
	if kind == attachKindResults {
		if fs := ValidateResultsContent(markdown, ResultsPhaseCheckpoint); len(fs) > 0 {
			return refuse(ResultInvalidState, ReasonAttachResultsContent,
				fmt.Sprintf("the results artifact fails checkpoint content validation: %s: %s", fs[0].Reason, fs[0].Message))
		}
		return nil
	}
	// A plan that merely MENTIONS a token — an instruction, a code example,
	// ambiguous prose — is build-actionable and attaches; completeness
	// judgment stays with plan authoring and review.
	if slot, found := planPlaceholderSlot(markdown); found {
		return refuse(ResultInvalidState, ReasonAttachPlaceholderToken,
			fmt.Sprintf("%s of the plan contains only a placeholder token; fill the slot with real content", slot))
	}
	return nil
}

// metadataArtifactBytes assembles the stored artifact: any submitted backlink
// block removed, the freshly rendered backlink prepended. Leading blank lines of
// the body are dropped, so an artifact resubmitted exactly as read back from the
// metadata branch reassembles byte-identical (a fixed point, never a duplicate
// block or a growing gap). A body whose managed markers are malformed returns
// the parse error untouched.
func metadataArtifactBytes(markdown []byte, backlink string) ([]byte, error) {
	doc, err := document.Parse(markdown)
	if err != nil {
		return nil, err
	}
	body := markdown
	if _, ok := doc.Block(backlinkBlockName); ok {
		var ps document.PatchSet
		ps.RemoveBlock(backlinkBlockName)
		if body, err = doc.Apply(ps); err != nil {
			return nil, err
		}
	}
	return assembleSpecFile(backlink, strings.TrimLeft(string(body), "\r\n")), nil
}

// attachFieldValue returns the change's current value of the kind's owned
// artifact field ("" when unset).
func attachFieldValue(c domain.Change, kind string) string {
	if kind == attachKindResults {
		return strings.TrimSpace(c.Results().Value)
	}
	return strings.TrimSpace(c.Plan().Value)
}

// pathMismatchMessage explains an artifact-path-mismatch refusal.
func pathMismatchMessage(kind, linked, requested string) string {
	return fmt.Sprintf("the change's %s: field already names %q; attach the %s at that path rather than %q", kind, linked, kind, requested)
}

// attachOpKey maps an artifact kind to its operation key.
func attachOpKey(kind string) string {
	if kind == attachKindResults {
		return OperationChangeAttachResults
	}
	return OperationChangeAttachPlan
}

// attachChange bundles the resolved change with its current record path.
type attachChange struct {
	change  domain.Change
	recPath string
}

// resolveAttachChange pins-adjacent: it reads the corpus once, builds the
// snapshot, and returns the change named by id (a typed unknown/ambiguous refusal
// otherwise) with its current path.
func resolveAttachChange(ctx context.Context, deps PlanningDeps, pin StatusPin, eff config.Effective, id int, opKey, kind string) (attachChange, *ChangeAttachResult) {
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := attachRefusal(opKey, result, kind, reason, err.Error())
		return attachChange{}, &r
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		r := attachRefusal(opKey, ResultInternalError, kind, ReasonStatusInternalError, err.Error())
		return attachChange{}, &r
	}
	c, out := build.Snapshot.Change(domain.ChangeID(id))
	if out != domain.LookupFound {
		reason, result := ReasonAttachUnknownChange, ResultInvalidInput
		msg := fmt.Sprintf("no change %04d is present in the corpus", id)
		if out == domain.LookupAmbiguous {
			reason, result = ReasonAttachAmbiguousID, ResultInvalidState
			msg = fmt.Sprintf("more than one record claims change id %04d; refusing to choose", id)
		}
		r := attachRefusal(opKey, result, kind, reason, msg)
		return attachChange{}, &r
	}
	return attachChange{
		change:  c,
		recPath: c.Path(),
	}, nil
}

// verifyAttachPath proves the artifact path is canonical repository-relative and
// inside the kind's allowed planning root.
func verifyAttachPath(opKey, kind, artifactPath string, eff config.Effective) *ChangeAttachResult {
	if strings.TrimSpace(artifactPath) == "" {
		r := attachRefusal(opKey, ResultInvalidInput, kind, ReasonAttachPathEscape, "artifact path is empty")
		return &r
	}
	if filepath.IsAbs(artifactPath) {
		r := attachRefusal(opKey, ResultInvalidInput, kind, ReasonAttachAbsolutePath,
			"artifact path is absolute; paths crossing the CLI are canonical repository-relative")
		return &r
	}
	if !filepath.IsLocal(filepath.FromSlash(artifactPath)) {
		r := attachRefusal(opKey, ResultInvalidInput, kind, ReasonAttachPathEscape,
			"artifact path escapes the repository root")
		return &r
	}
	// Clean is a no-op for a canonical path; an input that changes under Clean is
	// non-canonical (e.g. a `./` or `//` spelling) and is refused as an escape.
	clean := path.Clean(artifactPath)
	if clean != artifactPath {
		r := attachRefusal(opKey, ResultInvalidInput, kind, ReasonAttachPathEscape,
			"artifact path is not in canonical repository-relative form")
		return &r
	}
	root := attachPlanningRoot(kind, eff)
	if !withinPlanningRoot(root, artifactPath) {
		r := attachRefusal(opKey, ResultInvalidInput, kind, ReasonAttachPathOutsideRoot,
			fmt.Sprintf("artifact path is outside the allowed %s planning root %q", kind, root))
		return &r
	}
	return nil
}

// attachPlanningRoot resolves the allowed planning root for a kind: the fixed
// superpowers plans directory, or the configured results root.
func attachPlanningRoot(kind string, eff config.Effective) string {
	if kind == attachKindResults {
		return strings.TrimRight(eff.ResultsDir.Value, "/")
	}
	return plansPlanningRoot
}

// withinPlanningRoot reports whether a canonical repo-relative path lies strictly
// beneath root (a full path segment, never a prefix of a sibling name).
func withinPlanningRoot(root, p string) bool {
	if root == "" {
		return false
	}
	return strings.HasPrefix(p, root+"/")
}

// backlinkTargets reports whether artifactBytes, the bytes of the metadata-branch
// file at artifactPath, carries a balanced docket:backlink managed block whose
// interior equals the backlink rendered for ch from that path. It
// is the shared backlink-identity check the attach transaction (does a file
// already at the path point home to this change?) and change.mark-implemented
// (verifyImplementedResults) apply to an artifact's own bytes. A malformed
// managed-block population is returned as the error (the caller classifies it);
// an artifact with no backlink block, or one whose interior names a DIFFERENT
// change, is (false, nil).
func backlinkTargets(artifactBytes []byte, ch domain.Change, artifactPath string) (bool, error) {
	doc, err := document.Parse(artifactBytes)
	if err != nil {
		return false, err
	}
	block, ok := doc.Block(backlinkBlockName)
	if !ok {
		return false, nil
	}
	expected := render.ArtifactBacklinkContent(ch, artifactPath)
	got := strings.TrimRight(string(artifactBytes[block.Interior.Start:block.Interior.End]), "\n")
	return got == backlinkInterior(expected), nil
}

// changeAttachReceipt is the canonical receipt persisted with an attach commit.
// Field order is alphabetical so json.Marshal emits the sorted-key compact form
// the engine's receipt validator requires.
type changeAttachReceipt struct {
	ID   int    `json:"id"`
	Kind string `json:"kind"`
	Op   string `json:"op"`
	Path string `json:"path"`
}

// attachResultFromOutcome folds a transaction outcome into the attach result. A
// refusal from the transaction is state-shaped (an exact-revision CAS miss or an
// internal-consistency refusal), so it maps onto invalid-state.
func attachResultFromOutcome(opKey, kind, artifactPath string, res transaction.Result, execErr error) ChangeAttachResult {
	result, _ := mapOutcome(res, execErr, ResultInvalidState)
	out := ChangeAttachResult{Kind: kind, Path: artifactPath, Findings: findingsToStatus(res.Findings)}
	if result == ResultApplied {
		if rec, ok := decodeChangeAttachReceipt(res.Receipt); ok {
			out.ID = rec.ID
			out.Kind = rec.Kind
			out.Path = rec.Path
		}
		out.Revision = string(res.AppliedCommit)
	}
	r := newAttachResult(opKey, result, out)
	r.Failure = failureStatus(res, execErr)
	return r
}

// decodeChangeAttachReceipt decodes a persisted attach receipt.
func decodeChangeAttachReceipt(b []byte) (changeAttachReceipt, bool) {
	if len(b) == 0 {
		return changeAttachReceipt{}, false
	}
	var rec changeAttachReceipt
	if err := json.Unmarshal(b, &rec); err != nil {
		return changeAttachReceipt{}, false
	}
	return rec, true
}

// changeAttachOp is the SemanticOperation the engine drives per attempt. Every
// field is fixed before the transaction; the state-dependent work (the path
// checks, field patching, artifact-block and board rendering) re-runs from the
// attempt's own fresh state.
type changeAttachOp struct {
	opKey    string
	kind     string
	changeID int
	artifact string
	// artifactBytes is the stored artifact written at artifact in the same
	// commit (backlink + authored body).
	artifactBytes []byte
	eff           config.Effective
	clock         transaction.Clock
	inline        bool
	link          render.LinkContext
	changesDir    string
}

func (o changeAttachOp) Key() transaction.OperationKey { return transaction.OperationKey(o.opKey) }

// Plan stores the artifact path in the owned frontmatter field (plan or
// results), refreshes the updated date, re-renders the artifact block, and
// assembles the closed plan: the mutated change record, the re-rendered board
// when inline is enabled, and the artifact file itself.
func (o changeAttachOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot

	c, out := snap.Change(domain.ChangeID(o.changeID))
	if out != domain.LookupFound {
		reason := ReasonAttachUnknownChange
		if out == domain.LookupAmbiguous {
			reason = ReasonAttachAmbiguousID
		}
		return refuseLifecycle(FindingCode(reason), fmt.Sprintf("change %04d is not a single record in the current corpus", o.changeID))
	}

	src, ok := st.State.Sources[c.Path()]
	if !ok {
		return refuseLifecycle(FCPathMismatch,
			fmt.Sprintf("no record source loaded at %q for change %04d", c.Path(), o.changeID))
	}

	field := attachKindPlan
	if o.kind == attachKindResults {
		field = attachKindResults
	}

	// The artifact file is written here, so the decision is made on this
	// attempt's own tree: a field already naming another path never re-points,
	// and a file already at the path is overwritten only when the link already
	// names it or its backlink points home to THIS change (a prior attach of
	// this change). Anything else belongs to someone else and refuses.
	linked := attachFieldValue(c, o.kind)
	if linked != "" && linked != o.artifact {
		return refuseLifecycle(FindingCode(ReasonAttachPathMismatch), pathMismatchMessage(o.kind, linked, o.artifact))
	}
	existing, _, exists, err := treeBlob(ctx, st.Tree, o.artifact)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change attach: %w", err)
	}
	if exists && linked != o.artifact {
		if home, berr := backlinkTargets(existing, c, o.artifact); berr != nil || !home {
			return refuseLifecycle(FindingCode(ReasonAttachPathOccupied),
				fmt.Sprintf("a file already exists at %q on the metadata branch and its backlink does not point to change %04d; refusing to overwrite it", o.artifact, o.changeID))
		}
	}
	var artifactFile *transaction.FileMutation
	switch {
	case !exists:
		artifactFile = &transaction.FileMutation{Path: gitcli.RepoPath(o.artifact), Kind: transaction.MutationCreate, Bytes: o.artifactBytes}
	case !bytes.Equal(existing, o.artifactBytes):
		artifactFile = &transaction.FileMutation{Path: gitcli.RepoPath(o.artifact), Kind: transaction.MutationReplace, Bytes: o.artifactBytes}
	}

	// First patch pass: the owned artifact field plus the refreshed updated date.
	intermediate, err := upsertFieldBytes(src, field, document.String(o.artifact))
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change attach: patching %s: %w", field, err)
	}
	intermediate, err = upsertFieldBytes(intermediate, "updated", document.String(o.clock.Now().UTC().Format("2006-01-02")))
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change attach: stamping updated: %w", err)
	}

	// The candidate snapshot resolves the artifact block's rows and drives the board.
	candidate, err := buildGroomCandidate(o.eff, st.State.Documents, c.Path(), intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	gc, gout := candidate.Change(domain.ChangeID(o.changeID))
	if gout != domain.LookupFound {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change attach: mutated record %04d absent from candidate snapshot", o.changeID)
	}

	body, err := render.ArtifactBlockContent(gc, candidate, o.link)
	if err != nil {
		return refuseLifecycle(FCArtifactRenderFailed, err.Error())
	}
	doc2, err := document.Parse(intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change attach: reparsing patched record: %w", err)
	}
	var ps2 document.PatchSet
	ps2.ReplaceBlock("artifacts", body)
	finalBytes, err := doc2.Apply(ps2)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change attach: writing artifact block: %w", err)
	}

	// Declare only paths whose bytes actually change: the engine's delta
	// verifier rejects a declared path that is not an actual change, so a
	// same-path same-day re-attach that re-renders the record byte-identical
	// (field already set, updated: already today, artifact block unchanged)
	// must not declare it. An empty plan is the engine's clean no-op path —
	// the same skip includeBoard makes (change 0458).
	var files []transaction.FileMutation
	if !bytes.Equal(finalBytes, src) {
		files = append(files, transaction.FileMutation{
			Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: finalBytes,
		})
	}
	if artifactFile != nil {
		files = append(files, *artifactFile)
	}
	if o.inline {
		// Attaching an artifact edits no board-visible field, so includeBoard's
		// declare-only-when-changed shape can render byte-identical to the
		// committed board and correctly declare no board mutation.
		boardPath := path.Join(o.changesDir, "BOARD.md")
		if err := includeBoard(ctx, st.Tree, boardPath, candidate, boardUnrenderable(st.State, o.changesDir), boardPresentation(o.eff), &files); err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change attach: %w", err)
		}
	}

	receipt, err := json.Marshal(changeAttachReceipt{ID: o.changeID, Kind: o.kind, Op: o.opKey, Path: o.artifact})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change attach: encoding receipt: %w", err)
	}

	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d attach %s %s", o.changeID, o.kind, o.artifact),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}
