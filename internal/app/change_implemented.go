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
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// This file is the `change mark-implemented` operation: the final metadata
// mutation in this scope, moving an in-progress change to implemented once — and
// only once — every published external effect is reprobed from its authoritative
// source and proven to still agree. It composes the read-only planning seams,
// the workspace engine (local head), the Git client (remote head), the GitHub
// adapter (the ready PR), and the reparsed build-evidence bytes, then applies the
// landed domain.MarkImplemented action through one atomic, exact-revision
// transaction that records the PR reference, status, updated date, artifact
// block, the verified build evidence (the "## Build evidence" section), inline
// board, and an audit receipt. It does NOT clear the claim, delete
// a branch or workspace, merge the PR, archive the change, or close descendants
// (those are 0316 effects).
//
// The load-bearing property is the FIVE-CONDITION reprobe, all done before the
// transaction opens (so a refusal invokes no engine and writes nothing):
//
//	(1) the change is still the exact in-progress revision, reconciled, and linked
//	    to the verified plan;
//	(2) local and remote feature heads both equal the supplied head;
//	(3) valid build evidence names that head and a passed gate;
//	(4) exactly one open PR for the feature branch targets the resolved
//	    effective-base branch, names that head, and matches the supplied PR number
//	    (parsePRRef reads the --pr assertion as either the full URL or shorthand);
//	(5) a results artifact is attached (required since change 0410, trivial
//	    changes included) and resolves to a file on the metadata branch that
//	    carries this change's backlink and satisfies the final results content
//	    contract.
//
// The transaction records the verified PR's canonical URL (pr.URL) as the manifest
// pr:, the board-safe form; the --pr assertion may be either form. A response-loss
// retry re-reads authority: a change already `implemented` whose recorded PR names
// the same pull request as the request (by number, samePRRef) replays the prior
// applied outcome as a no-op rather than a second transition. Child-agent returns
// are never trusted;
// every condition is verified from Git/GitHub/evidence/metadata.

// OperationChangeMarkImplemented is the operation key the implemented transition
// records in its envelope and transaction trailer.
const OperationChangeMarkImplemented = "change.mark-implemented"

