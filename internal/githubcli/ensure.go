package githubcli

// This file owns EnsurePullRequest: the idempotent, recovery-safe publication of
// one ready-for-review pull request for an exact head branch. It follows the
// spec §"Probe, act, verify" fixed sequence:
//
//	1. list ALL PRs for the explicit repository and exact head branch, terminal
//	   states included;
//	2. more than one open PR is ambiguous and blocks; a closed/merged same-head
//	   PR with no open PR also blocks — Docket never creates duplicate history
//	   after a terminal PR;
//	3. one open PR already equal to the request (head commit, base, title, body,
//	   ready state) is adopted (empty ExpectedVersion — the lost-create-response
//	   recovery face) or unchanged (a supplied ExpectedVersion) with NO mutation;
//	4. one open PR that differs requires its current opaque version to equal
//	   ExpectedVersion before an edit; an empty or mismatched version is contended
//	   and leaves the PR untouched;
//	5. no PR is created with explicit head/base/repository/title and the body on
//	   stdin;
//	6. after a create or edit, the postcondition is re-derived by querying BOTH by
//	   head branch and by PR number, and both views must name one open ready PR
//	   equal to the request; and
//	7. a gh timeout, a cancel after launch, or an unsuccessful exit after a
//	   mutation may have reached GitHub is answered by the same requery — an exact
//	   postcondition is created/updated, an observed different state is contended,
//	   and an inability to establish truth is unknown.
//
// The idempotency key is the exact open PR on GitHub, re-derived from a fresh
// authoritative probe on every retry — never a local proxy, a process exit, or a
// cached response (learnings: idempotency-keying, cas-re-read-fresh-origin,
// decide-and-act-on-the-same-copy). There is NO automatic compensating close,
// reopen, body rollback, base rollback, or second create. The adapter refuses to
// create or update whenever GitHub reports a head commit other than ExpectedHead
// (the value a successful workspace.PublishHead reached), and an existing draft
// PR is surfaced as invalid state for the later human/workflow rather than having
// its review state silently changed. Authored body bytes travel only on stdin
// (`--body-file -`), never in an argv element or a diagnostic.

import (
	"context"
	"encoding/json"
	"strconv"
)

// ensureOp labels every Failure raised while ensuring a pull request.
const ensureOp = "ensure-pull-request"

// prJSONFields is the documented `--json` field set every list/view/create/edit
// decode consumes. It matches the nested shapes real gh emits (pr.go's
// prViewJSON), never a flattened fake-only shape.
const prJSONFields = "number,url,state,isDraft,headRefName,headRefOid,baseRefName,title,body"

// prViewJSONFields is the exact-number view's field set: the standard
// prJSONFields plus GitHub's nullable reviewDecision. Only ViewPullRequest
// requests review state — the list/create/edit paths keep the standard set, so
// their snapshots and write-CAS versions are untouched by review activity.
const prViewJSONFields = prJSONFields + ",reviewDecision"

// EnsureDisposition is the closed set of idempotent publication outcomes.
type EnsureDisposition string

const (
	EnsureCreated   EnsureDisposition = "created"
	EnsureAdopted   EnsureDisposition = "adopted"
	EnsureUpdated   EnsureDisposition = "updated"
	EnsureUnchanged EnsureDisposition = "unchanged"
	EnsureContended EnsureDisposition = "contended"
	EnsureUnknown   EnsureDisposition = "unknown"
	EnsureFailed    EnsureDisposition = "failed"
)

// EnsurePullRequestRequest is the idempotent publication request. ExpectedHead
// is the full object id from a successful workspace.PublishHead; BaseBranch is
// the resolved effective-base branch, never guessed. An empty ExpectedVersion
// permits create-or-adopt only; updating a differing open PR requires the exact
// current version.
type EnsurePullRequestRequest struct {
	Repository      Repository
	HeadBranch      string
	ExpectedHead    string
	BaseBranch      string
	Title           string
	Body            string
	ExpectedVersion string
}

// EnsureResult is the value outcome. PR carries the verified snapshot on
// created/adopted/updated/unchanged; it is the zero value on contended/unknown
// (no snapshot was established as the authoritative postcondition) and on the
// failed error paths.
type EnsureResult struct {
	Disposition EnsureDisposition
	PR          PullRequest
}

