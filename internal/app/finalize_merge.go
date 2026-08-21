package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is the `finalize merge`: the expected-head GitHub merge of one exact
// pull request, gated on a fresh recheck of every merge conjunct, followed by an
// authoritative post-merge verification that never permits a false closeout.
//
// It is the highest-consequence external effect in the terminal path — a merge
// is never rolled back — so its discipline is severe:
//
//   1. Every merge conjunct (domain.MergeConjuncts) is recomputed from a FRESH
//      reload of the metadata and live GitHub/Git facts IMMEDIATELY before the
//      effect. The first falsified conjunct refuses with its closed token and
//      issues NO merge call. An explicit id (attended, human-named) satisfies
//      the approval and finalize-blocked skips but never a wrong PR identity, an
//      unsafe stack, the repair sign-off (gate), or a superseding version.
//
//   2. `--admin` is honored ONLY on an explicitly-named run and is never
//      inferred — not from an approval absence and not from a permission error.
//      A merge the seam reports denied stays denied; it is never retried with
//      admin.
//
//   3. An already-merged exact PR (probed BEFORE the conjunct recheck, keyed on
//      the promised merged state) is a verified no-op regardless of who merged
//      it — never a second merge.
//
//   4. After the merge (or after a timeout/lost-response reprobe the seam folds
//      into its outcome) the merged facts are verified: the reported head and
//      base must match the authorized head and the resolved effective base, and
//      the Git adapter fetches the destination and proves the reported merge
//      commit reachable from its current remote tip. An open PR is not merged; a
//      divergent head/base is contended; an unobservable result is unknown. None
//      but the reachability proof produces a VerifiedMerge, so none but it
//      permits closeout.
//
// This layer owns the ORDER and the closed-outcome mapping; each mechanic it
// composes is landed: the expected-head merge and the authoritative reprobe live
// in internal/githubcli (MergePullRequest/ProbeMerged, Task 4), and the fetch +
// ancestry proof live in internal/gitcli (FetchBranch/IsAncestor). It holds no
// force-push or rollback escape hatch of its own.

// OperationFinalizeMerge is the operation key `finalize merge` records in its
// result envelope.
const OperationFinalizeMerge = "finalize.merge"

// finalizeBlockedHeading is the durable "## Finalize blocked" section heading a
// change record carries when a prior finalize attempt recorded a block. Task 11
// writes it; this operation only reads its presence, keyed on the heading shape
// (never an enumerated spelling of the interior).
const finalizeBlockedHeading = "Finalize blocked"

// The closed set of `finalize merge` dispositions.
const (
	// MergeDispMerged: this run merged the PR at the expected head and the merge
	// commit was verified reachable from the destination tip.
	MergeDispMerged = "merged"
	// MergeDispAlreadyMerged: the exact PR was already merged (by this run's lost
	// response, a prior run, or a human); a verified no-op, never a second merge.
	MergeDispAlreadyMerged = "already-merged"
	// MergeDispContended: a lost race the caller resolves by re-reading context —
	// a moved head, a divergent merged head/base, or a superseding version.
	MergeDispContended = "contended"
	// MergeDispNotMergeable: the PR cannot be merged (conflicting or closed
	// unmerged); no merge landed.
	MergeDispNotMergeable = "not-mergeable"
	// MergeDispDenied: the merge was issued and authoritatively rejected. It is
	// never retried with admin.
	MergeDispDenied = "denied"
	// MergeDispBlocked: a retained precondition refusal (a falsified conjunct, an
	// admin request without human authorization, an absent canonical PR).
	MergeDispBlocked = "blocked"
	// MergeDispUnknown: an external effect could not be established (a probe error
	// or an unverifiable merge commit). Retained; never permits closeout.
	MergeDispUnknown = "unknown"
)

