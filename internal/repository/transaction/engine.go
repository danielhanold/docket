package transaction

// This file is the transaction engine: the outer adapter that drives one durable
// metadata mutation to a typed outcome. Execute runs the spec's exact per-attempt
// sequence — fetch an authoritative base, allocate a private locked candidate,
// materialize the operation's closed plan in a detached worktree, and push under
// an exact expected-ref lease — and classifies the result. Only a structurally
// proven lease loss retries, always from a fresh fetch and a fresh plan; every
// other failure is terminal with its typed kind.
//
// Two return channels, per the spec's failure posture: a Go error is reserved for
// programmer/call-shape failures (a malformed request, an unsupported keyed
// request until Task 8). Every expected repository, domain, contention, external,
// and interruption outcome is a typed Result; an external or interruption outcome
// additionally carries a typed *Failure as the Go error so a caller can key on its
// stage and kind without parsing prose. A domain refusal — a validation gate or an
// operation's own refusal — is a Result alone, never a Go error.

import (
	"context"
	"errors"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
)

// Request is one transaction to execute. Its slices and byte payloads are copied
// at entry, so a caller may reuse or mutate them afterwards without affecting a
// run in flight.
type Request struct {
	Repository  gitcli.Repository
	Remote      gitcli.RemoteName
	TargetRef   gitcli.RefName // must be a fully qualified refs/heads/... branch
	Expected    []EntityExpectation
	Idempotency *IdempotencyKey
	Loader      StateLoader
	Operation   SemanticOperation
}

// maxAttempts bounds the attempt loop: the initial attempt plus at most three
// semantic retries. It is a package constant, never configuration — the bound is
// a correctness property (a livelock ceiling), not a tuning knob.
const maxAttempts = 4

// Engine executes transactions. It holds only the immutable git client and clock,
// so a single Engine is safe for concurrent Execute calls: every per-attempt
// datum lives on the stack of the call that created it.
type Engine struct {
	client *gitcli.Client
	clock  Clock
}

// NewEngine constructs an Engine over a git client and a clock. Both are
// required; a nil argument is a programmer error, reported as a Go error.
func NewEngine(client *gitcli.Client, clock Clock) (*Engine, error) {
	if client == nil {
		return nil, errors.New("transaction: nil git client")
	}
	if clock == nil {
		return nil, errors.New("transaction: nil clock")
	}
	return &Engine{client: client, clock: clock}, nil
}

// Execute runs the spec's exact attempt sequence and returns a typed Result. A
// non-nil Go error accompanies only a programmer/call-shape failure or an
// external/interruption outcome (where it is a typed *Failure keyed on stage and
// kind); a domain refusal, contention, no-op, or applied outcome returns a nil
// error. Engine and Client are safe for concurrent goroutines.
func (e *Engine) Execute(ctx context.Context, req Request) (Result, error) {
	op := req.Operation.Key()
	base := Result{Operation: op}
	if req.Idempotency != nil {
		base.RequestID = req.Idempotency.RequestID
	}

	// Call-shape validation, before any Git or filesystem work. Every one of these
	// is a programmer error, surfaced as a Go *Failure of kind invalid-input.
	if err := validateOperationKey(op); err != nil {
		return base, &Failure{Stage: StageValidateRequest, Kind: KindInvalidInput, Detail: "invalid operation key", Err: err}
	}
	if err := validateExpectations(copyExpectations(req.Expected)); err != nil {
		return base, &Failure{Stage: StageValidateRequest, Kind: KindInvalidInput, Detail: "invalid expectations", Err: err}
	}
	if err := validateIdempotencyKey(req.Idempotency); err != nil {
		return base, &Failure{Stage: StageValidateRequest, Kind: KindInvalidInput, Detail: "invalid idempotency key", Err: err}
	}
	if !isBranchRef(req.TargetRef) {
		return base, &Failure{Stage: StageValidateRequest, Kind: KindInvalidInput, Detail: "target ref must be a fully qualified refs/heads/ branch"}
	}
	if req.Loader == nil {
		return base, &Failure{Stage: StageValidateRequest, Kind: KindInvalidInput, Detail: "nil state loader"}
	}

	// Copy the mutable inputs so a concurrent caller reuse cannot perturb the run.
	expectations := copyExpectations(req.Expected)
	repo := req.Repository
	remote := req.Remote
	ref := req.TargetRef

	var lastRemote gitcli.ObjectID
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		base.Attempts = attempt
		if err := ctx.Err(); err != nil {
			base.Disposition = DispositionInterrupted
			return base, &Failure{Stage: StageFetch, Kind: KindCancelled, Detail: "context cancelled before attempt", Err: err}
		}

		outcome, done := e.runAttempt(ctx, repo, remote, ref, op, expectations, req, base, &lastRemote)
		if done {
			return outcome.result, outcome.err
		}
		base = outcome.result // carry BaseCommit/RemoteCommit/Attempts forward across a retry
	}

	// The loop exhausted maxAttempts on lease losses alone. That is contention: the
	// promised state repeatedly lost a race with a concurrent writer.
	base.Disposition = DispositionContended
	base.RemoteCommit = lastRemote
	base.Attempts = maxAttempts
	return base, nil
}

