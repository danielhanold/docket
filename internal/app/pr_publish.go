package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
)

// This file is the `pr publish` operation: the thin app-layer wiring that turns a
// published feature head, a reparsed build-evidence record, and authored PR prose
// into exactly one ready-for-review pull request, through the landed
// githubcli.EnsurePullRequest probe/act/verify adapter. The GitHub mechanics
// (the fixed list→decide→create-or-edit→requery sequence, the adoption rules, the
// ExpectedHead gate, the redaction of gh stderr) all stay in internal/githubcli
// and are only WIRED here; the app layer adds no second PR-lookup policy.
//
// Two properties are load-bearing and enforced HERE, before EnsurePullRequest is
// ever called:
//
//   - Identity agreement. The evidence bytes are reparsed (never a prior command
//     result is trusted) and verified against the requested head; the workspace's
//     current local HEAD must equal the requested head; the GitHub repository
//     identity must resolve; and the feature branch and effective-base branch are
//     read from the workspace target the domain resolved. The published remote
//     head is proven equal to the requested head by handing EnsurePullRequest an
//     ExpectedHead of exactly the requested head — the adapter refuses to create
//     or edit whenever GitHub reports any other head, so local == evidence ==
//     requested == published-remote is established transitively. A broken conjunct
//     is a typed refusal and gh is never invoked.
//   - Redaction. The authored PR prose is preserved byte-for-byte while only the
//     Docket-owned backlink and build-evidence blocks are inserted/replaced (via
//     the evidence upsert helper's loss-preserving document patch). The result
//     never carries the PR body bytes — no Body/Title field exists on it.

// OperationPRPublish is the operation key `pr publish` records in its envelope.
const OperationPRPublish = "pr.publish"

// The closed set of stable machine reasons `pr publish` reports for the typed
// refusals it raises before delegating. Message text is explanatory and must not
// be parsed. Every one of these predates the EnsurePullRequest call, so a refused
// call invokes gh for no mutation.
const (
	// ReasonPRHeadInvalid: the requested head is not a full lowercase-hex object
	// id, so it cannot pin a commit; maps to invalid-input.
	ReasonPRHeadInvalid = "head-invalid"
	// ReasonPREvidenceUnverified: the reparsed evidence record does not verify
	// against the requested head (missing, malformed, or a stale head) — the run
	// no longer certifies this commit; maps to invalid-state.
	ReasonPREvidenceUnverified = "evidence-unverified"
	// ReasonPRLocalHeadMismatch: the workspace's current local head differs from
	// the requested head, so a fix moved HEAD since publication; maps to
	// invalid-state.
	ReasonPRLocalHeadMismatch = "local-head-mismatch"
	// ReasonPRRepositoryUnresolved: the GitHub repository identity could not be
	// resolved from the checkout; maps to external-failed.
	ReasonPRRepositoryUnresolved = "repository-unresolved"
	// ReasonPRUnknownChange / -AmbiguousID: the id names no record, or more than
	// one; the operation never chooses.
	ReasonPRUnknownChange = "unknown-change"
	ReasonPRAmbiguousID   = "ambiguous-change"
	// ReasonPRBodyTooLarge: the authored PR body exceeds the authored-input bound.
	ReasonPRBodyTooLarge = "authored-input-too-large"
	// ReasonPRBodyAssemblyFailed: the backlink/evidence blocks could not be woven
	// into the authored body (a malformed managed-block population, e.g.); maps to
	// invalid-state and predates any gh call.
	ReasonPRBodyAssemblyFailed = "body-assembly-failed"
)

// GitHubService is the seam `pr publish` delegates its GitHub mechanics to.
// *githubcli.Client satisfies it; unit tests inject a recording fake so the
// agreement-check table can prove EnsurePullRequest was never reached on a broken
// conjunct without spawning gh. The protocol-faithful fake-gh probe/act/verify
// matrix lives in the landed internal/githubcli suite; this seam pins only the
// app-layer disposition mapping and body assembly.
type GitHubService interface {
	DiscoverRepository(ctx context.Context, dir string) (githubcli.Repository, error)
	EnsurePullRequest(ctx context.Context, req githubcli.EnsurePullRequestRequest) (githubcli.EnsureResult, error)
	// FindOpenPullRequestsByHead is the read-only reprobe `change mark-implemented`
	// keys on before its transaction: it returns the open PRs GitHub reports for a
	// feature head branch and mutates nothing. It shares this seam so the reprobe
	// composes the same adapter the publication path uses, rather than a second
	// GitHub client or the package-private fake.
	FindOpenPullRequestsByHead(ctx context.Context, repo githubcli.Repository, headBranch string) ([]githubcli.PullRequest, error)
}

