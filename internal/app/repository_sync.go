package app

import "fmt"

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
