package app

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/workspace"
)

// Change 0449 Task 9: a named operation on change B probes live branch facts
// only for B's own base/stack set (stackBranchesFor), never for every stack
// branch in the corpus. An unrelated stacked change A whose recorded parent
// branch cannot be probed must not block B; a broken branch B actually
// requires (its own stack ancestor's) still refuses.

// poisonProbe is the sentinel a poisoned branch-facts probe fails with.
const poisonProbe = "poisoned branch probe"

// poisonBranchReader wraps a StatusReader and fails BranchFacts — as a typed
// external failure, exactly as the git reader reports an unanswerable probe —
// whenever the requested set names a poisoned branch. Every ask is recorded.
type poisonBranchReader struct {
	StatusReader
	poison map[string]bool
	asks   [][]string
}

func (p *poisonBranchReader) BranchFacts(ctx context.Context, pin StatusPin, branches []string) (domain.BranchFacts, error) {
	p.asks = append(p.asks, append([]string(nil), branches...))
	for _, b := range branches {
		if p.poison[b] {
			return domain.BranchFacts{}, fmt.Errorf("%w: %s %q", ErrStatusExternal, poisonProbe, b)
		}
	}
	return p.StatusReader.BranchFacts(ctx, pin, branches)
}

func poisoned(inner StatusReader, branches ...string) *poisonBranchReader {
	set := make(map[string]bool, len(branches))
	for _, b := range branches {
		set[b] = true
	}
	return &poisonBranchReader{StatusReader: inner, poison: set}
}

// stackFixtureBlob renders a change record for the stackBranchesFor fixture.
func stackFixtureBlob(id int, slug, status, branch, extra string) StatusBlob {
	branchField := ""
	if branch != "" {
		branchField = "branch: '" + branch + "'\nclaimed_at: '2026-01-03T00:00:00Z'\nreconciled: true\n"
	}
	fm := fmt.Sprintf("---\nid: %d\nslug: %s\ntitle: Change %d\nstatus: '%s'\npriority: medium\ntype: feat\ncreated: 2026-01-02\ntrivial: true\n%s%s---\n\nBody of %d.\n",
		id, slug, id, status, branchField, extra, id)
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     fmt.Sprintf("docs/changes/active/%04d-%s.md", id, slug),
		Version:  fmt.Sprintf("blobchange%04d", id),
		Data:     []byte(fm),
	}
}

func snapshotOf(t *testing.T, blobs []StatusBlob) domain.Snapshot {
	t.Helper()
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: mainPin(t).Config.Effective, Documents: inputs})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	return build.Snapshot
}

func mustChange(t *testing.T, snap domain.Snapshot, id int) domain.Change {
	t.Helper()
	c, out := snap.Change(domain.ChangeID(id))
	if out != domain.LookupFound {
		t.Fatalf("change %d lookup = %v", id, out)
	}
	return c
}

// TestStackBranchesForBoundsToOwnBaseAndStack: B (stacked on a two-deep chain,
// depending on a stacked D) probes exactly its stack ancestors' branches — the
// set ResolveEffectiveBase(snap, B, …) consults. Never its own branch, never a
// dependency's stack (EvaluateDependencies reads no branch facts), and never
// the branch of an unrelated stacked A.
func TestStackBranchesForBoundsToOwnBaseAndStack(t *testing.T) {
	snap := snapshotOf(t, []StatusBlob{
		stackFixtureBlob(10, "a-parent", "in-progress", "feat/a-parent", ""),
		stackFixtureBlob(11, "a-child", "proposed", "", "stacked_on: 10\n"),
		stackFixtureBlob(20, "p-one", "in-progress", "feat/p-one", ""),
		stackFixtureBlob(21, "p-two", "in-progress", "feat/p-two", "stacked_on: 20\n"),
		stackFixtureBlob(30, "d-parent", "in-progress", "feat/d-parent", ""),
		stackFixtureBlob(31, "d", "in-progress", "feat/d", "stacked_on: 30\n"),
		stackFixtureBlob(40, "b", "in-progress", "feat/b", "stacked_on: 21\ndepends_on: [31]\n"),
	})
	got := stackBranchesFor(snap, mustChange(t, snap, 40))
	want := []string{"feat/p-one", "feat/p-two"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stackBranchesFor(B) = %v, want %v", got, want)
	}
	// The whole-corpus set still names A's parent: the bound is real.
	if all := stackBranches(snap); !contains(all, "feat/a-parent") {
		t.Fatalf("whole-corpus stackBranches = %v; fixture must include the unrelated A parent", all)
	}
}

