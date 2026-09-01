package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is `finalize block` and `finalize clear-block`: the durable,
// human-needed finalize-blocked state around an external PR comment plus an
// exact-version metadata transaction.
//
// `finalize block` records that a finalize attempt was blocked and a human must
// intervene. Its discipline is comment-first-then-marker:
//
//  1. It ensures exactly one owned PR comment carrying a Docket-owned attempt
//     marker and the bounded authored report (idempotent by that marker). The
//     comment is the FIRST external effect. A comment probe that cannot be
//     established is `unknown`: the operation returns without writing any marker,
//     never a marker claiming a comment that may not exist (Global Constraints:
//     unknown never shares a branch with a destructive/creative absence).
//  2. Only after the comment is present does it open ONE exact-version metadata
//     transaction that upserts a single "## Finalize blocked" section recording
//     the UTC date, the stable reason, the attempt identity, the verified
//     Git/GitHub facts, the comment URL, and the concrete remedy. A re-mark
//     appends a new attempt inside the one section; it never creates a second
//     heading (the section splice validates owned-heading uniqueness before any
//     rewrite). The inline board is rerendered atomically in the same
//     transaction.
//
// A crash between the comment and the marker replays cleanly: the comment ensure
// finds the owned marker and reuses it, and the marker transaction — keyed on
// whether the section already records THIS attempt (the promised state, not a
// local proxy) — either finishes or is a verified no-op.
//
// `finalize clear-block` is the only reader of the marker's removal: it reprobes
// an exact current head, valid local-gate evidence (unless the gate is off), a
// published remote feature ref, and a matching open PR before transactionally
// removing the section. Any missing conjunct refuses; the marker stays.
//
// Task 10's `finalize merge` reads the "## Finalize blocked" marker through a
// shape-keyed body check (changeHasFinalizeBlockedMarker / finalizeBlockedHeading
// in finalize_merge.go). This file WRITES that heading via finalizeBlockedSectionHeading,
// whose ATX text is exactly finalizeBlockedHeading, so the reader keeps working.

// The operation keys `finalize block` / `finalize clear-block` record in their
// result envelopes and transaction trailers.
const (
	OperationFinalizeBlock      = "finalize.block"
	OperationFinalizeClearBlock = "finalize.clear-block"
)

// finalizeBlockedSectionHeading is the full ATX H2 heading line the durable
// finalize-blocked section carries. Its heading text (everything after "## ") is
// exactly finalizeBlockedHeading (defined in finalize_merge.go), the text Task
// 10's shape-keyed reader matches — so writing this heading is what that reader
// detects.
const finalizeBlockedSectionHeading = "## " + finalizeBlockedHeading

// The closed set of `finalize block` / `finalize clear-block` dispositions.
const (
	// BlockDispRecorded: the comment is present and a marker attempt was written
	// (or refreshed) in the one section.
	BlockDispRecorded = "recorded"
	// BlockDispAlready: the comment is present and the section already records
	// this exact attempt; a verified no-op keyed on the promised state.
	BlockDispAlready = "already"
	// BlockDispCleared: `clear-block` reprobed every conjunct and removed the
	// section.
	BlockDispCleared = "cleared"
	// BlockDispNothingToClear: `clear-block` found no marker to remove; a no-op.
	BlockDispNothingToClear = "nothing-to-clear"
	// BlockDispUnknown: an external comment/PR/head probe could not be
	// established; retained, no marker written or removed.
	BlockDispUnknown = "unknown"
	// BlockDispContended: the exact-version transaction lost to a fresh
	// incompatible state.
	BlockDispContended = "contended"
	// BlockDispRefused: a retained precondition refusal (a clear-block conjunct
	// did not hold, or the change is not blockable).
	BlockDispRefused = "refused"
	// BlockDispFailed: a transaction failure; the cause is in the envelope's
	// failure field.
	BlockDispFailed = "failed"
)