// The stable machine reasons `finalize merge` reports for its non-conjunct
// outcomes. A conjunct refusal reports the domain conjunct token verbatim
// (domain.MergeConjuncts.FirstFailure). Message text is explanatory and must not
// be parsed.
const (
	// ReasonMergeAdminNotAuthorized: --admin was requested without an explicit,
	// attended human authorization; refused before any effect.
	ReasonMergeAdminNotAuthorized = "admin-requires-explicit-authorization"
	// ReasonMergeNotFinalizable: the change carries no canonical pull-request
	// reference, or is terminal — there is nothing to merge.
	ReasonMergeNotFinalizable = "not-finalizable"
	// ReasonMergeUnresolvedBase: the change's effective base did not resolve to a
	// branch, so no merge destination exists.
	ReasonMergeUnresolvedBase = "unresolved-base"
	// ReasonMergeMalformedTarget: the change identity/base do not form a valid
	// workspace target.
	ReasonMergeMalformedTarget = "malformed-target"
	// ReasonMergeRepoUnresolved: the GitHub repository identity did not resolve.
	ReasonMergeRepoUnresolved = "repository-unresolved"
	// ReasonMergeWorkspaceProbe: the feature workspace could not be inspected for
	// its local head.
	ReasonMergeWorkspaceProbe = "workspace-probe-failed"
	// ReasonMergeRemoteFeatureProbe: the remote feature ref could not be observed.
	ReasonMergeRemoteFeatureProbe = "remote-feature-probe-failed"
	// ReasonMergeRemoteFeatureAbsent: the remote feature ref is absent; there is
	// no published head to merge.
	ReasonMergeRemoteFeatureAbsent = "remote-feature-absent"
	// ReasonMergePRProbeFailed: the live open-PR probe could not be established.
	ReasonMergePRProbeFailed = "pr-probe-failed"
	// ReasonMergePRNotOpen: not exactly one open PR for the feature head.
	ReasonMergePRNotOpen = "pr-not-open"
	// ReasonMergeChildProbeFailed: an open-child PR probe could not be established.
	ReasonMergeChildProbeFailed = "child-probe-failed"
	// ReasonMergeNotMergeable: the seam reports the PR cannot be merged.
	ReasonMergeNotMergeable = "not-mergeable"
	// ReasonMergeDenied: the merge was authoritatively rejected.
	ReasonMergeDenied = "merge-denied"
	// ReasonMergeMethodUnavailable: repository settings and branch rules leave
	// no permitted merge method for this PR's base; observed cleanly, blocked
	// BEFORE any merge command. Not merge-denied (nothing was attempted) and
	// not unknown (the incompatible configuration was observed successfully).
	ReasonMergeMethodUnavailable = "merge-method-unavailable"
	// ReasonMergeProbeUnknown: an external merge/reprobe could not establish the
	// truth; retained, never a closeout-permitting success.
	ReasonMergeProbeUnknown = "merge-probe-unknown"
	// ReasonMergeHeadDivergence: the merged facts report a head other than the
	// authorized head; contended.
	ReasonMergeHeadDivergence = "merged-head-divergence"
	// ReasonMergeBaseDivergence: the merged facts report a base other than the
	// resolved effective base; contended.
	ReasonMergeBaseDivergence = "merged-base-divergence"
	// ReasonMergeUnverified: the merged facts carry no usable merge commit id.
	ReasonMergeUnverified = "merge-unverified"
	// ReasonMergeDestinationProbe: the destination ref could not be fetched or the
	// reachability proof could not run; unknown.
	ReasonMergeDestinationProbe = "destination-probe-failed"
	// ReasonMergeUnreachable: the reported merge commit is not reachable from the
	// destination tip; the merge is not verified on this destination.
	ReasonMergeUnreachable = "merge-commit-unreachable"
	// ReasonMergeHeadMoved: the seam reports the PR head moved from the expected
	// head; no merge landed.
	ReasonMergeHeadMoved = "head-moved"
)

// FinalizeMergeRequest is the closed request for `finalize merge`. ID names the
// change; Version is the exact record blob version from the authoritative
// context read; Head is the exact feature head the merge must match; Admin
// requests an admin-override merge (honored only with ExplicitID); ExplicitID is
// true when a human explicitly named this change (an attended run), which
// supplies the approval and finalize-blocked authorization.
type FinalizeMergeRequest struct {
	ID         int
	Version    string
	Head       string
	Admin      bool
	ExplicitID bool
}

// VerifiedMerge is the authoritative post-merge evidence a successful merge (or
// verified already-merged no-op) carries: the exact PR number/version, the head
// and base at merge, the GitHub mergedAt in UTC, and the merge commit object id
// proven reachable from the destination. Its presence is the ONLY signal that a
// closeout may proceed.
type VerifiedMerge struct {
	PRNumber    int    `json:"pr_number"`
	PRVersion   string `json:"pr_version,omitempty"`
	HeadOID     string `json:"head_oid"`
	BaseRef     string `json:"base_ref"`
	MergedAtUTC string `json:"merged_at_utc"`
	MergeCommit string `json:"merge_commit"`
}

