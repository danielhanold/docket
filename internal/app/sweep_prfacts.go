package app

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/githubcli"
)

// This file owns the maintenance sweep's batched PR selection: one shared GitHub
// identity and exact-number reads in deterministic ≤25 batches, replacing the
// per-change probe the sweep used to run over the whole finalize population. The
// sweep pins one inventory, so it can read every population change's live PR in a
// handful of aliased GraphQL calls instead of one process per change.
//
// A batch that fails — including the single identity resolution, which fails once
// for the whole invocation — reports its numbers as UNKNOWN (never a clean
// absence: learning probe-error-is-not-clean-absence) and surfaces exactly one
// finding for the batch, so the omission is visible rather than silent.

// SweepPRBatchReader reads exact-number PR facts in deterministic batches for
// the maintenance sweep. A failed batch reports its numbers in Failures and
// omits them from Facts — unknown, never absent/closed.
type SweepPRBatchReader interface {
	ProbePRSet(ctx context.Context, repoDir string, numbers []int) SweepPRSetResult
}

// SweepPRSetResult is the outcome of one batched read: the resolved facts keyed
// by PR number, and one Failures entry per batch (or the single identity
// resolution) that could not be read. A number that is neither in Facts nor
// covered by a Failure was resolved to a trustworthy "no such PR" (Found=false).
type SweepPRSetResult struct {
	Facts    map[int]domain.PRFacts
	Failures []SweepPRBatchFailure // one per failed batch (incl. identity resolution: one failure covering all numbers)
}

// SweepPRBatchFailure names the numbers a single failed batch (or a failed
// identity resolution covering all numbers) could not resolve, with the raw
// failure message for the finding.
type SweepPRBatchFailure struct {
	Numbers []int
	Message string
}

// sweepGitHub is the narrow GitHub seam the production batch reader composes: one
// repository resolution and the aliased exact-number batch read. *githubcli.Client
// satisfies it; unit tests inject a counting fake so the transport shape — one
// identity resolution, one process per ≤25-number batch, and no per-PR fallback —
// is proved without a live gh.
type sweepGitHub interface {
	DiscoverRepository(ctx context.Context, dir string) (githubcli.Repository, error)
	ViewPullRequestsBatch(ctx context.Context, repo githubcli.Repository, numbers []int) (map[int]githubcli.BatchPRResult, error)
}

// sweepPRBatchCap mirrors githubcli.ViewPullRequestsBatch's own conservative
// request ceiling: the reader chunks its (deduped, sorted) numbers into slices no
// larger than this so no single query exceeds the adapter's limit.
const sweepPRBatchCap = 25

// sweepPRBatchReader is the production SweepPRBatchReader. It resolves the
// repository once per invocation (memoizing a resolution failure as one Failure
// covering every number — no per-consumer retry), then reads the deduped, sorted
// numbers in ≤25-number batches, one ViewPullRequestsBatch process per batch.
type sweepPRBatchReader struct {
	gh sweepGitHub
}

// NewSweepPRBatchReader builds the production batched PR reader over a GitHub
// client. The CLI wires it into FinalizeDeps.PRBatch.
func NewSweepPRBatchReader(gh *githubcli.Client) SweepPRBatchReader {
	return &sweepPRBatchReader{gh: gh}
}

// ProbePRSet resolves the repository once, then reads numbers in deterministic
// ≤25-number batches. An empty request runs no process. A resolution failure is
// one Failure covering every requested number (never a per-consumer retry); a
// per-batch failure is one Failure covering that batch's numbers, and the batch
// is never shrunk to per-PR reads. Each Found result maps to domain.PRFacts;
// a Found=false slot is unknown and omitted (never a fabricated absence).
func (r *sweepPRBatchReader) ProbePRSet(ctx context.Context, repoDir string, numbers []int) SweepPRSetResult {
	sorted := sweepDedupeSortAsc(numbers)
	facts := make(map[int]domain.PRFacts)
	if len(sorted) == 0 {
		return SweepPRSetResult{Facts: facts}
	}
	// One shared identity for the whole invocation. A resolution failure is
	// memoized as a single Failure over every number — no consumer re-resolves.
	repo, err := r.gh.DiscoverRepository(ctx, repoDir)
	if err != nil {
		return SweepPRSetResult{Facts: facts, Failures: []SweepPRBatchFailure{{Numbers: sorted, Message: err.Error()}}}
	}
	var failures []SweepPRBatchFailure
	for _, chunk := range sweepChunkInts(sorted, sweepPRBatchCap) {
		res, err := r.gh.ViewPullRequestsBatch(ctx, repo, chunk)
		if err != nil {
			// A failed batch is UNKNOWN for its whole chunk — never shrunk to
			// per-PR reads (learning probe-error-is-not-clean-absence).
			failures = append(failures, SweepPRBatchFailure{Numbers: chunk, Message: err.Error()})
			continue
		}
		for n, br := range res {
			if !br.Found {
				continue // unknown: omitted, never a fabricated absence
			}
			facts[n] = sweepBatchResultToFacts(n, br)
		}
	}
	return SweepPRSetResult{Facts: facts, Failures: failures}
}

