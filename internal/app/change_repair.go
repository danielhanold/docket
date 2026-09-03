package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is `change repair-identity`: the version-pinned identity repair the
// finalize identity checkpoint hands a human's decision to. It writes exactly
// ONE frontmatter field — either branch: (adopt the PR's reported head, the
// missing-branch recovery) or pr: (adopt a PR reference the record's own branch
// vouches for) — plus the standard updated: stamp, and nothing else. Re-probing
// after the repair is the workflow's job (Task 9), not this op's.
//
// Every Expect* field in the request is the exact evidence the human approved.
// The op re-reads authority and refuses on any drift: a change-record version
// that moved, a PR head that no longer matches, a PR number that no longer
// names the approved pull request — each loses the race and is refused as
// stale-evidence rather than applied on stale facts. Two external truths gate
// the write and are read fail-closed (learning probe-error-is-not-clean-absence):
// the exact PR is read by its recorded number (a view error is pr-unknown, never
// a laundered clean absence), and the candidate branch the record will carry
// must be proven present on the remote before it is adopted (an absent OR
// unprovable branch is candidate-branch-absent). A workspace still owned by this
// change that targets a branch OTHER than the one the record will carry after
// the repair is a conflict the op stops before, in both directions; an inspect
// error is ambiguity and takes the same fail-closed conflict path.

// OperationChangeRepairIdentity is the operation key `change repair-identity`
// records in its result envelope and its transaction trailer.
const OperationChangeRepairIdentity = "change.repair-identity"

// The closed set of reason tokens `change repair-identity` reports (spec's
// failure vocabulary; every prohibition maps to a return value per learning
// prohibition-needs-a-return-value). Message text is explanatory and must not be
// parsed.
const (
	// RepairRepairedBranch: the PR's reported head was adopted as branch:.
	RepairRepairedBranch = "repaired-branch"
	// RepairRepairedPR: the supplied PR reference was adopted as pr:.
	RepairRepairedPR = "repaired-pr"
	// RepairStaleEvidence: the approved evidence lost the race — the change
	// version, the PR head, or the PR number no longer matches what the human saw.
	RepairStaleEvidence = "stale-evidence"
	// RepairWorkspaceConflict: an owned workspace targets a branch other than the
	// one the record will carry, or its inspection could not be answered.
	RepairWorkspaceConflict = "workspace-conflict"
	// RepairCandidateBranchAbsent: the branch the record will carry is not proven
	// present on the remote (absent, or an unanswerable probe — never adopted on
	// an unknown).
	RepairCandidateBranchAbsent = "candidate-branch-absent"
	// RepairPRUnknown: the exact PR read failed — the pull request could not be
	// authoritatively viewed, so nothing is adopted.
	RepairPRUnknown = "pr-unknown"
	// RepairInvalidRequest: the request shape is malformed — not exactly one mode,
	// missing evidence for the chosen mode, or an unparseable PR reference.
	RepairInvalidRequest = "invalid-request"
)

// RepairIdentityRequest is the version-pinned identity repair the finalize
// checkpoint hands a human's decision to. Exactly one of AdoptPRHead / AdoptPR
// is set. Every Expect* field is the exact evidence the human approved; any
// drift (change version, PR head, PR number) loses the race and is refused as
// stale-evidence rather than applied.
type RepairIdentityRequest struct {
	ID            int
	ExpectVersion string // change-record version token from the finalize report

	// AdoptPRHead trusts the PR: adopt the exact PR's reported head branch as
	// branch: (the missing/mismatched-branch recovery).
	AdoptPRHead    bool
	ExpectPRNumber int    // the exact PR number the evidence showed
	ExpectHead     string // the head (PR head branch) the human saw and approved

	// AdoptPR trusts the record: adopt this PR reference as pr: (only after an
	// exact read proves its head equals the recorded branch).
	AdoptPR      string
	ExpectBranch string // the recorded branch the human saw
}