// FinalizeMergeResult is the protocol-v1 document `finalize merge` returns. It
// names identity, the closed disposition, and — on a verified merge — the
// VerifiedMerge evidence; a refusal carries a stable reason and message, and a
// shape refusal carries findings. It leaks no PR body bytes.
type FinalizeMergeResult struct {
	Envelope
	ID          int            `json:"id,omitempty"`
	Disposition string         `json:"disposition,omitempty"`
	Number      int            `json:"number,omitempty"`
	Reference   string         `json:"reference,omitempty"`
	Merge       *VerifiedMerge `json:"merge,omitempty"`
	Reason      string         `json:"reason,omitempty"`
	// Method is the merge method Docket attempted — evidence of Docket's
	// choice, never an inference about how another actor historically merged.
	// Absent when no merge command was issued (already-merged recovery,
	// validation failures, pre-effect blocks).
	Method   string          `json:"method,omitempty"`
	Message  string          `json:"message,omitempty"`
	Findings []StatusFinding `json:"findings"`
}

// HumanText renders a one-line summary naming identity and disposition only —
// never an authored document body.
func (r FinalizeMergeResult) HumanText() string {
	if r.Result == ResultApplied || r.Result == ResultNoOp {
		s := fmt.Sprintf("%s: change %04d %s", r.Operation, r.ID, r.Disposition)
		if r.Merge != nil {
			s += fmt.Sprintf(" #%d (merge %s)", r.Merge.PRNumber, shortCommit(r.Merge.MergeCommit))
		}
		return s
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// newMergeResult stamps the envelope for the finalize.merge operation and
// normalizes the findings collection so a nil never leaks into the document.
func newMergeResult(result Result, out FinalizeMergeResult) FinalizeMergeResult {
	out.Envelope = NewEnvelope(OperationFinalizeMerge, result)
	if out.Findings == nil {
		out.Findings = []StatusFinding{}
	}
	return out
}

// mergeRefusal builds a refusing result carrying a stable reason, message, and
// disposition.
func mergeRefusal(result Result, disposition, reason, message string, id int) FinalizeMergeResult {
	return newMergeResult(result, FinalizeMergeResult{
		ID: id, Disposition: disposition, Reason: reason, Message: message,
	})
}

// mergeConjunctInputs is every fact the merge conjunct assembly reads. It is a
// plain-value bundle so the assembly is pure and directly testable per field.
type mergeConjunctInputs struct {
	status            domain.Status
	canonicalPRNumber int
	prNumber          int
	reqHead           string
	prHead            string
	remoteHead        string
	localHead         string
	prState           githubcli.State
	prDraft           bool
	prBase            string
	effectiveBase     string
	gateOff           bool
	evidenceGreen     bool
	evidenceHead      string
	explicitID        bool
	requireApproval   bool
	// unretargetedOpenChildren is the count of direct stack children whose live PR
	// is open and still targets the parent's feature branch.
	unretargetedOpenChildren int
	versionMatches           bool
	finalizeBlocked          bool
}

// mergeConjuncts assembles the domain merge conjuncts from the live inputs. The
// two human-overridable conjuncts read the explicit-id flag: an explicit id
// supplies approval and satisfies a finalize-blocked marker. A superseding
// version is NEVER overridable — NotSuperseded requires an exact version match
// regardless of authorization.
func mergeConjuncts(in mergeConjunctInputs) domain.MergeConjuncts {
	return domain.MergeConjuncts{
		Implemented:         in.status == domain.StatusImplemented,
		PRIdentityMatch:     in.prNumber == in.canonicalPRNumber,
		HeadsAgree:          in.reqHead == in.prHead && in.reqHead == in.remoteHead && in.reqHead == in.localHead,
		OpenNonDraft:        in.prState == githubcli.StateOpen && !in.prDraft,
		BaseIsEffectiveBase: in.prBase == in.effectiveBase,
		GateSatisfied:       in.gateOff || (in.evidenceGreen && in.evidenceHead == in.reqHead),
		ApprovalSatisfied:   in.explicitID || !in.requireApproval,
		NoOpenChildren:      in.unretargetedOpenChildren == 0,
		NotSuperseded:       in.versionMatches && (in.explicitID || !in.finalizeBlocked),
	}
}

// mergeConjunctOutcome maps a falsified-conjunct token to its result class and
// disposition. A moved head, a wrong PR identity, and a superseding version are
// lost races the caller resolves by re-reading context (contended); every other
// conjunct is a retained block a human resolves.
func mergeConjunctOutcome(token string) (Result, string) {
	switch token {
	case "head-moved", "pr-identity-mismatch", "superseded":
		return ResultContended, MergeDispContended
	default:
		return ResultBlocked, MergeDispBlocked
	}
}

// mergeContext is everything `finalize merge` resolves once from a fresh pin: the
// snapshot (for the stack graph), the resolved change and its exact record
// version and raw body (for the finalize-blocked marker), the resolved effective
// base and validated target, the discovered Git repository, and the effective
// config (for the gate/approval policy).
type mergeContext struct {
	snap    domain.Snapshot
	change  domain.Change
	version string
	body    []byte
	base    domain.EffectiveBase
	target  workspace.Target
	repo    gitcli.Repository
	eff     config.Effective
}

// loadMergeContext performs the fresh reload every merge decision reads from,
// running the capability preflight before any external effect and refusing with
// a typed merge result for every pre-effect condition. It pins once, reads the
// corpus once, and resolves the change/version/body/base/target/repo from that
// one authoritative copy (decide-and-act-on-the-same-copy).
func loadMergeContext(ctx context.Context, deps FinalizeDeps, repoDir string, id int) (*mergeContext, *FinalizeMergeResult) {
	reader := deps.Planning.Reader
	pin, err := reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := mergeRefusal(result, MergeDispBlocked, reason, err.Error(), id)
		return nil, &r
	}
	// Capability preflight before any external effect.
	if decision := config.PreflightMutation(&pin.Config); !decision.Allowed {
		r := newMergeResult(ResultUnsupportedConfig, FinalizeMergeResult{
			ID: id, Reason: ReasonDeferredCapRequested,
			Message: "configuration actively requests a deferred capability docket does not ship in this version (" +
				strings.Join(blockerPaths(decision.Blockers), ", ") + "); withdraw it before any mutation",
		})
		return nil, &r
	}
	eff := pin.Config.Effective

	blobs, err := reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := mergeRefusal(result, MergeDispBlocked, reason, err.Error(), id)
		return nil, &r
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		r := mergeRefusal(ResultInternalError, MergeDispBlocked, ReasonStatusInternalError, err.Error(), id)
		return nil, &r
	}
	snap := build.Snapshot

	c, out := snap.Change(domain.ChangeID(id))
	if out != domain.LookupFound {
		reason, result := ReasonWorkspaceUnknownChange, ResultInvalidInput
		msg := fmt.Sprintf("no change %04d is present in the corpus", id)
		if out == domain.LookupAmbiguous {
			reason, result = ReasonWorkspaceAmbiguousID, ResultInvalidState
			msg = fmt.Sprintf("more than one record claims change id %04d; refusing to choose", id)
		}
		r := mergeRefusal(result, MergeDispBlocked, reason, msg, id)
		return nil, &r
	}

	version, body := "", []byte(nil)
	for _, b := range blobs {
		if b.Path == c.Path() {
			version = b.Version
			body = b.Data
			break
		}
	}

	facts, err := reader.BranchFacts(ctx, pin, stackBranches(snap))
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := mergeRefusal(result, MergeDispBlocked, reason, err.Error(), id)
		return nil, &r
	}
	base := domain.ResolveEffectiveBase(snap, c, facts)
	if base.Kind != domain.BaseResolved {
		r := mergeRefusal(ResultInvalidState, MergeDispBlocked, ReasonMergeUnresolvedBase,
			fmt.Sprintf("change %04d's effective base did not resolve to a branch (kind %q)", id, base.Kind), id)
		return nil, &r
	}
	target, terr := workspace.NewTarget(c.ID(), c.Slug(), base)
	if terr != nil {
		r := mergeRefusal(ResultInvalidInput, MergeDispBlocked, ReasonMergeMalformedTarget, terr.Error(), id)
		return nil, &r
	}

	repo, err := deps.Planning.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		r := mergeRefusal(result, MergeDispBlocked, reason, err.Error(), id)
		return nil, &r
	}

	return &mergeContext{
		snap: snap, change: c, version: version, body: body,
		base: base, target: target, repo: repo, eff: eff,
	}, nil
}

