//go:build integration

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/layout"
)

// docket open's local opens over a real private repository (prefix
// TestIntegrationOpenArtifact): a private repository always opens from the
// metadata checkout, which Open prepares first and never blocks on. Not
// parallel: the helpers use t.Setenv.

// openPrivateFixture is one private repository with a single groomed change:
// the invocation clone's real planning node, its resolved private layout, and
// the change id.
type openPrivateFixture struct {
	node realNode
	lay  layout.Layout
	id   int
}

// newOpenPrivateFixture runs init --private, then creates and grooms one
// change with a spec (the groom commit, the metadata tip, adds the spec).
func newOpenPrivateFixture(t *testing.T) *openPrivateFixture {
	t.Helper()
	r, _ := newPrivateInitRepo(t, nil)
	if res := r.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
		t.Fatalf("init --private = %q (%s)", res.Result, res.HumanText())
	}
	node := planningDepsFor(t, r.invocation)
	id, _ := seedBuildReadyPrivate(t, node)
	return &openPrivateFixture{node: node, lay: resolvedPrivateLayout(t, r.invocation), id: id}
}

// open runs `docket open spec <id> --print` through the production seams
// (real reader, checkout probe, and prepare) with a fake opener.
func (f *openPrivateFixture) open(t *testing.T) OpenResult {
	t.Helper()
	client := f.node.deps.Client
	deps := OpenDeps{
		Reader:   f.node.deps.Reader,
		Checkout: client.WorktreeCheckoutState,
		Prepare: func(ctx context.Context, dir string) RepositoryPrepareResult {
			return RunRepositoryPrepare(ctx, SetupDeps{Git: client, RepoDir: dir}, PrepareOptions{})
		},
		Opener: &fakeOpener{},
	}
	return Open(context.Background(), deps, OpenOptions{RepoDir: f.node.dir, What: "spec", ID: f.id, Print: true})
}

// rewind resets the checkout to the commit before the metadata tip (before
// the spec existed) and returns the tip. Both preconditions are asserted so
// neither can pass vacuously.
func (f *openPrivateFixture) rewind(t *testing.T) string {
	t.Helper()
	wt := f.lay.MetadataWorktree
	runGit(t, wt, "fetch", "-q", f.lay.DefaultBareRemote, f.lay.MetadataBranch)
	tip := runGit(t, wt, "rev-parse", "FETCH_HEAD")
	if !strings.Contains(runGit(t, wt, "ls-tree", "-r", "--name-only", tip), "-design.md") {
		t.Fatalf("precondition: the metadata tip %s carries no spec", tip)
	}
	runGit(t, wt, "reset", "-q", "--hard", tip+"~1")
	if strings.Contains(runGit(t, wt, "ls-files"), "-design.md") {
		t.Fatalf("precondition: the rewound checkout still has the spec")
	}
	return tip
}

// dirty leaves an untracked file in the metadata checkout, which prepare
// refuses to sync over.
func (f *openPrivateFixture) dirty(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.lay.MetadataWorktree, "scratch.txt"), []byte("edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// One fixture, three phases: a stale checkout missing the file fails naming
// the path and prepare's finding; once clean, a behind checkout is
// fast-forwarded before the open; a dirty one opens with the stale note.
func TestIntegrationOpenArtifactLocalOpenSyncsFirst(t *testing.T) {
	f := newOpenPrivateFixture(t)
	wt := f.lay.MetadataWorktree
	tip := f.rewind(t)
	f.dirty(t)
	res := f.open(t)
	if res.Result != ResultInvalidState || res.Reason != ReasonOpenFileMissing || !strings.Contains(res.Message, wt) ||
		!strings.Contains(res.Message, "-design.md") || !strings.Contains(res.Message, "metadata-worktree-dirty") {
		t.Fatalf("missing: (%s, %s, %q), want file-missing naming the spec path and the finding", res.Result, res.Reason, res.Message)
	}

	if err := os.Remove(filepath.Join(wt, "scratch.txt")); err != nil {
		t.Fatal(err)
	}
	res = f.open(t)
	if res.Result != ResultApplied || res.TargetKind != OpenTargetKindFile || len(res.Notes) != 0 ||
		!strings.HasPrefix(res.Target, wt+string(filepath.Separator)) || !strings.HasSuffix(res.Target, "-design.md") {
		t.Fatalf("behind: (%s, %q, %q, %q), want the spec inside %s, no note", res.Result, res.Target, res.Notes, res.Message, wt)
	}
	if _, err := os.Stat(res.Target); err != nil {
		t.Errorf("target missing: %v", err)
	}
	if head := runGit(t, wt, "rev-parse", "HEAD"); head != tip {
		t.Errorf("checkout HEAD = %s, want fast-forwarded to %s", head, tip)
	}

	f.dirty(t)
	res = f.open(t)
	want := "metadata checkout not synced (metadata-worktree-dirty) — file may be stale"
	if res.Result != ResultApplied || len(res.Notes) != 1 || res.Notes[0] != want {
		t.Fatalf("dirty: (%s, %q, %q), want applied with %q", res.Result, res.Notes, res.Message, want)
	}
}
