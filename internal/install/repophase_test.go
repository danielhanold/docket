package install

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/testsupport"
)

// These tests drive applyPlan directly with a repository phase — the one seam
// this task grows. They live in package install (not install_test) because
// applyPlan is unexported: the app layer that assembles a real RepoPhase arrives
// in a later task, so the phase is hand-built here from ordinary install types.

// repoWorld is a bare installation universe plus a place to hang machine targets
// and one or more working trees.
type repoWorld struct {
	roots UserRoots
	base  string
}

func newRepoWorld(t *testing.T) repoWorld {
	t.Helper()
	base := testsupport.TempDir(t)
	home := filepath.Join(base, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", home, err)
	}
	roots, err := ResolveRoots(
		func() (string, error) { return home, nil },
		func(string) string { return "" },
	)
	if err != nil {
		t.Fatalf("ResolveRoots: %v", err)
	}
	return repoWorld{roots: roots, base: base}
}

func (w repoWorld) options() Options { return Options{Roots: w.roots, FS: RealFS{}} }

func (w repoWorld) applyOut() Outcome { return Outcome{StatePath: w.roots.StatePath()} }

// machinePlan is one plain create the machine half of the transaction carries,
// so a repository conflict has real machine work to block.
func (w repoWorld) machinePlan(path string) (plannedInstallation, Target) {
	target := Target{Path: path, Kind: KindFile, Content: []byte("machine\n"), Role: "agent"}
	return plannedInstallation{
		mode:          ModeRelease,
		harnesses:     []string{"toy"},
		targets:       []Target{target},
		owner:         map[string]string{filepath.Clean(path): "toy"},
		assetSetID:    "sha256:x",
		assetProtocol: 1,
	}, target
}

func actionFor(out Outcome, op, path string) (Action, bool) {
	for _, a := range out.Actions {
		if a.Op == op && a.Path == path {
			return a, true
		}
	}
	return Action{}, false
}

func assertAbsent(t *testing.T, path, why string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s: %s exists (err=%v)", why, path, err)
	}
}

// TestApplyPlanRepoConflictBlocksMachineWork is the all-or-nothing boundary
// across the machine/repository seam: one unowned repository surface refuses the
// entire operation before the first destination — machine or repository — is
// touched.
//
// MUTATION TEST (performed by hand, per the spec): commenting out the repository
// inspection block in applyPlan (so repo surfaces are never inspected and the
// conflict is never collected) lets the machine target apply on its own — a
// partial machine change — and this test reddens on the "machine file was
// created" assertion. Restored, it is green again.
func TestApplyPlanRepoConflictBlocksMachineWork(t *testing.T) {
	w := newRepoWorld(t)
	machineFile := filepath.Join(w.base, "machine", "agent.md")
	p, _ := w.machinePlan(machineFile)

	// A repository CLAUDE.md carrying a well-formed but UNOWNED dispatch block:
	// balanced markers (so it parses), an interior that is neither a prior record
	// nor a legacy reproduction, and no prior repository record — an ownership
	// conflict.
	wt := filepath.Join(w.base, "wt")
	claudeMD := filepath.Join(wt, "CLAUDE.md")
	foreign := "# my rules\n\n" +
		"<!-- docket:dispatch:start (managed by docket) -->\n" +
		"I hand-wrote this dispatch block; it is not docket's.\n" +
		"<!-- docket:dispatch:end -->\n\nmore of my own notes\n"
	writeFileOrDie(t, claudeMD, foreign)

	repo := &RepoPhase{
		Authorized:  true,
		Targets:     []Target{{Path: claudeMD, Kind: KindManagedBlock, BlockName: "dispatch", Annotation: "managed by docket", Content: []byte("dispatch interior\n"), Role: "dispatch"}},
		Owners:      map[string][]string{filepath.Clean(claudeMD): {"claude"}},
		RecordPath:  filepath.Join(w.base, "gitdir", "docket", "install.json"),
		RecordBytes: []byte("{}\n"),
		Worktree:    wt,
	}

	out := applyPlan(w.options(), p, repo, w.applyOut())

	if out.Reason != ReasonOwnershipConflict {
		t.Fatalf("reason = %q, want %q (err %v)", out.Reason, ReasonOwnershipConflict, out.Err)
	}
	if out.Applied {
		t.Fatalf("a repository conflict reported applied work")
	}
	if _, ok := actionFor(out, OpConflict, claudeMD); !ok {
		t.Fatalf("the repository conflict was not reported: %v", out.Actions)
	}
	// The machine target must NOT have been created: a repository conflict blocks
	// binary/wrapper/skill/global work alike.
	assertAbsent(t, machineFile, "repo conflict must block machine work")
	// The repository file is byte-identical; neither ownership record was published.
	if got := readOrDie(t, claudeMD); got != foreign {
		t.Errorf("the repository file was disturbed:\n%q", got)
	}
	assertAbsent(t, w.roots.StatePath(), "machine state published despite a repo conflict")
	assertAbsent(t, repo.RecordPath, "repo record published despite a conflict")
}