// FinalizeMerge merges one exact pull request at its authorized head after a
// fresh recheck of every merge conjunct, then verifies the merge authoritatively
// against the live PR and the destination Git history. It never rolls back, never
// requests branch deletion (the seam does not), and produces a VerifiedMerge —
// the sole closeout permit — only on a reachable, head/base-consistent merge.
func FinalizeMerge(ctx context.Context, deps FinalizeDeps, repoDir string, req FinalizeMergeRequest) FinalizeMergeResult {
	if findings := validateMergeShape(req); len(findings) > 0 {
		return newMergeResult(ResultInvalidInput, FinalizeMergeResult{ID: req.ID, Findings: findings})
	}
	// Admin is honored ONLY on an attended, explicitly-named run. An admin request
	// without human authorization is refused before any effect — admin is never
	// inferred from anything.
	if req.Admin && !req.ExplicitID {
		return mergeRefusal(ResultBlocked, MergeDispBlocked, ReasonMergeAdminNotAuthorized,
			"--admin requires an attended, explicitly-named run; it is never honored without human authorization", req.ID)
	}

	mc, refusal := loadMergeContext(ctx, deps, repoDir, req.ID)
	if refusal != nil {
		return *refusal
	}
	id := int(mc.change.ID())

	// The canonical PR the manifest tracks. Its absence is not-finalizable — there
	// is no pull request to merge.
	canonicalN, ok := parsePRNumber(mc.change.PR().Value)
	if !finalizeHasPRRef(mc.change) || !ok {
		return mergeRefusal(ResultBlocked, MergeDispBlocked, ReasonMergeNotFinalizable,
			fmt.Sprintf("change %04d carries no canonical pull-request reference to merge", id), id)
	}

	repo, err := deps.GitHub.DiscoverRepository(ctx, repoDir)
	if err != nil {
		return newMergeResult(ResultExternalFailed, FinalizeMergeResult{
			ID: id, Disposition: MergeDispUnknown, Reason: ReasonMergeRepoUnresolved, Message: err.Error(),
		})
	}

	// Already-merged short circuit, keyed on the PROMISED merged state (the
	// authoritative reprobe), not a local proxy. An exact PR already merged — by
	// this run's lost response, a prior run, or a human — is a verified no-op and
	// never a second merge. A probe error is unknown (retain).
	outcome, mfacts, perr := deps.GitHub.ProbeMerged(ctx, repo, canonicalN)
	if perr != nil {
		return newMergeResult(ResultExternalFailed, FinalizeMergeResult{
			ID: id, Disposition: MergeDispUnknown, Number: canonicalN, Reason: ReasonMergeProbeUnknown, Message: perr.Error(),
		})
	}
	if outcome == githubcli.MergeAlreadyMerged {
		// Pre-attempt recovery: Docket issued no merge command, so it attempted no
		// method — pass the empty method through.
		return verifyMerge(ctx, deps, mc, repo, canonicalN, req, mfacts, false, MergeDispAlreadyMerged, ResultNoOp, "")
	}

	// Recheck every conjunct from the fresh reload plus live probes. No merge call
	// is issued until every conjunct holds.
	featureBranch := strings.TrimPrefix(string(mc.target.FeatureRef), branchRefPrefix)
	prs, err := deps.GitHub.FindOpenPullRequestsByHead(ctx, repo, featureBranch)
	if err != nil {
		return newMergeResult(ResultExternalFailed, FinalizeMergeResult{
			ID: id, Disposition: MergeDispUnknown, Number: canonicalN, Reason: ReasonMergePRProbeFailed, Message: err.Error(),
		})
	}
	if len(prs) != 1 {
		return mergeRefusal(ResultBlocked, MergeDispBlocked, ReasonMergePRNotOpen,
			fmt.Sprintf("%d open pull requests for the feature head; a merge requires exactly one open PR", len(prs)), id)
	}
	pr := prs[0]

	rref, err := deps.Planning.Client.ProbeRemoteBranch(ctx, mc.repo, originRemote, mc.target.FeatureRef)
	if err != nil {
		return newMergeResult(ResultExternalFailed, FinalizeMergeResult{
			ID: id, Disposition: MergeDispUnknown, Number: canonicalN, Reason: ReasonMergeRemoteFeatureProbe, Message: err.Error(),
		})
	}
	if rref.State != gitcli.RemoteRefFound {
		return mergeRefusal(ResultBlocked, MergeDispBlocked, ReasonMergeRemoteFeatureAbsent,
			"the remote feature ref is absent; there is no published head to merge", id)
	}

	insp, err := deps.Workspace.Inspect(ctx, workspace.InspectRequest{Repository: mc.repo, Target: mc.target})
	if err != nil {
		return newMergeResult(ResultExternalFailed, FinalizeMergeResult{
			ID: id, Disposition: MergeDispUnknown, Number: canonicalN, Reason: ReasonMergeWorkspaceProbe, Message: err.Error(),
		})
	}

	openChildren, cerr := probeUnretargetedOpenChildren(ctx, deps, repo, mc.snap, mc.change, featureBranch)
	if cerr != nil {
		return newMergeResult(ResultExternalFailed, FinalizeMergeResult{
			ID: id, Disposition: MergeDispUnknown, Number: canonicalN, Reason: ReasonMergeChildProbeFailed, Message: cerr.Error(),
		})
	}

	evHead, evGreen := prBodyEvidence(pr)
	conj := mergeConjuncts(mergeConjunctInputs{
		status:                   mc.change.Status(),
		canonicalPRNumber:        canonicalN,
		prNumber:                 pr.Number,
		reqHead:                  req.Head,
		prHead:                   pr.HeadCommit,
		remoteHead:               string(rref.Commit),
		localHead:                string(insp.HeadCommit),
		prState:                  pr.State,
		prDraft:                  pr.Draft,
		prBase:                   pr.BaseBranch,
		effectiveBase:            mc.base.Branch,
		gateOff:                  mc.eff.Finalize.Gate.Value == "off",
		evidenceGreen:            evGreen,
		evidenceHead:             evHead,
		explicitID:               req.ExplicitID,
		requireApproval:          mc.eff.Finalize.RequirePRApproval.Value,
		unretargetedOpenChildren: len(openChildren),
		versionMatches:           mc.version == req.Version,
		finalizeBlocked:          changeHasFinalizeBlockedMarker(mc.body),
	})
	if token := conj.FirstFailure(); token != "" {
		result, disp := mergeConjunctOutcome(token)
		return mergeRefusal(result, disp, token, mergeConjunctMessage(token, id), id)
	}

	// Every conjunct holds. Issue the expected-head merge. Admin is honored only
	// with an explicit id (already gated above) — the AND is belt-and-braces so no
	// path can pass admin without it.
	admin := req.Admin && req.ExplicitID
	mres, merr := deps.GitHub.MergePullRequest(ctx, repo, canonicalN, githubcli.ObjectRef(req.Head), admin)
	if merr != nil {
		// A transport/launch failure is unknown — the merge may or may not have
		// landed; retain and reprobe on the next run, never fabricate a result.
		return newMergeResult(ResultExternalFailed, FinalizeMergeResult{
			ID: id, Disposition: MergeDispUnknown, Number: canonicalN, Reason: ReasonMergeProbeUnknown, Message: merr.Error(),
		})
	}
	switch mres.Outcome {
	case githubcli.MergeMerged:
		return verifyMerge(ctx, deps, mc, repo, canonicalN, req, mres.Facts, true, MergeDispMerged, ResultApplied, string(mres.Method))
	case githubcli.MergeAlreadyMerged:
		return verifyMerge(ctx, deps, mc, repo, canonicalN, req, mres.Facts, false, MergeDispAlreadyMerged, ResultNoOp, string(mres.Method))
	case githubcli.MergeMethodUnavailable:
		// Repository settings and branch rules left no permitted method for this
		// PR's base; the incompatible policy was observed cleanly and NO merge was
		// issued. A pre-effect block, never merge-denied and never unknown.
		return mergeRefusal(ResultBlocked, MergeDispBlocked, ReasonMergeMethodUnavailable,
			fmt.Sprintf("no merge method is permitted for this PR's base: repository enables %v, branch rules permit %v; align the repository merge settings with the branch rules", mres.RepoMethods, mres.BranchMethods), id)
	case githubcli.MergeHeadMoved:
		return mergeRefusal(ResultContended, MergeDispContended, ReasonMergeHeadMoved,
			"the pull request head moved from the expected head; re-read context finalize", id)
	case githubcli.MergeNotMergeable:
		return mergeRefusal(ResultBlocked, MergeDispNotMergeable, ReasonMergeNotMergeable,
			"the pull request cannot be merged (conflicting or closed unmerged)", id)
	case githubcli.MergeDenied:
		r := mergeRefusal(ResultExternalFailed, MergeDispDenied, ReasonMergeDenied,
			"the merge was authoritatively rejected; it is never retried with admin", id)
		r.Method = string(mres.Method)
		return r
	case githubcli.MergeUnknown:
		return newMergeResult(ResultExternalFailed, FinalizeMergeResult{
			ID: id, Disposition: MergeDispUnknown, Number: canonicalN, Reason: ReasonMergeProbeUnknown,
			Message: "the merge outcome could not be verified; retained, no closeout",
		})
	default:
		// An outcome outside the closed set is a fail-closed internal-error, never
		// a permissive fall-through.
		return mergeRefusal(ResultInternalError, MergeDispBlocked, ReasonStatusInternalError,
			fmt.Sprintf("unexpected merge outcome %q", mres.Outcome), id)
	}
}

