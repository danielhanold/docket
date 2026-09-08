//go:build integration

package app

import (
	"context"
	"errors"
	"github.com/danielhanold/docket/internal/githubcli"
	"sort"
	"strings"
	"testing"
)

// TestCloseoutBacklinkLegDocketMode proves the docket-mode split: the metadata
// transaction lands the archived record + spec + board on the metadata ref, and a
// SEPARATE isolated integration-ref commit patches only the merged plan/results
// backlinks — never a metadata record, never an authored byte.
func TestIntegrationFinalizeCloseoutBacklinkLegDocketMode(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0] // docket
	f := setupCloseoutFixture(t, m)
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)

	metaBefore := originTip(t, f.repo.origin, "docket")
	mainBefore := originTip(t, f.repo.origin, "main")

	res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if res.Result != ResultApplied || res.Disposition != CloseoutDispDoneArchived {
		t.Fatalf("closeout = %q disp %q (reason %q)", res.Result, res.Disposition, res.Reason)
	}
	// No terminal-backlink-pending finding: the leg landed.
	for _, fd := range res.Findings {
		if fd.Code == ReasonCloseoutBacklinkPending {
			t.Fatalf("the backlink leg did not land: %+v", fd)
		}
	}

	// Both refs advanced, on separate commits.
	metaAfter := originTip(t, f.repo.origin, "docket")
	mainAfter := originTip(t, f.repo.origin, "main")
	if metaAfter == metaBefore {
		t.Errorf("the metadata ref did not advance")
	}
	if mainAfter == mainBefore {
		t.Errorf("the integration ref did not advance (no backlink leg)")
	}

	// The integration-ref commit touched ONLY the merged plan/results — no metadata
	// record crossed onto the integration branch.
	got := originCommitPaths(t, f.repo.origin, mainAfter)
	want := []string{f.planPath, f.resultsPath}
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("integration-ref commit changed %v, want exactly %v", got, want)
	}
}

// TestCloseoutBacklinkLegIgnoresUnrelatedCorpusErrors is the 0337 regression:
// the integration branch carries a pre-existing corpus record the mutation
// never touches whose bytes fail document.Parse (an ADR with an unquoted
// colon-space title — the live ADR-0024 trigger). The backlink-only patch must
// LAND anyway: the leg's gate is scoped to the artifacts it patches, not the
// health of the integration branch's partial corpus.
func TestIntegrationFinalizeCloseoutBacklinkLegIgnoresUnrelatedCorpusErrors(t *testing.T) {
	requireRealGit(t)
	f := setupCloseoutFixture(t, planRepoModeDocket())
	// Pre-existing, mutation-unrelated corpus error on the integration branch.
	f.repo.writerAdvance(t, "main", map[string]string{
		"docs/adrs/0099-malformed.md": "---\n" +
			"id: 99\n" +
			"title: uses `context: fork` dispatch\n" +
			"status: Accepted\n" +
			"date: 2026-08-22\n" +
			"---\n\n# 99. Malformed on purpose\n",
	})
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)
	mainBefore := originTip(t, f.repo.origin, "main")

	res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if res.Result != ResultApplied || res.Disposition != CloseoutDispDoneArchived {
		t.Fatalf("closeout = %q disp %q (reason %q)", res.Result, res.Disposition, res.Reason)
	}
	// The leg LANDED: no pending finding, and the integration ref advanced.
	for _, fd := range res.Findings {
		if fd.Code == ReasonCloseoutBacklinkPending {
			t.Fatalf("unrelated corpus error refused the backlink leg: %+v", fd)
		}
	}
	mainAfter := originTip(t, f.repo.origin, "main")
	if mainAfter == mainBefore {
		t.Fatalf("the integration ref did not advance (no backlink leg)")
	}
	// The leg's commit touched exactly the plan/results — the malformed record
	// and every other corpus byte are untouched.
	got := originCommitPaths(t, f.repo.origin, mainAfter)
	want := []string{f.planPath, f.resultsPath}
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("integration-ref commit changed %v, want exactly %v", got, want)
	}
	// The retarget itself happened: the plan now backlinks the archive path.
	plan, ok := originFile(t, f.repo.origin, "main", f.planPath)
	if !ok {
		t.Fatalf("plan artifact vanished from main")
	}
	if !strings.Contains(plan, res.ArchivePath) {
		t.Errorf("plan backlink does not point at the archive path %q:\n%s", res.ArchivePath, plan)
	}
	if !strings.Contains(plan, "# Plan\n\nThe widget plan.") {
		t.Errorf("authored plan body disturbed:\n%s", plan)
	}
}

// TestCloseoutBacklinkPendingFindingNamesTheCause is the 0337 diagnosability
// proof (spec D): when the leg still cannot land — here an IN-SCOPE failure,
// the targeted plan artifact's own bytes fail document.Parse — the
// terminal-backlink-pending finding carries the typed cause (the offending
// artifact path), never a bare coarse token. The change itself still closes
// out done+archived: the leg stays best-effort.
func TestIntegrationFinalizeCloseoutBacklinkPendingFindingNamesTheCause(t *testing.T) {
	requireRealGit(t)
	f := setupCloseoutFixture(t, planRepoModeDocket())
	// Corrupt the targeted plan artifact on the integration branch: malformed
	// frontmatter fails document.Parse, an in-scope condition even after the
	// gate is scoped to the patched artifacts.
	f.repo.writerAdvance(t, "main", map[string]string{
		f.planPath: "---\ntitle: uses `context: fork` dispatch\n---\n\n" +
			artifactWithBacklink(groomPath(f.id, f.slug), "Plan", "The widget plan."),
	})
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)

	res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if res.Result != ResultApplied || res.Disposition != CloseoutDispDoneArchived {
		t.Fatalf("closeout = %q disp %q (reason %q)", res.Result, res.Disposition, res.Reason)
	}
	var pending *StatusFinding
	for i, fd := range res.Findings {
		if fd.Code == ReasonCloseoutBacklinkPending {
			pending = &res.Findings[i]
		}
	}
	if pending == nil {
		t.Fatalf("an in-scope malformed artifact did not leave the leg pending: %+v", res.Findings)
	}
	// The finding names the cause: the exact offending artifact path, and the
	// typed detail separator — proof it carries more than the old coarse token.
	if !strings.Contains(pending.Message, f.planPath) {
		t.Errorf("pending finding does not name the offending artifact:\n%s", pending.Message)
	}
	if !strings.Contains(pending.Message, "): ") {
		t.Errorf("pending finding carries no typed detail (still cause-free): %q", pending.Message)
	}
}

