package repoguard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file ports the PROSE / CONTRACT sentinels off the retired Bash suite
// (change 0370, Gate 2, batch B). Each retired tests/test_*.sh sentinel asserted
// that a maintained, agent-executed prose surface (a skill body, a convention
// reference, an operator runbook, README, or a config example) still carries the
// load-bearing sentence(s) of some agent contract — or, for a few, that a retired
// route stays ABSENT from it. Task 6 makes `docket development test` discovery
// fail-closed on any undeclared tests/*.sh, so these cannot survive as live Bash;
// their surviving invariant moves here.
//
// # Why a phrase table is the right guard shape here (not a shape-derived scan)
//
// AGENTS.md's "key a guard on syntactic shape, never a spelling" rule governs
// guards over OPERATIONS and LITERALS, where the spelling you miss is the house
// idiom that violates the gate. A prose-contract sentinel is the opposite: its
// subject IS a specific, load-bearing sentence of an agent contract, and the only
// faithful guard is "this exact contract sentence is still present" — which is
// exactly what every retired sentinel did (grep -qF "<sentence>" <file>). The
// phrases chosen here are the most distinctive, stable clause of each contract, so
// the guard reddens when that contract is deleted or reworded, and a reword is a
// deliberate, reviewable edit to this table (the drift is mechanically visible).
//
// # Current-prose anchor, and the Task 7 coupling
//
// Every phrase is anchored on the CURRENT, post-0377 prose (verified present, or
// verified absent, at authoring time). Task 7 later corrects some active prose off
// the retired route; if it rewords a clause this table depends on, it repoints
// the row in the same commit — the same coupling AGENTS.md already requires
// between a rewritten sentence and its dependent assert.
//
// # docs/ is read by path, not through MaintainedFiles
//
// MaintainedFiles categorically excludes docs/ as immutable point-in-time history,
// but docs/cursor/ and docs/codex/ are ACTIVE operator documentation an agent
// executes. This guard reads every contract file directly by path, fail-closed (a
// moved/renamed file is a read error and fails the test), so the docs/ exclusion
// does not blind it to those surfaces.
//
// # Two structural sentinels are NOT ported here (recorded, not stubbed)
//
// tests/test_config_read_channel.sh and tests/test_inline_role_stop_scoping.sh are
// not phrase-presence sentinels: each is a repo-wide STRUCTURAL scanner (the first
// classifies every config-read reference across the skill surface against inline
// markers; the second proximity-scopes every reader-directed "stop" across the
// role-skill surface). A phrase stub for either would be a vacuous guard — the
// exact named risk — so they are left for a dedicated structural port and must NOT
// be deleted (Task 8) until that port exists. The role-skill self-description and
// skill-handoff CONVENTION clauses those two touch ARE captured below.

// proseContract is one sentinel's surviving invariant over one maintained file:
// every string in present must appear, every string in absent must not.
type proseContract struct {
	sentinel string // the retired tests/test_*.sh this row replaces
	file     string // slash path relative to repo root
	present  []string
	absent   []string
}

