package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
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
// two exact-version metadata transitions that link a verified authored artifact
// (a plan, or an optional results record) to its change and re-render every
// affected v1-owned derived view (the change record's plan:/results: field, its
// refreshed updated date, its artifact block, and the inline board) as one
// atomic transaction.
//
// The load-bearing property is pre-transaction Git VERIFICATION, done from Git
// and never from the child agent's return: the writer commit must be the current
// feature head, the plan must be a regular tracked file inside the allowed
// planning directory carrying a balanced backlink that targets THIS change, and
// (for a plan) the commit must descend from the prepared base, carry the
// ADR-0094 single-artifact delta (exactly the plan file, rename detection off —
// learning diff-derived-allowlist-needs-no-renames), carry the plan-path commit
// trailer, and hold no unresolved planning placeholder token. Only after every
// check passes does the exact-version transaction open.
//
// Idempotency is keyed on the PROMISED state — the (id, path, blob-at-commit)
// triple — so a lost-response retry replays the original applied receipt rather
// than attaching whatever now occupies the path (learning idempotency-keying).

// The operation keys the two attach transitions record in their envelopes.
const (
	OperationChangeAttachPlan    = "change.attach-plan"
	OperationChangeAttachResults = "change.attach-results"
)

// The two artifact kinds an attach operation links. Kind selects the owned
// frontmatter field, the allowed planning root, and (plan only) the required
// commit trailer and single-artifact/placeholder verification.
const (
	attachKindPlan    = "plan"
	attachKindResults = "results"
)

// plansPlanningRoot is the allowed repository-relative directory a plan artifact
// lives under. It is the superpowers plan-writer convention (agents/
// docket-plan-writer.md), not a configured field; the results root IS configured
// (config.Effective.ResultsDir) and is read from the resolved workflow config.
const plansPlanningRoot = "docs/superpowers/plans"

// The commit trailer the plan-writer stamps (ADR-0094). Results carry their own
// path trailer. Both are verified verbatim off the writer commit, never trusted
// from the child return.
const (
	planPathTrailerKey    = "Docket-Plan-Path"
	resultsPathTrailerKey = "Docket-Results-Path"
)

// Stable machine reasons the attach operations report for their typed refusals.
// Message text is explanatory and must not be parsed. Every refusal predates the
// transaction, so a refused call writes nothing.
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
	// head, so it does not describe the checkout the change owns.
	ReasonAttachCommitNotHead = "commit-not-head"
	// ReasonAttachCommitNotDescendant: the commit does not descend from the
	// prepared base — it was not built on the workspace this change prepared.
	ReasonAttachCommitNotDescendant = "commit-not-descendant"
	// ReasonAttachUntrackedFile: the artifact path is not a tracked file at the
	// verified commit.
	ReasonAttachUntrackedFile = "untracked-file"
	// ReasonAttachSymlinkedPlan: the artifact path is a symlink at the commit, not
	// a regular file.
	ReasonAttachSymlinkedPlan = "symlinked-plan"
	// ReasonAttachMultiArtifactDelta: the writer commit changes more than the one
	// allowed artifact (ADR-0094). A rename with detection off reddens here too.
	ReasonAttachMultiArtifactDelta = "multi-artifact-delta"
	// ReasonAttachMissingTrailer: the writer commit lacks the required path trailer
	// (or its value does not name the artifact).
	ReasonAttachMissingTrailer = "missing-trailer"
	// ReasonAttachUnbalancedBacklink: the artifact's managed backlink markers are
	// malformed (dangling/out-of-order/nested) — the blob will not parse.
	ReasonAttachUnbalancedBacklink = "unbalanced-backlink"
	// ReasonAttachMissingBacklink: the artifact carries no docket:backlink block.
	ReasonAttachMissingBacklink = "missing-backlink"
	// ReasonAttachBacklinkMismatch: the artifact's backlink targets a different
	// change than the one being attached.
	ReasonAttachBacklinkMismatch = "backlink-mismatch"
	// ReasonAttachPlaceholderToken: the plan still carries an unresolved planning
	// placeholder token (the writing-plans "No Placeholders" contract).
	ReasonAttachPlaceholderToken = "placeholder-token"
	// ReasonAttachArtifactUnreadable: the verified commit or its blob could not be
	// read for a reason other than plain absence.
	ReasonAttachArtifactUnreadable = "artifact-unreadable"
)

// placeholderTokenRE matches an unresolved planning placeholder token as a
// whole word. The set mirrors the writing-plans "No Placeholders" contract; a
// plan carrying any is not build-actionable and is refused (placeholder-token).
var placeholderTokenRE = regexp.MustCompile(`\b(TBD|TODO|FIXME|TKTK|XXX|PLACEHOLDER)\b`)

