package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file drives sweepAssessHistorical — the full-scope snapshot assessment of
// historical (done/stacked-merged) sweep records. Each test builds one record and
// local state, a scripted sweepSharedFacts, and asserts the exact pre-dispatch
// entry {Disposition, Reason, Operation: ""} AND that no cleanup op and no per-item
// remote call was dispatched: the assessment carries no GitHub or remote-ref seam,
// so a non-actionable record enqueues nothing. An actionable record is returned in
// the actionable slice (no entry) to flow through the normal fresh-prepare cleanup.

// --- fakes + fixtures -----------------------------------------------------

// assessWS is a scriptable workspace service keyed by change id. Prepare and
// PublishHead exist only to satisfy WorkspaceService; the assessment is read-only,
// so a test asserts they were never called by watching mutations.
type assessWS struct {
	insp        map[int]workspace.Inspection
	inspErr     map[int]error
	inspectCall int
	mutateCall  int
}

func (w *assessWS) Prepare(context.Context, workspace.PrepareRequest) (workspace.Workspace, error) {
	w.mutateCall++
	return workspace.Workspace{}, nil
}

func (w *assessWS) PublishHead(context.Context, workspace.PublishRequest) (workspace.PublishResult, error) {
	w.mutateCall++
	return workspace.PublishResult{}, nil
}

func (w *assessWS) Inspect(_ context.Context, req workspace.InspectRequest) (workspace.Inspection, error) {
	w.inspectCall++
	id := int(req.Target.ChangeID)
	if w.inspErr != nil {
		if e := w.inspErr[id]; e != nil {
			return workspace.Inspection{}, e
		}
	}
	if w.insp != nil {
		if i, ok := w.insp[id]; ok {
			return i, nil
		}
	}
	// Default: an owned tombstone — the workspace leg is provably clean.
	return workspace.Inspection{Kind: workspace.StateCleaned}, nil
}

func cleanWS() *assessWS { return &assessWS{} }

func wsState(id int, kind workspace.StateKind) *assessWS {
	return &assessWS{insp: map[int]workspace.Inspection{id: {Kind: kind}}}
}

// assessDoneBlob builds a done record in the pinned corpus, optionally carrying a
// plan pointer whose integration artifact the backlink leg reads.
func assessDoneBlob(id int, slug, planPath string) StatusBlob {
	extra := ""
	if planPath != "" {
		extra = "plan: " + planPath + "\n"
	}
	return finalizeBlob(id, slug, "done", "high", prRefFor(id), extra)
}

// assessFixture pins one corpus and returns the inventory the assessment reads.
type assessFixture struct {
	pin    StatusPin
	inv    sweepInventory
	reader *fakeReader
}

func newAssessFixture(t *testing.T, corpus []StatusBlob, artifacts map[string]StatusArtifact) assessFixture {
	t.Helper()
	pin := docketPin(t)
	reader := &fakeReader{pin: pin, corpus: corpus, artifactData: artifacts}
	inv, refusal := sweepBuildSnapshot(context.Background(), reader, pin, pin.Config.Effective)
	if refusal != nil {
		t.Fatalf("sweepBuildSnapshot refused: %+v", *refusal)
	}
	return assessFixture{pin: pin, inv: inv, reader: reader}
}

func (f assessFixture) assess(t *testing.T, ws WorkspaceService, shared sweepSharedFacts, ids ...int) ([]MaintenanceEntry, []sweepWorkItem) {
	t.Helper()
	deps := FinalizeDeps{Planning: PlanningDeps{Reader: f.reader, Clock: testClock()}}
	wdeps := WorkspaceDeps{Service: ws}
	cands := make([]sweepWorkItem, 0, len(ids))
	for _, id := range ids {
		cands = append(cands, sweepWorkItem{id: id, kind: sweepKindCleanup})
	}
	return sweepAssessHistorical(context.Background(), deps, wdeps, f.inv, f.pin, shared, cands)
}

