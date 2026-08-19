package githubcli

// This file owns EnsureComment and FindComment: the idempotent, marker-keyed PR
// comment a finalize-blocked or run-halted checkpoint posts, and the read-only
// probe that keys it. The idempotency key is the live comment whose body starts
// with a Docket-owned attempt marker line — re-derived from a fresh probe on
// every call, never a local proxy — so a replayed attempt (a crash between the
// comment and its durable marker section) finds and reuses the same comment
// rather than posting a duplicate.
//
// The sequence mirrors ensure.go's fixed shape:
//
//	1. probe the PR's comments and search for the marker prefix;
//	2. a match is `already`, returning that comment's url, no create;
//	3. otherwise `gh pr comment <n> --body-file -` posts the body — the authored
//	   bytes cross ONLY on stdin, never in an argv element or a diagnostic; and
//	4. the postcondition is re-derived by the same marker probe: a match is
//	   `created`; an inability to re-establish it is `unknown`, never a fabricated
//	   success and never a second post.
//
// A probe that errors is never read as clean absence: a launch/timeout/cancel
// Failure or a non-zero exit is `unknown` (retain), and `unknown` never
// authorizes a create — Docket never claims a comment exists it could not see,
// and never posts a possible duplicate (learning probe-error-is-not-clean-absence).

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
)

// commentOp labels every Failure raised while ensuring or finding a PR comment.
const commentOp = "ensure-comment"

// CommentOutcome is the closed set of idempotent-comment outcomes.
type CommentOutcome string

const (
	// CommentCreated: no marked comment existed and one was posted and verified.
	CommentCreated CommentOutcome = "created"
	// CommentAlready: a comment carrying the marker already existed; none posted.
	CommentAlready CommentOutcome = "already"
	// CommentUnknown: an external probe could not establish the truth (including
	// invalid input); nothing was posted. The returned error carries the diagnostic.
	CommentUnknown CommentOutcome = "unknown"
)

// commentsEnvelope is the documented nested shape `gh pr view --json comments`
// emits: a comments array of objects each carrying a body and its permalink url.
// Decoding keys on these field names, never on display text.
type commentsEnvelope struct {
	Comments []struct {
		Body string `json:"body"`
		URL  string `json:"url"`
	} `json:"comments"`
}

// FindComment reports whether a comment whose body starts with marker exists on
// the PR, returning that comment's url. The three outcomes are distinct: found
// (true, url, nil), cleanly absent (false, "", nil), and error (false, "", err).
// A transport failure or a non-zero exit is a typed *Failure, never read as clean
// absence.
func (c *Client) FindComment(ctx context.Context, repo Repository, number int, marker string) (bool, string, error) {
	if err := validateCommentProbe(repo, number, marker); err != nil {
		return false, "", err
	}
	res, f := c.run(ctx, runRequest{
		op: commentOp,
		args: []string{
			"pr", "view", strconv.Itoa(number),
			"--repo", repo.Spec(),
			"--json", "comments",
		},
		network: true,
	})
	if f != nil {
		return false, "", f
	}
	if res.exitCode != 0 {
		return false, "", newFailure(commentOp, StageInvoke, KindExternal,
			"gh pr view --json comments failed: "+stderrExcerpt(res.stderr), nil)
	}
	var env commentsEnvelope
	if err := json.Unmarshal(res.stdout, &env); err != nil {
		return false, "", newFailure(commentOp, StageDecode, KindInvalidOutput, "comment list output is not valid JSON", err)
	}
	for _, cm := range env.Comments {
		if strings.HasPrefix(cm.Body, marker) {
			return true, cm.URL, nil
		}
	}
	return false, "", nil
}

// EnsureComment ensures exactly one comment whose body starts with marker.
// created/already are value outcomes returned with a nil error; unknown carries a
// typed *Failure. The returned url names the marked comment on created/already
// and is empty on unknown.
func (c *Client) EnsureComment(ctx context.Context, repo Repository, number int, marker, body string) (CommentOutcome, string, error) {
	if err := validateCommentProbe(repo, number, marker); err != nil {
		return CommentUnknown, "", err
	}
	// The marker must be the leading bytes of the body, or the idempotency key
	// would never re-find what this call posts — a replay would post a duplicate.
	if !strings.HasPrefix(body, marker) {
		return CommentUnknown, "", newFailure(commentOp, StageValidate, KindInvalidInput, "comment body does not start with the marker", nil)
	}

	found, url, err := c.FindComment(ctx, repo, number, marker)
	if err != nil {
		return CommentUnknown, "", err
	}
	if found {
		return CommentAlready, url, nil
	}

	_, mf := c.run(ctx, runRequest{
		op: commentOp,
		args: []string{
			"pr", "comment", strconv.Itoa(number),
			"--repo", repo.Spec(),
			"--body-file", "-",
		},
		stdin:   []byte(body),
		network: true,
	})
	if mf != nil && mf.Stage == StageLaunch {
		// gh never started; nothing was posted. Retain as unknown.
		return CommentUnknown, "", mf
	}
	// A timeout/cancel/non-zero exit may have posted the comment, and a zero exit
	// is the expected success — both are resolved by the same marker reprobe.
	found, url, err = c.FindComment(ctx, repo, number, marker)
	if err != nil {
		return CommentUnknown, "", err
	}
	if found {
		return CommentCreated, url, nil
	}
	// The post could not be confirmed present. Retain as unknown rather than
	// fabricate a success or post a second time.
	return CommentUnknown, "", newFailure(commentOp, StageInvoke, KindExternal,
		"comment post could not be verified present after the attempt", nil)
}

// validateCommentProbe rejects a request that cannot compose a safe, explicit gh
// invocation or a usable idempotency key.
func validateCommentProbe(repo Repository, number int, marker string) error {
	if err := validateRepository(repo); err != nil {
		return newFailure(commentOp, StageValidate, KindInvalidInput, "repository identity invalid: "+err.Error(), err)
	}
	if number <= 0 {
		return newFailure(commentOp, StageValidate, KindInvalidInput, "pull request number must be positive", nil)
	}
	if marker == "" {
		return newFailure(commentOp, StageValidate, KindInvalidInput, "comment marker is empty", nil)
	}
	return nil
}
