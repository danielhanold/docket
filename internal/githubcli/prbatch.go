package githubcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// This file owns ViewPullRequestsBatch: one aliased-GraphQL read that resolves
// several exact-number pull requests in a SINGLE `gh api graphql` process, for
// the maintenance sweep's batched discovery. It never lists, never searches by
// head, and never paginates — every slot is an exact `pullRequest(number:)`
// alias.
//
// A slot that cannot be resolved to a trustworthy snapshot is UNKNOWN
// (Found=false), never a closed/absent verdict (learning
// probe-error-is-not-clean-absence): a missing or null alias, a server number
// that disagrees with the requested one, and a malformed required field all
// yield Found=false for THAT slot only. A whole-response hazard — a transport
// failure, a non-zero exit, a malformed envelope, or a non-empty top-level
// errors array (even with data present) — fails the WHOLE batch as a typed
// *Failure and is never shrunk into per-record retries.

// batchOp labels every Failure raised while reading a pull-request batch.
const batchOp = "view-pull-requests-batch"

// batchMaxNumbers caps the numbers one batch may carry. It is THIS adapter's
// conservative request size, NOT a GitHub-imposed limit: a smaller ceiling keeps
// any one query's node budget and response payload bounded and predictable.
const batchMaxNumbers = 25

// BatchPRResult is one exact-number slot of a batch read. Found=false is UNKNOWN
// — a missing or null alias, a server number that disagrees with the request, or
// a malformed required field — NEVER a closed/absent verdict. PR is normalized
// exactly as ViewPullRequest normalizes (the shared toPullRequest path), so its
// Version is byte-identical to a single view of the same snapshot. MergedAtUTC
// and MergeCommit are populated only for a merged PR.
type BatchPRResult struct {
	Found       bool
	PR          PullRequest // normalized exactly as ViewPullRequest normalizes
	MergedAtUTC string      // merged PRs only
	MergeCommit string      // merged PRs only
}

// ViewPullRequestsBatch reads the exact-number pull requests named by numbers in
// one aliased GraphQL query and returns a result per requested number. It
// validates that numbers is non-empty, within the batchMaxNumbers cap, all
// positive, and all unique (KindInvalidInput otherwise) before any process runs.
// --repo is not used; owner/name are bound as typed String! variables so a
// caller's CWD or GH_REPO cannot retarget the query. A whole-response hazard is a
// returned typed *Failure and a nil map; an individually unresolvable slot is a
// Found=false result, never an inferred absence (probe-error-is-not-clean-absence).
func (c *Client) ViewPullRequestsBatch(ctx context.Context, repo Repository, numbers []int) (map[int]BatchPRResult, error) {
	if err := validateRepository(repo); err != nil {
		return nil, newFailure(batchOp, StageValidate, KindInvalidInput, "repository identity invalid: "+err.Error(), err)
	}
	if f := validateBatchNumbers(numbers); f != nil {
		return nil, f
	}
	res, f := c.run(ctx, runRequest{
		op: batchOp,
		// owner/name are passed with -f (RAW string) rather than -F: the query's
		// $owner/$name are GraphQL String! variables, and -F would coerce an
		// all-digit owner or repo name to a number and be rejected by the schema.
		args: []string{
			"api", "graphql",
			"-f", "query=" + batchGraphQLQuery(numbers),
			"-f", "owner=" + repo.Owner,
			"-f", "name=" + repo.Name,
		},
		network: true,
	})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(batchOp, StageInvoke, KindExternal,
			"gh api graphql failed: "+stderrExcerpt(res.stderr), nil)
	}
	out, df := decodeBatchResponse(batchOp, numbers, res.stdout)
	if df != nil {
		return nil, df
	}
	return out, nil
}

// validateBatchNumbers enforces the batch preconditions: at least one number, no
// more than batchMaxNumbers, every number positive, and no duplicates. It
// returns nil when the set is admissible.
func validateBatchNumbers(numbers []int) *Failure {
	if len(numbers) == 0 {
		return newFailure(batchOp, StageValidate, KindInvalidInput, "batch requires at least one pull-request number", nil)
	}
	if len(numbers) > batchMaxNumbers {
		return newFailure(batchOp, StageValidate, KindInvalidInput,
			fmt.Sprintf("batch of %d exceeds the %d-number limit", len(numbers), batchMaxNumbers), nil)
	}
	seen := make(map[int]struct{}, len(numbers))
	for _, n := range numbers {
		if n <= 0 {
			return newFailure(batchOp, StageValidate, KindInvalidInput, "pull-request number must be positive", nil)
		}
		if _, dup := seen[n]; dup {
			return newFailure(batchOp, StageValidate, KindInvalidInput, "batch numbers must be unique", nil)
		}
		seen[n] = struct{}{}
	}
	return nil
}

// batchAliasName is the deterministic alias for the i-th requested number:
// pr0, pr1, … The decoder keys results back by this same name, so the request
// order and the response mapping stay in lockstep.
func batchAliasName(i int) string { return "pr" + strconv.Itoa(i) }

