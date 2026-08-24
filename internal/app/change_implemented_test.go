package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"github.com/danielhanold/docket/internal/workspace"
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

// TestMarkImplementedAppliesEndToEnd (real git): every conjunct holds, so the
// operation opens exactly one exact-version transaction and returns applied.
func TestMarkImplementedAppliesEndToEnd(t *testing.T) {
	requireRealGit(t)
	repo := newMainModeRepo(t, nil)
	head := repo.writerAdvance(t, "feat/"+miSlug, map[string]string{"impl.go": "package impl\n"})
	client := newGitClient(t)
	pr := prRepo().Spec() + "#42"

	deps, wdeps, gdeps, inv, req, engine := buildMI(t, client, repo.invocation, miKit{
		reconciled: true, plan: miPlanPath(), version: miVersion, reqVersion: miVersion,
		reqHead: head, localHead: head, evidence: prEvidenceBytes(t, head),
		probePRs: []githubcli.PullRequest{happyPR(head)}, reqPR: pr,
	})

	res := ChangeMarkImplemented(context.Background(), deps, wdeps, gdeps, inv, req)
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied (findings %v)", res.Result, res.Findings)
	}
	if res.Status != "implemented" || res.Revision == "" {
		t.Errorf("applied result malformed: %+v", res)
	}
	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want exactly 1", len(engine.calls))
	}
	exp := engine.calls[0].Expected
	if len(exp) != 1 || string(exp[0].Version.ObjectID) != miVersion {
		t.Errorf("transaction did not pin the exact version: %+v", exp)
	}
	if engine.calls[0].Operation.Key() != transaction.OperationKey(OperationChangeMarkImplemented) {
		t.Errorf("operation key = %q", engine.calls[0].Operation.Key())
	}
}

// TestMarkImplementedConjuncts is the mutation test for the five-conjunct
// reprobe: each row breaks exactly one conjunct and proves the operation refuses
// with that conjunct's stable reason WITHOUT ever calling the engine. The happy
// fixture (proven by TestMarkImplementedAppliesEndToEnd) satisfies all five.
func TestMarkImplementedConjuncts(t *testing.T) {
	requireRealGit(t)
	repo := newMainModeRepo(t, nil)
	head := repo.writerAdvance(t, "feat/"+miSlug, map[string]string{"impl.go": "package impl\n"})
	client := newGitClient(t)
	pr := prRepo().Spec() + "#42"
	other := prOtherHead

	// happy returns the all-pass kit; each row mutates one field.
	happy := func() miKit {
		return miKit{
			reconciled: true, plan: miPlanPath(), version: miVersion, reqVersion: miVersion,
			reqHead: head, localHead: head, evidence: prEvidenceBytes(t, head),
			probePRs: []githubcli.PullRequest{happyPR(head)}, reqPR: pr,
		}
	}

	rows := []struct {
		name   string
		mutate func(k *miKit)
		reason string
	}{
		{ // conjunct 3
			name:   "evidence names another head",
			mutate: func(k *miKit) { k.evidence = prEvidenceBytes(t, other) },
			reason: ReasonImplementedEvidenceUnverified,
		},
		{ // conjunct 1
			name:   "record not reconciled",
			mutate: func(k *miKit) { k.reconciled = false },
			reason: ReasonImplementedNotReconciled,
		},
		{ // conjunct 1
			name:   "record not linked to a plan",
			mutate: func(k *miKit) { k.plan = "" },
			reason: ReasonImplementedPlanUnlinked,
		},
		{ // conjunct 1
			name:   "entity version moved",
			mutate: func(k *miKit) { k.reqVersion = "9999999999999999999999999999999999999999" },
			reason: ReasonImplementedVersionMismatch,
		},
		{ // conjunct 2a
			name: "local head differs from supplied head",
			mutate: func(k *miKit) {
				k.localHead = other
			},
			reason: ReasonImplementedLocalHeadMismatch,
		},
		{ // conjunct 2b
			name: "remote head differs from supplied head",
			mutate: func(k *miKit) {
				k.reqHead = other
				k.localHead = other
				k.evidence = prEvidenceBytes(t, other)
			},
			reason: ReasonImplementedRemoteHeadMismatch,
		},
		{ // conjunct 4
			name:   "no unique open PR for the feature branch",
			mutate: func(k *miKit) { k.probePRs = nil },
			reason: ReasonImplementedPRNotUnique,
		},
		{ // conjunct 4
			name: "open PR is not the supplied reference",
			mutate: func(k *miKit) {
				odd := happyPR(head)
				odd.Number = 99
				k.probePRs = []githubcli.PullRequest{odd}
			},
			reason: ReasonImplementedPRReferenceMismatch,
		},
		{ // conjunct 5
			name:   "attached results path no longer tracked at head",
			mutate: func(k *miKit) { k.results = "docs/changes/results/0003-widget-ghost.md" },
			reason: ReasonImplementedResultsIdentity,
		},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			k := happy()
			row.mutate(&k)
			deps, wdeps, gdeps, inv, req, engine := buildMI(t, client, repo.invocation, k)

			res := ChangeMarkImplemented(context.Background(), deps, wdeps, gdeps, inv, req)
			if res.Result == ResultApplied {
				t.Fatalf("conjunct %q did not refuse (result applied)", row.name)
			}
			if len(engine.calls) != 0 {
				t.Fatalf("engine was called %d times on a refusal; a broken conjunct must never open a transaction", len(engine.calls))
			}
			code := firstStatusFindingCode(res.Findings)
			if code != row.reason {
				t.Fatalf("refusal reason = %q, want %q (findings %v)", code, row.reason, res.Findings)
			}
		})
	}
}