// ChangeAttachRequest is the closed request for one attach. ID and Version pin
// the change record (exact submitted blob); Path is the canonical repo-relative
// artifact path; Commit is the exact feature commit the writer reported.
type ChangeAttachRequest struct {
	ID      int    `json:"id" docket:"required"`
	Version string `json:"version" docket:"required"`
	Path    string `json:"path" docket:"required"`
	Commit  string `json:"commit" docket:"required"`
}

// attachDigestPayload is the idempotency digest payload: the promised state a
// retry must match — the change id, the artifact path, and the exact blob object
// id at the verified commit. Keying on the blob (not merely the path) means a
// retry after the path was overwritten is NOT a replay.
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

// ChangeAttachPlan verifies a written plan from Git and links it to the change.
func ChangeAttachPlan(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir string, req ChangeAttachRequest) ChangeAttachResult {
	return changeAttach(ctx, deps, wdeps, repoDir, req, attachKindPlan)
}

// ChangeAttachResults verifies an optional authored results record from Git and
// links it to the change. It applies the canonical-path, containment,
// tracked-file, backlink, exact-head, and version rules — a results document is
// never gate evidence, so it carries no single-artifact/trailer/descent proof.
func ChangeAttachResults(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir string, req ChangeAttachRequest) ChangeAttachResult {
	return changeAttach(ctx, deps, wdeps, repoDir, req, attachKindResults)
}

// changeAttach is the shared driver. It validates the request shape, pins
// context, verifies the artifact from Git under the kind's verification profile,
// then drives one atomic, idempotency-keyed exact-version transaction.
func changeAttach(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir string, req ChangeAttachRequest, kind string) ChangeAttachResult {
	opKey := attachOpKey(kind)

	// (1) Request shape.
	findings := validateLifecycleShape("id", req.ID, req.Path, req.Version)
	if strings.TrimSpace(req.Commit) == "" {
		findings = append(findings, lifecycleFinding(FCEmptyCommit, "commit must be the exact feature commit the writer reported"))
	}
	if len(findings) > 0 {
		return newAttachResult(opKey, ResultInvalidInput, ChangeAttachResult{Kind: kind, Findings: findings})
	}

	// (2) Pin context, fence the board surface, discover the repository.
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return attachRefusal(opKey, result, kind, reason, err.Error())
	}
	eff := pin.Config.Effective
	inline, err := fenceBoardSurface(eff)
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

	// (6, plan only) The commit descends from the prepared base.
	if kind == attachKindPlan {
		desc, err := deps.Client.IsAncestor(ctx, repo, gitcli.ObjectID(insp.BaseCommit), gitcli.ObjectID(req.Commit))
		if err != nil {
			result, reason := classifyStatusError(ctx, classifyGitFailure(err))
			return attachRefusal(opKey, result, kind, reason, err.Error())
		}
		if !desc {
			return attachRefusal(opKey, ResultInvalidState, kind, ReasonAttachCommitNotDescendant,
				"the commit does not descend from the prepared base; it was not built on this change's workspace")
		}
	}

	// (7) Read the artifact blob AT THE VERIFIED COMMIT — tracked, regular, no
	// symlink — and keep its exact bytes and object id (never the working tree).
	blob, r := readAttachBlob(ctx, deps, repo, req.Commit, req.Path, opKey, kind)
	if r != nil {
		return *r
	}

	// (8, plan only) The writer commit carries the ADR-0094 single-artifact delta.
	if kind == attachKindPlan {
		if r := verifySingleArtifactDelta(ctx, deps, repo, req.Commit, req.Path, opKey); r != nil {
			return *r
		}
		// (9, plan only) The writer commit carries the required path trailer.
		if r := verifyPathTrailer(ctx, deps, repo, req.Commit, req.Path, planPathTrailerKey, opKey); r != nil {
			return *r
		}
	}

	// (10) The artifact carries a balanced backlink targeting THIS change.
	if r := verifyBacklink(opKey, kind, blob.Blob.Bytes, ac); r != nil {
		return *r
	}

	// (11, plan only) The plan carries no unresolved placeholder token.
	if kind == attachKindPlan {
		if placeholderTokenRE.Match(blob.Blob.Bytes) {
			return attachRefusal(opKey, ResultInvalidState, kind, ReasonAttachPlaceholderToken,
				"the plan still carries an unresolved planning placeholder token")
		}
	}

	// (12) Every verification passed: open the exact-version, idempotency-keyed
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
			Path:    gitcli.RepoPath(ac.recPath),
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(req.Version)},
		}},
		Idempotency: &transaction.IdempotencyKey{RequestID: attachRequestID(kind, req), Digest: digest},
		Loader:      newPlanningLoader(eff),
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