// TestCloseoutIdempotent proves a replay after a response-lost success is a
// verified no-op keyed on the promised archive record, never a second commit.
func TestIntegrationFinalizeCloseoutIdempotent(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupCloseoutFixture(t, m)
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)

	first := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if first.Result != ResultApplied || first.Disposition != CloseoutDispDoneArchived {
		t.Fatalf("first closeout = %q disp %q", first.Result, first.Disposition)
	}
	tipAfterFirst := originTip(t, f.repo.origin, f.branch)

	replay := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if replay.Disposition != CloseoutDispAlready {
		t.Fatalf("replay disposition = %q, want %q (result %q)", replay.Disposition, CloseoutDispAlready, replay.Result)
	}
	if replay.Result == ResultApplied {
		t.Fatalf("replay reported a fresh apply; want a no-op")
	}
	if tip := originTip(t, f.repo.origin, f.branch); tip != tipAfterFirst {
		t.Errorf("replay produced a second commit: %q -> %q", tipAfterFirst, tip)
	}
}

// TestCloseoutNeverEditsAuthoredBytes proves the authored content of the merged
// plan/results outside the docket:backlink block is byte-identical after a
// closeout.
func TestIntegrationFinalizeCloseoutNeverEditsAuthoredBytes(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			f := setupCloseoutFixture(t, m)
			mergeCommit := f.mergeIntoBase(t)
			gh := f.baselineMergedFake(f.head, mergeCommit)

			res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
			if res.Result != ResultApplied {
				t.Fatalf("closeout did not apply: %q (reason %q)", res.Result, res.Reason)
			}

			integrationBranch := f.branch
			if m.name == "docket" {
				integrationBranch = "main"
			}
			for _, p := range []string{f.planPath, f.resultsPath} {
				got, ok := originFile(t, f.repo.origin, integrationBranch, p)
				if !ok {
					t.Fatalf("artifact %q vanished", p)
				}
				// The authored body after the backlink block survives verbatim.
				_, body, found := strings.Cut(got, "<!-- docket:backlink:end -->\n")
				if !found {
					t.Fatalf("artifact %q lost its backlink block:\n%s", p, got)
				}
				if !strings.HasPrefix(body, "\n# ") {
					t.Errorf("artifact %q authored body was disturbed:\n%q", p, body)
				}
			}
		})
	}
}

// TestCloseoutNoNotesEmitsNoSection pins the byte-for-byte-today promise.
func TestIntegrationFinalizeCloseoutNoNotesEmitsNoSection(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupCloseoutFixture(t, m)
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)
	res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if res.Result != ResultApplied {
		t.Fatalf("closeout = %q", res.Result)
	}
	archived, _ := originFile(t, f.repo.origin, f.branch, res.ArchivePath)
	if strings.Contains(archived, "## Closeout notes") {
		t.Errorf("no-notes closeout conjured a notes section:\n%s", archived)
	}
}

// TestCloseoutNotesInvalidInputMutatesNothing: empty-after-trim, control
// characters, marker text, and an oversized entry each refuse before any
// probe or transaction — the change stays implemented and the tip unmoved.
func TestIntegrationFinalizeCloseoutNotesInvalidInputMutatesNothing(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupCloseoutFixture(t, m)
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)
	tipBefore := originTip(t, f.repo.origin, f.branch)

	bad := []CloseoutNotes{
		{VerificationOutcomes: []string{"   "}},
		{LateFindings: []string{"bell\x07"}},
		{LateFindings: []string{"crlf\r\nmid"}}, // interior CR survives trimming; must be rejected
		{VerificationOutcomes: []string{"<!-- docket:backlink:start -->"}},
		{VerificationOutcomes: []string{strings.Repeat("a", maxAuthoredMarkdownBytes+1)}},
	}
	for i, n := range bad {
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, n)
		if res.Result != ResultInvalidInput {
			t.Fatalf("bad[%d] result = %q, want invalid input", i, res.Result)
		}
	}
	if tip := originTip(t, f.repo.origin, f.branch); tip != tipBefore {
		t.Errorf("an invalid-notes request moved the metadata tip: %q -> %q", tipBefore, tip)
	}
	if gh.probes != 0 {
		t.Errorf("an invalid-notes request reached the PR probe (%d probes)", gh.probes)
	}
}

// TestCloseoutNotesLandWithArchive proves notes land in the SAME transaction as
// the ordinary archive, as the final section, in both repository modes — and
// that the lifecycle transition still happened.
func TestIntegrationFinalizeCloseoutNotesLandWithArchive(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			f := setupCloseoutFixture(t, m)
			mergeCommit := f.mergeIntoBase(t)
			gh := f.baselineMergedFake(f.head, mergeCommit)

			res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, closeoutTestNotes())
			if res.Result != ResultApplied || res.Disposition != CloseoutDispDoneArchived {
				t.Fatalf("closeout = %q disp %q (reason %q)", res.Result, res.Disposition, res.Reason)
			}
			archived, ok := originFile(t, f.repo.origin, f.branch, res.ArchivePath)
			if !ok {
				t.Fatalf("archived record absent at %q", res.ArchivePath)
			}
			if !strings.HasSuffix(archived, closeoutWantNotesSection) {
				t.Errorf("archived record does not END with the notes section:\n%s", archived)
			}
			if !strings.Contains(archived, "status: 'done'") {
				t.Errorf("notes landed without the lifecycle transition:\n%s", archived)
			}
		})
	}
}

// TestCloseoutNotesReplayAndFrozen: an identical-notes retry replays as a
// no-op with no second commit; a different-notes retry is refused with
// terminal-notes-frozen and moves nothing.
func TestIntegrationFinalizeCloseoutNotesReplayAndFrozen(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupCloseoutFixture(t, m)
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)

	first := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, closeoutTestNotes())
	if first.Result != ResultApplied {
		t.Fatalf("first closeout = %q", first.Result)
	}
	tipAfterFirst := originTip(t, f.repo.origin, f.branch)

	replay := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, closeoutTestNotes())
	if replay.Disposition != CloseoutDispAlready || replay.Result == ResultApplied {
		t.Fatalf("identical-notes replay = %q disp %q, want no-op already", replay.Result, replay.Disposition)
	}

	different := closeoutTestNotes()
	different.LateFindings = []string{"a different late finding"}
	refused := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, different)
	if refused.Reason != ReasonCloseoutNotesFrozen {
		t.Fatalf("different-notes retry reason = %q, want %q (result %q disp %q)",
			refused.Reason, ReasonCloseoutNotesFrozen, refused.Result, refused.Disposition)
	}
	if tip := originTip(t, f.repo.origin, f.branch); tip != tipAfterFirst {
		t.Errorf("a refused retry produced a commit")
	}
	archived, _ := originFile(t, f.repo.origin, f.branch, first.ArchivePath)
	if !strings.HasSuffix(archived, closeoutWantNotesSection) {
		t.Errorf("terminal record's notes changed after the refused retry:\n%s", archived)
	}
}

