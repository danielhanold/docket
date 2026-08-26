package reposeed

import (
	"bytes"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/cursor"
	"github.com/danielhanold/docket/internal/install"
)

// runGate is a stand-in run-gate payload. Plan is pure and never parses it, so
// any deterministic bytes exercise the wiring; the wording constraint asserts
// its exact propagation through harness.DispatchInterior below.
var runGate = []byte("## Run gate — bracket a dispatched run\n\nDrive the gate, never yield.\n")

const worktreeRoot = "/repo"

// byPath indexes a plan's targets by their (already cleaned) path so a case can
// assert one surface without depending on slice order.
func byPath(targets []install.Target) map[string]install.Target {
	m := make(map[string]install.Target, len(targets))
	for _, t := range targets {
		m[t.Path] = t
	}
	return m
}

func mustPlan(t *testing.T, in PlanInput) ([]install.Target, map[string][]string) {
	t.Helper()
	targets, owners, err := Plan(in)
	if err != nil {
		t.Fatalf("Plan(%v) errored: %v", in.Harnesses, err)
	}
	return targets, owners
}

func claudeMD() string { return filepath.Join(worktreeRoot, "CLAUDE.md") }
func agentsMD() string { return filepath.Join(worktreeRoot, "AGENTS.md") }
func cursorRule() string {
	return filepath.Join(worktreeRoot, ".cursor", "rules", "docket-dispatch.mdc")
}

func TestPlanClaudeAlone(t *testing.T) {
	targets, owners := mustPlan(t, PlanInput{
		WorktreeRoot: worktreeRoot,
		Harnesses:    []string{"claude"},
		RunGate:      runGate,
	})
	if len(targets) != 1 {
		t.Fatalf("claude alone planned %d targets, want 1: %+v", len(targets), targets)
	}
	tg := targets[0]
	if tg.Path != claudeMD() {
		t.Errorf("claude target path = %q, want %q", tg.Path, claudeMD())
	}
	if tg.Kind != install.KindManagedBlock {
		t.Errorf("claude kind = %q, want managed block", tg.Kind)
	}
	if tg.BlockName != "dispatch" {
		t.Errorf("claude block name = %q, want dispatch", tg.BlockName)
	}
	// claude alone never plans AGENTS.md.
	if _, ok := byPath(targets)[agentsMD()]; ok {
		t.Errorf("claude alone planned an AGENTS.md target")
	}
	if got := owners[claudeMD()]; !reflect.DeepEqual(got, []string{"claude"}) {
		t.Errorf("claude owners = %v, want [claude]", got)
	}
}

func TestPlanClaudeAloneAbsentStateStillBlock(t *testing.T) {
	// No shared AGENTS.md surface exists (claude only), so even an absent
	// CLAUDE.md is a managed block, never a symlink.
	targets, _ := mustPlan(t, PlanInput{
		WorktreeRoot:  worktreeRoot,
		Harnesses:     []string{"claude"},
		RunGate:       runGate,
		ClaudeMDState: ClaudeMDAbsent,
	})
	tg := byPath(targets)[claudeMD()]
	if tg.Kind != install.KindManagedBlock {
		t.Errorf("claude alone / absent kind = %q, want managed block (no shared AGENTS.md)", tg.Kind)
	}
}

func TestPlanCodexAlone(t *testing.T) {
	targets, owners := mustPlan(t, PlanInput{
		WorktreeRoot: worktreeRoot,
		Harnesses:    []string{"codex"},
		RunGate:      runGate,
	})
	if len(targets) != 1 {
		t.Fatalf("codex alone planned %d targets, want 1: %+v", len(targets), targets)
	}
	tg := targets[0]
	if tg.Path != agentsMD() || tg.Kind != install.KindManagedBlock || tg.BlockName != "dispatch" {
		t.Errorf("codex target = %+v, want AGENTS.md dispatch managed block", tg)
	}
	if got := owners[agentsMD()]; !reflect.DeepEqual(got, []string{"codex"}) {
		t.Errorf("codex owners = %v, want [codex]", got)
	}
}

func TestPlanOpencodeAlone(t *testing.T) {
	targets, owners := mustPlan(t, PlanInput{
		WorktreeRoot: worktreeRoot,
		Harnesses:    []string{"opencode"},
		RunGate:      runGate,
	})
	tg := byPath(targets)[agentsMD()]
	if tg.Kind != install.KindManagedBlock {
		t.Fatalf("opencode alone did not plan an AGENTS.md block: %+v", targets)
	}
	if got := owners[agentsMD()]; !reflect.DeepEqual(got, []string{"opencode"}) {
		t.Errorf("opencode owners = %v, want [opencode]", got)
	}
}