// RepairIdentityResult is the protocol-v1 document `change repair-identity`
// returns. It names identity and the closed reason token; a successful repair
// additionally carries the field it wrote and the committed revision. Findings
// marshals as [] on every path.
type RepairIdentityResult struct {
	Envelope
	ID       int             `json:"id,omitempty"`
	Reason   string          `json:"reason,omitempty"`
	Branch   string          `json:"branch,omitempty"`
	PR       string          `json:"pr,omitempty"`
	Revision string          `json:"committed_revision,omitempty"`
	Message  string          `json:"message,omitempty"`
	Findings []StatusFinding `json:"findings"`
}

// HumanText renders a one-line summary naming identity, the reason token, and —
// on a repair — the field written and the committed revision.
func (r RepairIdentityResult) HumanText() string {
	if r.Result == ResultApplied {
		switch {
		case r.Branch != "":
			return fmt.Sprintf("%s: change %04d %s branch %q — %s", r.Operation, r.ID, r.Reason, r.Branch, r.Revision)
		default:
			return fmt.Sprintf("%s: change %04d %s pr %q — %s", r.Operation, r.ID, r.Reason, r.PR, r.Revision)
		}
	}
	return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
}

// newRepairResult stamps the envelope and normalizes Findings so the array
// marshals as [] on every path.
func newRepairResult(result Result, out RepairIdentityResult) RepairIdentityResult {
	out.Envelope = NewEnvelope(OperationChangeRepairIdentity, result)
	if out.Findings == nil {
		out.Findings = []StatusFinding{}
	}
	return out
}

// repairRefusal builds a refusing result carrying the closed reason token and an
// explanatory message. A refusal mutates nothing.
func repairRefusal(result Result, reason, message string, id int) RepairIdentityResult {
	return newRepairResult(result, RepairIdentityResult{ID: id, Reason: reason, Message: message})
}

// changeRepairReceipt is the canonical receipt persisted with a repair commit.
// Field order is alphabetical for the engine's canonical-form validator.
type changeRepairReceipt struct {
	Field string `json:"field"`
	ID    int    `json:"id"`
	Op    string `json:"op"`
}

// RepairIdentity re-reads the change record and, when every conjunct the human
// approved still holds, drives one exact-version transaction that writes the one
// approved identity field. Every refusal predates the transaction (so a refused
// call runs no engine and leaves the metadata untouched); the write is gated on
// the exact PR read, the candidate-branch-present proof (AdoptPRHead), and the
// owned-workspace conflict check, all fail-closed.
func RepairIdentity(ctx context.Context, deps FinalizeDeps, repoDir string, req RepairIdentityRequest) RepairIdentityResult {
	// (0) Request shape: exactly one mode, all of that mode's evidence present.
	if reason, msg := validateRepairRequest(req); reason != "" {
		return repairRefusal(ResultInvalidInput, reason, msg, req.ID)
	}

	pin, err := deps.Planning.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return repairRefusal(result, reason, err.Error(), req.ID)
	}
	if decision := config.PreflightMutation(&pin.Config); !decision.Allowed {
		return repairRefusal(ResultUnsupportedConfig, ReasonDeferredCapRequested,
			"configuration actively requests a deferred capability docket does not ship in this version ("+
				strings.Join(blockerPaths(decision.Blockers), ", ")+"); withdraw it before any mutation", req.ID)
	}
	eff := pin.Config.Effective
	inline, err := fenceBoardSurface(eff)
	if err != nil {
		if pe, ok := asPlanningError(err); ok {
			return repairRefusal(pe.Result, pe.Reason, pe.Message, req.ID)
		}
		return repairRefusal(ResultInternalError, ReasonStatusInternalError, err.Error(), req.ID)
	}

	// (1) Re-read the change record. A version that no longer equals the approved
	// ExpectVersion lost the race — stale-evidence, no write.
	c, recPath, version, snap, refusal := resolveRepairChange(ctx, deps.Planning, pin, eff, req.ID)
	if refusal != nil {
		return *refusal
	}
	if version != req.ExpectVersion {
		return repairRefusal(ResultContended, RepairStaleEvidence,
			"the change record moved since the approved version; re-read authoritative context before repairing", req.ID)
	}

	// Resolve the mode into the exact field the record will carry and the branch
	// that field implies for the workspace gate. Each mode reads its own external
	// authority and refuses fail-closed before naming a write.
	field, value, proposedBranch, modeRefusal := repairResolveMode(ctx, deps, repoDir, c, req)
	if modeRefusal != nil {
		return *modeRefusal
	}

	// Discover the repository once for the remaining Git-backed gates and the
	// transaction target.
	repo, err := deps.Planning.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return repairRefusal(result, reason, err.Error(), req.ID)
	}

	// (2, AdoptPRHead) The branch the record will carry must be proven present on
	// the remote before it is adopted — an absent or unanswerable probe is
	// candidate-branch-absent (never adopted on an unknown).
	if field == "branch" {
		if refusal := repairProveCandidateBranch(ctx, deps.Planning, repo, proposedBranch, req.ID); refusal != nil {
			return *refusal
		}
	}

	// (4) Workspace gate, both directions: an owned workspace targeting a branch
	// other than the proposed one — or an inspection that cannot be answered — is
	// a conflict the op stops before, writing nothing.
	if refusal := repairProveWorkspaceClear(ctx, deps, pin, snap, repo, c, proposedBranch, req.ID); refusal != nil {
		return *refusal
	}

	// (5) Every conjunct holds: one exact-version transaction writes the one
	// approved field plus the refreshed updated stamp — nothing else.
	op := changeRepairOp{
		changeID:   req.ID,
		field:      field,
		value:      value,
		eff:        eff,
		clock:      deps.Planning.Clock,
		inline:     inline,
		link:       linkContextOf(pin),
		changesDir: eff.ChangesDir.Value,
	}
	res, execErr := deps.Planning.Engine.Execute(ctx, transaction.Request{
		Repository: repo,
		Remote:     originRemote,
		TargetRef:  gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName),
		Expected: []transaction.EntityExpectation{{
			Path:    gitcli.RepoPath(recPath),
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(req.ExpectVersion)},
		}},
		Loader:    newPlanningLoader(eff),
		Operation: op,
	})
	return repairResultFromOutcome(field, value, res, execErr, req.ID)
}

