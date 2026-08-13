package config

import (
	"os"
	"path/filepath"
	"testing"
)

// pinEnv makes the developer's real global config unreachable. Every test in
// this file calls it FIRST, before touching the adapter.
func pinEnv(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)
	return tmp
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestLoadAllThreeLayers(t *testing.T) {
	xdg := pinEnv(t)
	repo := t.TempDir()

	globalPath := filepath.Join(xdg, "docket", "config.yml")
	writeFile(t, globalPath, "learnings: {enabled: true}\n")
	writeFile(t, filepath.Join(repo, ".docket.yml"), "metadata_branch: docket\n")
	writeFile(t, filepath.Join(repo, ".docket.local.yml"), "finalize: {gate: off}\n")

	got, err := LoadFilesystemSources(FSOptions{RepoDir: repo})
	if err != nil {
		t.Fatalf("LoadFilesystemSources: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 sources, got %d: %+v", len(got), got)
	}
	want := []struct {
		layer LayerKind
		name  string
		data  string
	}{
		{LayerGlobal, globalPath, "learnings: {enabled: true}\n"},
		{LayerRepository, ".docket.yml", "metadata_branch: docket\n"},
		{LayerRepositoryLocal, ".docket.local.yml", "finalize: {gate: off}\n"},
	}
	for i, w := range want {
		if got[i].Layer != w.layer {
			t.Errorf("source %d layer = %q, want %q", i, got[i].Layer, w.layer)
		}
		if got[i].Name != w.name {
			t.Errorf("source %d name = %q, want %q", i, got[i].Name, w.name)
		}
		if string(got[i].Data) != w.data {
			t.Errorf("source %d data = %q, want %q", i, got[i].Data, w.data)
		}
	}
}

func TestLoadMissingFilesAreAbsent(t *testing.T) {
	pinEnv(t)
	repo := t.TempDir()

	got, err := LoadFilesystemSources(FSOptions{RepoDir: repo})
	if err != nil {
		t.Fatalf("LoadFilesystemSources: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want 0 sources, got %d: %+v", len(got), got)
	}
}

func TestLoadGlobalPathOverride(t *testing.T) {
	xdg := pinEnv(t)
	repo := t.TempDir()

	// An XDG-default global exists and must be ignored in favor of the override.
	writeFile(t, filepath.Join(xdg, "docket", "config.yml"), "xdg: true\n")
	override := filepath.Join(t.TempDir(), "elsewhere.yml")
	writeFile(t, override, "override: true\n")

	got, err := LoadFilesystemSources(FSOptions{RepoDir: repo, GlobalPath: override})
	if err != nil {
		t.Fatalf("LoadFilesystemSources: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 source, got %d: %+v", len(got), got)
	}
	if got[0].Layer != LayerGlobal || got[0].Name != override {
		t.Fatalf("got %+v, want global layer named %q", got[0], override)
	}
	if string(got[0].Data) != "override: true\n" {
		t.Fatalf("data = %q, want the override file's bytes", got[0].Data)
	}
}

func TestLoadXDGDefaultPath(t *testing.T) {
	xdg := pinEnv(t)
	repo := t.TempDir()

	globalPath := filepath.Join(xdg, "docket", "config.yml")
	writeFile(t, globalPath, "from: xdg\n")

	got, err := LoadFilesystemSources(FSOptions{RepoDir: repo})
	if err != nil {
		t.Fatalf("LoadFilesystemSources: %v", err)
	}
	if len(got) != 1 || got[0].Name != globalPath || string(got[0].Data) != "from: xdg\n" {
		t.Fatalf("got %+v, want the XDG-default global at %q", got, globalPath)
	}
}

func TestLoadHomeFallback(t *testing.T) {
	home := pinEnv(t)
	t.Setenv("XDG_CONFIG_HOME", "")
	repo := t.TempDir()

	globalPath := filepath.Join(home, ".config", "docket", "config.yml")
	writeFile(t, globalPath, "from: home\n")

	got, err := LoadFilesystemSources(FSOptions{RepoDir: repo})
	if err != nil {
		t.Fatalf("LoadFilesystemSources: %v", err)
	}
	if len(got) != 1 || got[0].Name != globalPath || string(got[0].Data) != "from: home\n" {
		t.Fatalf("got %+v, want the HOME-fallback global at %q", got, globalPath)
	}
}

func TestLoadRepoDirRequired(t *testing.T) {
	pinEnv(t)

	if _, err := LoadFilesystemSources(FSOptions{}); err == nil {
		t.Fatal("want an error for an empty RepoDir, got nil")
	}
}

func TestLoadRelativeRepoDirCleaned(t *testing.T) {
	pinEnv(t)
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, ".docket.yml"), "metadata_branch: main\n")

	t.Chdir(repo)

	got, err := LoadFilesystemSources(FSOptions{RepoDir: "."})
	if err != nil {
		t.Fatalf("LoadFilesystemSources: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 source, got %d: %+v", len(got), got)
	}
	if got[0].Layer != LayerRepository || got[0].Name != ".docket.yml" {
		t.Fatalf("got %+v, want the repository layer named .docket.yml", got[0])
	}
	if string(got[0].Data) != "metadata_branch: main\n" {
		t.Fatalf("data = %q, want the repo file's bytes", got[0].Data)
	}
}
