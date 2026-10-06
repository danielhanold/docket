//go:build integration

package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is the leak-check slice of the workflow lifecycle shard (prefix
// TestIntegrationWorkflowLifecycle). Over real repositories it proves that a
// private repository's workspace.publish scans exactly what the push would
// expose and refuses — with nothing reaching origin — on every seeded docket
// fingerprint; that leak_check.match_word: false lets only the bare word
// through; that a scan which cannot run refuses as unverified rather than
// reading clean; and that a shared repository never runs the scan at all.
//
// None of these tests is parallel: planningDepsFor uses t.Setenv.

// leakWorkspace is a claimed, reconciled change with a prepared feature
// workspace whose first commit is the spec copy, ready to seed and publish.
type leakWorkspace struct {
	node                       realNode
	wdeps                      WorkspaceDeps
	id                         int
	wp, specCommit             string // feature worktree; spec-copy commit
	featureRef, specPath       string // the feature ref publish pushes; the spec copy's repo path
	origin, writer, invocation string // bare origin; a clone with origin set; the invocation clone
}

// prepareLeakWorkspace runs the opening steps of driveClaimToImplemented in
// order — context, claim, reconcile, workspace prepare, and the spec-copy commit
// — and fails unless each applies. A change without a spec (spec false) must
// report no-spec instead, and its prepared head stands in for the spec commit.
func prepareLeakWorkspace(t *testing.T, repo *gitRepo, branch string, id int, recPath string, spec bool) *leakWorkspace {
	t.Helper()
	ctx := context.Background()
	node := planningDepsFor(t, repo.invocation)
	svc, err := workspace.NewService(node.deps.Client)
	if err != nil {
		t.Fatalf("workspace.NewService: %v", err)
	}
	wdeps := WorkspaceDeps{Service: svc}
	ver := func() string { return blobRevisionAt(t, repo.meta, branch, recPath) }

	ctxRes := ContextImplementation(ctx, node.deps, node.dir, ImplementationContextRequest{ID: id})
	if ctxRes.Result != ResultApplied || ctxRes.Context == nil {
		t.Fatalf("context implementation = %q (reason %q); want a bundle", ctxRes.Result, ctxRes.Reason)
	}
	claim := ChangeClaim(ctx, node.deps, node.dir, ChangeClaimRequest{ID: id, Revision: ctxRes.Context.Change.Revision, RunContext: ""})
	if claim.Result != ResultApplied || claim.Disposition != ClaimDispositionApplied {
		t.Fatalf("claim = (%q, %q), want applied/applied (findings %v)", claim.Result, claim.Disposition, claim.Findings)
	}
	rec := ChangeReconcile(ctx, node.deps, node.dir, ChangeReconcileRequest{
		ID: id, Revision: ver(), ReconcileLogEntry: "Reconciled against current reality.\n",
	})
	if rec.Result != ResultApplied {
		t.Fatalf("reconcile = %q (findings %v)", rec.Result, rec.Findings)
	}
	prep := WorkspacePrepare(ctx, node.deps, wdeps, node.dir, WorkspaceIDRequest{ID: id, Revision: ver()})
	if prep.Result != ResultApplied {
		t.Fatalf("workspace prepare = %q (reason %q msg %q)", prep.Result, prep.Reason, prep.Message)
	}
	cs := WorkspaceCommitSpec(ctx, node.deps, wdeps, node.dir, WorkspaceIDRequest{ID: id})
	if !spec {
		if cs.Result != ResultNoOp || cs.Disposition != SpecCopyNoSpec {
			t.Fatalf("workspace commit-spec = (%q, %q) (reason %q msg %q), want no-op/no-spec", cs.Result, cs.Disposition, cs.Reason, cs.Message)
		}
		return &leakWorkspace{
			node: node, wdeps: wdeps, id: id,
			wp: prep.Path, specCommit: runGit(t, prep.Path, "rev-parse", "HEAD"),
			featureRef: prep.FeatureRef,
			origin:     repo.origin, writer: repo.writer, invocation: repo.invocation,
		}
	}
	if cs.Result != ResultApplied || cs.Disposition != SpecCopyCommitted {
		t.Fatalf("workspace commit-spec = (%q, %q) (reason %q msg %q), want applied/committed", cs.Result, cs.Disposition, cs.Reason, cs.Message)
	}
	var specs []string
	for _, p := range strings.Split(runGit(t, prep.Path, "show", "--name-only", "--format=", cs.Head), "\n") {
		if strings.HasSuffix(p, "-design.md") {
			specs = append(specs, p)
		}
	}
	if len(specs) != 1 {
		t.Fatalf("the spec-copy commit changed %v, want exactly one spec copy", specs)
	}
	return &leakWorkspace{
		node: node, wdeps: wdeps, id: id,
		wp: prep.Path, specCommit: cs.Head,
		featureRef: prep.FeatureRef, specPath: specs[0],
		origin: repo.origin, writer: repo.writer, invocation: repo.invocation,
	}
}

// newPrivateLeakWorkspace repeats the private lifecycle test's setup through
// groom — init --private, a configured gate, change 1 created and groomed with
// a spec — and prepares its workspace against the dckt store.
func newPrivateLeakWorkspace(t *testing.T) (*leakWorkspace, layout.Layout) {
	t.Helper()
	return newPrivateLeakWorkspaceFor(t, validChangeCreateRequest(), true)
}

// newPrivateLeakWorkspaceFor is newPrivateLeakWorkspace for the given create
// request, groomed with a spec (spec true) or as trivial (no spec copy).
func newPrivateLeakWorkspaceFor(t *testing.T, create ChangeCreateRequest, spec bool) (*leakWorkspace, layout.Layout) {
	t.Helper()
	ctx := context.Background()
	r, _ := newPrivateInitRepo(t, nil)
	if res := r.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
		t.Fatalf("init --private = %q (%s), want applied", res.Result, res.HumanText())
	}
	gate := passingGateScript(t)
	if ct := r.runConfigureTestsWith(t, ConfigureTestsOptions{Command: &gate}); ct.Result != ResultApplied {
		t.Fatalf("configure-tests = %q (%s), want applied", ct.Result, ct.HumanText())
	}
	lay := resolvedPrivateLayout(t, r.invocation)
	repo := &gitRepo{root: r.root, origin: r.origin, meta: lay.DefaultBareRemote, writer: r.writer, invocation: r.invocation}

	node := planningDepsFor(t, r.invocation)
	created := ChangeCreate(ctx, node.deps, node.dir, create)
	if created.Result != ResultApplied {
		t.Fatalf("change create = %q (%s, findings %v)", created.Result, created.HumanText(), created.Findings)
	}
	if created.ID != 1 {
		t.Fatalf("created change id = %d, want 1 (the seeded change references name 0001)", created.ID)
	}
	req := ChangeGroomRequest{
		ChangeID:     created.ID,
		Path:         created.Path,
		Revision:     blobRevisionAt(t, repo.meta, privateFixtureBranch, created.Path),
		Outcome:      GroomSpec,
		SpecMarkdown: "# Widget: design\n\nBuild the widget.\n",
	}
	if !spec {
		req.Outcome, req.SpecMarkdown = GroomTrivial, ""
		req.Sections = []SectionEditRequest{{Heading: "## Why", Intent: "replace", Markdown: "Too small to design.\n"}}
	}
	groom := ChangeGroom(ctx, node.deps, node.dir, req)
	if groom.Result != ResultApplied || (spec && groom.SpecPath == "") {
		t.Fatalf("change groom = %q (%s, findings %v), want applied (with a spec: %v)", groom.Result, groom.HumanText(), groom.Findings, spec)
	}
	return prepareLeakWorkspace(t, repo, privateFixtureBranch, created.ID, created.Path, spec), lay
}