// attemptOutcome pairs a computed Result/error with whether the attempt loop is
// finished. done=false means "retry" (a lease loss) and carries only the updated
// bookkeeping Result forward.
type attemptOutcome struct {
	result Result
	err    error
	// done reported alongside via runAttempt's second return.
}

// runAttempt performs exactly one attempt of the spec's per-attempt order and
// classifies its outcome. It returns (outcome, done): done=true means the
// transaction is finished (outcome.result/outcome.err are the caller's return);
// done=false means a lease loss — the caller advances to the next attempt with
// outcome.result carrying the updated base/remote/attempt bookkeeping.
func (e *Engine) runAttempt(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName,
	ref gitcli.RefName, op OperationKey, expectations []EntityExpectation, req Request,
	acc Result, lastRemote *gitcli.ObjectID) (attemptOutcome, bool) {

	// 1. Fetch the exact authoritative base.
	rev, err := e.client.FetchBranch(ctx, repo, remote, ref)
	if err != nil {
		return e.externalOutcome(ctx, StageFetch, acc, "fetching target ref", err), true
	}
	baseCommit := rev.Commit
	acc.BaseCommit = baseCommit
	acc.RemoteCommit = baseCommit
	*lastRemote = baseCommit

	// 2. Idempotency scan. For a keyed request, search the fetched commit's full
	// reachable ancestry for a prior receipt BEFORE allocating any local state, so a
	// lost response replays the original receipt rather than allocating a second time.
	if req.Idempotency != nil {
		rep, serr := e.scanForRequest(ctx, repo, baseCommit, req.Idempotency)
		if serr != nil {
			return e.externalOutcome(ctx, StageIdempotencyScan, acc, "scanning request-id ancestry", serr), true
		}
		switch rep.kind {
		case replayFound:
			// Already applied: return the ORIGINAL receipt and authoritative commit, with
			// no new commit and no allocation. RemoteCommit stays the observed base tip.
			acc.Disposition = DispositionAlreadyApplied
			acc.AppliedCommit = rep.commit
			acc.Receipt = cloneBytes(rep.receipt)
			return attemptOutcome{result: acc}, true
		case replayIDReused:
			acc.Disposition = DispositionFailed
			return attemptOutcome{result: acc, err: &Failure{Stage: StageIdempotencyScan, Kind: KindInvalidInput, Detail: "request-id-reused"}}, true
		case replayInvalidState:
			acc.Disposition = DispositionFailed
			return attemptOutcome{result: acc, err: &Failure{Stage: StageIdempotencyScan, Kind: KindInvalidState, Detail: "request-id history is duplicate, malformed, or contradictory"}}, true
		case replayNone:
			// No prior receipt for this key — fall through to allocation.
		}
	}

	// 3. Allocate a private, live-locked candidate under the transactions root.
	cand, err := allocateCandidate(e.clock, repo, remote, ref, baseCommit)
	if err != nil {
		return e.externalOutcome(ctx, StageAllocate, acc, "allocating candidate", err), true
	}

	// From here, every exit path must clean the candidate up. runCandidate does the
	// remaining work and returns the outcome; cleanup happens here so it runs once.
	outcome, done, cleanupWarn := e.runCandidate(ctx, repo, remote, ref, op, expectations, req, acc, cand, rev, lastRemote)

	// Cleanup captures diagnostics first (already in outcome), then removes the
	// worktree, releases the live lock, and deletes the candidate directory. After a
	// successful push the applied result is never relabelled; it only grows a
	// cleanup-pending warning.
	if !cleanupWarn.skip {
		warns := e.cleanupCandidate(ctx, repo, cand)
		outcome.result.CleanupWarnings = append(outcome.result.CleanupWarnings, warns...)
	}
	return outcome, done
}