// sweepBatchResultToFacts maps one resolved batch slot to domain.PRFacts with the
// exact field coverage githubFinalizeProber.ProbePR writes for the same snapshot,
// so a batched read and a single view are byte-identical. The merged path is
// fully faithful. For an open/closed PR the probe carries no mergeability or diff
// size, so those fields (Mergeable, ChangedFiles, DiffLines) stay zero — a
// conservative reading (an open PR bands as UNKNOWN mergeability) that never
// over-permits.
func sweepBatchResultToFacts(number int, br githubcli.BatchPRResult) domain.PRFacts {
	if br.PR.State == githubcli.StateMerged {
		return domain.PRFacts{
			Number:      strconv.Itoa(number),
			Version:     br.PR.Version,
			State:       "merged",
			HeadBranch:  br.PR.HeadBranch,
			HeadOID:     br.PR.HeadCommit,
			BaseRef:     br.PR.BaseBranch,
			MergedAtUTC: br.MergedAtUTC,
			MergeCommit: br.MergeCommit,
		}
	}
	return domain.PRFacts{
		Number:     strconv.Itoa(number),
		Version:    br.PR.Version,
		State:      string(br.PR.State),
		Draft:      br.PR.Draft,
		Approved:   br.PR.Approved,
		HeadBranch: br.PR.HeadBranch,
		HeadOID:    br.PR.HeadCommit,
		BaseRef:    br.PR.BaseBranch,
	}
}

// sweepDedupeSortAsc returns the distinct positive numbers in ascending order.
// It drops non-positive numbers (they cannot name a pull request) so the batch
// adapter's positivity precondition is met before any process runs.
func sweepDedupeSortAsc(numbers []int) []int {
	seen := make(map[int]struct{}, len(numbers))
	out := make([]int, 0, len(numbers))
	for _, n := range numbers {
		if n <= 0 {
			continue
		}
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}

// sweepChunkInts splits nums into consecutive slices no longer than size,
// preserving order. size must be positive.
func sweepChunkInts(nums []int, size int) [][]int {
	var out [][]int
	for i := 0; i < len(nums); i += size {
		end := i + size
		if end > len(nums) {
			end = len(nums)
		}
		out = append(out, nums[i:end])
	}
	return out
}

// sweepSelectPRFacts derives the facts map for domain.SelectFinalizeQueue from
// batched exact-number reads. Changes whose numbers fall in a failed batch (or
// carry an unparseable ref) get zero-value PRFacts — unknown, exactly what the
// old per-change probe error produced — plus one StatusFinding per failed batch.
func sweepSelectPRFacts(ctx context.Context, batch SweepPRBatchReader, repoDir string, snap domain.Snapshot) (map[domain.ChangeID]domain.PRFacts, []StatusFinding) {
	// Collect the finalize population's PR numbers, keyed back to each change so
	// the resolved facts can be threaded to the domain selector by change id.
	numberByChange := make(map[domain.ChangeID]int)
	var numbers []int
	var unparseable []domain.ChangeID
	for _, c := range snap.Changes() {
		if !finalizeInPopulation(c) {
			continue
		}
		n, ok := parsePRNumber(c.PR().Value)
		if !ok {
			unparseable = append(unparseable, c.ID())
			continue
		}
		numberByChange[c.ID()] = n
		numbers = append(numbers, n)
	}

	res := batch.ProbePRSet(ctx, repoDir, numbers)

	facts := make(map[domain.ChangeID]domain.PRFacts, len(numberByChange)+len(unparseable))
	for id, n := range numberByChange {
		// A number absent from Facts is unknown — a failed batch, or a trustworthy
		// "no such PR" (Found=false); either way zero-value facts, never a clean
		// absence laundered into a closed/merged verdict.
		facts[id] = res.Facts[n]
	}
	for _, id := range unparseable {
		facts[id] = domain.PRFacts{}
	}

	var findings []StatusFinding
	for _, fail := range res.Failures {
		findings = append(findings, StatusFinding{
			Code:     "sweep-pr-facts-unresolved",
			Severity: string(domain.SeverityWarning),
			Message: fmt.Sprintf("pull-request facts for %d change(s) could not be resolved (PR %v): %s; treated as unknown",
				len(fail.Numbers), fail.Numbers, fail.Message),
		})
	}
	return facts, findings
}
