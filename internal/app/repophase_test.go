package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/install"
	"github.com/danielhanold/docket/internal/reposeed"
)

// These tests drive ResolveRepoPhase, the app-layer assembler that turns a
// repository selection into the install-package RepoPhase the transaction
// consumes. They exercise the rows of the spec's repository matrix that live at
// this layer: worktree discovery, explicit --repo-dir, the provenance guard that
// decides write authority, and the scoped-harness narrowing that carries an
// unrelated harness's ownership record forward.

// initGitRepo makes a bare working tree with an optional committed-shape
// .docket.yml and returns the canonical worktree root and its git dir — the two
// paths reposeed and DiscoverWorktree both canonicalize to.
func initGitRepo(t *testing.T, docketYML string) (root, gitDir string) {
	t.Helper()
	requireRealGit(t)
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	if docketYML != "" {
		writeRepoFile(t, dir, ".docket.yml", docketYML)
	}
	canon, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", dir, err)
	}
	return canon, filepath.Join(canon, ".git")
}

// targetPaths is the sorted set of a phase's planned target paths, for compact
// assertions about what a run reconciles.
func targetPaths(phase *install.RepoPhase) []string {
	var out []string
	for _, t := range phase.Targets {
		out = append(out, t.Path)
	}
	sort.Strings(out)
	return out
}

func TestResolveRepoPhaseDiscoversFromRootAndNestedDir(t *testing.T) {
	root, _ := initGitRepo(t, "agent_harnesses: [claude]\n")
	nested := filepath.Join(root, "docs", "changes")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	git := newGitClient(t)

	for _, dir := range []string{root, nested} {
		phase, gotRoot, err := ResolveRepoPhase(context.Background(), git, dir, nil, []byte("gate\n"), nil)
		if err != nil {
			t.Fatalf("ResolveRepoPhase(%s): %v", dir, err)
		}
		if phase == nil || !phase.Authorized {
			t.Fatalf("ResolveRepoPhase(%s) = %+v, want an authorized phase", dir, phase)
		}
		if gotRoot != root || phase.Worktree != root {
			t.Errorf("worktree root = %q / %q, want %q", gotRoot, phase.Worktree, root)
		}
		if want := filepath.Join(root, "CLAUDE.md"); targetPaths(phase)[0] != want {
			t.Errorf("targets = %v, want to include %q", targetPaths(phase), want)
		}
	}
}

func TestResolveRepoPhaseInvalidExplicitRepoDir(t *testing.T) {
	git := newGitClient(t)
	notARepo := t.TempDir()
	_, _, err := ResolveRepoPhase(context.Background(), git, notARepo, nil, nil, nil)
	if err == nil {
		t.Fatalf("an explicit --repo-dir that is not a worktree must refuse")
	}
	var re *RepoResolutionError
	if !errors.As(err, &re) || re.Reason != install.ReasonInvalidRepoDir {
		t.Fatalf("error = %v (reason via RepoResolutionError), want %q", err, install.ReasonInvalidRepoDir)
	}
}

func TestResolveRepoPhaseOutsideGitIsMachineOnly(t *testing.T) {
	git := newGitClient(t)
	outside := t.TempDir()
	t.Chdir(outside)
	// Empty repoDir + cwd outside any Git working tree: no phase at all, and no
	// error — the machine install proceeds and the not-authorized action prints.
	phase, root, err := ResolveRepoPhase(context.Background(), git, "", nil, nil, nil)
	if err != nil {
		t.Fatalf("machine-only resolution errored: %v", err)
	}
	if phase != nil || root != "" {
		t.Fatalf("outside Git = (%+v, %q), want (nil, \"\")", phase, root)
	}
}

func TestResolveRepoPhaseAbsentKeyNotAuthorized(t *testing.T) {
	root, gitDir := initGitRepo(t, "metadata_branch: main\n")
	git := newGitClient(t)
	phase, gotRoot, err := ResolveRepoPhase(context.Background(), git, root, nil, nil, nil)
	if err != nil {
		t.Fatalf("ResolveRepoPhase: %v", err)
	}
	if phase == nil || phase.Authorized {
		t.Fatalf("absent agent_harnesses = %+v, want a non-nil, NOT-authorized phase", phase)
	}
	if gotRoot != root || phase.Worktree != root {
		t.Errorf("root = %q / %q, want %q", gotRoot, phase.Worktree, root)
	}
	if want := reposeed.RecordPath(gitDir); phase.RecordPath != want {
		t.Errorf("record path = %q, want %q", phase.RecordPath, want)
	}
	if len(phase.Targets) != 0 {
		t.Errorf("an unauthorized phase planned targets: %v", phase.Targets)
	}
}

