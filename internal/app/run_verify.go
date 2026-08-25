package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository"
)

// This file is the `run verify` operation: a read-only postcondition report over
// one change's claim→implemented run. It writes nothing and opens no transaction;
// it reuses the same verification predicates the durable transitions enforce
// (workspace head, remote head, gate evidence, PR identity, plan and results
// artifact identity, claim lease) and folds them into ONE of three closed
// verdicts:
//
//   - run-unclaimed — the change is still proposed; no run was ever started.
//   - run-complete  — the change is implemented and every postcondition holds.
//   - run-incomplete — the run is not yet complete; the result enumerates every
//     unmet postcondition as a {reason, observed} pair.
//
// All three verdicts are REPORTS, not process failures, so each maps to a
// success-shaped envelope result and exits 0 (learning
// exit-code-encodes-a-non-failure). Only an operational error — an absent or
// ambiguous id, an unreadable repository, a probe that itself failed — exits
// non-zero; an errored probe is never folded into a clean "postcondition unmet"
// (learning probe-error-is-not-clean-absence).
//
// Child-agent returns are never consulted: every fact is read from
// Git/GitHub/evidence/metadata at call time.

// OperationRunVerify is the operation key `run verify` records in its envelope.
const OperationRunVerify = "run.verify"

// The three closed verdicts `run verify` reports. Automation keys on this field,
// never the exit code.
const (
	VerdictRunComplete   = "run-complete"
	VerdictRunUnclaimed  = "run-unclaimed"
	VerdictRunIncomplete = "run-incomplete"
	// VerdictRunHalted: the change carries a durable "## Run halted" marker — the
	// run was deliberately paused and a human must resume it. A halted run is
	// neither complete nor incomplete; automation keys on this closed verdict to
	// stop re-dispatching (never a re-dispatch of a halt).
	VerdictRunHalted = "run-halted"
	// VerdictRunWaiting: a safe local continuation exists — a fingerprinted gate
	// drive has an explicit unclaimed handoff that a fresh owner on THIS machine
	// can claim, and every independent local receipt agrees (see
	// evaluateRunWaiting). Its report line is `run-waiting <change-id>
	// <opaque-handoff-id> <phase>`. It is a closed verdict of its own — the change
	// stays in-progress and no metadata is written — consumed by its spelling, and
	// it exposes only the opaque drive/handoff locator and workflow phase, never an
	// owner credential or the suite command. It means "a safe local continuation
	// exists," not merely "a process might still be running."
	VerdictRunWaiting = "run-waiting"
)

// The stable machine reasons `run verify` records for each unmet postcondition.
// Message/observed text is explanatory and must not be parsed. Each names the one
// postcondition it maps to; a run-incomplete verdict carries one per broken
// conjunct.
const (
	// ReasonRunNotImplemented: the change is claimed but has not reached the
	// implemented status, so the run is not complete.
	ReasonRunNotImplemented = "not-implemented"
	// ReasonRunPlanUnlinked: the change carries no linked plan.
	ReasonRunPlanUnlinked = "plan-unlinked"
	// ReasonRunPlanMissing: the linked plan no longer resolves to a tracked regular
	// file at the current feature head.
	ReasonRunPlanMissing = "plan-file-missing"
	// ReasonRunRemoteHeadMismatch: the remote feature ref is absent or names a
	// commit other than the current local feature head.
	ReasonRunRemoteHeadMismatch = "remote-head-mismatch"
	// ReasonRunEvidenceUnverified: the durable build evidence (the PR body) does not
	// verify against the current feature head — missing, malformed, or stale.
	ReasonRunEvidenceUnverified = "evidence-unverified"
	// ReasonRunPRUnverified: there is not exactly one open PR for the feature branch
	// naming the current head, targeting the resolved base, and equal to the
	// recorded PR reference.
	ReasonRunPRUnverified = "pr-unverified"
	// ReasonRunResultsIdentity: an attached results path no longer resolves to a
	// tracked regular file at the current feature head.
	ReasonRunResultsIdentity = "results-identity-broken"
	// ReasonRunLeaseContended: the recorded claim branch is absent or does not match
	// the change's resolved feature branch — the lease was lost or overwritten.
	ReasonRunLeaseContended = "lease-contended"
)

