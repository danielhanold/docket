package githubcli

// This file owns EditPullRequestBody: the probe→act→verify body edit of one
// exact pull request by number, in any state (a merged PR included). It is the
// primitive close-out, cleanup, and `repository repair --pr-backlinks` use to
// repoint a merged PR's docket-owned backlink block at the archived change
// record. It mirrors retarget.go's fixed shape:
//
//	1. probe the PR by number for its current body and opaque revision;
//	2. a body already equal to the request is `already` — no edit (idempotency
//	   keyed on the promised remote state, never on the revision);
//	3. an edit is authorized only when ExpectedRevision equals the live revision;
//	   an empty or mismatched revision is `contended` and the PR is untouched;
//	4. `gh pr edit <n> --repo <spec> --body-file -` carries the body on stdin
//	   only — never in argv or a diagnostic — and edits nothing else; and
//	5. a fresh by-number probe re-derives the postcondition: an equal body is
//	   `edited`; a body still equal to the pre-edit body means the edit did not
//	   land (refused, locked, or lost) and is `unknown`, carrying a redacted
//	   stderr excerpt when gh exited non-zero; a third body is `contended` (a
//	   race, reported, never rolled back); and a probe that cannot be
//	   established is `unknown`.
//
// An errored probe is never read as clean absence (learning
// probe-error-is-not-clean-absence): a launch/timeout/cancel Failure or a
// non-zero exit is `unknown`, and `unknown` never authorizes the edit.

import (
	"context"
	"strconv"
)

// bodyEditOp labels every Failure raised while editing a pull-request body.
const bodyEditOp = "edit-pull-request-body"

// BodyEditOutcome is the closed set of body-edit outcomes.
type BodyEditOutcome string

const (
	// BodyEdited: the body was replaced and the postcondition verified.
	BodyEdited BodyEditOutcome = "edited"
	// BodyAlready: the PR already carried exactly the requested body; no edit.
	BodyAlready BodyEditOutcome = "already"
	// BodyContended: the live revision diverged from ExpectedRevision, or the
	// verified body is neither the request nor the pre-edit body (someone else
	// wrote it); the PR was not overwritten.
	BodyContended BodyEditOutcome = "contended"
	// BodyUnknown: an external probe could not establish the truth (or the input
	// was invalid); nothing was authorized. The returned error is the diagnostic.
	BodyUnknown BodyEditOutcome = "unknown"
)

// EditPullRequestBody replaces one exact PR's body, idempotently.
// edited/already/contended are value outcomes returned with a nil error; unknown
// carries a typed *Failure. The PR snapshot is populated on edited/already.
func (c *Client) EditPullRequestBody(ctx context.Context, repo Repository, number int, expectedRevision, body string) (BodyEditOutcome, PullRequest, error) {
	if err := validateRepository(repo); err != nil {
		return BodyUnknown, PullRequest{}, newFailure(bodyEditOp, StageValidate, KindInvalidInput, "repository identity invalid: "+err.Error(), err)
	}
	if number <= 0 {
		return BodyUnknown, PullRequest{}, newFailure(bodyEditOp, StageValidate, KindInvalidInput, "pull request number must be positive", nil)
	}

	pr, err := c.ViewPullRequest(ctx, repo, number)
	if err != nil {
		return BodyUnknown, PullRequest{}, err
	}
	if pr.Body == body {
		return BodyAlready, pr, nil
	}
	if expectedRevision == "" || expectedRevision != pr.Revision {
		return BodyContended, PullRequest{}, nil
	}

	res, mf := c.run(ctx, runRequest{
		op: bodyEditOp,
		args: []string{
			"pr", "edit", strconv.Itoa(number),
			"--repo", repo.Spec(),
			"--body-file", "-",
		},
		stdin:   []byte(body),
		network: true,
		write:   true,
	})
	if mf != nil && mf.Stage == StageLaunch {
		// gh never started; nothing was mutated.
		return BodyUnknown, PullRequest{}, mf
	}
	// A zero exit, a non-zero exit, a timeout, or a cancel may all have reached
	// GitHub: resolve every one by the same fresh verify probe.
	after, err := c.ViewPullRequest(ctx, repo, number)
	if err != nil {
		return BodyUnknown, PullRequest{}, err
	}
	if after.Body == body {
		return BodyEdited, after, nil
	}
	if after.Body == pr.Body {
		// The body never moved: the edit did not land. That is a refusal or a
		// lost write, never a race.
		return BodyUnknown, PullRequest{}, bodyEditNotApplied(res, mf)
	}
	return BodyContended, PullRequest{}, nil
}

// bodyEditNotApplied is the diagnostic for an edit the verify probe proved did
// not land: the run Failure (timeout/cancel) when there is one, else gh's exit
// with a bounded, redacted stderr excerpt.
func bodyEditNotApplied(res runResult, mf *Failure) *Failure {
	if mf != nil {
		return newFailure(bodyEditOp, StageInvoke, mf.Kind, "the body edit was not applied: "+mf.Detail, mf)
	}
	detail := "the body edit was not applied"
	if res.exitCode != 0 {
		detail = "gh pr edit exited " + strconv.Itoa(res.exitCode) + " and the body edit was not applied"
		if ex := stderrExcerpt(res.stderr); ex != "" {
			detail += ": " + ex
		}
	}
	return newFailure(bodyEditOp, StageInvoke, KindExternal, detail, nil)
}