// GitHubDeps carries the GitHub-service seam, kept separate from PlanningDeps and
// WorkspaceDeps so the operation composes the read-only planning seams, the
// workspace engine, and the GitHub adapter without folding one into another.
type GitHubDeps struct {
	Service GitHubService
}

// PRPublishRequest is the closed request for `pr publish`. Head is the exact
// feature head the publication must certify. Title and Body are the authored PR
// prose (decoded from the request file at the CLI boundary). EvidenceRecord is
// the canonical build-evidence bytes, reparsed here — never a prior result.
type PRPublishRequest struct {
	ID    int    `json:"id"`
	Head  string `json:"head"`
	Title string `json:"title"`
	Body  string `json:"body"`
	// EvidenceRecord is read from the --evidence request file at the CLI boundary,
	// never a JSON key of this request, so it is excluded from the emitted schema
	// (mirroring EvidenceVerifyRequest.RecordFile).
	EvidenceRecord []byte `json:"-"`
}

// PRPublishResult is the protocol-v1 document `pr publish` returns. It names the
// canonical PR reference, url, number, head, base, and the adapter's disposition
// verbatim; a refusal carries a stable reason and message. It deliberately holds
// NO body or title field — the authored PR body bytes are redacted from every
// result and diagnostic (Global Constraints).
type PRPublishResult struct {
	Envelope
	ID          int    `json:"id,omitempty"`
	Reference   string `json:"reference,omitempty"`
	URL         string `json:"url,omitempty"`
	Number      int    `json:"number,omitempty"`
	Head        string `json:"head,omitempty"`
	Base        string `json:"base,omitempty"`
	Disposition string `json:"disposition,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Message     string `json:"message,omitempty"`
}

// HumanText renders the one-line human summary. It names identity, disposition,
// and the PR reference/url/head only — never the authored PR body (redaction).
func (r PRPublishResult) HumanText() string {
	if r.Result == ResultApplied || r.Result == ResultNoOp {
		return fmt.Sprintf("pr publish: change %04d %s %s (head %s, base %s)",
			r.ID, r.Disposition, r.Reference, shortCommit(r.Head), r.Base)
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// newPRResult stamps the envelope for the pr.publish operation.
func newPRResult(result Result, out PRPublishResult) PRPublishResult {
	out.Envelope = NewEnvelope(OperationPRPublish, result)
	return out
}

// prRefusal builds a refusing result carrying a stable reason and message.
func prRefusal(result Result, reason, message string, id int) PRPublishResult {
	return newPRResult(result, PRPublishResult{ID: id, Reason: reason, Message: message})
}

// PRPublish verifies every identity conjunct from authoritative sources, assembles
// the PR body by weaving the Docket-owned backlink and evidence blocks into the
// authored prose without disturbing a byte of it, and delegates to the idempotent
// EnsurePullRequest adapter. contended and unknown dispositions pass through
// verbatim — no force, no compensating close, no second create.
func PRPublish(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, gdeps GitHubDeps, repoDir string, req PRPublishRequest) PRPublishResult {
	// (1) The requested head must be a full lowercase-hex object id.
	if !validFullOID(req.Head) {
		return prRefusal(ResultInvalidInput, ReasonPRHeadInvalid,
			"the requested head is not a full lowercase-hex object id", req.ID)
	}

	// (2) Authored input is size-bounded (Global Constraints).
	if len(req.Body) > maxAuthoredMarkdownBytes {
		return prRefusal(ResultInvalidInput, ReasonPRBodyTooLarge,
			fmt.Sprintf("the authored PR body is %d bytes, over the %d-byte authored-input bound", len(req.Body), maxAuthoredMarkdownBytes), req.ID)
	}

	// (3) Reparse the evidence bytes — never a prior command result — and require
	// them to verify against the requested head: a missing, malformed, or
	// stale-head record means the gate no longer certifies this commit.
	if verdict := evidence.Verify(req.EvidenceRecord, req.Head); verdict != evidence.VerdictVerified && verdict != evidence.VerdictSkipped {
		return prRefusal(ResultInvalidState, ReasonPREvidenceUnverified,
			"the reparsed evidence does not verify (green or skipped) against the requested head ("+string(verdict)+")", req.ID)
	}
	rec, err := evidence.Extract(req.EvidenceRecord)
	if err != nil {
		// Unreachable after a verified verdict, but fail closed rather than trust it.
		return prRefusal(ResultInvalidState, ReasonPREvidenceUnverified, err.Error(), req.ID)
	}

	// (4) The workspace's current local head must equal the requested head; the
	// inspection also yields the feature and effective-base branches the domain
	// resolved. A pre-delegation workspace refusal passes through verbatim.
	insp := WorkspaceInspect(ctx, deps, wdeps, repoDir, WorkspaceIDRequest{ID: req.ID})
	if insp.Result != ResultApplied {
		return prRefusal(insp.Result, insp.Reason, insp.Message, req.ID)
	}
	if insp.Head != req.Head {
		return prRefusal(ResultInvalidState, ReasonPRLocalHeadMismatch,
			"the workspace head differs from the requested head; re-read and re-publish the head before opening a PR", req.ID)
	}
	headBranch := strings.TrimPrefix(insp.FeatureRef, branchRefPrefix)
	baseBranch := strings.TrimPrefix(insp.BaseRef, branchRefPrefix)

	// (5) The GitHub repository identity must resolve from the checkout.
	repo, err := gdeps.Service.DiscoverRepository(ctx, repoDir)
	if err != nil {
		return prRefusal(ResultExternalFailed, ReasonPRRepositoryUnresolved, err.Error(), req.ID)
	}

	// (6) Resolve the change record for the deterministic backlink block.
	change, link, refusal := resolvePRChange(ctx, deps, repoDir, req.ID)
	if refusal != nil {
		return *refusal
	}
	backlink, err := render.BacklinkContent(change, link)
	if err != nil {
		return prRefusal(ResultInternalError, ReasonStatusInternalError, err.Error(), req.ID)
	}

	// (7) Assemble the PR body: authored prose preserved byte-for-byte, only the
	// backlink and evidence blocks inserted/replaced.
	body, err := assemblePRBody([]byte(req.Body), backlink, rec)
	if err != nil {
		return prRefusal(ResultInvalidState, ReasonPRBodyAssemblyFailed, err.Error(), req.ID)
	}

	// (8) Delegate to the idempotent adapter. ExpectedHead is the requested head,
	// so the adapter refuses any GitHub head other than the published one — that is
	// the published-remote-head conjunct. ExpectedVersion is empty: v1 tracks no PR
	// version, so this is the create-or-adopt face.
	res, ensErr := gdeps.Service.EnsurePullRequest(ctx, githubcli.EnsurePullRequestRequest{
		Repository:   repo,
		HeadBranch:   headBranch,
		ExpectedHead: req.Head,
		BaseBranch:   baseBranch,
		Title:        req.Title,
		Body:         string(body),
	})
	if ensErr != nil {
		return mapGitHubFailure(ensErr, req.ID)
	}
	return prResultFromEnsure(repo, req.ID, res)
}

// resolvePRChange pins context once, reads the corpus once, builds the snapshot,
// and returns the change named by id (a typed unknown/ambiguous refusal otherwise)
// with the link context its backlink renders under.
func resolvePRChange(ctx context.Context, deps PlanningDeps, repoDir string, id int) (domain.Change, render.LinkContext, *PRPublishResult) {
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := prRefusal(result, reason, err.Error(), id)
		return domain.Change{}, render.LinkContext{}, &r
	}
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := prRefusal(result, reason, err.Error(), id)
		return domain.Change{}, render.LinkContext{}, &r
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: pin.Config.Effective, Documents: inputs})
	if err != nil {
		r := prRefusal(ResultInternalError, ReasonStatusInternalError, err.Error(), id)
		return domain.Change{}, render.LinkContext{}, &r
	}
	c, out := build.Snapshot.Change(domain.ChangeID(id))
	if out != domain.LookupFound {
		reason, result := ReasonPRUnknownChange, ResultInvalidInput
		msg := fmt.Sprintf("no change %04d is present in the corpus", id)
		if out == domain.LookupAmbiguous {
			reason, result = ReasonPRAmbiguousID, ResultInvalidState
			msg = fmt.Sprintf("more than one record claims change id %04d; refusing to choose", id)
		}
		r := prRefusal(result, reason, msg, id)
		return domain.Change{}, render.LinkContext{}, &r
	}
	return c, linkContextOf(pin), nil
}

// assemblePRBody weaves the Docket-owned backlink and build-evidence blocks into
// the authored PR prose. The backlink block is inserted at the top (or replaced in
// place if already present), then the evidence block is upserted through the
// loss-preserving document patch API — so every authored byte outside the two
// managed blocks is preserved exactly. A malformed managed-block population in the
// authored body fails the parse and returns no bytes.
func assemblePRBody(authored []byte, backlink string, rec evidence.Record) ([]byte, error) {
	doc, err := document.Parse(authored)
	if err != nil {
		return nil, err
	}
	interior := backlinkInterior(backlink)
	var ps document.PatchSet
	if _, ok := doc.Block(backlinkBlockName); ok {
		ps.ReplaceBlock(backlinkBlockName, interior)
	} else {
		at := document.AtDocumentStart
		if doc.HasFrontmatter() {
			at = document.AfterFrontmatter
		}
		ps.InsertBlock(backlinkBlockName, backlinkBlockAnnotation, interior, at)
	}
	withBacklink, err := doc.Apply(ps)
	if err != nil {
		return nil, err
	}
	return evidence.Upsert(withBacklink, rec)
}

// prResultFromEnsure maps a value disposition onto the protocol taxonomy, carrying
// the verified PR snapshot's canonical fields (never its body). created/updated are
// applied work; adopted/unchanged are idempotent no-ops; contended is a lost race;
// unknown is an unverified external effect the caller must reprobe — all with the
// adapter's disposition verbatim.
func prResultFromEnsure(repo githubcli.Repository, id int, res githubcli.EnsureResult) PRPublishResult {
	var result Result
	switch res.Disposition {
	case githubcli.EnsureCreated, githubcli.EnsureUpdated:
		result = ResultApplied
	case githubcli.EnsureAdopted, githubcli.EnsureUnchanged:
		result = ResultNoOp
	case githubcli.EnsureContended:
		return newPRResult(ResultContended, PRPublishResult{ID: id, Disposition: string(res.Disposition)})
	case githubcli.EnsureUnknown:
		return newPRResult(ResultExternalFailed, PRPublishResult{ID: id, Disposition: string(res.Disposition)})
	default:
		return newPRResult(ResultInternalError, PRPublishResult{ID: id, Disposition: string(res.Disposition)})
	}
	pr := res.PR
	return newPRResult(result, PRPublishResult{
		ID:          id,
		Reference:   fmt.Sprintf("%s#%d", repo.Spec(), pr.Number),
		URL:         pr.URL,
		Number:      pr.Number,
		Head:        pr.HeadCommit,
		Base:        pr.BaseBranch,
		Disposition: string(res.Disposition),
	})
}

// mapGitHubFailure folds a githubcli.Failure (an EnsureFailed outcome) onto the
// protocol taxonomy. The Failure's Kind is the stable reason and its Detail is
// already bounded and redacted, so it rides through as the message.
func mapGitHubFailure(err error, id int) PRPublishResult {
	result := ResultInternalError
	reason := ReasonStatusInternalError
	message := err.Error()
	if f, ok := githubcli.AsFailure(err); ok {
		reason = string(f.Kind)
		message = f.Error()
		switch f.Kind {
		case githubcli.KindInvalidInput:
			result = ResultInvalidInput
		case githubcli.KindInvalidState:
			result = ResultInvalidState
		case githubcli.KindExternal, githubcli.KindInvalidOutput, githubcli.KindTimedOut:
			result = ResultExternalFailed
		case githubcli.KindCancelled:
			result = ResultInterrupted
		}
	}
	return prRefusal(result, reason, message, id)
}

// validFullOID reports whether s is a full lowercase hexadecimal object id: 40
// (SHA-1) or 64 (SHA-256) chars, each 0-9 or a-f. It mirrors the githubcli and
// evidence packages' own local checks; keeping a copy here avoids importing an
// unexported validator across a package boundary.
func validFullOID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
