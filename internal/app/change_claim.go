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
)

// This file is the `change claim` and `change refresh-claim` planning
// operations: two exact-version metadata transitions that land a change's claim
// fields and every affected v1-owned derived view (the change record's owned
// lifecycle fields, its refreshed updated date, its re-rendered artifact block,
// and the inline board) as one validated atomic transaction. The domain owns
// legality: domain.Claim and domain.RefreshClaim decide whether the current
// status may take the transition and yield the exact FieldChanges to apply.
//
// Claim additionally re-proves eligibility INSIDE the transaction against the
// attempt's own fresh snapshot — proposed, uniquely identified, build-ready,
// and a resolved effective base — through domain.ClaimEligibility, so a claim
// that lost a race against a concurrent edit of the surrounding corpus is
// refused rather than applied on stale facts. That in-transaction re-proof is
// the load-bearing guard the refusal table mutation-tests.
//
// Claim carries an idempotency key derived from its own (id, version) request so
// a lost response replays the original applied receipt as `already-claimed`
// rather than allocating a second claim; a foreign edit that moved the record
// fails the exact-version expectation as `contended`. Refresh is a small
// non-keyed exact-version transaction — a version mismatch is `contended`, which
// instructs the caller to stop, never to overwrite a newer record. Neither
// operation clears or steals an expired claim; reclaim is 0316.

// The operation keys the two claim transitions record in their result envelopes
// and transaction trailers.
const (
	OperationChangeClaim        = "change.claim"
	OperationChangeRefreshClaim = "change.refresh-claim"
)

// The closed set of claim dispositions a result may carry. `applied` is a fresh
// claim; `already-claimed` is an idempotent replay of this exact request's own
// prior claim; `contended` is a lost race the caller must not overwrite;
// `failed` is a transaction that failed mid-flight, its cause carried in the
// envelope's failure field; any other value is a policy refusal reason token
// (e.g. not-ready-<kind>).
const (
	ClaimDispositionApplied        = "applied"
	ClaimDispositionAlreadyClaimed = "already-claimed"
	ClaimDispositionContended      = "contended"
	// ClaimDispositionFailed is a transaction that failed mid-flight; the
	// cause is carried in the envelope's failure field.
	ClaimDispositionFailed = "failed"
)

// ChangeClaimRequest is the closed, caller-supplied request for one claim or
// refresh-claim. ID names the change; Version pins the exact submitted record
// blob (the version the authoritative context read reported).
type ChangeClaimRequest struct {
	ID      int    `json:"id"`
	Version string `json:"version"`
}

// claimDigestPayload is the semantic content of a claim request — everything
// that identifies the record being claimed. It is the idempotency digest
// payload, so a lost-response retry of the same request replays rather than
// re-allocates.
type claimDigestPayload struct {
	ID      int    `json:"id"`
	Version string `json:"version"`
}

// ChangeClaimResult is the protocol-v1 document `change claim` and
// `change refresh-claim` return. It embeds the envelope; the claim fields are
// populated on a successful apply or an idempotent replay, Disposition carries
// the closed claim disposition, and Findings carries every refusal or validation
// diagnostic (marshalled as [] never null).
type ChangeClaimResult struct {
	Envelope
	ID          int             `json:"id,omitempty"`
	Status      string          `json:"status,omitempty"`
	Branch      string          `json:"branch,omitempty"`
	ClaimedAt   string          `json:"claimed_at,omitempty"`
	Lease       string          `json:"lease,omitempty"`
	Revision    string          `json:"committed_revision,omitempty"`
	Disposition string          `json:"disposition,omitempty"`
	Findings    []StatusFinding `json:"findings"`
}

// HumanText renders the one-line human summary of a claim outcome. It names
// identity, branch, and disposition only — no authored document body.
func (r ChangeClaimResult) HumanText() string {
	switch r.Result {
	case ResultApplied:
		return fmt.Sprintf("change %04d %s (%s) lease %s — %s", r.ID, r.Disposition, r.Branch, r.Lease, r.Revision)
	default:
		disp := r.Disposition
		if disp == "" {
			disp = string(r.Result)
		}
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, disp)
	}
}