// assessInterior renders the exact final-backlink interior a record's already-
// correct integration artifact must carry — so a test can build an artifact whose
// backlink block is already retargeted (no work).
func assessInterior(t *testing.T, f assessFixture, id int) string {
	t.Helper()
	c, out := f.inv.snap.Change(domain.ChangeID(id))
	if out != domain.LookupFound {
		t.Fatalf("record %d not found in inventory", id)
	}
	block, err := render.BacklinkContent(c, linkContextOf(f.pin))
	if err != nil {
		t.Fatalf("render backlink: %v", err)
	}
	return backlinkInterior(block)
}

// assessArtifactWithBacklink wraps a backlink block carrying interior in a plan-shaped
// body the document parser locates the managed block within.
func assessArtifactWithBacklink(interior string) StatusArtifact {
	body := "## Plan\n\n" +
		"<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		interior + "\n" +
		"<!-- docket:backlink:end -->\n"
	return StatusArtifact{Found: true, Revision: "artv1", Data: []byte(body)}
}

// artifactDanglingMarker is a plan artifact whose backlink block has a start
// marker but no end marker — a dangling managed block the parser refuses.
func artifactDanglingMarker() StatusArtifact {
	body := "## Plan\n\n" +
		"<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		"> ↩ dangling with no end marker\n"
	return StatusArtifact{Found: true, Revision: "artbad", Data: []byte(body)}
}

func integrationArtifactKey(path string) string { return sourceIntegration + "|" + path }

// featureRefFor is the fully qualified local/remote ref for a feat/<slug> branch.
func featureRefFor(slug string) gitcli.RefName {
	return gitcli.RefName(branchRefPrefix + "feat/" + slug)
}

// findEntry returns the single entry for id, or nil.
func findEntry(entries []MaintenanceEntry, id int) *MaintenanceEntry {
	for i := range entries {
		if entries[i].ID == id {
			return &entries[i]
		}
	}
	return nil
}

func actionableHas(actionable []sweepWorkItem, id int) bool {
	for _, it := range actionable {
		if it.id == id {
			return true
		}
	}
	return false
}

// assertEntry checks the exact pre-dispatch entry contract: disposition, reason,
// and an EMPTY Operation (a pre-dispatch observation, never a dispatched result).
func assertEntry(t *testing.T, e *MaintenanceEntry, disp, reason string) {
	t.Helper()
	if e == nil {
		t.Fatalf("no entry produced")
	}
	if e.Disposition != disp || e.Reason != reason {
		t.Fatalf("entry = {disp:%q reason:%q}, want {disp:%q reason:%q}; msg=%q", e.Disposition, e.Reason, disp, reason, e.Message)
	}
	if e.Operation != "" {
		t.Fatalf("pre-dispatch observation must carry an EMPTY Operation, got %q", e.Operation)
	}
}

// --- tests ----------------------------------------------------------------

// TestAssessStackedMergedIsSnapshotRetainedNoDispatch: a stacked-merged record is
// retained until its root closes — skipped/snapshot-retained, never dispatched,
// and the done-record identity prerequisites are not applied to it.
func TestAssessStackedMergedIsSnapshotRetainedNoDispatch(t *testing.T) {
	corpus := []StatusBlob{finalizeBlob(40, "completed", "stacked-merged", "high", prRefFor(40), "")}
	f := newAssessFixture(t, corpus, nil)
	entries, actionable := f.assess(t, cleanWS(), sweepSharedFacts{}, 40)

	if actionableHas(actionable, 40) {
		t.Fatalf("stacked-merged must never be actionable; actionable=%v", actionable)
	}
	assertEntry(t, findEntry(entries, 40), SweepDispSkipped, ReasonSweepSnapshotRetained)
}

