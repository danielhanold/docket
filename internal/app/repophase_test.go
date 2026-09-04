package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/danielhanold/docket/internal/testsupport"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/danielhanold/docket/internal/config"
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
	dir := testsupport.TempDir(t)
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
		phase, gotRoot, _, err := ResolveRepoPhase(context.Background(), git, dir, nil, []byte("gate\n"), nil, config.ResolveContext{DefaultBranch: "main"})
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
	notARepo := testsupport.TempDir(t)
	_, _, _, err := ResolveRepoPhase(context.Background(), git, notARepo, nil, nil, nil, config.ResolveContext{DefaultBranch: "main"})
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
	outside := testsupport.TempDir(t)
	t.Chdir(outside)
	// Empty repoDir + cwd outside any Git working tree: no phase at all, and no
	// error — the machine install proceeds and the not-authorized action prints.
	phase, root, _, err := ResolveRepoPhase(context.Background(), git, "", nil, nil, nil, config.ResolveContext{DefaultBranch: "main"})
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
	phase, gotRoot, _, err := ResolveRepoPhase(context.Background(), git, root, nil, nil, nil, config.ResolveContext{DefaultBranch: "main"})
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
	xdg := testsupport.TempDir(t)
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

	phase, _, _, err := ResolveRepoPhase(context.Background(), git, root, nil, nil, nil, config.ResolveContext{DefaultBranch: "main"})
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
	phase, _, _, err := ResolveRepoPhase(context.Background(), git, root, nil, nil, nil, config.ResolveContext{DefaultBranch: "main"})
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

	phase, _, _, err := ResolveRepoPhase(context.Background(), git, root, []string{"codex"}, []byte("gate\n"), nil, config.ResolveContext{DefaultBranch: "main"})
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

// TestResolveRepoPhaseRetiresDroppedClaudeLink is the symlink-retirement row: a
// repo that once opted into [claude codex] now opts into [codex] alone, with a
// prior record owning CLAUDE.md as a claude symlink to the shared AGENTS.md. The
// dropped claude link must be a provable removal — the install no longer
// hard-fails on a KindSymlink retirement cannot reason about — carried as exactly
// one removal whose Path/Kind/LinkTarget/Harness name the link.
//
// MUTATION TEST: drop the LinkTarget threading in computeRemovals (the symlink
// arm that joins s.LinkTarget under root) and the LinkTarget assertion below
// reddens — the removal would name no destination.
func TestResolveRepoPhaseRetiresDroppedClaudeLink(t *testing.T) {
	root, gitDir := initGitRepo(t, "agent_harnesses: [codex]\n")
	git := newGitClient(t)

	// On disk: codex's shared AGENTS.md and the claude CLAUDE.md link to it.
	writeRepoFile(t, root, "AGENTS.md", "shared agents surface\n")
	if err := os.Symlink("AGENTS.md", filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	// A prior record owning CLAUDE.md as a claude symlink to AGENTS.md.
	prior := &reposeed.Record{
		FormatVersion: reposeed.RecordFormatVersion,
		Surfaces: []reposeed.SurfaceRecord{
			{Path: "CLAUDE.md", Kind: install.KindSymlink, LinkTarget: "AGENTS.md",
				Harnesses: []string{"claude"}},
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

	phase, _, _, err := ResolveRepoPhase(context.Background(), git, root, nil, []byte("gate\n"), nil, config.ResolveContext{DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("ResolveRepoPhase hard-failed on the dropped symlink: %v", err)
	}
	if phase == nil || !phase.Authorized {
		t.Fatalf("phase = %+v, want an authorized phase", phase)
	}
	if len(phase.Removals) != 1 {
		t.Fatalf("removals = %+v, want exactly one (the dropped claude link)", phase.Removals)
	}
	rem := phase.Removals[0]
	if want := filepath.Join(root, "CLAUDE.md"); rem.Path != want {
		t.Errorf("removal path = %q, want %q", rem.Path, want)
	}
	if rem.Kind != install.KindSymlink {
		t.Errorf("removal kind = %q, want %q", rem.Kind, install.KindSymlink)
	}
	if want := filepath.Join(root, "AGENTS.md"); rem.LinkTarget != want {
		t.Errorf("removal link target = %q, want %q", rem.LinkTarget, want)
	}
	if rem.Harness != "claude" {
		t.Errorf("removal harness = %q, want claude", rem.Harness)
	}
}

// TestResolveRepoPhaseToleratesUnknownKeys (change 0392): with a tolerant
// context, a .docket.yml carrying an unknown key plus an explicit
// agent_harnesses still yields an authorized phase, and the unknown-key
// warning comes back for the install result to surface. The strict control —
// today's ReasonInvalidConfig refusal — pins that the CLI's context, not this
// assembler, owns the decision.
func TestResolveRepoPhaseToleratesUnknownKeys(t *testing.T) {
	root, _ := initGitRepo(t, "agent_harnesses: [claude]\nsome_future_block: true\n")
	git := newGitClient(t)

	tolerant := config.ResolveContext{DefaultBranch: "main", TolerateUnknownKeys: true}
	phase, gotRoot, warnings, err := ResolveRepoPhase(context.Background(), git, root, nil, []byte("gate\n"), nil, tolerant)
	if err != nil {
		t.Fatalf("tolerant ResolveRepoPhase: %v", err)
	}
	if phase == nil || !phase.Authorized || gotRoot != root {
		t.Fatalf("phase = %+v root = %q, want an authorized phase at %q", phase, gotRoot, root)
	}
	found := false
	for _, w := range warnings {
		if w.Code == config.CodeUnknownKey && w.Severity == config.SeverityWarning {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %v, want the tolerated unknown-key warning", warnings)
	}

	strict := config.ResolveContext{DefaultBranch: "main"}
	_, _, _, err = ResolveRepoPhase(context.Background(), git, root, nil, []byte("gate\n"), nil, strict)
	var re *RepoResolutionError
	if !errors.As(err, &re) || re.Reason != ReasonInvalidConfig {
		t.Fatalf("strict err = %v, want RepoResolutionError with %q", err, ReasonInvalidConfig)
	}
}
