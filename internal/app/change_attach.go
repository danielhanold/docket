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
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// This file is the `change attach-plan` and `change attach-results` operations:
// two exact-revision metadata transitions that link an authored artifact (a
// plan, or a results record) to its change and re-render every affected v1-owned
// derived view (the change record's plan:/results: field, its refreshed updated
// date, its artifact block, and the inline board) as one atomic transaction.
// The two kinds run under different profiles:
//
//   - attach-plan (changeAttachMetadata) carries the plan Markdown itself and
//     WRITES the plan file on the metadata branch in the same transaction, the
//     way change.groom writes a spec: any submitted docket:backlink block is
//     replaced by the freshly rendered one, the body must be non-empty, bounded,
//     parse cleanly, and carry no placeholder-only slot (change 0414), and the
//     path must sit inside the planning root. Inside the transaction a field that
//     already names a different path refuses, and a file already at the path
//     that does not point home to THIS change is never overwritten. A re-plan at
//     the linked path replaces the content.
//   - attach-results (changeAttach) verifies an artifact already committed on
//     the feature branch from Git, never from the child agent's return: the
//     writer commit must be the current feature head, the results file a regular
//     tracked file inside the configured results root carrying a balanced
//     backlink that targets THIS change and passing checkpoint-phase content
//     validation. Only after every check passes does the transaction open.
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
// frontmatter field, the allowed planning root, and the verification profile
// (the plan is written on the metadata branch; results are verified at the
// feature head).
const (
	attachKindPlan    = "plan"
	attachKindResults = "results"
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
	// ReasonAttachCommitNotHead: the verified commit is not the current feature
	// head, so it does not describe the checkout the change owns (results).
	ReasonAttachCommitNotHead = "commit-not-head"
	// ReasonAttachUntrackedFile: the artifact path is not a tracked file at the
	// verified commit (results).
	ReasonAttachUntrackedFile = "untracked-file"
	// ReasonAttachSymlinkedPlan: the artifact path is a symlink at the commit, not
	// a regular file (results).
	ReasonAttachSymlinkedPlan = "symlinked-plan"
	// ReasonAttachUnbalancedBacklink: the artifact's managed backlink markers are
	// malformed (dangling/out-of-order/nested) — the body will not parse. For a
	// plan it is the submitted Markdown that refuses, before anything is read.
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
	// ReasonAttachMissingBacklink: the artifact carries no docket:backlink block.
	ReasonAttachMissingBacklink = "missing-backlink"
	// ReasonAttachBacklinkMismatch: the artifact's backlink targets a different
	// change than the one being attached.
	ReasonAttachBacklinkMismatch = "backlink-mismatch"
	// ReasonAttachPlaceholderToken: a plan slot — a section body or the
	// pre-heading preamble — contains only a bare placeholder token (an
	// unfilled slot; change 0414). A token mentioned inside substantive
	// content never refuses.
	ReasonAttachPlaceholderToken = "placeholder-token"
	// ReasonAttachArtifactUnreadable: the verified commit or its blob could not be
	// read for a reason other than plain absence.
	ReasonAttachArtifactUnreadable = "artifact-unreadable"
	// ReasonAttachResultsContent: a results artifact fails checkpoint-phase content
	// validation (raw template scaffolding, a missing title, or a malformed
	// document). The FINAL content contract binds at mark-implemented, not here.
	ReasonAttachResultsContent = "results-content-invalid"
)

// ChangeAttachRequest is the closed request for one attach. ID and Revision pin
// the change record (exact submitted blob); Path is the canonical repo-relative
// artifact path; Commit is the exact feature commit the writer reported
// (results only); Markdown is the authored plan body (plan only).
type ChangeAttachRequest struct {
	ID       int    `json:"id" docket:"required"`
	Revision string `json:"revision" docket:"required"`
	Path     string `json:"path" docket:"required"`
	Commit   string `json:"commit"` // results only
	// Markdown is the authored artifact body, read from --markdown at the CLI
	// boundary (a non-JSON file input, ADR-0138), never a JSON key.
	Markdown []byte `json:"-"`
}

