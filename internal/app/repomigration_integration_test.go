//go:build integration

package app

import (
	"context"
	"fmt"
	"github.com/danielhanold/docket/internal/testsupport"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/reposetup"
)

// This is the real-Git migration integration shard (prefix
// TestIntegrationRepoMigration). Each test scripts its own legacy single-branch
// repository (a bare origin + a writer clone that seeds it + an invocation clone
// migrate runs against) under testsupport.TempDir(t), drives RunRepositoryMigrate, and
// inspects the authoritative remote docket and integration branches with an
// independent git oracle. The fixtures reuse the established internal/app harness
// (runGit, writeRepoFile, newGitClient, the initRepo shape from
// reposetup_integration_test.go).

// --- fixtures ----------------------------------------------------------------

// legacyDocketYML is a legacy Bash-era configuration: it carries the
// metadata_branch key the migration removes, plus a comment on that key and
// other keys/comments the byte-preserving removal must leave untouched.
const legacyDocketYML = "# legacy docket configuration\n" +
	"metadata_branch: docket   # removed by migration\n" +
	"integration_branch: main\n" +
	"results_dir: docs/results\n" +
	"# trailing comment preserved verbatim\n"

// cleanLegacyFiles is a legacy planning surface with valid records that need NO
// mechanical repair, so --yes alone authorizes the migration and the source→prune
// diff is exactly the copy/removal sets with no repair noise.
func cleanLegacyFiles() map[string]string {
	return map[string]string{
		// Active surface (the live planning surface migration prunes from
		// integration). Non-terminal, since a terminal change in active/ is a
		// placement error.
		"docs/changes/active/0001-first-change.md":  migChangeRecord(1, "first-change", "proposed", "depends_on: [3]"),
		"docs/changes/active/0002-second-change.md": migChangeRecord(2, "second-change", "proposed", ""),
		"docs/changes/active/stray-note.txt":        "an unknown stray file that must be loss-preserved into the seed\n",
		// Archive (a terminal record; the archive is retained on integration).
		"docs/changes/archive/2026-01-02-0003-archived-change.md": migArchivedRecord(3, "archived-change", "done", ""),
		// Learnings dir present as a stray (an authored learning would add its own
		// filename grammar; the whole prefix is copied either way).
		"docs/changes/learnings/notes.txt": "loss-preserved learnings note\n",
		"docs/changes/BOARD.md":            "# Board\n\nlegacy board content\n",
		"docs/changes/README.md":           "# Changes\n\nlegacy entry-point readme\n",
		// ADR ledger (copied and validated).
		"docs/adrs/0001-first-decision.md": "---\nid: 1\nslug: first-decision\nstatus: Accepted\ntitle: First decision\n---\nContext.\n",
		// Specs (copied; no config key — the convention constant).
		"docs/superpowers/specs/2026-01-01-a-spec.md": "# A spec\n\nspec content\n",
		"docs/superpowers/specs/stray-spec.txt":       "unknown stray inside specs, loss-preserved\n",
		// Outside the copy set: plans and results are NOT copied to the metadata
		// branch and NOT removed from integration.
		"docs/superpowers/plans/2026-01-01-a-plan.md": "# A plan\n\nplan content\n",
		"docs/results/0001-first-change.md":           "# Results for 0001\n\nresults content\n",
	}
}

// changeRecord renders a minimal valid ACTIVE change record. extra is an
// optional additional frontmatter line (e.g. a list field).
func migChangeRecord(id int, slug, status, extra string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("id: %d\n", id))
	b.WriteString("slug: " + slug + "\n")
	b.WriteString("status: " + status + "\n")
	b.WriteString("title: Change " + slug + "\n")
	b.WriteString("type: feature\n")
	if extra != "" {
		b.WriteString(extra + "\n")
	}
	b.WriteString("---\n\nBody for " + slug + ".\n")
	return b.String()
}

// archivedRecord renders a minimal valid ARCHIVED (terminal) change record.
func migArchivedRecord(id int, slug, status, extra string) string {
	return migChangeRecord(id, slug, status, extra)
}

// --- oracle helpers over the bare origin -------------------------------------

// originTip resolves a branch tip on the bare origin.
func (r *initRepo) originTip(t *testing.T, branch string) string {
	t.Helper()
	return runGit(t, r.origin, "rev-parse", "refs/heads/"+branch)
}

