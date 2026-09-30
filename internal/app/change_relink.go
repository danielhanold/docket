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

// This file is `change relink`: the revision-pinned relink that finalize's link
// check hands a human's decision to. It writes exactly
// ONE frontmatter field — either branch: (adopt the PR's reported head, the
// missing-branch recovery) or pr: (adopt a PR reference the record's own branch
// vouches for) — plus the standard updated: stamp, and nothing else. Re-probing
// after the relink is the workflow's job (Task 9), not this op's.
//
// Every Expect* field in the request is the exact evidence the human approved.
// The op re-reads authority and refuses on any drift: a change-record revision
// that moved, a PR head that no longer matches, a PR number that no longer
// names the approved pull request — each loses the race and is refused as
// stale-evidence rather than applied on stale facts. Two external truths gate
// the write and are read fail-closed (learning probe-error-is-not-clean-absence):
// the exact PR is read by its recorded number (a view error is pr-unknown, never
// a laundered clean absence), and the candidate branch the record will carry
// must be proven present on the remote before it is adopted (an absent OR
// unprovable branch is candidate-branch-absent). A workspace still owned by this
// change that targets a branch OTHER than the one the record will carry after
// the relink is a conflict the op stops before, in both directions; an inspect
// error is ambiguity and takes the same fail-closed conflict path.

// OperationChangeRelink is the operation key `change relink`
// records in its result envelope and its transaction trailer.
const OperationChangeRelink = "change.relink"

// The closed set of reason tokens `change relink` reports (spec's
// failure vocabulary; every prohibition maps to a return value per learning
// prohibition-needs-a-return-value). Message text is explanatory and must not be
// parsed.
const (
	// RelinkedBranch: the PR's reported head was adopted as branch:.
	RelinkedBranch = "relinked-branch"
	// RelinkedPR: the supplied PR reference was adopted as pr:.
	RelinkedPR = "relinked-pr"
	// RelinkStaleEvidence: the approved evidence lost the race — the change
	// revision, the PR head, or the PR number no longer matches what the human saw.
	RelinkStaleEvidence = "stale-evidence"
	// RelinkWorkspaceConflict: an owned workspace targets a branch other than the
	// one the record will carry, or its inspection could not be answered.
	RelinkWorkspaceConflict = "workspace-conflict"
	// RelinkCandidateBranchAbsent: the branch the record will carry is not proven
	// present on the remote (absent, or an unanswerable probe — never adopted on
	// an unknown).
	RelinkCandidateBranchAbsent = "candidate-branch-absent"
	// RelinkPRUnknown: the exact PR read failed — the pull request could not be
	// authoritatively viewed, so nothing is adopted.
	RelinkPRUnknown = "pr-unknown"
	// RelinkInvalidRequest: the request shape is malformed — not exactly one mode,
	// missing evidence for the chosen mode, or an unparseable PR reference.
	RelinkInvalidRequest = "invalid-request"
)