// TestResolveRepoPhaseGlobalLayerNotAuthorized pins the provenance guard: an
// agent_harnesses declaration that resolves from the GLOBAL layer is never write
// authority for repository surfaces.
//
// MUTATION TEST: flip the guard in ResolveRepoPhase from
// `ah.Explicit && isRepositoryLayer(...)` to `ah.Explicit` alone and this test
// reddens — a global declaration would then authorize a repository write.
func TestResolveRepoPhaseGlobalLayerNotAuthorized(t *testing.T) {
	root, _ := initGitRepo(t, "metadata_branch: main\n")
	// The declaration lives in the GLOBAL layer only.
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	globalCfg := filepath.Join(xdg, "docket", "config.yml")
	if err := os.MkdirAll(filepath.Dir(globalCfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(globalCfg, []byte("agent_harnesses: [claude]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git, err := gitcli.NewClient()
	if err != nil {
		t.Fatalf("gitcli.NewClient: %v", err)
	}

	phase, _, err := ResolveRepoPhase(context.Background(), git, root, nil, nil, nil)
	if err != nil {
		t.Fatalf("ResolveRepoPhase: %v", err)
	}
	if phase == nil || phase.Authorized {
		t.Fatalf("a global-layer declaration authorized repository writes: %+v", phase)
	}
}

func TestResolveRepoPhaseAgentsTableAloneNotAuthorized(t *testing.T) {
	root, _ := initGitRepo(t, "agents:\n  claude:\n    build-standard:\n      model: opus\n")
	git := newGitClient(t)
	phase, _, err := ResolveRepoPhase(context.Background(), git, root, nil, nil, nil)
	if err != nil {
		t.Fatalf("ResolveRepoPhase: %v", err)
	}
	if phase == nil || phase.Authorized {
		t.Fatalf("an agents: table authorized repository writes: %+v", phase)
	}
}

// TestResolveRepoPhaseScopedHarnessCarriesUnrelatedRecord is the scoped-run row:
// opt-ins [claude codex], a prior record owning both surfaces, and a
// --harness codex scope. Only codex's surface is reconciled; claude's ownership
// record is carried forward unchanged.
func TestResolveRepoPhaseScopedHarnessCarriesUnrelatedRecord(t *testing.T) {
	root, gitDir := initGitRepo(t, "agent_harnesses: [claude, codex]\n")
	git := newGitClient(t)

	// A prior record owning claude's CLAUDE.md and codex's AGENTS.md.
	prior := &reposeed.Record{
		FormatVersion: reposeed.RecordFormatVersion,
		Surfaces: []reposeed.SurfaceRecord{
			{Path: "CLAUDE.md", Kind: install.KindManagedBlock, BlockName: "dispatch",
				SHA256: "sha256:claude-prior", Harnesses: []string{"claude"}},
			{Path: "AGENTS.md", Kind: install.KindManagedBlock, BlockName: "dispatch",
				SHA256: "sha256:codex-prior", Harnesses: []string{"codex"}},
		},
	}
	priorBytes, err := reposeed.EncodeRecord(prior)
	if err != nil {
		t.Fatal(err)
	}
	recPath := reposeed.RecordPath(gitDir)
	if err := os.MkdirAll(filepath.Dir(recPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recPath, priorBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	phase, _, err := ResolveRepoPhase(context.Background(), git, root, []string{"codex"}, []byte("gate\n"), nil)
	if err != nil {
		t.Fatalf("ResolveRepoPhase: %v", err)
	}
	if phase == nil || !phase.Authorized {
		t.Fatalf("scoped run = %+v, want an authorized phase", phase)
	}

	// Only codex's AGENTS.md is planned; claude's CLAUDE.md is NOT reconciled.
	if got, want := targetPaths(phase), []string{filepath.Join(root, "AGENTS.md")}; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("scoped targets = %v, want only %v", got, want)
	}

	// The published record carries claude's surface forward and reconciles codex's.
	var rec reposeed.Record
	if err := json.Unmarshal(phase.RecordBytes, &rec); err != nil {
		t.Fatalf("decoding published record: %v", err)
	}
	owners := map[string][]string{}
	for _, s := range rec.Surfaces {
		owners[s.Path] = s.Harnesses
	}
	if got := owners["CLAUDE.md"]; len(got) != 1 || got[0] != "claude" {
		t.Errorf("claude's carried record = %v, want [claude]; full record = %+v", got, rec.Surfaces)
	}
	if got := owners["AGENTS.md"]; len(got) != 1 || got[0] != "codex" {
		t.Errorf("codex's reconciled record = %v, want [codex]", got)
	}
	// No removals: nothing was dropped, both harnesses remain opted in.
	if len(phase.Removals) != 0 {
		t.Errorf("scoped run planned removals: %v", phase.Removals)
	}
}