// validateRepairRequest runs the configuration-independent request checks that
// never reach any authority: a positive id, a non-empty approved version, and
// exactly one mode with all of that mode's evidence present. It returns the
// closed reason token and an explanatory message, or ("", "") when the shape is
// well-formed. The unparseable-PR check for AdoptPR is deferred to the mode
// resolver, which parses it with the ADR-0097 parser.
func validateRepairRequest(req RepairIdentityRequest) (reason, message string) {
	if req.ID <= 0 {
		return RepairInvalidRequest, "id must be a positive change id"
	}
	if strings.TrimSpace(req.ExpectVersion) == "" {
		return RepairInvalidRequest, "expect-version must be the change-record version token from the finalize report"
	}
	headMode := req.AdoptPRHead
	prMode := strings.TrimSpace(req.AdoptPR) != ""
	if headMode == prMode {
		return RepairInvalidRequest, "exactly one of adopt-pr-head or adopt-pr must be set"
	}
	if headMode {
		if req.ExpectPRNumber <= 0 {
			return RepairInvalidRequest, "adopt-pr-head requires a positive expect-pr number"
		}
		if strings.TrimSpace(req.ExpectHead) == "" {
			return RepairInvalidRequest, "adopt-pr-head requires the approved expect-head branch"
		}
		return "", ""
	}
	if strings.TrimSpace(req.ExpectBranch) == "" {
		return RepairInvalidRequest, "adopt-pr requires the approved expect-branch"
	}
	return "", ""
}

// resolveRepairChange reads the corpus once, builds the snapshot, and returns
// the change named by id together with its record path, exact entity version,
// and the built snapshot (the workspace gate resolves the effective base from
// it). An id that names no single record is a request-shaped refusal.
func resolveRepairChange(ctx context.Context, deps PlanningDeps, pin StatusPin, eff config.Effective, id int) (domain.Change, string, string, domain.Snapshot, *RepairIdentityResult) {
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := repairRefusal(result, reason, err.Error(), id)
		return domain.Change{}, "", "", domain.Snapshot{}, &r
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		r := repairRefusal(ResultInternalError, ReasonStatusInternalError, err.Error(), id)
		return domain.Change{}, "", "", domain.Snapshot{}, &r
	}
	c, out := build.Snapshot.Change(domain.ChangeID(id))
	if out != domain.LookupFound {
		msg := fmt.Sprintf("no change %04d is present in the corpus", id)
		if out == domain.LookupAmbiguous {
			msg = fmt.Sprintf("more than one record claims change id %04d; refusing to choose", id)
		}
		r := repairRefusal(ResultInvalidInput, RepairInvalidRequest, msg, id)
		return domain.Change{}, "", "", domain.Snapshot{}, &r
	}
	version := ""
	for _, b := range blobs {
		if b.Path == c.Path() {
			version = b.Version
			break
		}
	}
	return c, c.Path(), version, build.Snapshot, nil
}

