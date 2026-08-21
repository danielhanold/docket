package githubcli

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// This file owns the merge-method vocabulary and policy: the closed MergeMethod
// set, the capability probes over GitHub's repository settings and active
// branch rules, and the pure fixed-priority selection "rebase, else merge, else
// squash, else unavailable". There is deliberately NO configuration surface —
// the preference order is product policy. Repository permissions and branch
// rules compose by intersection; unobservable or malformed policy fails closed
// (three-outcome discipline; learnings: probe-error-is-not-clean-absence).

// MergeMethod is the closed vocabulary of merge methods Docket can select.
type MergeMethod string

const (
	// MethodRebase → `gh pr merge --rebase`.
	MethodRebase MergeMethod = "rebase"
	// MethodMerge → `gh pr merge --merge` (a merge commit).
	MethodMerge MergeMethod = "merge"
	// MethodSquash → `gh pr merge --squash`.
	MethodSquash MergeMethod = "squash"
)

// mergeFlag renders the gh `pr merge` flag for a vocabulary member, and nothing
// for anything else — the act path guards on a non-empty flag rather than
// defaulting permissively.
func (m MergeMethod) mergeFlag() string {
	switch m {
	case MethodRebase:
		return "--rebase"
	case MethodMerge:
		return "--merge"
	case MethodSquash:
		return "--squash"
	default:
		return ""
	}
}

// methodSet is a capability set over the three methods.
type methodSet struct {
	rebase, merge, squash bool
}

// intersect composes two capability sets; multiple applicable restrictions
// always narrow, never widen.
func (s methodSet) intersect(o methodSet) methodSet {
	return methodSet{
		rebase: s.rebase && o.rebase,
		merge:  s.merge && o.merge,
		squash: s.squash && o.squash,
	}
}

// list renders the permitted methods in preference order, for diagnostics.
func (s methodSet) list() []MergeMethod {
	out := []MergeMethod{}
	if s.rebase {
		out = append(out, MethodRebase)
	}
	if s.merge {
		out = append(out, MethodMerge)
	}
	if s.squash {
		out = append(out, MethodSquash)
	}
	return out
}

// selectMergeMethod is the pure ordered selection over the effective set:
// rebase, else merge, else squash, else unavailable. `--admin` never widens the
// set or reorders it.
func selectMergeMethod(eff methodSet) (MergeMethod, bool) {
	switch {
	case eff.rebase:
		return MethodRebase, true
	case eff.merge:
		return MethodMerge, true
	case eff.squash:
		return MethodSquash, true
	default:
		return "", false
	}
}

// mergeMethodOp labels every Failure raised while observing merge-method policy.
const mergeMethodOp = "merge-method-capability"

// repoMethodsJSON decodes the repository merge-method settings. Pointer fields
// force explicit presence: a missing, malformed, or unrecognized boolean is
// invalid external output, never a permissive default.
type repoMethodsJSON struct {
	AllowRebase *bool `json:"allow_rebase_merge"`
	AllowMerge  *bool `json:"allow_merge_commit"`
	AllowSquash *bool `json:"allow_squash_merge"`
}

// probeRepoMergeMethods reads the repository-enabled merge methods from
// GitHub's repository endpoint via `gh api`, addressed by the validated host
// and owner/name — never by CWD inference.
func (c *Client) probeRepoMergeMethods(ctx context.Context, repo Repository) (methodSet, *Failure) {
	res, f := c.run(ctx, runRequest{
		op:      mergeMethodOp,
		args:    []string{"api", "--hostname", repo.Host, "repos/" + repo.Owner + "/" + repo.Name},
		network: true,
	})
	if f != nil {
		return methodSet{}, f
	}
	if res.exitCode != 0 {
		return methodSet{}, newFailure(mergeMethodOp, StageInvoke, KindExternal,
			"gh api repository settings failed: "+stderrExcerpt(res.stderr), nil)
	}
	var raw repoMethodsJSON
	if err := json.Unmarshal(res.stdout, &raw); err != nil {
		return methodSet{}, newFailure(mergeMethodOp, StageDecode, KindInvalidOutput, "repository settings are not valid JSON", err)
	}
	if raw.AllowRebase == nil || raw.AllowMerge == nil || raw.AllowSquash == nil {
		return methodSet{}, newFailure(mergeMethodOp, StageDecode, KindInvalidOutput,
			"repository settings omit a merge-method capability field", nil)
	}
	return methodSet{rebase: *raw.AllowRebase, merge: *raw.AllowMerge, squash: *raw.AllowSquash}, nil
}