// --- implementation context (fake reader) ---------------------------------

func TestContextImplementationNamedIDProbesOnlyOwnStack(t *testing.T) {
	pin := docketPin(t)
	unrelated := []StatusBlob{
		stackFixtureBlob(10, "a-parent", "in-progress", "feat/a-parent", ""),
		stackFixtureBlob(11, "a-child", "proposed", "", "stacked_on: 10\n"),
	}

	t.Run("unrelated-poison-does-not-block", func(t *testing.T) {
		corpus := append(append([]StatusBlob(nil), unrelated...), stackFixtureBlob(30, "b", "proposed", "", ""))
		reader := poisoned(&fakeReader{pin: pin, corpus: corpus, facts: domain.NewBranchFacts(nil)}, "feat/a-parent")
		got := ContextImplementation(context.Background(), PlanningDeps{Reader: reader, Clock: testClock()}, "", ImplementationContextRequest{ID: 30})
		if got.Result != ResultApplied {
			t.Fatalf("named context beside an unprobeable unrelated stack = %q (%s: %s), want applied", got.Result, got.Reason, got.Message)
		}
	})

	t.Run("own-ancestor-poison-refuses", func(t *testing.T) {
		corpus := append(append([]StatusBlob(nil), unrelated...),
			stackFixtureBlob(29, "b-parent", "in-progress", "feat/b-parent", ""),
			stackFixtureBlob(30, "b", "proposed", "", "stacked_on: 29\n"))
		reader := poisoned(&fakeReader{pin: pin, corpus: corpus, facts: domain.NewBranchFacts(nil)}, "feat/b-parent")
		got := ContextImplementation(context.Background(), PlanningDeps{Reader: reader, Clock: testClock()}, "", ImplementationContextRequest{ID: 30})
		if got.Result != ResultExternalFailed || !strings.Contains(got.Message, poisonProbe) {
			t.Fatalf("named context with B's own parent unprobeable = %q (%s: %s), want the external probe failure", got.Result, got.Reason, got.Message)
		}
	})

	// An explicit id naming no single record probes nothing, so an unrelated
	// unprobeable branch never replaces the typed unknown/ambiguous refusal
	// with an external failure.
	t.Run("absent-id-keeps-typed-refusal", func(t *testing.T) {
		corpus := append(append([]StatusBlob(nil), unrelated...), stackFixtureBlob(30, "b", "proposed", "", ""))
		reader := poisoned(&fakeReader{pin: pin, corpus: corpus, facts: domain.NewBranchFacts(nil)}, "feat/a-parent")
		got := ContextImplementation(context.Background(), PlanningDeps{Reader: reader, Clock: testClock()}, "", ImplementationContextRequest{ID: 77})
		if got.Result != ResultInvalidInput || got.Reason != ReasonContextUnknownChange {
			t.Fatalf("absent id beside an unprobeable unrelated stack = %q (%s: %s), want invalid-input %s", got.Result, got.Reason, got.Message, ReasonContextUnknownChange)
		}
	})
	t.Run("ambiguous-id-keeps-typed-refusal", func(t *testing.T) {
		dupe := stackFixtureBlob(30, "b-dupe", "proposed", "", "")
		corpus := append(append([]StatusBlob(nil), unrelated...), stackFixtureBlob(30, "b", "proposed", "", ""), dupe)
		reader := poisoned(&fakeReader{pin: pin, corpus: corpus, facts: domain.NewBranchFacts(nil)}, "feat/a-parent")
		got := ContextImplementation(context.Background(), PlanningDeps{Reader: reader, Clock: testClock()}, "", ImplementationContextRequest{ID: 30})
		if got.Result != ResultInvalidState || got.Reason != ReasonContextAmbiguousID {
			t.Fatalf("ambiguous id beside an unprobeable unrelated stack = %q (%s: %s), want invalid-state %s", got.Result, got.Reason, got.Message, ReasonContextAmbiguousID)
		}
	})

	t.Run("selection-path-keeps-whole-corpus-probe", func(t *testing.T) {
		corpus := append(append([]StatusBlob(nil), unrelated...), stackFixtureBlob(30, "b", "proposed", "", ""))
		reader := poisoned(&fakeReader{pin: pin, corpus: corpus, facts: domain.NewBranchFacts(nil)})
		ContextImplementation(context.Background(), PlanningDeps{Reader: reader, Clock: testClock()}, "", ImplementationContextRequest{})
		if len(reader.asks) != 1 || !contains(reader.asks[0], "feat/a-parent") {
			t.Fatalf("selection asks = %v, want one whole-corpus probe naming feat/a-parent", reader.asks)
		}
	})
}