// newChangeClaimResult stamps the envelope for opKey and normalizes Findings to
// an empty slice so the array marshals as [] on every path.
func newChangeClaimResult(opKey string, result Result, r ChangeClaimResult) ChangeClaimResult {
	r.Envelope = NewEnvelope(opKey, result)
	if r.Findings == nil {
		r.Findings = []StatusFinding{}
	}
	return r
}

// changeClaimReceipt is the canonical receipt persisted with a claim commit and
// replayed verbatim on an idempotent re-run. Field order is alphabetical so
// json.Marshal emits the canonical, sorted-key compact form the engine's receipt
// validator requires.
type changeClaimReceipt struct {
	Branch    string `json:"branch"`
	ClaimedAt string `json:"claimed_at"`
	ID        int    `json:"id"`
	Lease     string `json:"lease"`
	Op        string `json:"op"`
	Status    string `json:"status"`
}

// ChangeClaim validates the request, pins authoritative context, fetches the
// branch facts effective-base resolution consults, and drives one atomic,
// idempotency-keyed transaction that claims the change and — when inline is
// enabled — re-renders the board. Every failure that predates the transaction
// (bad request shape, a github board surface, a facts-read failure) returns
// without an engine call.
func ChangeClaim(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeClaimRequest) ChangeClaimResult {
	findings := validateLifecycleShape(req.ID, "", req.Version)
	// The claim request carries no path; validateLifecycleShape flags an empty
	// path, which is irrelevant here — drop that one finding.
	findings = dropFindingCode(findings, "empty-path")
	if len(findings) > 0 {
		return newChangeClaimResult(OperationChangeClaim, ResultInvalidInput, ChangeClaimResult{Findings: findings})
	}

	pin, eff, inline, repo, pre := claimPreflight(ctx, deps, repoDir, OperationChangeClaim)
	if pre != nil {
		return *pre
	}

	// Resolve the record's current path and the branch facts the in-transaction
	// eligibility re-proof consults, from one pre-read of the corpus. The request
	// carries only (id, version); the path is derived here so the exact-version
	// expectation can pin the record, and the facts feed effective-base
	// resolution (an unstacked change resolves without them). This pre-read is a
	// supporting observation; the authoritative record state is re-read fresh
	// inside the transaction.
	recPath, facts, terr := resolveClaimTarget(ctx, deps, pin, eff, req.ID, OperationChangeClaim)
	if terr != nil {
		return *terr
	}

	digest, derr := canonicalDigest(OperationChangeClaim, claimDigestPayload{ID: req.ID, Version: req.Version})
	if derr != nil {
		return newChangeClaimResult(OperationChangeClaim, ResultInternalError,
			ChangeClaimResult{Findings: []StatusFinding{lifecycleFinding(ReasonStatusInternalError, derr.Error())}})
	}

	op := changeClaimOp{
		opKey:      OperationChangeClaim,
		changeID:   req.ID,
		facts:      facts,
		eff:        eff,
		clock:      deps.Clock,
		ttlHours:   eff.Reclaim.LeaseTTL.Value,
		inline:     inline,
		link:       linkContextOf(pin),
		changesDir: eff.ChangesDir.Value,
	}

	res, execErr := deps.Engine.Execute(ctx, transaction.Request{
		Repository: repo,
		Remote:     originRemote,
		TargetRef:  gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName),
		Expected: []transaction.EntityExpectation{{
			Path:    gitcli.RepoPath(recPath),
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(req.Version)},
		}},
		Idempotency: &transaction.IdempotencyKey{RequestID: claimRequestID(req), Digest: digest},
		Loader:      newPlanningLoader(eff),
		Operation:   op,
	})

	return claimResultFromOutcome(OperationChangeClaim, res, execErr)
}

