package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is the `workspace checkout` operation: it puts an implemented
// change into its own docket-owned worktree at the head the change's open PR
// and its remote branch agree on, so a change built on another machine can be
// finalized here. The change, its exact record revision, and its effective base
// come from the shared workspace context (loadWorkspaceContext). The PR half of
// the head is proved here with probeRebasePR's whole predicate (one open PR for
// the feature branch, targeting the effective base, naming the caller's head
// when one is pinned); workspace.Service.Checkout proves the remote half under
// its per-workspace lock and owns every Git effect. A service error is an
// unknown outcome: the caller reprobes, never retries with force.

// OperationWorkspaceCheckout is the operation key the checkout records in its
// envelope.
const OperationWorkspaceCheckout = "workspace.checkout"

// The stable machine reasons workspace checkout reports for its own typed
// refusals (the service's reasons, such as head-mismatch, pass through
// verbatim). Message text is explanatory and must not be parsed.
const (
	// ReasonWorkspaceCheckoutPinIncomplete: --revision and --head pin the
	// caller's read together; one without the other is invalid input.
	ReasonWorkspaceCheckoutPinIncomplete = "revision-head-pair"
	// ReasonWorkspaceMalformedHead: --head is not a full lowercase object id.
	ReasonWorkspaceMalformedHead = "malformed-head"
	// ReasonWorkspaceNotImplemented: checkout takes over only an implemented
	// change's pushed branch.
	ReasonWorkspaceNotImplemented = "not-implemented"
	// ReasonWorkspaceRevisionDrift: the record moved since the pinned revision.
	ReasonWorkspaceRevisionDrift = "revision-drift"
	// ReasonWorkspaceCheckoutRepoUnresolved: the GitHub repository could not be
	// resolved, so the PR half of the head is unproven.
	ReasonWorkspaceCheckoutRepoUnresolved = "repository-unresolved"
	// ReasonWorkspaceCheckoutPRProbe: the open-PR lookup failed.
	ReasonWorkspaceCheckoutPRProbe = "pr-probe-failed"
	// ReasonWorkspaceCheckoutPRNotOpen: the feature branch has no single open PR.
	ReasonWorkspaceCheckoutPRNotOpen = "pr-not-open"
	// ReasonWorkspaceCheckoutPRBaseMismatch: the open PR targets a base other
	// than the change's effective base.
	ReasonWorkspaceCheckoutPRBaseMismatch = "pr-base-mismatch"
)

// The report-only findings an attached checkout surfaces.
const (
	// FindingForeignWorktreeRemoved names a clean foreign worktree checkout
	// removed to attach the change's branch at its owned path.
	FindingForeignWorktreeRemoved = "foreign-worktree-removed"
	// FindingLocalBranchFastForwarded names a local feature branch checkout
	// fast-forwarded to the pull request head.
	FindingLocalBranchFastForwarded = "local-branch-fast-forwarded"
)

// checkoutDispUnknown is the disposition of an unverified checkout outcome.
const checkoutDispUnknown = "unknown"

// WorkspaceCheckoutRequest is the closed request for checkout. Revision and
// Head are optional but travel together: the exact record revision and the PR
// head the caller read, pinning the checkout to that read.
type WorkspaceCheckoutRequest struct {
	ID       int    `json:"id"`
	Revision string `json:"revision,omitempty"`
	Head     string `json:"head,omitempty"`
}

// WorkspacePRReader is the GitHub seam checkout proves the PR half of the head
// through. *githubcli.Client satisfies it, as does any FinalizeGitHub.
type WorkspacePRReader interface {
	DiscoverRepository(ctx context.Context, dir string) (githubcli.Repository, error)
	FindOpenPullRequestsByHead(ctx context.Context, repo githubcli.Repository, headBranch string) ([]githubcli.PullRequest, error)
}

// WorkspaceCheckoutService is the seam checkout delegates its Git mechanics to.
// *workspace.Service satisfies it.
type WorkspaceCheckoutService interface {
	Checkout(ctx context.Context, req workspace.CheckoutRequest) (workspace.CheckoutResult, error)
}

// WorkspaceCheckoutDeps carries checkout's workspace-service and GitHub seams.
type WorkspaceCheckoutDeps struct {
	Service WorkspaceCheckoutService
	GitHub  WorkspacePRReader
}