// Operational (non-verdict) reasons. These predate any verdict determination and
// exit non-zero.
const (
	ReasonRunInvalidID      = "invalid-id"
	ReasonRunUnknownChange  = "unknown-change"
	ReasonRunAmbiguousID    = "ambiguous-change"
	ReasonRunRepoUnresolved = "repository-unresolved"
	ReasonRunRemoteProbe    = "remote-probe-failed"
	ReasonRunPRProbe        = "pr-probe-failed"
	ReasonRunArtifactRead   = "artifact-read-failed"
)

// RunVerifyRequest is the closed request for `run verify`: the change id whose run
// to report on. The operation reads everything else from authoritative state.
type RunVerifyRequest struct {
	ID int `json:"id"`
}

// RunVerifyConjunct is one unmet postcondition: its stable machine reason and a
// short observed detail (never an authored document body or credential).
type RunVerifyConjunct struct {
	Reason   string `json:"reason"`
	Observed string `json:"observed,omitempty"`
}

// RunVerifyResult is the protocol-v1 document `run verify` returns. Verdict is the
// closed report token; Unmet enumerates every broken postcondition (marshalled as
// [] never null). A refusal that predates verdict determination carries Reason and
// Message and no verdict instead. It never carries authored document bodies.
type RunVerifyResult struct {
	Envelope
	ID      int                 `json:"id,omitempty"`
	Verdict string              `json:"verdict,omitempty"`
	Head    string              `json:"head,omitempty"`
	PR      string              `json:"pr,omitempty"`
	Unmet   []RunVerifyConjunct `json:"unmet"`
	// HandoffID and Phase are populated on a run-waiting verdict only: the opaque
	// drive/handoff locator a fresh owner claims, and the workflow phase to resume.
	// They are the ONLY fields the run-waiting line exposes beyond the change id;
	// neither is an owner credential or the suite command.
	HandoffID string `json:"handoff_id,omitempty"`
	Phase     string `json:"phase,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Message   string `json:"message,omitempty"`
}

// WaitingReceipt is the redaction-safe bundle of agreeing local receipts `run
// verify` folds into the run-waiting verdict. It carries per-dimension identity
// hashes and structural booleans only — never an owner credential, the suite
// command, launch environment, or worktree content — so it is safe to compute in
// a read-only reporter. A WaitingReceiptReader assembles it from the durable gate
// drive record, the live worktree recomputation, and the native process
// ownership receipt; evaluateRunWaiting is the sole authority that folds it into
// a verdict, checking every field's agreement so each is independently
// mutation-testable.
type WaitingReceipt struct {
	// DriveID is the opaque drive/handoff locator exposed as <opaque-handoff-id>.
	DriveID string
	// HasUnclaimedHandoff is true only when the drive record carries an explicit
	// unclaimed handoff (an offered, not-yet-claimed transfer).
	HasUnclaimedHandoff bool
	// ChangeID/TaskID/Phase are the drive's recorded work identity — the chain the
	// verdict proves is unambiguous.
	ChangeID string
	TaskID   string
	Phase    string
	// Branch is the drive's recorded branch, cross-checked against the change's
	// recorded claim branch.
	Branch string
	// WorktreePath is the drive's linked worktree; WorktreeExists reports whether
	// it still resolves on disk.
	WorktreePath   string
	WorktreeExists bool
	// DriveHead is the drive-start HEAD object id; DriveFingerprint is the
	// drive-start execution identity; LiveFingerprint is the recomputation over the
	// current worktree. HEAD and the full dirty-worktree fingerprint must agree
	// across all three (and the workspace head) for a waiting continuation to be safe.
	DriveHead        string
	DriveFingerprint gatedrive.Fingerprint
	LiveFingerprint  gatedrive.Fingerprint
	// DeadlineLive is true while the drive's fixed deadline has not passed.
	// TerminalWaiting is true when a durable terminal result (a passed or failed
	// suite) is already waiting to be consumed — the one admitted exception to the
	// live-deadline condition.
	DeadlineLive    bool
	TerminalWaiting bool
	// RawRunMatches reports whether the referenced raw run and its native ownership
	// receipt still match the drive's active attempt.
	RawRunMatches bool
}

// WaitingReceiptReader gathers the local run-waiting receipts for a change. It
// returns found=false — never an invented waiting — when no local drive matches
// the change (for example on another machine, where the local state is absent),
// when the match is ambiguous, or when the identity cannot be recomputed. An
// error is a receipt-read fault the caller folds to "no waiting" rather than a
// verdict, so a receipt-reader problem can never upgrade an incomplete run.
type WaitingReceiptReader interface {
	Read(ctx context.Context, repoDir string, changeID int) (WaitingReceipt, bool, error)
}

// HumanText renders the one report line plus, for an incomplete run, its unmet
// postconditions. An operational refusal names its reason instead.
func (r RunVerifyResult) HumanText() string {
	if r.Verdict == VerdictRunWaiting {
		// The spec's one-line form: run-waiting <change-id> <opaque-handoff-id> <phase>.
		return fmt.Sprintf("run verify: change %04d %s %s %s", r.ID, r.Verdict, r.HandoffID, r.Phase)
	}
	if r.Verdict != "" {
		if len(r.Unmet) == 0 {
			return fmt.Sprintf("run verify: change %04d %s", r.ID, r.Verdict)
		}
		reasons := make([]string, 0, len(r.Unmet))
		for _, u := range r.Unmet {
			reasons = append(reasons, u.Reason)
		}
		return fmt.Sprintf("run verify: change %04d %s — %s", r.ID, r.Verdict, strings.Join(reasons, ", "))
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// newRunVerifyResult stamps the envelope and normalizes Unmet to an empty slice so
// the array marshals as [] on every path.
func newRunVerifyResult(result Result, out RunVerifyResult) RunVerifyResult {
	out.Envelope = NewEnvelope(OperationRunVerify, result)
	if out.Unmet == nil {
		out.Unmet = []RunVerifyConjunct{}
	}
	return out
}

// runOperationalRefusal builds a non-verdict refusal: an operational error that
// exits non-zero and carries no verdict.
func runOperationalRefusal(result Result, reason, message string, id int) RunVerifyResult {
	return newRunVerifyResult(result, RunVerifyResult{ID: id, Reason: reason, Message: message})
}

// runVerdict builds a report result. All three verdicts are success-shaped
// (applied) so they exit 0; the verdict field is what automation keys on.
func runVerdict(verdict string, id int, head, pr string, unmet []RunVerifyConjunct) RunVerifyResult {
	return newRunVerifyResult(ResultApplied, RunVerifyResult{
		ID: id, Verdict: verdict, Head: head, PR: pr, Unmet: unmet,
	})
}

// RunVerify reports on one change's claim→implemented run without mutating
// anything. It resolves the change, short-circuits an unclaimed (still-proposed)
// change to run-unclaimed, then reprobes every implemented-run postcondition and
// folds the results into run-complete (all hold) or run-incomplete (enumerating
// the unmet ones).
func RunVerify(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, gdeps GitHubDeps, repoDir string, req RunVerifyRequest) RunVerifyResult {
	if req.ID <= 0 {
		return runOperationalRefusal(ResultInvalidInput, ReasonRunInvalidID, "id must be a positive change id", req.ID)
	}

	// Resolve the change from one authoritative pin/corpus read.
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return runOperationalRefusal(result, reason, err.Error(), req.ID)
	}
	eff := pin.Config.Effective
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return runOperationalRefusal(result, reason, err.Error(), req.ID)
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		return runOperationalRefusal(ResultInternalError, ReasonStatusInternalError, err.Error(), req.ID)
	}
	c, out := build.Snapshot.Change(domain.ChangeID(req.ID))
	if out != domain.LookupFound {
		if out == domain.LookupAmbiguous {
			return runOperationalRefusal(ResultInvalidState, ReasonRunAmbiguousID,
				fmt.Sprintf("more than one record claims change id %04d; refusing to choose", req.ID), req.ID)
		}
		return runOperationalRefusal(ResultInvalidInput, ReasonRunUnknownChange,
			fmt.Sprintf("no change %04d is present in the corpus", req.ID), req.ID)
	}

	// A still-proposed change was never claimed: no run to verify.
	if c.Status() == domain.StatusProposed {
		return runVerdict(VerdictRunUnclaimed, req.ID, "", "", nil)
	}

	// A change carrying a durable run-halted marker is halted: the run was
	// deliberately paused and a human must resume it (via change resume-halted).
	// This is a closed verdict of its own — neither complete nor incomplete — so
	// it short-circuits the postcondition reprobe. HasRunHalted is the domain's
	// shape-keyed detection of the "## Run halted" body section.
	if c.HasRunHalted() {
		return runVerdict(VerdictRunHalted, req.ID, "", strings.TrimSpace(c.PR().Value), nil)
	}

	recordedPR := strings.TrimSpace(c.PR().Value)
	var unmet []RunVerifyConjunct
	add := func(reason, observed string) {
		unmet = append(unmet, RunVerifyConjunct{Reason: reason, Observed: observed})
	}

	// A claimed but not-yet-implemented change cannot be a complete run.
	if c.Status() != domain.StatusImplemented {
		add(ReasonRunNotImplemented, c.RawStatus())
	}

	// The workspace inspection is the authoritative reference head and the resolved
	// feature/base branches. A non-applied inspection is an operational error (we
	// cannot establish the reference head), never a silent postcondition failure.
	insp := WorkspaceInspect(ctx, deps, wdeps, repoDir, WorkspaceIDRequest{ID: req.ID})
	if insp.Result != ResultApplied {
		return runOperationalRefusal(insp.Result, insp.Reason, insp.Message, req.ID)
	}
	head := insp.Head
	featureBranch := strings.TrimPrefix(insp.FeatureRef, branchRefPrefix)
	baseBranch := strings.TrimPrefix(insp.BaseRef, branchRefPrefix)

	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return runOperationalRefusal(result, reason, err.Error(), req.ID)
	}

	// Lease: the recorded claim branch must still name the change's resolved
	// feature branch. An absent or divergent claim branch is a contended lease.
	if b := c.Branch(); b.State == domain.FieldAbsent || strings.TrimSpace(b.Value) == "" {
		add(ReasonRunLeaseContended, "no recorded claim branch")
	} else if b.Value != featureBranch {
		add(ReasonRunLeaseContended, b.Value)
	}

	// Plan link and plan-file identity at the feature head.
	planPath := strings.TrimSpace(c.Plan().Value)
	resultsPath := strings.TrimSpace(c.Results().Value)
	if planPath == "" {
		add(ReasonRunPlanUnlinked, "")
	}
	// Open the head's object source once when a blob read is needed.
	if planPath != "" || resultsPath != "" {
		src, err := deps.Client.OpenObjectSource(ctx, repo, gitcli.Revision{Commit: gitcli.ObjectID(head)})
		if err != nil {
			result, reason := classifyStatusError(ctx, classifyGitFailure(err))
			if reason == ReasonStatusInternalError {
				reason = ReasonRunArtifactRead
			}
			return runOperationalRefusal(result, reason, err.Error(), req.ID)
		}
		if planPath != "" {
			okFile, rerr := trackedRegularBlob(ctx, src, planPath)
			if rerr != nil {
				return runOperationalRefusal(ResultExternalFailed, ReasonRunArtifactRead, rerr.Error(), req.ID)
			}
			if !okFile {
				add(ReasonRunPlanMissing, planPath)
			}
		}
		if resultsPath != "" {
			okFile, rerr := trackedRegularBlob(ctx, src, resultsPath)
			if rerr != nil {
				return runOperationalRefusal(ResultExternalFailed, ReasonRunArtifactRead, rerr.Error(), req.ID)
			}
			if !okFile {
				add(ReasonRunResultsIdentity, resultsPath)
			}
		}
	}

	// Remote feature head must equal the local head. An errored probe is
	// operational; a clean absence or a divergent commit is a postcondition miss.
	rref, err := deps.Client.ProbeRemoteBranch(ctx, repo, originRemote, gitcli.RefName(insp.FeatureRef))
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		if reason == ReasonStatusInternalError {
			reason = ReasonRunRemoteProbe
		}
		return runOperationalRefusal(result, reason, err.Error(), req.ID)
	}
	if rref.State != gitcli.RemoteRefFound {
		add(ReasonRunRemoteHeadMismatch, "remote feature ref absent")
	} else if string(rref.Commit) != head {
		add(ReasonRunRemoteHeadMismatch, string(rref.Commit))
	}

	// PR identity and evidence: exactly one open PR for the feature branch, naming
	// the head, targeting the base, and naming the recorded PR number; the PR body
	// is the durable evidence store, verified against the head.
	ghRepo, err := gdeps.Service.DiscoverRepository(ctx, repoDir)
	if err != nil {
		return runOperationalRefusal(ResultExternalFailed, ReasonRunRepoUnresolved, err.Error(), req.ID)
	}
	prs, err := gdeps.Service.FindOpenPullRequestsByHead(ctx, ghRepo, featureBranch)
	if err != nil {
		return runOperationalRefusal(ResultExternalFailed, ReasonRunPRProbe, err.Error(), req.ID)
	}
	if len(prs) != 1 {
		add(ReasonRunPRUnverified, fmt.Sprintf("%d open pull requests for the feature branch", len(prs)))
		add(ReasonRunEvidenceUnverified, "no unique pull-request body to read evidence from")
	} else {
		pr := prs[0]
		// Identity is by parsed PR number within the already-resolved repository
		// (DiscoverRepository pinned the repo; FindOpenPullRequestsByHead pinned the
		// feature branch), so the number is a complete discriminator. parsePRRef
		// reads the recorded pr: in either form, so a manifest written as the
		// canonical URL or the legacy owner/repo#N shorthand both verify.
		recordedNum, recordedOK := parsePRRef(recordedPR)
		switch {
		case pr.HeadCommit != head:
			add(ReasonRunPRUnverified, "the open PR names a head other than the feature head")
		case pr.BaseBranch != baseBranch:
			add(ReasonRunPRUnverified, "the open PR targets a base other than the resolved effective base")
		case !recordedOK || recordedNum != pr.Number:
			add(ReasonRunPRUnverified, "the open PR is not the one recorded on the change")
		}
		if v := evidence.Verify([]byte(pr.Body), head); v != evidence.VerdictVerified {
			add(ReasonRunEvidenceUnverified, string(v))
		}
	}

	// Completed-run postconditions take precedence over a stale local handoff: a
	// run that satisfies every postcondition is complete regardless of any drive
	// receipt still on disk.
	if len(unmet) == 0 {
		return runVerdict(VerdictRunComplete, req.ID, head, recordedPR, nil)
	}

	// A valid local run-waiting precedes ordinary run-incomplete. It is derived
	// EXCLUSIVELY from agreeing local receipts; a missing/ambiguous/unreadable
	// receipt source, or any single disagreeing receipt, folds back to
	// run-incomplete rather than inventing waiting.
	if wdeps.Waiting != nil {
		if rcpt, ok, rerr := wdeps.Waiting.Read(ctx, repoDir, req.ID); rerr == nil && ok {
			if handoffID, phase, valid := evaluateRunWaiting(req.ID, c, head, rcpt); valid {
				return newRunVerifyResult(ResultApplied, RunVerifyResult{
					ID: req.ID, Verdict: VerdictRunWaiting, Head: head, PR: recordedPR,
					HandoffID: handoffID, Phase: phase,
				})
			}
		}
	}

	return runVerdict(VerdictRunIncomplete, req.ID, head, recordedPR, unmet)
}