// EnsurePullRequest ensures exactly one open, ready-for-review pull request for
// the request's head branch, idempotently. created/adopted/updated/unchanged/
// contended/unknown are value dispositions returned with a nil error; every
// block (ambiguity, a terminal same-head PR, a draft, a head commit other than
// ExpectedHead, a decode hazard, an auth/probe failure, or invalid input) is an
// EnsureFailed disposition paired with a typed *Failure and no mutation.
func (c *Client) EnsurePullRequest(ctx context.Context, req EnsurePullRequestRequest) (EnsureResult, error) {
	if err := validateEnsureRequest(req); err != nil {
		return EnsureResult{Disposition: EnsureFailed}, err
	}

	// Step 1: authoritative probe — ALL PRs for the explicit repo + exact head.
	prs, err := c.probeByHead(ctx, req)
	if err != nil {
		return EnsureResult{Disposition: EnsureFailed}, err
	}
	open := openPRs(prs)

	switch {
	case len(open) > 1:
		// Step 2: ambiguous — more than one open PR for the head branch.
		return EnsureResult{Disposition: EnsureFailed}, newFailure(ensureOp, StageDecode, KindInvalidState,
			"multiple open pull requests for the head branch (ambiguous)", nil)

	case len(open) == 0:
		if len(prs) > 0 {
			// Step 2: only terminal (closed/merged) PRs remain — refusing to open
			// a duplicate history after a terminal PR.
			return EnsureResult{Disposition: EnsureFailed}, newFailure(ensureOp, StageDecode, KindInvalidState,
				"a terminal-state pull request exists for the head branch; refusing to create duplicate history", nil)
		}
		// Step 5: no PR of any state exists — create it.
		return c.mutateAndVerify(ctx, req, createRequest(req), EnsureCreated)

	default:
		// Exactly one open PR.
		pr := open[0]
		// A draft is surfaced as invalid state for the later human/workflow; v1
		// only opens ready PRs and never silently changes review state.
		if pr.Draft {
			return EnsureResult{Disposition: EnsureFailed}, newFailure(ensureOp, StageDecode, KindInvalidState,
				"existing pull request is a draft", nil)
		}
		// Refuse to touch a PR GitHub reports at a head commit other than the one
		// workspace.PublishHead reached — this gates BOTH adopt and edit.
		if pr.HeadCommit != req.ExpectedHead {
			return EnsureResult{Disposition: EnsureFailed}, newFailure(ensureOp, StageDecode, KindInvalidState,
				"GitHub reports a head commit other than the expected published head", nil)
		}
		if matchesRequest(pr, req) {
			// Step 3: already in the desired end state — no mutation. adopted is the
			// lost-create-response recovery face (empty version); a supplied version
			// is unchanged.
			if req.ExpectedVersion == "" {
				return EnsureResult{Disposition: EnsureAdopted, PR: pr}, nil
			}
			return EnsureResult{Disposition: EnsureUnchanged, PR: pr}, nil
		}
		// Step 4: the open PR differs. Only an ExpectedVersion equal to its exact
		// current version authorizes an edit; empty or mismatched is contended and
		// leaves the PR untouched.
		if req.ExpectedVersion == "" || req.ExpectedVersion != pr.Version {
			return EnsureResult{Disposition: EnsureContended}, nil
		}
		return c.mutateAndVerify(ctx, req, editRequest(req, pr.Number), EnsureUpdated)
	}
}

// mutateAndVerify performs one create or edit and then re-derives the
// postcondition. A launch failure (gh could not start) is a hard EnsureFailed —
// no mutation reached GitHub. A timeout or cancellation after launch, and any
// process exit (zero OR non-zero), all flow into the same requery: the mutation
// may have landed even when gh reported failure, so truth is established by a
// fresh authoritative probe rather than the exit code.
func (c *Client) mutateAndVerify(ctx context.Context, req EnsurePullRequestRequest, mut runRequest, success EnsureDisposition) (EnsureResult, error) {
	_, f := c.run(ctx, mut)
	if f != nil {
		switch f.Kind {
		case KindTimedOut, KindCancelled:
			// The process launched; the mutation may have reached GitHub. Requery.
			return c.verifyPostMutation(ctx, req, success), nil
		default:
			// A start failure never mutated anything.
			return EnsureResult{Disposition: EnsureFailed}, f
		}
	}
	// A zero exit is the expected success path; a non-zero exit is a possible lost
	// response. Both are resolved by the same requery, never by trusting the code.
	return c.verifyPostMutation(ctx, req, success), nil
}

