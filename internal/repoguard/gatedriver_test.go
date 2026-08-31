package repoguard

import (
	"regexp"
	"strings"
	"testing"
)

// Ports tests/test_gate_driver_boundary.sh and its mutation sibling
// tests/test_gate_driver_boundary_mutation.sh (change 0342): the native gate
// DRIVER (`docket gate drive …`, internal/gatedrive) is the SOLE workflow
// surface for advancing a suite run. The raw verbs (`docket gate launch|observe|
// stop`) and the app-orchestration seam (GateLaunch|GateObserve|GateStop) survive
// only as PRIMITIVES the driver composes; a workflow caller never composes them
// directly, never re-parses raw observation state, never recreates a poll loop,
// and every task-level WAITING return names an explicit ownership handoff.
//
// Four detectors, each keyed on a syntactic SHAPE (never a per-file allowlist),
// classified by PATH shape (which layer a file is) plus CONTENT shape (a fenced
// runnable recipe vs an inline prose mention). Residual risk, recorded not hidden:
// an author who writes an imperative raw invocation in INLINE backticks rather
// than a fence dodges the markdown detector (A/C); the fence is the mechanical
// shape signal the house convention uses for every runnable recipe, and the
// Go/handoff detectors carry the rest of the teeth.

var (
	rawGateVerb    = regexp.MustCompile(`docket[[:space:]]+gate[[:space:]]+(launch|observe|stop)([[:space:]]|$)`)
	rawGateObserve = regexp.MustCompile(`docket[[:space:]]+gate[[:space:]]+observe`)
	pollIdiom      = regexp.MustCompile(`(?i)jq|sleep[[:space:]]+[0-9]|while`)
	directGoCall   = regexp.MustCompile(`\.(GateLaunch|GateObserve|GateStop)\(`)

	waitFwd      = regexp.MustCompile(`(^|[^-A-Za-z])WAITING([^-A-Za-z]).{0,45}(COMPLETE|BLOCKED|NEEDS_ESCALATION)`)
	waitRev      = regexp.MustCompile(`(COMPLETE|BLOCKED|NEEDS_ESCALATION)([^-A-Za-z]).{0,45}([^-A-Za-z])WAITING([^-A-Za-z])`)
	namesHandoff = regexp.MustCompile(`(?i)handoff token`)
	forbidsBare  = regexp.MustCompile(`(?i)no handoff token|without[^.]{0,25}handoff|bare[^.]{0,45}wait`)

	permittedGoLayer = regexp.MustCompile(`^(internal/cli/|internal/gatedrive/|internal/process/)`)
	appGateSeam      = regexp.MustCompile(`^internal/app/gate[^/]*\.go$`)
)

// isWorkflowMD reports whether rel is an executable-workflow markdown/toml file:
// skill and agent definitions, plus their embedded authored/generated copies.
func isWorkflowMD(rel string) bool {
	if !hasExt(rel, ".md", ".toml") {
		return false
	}
	return underDir(rel, "skills", "agents",
		"internal/assets/embedded/tree/skills", "internal/assets/embedded/tree/agents")
}

// isWorkflowSH reports whether rel is an executable-workflow shell script. Tests
// live under tests/ and are a permitted primitive-level category, so they are not
// workflow shell.
func isWorkflowSH(rel string) bool {
	return underDir(rel, "scripts") && hasExt(rel, ".sh")
}

// isOrchestrationGo reports whether rel is a non-test Go file OUTSIDE the layers
// where the raw gate seam legitimately lives.
func isOrchestrationGo(rel string) bool {
	if !hasExt(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
		return false
	}
	if !underDir(rel, "internal", "cmd") {
		return false
	}
	if permittedGoLayer.MatchString(rel) || appGateSeam.MatchString(rel) {
		return false
	}
	return true
}

// scanRawFenced (A): a raw gate verb inside a fenced markdown block, or on a
// non-comment command line of a workflow shell script.
func scanRawFencedMD(rel, content string) []string {
	var v []string
	inFence := false
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence && rawGateVerb.MatchString(line) {
			v = append(v, "A\t"+rel+": raw gate verb in a fenced recipe")
		}
	}
	return v
}

func scanRawFencedSH(rel, content string) []string {
	var v []string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if rawGateVerb.MatchString(line) {
			v = append(v, "A\t"+rel+": raw gate verb on a command line")
		}
	}
	return v
}

// scanPollLoop (C): a fenced block that re-parses raw observe state or recreates
// a sleep/poll loop.
func scanPollLoop(rel, content string) []string {
	inFence := false
	var fenced strings.Builder
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			fenced.WriteString(line)
			fenced.WriteByte('\n')
		}
	}
	block := fenced.String()
	if rawGateObserve.MatchString(block) && pollIdiom.MatchString(block) {
		return []string{"C\t" + rel + ": fenced observe poll/parse loop"}
	}
	return nil
}

// scanWaitingHandoff (D): a WAITING outcome contract that omits the explicit
// handoff identity. Back-ticks stripped and whitespace flattened first so a
// back-tick-wrapped vocabulary still reads as a token set.
func scanWaitingHandoff(rel, content string) []string {
	flat := strings.ReplaceAll(content, "`", "")
	flat = strings.Join(strings.Fields(flat), " ")
	if !waitFwd.MatchString(flat) && !waitRev.MatchString(flat) {
		return nil
	}
	if namesHandoff.MatchString(flat) && forbidsBare.MatchString(flat) {
		return nil
	}
	return []string{"D\t" + rel + ": WAITING outcome without an explicit handoff identity"}
}

