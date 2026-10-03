//go:build integration

package app

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// This is the real-Git `repository repair` shard (prefix TestIntegrationRepoRepair,
// tests/test_go_integration_app_reporepair.sh). Each test builds an
// already-migrated repository (newHealthyRepo), publishes drift onto its docket
// metadata branch from the .docket worktree, drives RunRepositoryRepair, and
// inspects the remote docket branch with an independent git oracle.

// --- moved helpers (from the migration shard) ------------------------------

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

// --- repository repair -------------------------------------------------------

// runRepair drives RunRepositoryRepair against the invocation clone.
func (r *initRepo) runRepair(t *testing.T, o RepairOptions) RepositoryRepairResult {
	t.Helper()
	client := newGitClient(t)
	return RunRepositoryRepair(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation}, o)
}

const (
	repairStampPath = "docs/changes/archive/2026-01-02-0003-archived-change.md"
	repairEmptyPath = "docs/changes/archive/2026-01-03-0004-archived-empty.md"
)

// repairArchivedRecord renders a minimal valid archived done record carrying the
// given claimed_at line.
func repairArchivedRecord(id int, slug, claimedLine string) string {
	return "---\nid: " + strconv.Itoa(id) + "\nslug: " + slug + "\nstatus: done\ntitle: Change " + slug +
		"\ntype: feature\n" + claimedLine + "\n---\n\nBody for " + slug + ".\n"
}

// publishFrontmatterDrift publishes one archived done record with a REAL claim
// stamp (a repairable drop-final-claimed-at) and one with the EMPTY cleared form
// (not a finding; it must survive every repair byte-identical).
func (r *initRepo) publishFrontmatterDrift(t *testing.T) {
	t.Helper()
	dotDocket := filepath.Join(r.invocation, ".docket")
	writeRepoFile(t, dotDocket, repairStampPath, repairArchivedRecord(3, "archived-change", "claimed_at: 2026-08-01T10:00:00Z"))
	writeRepoFile(t, dotDocket, repairEmptyPath, repairArchivedRecord(4, "archived-empty", "claimed_at:"))
	runGit(t, dotDocket, "add", "--", repairStampPath, repairEmptyPath)
	runGit(t, dotDocket, "commit", "-q", "-m", "publish archived claim stamps")
	runGit(t, dotDocket, "push", "-q", "origin", string(reposetup.MetadataBranchName))
}

// TestIntegrationRepoRepairPreviewListsBothKindsAndWritesNothing proves the
// unauthorized run returns confirmation-required pinned to the docket tip,
// listing the frontmatter repair AND the derived-view repairs, never the empty
// claimed_at record — and writes nothing.
func TestIntegrationRepoRepairPreviewListsBothKindsAndWritesNothing(t *testing.T) {
	r := newHealthyRepo(t)
	r.publishHealthyDrift(t)
	r.publishFrontmatterDrift(t)
	before := currentDocketTip(t, r)

	res := r.runRepair(t, RepairOptions{})
	if res.Result != ResultInvalidState || !res.ConfirmationRequired() {
		t.Fatalf("preview = %q/%q (%s), want invalid-state/confirmation-required", res.Result, res.RepositoryState, res.HumanText())
	}
	if res.SourceRevision != before {
		t.Errorf("SourceRevision = %q, want the pinned docket tip %q", res.SourceRevision, before)
	}
	for _, p := range []string{"docs/changes/BOARD.md", "docs/changes/active/0001-example.md", repairStampPath} {
		if !containsPath(res.RepairedFiles, p) {
			t.Errorf("RepairedFiles = %v, want %s", res.RepairedFiles, p)
		}
	}
	if containsPath(res.RepairedFiles, repairEmptyPath) {
		t.Errorf("the empty claimed_at record must not be a repair: %v", res.RepairedFiles)
	}
	if len(res.Repairs) != 1 || res.Repairs[0].Code != reposetup.RepairDropClaimedAt || res.Repairs[0].Path != repairStampPath {
		t.Errorf("Repairs = %+v, want exactly the stamped record's drop", res.Repairs)
	}
	for _, want := range []string{"--yes", "[drop-final-claimed-at] " + repairStampPath, "[board-stale] docs/changes/BOARD.md"} {
		if !strings.Contains(res.HumanText(), want) {
			t.Errorf("preview human lacks %q:\n%s", want, res.HumanText())
		}
	}
	if after := currentDocketTip(t, r); after != before {
		t.Errorf("preview advanced the docket branch %q -> %q", before, after)
	}
}