// originTreePaths lists every path in a branch's tree on the bare origin, sorted.
func (r *initRepo) originTreePaths(t *testing.T, branch string) []string {
	t.Helper()
	out := runGit(t, r.origin, "ls-tree", "-r", "--name-only", "refs/heads/"+branch)
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			paths = append(paths, line)
		}
	}
	sort.Strings(paths)
	return paths
}

// originBlob reads a path's bytes from a branch on the bare origin.
func (r *initRepo) originBlob(t *testing.T, branch, path string) (string, bool) {
	t.Helper()
	out, err := tryGit(r.origin, "show", "refs/heads/"+branch+":"+path)
	if err != nil {
		return "", false
	}
	return out, true
}

// originTrailers reads a commit's trailers on the bare origin.
func (r *initRepo) originTrailers(t *testing.T, rev string) string {
	t.Helper()
	return runGit(t, r.origin, "log", "-1", "--format=%(trailers:only,unfold)", rev)
}

// runMigrate drives RunRepositoryMigrate against the invocation clone.
func (r *initRepo) runMigrate(t *testing.T, o MigrateOptions) RepositoryMigrateResult {
	t.Helper()
	client := newGitClient(t)
	return RunRepositoryMigrate(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation}, o)
}

// runMigrateWithHooks drives RunRepositoryMigrate with the generalized
// interruption seam installed — the in-package channel package tests use to
// crash a run between two durable Git effects.
func (r *initRepo) runMigrateWithHooks(t *testing.T, o MigrateOptions, hooks setupHooks) RepositoryMigrateResult {
	t.Helper()
	client := newGitClient(t)
	return RunRepositoryMigrate(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation, hooks: hooks}, o)
}

// --- scenarios ---------------------------------------------------------------

// TestIntegrationRepoMigrationExactCopyAndRemovalSets proves the seed tree is
// exactly the three copy prefixes (including unknown stray files) and the pruned
// integration descendant differs from its source by exactly the removal set plus
// the config edit and the managed gitignore establishment — nothing else.
func TestIntegrationRepoMigrationExactCopyAndRemovalSets(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	sourceTip := r.originTip(t, "main")

	res := r.runMigrate(t, MigrateOptions{Authorized: true})
	if res.Result != ResultApplied {
		t.Fatalf("migrate = %q (%s), want applied", res.Result, res.HumanText())
	}

	// The metadata (docket) seed tree is exactly the copy set.
	seedPaths := r.originTreePaths(t, "docket")
	for _, want := range []string{
		"docs/changes/active/0001-first-change.md",
		"docs/changes/active/stray-note.txt",
		"docs/changes/archive/2026-01-02-0003-archived-change.md",
		"docs/changes/learnings/notes.txt",
		"docs/changes/BOARD.md",
		"docs/changes/README.md",
		"docs/adrs/0001-first-decision.md",
		"docs/superpowers/specs/2026-01-01-a-spec.md",
		"docs/superpowers/specs/stray-spec.txt",
	} {
		if !contains(seedPaths, want) {
			t.Errorf("seed tree missing copied path %q; got %v", want, seedPaths)
		}
	}
	for _, unwanted := range []string{
		"docs/superpowers/plans/2026-01-01-a-plan.md",
		"docs/results/0001-first-change.md",
		".docket.yml",
		"README.md",
		".gitignore",
	} {
		if contains(seedPaths, unwanted) {
			t.Errorf("seed tree carries a path outside the copy set: %q", unwanted)
		}
	}

	// The pruned integration descendant differs from its source by EXACTLY the
	// removal set (removed), the config edit (.docket.yml modified), and the
	// managed gitignore establishment (.gitignore added).
	prunePaths := r.originTreePaths(t, "main")
	changed := changedPathSet(t, r, sourceTip, r.originTip(t, "main"))
	wantChanged := map[string]bool{
		"docs/changes/active/0001-first-change.md":  true, // removed
		"docs/changes/active/0002-second-change.md": true, // removed
		"docs/changes/active/stray-note.txt":        true, // removed with the prefix
		"docs/changes/BOARD.md":                     true, // removed
		"docs/changes/README.md":                    true, // removed
		".docket.yml":                               true, // legacy key removed
		".gitignore":                                true, // managed block established
	}
	if !sameStringSet(changed, wantChanged) {
		t.Errorf("integration source→prune changed-path set = %v, want exactly %v", changed, keysOf(wantChanged))
	}

	// The retained surface is still present on integration.
	for _, want := range []string{
		"docs/changes/archive/2026-01-02-0003-archived-change.md",
		"docs/adrs/0001-first-decision.md",
		"docs/superpowers/specs/2026-01-01-a-spec.md",
		"docs/superpowers/plans/2026-01-01-a-plan.md",
		"docs/results/0001-first-change.md",
	} {
		if !contains(prunePaths, want) {
			t.Errorf("integration descendant dropped a retained path %q", want)
		}
	}
	// The active surface, board, and README are gone from integration.
	for _, gone := range []string{
		"docs/changes/active/0001-first-change.md",
		"docs/changes/BOARD.md",
		"docs/changes/README.md",
	} {
		if contains(prunePaths, gone) {
			t.Errorf("integration descendant still carries pruned path %q", gone)
		}
	}
}

