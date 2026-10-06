//go:build integration

package app

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/install"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/reposeed"
	"github.com/danielhanold/docket/internal/testsupport"
)

// This file is the real-Git shard (prefix TestIntegrationPrivateInstructions)
// for a private repository's instructions file: install's repository phase and
// private init write the dispatch block into <git-common-dir>/dckt/AGENTS.md and
// nothing into any working tree, retire the surfaces an earlier install wrote
// into the primary worktree, and leave shared repositories as they were.

// pinInstructionsUserRoots moves HOME and XDG_CONFIG_HOME to fresh temp dirs
// before anything reaches the installer; the user-level roots it resolves must
// never be the real machine's. XDG_DATA_HOME is pinned by newPrivateInitRepo (or
// by the shared test itself).
func pinInstructionsUserRoots(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", testsupport.TempDir(t))
	t.Setenv("XDG_CONFIG_HOME", testsupport.TempDir(t))
}

// newPrivateInstructionsRepo builds a private-initialized clone with no harness
// opted in and returns it with its canonical primary worktree and common dir.
func newPrivateInstructionsRepo(t *testing.T) (r *initRepo, primary, common string) {
	t.Helper()
	pinInstructionsUserRoots(t)
	r, _ = newPrivateInitRepo(t, nil)
	if res := r.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
		t.Fatalf("private init = %q (%s), want applied", res.Result, res.HumanText())
	}
	var err error
	if primary, err = filepath.EvalSymlinks(r.invocation); err != nil {
		t.Fatal(err)
	}
	common = filepath.Join(primary, ".git")
	return r, primary, common
}

var agentHarnessesLine = regexp.MustCompile(`(?m)^agent_harnesses:.*\n`)

// setPrivateHarnesses rewrites the agent_harnesses line of the private config
// (.git/dckt/config.yml) to the given flow list, e.g. "[claude, codex]".
func setPrivateHarnesses(t *testing.T, common, list string) {
	t.Helper()
	path := layout.PrivateConfigPath(common)
	cfg := string(mustReadFile(t, path))
	cfg = agentHarnessesLine.ReplaceAllString(cfg, "")
	if !strings.HasSuffix(cfg, "\n") {
		cfg += "\n"
	}
	cfg += "agent_harnesses: " + list + "\n"
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
}

// installSurfacesIn runs the init-time surface install against primary and
// fails the test on an error.
func installSurfacesIn(t *testing.T, primary string) []string {
	t.Helper()
	pending, _, err := installAuthorizedSurfaces(context.Background(), newGitClient(t), primary)
	if err != nil {
		t.Fatalf("installAuthorizedSurfaces(%s): %v", primary, err)
	}
	return pending
}

// assertNoWorktreeSurfaces fails when any parent-facing surface exists in dir.
func assertNoWorktreeSurfaces(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{"AGENTS.md", "CLAUDE.md", ".cursor"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s exists in %s (err=%v); a private repository gets nothing in its working tree", name, dir, err)
		}
	}
}

// recordedPaths is the surface paths of the ownership record at path.
func recordedPaths(t *testing.T, path string) []string {
	t.Helper()
	rec, err := reposeed.LoadRecord(path)
	if err != nil || rec == nil {
		t.Fatalf("LoadRecord(%s) = %v, %v; want a record", path, rec, err)
	}
	var out []string
	for _, s := range rec.Surfaces {
		out = append(out, s.Path)
	}
	return out
}

const dispatchStartMarker = "<!-- docket:dispatch:start"