// The stable machine reasons the block operations report for their typed
// outcomes. Message text is explanatory and must not be parsed.
const (
	// ReasonBlockUnknownChange: an id names no record in the corpus.
	ReasonBlockUnknownChange = "unknown-change"
	// ReasonBlockAmbiguousID: an id is claimed by more than one record.
	ReasonBlockAmbiguousID = "ambiguous-change"
	// ReasonBlockNotBlockable: the change is terminal — there is no finalize
	// attempt to block.
	ReasonBlockNotBlockable = "not-blockable"
	// ReasonBlockRepoUnresolved: the GitHub repository identity did not resolve.
	ReasonBlockRepoUnresolved = "repository-unresolved"
	// ReasonBlockCommentUnknown: the owned PR comment could not be established;
	// no marker is written.
	ReasonBlockCommentUnknown = "comment-probe-unknown"
	// ReasonBlockWorkspaceProbe: the feature workspace could not be inspected.
	ReasonBlockWorkspaceProbe = "workspace-probe-failed"
	// ReasonBlockRemoteFeatureProbe: the remote feature ref could not be observed.
	ReasonBlockRemoteFeatureProbe = "remote-feature-probe-failed"
	// ReasonBlockPRProbe: the live open-PR probe could not be established.
	ReasonBlockPRProbe = "pr-probe-failed"
	// ReasonClearHeadMismatch: the current feature head does not equal the
	// caller's expected head.
	ReasonClearHeadMismatch = "head-mismatch"
	// ReasonClearRemoteFeatureAbsent: the remote feature ref is absent; there is
	// no published head.
	ReasonClearRemoteFeatureAbsent = "remote-feature-absent"
	// ReasonClearRemoteHeadMismatch: the remote feature ref names a different
	// commit than the expected head.
	ReasonClearRemoteHeadMismatch = "remote-head-mismatch"
	// ReasonClearPRNotOpen: not exactly one open PR for the feature head.
	ReasonClearPRNotOpen = "pr-not-open"
	// ReasonClearEvidenceUnverified: the PR body evidence does not verify green
	// for the exact current head (and the gate is on).
	ReasonClearEvidenceUnverified = "evidence-unverified"
)

// BlockRequest is the closed request for `finalize block`. ID and Version pin
// the exact submitted record; PRNumber is the pull request the owned comment is
// ensured on; Attempt is the opaque owned attempt token that keys both the
// comment marker and the section's idempotency; Reason is the stable machine
// reason token; Head is the verified feature head recorded as a fact; Report is
// the authored bounded report that crosses to the PR comment; Remedy is the
// authored bounded concrete remedy recorded in the marker. The authored strings
// ride inside the JSON and never reach a shell or Git/gh argument.
type BlockRequest struct {
	ID       int    `json:"id"`
	Version  string `json:"version"`
	PRNumber int    `json:"pr_number"`
	Attempt  string `json:"attempt"`
	Reason   string `json:"reason"`
	Head     string `json:"head"`
	Report   string `json:"report"`
	Remedy   string `json:"remedy"`
}

// ClearBlockRequest is the closed request for `finalize clear-block`. ID and
// Version pin the exact submitted record; Head is the exact current feature head
// the reprobe must confirm; PRNumber is the canonical PR whose open state is
// reprobed.
type ClearBlockRequest struct {
	ID       int    `json:"id"`
	Version  string `json:"version"`
	Head     string `json:"head"`
	PRNumber int    `json:"pr_number"`
}

// BlockResult is the protocol-v1 document the block operations return. It names
// identity, the closed disposition, and — on a recorded block — the owned
// comment URL; a refusal carries a stable reason and message. It leaks no
// authored report or remedy bytes (redaction).
type BlockResult struct {
	Envelope
	ID          int             `json:"id,omitempty"`
	Disposition string          `json:"disposition,omitempty"`
	CommentURL  string          `json:"comment_url,omitempty"`
	Revision    string          `json:"committed_revision,omitempty"`
	Reason      string          `json:"reason,omitempty"`
	Message     string          `json:"message,omitempty"`
	Findings    []StatusFinding `json:"findings"`
}