// proseContracts is the ported table. One row per (sentinel, file); a sentinel
// that guarded prose in several files contributes several rows.
var proseContracts = []proseContract{
	// tests/test_auto_groom.sh — autonomous-groom contract.
	{sentinel: "test_auto_groom", file: "skills/docket-auto-groom/SKILL.md",
		present: []string{"Kill and defer are NEVER autonomous", "docket-auto-groom-critic"}},
	// tests/test_composition_wiring.sh — never-yield composition rule (change 0066).
	{sentinel: "test_composition_wiring", file: "skills/docket-convention/SKILL.md",
		present: []string{"to await a task-notification"}, absent: []string{"will spawn"}},
	// tests/test_consultant_brainstorm.sh — single-dispatch consultant flow (change 0056).
	{sentinel: "test_consultant_brainstorm", file: "skills/docket-brainstorm/SKILL.md",
		present: []string{"docket-brainstorm-consultant"}},
	{sentinel: "test_consultant_brainstorm", file: "README.md",
		present: []string{"brainstorm: docket-brainstorm"}},
	// tests/test_convention_extraction.sh — operating skills carry the load-first line
	// and never copy the convention (the begin marker is a copy tell).
	{sentinel: "test_convention_extraction", file: "skills/docket-implement-next/SKILL.md",
		present: []string{"## Convention (load first — blocking)"},
		absent:  []string{"<!-- docket:convention:begin -->"}},
	// tests/test_critic_return_channel.sh — the critic's return-channel contract.
	{sentinel: "test_critic_return_channel", file: "agents/docket-auto-groom-critic.md",
		present: []string{"adversarial critic", "not registered under its skill name"}},
	// tests/test_dummy_mode.sh — the shared dummy-mode definition + its reference.
	{sentinel: "test_dummy_mode", file: "skills/docket-convention/SKILL.md",
		present: []string{"### Dummy mode (shared definition)"}},
	{sentinel: "test_dummy_mode", file: "skills/docket-convention/references/dummy-mode.md",
		present: []string{"In plain terms"}},
	// tests/test_finalize_closeout_notes.sh — the closeout-notes handoff contract.
	{sentinel: "test_finalize_closeout_notes", file: "skills/docket-convention/SKILL.md",
		present: []string{"Written solely by the `finalize.closeout` operation"}},
	{sentinel: "test_finalize_closeout_notes", file: "skills/docket-finalize-change/SKILL.md",
		present: []string{"never pauses after merge"}},
	// tests/test_finalize_disposition.sh — Go-owned selection + id allowlist.
	{sentinel: "test_finalize_disposition", file: "skills/docket-finalize-change/SKILL.md",
		present: []string{"SelectFinalizeQueue", "--allowlist <ids>"}},
	// tests/test_finalize_gate.sh — the two conflict/repair dispatch names.
	{sentinel: "test_finalize_gate", file: "skills/docket-finalize-change/SKILL.md",
		present: []string{"docket-rebase-resolver", "docket-integration-repair"}},
	// change 0396 — the WAITING re-entry route: bound to re-running the identical
	// finalize.rebase invocation, with the gate-drive-advance misuse named as the
	// prohibition (the phrase is bound to its claim in one sentence, not floating;
	// learnings: prose-guard-binds-phrase-to-claim).
	{sentinel: "test_finalize_gate_waiting", file: "skills/docket-finalize-change/SKILL.md",
		present: []string{
			"`waiting` with `reason: gate-waiting`",
			"Re-run the **identical** `finalize.rebase` invocation",
			"Never re-enter through `gate drive advance`",
		}},
	{sentinel: "test_finalize_gate_waiting", file: "skills/docket-finalize-change/references/gate-failure.md",
		present: []string{"A `waiting` (`reason: gate-waiting`) is not in this set"}},
	// tests/test_groom_recap.sh — recap-then-groom Step 3.
	{sentinel: "test_groom_recap", file: "skills/docket-groom-next/SKILL.md",
		present: []string{"### Step 3 — Recap, then groom with the human"}},
	// tests/test_learnings_ledger.sh — ledger section + tiering criterion.
	{sentinel: "test_learnings_ledger", file: "skills/docket-convention/SKILL.md",
		present: []string{"### Learnings ledger", "will the agent know to search for this?"}},
	// tests/test_loop_continuation.sh — the run-does-not-end / aborted-run rule.
	{sentinel: "test_loop_continuation", file: "skills/docket-implement-next/SKILL.md",
		present: []string{"the run does not end until", "is by construction an aborted run"}},
	// tests/test_plan_writer_step4.sh — Step 4 plan-writer dispatch.
	{sentinel: "test_plan_writer_step4", file: "skills/docket-implement-next/SKILL.md",
		present: []string{"docket-plan-writer"}},
	// tests/test_results_artifact.sh — merged plan/results freeze rule.
	{sentinel: "test_results_artifact", file: "skills/docket-convention/SKILL.md",
		present: []string{"Merged plans and results are frozen build records."}},
	// tests/test_role_skill_self_description.sh — the role-skill self-description rule
	// (also the surviving convention clause test_inline_role_stop_scoping touches).
	{sentinel: "test_role_skill_self_description", file: "skills/docket-convention/SKILL.md",
		present: []string{"skills.<role>"}},
	// tests/test_skill_fork_dispatch.sh — fork-dispatch README contract.
	{sentinel: "test_skill_fork_dispatch", file: "README.md",
		present: []string{"completed (forked execution)", "The right model for each step."}},
	// tests/test_skill_handoff_precedence.sh — convention-clause half (site-scan half flagged).
	{sentinel: "test_skill_handoff_precedence", file: "skills/docket-convention/SKILL.md",
		present: []string{"never outranks", "DIRECTED to:"}},
	// tests/test_readme_finalize_docs.sh — README finalize/auto-mode docs.
	{sentinel: "test_readme_finalize_docs", file: "README.md",
		present: []string{"auto-mode classifier", "Fork-exclusion principle"}},
	// tests/test_readme_skill_catalog.sh — count-free catalog heading, no stale anchor.
	{sentinel: "test_readme_skill_catalog", file: "README.md",
		present: []string{"## Skills"}, absent: []string{"#the-eight-skills"}},
	// tests/test_cursor_dispatch_rule.sh — cursor dispatch head contract.
	{sentinel: "test_cursor_dispatch_rule", file: "cursor-rules/dispatch.head.md",
		present: []string{"## Required dispatch pattern", "run the skill inline"}},
	// tests/test_cursor_contract_docs.sh — cursor validation merge-gate obligation.
	{sentinel: "test_cursor_contract_docs", file: "docs/cursor/validation.md",
		present: []string{"## The merge-gate obligation"}},
	// tests/test_cursor_permissions_docs.sh — the guide exists and README links it.
	{sentinel: "test_cursor_permissions_docs", file: "docs/cursor/permissions.md",
		present: []string{"permissions"}},
	{sentinel: "test_cursor_permissions_docs", file: "README.md",
		present: []string{"](docs/cursor/permissions.md)"}},
	// tests/test_codex_runbook.sh — codex runbook slug-derivation + no fabricated path.
	{sentinel: "test_codex_runbook", file: "docs/codex/validation-runbook.md",
		present: []string{"codex debug models"}, absent: []string{"scripts/sync-agents.sh"}},
	// tests/test_docket_build.sh — the per-task worker contract.
	{sentinel: "test_docket_build", file: "skills/docket-build-task/SKILL.md",
		present: []string{"self-review is part of", "Implement only that task"}},
	// tests/test_docket_review.sh — the review role contract.
	{sentinel: "test_docket_review", file: "skills/docket-review/SKILL.md",
		present: []string{"build-evidence", "abort-and-report"}},
	// tests/test_gate_caller_loop.sh — the gate driver caller-loop reference.
	{sentinel: "test_gate_caller_loop", file: "skills/docket-build/references/gate-caller-loop.md",
		present: []string{"## The disposition vocabulary", "## Handoff"}},
	// tests/test_gate_execution_posture.sh — gate-execution reference points at the caller loop.
	{sentinel: "test_gate_execution_posture", file: "skills/docket-build/references/gate-execution.md",
		present: []string{"gate-caller-loop"}},
	// tests/test_dispatch_capability.sh — the convention's dispatch-capability rule.
	{sentinel: "test_dispatch_capability", file: "skills/docket-convention/SKILL.md",
		present: []string{"Dispatch-capability resolution", "never from a tool name"}},
	// tests/test_docket_metadata_branch.sh — deferred-publish prose + retired-route absence.
	{sentinel: "test_docket_metadata_branch", file: "skills/docket-finalize-change/SKILL.md",
		absent: []string{"checkout origin/docket"}},
	{sentinel: "test_docket_metadata_branch", file: "skills/docket-adr/SKILL.md",
		present: []string{"adr-unpublished"}},
	{sentinel: "test_docket_metadata_branch", file: "skills/docket-new-change/SKILL.md",
		present: []string{"terminal publication is deferred from Go v1"}},
	// tests/test_docket_example_yml.sh — key-presence core (full correspondence scan flagged).
	{sentinel: "test_docket_example_yml", file: ".docket.example.yml",
		present: []string{"board_surfaces", "agent_harnesses", "finalize:"}},
	// tests/test_typed_changes_docs.sh — README typed-change vocabulary rule.
	{sentinel: "test_typed_changes_docs", file: "README.md",
		present: []string{"untyped set can only shrink"}},
	// tests/test_change_types.sh — the change template still ships a type placeholder.
	{sentinel: "test_change_types", file: "skills/docket-new-change/change-template.md",
		present: []string{"type:"}},
	// change 0389 — implementation-scope sweep + the two completion barriers.
	// docket-status owns the COMMAND barrier: a backgrounded sweep is observed
	// to its terminal envelope, never declared done by proxy signals; and an
	// applied envelope is never read as all-items-succeeded.
	{sentinel: "change_0389_sweep_scope", file: "skills/docket-status/SKILL.md",
		present: []string{"--scope implementation", "a liveness transition, not completion",
			"never start a second shell watcher", "never that every item succeeded"}},
	// docket-implement-next owns the AGENT barrier: terminal evidence for the
	// requested scope, and a first-late terminal result is a violation.
	{sentinel: "change_0389_sweep_scope", file: "skills/docket-implement-next/SKILL.md",
		present: []string{"--scope implementation", "terminal sweep evidence for implementation scope",
			"a contract violation, not a dismissable duplicate"}},
	// The convention no longer implies a full historical sweep at startup, and
	// the status dispatch contract is hybrid.
	{sentinel: "change_0389_sweep_scope", file: "skills/docket-convention/SKILL.md",
		present: []string{"no longer implies a full historical sweep"}},
}