// repairResolveMode reads the mode's external PR authority and, when the
// approved evidence still holds, returns the field the record will carry
// ("branch" or "pr"), the value to write, and the proposed branch the workspace
// gate compares against. It refuses fail-closed on any drift or unreadable
// authority before naming any write.
func repairResolveMode(ctx context.Context, deps FinalizeDeps, repoDir string, c domain.Change, req RepairIdentityRequest) (field, value, proposedBranch string, refusal *RepairIdentityResult) {
	if req.AdoptPRHead {
		// (2) Trust the PR: read the exact recorded number and adopt its reported
		// head branch — but only if it still matches the approved evidence.
		pr, r := repairViewPR(ctx, deps, repoDir, req.ExpectPRNumber, req.ID)
		if r != nil {
			return "", "", "", r
		}
		if pr.HeadBranch != req.ExpectHead {
			r := repairRefusal(ResultContended, RepairStaleEvidence,
				"the PR's reported head branch no longer matches the approved head", req.ID)
			return "", "", "", &r
		}
		return "branch", pr.HeadBranch, pr.HeadBranch, nil
	}

	// (3) Trust the record: parse the supplied PR reference with the ADR-0097
	// parser, require the record's own branch to still equal the approved branch,
	// and prove the exact PR's head equals that recorded branch before adopting
	// the reference as pr:.
	number, ok := parsePRRef(req.AdoptPR)
	if !ok {
		r := repairRefusal(ResultInvalidInput, RepairInvalidRequest,
			fmt.Sprintf("adopt-pr reference %q carries no parseable pull-request number", req.AdoptPR), req.ID)
		return "", "", "", &r
	}
	branch, berr := recordedBranch(c)
	if berr != nil || branch != req.ExpectBranch {
		r := repairRefusal(ResultContended, RepairStaleEvidence,
			"the record's feature branch no longer equals the approved branch", req.ID)
		return "", "", "", &r
	}
	pr, r := repairViewPR(ctx, deps, repoDir, number, req.ID)
	if r != nil {
		return "", "", "", r
	}
	if pr.HeadBranch != branch {
		r := repairRefusal(ResultContended, RepairStaleEvidence,
			"the supplied PR's head does not equal the recorded branch; it does not prove identity", req.ID)
		return "", "", "", &r
	}
	return "pr", req.AdoptPR, branch, nil
}

// repairViewPR discovers the GitHub repository and reads exactly one pull
// request by its number. Any repository-resolution or view failure is pr-unknown
// — an errored read is never laundered into a clean absence
// (probe-error-is-not-clean-absence).
func repairViewPR(ctx context.Context, deps FinalizeDeps, repoDir string, number, id int) (githubPR, *RepairIdentityResult) {
	ghRepo, err := deps.GitHub.DiscoverRepository(ctx, repoDir)
	if err != nil {
		r := repairRefusal(ResultExternalFailed, RepairPRUnknown, err.Error(), id)
		return githubPR{}, &r
	}
	pr, err := deps.GitHub.ViewPullRequest(ctx, ghRepo, number)
	if err != nil {
		r := repairRefusal(ResultExternalFailed, RepairPRUnknown, err.Error(), id)
		return githubPR{}, &r
	}
	return githubPR{HeadBranch: pr.HeadBranch, HeadCommit: pr.HeadCommit}, nil
}

// githubPR is the narrow slice of a viewed pull request the repair reads: the
// reported head branch and commit. The op adopts the head BRANCH; the commit
// rides for diagnostics only.
type githubPR struct {
	HeadBranch string
	HeadCommit string
}

