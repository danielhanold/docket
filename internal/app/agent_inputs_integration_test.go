//go:build integration

package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/codexcontract"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

func TestIntegrationWorkflowAgentInputsAcceptsPrimaryStartupForRegisteredFeature(t *testing.T) {
	root := testsupport.TempDir(t)
	controlRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "primary")
	feature := filepath.Join(root, "feature")
	runGit := func(dir string, args ...string) string {
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid")
		b, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, b)
		}
		return strings.TrimSpace(string(b))
	}
	runGit(root, "init", "-b", "main", primary)
	if err := os.WriteFile(filepath.Join(primary, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(primary, "add", "README.md")
	runGit(primary, "commit", "-m", "base")
	runGit(primary, "worktree", "add", "-b", "codex/change", "--", feature, "HEAD")
	client, err := gitcli.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	repo, err := client.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: primary})
	if err != nil {
		t.Fatal(err)
	}
	featureWorktree, err := client.DiscoverWorktree(context.Background(), gitcli.DiscoverOptions{InvocationPath: feature})
	if err != nil {
		t.Fatal(err)
	}
	head := runGit(feature, "rev-parse", "HEAD")
	docketPath, err := filepath.EvalSymlinks("/usr/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	identity, err := codexcontract.ObserveRootIdentity(featureWorktree.Root, featureWorktree.GitDir)
	if err != nil {
		t.Fatal(err)
	}
	a := codexcontract.Assignment{SchemaVersion: 1, ChangeID: 425, Role: "docket-plan-writer", Phase: "plan", Mode: "fresh", Primary: repo.PrimaryWorktree, Feature: featureWorktree.Root, CommonDir: repo.CommonDir, Branch: "codex/change", EntryHEAD: head, MetadataRevision: head, ChangePath: "docs/changes/active/0425.md", ArtifactPath: "docs/plans/425.md", DocketExecutable: docketPath, DocketCommit: head, ReadRoots: []string{controlRoot, filepath.Dir(docketPath)}, WritePaths: []string{"docs/plans/425.md"}, RootIdentity: &identity}
	ab, _ := json.Marshal(a)
	ap := filepath.Join(root, "assignment.json")
	if err := os.WriteFile(ap, ab, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(ab)
	deps, err := NewAgentInputDeps("/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	deps.Workspace = &inputWorkspace{}
	res := CheckAgentInputs(context.Background(), deps, CheckInputsRequest{Assignment: ap, SHA256: hex.EncodeToString(sum[:]), Stage: "entry", RepoDir: primary})
	if res.Result != ResultApplied {
		t.Fatalf("result=%s reason=%s", res.Result, res.Reason)
	}
}