// verifyMerge establishes the authoritative post-merge truth. It requires the
// merged facts to name the resolved effective base, requires the merged head to
// equal the authorized head when this run performed the merge (an already-merged
// PR carries whatever head was merged, by anyone), and proves the reported merge
// commit reachable from the freshly-fetched destination tip. Only a fully proven
// merge yields a VerifiedMerge; every failure is contended or unknown and carries
// no closeout permit.
func verifyMerge(ctx context.Context, deps FinalizeDeps, mc *mergeContext, repo githubcli.Repository, number int, req FinalizeMergeRequest, facts githubcli.MergedFacts, requireHead bool, disp string, success Result, method string) FinalizeMergeResult {
	id := int(mc.change.ID())
	if requireHead && facts.HeadOID != req.Head {
		return mergeRefusal(ResultContended, MergeDispContended, ReasonMergeHeadDivergence,
			"the verified merge reports a head other than the authorized head; re-read context finalize", id)
	}
	if facts.BaseRef != mc.base.Branch {
		return mergeRefusal(ResultContended, MergeDispContended, ReasonMergeBaseDivergence,
			fmt.Sprintf("the verified merge targets base %q, not the resolved effective base %q", facts.BaseRef, mc.base.Branch), id)
	}
	if !validFullObjectID(facts.MergeCommit) {
		return newMergeResult(ResultExternalFailed, FinalizeMergeResult{
			ID: id, Disposition: MergeDispUnknown, Number: number, Reason: ReasonMergeUnverified,
			Message: "the verified merge carries no usable merge commit id; retained, no closeout",
		})
	}

	// Fetch the destination and prove the merge commit reachable from its current
	// remote tip. A fetch or ancestry probe error is unknown; a clean unreachable
	// answer is contended (the reported merge is not on this destination). Neither
	// permits closeout.
	rev, err := deps.Planning.Client.FetchBranch(ctx, mc.repo, originRemote, mc.target.BaseRef)
	if err != nil {
		return newMergeResult(ResultExternalFailed, FinalizeMergeResult{
			ID: id, Disposition: MergeDispUnknown, Number: number, Reason: ReasonMergeDestinationProbe, Message: err.Error(),
		})
	}
	reachable, err := deps.Planning.Client.IsAncestor(ctx, mc.repo, gitcli.ObjectID(facts.MergeCommit), rev.Commit)
	if err != nil {
		return newMergeResult(ResultExternalFailed, FinalizeMergeResult{
			ID: id, Disposition: MergeDispUnknown, Number: number, Reason: ReasonMergeDestinationProbe, Message: err.Error(),
		})
	}
	if !reachable {
		return mergeRefusal(ResultContended, MergeDispContended, ReasonMergeUnreachable,
			"the reported merge commit is not reachable from the destination tip; the merge is not verified on this destination", id)
	}

	return newMergeResult(success, FinalizeMergeResult{
		ID:          id,
		Disposition: disp,
		Number:      number,
		Reference:   fmt.Sprintf("%s#%d", repo.Spec(), number),
		Method:      method,
		Merge: &VerifiedMerge{
			PRNumber:    number,
			PRVersion:   facts.Version,
			HeadOID:     facts.HeadOID,
			BaseRef:     facts.BaseRef,
			MergedAtUTC: facts.MergedAtUTC,
			MergeCommit: facts.MergeCommit,
		},
	})
}