func TestApplyPlanCommitsBothDocuments(t *testing.T) {
	w := newRepoWorld(t)
	machineFile := filepath.Join(w.base, "machine", "agent.md")
	p, _ := w.machinePlan(machineFile)

	wt := filepath.Join(w.base, "wt")
	claudeMD := filepath.Join(wt, "CLAUDE.md")
	recordPath := filepath.Join(w.base, "gitdir", "docket", "install.json")
	recordBytes := []byte("repo-record-v1\n")

	repo := &RepoPhase{
		Authorized:  true,
		Targets:     []Target{{Path: claudeMD, Kind: KindManagedBlock, BlockName: "dispatch", Annotation: "managed by docket", Content: []byte("dispatch interior\n"), Role: "dispatch"}},
		Owners:      map[string][]string{filepath.Clean(claudeMD): {"claude"}},
		RecordPath:  recordPath,
		RecordBytes: recordBytes,
		Worktree:    wt,
	}

	out := applyPlan(w.options(), p, repo, w.applyOut())
	if out.Err != nil {
		t.Fatalf("applyPlan: %v (reason %q)", out.Err, out.Reason)
	}
	if !out.Applied {
		t.Fatalf("an authorized reconciliation reported no work")
	}

	// Machine target and repository surface both landed.
	if got := readOrDie(t, machineFile); got != "machine\n" {
		t.Errorf("machine target = %q", got)
	}
	if got := readOrDie(t, claudeMD); !strings.Contains(got, "docket:dispatch:start") {
		t.Errorf("repository dispatch block not written:\n%s", got)
	}
	// BOTH ownership documents were published together, and there is no
	// not-authorized no-op action on an authorized run.
	if LoadStateOrDie(t, w.roots) == nil {
		t.Fatalf("machine state not published")
	}
	if got := readOrDie(t, recordPath); got != string(recordBytes) {
		t.Errorf("repository record = %q, want %q", got, string(recordBytes))
	}
	for _, a := range out.Actions {
		if isRepoNotAuthorizedAction(a) {
			t.Errorf("an authorized run named a repository no-op: %+v", a)
		}
	}
	// The repository surface is reported with its harness owner.
	if a, ok := actionFor(out, OpCreate, claudeMD); !ok || a.Detail != "claude" {
		t.Errorf("repository surface action = %+v (ok=%v)", a, ok)
	}
}