// TestIntegrationRepoMigrationLegacyKeyRemovedBytePreserving proves the
// .docket.yml on the pruned integration byte-preserves the source (only the
// metadata_branch key line removed — every comment, key, and ordering byte
// preserved) and then folds in the generated test policy: the clean legacy tree
// has no recognizable suite, so both gates are declared off (change 0374).
func TestIntegrationRepoMigrationLegacyKeyRemovedBytePreserving(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	r.runMigrate(t, MigrateOptions{Authorized: true})

	got, ok := r.originBlob(t, "main", ".docket.yml")
	if !ok {
		t.Fatal(".docket.yml missing from the pruned integration")
	}
	// The metadata_branch removal is byte-preserving: the source with only that
	// line gone is the exact prefix of the result.
	wantPrefix := strings.Replace(legacyDocketYML, "metadata_branch: docket   # removed by migration\n", "", 1)
	if !strings.HasPrefix(got, wantPrefix) {
		t.Errorf(".docket.yml metadata removal not byte-preserved.\n got: %q\nwant prefix: %q", got, wantPrefix)
	}
	if strings.Contains(got, "metadata_branch") {
		t.Errorf(".docket.yml still carries the metadata_branch key: %q", got)
	}
	// The folded-in test policy: no suite detected → both gates off.
	if strings.Count(got, `gate: "off"`) != 2 {
		t.Errorf(".docket.yml must declare both gates off for a no-suite legacy tree: %q", got)
	}
}

// TestIntegrationRepoMigrationReceiptsNameExactRevisions proves both migration
// commits carry the versioned receipts naming the exact revisions: the seed names
// the source revision, and the prune names the source AND metadata revisions.
func TestIntegrationRepoMigrationReceiptsNameExactRevisions(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	sourceTip := r.originTip(t, "main")
	res := r.runMigrate(t, MigrateOptions{Authorized: true})

	metaTip := r.originTip(t, "docket")
	seedTrailers := r.originTrailers(t, metaTip)
	if !strings.Contains(seedTrailers, reposetup.OpMigrateSeed) {
		t.Errorf("seed trailers %q missing %s", seedTrailers, reposetup.OpMigrateSeed)
	}
	if !strings.Contains(seedTrailers, sourceTip) {
		t.Errorf("seed trailers %q must name the source revision %s", seedTrailers, sourceTip)
	}

	pruneTrailers := r.originTrailers(t, r.originTip(t, "main"))
	if !strings.Contains(pruneTrailers, reposetup.OpMigratePrune) {
		t.Errorf("prune trailers %q missing %s", pruneTrailers, reposetup.OpMigratePrune)
	}
	if !strings.Contains(pruneTrailers, sourceTip) {
		t.Errorf("prune trailers %q must name the source revision %s", pruneTrailers, sourceTip)
	}
	if !strings.Contains(pruneTrailers, metaTip) {
		t.Errorf("prune trailers %q must name the metadata revision %s", pruneTrailers, metaTip)
	}
	if res.SourceRevision != sourceTip {
		t.Errorf("result SourceRevision = %q, want %q", res.SourceRevision, sourceTip)
	}
}

