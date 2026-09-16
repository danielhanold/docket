package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is the `evidence recertify` operation (change 0415): for an
// implemented change whose open PR received a PUBLISHED follow-up commit, it
// reruns the BUILD gate at the current feature head, records and verifies
// canonical build evidence, and refreshes ONLY the existing PR's build-evidence
// block. The change stays implemented throughout; the operation performs no
// Docket metadata mutation, no push, no rebase, no merge, and no automatic
// repair. It composes only landed services: the workspace context/target
// resolution, the build-owned production local gate (NewBuildLocalGate), the
// build-owned evidence record path, and finalize publish's loss-preserving PR
// evidence-block edit (evidence.Upsert + finalizePublishEnsurer) WITHOUT its
// rebase receipt, PublishRewrite, or merge path.

// OperationEvidenceRecertify is the operation key `evidence recertify` records
// in its result envelope.
const OperationEvidenceRecertify = "evidence.recertify"

// The closed recertify outcomes.
const (
	// RecertifyOutcomeGreen: the build gate passed and the PR's evidence block
	// was converged onto the exact current head.
	RecertifyOutcomeGreen = "green"
	// RecertifyOutcomeSkipped: build.gate is off; truthful skipped evidence was
	// minted and verified. The PR block is untouched — evidence.Upsert is
	// green-only by design, and this operation preserves evidence rendering.
	RecertifyOutcomeSkipped = "skipped"
)

// Stable machine reasons `evidence recertify` reports. Message text is
// explanatory and must not be parsed. Config refusals reuse the evidence
// vocabulary (ReasonEvidenceUnconfiguredGate).
const (
	ReasonRecertifyNotImplemented    = "not-implemented"
	ReasonRecertifyWorkspaceDirty    = "workspace-dirty"
	ReasonRecertifyWorkspaceNotReady = "workspace-not-ready"
	ReasonRecertifyRemoteProbe       = "remote-feature-probe-failed"
	ReasonRecertifyRemoteAbsent      = "remote-feature-absent"
	// ReasonRecertifyHeadDisagreement: local, remote, and PR feature heads must
	// all agree before (and still agree after) the gate; the message names the
	// disagreeing leg. An unpushed follow-up must be published first.
	ReasonRecertifyHeadDisagreement = "head-disagreement"
	ReasonRecertifyRepoUnresolved   = "repository-unresolved"
	ReasonRecertifyPRProbeFailed    = "pr-probe-failed"
	ReasonRecertifyPRNotOpen        = "pr-not-open"
	ReasonRecertifyGateFailed       = "gate-failed"
	ReasonRecertifyGateHalted       = "gate-halted"
	// ReasonRecertifyIdentityDrift: something the gate certified moved before
	// the publish — the PR identity or the resolved build command. A changed
	// head or command can never inherit the earlier pass.
	ReasonRecertifyIdentityDrift      = "identity-drift"
	ReasonRecertifyEvidenceUnverified = "evidence-unverified"
	ReasonRecertifyBodyAssembly       = "body-assembly-failed"
	ReasonRecertifyEditorUnavailable  = "pr-editor-unavailable"
	ReasonRecertifyEditContended      = "pr-edit-contended"
	ReasonRecertifyEditUnknown        = "pr-edit-unknown"
)

// EvidenceRecertifyRequest is the closed request for `evidence recertify`.
type EvidenceRecertifyRequest struct {
	ID int `json:"id" docket:"required"`
}