// TestMarkImplementedRetry: a change already implemented whose recorded PR
// reference matches the request replays the prior applied outcome as a no-op —
// no duplicate transition, engine never called.
func TestMarkImplementedRetry(t *testing.T) {
	requireRealGit(t)
	repo := newMainModeRepo(t, nil)
	head := repo.writerAdvance(t, "feat/"+miSlug, map[string]string{"impl.go": "package impl\n"})
	client := newGitClient(t)
	pr := prRepo().Spec() + "#42"

	// An implemented record carrying the matching PR reference.
	src := lifecycleChange(3, miSlug, "in-progress")
	src = strings.Replace(src, "status: in-progress", "status: implemented", 1)
	src = strings.Replace(src, "plan:\n", "plan: '"+miPlanPath()+"'\n", 1)
	src = strings.Replace(src, "blocked_by:\n", "pr: '"+pr+"'\nblocked_by:\n", 1)
	blob := StatusBlob{Kind: repository.KindChange, Location: repository.LocationActive, Path: groomPath(3, miSlug), Version: miVersion, Data: []byte(src)}
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{blob}, facts: domain.NewBranchFacts(nil)}
	engine := &recordingEngine{}
	deps := PlanningDeps{Client: client, Engine: engine, Reader: reader, Clock: testClock()}
	wdeps := WorkspaceDeps{Service: &fakeWorkspaceService{inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(head)}}}
	gdeps := GitHubDeps{Service: &fakeGitHub{repo: prRepo(), probePRs: []githubcli.PullRequest{happyPR(head)}}}

	res := ChangeMarkImplemented(context.Background(), deps, wdeps, gdeps, repo.invocation,
		MarkImplementedRequest{ID: 3, Version: miVersion, Head: head, PR: pr, EvidenceRecord: prEvidenceBytes(t, head)})

	if res.Result != ResultNoOp {
		t.Fatalf("retry result = %q, want no-op (findings %v)", res.Result, res.Findings)
	}
	if res.Status != "implemented" {
		t.Errorf("retry status = %q, want implemented", res.Status)
	}
	if len(engine.calls) != 0 {
		t.Errorf("retry called the engine %d times, want 0 (no duplicate transition)", len(engine.calls))
	}
}