// scanProse checks one file's content against a contract, returning a violation
// per missing-required or present-forbidden phrase. This is the whole detector, so
// the non_vacuity subtest below can exercise it directly.
func scanProse(rel, content string, present, absent []string) []string {
	var v []string
	for _, p := range present {
		if !strings.Contains(content, p) {
			v = append(v, fmt.Sprintf("%s: required contract phrase is missing: %q", rel, p))
		}
	}
	for _, a := range absent {
		if strings.Contains(content, a) {
			v = append(v, fmt.Sprintf("%s: retired/forbidden phrase is present: %q", rel, a))
		}
	}
	return v
}

func TestProseContracts(t *testing.T) {
	root := guardRoot(t)

	// Population floor: the total number of phrase checks. A collapse means rows
	// were lost or the table was gutted — an empty walk is an error, not a pass
	// (marker-scoped-guard-needs-a-population-floor).
	checks := 0
	for _, c := range proseContracts {
		checks += len(c.present) + len(c.absent)
	}
	if checks < 40 {
		t.Fatalf("population floor: only %d prose-contract phrase checks (expected >= 40)", checks)
	}

	// Every distinct contract file must exist and be readable (fail-closed: a
	// moved/renamed/deleted contract surface is a read error and fails the test).
	var violations []string
	cache := map[string]string{}
	for _, c := range proseContracts {
		content, ok := cache[c.file]
		if !ok {
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.file)))
			if err != nil {
				t.Fatalf("read contract file %s (sentinel %s): %v (fail closed)", c.file, c.sentinel, err)
			}
			content = string(b)
			cache[c.file] = content
		}
		for _, msg := range scanProse(c.file, content, c.present, c.absent) {
			violations = append(violations, fmt.Sprintf("[%s] %s", c.sentinel, msg))
		}
	}
	if len(violations) != 0 {
		t.Errorf("prose-contract violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		// A missing required phrase is detected.
		if got := scanProse("x.md", "nothing here", []string{"must appear"}, nil); len(got) != 1 {
			t.Errorf("scanProse missed a missing-required phrase: %v", got)
		}
		// A present forbidden phrase is detected.
		if got := scanProse("x.md", "this has a forbidden token", nil, []string{"forbidden"}); len(got) != 1 {
			t.Errorf("scanProse missed a present-forbidden phrase: %v", got)
		}
		// A satisfied contract produces no violation.
		if got := scanProse("x.md", "must appear and nothing else", []string{"must appear"}, []string{"gone"}); len(got) != 0 {
			t.Errorf("scanProse flagged a satisfied contract: %v", got)
		}
	})
}
