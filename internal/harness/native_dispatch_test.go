// Change 0371: the maintained native-dispatch surface — everything docket
// renders that a parent reads for routing — must carry the canonical native
// policy and no runner-era routing.
//
// After change 0351 retired the user-global dispatch block, the four adapters
// (claude/codex/cursor/opencode) render NO dispatch surface of their own — the
// dispatch surface is produced by the two seams every host shares:
// harness.DispatchInterior (the docket:dispatch managed-block interior, which
// embeds dispatchPreamble) and cursor.DispatchRuleContent (the Cursor `.mdc`
// rule, which splices the same interior). reposeed.Plan is the sole caller that
// assembles them into installed targets (guarded there by
// TestPlanRunnerFreeAndByteStable). This test keys on those producers plus every
// target the adapters actually render, never a hand list of files (AGENTS.md:
// derive sites, don't enumerate them).
package harness_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/claude"
	"github.com/danielhanold/docket/internal/harness/codex"
	"github.com/danielhanold/docket/internal/harness/cursor"
	"github.com/danielhanold/docket/internal/harness/opencode"
)

func crossAdapters(t *testing.T) map[string]harness.Adapter {
	t.Helper()
	a := map[string]harness.Adapter{
		"claude":   claude.New(),
		"codex":    codex.New(),
		"cursor":   cursor.New(),
		"opencode": opencode.New(),
	}
	if len(a) != len(harness.Order) {
		t.Fatalf("guard covers %d adapters for %d harnesses in Order", len(a), len(harness.Order))
	}
	return a
}

// runnerEraTokens are the retired Bash delegation spellings. Zero occurrences
// in ANY dispatch surface and ANY rendered adapter target. The list is a ban on
// spellings, not the property itself (learnings: byte-pattern-guard-matches-a-
// spelling) — the property "no shell/cross-harness fallback" is carried by the
// never-fall-back clause assert below plus TestNoCrossHarnessDelegation's shape
// guard.
var runnerEraTokens = []string{"runner-dispatch", "docket.sh", "scripts/runners"}

func TestNativeDispatchSurfaceRunnerFree(t *testing.T) {
	in := crossPlanInput(t)
	adapters := crossAdapters(t)

	// The two clauses every parent-facing dispatch surface must carry, spelled
	// here independently of the emitter (an assert built FROM dispatchPreamble
	// would move in lockstep with a mutated emitter and stay green).
	const identityClause = "registered same-name `docket-*` agent"
	const neverFallBack = "Never reroute a registered workflow through a shell runner, another harness, a generic agent"

	// Part A: the two dispatch-surface producers every host shares. Each embeds
	// dispatchPreamble, so a runner token inserted into the preamble reddens the
	// ban here and the identity/never-fall-back asserts pin the policy clauses.
	rg, err := harness.RunGate(in.Assets)
	if err != nil {
		t.Fatalf("RunGate: %v", err)
	}
	surfaces := map[string]string{
		"docket:dispatch interior": harness.DispatchInterior(rg),
		"cursor dispatch rule":     string(cursor.DispatchRuleContent(rg)),
	}
	dispatchSurfaces := 0
	for label, raw := range surfaces {
		body := collapseWS(raw)
		lower := strings.ToLower(body)
		for _, tok := range runnerEraTokens {
			if strings.Contains(lower, tok) {
				t.Errorf("%s carries runner-era token %q", label, tok)
			}
		}
		// A producer that no longer opens with the shared heading has stopped
		// rendering the dispatch surface — the per-surface clause asserts would
		// then be silently vacuous, so this is a hard failure, not a skip.
		if !strings.Contains(body, harness.DispatchHeading) {
			t.Errorf("%s no longer carries the dispatch heading %q", label, harness.DispatchHeading)
			continue
		}
		dispatchSurfaces++
		if !strings.Contains(body, identityClause) {
			t.Errorf("%s lost the exact-identity clause", label)
		}
		if !strings.Contains(body, neverFallBack) {
			t.Errorf("%s lost the never-fall-back clause", label)
		}
	}
	// Population floor (learnings: marker-scoped-guard-needs-a-population-floor):
	// at least one producer still renders a dispatch surface, or every
	// per-surface assert above quietly iterated zero times.
	if dispatchSurfaces == 0 {
		t.Fatalf("no producer rendered a dispatch surface carrying %q", harness.DispatchHeading)
	}

	// Part B: no adapter-rendered target (agent wrappers, skills) carries a
	// runner-era spelling either — the maintained agent surface is native-only.
	for _, name := range harness.Order {
		targets, err := adapters[name].Plan(in)
		if err != nil {
			t.Fatalf("%s Plan: %v", name, err)
		}
		if len(targets) == 0 {
			t.Fatalf("%s rendered no targets — the ban below would be vacuous", name)
		}
		for _, tg := range targets {
			lower := strings.ToLower(collapseWS(string(tg.Content)))
			for _, tok := range runnerEraTokens {
				if strings.Contains(lower, tok) {
					t.Errorf("%s target %q carries runner-era token %q", name, tg.Path, tok)
				}
			}
		}
	}
}

// TestAdapterRenderByteStable: rendering is deterministic — a second Plan over
// the same input yields byte-identical content for every target (acceptance 10).
func TestAdapterRenderByteStable(t *testing.T) {
	in := crossPlanInput(t)
	adapters := crossAdapters(t)
	for _, name := range harness.Order {
		first, err := adapters[name].Plan(in)
		if err != nil {
			t.Fatalf("%s first Plan: %v", name, err)
		}
		second, err := adapters[name].Plan(in)
		if err != nil {
			t.Fatalf("%s second Plan: %v", name, err)
		}
		if len(first) != len(second) {
			t.Fatalf("%s target count moved between renders: %d then %d", name, len(first), len(second))
		}
		for i := range first {
			if first[i].Path != second[i].Path || !bytes.Equal(first[i].Content, second[i].Content) {
				t.Errorf("%s target %q is not byte-stable across renders", name, first[i].Path)
			}
		}
	}
}
