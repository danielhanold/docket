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

// TestClassifyClaudeMD pins the CLAUDE.md pre-state classifyClaudeMD hands
// reposeed.Plan. A link to the sibling AGENTS.md is the shareable state in both
// spellings: the relative one, and the absolute one install itself writes
// (reposeed.Plan joins the cleaned worktree root with AGENTS.md). Any other
// destination, absolute or relative, is a foreign link the plan leaves alone.
//
// MUTATION TEST: drop the absolute-link clause from classifyClaudeMD and the
// "absolute link to own AGENTS.md" row reddens; accept every absolute link and
// the "absolute link elsewhere" row reddens.
func TestClassifyClaudeMD(t *testing.T) {
	link := func(dest func(root string) string) func(t *testing.T, root string) {
		return func(t *testing.T, root string) {
			if err := os.Symlink(dest(root), filepath.Join(root, "CLAUDE.md")); err != nil {
				t.Fatal(err)
			}
		}
	}
	cases := []struct {
		name  string
		setup func(t *testing.T, root string)
		want  reposeed.ClaudeMDState
	}{
		{"no file", func(t *testing.T, root string) {}, reposeed.ClaudeMDAbsent},
		{"regular file", func(t *testing.T, root string) {
			writeRepoFile(t, root, "CLAUDE.md", "x\n")
		}, reposeed.ClaudeMDRegularFile},
		{"relative link to AGENTS.md", link(func(string) string { return "AGENTS.md" }), reposeed.ClaudeMDLinkToAgents},
		{"absolute link to own AGENTS.md", link(func(root string) string {
			return filepath.Join(root, "AGENTS.md")
		}), reposeed.ClaudeMDLinkToAgents},
		{"absolute link elsewhere", link(func(root string) string {
			return filepath.Join(root, "other", "AGENTS.md")
		}), reposeed.ClaudeMDForeignLink},
		{"relative link to another file", link(func(string) string { return "RULES.md" }), reposeed.ClaudeMDForeignLink},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := testsupport.TempDir(t)
			tc.setup(t, root)
			if got := classifyClaudeMD(root); got != tc.want {
				t.Errorf("classifyClaudeMD = %v, want %v", got, tc.want)
			}
		})
	}
}