// TestCloseoutNotesRootCarryNoPropagation proves root notes land ONLY on the
// root: a carried descendant's own notes survive root archival, and the root's
// notes never propagate onto the descendant.
func TestIntegrationFinalizeCloseoutNotesRootCarryNoPropagation(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0] // main mode: one ref carries every backlink

	descPlan := "docs/superpowers/plans/2026-08-16-gadget-plan.md"
	f := setupCloseoutFixture(t, m)
	childPath := groomPath(6, "gadget")
	desc := closeoutRecord(6, "gadget", "implemented", "github.com/acme/widget#8", "", descPlan, "")
	desc = strings.Replace(desc, "stacked_on:\n", "stacked_on: 5\n", 1)
	f.repo.writerAdvance(t, f.branch, map[string]string{
		childPath: desc,
		descPlan:  artifactWithBacklink(childPath, "Gadget plan", "The gadget plan."),
	})

	// Establish the ROOT's verified merge into integration up front (in main mode
	// the metadata ref IS main, so this must land before any closeout advances it).
	mergeCommit := f.mergeIntoBase(t)

	// First: close out the CHILD (id 6) as stacked-merged into the root's branch,
	// carrying its OWN notes.
	childNotes := CloseoutNotes{LateFindings: []string{"child-owned note"}}
	// The child's PR #8 merged its work into the root's live branch (feat/widget);
	// its merge result is that branch's head, so requireStackedPreservation proves
	// it preserved there by ancestry (a real object, not a fabricated one).
	ghChild := &fakeCloseoutGitHub{
		repo: retargetRepo(),
		merged: map[int]closeoutProbe{
			8: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feat/widget", f.head)},
		},
	}
	childRes := FinalizeCloseout(context.Background(), f.closeoutDeps(ghChild), f.repo.invocation, 6, childNotes)
	if childRes.Result != ResultApplied || childRes.Disposition != CloseoutDispStackedMerged {
		t.Fatalf("child closeout = %q disp %q (reason %q)", childRes.Result, childRes.Disposition, childRes.Reason)
	}

	// Then: carry the ROOT (id 5) with DIFFERENT notes.
	ghRoot := &fakeCloseoutGitHub{
		repo: retargetRepo(),
		merged: map[int]closeoutProbe{
			closeoutPR: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "main", mergeCommit)},
			8:          {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feat/widget", f.head)},
		},
	}
	rootRes := FinalizeCloseout(context.Background(), f.closeoutDeps(ghRoot), f.repo.invocation, f.id, closeoutTestNotes())
	if rootRes.Result != ResultApplied || rootRes.Disposition != CloseoutDispRootArchived {
		t.Fatalf("root carry = %q disp %q (reason %q)", rootRes.Result, rootRes.Disposition, rootRes.Reason)
	}

	// (a) the archived ROOT record ends with the root's notes section.
	rootArchived, ok := originFile(t, f.repo.origin, f.branch, "docs/changes/archive/2026-08-18-0005-widget.md")
	if !ok {
		t.Fatalf("root archived record absent")
	}
	if !strings.HasSuffix(rootArchived, closeoutWantNotesSection) {
		t.Errorf("archived ROOT record does not end with its notes section:\n%s", rootArchived)
	}

	childArchived, ok := originFile(t, f.repo.origin, f.branch, "docs/changes/archive/2026-08-18-0006-gadget.md")
	if !ok {
		t.Fatalf("child archived record absent")
	}
	// (b) the root's notes never propagated onto the descendant.
	if strings.Contains(childArchived, "Production health check") {
		t.Errorf("root notes propagated to the carried child:\n%s", childArchived)
	}
	// (c) the child's own notes survived root archival.
	if !strings.Contains(childArchived, "child-owned note") {
		t.Errorf("child's own notes did not survive root archival:\n%s", childArchived)
	}
}

// TestCloseoutNotesStackedInPlace proves the stacked-merged in-place path also
// carries notes into the terminal record, with the same replay/frozen semantics.
func TestIntegrationFinalizeCloseoutNotesStackedInPlace(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupCloseoutFixture(t, m)

	recPath, mc := f.carryLiveParent(t, "implemented", "feat/parent")

	gh := &fakeCloseoutGitHub{
		repo: retargetRepo(),
		merged: map[int]closeoutProbe{
			closeoutPR: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feat/parent", mc)},
		},
	}
	res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, closeoutTestNotes())
	if res.Result != ResultApplied || res.Disposition != CloseoutDispStackedMerged {
		t.Fatalf("closeout = %q disp %q (reason %q)", res.Result, res.Disposition, res.Reason)
	}

	rec, ok := originFile(t, f.repo.origin, f.branch, recPath)
	if !ok {
		t.Fatalf("stacked-merged record was archived away from %q", recPath)
	}
	if !strings.Contains(rec, "status: 'stacked-merged'") {
		t.Errorf("record not stacked-merged:\n%s", rec)
	}
	if !strings.HasSuffix(rec, closeoutWantNotesSection) {
		t.Errorf("in-place record does not END with the notes section:\n%s", rec)
	}

	// Identical-notes replay is a no-op; different notes are frozen.
	replay := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, closeoutTestNotes())
	if replay.Disposition != CloseoutDispAlready || replay.Result == ResultApplied {
		t.Fatalf("stacked identical-notes replay = %q disp %q, want no-op already", replay.Result, replay.Disposition)
	}
	different := closeoutTestNotes()
	different.LateFindings = []string{"a different late finding"}
	refused := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, different)
	if refused.Reason != ReasonCloseoutNotesFrozen {
		t.Fatalf("stacked different-notes retry reason = %q, want %q (result %q)", refused.Reason, ReasonCloseoutNotesFrozen, refused.Result)
	}
}