// attachDigestPayload is the idempotency digest payload: the promised state a
// retry must match — the change id, the artifact path, and the artifact's
// identity: for a plan, the sha256 of the stored artifact bytes; for results,
// the exact blob object id at the verified commit. Keying on the content (not
// merely the path) means a retry after the content changed is NOT a replay.
type attachDigestPayload struct {
	Blob string `json:"blob"`
	ID   int    `json:"id"`
	Path string `json:"path"`
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

// ChangeAttachResults verifies an authored results record (required at
// completion since change 0410) from Git and links it to the change. It applies
// the canonical-path, containment, tracked-file, backlink, exact-head, and
// revision rules — a results document is never gate evidence, so it carries no
// single-artifact/trailer/descent proof; checkpoint attaches validate
// checkpoint-phase content.
func ChangeAttachResults(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir string, req ChangeAttachRequest) ChangeAttachResult {
	return changeAttach(ctx, deps, wdeps, repoDir, req, attachKindResults)
}

// changeAttachMetadata is the metadata-profile driver: it validates the request
// and the authored body, pins context, assembles the stored artifact (the
// freshly rendered backlink prepended to the body), then drives one atomic,
// idempotency-keyed exact-revision transaction that writes the artifact file and
// links it. Every refusal before the transaction writes nothing; the
// state-dependent refusals (a field naming another path, an occupied path) are
// decided inside the transaction against the attempt's own fresh tree.
func changeAttachMetadata(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeAttachRequest, kind string) ChangeAttachResult {
	opKey := attachOpKey(kind)

	// (1) Request shape: the pinned-entity scalars, then the authored body —
	// non-empty, bounded, parsing cleanly (balanced managed markers), and free
	// of placeholder-only slots. All of it is decided before anything is read.
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
	backlink, err := render.BacklinkContent(ac.change, ac.link)
	if err != nil {
		return attachRefusal(opKey, ResultInternalError, kind, ReasonStatusInternalError, err.Error())
	}
	artifact, err := metadataArtifactBytes(req.Markdown, backlink)
	if err != nil {
		// Unreachable after checkAttachMarkdown parsed the same body; kept so a
		// future reorder can never store a malformed artifact.
		return attachRefusal(opKey, ResultInvalidInput, kind, ReasonAttachUnbalancedBacklink,
			fmt.Sprintf("the submitted Markdown has a malformed managed-block population: %v", err))
	}

	// (7) Open the exact-revision, idempotency-keyed transaction. The promised
	// state is (id, path, stored bytes), so a lost-response retry of the same
	// body replays and a different body is a new request.
	sum := sha256.Sum256(artifact)
	contentID := hex.EncodeToString(sum[:])
	digest, derr := canonicalDigest(opKey, attachDigestPayload{Blob: contentID, ID: req.ID, Path: req.Path})
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
		Remote:     originRemote,
		TargetRef:  gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName),
		Expected: []transaction.EntityExpectation{{
			Path:     gitcli.RepoPath(ac.recPath),
			Revision: transaction.ExpectedRevision{Kind: transaction.RevisionBlob, ObjectID: gitcli.ObjectID(req.Revision)},
		}},
		Idempotency: &transaction.IdempotencyKey{
			RequestID: fmt.Sprintf("attach-%s-%d-%s", kind, req.ID, contentID[:16]),
			Digest:    digest,
		},
		Loader:    newPlanningLoader(eff),
		Scope:     changeScope(req.ID, ac.recPath, false),
		Operation: op,
	})
	return attachResultFromOutcome(opKey, kind, req.Path, res, execErr)
}

