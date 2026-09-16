package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/domain"
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
	// Task 3 replaces this fail-closed tail with the gate + publish composition.
	return recertifyRefusal(ResultInternalError, ReasonRecertifyGateHalted,
		"recertify gate composition not yet wired", facts.id)
}