// TestCloseoutOrdinary proves an ordinary verified-integration merge is closed
// out in one transaction: the record is marked done and relocated to the dated
// archive path, its claim is cleared, its updated stamp is the merge date, its
// board is refreshed, and every backlink retargets to the archive path. It runs
// in both metadata modes.
func TestIntegrationFinalizeCloseoutOrdinary(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			f := setupCloseoutFixture(t, m)
			mergeCommit := f.mergeIntoBase(t)
			gh := f.baselineMergedFake(f.head, mergeCommit)

			res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
			if res.Result != ResultApplied || res.Disposition != CloseoutDispDoneArchived {
				t.Fatalf("closeout = %q disp %q (reason %q msg %q)", res.Result, res.Disposition, res.Reason, res.Message)
			}
			archivePath := "docs/changes/archive/2026-08-18-0005-widget.md"
			if res.ArchivePath != archivePath {
				t.Fatalf("archive path = %q, want %q", res.ArchivePath, archivePath)
			}

			// The active record is gone; the archived record is done, claimless, and
			// stamped with the merge date.
			recPath := groomPath(f.id, f.slug)
			if _, ok := originFile(t, f.repo.origin, f.branch, recPath); ok {
				t.Errorf("active record still present after closeout (presence-encoded state)")
			}
			archived, ok := originFile(t, f.repo.origin, f.branch, archivePath)
			if !ok {
				t.Fatalf("archived record absent at %q", archivePath)
			}
			if !strings.Contains(archived, "status: 'done'") {
				t.Errorf("archived record not done:\n%s", archived)
			}
			if !strings.Contains(archived, "updated: '2026-08-18'") {
				t.Errorf("archived record not stamped with the merge date:\n%s", archived)
			}
			if strings.Contains(archived, "claimed_at: '2026-08-02") {
				t.Errorf("archived record still carries a claim stamp:\n%s", archived)
			}
			// Historical branch/PR fields survive.
			if !strings.Contains(archived, "pr: '"+closeoutRef+"'") {
				t.Errorf("archived record dropped its historical PR field:\n%s", archived)
			}

			// The spec backlink (metadata ref) retargets to the archive path.
			spec, _ := originFile(t, f.repo.origin, f.branch, f.specPath)
			if !strings.Contains(spec, archivePath) || strings.Contains(spec, "`"+recPath+"`") {
				t.Errorf("spec backlink not retargeted to the archive path:\n%s", spec)
			}

			// The plan/results backlinks (integration ref in docket mode) retarget too.
			integrationBranch := f.branch
			if m.name == "docket" {
				integrationBranch = "main"
			}
			for _, p := range []string{f.planPath, f.resultsPath} {
				got, ok := originFile(t, f.repo.origin, integrationBranch, p)
				if !ok {
					t.Fatalf("artifact %q vanished", p)
				}
				if !strings.Contains(got, archivePath) || strings.Contains(got, "`"+recPath+"`") {
					t.Errorf("artifact %q backlink not retargeted:\n%s", p, got)
				}
			}

			// The board is refreshed and current; no feature-branch state is deleted.
			assertBoardMatchesCommitted(t, f.repo.origin, f.branch, f.repo.invocation)
		})
	}
}

// TestCloseoutRefusals proves an open PR, an unknown probe, a destination
// mismatch, and an illegal source status each refuse with a closed disposition
// and land nothing on the metadata ref.
func TestIntegrationFinalizeCloseoutRefusals(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]

	t.Run("open-pr-not-merged", func(t *testing.T) {
		f := setupCloseoutFixture(t, m)
		gh := &fakeCloseoutGitHub{repo: retargetRepo(), merged: map[int]closeoutProbe{}} // #7 not merged
		before := originTip(t, f.repo.origin, f.branch)
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result == ResultApplied || res.Result == ResultNoOp {
			t.Fatalf("a not-merged PR reported success %q", res.Result)
		}
		if res.Disposition != CloseoutDispBlocked {
			t.Errorf("disposition = %q, want %q", res.Disposition, CloseoutDispBlocked)
		}
		if after := originTip(t, f.repo.origin, f.branch); after != before {
			t.Errorf("a refusal moved the metadata ref: %q -> %q", before, after)
		}
	})

	t.Run("probe-unknown-retains", func(t *testing.T) {
		f := setupCloseoutFixture(t, m)
		gh := &fakeCloseoutGitHub{repo: retargetRepo(), probeErr: errors.New("gh probe boom")}
		before := originTip(t, f.repo.origin, f.branch)
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Disposition != CloseoutDispUnknown {
			t.Fatalf("disposition = %q, want %q (result %q)", res.Disposition, CloseoutDispUnknown, res.Result)
		}
		if after := originTip(t, f.repo.origin, f.branch); after != before {
			t.Errorf("an unknown probe moved the metadata ref: %q -> %q", before, after)
		}
	})

	t.Run("unreachable-merge-commit-contended", func(t *testing.T) {
		f := setupCloseoutFixture(t, m)
		// Merged facts name main + a real object (the feature head) that is NOT
		// reachable from main's tip: a present-but-unreachable answer, contended.
		gh := f.baselineMergedFake(f.head, f.head)
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result != ResultContended || res.Disposition != CloseoutDispContended {
			t.Fatalf("unreachable merge = %q disp %q, want contended", res.Result, res.Disposition)
		}
	})

	t.Run("illegal-source-status", func(t *testing.T) {
		f := setupCloseoutFixture(t, m)
		// A proposed record is not a legal closeout source.
		recPath := groomPath(f.id, f.slug)
		f.repo.writerAdvance(t, f.branch, map[string]string{
			recPath: closeoutRecord(f.id, f.slug, "proposed", closeoutRef, f.specPath, f.planPath, f.resultsPath),
		})
		mergeCommit := f.mergeIntoBase(t)
		gh := f.baselineMergedFake(f.head, mergeCommit)
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result == ResultApplied || res.Result == ResultNoOp {
			t.Fatalf("closeout of a proposed record reported success %q", res.Result)
		}
		if res.Disposition != CloseoutDispBlocked {
			t.Errorf("disposition = %q, want %q", res.Disposition, CloseoutDispBlocked)
		}
	})
}

// carryOntoRootFeature commits files onto the ROOT's real feature branch
// (feat/<slug>) on origin, extending f.head, and returns the new commit OID. The
// root merge (mergeIntoBase) then carries these files into main, so the returned
// commit is a real ancestor of the integration tip — the child's authoritative
// merge result really shipped through the root.
func (f *closeoutFixture) carryOntoRootFeature(t *testing.T, files map[string]string) string {
	t.Helper()
	w := f.repo.writer
	runGit(t, w, "fetch", "-q", "origin", "feat/"+f.slug)
	runGit(t, w, "checkout", "-q", "-B", "feat/"+f.slug, "FETCH_HEAD")
	for rel, content := range files {
		writeRepoFile(t, w, rel, content)
	}
	runGit(t, w, "add", "-A")
	runGit(t, w, "commit", "-q", "-m", "carry onto feat/"+f.slug)
	mc := runGit(t, w, "rev-parse", "HEAD")
	runGit(t, w, "push", "-q", "origin", "feat/"+f.slug)
	return mc
}