// scanDirectGoCall (B): a direct app-orchestration call to the gate seam outside
// the permitted layers.
func scanDirectGoCall(rel, content string) []string {
	var v []string
	for _, line := range strings.Split(content, "\n") {
		if directGoCall.MatchString(line) {
			v = append(v, "B\t"+rel+": direct GateLaunch/Observe/Stop call outside cli/driver/process")
		}
	}
	return v
}

func TestGateDriverBoundary(t *testing.T) {
	root := guardRoot(t)
	pop := maintainedPop(t, root)

	var wfMD, wfSH, orchGo []string
	for _, rel := range pop {
		switch {
		case isWorkflowMD(rel):
			wfMD = append(wfMD, rel)
		case isWorkflowSH(rel):
			wfSH = append(wfSH, rel)
		}
		if isOrchestrationGo(rel) {
			orchGo = append(orchGo, rel)
		}
	}

	// Population floors FIRST: an empty enumeration passes every "no violations"
	// negative by default.
	if len(wfMD) < 20 {
		t.Fatalf("population floor: only %d workflow-contract files (expected >= 20)", len(wfMD))
	}
	if len(orchGo) < 20 {
		t.Fatalf("population floor: only %d orchestration Go files (expected >= 20)", len(orchGo))
	}
	// The driver surface being protected actually exists, and detector D has live
	// input (a real WAITING contract).
	if !slices_containsPrefix(pop, "internal/gatedrive/") {
		t.Fatalf("the native gate driver package internal/gatedrive is absent")
	}
	buildTask := readMaintained(t, root, "skills/docket-build-task/SKILL.md")
	if !regexp.MustCompile(`(^|[^-A-Za-z])WAITING([^-A-Za-z])`).MatchString(buildTask) {
		t.Fatalf("detector D has no live input: skills/docket-build-task/SKILL.md declares no WAITING outcome")
	}

	var violations []string
	for _, rel := range wfMD {
		c := readMaintained(t, root, rel)
		violations = append(violations, scanRawFencedMD(rel, c)...)
		violations = append(violations, scanPollLoop(rel, c)...)
		violations = append(violations, scanWaitingHandoff(rel, c)...)
	}
	for _, rel := range wfSH {
		violations = append(violations, scanRawFencedSH(rel, readMaintained(t, root, rel))...)
	}
	for _, rel := range orchGo {
		violations = append(violations, scanDirectGoCall(rel, readMaintained(t, root, rel))...)
	}
	if len(violations) != 0 {
		t.Errorf("gate-driver boundary violations:\n%s", strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		// (A) raw verb fenced vs inline.
		fenced := "```bash\ndocket gate launch implement-next\n```\n"
		if got := scanRawFencedMD("skills/x/SKILL.md", fenced); len(got) == 0 {
			t.Errorf("A: missed a raw gate verb in a fenced recipe")
		}
		inline := "run `docket gate observe` only through the driver\n"
		if got := scanRawFencedMD("skills/x/SKILL.md", inline); len(got) != 0 {
			t.Errorf("A: wrongly flagged an inline prose mention: %v", got)
		}
		// A shell command line vs a comment.
		if got := scanRawFencedSH("scripts/x.sh", "docket gate stop\n"); len(got) == 0 {
			t.Errorf("A: missed a raw gate verb on a shell command line")
		}
		if got := scanRawFencedSH("scripts/x.sh", "# docket gate stop is a primitive\n"); len(got) != 0 {
			t.Errorf("A: wrongly flagged a shell comment: %v", got)
		}
		// (B) direct Go call outside vs inside permitted layers.
		call := "\treturn s.GateLaunch(root, cwd, argv)\n"
		if !isOrchestrationGo("internal/app/finalize.go") {
			t.Errorf("B: internal/app/finalize.go should be orchestration Go")
		}
		if got := scanDirectGoCall("internal/app/finalize.go", call); len(got) == 0 {
			t.Errorf("B: missed a direct GateLaunch call in an orchestration file")
		}
		if isOrchestrationGo("internal/cli/gate.go") {
			t.Errorf("B: internal/cli/gate.go must be a permitted layer")
		}
		if isOrchestrationGo("internal/app/gate_supervisor.go") {
			t.Errorf("B: internal/app/gate_supervisor.go (gate seam family) must be permitted")
		}
		// (C) fenced observe poll loop vs a clean driver recipe.
		poll := "```bash\nwhile :; do state=$(docket gate observe run | jq -r .state); sleep 5; done\n```\n"
		if got := scanPollLoop("skills/x/SKILL.md", poll); len(got) == 0 {
			t.Errorf("C: missed a fenced observe poll loop")
		}
		clean := "```bash\ndocket gate drive advance run\n```\n"
		if got := scanPollLoop("skills/x/SKILL.md", clean); len(got) != 0 {
			t.Errorf("C: wrongly flagged a clean driver recipe: %v", got)
		}
		// (D) WAITING contract missing the handoff clauses vs the real, complete one.
		bad := "Return COMPLETE, WAITING, or BLOCKED when the run stalls.\n"
		if got := scanWaitingHandoff("skills/x/SKILL.md", bad); len(got) == 0 {
			t.Errorf("D: missed a WAITING contract with no handoff identity")
		}
		if got := scanWaitingHandoff("skills/docket-build-task/SKILL.md", buildTask); len(got) != 0 {
			t.Errorf("D: wrongly flagged the real, complete build-task WAITING contract: %v", got)
		}
	})
}

func slices_containsPrefix(xs []string, prefix string) bool {
	for _, x := range xs {
		if strings.HasPrefix(x, prefix) {
			return true
		}
	}
	return false
}