// TestMarkImplementedRecordsURL: the transition records the verified PR's
// canonical URL (pr.URL from the reprobe) as the manifest pr:, NOT the
// owner/repo#N shorthand the caller supplied. This is the board-safe form
// (boardPRCell mangles a shorthand to "#owner/repo#N"); the value is sourced from
// the snapshot, so it is the canonical URL even when --pr arrives as shorthand
// (change 0344).
func TestMarkImplementedRecordsURL(t *testing.T) {
	requireRealGit(t)
	repo := newMainModeRepo(t, nil)
	head := repo.writerAdvance(t, "feat/"+miSlug, map[string]string{"impl.go": "package impl\n"})
	client := newGitClient(t)
	shorthand := prRepo().Spec() + "#42" // the caller may still assert the shorthand

	deps, wdeps, gdeps, inv, req, engine := buildMI(t, client, repo.invocation, miKit{
		reconciled: true, plan: miPlanPath(), version: miVersion, reqVersion: miVersion,
		reqHead: head, localHead: head, evidence: prEvidenceBytes(t, head),
		probePRs: []githubcli.PullRequest{happyPR(head)}, reqPR: shorthand,
	})

	res := ChangeMarkImplemented(context.Background(), deps, wdeps, gdeps, inv, req)
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied (findings %v)", res.Result, res.Findings)
	}
	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want exactly 1", len(engine.calls))
	}
	op, ok := engine.calls[0].Operation.(changeImplementedOp)
	if !ok {
		t.Fatalf("recorded operation is %T, want changeImplementedOp", engine.calls[0].Operation)
	}
	if op.pr != miPRURL() {
		t.Errorf("recorded pr: = %q, want the canonical URL %q (not the supplied shorthand %q)", op.pr, miPRURL(), shorthand)
	}
}

// TestMarkImplementedIdentityForms is the mutation test for the migrated identity
// conjunct (parsePRRef number vs the verified pr.Number): the transition applies
// when the supplied --pr names the verified PR in EITHER accepted form and
// refuses with pr-reference-mismatch when the number differs or the reference is
// unparseable. Number 42 is the verified PR (happyPR).
func TestMarkImplementedIdentityForms(t *testing.T) {
	requireRealGit(t)
	repo := newMainModeRepo(t, nil)
	head := repo.writerAdvance(t, "feat/"+miSlug, map[string]string{"impl.go": "package impl\n"})
	client := newGitClient(t)

	cases := []struct {
		name        string
		reqPR       string
		wantApplied bool
	}{
		{"url form matches", miPRURL(), true},
		{"shorthand form matches", prRepo().Spec() + "#42", true},
		{"url form wrong number", "https://github.com/acme/widget/pull/99", false},
		{"shorthand wrong number", prRepo().Spec() + "#99", false},
		{"unparseable reference", "not-a-pr-ref", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deps, wdeps, gdeps, inv, req, engine := buildMI(t, client, repo.invocation, miKit{
				reconciled: true, plan: miPlanPath(), version: miVersion, reqVersion: miVersion,
				reqHead: head, localHead: head, evidence: prEvidenceBytes(t, head),
				probePRs: []githubcli.PullRequest{happyPR(head)}, reqPR: tc.reqPR,
			})
			res := ChangeMarkImplemented(context.Background(), deps, wdeps, gdeps, inv, req)
			if tc.wantApplied {
				if res.Result != ResultApplied {
					t.Fatalf("reqPR %q: result = %q, want applied (findings %v)", tc.reqPR, res.Result, res.Findings)
				}
				return
			}
			if res.Result == ResultApplied {
				t.Fatalf("reqPR %q: applied, want refusal", tc.reqPR)
			}
			if len(engine.calls) != 0 {
				t.Fatalf("reqPR %q: engine called %d times on a refusal", tc.reqPR, len(engine.calls))
			}
			if code := firstStatusFindingCode(res.Findings); code != ReasonImplementedPRReferenceMismatch {
				t.Fatalf("reqPR %q: reason = %q, want %q", tc.reqPR, code, ReasonImplementedPRReferenceMismatch)
			}
		})
	}
}