// The closed set of stable machine reasons `change mark-implemented` reports for
// the typed refusals it raises before delegating. Message text is explanatory
// and must not be parsed. Every one of these predates the transaction, so a
// refused call runs no engine and leaves the metadata untouched.
const (
	// ReasonImplementedHeadInvalid: the supplied head is not a full lowercase-hex
	// object id; maps to invalid-input.
	ReasonImplementedHeadInvalid = "head-invalid"
	// ReasonImplementedUnknownChange / -AmbiguousID: the id names no record, or
	// more than one; the operation never chooses.
	ReasonImplementedUnknownChange = "unknown-change"
	ReasonImplementedAmbiguousID   = "ambiguous-change"
	// ReasonImplementedNotInProgress: the change is not in-progress, so it cannot
	// be marked implemented (condition 1); maps to invalid-state.
	ReasonImplementedNotInProgress = "not-in-progress"
	// ReasonImplementedRevisionMismatch: the record moved since the submitted
	// revision (condition 1) — a lost race; maps to a contended outcome.
	ReasonImplementedRevisionMismatch = "revision-mismatch"
	// ReasonImplementedNotReconciled: the change is not reconciled (condition 1).
	ReasonImplementedNotReconciled = "not-reconciled"
	// ReasonImplementedPlanUnlinked: the change carries no linked plan (condition 1).
	ReasonImplementedPlanUnlinked = "plan-unlinked"
	// ReasonImplementedEvidenceUnverified: the reparsed evidence does not verify
	// against the supplied head (condition 3); maps to invalid-state.
	ReasonImplementedEvidenceUnverified = "evidence-unverified"
	// ReasonImplementedLocalHeadMismatch: the workspace's local head differs from
	// the supplied head (condition 2); maps to invalid-state.
	ReasonImplementedLocalHeadMismatch = "local-head-mismatch"
	// ReasonImplementedRemoteHeadMismatch: the remote feature ref is absent or at a
	// different commit than the supplied head (condition 2); maps to invalid-state.
	ReasonImplementedRemoteHeadMismatch = "remote-head-mismatch"
	// ReasonImplementedRemoteProbeFailed: the remote-ref probe itself errored — an
	// errored probe is never clean absence; maps to external-failed.
	ReasonImplementedRemoteProbeFailed = "remote-probe-failed"
	// ReasonImplementedRepositoryUnresolved: the GitHub repository identity could
	// not be resolved from the checkout; maps to external-failed.
	ReasonImplementedRepositoryUnresolved = "repository-unresolved"
	// ReasonImplementedPRProbeFailed: the read-only PR probe errored (condition 4);
	// maps to external-failed.
	ReasonImplementedPRProbeFailed = "pr-probe-failed"
	// ReasonImplementedPRNotUnique: the feature branch has zero or more than one
	// open PR (condition 4) — the operation never chooses; maps to invalid-state.
	ReasonImplementedPRNotUnique = "pr-not-unique"
	// ReasonImplementedPRHeadMismatch: the open PR names a head other than the
	// supplied head (condition 4); maps to invalid-state.
	ReasonImplementedPRHeadMismatch = "pr-head-mismatch"
	// ReasonImplementedPRBaseMismatch: the open PR targets a base other than the
	// resolved effective-base branch (condition 4); maps to invalid-state.
	ReasonImplementedPRBaseMismatch = "pr-base-mismatch"
	// ReasonImplementedPRReferenceMismatch: the verified open PR is not the one the
	// caller supplied by reference (condition 4); maps to invalid-state.
	ReasonImplementedPRReferenceMismatch = "pr-reference-mismatch"
	// ReasonImplementedResultsIdentity: the attached results path no longer resolves
	// to a file on the metadata branch, or its docket:backlink no longer targets
	// this change (condition 5); invalid-state.
	ReasonImplementedResultsIdentity = "results-identity-broken"
	// ReasonImplementedResultsMissing: the change carries no attached results path;
	// a results artifact is REQUIRED at the implemented boundary — trivial changes
	// included (change 0410, condition 5); maps to invalid-state.
	ReasonImplementedResultsMissing = "results-missing"
	// ReasonImplementedResultsInvalid: the attached results artifact fails the FINAL
	// results content contract on the metadata branch (change 0410, condition 5);
	// invalid-state.
	ReasonImplementedResultsInvalid = "results-content-invalid"
)

// MarkImplementedRequest is the closed request for `change mark-implemented`. ID
// and Revision pin the exact record blob; Head is the exact tested feature head;
// PR is the canonical reference `pr publish` returned; EvidenceRecord is the
// canonical build-evidence bytes, reparsed here — never a prior result.
type MarkImplementedRequest struct {
	ID       int    `json:"id" docket:"required"`
	Revision string `json:"revision" docket:"required"`
	Head     string `json:"head" docket:"required"`
	PR       string `json:"pr" docket:"required"`
	// EvidenceRecord is read from the --evidence request file at the CLI boundary,
	// never a JSON key of this request, so it is excluded from the emitted schema
	// (mirroring EvidenceVerifyRequest.RecordFile).
	EvidenceRecord []byte `json:"-" docket:"required"`
}