// newSharedLeakWorkspace builds the shared (docket-branch) fixture
// runClaimToImplemented uses — change 3 with a linked spec — and prepares it.
func newSharedLeakWorkspace(t *testing.T) *leakWorkspace {
	t.Helper()
	const (
		id   = 3
		slug = "widget"
	)
	recPath := groomPath(id, slug)
	specPath := "docs/superpowers/specs/2026-08-17-" + slug + "-design.md"
	m := planRepoModes()[0]
	repo := buildConfiguredRepoWith(t, m, map[string]string{
		recPath:  specLinkedBuildReadyChange(id, slug, specPath),
		specPath: workflowMetadataSpec(id, slug),
	})
	return prepareLeakWorkspace(t, repo, m.branch, id, recPath, true)
}

// commitInWorkspace writes files into the worktree, commits everything with
// msg, and returns the new head.
func commitInWorkspace(t *testing.T, wp, msg string, files map[string]string) string {
	t.Helper()
	for rel, content := range files {
		writeRepoFile(t, wp, rel, content)
	}
	runGit(t, wp, "add", "-A")
	runGit(t, wp, "commit", "-q", "--allow-empty", "-m", msg)
	return runGit(t, wp, "rev-parse", "HEAD")
}

// resetWorkspace moves the worktree's branch and tree to commit.
func resetWorkspace(t *testing.T, wp, commit string) {
	t.Helper()
	runGit(t, wp, "reset", "-q", "--hard", commit)
}

