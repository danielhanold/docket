package config

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// privateFixture is a temp working-tree root whose .git common dir carries the
// dckt/ folder that marks a private repository.
func privateFixture(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, ".git", "dckt", "config.yml"),
		"visibility: private\nchanges_dir: local-private\n")
	return repo
}

func TestLoadFilesystemSourcesPrivateReadsDcktConfigNotDocketYML(t *testing.T) {
	pinEnv(t)
	repo := privateFixture(t)
	writeFile(t, filepath.Join(repo, ".docket.yml"), "changes_dir: committed\n")

	got, err := LoadFilesystemSources(FSOptions{RepoDir: repo})
	if err != nil {
		t.Fatalf("LoadFilesystemSources: %v", err)
	}
	if len(got) != 1 || got[0].Layer != LayerRepository || got[0].Name != ".git/dckt/config.yml" {
		t.Fatalf("sources = %+v, want exactly [{repository .git/dckt/config.yml}]", got)
	}

	snap, _, err := Resolve(got, mainCtx)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v := snap.Effective.ChangesDir.Value; v != "local-private" {
		t.Errorf("changes_dir = %q, want %q (an identity key honored from the private config)", v, "local-private")
	}
	if v := snap.Effective.Visibility.Value; v != "private" {
		t.Errorf("visibility = %q, want %q", v, "private")
	}
}

func TestLoadFilesystemSourcesPrivateRefusesBothLocalFiles(t *testing.T) {
	pinEnv(t)
	repo := privateFixture(t)
	writeFile(t, filepath.Join(repo, ".docket.local.yml"), "finalize: {gate: off}\n")

	_, err := LoadFilesystemSources(FSOptions{RepoDir: repo})
	var conflict *ConflictingLocalConfigError
	if !errors.As(err, &conflict) {
		t.Fatalf("err = %v, want a *ConflictingLocalConfigError", err)
	}
	msg := err.Error()
	for _, want := range []string{".docket.local.yml", ".git/dckt/config.yml"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not name %s", msg, want)
		}
	}
}

func TestLoadFilesystemSourcesSharedUnchanged(t *testing.T) {
	pinEnv(t)
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/main\n")
	writeFile(t, filepath.Join(repo, ".docket.yml"), "changes_dir: committed\n")

	got, err := LoadFilesystemSources(FSOptions{RepoDir: repo})
	if err != nil {
		t.Fatalf("LoadFilesystemSources: %v", err)
	}
	if len(got) != 1 || got[0].Layer != LayerRepository || got[0].Name != ".docket.yml" {
		t.Fatalf("sources = %+v, want exactly [{repository .docket.yml}]", got)
	}
}

func TestGlobalVisibilityIsOrdinary(t *testing.T) {
	res := mustResolve(t, []Source{srcG("visibility: private\n")}, mainCtx)
	if v := res.effective.Visibility; v.Value != "private" || v.Provenance.Layer != LayerGlobal {
		t.Errorf("visibility = %+v, want private from %q", v, LayerGlobal)
	}
	for _, d := range res.diags {
		if d.Severity == SeverityWarning || d.Severity == SeverityError {
			t.Errorf("unexpected diagnostic %s/%s/%s", d.Severity, d.Code, d.Path)
		}
	}
}

func TestSharedSettingRemedyNamesPrivateConfig(t *testing.T) {
	private := Source{Layer: LayerRepository, Name: ".git/dckt/config.yml", Data: []byte("visibility: private\n")}
	res := mustResolve(t, []Source{srcG("integration_branch: trunk\n"), private}, mainCtx)
	guarded := diagsWithCode(res, CodeSharedSettingIgnored)
	if len(guarded) != 1 {
		t.Fatalf("guarded = %v, want one shared-setting-ignored warning", diagSummary(res))
	}
	if r := guarded[0].Remedy; !strings.Contains(r, ".git/dckt/config.yml") || strings.Contains(r, ".docket.yml") {
		t.Errorf("remedy = %q, want it to name .git/dckt/config.yml and not .docket.yml", r)
	}
}