// batchGraphQLQuery builds one query aliasing each requested number to an exact
// `pullRequest(number:)` field, in input order. Every alias selects the same
// field set a single view decodes (number, url, state, isDraft, reviewDecision,
// head/base refs, title, body) plus the merge facts (mergedAt, mergeCommit.oid).
// There is no pagination and no nested connection.
func batchGraphQLQuery(numbers []int) string {
	var b strings.Builder
	b.WriteString("query($owner:String!,$name:String!){repository(owner:$owner,name:$name){")
	for i, n := range numbers {
		fmt.Fprintf(&b, "%s: pullRequest(number: %d){number url state isDraft reviewDecision headRefName headRefOid baseRefName title body mergedAt mergeCommit { oid }} ",
			batchAliasName(i), n)
	}
	b.WriteString("}}")
	return b.String()
}

// batchEnvelope is the GraphQL response shape: data.repository carries the aliased
// pull-request values as raw messages (so a null alias is distinguishable from an
// object), and a non-empty errors array is a whole-batch hazard even alongside
// data.
type batchEnvelope struct {
	Data   *batchDataEnvelope  `json:"data"`
	Errors []batchGraphQLError `json:"errors"`
}

type batchDataEnvelope struct {
	Repository map[string]json.RawMessage `json:"repository"`
}

type batchGraphQLError struct {
	Message string `json:"message"`
}

// batchPRAlias is one aliased pull-request value: the standard view fields
// (embedded prViewJSON, decoded through the shared toPullRequest normalization)
// plus the merge facts. A pointer mergeCommit lets the decoder tell an absent
// object from an empty oid.
type batchPRAlias struct {
	prViewJSON
	MergedAt    *string `json:"mergedAt"`
	MergeCommit *struct {
		OID string `json:"oid"`
	} `json:"mergeCommit"`
}

// decodeBatchResponse interprets the GraphQL envelope strictly. A malformed
// envelope, or a non-empty top-level errors array, fails the WHOLE batch as a
// typed *Failure and returns a nil map. Otherwise each requested number gets a
// result: an absent or null alias, a number mismatch, or a malformed required
// field is Found=false (UNKNOWN) for that slot only.
func decodeBatchResponse(op string, numbers []int, stdout []byte) (map[int]BatchPRResult, *Failure) {
	var env batchEnvelope
	if err := json.Unmarshal(stdout, &env); err != nil {
		return nil, newFailure(op, StageDecode, KindInvalidOutput, "graphql batch response is not valid JSON", err)
	}
	if len(env.Errors) > 0 {
		// A non-empty errors array is a whole-batch hazard even when data is
		// present: the response is untrustworthy, so no slot is decoded.
		return nil, newFailure(op, StageInvoke, KindExternal,
			"gh api graphql reported errors: "+batchErrorSummary(env.Errors), nil)
	}
	if env.Data == nil {
		return nil, newFailure(op, StageDecode, KindInvalidOutput, "graphql batch response carried neither data nor errors", nil)
	}
	out := make(map[int]BatchPRResult, len(numbers))
	for i, num := range numbers {
		raw, ok := env.Data.Repository[batchAliasName(i)]
		if !ok || isJSONNull(raw) {
			out[num] = BatchPRResult{Found: false}
			continue
		}
		out[num] = decodeBatchAlias(op, num, raw)
	}
	return out, nil
}

// decodeBatchAlias resolves one non-null alias to a result. It reuses the
// package's single normalization (toPullRequest) so approval, state, oid
// validation, and the version token match a single view exactly. Any decode or
// validation hazard — malformed JSON, a rejected required field, a server number
// that disagrees with the request, or a merged PR missing its merge facts — is
// Found=false for this slot, never an error that fails the batch.
func decodeBatchAlias(op string, requested int, raw json.RawMessage) BatchPRResult {
	var alias batchPRAlias
	if err := json.Unmarshal(raw, &alias); err != nil {
		return BatchPRResult{Found: false}
	}
	pr, err := alias.prViewJSON.toPullRequest(op)
	if err != nil {
		return BatchPRResult{Found: false}
	}
	if pr.Number != requested {
		return BatchPRResult{Found: false}
	}
	result := BatchPRResult{Found: true, PR: pr}
	if pr.State == StateMerged {
		mergedAt, mergeCommit, ok := alias.mergeFacts()
		if !ok {
			return BatchPRResult{Found: false}
		}
		result.MergedAtUTC = mergedAt
		result.MergeCommit = mergeCommit
	}
	return result
}

// mergeFacts returns the merge timestamp and full merge-commit oid of a merged
// PR. A missing timestamp, a missing mergeCommit object, or an invalid oid makes
// the merged snapshot untrustworthy (ok=false), so the slot is UNKNOWN rather
// than a merged verdict with no provenance.
func (a batchPRAlias) mergeFacts() (mergedAt, mergeCommit string, ok bool) {
	if a.MergedAt == nil || *a.MergedAt == "" {
		return "", "", false
	}
	if a.MergeCommit == nil {
		return "", "", false
	}
	if err := validateFullObjectID(a.MergeCommit.OID); err != nil {
		return "", "", false
	}
	return *a.MergedAt, a.MergeCommit.OID, true
}

// batchErrorSummary renders the first GraphQL error's message for a bounded
// diagnostic (newFailure redacts it). The count is included so a multi-error
// response is not silently reported as one.
func batchErrorSummary(errs []batchGraphQLError) string {
	first := errs[0].Message
	if len(errs) > 1 {
		return fmt.Sprintf("%s (and %d more)", first, len(errs)-1)
	}
	return first
}

// isJSONNull reports whether a raw message is the JSON literal null, tolerating
// surrounding whitespace.
func isJSONNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
}
