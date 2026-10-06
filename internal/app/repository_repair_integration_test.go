//go:build integration

package app

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/layout"
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
	runGit(t, dotDocket, "push", "-q", "origin", string(layout.SharedName))
}

// currentDocketTip returns the remote docket branch tip via an independent git
// oracle.
func currentDocketTip(t *testing.T, r *initRepo) string {
	t.Helper()
	runGit(t, r.invocation, "fetch", "-q", "origin", string(layout.SharedName))
	return strings.TrimSpace(runGit(t, r.invocation, "rev-parse", "FETCH_HEAD"))
}

// showDocketFile reads a file from the remote docket branch tip.
func showDocketFile(t *testing.T, r *initRepo, relPath string) string {
	t.Helper()
	runGit(t, r.invocation, "fetch", "-q", "origin", string(layout.SharedName))
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
	runGit(t, dotDocket, "push", "-q", "origin", string(layout.SharedName))
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

// --- one-time relative-link conversion --------------------------------------

const (
	convertActivePath = "docs/changes/active/0001-example.md"
	convertLegacyPath = "docs/changes/archive/2026-01-02-0002-legacy.md"
	convertADRPath    = "docs/adrs/0001-first-decision.md"
	convertWebBase    = "https://github.com/acme/widgets/blob/"
)

// convertActiveRecord is an active change whose ## Artifacts block was written
// before same-branch links became relative: its spec and ADR rows are absolute
// metadata-branch URLs.
func convertActiveRecord() string {
	return "---\n" +
		"id: 1\nslug: example\ntitle: Example change\nstatus: proposed\npriority: medium\n" +
		"type: feature\ncreated: 2026-08-30\nupdated: 2026-08-30\nadrs: [1]\n" +
		"spec: docs/superpowers/specs/2026-08-30-example-design.md\n" +
		"---\n\n## Artifacts\n\n" +
		"<!-- docket:artifacts:start (generated — do not hand-edit) -->\n" +
		"| Artifact | Link |\n|---|---|\n" +
		"| Spec | [2026-08-30-example-design.md](" + convertWebBase + "docket/docs/superpowers/specs/2026-08-30-example-design.md) |\n" +
		"| ADRs | [ADR-0001](" + convertWebBase + "docket/docs/adrs/0001-first-decision.md) |\n" +
		"<!-- docket:artifacts:end -->\n\n## Why\n\n" + repairAuthoredSentinel + "\n"
}

// convertLegacyPlanRow and convertLegacyResultsRow are a legacy done record's
// Plan/Results rows: its files merged to the integration branch, so the rows are
// absolute there and must survive the conversion byte-identical.
const (
	convertLegacyPlanRow    = "| Plan | [2026-01-02-legacy.md](" + convertWebBase + "main/docs/superpowers/plans/2026-01-02-legacy.md) |"
	convertLegacyResultsRow = "| Results | [2026-01-02-legacy-results.md](" + convertWebBase + "main/docs/results/2026-01-02-legacy-results.md) |"
)

// convertLegacyRecord is a legacy done archived record (no "## Build evidence"
// section) whose block carries the old absolute spec/ADR rows and its
// integration-branch Plan/Results rows.
func convertLegacyRecord() string {
	return "---\n" +
		"id: 2\nslug: legacy\ntitle: Legacy change\nstatus: done\npriority: medium\n" +
		"type: feature\ncreated: 2026-01-01\nupdated: 2026-01-02\nadrs: [1]\n" +
		"spec: docs/superpowers/specs/2026-01-02-legacy-design.md\n" +
		"plan: docs/superpowers/plans/2026-01-02-legacy.md\n" +
		"results: docs/results/2026-01-02-legacy-results.md\n" +
		"---\n\n## Artifacts\n\n" +
		"<!-- docket:artifacts:start (generated — do not hand-edit) -->\n" +
		"| Artifact | Link |\n|---|---|\n" +
		"| Spec | [2026-01-02-legacy-design.md](" + convertWebBase + "docket/docs/superpowers/specs/2026-01-02-legacy-design.md) |\n" +
		convertLegacyPlanRow + "\n" +
		convertLegacyResultsRow + "\n" +
		"| ADRs | [ADR-0001](" + convertWebBase + "docket/docs/adrs/0001-first-decision.md) |\n" +
		"<!-- docket:artifacts:end -->\n\n## Why\n\nLegacy body.\n"
}

// TestIntegrationRepoRepairConvertsAbsoluteSameBranchLinksOnce is the one-time
// conversion (spec acceptance 6) against real git with a GitHub-shaped origin:
// check reports both records' absolute same-branch rows as artifact-links-stale,
// one authorized repair lands exactly one metadata commit that makes the active
// record's rows relative and the legacy record's spec/ADR rows relative while
// its Plan/Results rows keep pointing at the integration branch, and a second
// repair is a clean no-op.
func TestIntegrationRepoRepairConvertsAbsoluteSameBranchLinksOnce(t *testing.T) {
	r := newHealthyRepo(t)
	// origin's CONFIGURED url is the GitHub spelling (so the link context carries
	// a web URL); insteadOf routes every fetch/push to the local bare origin.
	runGit(t, r.invocation, "remote", "set-url", "origin", "https://github.com/acme/widgets.git")
	runGit(t, r.invocation, "config", "url."+r.origin+".insteadOf", "https://github.com/acme/widgets.git")

	dotDocket := filepath.Join(r.invocation, ".docket")
	writeRepoFile(t, dotDocket, convertADRPath, "---\nid: 1\nslug: first-decision\nstatus: Accepted\ntitle: First decision\n---\nContext.\n")
	writeRepoFile(t, dotDocket, convertActivePath, convertActiveRecord())
	writeRepoFile(t, dotDocket, convertLegacyPath, convertLegacyRecord())
	runGit(t, dotDocket, "add", "--", convertADRPath, convertActivePath, convertLegacyPath)
	runGit(t, dotDocket, "commit", "-q", "-m", "publish records with absolute same-branch links")
	runGit(t, dotDocket, "push", "-q", "origin", string(layout.SharedName))

	stale := map[string]bool{}
	for _, f := range r.runCheck(t).Findings {
		if f.Code == reposetup.CodeArtifactLinksStale {
			stale[f.Ref] = true
		}
	}
	for _, p := range []string{convertActivePath, convertLegacyPath} {
		if !stale[p] {
			t.Errorf("check did not report artifact-links-stale for %s; stale refs: %v", p, keysOf(stale))
		}
	}

	before := currentDocketTip(t, r)
	res := r.runRepair(t, RepairOptions{Authorized: true})
	if res.Result != ResultApplied {
		t.Fatalf("repair = %q (%s), want applied", res.Result, res.HumanText())
	}
	after := currentDocketTip(t, r)
	if parent := runGit(t, r.invocation, "rev-parse", after+"^"); parent != before {
		t.Errorf("repair commit parent = %s, want the pinned tip %s (exactly one metadata commit)", parent, before)
	}

	active := showDocketFile(t, r, convertActivePath)
	for _, row := range []string{
		"| Spec | [2026-08-30-example-design.md](../../superpowers/specs/2026-08-30-example-design.md) |",
		"| ADRs | [ADR-0001](../../adrs/0001-first-decision.md) |",
	} {
		if !strings.Contains(active, row) {
			t.Errorf("active record missing relative row %q:\n%s", row, active)
		}
	}
	if strings.Contains(active, "/blob/") || !strings.Contains(active, repairAuthoredSentinel) {
		t.Errorf("active record must carry no absolute row and its untouched prose:\n%s", active)
	}

	legacy := showDocketFile(t, r, convertLegacyPath)
	for _, row := range []string{
		"| Spec | [2026-01-02-legacy-design.md](../../superpowers/specs/2026-01-02-legacy-design.md) |",
		convertLegacyPlanRow,
		convertLegacyResultsRow,
		"| ADRs | [ADR-0001](../../adrs/0001-first-decision.md) |",
	} {
		if !strings.Contains(legacy, row) {
			t.Errorf("legacy record missing row %q:\n%s", row, legacy)
		}
	}
	if strings.Contains(legacy, convertWebBase+"docket/") {
		t.Errorf("legacy record still carries an absolute metadata-branch row:\n%s", legacy)
	}

	second := r.runRepair(t, RepairOptions{Authorized: true})
	if second.Result != ResultNoOp {
		t.Errorf("second repair = %q (%s), want no-op", second.Result, second.HumanText())
	}
	if got := currentDocketTip(t, r); got != after {
		t.Errorf("the second repair moved the docket branch %s -> %s", after, got)
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
	if _, err := tryGit(r.origin, "rev-parse", "--verify", "refs/heads/"+string(layout.SharedName)); err == nil {
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

// fakeRepairGitHub is a scriptable RepairGitHub: bodies by PR number, a batch
// failure switch, per-number edit outcomes, and call counters.
type fakeRepairGitHub struct {
	bodies     map[int]string
	failBatch  bool
	editOut    map[int]githubcli.BodyEditOutcome
	batchCalls int
	edits      int
	t          *testing.T
	forbidAll  bool // any call fails the test (routine-commands guard)
}

func (f *fakeRepairGitHub) rev(n int) string {
	return "rev:" + strconv.Itoa(len(f.bodies[n])) + ":" + f.bodies[n][:min(8, len(f.bodies[n]))]
}

func (f *fakeRepairGitHub) DiscoverRepository(context.Context, string) (githubcli.Repository, error) {
	if f.forbidAll {
		f.t.Errorf("DiscoverRepository called by a command that must not touch GitHub")
		return githubcli.Repository{}, errors.New("forbidden")
	}
	return githubcli.Repository{Host: "github.com", Owner: "acme", Name: "widget"}, nil
}

func (f *fakeRepairGitHub) ViewPullRequestsBatch(_ context.Context, _ githubcli.Repository, numbers []int) (map[int]githubcli.BatchPRResult, error) {
	f.batchCalls++
	if f.forbidAll {
		f.t.Errorf("ViewPullRequestsBatch called by a command that must not read PR bodies")
		return nil, errors.New("forbidden")
	}
	if f.failBatch {
		return nil, errors.New("gh api graphql failed")
	}
	out := map[int]githubcli.BatchPRResult{}
	for _, n := range numbers {
		b, ok := f.bodies[n]
		if !ok {
			out[n] = githubcli.BatchPRResult{Found: false}
			continue
		}
		out[n] = githubcli.BatchPRResult{Found: true, PR: githubcli.PullRequest{Number: n, State: githubcli.StateMerged, Body: b, Revision: f.rev(n)}}
	}
	return out, nil
}

func (f *fakeRepairGitHub) EditPullRequestBody(_ context.Context, _ githubcli.Repository, n int, rev, body string) (githubcli.BodyEditOutcome, githubcli.PullRequest, error) {
	f.edits++
	if f.forbidAll {
		f.t.Errorf("EditPullRequestBody called by a command that must not touch GitHub")
		return githubcli.BodyUnknown, githubcli.PullRequest{}, errors.New("forbidden")
	}
	if o, ok := f.editOut[n]; ok {
		return o, githubcli.PullRequest{}, nil
	}
	if rev != f.rev(n) {
		return githubcli.BodyContended, githubcli.PullRequest{}, nil
	}
	f.bodies[n] = body
	return githubcli.BodyEdited, githubcli.PullRequest{Number: n, Body: body}, nil
}

const (
	prRepairPath   = "docs/changes/archive/2026-08-29-0363-old-change.md"
	prRepairActive = "docs/changes/active/0363-old-change.md"
	prRepairOKPath = "docs/changes/archive/2026-08-30-0364-fixed-change.md"
)

// prArchivedRecord is a minimal valid archived done record carrying a PR URL.
func prArchivedRecord(id int, slug string, pr int) string {
	return "---\nid: " + strconv.Itoa(id) + "\nslug: " + slug + "\nstatus: done\ntitle: Change " + slug +
		"\ntype: feature\npr: 'https://github.com/acme/widget/pull/" + strconv.Itoa(pr) + "'\n---\n\nBody for " + slug + ".\n"
}

func prBodyNaming(path string) string {
	return "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		"> ↩ **Change — x** — `" + path + "`\n<!-- docket:backlink:end -->\n\nAuthored PR prose.\n"
}

// publishPRBacklinkRecords publishes one archived record whose PR still names
// the active path (dead) and one whose PR already names its archive path.
func (r *initRepo) publishPRBacklinkRecords(t *testing.T) {
	t.Helper()
	dotDocket := filepath.Join(r.invocation, ".docket")
	writeRepoFile(t, dotDocket, prRepairPath, prArchivedRecord(363, "old-change", 250))
	writeRepoFile(t, dotDocket, prRepairOKPath, prArchivedRecord(364, "fixed-change", 251))
	runGit(t, dotDocket, "add", "--", prRepairPath, prRepairOKPath)
	runGit(t, dotDocket, "commit", "-q", "-m", "publish archived PR-bearing records")
	runGit(t, dotDocket, "push", "-q", "origin", string(layout.SharedName))
}

func (r *initRepo) runPRRepair(t *testing.T, gh RepairGitHub, o RepairOptions) RepositoryRepairResult {
	t.Helper()
	o.PRBacklinks = true
	return RunRepositoryRepair(context.Background(), SetupDeps{Git: newGitClient(t), RepoDir: r.invocation, GitHub: gh}, o)
}

// TestIntegrationRepoRepairPRBacklinksPreviewApplyThenNothing covers spec
// acceptance 3: the preview lists exactly the dead-link PR, --yes fixes it, and
// a second run reports nothing to do.
func TestIntegrationRepoRepairPRBacklinksPreviewApplyThenNothing(t *testing.T) {
	r := newHealthyRepo(t)
	r.publishPRBacklinkRecords(t)
	gh := &fakeRepairGitHub{t: t, bodies: map[int]string{250: prBodyNaming(prRepairActive), 251: prBodyNaming(prRepairOKPath)}}

	preview := r.runPRRepair(t, gh, RepairOptions{})
	if !preview.ConfirmationRequired() || len(preview.PRBacklinks) != 1 {
		t.Fatalf("preview = %q state %q entries %+v, want one planned PR", preview.Result, preview.RepositoryState, preview.PRBacklinks)
	}
	e := preview.PRBacklinks[0]
	if e.ID != 363 || e.PR != 250 || e.Outcome != PRBacklinkRepairPlanned ||
		!strings.Contains(e.Current, prRepairActive) || !strings.Contains(e.Corrected, prRepairPath) {
		t.Fatalf("preview entry = %+v", e)
	}
	if gh.edits != 0 {
		t.Fatalf("a preview must edit nothing; edits=%d", gh.edits)
	}

	applied := r.runPRRepair(t, gh, RepairOptions{Authorized: true, ExpectedSource: preview.SourceRev()})
	if applied.Result != ResultApplied || len(applied.PRBacklinks) != 1 || applied.PRBacklinks[0].Outcome != PRBacklinkRepairRepointed {
		t.Fatalf("apply = %q %+v", applied.Result, applied.PRBacklinks)
	}
	if !strings.Contains(gh.bodies[250], prRepairPath) || !strings.HasSuffix(gh.bodies[250], "\n\nAuthored PR prose.\n") {
		t.Fatalf("PR #250 not repointed with prose intact:\n%s", gh.bodies[250])
	}
	if gh.bodies[251] != prBodyNaming(prRepairOKPath) {
		t.Fatalf("an already-correct PR was edited")
	}

	again := r.runPRRepair(t, gh, RepairOptions{})
	if again.Result != ResultNoOp || len(again.PRBacklinks) != 0 {
		t.Fatalf("second run = %q %+v, want nothing to do", again.Result, again.PRBacklinks)
	}
}

// TestIntegrationRepoRepairPRBacklinksSkipsUnreadable: an unreadable PR (a
// Found=false slot) and a malformed block are reported and skipped; a contended
// edit is reported; none aborts the batch.
func TestIntegrationRepoRepairPRBacklinksSkipsUnreadable(t *testing.T) {
	r := newHealthyRepo(t)
	r.publishPRBacklinkRecords(t)
	dotDocket := filepath.Join(r.invocation, ".docket")
	third := "docs/changes/archive/2026-08-31-0365-third-change.md"
	writeRepoFile(t, dotDocket, third, prArchivedRecord(365, "third-change", 252))
	runGit(t, dotDocket, "add", "--", third)
	runGit(t, dotDocket, "commit", "-q", "-m", "third")
	runGit(t, dotDocket, "push", "-q", "origin", string(layout.SharedName))

	gh := &fakeRepairGitHub{t: t, bodies: map[int]string{
		250: prBodyNaming(prRepairActive),
		252: "<!-- docket:backlink:start (generated — do not hand-edit) -->\ndangling\n",
	}} // 251 absent: Found=false
	preview := r.runPRRepair(t, gh, RepairOptions{})
	byPR := map[int]PRBacklinkRepair{}
	for _, e := range preview.PRBacklinks {
		byPR[e.PR] = e
	}
	if byPR[250].Outcome != PRBacklinkRepairPlanned || byPR[251].Outcome != PRBacklinkRepairUnreadable || byPR[252].Outcome != PRBacklinkRepairUnreadable {
		t.Fatalf("entries = %+v", preview.PRBacklinks)
	}

	gh.editOut = map[int]githubcli.BodyEditOutcome{250: githubcli.BodyContended}
	applied := r.runPRRepair(t, gh, RepairOptions{Authorized: true, ExpectedSource: preview.SourceRev()})
	for _, e := range applied.PRBacklinks {
		if e.PR == 250 && e.Outcome != PRBacklinkRepairContended {
			t.Fatalf("PR #250 outcome = %q, want contended", e.Outcome)
		}
	}
	if gh.edits != 1 {
		t.Fatalf("only the planned PR may be edited; edits=%d", gh.edits)
	}
}

// TestIntegrationRepoRepairPRBacklinksSkipsSharedPR: two done changes naming one
// PR give that PR no single owner, so each is a skipped row and the PR is neither
// read nor planned; an unshared candidate beside them is still planned.
func TestIntegrationRepoRepairPRBacklinksSkipsSharedPR(t *testing.T) {
	gh := &fakeRepairGitHub{t: t, bodies: map[int]string{
		250: prBodyNaming(prRepairActive),
		260: prBodyNaming(prRepairActive),
	}}
	cands := []prRepairCandidate{
		{id: 363, pr: 250, path: prRepairPath, interior: "> x `" + prRepairPath + "`"},
		{id: 370, pr: 260, path: "docs/changes/archive/2026-09-01-0370-a.md", interior: "> a"},
		{id: 371, pr: 260, path: "docs/changes/archive/2026-09-01-0371-b.md", interior: "> b"},
	}
	planned, skipped := planPRRepairs(context.Background(), gh, githubcli.Repository{}, cands)
	if len(planned) != 1 || planned[0].row.PR != 250 {
		t.Fatalf("planned = %+v, want only PR #250", planned)
	}
	if len(skipped) != 2 {
		t.Fatalf("skipped = %+v, want both sharers of PR #260", skipped)
	}
	for i, id := range []int{370, 371} {
		e := skipped[i]
		if e.ID != id || e.PR != 260 || e.Outcome != PRBacklinkRepairSkipped ||
			!strings.Contains(e.Message, "0370, 0371") {
			t.Fatalf("skipped[%d] = %+v", i, e)
		}
	}
}

// TestIntegrationRepoRepairRoutineCommandsReadNoPRBodies pins spec acceptance 4:
// with a GitHub fake that fails on ANY call wired into SetupDeps, routine
// `repository repair` (preview and --yes), `check`, and `prepare` never call it.
func TestIntegrationRepoRepairRoutineCommandsReadNoPRBodies(t *testing.T) {
	r := newHealthyRepo(t)
	r.publishPRBacklinkRecords(t)
	r.publishHealthyDrift(t)
	gh := &fakeRepairGitHub{t: t, forbidAll: true}
	d := SetupDeps{Git: newGitClient(t), RepoDir: r.invocation, GitHub: gh}

	_ = RunRepositoryCheck(context.Background(), d)
	_ = RunRepositoryPrepare(context.Background(), d, PrepareOptions{})
	preview := RunRepositoryRepair(context.Background(), d, RepairOptions{})
	_ = RunRepositoryRepair(context.Background(), d, RepairOptions{Authorized: true, ExpectedSource: preview.SourceRev()})
	if gh.batchCalls != 0 || gh.edits != 0 {
		t.Fatalf("routine commands touched GitHub: batch=%d edits=%d", gh.batchCalls, gh.edits)
	}
}