// repairProveCandidateBranch proves the branch the record will carry is present
// on the remote (the same remote branch-facts probe reclaim gathers). An absent
// branch, or a probe that cannot be answered, is candidate-branch-absent —
// never adopted on an unknown (probe-error-is-not-clean-absence).
func repairProveCandidateBranch(ctx context.Context, deps PlanningDeps, repo gitcli.Repository, branch string, id int) *RepairIdentityResult {
	ref := gitcli.RefName(branchRefPrefix + branch)
	rref, err := deps.Client.ProbeRemoteBranch(ctx, repo, originRemote, ref)
	if err != nil {
		r := repairRefusal(ResultInvalidState, RepairCandidateBranchAbsent,
			fmt.Sprintf("could not probe remote branch %q; refusing to adopt a branch on an unknown probe", branch), id)
		return &r
	}
	if rref.State != gitcli.RemoteRefFound {
		r := repairRefusal(ResultInvalidState, RepairCandidateBranchAbsent,
			fmt.Sprintf("remote branch %q is absent; refusing to adopt a branch the remote does not carry", branch), id)
		return &r
	}
	return nil
}

// repairProveWorkspaceClear inspects the workspace owned by this change at its
// currently-recorded branch and refuses when that owned workspace targets a
// branch other than the one the record will carry after the repair. A recorded
// branch that is missing/malformed names no branch-keyed workspace to conflict
// (the missing-branch recovery), so the gate passes. An inspection that cannot
// be answered — an unresolved base, a malformed target, or a probe error — is
// ambiguity and takes the fail-closed conflict path (unknown never authorizes a
// write; probe-error-is-not-clean-absence).
func repairProveWorkspaceClear(ctx context.Context, deps FinalizeDeps, pin StatusPin, snap domain.Snapshot, repo gitcli.Repository, c domain.Change, proposedBranch string, id int) *RepairIdentityResult {
	branch, berr := recordedBranch(c)
	if berr != nil {
		// No resolvable current branch: no branch-keyed owned workspace to conflict.
		return nil
	}
	facts, err := deps.Planning.Reader.BranchFacts(ctx, pin, stackBranches(snap))
	if err != nil {
		return repairConflict(fmt.Sprintf("could not resolve branch facts for change %04d's workspace check", id), id)
	}
	base := domain.ResolveEffectiveBase(snap, c, facts)
	if base.Kind != domain.BaseResolved {
		return repairConflict(fmt.Sprintf("change %04d's effective base did not resolve to a branch; cannot prove the workspace is clear", id), id)
	}
	target, terr := workspace.NewTarget(c.ID(), c.Slug(), base, branch)
	if terr != nil {
		return repairConflict(terr.Error(), id)
	}
	insp, err := deps.Workspace.Inspect(ctx, workspace.InspectRequest{Repository: repo, Target: target})
	if err != nil {
		return repairConflict(err.Error(), id)
	}
	if insp.Kind == workspace.StateForeign {
		// No owned workspace at the recorded branch: nothing to conflict.
		return nil
	}
	if target.FeatureBranch() != proposedBranch {
		return repairConflict(
			fmt.Sprintf("an owned workspace targets %q, not the proposed branch %q; the repair would orphan it", target.FeatureBranch(), proposedBranch), id)
	}
	return nil
}

// repairConflict builds the workspace-conflict refusal — the fail-closed return
// shared by a proven conflict and every unanswerable inspection.
func repairConflict(message string, id int) *RepairIdentityResult {
	r := repairRefusal(ResultInvalidState, RepairWorkspaceConflict, message, id)
	return &r
}