// verifyPostMutation re-derives the postcondition after a mutation by querying
// BOTH by head branch and by PR number. Both views must name one open, ready PR
// whose head commit, base, title, and body equal the request: that is
// created/updated. An open PR observed in a different state is contended. An
// inability to establish a single open PR through both views — a failed,
// timed-out, unmatched, or undecodable query, or zero/many open PRs — is
// unknown, never a fabricated success and never a second mutation.
func (c *Client) verifyPostMutation(ctx context.Context, req EnsurePullRequestRequest, success EnsureDisposition) EnsureResult {
	byHead, ok := c.verifyListOpen(ctx, req)
	if !ok {
		return EnsureResult{Disposition: EnsureUnknown}
	}
	byNumber, ok := c.verifyViewByNumber(ctx, req, byHead.Number)
	if !ok {
		return EnsureResult{Disposition: EnsureUnknown}
	}
	if byHead.Number != byNumber.Number {
		return EnsureResult{Disposition: EnsureUnknown}
	}
	if !matchesRequest(byHead, req) || !matchesRequest(byNumber, req) {
		// A PR exists but is not the exact requested end state — a race is reported
		// honestly, never rolled back.
		return EnsureResult{Disposition: EnsureContended}
	}
	return EnsureResult{Disposition: success, PR: byNumber}
}

// probeByHead lists ALL states of PRs for the explicit repository and exact head
// branch. A launch/timeout/cancel Failure and a non-zero exit are both external
// probe failures — an errored probe is never read as clean absence (learnings:
// probe-error-is-not-clean-absence). Decoding validates every element.
func (c *Client) probeByHead(ctx context.Context, req EnsurePullRequestRequest) ([]PullRequest, error) {
	res, f := c.run(ctx, runRequest{
		op:      ensureOp,
		args:    listArgs(req, "all"),
		network: true,
	})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(ensureOp, StageInvoke, KindExternal,
			"gh pr list failed: "+stderrExcerpt(res.stderr), nil)
	}
	return decodePullRequestList(ensureOp, res.stdout)
}

// verifyListOpen queries the open PRs for the exact head branch and requires
// exactly one. Any transport/decoding failure, or a count other than one, means
// the single-open-PR postcondition could not be established (ok=false); the
// caller maps that to unknown.
func (c *Client) verifyListOpen(ctx context.Context, req EnsurePullRequestRequest) (PullRequest, bool) {
	res, f := c.run(ctx, runRequest{
		op:      ensureOp,
		args:    listArgs(req, "open"),
		network: true,
	})
	if f != nil || res.exitCode != 0 {
		return PullRequest{}, false
	}
	prs, err := decodePullRequestList(ensureOp, res.stdout)
	if err != nil {
		return PullRequest{}, false
	}
	open := openPRs(prs)
	if len(open) != 1 {
		return PullRequest{}, false
	}
	return open[0], true
}

// verifyViewByNumber reads one PR by its number. Any transport/decoding failure
// means the by-number view could not be established (ok=false).
func (c *Client) verifyViewByNumber(ctx context.Context, req EnsurePullRequestRequest, number int) (PullRequest, bool) {
	res, f := c.run(ctx, runRequest{
		op:      ensureOp,
		args:    viewArgs(req, number),
		network: true,
	})
	if f != nil || res.exitCode != 0 {
		return PullRequest{}, false
	}
	pr, err := decodePullRequest(ensureOp, res.stdout)
	if err != nil {
		return PullRequest{}, false
	}
	return pr, true
}

// matchesRequest reports whether an observed PR is exactly the requested
// ready-for-review end state: open, not a draft, at the expected head commit,
// and equal in base, title, and body. The opaque version is deliberately not
// part of this comparison — it is the CAS token for authorizing an edit, not the
// definition of the desired content.
func matchesRequest(pr PullRequest, req EnsurePullRequestRequest) bool {
	return pr.State == StateOpen &&
		!pr.Draft &&
		pr.HeadCommit == req.ExpectedHead &&
		pr.BaseBranch == req.BaseBranch &&
		pr.Title == req.Title &&
		pr.Body == req.Body
}