// probeUnretargetedOpenChildren returns the direct stack children whose live PR
// is open and still targets the parent's feature branch — the exact set a merge
// of the parent must NOT strand. A retargeted child (its PR now targets some
// other base) does not block; a terminal child has no open PR. A probe error is
// returned so the caller retains it as unknown (never a clean empty set).
func probeUnretargetedOpenChildren(ctx context.Context, deps FinalizeDeps, repo githubcli.Repository, snap domain.Snapshot, parent domain.Change, parentFeatureBranch string) ([]int, error) {
	var open []int
	for _, cid := range domain.StackChildren(snap, parent.ID()) {
		child, lookup := snap.Change(cid)
		if lookup != domain.LookupFound {
			continue
		}
		head := domain.BranchForSlug(child.Slug())
		prs, err := deps.GitHub.FindOpenPullRequestsByHead(ctx, repo, head)
		if err != nil {
			return nil, err
		}
		for _, pr := range prs {
			if pr.State == githubcli.StateOpen && pr.BaseBranch == parentFeatureBranch {
				open = append(open, int(cid))
				break
			}
		}
	}
	return open, nil
}

// changeHasFinalizeBlockedMarker reports whether a change record body carries a
// durable "## Finalize blocked" section. It keys on the heading SHAPE — a
// markdown heading line whose text is exactly the block heading — not on the
// interior spelling, so a re-marked block with new interior still reads as
// blocked (AGENTS.md: key a guard on shape, not an enumerated list).
func changeHasFinalizeBlockedMarker(body []byte) bool {
	for _, line := range strings.Split(string(body), "\n") {
		if headingText(line) == finalizeBlockedHeading {
			return true
		}
	}
	return false
}

