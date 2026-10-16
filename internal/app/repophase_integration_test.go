//go:build integration

package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/danielhanold/docket/internal/testsupport"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/install"
	"github.com/danielhanold/docket/internal/layout"
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

func TestIntegrationContextProbeResolveRepoPhaseDiscoversFromRootAndNestedDir(t *testing.T) {
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

func TestIntegrationContextProbeResolveRepoPhaseInvalidExplicitRepoDir(t *testing.T) {
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

func TestIntegrationContextProbeResolveRepoPhaseOutsideGitIsMachineOnly(t *testing.T) {
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

func TestIntegrationContextProbeResolveRepoPhaseAbsentKeyNotAuthorized(t *testing.T) {
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
	if want := reposeed.RecordPath(gitDir, layout.SharedName); phase.RecordPath != want {
		t.Errorf("record path = %q, want %q", phase.RecordPath, want)
	}
	if len(phase.Targets) != 0 {
		t.Errorf("an unauthorized phase planned targets: %v", phase.Targets)
	}
}

// TestIntegrationContextProbeResolveRepoPhaseGlobalLayerNotAuthorized pins the provenance guard: an
// agent_harnesses declaration that resolves from the GLOBAL layer is never write
// authority for repository surfaces.
//
// MUTATION TEST: flip the guard in ResolveRepoPhase from
// `ah.Explicit && isRepositoryLayer(...)` to `ah.Explicit` alone and this test
// reddens — a global declaration would then authorize a repository write.
func TestIntegrationContextProbeResolveRepoPhaseGlobalLayerNotAuthorized(t *testing.T) {
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

func TestIntegrationContextProbeResolveRepoPhaseAgentsTableAloneNotAuthorized(t *testing.T) {
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

// TestIntegrationContextProbeResolveRepoPhaseScopedHarnessCarriesUnrelatedRecord is the scoped-run row:
// opt-ins [claude codex], a prior record owning both surfaces, and a
// --harness codex scope. Only codex's surface is reconciled; claude's ownership
// record is carried forward unchanged.
func TestIntegrationContextProbeResolveRepoPhaseScopedHarnessCarriesUnrelatedRecord(t *testing.T) {
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
	recPath := reposeed.RecordPath(gitDir, layout.SharedName)
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

// TestIntegrationContextProbeResolveRepoPhaseRetiresDroppedClaudeLink is the symlink-retirement row: a
// repo that once opted into [claude codex] now opts into [codex] alone, with a
// prior record owning CLAUDE.md as a claude symlink to the shared AGENTS.md. The
// dropped claude link must be a provable removal — the install no longer
// hard-fails on a KindSymlink retirement cannot reason about — carried as exactly
// one removal whose Path/Kind/LinkTarget/Harness name the link.
//
// MUTATION TEST: drop the LinkTarget threading in computeRemovals (the symlink
// arm that joins s.LinkTarget under root) and the LinkTarget assertion below
// reddens — the removal would name no destination.
func TestIntegrationContextProbeResolveRepoPhaseRetiresDroppedClaudeLink(t *testing.T) {
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
	recPath := reposeed.RecordPath(gitDir, layout.SharedName)
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

// TestIntegrationContextProbeResolveRepoPhaseToleratesUnknownKeys (change 0392): with a tolerant
// context, a .docket.yml carrying an unknown key plus an explicit
// agent_harnesses still yields an authorized phase, and the unknown-key
// warning comes back for the install result to surface. The strict control —
// today's ReasonInvalidConfig refusal — pins that the CLI's context, not this
// assembler, owns the decision.
func TestIntegrationContextProbeResolveRepoPhaseToleratesUnknownKeys(t *testing.T) {
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

// seedSharedSurfaces writes the surfaces reposeed.Plan plans for the given inputs
// and publishes their ownership record, as an earlier install would have. A
// non-nil agentsOwners overrides the recorded AGENTS.md owners, to reproduce a
// record written under an older owner rule.
func seedSharedSurfaces(t *testing.T, root, gitDir string, harnesses []string, claudeMD reposeed.ClaudeMDState, agentsMD reposeed.AgentsMDState, agentsOwners []string) {
	t.Helper()
	rt, err := buildRunTracker()
	if err != nil {
		t.Fatal(err)
	}
	targets, owners, err := reposeed.Plan(reposeed.PlanInput{
		WorktreeRoot:  root,
		Harnesses:     harnesses,
		RunTracker:    rt,
		ClaudeMDState: claudeMD,
		AgentsMDState: agentsMD,
	})
	if err != nil {
		t.Fatalf("seed Plan: %v", err)
	}
	if agentsOwners != nil {
		owners[filepath.Join(root, "AGENTS.md")] = agentsOwners
	}
	rec, err := reposeed.DesiredRecord(targets, owners, root)
	if err != nil {
		t.Fatalf("seed DesiredRecord: %v", err)
	}
	recBytes, err := reposeed.EncodeRecord(rec)
	if err != nil {
		t.Fatal(err)
	}
	roots, err := install.ResolveRoots(os.UserHomeDir, os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	phase := &install.RepoPhase{
		Authorized:  true,
		Worktree:    root,
		Targets:     targets,
		Owners:      owners,
		RecordPath:  reposeed.RecordPath(gitDir, layout.SharedName),
		RecordBytes: recBytes,
	}
	if _, err := applyRepoPhaseSurfaces(phase, roots); err != nil {
		t.Fatalf("seed apply: %v", err)
	}
}

// installShared runs the shared-mode repository phase of an install over root,
// scoped to scope (nil for every opted-in harness), and applies it. It returns
// the phase and whether the apply wrote anything.
func installShared(t *testing.T, root string, scope []string) (*install.RepoPhase, bool) {
	t.Helper()
	rt, err := buildRunTracker()
	if err != nil {
		t.Fatal(err)
	}
	phase, _, _, err := ResolveRepoPhase(context.Background(), newGitClient(t), root, scope, rt, nil, config.ResolveContext{DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("ResolveRepoPhase: %v", err)
	}
	if phase == nil || !phase.Authorized {
		t.Fatalf("phase = %+v, want an authorized phase", phase)
	}
	roots, err := install.ResolveRoots(os.UserHomeDir, os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	wrote, err := applyRepoPhaseSurfaces(phase, roots)
	if err != nil {
		t.Fatalf("applyRepoPhaseSurfaces: %v", err)
	}
	return phase, wrote
}

// writeLinkOnlyRecord publishes a shared ownership record holding only the
// claude CLAUDE.md link to AGENTS.md.
func writeLinkOnlyRecord(t *testing.T, gitDir string) {
	t.Helper()
	b, err := reposeed.EncodeRecord(&reposeed.Record{
		FormatVersion: reposeed.RecordFormatVersion,
		Surfaces: []reposeed.SurfaceRecord{
			{Path: "CLAUDE.md", Kind: install.KindSymlink, LinkTarget: "AGENTS.md", Harnesses: []string{"claude"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	recPath := reposeed.RecordPath(gitDir, layout.SharedName)
	if err := os.MkdirAll(filepath.Dir(recPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recPath, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// agentsDispatch reads AGENTS.md and returns its dispatch block's interior and
// the file with the whole block removed, trimmed of leading blank lines.
func agentsDispatch(t *testing.T, root string) (interior, rest string) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	doc, err := document.Parse(src)
	if err != nil {
		t.Fatalf("parse AGENTS.md: %v", err)
	}
	b, ok := doc.Block("dispatch")
	if !ok {
		t.Fatalf("AGENTS.md has no dispatch block:\n%s", src)
	}
	interior = string(src[b.Interior.Start:b.Interior.End])
	rest = strings.TrimLeft(string(src[:b.Start.Start])+string(src[b.End.End:]), "\n")
	return interior, rest
}

// assertClaudeLink fails unless CLAUDE.md is a symlink reading "AGENTS.md".
func assertClaudeLink(t *testing.T, root string) {
	t.Helper()
	p := filepath.Join(root, "CLAUDE.md")
	info, err := os.Lstat(p)
	if err != nil {
		t.Fatalf("lstat CLAUDE.md: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("CLAUDE.md mode = %v, want a symlink", info.Mode())
	}
	if dest, err := os.Readlink(p); err != nil || dest != "AGENTS.md" {
		t.Fatalf("CLAUDE.md -> %q (%v), want AGENTS.md", dest, err)
	}
}

// assertClaudeResolvesToAgents fails unless CLAUDE.md is a symlink that resolves
// to the sibling AGENTS.md (whatever spelling the link's destination uses).
func assertClaudeResolvesToAgents(t *testing.T, root string) {
	t.Helper()
	p := filepath.Join(root, "CLAUDE.md")
	if info, err := os.Lstat(p); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("CLAUDE.md = %v (%v), want a symlink", info, err)
	}
	ci, err := os.Stat(p)
	if err != nil {
		t.Fatalf("CLAUDE.md does not resolve: %v", err)
	}
	ai, err := os.Stat(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(ci, ai) {
		t.Errorf("CLAUDE.md does not resolve to AGENTS.md")
	}
}

// recordOwners loads the published shared record and maps each surface path to
// its surface record.
func recordOwners(t *testing.T, gitDir string) map[string]reposeed.SurfaceRecord {
	t.Helper()
	rec, err := reposeed.LoadRecord(reposeed.RecordPath(gitDir, layout.SharedName))
	if err != nil || rec == nil {
		t.Fatalf("LoadRecord = %v, %v", rec, err)
	}
	out := map[string]reposeed.SurfaceRecord{}
	for _, s := range rec.Surfaces {
		out[s.Path] = s
	}
	return out
}

const claudeShareUserText = "# Project rules\n\nUse tabs.\n"

// TestIntegrationContextProbeClaudeShareHandMadeLinkAddsBlock: a claude-only
// repository whose user hand-linked CLAUDE.md to their own AGENTS.md gets the
// dispatch block inserted into AGENTS.md, every user byte kept, the link kept.
func TestIntegrationContextProbeClaudeShareHandMadeLinkAddsBlock(t *testing.T) {
	pinInstructionsUserRoots(t)
	root, gitDir := initGitRepo(t, "agent_harnesses: [claude]\n")
	writeRepoFile(t, root, "AGENTS.md", claudeShareUserText)
	if err := os.Symlink("AGENTS.md", filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}

	installShared(t, root, nil)

	if _, rest := agentsDispatch(t, root); rest != claudeShareUserText {
		t.Errorf("AGENTS.md outside the block = %q, want the user text %q", rest, claudeShareUserText)
	}
	assertClaudeLink(t, root)
	surfaces := recordOwners(t, gitDir)
	if s := surfaces["CLAUDE.md"]; s.Kind != install.KindSymlink || !reflect.DeepEqual(s.Harnesses, []string{"claude"}) {
		t.Errorf("CLAUDE.md record = %+v, want a claude symlink", s)
	}
	if s := surfaces["AGENTS.md"]; !reflect.DeepEqual(s.Harnesses, []string{"claude"}) {
		t.Errorf("AGENTS.md record = %+v, want owners [claude]", s)
	}
}

// TestIntegrationContextProbeClaudeShareRepairsLostBlock: the record holds only
// the claude link and AGENTS.md lost its block; install puts the block back and
// leaves the user text untouched.
func TestIntegrationContextProbeClaudeShareRepairsLostBlock(t *testing.T) {
	pinInstructionsUserRoots(t)
	root, gitDir := initGitRepo(t, "agent_harnesses: [claude]\n")
	writeRepoFile(t, root, "AGENTS.md", claudeShareUserText)
	if err := os.Symlink("AGENTS.md", filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	writeLinkOnlyRecord(t, gitDir)

	installShared(t, root, nil)

	if _, rest := agentsDispatch(t, root); rest != claudeShareUserText {
		t.Errorf("AGENTS.md outside the block = %q, want the user text %q", rest, claudeShareUserText)
	}
	assertClaudeLink(t, root)
}

// TestIntegrationContextProbeClaudeShareRepairsMissingAgentsMD: the CLAUDE.md
// link dangles because AGENTS.md is gone; install recreates AGENTS.md with the
// block so the link resolves again.
func TestIntegrationContextProbeClaudeShareRepairsMissingAgentsMD(t *testing.T) {
	pinInstructionsUserRoots(t)
	root, gitDir := initGitRepo(t, "agent_harnesses: [claude]\n")
	if err := os.Symlink("AGENTS.md", filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	writeLinkOnlyRecord(t, gitDir)

	installShared(t, root, nil)

	agentsDispatch(t, root)
	assertClaudeLink(t, root)
	assertClaudeResolvesToAgents(t, root)
}

// TestIntegrationContextProbeClaudeShareNoClaudeMDLinksOwnAgentsMD: a
// claude-only repository with its own AGENTS.md and no CLAUDE.md gets CLAUDE.md
// linked to AGENTS.md and the block inserted beside the user text.
func TestIntegrationContextProbeClaudeShareNoClaudeMDLinksOwnAgentsMD(t *testing.T) {
	pinInstructionsUserRoots(t)
	root, _ := initGitRepo(t, "agent_harnesses: [claude]\n")
	writeRepoFile(t, root, "AGENTS.md", claudeShareUserText)

	installShared(t, root, nil)

	assertClaudeLink(t, root)
	assertClaudeResolvesToAgents(t, root)
	if _, rest := agentsDispatch(t, root); rest != claudeShareUserText {
		t.Errorf("AGENTS.md outside the block = %q, want the user text %q", rest, claudeShareUserText)
	}
}

// TestIntegrationContextProbeClaudeShareRewritesAbsoluteLink: an older install
// wrote CLAUDE.md as an absolute link, which dangles in every other clone once
// committed. Reinstalling rewrites docket's own link to the relative
// "AGENTS.md", and the install after that is a steady state.
//
// MUTATION TEST: drop the LinkRelative rewrite from inspectSymlink and the
// absolute link stays a no-op; this test reddens.
func TestIntegrationContextProbeClaudeShareRewritesAbsoluteLink(t *testing.T) {
	pinInstructionsUserRoots(t)
	root, _ := initGitRepo(t, "agent_harnesses: [claude]\n")
	writeRepoFile(t, root, "AGENTS.md", claudeShareUserText)
	installShared(t, root, nil)

	claude := filepath.Join(root, "CLAUDE.md")
	if err := os.Remove(claude); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "AGENTS.md"), claude); err != nil {
		t.Fatal(err)
	}

	installShared(t, root, nil)
	assertClaudeLink(t, root)

	if _, wrote := installShared(t, root, nil); wrote {
		t.Errorf("third install wrote again, want a steady state")
	}
}

// TestIntegrationContextProbeClaudeShareNonRegularAgentsMDUnchanged: CLAUDE.md
// links to an AGENTS.md that is a directory; nothing is planned for Claude and
// the run still succeeds without touching either path.
func TestIntegrationContextProbeClaudeShareNonRegularAgentsMDUnchanged(t *testing.T) {
	pinInstructionsUserRoots(t)
	root, _ := initGitRepo(t, "agent_harnesses: [claude]\n")
	if err := os.Mkdir(filepath.Join(root, "AGENTS.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("AGENTS.md", filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}

	phase, _ := installShared(t, root, nil)

	if len(phase.Targets) != 0 {
		t.Errorf("targets = %v, want none", targetPaths(phase))
	}
	assertClaudeLink(t, root)
	if info, err := os.Lstat(filepath.Join(root, "AGENTS.md")); err != nil || !info.IsDir() {
		t.Errorf("AGENTS.md = %v (%v), want the untouched directory", info, err)
	}
	if entries, err := os.ReadDir(filepath.Join(root, "AGENTS.md")); err != nil || len(entries) != 0 {
		t.Errorf("AGENTS.md directory entries = %v (%v), want empty", entries, err)
	}
}

// TestIntegrationContextProbeClaudeShareScopedRunKeepsCodexClause: a
// --harness opencode run in a repository that opts codex in too keeps the
// AGENTS.md interior, Codex clause included, byte-identical.
func TestIntegrationContextProbeClaudeShareScopedRunKeepsCodexClause(t *testing.T) {
	pinInstructionsUserRoots(t)
	root, _ := initGitRepo(t, "agent_harnesses: [claude, codex, opencode]\n")

	installShared(t, root, nil)
	before, _ := agentsDispatch(t, root)
	if !strings.Contains(before, "### Codex root-coordinator entry") {
		t.Fatalf("full install interior lacks the Codex clause:\n%s", before)
	}

	installShared(t, root, []string{"opencode"})
	if after, _ := agentsDispatch(t, root); after != before {
		t.Errorf("scoped run changed the AGENTS.md interior:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestIntegrationContextProbeClaudeShareReinstallKeepsOwnLink: a claude-only
// repository with its own AGENTS.md and no CLAUDE.md is installed twice. The
// second install must read the CLAUDE.md link the first one wrote as a share of
// AGENTS.md, so it re-plans the AGENTS.md block with claude among its owners
// rather than retiring it, and the user text survives.
//
// MUTATION TEST: drop the absolute-link clause from classifyClaudeMD and the
// second install reads docket's own link as foreign; this test reddens.
func TestIntegrationContextProbeClaudeShareReinstallKeepsOwnLink(t *testing.T) {
	pinInstructionsUserRoots(t)
	root, gitDir := initGitRepo(t, "agent_harnesses: [claude]\n")
	writeRepoFile(t, root, "AGENTS.md", claudeShareUserText)

	installShared(t, root, nil)
	assertClaudeLink(t, root)

	phase, _ := installShared(t, root, nil)

	agentsPath := filepath.Join(root, "AGENTS.md")
	if owners := phase.Owners[agentsPath]; !slices.Contains(owners, "claude") {
		t.Errorf("second install AGENTS.md owners = %v, want claude among them", owners)
	}
	for _, r := range phase.Removals {
		if r.Path == agentsPath {
			t.Errorf("second install retires AGENTS.md: %+v", r)
		}
	}
	if _, rest := agentsDispatch(t, root); rest != claudeShareUserText {
		t.Errorf("AGENTS.md outside the block = %q, want the user text %q", rest, claudeShareUserText)
	}
	assertClaudeResolvesToAgents(t, root)
	if s := recordOwners(t, gitDir)["AGENTS.md"]; !reflect.DeepEqual(s.Harnesses, []string{"claude"}) {
		t.Errorf("AGENTS.md record = %+v, want owners [claude]", s)
	}
}

// TestIntegrationContextProbeClaudeShareDropKeepsAgentsBlock: a repository whose
// record lists AGENTS.md under its codex/opencode owner only (the claude link
// owned separately) drops to claude alone. The AGENTS.md block Claude now
// co-owns is planned and must not be retired in the same run, nor the run
// refused for writing one path twice.
func TestIntegrationContextProbeClaudeShareDropKeepsAgentsBlock(t *testing.T) {
	for _, tc := range []struct {
		name      string
		other     string
		userNotes bool
	}{
		{name: "opencode, only the block", other: "opencode"},
		{name: "opencode, user text around it", other: "opencode", userNotes: true},
		{name: "codex", other: "codex"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pinInstructionsUserRoots(t)
			root, gitDir := initGitRepo(t, "agent_harnesses: [claude, "+tc.other+"]\n")
			seedSharedSurfaces(t, root, gitDir, []string{"claude", tc.other}, reposeed.ClaudeMDAbsent, reposeed.AgentsMDMissing, []string{tc.other})
			if s := recordOwners(t, gitDir)["AGENTS.md"]; !reflect.DeepEqual(s.Harnesses, []string{tc.other}) {
				t.Fatalf("seeded AGENTS.md record = %+v, want owners [%s]", s, tc.other)
			}
			if tc.other == "codex" {
				if interior, _ := agentsDispatch(t, root); !strings.Contains(interior, "### Codex root-coordinator entry") {
					t.Fatalf("seeded interior lacks the Codex clause:\n%s", interior)
				}
			}
			if tc.userNotes {
				f, err := os.OpenFile(filepath.Join(root, "AGENTS.md"), os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.WriteString("\n## My notes\n\nkeep me\n"); err != nil {
					t.Fatal(err)
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
			}
			writeRepoFile(t, root, ".docket.yml", "agent_harnesses: [claude]\n")

			phase, _ := installShared(t, root, nil)

			agentsPath := filepath.Join(root, "AGENTS.md")
			for _, r := range phase.Removals {
				if r.Path == agentsPath {
					t.Errorf("removals retire AGENTS.md: %+v", r)
				}
			}
			interior, rest := agentsDispatch(t, root)
			if strings.Contains(interior, "### Codex root-coordinator entry") {
				t.Errorf("interior still carries the Codex clause after codex dropped:\n%s", interior)
			}
			if tc.userNotes && !strings.Contains(rest, "keep me") {
				t.Errorf("AGENTS.md lost the user notes; outside the block = %q", rest)
			}
			assertClaudeResolvesToAgents(t, root)
			if s := recordOwners(t, gitDir)["AGENTS.md"]; !reflect.DeepEqual(s.Harnesses, []string{"claude"}) {
				t.Errorf("AGENTS.md record = %+v, want owners [claude]", s)
			}

			if _, wrote := installShared(t, root, nil); wrote {
				t.Errorf("second install wrote again, want a steady state")
			}
			agentsDispatch(t, root)
		})
	}
}

// TestIntegrationContextProbeClaudeShareDropEverythingRetires: a claude-only
// repository sharing AGENTS.md through the CLAUDE.md link drops every harness;
// both the link and the AGENTS.md block docket wrote are retired.
func TestIntegrationContextProbeClaudeShareDropEverythingRetires(t *testing.T) {
	pinInstructionsUserRoots(t)
	root, gitDir := initGitRepo(t, "agent_harnesses: [claude]\n")
	if err := os.Symlink("AGENTS.md", filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	seedSharedSurfaces(t, root, gitDir, []string{"claude"}, reposeed.ClaudeMDLinkToAgents, reposeed.AgentsMDMissing, nil)
	if s := recordOwners(t, gitDir)["AGENTS.md"]; !reflect.DeepEqual(s.Harnesses, []string{"claude"}) {
		t.Fatalf("seeded AGENTS.md record = %+v, want owners [claude]", s)
	}
	writeRepoFile(t, root, ".docket.yml", "agent_harnesses: []\n")

	installShared(t, root, nil)

	for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
		if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Errorf("%s still present (lstat err %v), want retired", name, err)
		}
	}
}