// RelinkRequest is the revision-pinned relink that finalize's link
// check hands a human's decision to. Exactly one of AdoptPRHead / AdoptPR
// is set. Every Expect* field is the exact evidence the human approved; any
// drift (change revision, PR head, PR number) loses the race and is refused as
// stale-evidence rather than applied.
type RelinkRequest struct {
	ID             int
	ExpectRevision string // change-record revision token from the finalize report

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

// RelinkResult is the protocol-v1 document `change relink`
// returns. It names identity and the closed reason token; a successful relink
// additionally carries the field it wrote and the committed revision. Findings
// marshals as [] on every path.
type RelinkResult struct {
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
// on a relink — the field written and the committed revision.
func (r RelinkResult) HumanText() string {
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

// newRelinkResult stamps the envelope and normalizes Findings so the array
// marshals as [] on every path.
func newRelinkResult(result Result, out RelinkResult) RelinkResult {
	out.Envelope = NewEnvelope(OperationChangeRelink, result)
	if out.Findings == nil {
		out.Findings = []StatusFinding{}
	}
	return out
}

// relinkRefusal builds a refusing result carrying the closed reason token and an
// explanatory message. A refusal mutates nothing.
func relinkRefusal(result Result, reason, message string, id int) RelinkResult {
	return newRelinkResult(result, RelinkResult{ID: id, Reason: reason, Message: message})
}

// changeRelinkReceipt is the canonical receipt persisted with a relink commit.
// Field order is alphabetical for the engine's canonical-form validator.
type changeRelinkReceipt struct {
	Field string `json:"field"`
	ID    int    `json:"id"`
	Op    string `json:"op"`
}

// Relink re-reads the change record and, when every condition the human
// approved still holds, drives one exact-revision transaction that writes the one
// approved identity field. Every refusal predates the transaction (so a refused
// call runs no engine and leaves the metadata untouched); the write is gated on
// the exact PR read, the candidate-branch-present proof (AdoptPRHead), and the
// owned-workspace conflict check, all fail-closed.
func Relink(ctx context.Context, deps FinalizeDeps, repoDir string, req RelinkRequest) RelinkResult {
	// (0) Request shape: exactly one mode, all of that mode's evidence present.
	if reason, msg := validateRelinkRequest(req); reason != "" {
		return relinkRefusal(ResultInvalidInput, reason, msg, req.ID)
	}

	pin, err := deps.Planning.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return relinkRefusal(result, reason, err.Error(), req.ID)
	}
	if decision := config.PreflightMutation(&pin.Config); !decision.Allowed {
		return relinkRefusal(ResultUnsupportedConfig, ReasonDeferredCapRequested,
			"configuration actively requests a deferred capability docket does not ship in this version ("+
				strings.Join(blockerPaths(decision.Blockers), ", ")+"); withdraw it before any mutation", req.ID)
	}
	eff := pin.Config.Effective
	inline, err := resolveBoardSurface(eff)
	if err != nil {
		if pe, ok := asPlanningError(err); ok {
			return relinkRefusal(pe.Result, pe.Reason, pe.Message, req.ID)
		}
		return relinkRefusal(ResultInternalError, ReasonStatusInternalError, err.Error(), req.ID)
	}

	// (1) Re-read the change record. A revision that no longer equals the approved
	// ExpectRevision lost the race — stale-evidence, no write.
	c, recPath, revision, snap, refusal := resolveRelinkChange(ctx, deps.Planning, pin, eff, req.ID)
	if refusal != nil {
		return *refusal
	}
	if revision != req.ExpectRevision {
		return relinkRefusal(ResultContended, RelinkStaleEvidence,
			"the change record moved since the approved revision; re-read authoritative context before repairing", req.ID)
	}

	// Resolve the mode into the exact field the record will carry and the branch
	// that field implies for the workspace gate. Each mode reads its own external
	// authority and refuses fail-closed before naming a write.
	field, value, proposedBranch, modeRefusal := relinkResolveMode(ctx, deps, repoDir, c, req)
	if modeRefusal != nil {
		return *modeRefusal
	}

	// Discover the repository once for the remaining Git-backed gates and the
	// transaction target.
	repo, err := deps.Planning.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return relinkRefusal(result, reason, err.Error(), req.ID)
	}

	// (2, AdoptPRHead) The branch the record will carry must be proven present on
	// the remote before it is adopted — an absent or unanswerable probe is
	// candidate-branch-absent (never adopted on an unknown).
	if field == "branch" {
		if refusal := relinkProveCandidateBranch(ctx, deps.Planning, repo, proposedBranch, req.ID); refusal != nil {
			return *refusal
		}
	}

	// (4) Workspace gate, both directions: an owned workspace targeting a branch
	// other than the proposed one — or an inspection that cannot be answered — is
	// a conflict the op stops before, writing nothing.
	if refusal := relinkProveWorkspaceClear(ctx, deps, pin, snap, repo, c, proposedBranch, req.ID); refusal != nil {
		return *refusal
	}

	// (5) Every condition holds: one exact-revision transaction writes the one
	// approved field plus the refreshed updated stamp — nothing else.
	op := changeRelinkOp{
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
			Path:     gitcli.RepoPath(recPath),
			Revision: transaction.ExpectedRevision{Kind: transaction.RevisionBlob, ObjectID: gitcli.ObjectID(req.ExpectRevision)},
		}},
		Loader:    newPlanningLoader(eff),
		Scope:     changeScope(req.ID, recPath, false),
		Operation: op,
	})
	return relinkResultFromOutcome(field, value, res, execErr, req.ID)
}