// --- retarget (fake reader + fake GitHub) --------------------------------

func TestRetargetProbesOnlyParentStack(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{
		finalizeBlob(80, "root", "implemented", "high", prRefFor(800), ""),
		finalizeBlob(81, "child-a", "implemented", "high", prRefFor(810), "stacked_on: 80\n"),
		finalizeBlob(90, "a-parent", "in-progress", "high", prRefFor(900), ""),
		finalizeBlob(91, "a-child", "in-progress", "high", prRefFor(910), "stacked_on: 90\n"),
	}
	gh := &fakeRetargetGitHub{
		repo: retargetRepo(),
		prs:  []*fakePR{{number: 810, head: "feat/child-a", base: "feat/root", version: "cv810"}},
	}
	deps := retargetDeps(&fakeReader{pin: pin, corpus: corpus}, gh, &recordingEngine{})
	deps.Planning.Reader = poisoned(deps.Planning.Reader, "feat/a-parent")
	req := RetargetChildrenRequest{
		ID: 80, Version: "blobfin0080",
		Children: []AuthorizedChild{{ID: 81, PRNumber: 810, PRVersion: "cv810"}},
	}
	got := FinalizeRetargetChildren(context.Background(), deps, "", req)
	if got.Result != ResultApplied {
		t.Fatalf("retarget beside an unprobeable unrelated stack = %q (%s: %s), want applied", got.Result, got.Reason, got.Message)
	}
}

// --- repair workspace ownership gate (fake reader + fake workspace) ------

func TestRepairWorkspaceClearProbesOnlyOwnStack(t *testing.T) {
	pin := mainPin(t)
	unrelated := []StatusBlob{
		stackFixtureBlob(10, "a-parent", "in-progress", "feat/a-parent", ""),
		stackFixtureBlob(11, "a-child", "proposed", "", "stacked_on: 10\n"),
	}
	run := func(t *testing.T, corpus []StatusBlob, poison string) *RepairIdentityResult {
		t.Helper()
		snap := snapshotOf(t, corpus)
		deps := FinalizeDeps{
			Planning:  PlanningDeps{Reader: poisoned(&fakeReader{pin: pin, corpus: corpus, facts: domain.NewBranchFacts(map[string]bool{"feat/b-parent": true})}, poison), Clock: testClock()},
			Workspace: &fakeRepairWorkspace{inspection: workspace.Inspection{Kind: workspace.StateAbsent}},
		}
		return repairProveWorkspaceClear(context.Background(), deps, pin, snap, gitcli.Repository{}, mustChange(t, snap, 30), "feat/b", 30)
	}

	t.Run("unrelated-poison-does-not-block", func(t *testing.T) {
		corpus := append(append([]StatusBlob(nil), unrelated...), stackFixtureBlob(30, "b", "in-progress", "feat/b", ""))
		if r := run(t, corpus, "feat/a-parent"); r != nil {
			t.Fatalf("repair workspace gate beside an unprobeable unrelated stack refused: %s: %s", r.Reason, r.Message)
		}
	})
	t.Run("own-ancestor-poison-refuses", func(t *testing.T) {
		corpus := append(append([]StatusBlob(nil), unrelated...),
			stackFixtureBlob(29, "b-parent", "in-progress", "feat/b-parent", ""),
			stackFixtureBlob(30, "b", "in-progress", "feat/b", "stacked_on: 29\n"))
		if r := run(t, corpus, "feat/b-parent"); r == nil {
			t.Fatal("repair workspace gate with B's own parent unprobeable passed; want the fail-closed conflict")
		}
	})
}