// TestAssessCleanTombstoneAbsentRefsCorrectBacklinksIsNoWork: a cleaned workspace
// tombstone, absent local and remote refs, and an already-correct final
// backlink is snapshot-no-work. It asserts the absence of ALL mutations: nothing
// enqueued, and the workspace service was only inspected, never mutated.
func TestAssessCleanTombstoneAbsentRefsCorrectBacklinksIsNoWork(t *testing.T) {
	planPath := "docs/changes/plans/plan-41.md"
	f := newAssessFixture(t, []StatusBlob{assessDoneBlob(41, "archived", planPath)}, nil)
	// Build the artifact so its backlink block is already correct: no work.
	interior := assessInterior(t, f, 41)
	f.reader.artifactData = map[string]StatusArtifact{integrationArtifactKey(planPath): assessArtifactWithBacklink(interior)}

	ws := cleanWS()
	// Empty non-nil advertisement + no worktrees: absent refs are PROVEN.
	shared := sweepSharedFacts{remoteHeads: map[gitcli.RefName]gitcli.ObjectID{}}
	entries, actionable := f.assess(t, ws, shared, 41)

	if len(actionable) != 0 {
		t.Fatalf("a provably-clean record must dispatch nothing; actionable=%v", actionable)
	}
	assertEntry(t, findEntry(entries, 41), SweepDispSkipped, ReasonSweepSnapshotNoWork)
	if ws.mutateCall != 0 {
		t.Fatalf("the assessment must never mutate the workspace; mutateCall=%d", ws.mutateCall)
	}
}

// TestAssessMissingManifestNoBacklinkWorkIsBlockedNotClean: a missing/foreign
// workspace manifest never certifies clean — with no other leg's work it is
// blocked/snapshot-blocked, distinct from a no-op.
func TestAssessMissingManifestNoBacklinkWorkIsBlockedNotClean(t *testing.T) {
	f := newAssessFixture(t, []StatusBlob{assessDoneBlob(41, "archived", "")}, nil)
	ws := wsState(41, workspace.StateForeign)
	shared := sweepSharedFacts{remoteHeads: map[gitcli.RefName]gitcli.ObjectID{}}
	entries, actionable := f.assess(t, ws, shared, 41)

	if len(actionable) != 0 {
		t.Fatalf("a blocked record must dispatch nothing; actionable=%v", actionable)
	}
	assertEntry(t, findEntry(entries, 41), SweepDispBlocked, ReasonSweepSnapshotBlocked)
}

// TestAssessAbsentManifestIsBlockedNotClean is change 0368's regression: a
// proven cleanly-absent workspace (StateAbsent) must not certify a clean
// checkout any more than a foreign one did pre-change — it is a blocker, never a
// no-op. This MUST fail against StateAbsent falling to the default markWork arm.
func TestAssessAbsentManifestIsBlockedNotClean(t *testing.T) {
	f := newAssessFixture(t, []StatusBlob{assessDoneBlob(41, "archived", "")}, nil)
	ws := wsState(41, workspace.StateAbsent)
	shared := sweepSharedFacts{remoteHeads: map[gitcli.RefName]gitcli.ObjectID{}}
	entries, actionable := f.assess(t, ws, shared, 41)

	if len(actionable) != 0 {
		t.Fatalf("a blocked record must dispatch nothing; actionable=%v", actionable)
	}
	assertEntry(t, findEntry(entries, 41), SweepDispBlocked, ReasonSweepSnapshotBlocked)
}

// TestAssessMissingManifestWithStaleBacklinkIsActionable: the backlink leg is
// INDEPENDENT of the workspace blocker — a stale backlink makes the record
// actionable even though the workspace manifest is foreign.
func TestAssessMissingManifestWithStaleBacklinkIsActionable(t *testing.T) {
	planPath := "docs/changes/plans/plan-41.md"
	f := newAssessFixture(t, []StatusBlob{assessDoneBlob(41, "archived", planPath)}, map[string]StatusArtifact{
		integrationArtifactKey(planPath): assessArtifactWithBacklink("> ↩ **stale — points at the active path**"),
	})
	ws := wsState(41, workspace.StateForeign)
	shared := sweepSharedFacts{remoteHeads: map[gitcli.RefName]gitcli.ObjectID{}}
	entries, actionable := f.assess(t, ws, shared, 41)

	if !actionableHas(actionable, 41) {
		t.Fatalf("a stale backlink must make the record actionable despite the workspace blocker; actionable=%v", actionable)
	}
	if findEntry(entries, 41) != nil {
		t.Fatalf("an actionable record produces no pre-dispatch entry, got %+v", findEntry(entries, 41))
	}
}