// EvidenceRecertifyResult is the protocol-v1 document the operation returns. It
// names identity, the exact certified head, the closed outcome, the PR
// reference, and — when a gate ran — its outcome facts. It holds no authored PR
// body bytes.
type EvidenceRecertifyResult struct {
	Envelope
	ID          int    `json:"id,omitempty"`
	Head        string `json:"head,omitempty"`
	Outcome     string `json:"outcome,omitempty"`
	Command     string `json:"command,omitempty"`
	Number      int    `json:"number,omitempty"`
	Reference   string `json:"reference,omitempty"`
	URL         string `json:"url,omitempty"`
	GateOutcome string `json:"gate_outcome,omitempty"`
	HaltCause   string `json:"halt_cause,omitempty"`
	RunDir      string `json:"run_dir,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Message     string `json:"message,omitempty"`
}

// HumanText renders a one-line summary naming identity, outcome, head, and PR —
// never a body.
func (r EvidenceRecertifyResult) HumanText() string {
	if r.Result == ResultApplied || r.Result == ResultNoOp {
		s := fmt.Sprintf("%s: change %04d %s (head %s)", r.Operation, r.ID, r.Outcome, shortCommit(r.Head))
		if r.Reference != "" {
			s += " " + r.Reference
		}
		return s
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// recertifyRefusal stamps a typed refusal for the recertify operation.
func recertifyRefusal(result Result, reason, message string, id int) EvidenceRecertifyResult {
	return EvidenceRecertifyResult{Envelope: NewEnvelope(OperationEvidenceRecertify, result), ID: id, Reason: reason, Message: message}
}

// recertifyFacts is the identity bundle one probe pass resolves: the exact
// agreed feature head, the workspace checkout, the discovered GitHub repo, and
// the single open PR, plus the pinned build gate policy.
type recertifyFacts struct {
	id     int
	head   string
	wsDir  string
	branch string
	repo   githubcli.Repository
	pr     githubcli.PullRequest
	build  config.Build
}

// recertifyProbe resolves and validates every identity precondition, in one
// reusable predicate (learning duplicated-gate-copies-the-whole-predicate): the
// change is implemented; the manifest-owned feature workspace is clean,
// registered, and inspectable; the local head, the remote feature head, and the
// single open PR's head all agree. It is run BEFORE the gate and AGAIN before
// the publish — the recheck is the same whole predicate, not a copied subset.
func recertifyProbe(ctx context.Context, deps FinalizeDeps, repoDir string, id int) (recertifyFacts, *EvidenceRecertifyResult) {
	pin, err := deps.Planning.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := recertifyRefusal(result, reason, err.Error(), id)
		return recertifyFacts{}, &r
	}
	wc, wref := loadWorkspaceContext(ctx, deps.Planning, repoDir, id, OperationEvidenceRecertify)
	if wref != nil {
		r := recertifyRefusal(wref.Result, wref.Reason, wref.Message, wref.ID)
		return recertifyFacts{}, &r
	}
	cid := int(wc.change.ID())
	if wc.change.Status() != domain.StatusImplemented {
		r := recertifyRefusal(ResultBlocked, ReasonRecertifyNotImplemented,
			fmt.Sprintf("change %04d is %q, not implemented; recertify only refreshes an implemented change's evidence", cid, wc.change.RawStatus()), cid)
		return recertifyFacts{}, &r
	}
	target, tref := resolveWorkspaceTarget(OperationEvidenceRecertify, wc)
	if tref != nil {
		r := recertifyRefusal(tref.Result, tref.Reason, tref.Message, tref.ID)
		return recertifyFacts{}, &r
	}
	insp, err := deps.Workspace.Inspect(ctx, workspace.InspectRequest{Repository: wc.repo, Target: target})
	if err != nil {
		r := recertifyRefusal(ResultExternalFailed, ReasonRecertifyWorkspaceNotReady, err.Error(), cid)
		return recertifyFacts{}, &r
	}
	switch insp.Kind {
	case workspace.StateReady:
		// clean and registered; fall through
	case workspace.StateDirty:
		r := recertifyRefusal(ResultBlocked, ReasonRecertifyWorkspaceDirty,
			"the feature workspace has uncommitted changes; a recertify requires a clean tree", cid)
		return recertifyFacts{}, &r
	default:
		r := recertifyRefusal(ResultBlocked, ReasonRecertifyWorkspaceNotReady,
			fmt.Sprintf("the feature workspace is %q, not the clean registered feature state", insp.Kind), cid)
		return recertifyFacts{}, &r
	}
	head := strings.ToLower(string(insp.HeadCommit))

	// The remote feature head must equal the local head: an unpushed follow-up
	// is published through the existing workflow first, never certified here.
	rref, err := deps.Planning.Client.ProbeRemoteBranch(ctx, wc.repo, originRemote, target.FeatureRef)
	if err != nil {
		r := recertifyRefusal(ResultExternalFailed, ReasonRecertifyRemoteProbe, err.Error(), cid)
		return recertifyFacts{}, &r
	}
	if rref.State != gitcli.RemoteRefFound {
		r := recertifyRefusal(ResultBlocked, ReasonRecertifyRemoteAbsent,
			"the remote feature ref is absent; publish the feature head before recertifying", cid)
		return recertifyFacts{}, &r
	}
	if strings.ToLower(string(rref.Commit)) != head {
		r := recertifyRefusal(ResultBlocked, ReasonRecertifyHeadDisagreement,
			"the remote feature head is not the local head; publish the follow-up through the existing workflow first", cid)
		return recertifyFacts{}, &r
	}

	// Exactly one open PR, naming exactly this head.
	repo, err := deps.GitHub.DiscoverRepository(ctx, repoDir)
	if err != nil {
		r := recertifyRefusal(ResultExternalFailed, ReasonRecertifyRepoUnresolved, err.Error(), cid)
		return recertifyFacts{}, &r
	}
	branch := strings.TrimPrefix(string(target.FeatureRef), branchRefPrefix)
	prs, err := deps.GitHub.FindOpenPullRequestsByHead(ctx, repo, branch)
	if err != nil {
		r := recertifyRefusal(ResultExternalFailed, ReasonRecertifyPRProbeFailed, err.Error(), cid)
		return recertifyFacts{}, &r
	}
	if len(prs) != 1 {
		r := recertifyRefusal(ResultBlocked, ReasonRecertifyPRNotOpen,
			fmt.Sprintf("%d open pull requests for the feature head; a recertify requires exactly one", len(prs)), cid)
		return recertifyFacts{}, &r
	}
	pr := prs[0]
	if strings.ToLower(pr.HeadCommit) != head {
		r := recertifyRefusal(ResultBlocked, ReasonRecertifyHeadDisagreement,
			"the open PR names a head other than the current feature head; re-read the change state", cid)
		return recertifyFacts{}, &r
	}
	return recertifyFacts{
		id: cid, head: head, wsDir: insp.Path, branch: branch,
		repo: repo, pr: pr, build: pin.Config.Effective.Build,
	}, nil
}

// EvidenceRecertify re-certifies an implemented change's build evidence in
// place. Task 3 composes the gate and publish legs; until then the tail fails
// closed rather than shaping a success.
func EvidenceRecertify(ctx context.Context, deps FinalizeDeps, wdeps WorkspaceDeps, repoDir string, req EvidenceRecertifyRequest) EvidenceRecertifyResult {
	if req.ID <= 0 {
		return recertifyRefusal(ResultInvalidInput, "", "id must be a positive change id", req.ID)
	}
	// Capability preflight before any external effect (mirrors FinalizePublish).
	pin, err := deps.Planning.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return recertifyRefusal(result, reason, err.Error(), req.ID)
	}
	if decision := config.PreflightMutation(&pin.Config); !decision.Allowed {
		return recertifyRefusal(ResultUnsupportedConfig, ReasonDeferredCapRequested,
			"configuration actively requests a deferred capability docket does not ship in this version ("+
				strings.Join(blockerPaths(decision.Blockers), ", ")+"); withdraw it before any mutation", req.ID)
	}
	facts, refusal := recertifyProbe(ctx, deps, repoDir, req.ID)
	if refusal != nil {
		return *refusal
	}

	// Config policy branch (the pinned build gate decides what "recertify" means).
	if facts.build.Gate.Value == "off" {
		// Truthful skipped evidence at the verified current head; no run, and no
		// PR edit — evidence.Upsert is green-only by design (see
		// TestPRPublishAcceptsSkippedEvidenceAtExactHead's note), and this
		// operation preserves evidence rendering.
		evd := EvidenceRecord(ctx, deps.Planning, wdeps, repoDir, EvidenceRecordRequest{ID: facts.id, Head: facts.head})
		if evd.Result != ResultApplied || evd.Block == "" {
			return recertifyRefusal(evd.Result, evd.Reason, evd.Message, facts.id)
		}
		if v := evidence.Verify([]byte(evd.Block), facts.head); v != evidence.VerdictSkipped {
			return recertifyRefusal(ResultInvalidState, ReasonRecertifyEvidenceUnverified,
				"the skipped evidence record did not verify against the current head ("+string(v)+")", facts.id)
		}
		return EvidenceRecertifyResult{
			Envelope: NewEnvelope(OperationEvidenceRecertify, ResultApplied),
			ID:       facts.id, Head: facts.head, Outcome: RecertifyOutcomeSkipped,
			Number: facts.pr.Number, Reference: fmt.Sprintf("%s#%d", facts.repo.Spec(), facts.pr.Number), URL: facts.pr.URL,
			Message: "build.gate is off; truthful skipped evidence was recorded and verified (the PR evidence block is untouched — skipped evidence is never woven into a PR body)",
		}
	}
	if facts.build.TestCommand.Value == "" {
		return recertifyRefusal(ResultUnsupportedConfig, ReasonEvidenceUnconfiguredGate,
			"build.gate is local but build.test_command is unconfigured; run `docket repository configure-tests` and review the pending edit", facts.id)
	}

	block, runDir, gref := runRecertifyGate(ctx, deps, repoDir, facts)
	if gref != nil {
		return *gref
	}
	res := publishRecertifiedEvidence(ctx, deps, repoDir, facts, block)
	res.RunDir = runDir
	return res
}

// runRecertifyGate drives the BUILD-owned local gate to a terminal within this
// operation: it advances WAITING slices of the SAME drive (each RunLocalGate
// call blocks for one bounded driver slice; the drive's own observation budget
// turns an overlong run into a running-at-budget halt, so the loop terminates).
// WAITING never means success and never starts a second suite. A PASSED
// terminal returns the canonical evidence block the seam minted through the
// landed build-owned evidence-record path (which re-verifies the head at mint
// time); FAILED is repair work; everything else is a halt — never a fabricated
// red, and never a PR edit.
func runRecertifyGate(ctx context.Context, deps FinalizeDeps, repoDir string, facts recertifyFacts) (block, runDir string, refusal *EvidenceRecertifyResult) {
	if deps.Gate == nil {
		r := recertifyRefusal(ResultInternalError, ReasonRecertifyGateHalted, "no local-gate seam is wired; cannot run the suite", facts.id)
		return "", "", &r
	}
	cont := GateContinuation{}
	for {
		gres, gerr := deps.Gate.RunLocalGate(ctx, LocalGateRequest{
			RepoDir: repoDir, ID: facts.id, WorkspaceDir: facts.wsDir, Head: facts.head, Continuation: cont,
		})
		if gerr != nil {
			r := recertifyRefusal(ResultBlocked, ReasonRecertifyGateHalted,
				"the local gate could not be established; retained, no red fabricated: "+gerr.Error(), facts.id)
			return "", "", &r
		}
		switch gres.Outcome {
		case FinalizeGateWaiting:
			if gres.Continuation.DriveID == "" {
				// A WAITING with no drive handle can never be advanced; fail closed
				// rather than loop forever.
				r := recertifyRefusal(ResultBlocked, ReasonRecertifyGateHalted,
					"the gate reported waiting without a continuation; retained", facts.id)
				return "", "", &r
			}
			cont = gres.Continuation
			continue
		case FinalizeGatePassed:
			return gres.Evidence, gres.RunDir, nil
		case FinalizeGateFailed:
			r := recertifyRefusal(ResultGateFailed, ReasonRecertifyGateFailed,
				"the build suite failed at the current head; this is repair work — no evidence was published", facts.id)
			r.GateOutcome, r.RunDir = string(gres.Outcome), gres.RunDir
			return "", "", &r
		default: // FinalizeGateHalted
			r := recertifyRefusal(ResultBlocked, ReasonRecertifyGateHalted,
				"the build gate did not reach a decidable pass/fail; retained, no red fabricated", facts.id)
			r.GateOutcome, r.HaltCause = string(gres.Outcome), gres.HaltCause
			return "", "", &r
		}
	}
}

// publishRecertifiedEvidence re-proves the whole identity predicate AFTER the
// gate (the same recertifyProbe — a changed head, status, worktree state, or PR
// cannot inherit the pass), re-pins the build command against the evidence, and
// then loss-preservingly replaces ONLY the PR's build-evidence block, exactly
// as FinalizePublish does — without its rebase receipt, PublishRewrite, or
// merge path. A failed or uncertain edit is never reported as completion.
func publishRecertifiedEvidence(ctx context.Context, deps FinalizeDeps, repoDir string, first recertifyFacts, block string) EvidenceRecertifyResult {
	if len(block) > maxAuthoredMarkdownBytes {
		return recertifyRefusal(ResultInvalidInput, ReasonRecertifyEvidenceUnverified,
			fmt.Sprintf("the evidence record is %d bytes, over the %d-byte authored-input bound", len(block), maxAuthoredMarkdownBytes), first.id)
	}
	if v := evidence.Verify([]byte(block), first.head); v != evidence.VerdictVerified {
		return recertifyRefusal(ResultInvalidState, ReasonRecertifyEvidenceUnverified,
			"the minted evidence does not verify green against the tested head ("+string(v)+")", first.id)
	}
	rec, err := evidence.Extract([]byte(block))
	if err != nil {
		// Unreachable after a verified verdict, but fail closed rather than trust it.
		return recertifyRefusal(ResultInvalidState, ReasonRecertifyEvidenceUnverified, err.Error(), first.id)
	}

	// Recheck: the SAME whole predicate that admitted the gate (implemented
	// status, clean workspace, local/remote/PR head agreement, one open PR).
	second, refusal := recertifyProbe(ctx, deps, repoDir, first.id)
	if refusal != nil {
		return *refusal
	}
	if second.head != first.head {
		return recertifyRefusal(ResultContended, ReasonRecertifyIdentityDrift,
			"the feature head moved after the gate; the run no longer certifies the current commit — rerun recertify", first.id)
	}
	if second.pr.Number != first.pr.Number {
		return recertifyRefusal(ResultContended, ReasonRecertifyIdentityDrift,
			"the open pull request changed after the gate; re-read the change state and rerun recertify", first.id)
	}
	// A changed build configuration cannot inherit the pass: the recorded
	// command must be byte-equal to the currently resolved build.test_command.
	if second.build.Gate.Value == "off" || rec.Command == "" || rec.Command != second.build.TestCommand.Value {
		return recertifyRefusal(ResultBlocked, ReasonRecertifyIdentityDrift,
			"the resolved build gate configuration changed after the run (or the evidence names a different command); rerun recertify under the current configuration", first.id)
	}

	newBody, err := evidence.Upsert([]byte(second.pr.Body), rec)
	if err != nil {
		return recertifyRefusal(ResultInvalidState, ReasonRecertifyBodyAssembly, err.Error(), first.id)
	}
	ensurer, ok := deps.GitHub.(finalizePublishEnsurer)
	if !ok {
		return recertifyRefusal(ResultInternalError, ReasonRecertifyEditorUnavailable,
			"the wired GitHub seam does not provide the pull-request edit face", first.id)
	}
	eres, eerr := ensurer.EnsurePullRequest(ctx, githubcli.EnsurePullRequestRequest{
		Repository:      second.repo,
		HeadBranch:      second.branch,
		ExpectedHead:    second.head,
		BaseBranch:      second.pr.BaseBranch,
		Title:           second.pr.Title,
		Body:            string(newBody),
		ExpectedVersion: second.pr.Version,
	})
	if eerr != nil {
		return mapRecertifyEnsureFailure(first.id, second.head, eerr)
	}
	base := EvidenceRecertifyResult{
		ID: first.id, Head: second.head, Outcome: RecertifyOutcomeGreen, Command: rec.Command,
		Number: eres.PR.Number, URL: eres.PR.URL,
	}
	if eres.PR.Number != 0 {
		base.Reference = fmt.Sprintf("%s#%d", second.repo.Spec(), eres.PR.Number)
	}
	switch eres.Disposition {
	case githubcli.EnsureCreated, githubcli.EnsureUpdated:
		base.Envelope = NewEnvelope(OperationEvidenceRecertify, ResultApplied)
		return base
	case githubcli.EnsureAdopted, githubcli.EnsureUnchanged:
		// The PR already carried this exact evidence — an idempotent replay.
		base.Envelope = NewEnvelope(OperationEvidenceRecertify, ResultNoOp)
		base.Message = "the pull request already carries verified evidence for this head"
		return base
	case githubcli.EnsureContended:
		return recertifyRefusal(ResultContended, ReasonRecertifyEditContended,
			"the pull request diverged under the update; rerun recertify", first.id)
	case githubcli.EnsureUnknown:
		return recertifyRefusal(ResultExternalFailed, ReasonRecertifyEditUnknown,
			"the pull-request update could not be verified; retained, no second mutation — rerun recertify to converge", first.id)
	default:
		return recertifyRefusal(ResultInternalError, ReasonStatusInternalError,
			fmt.Sprintf("unexpected pull-request edit disposition %q", eres.Disposition), first.id)
	}
}

// mapRecertifyEnsureFailure folds a githubcli EnsureFailed error onto the
// protocol taxonomy (mirrors mapPublishEnsureFailure's kind mapping; the
// failure's kind is the stable reason and its detail is bounded/redacted).
func mapRecertifyEnsureFailure(id int, head string, err error) EvidenceRecertifyResult {
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
	r := recertifyRefusal(result, reason, message, id)
	r.Head = head
	return r
}