// originHasBranch reports whether the bare origin carries ref. A probe that
// fails for any reason other than an absent ref fails the test, so an
// unreadable origin never reads as "absent".
func originHasBranch(t *testing.T, origin, ref string) bool {
	t.Helper()
	out := runGit(t, origin, "for-each-ref", "--format=%(refname)", ref)
	return out == ref
}

// originRefTip returns ref's commit on the bare origin, failing when absent.
func originRefTip(t *testing.T, origin, ref string) string {
	t.Helper()
	if !originHasBranch(t, origin, ref) {
		t.Fatalf("origin lacks %s", ref)
	}
	return runGit(t, origin, "rev-parse", ref)
}

// publish runs workspace.publish for head on lw.
func (lw *leakWorkspace) publish(t *testing.T, head string) WorkspaceOpResult {
	t.Helper()
	return WorkspacePublish(context.Background(), lw.node.deps, lw.wdeps, lw.node.dir, WorkspacePublishRequest{ID: lw.id, Head: head})
}

// requireLeakRefusal requires a blocked/leak-detected publish whose leaks hold
// exactly one hit matching want (Commit is compared only when want sets it,
// Line only when non-zero), whose message names the hit's text and rule, and
// whose rule-shown hit carries only the matched text.
func requireLeakRefusal(t *testing.T, pub WorkspaceOpResult, want LeakHit) {
	t.Helper()
	if pub.Result != ResultBlocked || pub.Reason != ReasonLeakDetected {
		t.Fatalf("publish = %q/%q (msg %q), want blocked/%s", pub.Result, pub.Reason, pub.Message, ReasonLeakDetected)
	}
	if len(pub.Leaks) != 1 {
		t.Fatalf("leaks = %+v, want exactly one hit %+v", pub.Leaks, want)
	}
	got := pub.Leaks[0]
	if got.Source != want.Source || got.Rule != want.Rule || got.Text != want.Text || got.File != want.File ||
		(want.Commit != "" && got.Commit != want.Commit) || (want.Line != 0 && got.Line != want.Line) {
		t.Errorf("leak hit = %+v, want %+v", got, want)
	}
	if !strings.Contains(pub.Message, "("+want.Rule+")") || !strings.Contains(pub.Message, want.Text) {
		t.Errorf("refusal message does not show the hit %q (%s): %q", want.Text, want.Rule, pub.Message)
	}
}