// TestAssessStaleBacklinkLegIsActionable: a stale final backlink alone makes
// the record actionable (workspace clean, refs absent).
func TestAssessStaleBacklinkLegIsActionable(t *testing.T) {
	planPath := "docs/changes/plans/plan-41.md"
	f := newAssessFixture(t, []StatusBlob{assessDoneBlob(41, "archived", planPath)}, map[string]StatusArtifact{
		integrationArtifactKey(planPath): assessArtifactWithBacklink("> ↩ **stale line**"),
	})
	shared := sweepSharedFacts{remoteHeads: map[gitcli.RefName]gitcli.ObjectID{}}
	_, actionable := f.assess(t, cleanWS(), shared, 41)
	if !actionableHas(actionable, 41) {
		t.Fatalf("stale backlink leg must be actionable; actionable=%v", actionable)
	}
}

// TestAssessReadyWorkspaceLegIsActionable: an owned ready checkout is possible
// cleanup work (workspace removal) → actionable.
func TestAssessReadyWorkspaceLegIsActionable(t *testing.T) {
	f := newAssessFixture(t, []StatusBlob{assessDoneBlob(41, "archived", "")}, nil)
	ws := wsState(41, workspace.StateReady)
	shared := sweepSharedFacts{remoteHeads: map[gitcli.RefName]gitcli.ObjectID{}}
	_, actionable := f.assess(t, ws, shared, 41)
	if !actionableHas(actionable, 41) {
		t.Fatalf("a ready owned workspace must be actionable; actionable=%v", actionable)
	}
}

// TestAssessLeftoverLocalRefLegIsActionable: a worktree still checked out on the
// feature ref is a leftover local ref → actionable (workspace clean, remote absent).
func TestAssessLeftoverLocalRefLegIsActionable(t *testing.T) {
	f := newAssessFixture(t, []StatusBlob{assessDoneBlob(41, "archived", "")}, nil)
	shared := sweepSharedFacts{
		remoteHeads: map[gitcli.RefName]gitcli.ObjectID{},
		worktrees:   []gitcli.WorktreeInfo{{Path: "/wt", Branch: featureRefFor("archived")}},
	}
	_, actionable := f.assess(t, cleanWS(), shared, 41)
	if !actionableHas(actionable, 41) {
		t.Fatalf("a leftover local ref checked out in a worktree must be actionable; actionable=%v", actionable)
	}
}

// TestAssessLeftoverRemoteRefLegIsActionable: the feature ref present in the shared
// remote-heads advertisement is a leftover remote ref → actionable.
func TestAssessLeftoverRemoteRefLegIsActionable(t *testing.T) {
	f := newAssessFixture(t, []StatusBlob{assessDoneBlob(41, "archived", "")}, nil)
	shared := sweepSharedFacts{remoteHeads: map[gitcli.RefName]gitcli.ObjectID{
		featureRefFor("archived"): gitcli.ObjectID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
	}}
	_, actionable := f.assess(t, cleanWS(), shared, 41)
	if !actionableHas(actionable, 41) {
		t.Fatalf("a leftover remote ref must be actionable; actionable=%v", actionable)
	}
}