// TestIntegrationRepoRepairAppliesOneDescendantCommit proves an authorized repair
// publishes exactly ONE descendant of the pinned tip, subject `docket: repository
// repair`, changing exactly the listed files; the record carries both repairs'
// canonical bytes and its untouched prose; the empty claimed_at record is
// byte-identical; a second run is a no-op; and a following check reports no
// repairable finding (Review Focus 2).
func TestIntegrationRepoRepairAppliesOneDescendantCommit(t *testing.T) {
	r := newHealthyRepo(t)
	r.publishHealthyDrift(t)
	r.publishFrontmatterDrift(t)
	before := currentDocketTip(t, r)
	emptyBefore := showDocketFile(t, r, repairEmptyPath)

	res := r.runRepair(t, RepairOptions{Authorized: true})
	if res.Result != ResultApplied {
		t.Fatalf("repair = %q (%s), want applied", res.Result, res.HumanText())
	}
	after := currentDocketTip(t, r)
	if res.MetadataTip != after || res.SourceRevision != before {
		t.Errorf("result tips = %q <- %q, want %q <- %q", res.MetadataTip, res.SourceRevision, after, before)
	}
	if parent := runGit(t, r.invocation, "rev-parse", after+"^"); parent != before {
		t.Errorf("repair commit parent = %s, want the pinned tip %s (one descendant)", parent, before)
	}
	// The literal, not repositoryRepairSubject: comparing against the constant
	// under test would stay green when the constant drifts.
	const wantRepairSubject = "docket: repository repair"
	if subject := runGit(t, r.invocation, "log", "-1", "--format=%s", after); subject != wantRepairSubject {
		t.Errorf("subject = %q, want %q", subject, wantRepairSubject)
	}
	want := map[string]bool{}
	for _, f := range res.RepairedFiles {
		want[f] = true
	}
	if got := changedPathSet(t, r, before, after); !sameStringSet(got, want) {
		t.Errorf("changed paths = %v, want exactly RepairedFiles %v", keysOf(got), keysOf(want))
	}
	record := showDocketFile(t, r, "docs/changes/active/0001-example.md")
	if !strings.Contains(record, "| Spec |") || !strings.Contains(record, repairAuthoredSentinel) {
		t.Errorf("repaired record must carry the Spec row and the untouched prose:\n%s", record)
	}
	if stamped := showDocketFile(t, r, repairStampPath); strings.Contains(stamped, "claimed_at") {
		t.Errorf("stamped archived record still carries claimed_at:\n%s", stamped)
	}
	if got := showDocketFile(t, r, repairEmptyPath); got != emptyBefore {
		t.Errorf("empty claimed_at record changed:\n%s", got)
	}
	if strings.Contains(showDocketFile(t, r, "docs/changes/BOARD.md"), "hand-written stale board") {
		t.Errorf("board was not recomputed")
	}

	second := r.runRepair(t, RepairOptions{Authorized: true})
	if second.Result != ResultNoOp {
		t.Errorf("second repair = %q (%s), want no-op", second.Result, second.HumanText())
	}
	if got := currentDocketTip(t, r); got != after {
		t.Errorf("idempotent re-run moved the docket branch %s -> %s", after, got)
	}
	for _, f := range r.runCheck(t).Findings {
		if f.Repairable != nil && *f.Repairable {
			t.Errorf("check after repair still reports a repairable finding: %+v", f)
		}
	}
}

// TestIntegrationRepoRepairMovedTipIsContended proves an authorized run pinned to
// a preview's tip, after a concurrent write moved the docket branch, is contended
// and overwrites nothing.
func TestIntegrationRepoRepairMovedTipIsContended(t *testing.T) {
	r := newHealthyRepo(t)
	r.publishHealthyDrift(t)
	preview := r.runRepair(t, RepairOptions{})
	if !preview.ConfirmationRequired() {
		t.Fatalf("preview = %q (%s), want confirmation-required", preview.Result, preview.HumanText())
	}
	r.writeDocketFileAndPush(t, "docs/changes/active/0002-concurrent.md",
		"---\nid: 2\nslug: concurrent\nstatus: proposed\ntitle: Concurrent\ntype: feature\n---\n\nBody.\n",
		"a concurrent metadata write")
	moved := currentDocketTip(t, r)

	res := r.runRepair(t, RepairOptions{Authorized: true, ExpectedSource: preview.SourceRevision})
	if res.Result != ResultContended {
		t.Fatalf("repair = %q (%s), want contended", res.Result, res.HumanText())
	}
	if got := currentDocketTip(t, r); got != moved {
		t.Errorf("a contended repair wrote: docket %s -> %s", moved, got)
	}
	if h := res.HumanText(); !strings.Contains(h, moved) || !strings.Contains(h, preview.SourceRevision) {
		t.Errorf("contended human must name both revisions: %q", h)
	}
}

