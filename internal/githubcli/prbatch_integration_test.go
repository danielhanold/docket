//go:build integration

package githubcli

import (
	"context"
	"testing"
)

// The aliased-GraphQL batch read drives a real fake-gh subprocess so the
// transport-boundary claims — ONE process per batch, a failed batch isolated
// from its neighbours — are proven by counting actual invocations, never by a
// batch-method fake. The pure query-construction and decode units stay in the
// untagged prbatch_test.go.

// batchArm answers any `gh api graphql` invocation with the given envelope and
// exit. Batch reads are all one argv shape (api graphql -f query=…), so the
// prefix discriminates the batch call from every other gh verb.
func batchArm(envelope []byte, exit int, stderr string) fakeArm {
	return fakeArm{ArgvPrefix: []string{"api", "graphql"}, Stdout: string(envelope), Exit: exit, Stderr: stderr}
}

// TestIntegrationBatchOneProcessForTwentyFiveNumbers: a 25-number batch is
// exactly ONE gh process — the whole point of the aliased read.
func TestIntegrationBatchOneProcessForTwentyFiveNumbers(t *testing.T) {
	numbers := make([]int, 25)
	aliases := map[string]any{}
	for i := range numbers {
		numbers[i] = i + 1
		aliases[batchAliasName(i)] = batchAliasObj(i+1, "OPEN", nil, "", "")
	}
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		batchArm(batchEnvelopeBytes(t, aliases, nil), 0, ""),
	}})

	got, err := c.ViewPullRequestsBatch(context.Background(), probeRepo(), numbers)
	if err != nil {
		t.Fatalf("ViewPullRequestsBatch: %v", err)
	}
	if len(got) != 25 {
		t.Fatalf("result count = %d, want 25", len(got))
	}
	recs := log.records(t)
	if n := countArgv(recs, "api", "graphql"); n != 1 {
		t.Errorf("api graphql invocations = %d, want exactly 1 for a 25-number batch", n)
	}
	if len(recs) != 1 {
		t.Errorf("total gh invocations = %d, want 1", len(recs))
	}
}

// TestIntegrationBatchFailedBatchBetweenSuccessesIsolated: three sequential
// batches where the middle one exits non-zero — it errors, and the batches on
// either side stay usable. A failed batch is never shrunk into per-record
// retries.
func TestIntegrationBatchFailedBatchBetweenSuccessesIsolated(t *testing.T) {
	okEnv := batchEnvelopeBytes(t, map[string]any{"pr0": batchAliasObj(1, "OPEN", nil, "", "")}, nil)
	c, _ := newFakeClient(t, fakeScenario{
		Sequential: true,
		Invocations: []fakeArm{
			batchArm(okEnv, 0, ""),
			batchArm(nil, 1, "server exploded"),
			batchArm(okEnv, 0, ""),
		},
	})

	first, err := c.ViewPullRequestsBatch(context.Background(), probeRepo(), []int{1})
	if err != nil {
		t.Fatalf("first batch: %v", err)
	}
	if !first[1].Found {
		t.Errorf("first batch pr 1 not found: %+v", first[1])
	}

	_, err = c.ViewPullRequestsBatch(context.Background(), probeRepo(), []int{1})
	if err == nil {
		t.Fatalf("middle batch (exit 1) returned no error")
	}
	if f, ok := AsFailure(err); !ok || f.Kind != KindExternal {
		t.Errorf("middle batch failure = %v, want external-kind *Failure", err)
	}

	third, err := c.ViewPullRequestsBatch(context.Background(), probeRepo(), []int{1})
	if err != nil {
		t.Fatalf("third batch: %v", err)
	}
	if !third[1].Found {
		t.Errorf("third batch pr 1 not found: %+v", third[1])
	}
}

// TestIntegrationBatchDeletedHeadRefMergedPR: a merged PR whose head branch was
// deleted still decodes to a merged result carrying its merge facts — GitHub
// retains headRefName/headRefOid, so the snapshot is complete.
func TestIntegrationBatchDeletedHeadRefMergedPR(t *testing.T) {
	mergeOID := "4444444444444444444444444444444444444444"
	alias := batchAliasObj(88, "MERGED", nil, "2026-08-31T09:30:00Z", mergeOID)
	alias["headRefName"] = "feature/deleted-after-merge"
	env := batchEnvelopeBytes(t, map[string]any{"pr0": alias}, nil)
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{batchArm(env, 0, "")}})

	got, err := c.ViewPullRequestsBatch(context.Background(), probeRepo(), []int{88})
	if err != nil {
		t.Fatalf("ViewPullRequestsBatch: %v", err)
	}
	r := got[88]
	if !r.Found || r.PR.State != StateMerged {
		t.Fatalf("merged deleted-head PR = %+v", r)
	}
	if r.MergeCommit != mergeOID || r.MergedAtUTC != "2026-08-31T09:30:00Z" {
		t.Errorf("merge facts = {%q, %q}, want {%q, %q}", r.MergedAtUTC, r.MergeCommit, "2026-08-31T09:30:00Z", mergeOID)
	}
	if r.PR.HeadBranch != "feature/deleted-after-merge" {
		t.Errorf("HeadBranch = %q, want the retained head ref name", r.PR.HeadBranch)
	}
}

// TestIntegrationBatchPartialResponseWithErrorsFailsWhole: a response carrying
// data for some aliases AND a top-level errors array (HTTP 200) fails the whole
// batch — never a partial trusted map.
func TestIntegrationBatchPartialResponseWithErrorsFailsWhole(t *testing.T) {
	aliases := map[string]any{
		"pr0": batchAliasObj(1, "OPEN", nil, "", ""),
		"pr1": nil,
	}
	env := batchEnvelopeBytes(t, aliases, []any{batchGraphQLErr("Could not resolve pr1")})
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{batchArm(env, 0, "")}})

	got, err := c.ViewPullRequestsBatch(context.Background(), probeRepo(), []int{1, 2})
	if err == nil {
		t.Fatalf("partial-with-errors returned no error (got %+v)", got)
	}
	if got != nil {
		t.Errorf("failed batch returned a non-nil map: %+v", got)
	}
	if f, ok := AsFailure(err); !ok || f.Kind != KindExternal {
		t.Errorf("failure = %v, want external-kind *Failure", err)
	}
}