// TestIntegrationWorkflowLifecyclePrivateLeakCheckBlocksSeededLeaks seeds each
// fingerprint kind onto the feature branch and proves publish refuses with the
// hit shown and pushes nothing; a fingerprint-free year and ADR reference
// publish, and upstream main merged into the branch is not scanned as outgoing.
func TestIntegrationWorkflowLifecyclePrivateLeakCheckBlocksSeededLeaks(t *testing.T) {
	lw, _ := newPrivateLeakWorkspace(t)

	specBytes, err := os.ReadFile(filepath.Join(lw.wp, lw.specPath))
	if err != nil {
		t.Fatalf("reading the spec copy: %v", err)
	}
	spec := string(specBytes)
	if !strings.HasSuffix(spec, "\n") {
		spec += "\n"
	}
	appendedLine := strings.Count(spec, "\n") + 1

	for _, tc := range []struct {
		name  string
		msg   string
		files map[string]string
		want  func(head string) LeakHit
	}{
		{"subject-change-id", "Add the widget (0001)", map[string]string{"widget.go": "package widget\n"},
			func(head string) LeakHit {
				return LeakHit{Source: "commit-message", Commit: head, Line: 1, Text: "(0001)", Rule: "change-ref"}
			}},
		{"added-docket-path", "Add notes", map[string]string{"notes.txt": "see .docket/x\n"},
			func(string) LeakHit {
				return LeakHit{Source: "added-line", File: "notes.txt", Line: 1, Text: ".docket", Rule: "path"}
			}},
		{"spec-copy-word", "Note the tracking", map[string]string{lw.specPath: spec + "Tracked in docket.\n"},
			func(string) LeakHit {
				return LeakHit{Source: "added-line", File: lw.specPath, Line: appendedLine, Text: "docket", Rule: "word"}
			}},
		{"dckt-in-message", "wire the dckt alias", map[string]string{"alias.txt": "alias\n"},
			func(head string) LeakHit {
				return LeakHit{Source: "commit-message", Commit: head, Line: 1, Text: "dckt", Rule: "alias"}
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetWorkspace(t, lw.wp, lw.specCommit)
			head := commitInWorkspace(t, lw.wp, tc.msg, tc.files)
			requireLeakRefusal(t, lw.publish(t, head), tc.want(head))
			if originHasBranch(t, lw.origin, lw.featureRef) {
				t.Errorf("a refused publish left %s on origin", lw.featureRef)
			}
		})
	}

	// A fingerprint one outgoing commit adds and a later one removes is absent
	// from the net diff, yet the push still carries the adding commit: the
	// refusal names that commit.
	t.Run("added-then-removed-docket-path", func(t *testing.T) {
		resetWorkspace(t, lw.wp, lw.specCommit)
		added := commitInWorkspace(t, lw.wp, "Add notes", map[string]string{"notes.txt": "see .docket/x\n"})
		runGit(t, lw.wp, "rm", "-q", "notes.txt")
		head := commitInWorkspace(t, lw.wp, "Drop notes", nil)
		pub := lw.publish(t, head)
		requireLeakRefusal(t, pub,
			LeakHit{Source: "added-line", Commit: added, File: "notes.txt", Line: 1, Text: ".docket", Rule: "path"})
		if !strings.Contains(pub.Message, shortCommit(added)) {
			t.Errorf("refusal message does not name the adding commit %s: %q", shortCommit(added), pub.Message)
		}
		if originHasBranch(t, lw.origin, lw.featureRef) {
			t.Errorf("a refused publish left %s on origin", lw.featureRef)
		}
	})

	var cleanHead string
	t.Run("clean-year-and-adr-publishes", func(t *testing.T) {
		resetWorkspace(t, lw.wp, lw.specCommit)
		cleanHead = commitInWorkspace(t, lw.wp, "Lay out the widget per ADR-0001 (2026)",
			map[string]string{"layout.md": "See ADR-0001 (2026).\n"})
		pub := lw.publish(t, cleanHead)
		if pub.Result != ResultApplied {
			t.Fatalf("publish = %q/%q (msg %q, leaks %+v), want applied", pub.Result, pub.Reason, pub.Message, pub.Leaks)
		}
		if got := originRefTip(t, lw.origin, lw.featureRef); got != cleanHead {
			t.Errorf("origin %s = %s, want the published head %s", lw.featureRef, got, cleanHead)
		}
	})

	t.Run("merged-main-is-not-outgoing", func(t *testing.T) {
		if cleanHead == "" {
			t.Fatal("the clean publish did not run")
		}
		runGit(t, lw.writer, "fetch", "-q", "origin", "main")
		runGit(t, lw.writer, "reset", "-q", "--hard", "origin/main")
		writeRepoFile(t, lw.writer, "upstream.txt", "docket\n")
		runGit(t, lw.writer, "add", "upstream.txt")
		runGit(t, lw.writer, "commit", "-q", "-m", "Upstream docket note (0001)")
		runGit(t, lw.writer, "push", "-q", "origin", "main")

		runGit(t, lw.wp, "fetch", "-q", "origin", "main")
		runGit(t, lw.wp, "merge", "-q", "--no-edit", "origin/main")
		head := runGit(t, lw.wp, "rev-parse", "HEAD")
		pub := lw.publish(t, head)
		if pub.Result != ResultApplied {
			t.Fatalf("publish of a branch with upstream main merged in = %q/%q (msg %q, leaks %+v), want applied",
				pub.Result, pub.Reason, pub.Message, pub.Leaks)
		}
		if got := originRefTip(t, lw.origin, lw.featureRef); got != head {
			t.Errorf("origin %s = %s, want the merged head %s", lw.featureRef, got, head)
		}
		if len(pub.Findings) != 0 {
			t.Errorf("publish findings = %+v, want none while origin holds no docket-named ref", pub.Findings)
		}
	})

	t.Run("docket-named-ref-on-origin-is-reported", func(t *testing.T) {
		if cleanHead == "" {
			t.Fatal("the clean publish did not run")
		}
		runGit(t, lw.writer, "push", "-q", "origin", "main:refs/heads/dckt")
		head := commitInWorkspace(t, lw.wp, "Polish the widget layout", map[string]string{"polish.md": "Polished.\n"})
		pub := lw.publish(t, head)
		if pub.Result != ResultApplied {
			t.Fatalf("publish = %q/%q (msg %q, leaks %+v), want applied; a docket-named origin ref never blocks",
				pub.Result, pub.Reason, pub.Message, pub.Leaks)
		}
		if len(pub.Findings) != 1 || pub.Findings[0].Code != FindingMetadataOnSharedRemote ||
			pub.Findings[0].Path != "refs/heads/dckt" || pub.Findings[0].Severity != "warning" {
			t.Errorf("publish findings = %+v, want one %s warning for refs/heads/dckt", pub.Findings, FindingMetadataOnSharedRemote)
		}
		if got := originRefTip(t, lw.origin, lw.featureRef); got != head {
			t.Errorf("origin %s = %s, want the published head %s", lw.featureRef, got, head)
		}
	})
}