// verifySingleArtifactDelta proves the writer commit changes exactly the one
// artifact path against its first parent, rename detection off.
func verifySingleArtifactDelta(ctx context.Context, deps PlanningDeps, repo gitcli.Repository, commit, artifactPath, opKey string) *ChangeAttachResult {
	paths, err := deps.Client.CommitChangedPaths(ctx, repo, gitcli.ObjectID(commit))
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		r := attachRefusal(opKey, result, attachKindPlan, reason, err.Error())
		return &r
	}
	if len(paths) != 1 || string(paths[0]) != artifactPath {
		r := attachRefusal(opKey, ResultInvalidState, attachKindPlan, ReasonAttachMultiArtifactDelta,
			fmt.Sprintf("the writer commit changes %d paths, not exactly the one allowed artifact", len(paths)))
		return &r
	}
	return nil
}

// verifyPathTrailer proves the writer commit carries the required path trailer,
// whose value names the artifact.
func verifyPathTrailer(ctx context.Context, deps PlanningDeps, repo gitcli.Repository, commit, artifactPath, key, opKey string) *ChangeAttachResult {
	scanned, err := deps.Client.ScanCommitTrailers(ctx, repo, gitcli.ObjectID(commit), []string{key})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		r := attachRefusal(opKey, result, attachKindPlan, reason, err.Error())
		return &r
	}
	for _, ct := range scanned {
		if string(ct.Commit) != commit {
			continue
		}
		for _, tr := range ct.Trailers {
			if tr.Key == key && tr.Value == artifactPath {
				return nil
			}
		}
	}
	r := attachRefusal(opKey, ResultInvalidState, attachKindPlan, ReasonAttachMissingTrailer,
		fmt.Sprintf("the writer commit lacks the required %q trailer naming the artifact", key))
	return &r
}

// verifyBacklink proves the artifact carries a balanced docket:backlink block
// whose interior targets THIS change. A malformed managed-block population fails
// the parse (unbalanced-backlink); an absent block is missing-backlink; a block
// naming a different change is backlink-mismatch.
func verifyBacklink(opKey, kind string, artifactBytes []byte, ac attachChange) *ChangeAttachResult {
	doc, err := document.Parse(artifactBytes)
	if err != nil {
		r := attachRefusal(opKey, ResultInvalidState, kind, ReasonAttachUnbalancedBacklink,
			fmt.Sprintf("the artifact has a malformed managed-block population: %v", err))
		return &r
	}
	block, ok := doc.Block(backlinkBlockName)
	if !ok {
		r := attachRefusal(opKey, ResultInvalidState, kind, ReasonAttachMissingBacklink,
			"the artifact carries no docket:backlink block")
		return &r
	}
	expected, err := render.BacklinkContent(ac.change, ac.link)
	if err != nil {
		r := attachRefusal(opKey, ResultInternalError, kind, ReasonStatusInternalError, err.Error())
		return &r
	}
	got := strings.TrimRight(string(artifactBytes[block.Interior.Start:block.Interior.End]), "\n")
	if got != backlinkInterior(expected) {
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
// refusal from the transaction is state-shaped (an exact-version CAS miss or an
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
// field is fixed before the transaction; the state-dependent work (field
// patching, artifact-block and board rendering) re-runs from the attempt's own
// fresh state.
type changeAttachOp struct {
	opKey      string
	kind       string
	changeID   int
	artifact   string
	eff        config.Effective
	clock      transaction.Clock
	inline     bool
	link       render.LinkContext
	changesDir string
}

func (o changeAttachOp) Key() transaction.OperationKey { return transaction.OperationKey(o.opKey) }

// Plan stores the artifact path in the owned frontmatter field (plan or
// results), refreshes the updated date, re-renders the artifact block, and
// assembles the closed plan: the mutated change record and the re-rendered board
// when inline is enabled.
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

	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: finalBytes},
	}
	if o.inline {
		// Attaching an artifact edits no board-visible field, so includeBoard's
		// declare-only-when-changed shape can render byte-identical to the
		// committed board and correctly declare no board mutation.
		boardPath := path.Join(o.changesDir, "BOARD.md")
		if err := includeBoard(ctx, st.Tree, boardPath, candidate, boardPresentation(o.eff), &files); err != nil {
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