func TestPlanCursorAlone(t *testing.T) {
	targets, owners := mustPlan(t, PlanInput{
		WorktreeRoot: worktreeRoot,
		Harnesses:    []string{"cursor"},
		RunGate:      runGate,
	})
	if len(targets) != 1 {
		t.Fatalf("cursor alone planned %d targets, want 1: %+v", len(targets), targets)
	}
	tg := targets[0]
	if tg.Path != cursorRule() || tg.Kind != install.KindFile {
		t.Errorf("cursor target = %+v, want %q as a KindFile", tg, cursorRule())
	}
	if !bytes.Equal(tg.Content, cursor.DispatchRuleContent(runGate)) {
		t.Errorf("cursor content is not cursor.DispatchRuleContent(runGate)")
	}
	if got := owners[cursorRule()]; !reflect.DeepEqual(got, []string{"cursor"}) {
		t.Errorf("cursor owners = %v, want [cursor]", got)
	}
}

func TestPlanCodexOpencodeShareOneTarget(t *testing.T) {
	targets, owners := mustPlan(t, PlanInput{
		WorktreeRoot: worktreeRoot,
		Harnesses:    []string{"opencode", "codex"},
		RunGate:      runGate,
	})
	if len(targets) != 1 {
		t.Fatalf("codex+opencode planned %d targets, want 1 shared AGENTS.md: %+v", len(targets), targets)
	}
	tg := targets[0]
	if tg.Path != agentsMD() || tg.Kind != install.KindManagedBlock {
		t.Fatalf("shared target = %+v, want a single AGENTS.md managed block", tg)
	}
	if got := owners[agentsMD()]; !reflect.DeepEqual(got, []string{"codex", "opencode"}) {
		t.Errorf("shared AGENTS.md owners = %v, want [codex opencode]", got)
	}
}

func TestPlanClaudeCodexAbsentSymlinks(t *testing.T) {
	targets, owners := mustPlan(t, PlanInput{
		WorktreeRoot:  worktreeRoot,
		Harnesses:     []string{"claude", "codex"},
		RunGate:       runGate,
		ClaudeMDState: ClaudeMDAbsent,
	})
	idx := byPath(targets)
	// AGENTS.md shared block, owned by codex only (claude shares via the link,
	// it does not co-own the block).
	ag, ok := idx[agentsMD()]
	if !ok || ag.Kind != install.KindManagedBlock {
		t.Fatalf("claude+codex/absent did not plan a shared AGENTS.md block: %+v", targets)
	}
	if got := owners[agentsMD()]; !reflect.DeepEqual(got, []string{"codex"}) {
		t.Errorf("AGENTS.md owners = %v, want [codex] (claude shares via link)", got)
	}
	cl, ok := idx[claudeMD()]
	if !ok {
		t.Fatalf("no CLAUDE.md target planned")
	}
	if cl.Kind != install.KindSymlink {
		t.Errorf("CLAUDE.md kind = %q, want symlink when AGENTS.md is shared and CLAUDE.md is absent", cl.Kind)
	}
	if cl.LinkTarget != agentsMD() {
		t.Errorf("CLAUDE.md link target = %q, want %q", cl.LinkTarget, agentsMD())
	}
	if got := owners[claudeMD()]; !reflect.DeepEqual(got, []string{"claude"}) {
		t.Errorf("CLAUDE.md owners = %v, want [claude]", got)
	}
}

func TestPlanClaudeCodexLinkStateSymlinks(t *testing.T) {
	// An existing proven relative link to AGENTS.md is also a safe share.
	targets, _ := mustPlan(t, PlanInput{
		WorktreeRoot:  worktreeRoot,
		Harnesses:     []string{"claude", "codex"},
		RunGate:       runGate,
		ClaudeMDState: ClaudeMDLinkToAgents,
	})
	cl := byPath(targets)[claudeMD()]
	if cl.Kind != install.KindSymlink || cl.LinkTarget != agentsMD() {
		t.Errorf("CLAUDE.md target = %+v, want a symlink to AGENTS.md", cl)
	}
}

func TestPlanClaudeCodexRegularFileGetsBlock(t *testing.T) {
	targets, owners := mustPlan(t, PlanInput{
		WorktreeRoot:  worktreeRoot,
		Harnesses:     []string{"claude", "codex"},
		RunGate:       runGate,
		ClaudeMDState: ClaudeMDRegularFile,
	})
	idx := byPath(targets)
	cl := idx[claudeMD()]
	if cl.Kind != install.KindManagedBlock {
		t.Errorf("CLAUDE.md kind = %q, want managed block for an existing regular file", cl.Kind)
	}
	// AGENTS.md is still shared, but by codex only.
	if got := owners[agentsMD()]; !reflect.DeepEqual(got, []string{"codex"}) {
		t.Errorf("AGENTS.md owners = %v, want [codex]", got)
	}
	if got := owners[claudeMD()]; !reflect.DeepEqual(got, []string{"claude"}) {
		t.Errorf("CLAUDE.md owners = %v, want [claude]", got)
	}
}

