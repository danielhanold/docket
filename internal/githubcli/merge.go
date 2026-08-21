package githubcli

// This file owns MergePullRequest and ProbeMerged: the expected-head merge of one
// exact pull request, and the authoritative merged reprobe usable after success,
// timeout, cancellation, or a lost response.
//
// MergePullRequest selects the effective merge method — via probeRepoMergeMethods
// and probeBranchMergeRules (mergemethod.go), composed by intersect and resolved
// by selectMergeMethod in the fixed priority rebase → merge commit → squash — and
// attempts exactly that one method, never retrying a lower-priority one on
// rejection. A cleanly observed empty permitted set is MergeMethodUnavailable and
// issues no merge; an unobservable capability probe is unknown (retain).
//
// The merge is gated on an authoritative pre-decision snapshot and issued with an
// exact `--match-head-commit`, so GitHub itself refuses if the head moved between
// the decision and the effect. The merge is NEVER rolled back and NEVER requests
// branch deletion (`--delete-branch`) — cleanup is an independent, separately
// owned suffix. Post-merge truth is established by a fresh reprobe, never by the
// merge process exit code: a lost response over a merge that actually landed is
// recovered to `merged`, while a non-zero exit over a PR left cleanly open,
// mergeable, and at the same head is an authoritative denial.
//
// The three-outcome discipline holds throughout: a transport/decode failure is
// `unknown` (retain) and never authorizes closeout, distinct from a cleanly
// observed non-merged state. `UNKNOWN` mergeability never authorizes a merge
// (learnings: probe-error-is-not-clean-absence, idempotency-keying).

import (
	"context"
	"encoding/json"
	"strconv"
)

// mergeOp labels every Failure raised while merging or reprobing a pull request.
const mergeOp = "merge-pull-request"

// mergeJSONFields is the field set the merge decision and the merged-facts
// reprobe consume. It extends the standard PR fields with the merge vocabulary —
// mergedAt, mergeCommit, mergeable — in gh's documented nested shapes.
const mergeJSONFields = "number,url,state,isDraft,headRefName,headRefOid,baseRefName,title,body,mergedAt,mergeCommit,mergeable"

// GitHub's mergeable enum. UNKNOWN is GitHub still computing mergeability
// lazily; it is NEVER read as clean.
const (
	mergeableMergeable   = "MERGEABLE"
	mergeableConflicting = "CONFLICTING"
	mergeableUnknown     = "UNKNOWN"
)

// ObjectRef is a full GitHub-reported object id (validated 40/64 lowercase hex)
// naming the exact head a merge must match. githubcli keeps object ids as plain
// validated strings across its package boundary rather than importing gitcli.
type ObjectRef string

// MergeOutcome is the closed set of merge / reprobe outcomes.
type MergeOutcome string

const (
	// MergeMerged: this attempt's merge landed and was verified merged.
	MergeMerged MergeOutcome = "merged"
	// MergeAlreadyMerged: the PR was already merged (idempotent replay or a prior
	// merge by anyone); no second merge was issued. Carries the merged facts. From
	// ProbeMerged this is the definitive "merged" answer.
	MergeAlreadyMerged MergeOutcome = "already-merged"
	// MergeHeadMoved: GitHub reports a head other than the expected one; no merge.
	MergeHeadMoved MergeOutcome = "head-moved"
	// MergeNotMergeable: the PR cannot be closed out as merged — a conflicting
	// mergeability, or (from ProbeMerged) a cleanly observed non-merged state (still
	// open, or closed unmerged). No merge was issued.
	MergeNotMergeable MergeOutcome = "not-mergeable"
	// MergeDenied: the merge was issued and rejected (non-zero exit) while the PR
	// remained cleanly open, mergeable, and at the expected head — an authoritative
	// policy/permission denial, never inferred from a transient error.
	MergeDenied MergeOutcome = "denied"
	// MergeUnknown: an external probe could not establish the truth, or a state
	// that cannot authorize a merge (UNKNOWN mergeability). Never permits closeout.
	// A transport/decode failure carries a diagnostic error; a cleanly observed
	// non-authorizing state carries a nil error.
	MergeUnknown MergeOutcome = "unknown"
	// MergeMethodUnavailable: repository settings and branch rules leave no
	// permitted merge method; the incompatible policy was observed cleanly and
	// NO merge was issued. Distinct from denied (nothing was attempted) and
	// from unknown (the policy WAS observable).
	MergeMethodUnavailable MergeOutcome = "method-unavailable"
)

