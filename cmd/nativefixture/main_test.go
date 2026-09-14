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

func TestPrepareRefusesExistingDestinationBeforeCandidateActions(t *testing.T) {
	root := testsupport.TempDir(t)
	dest := filepath.Join(root, "existing")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	err := prepare(options{Source: filepath.Join(root, "source"), Binary: filepath.Join(root, "docket"), Destination: dest, Pins: filepath.Join(root, "pins.yml")})
	if err == nil || !strings.Contains(err.Error(), "destination must not exist") {
		t.Fatalf("prepare error=%v", err)
	}
}

func TestPrepareBuildsConfiguredBuildReadyFixtureAndCompleteManifest(t *testing.T) {
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
	runTest := func(dir, name string, args ...string) string {
		t.Helper()
		c := exec.Command(name, args...)
		c.Dir = dir
		b, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v: %s", name, args, err, b)
		}
		return strings.TrimSpace(string(b))
	}
	runTest(source, "git", "init", "-b", "main")
	runTest(source, "git", "config", "user.name", "Fixture Test")
	runTest(source, "git", "config", "user.email", "fixture@example.invalid")
	runTest(source, "git", "add", "-A")
	runTest(source, "git", "commit", "-m", "candidate")
	head := runTest(source, "git", "rev-parse", "HEAD")
	binary := filepath.Join(root, "docket")
	ldflags := "-X github.com/danielhanold/docket/internal/buildinfo.Version=test -X github.com/danielhanold/docket/internal/buildinfo.Commit=" + head + " -X github.com/danielhanold/docket/internal/buildinfo.BuildDate=2026-09-14"
	runTest(source, "go", "build", "-ldflags", ldflags, "-o", binary, "./cmd/docket")
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
	body, err := os.ReadFile(filepath.Join(destination, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got manifest
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.SourceCommit != head || got.PrimaryHEAD == "" || got.MetadataRevision == "" || !got.BuildReady || !got.BaselinePassed {
		t.Fatalf("incomplete manifest: %+v", got)
	}
	if got.BuildTestCommand != "go test ./..." || got.FinalizeTestCommand != "go test ./..." || got.BaselineCommand != "go test ./..." || got.PinsSHA256 == "" {
		t.Fatalf("test policy/provenance incomplete: %+v", got)
	}
	for _, rel := range []string{".docket.yml", "AGENTS.md", "go.mod", "fixture/value_test.go", "../LAUNCH.md"} {
		if got.Files[rel] == "" {
			t.Errorf("manifest omits %s", rel)
		}
	}
	if dirty := runTest(filepath.Join(destination, "primary"), "git", "status", "--porcelain=v2"); dirty != "" {
		t.Fatalf("fixture primary dirty: %s", dirty)
	}
}
