package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is the `finalize publish` operation: the narrow, receipt-scoped
// publication of a rebased (rewritten) feature head onto its remote feature ref,
// followed by the loss-preserving update of the Docket build-evidence block on
// the existing pull request. It owns the ORDER and the closed-outcome mapping;
// every mechanic it composes is landed: the receipt-scoped force-with-lease push
// lives in internal/workspace (PublishRewrite, Task 5), the evidence-block
// replacement lives in internal/evidence (Upsert, the same writer `pr publish`
// uses), and the probe/act/verify PR edit lives in internal/githubcli
// (EnsurePullRequest). This layer wires them and holds no force-push escape hatch
// of its own — the only non-fast-forward push is the one the receipt authorizes.
//
// The order (spec §"Rewritten-head publication and PR evidence"):
//
//  1. gate the request shape, capability preflight, and canonical evidence bytes
//     (bounded, and green for the EXACT requested head);
//  2. read the owned rebase receipt and refuse a foreign attempt token BEFORE any
//     push — the attempt is the authorization, and a mismatch never pushes;
//  3. PublishRewrite probes the remote feature ref, is a no-op when it already
//     equals the intended head, and otherwise pushes exactly that head under the
//     receipt's exact old-remote-head lease and reprobes to equality; a divergent
//     remote is contended (untouched) and an unobservable remote is unknown
//     (retain — never a second, forced push);
//  4. only a published-or-noop rewrite proceeds — the PR is reprobed for the
//     feature head, its version captured, and its head required to equal the
//     requested head;
//  5. the current PR body's build-evidence block is loss-preservingly replaced
//     with the exact current-head green record (every authored byte, the title,
//     and every other block preserved); and
//  6. EnsurePullRequest converges the PR onto that body under the exact expected
//     head and version, never creating a second PR.
//
// Crash replay is ordinary. A crash after the push but before the PR update is a
// no-op rewrite (the remote already holds the head) that resumes the PR update. A
// crash after both is a no-op rewrite plus an already-equal PR body — a full
// no-op. An unknown probe never authorizes a second mutation or a merge-enabling
// success.

// OperationFinalizePublish is the operation key `finalize publish` records in its
// result envelope.
const OperationFinalizePublish = "finalize.publish"

// The closed set of `finalize publish` dispositions.
const (
	// PublishDispPublished: the rewrite reached the remote (or was already there)
	// AND the PR build-evidence block was converged to the current head.
	PublishDispPublished = "published"
	// PublishDispNoop: the remote already held the head and the PR already carried
	// the exact current-head evidence — a full idempotent replay.
	PublishDispNoop = "noop"
	// PublishDispContended: a lost race the caller resolves by re-reading context —
	// a diverged remote, or a PR at an unexpected head/version. Nothing was forced.
	PublishDispContended = "contended"
	// PublishDispUnknown: an external effect could not be established (a remote or
	// PR probe error). Retained; never a second mutation and never a merge-enabling
	// success.
	PublishDispUnknown = "unknown"
	// PublishDispBlocked: a retained precondition refusal (no owned attempt, a
	// foreign attempt, a not-implemented record, no single open PR); a human or a
	// re-read is needed.
	PublishDispBlocked = "blocked"
)