// HumanText renders a one-line summary naming identity and disposition only —
// never the authored report or remedy body.
func (r BlockResult) HumanText() string {
	if r.Result == ResultApplied || r.Result == ResultNoOp {
		s := fmt.Sprintf("%s: change %04d %s", r.Operation, r.ID, r.Disposition)
		if r.CommentURL != "" {
			s += " (comment recorded)"
		}
		return s
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// newBlockResult stamps the envelope for the named block operation and
// normalizes the findings collection so a nil never leaks into the document.
func newBlockResult(op string, result Result, out BlockResult) BlockResult {
	out.Envelope = NewEnvelope(op, result)
	if out.Findings == nil {
		out.Findings = []StatusFinding{}
	}
	return out
}

// blockRefusal builds a refusing result carrying a stable reason, message, and
// disposition for the named block operation.
func blockRefusal(op string, result Result, disposition, reason, message string, id int) BlockResult {
	return newBlockResult(op, result, BlockResult{
		ID: id, Disposition: disposition, Reason: reason, Message: message,
	})
}

// blockReceipt is the canonical receipt persisted with a block/clear commit.
// Field order is alphabetical so json.Marshal emits the canonical sorted-key
// compact form the engine's receipt validator requires.
type blockReceipt struct {
	ID int    `json:"id"`
	Op string `json:"op"`
}

// FinalizeBlock ensures the owned PR comment first, then upserts the single
// durable "## Finalize blocked" section in one exact-version transaction. A
// comment that cannot be established is unknown and writes no marker.
func FinalizeBlock(ctx context.Context, deps FinalizeDeps, repoDir string, req BlockRequest) BlockResult {
	if findings := validateBlockShape(req); len(findings) > 0 {
		return newBlockResult(OperationFinalizeBlock, ResultInvalidInput, BlockResult{ID: req.ID, Findings: findings})
	}

	reader := deps.Planning.Reader
	pin, err := reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return blockRefusal(OperationFinalizeBlock, result, BlockDispRefused, reason, err.Error(), req.ID)
	}
	// Capability preflight before any external effect.
	if decision := config.PreflightMutation(&pin.Config); !decision.Allowed {
		return newBlockResult(OperationFinalizeBlock, ResultUnsupportedConfig, BlockResult{
			ID: req.ID, Reason: ReasonDeferredCapRequested,
			Message: "configuration actively requests a deferred capability docket does not ship in this version (" +
				strings.Join(blockerPaths(decision.Blockers), ", ") + "); withdraw it before any mutation",
		})
	}
	eff := pin.Config.Effective

	inline, err := fenceBoardSurface(eff)
	if err != nil {
		return blockPlanningError(OperationFinalizeBlock, err, req.ID)
	}

	blobs, err := reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return blockRefusal(OperationFinalizeBlock, result, BlockDispRefused, reason, err.Error(), req.ID)
	}
	recPath, refusal := resolveBlockTarget(OperationFinalizeBlock, eff, blobs, req.ID, false)
	if refusal != nil {
		return *refusal
	}

	// Ensure the owned PR comment FIRST. It is idempotent by its attempt marker;
	// an unresolved probe is unknown and writes no marker.
	ghRepo, err := deps.GitHub.DiscoverRepository(ctx, repoDir)
	if err != nil {
		return newBlockResult(OperationFinalizeBlock, ResultExternalFailed, BlockResult{
			ID: req.ID, Disposition: BlockDispUnknown, Reason: ReasonBlockRepoUnresolved, Message: err.Error(),
		})
	}
	marker := finalizeBlockedCommentMarker(req.Attempt)
	commentBody := marker + "\n\n" + strings.TrimRight(req.Report, "\r\n") + "\n"
	outcome, url, cerr := deps.GitHub.EnsureComment(ctx, ghRepo, req.PRNumber, marker, commentBody)
	if cerr != nil {
		return newBlockResult(OperationFinalizeBlock, ResultExternalFailed, BlockResult{
			ID: req.ID, Disposition: BlockDispUnknown, Reason: ReasonBlockCommentUnknown, Message: cerr.Error(),
		})
	}
	if outcome == githubcli.CommentUnknown {
		return newBlockResult(OperationFinalizeBlock, ResultExternalFailed, BlockResult{
			ID: req.ID, Disposition: BlockDispUnknown, Reason: ReasonBlockCommentUnknown,
			Message: "the owned pull-request comment could not be established; no finalize-blocked marker was written",
		})
	}

	// The comment is present. Open the exact-version transaction that upserts the
	// single marker section.
	repo, err := deps.Planning.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return blockRefusal(OperationFinalizeBlock, result, BlockDispRefused, reason, err.Error(), req.ID)
	}

	op := finalizeBlockOp{
		req:        req,
		commentURL: url,
		eff:        eff,
		clock:      deps.Planning.Clock,
		inline:     inline,
		changesDir: eff.ChangesDir.Value,
	}
	res, execErr := deps.Planning.Engine.Execute(ctx, transaction.Request{
		Repository: repo,
		Remote:     originRemote,
		TargetRef:  gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName),
		Expected: []transaction.EntityExpectation{{
			Path:    gitcli.RepoPath(recPath),
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(req.Version)},
		}},
		Loader:    newPlanningLoader(eff),
		Operation: op,
	})
	return blockResultFromOutcome(OperationFinalizeBlock, res, execErr, url)
}