func TestApplyPlanUnauthorizedKeepsPriorRecord(t *testing.T) {
	t.Run("unauthorized phase keeps the prior record and names the worktree", func(t *testing.T) {
		w := newRepoWorld(t)
		machineFile := filepath.Join(w.base, "machine", "agent.md")
		p, _ := w.machinePlan(machineFile)

		wt := filepath.Join(w.base, "wt")
		claudeMD := filepath.Join(wt, "CLAUDE.md")
		surface := "# untouched user surface\n"
		writeFileOrDie(t, claudeMD, surface)
		recordPath := filepath.Join(w.base, "gitdir", "docket", "install.json")
		priorRecord := "prior repository record bytes\n"
		writeFileOrDie(t, recordPath, priorRecord)

		// A non-nil phase that is NOT authorized (an absent agent_harnesses key):
		// no surface is inspected or touched, and the prior record is left exactly
		// as it was.
		repo := &RepoPhase{Authorized: false, Worktree: wt, RecordPath: recordPath}
		out := applyPlan(w.options(), p, repo, w.applyOut())
		if out.Err != nil {
			t.Fatalf("applyPlan: %v (reason %q)", out.Err, out.Reason)
		}
		if got := readOrDie(t, machineFile); got != "machine\n" {
			t.Errorf("machine work did not proceed: %q", got)
		}
		if got := readOrDie(t, claudeMD); got != surface {
			t.Errorf("an unauthorized run touched a repository surface:\n%q", got)
		}
		if got := readOrDie(t, recordPath); got != priorRecord {
			t.Errorf("an unauthorized run rewrote the prior record:\n%q", got)
		}
		a, ok := actionFor(out, OpKeep, wt)
		if !ok || !strings.Contains(a.Detail, "not authorized") {
			t.Fatalf("the repository no-op was not named at the worktree: %v", out.Actions)
		}
	})

	t.Run("nil phase names none", func(t *testing.T) {
		w := newRepoWorld(t)
		machineFile := filepath.Join(w.base, "machine", "agent.md")
		p, _ := w.machinePlan(machineFile)

		out := applyPlan(w.options(), p, nil, w.applyOut())
		if out.Err != nil {
			t.Fatalf("applyPlan: %v (reason %q)", out.Err, out.Reason)
		}
		if _, ok := actionFor(out, OpKeep, "(none)"); !ok {
			t.Fatalf("a machine-only run did not name the repository no-op as (none): %v", out.Actions)
		}
	})
}

func TestApplyPlanEmptyListRetiresAndPublishesEmptyRecord(t *testing.T) {
	w := newRepoWorld(t)
	// No machine work: this run exists to retire a repository surface and empty
	// the repository record.
	p := plannedInstallation{mode: ModeRelease, harnesses: []string{"claude"}, assetSetID: "sha256:x", assetProtocol: 1}

	wt := filepath.Join(w.base, "wt")
	claudeMD := filepath.Join(wt, "CLAUDE.md")
	writeFileOrDie(t, claudeMD, managedFile("dispatch interior\n"))
	proseOnly := managedFile("dispatch interior\n")
	// Strip exactly the block's marker-to-marker span the fixture lays down.
	block := "<!-- docket:dispatch:start (managed by docket) -->\ndispatch interior\n<!-- docket:dispatch:end -->\n"
	proseOnly = strings.Replace(proseOnly, block, "", 1)

	recordPath := filepath.Join(w.base, "gitdir", "docket", "install.json")
	emptyRecord := []byte("{\"format_version\":1,\"surfaces\":[]}\n")

	repo := &RepoPhase{
		Authorized:  true,
		Targets:     nil, // explicit empty list: retire, do not reconcile
		Removals:    []TargetRecord{{Path: claudeMD, Kind: KindManagedBlock, BlockName: "dispatch", Role: "dispatch", Harness: "claude"}},
		RecordPath:  recordPath,
		RecordBytes: emptyRecord,
		Worktree:    wt,
	}

	out := applyPlan(w.options(), p, repo, w.applyOut())
	if out.Err != nil {
		t.Fatalf("applyPlan: %v (reason %q)", out.Err, out.Reason)
	}
	if !out.Applied {
		t.Fatalf("a retire-everything run reported no work")
	}
	// The block was retired and the surrounding prose preserved; the file remains.
	if got := readOrDie(t, claudeMD); got != proseOnly {
		t.Errorf("after retirement =\n%q\nwant\n%q", got, proseOnly)
	}
	if _, present := actionFor(out, OpRemove, claudeMD); !present {
		t.Errorf("the retirement was not reported as a removal: %v", out.Actions)
	}
	// The record was published empty, not deleted.
	if got := readOrDie(t, recordPath); got != string(emptyRecord) {
		t.Errorf("repository record = %q, want the empty record %q", got, string(emptyRecord))
	}
}

