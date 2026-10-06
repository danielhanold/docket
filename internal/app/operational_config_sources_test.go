package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/testsupport"
)

// privateOperationalRepo is a primary worktree whose common dir carries the
// private config. The global layer is pinned to an empty temp home.
func privateOperationalRepo(t *testing.T) (gitcli.Repository, layout.Layout) {
	t.Helper()
	home := testsupport.TempDir(t)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("HOME", home)
	primary := testsupport.TempDir(t)
	common := filepath.Join(primary, ".git")
	if err := os.MkdirAll(filepath.Join(common, layout.PrivateName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(layout.PrivateConfigPath(common), []byte("changes_dir: local-private\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := gitcli.Repository{PrimaryWorktree: primary, CommonDir: common}
	return repo, layout.PrivateLayout(common, primary, filepath.Join(home, "data"), "owner-repo")
}

// TestOperationalConfigSourcesPrivateReadsPrivateConfig: a private layout
// reads .git/dckt/config.yml as the repository layer and ignores the pinned
// .docket.yml bytes it is handed (the caller skips that read in private mode).
func TestOperationalConfigSourcesPrivateReadsPrivateConfig(t *testing.T) {
	repo, lay := privateOperationalRepo(t)
	sources, err := operationalConfigSources(repo, lay, []byte("changes_dir: committed\n"))
	if err != nil {
		t.Fatalf("operationalConfigSources: %v", err)
	}
	if len(sources) != 1 || sources[0].Layer != config.LayerRepository || sources[0].Name != layout.PrivateConfigDisplay {
		t.Fatalf("sources = %+v, want exactly the private repository layer", sources)
	}
	if string(sources[0].Data) != "changes_dir: local-private\n" {
		t.Errorf("repository layer data = %q, want the private config", sources[0].Data)
	}
}

// TestOperationalConfigSourcesPrivateLocalConflictIsInvalidConfiguration: a
// .docket.local.yml beside a private config is a configuration refusal.
func TestOperationalConfigSourcesPrivateLocalConflictIsInvalidConfiguration(t *testing.T) {
	repo, lay := privateOperationalRepo(t)
	if err := os.WriteFile(filepath.Join(repo.PrimaryWorktree, ".docket.local.yml"), []byte("finalize: {gate: off}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := operationalConfigSources(repo, lay, nil)
	var eic *errInvalidConfiguration
	if !errors.As(err, &eic) || !errors.Is(err, ErrStatusInvalidInput) {
		t.Fatalf("err = %v, want an errInvalidConfiguration classified as invalid input", err)
	}
	var conflict *config.ConflictingLocalConfigError
	if !errors.As(err, &conflict) {
		t.Errorf("err = %v, want it to wrap *config.ConflictingLocalConfigError", err)
	}
}