// MergedFacts is the authoritative post-merge evidence. It is populated on
// merged/already-merged and is the zero value on every non-merged outcome.
type MergedFacts struct {
	HeadOID, BaseRef, MergedAtUTC, MergeCommit string
	Version                                    string
}

// MergeResult is the outcome of one MergePullRequest call. Method is attempt
// metadata, not a merged fact: it is set exactly when Docket issued the merge
// command — success, authoritative denial, or a lost response later recovered —
// and empty for validation failures, pre-effect refusals, and already-merged
// recovery (Docket did not choose the historical merge's method). RepoMethods
// and BranchMethods are populated only on MergeMethodUnavailable, naming the
// two observed permitted sets so a human can correct the conflicting setting.
type MergeResult struct {
	Outcome                    MergeOutcome
	Method                     MergeMethod
	Facts                      MergedFacts
	RepoMethods, BranchMethods []MergeMethod
}

// mergePRJSON is the merge-specific projection of gh's nested PR shape. mergedAt
// is null for an unmerged PR (decodes to ""); mergeCommit is a nested {"oid"}
// object or null; mergeable is GitHub's uppercase enum verbatim.
type mergePRJSON struct {
	MergedAt    string `json:"mergedAt"`
	Mergeable   string `json:"mergeable"`
	MergeCommit *struct {
		OID string `json:"oid"`
	} `json:"mergeCommit"`
}

// mergeSnapshot is the authoritative merge-decision state: the validated PR plus
// its merge vocabulary.
type mergeSnapshot struct {
	pr          PullRequest
	mergeable   string
	mergedAt    string
	mergeCommit string
}

// facts renders the merged evidence from the snapshot.
func (s mergeSnapshot) facts() MergedFacts {
	return MergedFacts{
		HeadOID:     s.pr.HeadCommit,
		BaseRef:     s.pr.BaseBranch,
		MergedAtUTC: s.mergedAt,
		MergeCommit: s.mergeCommit,
		Version:     s.pr.Version,
	}
}