// FinalizeClearBlock reprobes an exact current head, valid gate evidence (unless
// the gate is off), a published remote feature ref, and a matching open PR, then
// removes the single "## Finalize blocked" section in one exact-version
// transaction. Any missing conjunct refuses; the marker stays.
func FinalizeClearBlock(ctx context.Context, deps FinalizeDeps, repoDir string, req ClearBlockRequest) BlockResult {
	if findings := validateClearBlockShape(req); len(findings) > 0 {
		return newBlockResult(OperationFinalizeClearBlock, ResultInvalidInput, BlockResult{ID: req.ID, Findings: findings})
	}

	reader := deps.Planning.Reader
	pin, err := reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return blockRefusal(OperationFinalizeClearBlock, result, BlockDispRefused, reason, err.Error(), req.ID)
	}
	if decision := config.PreflightMutation(&pin.Config); !decision.Allowed {
		return newBlockResult(OperationFinalizeClearBlock, ResultUnsupportedConfig, BlockResult{
			ID: req.ID, Reason: ReasonDeferredCapRequested,
			Message: "configuration actively requests a deferred capability docket does not ship in this version (" +
				strings.Join(blockerPaths(decision.Blockers), ", ") + "); withdraw it before any mutation",
		})
	}
	eff := pin.Config.Effective

	inline, err := fenceBoardSurface(eff)
	if err != nil {
		return blockPlanningError(OperationFinalizeClearBlock, err, req.ID)
	}

	blobs, err := reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return blockRefusal(OperationFinalizeClearBlock, result, BlockDispRefused, reason, err.Error(), req.ID)
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		return newBlockResult(OperationFinalizeClearBlock, ResultInternalError, BlockResult{
			ID: req.ID, Reason: ReasonStatusInternalError, Message: err.Error(),
		})
	}
	snap := build.Snapshot
	c, out := snap.Change(domain.ChangeID(req.ID))
	if refusal := blockLookupRefusal(OperationFinalizeClearBlock, out, req.ID); refusal != nil {
		return *refusal
	}
	recPath := c.Path()

	// Reprobe the four removal conjuncts against fresh live facts before any
	// mutation. Each unresolved external probe is unknown (retain); each cleanly
	// missing conjunct refuses and leaves the marker.
	facts, err := reader.BranchFacts(ctx, pin, stackBranches(snap))
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return blockRefusal(OperationFinalizeClearBlock, result, BlockDispRefused, reason, err.Error(), req.ID)
	}
	base := domain.ResolveEffectiveBase(snap, c, facts)
	if base.Kind != domain.BaseResolved {
		return blockRefusal(OperationFinalizeClearBlock, ResultInvalidState, BlockDispRefused,
			ReasonMergeUnresolvedBase, fmt.Sprintf("change %04d's effective base did not resolve to a branch", req.ID), req.ID)
	}
	branch, berr := recordedBranch(c)
	if berr != nil {
		return blockRefusal(OperationFinalizeClearBlock, ResultInvalidState, BlockDispRefused,
			berr.Error(), fmt.Sprintf("change %04d's recorded feature branch is unusable (%v); the marker stays", req.ID, berr), req.ID)
	}
	target, terr := workspace.NewTarget(c.ID(), c.Slug(), base, branch)
	if terr != nil {
		return blockRefusal(OperationFinalizeClearBlock, ResultInvalidInput, BlockDispRefused,
			ReasonMergeMalformedTarget, terr.Error(), req.ID)
	}

	repo, err := deps.Planning.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return blockRefusal(OperationFinalizeClearBlock, result, BlockDispRefused, reason, err.Error(), req.ID)
	}
	ghRepo, err := deps.GitHub.DiscoverRepository(ctx, repoDir)
	if err != nil {
		return newBlockResult(OperationFinalizeClearBlock, ResultExternalFailed, BlockResult{
			ID: req.ID, Disposition: BlockDispUnknown, Reason: ReasonBlockRepoUnresolved, Message: err.Error(),
		})
	}

	// Current head: the workspace's local head must equal the caller's expected
	// head.
	insp, err := deps.Workspace.Inspect(ctx, workspace.InspectRequest{Repository: repo, Target: target})
	if err != nil {
		return newBlockResult(OperationFinalizeClearBlock, ResultExternalFailed, BlockResult{
			ID: req.ID, Disposition: BlockDispUnknown, Reason: ReasonBlockWorkspaceProbe, Message: err.Error(),
		})
	}
	if string(insp.HeadCommit) != req.Head {
		return blockRefusal(OperationFinalizeClearBlock, ResultBlocked, BlockDispRefused,
			ReasonClearHeadMismatch, "the current feature head does not equal the expected head; the marker stays", req.ID)
	}

	// Published remote feature ref: present and naming the expected head.
	rref, err := deps.Planning.Client.ProbeRemoteBranch(ctx, repo, originRemote, target.FeatureRef)
	if err != nil {
		return newBlockResult(OperationFinalizeClearBlock, ResultExternalFailed, BlockResult{
			ID: req.ID, Disposition: BlockDispUnknown, Reason: ReasonBlockRemoteFeatureProbe, Message: err.Error(),
		})
	}
	if rref.State != gitcli.RemoteRefFound {
		return blockRefusal(OperationFinalizeClearBlock, ResultBlocked, BlockDispRefused,
			ReasonClearRemoteFeatureAbsent, "the remote feature ref is absent; the marker stays", req.ID)
	}
	if string(rref.Commit) != req.Head {
		return blockRefusal(OperationFinalizeClearBlock, ResultBlocked, BlockDispRefused,
			ReasonClearRemoteHeadMismatch, "the remote feature ref names a commit other than the expected head; the marker stays", req.ID)
	}

	// Matching open PR at the exact head, and (unless the gate is off) green body
	// evidence for that head.
	featureBranch := strings.TrimPrefix(string(target.FeatureRef), branchRefPrefix)
	prs, err := deps.GitHub.FindOpenPullRequestsByHead(ctx, ghRepo, featureBranch)
	if err != nil {
		return newBlockResult(OperationFinalizeClearBlock, ResultExternalFailed, BlockResult{
			ID: req.ID, Disposition: BlockDispUnknown, Reason: ReasonBlockPRProbe, Message: err.Error(),
		})
	}
	if len(prs) != 1 || prs[0].Number != req.PRNumber {
		return blockRefusal(OperationFinalizeClearBlock, ResultBlocked, BlockDispRefused,
			ReasonClearPRNotOpen, "there is not exactly one matching open pull request for the feature head; the marker stays", req.ID)
	}
	if eff.Finalize.Gate.Value != "off" {
		evHead, _, evGreen := prBodyEvidence(prs[0])
		if !evGreen || evHead != req.Head {
			return blockRefusal(OperationFinalizeClearBlock, ResultBlocked, BlockDispRefused,
				ReasonClearEvidenceUnverified, "the pull-request body evidence does not verify green for the exact current head; the marker stays", req.ID)
		}
	}

	op := finalizeClearBlockOp{
		id:         req.ID,
		eff:        eff,
		inline:     inline,
		changesDir: eff.ChangesDir.Value,
	}
	res, execErr := deps.Planning.Engine.Execute(ctx, transaction.Request{
		Repository: repo,
		Remote:     originRemote,
		TargetRef:  gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName),
		Expected: []transaction.EntityExpectation{{
			Path:    gitcli.RepoPath(recPath),
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(req.Version)},
		}},
		Loader:    newPlanningLoader(eff),
		Operation: op,
	})
	return clearBlockResultFromOutcome(res, execErr)
}

