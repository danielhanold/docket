package app

import (
	"context"
	"fmt"

	"github.com/danielhanold/docket/internal/gitcli"
)

// OperationRepositorySyncIntegration is the operation key `repository
// sync-integration` records in its result envelope and the id the capability
// catalog and schema registry join on.
const OperationRepositorySyncIntegration = "repository.sync-integration"

// The closed integration-sync disposition vocabulary (change 0388). advanced is
// the one mutating success (the primary checkout was fast-forwarded to the
// freshly fetched integration tip); already-current is a successful no-op;
// skipped is a deliberate safety decline that leaves the tree untouched;
// refused is an invalid input/config/state the loader classified before any
// observation; failed is unobservable or failed Git work. Human messages are
// explanatory, never decision inputs — every outcome carries exactly one of
// these tokens. This is a prefixed const group (SyncDisp*) so
// TestVocabularyConstCompleteness holds the emitted `sync_dispositions`
// vocabulary in correspondence with it.
const (
	SyncDispAdvanced       = "advanced"
	SyncDispAlreadyCurrent = "already-current"
	SyncDispSkipped        = "skipped"
	SyncDispRefused        = "refused"
	SyncDispFailed         = "failed"
)

// The stable reasons a skip or a failure reports. Message text is explanatory
// and must not be parsed. Discovery/configuration/fetch failures reuse the
// loader's existing classifyStatusError reasons rather than minting parallel
// spellings; these cover only the sync ladder's own decisions.
const (
	// Skip reasons — a deliberate safety decline, tree untouched.
	ReasonSyncDirtyWorktree       = "dirty-worktree"
	ReasonSyncDetachedHead        = "detached-head"
	ReasonSyncOtherBranch         = "other-branch"
	ReasonSyncOperationInProgress = "operation-in-progress"
	ReasonSyncLocalAhead          = "local-ahead"
	ReasonSyncDiverged            = "diverged"
	ReasonSyncCheckoutChanged     = "checkout-changed"
	// Failure reasons — unobservable or failed Git work.
	ReasonSyncStateProbeFailed    = "state-probe-failed"
	ReasonSyncAncestryProbeFailed = "ancestry-probe-failed"
	ReasonSyncUpdateFailed        = "update-failed"
	ReasonSyncPostcheckUnverified = "postcheck-unverified"
)

// SyncOutcome is the structured integration-sync outcome: disposition, reason,
// the primary checkout it decided about, and the object ids it knew. It is
// embedded by RepositorySyncResult and carried additively by MaintenanceResult;
// human messages are explanatory, never decision inputs.
type SyncOutcome struct {
	Disposition       string `json:"disposition" docket:"enum=sync_dispositions"`
	Reason            string `json:"reason,omitempty"`
	Message           string `json:"message,omitempty"`
	PrimaryPath       string `json:"primary_path,omitempty"`
	IntegrationBranch string `json:"integration_branch,omitempty"`
	BeforeOID         string `json:"before_oid,omitempty"`
	TargetOID         string `json:"target_oid,omitempty"`
	AfterOID          string `json:"after_oid,omitempty"`
}

// RepositorySyncResult is the protocol-v1 document `repository sync-integration`
// returns.
type RepositorySyncResult struct {
	Envelope
	SyncOutcome
}

// syncDirtyMessage is the one dirty-skip human string every dirty path shares.
// It explicitly names non-ignored untracked files as a blocker and tells the
// user to inspect and resolve the dirt before retrying.
func syncDirtyMessage() string {
	return "primary checkout has uncommitted changes (non-ignored untracked files also block sync); inspect and resolve the dirt, then rerun"
}

// syncShortOID renders an object id in its short form for human text; an empty
// or already-short id is returned unchanged.
func syncShortOID(oid string) string {
	const shortLen = 12
	if len(oid) > shortLen {
		return oid[:shortLen]
	}
	return oid
}