func TestApplyPlanEditedOwnedSurfaceBlocksEverything(t *testing.T) {
	w := newRepoWorld(t)
	machineFile := filepath.Join(w.base, "machine", "agent.md")
	p, _ := w.machinePlan(machineFile)

	wt := filepath.Join(w.base, "wt")
	claudeMD := filepath.Join(wt, "CLAUDE.md")
	// The surface carries a docket block, but its interior has been edited away
	// from what the prior record proves — so it is drifted, not docket's to
	// rewrite.
	writeFileOrDie(t, claudeMD, managedFile("the user edited this interior\n"))
	prior := &State{FormatVersion: StateFormatVersion, Targets: []TargetRecord{{
		Path: claudeMD, Kind: KindManagedBlock, BlockName: "dispatch", SHA256: "sha256:a-different-interior",
	}}}
	before := readOrDie(t, claudeMD)

	repo := &RepoPhase{
		Authorized: true,
		Targets:    []Target{{Path: claudeMD, Kind: KindManagedBlock, BlockName: "dispatch", Annotation: "managed by docket", Content: []byte("docket interior\n"), Role: "dispatch"}},
		Owners:     map[string][]string{filepath.Clean(claudeMD): {"claude"}},
		PriorState: prior,
		RecordPath: filepath.Join(w.base, "gitdir", "docket", "install.json"),
		Worktree:   wt,
	}

	out := applyPlan(w.options(), p, repo, w.applyOut())
	if out.Reason != ReasonOwnershipConflict {
		t.Fatalf("reason = %q, want %q (err %v)", out.Reason, ReasonOwnershipConflict, out.Err)
	}
	if _, okc := actionFor(out, OpConflict, claudeMD); !okc {
		t.Fatalf("the drifted surface conflict was not reported: %v", out.Actions)
	}
	// Everything is blocked: the machine target is not created and the surface is
	// byte-identical.
	assertAbsent(t, machineFile, "an edited owned surface must block machine work")
	if got := readOrDie(t, claudeMD); got != before {
		t.Errorf("the drifted surface was disturbed:\n%q", got)
	}
	assertAbsent(t, w.roots.StatePath(), "machine state published despite a repo conflict")
}

// TestApplyPlanTwoWorktreesIsolation runs against one working tree and proves the
// other's surface and record are byte-identical afterward: two working trees keep
// separate record paths, so a run against one can never reach the other.
func TestApplyPlanTwoWorktreesIsolation(t *testing.T) {
	w := newRepoWorld(t)
	p := plannedInstallation{mode: ModeRelease, harnesses: []string{"claude"}, assetSetID: "sha256:x", assetProtocol: 1}

	// Working tree B, untouched by this run: an existing surface and an existing
	// record.
	wtB := filepath.Join(w.base, "wtB")
	surfaceB := filepath.Join(wtB, "CLAUDE.md")
	recordB := filepath.Join(w.base, "gitB", "docket", "install.json")
	writeFileOrDie(t, surfaceB, "B's own surface\n")
	writeFileOrDie(t, recordB, "B's own record\n")
	beforeSurfaceB := readOrDie(t, surfaceB)
	beforeRecordB := readOrDie(t, recordB)

	// Working tree A: the one this run reconciles.
	wtA := filepath.Join(w.base, "wtA")
	surfaceA := filepath.Join(wtA, "CLAUDE.md")
	recordA := filepath.Join(w.base, "gitA", "docket", "install.json")
	repoA := &RepoPhase{
		Authorized:  true,
		Targets:     []Target{{Path: surfaceA, Kind: KindManagedBlock, BlockName: "dispatch", Annotation: "managed by docket", Content: []byte("A interior\n"), Role: "dispatch"}},
		Owners:      map[string][]string{filepath.Clean(surfaceA): {"claude"}},
		RecordPath:  recordA,
		RecordBytes: []byte("A's record\n"),
		Worktree:    wtA,
	}

	out := applyPlan(w.options(), p, repoA, w.applyOut())
	if out.Err != nil {
		t.Fatalf("applyPlan: %v (reason %q)", out.Err, out.Reason)
	}
	// A was reconciled...
	if got := readOrDie(t, surfaceA); !strings.Contains(got, "docket:dispatch:start") {
		t.Errorf("A's surface was not reconciled:\n%s", got)
	}
	if got := readOrDie(t, recordA); got != "A's record\n" {
		t.Errorf("A's record = %q", got)
	}
	// ...and B is byte-identical, surface and record alike.
	if got := readOrDie(t, surfaceB); got != beforeSurfaceB {
		t.Errorf("B's surface changed under a run against A:\n%q", got)
	}
	if got := readOrDie(t, recordB); got != beforeRecordB {
		t.Errorf("B's record changed under a run against A:\n%q", got)
	}
}