// resolveBlockTarget resolves the change record's canonical path from the pinned
// corpus, refusing an unknown/ambiguous id or a terminal (non-blockable) change.
// allowTerminal is false for `finalize block`. The authoritative blockability
// gate re-runs inside the transaction on fresh state; this pre-read is a
// supporting observation that resolves the exact target path.
func resolveBlockTarget(op string, eff config.Effective, blobs []StatusBlob, id int, allowTerminal bool) (string, *BlockResult) {
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		r := newBlockResult(op, ResultInternalError, BlockResult{ID: id, Reason: ReasonStatusInternalError, Message: err.Error()})
		return "", &r
	}
	snap := build.Snapshot
	c, out := snap.Change(domain.ChangeID(id))
	if refusal := blockLookupRefusal(op, out, id); refusal != nil {
		return "", refusal
	}
	if !allowTerminal && c.Status().Terminal() {
		r := blockRefusal(op, ResultInvalidState, BlockDispRefused, ReasonBlockNotBlockable,
			fmt.Sprintf("change %04d is terminal; there is no finalize attempt to block", id), id)
		return "", &r
	}
	return c.Path(), nil
}

// blockLookupRefusal folds a change lookup outcome into a typed block refusal.
func blockLookupRefusal(op string, out domain.LookupOutcome, id int) *BlockResult {
	switch out {
	case domain.LookupAbsent:
		r := blockRefusal(op, ResultInvalidInput, BlockDispRefused, ReasonBlockUnknownChange,
			fmt.Sprintf("no change %04d is present in the corpus", id), id)
		return &r
	case domain.LookupAmbiguous:
		r := blockRefusal(op, ResultInvalidState, BlockDispRefused, ReasonBlockAmbiguousID,
			fmt.Sprintf("more than one record claims change id %04d; refusing to choose", id), id)
		return &r
	}
	return nil
}

// blockPlanningError folds a board-surface fence planning error into a block
// result.
func blockPlanningError(op string, err error, id int) BlockResult {
	if pe, ok := asPlanningError(err); ok {
		return blockRefusal(op, pe.Result, BlockDispRefused, pe.Reason, pe.Message, id)
	}
	return blockRefusal(op, ResultInternalError, BlockDispRefused, ReasonStatusInternalError, err.Error(), id)
}

