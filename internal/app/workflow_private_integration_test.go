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

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/layout"
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
	driveClaimToImplemented(t, repo, privateFixtureBranch, ghBin, created.ID, created.Slug, created.Path, groom.SpecPath)

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