// openPRs returns the subset of PRs in the open state, preserving order.
func openPRs(prs []PullRequest) []PullRequest {
	var out []PullRequest
	for _, pr := range prs {
		if pr.State == StateOpen {
			out = append(out, pr)
		}
	}
	return out
}

// listArgs builds a `gh pr list` argument vector for the explicit repository and
// exact head branch in the given state filter ("all" for the authoritative
// probe, "open" for the post-mutation verify). --repo is always explicit so a
// caller's CWD or GH_REPO cannot retarget the query.
func listArgs(req EnsurePullRequestRequest, state string) []string {
	return []string{
		"pr", "list",
		"--repo", req.Repository.Spec(),
		"--head", req.HeadBranch,
		"--state", state,
		"--json", prJSONFields,
	}
}

// viewArgs builds a `gh pr view <number>` argument vector for the explicit
// repository.
func viewArgs(req EnsurePullRequestRequest, number int) []string {
	return []string{
		"pr", "view", strconv.Itoa(number),
		"--repo", req.Repository.Spec(),
		"--json", prJSONFields,
	}
}

// createRequest builds the `gh pr create` invocation. The head, base, repository,
// and title are explicit argv; the authored body reaches gh only on stdin via
// `--body-file -`, never as an argv value or a temporary file.
func createRequest(req EnsurePullRequestRequest) runRequest {
	return runRequest{
		op: ensureOp,
		args: []string{
			"pr", "create",
			"--repo", req.Repository.Spec(),
			"--head", req.HeadBranch,
			"--base", req.BaseBranch,
			"--title", req.Title,
			"--body-file", "-",
		},
		stdin:   []byte(req.Body),
		network: true,
		write:   true,
	}
}

// editRequest builds the `gh pr edit <number>` invocation converging the PR onto
// the request's base, title, and body. The head is never edited (a head commit
// other than ExpectedHead is refused before reaching here); the body travels
// only on stdin.
func editRequest(req EnsurePullRequestRequest, number int) runRequest {
	return runRequest{
		op: ensureOp,
		args: []string{
			"pr", "edit", strconv.Itoa(number),
			"--repo", req.Repository.Spec(),
			"--base", req.BaseBranch,
			"--title", req.Title,
			"--body-file", "-",
		},
		stdin:   []byte(req.Body),
		network: true,
		write:   true,
	}
}

// decodePullRequestList decodes a JSON ARRAY of PR objects and validates every
// element with the same required-field rules as a single view. An empty array is
// a legitimate "no PR" result (unlike decodeSolePullRequest, which requires
// one); a malformed element is invalid-output/invalid-state, never zero-value
// data mixed in with valid rows.
func decodePullRequestList(op string, data []byte) ([]PullRequest, error) {
	var rawList []prViewJSON
	if err := json.Unmarshal(data, &rawList); err != nil {
		return nil, newFailure(op, StageDecode, KindInvalidOutput, "pull-request list output is not valid JSON", err)
	}
	out := make([]PullRequest, 0, len(rawList))
	for _, raw := range rawList {
		pr, err := raw.toPullRequest(op)
		if err != nil {
			return nil, err
		}
		out = append(out, pr)
	}
	return out, nil
}

// validateEnsureRequest rejects a request that cannot compose a safe, explicit
// gh invocation: an invalid repository identity, an empty head/base branch or
// title, or an ExpectedHead that is not a full lowercase-hex object id. The body
// may be empty; ExpectedVersion may be empty (create-or-adopt only).
func validateEnsureRequest(req EnsurePullRequestRequest) error {
	if err := validateRepository(req.Repository); err != nil {
		return newFailure(ensureOp, StageValidate, KindInvalidInput, "repository identity invalid: "+err.Error(), err)
	}
	if req.HeadBranch == "" {
		return newFailure(ensureOp, StageValidate, KindInvalidInput, "head branch is empty", nil)
	}
	if err := validateFullObjectID(req.ExpectedHead); err != nil {
		return newFailure(ensureOp, StageValidate, KindInvalidInput, "expected head oid invalid: "+err.Error(), err)
	}
	if req.BaseBranch == "" {
		return newFailure(ensureOp, StageValidate, KindInvalidInput, "base branch is empty", nil)
	}
	if req.Title == "" {
		return newFailure(ensureOp, StageValidate, KindInvalidInput, "title is empty", nil)
	}
	return nil
}