// TestIntegrationWorkflowLifecyclePrivateLeakCheckMatchWordOff proves
// leak_check.match_word: false lets the bare word docket through while every
// other rule still blocks.
func TestIntegrationWorkflowLifecyclePrivateLeakCheckMatchWordOff(t *testing.T) {
	lw, lay := newPrivateLeakWorkspace(t)
	f, err := os.OpenFile(lay.ConfigPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("opening the private config %s: %v", lay.ConfigPath, err)
	}
	if _, err := f.WriteString("leak_check:\n  match_word: false\n"); err != nil {
		t.Fatalf("appending to the private config: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing the private config: %v", err)
	}

	resetWorkspace(t, lw.wp, lw.specCommit)
	published := commitInWorkspace(t, lw.wp, "File the docket number", map[string]string{"entry.txt": "the docket entry\n"})
	if pub := lw.publish(t, published); pub.Result != ResultApplied {
		t.Fatalf("publish under match_word false = %q/%q (msg %q, leaks %+v), want applied", pub.Result, pub.Reason, pub.Message, pub.Leaks)
	}
	if got := originRefTip(t, lw.origin, lw.featureRef); got != published {
		t.Fatalf("origin %s = %s, want %s", lw.featureRef, got, published)
	}

	for _, tc := range []struct {
		name  string
		msg   string
		files map[string]string
		want  LeakHit
	}{
		{"marker", "Add a marker file", map[string]string{"marker.md": "<!-- docket:backlink:start -->\n"},
			LeakHit{Source: "added-line", File: "marker.md", Line: 1, Text: "docket:", Rule: "marker"}},
		{"trailer", "Tidy the widget\n\nDocket-Operation: x", map[string]string{"tidy.txt": "tidy\n"},
			LeakHit{Source: "commit-message", Text: "Docket-Operation:", Rule: "trailer"}},
		{"path", "Add a path note", map[string]string{"path.txt": ".docket/x\n"},
			LeakHit{Source: "added-line", File: "path.txt", Line: 1, Text: ".docket", Rule: "path"}},
		{"alias", "dckt", map[string]string{"alias.txt": "alias\n"},
			LeakHit{Source: "commit-message", Line: 1, Text: "dckt", Rule: "alias"}},
		{"change-ref", "(0001)", map[string]string{"ref.txt": "ref\n"},
			LeakHit{Source: "commit-message", Line: 1, Text: "(0001)", Rule: "change-ref"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetWorkspace(t, lw.wp, published)
			head := commitInWorkspace(t, lw.wp, tc.msg, tc.files)
			requireLeakRefusal(t, lw.publish(t, head), tc.want)
			if got := originRefTip(t, lw.origin, lw.featureRef); got != published {
				t.Errorf("a refused publish moved origin %s to %s, want %s", lw.featureRef, got, published)
			}
		})
	}
}

