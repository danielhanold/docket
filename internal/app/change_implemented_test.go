package app

import (
	"context"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"github.com/danielhanold/docket/internal/workspace"
	"strings"
	"testing"
)

// miVersion is the exact entity version the happy fixtures pin.
const miVersion = "1234123412341234123412341234123412341234"

// miRecord renders an in-progress change record with the given plan/results
// linkage and reconciled flag — the shape mark-implemented reprobes.
func miRecord(id int, slug, plan, results string, reconciled bool) string {
	src := lifecycleChange(id, slug, "in-progress")
	if !reconciled {
		src = strings.Replace(src, "reconciled: true", "reconciled: false", 1)
	}
	if plan != "" {
		src = strings.Replace(src, "plan:\n", "plan: '"+plan+"'\n", 1)
	}
	if results != "" {
		src = strings.Replace(src, "results:\n", "results: '"+results+"'\n", 1)
	}
	return src
}

// baseImplementedOp builds a mark-implemented transaction op for the plan-closure
// tests, wiring the inline board when the surfaces request it.
func baseImplementedOp(surfaces []string, id int, pr string) changeImplementedOp {
	return changeImplementedOp{
		changeID:   id,
		pr:         pr,
		eff:        planningTestConfig(surfaces),
		clock:      testClock(),
		inline:     len(surfaces) > 0 && surfaces[0] == "inline",
		link:       render.LinkContext{MetadataBranch: "main"},
		changesDir: "docs/changes",
	}
}

// implementedPlanFor runs the mark-implemented op's Plan closure over a fake tree
// (mirrors claimPlanFor) so a test can inspect the patched record bytes directly.
func implementedPlanFor(t *testing.T, files map[string]string, op changeImplementedOp) (transaction.MutationPlan, transaction.OperationResult) {
	t.Helper()
	tree := newFakeTree(files)
	loader := newPlanningLoader(op.eff)
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

// TestMarkImplementedApplies (plan closure): the transaction patches status, pr,
// and updated, re-renders the artifact block + board, and leaves the claim fields
// (claimed_at, branch) byte-identical — the transition never releases the claim.
func TestMarkImplementedApplies(t *testing.T) {
	recPath := groomPath(3, "widget")
	planPath := "docs/superpowers/plans/2026-08-17-widget-plan.md"
	files := map[string]string{
		recPath:                 miRecord(3, "widget", planPath, "", true),
		"docs/changes/BOARD.md": "# Backlog\n\nold\n",
	}
	plan, opRes := implementedPlanFor(t, files, baseImplementedOp([]string{"inline"}, 3, "github.com/acme/widget#42"))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		recPath:                 transaction.MutationReplace,
		"docs/changes/BOARD.md": transaction.MutationReplace,
	})

	rec := lifecycleRecordBytes(t, plan, recPath)
	for _, want := range []string{
		"status: 'implemented'",
		"pr: 'github.com/acme/widget#42'",
		"updated: '2026-08-16'",
		"branch: feat/widget",              // claim field untouched
		"claimed_at: 2026-08-02T00:00:00Z", // claim field untouched
		"docket:artifacts:start",
	} {
		if !strings.Contains(rec, want) {
			t.Errorf("implemented record missing %q:\n%s", want, rec)
		}
	}
	// The updated date moved; the pre-transition date must be gone.
	if strings.Contains(rec, "updated: 2026-08-02") {
		t.Errorf("updated date not refreshed:\n%s", rec)
	}
}

// --- reprobe fixture --------------------------------------------------------

// miKit is the happy configuration of every reprobe input; each conjunct row
// overrides exactly one field and asserts the operation refuses with that
// conjunct's stable reason, having never called the engine.
type miKit struct {
	reconciled bool
	plan       string
	results    string
	version    string // corpus blob version
	reqVersion string
	reqHead    string
	localHead  string
	evidence   []byte
	probePRs   []githubcli.PullRequest
	probeErr   error
	reqPR      string
}

const miSlug = "widget"

func miPlanPath() string { return "docs/superpowers/plans/2026-08-17-widget-plan.md" }

// buildMI assembles the deps/req from a kit over the shared real repo.
func buildMI(t *testing.T, client *gitcli.Client, invocation string, k miKit) (
	PlanningDeps, WorkspaceDeps, GitHubDeps, string, MarkImplementedRequest, *recordingEngine) {
	t.Helper()
	blob := StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(3, miSlug),
		Version:  k.version,
		Data:     []byte(miRecord(3, miSlug, k.plan, k.results, k.reconciled)),
	}
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{blob}, facts: domain.NewBranchFacts(nil)}
	engine := &recordingEngine{result: transaction.Result{
		Disposition:   transaction.DispositionApplied,
		AppliedCommit: "cafebabecafebabecafebabecafebabecafebabe",
		Receipt:       mustMarshal(t, changeLifecycleReceipt{ID: 3, Op: OperationChangeMarkImplemented, Status: "implemented"}),
	}}
	deps := PlanningDeps{Client: client, Engine: engine, Reader: reader, Clock: testClock()}
	wdeps := WorkspaceDeps{Service: &fakeWorkspaceService{
		inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(k.localHead)},
	}}
	gdeps := GitHubDeps{Service: &fakeGitHub{repo: prRepo(), probePRs: k.probePRs, probeErr: k.probeErr}}
	req := MarkImplementedRequest{ID: 3, Version: k.reqVersion, Head: k.reqHead, PR: k.reqPR, EvidenceRecord: k.evidence}
	return deps, wdeps, gdeps, invocation, req, engine
}

// miPRURL is the verified PR's canonical full-URL form — the board-safe value
// the transition records into the manifest pr: (change 0344). Its host/owner/name
// mirror prRepo(); parsePRRef reads the number after "/pull/".
func miPRURL() string { return "https://github.com/acme/widget/pull/42" }

// happyPR is the single open PR that satisfies conjunct 4 for the given head. It
// carries the canonical URL the adapter always decodes for a real PR, which the
// transition records as the manifest pr:.
func happyPR(head string) githubcli.PullRequest {
	return githubcli.PullRequest{
		Number: 42, State: githubcli.StateOpen, HeadBranch: "feat/" + miSlug,
		HeadCommit: head, BaseBranch: "main", URL: miPRURL(),
	}
}

// firstStatusFindingCode returns the code of the first finding, or "".
func firstStatusFindingCode(findings []StatusFinding) string {
	if len(findings) > 0 {
		return findings[0].Code
	}
	return ""
}
