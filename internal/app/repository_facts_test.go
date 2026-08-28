package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/reposetup"
)

// fakeSetupProber is a configurable stand-in for the gitcli methods the fact
// gatherer calls, so a test can force a per-probe failure and prove it maps to
// PresenceUnknown, never a silent absence.
type fakeSetupProber struct {
	repo        gitcli.Repository
	discoverErr error
	defaultRef  gitcli.RefName
	defaultErr  error
	fetchErr    error
	fetchRev    gitcli.Revision
	probeErr    error
	probe       gitcli.RemoteRef
	changed     []gitcli.PathChange
	changedErr  error
	head        gitcli.ObjectID
	resolveErr  error
	worktrees   []gitcli.WorktreeInfo
}

func (f *fakeSetupProber) Discover(ctx context.Context, opts gitcli.DiscoverOptions) (gitcli.Repository, error) {
	return f.repo, f.discoverErr
}
func (f *fakeSetupProber) RemoteDefaultBranch(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName) (gitcli.RefName, error) {
	return f.defaultRef, f.defaultErr
}
func (f *fakeSetupProber) FetchBranch(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, branch gitcli.RefName) (gitcli.Revision, error) {
	return f.fetchRev, f.fetchErr
}
func (f *fakeSetupProber) ProbeRemoteBranch(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, ref gitcli.RefName) (gitcli.RemoteRef, error) {
	return f.probe, f.probeErr
}
func (f *fakeSetupProber) OpenObjectSource(ctx context.Context, repo gitcli.Repository, rev gitcli.Revision) (gitcli.ObjectSource, error) {
	return nil, errors.New("fake: OpenObjectSource unreachable in this test")
}
func (f *fakeSetupProber) ChangedPaths(ctx context.Context, dir string) ([]gitcli.PathChange, error) {
	return f.changed, f.changedErr
}
func (f *fakeSetupProber) ResolveRef(ctx context.Context, repo gitcli.Repository, ref gitcli.RefName) (gitcli.ObjectID, error) {
	return f.head, f.resolveErr
}
func (f *fakeSetupProber) ListWorktrees(ctx context.Context, repo gitcli.Repository) ([]gitcli.WorktreeInfo, error) {
	return f.worktrees, nil
}

// setupProberRepoDir writes an isolated repository fixture: an empty global
// config layer and a repository .docket.yml with the given body, returning the
// worktree root.
func setupProberRepoDir(t *testing.T, docketYML string) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".docket.yml"), []byte(docketYML), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestGatherSetupFactsProbeErrorMapsToUnknown proves the three-valued contract:
// an errored integration fetch and an errored metadata probe each land as
// PresenceUnknown with the error retained in diagnostics — never as a clean
// absence that could authorize a create — and the classifier reports `unknown`.
func TestGatherSetupFactsProbeErrorMapsToUnknown(t *testing.T) {
	root := setupProberRepoDir(t, "metadata_branch: docket\nintegration_branch: main\n")
	p := &fakeSetupProber{
		repo:       gitcli.Repository{PrimaryWorktree: root},
		defaultRef: gitcli.RefName("refs/heads/main"),
		fetchErr:   errors.New("boom: unreachable remote"),
		probeErr:   errors.New("boom: ls-remote failed"),
	}

	f, sc, err := gatherSetupFacts(context.Background(), p, root, true)
	if err != nil {
		t.Fatalf("gatherSetupFacts: %v", err)
	}
	if f.RemoteIntegration.Presence != reposetup.PresenceUnknown {
		t.Errorf("RemoteIntegration.Presence = %v, want Unknown (errored fetch is never Absent)", f.RemoteIntegration.Presence)
	}
	if f.RemoteMetadata.Presence != reposetup.PresenceUnknown {
		t.Errorf("RemoteMetadata.Presence = %v, want Unknown (errored probe is never Absent)", f.RemoteMetadata.Presence)
	}
	if !hasDiag(sc.diagnostics, "remote-integration-branch") || !hasDiag(sc.diagnostics, "remote-metadata-branch") {
		t.Errorf("diagnostics = %+v, want the integration and metadata probe errors retained", sc.diagnostics)
	}
	if got := reposetup.Classify(f).State; got != reposetup.StateUnknown {
		t.Errorf("Classify = %q, want unknown (an errored probe must never classify fresh/absent)", got)
	}
}

// TestGatherSetupFactsRepositoryHarnessesAuthorize proves the write-authority
// gate: an explicit repository-layer agent_harnesses declaration authorizes
// surfaces.
func TestGatherSetupFactsRepositoryHarnessesAuthorize(t *testing.T) {
	root := setupProberRepoDir(t, "metadata_branch: docket\nintegration_branch: main\nagent_harnesses: [claude]\n")
	p := &fakeSetupProber{repo: gitcli.Repository{PrimaryWorktree: root}, defaultRef: gitcli.RefName("refs/heads/main")}
	f, _, err := gatherSetupFacts(context.Background(), p, root, true)
	if err != nil {
		t.Fatalf("gatherSetupFacts: %v", err)
	}
	if !f.SurfacesAuthorized {
		t.Errorf("SurfacesAuthorized = false, want true for an explicit repository-layer agent_harnesses")
	}
}

// TestGatherSetupFactsGlobalHarnessesDoNotAuthorize proves a global-layer
// agent_harnesses declaration resolves but never authorizes repository surfaces.
func TestGatherSetupFactsGlobalHarnessesDoNotAuthorize(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if err := os.MkdirAll(filepath.Join(xdg, "docket"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdg, "docket", "config.yml"), []byte("agent_harnesses: [claude]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".docket.yml"), []byte("metadata_branch: docket\nintegration_branch: main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &fakeSetupProber{repo: gitcli.Repository{PrimaryWorktree: root}, defaultRef: gitcli.RefName("refs/heads/main")}
	f, _, err := gatherSetupFacts(context.Background(), p, root, true)
	if err != nil {
		t.Fatalf("gatherSetupFacts: %v", err)
	}
	if f.SurfacesAuthorized {
		t.Errorf("SurfacesAuthorized = true, want false for a global-layer agent_harnesses")
	}
}

// TestGatherSetupFactsAbsentHarnessesDoNotAuthorize proves the touch-nothing
// default: no agent_harnesses declaration never authorizes surfaces.
func TestGatherSetupFactsAbsentHarnessesDoNotAuthorize(t *testing.T) {
	root := setupProberRepoDir(t, "metadata_branch: docket\nintegration_branch: main\n")
	p := &fakeSetupProber{repo: gitcli.Repository{PrimaryWorktree: root}, defaultRef: gitcli.RefName("refs/heads/main")}
	f, _, err := gatherSetupFacts(context.Background(), p, root, true)
	if err != nil {
		t.Fatalf("gatherSetupFacts: %v", err)
	}
	if f.SurfacesAuthorized {
		t.Errorf("SurfacesAuthorized = true, want false when agent_harnesses is unset")
	}
}

func hasDiag(diags []setupDiag, probe string) bool {
	for _, d := range diags {
		if d.Probe == probe && d.Err != nil {
			return true
		}
	}
	return false
}
