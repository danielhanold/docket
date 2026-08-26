package app

import (
	"context"
	"fmt"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository"
)

// This file is the `finalize retarget-children` operation: before a stack parent
// merges, it moves each authorized open child PR off the parent's branch and onto
// the parent's own effective base, so the children never silently re-target
// themselves to the integration branch when the parent's branch is deleted. The
// GitHub mechanics (probe/act/verify each PR, the exact-version gate, the
// idempotent adoption of an already-retargeted PR) live in
// githubcli.RetargetPullRequest; this layer only WIRES them, adds no second
// PR-lookup policy, and executes NO metadata transaction.
//
// Two properties are load-bearing:
//
//   - The authorized set is exact. The children the human authorized travel in
//     from the context read as {id, pr_number, pr_version}. The current stack
//     graph is re-derived from domain.StackChildren over a freshly pinned
//     snapshot — never the parent's rendered artifact table — and every open child
//     found in the live graph must be named in the authorized set. A child open in
//     the live graph but absent from the authorization (a concurrently added
//     child), an ambiguous head (two open PRs), or a changed PR version is
//     `contended` and issues NO edit; a probe that cannot be established is
//     `unknown` (retain). None of those permit a parent merge to start.
//   - stacked_on is never touched. The operation opens no transaction and writes
//     no metadata: `stacked_on:` continues to describe where the child was
//     designed; ADR-0092 and the parent's later lifecycle determine the child's
//     next effective base.

// OperationFinalizeRetargetChildren is the operation key `finalize
// retarget-children` records in its result envelope.
const OperationFinalizeRetargetChildren = "finalize.retarget-children"

// The closed set of overall retarget dispositions. `retargeted` means every
// authorized open child now targets the parent's effective base (or already did)
// and no unauthorized/ambiguous/errored child was found — the only disposition
// under which a parent merge may start. `contended` is a lost race the caller
// resolves by re-reading context; `unknown` is an unestablished external probe
// the caller must reprobe.
const (
	RetargetDispositionRetargeted = "retargeted"
	RetargetDispositionContended  = "contended"
	RetargetDispositionUnknown    = "unknown"
)

// The closed set of per-child outcome tokens a result reports for each live child.
const (
	childOutcomeRetargeted     = "retargeted"       // moved onto the effective base and verified
	childOutcomeAlready        = "already"          // already at the effective base; no edit issued
	childOutcomeContended      = "contended"        // unauthorized-open, ambiguous head, or version drift
	childOutcomeUnknown        = "unknown"          // a probe could not establish the truth
	childOutcomeSkippedDone    = "skipped-terminal" // stacked-merged/done/killed: does not block, not edited
	childOutcomeSkippedNotOpen = "skipped-not-open" // non-terminal child with no open PR: does not block
)

// The stable machine reasons the operation reports for its typed refusals. Message
// text is explanatory and must not be parsed.
const (
	// ReasonRetargetUnknownChange / -AmbiguousID: the parent id names no record, or
	// more than one; the operation never chooses.
	ReasonRetargetUnknownChange = "unknown-change"
	ReasonRetargetAmbiguousID   = "ambiguous-change"
	// ReasonRetargetVersionDrift: the pinned parent record version no longer matches
	// the live record — the authorization was computed against a stale context;
	// maps to contended.
	ReasonRetargetVersionDrift = "version-drift"
	// ReasonRetargetRepositoryUnresolved: the GitHub repository identity could not
	// be resolved from the checkout; maps to external-failed.
	ReasonRetargetRepositoryUnresolved = "repository-unresolved"
	// ReasonRetargetBranchFactsUnresolved: the remote feature-branch facts the
	// effective-base resolution consults could not be read; maps to external-failed.
	ReasonRetargetBranchFactsUnresolved = "branch-facts-unresolved"
	// ReasonRetargetBaseUnresolved: the parent's effective base did not resolve to a
	// concrete branch, so there is no target to retarget onto; maps to
	// invalid-state.
	ReasonRetargetBaseUnresolved = "effective-base-unresolved"
	// ReasonRetargetNewChild: a child open in the live graph is absent from the
	// authorized set; maps to contended.
	ReasonRetargetNewChild = "unauthorized-open-child"
	// ReasonRetargetAmbiguousChildPR: a child head carries more than one open PR;
	// maps to contended.
	ReasonRetargetAmbiguousChildPR = "ambiguous-child-pr"
	// ReasonRetargetChildProbeFailed: a child PR probe could not be established;
	// maps to external-failed (unknown, retain).
	ReasonRetargetChildProbeFailed = "child-probe-failed"
	// ReasonRetargetChildContended / -ChildUnknown: the act/verify of a queued child
	// retarget came back contended / unknown.
	ReasonRetargetChildContended = "child-retarget-contended"
	ReasonRetargetChildUnknown   = "child-retarget-unknown"
)