// HumanText renders the one-line summary. An advance names the branch, the
// before..after object ids, and the primary checkout; every other disposition
// names the disposition, its reason, and the explanatory message.
func (r RepositorySyncResult) HumanText() string {
	if r.Disposition == SyncDispAdvanced {
		return fmt.Sprintf("%s: advanced %s %s..%s at %s",
			OperationRepositorySyncIntegration, r.IntegrationBranch,
			syncShortOID(r.BeforeOID), syncShortOID(r.AfterOID), r.PrimaryPath)
	}
	out := fmt.Sprintf("%s: %s", OperationRepositorySyncIntegration, r.Disposition)
	if r.Reason != "" {
		out += fmt.Sprintf(" (%s)", r.Reason)
	}
	if r.Message != "" {
		out += fmt.Sprintf(" — %s", r.Message)
	}
	return out
}

// newRepositorySyncResult stamps the operation envelope onto the outcome,
// mirroring newMaintenanceResult.
func newRepositorySyncResult(result Result, out RepositorySyncResult) RepositorySyncResult {
	out.Envelope = NewEnvelope(OperationRepositorySyncIntegration, result)
	return out
}

// syncSeams is the injection seam repositorySyncIntegration runs over.
// Production (Task 4) wires it from loadOperationalContext and *gitcli.Client;
// unit tests inject fakes so the whole decision ladder is proved without a
// repository.
type syncSeams struct {
	// load performs the one ordered read: discovery, config resolution,
	// legacy refusal, and the fetch-and-pin of the integration branch.
	load func(ctx context.Context) (syncContext, error)
	// state observes the primary checkout (branch/detached/head/in-progress).
	state func(ctx context.Context, worktreeDir string) (gitcli.CheckoutState, error)
	// dirty reports tracked or non-ignored-untracked dirt in the primary
	// (ignored files are not dirt).
	dirty func(ctx context.Context, worktreeDir string) (bool, error)
	// isAncestor: exit-1 false is a clean negative; an error is a probe failure.
	isAncestor func(ctx context.Context, ancestor, descendant string) (bool, error)
	// fastForward applies merge --ff-only to the pinned target.
	fastForward func(ctx context.Context, worktreeDir, target string) (bool, error)
}

// syncContext is the loader's slice the ladder needs.
type syncContext struct {
	primaryWorktree     string
	integrationBranch   string // short name, e.g. main
	integrationRevision string // pinned, freshly fetched object id
}

