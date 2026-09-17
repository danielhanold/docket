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

func TestIntegrationNativeFixtureBuildsConfiguredBuildReadyFixtureFromCandidateSource(t *testing.T) {
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
	const sourceMarker = "source-bound-role-definition"
	agentPath := filepath.Join(source, "agents", "docket-plan-writer.md")
	agentBody, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agentPath, append(agentBody, []byte("\nCandidate source marker: "+sourceMarker+".\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	runTest(source, "go", "run", "./cmd/genassets", "-repo", source)
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
	var readiness struct {
		MutationAllowed bool `json:"mutation_allowed"`
	}
	if err := json.Unmarshal(body, &readiness); err != nil {
		t.Fatal(err)
	}
	if !readiness.MutationAllowed {
		t.Fatal("fixture advertised build readiness without certifying mutation eligibility")
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
	for _, agent := range agents {
		rel := filepath.ToSlash(filepath.Join(".codex", "agents", agent.Name+".toml"))
		body, err := os.ReadFile(filepath.Join(destination, "primary", filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read rendered agent %s: %v", rel, err)
		}
		if got.Files[rel] != hash(body) {
			t.Errorf("manifest hash for rendered agent %s = %q, want %q", rel, got.Files[rel], hash(body))
		}
		if agent.Name == "docket-plan-writer" && !strings.Contains(string(body), sourceMarker) {
			t.Errorf("rendered planner definition does not come from the supplied candidate source")
		}
	}
	if dirty := runTest(filepath.Join(destination, "primary"), "git", "status", "--porcelain=v2"); dirty != "" {
		t.Fatalf("fixture primary dirty: %s", dirty)
	}
	// Consume the actual producer manifest, including its non-runtime launch
	// document, against an isolated installation of every pinned runtime file.
	runtimeHome := filepath.Join(root, "runtime-home")
	for rel := range got.Files {
		if !strings.HasPrefix(rel, ".codex/agents/") && !strings.HasPrefix(rel, ".agents/skills/") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(destination, "primary", filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if err := writeFile(filepath.Join(runtimeHome, filepath.FromSlash(rel)), body, 0o644); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(rel, ".agents/skills/") {
			alias := strings.Replace(rel, ".agents/skills/", ".codex/skills/", 1)
			if err := writeFile(filepath.Join(runtimeHome, filepath.FromSlash(alias)), body, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := verifyRuntimePins(filepath.Join(destination, "manifest.json"), runtimeHome); err != nil {
		t.Fatalf("prepared manifest rejected by runtime verifier: %v", err)
	}
	checkNativePlannerEntryDefaultsToStartup(t, root, destination, binary, got)

	primary := filepath.Join(destination, "primary")
	local := filepath.Join(primary, ".docket.local.yml")
	if err := os.WriteFile(local, []byte(pins.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyMutationConfiguration(binary, primary); err == nil || !strings.Contains(err.Error(), "deferred-capability-requested") {
		t.Fatalf("repository-local routing pins must block acceptance: %v", err)
	}
	// The generated role definitions retain the model pins without a routing
	// override in .docket.local.yml. Removing only that override restores eligibility.
	if err := os.Remove(local); err != nil {
		t.Fatal(err)
	}
	if err := verifyMutationConfiguration(binary, primary); err != nil {
		t.Fatal(err)
	}
	for _, agent := range agents {
		rel := filepath.ToSlash(filepath.Join(".codex", "agents", agent.Name+".toml"))
		body, err := os.ReadFile(filepath.Join(primary, filepath.FromSlash(rel)))
		if err != nil || hash(body) != got.Files[rel] {
			t.Fatalf("native role pins changed: %s: %v", rel, err)
		}
	}

}