// AuthorizedChild is one entry of the exact human-authorized child set carried in
// from the context read: the child change id, its live PR number, and the PR's
// opaque version the authorization was granted against.
type AuthorizedChild struct {
	ID        int    `json:"id"`
	PRNumber  int    `json:"pr_number"`
	PRVersion string `json:"pr_version"`
}

// RetargetChildrenRequest is the closed request. ID and Version pin the parent
// record the authorization was based on (its exact entity version); Children is
// the exact authorized set the human approved from the context read. The scalar
// identities ride on flags; the authorized set rides in a bounded request file.
type RetargetChildrenRequest struct {
	ID       int               `json:"id"`
	Version  string            `json:"version"`
	Children []AuthorizedChild `json:"children"`
}

// ChildRetargetOutcome is one live child's disposition: its id, the PR number
// acted on (when one was), the base it now targets (on a retarget), and the closed
// outcome token.
type ChildRetargetOutcome struct {
	ID       int    `json:"id"`
	PRNumber int    `json:"pr_number,omitempty"`
	Base     string `json:"base,omitempty"`
	Outcome  string `json:"outcome"`
}

// RetargetChildrenResult is the protocol-v1 document the operation returns. On a
// clean pass it names the parent id, the resolved effective base, the
// `retargeted` disposition, and every live child's outcome. A refusal carries a
// stable reason and message; a shape refusal carries findings. It holds no
// authored bytes.
type RetargetChildrenResult struct {
	Envelope
	ID          int                    `json:"id,omitempty"`
	Base        string                 `json:"base,omitempty"`
	Disposition string                 `json:"disposition,omitempty"`
	Children    []ChildRetargetOutcome `json:"children"`
	Reason      string                 `json:"reason,omitempty"`
	Message     string                 `json:"message,omitempty"`
	Findings    []StatusFinding        `json:"findings"`
}