// MergePullRequest merges one PR at an exact expected head with the effective
// merge method the repository settings and the base branch's active rules permit,
// selected in the fixed priority rebase → merge commit → squash. It attempts
// exactly one method and never retries a lower-priority one on rejection.
// merged/already-merged/head-moved/not-mergeable/denied/method-unavailable are
// value outcomes with a nil error; unknown carries a typed *Failure only when an
// actual transport/decode failure occurred (an unobservable capability probe
// included). It never issues a second merge for an already-merged PR and never
// requests branch deletion.
func (c *Client) MergePullRequest(ctx context.Context, repo Repository, number int, expectedHead ObjectRef, admin bool) (MergeResult, error) {
	if err := validateRepository(repo); err != nil {
		return MergeResult{Outcome: MergeUnknown}, newFailure(mergeOp, StageValidate, KindInvalidInput, "repository identity invalid: "+err.Error(), err)
	}
	if number <= 0 {
		return MergeResult{Outcome: MergeUnknown}, newFailure(mergeOp, StageValidate, KindInvalidInput, "pull request number must be positive", nil)
	}
	if err := validateFullObjectID(string(expectedHead)); err != nil {
		return MergeResult{Outcome: MergeUnknown}, newFailure(mergeOp, StageValidate, KindInvalidInput, "expected head oid invalid: "+err.Error(), err)
	}

	// Pre-decision: an authoritative snapshot decides whether a merge is authorized.
	snap, f := c.probeMergeSnapshot(ctx, repo, number)
	if f != nil {
		return MergeResult{Outcome: MergeUnknown}, f
	}
	switch snap.pr.State {
	case StateMerged:
		return MergeResult{Outcome: MergeAlreadyMerged, Facts: snap.facts()}, nil
	case StateOpen:
		if snap.pr.HeadCommit != string(expectedHead) {
			return MergeResult{Outcome: MergeHeadMoved}, nil
		}
		switch snap.mergeable {
		case mergeableMergeable:
			// proceed to policy + act
		case mergeableConflicting:
			return MergeResult{Outcome: MergeNotMergeable}, nil
		default:
			// UNKNOWN (lazy mergeability) or an unrecognized enum: never authorize.
			return MergeResult{Outcome: MergeUnknown}, nil
		}
	default:
		// Closed, unmerged: cannot be merged.
		return MergeResult{Outcome: MergeNotMergeable}, nil
	}

	// Policy: read repository-enabled methods and the active rules for the PR's
	// ACTUAL base branch, intersect, and select the fixed priority. An
	// unobservable policy is unknown; a cleanly observed empty set is
	// method-unavailable. Neither issues a merge.
	repoSet, pf := c.probeRepoMergeMethods(ctx, repo)
	if pf != nil {
		return MergeResult{Outcome: MergeUnknown}, pf
	}
	branchSet, pf := c.probeBranchMergeRules(ctx, repo, snap.pr.BaseBranch)
	if pf != nil {
		return MergeResult{Outcome: MergeUnknown}, pf
	}
	method, ok := selectMergeMethod(repoSet.intersect(branchSet))
	if !ok {
		return MergeResult{
			Outcome:       MergeMethodUnavailable,
			RepoMethods:   repoSet.list(),
			BranchMethods: branchSet.list(),
		}, nil
	}

	// Act: the selected method at the exact expected head. No --delete-branch. The
	// closed vocabulary is guarded — a method outside it renders no flag.
	flag := method.mergeFlag()
	if flag == "" {
		return MergeResult{Outcome: MergeUnknown}, newFailure(mergeOp, StageValidate, KindInvalidInput,
			"selected merge method outside the closed vocabulary", nil)
	}
	args := []string{
		"pr", "merge", strconv.Itoa(number),
		"--repo", repo.Spec(),
		flag,
		"--match-head-commit", string(expectedHead),
	}
	if admin {
		args = append(args, "--admin")
	}
	res, mf := c.run(ctx, runRequest{op: mergeOp, args: args, network: true})
	if mf != nil {
		if mf.Stage == StageLaunch {
			// gh never started; nothing merged. Retain as unknown with no method.
			return MergeResult{Outcome: MergeUnknown}, mf
		}
		// A timeout/cancel may have landed the merge; it is NOT a denial. Verify.
		return c.verifyMerge(ctx, repo, number, expectedHead, false, method)
	}
	// A non-zero exit is a candidate denial; a zero exit is the expected success.
	// Both are resolved against a fresh authoritative reprobe.
	return c.verifyMerge(ctx, repo, number, expectedHead, res.exitCode != 0, method)
}

// verifyMerge re-derives the post-merge truth from a fresh authoritative snapshot.
// mergeRejected marks that the merge command exited non-zero (a candidate denial);
// it is honored only when the PR is left cleanly open, mergeable, and at the
// expected head. method is the method Docket issued: the merge command WAS issued
// on every path that reaches here, so every returned MergeResult carries it.
func (c *Client) verifyMerge(ctx context.Context, repo Repository, number int, expectedHead ObjectRef, mergeRejected bool, method MergeMethod) (MergeResult, error) {
	snap, f := c.probeMergeSnapshot(ctx, repo, number)
	if f != nil {
		return MergeResult{Outcome: MergeUnknown, Method: method}, f
	}
	switch snap.pr.State {
	case StateMerged:
		return MergeResult{Outcome: MergeMerged, Method: method, Facts: snap.facts()}, nil
	case StateOpen:
		if snap.pr.HeadCommit != string(expectedHead) {
			return MergeResult{Outcome: MergeHeadMoved, Method: method}, nil
		}
		if snap.mergeable == mergeableConflicting {
			return MergeResult{Outcome: MergeNotMergeable, Method: method}, nil
		}
		if mergeRejected && snap.mergeable == mergeableMergeable {
			return MergeResult{Outcome: MergeDenied, Method: method}, nil
		}
		// A zero-exit merge that is not observed merged, a transient error, or an
		// UNKNOWN mergeability: retain rather than fabricate a success.
		return MergeResult{Outcome: MergeUnknown, Method: method}, newFailure(mergeOp, StageInvoke, KindExternal,
			"merge outcome could not be verified merged after the attempt", nil)
	default:
		return MergeResult{Outcome: MergeNotMergeable, Method: method}, nil
	}
}