// cleanupControl signals whether runAttempt should run cleanup. It always does in
// Task 7; the struct leaves room for a future post-push handoff that defers it.
type cleanupControl struct{ skip bool }

// runCandidate performs the per-candidate work: worktree, load/gate/plan, materialize,
// commit, and push classification. It returns the attempt outcome, the loop's done
// flag, and cleanup control. It never itself cleans the candidate up — runAttempt
// owns that, so cleanup runs exactly once on every path.
func (e *Engine) runCandidate(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName,
	ref gitcli.RefName, op OperationKey, expectations []EntityExpectation, req Request,
	acc Result, cand *candidate, rev gitcli.Revision, lastRemote *gitcli.ObjectID) (attemptOutcome, bool, cleanupControl) {

	run := cleanupControl{}
	baseCommit := rev.Commit

	// 4. Add the detached worktree at the fetched base.
	if err := e.client.AddDetachedWorktree(ctx, repo, cand.worktree, baseCommit); err != nil {
		return e.externalOutcome(ctx, StageWorktree, acc, "adding detached worktree", err), true, run
	}

	// 5. Open the object source at base and load the complete before-state.
	src, err := e.client.OpenObjectSource(ctx, repo, rev)
	if err != nil {
		return e.externalOutcome(ctx, StageLoadBefore, acc, "opening object source", err), true, run
	}
	baseTree := newBaseTree(src)
	before, err := req.Loader.Load(ctx, baseTree)
	if err != nil {
		return e.externalOutcome(ctx, StageLoadBefore, acc, "loading base state", err), true, run
	}
	if before.Report.HasErrors() {
		return refusedOutcome(acc, StageLoadBefore, errorFindings(before.Report.Findings())), true, run
	}

	// 6. Check every expectation against the base tree. A mismatch — on any attempt —
	// is contention: never text-merged, never retried through a fresh plan.
	if mism, err := checkExpectations(ctx, baseTree, expectations); err != nil {
		return e.externalOutcome(ctx, StageExpectations, acc, "reading expectation paths", err), true, run
	} else if len(mism) > 0 {
		acc.Disposition = DispositionContended
		acc.ContendedPaths = mism
		return attemptOutcome{result: acc}, true, run
	}

	// 7. Ask the operation for a closed plan for THIS attempt's state.
	plan, opRes, err := req.Operation.Plan(ctx, AttemptState{Base: rev, State: before, Tree: baseTree})
	if err != nil {
		return e.externalOutcome(ctx, StagePlan, acc, "operation plan failed", err), true, run
	}
	if opRes.Refused {
		return refusedOutcome(acc, StagePlan, opRes.Findings), true, run
	}
	if err := validatePlan(plan); err != nil {
		acc.Disposition = DispositionFailed
		return attemptOutcome{result: acc, err: &Failure{Stage: StagePlan, Kind: KindInvalidInput, Detail: "operation produced an invalid plan", Err: err}}, true, run
	}

	// 8. Layer the plan over base, load the after-state, and run both gates.
	overlay, err := newOverlayTree(baseTree, plan)
	if err != nil {
		acc.Disposition = DispositionFailed
		return attemptOutcome{result: acc, err: &Failure{Stage: StageLoadAfter, Kind: KindInvalidState, Detail: "plan violates before/after tree rules", Err: err}}, true, run
	}
	after, err := req.Loader.Load(ctx, overlay)
	if err != nil {
		return e.externalOutcome(ctx, StageLoadAfter, acc, "loading after state", err), true, run
	}
	if after.Report.HasErrors() {
		return refusedOutcome(acc, StageLoadAfter, errorFindings(after.Report.Findings())), true, run
	}
	if evo := errorFindings(req.Loader.ValidateEvolution(before, after)); len(evo) > 0 {
		return refusedOutcome(acc, StageLoadAfter, evo), true, run
	}

	// 9. An empty plan is the no-op path: nothing changed, so nothing is committed.
	//
	// Keyed no-ops are intentionally NOT idempotency-persisted. Because no commit
	// or receipt is written, a keyed request that resolves to an empty plan leaves
	// nothing in ancestry for a later replay to find: replaying the same request id
	// re-evaluates the operation from fresh state and yields no-op again (never
	// already-applied). This is by design — a no-op changed nothing, so there is
	// nothing to replay — and must not be "fixed" by persisting a receipt here.
	if len(plan.Files) == 0 {
		acc.Disposition = DispositionNoOp
		return attemptOutcome{result: acc}, true, run
	}

	// 10. Materialize exactly the declared paths, read them back, and prove the
	// worktree's actual delta equals the declared set in both directions.
	if err := materializePlan(cand.worktree, plan); err != nil {
		return failedFromFailure(acc, err), true, run
	}
	if err := verifyMaterialized(cand.worktree, plan); err != nil {
		return failedFromFailure(acc, err), true, run
	}
	if err := verifyActualDelta(ctx, e.client, repo, cand.worktree, plan); err != nil {
		return failedFromFailure(acc, err), true, run
	}

	// 11. Commit exactly the declared paths on the detached HEAD with the engine
	// trailer block, hooks and signing disabled, dates pinned to the clock.
	commit, err := e.client.CommitPaths(ctx, repo, gitcli.CommitRequest{
		Dir:       cand.worktree,
		Paths:     planPaths(plan),
		Subject:   plan.CommitSubject,
		Trailers:  engineTrailers(cand.id, op, req.Idempotency, plan.Receipt),
		HooksPath: cand.hooks,
		When:      e.clock.Now(),
	})
	if err != nil {
		return e.externalOutcome(ctx, StageCommit, acc, "committing declared paths", err), true, run
	}
	_ = cand.setPhase(e.clock, phaseCommitted)

	// 12. Push under the exact expected-old lease <ref>:<base>.
	pushRes, perr := e.client.PushLease(ctx, repo, remote, ref, commit, baseCommit)
	if perr != nil {
		// The push could not be classified structurally — probe whether it landed.
		return e.classifyUnknownPush(ctx, repo, remote, ref, commit, acc, plan, cand, &run), true, run
	}
	switch pushRes.Disposition {
	case gitcli.PushApplied:
		return e.appliedOutcome(acc, commit, plan, cand), true, run
	case gitcli.PushLeaseLost:
		acc.RemoteCommit = pushRes.Remote
		*lastRemote = pushRes.Remote
		// Not done: retry from a fresh fetch and a fresh plan next attempt.
		return attemptOutcome{result: acc}, false, run
	default: // gitcli.PushFailed
		if pushRes.Remote != "" {
			acc.RemoteCommit = pushRes.Remote
			*lastRemote = pushRes.Remote
		}
		return e.classifyUnknownPush(ctx, repo, remote, ref, commit, acc, plan, cand, &run), true, run
	}
}

