package app

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository"
)

// This file proves the maintenance sweep's batched PR selection: deterministic
// ≤25-number batches over one shared GitHub identity, a whole-batch (never
// per-PR) unknown on failure, and byte-parity with the single-view ProbePR
// shapes.

func sweepTestRepo() githubcli.Repository {
	return githubcli.Repository{Host: "github.com", Owner: "acme", Name: "widgets"}
}

// countingSweepGitHub is a fake sweepGitHub that counts the identity resolution
// and each aliased batch, records the numbers every batch received, and answers
// from scripted per-number results. A number in failBatchFor forces the WHOLE
// batch containing it to error — the reader must treat that as unknown for the
// whole chunk, never shrink it to per-PR reads.
type countingSweepGitHub struct {
	repo         githubcli.Repository
	discoverErr  error
	discovers    int
	batchCalls   int
	batches      [][]int
	results      map[int]githubcli.BatchPRResult
	failBatchFor map[int]bool
}

func (g *countingSweepGitHub) DiscoverRepository(context.Context, string) (githubcli.Repository, error) {
	g.discovers++
	if g.discoverErr != nil {
		return githubcli.Repository{}, g.discoverErr
	}
	return g.repo, nil
}

func (g *countingSweepGitHub) ViewPullRequestsBatch(_ context.Context, _ githubcli.Repository, numbers []int) (map[int]githubcli.BatchPRResult, error) {
	g.batchCalls++
	g.batches = append(g.batches, append([]int(nil), numbers...))
	for _, n := range numbers {
		if g.failBatchFor[n] {
			return nil, fmt.Errorf("batch failed containing PR %d", n)
		}
	}
	out := make(map[int]githubcli.BatchPRResult, len(numbers))
	for _, n := range numbers {
		if r, ok := g.results[n]; ok {
			out[n] = r
			continue
		}
		out[n] = githubcli.BatchPRResult{Found: false}
	}
	return out, nil
}

// fakeSweepBatchReader returns a scripted SweepPRSetResult, ignoring its inputs,
// so the app-level selection helper is tested independently of the transport.
type fakeSweepBatchReader struct {
	result SweepPRSetResult
	calls  int
}

func (f *fakeSweepBatchReader) ProbePRSet(context.Context, string, []int) SweepPRSetResult {
	f.calls++
	return f.result
}

// TestSweepPRSetBatchesOf25: the number of aliased batch processes is
// ceil(n/25) — 0→0, 1→1, 25→1, 26→2, 51→3 — and no single batch exceeds the cap.
func TestSweepPRSetBatchesOf25(t *testing.T) {
	for _, tc := range []struct{ n, wantCalls int }{{0, 0}, {1, 1}, {25, 1}, {26, 2}, {51, 3}} {
		gh := &countingSweepGitHub{repo: sweepTestRepo()}
		r := &sweepPRBatchReader{gh: gh}
		nums := make([]int, 0, tc.n)
		for i := 1; i <= tc.n; i++ {
			nums = append(nums, i)
		}
		r.ProbePRSet(context.Background(), "repo", nums)
		if gh.batchCalls != tc.wantCalls {
			t.Errorf("n=%d: batch processes = %d, want %d", tc.n, gh.batchCalls, tc.wantCalls)
		}
		for _, b := range gh.batches {
			if len(b) > sweepPRBatchCap {
				t.Errorf("n=%d: a batch carried %d numbers, exceeds cap %d", tc.n, len(b), sweepPRBatchCap)
			}
		}
	}
}

// TestSweepPRSetDedupesAndSorts: duplicate refs share one response slot and the
// batch receives the numbers deduped and sorted ascending.
func TestSweepPRSetDedupesAndSorts(t *testing.T) {
	gh := &countingSweepGitHub{repo: sweepTestRepo()}
	r := &sweepPRBatchReader{gh: gh}
	r.ProbePRSet(context.Background(), "repo", []int{5, 1, 5, 3, 1, 3})
	if len(gh.batches) != 1 {
		t.Fatalf("want a single batch, got %d", len(gh.batches))
	}
	got := gh.batches[0]
	want := []int{1, 3, 5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("batch numbers = %v, want deduped+sorted %v", got, want)
	}
}