// TestIntegrationRepoMigrationRepairsLandInBothTreesForArchives proves an
// archived record's mechanical repair lands byte-identically in BOTH the seed and
// the retained integration copy, while an active record's repair lands only in
// the seed (its integration copy is pruned).
func TestIntegrationRepoMigrationRepairsLandInBothTreesForArchives(t *testing.T) {
	files := cleanLegacyFiles()
	// An active record and an archived record each carry a scalar-encoded id list
	// that the closed roster converts to an unquoted flow sequence.
	files["docs/changes/active/0001-first-change.md"] = migChangeRecord(1, "first-change", "proposed", "depends_on: 3")
	files["docs/changes/archive/2026-01-02-0003-archived-change.md"] = migArchivedRecord(3, "archived-change", "done", "related: 1")
	r := newInitRepo(t, legacyDocketYML, files)

	res := r.runMigrate(t, MigrateOptions{Authorized: true, RepairAuthorized: true})
	if res.Result != ResultApplied {
		t.Fatalf("migrate = %q (%s), want applied", res.Result, res.HumanText())
	}
	if len(res.Repairs) == 0 {
		t.Fatal("result must report the applied repairs")
	}

	// Active repair is in the seed only.
	seedActive, ok := r.originBlob(t, "docket", "docs/changes/active/0001-first-change.md")
	if !ok {
		t.Fatal("seed missing the active record")
	}
	if !strings.Contains(seedActive, "[3]") {
		t.Errorf("seed active record not repaired to a flow sequence: %q", seedActive)
	}
	if _, present := r.originBlob(t, "main", "docs/changes/active/0001-first-change.md"); present {
		t.Error("the active record must be pruned from integration, not retained")
	}

	// Archived repair is byte-identical in BOTH the seed and the retained
	// integration copy.
	seedArchive, ok := r.originBlob(t, "docket", "docs/changes/archive/2026-01-02-0003-archived-change.md")
	if !ok {
		t.Fatal("seed missing the archived record")
	}
	integrationArchive, ok := r.originBlob(t, "main", "docs/changes/archive/2026-01-02-0003-archived-change.md")
	if !ok {
		t.Fatal("integration missing the retained archived record")
	}
	if seedArchive != integrationArchive {
		t.Errorf("archived repair differs between trees:\nseed: %q\nint:  %q", seedArchive, integrationArchive)
	}
	if !strings.Contains(integrationArchive, "[1]") {
		t.Errorf("archived record not repaired in the retained integration copy: %q", integrationArchive)
	}
}

// TestIntegrationRepoMigrationNonRepairableFindingBlocksBeforeAnyWrite proves a
// non-repairable corpus error blocks the migration before any branch change:
// neither remote branch is created or moved.
func TestIntegrationRepoMigrationNonRepairableFindingBlocksBeforeAnyWrite(t *testing.T) {
	files := cleanLegacyFiles()
	// A dangling depends_on reference is a decodable but domain-invalid record the
	// closed repair roster deliberately will not touch.
	files["docs/changes/active/0001-first-change.md"] = migChangeRecord(1, "first-change", "proposed", "depends_on: [99]")
	r := newInitRepo(t, legacyDocketYML, files)
	sourceTip := r.originTip(t, "main")

	res := r.runMigrate(t, MigrateOptions{Authorized: true, RepairAuthorized: true})
	if res.Result != ResultInvalidState {
		t.Fatalf("migrate = %q (%s), want invalid-state (blocked)", res.Result, res.HumanText())
	}
	if r.remoteBranchExists(t, "docket") {
		t.Error("a blocked migration created the remote docket branch; nothing must be written")
	}
	if after := r.originTip(t, "main"); after != sourceTip {
		t.Errorf("a blocked migration moved integration from %s to %s", sourceTip, after)
	}
}

// TestIntegrationRepoMigrationAmbiguousTestDiscoveryBlocksBeforeAnyWrite proves
// an ambiguous test-suite discovery (two suite families in the source tree) is a
// typed refusal surfaced BEFORE any remote mutation: the migration is invalid
// state, names the remedy, and creates/moves neither remote branch.
func TestIntegrationRepoMigrationAmbiguousTestDiscoveryBlocksBeforeAnyWrite(t *testing.T) {
	files := cleanLegacyFiles()
	// A Go module AND a Cargo manifest both match — discovery cannot choose one.
	files["go.mod"] = "module example.com/x\n\ngo 1.22\n"
	files["x_test.go"] = "package x\n"
	files["Cargo.toml"] = "[package]\nname = \"x\"\n"
	r := newInitRepo(t, legacyDocketYML, files)
	sourceTip := r.originTip(t, "main")

	res := r.runMigrate(t, MigrateOptions{Authorized: true, RepairAuthorized: true})
	if res.Result != ResultInvalidState {
		t.Fatalf("migrate = %q (%s), want invalid-state (ambiguous discovery)", res.Result, res.HumanText())
	}
	if !strings.Contains(res.HumanText(), "docket repository configure-tests") {
		t.Errorf("ambiguous refusal %q must name the remedy", res.HumanText())
	}
	if r.remoteBranchExists(t, "docket") {
		t.Error("an ambiguous-discovery refusal created the remote docket branch; nothing must be written")
	}
	if after := r.originTip(t, "main"); after != sourceTip {
		t.Errorf("an ambiguous-discovery refusal moved integration from %s to %s", sourceTip, after)
	}
}