func TestPlanClaudeCodexOtherGetsBlock(t *testing.T) {
	// `other` (e.g. an unowned link, or a foreign kind) is planned as a managed
	// block so inspection surfaces the conflict with a remedy rather than
	// silently overwriting.
	targets, _ := mustPlan(t, PlanInput{
		WorktreeRoot:  worktreeRoot,
		Harnesses:     []string{"claude", "codex"},
		RunGate:       runGate,
		ClaudeMDState: ClaudeMDOther,
	})
	cl := byPath(targets)[claudeMD()]
	if cl.Kind != install.KindManagedBlock {
		t.Errorf("CLAUDE.md kind = %q for ClaudeMDOther, want managed block", cl.Kind)
	}
}

func TestPlanEmptyHarnessesPlansNothing(t *testing.T) {
	targets, owners, err := Plan(PlanInput{WorktreeRoot: worktreeRoot, RunGate: runGate})
	if err != nil {
		t.Fatalf("empty Harnesses errored: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("empty Harnesses planned %d targets, want 0", len(targets))
	}
	if len(owners) != 0 {
		t.Errorf("empty Harnesses produced %d owners, want 0", len(owners))
	}
}

func TestPlanUnknownTokenErrors(t *testing.T) {
	_, _, err := Plan(PlanInput{
		WorktreeRoot: worktreeRoot,
		Harnesses:    []string{"claude", "emacs"},
		RunGate:      runGate,
	})
	if err == nil {
		t.Fatal("unknown harness token did not error")
	}
}

// TestPlanNeverSkillOrAgent proves the repository planner emits parent-facing
// surfaces only — never a skill symlink or an agent definition.
func TestPlanNeverSkillOrAgent(t *testing.T) {
	targets, _ := mustPlan(t, PlanInput{
		WorktreeRoot:  worktreeRoot,
		Harnesses:     []string{"claude", "codex", "opencode", "cursor"},
		RunGate:       runGate,
		ClaudeMDState: ClaudeMDRegularFile,
	})
	for _, tg := range targets {
		if tg.Role == "skill" || tg.Role == "agent" {
			t.Errorf("target %q has forbidden role %q", tg.Path, tg.Role)
		}
	}
}

// TestPlanDispatchInteriorWordingConstraint pins the "No wording changes"
// constraint: every managed block's interior is byte-identical to
// harness.DispatchInterior(runGate) — the exact bytes the Task 3 adapters emit.
func TestPlanDispatchInteriorWordingConstraint(t *testing.T) {
	want := []byte(harness.DispatchInterior(runGate))
	targets, _ := mustPlan(t, PlanInput{
		WorktreeRoot:  worktreeRoot,
		Harnesses:     []string{"claude", "codex"},
		RunGate:       runGate,
		ClaudeMDState: ClaudeMDRegularFile, // both CLAUDE.md and AGENTS.md are blocks
	})
	blocks := 0
	for _, tg := range targets {
		if tg.Kind != install.KindManagedBlock {
			continue
		}
		blocks++
		if !bytes.Equal(tg.Content, want) {
			t.Errorf("block %q interior != harness.DispatchInterior(runGate)", tg.Path)
		}
	}
	if blocks != 2 {
		t.Fatalf("expected 2 managed-block targets (CLAUDE.md + AGENTS.md), got %d", blocks)
	}
}

// TestPlanTargetsSorted proves the plan is deterministic — targets ascend by
// path and owner slices are sorted.
func TestPlanTargetsSorted(t *testing.T) {
	targets, owners := mustPlan(t, PlanInput{
		WorktreeRoot:  worktreeRoot,
		Harnesses:     []string{"cursor", "opencode", "codex", "claude"},
		RunGate:       runGate,
		ClaudeMDState: ClaudeMDRegularFile,
	})
	paths := make([]string, len(targets))
	for i, tg := range targets {
		paths[i] = tg.Path
	}
	if !sort.StringsAreSorted(paths) {
		t.Errorf("targets not sorted by path: %v", paths)
	}
	for path, o := range owners {
		if !sort.StringsAreSorted(o) {
			t.Errorf("owners for %q not sorted: %v", path, o)
		}
	}
}