// ChangeRefreshClaim validates the request, pins authoritative context, and
// drives one small non-keyed exact-version transaction that re-stamps the
// change's claimed_at (plus its refreshed updated date and derived views) and
// nothing else. It requires the change to still be in-progress at the exact
// submitted version; a mismatch is `contended`, which stops the run rather than
// overwriting a newer record.
func ChangeRefreshClaim(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeClaimRequest) ChangeClaimResult {
	findings := dropFindingCode(validateLifecycleShape(req.ID, "", req.Version), "empty-path")
	if len(findings) > 0 {
		return newChangeClaimResult(OperationChangeRefreshClaim, ResultInvalidInput, ChangeClaimResult{Findings: findings})
	}

	pin, eff, inline, repo, pre := claimPreflight(ctx, deps, repoDir, OperationChangeRefreshClaim)
	if pre != nil {
		return *pre
	}

	// Resolve the record's current path (the request carries only id + version).
	// Refresh consults no branch facts — it re-proves nothing about readiness —
	// so the resolved facts are discarded.
	recPath, _, terr := resolveClaimTarget(ctx, deps, pin, eff, req.ID, OperationChangeRefreshClaim)
	if terr != nil {
		return *terr
	}

	op := changeClaimOp{
		opKey:      OperationChangeRefreshClaim,
		changeID:   req.ID,
		refresh:    true,
		eff:        eff,
		clock:      deps.Clock,
		ttlHours:   eff.Reclaim.LeaseTTL.Value,
		inline:     inline,
		link:       linkContextOf(pin),
		changesDir: eff.ChangesDir.Value,
	}

	res, execErr := deps.Engine.Execute(ctx, transaction.Request{
		Repository: repo,
		Remote:     originRemote,
		TargetRef:  gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName),
		Expected: []transaction.EntityExpectation{{
			Path:    gitcli.RepoPath(recPath),
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(req.Version)},
		}},
		Loader:    newPlanningLoader(eff),
		Operation: op,
	})

	return claimResultFromOutcome(OperationChangeRefreshClaim, res, execErr)
}

// claimPreflight performs the shared pre-transaction plumbing both claim
// transitions need: pin context, fence the board surface, and discover the
// repository. It returns a non-nil result pointer for any preflight failure.
func claimPreflight(ctx context.Context, deps PlanningDeps, repoDir, opKey string) (
	pin StatusPin, eff config.Effective, inline bool, repo gitcli.Repository, refusal *ChangeClaimResult) {

	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := newChangeClaimResult(opKey, result, ChangeClaimResult{Findings: []StatusFinding{lifecycleFinding(reason, err.Error())}})
		return pin, eff, false, repo, &r
	}
	eff = pin.Config.Effective

	inline, err = fenceBoardSurface(eff)
	if err != nil {
		if pe, ok := asPlanningError(err); ok {
			r := newChangeClaimResult(opKey, pe.Result, ChangeClaimResult{Findings: []StatusFinding{lifecycleFinding(pe.Reason, pe.Message)}})
			return pin, eff, false, repo, &r
		}
		r := newChangeClaimResult(opKey, ResultInternalError, ChangeClaimResult{Findings: []StatusFinding{lifecycleFinding(ReasonStatusInternalError, err.Error())}})
		return pin, eff, false, repo, &r
	}

	// Capability preflight: a configuration that actively requests a deferred
	// capability docket does not ship in this version blocks every metadata
	// mutation before its transaction (spec "Failure, concurrency, and security
	// rules"; maps to unsupported-config). It runs here, before Discover and the
	// engine, so a blocked configuration writes nothing.
	if decision := config.PreflightMutation(&pin.Config); !decision.Allowed {
		r := newChangeClaimResult(opKey, ResultUnsupportedConfig, ChangeClaimResult{
			Findings: []StatusFinding{lifecycleFinding(ReasonDeferredCapRequested,
				"configuration actively requests a deferred capability docket does not ship in this version ("+
					strings.Join(blockerPaths(decision.Blockers), ", ")+"); withdraw it before any mutation")},
		})
		return pin, eff, false, repo, &r
	}

	repo, err = deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		r := newChangeClaimResult(opKey, result, ChangeClaimResult{Findings: []StatusFinding{lifecycleFinding(reason, err.Error())}})
		return pin, eff, false, repo, &r
	}
	return pin, eff, inline, repo, nil
}