// TestSelectPlannersUnionWithOptIns pins the machine-selection contract change
// 0351 introduced: without an explicit --harness scope, the default set is
// detection ∪ the repository's opt-ins, so a harness a repository newly opted
// into is installed even when it is not otherwise present; an explicit scope
// stays authoritative and consults neither detection nor the opt-ins.
//
// MUTATION TEST (by hand): deleting the opt-in union loop in selectPlanners (so
// optIns never join `chosen`) reddens the "detection ∪ opt-ins" and
// "opt-in only" sub-cases below — they would then resolve to detection alone.
func TestSelectPlannersUnionWithOptIns(t *testing.T) {
	detected := "a"
	planners := []Planner{
		selectTestPlanner("a", true),
		selectTestPlanner("b", false),
		selectTestPlanner("c", false),
	}
	names := func(ps []Planner) []string {
		var out []string
		for _, p := range ps {
			out = append(out, p.Name)
		}
		sort.Strings(out)
		return out
	}

	// Explicit scope is authoritative: opt-ins are ignored.
	sel, err := selectPlanners(planners, []string{"b"}, []string{"c"}, UserRoots{})
	if err != nil || strings.Join(names(sel), ",") != "b" {
		t.Fatalf("explicit scope = %v (%v), want [b]", names(sel), err)
	}

	// No scope: detection ∪ opt-ins.
	sel, err = selectPlanners(planners, nil, []string{"c"}, UserRoots{})
	if err != nil || strings.Join(names(sel), ",") != "a,c" {
		t.Fatalf("union = %v (%v), want [a c]", names(sel), err)
	}

	// No scope, no opt-ins: detection alone.
	sel, err = selectPlanners(planners, nil, nil, UserRoots{})
	if err != nil || strings.Join(names(sel), ",") != detected {
		t.Fatalf("detection = %v (%v), want [a]", names(sel), err)
	}

	// An opt-in naming no planner is a wiring bug, refused.
	if _, err := selectPlanners(planners, nil, []string{"zzz"}, UserRoots{}); err == nil {
		t.Fatalf("an unknown opt-in token was accepted")
	}
}

func selectTestPlanner(name string, present bool) Planner {
	return Planner{
		Name:   name,
		Detect: func(UserRoots) (bool, string) { return present, "" },
		Plan: func(Mode, string, assets.Catalog) ([]Target, error) {
			return []Target{{Path: "/" + name, Kind: KindFile, Content: []byte(name)}}, nil
		},
	}
}

// --- small local helpers -------------------------------------------------

func isRepoNotAuthorizedAction(a Action) bool {
	return a.Op == OpKeep && strings.Contains(a.Detail, "not authorized")
}

func LoadStateOrDie(t *testing.T, roots UserRoots) *State {
	t.Helper()
	s, err := LoadState(roots.StatePath())
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	return s
}