// TestIntegrationRepoMigrationNoTerminalPublishResurrection proves the active
// planning surface — the surface terminal-publish operates on — is not
// resurrected onto integration: no path under the active dir survives the prune.
func TestIntegrationRepoMigrationNoTerminalPublishResurrection(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	r.runMigrate(t, MigrateOptions{Authorized: true})

	for _, p := range r.originTreePaths(t, "main") {
		if strings.HasPrefix(p, "docs/changes/active/") {
			t.Errorf("integration still carries an active-surface path after migration: %q", p)
		}
	}
	// The records live only on the metadata branch now.
	if !contains(r.originTreePaths(t, "docket"), "docs/changes/active/0001-first-change.md") {
		t.Error("the active records must live on the metadata branch after migration")
	}
}

// TestIntegrationRepoMigrationMigrateIsIdempotent proves a second authorized run
// is a no-op keyed on the remote postconditions (metadata branch published, no
// live surface on integration) and moves neither remote branch.
func TestIntegrationRepoMigrationMigrateIsIdempotent(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	first := r.runMigrate(t, MigrateOptions{Authorized: true})
	if first.Result != ResultApplied {
		t.Fatalf("first migrate = %q (%s), want applied", first.Result, first.HumanText())
	}
	metaTip := r.originTip(t, "docket")
	intTip := r.originTip(t, "main")

	second := r.runMigrate(t, MigrateOptions{Authorized: true})
	if second.Result != ResultNoOp {
		t.Fatalf("second migrate = %q (%s), want no-op", second.Result, second.HumanText())
	}
	if got := r.originTip(t, "docket"); got != metaTip {
		t.Errorf("idempotent re-run moved the metadata branch from %s to %s", metaTip, got)
	}
	if got := r.originTip(t, "main"); got != intTip {
		t.Errorf("idempotent re-run moved the integration branch from %s to %s", intTip, got)
	}
}

// TestIntegrationRepoMigrationPrimaryFastForwardsHealthy proves a clean primary
// advances in place to the re-read migrated integration tip.
func TestIntegrationRepoMigrationPrimaryFastForwardsHealthy(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	before := runGit(t, r.invocation, "rev-parse", "HEAD")

	res := r.runMigrate(t, MigrateOptions{Authorized: true})
	if res.Result != ResultApplied {
		t.Fatalf("migrate = %q (%s), want applied", res.Result, res.HumanText())
	}
	want := r.originTip(t, "main")
	if want == before {
		t.Fatal("test setup did not publish a descendant integration commit")
	}
	if got := runGit(t, r.invocation, "rev-parse", "HEAD"); got != want {
		t.Errorf("primary HEAD = %s, want migrated integration %s", got, want)
	}
	if res.RepositoryState != string(reposetup.StateHealthy) {
		t.Errorf("RepositoryState = %q, want healthy", res.RepositoryState)
	}
	if len(res.PendingLocal) != 0 {
		t.Errorf("PendingLocal = %v, want no primary-sync remedy", res.PendingLocal)
	}
	if strings.Contains(res.HumanText(), "pending local sync:") {
		t.Errorf("healthy human result carries an empty pending line: %q", res.HumanText())
	}
}