// TestIntegrationWorkflowLifecyclePrivateLeakCheckUnverifiedPushesNothing
// proves a scan that cannot run — a feature branch sharing no history with
// origin's base, so the merge-base diff fails — refuses as unverified and
// pushes nothing, never reading as a clean scan.
func TestIntegrationWorkflowLifecyclePrivateLeakCheckUnverifiedPushesNothing(t *testing.T) {
	lw, _ := newPrivateLeakWorkspace(t)
	orphan := runGit(t, lw.wp, "commit-tree", lw.specCommit+"^{tree}", "-m", "Unrelated root")
	resetWorkspace(t, lw.wp, orphan)

	pub := lw.publish(t, orphan)
	if pub.Result != ResultExternalFailed || pub.Reason != ReasonLeakCheckUnverified {
		t.Fatalf("publish of an unscannable head = %q/%q (msg %q), want %s/%s",
			pub.Result, pub.Reason, pub.Message, ResultExternalFailed, ReasonLeakCheckUnverified)
	}
	if originHasBranch(t, lw.origin, lw.featureRef) {
		t.Errorf("an unverified scan still pushed %s to origin", lw.featureRef)
	}
}

// TestIntegrationWorkflowLifecyclePrivateLeakCheckScansBranchName proves the
// pushed feature branch name is scanned like any other published text: a
// trivial change titled "Fix the dckt alias" — no spec copy, clean commits —
// still refuses, because its slug puts dckt in the branch origin would show.
func TestIntegrationWorkflowLifecyclePrivateLeakCheckScansBranchName(t *testing.T) {
	create := validChangeCreateRequest()
	create.Title, create.Type = "Fix the dckt alias", "fix"
	lw, _ := newPrivateLeakWorkspaceFor(t, create, false)
	if lw.featureRef != "refs/heads/fix/fix-the-dckt-alias" {
		t.Fatalf("feature ref = %q, want refs/heads/fix/fix-the-dckt-alias", lw.featureRef)
	}

	head := commitInWorkspace(t, lw.wp, "Tidy the alias table", map[string]string{"aliases.txt": "tidy\n"})
	requireLeakRefusal(t, lw.publish(t, head), LeakHit{Source: "branch-name", Text: "dckt", Rule: "alias"})
	if originHasBranch(t, lw.origin, lw.featureRef) {
		t.Errorf("a refused publish left %s on origin", lw.featureRef)
	}
}

