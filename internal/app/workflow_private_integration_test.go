//go:build integration

package app

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is the private-visibility acceptance slice of the workflow
// lifecycle shard (prefix TestIntegrationWorkflowLifecycle). It drives a
// private repository from `init --private` through change creation, grooming,
// and the whole claim-to-implemented run, then proves the repository is left
// with nothing docket-named in it: the metadata branch lives on the bare
// `dckt` store, origin gains only the feature branch, and a walk of the whole
// clone — .git included — finds no path component spelled `docket`.

// privateFixtureBranch is the private metadata branch as the spec spells it.
// It is a literal, not layout.PrivateName, so a resolver that drifts from the
// spec's spelling cannot carry the oracle with it.
const privateFixtureBranch = "dckt"

// assertNoDocketPathUnder walks root — its .git directory included — and fails
// listing every path whose component contains "docket" in any case. A
// docket-named directory is listed once and not descended into. The walk proves
// it is not vacuous: it must have seen the private state folder under .git.
func assertNoDocketPathUnder(t *testing.T, root string) {
	t.Helper()
	var hits []string
	sawState := false
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		if rel == "." {
			return nil
		}
		if rel == filepath.Join(".git", layout.PrivateName) {
			sawState = true
		}
		if strings.Contains(strings.ToLower(d.Name()), "docket") {
			hits = append(hits, rel)
			if d.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	if !sawState {
		t.Errorf("the walk of %s never reached .git/%s; it must cover .git", root, layout.PrivateName)
	}
	if len(hits) != 0 {
		sort.Strings(hits)
		t.Errorf("found %d docket-named path(s) under %s; a private repository holds none:\n  %s",
			len(hits), root, strings.Join(hits, "\n  "))
	}
}

// localHeads returns the sorted refs/heads/* names of the repository at dir.
func localHeads(t *testing.T, dir string) []string {
	t.Helper()
	out := runGit(t, dir, "for-each-ref", "--format=%(refname)", "refs/heads")
	heads := strings.Fields(out)
	sort.Strings(heads)
	return heads
}

// resolvedPrivateLayout resolves the clone's layout through the production
// resolver with a real client, and requires it to be private.
func resolvedPrivateLayout(t *testing.T, dir string) layout.Layout {
	t.Helper()
	client := newGitClient(t)
	repo, err := client.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: dir})
	if err != nil {
		t.Fatalf("Discover %s: %v", dir, err)
	}
	lay, err := resolveLayout(context.Background(), client, repo)
	if err != nil {
		t.Fatalf("resolveLayout %s: %v", dir, err)
	}
	if lay.Mode != layout.Private {
		t.Fatalf("layout mode = %q after init --private, want private", lay.Mode)
	}
	return lay
}