// evaluateRunWaiting folds one WaitingReceipt into the run-waiting decision. It is
// the sole authority for the verdict and fails closed: EVERY independent
// condition from the spec's "Local run-waiting verdict" must agree, so any single
// disagreeing receipt makes waiting disappear (and each is mutation-testable). On
// success it returns the opaque handoff locator and phase the report line
// exposes; it never returns, and the receipt never carries, an owner credential
// or the suite command.
func evaluateRunWaiting(changeID int, c domain.Change, workspaceHead string, r WaitingReceipt) (handoffID, phase string, ok bool) {
	// The change is still the claimed in-progress change the workflow expects.
	if c.Status() != domain.StatusInProgress {
		return "", "", false
	}
	// The referenced change identity resolves to exactly this change — one link of
	// the unambiguous change/task/phase/drive chain.
	if n, err := strconv.Atoi(strings.TrimSpace(r.ChangeID)); err != nil || n != changeID {
		return "", "", false
	}
	// The driver record is recognized and carries an explicit UNCLAIMED handoff.
	if !r.HasUnclaimedHandoff {
		return "", "", false
	}
	// The change's recorded claim branch exists and matches the handoff's branch.
	b := c.Branch()
	if b.State == domain.FieldAbsent || strings.TrimSpace(b.Value) == "" || b.Value != r.Branch {
		return "", "", false
	}
	// The recorded/linked worktree is named and still exists on disk.
	if strings.TrimSpace(r.WorktreePath) == "" || !r.WorktreeExists {
		return "", "", false
	}
	// HEAD agrees across the workspace, the drive receipt, and the live
	// recomputation.
	if r.DriveHead == "" || r.DriveHead != workspaceHead || r.LiveFingerprint.Head != r.DriveHead {
		return "", "", false
	}
	// The full dirty-worktree fingerprint still matches the drive-start identity.
	if !r.LiveFingerprint.Equal(r.DriveFingerprint) {
		return "", "", false
	}
	// The deadline is still live, UNLESS a durable terminal result is already
	// waiting to be consumed.
	if !r.DeadlineLive && !r.TerminalWaiting {
		return "", "", false
	}
	// The referenced raw run and its native ownership receipt match the active
	// driver attempt.
	if !r.RawRunMatches {
		return "", "", false
	}
	// The exposed chain links are present: an opaque drive/handoff locator and a
	// workflow phase (the change link was proven above).
	if strings.TrimSpace(r.DriveID) == "" || strings.TrimSpace(r.Phase) == "" {
		return "", "", false
	}
	return r.DriveID, r.Phase, true
}

// trackedRegularBlob reports whether path resolves to a tracked, regular (non
// symlink) file at the source's pinned commit. A read-infrastructure error is
// returned as an error (operational); a missing path or a symlink is a clean
// false — the caller maps that to the relevant postcondition miss.
func trackedRegularBlob(ctx context.Context, src gitcli.ObjectSource, path string) (bool, error) {
	results, err := src.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(path)})
	if err != nil {
		return false, err
	}
	if len(results) != 1 || !results[0].Found {
		return false, nil
	}
	if results[0].Blob.Mode == "120000" {
		return false, nil
	}
	return true, nil
}