// TestIntegrationPrivateInstructionsInitWritesPrivateFileOnly is acceptance 1:
// with every harness opted in, private init writes the dispatch block (Codex
// clause included) into .git/dckt/AGENTS.md and nothing else — no working-tree
// file, no .git/info/exclude edit, no pending review path.
func TestIntegrationPrivateInstructionsInitWritesPrivateFileOnly(t *testing.T) {
	r, primary, common := newPrivateInstructionsRepo(t)
	excludePath := filepath.Join(common, "info", "exclude")
	exclude := mustReadFile(t, excludePath)

	setPrivateHarnesses(t, common, "[claude, codex, cursor, opencode]")
	res := r.runInitWith(t, InitOptions{})
	if res.Result != ResultApplied {
		t.Fatalf("init with harnesses = %q (%s), want applied", res.Result, res.HumanText())
	}

	private := string(mustReadFile(t, layout.PrivateInstructionsPath(common)))
	if !strings.Contains(private, dispatchStartMarker) || !strings.Contains(private, "### Codex root-coordinator entry") {
		t.Errorf("%s = %q, want a docket:dispatch block carrying the Codex clause", layout.PrivateInstructionsDisplay, private)
	}
	assertNoWorktreeSurfaces(t, primary)
	if st := runGit(t, r.invocation, "status", "--porcelain"); st != "" {
		t.Errorf("git status --porcelain = %q, want empty", st)
	}
	if got := mustReadFile(t, excludePath); string(got) != string(exclude) {
		t.Errorf(".git/info/exclude changed:\nbefore %q\nafter  %q", exclude, got)
	}
	for _, p := range res.PendingPaths {
		if strings.HasSuffix(p, "AGENTS.md") || strings.HasSuffix(p, "CLAUDE.md") || strings.Contains(p, ".cursor") || strings.HasPrefix(p, ".git/") {
			t.Errorf("PendingPaths = %v, names surface %q; a private repository has no pending surface", res.PendingPaths, p)
		}
	}
	if got := recordedPaths(t, reposeed.RecordPath(common, layout.PrivateName)); len(got) != 1 || got[0] != ".git/dckt/AGENTS.md" {
		t.Errorf("ownership record surfaces = %v, want only .git/dckt/AGENTS.md", got)
	}
}

// TestIntegrationPrivateInstructionsInstallPhaseFromFeatureWorktree proves the
// repository phase resolved from a linked worktree (a feature worktree, and the
// metadata checkout) plans exactly the private file against the primary
// worktree and records at the common dir.
func TestIntegrationPrivateInstructionsInstallPhaseFromFeatureWorktree(t *testing.T) {
	r, primary, common := newPrivateInstructionsRepo(t)
	setPrivateHarnesses(t, common, "[claude, cursor]")
	feature := filepath.Join(r.root, "feature")
	runGit(t, r.invocation, "worktree", "add", "-q", "-b", "feat/x", feature)
	metadata := privateLayoutOf(t, r.invocation, os.Getenv("XDG_DATA_HOME")).MetadataWorktree
	if _, err := os.Stat(metadata); err != nil {
		t.Fatalf("precondition: metadata checkout %s: %v", metadata, err)
	}

	rt, err := buildRunTracker()
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{feature, metadata} {
		phase, gotRoot, _, err := ResolveRepoPhase(context.Background(), newGitClient(t), dir, nil, rt, nil, config.ResolveContext{DefaultBranch: "main"})
		if err != nil {
			t.Fatalf("ResolveRepoPhase(%s): %v", dir, err)
		}
		if phase == nil || !phase.Authorized {
			t.Fatalf("ResolveRepoPhase(%s) = %+v, want an authorized phase", dir, phase)
		}
		if got, want := targetPaths(phase), layout.PrivateInstructionsPath(common); len(got) != 1 || got[0] != want {
			t.Errorf("from %s: targets = %v, want exactly [%s]", dir, got, want)
		}
		if want := filepath.Join(common, layout.PrivateName, "install.json"); phase.RecordPath != want {
			t.Errorf("from %s: record path = %q, want %q", dir, phase.RecordPath, want)
		}
		if gotRoot != primary || phase.Worktree != primary {
			t.Errorf("from %s: worktree = %q / %q, want the primary %q", dir, gotRoot, phase.Worktree, primary)
		}
	}
}