// blockResultFromOutcome folds a `finalize block` transaction outcome into the
// result document. An empty-plan no-op (the attempt was already recorded) is the
// idempotent already disposition; a refusal maps to blocked/contended.
func blockResultFromOutcome(op string, res transaction.Result, execErr error, url string) BlockResult {
	result, _ := mapOutcome(res, execErr, ResultBlocked)
	out := BlockResult{ID: 0, CommentURL: url, Findings: findingsToStatus(res.Findings)}
	if rec, ok := decodeBlockReceipt(res.Receipt); ok {
		out.ID = rec.ID
	}
	switch {
	case res.Disposition == transaction.DispositionFailed:
		out.Disposition = BlockDispFailed
	case result == ResultApplied:
		out.Disposition = BlockDispRecorded
		out.Revision = string(res.AppliedCommit)
	case result == ResultNoOp:
		out.Disposition = BlockDispAlready
	case result == ResultContended:
		out.Disposition = BlockDispContended
	default:
		out.Disposition = BlockDispRefused
	}
	r := newBlockResult(op, result, out)
	r.Failure = failureStatus(res, execErr)
	return r
}

// clearBlockResultFromOutcome folds a `finalize clear-block` transaction outcome
// into the result document.
func clearBlockResultFromOutcome(res transaction.Result, execErr error) BlockResult {
	result, _ := mapOutcome(res, execErr, ResultBlocked)
	out := BlockResult{Findings: findingsToStatus(res.Findings)}
	if rec, ok := decodeBlockReceipt(res.Receipt); ok {
		out.ID = rec.ID
	}
	switch {
	case res.Disposition == transaction.DispositionFailed:
		out.Disposition = BlockDispFailed
	case result == ResultApplied:
		out.Disposition = BlockDispCleared
		out.Revision = string(res.AppliedCommit)
	case result == ResultNoOp:
		out.Disposition = BlockDispNothingToClear
	case result == ResultContended:
		out.Disposition = BlockDispContended
	default:
		out.Disposition = BlockDispRefused
	}
	r := newBlockResult(OperationFinalizeClearBlock, result, out)
	r.Failure = failureStatus(res, execErr)
	return r
}

// decodeBlockReceipt decodes a persisted block/clear receipt.
func decodeBlockReceipt(b []byte) (blockReceipt, bool) {
	if len(b) == 0 {
		return blockReceipt{}, false
	}
	var rec blockReceipt
	if err := json.Unmarshal(b, &rec); err != nil {
		return blockReceipt{}, false
	}
	return rec, true
}

// validateBlockShape runs the configuration-independent request checks for
// `finalize block` that never reach the engine.
func validateBlockShape(req BlockRequest) []StatusFinding {
	findings := dropFindingCode(validateLifecycleShape(req.ID, "", req.Version), "empty-path")
	add := func(code, msg string) { findings = append(findings, lifecycleFinding(code, msg)) }
	if req.PRNumber <= 0 {
		add("invalid-pr_number", "pr_number must be a positive pull-request number")
	}
	if !validAttemptToken(req.Attempt) {
		add("invalid-attempt", "attempt must be a non-empty token of letters, digits, '.', '_', or '-'")
	}
	if strings.TrimSpace(req.Reason) == "" {
		add("empty-reason", "reason must be a non-empty stable reason token")
	}
	if strings.TrimSpace(req.Head) == "" {
		add("empty-head", "head must name the verified feature head")
	}
	if strings.TrimSpace(req.Report) == "" {
		add("empty-report", "report must be a non-empty authored bounded report")
	}
	boundAuthored(&findings, "report", req.Report)
	boundAuthored(&findings, "remedy", req.Remedy)
	return findings
}

// validateClearBlockShape runs the configuration-independent request checks for
// `finalize clear-block`.
func validateClearBlockShape(req ClearBlockRequest) []StatusFinding {
	findings := dropFindingCode(validateLifecycleShape(req.ID, "", req.Version), "empty-path")
	add := func(code, msg string) { findings = append(findings, lifecycleFinding(code, msg)) }
	if strings.TrimSpace(req.Head) == "" {
		add("empty-head", "head must name the exact current feature head to reprobe")
	}
	if req.PRNumber <= 0 {
		add("invalid-pr_number", "pr_number must be a positive pull-request number")
	}
	return findings
}