// ChangeMarkImplemented reprobes the five implemented-transition conditions from
// their authoritative sources and, only when all agree, applies the exact-revision
// transaction that records the implemented transition. It returns a
// ChangeLifecycleResult; a pre-transaction refusal carries the offending
// condition's stable reason as its finding code.
func ChangeMarkImplemented(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, gdeps GitHubDeps, repoDir string, req MarkImplementedRequest) ChangeLifecycleResult {
	op := OperationChangeMarkImplemented

	// (0) Request shape: a positive id, a non-empty revision, a full-hex head, a
	// non-empty PR reference, and non-empty evidence bytes.
	findings := dropFindingCode(validateLifecycleShape("id", req.ID, "", req.Revision), FCEmptyPath)
	if !validFullOID(req.Head) {
		findings = append(findings, lifecycleFinding(FindingCode(ReasonImplementedHeadInvalid), "head must be a full lowercase-hex object id"))
	}
	if strings.TrimSpace(req.PR) == "" {
		findings = append(findings, lifecycleFinding(FCEmptyPR, "pr must be the canonical PR reference from pr publish"))
	}
	if len(req.EvidenceRecord) == 0 {
		findings = append(findings, lifecycleFinding(FCEmptyEvidence, "evidence must be the canonical build-evidence record bytes"))
	}
	if len(findings) > 0 {
		return newChangeLifecycleResult(op, ResultInvalidInput, ChangeLifecycleResult{ID: req.ID, Findings: findings})
	}

	// (Condition 3) Reparse the evidence bytes — never a prior command result — and
	// require them to verify against the supplied head: a missing, malformed, or
	// stale-head record means the gate no longer certifies this commit.
	if verdict := evidence.Verify(req.EvidenceRecord, req.Head); verdict != evidence.VerdictVerified && verdict != evidence.VerdictSkipped {
		return implementedRefusal(ResultInvalidState, ReasonImplementedEvidenceUnverified,
			"the reparsed evidence does not verify (green or skipped) against the supplied head ("+string(verdict)+")", req.ID)
	}
	rec, err := evidence.Extract(req.EvidenceRecord)
	if err != nil {
		// Unreachable after a verified verdict, but fail closed rather than trust it.
		return implementedRefusal(ResultInvalidState, ReasonImplementedEvidenceUnverified, err.Error(), req.ID)
	}

	// Pin authoritative context, check the board surface, discover the repository.
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return implementedRefusal(result, reason, err.Error(), req.ID)
	}
	eff := pin.Config.Effective
	inline, err := resolveBoardSurface(eff)
	if err != nil {
		if pe, ok := asPlanningError(err); ok {
			return implementedRefusal(pe.Result, pe.Reason, pe.Message, req.ID)
		}
		return implementedRefusal(ResultInternalError, ReasonStatusInternalError, err.Error(), req.ID)
	}
	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return implementedRefusal(result, reason, err.Error(), req.ID)
	}

	// Resolve the change record and its current record revision from one corpus read.
	c, recPath, revision, refusal := resolveImplementedChange(ctx, deps, pin, eff, req.ID)
	if refusal != nil {
		return *refusal
	}

	// (Retry) An already-implemented change whose recorded PR names the same pull
	// request as the request is a response-loss replay: return the prior applied
	// outcome as a no-op rather than a second transition. A different recorded PR
	// is a genuine conflict, refused as contended. The comparison is by parsed
	// number (samePRRef) so the recorded canonical URL and a supplied shorthand
	// (or the reverse) read as the same PR — the transition now records the URL
	// form while callers may still assert either.
	if c.Status() == domain.StatusImplemented {
		if samePRRef(c.PR().Value, req.PR) {
			return newChangeLifecycleResult(op, ResultNoOp, ChangeLifecycleResult{ID: req.ID, Status: string(domain.StatusImplemented)})
		}
		return implementedRefusal(ResultContended, ReasonImplementedRevisionMismatch,
			fmt.Sprintf("change %04d is already implemented with a different PR reference", req.ID), req.ID)
	}

	// (Condition 1) exact in-progress revision, reconciled, linked to a plan.
	if c.Status() != domain.StatusInProgress {
		return implementedRefusal(ResultInvalidState, ReasonImplementedNotInProgress,
			fmt.Sprintf("change %04d is %q, not in-progress", req.ID, c.RawStatus()), req.ID)
	}
	if revision != req.Revision {
		return implementedRefusal(ResultContended, ReasonImplementedRevisionMismatch,
			"the change record moved since the submitted revision; re-read authoritative context", req.ID)
	}
	if !c.Reconciled() {
		return implementedRefusal(ResultInvalidState, ReasonImplementedNotReconciled,
			fmt.Sprintf("change %04d is not reconciled; reconcile before marking implemented", req.ID), req.ID)
	}
	if strings.TrimSpace(c.Plan().Value) == "" {
		return implementedRefusal(ResultInvalidState, ReasonImplementedPlanUnlinked,
			fmt.Sprintf("change %04d carries no linked plan; attach the verified plan first", req.ID), req.ID)
	}

	// (Condition 2a) local feature head equals the supplied head; the inspection
	// also yields the feature and effective-base branches the domain resolved.
	insp := WorkspaceInspect(ctx, deps, wdeps, repoDir, WorkspaceIDRequest{ID: req.ID})
	if insp.Result != ResultApplied {
		return implementedRefusal(insp.Result, insp.Reason, insp.Message, req.ID)
	}
	if insp.Head != req.Head {
		return implementedRefusal(ResultInvalidState, ReasonImplementedLocalHeadMismatch,
			"the workspace's local head differs from the supplied head", req.ID)
	}
	featureRef := insp.FeatureRef
	featureBranch := strings.TrimPrefix(featureRef, branchRefPrefix)
	baseBranch := strings.TrimPrefix(insp.BaseRef, branchRefPrefix)

	// (Condition 2b) remote feature head equals the supplied head. An errored probe
	// is never clean absence.
	rref, err := deps.Client.ProbeRemoteBranch(ctx, repo, originRemote, gitcli.RefName(featureRef))
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		if reason == ReasonStatusInternalError {
			reason = ReasonImplementedRemoteProbeFailed
		}
		return implementedRefusal(result, reason, err.Error(), req.ID)
	}
	if rref.State != gitcli.RemoteRefFound || string(rref.Commit) != req.Head {
		return implementedRefusal(ResultInvalidState, ReasonImplementedRemoteHeadMismatch,
			"the remote feature head is absent or differs from the supplied head", req.ID)
	}

	// (Condition 4) exactly one open PR for the feature branch, targeting the
	// resolved effective base, naming the supplied head, and naming the supplied
	// PR number. The adapter's read-only probe mutates nothing.
	ghRepo, err := gdeps.Service.DiscoverRepository(ctx, repoDir)
	if err != nil {
		return implementedRefusal(ResultExternalFailed, ReasonImplementedRepositoryUnresolved, err.Error(), req.ID)
	}
	prs, err := gdeps.Service.FindOpenPullRequestsByHead(ctx, ghRepo, featureBranch)
	if err != nil {
		return mapImplementedGitHubFailure(err, req.ID)
	}
	if len(prs) != 1 {
		return implementedRefusal(ResultInvalidState, ReasonImplementedPRNotUnique,
			fmt.Sprintf("the feature branch has %d open pull requests, want exactly one", len(prs)), req.ID)
	}
	pr := prs[0]
	if pr.HeadCommit != req.Head {
		return implementedRefusal(ResultInvalidState, ReasonImplementedPRHeadMismatch,
			"the open PR names a head other than the supplied head", req.ID)
	}
	if pr.BaseBranch != baseBranch {
		return implementedRefusal(ResultInvalidState, ReasonImplementedPRBaseMismatch,
			"the open PR targets a base other than the resolved effective-base branch", req.ID)
	}
	// Identity is by parsed PR number within the already-resolved repository: the
	// repo was pinned by DiscoverRepository and the PR was found by
	// FindOpenPullRequestsByHead on this feature branch, so the number is a
	// complete discriminator — the owner/repo prefix of the old shorthand compare
	// was redundant with an already-verified fact. Routing through parsePRRef lets
	// the supplied --pr assertion arrive as either the full URL or the shorthand.
	if want, ok := parsePRRef(req.PR); !ok || want != pr.Number {
		return implementedRefusal(ResultInvalidState, ReasonImplementedPRReferenceMismatch,
			"the verified open PR is not the one supplied by reference", req.ID)
	}

	// (Condition 5) a results artifact is REQUIRED at the implemented boundary
	// (change 0410) — for trivial changes too. The attached path must resolve to a
	// file on the pinned metadata tip, carry THIS change's backlink, and satisfy
	// the FINAL results content contract. The feature head is never consulted.
	resultsPath := strings.TrimSpace(c.Results().Value)
	if resultsPath == "" {
		return implementedRefusal(ResultInvalidState, ReasonImplementedResultsMissing,
			fmt.Sprintf("change %04d has no attached results artifact; author and attach the final results before marking implemented", req.ID), req.ID)
	}
	if r := verifyImplementedResults(ctx, deps, pin, resultsPath, c, req.ID); r != nil {
		return *r
	}

	// Every condition holds: open the exact-revision transaction that applies
	// domain.MarkImplemented and re-renders the derived views.
	// Record the verified PR's canonical URL (never the owner/repo#N shorthand):
	// it is the only board-safe form — boardPRCell renders "[#N](url)" from a URL
	// but mangles a shorthand to "#owner/repo#N". The value is sourced from the
	// reprobed snapshot, so the manifest pr: is the canonical URL regardless of
	// which form the --pr assertion arrived in (change 0344).
	txOp := changeImplementedOp{
		changeID:   req.ID,
		pr:         pr.URL,
		eff:        eff,
		clock:      deps.Clock,
		inline:     inline,
		link:       linkContextOf(pin),
		changesDir: eff.ChangesDir.Value,
		evidence:   rec,
	}
	// The run mutation fence (change 0375 Task 11) covers this implemented
	// transition VIA the engine's AdmissionHook — the transition is one
	// Engine.Execute, so a cancelled/superseded run owning this change's
	// worktree refuses it before any Git work. No second admission call is added here
	// (admitWorkflowMutation is not duplicated at engine boundaries).
	res, execErr := deps.Engine.Execute(ctx, transaction.Request{
		Repository: repo,
		Remote:     originRemote,
		TargetRef:  gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName),
		Expected: []transaction.EntityExpectation{{
			Path:     gitcli.RepoPath(recPath),
			Revision: transaction.ExpectedRevision{Kind: transaction.RevisionBlob, ObjectID: gitcli.ObjectID(req.Revision)},
		}},
		Loader:    newPlanningLoader(eff),
		Scope:     changeScope(req.ID, recPath, false),
		Operation: txOp,
	})
	return lifecycleResultFromOutcome(op, res, execErr)
}