// appliedOutcome finalizes an applied push: it stamps the pushed phase (best
// effort — the promised remote state already exists), records the applied commit
// and decoded receipt, and lets runAttempt run cleanup.
func (e *Engine) appliedOutcome(acc Result, commit gitcli.ObjectID, plan MutationPlan, cand *candidate) attemptOutcome {
	_ = cand.setPhase(e.clock, phasePushed)
	acc.Disposition = DispositionApplied
	acc.AppliedCommit = commit
	acc.RemoteCommit = commit
	acc.Receipt = cloneBytes(plan.Receipt)
	return attemptOutcome{result: acc}
}

// classifyUnknownPush runs the post-push probe when a push could not be classified
// structurally (a transport error) or was reported failed. It fetches the ref and
// asks whether the pushed commit is reachable from the observed remote: reachable
// means the push actually landed (applied); otherwise the outcome is a failed
// external result, or interrupted when the context was cancelled.
func (e *Engine) classifyUnknownPush(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName,
	ref gitcli.RefName, commit gitcli.ObjectID, acc Result, plan MutationPlan, cand *candidate, run *cleanupControl) attemptOutcome {

	probe, err := e.client.FetchBranch(ctx, repo, remote, ref)
	if err != nil {
		return e.externalOutcome(ctx, StageProbe, acc, "post-push probe fetch failed", err)
	}
	acc.RemoteCommit = probe.Commit
	reachable, err := e.client.IsAncestor(ctx, repo, commit, probe.Commit)
	if err != nil {
		return e.externalOutcome(ctx, StageProbe, acc, "post-push reachability probe failed", err)
	}
	if reachable {
		return e.appliedOutcome(acc, commit, plan, cand)
	}
	acc.Disposition = DispositionFailed
	return attemptOutcome{result: acc, err: &Failure{Stage: StagePush, Kind: KindExternal, Detail: "push rejected and commit not reachable from remote"}}
}