// resolveClaimTarget reads the metadata corpus once, resolves the target
// change's current canonical record path, and asks the reader which
// stack-ancestor feature branches exist on the remote (the facts effective-base
// resolution consults). An id that names no single record is refused here,
// before any engine call, with a typed unknown-change or ambiguous-change
// reason. This pre-read is a supporting observation; the authoritative record
// state is re-read fresh inside the transaction.
func resolveClaimTarget(ctx context.Context, deps PlanningDeps, pin StatusPin, eff config.Effective, id int, opKey string) (string, domain.BranchFacts, *ChangeClaimResult) {
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := newChangeClaimResult(opKey, result, ChangeClaimResult{Findings: []StatusFinding{lifecycleFinding(reason, err.Error())}})
		return "", domain.BranchFacts{}, &r
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		r := newChangeClaimResult(opKey, ResultInternalError, ChangeClaimResult{Findings: []StatusFinding{lifecycleFinding(ReasonStatusInternalError, err.Error())}})
		return "", domain.BranchFacts{}, &r
	}
	snap := build.Snapshot

	c, out := snap.Change(domain.ChangeID(id))
	if out != domain.LookupFound {
		reason, result := "unknown-change", ResultInvalidInput
		msg := fmt.Sprintf("no change %04d is present in the corpus", id)
		if out == domain.LookupAmbiguous {
			reason, result = "ambiguous-change", ResultInvalidState
			msg = fmt.Sprintf("more than one record claims change id %04d; refusing to choose", id)
		}
		r := newChangeClaimResult(opKey, result, ChangeClaimResult{
			Disposition: reason,
			Findings:    []StatusFinding{lifecycleFinding(reason, msg)},
		})
		return "", domain.BranchFacts{}, &r
	}

	facts, err := deps.Reader.BranchFacts(ctx, pin, stackBranches(snap))
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := newChangeClaimResult(opKey, result, ChangeClaimResult{Findings: []StatusFinding{lifecycleFinding(reason, err.Error())}})
		return "", domain.BranchFacts{}, &r
	}
	return c.Path(), facts, nil
}

// claimRequestID derives the idempotency request id for a claim from its own
// (id, version) content, so a lost-response retry of the same request reuses the
// key and replays the original receipt. Version is a full-hex blob id, so the
// composed id satisfies the engine's request-id grammar.
func claimRequestID(req ChangeClaimRequest) string {
	return fmt.Sprintf("claim-%d-%s", req.ID, req.Version)
}

// claimResultFromOutcome folds a transaction outcome into the claim result. On
// an applied or replayed outcome it decodes the claim receipt; a refusal from a
// claim is always state-shaped (an eligibility re-proof failure or an illegal
// source status), so the refusal maps onto invalid-state.
func claimResultFromOutcome(opKey string, res transaction.Result, execErr error) ChangeClaimResult {
	result, replayed := mapOutcome(res, execErr, ResultInvalidState)

	out := ChangeClaimResult{Findings: findingsToStatus(res.Findings), Disposition: claimDisposition(res, result, replayed)}
	if result == ResultApplied {
		if rec, ok := decodeChangeClaimReceipt(res.Receipt); ok {
			out.ID = rec.ID
			out.Status = rec.Status
			out.Branch = rec.Branch
			out.ClaimedAt = rec.ClaimedAt
			out.Lease = rec.Lease
		}
		out.Revision = string(res.AppliedCommit)
	}
	r := newChangeClaimResult(opKey, result, out)
	r.Failure = failureStatus(res, execErr)
	return r
}

