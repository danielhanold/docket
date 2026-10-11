package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/testsupport"
)

func writeGlobalConfig(t *testing.T, home, body string) {
	t.Helper()
	dir := filepath.Join(home, ".config", "docket")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A global cache_ttl that reaches a non-Claude harness surfaces as an install
// warning naming the harness and agent; the configuration never blocks.
func TestInstallOptionsWarnsOnDroppedCacheTTL(t *testing.T) {
	home := pinInstallEnv(t)
	t.Chdir(testsupport.TempDir(t))
	writeGlobalConfig(t, home, "agents:\n  default:\n    status:\n      cache_ttl: 1h\n")

	opts, refusal := installOptions(context.Background(), []string{"codex", "claude"}, "", false, devInfo())
	if refusal != nil {
		t.Fatalf("installOptions refused: %v", refusal)
	}
	var paths []string
	for _, w := range opts.ConfigWarnings {
		if w.Code == string(app.FCAgentCacheTTLDropped) {
			paths = append(paths, w.Path)
		}
	}
	if len(paths) != 1 || paths[0] != "agents.codex.status.cache_ttl" {
		t.Fatalf("cache TTL warnings = %v, want exactly agents.codex.status.cache_ttl", paths)
	}
}

// The shipped defaults alone print no cache TTL warning on any harness.
func TestInstallOptionsShippedDefaultsWarnNoCacheTTL(t *testing.T) {
	pinInstallEnv(t)
	t.Chdir(testsupport.TempDir(t))
	opts, refusal := installOptions(context.Background(), nil, "", false, devInfo())
	if refusal != nil {
		t.Fatalf("installOptions refused: %v", refusal)
	}
	for _, w := range opts.ConfigWarnings {
		if w.Code == string(app.FCAgentCacheTTLDropped) {
			t.Errorf("shipped defaults warned: %+v", w)
		}
	}
}

// End to end: a real install writes the shipped 1h TTL into claude's
// implement-next wrapper, and only there.
func TestInstallWritesShippedCacheTTLForImplementNext(t *testing.T) {
	home := pinInstallEnv(t)
	t.Chdir(testsupport.TempDir(t))
	if out, stderr, code := runCLI(t, "install", "--harness", "claude", "--json"); code != 0 {
		t.Fatalf("install: code=%d out=%s stderr=%s", code, out, stderr)
	}
	agentsDir := filepath.Join(home, ".claude", "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("ReadDir %s: %v (%d entries)", agentsDir, err, len(entries))
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(agentsDir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		has := strings.Contains(string(b), "\nexperimental:\n  cacheTtl: 1h\n")
		if want := e.Name() == "docket-implement-next.md"; has != want {
			t.Errorf("%s: carries experimental.cacheTtl 1h = %v, want %v", e.Name(), has, want)
		}
	}
}