// --- real-git callers: claim, workspace context, merge context, clear-block --

// namedFactsRepo seeds B (from bSrc, plus an optional in-progress parent 4 on
// feat/b-parent) beside an unrelated stacked pair A (20 in-progress on
// feat/a-parent, 21 stacked on it).
func namedFactsRepo(t *testing.T, bSrc string, withParent bool) *gitRepo {
	t.Helper()
	files := map[string]string{
		groomPath(3, "widget"):    bSrc,
		groomPath(20, "a-parent"): lifecycleChange(20, "a-parent", "in-progress"),
		groomPath(21, "a-child"):  stackedOn(lifecycleChange(21, "a-child", "proposed"), 20),
	}
	if withParent {
		files[groomPath(4, "b-parent")] = lifecycleChange(4, "b-parent", "in-progress")
		files[groomPath(3, "widget")] = stackedOn(bSrc, 4)
	}
	return newWorkingRepo(t, files)
}

func assertPoisonRefusal(t *testing.T, what string, result Result, reason, msg string) {
	t.Helper()
	if result != ResultExternalFailed || reason != ReasonStatusExternal || !strings.Contains(msg, poisonProbe) {
		t.Fatalf("%s with B's own parent unprobeable = %q (%s: %s), want the external probe failure", what, result, reason, msg)
	}
}

func TestChangeClaimProbesOnlyOwnStack(t *testing.T) {
	requireRealGit(t)
	t.Run("unrelated-poison-does-not-block", func(t *testing.T) {
		repo := namedFactsRepo(t, claimableChange(3, "widget"), false)
		node := planningDepsFor(t, repo.invocation)
		node.deps.Reader = poisoned(node.deps.Reader, "feat/a-parent")
		res := ChangeClaim(context.Background(), node.deps, node.dir,
			ChangeClaimRequest{ID: 3, Version: blobVersionAt(t, repo.origin, "docket", groomPath(3, "widget"))})
		if res.Result != ResultApplied {
			t.Fatalf("claim beside an unprobeable unrelated stack = %q (disposition %q findings %v), want applied", res.Result, res.Disposition, res.Findings)
		}
	})
	t.Run("own-ancestor-poison-refuses", func(t *testing.T) {
		repo := namedFactsRepo(t, claimableChange(3, "widget"), true)
		node := planningDepsFor(t, repo.invocation)
		node.deps.Reader = poisoned(node.deps.Reader, "feat/b-parent")
		res := ChangeClaim(context.Background(), node.deps, node.dir,
			ChangeClaimRequest{ID: 3, Version: blobVersionAt(t, repo.origin, "docket", groomPath(3, "widget"))})
		msg := ""
		for _, f := range res.Findings {
			msg += f.Message
		}
		if res.Result != ResultExternalFailed || !strings.Contains(msg, poisonProbe) {
			t.Fatalf("claim with B's own parent unprobeable = %q (findings %v), want the external probe failure", res.Result, res.Findings)
		}
	})
}