// externalOutcome classifies a Git/transport error at stage into a typed outcome:
// interrupted when the context was cancelled or the adapter reported cancellation/
// timeout, else a failed external outcome. Either way it returns the *Failure as
// the Go error so a caller keys on stage and kind, never prose.
func (e *Engine) externalOutcome(ctx context.Context, stage Stage, acc Result, detail string, cause error) attemptOutcome {
	if ctx.Err() != nil || isGitCancellation(cause) {
		acc.Disposition = DispositionInterrupted
		return attemptOutcome{result: acc, err: &Failure{Stage: stage, Kind: KindCancelled, Detail: detail, Err: cause}}
	}
	acc.Disposition = DispositionFailed
	return attemptOutcome{result: acc, err: &Failure{Stage: stage, Kind: KindExternal, Detail: detail, Err: cause}}
}

// failedFromFailure wraps a materialize/verify step's own typed *Failure as a
// failed outcome, preserving its stage and kind.
func failedFromFailure(acc Result, err error) attemptOutcome {
	acc.Disposition = DispositionFailed
	return attemptOutcome{result: acc, err: err}
}

// refusedOutcome builds a domain refusal: a Result with the refusal findings and a
// nil Go error. A validation gate and an operation's own refusal both land here.
func refusedOutcome(acc Result, _ Stage, findings []domain.Finding) attemptOutcome {
	acc.Disposition = DispositionRefused
	acc.Findings = cloneFindings(findings)
	return attemptOutcome{result: acc}
}