// The stable machine reasons `finalize publish` reports. Message text is
// explanatory and must not be parsed.
const (
	// ReasonPublishNotImplemented: the change is not `implemented`, so there is no
	// finalize-half publication to perform.
	ReasonPublishNotImplemented = "not-implemented"
	// ReasonPublishNoReceipt: no owned rebase receipt authorizes a rewrite
	// publication for this change.
	ReasonPublishNoReceipt = "no-rebase-receipt"
	// ReasonPublishReceiptRead: the receipt was present but unreadable/corrupt —
	// never a clean absence.
	ReasonPublishReceiptRead = "receipt-read-failed"
	// ReasonPublishForeignAttempt: the supplied attempt token does not match the
	// owned receipt; refused before any push.
	ReasonPublishForeignAttempt = "attempt-token-mismatch"
	// ReasonPublishEvidenceTooLarge: the canonical evidence bytes exceed the
	// authored-input bound.
	ReasonPublishEvidenceTooLarge = "authored-input-too-large"
	// ReasonPublishEvidenceUnverified: the canonical evidence does not parse green
	// for the exact requested head; the run no longer certifies this commit.
	ReasonPublishEvidenceUnverified = "evidence-unverified"
	// ReasonPublishRewriteFailed: the receipt-scoped rewrite push failed
	// definitely (the remote is unchanged) — a retryable external failure.
	ReasonPublishRewriteFailed = "rewrite-push-failed"
	// ReasonPublishRewriteContended: the remote feature ref diverged from the
	// recorded old value and is not the intended head; the remote is untouched.
	ReasonPublishRewriteContended = "rewrite-contended"
	// ReasonPublishRewriteUnknown: the remote feature ref could not be observed;
	// retained, never forced.
	ReasonPublishRewriteUnknown = "rewrite-unknown"
	// ReasonPublishRepoUnresolved: the GitHub repository identity did not resolve.
	ReasonPublishRepoUnresolved = "repository-unresolved"
	// ReasonPublishPRProbeFailed: the PR reprobe could not be established after the
	// rewrite; unknown, never a second mutation.
	ReasonPublishPRProbeFailed = "pr-probe-failed"
	// ReasonPublishPRNotOpen: not exactly one open PR for the feature head; a
	// rewrite publication requires exactly one.
	ReasonPublishPRNotOpen = "pr-not-open"
	// ReasonPublishPRHeadMismatch: the open PR names a head other than the
	// requested (rewritten) head; contended.
	ReasonPublishPRHeadMismatch = "pr-head-mismatch"
	// ReasonPublishBodyAssembly: the build-evidence block could not be woven into
	// the current PR body (a malformed managed-block population); predates the edit.
	ReasonPublishBodyAssembly = "body-assembly-failed"
	// ReasonPublishEnsurerUnavailable: the wired GitHub seam does not provide the
	// PR create-or-edit face; an internal wiring error.
	ReasonPublishEnsurerUnavailable = "pr-editor-unavailable"
)

// FinalizePublishRequest is the closed request for `finalize publish`. ID names
// the change; Attempt is the owned rebase attempt token that authorizes the
// rewrite (it must match the receipt); Head is the exact rewritten feature head
// to publish and certify; EvidenceRecord is the canonical build-evidence bytes
// (read from the request file at the CLI boundary, never a prior command result),
// reparsed here and required to certify Head.
type FinalizePublishRequest struct {
	ID             int    `docket:"required"`
	Attempt        string `docket:"required"`
	Head           string `docket:"required"`
	EvidenceRecord []byte `docket:"required"`
}

// FinalizePublishResult is the protocol-v1 document `finalize publish` returns. It
// names identity, the closed disposition, the exact head/base the publication
// certified, the PR reference/number/url, and the rewrite outcome token; a
// refusal carries a stable reason and message, and a shape refusal carries
// findings. It holds NO authored PR body bytes (redaction: the body crosses only
// through the evidence writer and the gh stdin, never a result).
type FinalizePublishResult struct {
	Envelope
	ID          int             `json:"id,omitempty"`
	Disposition string          `json:"disposition,omitempty"`
	Head        string          `json:"head,omitempty"`
	Base        string          `json:"base,omitempty"`
	Number      int             `json:"number,omitempty"`
	Reference   string          `json:"reference,omitempty"`
	URL         string          `json:"url,omitempty"`
	Rewrite     string          `json:"rewrite,omitempty"`
	Reason      string          `json:"reason,omitempty"`
	Message     string          `json:"message,omitempty"`
	Findings    []StatusFinding `json:"findings"`
}