// TestSweepPRSetFailedBatchIsUnknownPlusFinding: a middle batch that fails leaves
// the other batches usable, reports its whole chunk as one Failure (never shrunk
// to per-PR reads), and the app helper maps the failed numbers to zero-value
// (unknown) PRFacts plus exactly one finding per failed batch.
func TestSweepPRSetFailedBatchIsUnknownPlusFinding(t *testing.T) {
	// --- reader: middle chunk fails, others usable, no per-PR fallback ---
	results := map[int]githubcli.BatchPRResult{}
	for i := 1; i <= 60; i++ {
		results[i] = githubcli.BatchPRResult{Found: true, PR: githubcli.PullRequest{
			Number: i, State: githubcli.StateOpen, HeadBranch: "feat/x", HeadCommit: "h", BaseBranch: "main", Version: "v",
		}}
	}
	gh := &countingSweepGitHub{repo: sweepTestRepo(), results: results, failBatchFor: map[int]bool{30: true}}
	r := &sweepPRBatchReader{gh: gh}
	nums := make([]int, 0, 60)
	for i := 1; i <= 60; i++ {
		nums = append(nums, i)
	}
	res := r.ProbePRSet(context.Background(), "repo", nums)

	if gh.discovers != 1 {
		t.Errorf("identity resolved %d times, want 1", gh.discovers)
	}
	if gh.batchCalls != 3 {
		t.Errorf("batch processes = %d, want 3 (25/25/10); a failed batch must NOT fall back to per-PR reads", gh.batchCalls)
	}
	// chunk1 (1..25) and chunk3 (51..60) are usable; chunk2 (26..50) is unknown.
	if _, ok := res.Facts[1]; !ok {
		t.Errorf("first-chunk PR 1 must be resolved")
	}
	if _, ok := res.Facts[55]; !ok {
		t.Errorf("third-chunk PR 55 must be resolved")
	}
	if _, ok := res.Facts[30]; ok {
		t.Errorf("failed-chunk PR 30 must be UNKNOWN (absent from Facts), never resolved")
	}
	if len(res.Failures) != 1 {
		t.Fatalf("want exactly one Failure for the failed chunk, got %d", len(res.Failures))
	}
	if len(res.Failures[0].Numbers) != 25 {
		t.Errorf("failed batch must cover its whole 25-number chunk, got %d", len(res.Failures[0].Numbers))
	}

	// --- helper: failed numbers become zero-value facts + one finding ---
	snap := sweepTestSnapshot(t, []StatusBlob{
		finalizeBlob(70, "resolved", "implemented", "high", prRefFor(70), ""),
		finalizeBlob(71, "unresolved", "implemented", "high", prRefFor(71), ""),
	})
	resolved := domain.PRFacts{Number: "70", Version: "v70", State: "open", HeadBranch: "feat/resolved", HeadOID: "h70", BaseRef: "main"}
	fake := &fakeSweepBatchReader{result: SweepPRSetResult{
		Facts:    map[int]domain.PRFacts{70: resolved},
		Failures: []SweepPRBatchFailure{{Numbers: []int{71}, Message: "gh graphql failed"}},
	}}
	facts, findings := sweepSelectPRFacts(context.Background(), fake, "repo", snap)
	if !reflect.DeepEqual(facts[70], resolved) {
		t.Errorf("change 70 facts = %+v, want resolved %+v", facts[70], resolved)
	}
	if facts[71] != (domain.PRFacts{}) {
		t.Errorf("change 71 (failed batch) facts = %+v, want zero-value unknown", facts[71])
	}
	if len(findings) != 1 {
		t.Fatalf("want exactly one finding for the failed batch, got %d: %+v", len(findings), findings)
	}
	if findings[0].Severity != string(domain.SeverityWarning) {
		t.Errorf("finding severity = %q, want warning", findings[0].Severity)
	}
}