// implementedRefusal builds a refusing ChangeLifecycleResult carrying one
// state-shaped finding whose code is the offending condition's stable reason.
func implementedRefusal(result Result, reason, message string, id int) ChangeLifecycleResult {
	return newChangeLifecycleResult(OperationChangeMarkImplemented, result, ChangeLifecycleResult{
		ID:       id,
		Findings: []StatusFinding{lifecycleFinding(FindingCode(reason), message)},
	})
}

// mapImplementedGitHubFailure folds a githubcli failure from the read-only PR
// probe onto the protocol taxonomy. It is always a probe failure (never a
// mutation), so it maps to external-failed under the pr-probe-failed reason.
func mapImplementedGitHubFailure(err error, id int) ChangeLifecycleResult {
	return implementedRefusal(ResultExternalFailed, ReasonImplementedPRProbeFailed, err.Error(), id)
}

// resolveImplementedChange reads the corpus once, builds the snapshot, and
// returns the change named by id together with its current record path and exact
// record revision. An id that names no single record is a typed refusal.
func resolveImplementedChange(ctx context.Context, deps PlanningDeps, pin StatusPin, eff config.Effective, id int) (domain.Change, string, string, *ChangeLifecycleResult) {
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := implementedRefusal(result, reason, err.Error(), id)
		return domain.Change{}, "", "", &r
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		r := implementedRefusal(ResultInternalError, ReasonStatusInternalError, err.Error(), id)
		return domain.Change{}, "", "", &r
	}
	c, out := build.Snapshot.Change(domain.ChangeID(id))
	if out != domain.LookupFound {
		reason, result := ReasonImplementedUnknownChange, ResultInvalidInput
		msg := fmt.Sprintf("no change %04d is present in the corpus", id)
		if out == domain.LookupAmbiguous {
			reason, result = ReasonImplementedAmbiguousID, ResultInvalidState
			msg = fmt.Sprintf("more than one record claims change id %04d; refusing to choose", id)
		}
		r := implementedRefusal(result, reason, msg, id)
		return domain.Change{}, "", "", &r
	}
	revision := ""
	for _, b := range blobs {
		if b.Path == c.Path() {
			revision = b.Revision
			break
		}
	}
	return c, c.Path(), revision, nil
}