func TestWorkspaceContextProbesOnlyOwnStack(t *testing.T) {
	requireRealGit(t)
	t.Run("unrelated-poison-does-not-block", func(t *testing.T) {
		repo := namedFactsRepo(t, lifecycleChange(3, "widget", "in-progress"), false)
		node := planningDepsFor(t, repo.invocation)
		node.deps.Reader = poisoned(node.deps.Reader, "feat/a-parent")
		if _, r := loadWorkspaceContext(context.Background(), node.deps, node.dir, 3, OperationWorkspaceInspect); r != nil {
			t.Fatalf("workspace context beside an unprobeable unrelated stack refused: %s: %s", r.Reason, r.Message)
		}
	})
	t.Run("own-ancestor-poison-refuses", func(t *testing.T) {
		repo := namedFactsRepo(t, lifecycleChange(3, "widget", "in-progress"), true)
		node := planningDepsFor(t, repo.invocation)
		node.deps.Reader = poisoned(node.deps.Reader, "feat/b-parent")
		_, r := loadWorkspaceContext(context.Background(), node.deps, node.dir, 3, OperationWorkspaceInspect)
		if r == nil {
			t.Fatal("workspace context with B's own parent unprobeable resolved; want the external probe failure")
		}
		assertPoisonRefusal(t, "workspace context", r.Result, r.Reason, r.Message)
	})
}

func TestMergeContextProbesOnlyOwnStack(t *testing.T) {
	requireRealGit(t)
	t.Run("unrelated-poison-does-not-block", func(t *testing.T) {
		repo := namedFactsRepo(t, lifecycleChange(3, "widget", "in-progress"), false)
		node := planningDepsFor(t, repo.invocation)
		node.deps.Reader = poisoned(node.deps.Reader, "feat/a-parent")
		if _, r := loadMergeContext(context.Background(), FinalizeDeps{Planning: node.deps}, node.dir, 3); r != nil {
			t.Fatalf("merge context beside an unprobeable unrelated stack refused: %s: %s", r.Reason, r.Message)
		}
	})
	t.Run("own-ancestor-poison-refuses", func(t *testing.T) {
		repo := namedFactsRepo(t, lifecycleChange(3, "widget", "in-progress"), true)
		node := planningDepsFor(t, repo.invocation)
		node.deps.Reader = poisoned(node.deps.Reader, "feat/b-parent")
		_, r := loadMergeContext(context.Background(), FinalizeDeps{Planning: node.deps}, node.dir, 3)
		if r == nil {
			t.Fatal("merge context with B's own parent unprobeable resolved; want the external probe failure")
		}
		assertPoisonRefusal(t, "merge context", r.Result, r.Reason, r.Message)
	})
}

func TestFinalizeClearBlockProbesOnlyOwnStack(t *testing.T) {
	requireRealGit(t)
	probeErr := errors.New("workspace probe reached")
	run := func(t *testing.T, withParent bool, poison string) BlockResult {
		t.Helper()
		repo := namedFactsRepo(t, lifecycleChange(3, "widget", "in-progress"), withParent)
		node := planningDepsFor(t, repo.invocation)
		node.deps.Reader = poisoned(node.deps.Reader, poison)
		deps := FinalizeDeps{
			Planning:  node.deps,
			GitHub:    repairGitHub("feat/widget"),
			Workspace: &fakeRepairWorkspace{inspectErr: probeErr},
		}
		return FinalizeClearBlock(context.Background(), deps, node.dir, ClearBlockRequest{
			ID: 3, Version: blobVersionAt(t, repo.origin, "docket", groomPath(3, "widget")), Head: prHead, PRNumber: 7,
		})
	}
	t.Run("unrelated-poison-does-not-block", func(t *testing.T) {
		// The reprobe proceeds past branch facts to the (scripted-failing)
		// workspace probe: the unrelated stack's branch was never asked for.
		if r := run(t, false, "feat/a-parent"); r.Reason != ReasonBlockWorkspaceProbe {
			t.Fatalf("clear-block beside an unprobeable unrelated stack = %q (%s: %s), want it to reach the workspace probe", r.Result, r.Reason, r.Message)
		}
	})
	t.Run("own-ancestor-poison-refuses", func(t *testing.T) {
		r := run(t, true, "feat/b-parent")
		assertPoisonRefusal(t, "clear-block", r.Result, r.Reason, r.Message)
	})
}
