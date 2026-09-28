package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

// The Prepare tests exercise the fresh-allocation path against real temporary
// Git repositories with local bare remotes, on both the plain and the
// docket-style topologies. Existing/resume/blocked arms are Task 6.

// prepSlug is the feature slug every fresh-allocation scenario prepares. Its
// derived feature ref is refs/heads/feat/<prepSlug>.
const prepSlug = "fix-the-thing"

func prepFeatureRef() gitcli.RefName { return gitcli.RefName("refs/heads/feat/" + prepSlug) }

// resolveBase wires a real domain.ResolveEffectiveBase outcome — proving the
// service consumes the resolver rather than shadowing its rules — and asserts
// the outcome actually resolved (a fixture bug otherwise).
func resolveBase(t *testing.T, specs []domain.ChangeSpec, branches []string, subject domain.ChangeID) domain.EffectiveBase {
	t.Helper()
	changes := make([]domain.Change, 0, len(specs))
	for _, sp := range specs {
		changes = append(changes, domain.NewChange(sp))
	}
	snap := domain.NewSnapshot(domain.SnapshotSpec{
		Policy:  domain.RepositoryPolicy{IntegrationBranch: "main"},
		Changes: changes,
	})
	set := make(map[string]bool, len(branches))
	for _, b := range branches {
		set[b] = true
	}
	facts := domain.NewBranchFacts(set)
	c, out := snap.Change(subject)
	if out != domain.LookupFound {
		t.Fatalf("Change(%d) = %v; want found", subject, out)
	}
	base := domain.ResolveEffectiveBase(snap, c, facts)
	if base.Kind != domain.BaseResolved {
		t.Fatalf("resolver base kind = %q; want resolved (fixture bug)", base.Kind)
	}
	return base
}

// assertNothingCreated proves a rejected Prepare left no checkout, no local
// feature branch, and no manifest under the real repository.
func assertNothingCreated(t *testing.T, r *wsRepos, commonDir string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(r.Primary, ".worktrees", prepSlug)); !os.IsNotExist(err) {
		t.Errorf(".worktrees/%s exists or is unstattable (%v); want absent", prepSlug, err)
	}
	if branchExists(r.Primary, "feat/"+prepSlug) {
		t.Errorf("local branch feat/%s exists; want absent", prepSlug)
	}
	if commonDir != "" {
		if _, present, err := loadManifest(workspaceDir(commonDir, prepFeatureRef())); err != nil || present {
			t.Errorf("manifest present=%v err=%v; want cleanly absent", present, err)
		}
	}
}

// assertFreshCreated asserts the full created-workspace postcondition: the
// returned disposition and facts, live Git registration at the canonical path on
// the feature branch whose tip is wantBase, and a ready manifest recording the
// exact base commit.
func assertFreshCreated(t *testing.T, r *wsRepos, repo gitcli.Repository, ws Workspace, wantBase gitcli.ObjectID) {
	t.Helper()
	if ws.Disposition != PrepareCreated {
		t.Fatalf("Disposition = %q; want %q", ws.Disposition, PrepareCreated)
	}
	wantPath := filepath.Join(repo.PrimaryWorktree, ".worktrees", prepSlug)
	if ws.Path != wantPath {
		t.Errorf("Path = %q; want %q", ws.Path, wantPath)
	}
	if ws.FeatureRef != prepFeatureRef() {
		t.Errorf("FeatureRef = %q; want %q", ws.FeatureRef, prepFeatureRef())
	}
	if ws.BaseCommit != wantBase {
		t.Errorf("BaseCommit = %q; want %q", ws.BaseCommit, wantBase)
	}
	if ws.HeadCommit != wantBase {
		t.Errorf("HeadCommit = %q; want %q (fresh head == base)", ws.HeadCommit, wantBase)
	}
	if ws.Dirty {
		t.Errorf("Dirty = true; want false on a fresh checkout")
	}

	// Live Git: registered at the canonical path, symbolic HEAD is the feature
	// ref, and the branch tip equals the fetched base.
	if got := symbolicHead(t, wantPath); got != string(prepFeatureRef()) {
		t.Errorf("workspace symbolic HEAD = %q; want %q", got, prepFeatureRef())
	}
	if got := gitcli.ObjectID(gitOut(t, r.Primary, "rev-parse", string(prepFeatureRef()))); got != wantBase {
		t.Errorf("branch tip = %q; want %q", got, wantBase)
	}
	wl := gitOut(t, r.Primary, "worktree", "list", "--porcelain")
	if !containsWorktreePath(t, wl, wantPath) {
		t.Errorf("worktree list does not register %q:\n%s", wantPath, wl)
	}

	// Manifest advanced to ready with the exact base commit.
	m, present, err := loadManifest(workspaceDir(repo.CommonDir, prepFeatureRef()))
	if err != nil || !present {
		t.Fatalf("loadManifest present=%v err=%v; want present", present, err)
	}
	if m.Phase != PhaseReady {
		t.Errorf("manifest phase = %q; want ready", m.Phase)
	}
	if m.BaseCommit != wantBase {
		t.Errorf("manifest BaseCommit = %q; want %q", m.BaseCommit, wantBase)
	}
	if m.Path != wantPath {
		t.Errorf("manifest Path = %q; want %q", m.Path, wantPath)
	}
}