// TestCloseoutRootCarry proves a stack root merged to integration archives the
// root plus every proven carried descendant in ONE transaction using the root's
// merge date for every filename; a single unproven descendant keeps the root
// recoverable with zero descendant writes; and — the change-0327 headline — the
// archive is refused unless every carried descendant's merged work is proven
// PRESENT IN GIT (ancestry in the pinned integration history, or exact content at
// the ROOT's merge result), never inferred from a metadata destination.
//
// The safety-net sweep reaches this same enforcement: its only closeout action is
// the verbatim call `return FinalizeCloseout(ctx, depsFor(obs), repoDir, id,
// CloseoutNotes{})` (internal/app/maintenance.go), so a direct FinalizeCloseout
// invocation exercises the exact path the sweep drives (maintenance-sweep-same-refusal).
func TestIntegrationFinalizeCloseoutRootCarry(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0] // main mode: one ref carries every backlink

	descPlan := "docs/superpowers/plans/2026-08-16-gadget-plan.md"

	// seed returns a descendant record (id 6, gadget) stacked on the root (id 5)
	// with the given status, carrying PR #8 whose destination is the root's branch.
	seed := func(t *testing.T, descStatus string) *closeoutFixture {
		f := setupCloseoutFixture(t, m)
		recPath := groomPath(6, "gadget")
		desc := closeoutRecord(6, "gadget", descStatus, "github.com/acme/widget#8", "", descPlan, "")
		desc = strings.Replace(desc, "stacked_on:\n", "stacked_on: 5\n", 1)
		f.repo.writerAdvance(t, f.branch, map[string]string{
			recPath:  desc,
			descPlan: artifactWithBacklink(recPath, "Gadget plan", "The gadget plan."),
		})
		return f
	}

	// rootFake scripts PR #7 (root) merged into main at rootMerge and PR #8
	// (descendant id 6) merged into the root's branch at childMerge.
	rootFake := func(f *closeoutFixture, rootMerge, childMerge string) *fakeCloseoutGitHub {
		return &fakeCloseoutGitHub{
			repo: retargetRepo(),
			merged: map[int]closeoutProbe{
				closeoutPR: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "main", rootMerge)},
				8:          {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(childMerge, "feat/widget", childMerge)},
			},
		}
	}

	assertBothArchived := func(t *testing.T, f *closeoutFixture, res CloseoutResult) {
		t.Helper()
		if res.Result != ResultApplied || res.Disposition != CloseoutDispRootArchived {
			t.Fatalf("root carry = %q disp %q (reason %q msg %q)", res.Result, res.Disposition, res.Reason, res.Message)
		}
		// Both records archived under the ROOT's merge date.
		for _, p := range []string{
			"docs/changes/archive/2026-08-18-0005-widget.md",
			"docs/changes/archive/2026-08-18-0006-gadget.md",
		} {
			archived, ok := originFile(t, f.repo.origin, f.branch, p)
			if !ok {
				t.Fatalf("archived record absent at %q", p)
			}
			if !strings.Contains(archived, "status: 'done'") {
				t.Errorf("archived record %q not done:\n%s", p, archived)
			}
		}
		if got := res.CarriedIDs; len(got) != 1 || got[0] != 6 {
			t.Errorf("carried ids = %v, want [6]", got)
		}
		assertBoardMatchesCommitted(t, f.repo.origin, f.branch, f.repo.invocation)
	}

	t.Run("all-proven-archives-root-and-descendants", func(t *testing.T) {
		f := seed(t, "stacked-merged")
		// Descendant id 6's PR merged its gadget.txt into the ROOT's own feature
		// branch; the root merge then carries it into main, so the child's real
		// merge result is an ancestor of the integration tip (proven by ancestry).
		childMerge := f.carryOntoRootFeature(t, map[string]string{"gadget.txt": "gadget work\n"})
		mergeCommit := f.mergeIntoBase(t)
		f.fetchAllIntoInvocation(t)
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(rootFake(f, mergeCommit, childMerge)), f.repo.invocation, f.id, CloseoutNotes{})
		assertBothArchived(t, f, res)
	})

	t.Run("one-unproven-descendant-keeps-root-recoverable", func(t *testing.T) {
		f := seed(t, "implemented") // NOT stacked-merged: carry unproven
		mergeCommit := f.mergeIntoBase(t)
		gh := &fakeCloseoutGitHub{
			repo: retargetRepo(),
			merged: map[int]closeoutProbe{
				closeoutPR: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "main", mergeCommit)},
			},
		}
		before := originTip(t, f.repo.origin, f.branch)
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result == ResultApplied || res.Result == ResultNoOp {
			t.Fatalf("an unproven descendant let the root close out: %q", res.Result)
		}
		if res.Disposition != CloseoutDispChildrenRetargetRequired {
			t.Errorf("disposition = %q, want %q", res.Disposition, CloseoutDispChildrenRetargetRequired)
		}
		// Zero writes: neither record moved.
		if after := originTip(t, f.repo.origin, f.branch); after != before {
			t.Errorf("an unproven root carry moved the metadata ref: %q -> %q", before, after)
		}
		if _, ok := originFile(t, f.repo.origin, f.branch, groomPath(f.id, f.slug)); !ok {
			t.Errorf("the recoverable root was archived away")
		}
	})

	t.Run("absent-merge-object-refuses", func(t *testing.T) {
		// The carry RELATIONSHIP is proven (PR #8 merged into the root's branch),
		// but the recorded child merge id is well-formed and NONEXISTENT: the Git
		// proof cannot observe it, so the closeout is retained as unknown and
		// archives nothing (green-suite-untested-branch: a fabricated id must no
		// longer pass — distinct from a real object that is dropped).
		f := seed(t, "stacked-merged")
		mergeCommit := f.mergeIntoBase(t)
		before := originTip(t, f.repo.origin, f.branch)
		gh := rootFake(f, mergeCommit, strings.Repeat("b", 40))
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result == ResultApplied || res.Result == ResultNoOp {
			t.Fatalf("a fabricated child merge id let the root close out: %q disp %q", res.Result, res.Disposition)
		}
		if res.Disposition != CloseoutDispUnknown {
			t.Fatalf("absent merge object = disp %q reason %q, want %q (an unobservable object is a retained observation failure)", res.Disposition, res.Reason, CloseoutDispUnknown)
		}
		if after := originTip(t, f.repo.origin, f.branch); after != before {
			t.Errorf("a retained closeout moved the metadata ref: %q -> %q", before, after)
		}
		for _, p := range []string{groomPath(5, "widget"), groomPath(6, "gadget")} {
			if _, ok := originFile(t, f.repo.origin, f.branch, p); !ok {
				t.Errorf("a retained closeout archived %q away", p)
			}
		}
	})

	t.Run("descendant-dropped-refuses", func(t *testing.T) {
		// A REAL child merge commit adds gadget.txt on a side branch (so the object
		// survives), but the root branch never carries the content — mergeIntoBase
		// ships only feature.txt. The Git proof observes the drop and blocks; the
		// root stays fully recoverable. This is the discriminating fixture the
		// fabricated `bbbb…` positive used to hide (green-suite-untested-branch).
		f := seed(t, "stacked-merged")
		childMerge := f.carryCommit(t, "gadget-mc", "main", map[string]string{"gadget.txt": "gadget work\n"})
		mergeCommit := f.mergeIntoBase(t)
		f.fetchAllIntoInvocation(t)
		// The source merge object EXISTS (a dropped object, not a missing one).
		if _, err := tryGit(f.repo.invocation, "cat-file", "-e", childMerge); err != nil {
			t.Fatalf("the child merge object must exist to pin a drop (not a missing object): %v", err)
		}
		before := originTip(t, f.repo.origin, f.branch)
		featBefore := originTip(t, f.repo.origin, "feat/"+f.slug)
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(rootFake(f, mergeCommit, childMerge)), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result != ResultBlocked || res.Disposition != CloseoutDispBlocked || res.Reason != ReasonCloseoutChildUnproven {
			t.Fatalf("dropped descendant = %q disp %q reason %q, want blocked/%s/%s", res.Result, res.Disposition, res.Reason, CloseoutDispBlocked, ReasonCloseoutChildUnproven)
		}
		// Pin the missing effects: metadata ref byte-identical, no archive files,
		// the child's carrier ref untouched.
		if after := originTip(t, f.repo.origin, f.branch); after != before {
			t.Errorf("a refused root carry moved the metadata ref: %q -> %q", before, after)
		}
		if after := originTip(t, f.repo.origin, "feat/"+f.slug); after != featBefore {
			t.Errorf("a refused root carry moved the child's feature ref: %q -> %q", featBefore, after)
		}
		for _, p := range []string{
			"docs/changes/archive/2026-08-18-0005-widget.md",
			"docs/changes/archive/2026-08-18-0006-gadget.md",
		} {
			if _, ok := originFile(t, f.repo.origin, f.branch, p); ok {
				t.Errorf("a refused root carry archived %q", p)
			}
		}
		for _, p := range []string{groomPath(5, "widget"), groomPath(6, "gadget")} {
			if _, ok := originFile(t, f.repo.origin, f.branch, p); !ok {
				t.Errorf("a refused root carry removed the active record %q", p)
			}
		}
	})

	t.Run("squash-rewrite-archives", func(t *testing.T) {
		// The child's merge id is squashed away: NOT an ancestor of the root merge
		// result, but the identical gadget.txt bytes are carried onto the root
		// branch, so the exact-content arm proves preservation against the ROOT's
		// merge result and the root archives.
		f := seed(t, "stacked-merged")
		childMerge := f.carryCommit(t, "gadget-mc", "main", map[string]string{"gadget.txt": "X\n"})
		f.carryOntoRootFeature(t, map[string]string{"gadget.txt": "X\n"})
		mergeCommit := f.mergeIntoBase(t)
		f.fetchAllIntoInvocation(t)
		// The child merge id must NOT be an ancestor of the root merge result, or
		// the ancestry arm — not the content arm — would carry the proof.
		if _, err := tryGit(f.repo.invocation, "merge-base", "--is-ancestor", childMerge, mergeCommit); err == nil {
			t.Fatalf("the squash fixture requires the child merge id NOT be an ancestor of the root merge result (content arm must carry the proof)")
		}
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(rootFake(f, mergeCommit, childMerge)), f.repo.invocation, f.id, CloseoutNotes{})
		assertBothArchived(t, f, res)
	})

	t.Run("integration-advanced-after-root-merge", func(t *testing.T) {
		// After the root merge, integration advances with an unrelated commit that
		// EDITS gadget.txt. The content fallback must target the ROOT's merge
		// result, not the moving integration tip, so preservation still holds — this
		// reddens if the implementation compares against the tip.
		f := seed(t, "stacked-merged")
		childMerge := f.carryCommit(t, "gadget-mc", "main", map[string]string{"gadget.txt": "X\n"})
		f.carryOntoRootFeature(t, map[string]string{"gadget.txt": "X\n"})
		mergeCommit := f.mergeIntoBase(t)
		f.carryCommit(t, "main", "main", map[string]string{"gadget.txt": "Y (edited downstream)\n"})
		f.fetchAllIntoInvocation(t)
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(rootFake(f, mergeCommit, childMerge)), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result != ResultApplied || res.Disposition != CloseoutDispRootArchived {
			t.Fatalf("advanced-integration carry = %q disp %q (reason %q msg %q); the fallback must target the ROOT merge result, not the tip", res.Result, res.Disposition, res.Reason, res.Message)
		}
	})

	t.Run("maintenance-sweep-same-refusal", func(t *testing.T) {
		// The safety-net sweep's only closeout action is the verbatim call
		// `return FinalizeCloseout(ctx, depsFor(obs), repoDir, id, CloseoutNotes{})`
		// (internal/app/maintenance.go), so this direct invocation exercises the
		// exact path the sweep drives: the root-carry Git proof refuses a dropped
		// descendant identically whether reached from the CLI or the sweep.
		f := seed(t, "stacked-merged")
		childMerge := f.carryCommit(t, "gadget-mc", "main", map[string]string{"gadget.txt": "gadget work\n"})
		mergeCommit := f.mergeIntoBase(t)
		f.fetchAllIntoInvocation(t)
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(rootFake(f, mergeCommit, childMerge)), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result != ResultBlocked || res.Disposition != CloseoutDispBlocked || res.Reason != ReasonCloseoutChildUnproven {
			t.Fatalf("sweep-entry closeout = %q disp %q reason %q, want blocked/%s/%s", res.Result, res.Disposition, res.Reason, CloseoutDispBlocked, ReasonCloseoutChildUnproven)
		}
	})
}