// checkExpectations reads each expectation's exact path from the base tree and
// reports the paths whose observed state does not match. A blob expectation
// requires an entry at that exact path whose object id equals the pinned id; an
// absent expectation requires no entry at that path. Bytes are never read — only
// the tree listing — so a mismatch never leaks content.
func checkExpectations(ctx context.Context, tree Tree, exps []EntityExpectation) ([]gitcli.RepoPath, error) {
	if len(exps) == 0 {
		return nil, nil
	}
	paths := make([]gitcli.RepoPath, 0, len(exps))
	for _, e := range exps {
		paths = append(paths, e.Path)
	}
	entries, err := tree.ListTree(ctx, paths)
	if err != nil {
		return nil, err
	}
	byPath := make(map[gitcli.RepoPath]gitcli.TreeEntry, len(entries))
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}
	var mismatched []gitcli.RepoPath
	for _, e := range exps {
		entry, present := byPath[e.Path]
		switch e.Version.Kind {
		case VersionBlob:
			if !present || entry.ObjectID != e.Version.ObjectID {
				mismatched = append(mismatched, e.Path)
			}
		case VersionAbsent:
			if present {
				mismatched = append(mismatched, e.Path)
			}
		}
	}
	return mismatched, nil
}

// planPaths returns the declared path set of a plan, in declaration order.
func planPaths(plan MutationPlan) []gitcli.RepoPath {
	paths := make([]gitcli.RepoPath, 0, len(plan.Files))
	for _, f := range plan.Files {
		paths = append(paths, f.Path)
	}
	return paths
}

// copyExpectations returns a defensive copy of a request's expectations so a
// concurrent caller mutation cannot perturb a run.
func copyExpectations(exps []EntityExpectation) []EntityExpectation {
	if len(exps) == 0 {
		return nil
	}
	out := make([]EntityExpectation, len(exps))
	copy(out, exps)
	return out
}

// errorFindings keeps only the error-severity findings — the ones that make a
// gate refuse. Warnings stay in the loaded state's report but never block.
func errorFindings(findings []domain.Finding) []domain.Finding {
	var out []domain.Finding
	for _, f := range findings {
		if f.Severity == domain.SeverityError {
			out = append(out, f)
		}
	}
	return out
}

// cloneFindings copies a findings slice so a returned Result cannot alias the
// loader's internal state.
func cloneFindings(findings []domain.Finding) []domain.Finding {
	if len(findings) == 0 {
		return nil
	}
	out := make([]domain.Finding, len(findings))
	copy(out, findings)
	return out
}

// isBranchRef reports whether ref is a valid, fully qualified local branch
// (refs/heads/<name> with a non-empty name). A tag ref, a remote-tracking ref, or
// a short name is rejected so the engine never fetches or pushes a non-branch.
func isBranchRef(ref gitcli.RefName) bool {
	s := string(ref)
	const prefix = "refs/heads/"
	if !strings.HasPrefix(s, prefix) || len(s) == len(prefix) {
		return false
	}
	return validRefShape(ref)
}

// validRefShape reports whether ref passes gitcli's ref-name grammar. gitcli only
// exposes it through operations, so the engine reproduces the one predicate it
// needs: reject anything the push/fetch surface would reject as invalid-request.
func validRefShape(ref gitcli.RefName) bool {
	s := string(ref)
	if strings.ContainsAny(s, " \t\r\n\v\f\\*") {
		return false
	}
	if strings.Contains(s, "@{") || strings.Contains(s, "..") {
		return false
	}
	for _, comp := range strings.Split(s, "/") {
		if comp == "" || comp == "." || comp == ".." || strings.HasPrefix(comp, ".") || strings.HasSuffix(comp, ".lock") {
			return false
		}
	}
	return true
}

// isGitCancellation reports whether a cause is (or wraps) a cancellation or
// deadline — a gitcli Failure of cancelled/timed-out kind, or a context error.
func isGitCancellation(cause error) bool {
	if gf, ok := gitcli.AsFailure(cause); ok {
		if gf.Kind == gitcli.KindCancelled || gf.Kind == gitcli.KindTimedOut {
			return true
		}
	}
	return errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded)
}