// validateRelinkRequest runs the configuration-independent request checks that
// never reach any authority: a positive id, a non-empty approved revision, and
// exactly one mode with all of that mode's evidence present. It returns the
// closed reason token and an explanatory message, or ("", "") when the shape is
// well-formed. The unparseable-PR check for AdoptPR is deferred to the mode
// resolver, which parses it with the ADR-0097 parser.
func validateRelinkRequest(req RelinkRequest) (reason, message string) {
	if req.ID <= 0 {
		return RelinkInvalidRequest, "id must be a positive change id"
	}
	if strings.TrimSpace(req.ExpectRevision) == "" {
		return RelinkInvalidRequest, "expect-revision must be the change-record revision from the finalize report"
	}
	headMode := req.AdoptPRHead
	prMode := strings.TrimSpace(req.AdoptPR) != ""
	if headMode == prMode {
		return RelinkInvalidRequest, "exactly one of adopt-pr-head or adopt-pr must be set"
	}
	if headMode {
		if req.ExpectPRNumber <= 0 {
			return RelinkInvalidRequest, "adopt-pr-head requires a positive expect-pr number"
		}
		if strings.TrimSpace(req.ExpectHead) == "" {
			return RelinkInvalidRequest, "adopt-pr-head requires the approved expect-head branch"
		}
		return "", ""
	}
	if strings.TrimSpace(req.ExpectBranch) == "" {
		return RelinkInvalidRequest, "adopt-pr requires the approved expect-branch"
	}
	return "", ""
}

// resolveRelinkChange reads the corpus once, builds the snapshot, and returns
// the change named by id together with its record path, exact record revision,
// and the built snapshot (the workspace gate resolves the effective base from
// it). An id that names no single record is a request-shaped refusal.
func resolveRelinkChange(ctx context.Context, deps PlanningDeps, pin StatusPin, eff config.Effective, id int) (domain.Change, string, string, domain.Snapshot, *RelinkResult) {
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := relinkRefusal(result, reason, err.Error(), id)
		return domain.Change{}, "", "", domain.Snapshot{}, &r
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		r := relinkRefusal(ResultInternalError, ReasonStatusInternalError, err.Error(), id)
		return domain.Change{}, "", "", domain.Snapshot{}, &r
	}
	c, out := build.Snapshot.Change(domain.ChangeID(id))
	if out != domain.LookupFound {
		msg := fmt.Sprintf("no change %04d is present in the corpus", id)
		if out == domain.LookupAmbiguous {
			msg = fmt.Sprintf("more than one record claims change id %04d; refusing to choose", id)
		}
		r := relinkRefusal(ResultInvalidInput, RelinkInvalidRequest, msg, id)
		return domain.Change{}, "", "", domain.Snapshot{}, &r
	}
	revision := ""
	for _, b := range blobs {
		if b.Path == c.Path() {
			revision = b.Revision
			break
		}
	}
	return c, c.Path(), revision, build.Snapshot, nil
}

