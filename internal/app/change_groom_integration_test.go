//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_recordops.sh (prefix ^TestIntegrationRecordOps).

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// TestIntegrationRecordOpsChangeGroomReviseSpecVersionContendsRealGit drives the finding's exact
// race through a real engine and a bare origin: two revises pinned to the SAME
// record version (a same-day spec-only revise leaves the record bytes
// unchanged) and the same spec version. The first applies; the second's spec
// pin is stale, so it contends and writes nothing instead of clobbering the
// first revise's spec body.
func TestIntegrationRecordOpsChangeGroomReviseSpecVersionContendsRealGit(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(2, "add-a-widget")
	repo := newWorkingRepo(t, reviseFixtureFiles())
	node := planningDepsFor(t, repo.invocation)

	revise := func(body, recV, specV string) ChangeGroomResult {
		req := validReviseRequest()
		req.Sections = nil
		req.SpecMarkdown = "# Design\n\n" + body + "\n"
		req.Revision, req.SpecRevision = recV, specV
		return ChangeGroom(context.Background(), node.deps, node.dir, req)
	}

	// Settle the record (updated: today, artifacts rendered) with a matching
	// pin — the matching-version apply path.
	if res := revise("Settling body.", blobVersionAt(t, repo.origin, "docket", recPath),
		blobVersionAt(t, repo.origin, "docket", reviseSpecPath)); res.Result != ResultApplied {
		t.Fatalf("settling revise = %q (findings %v), want applied", res.Result, res.Findings)
	}
	recV := blobVersionAt(t, repo.origin, "docket", recPath)
	specV := blobVersionAt(t, repo.origin, "docket", reviseSpecPath)

	if res := revise("Body A.", recV, specV); res.Result != ResultApplied {
		t.Fatalf("revise A = %q (findings %v), want applied", res.Result, res.Findings)
	}
	if got := blobVersionAt(t, repo.origin, "docket", recPath); got != recV {
		t.Fatalf("precondition: a same-day spec-only revise must leave the record version unchanged (%s -> %s)", recV, got)
	}
	tip := originTip(t, repo.origin, "docket")

	res := revise("Body B.", recV, specV) // stale spec pin, current record pin
	if res.Result != ResultContended {
		t.Fatalf("revise B over a stale spec_version = %q (findings %v), want contended", res.Result, res.Findings)
	}
	if !hasFindingCode(res.Findings, "spec-version-mismatch") {
		t.Errorf("missing finding spec-version-mismatch; got %v", res.Findings)
	}
	if got := originTip(t, repo.origin, "docket"); got != tip {
		t.Errorf("a contended revise moved the metadata branch %s -> %s", tip, got)
	}
	spec, _ := originFile(t, repo.origin, "docket", reviseSpecPath)
	if !strings.Contains(spec, "Body A.") || strings.Contains(spec, "Body B.") {
		t.Errorf("revise A's spec body was clobbered:\n%s", spec)
	}
}

// TestIntegrationRecordOpsChangeGroomAbstainThenRearmRealGit drives both outcomes through the real
// engine and a bare origin: the abstain lands the record and BOARD.md in ONE
// commit; a re-arm pinned to the pre-abstain version contends and writes
// nothing; a re-arm at the current version restores needs-brainstorm.
func TestIntegrationRecordOpsChangeGroomAbstainThenRearmRealGit(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(2, "add-a-widget")
	repo := newWorkingRepo(t, map[string]string{recPath: groomableChange(2, "add-a-widget")})
	node := planningDepsFor(t, repo.invocation)

	ab := abstainRequest()
	ab.Revision = blobVersionAt(t, repo.origin, "docket", recPath)
	if res := ChangeGroom(context.Background(), node.deps, node.dir, ab); res.Result != ResultApplied {
		t.Fatalf("abstain = %q (findings %v), want applied", res.Result, res.Findings)
	}
	tip := originTip(t, repo.origin, "docket")
	paths := originCommitPaths(t, repo.origin, tip)
	if !slices.Contains(paths, recPath) || !slices.Contains(paths, "docs/changes/BOARD.md") {
		t.Fatalf("abstain commit paths = %v, want the record and BOARD.md in one commit", paths)
	}
	if board, _ := originFile(t, repo.origin, "docket", "docs/changes/BOARD.md"); !strings.Contains(board, "auto-groom blocked — needs you") {
		t.Errorf("committed board does not show the abstain:\n%s", board)
	}

	stale := rearmRequest()
	stale.Revision = ab.Revision // pre-abstain pin
	if res := ChangeGroom(context.Background(), node.deps, node.dir, stale); res.Result != ResultContended {
		t.Fatalf("stale re-arm = %q (findings %v), want contended", res.Result, res.Findings)
	}
	if got := originTip(t, repo.origin, "docket"); got != tip {
		t.Fatalf("a contended re-arm moved the metadata branch %s -> %s", tip, got)
	}

	fresh := rearmRequest()
	fresh.Revision = blobVersionAt(t, repo.origin, "docket", recPath)
	if res := ChangeGroom(context.Background(), node.deps, node.dir, fresh); res.Result != ResultApplied {
		t.Fatalf("re-arm = %q (findings %v), want applied", res.Result, res.Findings)
	}
	rec, _ := originFile(t, repo.origin, "docket", recPath)
	board, _ := originFile(t, repo.origin, "docket", "docs/changes/BOARD.md")
	if strings.Contains(rec, "## Auto-groom blocked") || !strings.Contains(rec, "\nauto_groomable: true\n") {
		t.Errorf("re-armed record:\n%s", rec)
	}
	if strings.Contains(board, "auto-groom blocked — needs you") {
		t.Errorf("committed board still shows the abstain after re-arm:\n%s", board)
	}
}