// ProbeMerged is the authoritative read-only reprobe answering "is this PR
// merged?". already-merged carries the merged facts; not-mergeable is a cleanly
// observed non-merged state (open, or closed unmerged); unknown is an
// unobservable probe (carrying a diagnostic error). It issues no mutation.
func (c *Client) ProbeMerged(ctx context.Context, repo Repository, number int) (MergeOutcome, MergedFacts, error) {
	if err := validateRepository(repo); err != nil {
		return MergeUnknown, MergedFacts{}, newFailure(mergeOp, StageValidate, KindInvalidInput, "repository identity invalid: "+err.Error(), err)
	}
	if number <= 0 {
		return MergeUnknown, MergedFacts{}, newFailure(mergeOp, StageValidate, KindInvalidInput, "pull request number must be positive", nil)
	}
	snap, f := c.probeMergeSnapshot(ctx, repo, number)
	if f != nil {
		return MergeUnknown, MergedFacts{}, f
	}
	if snap.pr.State == StateMerged {
		return MergeAlreadyMerged, snap.facts(), nil
	}
	return MergeNotMergeable, MergedFacts{}, nil
}

// probeMergeSnapshot reads one PR by number with the merge field set. --repo is
// explicit so a caller's CWD or GH_REPO cannot retarget the query. A transport
// failure, a non-zero exit, or a decode hazard is a typed *Failure — never a
// zero-value snapshot read as truth.
func (c *Client) probeMergeSnapshot(ctx context.Context, repo Repository, number int) (mergeSnapshot, *Failure) {
	res, f := c.run(ctx, runRequest{
		op: mergeOp,
		args: []string{
			"pr", "view", strconv.Itoa(number),
			"--repo", repo.Spec(),
			"--json", mergeJSONFields,
		},
		network: true,
	})
	if f != nil {
		return mergeSnapshot{}, f
	}
	if res.exitCode != 0 {
		return mergeSnapshot{}, newFailure(mergeOp, StageInvoke, KindExternal,
			"gh pr view failed: "+stderrExcerpt(res.stderr), nil)
	}
	snap, err := decodeMergeSnapshot(mergeOp, res.stdout)
	if err != nil {
		if ff, ok := AsFailure(err); ok {
			return mergeSnapshot{}, ff
		}
		return mergeSnapshot{}, newFailure(mergeOp, StageDecode, KindInvalidOutput, "merge snapshot undecodable", err)
	}
	return snap, nil
}

// decodeMergeSnapshot decodes the standard PR fields (reusing the package's
// validated PR decode, which computes the version) and the merge vocabulary from
// the same bytes. An UNKNOWN mergeability is preserved verbatim, never coerced.
func decodeMergeSnapshot(op string, data []byte) (mergeSnapshot, error) {
	pr, err := decodePullRequest(op, data)
	if err != nil {
		return mergeSnapshot{}, err
	}
	var mf mergePRJSON
	if err := json.Unmarshal(data, &mf); err != nil {
		return mergeSnapshot{}, newFailure(op, StageDecode, KindInvalidOutput, "pull-request merge fields are not valid JSON", err)
	}
	mergeCommit := ""
	if mf.MergeCommit != nil {
		mergeCommit = mf.MergeCommit.OID
	}
	return mergeSnapshot{pr: pr, mergeable: mf.Mergeable, mergedAt: mf.MergedAt, mergeCommit: mergeCommit}, nil
}