// checkAttachMarkdown applies the authored-body request checks shared by the
// metadata profile: non-empty, within the authored-input bound, a clean parse
// (balanced managed markers), and — for a plan — no slot whose whole body is a
// bare placeholder token (change 0414). Managed blocks are not author content,
// so the slot check reads the submitted body exactly as it reads the stored
// artifact. It returns nil when the body is usable.
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
	if kind == attachKindPlan {
		// A plan that merely MENTIONS a token — an instruction, a code example,
		// ambiguous prose — is build-actionable and attaches; completeness
		// judgment stays with plan authoring and review.
		if slot, found := planPlaceholderSlot(markdown); found {
			return refuse(ResultInvalidState, ReasonAttachPlaceholderToken,
				fmt.Sprintf("%s of the plan contains only a placeholder token; fill the slot with real content", slot))
		}
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

// changeAttach is the feature-head driver attach-results still runs. It
// validates the request shape, pins context, verifies the committed artifact
// from Git, then drives one atomic, idempotency-keyed exact-revision
// transaction.
func changeAttach(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir string, req ChangeAttachRequest, kind string) ChangeAttachResult {
	opKey := attachOpKey(kind)

	// (1) Request shape.
	findings := validateLifecycleShape("id", req.ID, req.Path, req.Revision)
	if strings.TrimSpace(req.Commit) == "" {
		findings = append(findings, lifecycleFinding(FCEmptyCommit, "commit must be the exact feature commit the writer reported"))
	}
	if len(findings) > 0 {
		return newAttachResult(opKey, ResultInvalidInput, ChangeAttachResult{Kind: kind, Findings: findings})
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

	// (5) The commit is the current feature head (workspace inspection).
	insp := WorkspaceInspect(ctx, deps, wdeps, repoDir, WorkspaceIDRequest{ID: req.ID})
	if insp.Result != ResultApplied {
		return attachRefusal(opKey, insp.Result, kind, insp.Reason, insp.Message)
	}
	if req.Commit != insp.Head {
		return attachRefusal(opKey, ResultInvalidState, kind, ReasonAttachCommitNotHead,
			"the verified commit is not the current feature head; re-read the workspace head before attaching")
	}

	// (6) Read the artifact blob AT THE VERIFIED COMMIT — tracked, regular, no
	// symlink — and keep its exact bytes and object id (never the working tree).
	blob, r := readAttachBlob(ctx, deps, repo, req.Commit, req.Path, opKey, kind)
	if r != nil {
		return *r
	}

	// (7) The artifact carries a balanced backlink targeting THIS change.
	if r := verifyBacklink(opKey, kind, blob.Blob.Bytes, ac); r != nil {
		return *r
	}

	// (8) Checkpoint-phase content sanity: a truthful in-progress artifact
	// passes; raw template scaffolding, a missing title, or a malformed document
	// refuses. The FINAL content contract binds at mark-implemented, not here —
	// the phase is explicit so checkpoint attachment can never claim it.
	if fs := ValidateResultsContent(blob.Blob.Bytes, ResultsPhaseCheckpoint); len(fs) > 0 {
		return attachRefusal(opKey, ResultInvalidState, kind, ReasonAttachResultsContent,
			fmt.Sprintf("the results artifact fails checkpoint content validation: %s: %s", fs[0].Reason, fs[0].Message))
	}

	// (9) Every verification passed: open the exact-revision, idempotency-keyed
	// transaction that stores the artifact path and re-renders the derived views.
	digest, derr := canonicalDigest(opKey, attachDigestPayload{Blob: string(blob.Blob.ObjectID), ID: req.ID, Path: req.Path})
	if derr != nil {
		return attachRefusal(opKey, ResultInternalError, kind, ReasonStatusInternalError, derr.Error())
	}

	op := changeAttachOp{
		opKey:      opKey,
		kind:       kind,
		changeID:   req.ID,
		artifact:   req.Path,
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
			Path:     gitcli.RepoPath(ac.recPath),
			Revision: transaction.ExpectedRevision{Kind: transaction.RevisionBlob, ObjectID: gitcli.ObjectID(req.Revision)},
		}},
		Idempotency: &transaction.IdempotencyKey{RequestID: attachRequestID(kind, req), Digest: digest},
		Loader:      newPlanningLoader(eff),
		Scope:       changeScope(req.ID, ac.recPath, false),
		Operation:   op,
	})

	return attachResultFromOutcome(opKey, kind, req.Path, res, execErr)
}

// attachOpKey maps an artifact kind to its operation key.
func attachOpKey(kind string) string {
	if kind == attachKindResults {
		return OperationChangeAttachResults
	}
	return OperationChangeAttachPlan
}

// attachRequestID derives the idempotency request id from the kind and the
// (id, commit) pair, so a lost-response retry of the same request replays.
func attachRequestID(kind string, req ChangeAttachRequest) string {
	return fmt.Sprintf("attach-%s-%d-%s", kind, req.ID, req.Commit)
}

// attachChange bundles the resolved change with its current record path and the
// link context its backlink renders under.
type attachChange struct {
	change  domain.Change
	recPath string
	link    render.LinkContext
}

// resolveAttachChange pins-adjacent: it reads the corpus once, builds the
// snapshot, and returns the change named by id (a typed unknown/ambiguous refusal
// otherwise) with its current path and backlink link context.
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
		link:    linkContextOf(pin),
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

// readAttachBlob opens the verified commit and reads the artifact blob: absent is
// untracked-file, a symlink (mode 120000) is symlinked-plan, anything else
// unreadable. On success it returns the BlobResult carrying exact bytes and oid.
func readAttachBlob(ctx context.Context, deps PlanningDeps, repo gitcli.Repository, commit, artifactPath, opKey, kind string) (gitcli.BlobResult, *ChangeAttachResult) {
	src, err := deps.Client.OpenObjectSource(ctx, repo, gitcli.Revision{Commit: gitcli.ObjectID(commit)})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		r := attachRefusal(opKey, result, kind, reason, err.Error())
		return gitcli.BlobResult{}, &r
	}
	results, err := src.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(artifactPath)})
	if err != nil {
		// A directory/gitlink requested as a blob, or an unreadable object store.
		r := attachRefusal(opKey, ResultInvalidState, kind, ReasonAttachArtifactUnreadable, err.Error())
		return gitcli.BlobResult{}, &r
	}
	if len(results) != 1 || !results[0].Found {
		r := attachRefusal(opKey, ResultInvalidState, kind, ReasonAttachUntrackedFile,
			fmt.Sprintf("no tracked file at %q in the verified commit", artifactPath))
		return gitcli.BlobResult{}, &r
	}
	if results[0].Blob.Mode == "120000" {
		r := attachRefusal(opKey, ResultInvalidState, kind, ReasonAttachSymlinkedPlan,
			fmt.Sprintf("the artifact at %q is a symlink at the verified commit, not a regular file", artifactPath))
		return gitcli.BlobResult{}, &r
	}
	return results[0], nil
}