// headingText returns the trimmed text of a markdown ATX heading line (one or
// more leading '#'), or "" when the line is not a heading. It is the shape probe
// changeHasFinalizeBlockedMarker keys on.
func headingText(line string) string {
	trimmed := strings.TrimSpace(line)
	hashes := 0
	for hashes < len(trimmed) && trimmed[hashes] == '#' {
		hashes++
	}
	if hashes == 0 || hashes == len(trimmed) || trimmed[hashes] != ' ' {
		return ""
	}
	return strings.TrimSpace(trimmed[hashes:])
}

// mergeConjunctMessage renders the explanatory (non-parsed) message for a
// falsified conjunct token.
func mergeConjunctMessage(token string, id int) string {
	switch token {
	case "not-implemented":
		return fmt.Sprintf("change %04d is not implemented; there is nothing to merge", id)
	case "pr-identity-mismatch":
		return "the live open PR is not the canonical PR the manifest tracks; re-read context finalize"
	case "head-moved":
		return "the local workspace, remote feature ref, PR head, and requested head do not all agree; re-read context finalize"
	case "not-open-nondraft":
		return "the pull request is not open and non-draft"
	case "base-mismatch":
		return "the pull request does not target the resolved effective base; retarget or rebase first"
	case "gate-unsatisfied":
		return "the local gate is not satisfied by exact-head green evidence; re-run the gate for this head"
	case "approval-required":
		return "approval is required and was not supplied; an explicit id authorizes an attended merge"
	case "open-children":
		return "an open child PR still targets this change's feature branch; retarget children first"
	case "superseded":
		return "the request was superseded by a newer record version or a finalize-blocked marker; re-read context finalize"
	default:
		return "a merge precondition does not hold"
	}
}

// validateMergeShape runs the configuration-independent request checks for
// `finalize merge`: a positive id, a non-empty pinned version, and a valid
// full-length object id for the expected head.
func validateMergeShape(req FinalizeMergeRequest) []StatusFinding {
	findings := dropFindingCode(validateLifecycleShape(req.ID, "", req.Version), "empty-path")
	if !validFullObjectID(req.Head) {
		findings = append(findings, lifecycleFinding("invalid-head",
			"head must be a full 40- or 64-character lowercase hex object id"))
	}
	return findings
}