// TestCloseoutStackedMerged proves a change whose verified PR destination is its
// live parent's branch is marked stacked-merged IN PLACE — not archived, its
// feature branch and workspace retained — and the board is rerendered.
// seedStackedChild patches the fixture child (id 5) to stack on a live parent
// (id 4) recorded with parentBranch at childStatus, writing only metadata. The
// parent's Git branch (and the merge commit it carries) is built separately by a
// carry helper so requireStackedPreservation reads a REAL parent head.
func (f *closeoutFixture) seedStackedChild(t *testing.T, childStatus, parentBranch string) string {
	t.Helper()
	recPath := groomPath(f.id, f.slug)
	child := closeoutRecord(f.id, f.slug, childStatus, closeoutRef, f.specPath, f.planPath, f.resultsPath)
	child = strings.Replace(child, "stacked_on:\n", "stacked_on: 4\n", 1)
	parent := strings.Replace(lifecycleChange(4, "parent", "in-progress"), "branch: feat/parent\n", "branch: "+parentBranch+"\n", 1)
	f.repo.writerAdvance(t, f.branch, map[string]string{
		groomPath(4, "parent"): parent,
		recPath:                child,
	})
	return recPath
}

// carryLiveParent seeds the stack (child at childStatus on parentBranch) and
// builds parentBranch on origin so its head IS the child's merge-result commit
// (proven by ancestry), returning the record path and that real merge id.
func (f *closeoutFixture) carryLiveParent(t *testing.T, childStatus, parentBranch string) (string, string) {
	t.Helper()
	recPath := f.seedStackedChild(t, childStatus, parentBranch)
	mc := f.carryCommit(t, parentBranch, "main", map[string]string{"parent-carry.txt": "carried\n"})
	f.fetchAllIntoInvocation(t)
	return recPath, mc
}