// validAttemptToken reports whether s is a safe opaque attempt token: non-empty
// and composed only of letters, digits, '.', '_', or '-'. It bounds the token
// so it can be embedded in a marker line without becoming an injection vector.
func validAttemptToken(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}

// finalizeBlockedCommentMarker is the Docket-owned attempt marker line the PR
// comment body begins with. EnsureComment keys idempotency on this exact line,
// so it is deterministic in the attempt token alone.
func finalizeBlockedCommentMarker(attempt string) string {
	return "<!-- docket:finalize-blocked attempt:" + attempt + " -->"
}

// finalizeBlockedAttemptMarker is the machine marker embedded in each attempt
// entry inside the "## Finalize blocked" section. The section-upsert idempotency
// probe keys on this exact line (the promised state — this attempt is recorded —
// not a byte proxy).
func finalizeBlockedAttemptMarker(attempt string) string {
	return "<!-- attempt:" + attempt + " -->"
}

// finalizeBlockOp is the SemanticOperation the engine drives for `finalize
// block`. Every field is fixed before the transaction; the state-dependent work
// (the fresh lookup, the section upsert keyed on the attempt, the inline board)
// re-runs from the attempt's own fresh state.
type finalizeBlockOp struct {
	req        BlockRequest
	commentURL string
	eff        config.Effective
	clock      transaction.Clock
	inline     bool
	changesDir string
}

func (o finalizeBlockOp) Key() transaction.OperationKey {
	return transaction.OperationKey(OperationFinalizeBlock)
}

// Plan upserts the single "## Finalize blocked" section on the attempt's fresh
// record. If the section already records THIS attempt, it plans no record
// mutation (an idempotent no-op keyed on the promised state). Otherwise it
// appends the attempt inside the one section (never a second heading) and
// rerenders the inline board atomically.
func (o finalizeBlockOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot
	c, out := snap.Change(domain.ChangeID(o.req.ID))
	if out != domain.LookupFound {
		return refuseBlock("not-found", fmt.Sprintf("change %04d is not present in the current corpus", o.req.ID))
	}
	if c.Status().Terminal() {
		return refuseBlock(ReasonBlockNotBlockable,
			fmt.Sprintf("change %04d is terminal; there is no finalize attempt to block", o.req.ID))
	}
	src, ok := st.State.Sources[c.Path()]
	if !ok {
		return refuseBlock("path-mismatch", fmt.Sprintf("no record source loaded at %q for change %04d", c.Path(), o.req.ID))
	}

	oldBody, present, err := namedSectionBody(src, finalizeBlockedSectionHeading)
	if err != nil {
		return refuseBlock("marker-scan-failed", err.Error())
	}
	// Idempotency keyed on the promised state: this attempt already recorded.
	if present && strings.Contains(oldBody, finalizeBlockedAttemptMarker(o.req.Attempt)) {
		return transaction.MutationPlan{}, transaction.OperationResult{}, nil
	}

	entry := o.blockedEntry()
	newBody := entry
	if present {
		if trimmed := strings.Trim(oldBody, "\r\n"); trimmed != "" {
			newBody = trimmed + "\n\n" + entry
		}
	}
	edited, err := render.ApplySectionEdits(src, []string{finalizeBlockedSectionHeading},
		[]render.SectionEdit{{Heading: finalizeBlockedSectionHeading, Intent: render.SectionReplace, Markdown: newBody}})
	if err != nil {
		return refuseBlock("marker-edit-failed", err.Error())
	}

	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: edited},
	}
	files, err = planInlineBoard(ctx, st, snap, o.inline, o.changesDir, boardPresentation(o.eff), files)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}

	receipt, err := json.Marshal(blockReceipt{ID: o.req.ID, Op: OperationFinalizeBlock})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("finalize block: encoding receipt: %w", err)
	}
	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d finalize blocked", o.req.ID),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}

// blockedEntry renders one attempt entry recorded inside the "## Finalize
// blocked" section: a dated sub-heading, the machine attempt marker (the
// idempotency key), and the verified facts, comment URL, and concrete remedy.
func (o finalizeBlockOp) blockedEntry() string {
	date := o.clock.Now().UTC().Format("2006-01-02")
	var b strings.Builder
	fmt.Fprintf(&b, "### %s — attempt %s\n\n", date, o.req.Attempt)
	b.WriteString(finalizeBlockedAttemptMarker(o.req.Attempt) + "\n\n")
	fmt.Fprintf(&b, "- Reason: %s\n", o.req.Reason)
	fmt.Fprintf(&b, "- Head: %s\n", o.req.Head)
	fmt.Fprintf(&b, "- PR: #%d\n", o.req.PRNumber)
	if o.commentURL != "" {
		fmt.Fprintf(&b, "- Comment: %s\n", o.commentURL)
	}
	if remedy := strings.TrimRight(o.req.Remedy, "\r\n"); remedy != "" {
		fmt.Fprintf(&b, "\nRemedy: %s\n", remedy)
	}
	return b.String()
}