// branchRuleJSON is one active rule from GitHub's branch-rules endpoint.
type branchRuleJSON struct {
	Type       string          `json:"type"`
	Parameters json.RawMessage `json:"parameters"`
}

// prRuleParamsJSON extracts the merge-method restriction from a pull_request
// rule. The pointer distinguishes "key absent — no restriction" from "key
// present and empty — invalid policy that fails closed".
type prRuleParamsJSON struct {
	AllowedMergeMethods *[]string `json:"allowed_merge_methods"`
}

// probeBranchMergeRules reads the active rules for the exact PR base branch and
// composes the branch-permitted method set: every applicable
// allowed_merge_methods restriction intersects, and required_linear_history
// removes merge-commit semantics. No method-specific rule means no additional
// restriction. The full branch name is path-escaped so a stacked destination
// like "feat/parent" is one endpoint segment. Constraints GitHub does not
// expose through this read surface can still reject the later merge command —
// that remains an ordinary denial and never triggers fallback.
func (c *Client) probeBranchMergeRules(ctx context.Context, repo Repository, baseBranch string) (methodSet, *Failure) {
	if baseBranch == "" {
		return methodSet{}, newFailure(mergeMethodOp, StageValidate, KindInvalidInput, "base branch is empty", nil)
	}
	path := "repos/" + repo.Owner + "/" + repo.Name + "/rules/branches/" + url.PathEscape(baseBranch)
	res, f := c.run(ctx, runRequest{
		op:      mergeMethodOp,
		args:    []string{"api", "--hostname", repo.Host, path},
		network: true,
	})
	if f != nil {
		return methodSet{}, f
	}
	if res.exitCode != 0 {
		return methodSet{}, newFailure(mergeMethodOp, StageInvoke, KindExternal,
			"gh api branch rules failed: "+stderrExcerpt(res.stderr), nil)
	}
	var rules []branchRuleJSON
	if err := json.Unmarshal(res.stdout, &rules); err != nil {
		return methodSet{}, newFailure(mergeMethodOp, StageDecode, KindInvalidOutput, "branch rules are not a valid JSON array", err)
	}
	permitted := methodSet{rebase: true, merge: true, squash: true}
	for _, r := range rules {
		switch r.Type {
		case "pull_request":
			if len(r.Parameters) == 0 {
				continue
			}
			var p prRuleParamsJSON
			if err := json.Unmarshal(r.Parameters, &p); err != nil {
				return methodSet{}, newFailure(mergeMethodOp, StageDecode, KindInvalidOutput, "pull_request rule parameters undecodable", err)
			}
			if p.AllowedMergeMethods == nil {
				continue // rule present, no merge-method restriction
			}
			if len(*p.AllowedMergeMethods) == 0 {
				return methodSet{}, newFailure(mergeMethodOp, StageDecode, KindInvalidOutput,
					"allowed_merge_methods is present but empty", nil)
			}
			var s methodSet
			for _, tok := range *p.AllowedMergeMethods {
				switch tok {
				case "rebase":
					s.rebase = true
				case "merge":
					s.merge = true
				case "squash":
					s.squash = true
				default:
					return methodSet{}, newFailure(mergeMethodOp, StageDecode, KindInvalidOutput,
						"unknown merge-method token "+strconv.Quote(tok)+" in branch rules", nil)
				}
			}
			permitted = permitted.intersect(s)
		case "required_linear_history":
			permitted.merge = false
		}
	}
	return permitted, nil
}