func TestIntegrationFinalizeCloseoutStackedMerged(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupCloseoutFixture(t, m)

	// A live parent (id 4) the fixture child (id 5) stacks on; the child's PR
	// merged into the parent's feature branch feat/parent, which really carries
	// the merge (requireStackedPreservation proves it by ancestry).
	recPath, mc := f.carryLiveParent(t, "implemented", "feat/parent")

	gh := &fakeCloseoutGitHub{
		repo: retargetRepo(),
		merged: map[int]closeoutProbe{
			closeoutPR: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feat/parent", mc)},
		},
	}
	res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if res.Result != ResultApplied || res.Disposition != CloseoutDispStackedMerged {
		t.Fatalf("closeout = %q disp %q (reason %q msg %q)", res.Result, res.Disposition, res.Reason, res.Message)
	}

	// The record is edited in place, not archived.
	rec, ok := originFile(t, f.repo.origin, f.branch, recPath)
	if !ok {
		t.Fatalf("stacked-merged record was archived away from %q", recPath)
	}
	if !strings.Contains(rec, "status: 'stacked-merged'") {
		t.Errorf("record not stacked-merged:\n%s", rec)
	}
	if _, ok := originFile(t, f.repo.origin, f.branch, "docs/changes/archive/2026-08-18-0005-widget.md"); ok {
		t.Errorf("a stacked-merged closeout archived the record")
	}
	assertBoardMatchesCommitted(t, f.repo.origin, f.branch, f.repo.invocation)

	// A replay is a verified no-op keyed on the promised stacked-merged state.
	replay := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if replay.Disposition != CloseoutDispAlready {
		t.Fatalf("stacked replay disposition = %q, want %q", replay.Disposition, CloseoutDispAlready)
	}
}

// TestCloseoutStackedParentBranchIdentity proves the stacked-merged path reads
// the live PARENT's OWN recorded branch (never a slug-derived name): a merge
// into the parent's non-derived recorded branch takes the in-place stacked path,
// and a live parent whose record carries no branch fails closed to invalid-state
// with the child record left untouched.
func TestIntegrationFinalizeCloseoutStackedParentBranchIdentity(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]

	seedChild := func(t *testing.T, f *closeoutFixture, parentRecord string) string {
		t.Helper()
		recPath := groomPath(f.id, f.slug)
		child := closeoutRecord(f.id, f.slug, "implemented", closeoutRef, f.specPath, f.planPath, f.resultsPath)
		child = strings.Replace(child, "stacked_on:\n", "stacked_on: 4\n", 1)
		f.repo.writerAdvance(t, f.branch, map[string]string{
			groomPath(4, "parent"): parentRecord,
			recPath:                child,
		})
		return recPath
	}

	t.Run("non-derived-parent-branch-honored", func(t *testing.T) {
		f := setupCloseoutFixture(t, m)
		recPath, mc := f.carryLiveParent(t, "implemented", "feature/live-parent")
		gh := &fakeCloseoutGitHub{
			repo: retargetRepo(),
			merged: map[int]closeoutProbe{
				closeoutPR: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feature/live-parent", mc)},
			},
		}
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result != ResultApplied || res.Disposition != CloseoutDispStackedMerged {
			t.Fatalf("closeout = %q disp %q (reason %q msg %q); the parent's recorded non-derived branch must anchor the stacked path", res.Result, res.Disposition, res.Reason, res.Message)
		}
		rec, ok := originFile(t, f.repo.origin, f.branch, recPath)
		if !ok || !strings.Contains(rec, "status: 'stacked-merged'") {
			t.Errorf("record not marked stacked-merged in place:\n%s", rec)
		}
	})

	t.Run("missing-parent-branch-refuses-untouched", func(t *testing.T) {
		f := setupCloseoutFixture(t, m)
		parent := strings.Replace(lifecycleChange(4, "parent", "in-progress"), "branch: feat/parent\n", "", 1)
		recPath := seedChild(t, f, parent)
		gh := &fakeCloseoutGitHub{
			repo: retargetRepo(),
			merged: map[int]closeoutProbe{
				closeoutPR: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feature/live-parent", strings.Repeat("a", 40))},
			},
		}
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result != ResultInvalidState {
			t.Fatalf("closeout = %q disp %q reason %q, want invalid-state on a live parent with no recorded branch", res.Result, res.Disposition, res.Reason)
		}
		rec, ok := originFile(t, f.repo.origin, f.branch, recPath)
		if !ok || strings.Contains(rec, "stacked-merged") {
			t.Errorf("a refused stacked closeout must leave the child record untouched (never marked stacked-merged):\n%s", rec)
		}
	})
}

// carryStackDroppedContent seeds the stack (child at childStatus on parentBranch)
// and builds two real siblings off main: the child's merge-result commit (which
// EXISTS on its own origin branch, so it is a dropped object, not a missing one)
// carrying catalog.yaml, and parentBranch WITHOUT that content. Returns the
// record path and the surviving merge id.
func (f *closeoutFixture) carryStackDroppedContent(t *testing.T, childStatus, parentBranch string) (string, string) {
	t.Helper()
	recPath := f.seedStackedChild(t, childStatus, parentBranch)
	mc := f.carryCommit(t, "mc-keep", "main", map[string]string{"catalog.yaml": "child-work\n"})
	f.carryCommit(t, parentBranch, "main", map[string]string{"other.txt": "unrelated\n"})
	f.fetchAllIntoInvocation(t)
	return recPath, mc
}

// carryStackRebasedPreserved seeds the stack and builds two siblings off main
// both adding catalog.yaml with identical bytes: the child's merge-result commit
// (mc) and parentBranch's head. mc is NOT an ancestor of the parent head (a
// legitimate rebase rewrote the commit id), but the content reproduces exactly —
// the exact-content preservation arm proves it. Returns the record path and mc.
func (f *closeoutFixture) carryStackRebasedPreserved(t *testing.T, childStatus, parentBranch string) (string, string) {
	t.Helper()
	recPath := f.seedStackedChild(t, childStatus, parentBranch)
	mc := f.carryCommit(t, "mc-keep", "main", map[string]string{"catalog.yaml": "X\n"})
	f.carryCommit(t, parentBranch, "main", map[string]string{"catalog.yaml": "X\n"})
	f.fetchAllIntoInvocation(t)
	return recPath, mc
}