// finalizeClearBlockOp is the SemanticOperation the engine drives for `finalize
// clear-block`: it removes the single "## Finalize blocked" section from the
// attempt's fresh record and rerenders the inline board. A record with no marker
// is an empty-plan no-op.
type finalizeClearBlockOp struct {
	id         int
	eff        config.Effective
	inline     bool
	changesDir string
}

func (o finalizeClearBlockOp) Key() transaction.OperationKey {
	return transaction.OperationKey(OperationFinalizeClearBlock)
}

func (o finalizeClearBlockOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot
	c, out := snap.Change(domain.ChangeID(o.id))
	if out != domain.LookupFound {
		return refuseBlock("not-found", fmt.Sprintf("change %04d is not present in the current corpus", o.id))
	}
	src, ok := st.State.Sources[c.Path()]
	if !ok {
		return refuseBlock("path-mismatch", fmt.Sprintf("no record source loaded at %q for change %04d", c.Path(), o.id))
	}
	if !namedSectionPresent(src, finalizeBlockedSectionHeading) {
		// Nothing to remove: an empty-plan no-op.
		return transaction.MutationPlan{}, transaction.OperationResult{}, nil
	}
	edited, err := render.ApplySectionEdits(src, []string{finalizeBlockedSectionHeading},
		[]render.SectionEdit{{Heading: finalizeBlockedSectionHeading, Intent: render.SectionRemove}})
	if err != nil {
		return refuseBlock("marker-remove-failed", err.Error())
	}
	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: edited},
	}
	files, err = planInlineBoard(ctx, st, snap, o.inline, o.changesDir, boardPresentation(o.eff), files)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	receipt, err := json.Marshal(blockReceipt{ID: o.id, Op: OperationFinalizeClearBlock})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("finalize clear-block: encoding receipt: %w", err)
	}
	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d finalize block cleared", o.id),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}

// refuseBlock builds a refusing (plan, OperationResult) pair carrying one
// state-shaped finding for the block operations' Plan closures.
func refuseBlock(code, msg string) (transaction.MutationPlan, transaction.OperationResult, error) {
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

// planInlineBoard appends the inline board mutation to files when inline board
// rendering is enabled AND the freshly rendered board differs from the committed
// one. The board renders from the current snapshot: the marker/halt/claim edits
// these operations make are not board-visible, so the render is normally
// byte-identical and no board mutation is declared — but the render still runs
// inside the same transaction, so any board drift is corrected atomically with
// the record edit (the engine refuses a declared path that is not an actual
// change, so the mutation is declared only when it truly changes the tree).
func planInlineBoard(ctx context.Context, st transaction.AttemptState, snap domain.Snapshot, inline bool, changesDir string, pres render.BoardPresentation, files []transaction.FileMutation) ([]transaction.FileMutation, error) {
	if !inline {
		return files, nil
	}
	boardPath := path.Join(changesDir, "BOARD.md")
	if err := includeBoard(ctx, st.Tree, boardPath, snap, pres, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// namedSectionPresent reports whether src carries a top-level ATX section whose
// heading line is exactly heading, scanned fence-aware (a heading-shaped line in
// a fenced code block is authored content). It keys on shape, never an
// enumerated interior spelling.
func namedSectionPresent(src []byte, heading string) bool {
	for _, h := range scanTopHeadings(src) {
		if h.heading == heading {
			return true
		}
	}
	return false
}

// namedSectionBody returns the body of the single section whose heading line is
// exactly heading — the bytes between its heading line and the NAMED terminator
// (the next top-level heading, or EOF) — and whether the section is present. A
// duplicate heading yields no single body and is an error (the whole-record
// splice refuses it too, but the slice must not pick one silently).
func namedSectionBody(src []byte, heading string) (body string, present bool, err error) {
	heads := scanTopHeadings(src)
	idx := -1
	for i, h := range heads {
		if h.heading == heading {
			if idx >= 0 {
				return "", false, fmt.Errorf("heading %q appears more than once; the section slice has no single terminator", heading)
			}
			idx = i
		}
	}
	if idx < 0 {
		return "", false, nil
	}
	bodyStart := heads[idx].lineEnd
	bodyEnd := len(src)
	if idx+1 < len(heads) {
		bodyEnd = heads[idx+1].start
	}
	return string(src[bodyStart:bodyEnd]), true, nil
}