// privateLifecycleChecks drives the whole claim-to-implemented run, then proves
// what a private repository publishes on origin: the one open PR carries the
// authored title and prose byte-for-byte (no backlink, no artifacts block), and
// the spec copy at the feature head carries no change line. A finalize block
// then records its marker on the store's record and posts no PR comment.
func privateLifecycleChecks(t *testing.T, created ChangeCreateResult, store string) workflowEntry {
	return func(node realNode, wdeps WorkspaceDeps, complete func(string) GitHubDeps) {
		t.Helper()
		ctx := context.Background()
		gdeps := complete("")

		insp := WorkspaceInspect(ctx, node.deps, wdeps, node.dir, WorkspaceIDRequest{ID: created.ID})
		if insp.Result != ResultApplied || insp.FeatureRef == "" || insp.Head == "" {
			t.Fatalf("workspace inspect = %q (reason %q), want a feature ref and head", insp.Result, insp.Reason)
		}
		headBranch := strings.TrimPrefix(insp.FeatureRef, "refs/heads/")
		repo, err := gdeps.Service.DiscoverRepository(ctx, node.dir)
		if err != nil {
			t.Fatalf("discover repository: %v", err)
		}
		prs, err := gdeps.Service.FindOpenPullRequestsByHead(ctx, repo, headBranch)
		if err != nil {
			t.Fatalf("find open PRs for %s: %v", headBranch, err)
		}
		if len(prs) != 1 {
			t.Fatalf("open PRs for %s = %d, want exactly one", headBranch, len(prs))
		}
		if want := "Authored PR prose for the widget.\n"; prs[0].Body != want {
			t.Errorf("private PR body = %q, want the authored prose verbatim %q", prs[0].Body, want)
		}
		if prs[0].Title != "Add the widget" {
			t.Errorf("private PR title = %q, want %q", prs[0].Title, "Add the widget")
		}

		// The spec copy at the feature head: present, and no change line.
		var specs []string
		for _, p := range strings.Split(runGit(t, node.dir, "ls-tree", "-r", "--name-only", insp.Head), "\n") {
			if strings.HasSuffix(p, "-design.md") {
				specs = append(specs, p)
			}
		}
		if len(specs) != 1 {
			t.Fatalf("spec copies at the feature head = %v, want exactly one", specs)
		}
		spec := runGit(t, node.dir, "show", insp.Head+":"+specs[0])
		if !strings.Contains(spec, "Build the widget.") {
			t.Fatalf("spec copy %s lacks the authored spec:\n%s", specs[0], spec)
		}
		for _, line := range strings.Split(spec, "\n") {
			if strings.HasPrefix(line, "Change 0") {
				t.Errorf("private spec copy %s carries a change line %q:\n%s", specs[0], line, spec)
			}
		}

		svc, err := workspace.NewService(node.deps.Client)
		if err != nil {
			t.Fatalf("workspace service: %v", err)
		}
		gh := &fakeBlockGitHub{repo: prRepo(), commentOutcome: githubcli.CommentCreated, commentURL: "https://example.invalid/c/1"}

		// finalize.publish scans the exact head it would push before it reads the
		// rebase receipt: a head whose own message carries a change reference is
		// leak-detected, and the clean head passes the scan and reaches the
		// receipt read (no-rebase-receipt). Neither moves the feature branch.
		finalizePublish := func(head string) FinalizePublishResult {
			rec, rerr := evidence.NewRecord("gate", head, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
			if rerr != nil {
				t.Fatalf("evidence.NewRecord: %v", rerr)
			}
			return FinalizePublish(ctx, FinalizeDeps{Planning: node.deps, GitHub: gh, Workspace: svc}, node.dir, FinalizePublishRequest{
				ID: created.ID, Attempt: "a1", Head: head, EvidenceRecord: []byte(evidence.Render(rec)),
			})
		}
		leaky := runGit(t, node.dir, "commit-tree", insp.Head+"^{tree}", "-p", insp.Head, "-m", "Finalize (0001)")
		fp := finalizePublish(leaky)
		if fp.Result != ResultBlocked || fp.Reason != ReasonLeakDetected {
			t.Fatalf("private finalize publish of a leaky head = %q/%q (msg %q), want blocked/%s", fp.Result, fp.Reason, fp.Message, ReasonLeakDetected)
		}
		if len(fp.Leaks) != 1 || fp.Leaks[0].Commit != leaky || fp.Leaks[0].Rule != "change-ref" || fp.Leaks[0].Source != "commit-message" {
			t.Errorf("finalize publish leaks = %+v, want one change-ref hit in commit %s", fp.Leaks, leaky)
		}
		if !strings.Contains(fp.Message, shortCommit(leaky)) {
			t.Errorf("finalize publish refusal does not name the leaky commit %s: %q", shortCommit(leaky), fp.Message)
		}
		if clean := finalizePublish(insp.Head); clean.Result != ResultBlocked || clean.Reason != ReasonPublishNoReceipt {
			t.Errorf("private finalize publish of the clean head = %q/%q (msg %q, leaks %+v), want blocked/%s",
				clean.Result, clean.Reason, clean.Message, clean.Leaks, ReasonPublishNoReceipt)
		}
		if got := runGit(t, node.dir, "rev-parse", insp.FeatureRef); got != insp.Head {
			t.Errorf("finalize publish moved %s to %s, want %s", insp.FeatureRef, got, insp.Head)
		}

		// A private finalize block posts no PR comment: the marker on the record
		// is its only effect (acceptance 6).
		blocked := FinalizeBlock(ctx, FinalizeDeps{Planning: node.deps, GitHub: gh, Workspace: svc}, node.dir, BlockRequest{
			ID:       created.ID,
			Revision: blobRevisionAt(t, store, privateFixtureBranch, created.Path),
			PRNumber: prs[0].Number,
			Attempt:  "a1",
			Reason:   "gate-failed",
			Head:     insp.Head,
			Report:   "The gate failed.\n",
		})
		if blocked.Result != ResultApplied || blocked.Disposition != BlockDispRecorded {
			t.Fatalf("private finalize block = %q/%q (%s), want applied/recorded", blocked.Result, blocked.Disposition, blocked.HumanText())
		}
		if gh.ensureCalls != 0 || blocked.CommentURL != "" {
			t.Errorf("private finalize block posted a PR comment: ensureCalls=%d url=%q", gh.ensureCalls, blocked.CommentURL)
		}
		record := runGit(t, store, "show", privateFixtureBranch+":"+created.Path)
		if !strings.Contains(record, "## Finalize blocked") {
			t.Errorf("the record at the store tip lacks the finalize-blocked marker:\n%s", record)
		}
		if strings.Contains(record, "- Comment:") {
			t.Errorf("the private finalize-blocked marker names a comment:\n%s", record)
		}
	}
}

// TestIntegrationWorkflowLifecyclePrivateInitToImplemented is acceptance 1: a
// private repository goes from init through implemented, and afterwards the
// clone holds nothing docket-named and origin holds nothing but the feature
// branch.
func TestIntegrationWorkflowLifecyclePrivateInitToImplemented(t *testing.T) {
	t.Setenv("DOCKET_SCRIPTS_DIR", "")
	ghBin := buildFakeGH(t)
	r, data := newPrivateInitRepo(t, nil)
	ctx := context.Background()

	// The walk below is relative to the clone, so only names the fixture puts
	// INSIDE the clone could make it fire on the fixture's account rather than
	// the product's. (testsupport.TempDir's "docketfix-" prefix sits above the
	// walk root, and the store under the data home is outside the clone.)
	req := validChangeCreateRequest()
	for _, name := range []string{filepath.Base(r.invocation), layout.CloneID(r.invocation), req.Title} {
		if strings.Contains(strings.ToLower(name), "docket") {
			t.Fatalf("fixture name %q contains docket; the walk would test the fixture, not the product", name)
		}
	}

	// Init and config.
	if res := r.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
		t.Fatalf("init --private = %q (%s), want applied", res.Result, res.HumanText())
	}
	gate := passingGateScript(t)
	if ct := r.runConfigureTestsWith(t, ConfigureTestsOptions{Command: &gate}); ct.Result != ResultApplied {
		t.Fatalf("configure-tests = %q (%s), want applied", ct.Result, ct.HumanText())
	}
	originBefore := localHeads(t, r.origin)

	lay := resolvedPrivateLayout(t, r.invocation)
	if want := privateLayoutOf(t, r.invocation, data).DefaultBareRemote; lay.DefaultBareRemote != want {
		t.Fatalf("resolved store %q, want %q", lay.DefaultBareRemote, want)
	}
	if url := runGit(t, r.invocation, "config", "--get", "remote.dckt.url"); url != lay.DefaultBareRemote {
		t.Fatalf("dckt remote URL = %q, want the store %q", url, lay.DefaultBareRemote)
	}
	repo := &gitRepo{root: r.root, origin: r.origin, meta: lay.DefaultBareRemote, writer: r.writer, invocation: r.invocation}

	// Create and groom to build-ready with a spec.
	node := planningDepsFor(t, r.invocation)
	created := ChangeCreate(ctx, node.deps, node.dir, req)
	if created.Result != ResultApplied {
		t.Fatalf("change create = %q (%s, findings %v)", created.Result, created.HumanText(), created.Findings)
	}
	for _, name := range []string{created.Slug, created.Path} {
		if strings.Contains(strings.ToLower(name), "docket") {
			t.Fatalf("fixture name %q contains docket", name)
		}
	}
	groom := ChangeGroom(ctx, node.deps, node.dir, ChangeGroomRequest{
		ChangeID:     created.ID,
		Path:         created.Path,
		Revision:     blobRevisionAt(t, repo.meta, privateFixtureBranch, created.Path),
		Outcome:      GroomSpec,
		SpecMarkdown: "# Widget: design\n\nBuild the widget.\n",
	})
	if groom.Result != ResultApplied || groom.SpecPath == "" {
		t.Fatalf("change groom = %q (%s, findings %v), want applied with a spec", groom.Result, groom.HumanText(), groom.Findings)
	}

	// Drive the whole run against the private store.
	driveClaimToImplemented(t, repo, privateFixtureBranch, ghBin, created.ID, created.Slug, created.Path, groom.SpecPath,
		privateLifecycleChecks(t, created, repo.meta))

	// Origin gained exactly one branch, and it is the feature branch.
	originAfter := localHeads(t, r.origin)
	var gained []string
	for _, h := range originAfter {
		if !contains(originBefore, h) {
			gained = append(gained, h)
		}
	}
	if len(gained) != 1 || !strings.HasPrefix(gained[0], "refs/heads/feat/") || len(originAfter) != len(originBefore)+1 {
		t.Errorf("origin heads %v -> %v; want exactly one new refs/heads/feat/... and nothing else", originBefore, originAfter)
	}

	// The working tree is clean; the only ignored entry is the feature worktree
	// folder.
	for _, line := range strings.Split(runGit(t, r.invocation, "status", "--porcelain", "--ignored"), "\n") {
		if line != "!! .worktrees/" {
			t.Errorf("git status --porcelain --ignored line %q; want only %q", line, "!! .worktrees/")
		}
	}

	branches := strings.Fields(runGit(t, r.invocation, "branch", "--format=%(refname:short)"))
	if !contains(branches, privateFixtureBranch) || contains(branches, "docket") {
		t.Errorf("local branches = %v; want %s and no docket", branches, privateFixtureBranch)
	}
	if remotes := strings.Fields(runGit(t, r.invocation, "remote")); !contains(remotes, privateFixtureBranch) {
		t.Errorf("remotes = %v; want a %s remote", remotes, privateFixtureBranch)
	}

	assertNoDocketPathUnder(t, r.invocation)

	if got := os.Getenv("DOCKET_SCRIPTS_DIR"); got != "" {
		t.Errorf("the workflow relied on a legacy Bash facade: DOCKET_SCRIPTS_DIR = %q", got)
	}
}