// repairResultFromOutcome folds the transaction outcome into the result
// document. An applied outcome is the repair keyed on the field written; a
// contended outcome is the record moving out from under the exact version
// (stale-evidence); a failure carries its typed cause in the envelope.
func repairResultFromOutcome(field, value string, res transaction.Result, execErr error, id int) RepairIdentityResult {
	switch res.Disposition {
	case transaction.DispositionApplied, transaction.DispositionAlreadyApplied:
		out := RepairIdentityResult{ID: id, Revision: string(res.AppliedCommit)}
		if field == "branch" {
			out.Reason = RepairRepairedBranch
			out.Branch = value
		} else {
			out.Reason = RepairRepairedPR
			out.PR = value
		}
		return newRepairResult(ResultApplied, out)
	case transaction.DispositionContended:
		return newRepairResult(ResultContended, RepairIdentityResult{
			ID: id, Reason: RepairStaleEvidence,
			Message: "the change record moved during the repair transaction; re-read authoritative context",
		})
	case transaction.DispositionFailed:
		// A mid-flight transaction failure carries its typed cause in the envelope's
		// failure diagnosis, not a repair reason token.
		r := newRepairResult(mapFailure(execErr), RepairIdentityResult{
			ID: id, Findings: findingsToStatus(res.Findings),
		})
		r.Failure = failureStatus(res, execErr)
		return r
	default:
		result, _ := mapOutcome(res, execErr, ResultInvalidState)
		return newRepairResult(result, RepairIdentityResult{
			ID: id, Reason: firstFindingCode(res.Findings), Findings: findingsToStatus(res.Findings),
		})
	}
}

// changeRepairOp is the SemanticOperation the engine drives per attempt. It
// upserts the one approved identity field plus the refreshed updated stamp over
// the attempt's own fresh source bytes, re-renders the artifact block against
// the mutated candidate snapshot, and — when inline is enabled — the board. It
// writes NO other frontmatter field: the identity repair is a single-field
// mutation, and re-probing after it is the workflow's job.
type changeRepairOp struct {
	changeID   int
	field      string // "branch" or "pr"
	value      string
	eff        config.Effective
	clock      transaction.Clock
	inline     bool
	link       render.LinkContext
	changesDir string
}

func (o changeRepairOp) Key() transaction.OperationKey {
	return transaction.OperationKey(OperationChangeRepairIdentity)
}

func (o changeRepairOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot

	c, out := snap.Change(domain.ChangeID(o.changeID))
	if out != domain.LookupFound {
		return refuseLifecycle(FCNotFound, fmt.Sprintf("change %04d is not present in the current corpus", o.changeID))
	}

	src, ok := st.State.Sources[c.Path()]
	if !ok {
		return refuseLifecycle(FCPathMismatch,
			fmt.Sprintf("no record source loaded at %q for change %04d", c.Path(), o.changeID))
	}

	// Upsert the one approved field, then the refreshed updated date — each in its
	// own parse/apply cycle so an inserted absent field (a branch that was missing)
	// never collides with the updated stamp at the pre-fence insertion point.
	intermediate, err := upsertFieldBytes(src, o.field, document.String(o.value))
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("repair-identity: patching %s: %w", o.field, err)
	}
	intermediate, err = upsertFieldBytes(intermediate, "updated", document.String(o.clock.Now().UTC().Format("2006-01-02")))
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("repair-identity: stamping updated: %w", err)
	}

	// The candidate snapshot is the before-state with this record replaced by its
	// mutated bytes: it resolves the artifact block's rows and drives the board.
	candidate, err := buildGroomCandidate(o.eff, st.State.Documents, c.Path(), intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	gc, gout := candidate.Change(domain.ChangeID(o.changeID))
	if gout != domain.LookupFound {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("repair-identity: mutated record %04d absent from candidate snapshot", o.changeID)
	}

	body, err := render.ArtifactBlockContent(gc, candidate, o.link)
	if err != nil {
		return refuseLifecycle(FCArtifactRenderFailed, err.Error())
	}
	doc2, err := document.Parse(intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("repair-identity: reparsing patched record: %w", err)
	}
	var ps2 document.PatchSet
	ps2.ReplaceBlock("artifacts", body)
	finalBytes, err := doc2.Apply(ps2)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("repair-identity: writing artifact block: %w", err)
	}

	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: finalBytes},
	}
	if o.inline {
		boardPath := path.Join(o.changesDir, "BOARD.md")
		if err := includeBoard(ctx, st.Tree, boardPath, candidate, boardPresentation(o.eff), &files); err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("repair-identity: %w", err)
		}
	}

	receipt, err := json.Marshal(changeRepairReceipt{Field: o.field, ID: o.changeID, Op: OperationChangeRepairIdentity})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("repair-identity: encoding receipt: %w", err)
	}
	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d identity repaired (%s)", o.changeID, o.field),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}
