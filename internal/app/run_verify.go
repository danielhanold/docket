package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/evidence"
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
	Reason  string              `json:"reason,omitempty"`
	Message string              `json:"message,omitempty"`
}

// HumanText renders the one report line plus, for an incomplete run, its unmet
// postconditions. An operational refusal names its reason instead.
func (r RunVerifyResult) HumanText() string {
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
	// the head, targeting the base, equal to the recorded reference; the PR body is
	// the durable evidence store, verified against the head.
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
		reference := fmt.Sprintf("%s#%d", ghRepo.Spec(), pr.Number)
		switch {
		case pr.HeadCommit != head:
			add(ReasonRunPRUnverified, "the open PR names a head other than the feature head")
		case pr.BaseBranch != baseBranch:
			add(ReasonRunPRUnverified, "the open PR targets a base other than the resolved effective base")
		case reference != recordedPR:
			add(ReasonRunPRUnverified, "the open PR is not the one recorded on the change")
		}
		if v := evidence.Verify([]byte(pr.Body), head); v != evidence.VerdictVerified {
			add(ReasonRunEvidenceUnverified, string(v))
		}
	}

	if len(unmet) == 0 {
		return runVerdict(VerdictRunComplete, req.ID, head, recordedPR, nil)
	}
	return runVerdict(VerdictRunIncomplete, req.ID, head, recordedPR, unmet)
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