// repositorySyncIntegration runs the safety ladder over the injected seams and
// returns the protocol-v1 sync document. Each numbered step below maps to the
// spec's §Safe advancement. The mutating update is the only side effect and it
// happens only on the strict-ancestor, still-observed-clean path.
func repositorySyncIntegration(ctx context.Context, seams syncSeams) RepositorySyncResult {
	// Step 1: the one ordered read — discovery, config, legacy refusal, and the
	// fetch-and-pin of the integration branch. A load error never authorizes a
	// mutating branch; the classifier supplies the envelope Result and reason,
	// and only an input/config/state refusal reads as `refused`.
	sc, err := seams.load(ctx)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		disp := SyncDispFailed
		switch result {
		case ResultInvalidInput, ResultUnsupportedConfig, ResultInvalidState:
			disp = SyncDispRefused
		}
		return newRepositorySyncResult(result, RepositorySyncResult{SyncOutcome: SyncOutcome{
			Disposition: disp,
			Reason:      reason,
			Message:     err.Error(),
		}})
	}

	primary := sc.primaryWorktree
	branch := sc.integrationBranch
	target := sc.integrationRevision
	fullRef := "refs/heads/" + branch

	// base carries the facts known once load succeeds; every outcome inherits
	// PrimaryPath, IntegrationBranch, and the pinned TargetOID.
	base := SyncOutcome{
		PrimaryPath:       primary,
		IntegrationBranch: branch,
		TargetOID:         target,
	}
	skip := func(reason, msg string) RepositorySyncResult {
		out := base
		out.Disposition = SyncDispSkipped
		out.Reason = reason
		out.Message = msg
		return newRepositorySyncResult(ResultNoOp, RepositorySyncResult{SyncOutcome: out})
	}
	fail := func(reason, msg string) RepositorySyncResult {
		out := base
		out.Disposition = SyncDispFailed
		out.Reason = reason
		out.Message = msg
		return newRepositorySyncResult(ResultExternalFailed, RepositorySyncResult{SyncOutcome: out})
	}

	// Step 2: observe the primary checkout, then decline every unsafe shape.
	st, err := seams.state(ctx, primary)
	if err != nil {
		return fail(ReasonSyncStateProbeFailed, err.Error())
	}
	if st.Detached {
		return skip(ReasonSyncDetachedHead, "primary checkout is in detached HEAD state")
	}
	if string(st.Branch) != fullRef {
		return skip(ReasonSyncOtherBranch, fmt.Sprintf("primary checkout is on %q, not the integration branch %q", st.Branch, fullRef))
	}
	if st.OperationInProgress {
		return skip(ReasonSyncOperationInProgress, "an unfinished Git operation is in progress in the primary checkout")
	}
	dirty, err := seams.dirty(ctx, primary)
	if err != nil {
		return fail(ReasonSyncStateProbeFailed, err.Error())
	}
	if dirty {
		return skip(ReasonSyncDirtyWorktree, syncDirtyMessage())
	}

	before := string(st.Head)
	base.BeforeOID = before

	// Step 3 is implicit: the target came from load (already fetched fresh); no
	// other source is consulted, so a stale remote-tracking fallback is
	// structurally impossible.

	// Step 4: compare local HEAD with the pinned target. Equality is already
	// current; strict-ancestor is the only shape that admits advancement.
	// Distinguish a negative ancestry result from a probe error.
	if before == target {
		out := base
		out.Disposition = SyncDispAlreadyCurrent
		out.AfterOID = before
		return newRepositorySyncResult(ResultNoOp, RepositorySyncResult{SyncOutcome: out})
	}
	ancestor, err := seams.isAncestor(ctx, before, target)
	if err != nil {
		return fail(ReasonSyncAncestryProbeFailed, err.Error())
	}
	if !ancestor {
		reverse, err := seams.isAncestor(ctx, target, before)
		if err != nil {
			return fail(ReasonSyncAncestryProbeFailed, err.Error())
		}
		if reverse {
			return skip(ReasonSyncLocalAhead, "primary checkout is ahead of the integration tip")
		}
		return skip(ReasonSyncDiverged, "primary checkout has diverged from the integration tip")
	}

	// Step 5: recheck the checkout immediately before the advance; a changed or
	// unobservable state is left alone. FastForwardWorktree rechecks cleanliness
	// and enforces the FF-only update itself against the pinned object id.
	recheck, err := seams.state(ctx, primary)
	if err != nil {
		return fail(ReasonSyncStateProbeFailed, err.Error())
	}
	if recheck.Detached || string(recheck.Branch) != fullRef || recheck.OperationInProgress || string(recheck.Head) != before {
		return skip(ReasonSyncCheckoutChanged, "primary checkout changed between observation and advance")
	}
	if _, err := seams.fastForward(ctx, primary, target); err != nil {
		return fail(ReasonSyncUpdateFailed, err.Error())
	}

	// Step 6: verify the resulting branch/HEAD before claiming advancement. A
	// post-check failure reports the known before/target facts with After empty
	// and never claims the tree untouched or rolls anything back.
	post, err := seams.state(ctx, primary)
	if err != nil {
		return fail(ReasonSyncPostcheckUnverified, err.Error())
	}
	if string(post.Head) != target || string(post.Branch) != fullRef {
		return fail(ReasonSyncPostcheckUnverified, "post-advance verification did not observe the integration tip at the primary HEAD")
	}
	out := base
	out.Disposition = SyncDispAdvanced
	out.AfterOID = target
	return newRepositorySyncResult(ResultApplied, RepositorySyncResult{SyncOutcome: out})
}