// TestIntegrationRepoRepairRefusesLegacyRepository proves a legacy repository is
// refused naming `docket repository migrate` and no docket branch is created.
func TestIntegrationRepoRepairRefusesLegacyRepository(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	res := r.runRepair(t, RepairOptions{Authorized: true})
	if res.Result != ResultInvalidState || res.RepositoryState != string(reposetup.StateLegacy) {
		t.Fatalf("legacy repair = %q/%q (%s), want invalid-state/legacy", res.Result, res.RepositoryState, res.HumanText())
	}
	if !strings.Contains(res.HumanText(), "docket repository migrate") {
		t.Errorf("legacy refusal must name `docket repository migrate`: %q", res.HumanText())
	}
	if _, err := tryGit(r.origin, "rev-parse", "--verify", "refs/heads/"+string(reposetup.MetadataBranchName)); err == nil {
		t.Errorf("a legacy refusal must not create the docket branch")
	}
}

// TestIntegrationRepoRepairRefusesFreshRepository proves a fresh repository is
// refused naming `docket repository init` (a remedy valid in that state).
func TestIntegrationRepoRepairRefusesFreshRepository(t *testing.T) {
	r := newInitRepo(t, healthySetupYML, nil)
	res := r.runRepair(t, RepairOptions{Authorized: true})
	if res.Result != ResultInvalidState || res.RepositoryState != string(reposetup.StateFresh) {
		t.Fatalf("fresh repair = %q/%q (%s), want invalid-state/fresh", res.Result, res.RepositoryState, res.HumanText())
	}
	if !strings.Contains(res.HumanText(), "docket repository init") {
		t.Errorf("fresh refusal must name `docket repository init`: %q", res.HumanText())
	}
}

// TestIntegrationRepoRepairEmptyClaimedAtIsCleanNoOp proves the 269-record shape:
// an archived done record with the empty cleared claimed_at is no finding — the
// repair is a no-op that writes nothing, and check exits 0 on an otherwise healthy
// repository.
func TestIntegrationRepoRepairEmptyClaimedAtIsCleanNoOp(t *testing.T) {
	r := newHealthyRepo(t)
	r.writeDocketFileAndPush(t, repairEmptyPath, repairArchivedRecord(4, "archived-empty", "claimed_at:"), "publish an archived record with the cleared claimed_at")
	before := currentDocketTip(t, r)

	res := r.runRepair(t, RepairOptions{Authorized: true})
	if res.Result != ResultNoOp {
		t.Fatalf("repair = %q (%s), want no-op", res.Result, res.HumanText())
	}
	if got := currentDocketTip(t, r); got != before {
		t.Errorf("a no-op repair wrote: docket %s -> %s", before, got)
	}
	check := r.runCheck(t)
	if code := check.CheckExitCode(); code != 0 {
		t.Errorf("check exit = %d (%s), want 0: an empty claimed_at is not a finding", code, check.HumanText())
	}
}

// TestIntegrationRepoRepairMalformedMarkerIsManualReview proves a record under an
// unbalanced managed marker is listed as manual review and left byte-identical,
// while the other repairs in the same run still apply (Review Focus 3).
func TestIntegrationRepoRepairMalformedMarkerIsManualReview(t *testing.T) {
	r := newHealthyRepo(t)
	r.publishHealthyDrift(t)
	malformedPath := "docs/changes/active/0005-malformed.md"
	malformed := "---\nid: 5\nslug: malformed\ntitle: Malformed\nstatus: proposed\npriority: medium\ntype: feature\n" +
		"created: 2026-08-30\nupdated: 2026-08-30\nspec: docs/superpowers/specs/2026-08-30-example-design.md\n---\n\n" +
		"## Artifacts\n\n<!-- docket:artifacts:start (generated — do not hand-edit) -->\n| Artifact | Link |\n\n## Why\n\nbody\n"
	r.writeDocketFileAndPush(t, malformedPath, malformed, "publish a record with an unbalanced managed block")

	res := r.runRepair(t, RepairOptions{Authorized: true})
	if res.Result != ResultApplied {
		t.Fatalf("repair = %q (%s), want applied (the board still repairs)", res.Result, res.HumanText())
	}
	if containsPath(res.RepairedFiles, malformedPath) {
		t.Errorf("the malformed record must never be written: %v", res.RepairedFiles)
	}
	var listed bool
	for _, m := range res.ManualReview {
		if strings.HasPrefix(m, "["+reposetup.CodeArtifactLinksMalformed+"] "+malformedPath) {
			listed = true
		}
	}
	if !listed {
		t.Errorf("ManualReview = %v, want the malformed record listed", res.ManualReview)
	}
	if got := showDocketFile(t, r, malformedPath); got != strings.TrimSpace(malformed) {
		t.Errorf("malformed record changed:\n%s", got)
	}
}