// verifyImplementedResults reprobes the attached results artifact on the pinned
// metadata tip against the FINAL results contract (change 0410). It confirms, in
// order: the path still resolves to a file on the metadata branch
// (results-identity-broken); the artifact still carries THIS change's backlink —
// a missing/mismatched block or a malformed managed-block population is broken
// results identity, not a prose defect (results-identity-broken); and the artifact
// satisfies the final results content contract (results-content-invalid, naming
// the first finding). The presence reprobe catches a record whose results file
// was deleted or never written; the backlink and content reprobes catch results
// edited away from the checkpoint attach-results validated.
func verifyImplementedResults(ctx context.Context, deps PlanningDeps, pin StatusPin, resultsPath string, ch domain.Change, id int) *ChangeLifecycleResult {
	art, err := deps.Reader.ReadArtifact(ctx, pin, sourceMetadata, resultsPath)
	if err != nil {
		r := implementedRefusal(ResultInvalidState, ReasonImplementedResultsIdentity, err.Error(), id)
		return &r
	}
	if !art.Found {
		r := implementedRefusal(ResultInvalidState, ReasonImplementedResultsIdentity,
			fmt.Sprintf("the attached results path %q is not a file on the metadata branch", resultsPath), id)
		return &r
	}

	artifactBytes := art.Data

	// The artifact must still carry this change's backlink.
	targets, err := backlinkTargets(artifactBytes, ch, resultsPath)
	if err != nil {
		r := implementedRefusal(ResultInvalidState, ReasonImplementedResultsIdentity,
			fmt.Sprintf("the attached results artifact %q has a malformed or unreadable backlink on the metadata branch: %v", resultsPath, err), id)
		return &r
	}
	if !targets {
		r := implementedRefusal(ResultInvalidState, ReasonImplementedResultsIdentity,
			fmt.Sprintf("the attached results artifact %q does not carry this change's backlink on the metadata branch", resultsPath), id)
		return &r
	}

	// The FINAL results content contract: a substantive ## Outcome and no empty or
	// filler sections. A content-prose defect is distinct from broken identity.
	if fs := ValidateResultsContent(artifactBytes, ResultsPhaseFinal); len(fs) > 0 {
		r := implementedRefusal(ResultInvalidState, ReasonImplementedResultsInvalid,
			fmt.Sprintf("the attached results artifact fails the final content contract: %s: %s", fs[0].Reason, fs[0].Message), id)
		return &r
	}
	return nil
}