// HumanText renders a one-line summary naming identity, disposition, and the PR
// reference/head only — never the authored PR body.
func (r FinalizePublishResult) HumanText() string {
	if r.Result == ResultApplied || r.Result == ResultNoOp {
		s := fmt.Sprintf("%s: change %04d %s", r.Operation, r.ID, r.Disposition)
		if r.Reference != "" {
			s += fmt.Sprintf(" %s (head %s)", r.Reference, shortCommit(r.Head))
		}
		return s
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// newPublishResult stamps the envelope for the finalize.publish operation and
// normalizes the findings collection so a nil never leaks into the document.
func newPublishResult(result Result, out FinalizePublishResult) FinalizePublishResult {
	out.Envelope = NewEnvelope(OperationFinalizePublish, result)
	if out.Findings == nil {
		out.Findings = []StatusFinding{}
	}
	return out
}

// publishRefusal builds a refusing result carrying a stable reason, message, and
// disposition.
func publishRefusal(result Result, disposition, reason, message string, id int) FinalizePublishResult {
	return newPublishResult(result, FinalizePublishResult{
		ID: id, Disposition: disposition, Reason: reason, Message: message,
	})
}

// finalizePublishEnsurer is the PR create-or-edit face `finalize publish` needs
// to converge the build-evidence block on the existing pull request. The shared
// FinalizeGitHub seam deliberately does not name it — it is `pr publish`'s
// idempotent adapter — so this operation resolves it from the concrete GitHub
// client through a narrow local interface rather than growing that shared seam
// for one operation. *githubcli.Client satisfies it; a unit-test fake implements
// FinalizeGitHub and this face together.
type finalizePublishEnsurer interface {
	EnsurePullRequest(ctx context.Context, req githubcli.EnsurePullRequestRequest) (githubcli.EnsureResult, error)
}

// FinalizePublish publishes a rewritten feature head onto its remote ref under
// the owned receipt's exact lease and converges the PR build-evidence block onto
// the exact current head, idempotently and in that order. It never forces past
// the recorded lease, never creates a second PR, and treats every unknown probe
// as retain — no second mutation and no merge-enabling success.
func FinalizePublish(ctx context.Context, deps FinalizeDeps, repoDir string, req FinalizePublishRequest) FinalizePublishResult {
	if findings := validatePublishShape(req); len(findings) > 0 {
		return newPublishResult(ResultInvalidInput, FinalizePublishResult{ID: req.ID, Findings: findings})
	}

	// Capability preflight before any external effect: a configuration actively
	// requesting a deferred capability is unsupported-config, not a publication.
	pin, err := deps.Planning.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return publishRefusal(result, PublishDispBlocked, reason, err.Error(), req.ID)
	}
	if decision := config.PreflightMutation(&pin.Config); !decision.Allowed {
		return publishRefusal(ResultUnsupportedConfig, "", ReasonDeferredCapRequested,
			"configuration actively requests a deferred capability docket does not ship in this version ("+
				strings.Join(blockerPaths(decision.Blockers), ", ")+"); withdraw it before any mutation", req.ID)
	}

	// The canonical evidence bytes are reparsed — never a prior command result —
	// bounded, and required to certify the EXACT requested head.
	if len(req.EvidenceRecord) > maxAuthoredMarkdownBytes {
		return publishRefusal(ResultInvalidInput, "", ReasonPublishEvidenceTooLarge,
			fmt.Sprintf("the evidence record is %d bytes, over the %d-byte authored-input bound", len(req.EvidenceRecord), maxAuthoredMarkdownBytes), req.ID)
	}
	if verdict := evidence.Verify(req.EvidenceRecord, req.Head); verdict != evidence.VerdictVerified && verdict != evidence.VerdictSkipped {
		return publishRefusal(ResultInvalidState, "", ReasonPublishEvidenceUnverified,
			"the reparsed evidence does not verify (green or skipped) against the requested head ("+string(verdict)+")", req.ID)
	}
	rec, err := evidence.Extract(req.EvidenceRecord)
	if err != nil {
		// Unreachable after a verified verdict, but fail closed rather than trust it.
		return publishRefusal(ResultInvalidState, "", ReasonPublishEvidenceUnverified, err.Error(), req.ID)
	}

	// Resolve the change, the discovered repository, and the validated target.
	wc, wref := loadWorkspaceContext(ctx, deps.Planning, repoDir, req.ID, OperationFinalizePublish)
	if wref != nil {
		return translateWorkspaceRefusalToPublish(*wref)
	}
	id := int(wc.change.ID())
	if wc.change.Status() != domain.StatusImplemented {
		return publishRefusal(ResultBlocked, PublishDispBlocked, ReasonPublishNotImplemented,
			fmt.Sprintf("change %04d is %q, not implemented; there is no finalize-half publication to perform", id, wc.change.RawStatus()), id)
	}
	target, tref := resolveWorkspaceTarget(OperationFinalizePublish, wc)
	if tref != nil {
		return translateWorkspaceRefusalToPublish(*tref)
	}
	metaDir := workspace.MetaDir(wc.repo.CommonDir, target.FeatureRef)

	// The owned receipt authorizes the rewrite. A foreign attempt token is refused
	// BEFORE any push — the attempt, not the request, is the authorization.
	receipt, present, rerr := deps.Workspace.ReadRebaseReceipt(ctx, metaDir)
	if rerr != nil {
		return publishRefusal(ResultExternalFailed, PublishDispBlocked, ReasonPublishReceiptRead, rerr.Error(), id)
	}
	if !present {
		return publishRefusal(ResultBlocked, PublishDispBlocked, ReasonPublishNoReceipt,
			"no owned rebase attempt is recorded for this change; nothing to publish", id)
	}
	if receipt.ChangeID != strconv.Itoa(id) || receipt.Attempt != req.Attempt {
		return publishRefusal(ResultBlocked, PublishDispBlocked, ReasonPublishForeignAttempt,
			"the supplied attempt token does not match the owned rebase receipt; refusing to push", id)
	}

	// Publish the rewrite under the receipt's exact lease. This probes the remote
	// first, is a no-op when it already holds the intended head, and otherwise
	// pushes exactly that head under --force-with-lease against the recorded old
	// value. A diverged remote is contended (untouched); an unobservable remote is
	// unknown (retained, never forced).
	rout, perr := deps.Workspace.PublishRewrite(ctx, workspace.RewriteRequest{Dir: metaDir, Receipt: receipt, NewHead: req.Head})
	if perr != nil {
		return mapPublishRewriteFailure(id, req.Head, perr)
	}
	switch rout {
	case workspace.RewriteContended:
		return newPublishResult(ResultContended, FinalizePublishResult{
			ID: id, Disposition: PublishDispContended, Head: req.Head, Rewrite: string(rout),
			Reason:  ReasonPublishRewriteContended,
			Message: "the remote feature ref diverged from the recorded old value; re-read context finalize",
		})
	case workspace.RewriteUnknown:
		return newPublishResult(ResultExternalFailed, FinalizePublishResult{
			ID: id, Disposition: PublishDispUnknown, Head: req.Head, Rewrite: string(rout),
			Reason:  ReasonPublishRewriteUnknown,
			Message: "the remote feature ref could not be observed; retained, no forced push",
		})
	case workspace.RewritePublished, workspace.RewriteNoop:
		// The remote holds exactly the intended head; resume the PR update. A noop
		// here is the crash-after-push replay face — the PR update still runs.
	default:
		return publishRefusal(ResultInternalError, PublishDispBlocked, ReasonStatusInternalError,
			fmt.Sprintf("unexpected rewrite outcome %q", rout), id)
	}

	// Reprobe the existing PR for the feature head and converge its evidence block.
	repo, err := deps.GitHub.DiscoverRepository(ctx, repoDir)
	if err != nil {
		return newPublishResult(ResultExternalFailed, FinalizePublishResult{
			ID: id, Disposition: PublishDispUnknown, Head: req.Head, Rewrite: string(rout),
			Reason: ReasonPublishRepoUnresolved, Message: err.Error(),
		})
	}
	featureBranch := strings.TrimPrefix(string(target.FeatureRef), branchRefPrefix)
	prs, err := deps.GitHub.FindOpenPullRequestsByHead(ctx, repo, featureBranch)
	if err != nil {
		// The reprobe could not be established: unknown. Retain — no PR mutation.
		return newPublishResult(ResultExternalFailed, FinalizePublishResult{
			ID: id, Disposition: PublishDispUnknown, Head: req.Head, Rewrite: string(rout),
			Reason: ReasonPublishPRProbeFailed, Message: err.Error(),
		})
	}
	if len(prs) != 1 {
		return publishRefusal(ResultBlocked, PublishDispBlocked, ReasonPublishPRNotOpen,
			fmt.Sprintf("%d open pull requests for the feature head; a rewrite publication requires exactly one", len(prs)), id)
	}
	pr := prs[0]
	if pr.HeadCommit != req.Head {
		return newPublishResult(ResultContended, FinalizePublishResult{
			ID: id, Disposition: PublishDispContended, Head: req.Head, Number: pr.Number, Rewrite: string(rout),
			Reason:  ReasonPublishPRHeadMismatch,
			Message: "the open PR names a head other than the requested rewritten head; re-read context finalize",
		})
	}

	// Replace ONLY the build-evidence block in the current PR body — every authored
	// byte, the title, and every other block are preserved.
	newBody, err := evidence.Upsert([]byte(pr.Body), rec)
	if err != nil {
		return publishRefusal(ResultInvalidState, PublishDispBlocked, ReasonPublishBodyAssembly, err.Error(), id)
	}

	ensurer, ok := deps.GitHub.(finalizePublishEnsurer)
	if !ok {
		return publishRefusal(ResultInternalError, PublishDispBlocked, ReasonPublishEnsurerUnavailable,
			"the wired GitHub seam does not provide the pull-request edit face", id)
	}
	eres, eerr := ensurer.EnsurePullRequest(ctx, githubcli.EnsurePullRequestRequest{
		Repository:      repo,
		HeadBranch:      featureBranch,
		ExpectedHead:    req.Head,
		BaseBranch:      pr.BaseBranch,
		Title:           pr.Title,
		Body:            string(newBody),
		ExpectedVersion: pr.Version,
	})
	if eerr != nil {
		return mapPublishEnsureFailure(id, req.Head, string(rout), eerr)
	}
	return publishResultFromEnsure(id, req.Head, rout, repo, eres)
}

// translateWorkspaceRefusalToPublish maps a workspace-shaped pre-delegation
// refusal onto a publish result, preserving the protocol Result/Reason/Message
// and deriving a publish disposition from the Result class.
func translateWorkspaceRefusalToPublish(w WorkspaceOpResult) FinalizePublishResult {
	disp := PublishDispBlocked
	switch w.Result {
	case ResultContended:
		disp = PublishDispContended
	case ResultInvalidInput:
		disp = ""
	}
	return publishRefusal(w.Result, disp, w.Reason, w.Message, w.ID)
}

// publishResultFromEnsure maps the PR edit disposition onto the publish taxonomy.
// created/updated are applied work (published). adopted/unchanged are idempotent:
// they are `published`/applied when the rewrite itself did work this run, and a
// full `noop`/no-op only when the rewrite was already in place too. contended and
// unknown pass through; nothing is forced and no second PR is created.
func publishResultFromEnsure(id int, head string, rewrite workspace.RewriteOutcome, repo githubcli.Repository, eres githubcli.EnsureResult) FinalizePublishResult {
	switch eres.Disposition {
	case githubcli.EnsureCreated, githubcli.EnsureUpdated:
		return newPublishResult(ResultApplied, publishSuccess(id, head, rewrite, repo, eres.PR, PublishDispPublished))
	case githubcli.EnsureAdopted, githubcli.EnsureUnchanged:
		if rewrite == workspace.RewritePublished {
			return newPublishResult(ResultApplied, publishSuccess(id, head, rewrite, repo, eres.PR, PublishDispPublished))
		}
		return newPublishResult(ResultNoOp, publishSuccess(id, head, rewrite, repo, eres.PR, PublishDispNoop))
	case githubcli.EnsureContended:
		return newPublishResult(ResultContended, FinalizePublishResult{
			ID: id, Disposition: PublishDispContended, Head: head, Rewrite: string(rewrite),
			Reason:  ReasonPublishPRHeadMismatch,
			Message: "the pull request diverged under the update; re-read context finalize",
		})
	case githubcli.EnsureUnknown:
		return newPublishResult(ResultExternalFailed, FinalizePublishResult{
			ID: id, Disposition: PublishDispUnknown, Head: head, Rewrite: string(rewrite),
			Reason:  ReasonPublishPRProbeFailed,
			Message: "the pull-request update could not be verified; retained, no second mutation",
		})
	default:
		return publishRefusal(ResultInternalError, PublishDispBlocked, ReasonStatusInternalError,
			fmt.Sprintf("unexpected pull-request edit disposition %q", eres.Disposition), id)
	}
}

// publishSuccess assembles the success payload from the verified PR snapshot,
// carrying the canonical reference/number/url/base — never the body.
func publishSuccess(id int, head string, rewrite workspace.RewriteOutcome, repo githubcli.Repository, pr githubcli.PullRequest, disposition string) FinalizePublishResult {
	return FinalizePublishResult{
		ID:          id,
		Disposition: disposition,
		Head:        head,
		Base:        pr.BaseBranch,
		Number:      pr.Number,
		Reference:   fmt.Sprintf("%s#%d", repo.Spec(), pr.Number),
		URL:         pr.URL,
		Rewrite:     string(rewrite),
	}
}

// mapPublishRewriteFailure maps a definite PublishRewrite error onto the publish
// taxonomy. A workspace failure carries its kind; a receipt/verify refusal is an
// invalid-state block, a definite push failure is an external retryable failure.
func mapPublishRewriteFailure(id int, head string, err error) FinalizePublishResult {
	result := ResultInternalError
	message := err.Error()
	if f, ok := workspace.AsFailure(err); ok {
		message = f.Error()
		switch f.Kind {
		case workspace.KindInvalidInput:
			result = ResultInvalidInput
		case workspace.KindInvalidState:
			result = ResultBlocked
		case workspace.KindExternal, workspace.KindInvalidOutput, workspace.KindTimedOut:
			result = ResultExternalFailed
		case workspace.KindCancelled:
			result = ResultInterrupted
		}
	}
	return newPublishResult(result, FinalizePublishResult{
		ID: id, Disposition: PublishDispBlocked, Head: head,
		Reason: ReasonPublishRewriteFailed, Message: message,
	})
}

// mapPublishEnsureFailure folds a githubcli EnsureFailed error onto the publish
// taxonomy. The failure's kind is the stable reason and its detail is already
// bounded and redacted.
func mapPublishEnsureFailure(id int, head, rewrite string, err error) FinalizePublishResult {
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
	return newPublishResult(result, FinalizePublishResult{
		ID: id, Disposition: PublishDispBlocked, Head: head, Rewrite: rewrite,
		Reason: reason, Message: message,
	})
}

// validatePublishShape runs the configuration-independent request checks for
// `finalize publish`: a positive id, a non-empty attempt token, a valid
// full-length object id for the requested head, and non-empty evidence bytes.
func validatePublishShape(req FinalizePublishRequest) []StatusFinding {
	var findings []StatusFinding
	if req.ID <= 0 {
		findings = append(findings, lifecycleFinding(FCInvalidChangeID, "change_id must be a positive change id"))
	}
	if strings.TrimSpace(req.Attempt) == "" {
		findings = append(findings, lifecycleFinding(FCEmptyAttempt, "attempt must name the owned rebase attempt token"))
	}
	if !validFullObjectID(req.Head) {
		findings = append(findings, lifecycleFinding(FCInvalidHead,
			"head must be a full 40- or 64-character lowercase hex object id"))
	}
	if len(req.EvidenceRecord) == 0 {
		findings = append(findings, lifecycleFinding(FCEmptyEvidence, "evidence must carry the canonical build-evidence record bytes"))
	}
	return findings
}