// TestIntegrationFinalizeCloseoutStackedPreservation proves the stacked-merged
// closeout path (fresh marking AND the already-stacked-merged replay) refuses
// unless the child's merge result is preserved at the parent's freshly pinned
// remote head — a historical PR destination is a relationship, never evidence the
// parent still carries the merge (change 0327).
func TestIntegrationFinalizeCloseoutStackedPreservation(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]

	factsFor := func(mc string) *fakeCloseoutGitHub {
		return &fakeCloseoutGitHub{
			repo: retargetRepo(),
			merged: map[int]closeoutProbe{
				closeoutPR: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor("headoid", "feat/parent", mc)},
			},
		}
	}

	t.Run("fresh-marking-content-dropped-refuses", func(t *testing.T) {
		f := setupCloseoutFixture(t, m)
		recPath, mc := f.carryStackDroppedContent(t, "implemented", "feat/parent")
		// The source merge object EXISTS (a dropped object, not a missing one).
		if _, err := tryGit(f.repo.invocation, "cat-file", "-e", mc); err != nil {
			t.Fatalf("the child merge object must exist to pin a drop (not a missing object): %v", err)
		}
		before := originTip(t, f.repo.origin, f.branch)
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(factsFor(mc)), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result != ResultBlocked || res.Disposition != CloseoutDispBlocked || res.Reason != ReasonCloseoutStackedUnpreserved {
			t.Fatalf("dropped-content fresh marking = %q disp %q reason %q, want blocked/%s/%s", res.Result, res.Disposition, res.Reason, CloseoutDispBlocked, ReasonCloseoutStackedUnpreserved)
		}
		if after := originTip(t, f.repo.origin, f.branch); after != before {
			t.Errorf("a refused stacked marking moved the metadata ref: %q -> %q", before, after)
		}
		rec, ok := originFile(t, f.repo.origin, f.branch, recPath)
		if !ok || !strings.Contains(rec, "status: implemented") || strings.Contains(rec, "stacked-merged") {
			t.Errorf("a refused fresh marking must leave the child implemented and untouched:\n%s", rec)
		}
	})

	t.Run("fresh-marking-content-preserved-after-rebase-applies", func(t *testing.T) {
		f := setupCloseoutFixture(t, m)
		_, mc := f.carryStackRebasedPreserved(t, "implemented", "feat/parent")
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(factsFor(mc)), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result != ResultApplied || res.Disposition != CloseoutDispStackedMerged {
			t.Fatalf("content-preserved rebase = %q disp %q reason %q, want applied/%s (the exact-content arm accepts a legitimate rewrite)", res.Result, res.Disposition, res.Reason, CloseoutDispStackedMerged)
		}
	})

	t.Run("replay-content-dropped-refuses", func(t *testing.T) {
		// The record is ALREADY stacked-merged; the parent has since dropped the
		// content. The proof gates the replay too, so it refuses instead of
		// returning a verified no-op (spec: "This also applies to the replay path").
		f := setupCloseoutFixture(t, m)
		_, mc := f.carryStackDroppedContent(t, "stacked-merged", "feat/parent")
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(factsFor(mc)), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result != ResultBlocked || res.Disposition != CloseoutDispBlocked || res.Reason != ReasonCloseoutStackedUnpreserved {
			t.Fatalf("dropped-content replay = %q disp %q reason %q, want blocked/%s/%s (never a verified no-op)", res.Result, res.Disposition, res.Reason, CloseoutDispBlocked, ReasonCloseoutStackedUnpreserved)
		}
		if res.Disposition == CloseoutDispAlready {
			t.Fatalf("a replay whose parent dropped the content must not report %q", CloseoutDispAlready)
		}
	})

	t.Run("replay-content-intact-stays-idempotent", func(t *testing.T) {
		// Already stacked-merged AND the parent still carries the merge: idempotency
		// is preserved — the promise still holds, so the replay is a verified no-op.
		f := setupCloseoutFixture(t, m)
		_, mc := f.carryLiveParent(t, "stacked-merged", "feat/parent")
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(factsFor(mc)), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Disposition != CloseoutDispAlready || res.Result == ResultApplied {
			t.Fatalf("content-intact replay = %q disp %q, want a no-op %s", res.Result, res.Disposition, CloseoutDispAlready)
		}
	})

	t.Run("unusable-merge-id-distinguishes-missing-evidence", func(t *testing.T) {
		// An unusable merge id is missing evidence (CarryFindingMissingMerge), NOT
		// an observed content mismatch. reprobeMerged's "no usable merge commit or
		// merge date" guard catches it upstream of requireStackedPreservation, so the
		// closeout is retained as unverified-merge and never reaches — or is confused
		// with — the stacked-unpreserved content proof.
		f := setupCloseoutFixture(t, m)
		recPath, _ := f.carryLiveParent(t, "implemented", "feat/parent")
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(factsFor("not-a-full-object-id")), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result == ResultApplied || res.Result == ResultNoOp {
			t.Fatalf("an unusable merge id let the stacked closeout proceed: %q", res.Result)
		}
		if res.Reason != ReasonCloseoutUnverifiedMerge {
			t.Fatalf("unusable merge id reason = %q, want %q (missing evidence, distinct from %q)", res.Reason, ReasonCloseoutUnverifiedMerge, ReasonCloseoutStackedUnpreserved)
		}
		rec, ok := originFile(t, f.repo.origin, f.branch, recPath)
		if !ok || strings.Contains(rec, "stacked-merged") {
			t.Errorf("a retained closeout must leave the child untouched:\n%s", rec)
		}
	})

	t.Run("parent-branch-fetch-error-is-unknown", func(t *testing.T) {
		// The parent's recorded branch was never pushed to origin: routing still
		// reaches the stacked path (the destination matches the recorded branch), but
		// FetchBranch fails, so the preservation query cannot run — an observation
		// failure is retained as unknown, never blocked and never applied.
		f := setupCloseoutFixture(t, m)
		recPath := f.seedStackedChild(t, "implemented", "feat/parent")
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(factsFor(strings.Repeat("a", 40))), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result != ResultExternalFailed || res.Disposition != CloseoutDispUnknown {
			t.Fatalf("missing parent branch = %q disp %q reason %q, want external-failed/%s", res.Result, res.Disposition, res.Reason, CloseoutDispUnknown)
		}
		rec, ok := originFile(t, f.repo.origin, f.branch, recPath)
		if !ok || strings.Contains(rec, "stacked-merged") {
			t.Errorf("an unknown-outcome closeout must leave the child untouched:\n%s", rec)
		}
	})
}
