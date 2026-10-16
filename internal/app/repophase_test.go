package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/reposeed"
	"github.com/danielhanold/docket/internal/testsupport"
)

// TestClassifyAgentsMD pins the AGENTS.md pre-state classifyAgentsMD hands
// reposeed.Plan: only a clean absence is AgentsMDMissing and only a regular file
// is AgentsMDRegularFile; a symlink (circular included), a directory, and a probe
// error other than not-exist are all AgentsMDOther, never "missing".
//
// MUTATION TEST: make classifyAgentsMD return AgentsMDMissing for a stat error
// other than not-exist, or for a symlink, and the matching row reddens.
func TestClassifyAgentsMD(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, root string) string // returns the root to classify
		want  reposeed.AgentsMDState
	}{
		{"no file", func(t *testing.T, root string) string { return root }, reposeed.AgentsMDMissing},
		{"regular file", func(t *testing.T, root string) string {
			writeRepoFile(t, root, "AGENTS.md", "x\n")
			return root
		}, reposeed.AgentsMDRegularFile},
		{"directory", func(t *testing.T, root string) string {
			if err := os.Mkdir(filepath.Join(root, "AGENTS.md"), 0o755); err != nil {
				t.Fatal(err)
			}
			return root
		}, reposeed.AgentsMDOther},
		{"symlink to a regular sibling", func(t *testing.T, root string) string {
			writeRepoFile(t, root, "RULES.md", "x\n")
			if err := os.Symlink("RULES.md", filepath.Join(root, "AGENTS.md")); err != nil {
				t.Fatal(err)
			}
			return root
		}, reposeed.AgentsMDOther},
		{"circular symlink", func(t *testing.T, root string) string {
			if err := os.Symlink("AGENTS.md", filepath.Join(root, "AGENTS.md")); err != nil {
				t.Fatal(err)
			}
			return root
		}, reposeed.AgentsMDOther},
		{"probe error other than not-exist", func(t *testing.T, root string) string {
			// The "root" is a regular file, so the probe fails with ENOTDIR.
			writeRepoFile(t, root, "notadir", "x\n")
			return filepath.Join(root, "notadir")
		}, reposeed.AgentsMDOther},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.setup(t, testsupport.TempDir(t))
			if got := classifyAgentsMD(root); got != tc.want {
				t.Errorf("classifyAgentsMD = %v, want %v", got, tc.want)
			}
		})
	}
}