// TestSweepPRSetIdentityResolvedOnce: the repository identity is resolved exactly
// once for a multi-batch invocation, and a failed resolution fails once for the
// whole invocation (one Failure over all numbers) with no per-consumer retry and
// no batch process attempted.
func TestSweepPRSetIdentityResolvedOnce(t *testing.T) {
	nums := make([]int, 0, 30)
	for i := 1; i <= 30; i++ { // two chunks
		nums = append(nums, i)
	}

	t.Run("resolved once across batches", func(t *testing.T) {
		gh := &countingSweepGitHub{repo: sweepTestRepo()}
		r := &sweepPRBatchReader{gh: gh}
		r.ProbePRSet(context.Background(), "repo", nums)
		if gh.discovers != 1 {
			t.Errorf("identity resolved %d times across 2 batches, want exactly 1", gh.discovers)
		}
	})

	t.Run("failed resolution fails once for the whole invocation", func(t *testing.T) {
		gh := &countingSweepGitHub{repo: sweepTestRepo(), discoverErr: fmt.Errorf("no gh auth")}
		r := &sweepPRBatchReader{gh: gh}
		res := r.ProbePRSet(context.Background(), "repo", nums)
		if gh.discovers != 1 {
			t.Errorf("failed identity resolved %d times, want exactly 1 (no per-consumer retry)", gh.discovers)
		}
		if gh.batchCalls != 0 {
			t.Errorf("a failed resolution must attempt no batch process, got %d", gh.batchCalls)
		}
		if len(res.Failures) != 1 || len(res.Failures[0].Numbers) != len(nums) {
			t.Fatalf("want one Failure covering all %d numbers, got %+v", len(nums), res.Failures)
		}
		if len(res.Facts) != 0 {
			t.Errorf("a failed resolution resolves no facts, got %+v", res.Facts)
		}
	})
}

// TestSweepPRSetFactsParity: the batched slot maps to the same domain.PRFacts a
// single ProbePR of the same snapshot produces — for merged, open, closed, draft,
// and approved shapes, including the conservative empty Mergeable/diff for an
// open PR.
func TestSweepPRSetFactsParity(t *testing.T) {
	cases := []struct {
		name    string
		pr      githubcli.PullRequest
		merged  *closeoutProbe // set for the merged case
		batchMC string         // MergeCommit for the batch merged slot
		batchAt string         // MergedAtUTC for the batch merged slot
	}{
		{
			name: "open approved",
			pr:   githubcli.PullRequest{Number: 7, State: githubcli.StateOpen, Approved: true, HeadBranch: "feat/x", HeadCommit: "h7", BaseBranch: "main", Version: "v7"},
		},
		{
			name: "open draft unapproved",
			pr:   githubcli.PullRequest{Number: 8, State: githubcli.StateOpen, Draft: true, HeadBranch: "feat/y", HeadCommit: "h8", BaseBranch: "main", Version: "v8"},
		},
		{
			name: "closed",
			pr:   githubcli.PullRequest{Number: 9, State: githubcli.StateClosed, HeadBranch: "feat/z", HeadCommit: "h9", BaseBranch: "main", Version: "v9"},
		},
		{
			name:    "merged",
			pr:      githubcli.PullRequest{Number: 10, State: githubcli.StateMerged, HeadBranch: "feat/m", HeadCommit: "h10", BaseBranch: "main", Version: "v10"},
			merged:  &closeoutProbe{outcome: githubcli.MergeAlreadyMerged, facts: githubcli.MergedFacts{HeadBranch: "feat/m", HeadOID: "h10", BaseRef: "main", MergedAtUTC: "2026-08-10T00:00:00Z", MergeCommit: "mc10", Version: "v10"}},
			batchMC: "mc10",
			batchAt: "2026-08-10T00:00:00Z",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			num := tc.pr.Number
			// Single-view path (the parity oracle): the production prober.
			ghProber := &fakeProberGitHub{repo: sweepTestRepo(), views: map[int]githubcli.PullRequest{num: tc.pr}}
			if tc.merged != nil {
				ghProber.merged = map[int]closeoutProbe{num: *tc.merged}
			}
			want, err := NewGitHubFinalizeProber(ghProber).ProbePR(context.Background(), "repo", prRefFor(num))
			if err != nil {
				t.Fatalf("oracle ProbePR: %v", err)
			}
			// Batched path.
			br := githubcli.BatchPRResult{Found: true, PR: tc.pr, MergeCommit: tc.batchMC, MergedAtUTC: tc.batchAt}
			got := sweepBatchResultToFacts(num, br)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("batched facts diverge from ProbePR:\n got=%+v\nwant=%+v", got, want)
			}
		})
	}
}

// sweepTestSnapshot builds a snapshot from change blobs for the selection helper.
func sweepTestSnapshot(t *testing.T, blobs []StatusBlob) domain.Snapshot {
	t.Helper()
	pin := docketPin(t)
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: pin.Config.Effective, Documents: inputs})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	return build.Snapshot
}