// TestIntegrationRepoMigrationPrimarySyncRefusesUnprovenWorktree proves a
// primary fast-forward requires successful registration discovery: a failed
// list or a list without the primary retains the established manual remedy.
func TestIntegrationRepoMigrationPrimarySyncRefusesUnprovenWorktree(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	repo := r.discoverRepo(t, newGitClient(t))
	source := gitcli.ObjectID(runGit(t, r.invocation, "rev-parse", "HEAD"))
	sc := setupContext{repo: repo, integrationBranch: "main"}
	want := fmt.Sprintf("fast-forward your primary worktree to the migrated integration branch: `git -C %s merge --ff-only origin/main`", repo.PrimaryWorktree)

	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "list failure", body: "exit 1"},
		{name: "primary absent", body: "exit 0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			git := newPrimarySyncFaultGitClient(t, tc.body)
			if got := migratePrimarySyncRemedy(context.Background(), git, sc, source, source); got != want {
				t.Fatalf("migratePrimarySyncRemedy = %q, want exact manual remedy %q", got, want)
			}
		})
	}
}

// newPrimarySyncFaultGitClient intercepts only worktree discovery. A successful
// empty response is valid porcelain with no primary; all other operations use
// the real Git executable, so an unintended fast-forward succeeds and reddens
// TestIntegrationRepoMigrationPrimarySyncRefusesUnprovenWorktree.
func newPrimarySyncFaultGitClient(t *testing.T, listResponse string) *gitcli.Client {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(testsupport.TempDir(t), "git")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"worktree\" ] && [ \"$2\" = \"list\" ]; then\n" +
		"  " + listResponse + "\n" +
		"fi\n" +
		fmt.Sprintf("exec %q \"$@\"\n", realGit)
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	client, err := gitcli.NewClient(gitcli.WithExecutable(wrapper))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// TestIntegrationRepoMigrationPrimaryDirtyAfterPublish proves a dirty primary
// remains untouched and retains the established fast-forward remedy.
func TestIntegrationRepoMigrationPrimaryDirtyAfterPublish(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	before := runGit(t, r.invocation, "rev-parse", "HEAD")
	dirtyPath := filepath.Join(r.invocation, ".docket.yml")

	hooks := setupHooks{beforeLocalFinish: func() error {
		return os.WriteFile(dirtyPath, []byte("uncommitted bytes\n"), 0o644)
	}}
	res := r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, hooks)
	if res.Result != ResultApplied {
		t.Fatalf("migrate = %q (%s), want applied", res.Result, res.HumanText())
	}
	if res.RepositoryState != string(reposetup.StateNeedsReview) {
		t.Errorf("RepositoryState = %q, want needs-review", res.RepositoryState)
	}
	if got := runGit(t, r.invocation, "rev-parse", "HEAD"); got != before {
		t.Errorf("dirty primary HEAD = %s, want source %s", got, before)
	}
	got, err := os.ReadFile(dirtyPath)
	if err != nil || string(got) != "uncommitted bytes\n" {
		t.Fatalf("dirty bytes after migration = %q, %v", got, err)
	}
	primary, err := filepath.EvalSymlinks(r.invocation)
	if err != nil {
		t.Fatal(err)
	}
	wantRemedy := fmt.Sprintf("fast-forward your primary worktree to the migrated integration branch: `git -C %s merge --ff-only origin/main`", primary)
	if len(res.PendingLocal) != 1 || res.PendingLocal[0] != wantRemedy {
		t.Fatalf("PendingLocal = %v, want exact fast-forward remedy %q", res.PendingLocal, wantRemedy)
	}
}