// WorkspaceCheckout checks an implemented change's pushed feature branch out
// into its own worktree at the head its open PR and remote agree on. Every
// refusal before the service call changes nothing; the service's refusals
// change only its lock dir and fetched remote-tracking refs.
func WorkspaceCheckout(ctx context.Context, deps PlanningDeps, cdeps WorkspaceCheckoutDeps, repoDir string, req WorkspaceCheckoutRequest) WorkspaceOpResult {
	op := OperationWorkspaceCheckout
	refuse := func(result Result, disposition, reason, message string) WorkspaceOpResult {
		return newWorkspaceResult(op, result, WorkspaceOpResult{ID: req.ID, Disposition: disposition, Reason: reason, Message: message})
	}

	if (req.Revision == "") != (req.Head == "") {
		return refuse(ResultInvalidInput, "", ReasonWorkspaceCheckoutPinIncomplete,
			"--revision and --head pin one read together; give both or neither")
	}
	if req.Head != "" && !validFullObjectID(req.Head) {
		return refuse(ResultInvalidInput, "", ReasonWorkspaceMalformedHead,
			fmt.Sprintf("head %q must be a full 40- or 64-character lowercase hex object id", req.Head))
	}

	wc, refusal := loadWorkspaceContext(ctx, deps, repoDir, req.ID, op)
	if refusal != nil {
		return *refusal
	}
	if wc.change.Status() != domain.StatusImplemented {
		return refuse(ResultBlocked, string(workspace.CheckoutBlocked), ReasonWorkspaceNotImplemented,
			fmt.Sprintf("change %04d is %q, not implemented; workspace checkout takes over only an implemented change's pushed branch", req.ID, wc.change.RawStatus()))
	}
	if req.Revision != "" && wc.revision != req.Revision {
		return refuse(ResultContended, string(workspace.CheckoutContended), ReasonWorkspaceRevisionDrift,
			fmt.Sprintf("the change record is at revision %s, not the pinned %s; re-read authoritative context before checking out", wc.revision, req.Revision))
	}
	target, tRefusal := resolveWorkspaceTarget(op, wc)
	if tRefusal != nil {
		return *tRefusal
	}

	// The PR half of the head: exactly one open PR for the feature branch,
	// targeting the effective base, and naming the pinned head when one is given.
	repo, err := cdeps.GitHub.DiscoverRepository(ctx, repoDir)
	if err != nil {
		return refuse(ResultExternalFailed, checkoutDispUnknown, ReasonWorkspaceCheckoutRepoUnresolved, err.Error())
	}
	featureBranch := strings.TrimPrefix(string(target.FeatureRef), branchRefPrefix)
	prs, err := cdeps.GitHub.FindOpenPullRequestsByHead(ctx, repo, featureBranch)
	if err != nil {
		return refuse(ResultExternalFailed, checkoutDispUnknown, ReasonWorkspaceCheckoutPRProbe, err.Error())
	}
	if len(prs) != 1 {
		return refuse(ResultBlocked, string(workspace.CheckoutBlocked), ReasonWorkspaceCheckoutPRNotOpen,
			fmt.Sprintf("%d open pull requests for branch %s; checkout requires exactly one", len(prs), featureBranch))
	}
	pr := prs[0]
	if pr.BaseBranch != wc.base.Branch {
		return refuse(ResultBlocked, string(workspace.CheckoutBlocked), ReasonWorkspaceCheckoutPRBaseMismatch,
			fmt.Sprintf("pull request #%d targets %s, not the change's effective base %s", pr.Number, pr.BaseBranch, wc.base.Branch))
	}
	if req.Head != "" && req.Head != pr.HeadCommit {
		return refuse(ResultContended, string(workspace.CheckoutContended), ReasonWorkspaceHeadMismatch,
			fmt.Sprintf("pull request #%d names head %s, not the pinned %s; re-read before checking out", pr.Number, pr.HeadCommit, req.Head))
	}

	res, err := cdeps.Service.Checkout(ctx, workspace.CheckoutRequest{
		Repository:       wc.repo,
		Remote:           originRemote,
		Target:           target,
		ExpectedHead:     gitcli.ObjectID(pr.HeadCommit),
		MetadataWorktree: wc.pin.Layout.MetadataWorktree,
	})
	if err != nil {
		out := mapWorkspaceFailure(op, req.ID, err)
		out.Disposition = checkoutDispUnknown
		return out
	}
	return checkoutResult(op, req.ID, res)
}

// checkoutResult maps a checkout disposition onto the protocol result taxonomy:
// attached is applied work, existing a no-op, blocked and contended pass
// through. A removed foreign worktree and a fast-forwarded local branch are
// reported as warning findings.
func checkoutResult(opKey string, id int, res workspace.CheckoutResult) WorkspaceOpResult {
	var result Result
	switch res.Disposition {
	case workspace.CheckoutAttached:
		result = ResultApplied
	case workspace.CheckoutExisting:
		result = ResultNoOp
	case workspace.CheckoutBlocked:
		result = ResultBlocked
	case workspace.CheckoutContended:
		result = ResultContended
	default:
		result = ResultInternalError
	}
	out := WorkspaceOpResult{
		ID:          id,
		Path:        res.Path,
		FeatureRef:  string(res.FeatureRef),
		BaseRef:     string(res.BaseRef),
		BaseCommit:  string(res.BaseCommit),
		Head:        string(res.HeadCommit),
		Disposition: string(res.Disposition),
		Reason:      res.Reason,
		Message:     res.Message,
	}
	if res.RemovedWorktree != "" {
		out.Findings = append(out.Findings, StatusFinding{
			Code:     FindingForeignWorktreeRemoved,
			Severity: string(domain.SeverityWarning),
			Path:     res.RemovedWorktree,
			Message:  "removed the clean worktree another tool made for this change's branch; ignored files in it, such as build output, went with it",
		})
	}
	if res.FastForwardedFrom != "" {
		out.Findings = append(out.Findings, StatusFinding{
			Code:     FindingLocalBranchFastForwarded,
			Severity: string(domain.SeverityWarning),
			Message:  fmt.Sprintf("fast-forwarded %s from %s to the pull request head %s", res.FeatureRef, res.FastForwardedFrom, res.HeadCommit),
		})
	}
	return newWorkspaceResult(opKey, result, out)
}