// claimDisposition maps a transaction outcome onto the closed claim disposition
// vocabulary. A refusal carries the domain reason token so a caller can key on
// exactly why the claim was refused.
func claimDisposition(res transaction.Result, result Result, replayed bool) string {
	switch res.Disposition {
	case transaction.DispositionApplied:
		return ClaimDispositionApplied
	case transaction.DispositionAlreadyApplied:
		return ClaimDispositionAlreadyClaimed
	case transaction.DispositionContended:
		return ClaimDispositionContended
	case transaction.DispositionFailed:
		return ClaimDispositionFailed
	case transaction.DispositionRefused:
		if code := firstFindingCode(res.Findings); code != "" {
			return code
		}
		return string(result)
	default:
		if replayed {
			return ClaimDispositionAlreadyClaimed
		}
		return string(result)
	}
}

// firstFindingCode returns the code of the first domain finding, or "".
func firstFindingCode(findings []domain.Finding) string {
	if len(findings) > 0 {
		return findings[0].Code
	}
	return ""
}

// dropFindingCode returns findings with every entry carrying code removed.
func dropFindingCode(findings []StatusFinding, code string) []StatusFinding {
	out := findings[:0]
	for _, f := range findings {
		if f.Code != code {
			out = append(out, f)
		}
	}
	return out
}

// decodeChangeClaimReceipt decodes a persisted claim receipt.
func decodeChangeClaimReceipt(b []byte) (changeClaimReceipt, bool) {
	if len(b) == 0 {
		return changeClaimReceipt{}, false
	}
	var rec changeClaimReceipt
	if err := json.Unmarshal(b, &rec); err != nil {
		return changeClaimReceipt{}, false
	}
	return rec, true
}

// changeClaimOp is the SemanticOperation the engine drives per attempt for both
// claim and refresh-claim. Every field is fixed before the transaction; the
// state-dependent work (the eligibility re-proof, the domain gate, field
// patching, rendering) re-runs from the attempt's own fresh state.
type changeClaimOp struct {
	opKey      string
	changeID   int
	refresh    bool
	facts      domain.BranchFacts
	eff        config.Effective
	clock      transaction.Clock
	ttlHours   int
	inline     bool
	link       render.LinkContext
	changesDir string
}

func (o changeClaimOp) Key() transaction.OperationKey { return transaction.OperationKey(o.opKey) }