// relinkResolveMode reads the mode's external PR authority and, when the
// approved evidence still holds, returns the field the record will carry
// ("branch" or "pr"), the value to write, and the proposed branch the workspace
// gate compares against. It refuses fail-closed on any drift or unreadable
// authority before naming any write.
func relinkResolveMode(ctx context.Context, deps FinalizeDeps, repoDir string, c domain.Change, req RelinkRequest) (field, value, proposedBranch string, refusal *RelinkResult) {
	if req.AdoptPRHead {
		// (2) Trust the PR: read the exact recorded number and adopt its reported
		// head branch — but only if it still matches the approved evidence.
		pr, r := relinkViewPR(ctx, deps, repoDir, req.ExpectPRNumber, req.ID)
		if r != nil {
			return "", "", "", r
		}
		if pr.HeadBranch != req.ExpectHead {
			r := relinkRefusal(ResultContended, RelinkStaleEvidence,
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
		r := relinkRefusal(ResultInvalidInput, RelinkInvalidRequest,
			fmt.Sprintf("adopt-pr reference %q carries no parseable pull-request number", req.AdoptPR), req.ID)
		return "", "", "", &r
	}
	branch, berr := recordedBranch(c)
	if berr != nil || branch != req.ExpectBranch {
		r := relinkRefusal(ResultContended, RelinkStaleEvidence,
			"the record's feature branch no longer equals the approved branch", req.ID)
		return "", "", "", &r
	}
	pr, r := relinkViewPR(ctx, deps, repoDir, number, req.ID)
	if r != nil {
		return "", "", "", r
	}
	if pr.HeadBranch != branch {
		r := relinkRefusal(ResultContended, RelinkStaleEvidence,
			"the supplied PR's head does not equal the recorded branch; it does not prove identity", req.ID)
		return "", "", "", &r
	}
	return "pr", req.AdoptPR, branch, nil
}

// relinkViewPR discovers the GitHub repository and reads exactly one pull
// request by its number. Any repository-resolution or view failure is pr-unknown
// — an errored read is never laundered into a clean absence
// (probe-error-is-not-clean-absence).
func relinkViewPR(ctx context.Context, deps FinalizeDeps, repoDir string, number, id int) (githubPR, *RelinkResult) {
	ghRepo, err := deps.GitHub.DiscoverRepository(ctx, repoDir)
	if err != nil {
		r := relinkRefusal(ResultExternalFailed, RelinkPRUnknown, err.Error(), id)
		return githubPR{}, &r
	}
	pr, err := deps.GitHub.ViewPullRequest(ctx, ghRepo, number)
	if err != nil {
		r := relinkRefusal(ResultExternalFailed, RelinkPRUnknown, err.Error(), id)
		return githubPR{}, &r
	}
	return githubPR{HeadBranch: pr.HeadBranch, HeadCommit: pr.HeadCommit}, nil
}

// githubPR is the narrow slice of a viewed pull request the relink reads: the
// reported head branch and commit. The op adopts the head BRANCH; the commit
// rides for diagnostics only.
type githubPR struct {
	HeadBranch string
	HeadCommit string
}

// relinkProveCandidateBranch proves the branch the record will carry is present
// on the remote (the same remote branch-facts probe reclaim gathers). An absent
// branch, or a probe that cannot be answered, is candidate-branch-absent —
// never adopted on an unknown (probe-error-is-not-clean-absence).
func relinkProveCandidateBranch(ctx context.Context, deps PlanningDeps, repo gitcli.Repository, branch string, id int) *RelinkResult {
	ref := gitcli.RefName(branchRefPrefix + branch)
	rref, err := deps.Client.ProbeRemoteBranch(ctx, repo, originRemote, ref)
	if err != nil {
		r := relinkRefusal(ResultInvalidState, RelinkCandidateBranchAbsent,
			fmt.Sprintf("could not probe remote branch %q; refusing to adopt a branch on an unknown probe", branch), id)
		return &r
	}
	if rref.State != gitcli.RemoteRefFound {
		r := relinkRefusal(ResultInvalidState, RelinkCandidateBranchAbsent,
			fmt.Sprintf("remote branch %q is absent; refusing to adopt a branch the remote does not carry", branch), id)
		return &r
	}
	return nil
}

// relinkProveWorkspaceClear inspects the workspace owned by this change at its
// currently-recorded branch and refuses when that owned workspace targets a
// branch other than the one the record will carry after the relink. A recorded
// branch that is missing/malformed names no branch-keyed workspace to conflict
// (the missing-branch recovery), so the gate passes. An inspection that cannot
// be answered — an unresolved base, a malformed target, or a probe error — is
// ambiguity and takes the fail-closed conflict path (unknown never authorizes a
// write; probe-error-is-not-clean-absence).
func relinkProveWorkspaceClear(ctx context.Context, deps FinalizeDeps, pin StatusPin, snap domain.Snapshot, repo gitcli.Repository, c domain.Change, proposedBranch string, id int) *RelinkResult {
	branch, berr := recordedBranch(c)
	if berr != nil {
		// No resolvable current branch: no branch-keyed owned workspace to conflict.
		return nil
	}
	facts, err := deps.Planning.Reader.BranchFacts(ctx, pin, stackBranchesFor(snap, c))
	if err != nil {
		return relinkConflict(fmt.Sprintf("could not resolve branch facts for change %04d's workspace check", id), id)
	}
	base := domain.ResolveEffectiveBase(snap, c, facts)
	if base.Kind != domain.BaseResolved {
		return relinkConflict(fmt.Sprintf("change %04d's effective base did not resolve to a branch; cannot prove the workspace is clear", id), id)
	}
	target, terr := workspace.NewTarget(c.ID(), c.Slug(), base, branch)
	if terr != nil {
		return relinkConflict(terr.Error(), id)
	}
	insp, err := deps.Workspace.Inspect(ctx, workspace.InspectRequest{Repository: repo, Target: target})
	if err != nil {
		return relinkConflict(err.Error(), id)
	}
	if insp.Kind == workspace.StateForeign || insp.Kind == workspace.StateAbsent {
		// No owned workspace at the recorded branch: nothing to conflict.
		// StateAbsent is foreign-equivalent here — a proven cleanly-absent slot
		// names no owned checkout to orphan, just as an absent manifest inspected
		// as StateForeign did pre-0368 (change 0368).
		return nil
	}
	if target.FeatureBranch() != proposedBranch {
		return relinkConflict(
			fmt.Sprintf("an owned workspace targets %q, not the proposed branch %q; the repair would orphan it", target.FeatureBranch(), proposedBranch), id)
	}
	return nil
}

// relinkConflict builds the workspace-conflict refusal — the fail-closed return
// shared by a proven conflict and every unanswerable inspection.
func relinkConflict(message string, id int) *RelinkResult {
	r := relinkRefusal(ResultInvalidState, RelinkWorkspaceConflict, message, id)
	return &r
}

// relinkResultFromOutcome folds the transaction outcome into the result
// document. An applied outcome is the relink keyed on the field written; a
// contended outcome is the record moving out from under the exact revision
// (stale-evidence); a failure — mid-flight (failed disposition) or the engine's
// early call-shape validation return (empty disposition with an error) — carries
// its typed cause in the envelope's failure diagnosis.
func relinkResultFromOutcome(field, value string, res transaction.Result, execErr error, id int) RelinkResult {
	switch res.Disposition {
	case transaction.DispositionApplied, transaction.DispositionAlreadyApplied:
		// Unrelated grandfathered findings ride along (change 0449); the disposition
		// stays keyed on the engine's, never on the presence of a finding.
		out := RelinkResult{ID: id, Revision: string(res.AppliedCommit), Findings: findingsToStatus(res.Findings)}
		if field == "branch" {
			out.Reason = RelinkedBranch
			out.Branch = value
		} else {
			out.Reason = RelinkedPR
			out.PR = value
		}
		return newRelinkResult(ResultApplied, out)
	case transaction.DispositionContended:
		return newRelinkResult(ResultContended, RelinkResult{
			ID: id, Reason: RelinkStaleEvidence,
			Message: "the change record moved during the repair transaction; re-read authoritative context",
		})
	case transaction.DispositionFailed:
		// A mid-flight transaction failure carries its typed cause in the envelope's
		// failure diagnosis, not a relink reason token.
		r := newRelinkResult(mapFailure(execErr), RelinkResult{
			ID: id, Findings: findingsToStatus(res.Findings),
		})
		r.Failure = failureStatus(res, execErr)
		return r
	default:
		result, _ := mapOutcome(res, execErr, ResultInvalidState)
		out := RelinkResult{ID: id, Findings: findingsToStatus(res.Findings)}
		// Only a refusal's findings name its reason: a no-op may carry unrelated
		// grandfathered findings (change 0449) that are not a relink reason.
		if res.Disposition == transaction.DispositionRefused {
			out.Reason = firstFindingCode(res.Findings)
		}
		r := newRelinkResult(result, out)
		r.Failure = failureStatus(res, execErr)
		return r
	}
}

// changeRelinkOp is the SemanticOperation the engine drives per attempt. It
// upserts the one approved identity field plus the refreshed updated stamp over
// the attempt's own fresh source bytes, re-renders the artifact block against
// the mutated candidate snapshot, and — when inline is enabled — the board. It
// writes NO other frontmatter field: the relink is a single-field
// mutation, and re-probing after it is the workflow's job.
type changeRelinkOp struct {
	changeID   int
	field      string // "branch" or "pr"
	value      string
	eff        config.Effective
	clock      transaction.Clock
	inline     bool
	link       render.LinkContext
	changesDir string
}

func (o changeRelinkOp) Key() transaction.OperationKey {
	return transaction.OperationKey(OperationChangeRelink)
}

func (o changeRelinkOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
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
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("relink: patching %s: %w", o.field, err)
	}
	intermediate, err = upsertFieldBytes(intermediate, "updated", document.String(o.clock.Now().UTC().Format("2006-01-02")))
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("relink: stamping updated: %w", err)
	}

	// The candidate snapshot is the before-state with this record replaced by its
	// mutated bytes: it resolves the artifact block's rows and drives the board.
	candidate, err := buildGroomCandidate(o.eff, st.State.Documents, c.Path(), intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	gc, gout := candidate.Change(domain.ChangeID(o.changeID))
	if gout != domain.LookupFound {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("relink: mutated record %04d absent from candidate snapshot", o.changeID)
	}

	body, err := render.ArtifactBlockContent(gc, candidate, o.link)
	if err != nil {
		return refuseLifecycle(FCArtifactRenderFailed, err.Error())
	}
	doc2, err := document.Parse(intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("relink: reparsing patched record: %w", err)
	}
	var ps2 document.PatchSet
	ps2.ReplaceBlock("artifacts", body)
	finalBytes, err := doc2.Apply(ps2)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("relink: writing artifact block: %w", err)
	}

	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: finalBytes},
	}
	if o.inline {
		boardPath := path.Join(o.changesDir, "BOARD.md")
		if err := includeBoard(ctx, st.Tree, boardPath, candidate, boardUnrenderable(st.State, o.changesDir), boardPresentation(o.eff), &files); err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("relink: %w", err)
		}
	}

	receipt, err := json.Marshal(changeRelinkReceipt{Field: o.field, ID: o.changeID, Op: OperationChangeRelink})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("relink: encoding receipt: %w", err)
	}
	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d relinked (%s)", o.changeID, o.field),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}