// TestMarkImplementedRetryCrossForm: the response-loss replay guard is now by
// parsed number (samePRRef), so an already-implemented change recorded in the
// canonical URL form replays as a no-op when the retry asserts the same PR in the
// shorthand form, and still refuses as contended when the asserted number
// differs. This mutation-tests the migrated guard on the recorded-URL path 0344
// introduces.
func TestMarkImplementedRetryCrossForm(t *testing.T) {
	requireRealGit(t)
	repo := newMainModeRepo(t, nil)
	head := repo.writerAdvance(t, "feat/"+miSlug, map[string]string{"impl.go": "package impl\n"})
	client := newGitClient(t)

	// An implemented record carrying the canonical URL form (what 0344 records).
	src := lifecycleChange(3, miSlug, "in-progress")
	src = strings.Replace(src, "status: in-progress", "status: implemented", 1)
	src = strings.Replace(src, "plan:\n", "plan: '"+miPlanPath()+"'\n", 1)
	src = strings.Replace(src, "blocked_by:\n", "pr: '"+miPRURL()+"'\nblocked_by:\n", 1)

	newDeps := func() (PlanningDeps, WorkspaceDeps, GitHubDeps, *recordingEngine) {
		blob := StatusBlob{Kind: repository.KindChange, Location: repository.LocationActive, Path: groomPath(3, miSlug), Version: miVersion, Data: []byte(src)}
		reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{blob}, facts: domain.NewBranchFacts(nil)}
		engine := &recordingEngine{}
		deps := PlanningDeps{Client: client, Engine: engine, Reader: reader, Clock: testClock()}
		wdeps := WorkspaceDeps{Service: &fakeWorkspaceService{inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(head)}}}
		gdeps := GitHubDeps{Service: &fakeGitHub{repo: prRepo(), probePRs: []githubcli.PullRequest{happyPR(head)}}}
		return deps, wdeps, gdeps, engine
	}

	// Same PR asserted in the shorthand form: response-loss replay ⇒ no-op.
	deps, wdeps, gdeps, engine := newDeps()
	res := ChangeMarkImplemented(context.Background(), deps, wdeps, gdeps, repo.invocation,
		MarkImplementedRequest{ID: 3, Version: miVersion, Head: head, PR: prRepo().Spec() + "#42", EvidenceRecord: prEvidenceBytes(t, head)})
	if res.Result != ResultNoOp {
		t.Fatalf("cross-form replay result = %q, want no-op (findings %v)", res.Result, res.Findings)
	}
	if len(engine.calls) != 0 {
		t.Errorf("cross-form replay called the engine %d times, want 0", len(engine.calls))
	}

	// A different PR number: genuine conflict ⇒ contended.
	deps, wdeps, gdeps, engine = newDeps()
	res = ChangeMarkImplemented(context.Background(), deps, wdeps, gdeps, repo.invocation,
		MarkImplementedRequest{ID: 3, Version: miVersion, Head: head, PR: prRepo().Spec() + "#99", EvidenceRecord: prEvidenceBytes(t, head)})
	if res.Result != ResultContended {
		t.Fatalf("different-PR replay result = %q, want contended (findings %v)", res.Result, res.Findings)
	}
	if len(engine.calls) != 0 {
		t.Errorf("different-PR replay called the engine %d times, want 0", len(engine.calls))
	}
}

// firstStatusFindingCode returns the code of the first finding, or "".
func firstStatusFindingCode(findings []StatusFinding) string {
	if len(findings) > 0 {
		return findings[0].Code
	}
	return ""
}
