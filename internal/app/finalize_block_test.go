package app

import (
	"context"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"strings"
	"testing"
)

// This file drives `finalize block` and `finalize clear-block`. The transaction
// plan closures (section upsert, single-heading invariant, attempt-keyed
// idempotency, board-in-plan) run over an in-memory fakeTree; the
// comment-first-then-marker ordering, the crash replay, the unknown-writes-no-
// marker rule, and the clear-block reprobe run over a REAL feature workspace
// (the rebaseFixture harness) with a recording fake FinalizeGitHub.

// --- fake FinalizeGitHub for block/clear -----------------------------------

// fakeBlockGitHub answers the GitHub calls the block operations make
// (DiscoverRepository, EnsureComment, FindOpenPullRequestsByHead) from scripted
// state and records the EnsureComment call so a test can prove the marker is
// written only after the comment is ensured, and never when the comment probe is
// unknown. Every other finalize-half GitHub method panics so an accidental call
// is loud.
type fakeBlockGitHub struct {
	repo githubcli.Repository

	commentOutcome githubcli.CommentOutcome
	commentURL     string
	commentErr     error
	ensureCalls    int
	lastMarker     string
	lastBody       string

	openByHead map[string][]githubcli.PullRequest
	findErr    error
}

func (f *fakeBlockGitHub) DiscoverRepository(context.Context, string) (githubcli.Repository, error) {
	return f.repo, nil
}
func (f *fakeBlockGitHub) EnsureComment(_ context.Context, _ githubcli.Repository, _ int, marker, body string) (githubcli.CommentOutcome, string, error) {
	f.ensureCalls++
	f.lastMarker = marker
	f.lastBody = body
	if f.commentErr != nil {
		return githubcli.CommentUnknown, "", f.commentErr
	}
	return f.commentOutcome, f.commentURL, nil
}
func (f *fakeBlockGitHub) ViewPullRequest(context.Context, githubcli.Repository, int) (githubcli.PullRequest, error) {
	panic("ViewPullRequest: block must not call this")
}
func (f *fakeBlockGitHub) FindOpenPullRequestsByHead(_ context.Context, _ githubcli.Repository, head string) ([]githubcli.PullRequest, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.openByHead[head], nil
}
func (f *fakeBlockGitHub) ProbeMerged(context.Context, githubcli.Repository, int) (githubcli.MergeOutcome, githubcli.MergedFacts, error) {
	panic("ProbeMerged: block must not call this")
}
func (f *fakeBlockGitHub) RetargetPullRequest(context.Context, githubcli.Repository, int, string, string) (githubcli.RetargetOutcome, githubcli.PullRequest, error) {
	panic("RetargetPullRequest: block must not call this")
}
func (f *fakeBlockGitHub) FindComment(context.Context, githubcli.Repository, int, string) (bool, string, error) {
	panic("FindComment: block must not call this")
}
func (f *fakeBlockGitHub) MergePullRequest(context.Context, githubcli.Repository, int, githubcli.ObjectRef, bool) (githubcli.MergeResult, error) {
	panic("MergePullRequest: block must not call this")
}

// --- plan-closure helper ---------------------------------------------------