// TestAssessMalformedMarkersAreUnknownNeverNoWork: a malformed/unbalanced final
// backlink block is unresolved — unknown/snapshot-unknown — never a clean no-op.
func TestAssessMalformedMarkersAreUnknownNeverNoWork(t *testing.T) {
	planPath := "docs/changes/plans/plan-41.md"
	f := newAssessFixture(t, []StatusBlob{assessDoneBlob(41, "archived", planPath)}, map[string]StatusArtifact{
		integrationArtifactKey(planPath): artifactDanglingMarker(),
	})
	shared := sweepSharedFacts{remoteHeads: map[gitcli.RefName]gitcli.ObjectID{}}
	entries, actionable := f.assess(t, cleanWS(), shared, 41)

	if len(actionable) != 0 {
		t.Fatalf("a malformed block must not be actionable; actionable=%v", actionable)
	}
	e := findEntry(entries, 41)
	assertEntry(t, e, SweepDispUnknown, ReasonSweepSnapshotUnknown)
	if !strings.Contains(e.Message, sweepLegBacklink) {
		t.Fatalf("the unknown message must name the backlink leg, got %q", e.Message)
	}
}

// TestAssessInvalidRecordDataIsSnapshotInvalid: a done record whose canonical PR
// reference does not parse is snapshot-invalid — distinguishable from clean
// absence — and nothing is dispatched.
func TestAssessInvalidRecordDataIsSnapshotInvalid(t *testing.T) {
	corpus := []StatusBlob{finalizeBlob(41, "archived", "done", "high", "not-a-pull-request", "")}
	f := newAssessFixture(t, corpus, nil)
	entries, actionable := f.assess(t, cleanWS(), sweepSharedFacts{remoteHeads: map[gitcli.RefName]gitcli.ObjectID{}}, 41)

	if len(actionable) != 0 {
		t.Fatalf("an invalid record must dispatch nothing; actionable=%v", actionable)
	}
	assertEntry(t, findEntry(entries, 41), SweepDispSkipped, ReasonSweepSnapshotInvalid)
}

// TestAssessFailedRemoteHeadsBlocksNoWorkButNotLocalLegs: a failed shared remote
// advertisement makes the remote-ref absence UNPROVABLE — a record with no
// locally-established work becomes unknown (never no-work) — but a record whose
// local backlink leg already established work still dispatches.
func TestAssessFailedRemoteHeadsBlocksNoWorkButNotLocalLegs(t *testing.T) {
	planPath := "docs/changes/plans/plan-42.md"
	corpus := []StatusBlob{
		assessDoneBlob(41, "cleanarchived", ""),       // clean everywhere except the failed remote
		assessDoneBlob(42, "stalearchived", planPath), // stale backlink = local work
	}
	f := newAssessFixture(t, corpus, map[string]StatusArtifact{
		integrationArtifactKey(planPath): assessArtifactWithBacklink("> ↩ **stale**"),
	})
	shared := sweepSharedFacts{remoteHeadsErr: errors.New("ls-remote transport failed")}
	entries, actionable := f.assess(t, cleanWS(), shared, 41, 42)

	// 41: only the remote leg could speak, and it is unknown → no-work is blocked.
	if actionableHas(actionable, 41) {
		t.Fatalf("41 has no local work and an unprovable remote absence; it must not dispatch")
	}
	assertEntry(t, findEntry(entries, 41), SweepDispUnknown, ReasonSweepSnapshotUnknown)
	// 42: local backlink work stands regardless of the remote inventory failure.
	if !actionableHas(actionable, 42) {
		t.Fatalf("42's locally-established work must still dispatch despite the failed remote read; actionable=%v", actionable)
	}
}

// TestAssessEmptyAdvertisementMeansNoHeads: a clean, complete but EMPTY remote
// advertisement PROVES the feature ref's absence, so a clean record is snapshot-
// no-work (not unknown).
func TestAssessEmptyAdvertisementMeansNoHeads(t *testing.T) {
	f := newAssessFixture(t, []StatusBlob{assessDoneBlob(41, "archived", "")}, nil)
	shared := sweepSharedFacts{remoteHeads: map[gitcli.RefName]gitcli.ObjectID{}} // empty, non-nil, no error
	entries, actionable := f.assess(t, cleanWS(), shared, 41)

	if len(actionable) != 0 {
		t.Fatalf("a proven-absent record must dispatch nothing; actionable=%v", actionable)
	}
	assertEntry(t, findEntry(entries, 41), SweepDispSkipped, ReasonSweepSnapshotNoWork)
}

