//go:build integration

package app

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/reposetup"
)

// This file is the real-Git `repository set-visibility` shard (prefix
// TestIntegrationRepoVisibility): the preview, and every refusal that stops a
// switch before it writes.

// runSetVisibility runs RunRepositorySetVisibility against the invocation clone
// with the user's machine roots pinned to temp dirs.
func (r *initRepo) runSetVisibility(t *testing.T, o SetVisibilityOptions) RepositorySetVisibilityResult {
	t.Helper()
	pinInitUserRoots(t)
	client := newGitClient(t)
	return RunRepositorySetVisibility(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation}, o)
}

// newSharedVisibilityRepo is a shared repository right after init.
func newSharedVisibilityRepo(t *testing.T) *initRepo {
	t.Helper()
	r := newInitRepo(t, defaultSetupYML, nil)
	if res := r.runInit(t); res.Result != ResultApplied {
		t.Fatalf("init = %q (%s), want applied", res.Result, res.HumanText())
	}
	return r
}

// visibilitySnapshot captures what a preview must never change: the working
// tree status, every ref on origin, the top-level .git listing, and whether the
// state folders exist.
func visibilitySnapshot(t *testing.T, r *initRepo) string {
	t.Helper()
	var b strings.Builder
	b.WriteString(runGit(t, r.invocation, "status", "--porcelain", "--untracked-files=all") + "\n--\n")
	b.WriteString(runGit(t, r.origin, "for-each-ref", "--format=%(refname) %(objectname)") + "\n--\n")
	entries, err := os.ReadDir(r.gitDir(t))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if e.Name() == "FETCH_HEAD" {
			continue // a read-only probe fetches; FETCH_HEAD is git's scratch record of that
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	b.WriteString(strings.Join(names, " "))
	return b.String()
}

func requireVisibilityRefusal(t *testing.T, res RepositorySetVisibilityResult, want ...string) {
	t.Helper()
	if res.Result != ResultInvalidState || res.ConfirmationRequired() {
		t.Fatalf("Result = %q state %q (%s), want an invalid-state refusal", res.Result, res.RepositoryState, res.HumanText())
	}
	for _, w := range want {
		if !strings.Contains(res.HumanText(), w) {
			t.Errorf("refusal text lacks %q:\n%s", w, res.HumanText())
		}
	}
}

// TestIntegrationRepoVisibilityPreviewWritesNothing proves a private preview
// lists its phases and pin while the working tree, origin's refs, the .git
// listing, and the private store stay untouched.
func TestIntegrationRepoVisibilityPreviewWritesNothing(t *testing.T) {
	r := newSharedVisibilityRepo(t)
	before := visibilitySnapshot(t, r)
	res := r.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	after := visibilitySnapshot(t, r)

	if res.Result != ResultInvalidState || !res.ConfirmationRequired() {
		t.Fatalf("Result = %q state %q (%s), want a confirmation-required preview", res.Result, res.RepositoryState, res.HumanText())
	}
	if before != after {
		t.Errorf("the preview changed the repository:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if res.Current != "shared" || res.Target != "private" {
		t.Errorf("current/target = %q/%q, want shared/private", res.Current, res.Target)
	}
	if !strings.HasPrefix(res.SourceRevision, "origin-docket=") || !strings.Contains(res.SourceRevision, " dckt=absent ") {
		t.Errorf("SourceRevision = %q, want the composite pin with no dckt branch", res.SourceRevision)
	}
	pending := 0
	for _, p := range res.Phases {
		if p.Status == visibilityPhasePending {
			pending++
		}
		if !strings.Contains(res.HumanText(), "] "+p.Name+": ") {
			t.Errorf("preview text does not list phase %s:\n%s", p.Name, res.HumanText())
		}
	}
	if pending == 0 {
		t.Errorf("phases = %+v, want pending phases", res.Phases)
	}
	priv := expectedPrivateLayout(t, r.invocation, os.Getenv("XDG_DATA_HOME"))
	if _, err := os.Stat(priv.StoreDir); !os.IsNotExist(err) {
		t.Errorf("the preview touched the private store %s (stat err %v)", priv.StoreDir, err)
	}
}

// TestIntegrationRepoVisibilityRefusesLiveRun proves an active run refuses the
// switch, naming the cancel command for its key.
func TestIntegrationRepoVisibilityRefusesLiveRun(t *testing.T) {
	r := newSharedVisibilityRepo(t)
	key, err := MintRunTrackerRecord(r.invocation, sampleRunTrackerRecord())
	if err != nil {
		t.Fatalf("MintRunTrackerRecord: %v", err)
	}
	if _, err := MintRunRecord(r.invocation, key, "375"); err != nil {
		t.Fatalf("MintRunRecord: %v", err)
	}
	res := r.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	requireVisibilityRefusal(t, res, "docket run cancel --key "+key)
}

// TestIntegrationRepoVisibilityRefusesBusyGateLock proves a held gate lock in
// the state folder refuses the switch.
func TestIntegrationRepoVisibilityRefusesBusyGateLock(t *testing.T) {
	r := newSharedVisibilityRepo(t)
	dir := filepath.Join(r.gitDir(t), layout.SharedName, "worktree-locks", "x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, busy, err := process.TryExclusiveLock(filepath.Join(dir, "busy.lock"))
	if err != nil || busy {
		t.Fatalf("TryExclusiveLock: busy=%v err=%v", busy, err)
	}
	defer f.Close()
	res := r.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	requireVisibilityRefusal(t, res, "busy.lock")
}

// TestIntegrationRepoVisibilityRefusesDirtyMetadataWorktree proves an
// uncommitted change in .docket refuses the switch.
func TestIntegrationRepoVisibilityRefusesDirtyMetadataWorktree(t *testing.T) {
	r := newSharedVisibilityRepo(t)
	writeRepoFile(t, filepath.Join(r.invocation, ".docket"), "scratch.md", "unsaved\n")
	res := r.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	requireVisibilityRefusal(t, res, "commit or discard the changes in")
}

// TestIntegrationRepoVisibilityRefusesUnpublishedMetadataCommit proves a
// metadata commit origin lacks refuses the switch.
func TestIntegrationRepoVisibilityRefusesUnpublishedMetadataCommit(t *testing.T) {
	r := newSharedVisibilityRepo(t)
	dotDocket := filepath.Join(r.invocation, ".docket")
	gitIdentity(t, dotDocket)
	writeRepoFile(t, dotDocket, "local.md", "not pushed\n")
	runGit(t, dotDocket, "add", "--", "local.md")
	runGit(t, dotDocket, "commit", "-q", "-m", "local only")
	res := r.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	requireVisibilityRefusal(t, res, "docket repository prepare")
}

// TestIntegrationRepoVisibilityRefusesFreshAndLegacy proves a repository that
// was never set up names init, and a legacy one names migrate.
func TestIntegrationRepoVisibilityRefusesFreshAndLegacy(t *testing.T) {
	fresh := newInitRepo(t, defaultSetupYML, nil)
	requireVisibilityRefusal(t, fresh.runSetVisibility(t, SetVisibilityOptions{Target: "private"}), "docket repository init")

	legacy := newInitRepo(t, defaultSetupYML, cleanLegacyFiles())
	res := legacy.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	requireVisibilityRefusal(t, res, "docket repository migrate")
	if res.RepositoryState != string(reposetup.StateLegacy) {
		t.Errorf("RepositoryState = %q, want legacy", res.RepositoryState)
	}
}

// TestIntegrationRepoVisibilityWarnsAbsoluteLinksWithoutRefusing proves a
// metadata file whose prose links absolutely into the docket branch is named in
// a warning that names `docket repository repair`, and the preview still stands.
func TestIntegrationRepoVisibilityWarnsAbsoluteLinksWithoutRefusing(t *testing.T) {
	r := newSharedVisibilityRepo(t)
	runGit(t, r.invocation, "remote", "set-url", "origin", "https://github.com/acme/app.git")
	runGit(t, r.invocation, "config", "url."+r.origin+".insteadOf", "https://github.com/acme/app.git")

	const spec = "docs/superpowers/specs/2026-10-01-example-design.md"
	dotDocket := filepath.Join(r.invocation, ".docket")
	gitIdentity(t, dotDocket)
	writeRepoFile(t, dotDocket, spec, "# Example\n\nSee https://github.com/acme/app/blob/docket/x.md for context.\n")
	runGit(t, dotDocket, "add", "--", spec)
	runGit(t, dotDocket, "commit", "-q", "-m", "a spec with an absolute link")
	runGit(t, dotDocket, "push", "-q", "origin", layout.SharedName)

	res := r.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	if res.Result != ResultInvalidState || !res.ConfirmationRequired() {
		t.Fatalf("Result = %q state %q (%s), want a confirmation-required preview", res.Result, res.RepositoryState, res.HumanText())
	}
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w, spec) && strings.Contains(w, "docket repository repair") {
			found = true
		}
	}
	if !found {
		t.Errorf("warnings %q do not name %s and `docket repository repair`", res.Warnings, spec)
	}
}