// Plan re-proves eligibility (claim only) against the attempt's fresh snapshot,
// gates the transition through the domain action, patches the owned claim fields
// plus the refreshed updated date, applies any domain-owned section removals,
// re-renders the artifact block, and assembles the closed plan: the mutated
// change record and the re-rendered board when inline is enabled.
func (o changeClaimOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot

	c, out := snap.Change(domain.ChangeID(o.changeID))
	if out != domain.LookupFound {
		reason := "unknown-change"
		if out == domain.LookupAmbiguous {
			reason = "ambiguous-change"
		}
		return refuseLifecycle(reason, fmt.Sprintf("change %04d is not a single record in the current corpus", o.changeID))
	}

	var result domain.ActionResult
	if o.refresh {
		r, fail := domain.RefreshClaim(c, o.clock.Now())
		if fail != nil {
			return refuseLifecyclePolicy(fail)
		}
		result = r
	} else {
		// In-transaction re-proof: proposed, uniquely identified, build-ready, and
		// a resolved effective base — all from the attempt's own fresh snapshot.
		// Stripping this line lets an ineligible change be claimed on stale facts;
		// the refusal table mutation-tests exactly that.
		if fail := domain.ClaimEligibility(snap, c, o.facts); fail != nil {
			return refuseLifecyclePolicy(fail)
		}
		r, fail := domain.Claim(c, o.clock.Now())
		if fail != nil {
			return refuseLifecyclePolicy(fail)
		}
		result = r
	}

	src, ok := st.State.Sources[c.Path()]
	if !ok {
		return refuseLifecycle("path-mismatch",
			fmt.Sprintf("no record source loaded at %q for change %04d", c.Path(), o.changeID))
	}

	// Apply any domain-owned body-section removals (e.g. a historical ## Run
	// halted section on a re-claimed record) over the exact source bytes.
	edited, err := applyClaimRemovals(src, result.OwnedRemovals)
	if err != nil {
		return refuseLifecycle("section-edit-failed", err.Error())
	}

	// First patch pass: the domain's owned claim FieldChanges plus the refreshed
	// updated date. Each field is upserted in its own parse/apply cycle rather
	// than batched, because a claim inserts SEVERAL absent fields (branch,
	// claimed_at) into a proposed record and two InsertFields batched into one
	// PatchSet resolve to the same pre-fence position — an overlapping edit.
	intermediate := edited
	for _, fc := range result.Changed {
		intermediate, err = upsertFieldBytes(intermediate, fc.Field, lifecycleFieldValue(fc.To))
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change claim: patching %s: %w", fc.Field, err)
		}
	}
	intermediate, err = upsertFieldBytes(intermediate, "updated", document.String(o.clock.Now().UTC().Format("2006-01-02")))
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change claim: stamping updated: %w", err)
	}

	// The candidate snapshot is the before-state with this record replaced by its
	// mutated bytes: it resolves the artifact block's rows and drives the board.
	candidate, err := buildGroomCandidate(o.eff, st.State.Documents, c.Path(), intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	gc, gout := candidate.Change(domain.ChangeID(o.changeID))
	if gout != domain.LookupFound {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change claim: mutated record %04d absent from candidate snapshot", o.changeID)
	}

	body, err := render.ArtifactBlockContent(gc, candidate, o.link)
	if err != nil {
		return refuseLifecycle("artifact-render-failed", err.Error())
	}
	doc2, err := document.Parse(intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change claim: reparsing patched record: %w", err)
	}
	var ps2 document.PatchSet
	ps2.ReplaceBlock("artifacts", body)
	finalBytes, err := doc2.Apply(ps2)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change claim: writing artifact block: %w", err)
	}

	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(c.Path()), Kind: transaction.MutationReplace, Bytes: finalBytes},
	}

	if o.inline {
		// A refresh re-stamps only claimed_at and the updated date — neither is
		// board-visible — so includeBoard's declare-only-when-changed shape can
		// render byte-identical to the committed board and correctly declare no
		// board mutation.
		boardPath := path.Join(o.changesDir, "BOARD.md")
		if err := includeBoard(ctx, st.Tree, boardPath, candidate, &files); err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change claim: %w", err)
		}
	}

	lease := string(domain.EvaluateLease(result.Change, o.clock.Now(), o.ttlHours))
	receipt, err := json.Marshal(changeClaimReceipt{
		Branch:    result.Change.Branch().Value,
		ClaimedAt: result.Change.ClaimedAt().Raw,
		ID:        o.changeID,
		Lease:     lease,
		Op:        o.opKey,
		Status:    string(result.Change.Status()),
	})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change claim: encoding receipt: %w", err)
	}

	subject := fmt.Sprintf("change %04d claimed", o.changeID)
	if o.refresh {
		subject = fmt.Sprintf("change %04d claim refreshed", o.changeID)
	}
	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: subject,
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}

// upsertFieldBytes parses src, upserts one owned field (SetField when present,
// InsertField when absent), and applies — one field per parse/apply cycle so a
// batch of inserts never collides at the pre-fence insertion point.
func upsertFieldBytes(src []byte, name string, v document.Value) ([]byte, error) {
	doc, err := document.Parse(src)
	if err != nil {
		return nil, err
	}
	var ps document.PatchSet
	upsertField(&ps, doc, name, v)
	return doc.Apply(ps)
}

// applyClaimRemovals deletes each domain-owned body section named in removals
// over src, treating those headings as owned for the splice. With no removals it
// returns src unchanged.
func applyClaimRemovals(src []byte, removals []string) ([]byte, error) {
	if len(removals) == 0 {
		return src, nil
	}
	owned := append(append([]string(nil), render.ChangeOwnedHeadings...), removals...)
	edits := make([]render.SectionEdit, 0, len(removals))
	for _, h := range removals {
		edits = append(edits, render.SectionEdit{Heading: h, Intent: render.SectionRemove})
	}
	return render.ApplySectionEdits(src, owned, edits)
}