// TestIntegrationRepoMigrationLocalMovedAfterPublish proves that when the local
// primary advances after the remote publication (via the test seam), the remote
// migration is preserved, the result names a pending local synchronization
// remedy, and a retry performs only local work (no remote change).
func TestIntegrationRepoMigrationLocalMovedAfterPublish(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	before := runGit(t, r.invocation, "rev-parse", "HEAD")
	var localTip string

	hooks := setupHooks{beforeLocalFinish: func() error {
		// Advance the local primary between the remote publication and the local
		// fast-forward decision. Returning nil lets the local finish still run and
		// report the pending local sync.
		writeRepoFile(t, r.invocation, "local-only.txt", "advanced after publish\n")
		runGit(t, r.invocation, "add", "--", "local-only.txt")
		runGit(t, r.invocation, "commit", "-q", "-m", "local advance after publish")
		localTip = runGit(t, r.invocation, "rev-parse", "HEAD")
		return nil
	}}

	res := r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, hooks)
	if res.Result != ResultApplied {
		t.Fatalf("migrate = %q (%s), want applied", res.Result, res.HumanText())
	}
	if res.RepositoryState != string(reposetup.StateNeedsReview) {
		t.Errorf("RepositoryState = %q, want needs-review", res.RepositoryState)
	}
	if localTip == before {
		t.Fatal("test setup did not advance the local primary")
	}
	if got := runGit(t, r.invocation, "rev-parse", "HEAD"); got != localTip {
		t.Errorf("local-only commit HEAD = %s, want %s", got, localTip)
	}
	primary, err := filepath.EvalSymlinks(r.invocation)
	if err != nil {
		t.Fatal(err)
	}
	wantMoved := fmt.Sprintf("your local main has moved past the migrated tip; reconcile it: `git -C %s pull --rebase origin main`", primary)
	if len(res.PendingLocal) != 1 || res.PendingLocal[0] != wantMoved {
		t.Fatalf("PendingLocal = %v, want exact pull-rebase remedy %q", res.PendingLocal, wantMoved)
	}
	metaTip := r.originTip(t, "docket")
	intTip := r.originTip(t, "main")

	// Retry: the remote is already migrated, so the retry performs no remote work.
	retry := r.runMigrate(t, MigrateOptions{Authorized: true})
	if retry.Result != ResultNoOp {
		t.Fatalf("retry = %q (%s), want no-op (remote already migrated)", retry.Result, retry.HumanText())
	}
	if got := r.originTip(t, "docket"); got != metaTip {
		t.Errorf("retry moved the metadata branch from %s to %s", metaTip, got)
	}
	if got := r.originTip(t, "main"); got != intTip {
		t.Errorf("retry moved the integration branch from %s to %s", intTip, got)
	}
}

// --- diff helpers ------------------------------------------------------------

// changedPathSet returns the set of paths that differ between two commits on the
// invocation clone (which has both objects), via `git diff --name-only`.
func changedPathSet(t *testing.T, r *initRepo, a, b string) map[string]bool {
	t.Helper()
	// Ensure the invocation clone has both commits (it created b and has a as its
	// own main's former tip; fetch to be safe).
	runGit(t, r.invocation, "fetch", "-q", "origin")
	out := runGit(t, r.invocation, "diff", "--name-only", a, b)
	set := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			set[line] = true
		}
	}
	return set
}

// sameStringSet reports whether two path sets are equal.
func sameStringSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// keysOf renders a set's keys for an error message.
func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// --- healthy-repository derived-view repair (change 0377) --------------------
//
// migrate's second job is authorized mechanical repair of deterministic
// derived-view drift on an ALREADY-HEALTHY repository. These exercise it end to
// end: publish drift onto the healthy metadata branch, preview it, authorize the
// repair, and prove the canonical bytes were recomputed while authored content
// stayed byte-identical.

// staleRepairRecord is a change record with a spec but an EMPTY managed
// artifact-links block (so the canonical render drifts) and a distinctive
// authored sentence the repair must never touch.
const repairAuthoredSentinel = "AUTHORED-PROSE-SENTINEL-do-not-touch"

func staleRepairRecord() string {
	return "---\n" +
		"id: 1\nslug: example\ntitle: Example change\nstatus: proposed\npriority: medium\n" +
		"type: feature\ncreated: 2026-08-30\nupdated: 2026-08-30\n" +
		"spec: docs/superpowers/specs/2026-08-30-example-design.md\n" +
		"---\n\n## Artifacts\n\n" +
		"<!-- docket:artifacts:start (generated — do not hand-edit) -->\n" +
		"<!-- docket:artifacts:end -->\n\n## Why\n\n" + repairAuthoredSentinel + "\n"
}

// publishHealthyDrift publishes a stale board and a stale artifact-links record
// onto the healthy metadata branch, returning the pinned docket tip afterward.
func (r *initRepo) publishHealthyDrift(t *testing.T) {
	t.Helper()
	dotDocket := filepath.Join(r.invocation, ".docket")
	writeRepoFile(t, dotDocket, "docs/changes/BOARD.md", "# Backlog\n\nhand-written stale board\n")
	writeRepoFile(t, dotDocket, "docs/changes/active/0001-example.md", staleRepairRecord())
	runGit(t, dotDocket, "add", "--", "docs/changes/BOARD.md", "docs/changes/active/0001-example.md")
	runGit(t, dotDocket, "commit", "-q", "-m", "publish stale derived views")
	runGit(t, dotDocket, "push", "-q", "origin", string(reposetup.MetadataBranchName))
}

