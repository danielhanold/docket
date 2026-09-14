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

// TestAgentInputReviewRegressions keeps the phase-4 overlay reproductions in
// the permanent suite. The observer is the real Git implementation; only the
// metadata/workspace lookup is isolated so these cases exercise the input
// boundary independently of the workspace service.
func TestAgentInputReviewRegressions(t *testing.T) {
	for _, scenario := range []string{"committed-owned-path", "committed-unowned-path", "non-descendant-head", "review-head-mismatch", "review-dirty", "review-evidence-drift", "worker-entry-without-payload"} {
		t.Run(scenario, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(testsupport.TempDir(t))
			if err != nil {
				t.Fatal(err)
			}
			primary, feature := filepath.Join(root, "primary"), filepath.Join(root, "feature")
			git := func(dir string, args ...string) string {
				c := exec.Command("git", args...)
				c.Dir = dir
				c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=review", "GIT_AUTHOR_EMAIL=review@example.invalid", "GIT_COMMITTER_NAME=review", "GIT_COMMITTER_EMAIL=review@example.invalid")
				b, e := c.CombinedOutput()
				if e != nil {
					t.Fatalf("git %v: %v: %s", args, e, b)
				}
				return strings.TrimSpace(string(b))
			}
			git(root, "init", "-b", "main", primary)
			if err := os.WriteFile(filepath.Join(primary, "README.md"), []byte("base\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			git(primary, "add", "README.md")
			git(primary, "commit", "-m", "base")
			base := git(primary, "rev-parse", "HEAD")
			git(primary, "worktree", "add", "-b", "codex/review", feature, "HEAD")
			client, err := gitcli.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			repo, err := client.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: primary})
			if err != nil {
				t.Fatal(err)
			}
			wt, err := client.DiscoverWorktree(context.Background(), gitcli.DiscoverOptions{InvocationPath: feature})
			if err != nil {
				t.Fatal(err)
			}
			identity, err := codexcontract.ObserveRootIdentity(wt.Root, wt.GitDir)
			if err != nil {
				t.Fatal(err)
			}
			binary, err := filepath.EvalSymlinks("/usr/bin/true")
			if err != nil {
				t.Fatal(err)
			}
			a := codexcontract.Assignment{SchemaVersion: 1, ChangeID: 425, Role: "docket-build-standard", Phase: "build", TaskID: "task-1", Mode: "fresh", Primary: repo.PrimaryWorktree, Feature: wt.Root, CommonDir: repo.CommonDir, Branch: "codex/review", EntryHEAD: base, MetadataRevision: base, ChangePath: "docs/changes/active/0425.md", DocketExecutable: binary, DocketCommit: base, ReadRoots: []string{root, filepath.Dir(binary)}, WritePaths: []string{"owned.go"}, RootIdentity: &identity}
			stage := "entry"
			if scenario == "committed-owned-path" {
				if err := os.WriteFile(filepath.Join(feature, "owned.go"), []byte("package owned\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				git(feature, "add", "owned.go")
				git(feature, "commit", "-m", "owned edit")
				stage = "active"
			}
			if scenario == "committed-unowned-path" {
				if err := os.WriteFile(filepath.Join(feature, "README.md"), []byte("unowned change\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				git(feature, "add", "README.md")
				git(feature, "commit", "-m", "unowned edit")
				stage = "active"
			}
			if scenario == "non-descendant-head" {
				tree := git(feature, "write-tree")
				replacement := git(feature, "commit-tree", tree, "-m", "unrelated root")
				git(feature, "reset", "--hard", replacement)
				stage = "active"
			}
			if strings.HasPrefix(scenario, "review-") {
				evidencePath := filepath.Join(root, "build-evidence.md")
				evidenceBody := []byte("green at pinned head\n")
				if err := os.WriteFile(evidencePath, evidenceBody, 0o600); err != nil {
					t.Fatal(err)
				}
				evidenceHash := sha256.Sum256(evidenceBody)
				a.Role = "docket-review-standard"
				a.Phase = "review"
				a.Mode = "review"
				a.ReviewBase = base
				a.ReviewHEAD = base
				a.BuildEvidence = "build-evidence"
				a.Resources = []codexcontract.Resource{{LogicalID: "build-evidence", Path: evidencePath, SHA256: hex.EncodeToString(evidenceHash[:]), Source: "controller"}}
				if scenario == "review-head-mismatch" {
					a.EntryHEAD = strings.Repeat("a", 40)
				}
				if scenario == "review-dirty" {
					if err := os.WriteFile(filepath.Join(feature, "dirty.txt"), []byte("dirty\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			b, err := json.Marshal(a)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "assignment.json")
			if err := os.WriteFile(path, b, 0o600); err != nil {
				t.Fatal(err)
			}
			if scenario == "review-evidence-drift" {
				if err := os.WriteFile(filepath.Join(root, "build-evidence.md"), []byte("changed\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			sum := sha256.Sum256(b)
			deps, err := NewAgentInputDeps(binary)
			if err != nil {
				t.Fatal(err)
			}
			deps.Workspace = &inputWorkspace{}
			r := CheckAgentInputs(context.Background(), deps, CheckInputsRequest{Assignment: path, SHA256: hex.EncodeToString(sum[:]), Stage: stage, RepoDir: primary})
			if scenario == "committed-owned-path" && r.Result != ResultApplied {
				t.Fatalf("valid owned descendant commit refused: result=%s reason=%s", r.Result, r.Reason)
			}
			if scenario != "committed-owned-path" && r.Result == ResultApplied {
				t.Fatalf("required rejection missing: %s returned applied", scenario)
			}
		})
	}
}