// TestIntegrationPrivateInstructionsRetiresBuggyWorktreeSurfaces proves an
// install over a private repository retires the working-tree surfaces an
// earlier install planned there (the shared plan, recorded at
// <common>/dckt/install.json) and writes the private file instead.
func TestIntegrationPrivateInstructionsRetiresBuggyWorktreeSurfaces(t *testing.T) {
	_, primary, common := newPrivateInstructionsRepo(t)
	harnesses := []string{"claude", "codex", "cursor"}
	setPrivateHarnesses(t, common, "[claude, codex, cursor]")

	rt, err := buildRunTracker()
	if err != nil {
		t.Fatal(err)
	}
	targets, owners, err := reposeed.Plan(reposeed.PlanInput{
		WorktreeRoot: primary, Harnesses: harnesses, RunTracker: rt, ClaudeMDState: reposeed.ClaudeMDAbsent,
	})
	if err != nil {
		t.Fatal(err)
	}
	desired, err := reposeed.DesiredRecord(targets, owners, primary)
	if err != nil {
		t.Fatal(err)
	}
	recordBytes, err := reposeed.EncodeRecord(desired)
	if err != nil {
		t.Fatal(err)
	}
	roots, err := install.ResolveRoots(os.UserHomeDir, os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := applyRepoPhaseSurfaces(&install.RepoPhase{
		Authorized: true, Targets: targets, Owners: owners,
		RecordPath: reposeed.RecordPath(common, layout.PrivateName), RecordBytes: recordBytes, Worktree: primary,
	}, roots); err != nil {
		t.Fatalf("seeding the old working-tree surfaces: %v", err)
	}
	buggy := []string{"AGENTS.md", "CLAUDE.md", filepath.Join(".cursor", "rules", "docket-dispatch.mdc")}
	for _, rel := range buggy {
		if _, err := os.Lstat(filepath.Join(primary, rel)); err != nil {
			t.Fatalf("precondition: %s not seeded: %v", rel, err)
		}
	}

	// A run scoped to a harness that owns none of them leaves them alone: no
	// removal, and every surface carried in the record untouched.
	scoped, _, _, err := ResolveRepoPhase(context.Background(), newGitClient(t), primary, []string{"opencode"}, rt, nil, config.ResolveContext{DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("scoped ResolveRepoPhase: %v", err)
	}
	if len(scoped.Removals) != 0 || string(scoped.RecordBytes) != string(recordBytes) {
		t.Errorf("scoped to opencode: removals %v, record %s; want none and the record unchanged", scoped.Removals, scoped.RecordBytes)
	}

	if pending := installSurfacesIn(t, primary); len(pending) != 0 {
		t.Errorf("pending paths = %v, want none in a private repository", pending)
	}

	// The CLAUDE.md link and the Cursor rule are wholly docket's and are deleted.
	// AGENTS.md is a managed block inside a file the user may also own, so its
	// retirement cuts only the block and keeps the file and every other byte.
	for _, rel := range buggy[1:] {
		if _, err := os.Lstat(filepath.Join(primary, rel)); !os.IsNotExist(err) {
			t.Errorf("%s survived the install (err=%v); want it retired", rel, err)
		}
	}
	// The seeded file held only the block, so nothing of it is left.
	if agents, err := os.ReadFile(filepath.Join(primary, "AGENTS.md")); (err != nil && !os.IsNotExist(err)) || len(agents) != 0 {
		t.Errorf("AGENTS.md = %q (%v) after the install; want its dispatch block retired and nothing left", agents, err)
	}
	if private := string(mustReadFile(t, layout.PrivateInstructionsPath(common))); !strings.Contains(private, dispatchStartMarker) {
		t.Errorf("%s = %q, want the dispatch block", layout.PrivateInstructionsDisplay, private)
	}
	if got := recordedPaths(t, reposeed.RecordPath(common, layout.PrivateName)); len(got) != 1 || got[0] != ".git/dckt/AGENTS.md" {
		t.Errorf("ownership record surfaces = %v, want only .git/dckt/AGENTS.md", got)
	}
}

// TestIntegrationPrivateInstructionsOwnerRemovalKeepsSharedFile proves the
// private file is shared by every opted-in harness: dropping codex rewrites the
// block with the plain interior, dropping the last harness retires the block,
// and a lesson the user added outside the block survives both.
func TestIntegrationPrivateInstructionsOwnerRemovalKeepsSharedFile(t *testing.T) {
	_, primary, common := newPrivateInstructionsRepo(t)
	privatePath := layout.PrivateInstructionsPath(common)
	const lesson = "\n## A promoted lesson\n\nAlways run the focused tests first.\n"

	setPrivateHarnesses(t, common, "[claude, codex]")
	installSurfacesIn(t, primary)
	f, err := os.OpenFile(privatePath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(lesson); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if got := string(mustReadFile(t, privatePath)); !strings.Contains(got, "### Codex root-coordinator entry") {
		t.Fatalf("precondition: %s = %q, want the Codex clause", layout.PrivateInstructionsDisplay, got)
	}

	setPrivateHarnesses(t, common, "[claude]")
	installSurfacesIn(t, primary)
	got := string(mustReadFile(t, privatePath))
	if !strings.Contains(got, dispatchStartMarker) || strings.Contains(got, "### Codex root-coordinator entry") || !strings.Contains(got, lesson) {
		t.Errorf("after dropping codex: %s = %q, want the plain block and the lesson", layout.PrivateInstructionsDisplay, got)
	}

	setPrivateHarnesses(t, common, "[]")
	installSurfacesIn(t, primary)
	got = string(mustReadFile(t, privatePath))
	if strings.Contains(got, "docket:dispatch") || !strings.Contains(got, lesson) {
		t.Errorf("after dropping every harness: %s = %q, want the block retired and the lesson kept", layout.PrivateInstructionsDisplay, got)
	}
	assertNoWorktreeSurfaces(t, primary)
}

// TestIntegrationPrivateInstructionsSharedRepositoryUnchanged is acceptance 7:
// a shared init writes the repository-level AGENTS.md block and the CLAUDE.md
// link, and no private file (no .git/dckt at all).
func TestIntegrationPrivateInstructionsSharedRepositoryUnchanged(t *testing.T) {
	pinInstructionsUserRoots(t)
	t.Setenv("XDG_DATA_HOME", testsupport.TempDir(t))
	r := newInitRepo(t, defaultSetupYML+"agent_harnesses: [claude, codex]\n", nil)
	if res := r.runInit(t); res.Result != ResultApplied {
		t.Fatalf("shared init = %q (%s), want applied", res.Result, res.HumanText())
	}
	if agents := string(mustReadFile(t, filepath.Join(r.invocation, "AGENTS.md"))); !strings.Contains(agents, dispatchStartMarker) {
		t.Errorf("AGENTS.md = %q, want the dispatch block", agents)
	}
	link, lerr := os.Lstat(filepath.Join(r.invocation, "CLAUDE.md"))
	via, verr := os.Stat(filepath.Join(r.invocation, "CLAUDE.md"))
	agents, aerr := os.Stat(filepath.Join(r.invocation, "AGENTS.md"))
	if lerr != nil || verr != nil || aerr != nil || link.Mode()&os.ModeSymlink == 0 || !os.SameFile(via, agents) {
		t.Errorf("CLAUDE.md is not a link to AGENTS.md (errs %v, %v, %v)", lerr, verr, aerr)
	}
	if _, err := os.Lstat(filepath.Join(r.gitDir(t), layout.PrivateName)); !os.IsNotExist(err) {
		t.Errorf(".git/dckt exists in a shared repository (err=%v)", err)
	}
}