// backlinkTargets reports whether artifactBytes carries a balanced docket:backlink
// managed block whose interior equals the backlink rendered for ch under link. It
// is the shared backlink-identity check both change.attach-results (verifyBacklink)
// and change.mark-implemented (verifyImplementedResults) apply to an artifact's own
// bytes. A malformed managed-block population or a backlink-render failure is
// returned as the error (the caller classifies it); an artifact with no backlink
// block, or one whose interior names a DIFFERENT change, is (false, nil).
func backlinkTargets(artifactBytes []byte, ch domain.Change, link render.LinkContext) (bool, error) {
	doc, err := document.Parse(artifactBytes)
	if err != nil {
		return false, err
	}
	block, ok := doc.Block(backlinkBlockName)
	if !ok {
		return false, nil
	}
	expected, err := render.BacklinkContent(ch, link)
	if err != nil {
		return false, err
	}
	got := strings.TrimRight(string(artifactBytes[block.Interior.Start:block.Interior.End]), "\n")
	return got == backlinkInterior(expected), nil
}

// verifyBacklink proves the artifact carries a balanced docket:backlink block
// whose interior targets THIS change. A malformed managed-block population fails
// the parse (unbalanced-backlink); an absent block is missing-backlink; a block
// naming a different change is backlink-mismatch. It classifies the parse and
// presence cases here — each is its own stable attach reason — before delegating
// the interior comparison to backlinkTargets (which re-parses; the redundant parse
// is cheap and keeps the shared helper self-contained for the mark-implemented
// caller that wants a single yes/no).
func verifyBacklink(opKey, kind string, artifactBytes []byte, ac attachChange) *ChangeAttachResult {
	doc, err := document.Parse(artifactBytes)
	if err != nil {
		r := attachRefusal(opKey, ResultInvalidState, kind, ReasonAttachUnbalancedBacklink,
			fmt.Sprintf("the artifact has a malformed managed-block population: %v", err))
		return &r
	}
	if _, ok := doc.Block(backlinkBlockName); !ok {
		r := attachRefusal(opKey, ResultInvalidState, kind, ReasonAttachMissingBacklink,
			"the artifact carries no docket:backlink block")
		return &r
	}
	ok, err := backlinkTargets(artifactBytes, ac.change, ac.link)
	if err != nil {
		// The parse above already succeeded, so this is a backlink-render failure.
		r := attachRefusal(opKey, ResultInternalError, kind, ReasonStatusInternalError, err.Error())
		return &r
	}
	if !ok {
		r := attachRefusal(opKey, ResultInvalidState, kind, ReasonAttachBacklinkMismatch,
			"the artifact's backlink targets a different change than the one being attached")
		return &r
	}
	return nil
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
	// artifactBytes is the stored artifact the metadata profile writes at
	// artifact in the same commit (backlink + authored body); nil for the
	// feature-head profile, which links a file already committed elsewhere.
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
// when inline is enabled, and — for the metadata profile — the artifact file
// itself.
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

	// The metadata profile writes the artifact file, so it decides on this
	// attempt's own tree: a field already naming another path never re-points,
	// and a file already at the path is overwritten only when the link already
	// names it or its backlink points home to THIS change (a prior attach of
	// this change). Anything else belongs to someone else and refuses.
	var artifactFile *transaction.FileMutation
	if o.artifactBytes != nil {
		linked := attachFieldValue(c, o.kind)
		if linked != "" && linked != o.artifact {
			return refuseLifecycle(FindingCode(ReasonAttachPathMismatch), pathMismatchMessage(o.kind, linked, o.artifact))
		}
		existing, _, exists, err := treeBlob(ctx, st.Tree, o.artifact)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change attach: %w", err)
		}
		if exists && linked != o.artifact {
			if home, berr := backlinkTargets(existing, c, o.link); berr != nil || !home {
				return refuseLifecycle(FindingCode(ReasonAttachPathOccupied),
					fmt.Sprintf("a file already exists at %q on the metadata branch and its backlink does not point to change %04d; refusing to overwrite it", o.artifact, o.changeID))
			}
		}
		switch {
		case !exists:
			artifactFile = &transaction.FileMutation{Path: gitcli.RepoPath(o.artifact), Kind: transaction.MutationCreate, Bytes: o.artifactBytes}
		case !bytes.Equal(existing, o.artifactBytes):
			artifactFile = &transaction.FileMutation{Path: gitcli.RepoPath(o.artifact), Kind: transaction.MutationReplace, Bytes: o.artifactBytes}
		}
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
