//go:build integration

package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// This is the real-Git migration integration shard (prefix
// TestIntegrationRepoMigration). Each test scripts its own legacy single-branch
// repository (a bare origin + a writer clone that seeds it + an invocation clone
// migrate runs against) under t.TempDir(), drives RunRepositoryMigrate, and
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
// .docket.yml on the pruned integration is the source bytes with ONLY the
// metadata_branch key line removed — every comment, key, and ordering byte
// preserved.
func TestIntegrationRepoMigrationLegacyKeyRemovedBytePreserving(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	r.runMigrate(t, MigrateOptions{Authorized: true})

	got, ok := r.originBlob(t, "main", ".docket.yml")
	if !ok {
		t.Fatal(".docket.yml missing from the pruned integration")
	}
	want := strings.Replace(legacyDocketYML, "metadata_branch: docket   # removed by migration\n", "", 1)
	if got != want {
		t.Errorf(".docket.yml not byte-preserved.\n got: %q\nwant: %q", got, want)
	}
	if strings.Contains(got, "metadata_branch") {
		t.Errorf(".docket.yml still carries the metadata_branch key: %q", got)
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

// TestIntegrationRepoMigrationLocalMovedAfterPublish proves that when the local
// primary advances after the remote publication (via the test seam), the remote
// migration is preserved, the result names a pending local synchronization
// remedy, and a retry performs only local work (no remote change).
func TestIntegrationRepoMigrationLocalMovedAfterPublish(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())

	testHookAfterRemotePublish = func() {
		// Advance the local primary between the remote publication and the local
		// fast-forward decision.
		writeRepoFile(t, r.invocation, "local-only.txt", "advanced after publish\n")
		runGit(t, r.invocation, "add", "--", "local-only.txt")
		runGit(t, r.invocation, "commit", "-q", "-m", "local advance after publish")
	}
	defer func() { testHookAfterRemotePublish = nil }()

	res := r.runMigrate(t, MigrateOptions{Authorized: true})
	if res.Result != ResultApplied {
		t.Fatalf("migrate = %q (%s), want applied", res.Result, res.HumanText())
	}
	if len(res.PendingLocal) == 0 {
		t.Fatal("a local-moved migration must name a pending local synchronization remedy")
	}
	metaTip := r.originTip(t, "docket")
	intTip := r.originTip(t, "main")

	// Retry: the remote is already migrated, so the retry performs no remote work.
	testHookAfterRemotePublish = nil
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