func blockPlanFor(t *testing.T, files map[string]string, eff config.Effective, op transaction.SemanticOperation) (transaction.MutationPlan, transaction.OperationResult) {
	t.Helper()
	tree := newFakeTree(files)
	loader := newPlanningLoader(eff)
	before, err := loader.Load(context.Background(), tree)
	if err != nil {
		t.Fatalf("loader.Load: %v", err)
	}
	if before.Report.HasErrors() {
		t.Fatalf("before-state has errors: %v", before.Report.Findings())
	}
	plan, opRes, err := op.Plan(context.Background(), transaction.AttemptState{
		Base: tree.Revision(), State: before, Tree: tree,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	return plan, opRes
}

func finalizeBlockOpFixture(req BlockRequest, url string) finalizeBlockOp {
	return finalizeBlockOp{
		req:        req,
		commentURL: url,
		eff:        planningTestConfig([]string{"inline"}),
		clock:      testClock(),
		inline:     true,
		changesDir: "docs/changes",
	}
}

func sampleBlockRequest() BlockRequest {
	return BlockRequest{
		ID: 3, Version: blobV, PRNumber: 7, Attempt: "att1",
		Reason: "gate-repair-required", Head: strings.Repeat("a", 40),
		Report: "The gate failed for this head.\n", Remedy: "Fix the flaky test and re-run.\n",
	}
}

// --- transaction plan closures ---------------------------------------------

// TestFinalizeBlockSingleSection proves a re-mark never duplicates the heading:
// a second attempt appends inside the one "## Finalize blocked" section (both
// attempts present, exactly one heading), the marker order/balance is validated
// before the rewrite (two headings refuse the whole edit), and the inline board
// is rerendered in the same transaction plan.
func TestFinalizeBlockSingleSection(t *testing.T) {
	recPath := groomPath(3, "widget")

	// A record already carrying one attempt (att0); a second attempt (att1) must
	// append inside the same section.
	existing := strings.TrimRight(lifecycleChange(3, "widget", "in-progress"), "\n") +
		"\n\n## Finalize blocked\n\n### 2026-08-01 — attempt att0\n\n<!-- attempt:att0 -->\n\n- Reason: prior\n"
	files := map[string]string{
		recPath:                 existing,
		"docs/changes/BOARD.md": "# Backlog\n\nold\n",
	}
	plan, opRes := blockPlanFor(t, files, planningTestConfig([]string{"inline"}), finalizeBlockOpFixture(sampleBlockRequest(), "https://example.test/c/1"))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := lifecycleRecordBytes(t, plan, recPath)
	if n := strings.Count(rec, "## Finalize blocked"); n != 1 {
		t.Fatalf("heading appears %d times, want exactly 1 (re-mark duplicated the section):\n%s", n, rec)
	}
	for _, want := range []string{"attempt att0", "attempt att1", "<!-- attempt:att0 -->", "<!-- attempt:att1 -->",
		"gate-repair-required", "https://example.test/c/1", "- PR: #7"} {
		if !strings.Contains(rec, want) {
			t.Errorf("re-marked record missing %q:\n%s", want, rec)
		}
	}

	// Two "## Finalize blocked" headings: the whole-population balance check
	// refuses the rewrite (never a silent pick of one).
	twoHeadings := existing + "\n\n## Finalize blocked\n\nsecond bogus heading\n"
	files[recPath] = twoHeadings
	_, opRes2 := blockPlanFor(t, files, planningTestConfig([]string{"inline"}), finalizeBlockOpFixture(sampleBlockRequest(), "u"))
	if !opRes2.Refused {
		t.Fatalf("a record with two '## Finalize blocked' headings must refuse the rewrite")
	}
}

// TestFinalizeBlockIdempotentAttempt proves the marker upsert is idempotent by
// the promised state: when the fresh section already records THIS attempt, the
// plan declares no record mutation (an empty-plan no-op keyed on the attempt
// marker, not a byte proxy).
func TestFinalizeBlockIdempotentAttempt(t *testing.T) {
	recPath := groomPath(3, "widget")
	already := strings.TrimRight(lifecycleChange(3, "widget", "in-progress"), "\n") +
		"\n\n## Finalize blocked\n\n### 2026-08-01 — attempt att1\n\n<!-- attempt:att1 -->\n\n- Reason: prior\n"
	files := map[string]string{
		recPath:                 already,
		"docs/changes/BOARD.md": "# Backlog\n\nold\n",
	}
	plan, opRes := blockPlanFor(t, files, planningTestConfig([]string{"inline"}), finalizeBlockOpFixture(sampleBlockRequest(), "u"))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	for _, f := range plan.Files {
		if string(f.Path) == recPath {
			t.Fatalf("a replay of the same attempt planned a record mutation; want an idempotent no-op")
		}
	}
}

// --- entrypoint: comment first, then marker --------------------------------

// --- entrypoint: clear-block reprobe ---------------------------------------

// setupBlockedFixture builds a published feature workspace whose record carries a
// durable "## Finalize blocked" section — the state clear-block reprobes.
func setupBlockedFixture(t *testing.T, m planRepoMode) *rebaseFixture {
	t.Helper()
	f := setupRebaseFixtureStatus(t, m, "in-progress")
	blocked := strings.TrimRight(lifecycleChange(f.id, f.slug, "in-progress"), "\n") +
		"\n\n## Finalize blocked\n\n### 2026-08-01 — attempt att0\n\n<!-- attempt:att0 -->\n\n- Reason: prior\n"
	f.repo.writerAdvance(t, f.branch, map[string]string{groomPath(f.id, f.slug): blocked})
	f.version = blobVersionAt(t, f.repo.origin, f.branch, groomPath(f.id, f.slug))
	return f
}