// TestIntegrationRepoMigrationHealthyRepairPreview proves migrate on a healthy
// repository with derived-view drift, WITHOUT --yes, returns confirmation-required
// naming the pinned metadata revision and the repaired file set — and writes
// nothing to the remote docket branch.
func TestIntegrationRepoMigrationHealthyRepairPreview(t *testing.T) {
	r := newHealthyRepo(t)
	r.publishHealthyDrift(t)
	before := currentDocketTip(t, r)

	res := r.runMigrate(t, MigrateOptions{RepairAuthorized: true})
	if res.Result != ResultInvalidState || res.RepositoryState != "confirmation-required" {
		t.Fatalf("preview = %q/%q (%s), want invalid-state/confirmation-required", res.Result, res.RepositoryState, res.HumanText())
	}
	if res.SourceRevision != before {
		t.Errorf("preview SourceRevision = %q, want the pinned docket tip %q", res.SourceRevision, before)
	}
	if !containsPath(res.RepairedViews, "docs/changes/BOARD.md") || !containsPath(res.RepairedViews, "docs/changes/active/0001-example.md") {
		t.Errorf("RepairedViews = %v, want both the board and the record", res.RepairedViews)
	}
	if after := currentDocketTip(t, r); after != before {
		t.Errorf("preview advanced the docket branch %q -> %q; a preview must not write", before, after)
	}
}

// TestIntegrationRepoMigrationHealthyRepairApplies proves an authorized repair
// recomputes the canonical derived bytes on the metadata branch, leaves the
// authored prose byte-identical, and is idempotent on a second run.
func TestIntegrationRepoMigrationHealthyRepairApplies(t *testing.T) {
	r := newHealthyRepo(t)
	r.publishHealthyDrift(t)
	before := currentDocketTip(t, r)

	res := r.runMigrate(t, MigrateOptions{Authorized: true, RepairAuthorized: true})
	if res.Result != ResultApplied {
		t.Fatalf("repair = %q (%s), want applied", res.Result, res.HumanText())
	}
	if !containsPath(res.RepairedViews, "docs/changes/BOARD.md") {
		t.Errorf("RepairedViews = %v, want the board", res.RepairedViews)
	}
	after := currentDocketTip(t, r)
	if after == before {
		t.Fatalf("repair did not advance the docket branch")
	}

	// The repaired record carries the canonical Spec row AND the untouched authored
	// prose; the board is no longer the stale bytes.
	record := showDocketFile(t, r, "docs/changes/active/0001-example.md")
	if !strings.Contains(record, "| Spec |") {
		t.Errorf("repaired record missing the canonical Spec row:\n%s", record)
	}
	if !strings.Contains(record, repairAuthoredSentinel) {
		t.Errorf("repair modified authored prose; the sentinel is gone:\n%s", record)
	}
	board := showDocketFile(t, r, "docs/changes/BOARD.md")
	if strings.Contains(board, "hand-written stale board") {
		t.Errorf("board was not recomputed:\n%s", board)
	}

	// Idempotent: a second authorized run finds no drift and is a no-op.
	second := r.runMigrate(t, MigrateOptions{Authorized: true, RepairAuthorized: true})
	if second.Result != ResultNoOp {
		t.Errorf("second repair = %q (%s), want no-op (nothing left to repair)", second.Result, second.HumanText())
	}
}

// currentDocketTip returns the remote docket branch tip via an independent git
// oracle.
func currentDocketTip(t *testing.T, r *initRepo) string {
	t.Helper()
	runGit(t, r.invocation, "fetch", "-q", "origin", string(reposetup.MetadataBranchName))
	return strings.TrimSpace(runGit(t, r.invocation, "rev-parse", "FETCH_HEAD"))
}

// showDocketFile reads a file from the remote docket branch tip.
func showDocketFile(t *testing.T, r *initRepo, relPath string) string {
	t.Helper()
	runGit(t, r.invocation, "fetch", "-q", "origin", string(reposetup.MetadataBranchName))
	return runGit(t, r.invocation, "show", "FETCH_HEAD:"+relPath)
}

// containsPath reports whether s contains p.
func containsPath(s []string, p string) bool {
	for _, v := range s {
		if v == p {
			return true
		}
	}
	return false
}
