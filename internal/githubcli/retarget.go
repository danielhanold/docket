package githubcli

// This file owns RetargetPullRequest: the probe→act→verify base retarget of one
// exact pull request onto a new base branch. It is the primitive a stack parent's
// finalize uses to move each open child PR off the parent's branch and onto the
// parent's effective base before the parent merges, so the children never
// silently re-target themselves to the integration branch when the parent's
// branch is deleted.
//
// The sequence mirrors ensure.go's fixed shape:
//
//	1. probe the PR by number for its current base and opaque version;
//	2. if the PR is ALREADY at newBase the promised end-state holds — `already`,
//	   no edit (idempotency keyed on the promised remote state, never on the CAS
//	   version, which a completed retarget has already changed);
//	3. otherwise an edit is authorized only when the caller's ExpectedVersion
//	   equals the live version — an empty or mismatched version is `contended` and
//	   leaves the PR untouched;
//	4. `gh pr edit <n> --base <newBase>` carries the base as an explicit argv flag,
//	   no authored bytes; and
//	5. the postcondition is re-derived by a fresh by-number probe: base == newBase
//	   is `retargeted`, a base still different is `contended`, and an inability to
//	   re-establish the snapshot is `unknown`.
//
// An errored probe is never read as clean absence: a launch/timeout/cancel
// Failure or a non-zero exit is `unknown` (retain), never a definite outcome —
// and `unknown` never authorizes the edit (learning
// probe-error-is-not-clean-absence). There is no compensating rollback.

import (
	"context"
	"strconv"
)

// retargetOp labels every Failure raised while retargeting a pull request base.
const retargetOp = "retarget-pull-request"

// RetargetOutcome is the closed set of base-retarget outcomes.
type RetargetOutcome string

const (
	// RetargetRetargeted: the base was moved to newBase and the postcondition
	// verified.
	RetargetRetargeted RetargetOutcome = "retargeted"
	// RetargetAlready: the PR already sat at newBase; no edit was issued.
	RetargetAlready RetargetOutcome = "already"
	// RetargetContended: the live version diverged from ExpectedVersion, or a
	// verified snapshot still showed a different base; the PR is left untouched.
	RetargetContended RetargetOutcome = "contended"
	// RetargetUnknown: an external probe could not establish the truth; nothing
	// was authorized. The returned error carries the diagnostic.
	RetargetUnknown RetargetOutcome = "unknown"
)

// RetargetPullRequest moves one exact PR's base onto newBase, idempotently.
// retargeted/already/contended are value outcomes returned with a nil error;
// unknown (including invalid input) carries a typed *Failure so the caller has a
// diagnostic. The PR snapshot is populated on retargeted/already and is the zero
// value otherwise.
func (c *Client) RetargetPullRequest(ctx context.Context, repo Repository, number int, expectedVersion, newBase string) (RetargetOutcome, PullRequest, error) {
	if err := validateRepository(repo); err != nil {
		return RetargetUnknown, PullRequest{}, newFailure(retargetOp, StageValidate, KindInvalidInput, "repository identity invalid: "+err.Error(), err)
	}
	if number <= 0 {
		return RetargetUnknown, PullRequest{}, newFailure(retargetOp, StageValidate, KindInvalidInput, "pull request number must be positive", nil)
	}
	if newBase == "" {
		return RetargetUnknown, PullRequest{}, newFailure(retargetOp, StageValidate, KindInvalidInput, "new base branch is empty", nil)
	}

	pr, f := c.viewPullRequest(ctx, repo, number)
	if f != nil {
		return RetargetUnknown, PullRequest{}, f
	}

	// The promised end-state — the PR at newBase — is the idempotency key, checked
	// before the version gate: a completed retarget has already changed the CAS
	// version, so keying idempotency on the version would misread a success as
	// contention.
	if pr.BaseBranch == newBase {
		return RetargetAlready, pr, nil
	}

	// An edit is authorized only by the exact live version.
	if expectedVersion == "" || expectedVersion != pr.Version {
		return RetargetContended, PullRequest{}, nil
	}

	_, mf := c.run(ctx, runRequest{
		op: retargetOp,
		args: []string{
			"pr", "edit", strconv.Itoa(number),
			"--repo", repo.Spec(),
			"--base", newBase,
		},
		network: true,
	})
	if mf != nil && mf.Stage == StageLaunch {
		// gh never started; nothing was mutated. Retain as unknown.
		return RetargetUnknown, PullRequest{}, mf
	}
	// A timeout/cancel/non-zero exit may have reached GitHub, and a zero exit is
	// the expected success — both are resolved by the same fresh verify probe, not
	// by trusting the process outcome.
	return c.verifyRetarget(ctx, repo, number, newBase)
}

// verifyRetarget re-derives the postcondition by a fresh by-number probe. base ==
// newBase is retargeted; a still-different base is contended (a race, reported
// honestly, never rolled back); a probe that cannot be established is unknown.
func (c *Client) verifyRetarget(ctx context.Context, repo Repository, number int, newBase string) (RetargetOutcome, PullRequest, error) {
	pr, f := c.viewPullRequest(ctx, repo, number)
	if f != nil {
		return RetargetUnknown, PullRequest{}, f
	}
	if pr.BaseBranch == newBase {
		return RetargetRetargeted, pr, nil
	}
	return RetargetContended, PullRequest{}, nil
}

// viewPullRequest reads one PR by number for the explicit repository with the
// standard field set. --repo is always explicit so a caller's CWD or GH_REPO
// cannot retarget the query. A transport failure, a non-zero exit, or a decode
// hazard is returned as a typed *Failure — never a zero-value PR read as truth.
func (c *Client) viewPullRequest(ctx context.Context, repo Repository, number int) (PullRequest, *Failure) {
	res, f := c.run(ctx, runRequest{
		op: retargetOp,
		args: []string{
			"pr", "view", strconv.Itoa(number),
			"--repo", repo.Spec(),
			"--json", prJSONFields,
		},
		network: true,
	})
	if f != nil {
		return PullRequest{}, f
	}
	if res.exitCode != 0 {
		return PullRequest{}, newFailure(retargetOp, StageInvoke, KindExternal,
			"gh pr view failed: "+stderrExcerpt(res.stderr), nil)
	}
	pr, err := decodePullRequest(retargetOp, res.stdout)
	if err != nil {
		if ff, ok := AsFailure(err); ok {
			return PullRequest{}, ff
		}
		return PullRequest{}, newFailure(retargetOp, StageDecode, KindInvalidOutput, "pull-request view undecodable", err)
	}
	return pr, nil
}