// changeImplementedOp is the SemanticOperation the engine drives per attempt. It
// re-runs domain.MarkImplemented against the attempt's own fresh change (so a
// concurrent edit that unreconciled or unlinked the record refuses inside the
// transaction as well as at the pre-transaction reprobe), upserts the owned
// fields (status, pr, plan, updated), re-renders the artifact block, writes the
// verified evidence record as the record's "## Build evidence" section, and —
// when inline is enabled — the board. It never touches claimed_at or branch: the claim
// survives the transition (0316 owns claim release).
type changeImplementedOp struct {
	changeID   int
	pr         string
	eff        config.Effective
	clock      transaction.Clock
	inline     bool
	link       render.LinkContext
	changesDir string
	evidence   evidence.Record
}

func (o changeImplementedOp) Key() transaction.OperationKey {
	return transaction.OperationKey(OperationChangeMarkImplemented)
}

func (o changeImplementedOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot

	c, out := snap.Change(domain.ChangeID(o.changeID))
	if out != domain.LookupFound {
		reason := ReasonImplementedUnknownChange
		if out == domain.LookupAmbiguous {
			reason = ReasonImplementedAmbiguousID
		}
		return refuseLifecycle(FindingCode(reason), fmt.Sprintf("change %04d is not a single record in the current corpus", o.changeID))
	}

	// Domain legality gate re-run against fresh state: MarkImplemented re-checks
	// in-progress + reconciled + a linked plan and yields the exact owned
	// FieldChanges. The plan reference is read from the fresh record, never the
	// request.
	result, fail := domain.MarkImplemented(c, domain.ImplementedFacts{
		PR:   o.pr,
		Plan: c.Plan().Value,
		Now:  o.clock.Now(),
	})
	if fail != nil {
		return refuseLifecyclePolicy(fail)
	}

	src, ok := st.State.Sources[c.Path()]
	if !ok {
		return refuseLifecycle(FCPathMismatch,
			fmt.Sprintf("no record source loaded at %q for change %04d", c.Path(), o.changeID))
	}

	// First patch pass: the domain's owned FieldChanges (status, pr, plan) plus the
	// refreshed updated date. Each field is upserted in its own parse/apply cycle
	// so an inserted absent field (pr on a record that never carried one) never
	// collides with another at the pre-fence insertion point. claimed_at and branch
	// are absent from result.Changed, so they are left byte-identical.
	intermediate := src
	var err error
	for _, fc := range result.Changed {
		intermediate, err = upsertFieldBytes(intermediate, fc.Field, lifecycleFieldValue(fc.To))
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("mark-implemented: patching %s: %w", fc.Field, err)
		}
	}

	candidate, err := buildGroomCandidate(o.eff, st.State.Documents, c.Path(), intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	gc, gout := candidate.Change(domain.ChangeID(o.changeID))
	if gout != domain.LookupFound {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("mark-implemented: mutated record %04d absent from candidate snapshot", o.changeID)
	}

	body, err := render.ArtifactBlockContent(gc, candidate, o.link)
	if err != nil {
		return refuseLifecycle(FCArtifactRenderFailed, err.Error())
	}
	doc2, err := document.Parse(intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("mark-implemented: reparsing patched record: %w", err)
	}
	var ps2 document.PatchSet
	ps2.ReplaceBlock("artifacts", body)
	finalBytes, err := doc2.Apply(ps2)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("mark-implemented: writing artifact block: %w", err)
	}
	// The implemented transition records the evidence it just verified as the
	// record's durable "## Build evidence" section, in this same commit.
	finalBytes, err = UpsertRecordEvidence(finalBytes, o.evidence)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("mark-implemented: recording build evidence: %w", err)
	}

	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: finalBytes},
	}
	if o.inline {
		boardPath := path.Join(o.changesDir, "BOARD.md")
		if err := includeBoard(ctx, st.Tree, boardPath, candidate, boardUnrenderable(st.State, o.changesDir), boardPresentation(o.eff), &files); err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("mark-implemented: %w", err)
		}
	}

	receipt, err := json.Marshal(changeLifecycleReceipt{
		ID: o.changeID, Op: OperationChangeMarkImplemented, Status: string(result.Change.Status()),
	})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("mark-implemented: encoding receipt: %w", err)
	}

	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d → implemented", o.changeID),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}