// containsWorktreePath reports whether the porcelain worktree list registers a
// worktree whose canonical path equals want.
func containsWorktreePath(t *testing.T, porcelain, want string) bool {
	t.Helper()
	for _, line := range splitLines(porcelain) {
		if p, ok := cutPrefix(line, "worktree "); ok {
			cp, err := filepath.EvalSymlinks(p)
			if err != nil {
				continue
			}
			if cp == want {
				return true
			}
		}
	}
	return false
}

func splitLines(s string) []string {
	var out []string
	cur := ""
	for _, c := range s {
		if c == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(c)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func cutPrefix(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):], true
	}
	return "", false
}

// ---------------------------------------------------------------------------
// Task 6: existing / resume / blocked matrix.
//
// These tests construct the on-disk states a crash or a collision leaves and
// assert Prepare's disposition and its byte-for-byte preservation guarantees.
// Blocked is a value disposition (PrepareBlocked, nil error); a probe that
// cannot see a resource is an error. Manual manifests are published through the
// production writeManifest so a constructed state is one loadManifest accepts.
// ---------------------------------------------------------------------------

// countWorktreePathOccurrences counts how many registered worktrees resolve to
// want, canonicalizing each porcelain path through every symlink hop.
func countWorktreePathOccurrences(porcelain, want string) int {
	n := 0
	for _, line := range splitLines(porcelain) {
		p, ok := cutPrefix(line, "worktree ")
		if !ok {
			continue
		}
		if canon, err := filepath.EvalSymlinks(p); err == nil && canon == want {
			n++
		}
	}
	return n
}

// freshTarget builds the unstacked target every matrix scenario prepares.
func freshTarget(t *testing.T, id int) Target {
	t.Helper()
	base := resolveBase(t, []domain.ChangeSpec{{ID: domain.ChangeID(id), Status: domain.StatusProposed}}, nil, domain.ChangeID(id))
	tgt, err := NewTarget(domain.ChangeID(id), prepSlug, base, "feat/"+prepSlug)
	if err != nil {
		t.Fatalf("NewTarget: %v", err)
	}
	return tgt
}

// prepareOK runs Prepare and fails the test on any error, returning the result.
func prepareOK(t *testing.T, svc *Service, repo gitcli.Repository, tgt Target) Workspace {
	t.Helper()
	ws, err := svc.Prepare(context.Background(), PrepareRequest{Repository: repo, Remote: "origin", Target: tgt})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	return ws
}

// wsPathOf is the canonical checkout path a target's workspace lands at.
func wsPathOf(repo gitcli.Repository) string {
	return filepath.Join(repo.PrimaryWorktree, ".worktrees", prepSlug)
}

// metaDirOf is the hashed workspace metadata directory for a target.
func metaDirOf(repo gitcli.Repository, tgt Target) string {
	return workspaceDir(repo.CommonDir, tgt.FeatureRef)
}

// writeStateManifest publishes a manifest in the target's metadata directory at
// the requested phase and recorded base, via the production writer. It is how a
// crash-left partial state is constructed for the resume tests.
func writeStateManifest(t *testing.T, repo gitcli.Repository, tgt Target, base gitcli.ObjectID, phase Phase) {
	t.Helper()
	m := Manifest{
		Schema:     manifestSchemaVersion,
		ID:         workspaceID(tgt.FeatureRef),
		CommonDir:  repo.CommonDir,
		ChangeID:   tgt.ChangeID,
		Slug:       tgt.Slug,
		FeatureRef: tgt.FeatureRef,
		BaseRef:    tgt.BaseRef,
		BaseCommit: base,
		Path:       wsPathOf(repo),
		Phase:      phase,
		CreatedUTC: time.Now().UTC().Format(time.RFC3339),
		UpdatedUTC: time.Now().UTC().Format(time.RFC3339),
	}
	if err := writeManifest(metaDirOf(repo, tgt), m); err != nil {
		t.Fatalf("writeManifest: %v", err)
	}
}

// localBranchTip returns refs/heads/feat/<slug>'s tip in the primary clone.
func localBranchTip(t *testing.T, r *wsRepos) gitcli.ObjectID {
	t.Helper()
	return gitcli.ObjectID(gitOut(t, r.Primary, "rev-parse", string(prepFeatureRef())))
}

