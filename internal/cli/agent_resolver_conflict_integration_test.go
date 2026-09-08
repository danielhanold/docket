//go:build integration

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/testsupport"
)

// This catches the production boundary regressing from the selected feature
// worktree to the caller/coordinator worktree: a resolver would then see an
// empty unmerged index while its owned rebase remains conflicted elsewhere.
func TestIntegrationAgentEnterFeatureResolverObservesSelectedWorktreeConflict(t *testing.T) {
	paths := newAgentEntryWorktrees(t)
	cleanupResolverConflictWorktrees(t, paths)
	prepareResolverRebaseConflict(t, paths)

	if got := gitUnmergedEntries(t, paths.a); got != "" {
		t.Fatalf("coordinator worktree %s has unmerged entries before entry: %q", paths.a, got)
	}
	if got := gitUnmergedEntries(t, paths.b); got == "" {
		t.Fatalf("feature worktree %s has no owned rebase conflict", paths.b)
	}

	seedAgentInstallation(t)
	stubDir := testsupport.TempDir(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	stub := "#!/bin/sh\nexec '" + strings.ReplaceAll(exe, "'", "'\\''") + "' -test.run=^TestAgentEnterResolverConflictServerProcess$ -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(stubDir, "codex"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DOCKET_AGENT_RESOLVER_CONFLICT_SERVER", "1")
	t.Setenv("DOCKET_AGENT_TEST_SKILL", "docket-convention")
	resolver := installedAgentTestContract(t, "docket-rebase-resolver")
	t.Setenv("DOCKET_AGENT_TEST_DEVELOPER", resolver.DeveloperInstructions)
	t.Setenv("DOCKET_AGENT_TEST_MODEL", resolver.Model)
	t.Setenv("DOCKET_AGENT_TEST_EFFORT", resolver.Effort)

	request := "Resolve only the owned rebase conflict.\nFeature worktree: " + paths.b + "\nPreserve these request bytes: `unchanged`.\n"
	t.Setenv("DOCKET_AGENT_TEST_REQUEST", request)

	var out, stderr bytes.Buffer
	code := Run([]string{"agent", "enter", "--role", "docket-rebase-resolver", "--request", "-", "--cwd", paths.a, "--worktree", paths.b, "--approval-policy", "never", "--sandbox", "workspace-write", "--json"}, strings.NewReader(request), &out, &stderr, devInfo(), hostFacts())
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d out=%q stderr=%q", code, out.String(), stderr.String())
	}
	var result app.AgentEnterResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Result != app.ResultApplied || result.Role != "docket-rebase-resolver" || result.ThreadID != "root" || result.TurnID != "turn" {
		t.Fatalf("receipt: %+v", result)
	}
	const observationPrefix = "resolver-observation cwd="
	if !strings.HasPrefix(result.Output, observationPrefix) {
		t.Fatalf("resolver output = %q, want %q prefix", result.Output, observationPrefix)
	}
	parts := strings.SplitN(strings.TrimPrefix(result.Output, observationPrefix), "\nunmerged:\n", 2)
	if len(parts) != 2 {
		t.Fatalf("resolver output has no unmerged observation: %q", result.Output)
	}
	if parts[1] == "" {
		t.Fatalf("resolver observed no unmerged entries from %s; original cross-worktree symptom", parts[0])
	}
	if parts[0] != paths.b {
		t.Fatalf("resolver thread/start.cwd = %q, want canonical feature worktree %q", parts[0], paths.b)
	}
	if !strings.Contains(parts[1], "conflict.txt") {
		t.Fatalf("resolver unmerged observation = %q, want conflict.txt", parts[1])
	}
}