// HumanText renders the one-line human summary. It names identity, disposition,
// base, and child counts only.
func (r RetargetChildrenResult) HumanText() string {
	if r.Result == ResultApplied || r.Result == ResultNoOp {
		return fmt.Sprintf("finalize retarget-children: change %04d %s onto %s — %d child outcome(s)",
			r.ID, r.Disposition, r.Base, len(r.Children))
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// newRetargetResult stamps the envelope and normalizes the collections so a nil
// never leaks into the protocol document.
func newRetargetResult(result Result, out RetargetChildrenResult) RetargetChildrenResult {
	out.Envelope = NewEnvelope(OperationFinalizeRetargetChildren, result)
	if out.Children == nil {
		out.Children = []ChildRetargetOutcome{}
	}
	if out.Findings == nil {
		out.Findings = []StatusFinding{}
	}
	return out
}

// retargetRefusal builds a refusing result carrying a stable reason and message
// (no per-child outcomes).
func retargetRefusal(result Result, reason, message string, id int) RetargetChildrenResult {
	return newRetargetResult(result, RetargetChildrenResult{ID: id, Reason: reason, Message: message})
}

// FinalizeRetargetChildren moves each authorized open child PR onto the parent's
// effective base. It is read-mostly: it pins one metadata revision, re-derives the
// live stack graph, validates the authorized set against it with zero edits, and
// only then probe/act/verifies each queued child through the idempotent GitHub
// adapter. It opens NO transaction and changes NO metadata (stacked_on included).
func FinalizeRetargetChildren(ctx context.Context, deps FinalizeDeps, repoDir string, req RetargetChildrenRequest) RetargetChildrenResult {
	if findings := validateRetargetShape(req); len(findings) > 0 {
		return newRetargetResult(ResultInvalidInput, RetargetChildrenResult{ID: req.ID, Findings: findings})
	}

	reader := deps.Planning.Reader
	pin, err := reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return retargetRefusal(result, reason, err.Error(), req.ID)
	}
	eff := pin.Config.Effective

	blobs, err := reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return retargetRefusal(result, reason, err.Error(), req.ID)
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		return retargetRefusal(ResultInternalError, ReasonStatusInternalError, err.Error(), req.ID)
	}
	snap := build.Snapshot

	// Resolve the parent record and gate on its exact pinned version: an id that
	// names no single record, or a record whose live version drifted from the
	// authorization, refuses before any external effect.
	parent, out := snap.Change(domain.ChangeID(req.ID))
	if out != domain.LookupFound {
		if out == domain.LookupAmbiguous {
			return retargetRefusal(ResultInvalidState, ReasonRetargetAmbiguousID,
				fmt.Sprintf("more than one record claims change id %04d; refusing to choose", req.ID), req.ID)
		}
		return retargetRefusal(ResultInvalidInput, ReasonRetargetUnknownChange,
			fmt.Sprintf("no change %04d is present in the corpus", req.ID), req.ID)
	}
	blobByPath := make(map[string]StatusBlob, len(blobs))
	for _, b := range blobs {
		blobByPath[b.Path] = b
	}
	if blobByPath[parent.Path()].Version != req.Version {
		return retargetRefusal(ResultContended, ReasonRetargetVersionDrift,
			fmt.Sprintf("change %04d record version moved under the authorization; re-read context finalize", req.ID), req.ID)
	}

	// Resolve the GitHub repository identity and the parent's own effective base
	// (the branch the children must be retargeted onto). The base facts are a
	// read-only remote probe; an unresolved base is a precondition refusal.
	repo, err := deps.GitHub.DiscoverRepository(ctx, repoDir)
	if err != nil {
		return retargetRefusal(ResultExternalFailed, ReasonRetargetRepositoryUnresolved, err.Error(), req.ID)
	}
	facts, err := reader.BranchFacts(ctx, pin, stackBranches(snap))
	if err != nil {
		return retargetRefusal(ResultExternalFailed, ReasonRetargetBranchFactsUnresolved, err.Error(), req.ID)
	}
	base := domain.ResolveEffectiveBase(snap, parent, facts)
	if base.Kind != domain.BaseResolved || base.Branch == "" {
		return retargetRefusal(ResultInvalidState, ReasonRetargetBaseUnresolved,
			fmt.Sprintf("change %04d effective base did not resolve (%s); nothing to retarget onto", req.ID, base.Kind), req.ID)
	}
	newBase := base.Branch

	authByID := make(map[domain.ChangeID]AuthorizedChild, len(req.Children))
	for _, ch := range req.Children {
		authByID[domain.ChangeID(ch.ID)] = ch
	}

	// Phase 1 — discover the live open-child set and validate it against the
	// authorization. This phase issues NO edit: a blocker returns immediately, so a
	// contended/unknown outcome leaves every child PR untouched.
	children := make([]ChildRetargetOutcome, 0)
	queue := make([]AuthorizedChild, 0, len(req.Children))
	for _, childID := range domain.StackChildren(snap, parent.ID()) {
		child, lookup := snap.Change(childID)
		if lookup != domain.LookupFound {
			continue
		}
		// A stacked-merged, done, or killed child does not gate the parent merge and
		// is never edited (spec "Open-child gate and retargeting").
		if childBlocksNothing(child.Status()) {
			children = append(children, ChildRetargetOutcome{ID: int(childID), Outcome: childOutcomeSkippedDone})
			continue
		}
		// Address the child by ITS OWN recorded branch (spec: "Stack parent and
		// child operations use each record's branch independently"), never a
		// slug-derived name. A child whose branch is unusable fails closed before any
		// probe or edit — no child PR is retargeted.
		head, berr := recordedBranch(child)
		if berr != nil {
			children = append(children, ChildRetargetOutcome{ID: int(childID), Outcome: childOutcomeUnknown})
			return newRetargetResult(ResultInvalidState, RetargetChildrenResult{
				ID: req.ID, Base: newBase, Disposition: RetargetDispositionUnknown,
				Children: children, Reason: berr.Error(),
				Message: fmt.Sprintf("child %04d's recorded feature branch is unusable (%v); no child PR is retargeted", int(childID), berr),
			})
		}
		prs, perr := deps.GitHub.FindOpenPullRequestsByHead(ctx, repo, head)
		if perr != nil {
			// A probe that cannot be established is unknown — retain, no edit, no merge.
			children = append(children, ChildRetargetOutcome{ID: int(childID), Outcome: childOutcomeUnknown})
			return newRetargetResult(ResultExternalFailed, RetargetChildrenResult{
				ID: req.ID, Base: newBase, Disposition: RetargetDispositionUnknown,
				Children: children, Reason: ReasonRetargetChildProbeFailed, Message: perr.Error(),
			})
		}
		switch {
		case len(prs) == 0:
			// A non-terminal child with no open PR poses no orphan hazard when the
			// parent branch is deleted; it does not block and there is nothing to edit.
			children = append(children, ChildRetargetOutcome{ID: int(childID), Outcome: childOutcomeSkippedNotOpen})
		case len(prs) == 1:
			auth, ok := authByID[childID]
			if !ok {
				// A child open in the live graph but absent from the authorized set is a
				// concurrently added child: contended, zero edits.
				children = append(children, ChildRetargetOutcome{ID: int(childID), PRNumber: prs[0].Number, Outcome: childOutcomeContended})
				return newRetargetResult(ResultContended, RetargetChildrenResult{
					ID: req.ID, Base: newBase, Disposition: RetargetDispositionContended,
					Children: children, Reason: ReasonRetargetNewChild,
					Message: fmt.Sprintf("child %04d has an open PR (#%d) that the authorization does not cover", int(childID), prs[0].Number),
				})
			}
			queue = append(queue, auth)
		default:
			// More than one open PR for one child head: the child PR is ambiguous, so no
			// single retarget target can be attributed. Contended, zero edits.
			children = append(children, ChildRetargetOutcome{ID: int(childID), Outcome: childOutcomeContended})
			return newRetargetResult(ResultContended, RetargetChildrenResult{
				ID: req.ID, Base: newBase, Disposition: RetargetDispositionContended,
				Children: children, Reason: ReasonRetargetAmbiguousChildPR,
				Message: fmt.Sprintf("child %04d head %q carries %d open PRs; refusing to choose", int(childID), head, len(prs)),
			})
		}
	}

	// Phase 2 — probe/act/verify each queued authorized child onto the effective
	// base. RetargetPullRequest is idempotent (an already-retargeted exact PR is
	// adopted as a no-op) and gates every edit on the exact PR version, so a changed
	// PR version comes back contended without an edit.
	worst := RetargetDispositionRetargeted
	edited := false
	var blockReason, blockMessage string
	blockResult := ResultApplied
	for _, auth := range queue {
		outcome, _, rerr := deps.GitHub.RetargetPullRequest(ctx, repo, auth.PRNumber, auth.PRVersion, newBase)
		switch outcome {
		case githubcli.RetargetRetargeted:
			edited = true
			children = append(children, ChildRetargetOutcome{ID: auth.ID, PRNumber: auth.PRNumber, Base: newBase, Outcome: childOutcomeRetargeted})
		case githubcli.RetargetAlready:
			children = append(children, ChildRetargetOutcome{ID: auth.ID, PRNumber: auth.PRNumber, Base: newBase, Outcome: childOutcomeAlready})
		case githubcli.RetargetContended:
			children = append(children, ChildRetargetOutcome{ID: auth.ID, PRNumber: auth.PRNumber, Outcome: childOutcomeContended})
			if worst != RetargetDispositionUnknown {
				worst = RetargetDispositionContended
				blockResult, blockReason = ResultContended, ReasonRetargetChildContended
				blockMessage = fmt.Sprintf("child %04d PR #%d version drifted; retarget refused without an edit", auth.ID, auth.PRNumber)
			}
		default: // RetargetUnknown
			children = append(children, ChildRetargetOutcome{ID: auth.ID, PRNumber: auth.PRNumber, Outcome: childOutcomeUnknown})
			worst = RetargetDispositionUnknown
			blockResult, blockReason = ResultExternalFailed, ReasonRetargetChildUnknown
			blockMessage = fmt.Sprintf("child %04d PR #%d retarget could not be established", auth.ID, auth.PRNumber)
			if rerr != nil {
				blockMessage = rerr.Error()
			}
		}
	}

	switch worst {
	case RetargetDispositionUnknown:
		return newRetargetResult(blockResult, RetargetChildrenResult{
			ID: req.ID, Base: newBase, Disposition: RetargetDispositionUnknown,
			Children: children, Reason: blockReason, Message: blockMessage,
		})
	case RetargetDispositionContended:
		return newRetargetResult(blockResult, RetargetChildrenResult{
			ID: req.ID, Base: newBase, Disposition: RetargetDispositionContended,
			Children: children, Reason: blockReason, Message: blockMessage,
		})
	}

	// Every authorized open child now targets the effective base. An actual edit is
	// `applied`; an all-adopt/all-skip pass is an idempotent no-op. Both clear the
	// open-child gate for a parent merge.
	result := ResultNoOp
	if edited {
		result = ResultApplied
	}
	return newRetargetResult(result, RetargetChildrenResult{
		ID: req.ID, Base: newBase, Disposition: RetargetDispositionRetargeted, Children: children,
	})
}