// readFileBytes reads a file as a string, failing the test on error.
func readFileBytes(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// freshTopology rebuilds the same fixture kind as r for an isolated subtest, so
// each blocked-matrix case runs against its own repository.
func freshTopology(t *testing.T, r *wsRepos) *wsRepos {
	t.Helper()
	if len(r.Preserve) > 1 {
		return docketModeRepo(t)
	}
	return mainModeRepo(t)
}

// assertBlocked runs Prepare and asserts a PrepareBlocked disposition with no error.
func assertBlocked(t *testing.T, svc *Service, repo gitcli.Repository, tgt Target) {
	t.Helper()
	out, err := svc.Prepare(context.Background(), PrepareRequest{Repository: repo, Remote: "origin", Target: tgt})
	if err != nil {
		t.Fatalf("Prepare = error %v; want blocked disposition", err)
	}
	if out.Disposition != PrepareBlocked {
		t.Errorf("disposition = %q; want blocked", out.Disposition)
	}
}

// assertNoManifest asserts Prepare published no manifest of its own.
func assertNoManifest(t *testing.T, repo gitcli.Repository, tgt Target) {
	t.Helper()
	if _, present, err := loadManifest(metaDirOf(repo, tgt)); err != nil || present {
		t.Errorf("manifest present=%v err=%v; want cleanly absent (none published)", present, err)
	}
}

// countPathOccurrences counts porcelain "worktree <path>" lines whose canonical
// path equals want.
func countPathOccurrences(porcelain, want string) int {
	n := 0
	for _, line := range splitLines(porcelain) {
		if p, ok := cutPrefix(line, "worktree "); ok {
			if cp, err := filepath.EvalSymlinks(p); err == nil && cp == want {
				n++
			}
		}
	}
	return n
}

// writeFailingGit writes an executable git wrapper that forwards to the real git
// on PATH except for the named subcommand, which it fails with exit 1. The
// wrapper is invoked by absolute path, so PATH still resolves the real git.
func writeFailingGit(t *testing.T, failSubcommand string) string {
	t.Helper()
	dir := testsupport.TempDir(t)
	p := filepath.Join(dir, "git")
	script := "#!/bin/sh\nif [ \"$1\" = \"" + failSubcommand + "\" ]; then\n  echo \"fake git: $1 disabled for test\" >&2\n  exit 1\nfi\nexec git \"$@\"\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// newServiceWithGit builds a Service whose gitcli.Client uses the given git
// executable, discovering the canonical Repository through that same client.
func (r *wsRepos) newServiceWithGit(t *testing.T, exe string) (*Service, gitcli.Repository) {
	t.Helper()
	c, err := gitcli.NewClient(gitcli.WithExecutable(exe))
	if err != nil {
		t.Fatalf("gitcli.NewClient(WithExecutable): %v", err)
	}
	svc, err := NewService(c)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	repo, err := c.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: r.Primary})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	return svc, repo
}

// TestClassifyRegistrationAbsence pins the fail-closed three-outcome
// registration probe (change 0368). It needs no git: it drives the pure
// classifier over synthetic []gitcli.WorktreeInfo. Two intended-path shapes are
// exercised: an existing canonical dir (a live registration at it canonicalizes
// and matches) and a not-yet-created canonical leaf (mirroring intendedPath for
// an unallocated worktree, where an exact registration fails canonicalization
// and must match lexically as a stale registration).
func TestClassifyRegistrationAbsence(t *testing.T) {
	feature := gitcli.RefName("refs/heads/feat/absent-probe")
	other := gitcli.RefName("refs/heads/feat/other")

	existingWant, err := canonicalizePath(testsupport.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	missingParent, err := canonicalizePath(testsupport.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	missingWant := filepath.Join(missingParent, "ws")         // never created
	gone := filepath.Join(testsupport.TempDir(t), "vanished") // never created: canonicalization fails, cleans to != want

	cases := []struct {
		name  string
		infos []gitcli.WorktreeInfo
		want  string
		exp   registrationAbsence
	}{
		{"empty list is absent", nil, existingWant, regAbsent},
		{"unrelated registration is absent", []gitcli.WorktreeInfo{{Path: testsupport.TempDir(t), Branch: other}}, existingWant, regAbsent},
		{"live registration at the path is present", []gitcli.WorktreeInfo{{Path: existingWant, Branch: other}}, existingWant, regPresent},
		{"registration on the feature ref elsewhere is present", []gitcli.WorktreeInfo{{Path: testsupport.TempDir(t), Branch: feature}}, existingWant, regPresent},
		{"stale registration at the exact path is present", []gitcli.WorktreeInfo{{Path: missingWant, Branch: other}}, missingWant, regPresent},
		{"unresolvable unrelated registration is unresolved", []gitcli.WorktreeInfo{{Path: gone, Branch: other}}, existingWant, regUnresolved},
		{"present beats unresolved", []gitcli.WorktreeInfo{{Path: gone, Branch: other}, {Path: existingWant, Branch: other}}, existingWant, regPresent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyRegistrationAbsence(tc.infos, tc.want, feature); got != tc.exp {
				t.Errorf("classifyRegistrationAbsence = %v; want %v", got, tc.exp)
			}
		})
	}
}