// TestAssessUnknownLegNamedInMessageEvenWhenBlocked: when one leg is unresolved and
// another is blocked (no actionable leg), the outcome is unknown and the unknown
// leg stays named in the message.
func TestAssessUnknownLegNamedInMessageEvenWhenBlocked(t *testing.T) {
	f := newAssessFixture(t, []StatusBlob{assessDoneBlob(41, "archived", "")}, nil)
	ws := wsState(41, workspace.StateForeign)                                  // workspace leg: blocked
	shared := sweepSharedFacts{remoteHeadsErr: errors.New("ls-remote failed")} // remote leg: unknown
	entries, actionable := f.assess(t, ws, shared, 41)

	if len(actionable) != 0 {
		t.Fatalf("no leg has work; nothing must dispatch; actionable=%v", actionable)
	}
	e := findEntry(entries, 41)
	assertEntry(t, e, SweepDispUnknown, ReasonSweepSnapshotUnknown)
	if !strings.Contains(e.Message, sweepLegRemoteRef) {
		t.Fatalf("the unknown remote-ref leg must stay named even when the workspace is blocked, got %q", e.Message)
	}
}

// prShared builds shared facts whose PR-body inventory was gathered, carrying
// the given bodies (and nothing unknown).
func prShared(bodies map[int]string) sweepSharedFacts {
	return sweepSharedFacts{
		remoteHeads:      map[gitcli.RefName]gitcli.ObjectID{},
		prBodiesGathered: true,
		prBodies:         bodies,
		prBodiesUnknown:  map[int]bool{},
	}
}

// assessArchivedDoneBlob is assessDoneBlob relocated to the archive, as close-out
// leaves every done record: the PR-backlink leg keys on the record's own path, so
// a stale body (naming the active path) is distinguishable from a repointed one.
func assessArchivedDoneBlob(id int, slug string) StatusBlob {
	b := assessDoneBlob(id, slug, "")
	b.Location = repository.LocationArchive
	b.Path = fmt.Sprintf("docs/changes/archive/2026-01-03-%04d-%s.md", id, slug)
	return b
}

func prNumberOf(t *testing.T, f assessFixture, id int) int {
	t.Helper()
	c, _ := f.inv.snap.Change(domain.ChangeID(id))
	n, ok := parsePRNumber(c.PR().Value)
	if !ok {
		t.Fatalf("record %d carries no parsable PR", id)
	}
	return n
}

// TestAssessStalePRBacklinkIsActionable: a done record whose PR body still names
// the active path has work — it is dispatched to cleanup, which repoints it.
func TestAssessStalePRBacklinkIsActionable(t *testing.T) {
	f := newAssessFixture(t, []StatusBlob{assessArchivedDoneBlob(41, "archived")}, nil)
	n := prNumberOf(t, f, 41)
	shared := prShared(map[int]string{n: prBodyWithActiveBacklink("docs/changes/active/0041-archived.md", "prose")})
	_, actionable := f.assess(t, cleanWS(), shared, 41)
	if !actionableHas(actionable, 41) {
		t.Fatalf("a stale PR backlink must make the record actionable; actionable=%v", actionable)
	}
}

// TestAssessCorrectPRBacklinkIsNoWork: a PR body already naming the archive path
// adds no work.
func TestAssessCorrectPRBacklinkIsNoWork(t *testing.T) {
	f := newAssessFixture(t, []StatusBlob{assessArchivedDoneBlob(41, "archived")}, nil)
	n := prNumberOf(t, f, 41)
	body := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" + assessInterior(t, f, 41) + "\n<!-- docket:backlink:end -->\n\nprose\n"
	entries, actionable := f.assess(t, cleanWS(), prShared(map[int]string{n: body}), 41)
	if len(actionable) != 0 {
		t.Fatalf("an already-correct PR backlink must add no work; actionable=%v", actionable)
	}
	assertEntry(t, findEntry(entries, 41), SweepDispSkipped, ReasonSweepSnapshotNoWork)
}