// TestIntegrationWorkflowLifecyclePrivateLeakCheckGatesPR proves a private
// repository's pr.publish scans the authored title and body (as well as the
// outgoing commits) before any GitHub write: a change reference in the title or
// a .docket path in the body refuses with nothing ensured and the body never
// echoed back, while clean prose is ensured byte-for-byte.
func TestIntegrationWorkflowLifecyclePrivateLeakCheckGatesPR(t *testing.T) {
	lw, _ := newPrivateLeakWorkspace(t)
	head := commitInWorkspace(t, lw.wp, "Add the widget", map[string]string{"widget.go": "package widget\n"})
	if pub := lw.publish(t, head); pub.Result != ResultApplied {
		t.Fatalf("publish of the clean head = %q/%q (msg %q, leaks %+v), want applied", pub.Result, pub.Reason, pub.Message, pub.Leaks)
	}
	evidenceBytes := prEvidenceBytes(t, head)

	prPublish := func(gh *fakeGitHub, title, body string) PRPublishResult {
		return PRPublish(context.Background(), lw.node.deps, lw.wdeps, GitHubDeps{Service: gh}, lw.node.dir, PRPublishRequest{
			ID: lw.id, Head: head, Title: title, Body: body, EvidenceRecord: evidenceBytes,
		})
	}
	newGH := func() *fakeGitHub {
		return &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR("x")}}
	}
	requirePRLeak := func(t *testing.T, gh *fakeGitHub, res PRPublishResult, want LeakHit) {
		t.Helper()
		if res.Result != ResultBlocked || res.Reason != ReasonLeakDetected {
			t.Fatalf("pr publish = %q/%q (msg %q), want blocked/%s", res.Result, res.Reason, res.Message, ReasonLeakDetected)
		}
		if len(gh.ensureCalls) != 0 {
			t.Errorf("a leak-refused pr publish still ensured a PR: %+v", gh.ensureCalls)
		}
		if len(res.Leaks) != 1 {
			t.Fatalf("leaks = %+v, want exactly one hit %+v", res.Leaks, want)
		}
		got := res.Leaks[0]
		if got.Source != want.Source || got.Line != want.Line || got.Rule != want.Rule || (want.Text != "" && got.Text != want.Text) {
			t.Errorf("leak hit = %+v, want %+v", got, want)
		}
		if !strings.Contains(res.Message, "("+want.Rule+")") {
			t.Errorf("refusal message does not show the rule %s: %q", want.Rule, res.Message)
		}
	}

	t.Run("change-id-in-title", func(t *testing.T) {
		gh := newGH()
		requirePRLeak(t, gh, prPublish(gh, "Add the widget for change 0001", "Plain prose.\n"),
			LeakHit{Source: "pr-title", Line: 1, Rule: "change-ref"})
	})

	t.Run("docket-path-in-body", func(t *testing.T) {
		gh := newGH()
		res := prPublish(gh, "Add the widget", "Plain.\nSee .docket/notes for more.\n")
		requirePRLeak(t, gh, res, LeakHit{Source: "pr-body", Line: 2, Rule: "path", Text: ".docket"})
		doc, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("marshal the refusal: %v", err)
		}
		if strings.Contains(string(doc), "See .docket/notes for more") {
			t.Errorf("the refusal carries the PR body line; a hit holds only the matched token:\n%s", doc)
		}
	})

	t.Run("clean-prose-is-ensured-verbatim", func(t *testing.T) {
		gh := newGH()
		res := prPublish(gh, "Add the widget", "Plain prose.\n")
		if res.Result != ResultApplied {
			t.Fatalf("pr publish of clean prose = %q/%q (msg %q, leaks %+v), want applied", res.Result, res.Reason, res.Message, res.Leaks)
		}
		if len(gh.ensureCalls) != 1 {
			t.Fatalf("ensure calls = %d, want 1", len(gh.ensureCalls))
		}
		if got := gh.ensureCalls[0]; got.Body != "Plain prose.\n" || got.Title != "Add the widget" {
			t.Errorf("ensured title/body = %q/%q, want the authored prose verbatim", got.Title, got.Body)
		}
	})
}

// TestIntegrationWorkflowLifecycleSharedPublishRunsNoLeakCheck pins that a
// shared repository never runs the scan: a change reference and a .docket path
// publish untouched.
func TestIntegrationWorkflowLifecycleSharedPublishRunsNoLeakCheck(t *testing.T) {
	lw := newSharedLeakWorkspace(t)
	head := commitInWorkspace(t, lw.wp, "Add the widget (0003)", map[string]string{"x.txt": ".docket/x\n"})
	pub := lw.publish(t, head)
	if pub.Result != ResultApplied {
		t.Fatalf("shared publish = %q/%q (msg %q, leaks %+v), want applied with no scan", pub.Result, pub.Reason, pub.Message, pub.Leaks)
	}
	if len(pub.Leaks) != 0 {
		t.Errorf("a shared publish reported leaks %+v", pub.Leaks)
	}
	if got := originRefTip(t, lw.origin, lw.featureRef); got != head {
		t.Errorf("origin %s = %s, want %s", lw.featureRef, got, head)
	}
}
