package githubcli

// This file owns FindOpenPullRequestsByHead: the read-only PR reprobe
// `change mark-implemented` uses to VERIFY (never create or edit) that exactly
// one ready pull request exists for a published feature head before the
// implemented metadata transition. It reuses the same authoritative
// `gh pr list --repo <spec> --head <branch>` query the publication path probes
// with, but performs no mutation of any kind and returns the open PRs verbatim
// for the app layer to check against the requested head, base, and reference.
//
// An errored probe is never read as clean absence: a launch/timeout/cancel
// Failure and any non-zero exit are external probe failures, not "no PR here"
// (learning probe-error-is-not-clean-absence). Every decoded element is
// validated with the same required-field rules as a single view.

import "context"

// probeOp labels every Failure raised while probing pull requests read-only.
const probeOp = "probe-pull-requests"

// FindOpenPullRequestsByHead lists the OPEN pull requests GitHub reports for the
// explicit repository and exact head branch. --repo is always explicit so a
// caller's CWD or GH_REPO cannot retarget the query. It never creates, edits, or
// otherwise mutates: it is the authoritative read a reprobe keys on. A transport
// failure or a non-zero exit is returned as a typed *Failure, never an empty
// slice.
func (c *Client) FindOpenPullRequestsByHead(ctx context.Context, repo Repository, headBranch string) ([]PullRequest, error) {
	if err := validateRepository(repo); err != nil {
		return nil, newFailure(probeOp, StageValidate, KindInvalidInput, "repository identity invalid: "+err.Error(), err)
	}
	if headBranch == "" {
		return nil, newFailure(probeOp, StageValidate, KindInvalidInput, "head branch is empty", nil)
	}
	res, f := c.run(ctx, runRequest{
		op: probeOp,
		args: []string{
			"pr", "list",
			"--repo", repo.Spec(),
			"--head", headBranch,
			"--state", "open",
			"--json", prJSONFields,
		},
		network: true,
	})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(probeOp, StageInvoke, KindExternal,
			"gh pr list failed: "+stderrExcerpt(res.stderr), nil)
	}
	prs, err := decodePullRequestList(probeOp, res.stdout)
	if err != nil {
		return nil, err
	}
	return openPRs(prs), nil
}
