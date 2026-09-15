//go:build integration

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/testsupport"
)

func TestIntegrationNativeFixtureRendersWithCandidateImplementation(t *testing.T) {
	root := testsupport.TempDir(t)
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = repoRoot
	listed, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range strings.Split(string(listed), "\x00") {
		if rel == "" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(source, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(dir, name string, args ...string) string {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		body, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v: %s", name, args, err, body)
		}
		return strings.TrimSpace(string(body))
	}
	run(source, "git", "init", "-b", "main")
	run(source, "git", "config", "user.name", "Fixture Test")
	run(source, "git", "config", "user.email", "fixture@example.invalid")
	const marker = "review-candidate-renderer-marker"
	adapterPath := filepath.Join(source, "internal", "harness", "codex", "codex.go")
	body, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatal(err)
	}
	body = []byte(strings.Replace(string(body), "When your active charter requires another registered role, ", marker+". When your active charter requires another registered role, ", 1))
	if err := os.WriteFile(adapterPath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	run(source, "go", "run", "./cmd/genassets", "-repo", source)
	run(source, "git", "add", "-A")
	run(source, "git", "commit", "-m", "candidate")
	head := run(source, "git", "rev-parse", "HEAD")
	binary := filepath.Join(root, "docket")
	ldflags := "-X github.com/danielhanold/docket/internal/buildinfo.Version=test -X github.com/danielhanold/docket/internal/buildinfo.Commit=" + head + " -X github.com/danielhanold/docket/internal/buildinfo.BuildDate=2026-09-14"
	run(source, "go", "build", "-ldflags", ldflags, "-o", binary, "./cmd/docket")
	catalog, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	agents, err := harness.ParseInventory(catalog)
	if err != nil {
		t.Fatal(err)
	}
	var pins strings.Builder
	pins.WriteString("agents:\n  codex:\n")
	for _, agent := range agents {
		pins.WriteString("    " + agent.ShortName + ": { model: gpt-5.6-terra, effort: low }\n")
	}
	pinsPath := filepath.Join(root, "pins.yml")
	if err := os.WriteFile(pinsPath, []byte(pins.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "fixture")
	if err := prepare(options{Source: source, Binary: binary, Destination: destination, Pins: pinsPath}); err != nil {
		t.Fatal(err)
	}
	manifestBody, err := os.ReadFile(filepath.Join(destination, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got manifest
	if err := json.Unmarshal(manifestBody, &got); err != nil {
		t.Fatal(err)
	}
	if got.SourceCommit != head || got.PrimaryHEAD == "" || got.MetadataRevision == "" || !got.BuildReady || !got.BaselinePassed {
		t.Fatalf("incomplete manifest: %+v", got)
	}
	for _, agent := range agents {
		rel := filepath.ToSlash(filepath.Join(".codex", "agents", agent.Name+".toml"))
		definition, err := os.ReadFile(filepath.Join(destination, "primary", filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if agent.Name == "docket-plan-writer" && !strings.Contains(string(definition), marker) {
			t.Fatal("planner definition did not use the supplied candidate implementation")
		}
	}
}