// childBlocksNothing reports whether a child in the given lifecycle neither gates
// the parent merge nor is a retarget target: a stacked-merged child's code has
// already landed in the parent, and a done or killed child is terminal (spec
// "a child already stacked-merged or done does not block").
func childBlocksNothing(s domain.Status) bool {
	return s == domain.StatusStackedMerged || s == domain.StatusDone || s == domain.StatusKilled
}

// validateRetargetShape runs the configuration-independent request checks that
// never reach any external seam: the pinned parent id/version, and each authorized
// child's id/pr_number/pr_version, with no duplicate child ids.
func validateRetargetShape(req RetargetChildrenRequest) []StatusFinding {
	findings := dropFindingCode(validateLifecycleShape(req.ID, "", req.Version), "empty-path")
	seen := make(map[int]bool, len(req.Children))
	for i, ch := range req.Children {
		if ch.ID <= 0 {
			findings = append(findings, lifecycleFinding("invalid-child_id",
				fmt.Sprintf("children[%d].id must be a positive change id", i)))
		}
		if ch.PRNumber <= 0 {
			findings = append(findings, lifecycleFinding("invalid-child_pr_number",
				fmt.Sprintf("children[%d].pr_number must be a positive pull-request number", i)))
		}
		if ch.PRVersion == "" {
			findings = append(findings, lifecycleFinding("empty-child_pr_version",
				fmt.Sprintf("children[%d].pr_version must be the exact PR version from context finalize", i)))
		}
		if ch.ID > 0 {
			if seen[ch.ID] {
				findings = append(findings, lifecycleFinding("duplicate-child_id",
					fmt.Sprintf("children names change %04d more than once", ch.ID)))
			}
			seen[ch.ID] = true
		}
	}
	return findings
}