// TestAssessUnreadPRBodyIsUnknownNeverNoWork: a gathered inventory missing this
// PR (a failed batch or a Found=false slot) is unknown, never certified no-work.
func TestAssessUnreadPRBodyIsUnknownNeverNoWork(t *testing.T) {
	f := newAssessFixture(t, []StatusBlob{assessArchivedDoneBlob(41, "archived")}, nil)
	n := prNumberOf(t, f, 41)
	shared := prShared(map[int]string{})
	shared.prBodiesUnknown[n] = true
	entries, actionable := f.assess(t, cleanWS(), shared, 41)
	if len(actionable) != 0 {
		t.Fatalf("an unread PR body must not dispatch; actionable=%v", actionable)
	}
	e := findEntry(entries, 41)
	assertEntry(t, e, SweepDispUnknown, ReasonSweepSnapshotUnknown)
	if !strings.Contains(e.Message, sweepLegPRBacklink) {
		t.Fatalf("the unknown leg must be named; msg=%q", e.Message)
	}
}

// TestAssessPRBacklinkMalformedIsUnknown: malformed backlink markers in the PR
// body are unknown, never no-work and never work.
func TestAssessPRBacklinkMalformedIsUnknown(t *testing.T) {
	f := newAssessFixture(t, []StatusBlob{assessArchivedDoneBlob(41, "archived")}, nil)
	n := prNumberOf(t, f, 41)
	shared := prShared(map[int]string{n: "<!-- docket:backlink:start (generated — do not hand-edit) -->\ndangling\n"})
	entries, actionable := f.assess(t, cleanWS(), shared, 41)
	if len(actionable) != 0 {
		t.Fatalf("malformed markers must not dispatch; actionable=%v", actionable)
	}
	assertEntry(t, findEntry(entries, 41), SweepDispUnknown, ReasonSweepSnapshotUnknown)
}

// TestGatherSweepSharedFactsMarksUnreadPRBodiesUnknown: every done candidate's PR
// number lands in exactly one of prBodies or prBodiesUnknown, so no done record
// is silently unassessed; one batched read serves them all.
func TestGatherSweepSharedFactsMarksUnreadPRBodiesUnknown(t *testing.T) {
	f := newAssessFixture(t, []StatusBlob{assessDoneBlob(41, "a", ""), assessDoneBlob(42, "b", "")}, nil)
	n41, n42 := prNumberOf(t, f, 41), prNumberOf(t, f, 42)
	batch := &fakeSweepBatchReader{result: SweepPRSetResult{
		Facts:  map[int]domain.PRFacts{n41: {Number: strconv.Itoa(n41), State: "merged"}},
		Bodies: map[int]string{n41: "body"},
	}}
	deps := FinalizeDeps{Planning: PlanningDeps{Reader: f.reader, Clock: testClock()}, PRBatch: batch}
	items := []sweepWorkItem{{id: 41, kind: sweepKindCleanup}, {id: 42, kind: sweepKindCleanup}}
	shared := gatherSweepSharedFacts(context.Background(), deps, "repo", f.inv, items)
	if !shared.prBodiesGathered || batch.calls != 1 {
		t.Fatalf("gathered=%v calls=%d, want one batched read", shared.prBodiesGathered, batch.calls)
	}
	if shared.prBodies[n41] != "body" || shared.prBodiesUnknown[n41] {
		t.Errorf("PR #%d should be read", n41)
	}
	if _, ok := shared.prBodies[n42]; ok || !shared.prBodiesUnknown[n42] {
		t.Errorf("PR #%d was not returned, so it must be unknown", n42)
	}
}
