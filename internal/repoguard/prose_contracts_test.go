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
// but docs/reference/harness/ is ACTIVE operator documentation an agent
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
	{sentinel: "test_consultant_brainstorm", file: "docs/guide/designing-before-building.md",
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
	{sentinel: "test_skill_fork_dispatch", file: "docs/install/models-and-effort.md",
		present: []string{"completed (forked execution)"}},
	{sentinel: "test_skill_fork_dispatch", file: "README.md",
		present: []string{"The right model for each step."}},
	// tests/test_skill_handoff_precedence.sh — convention-clause half (site-scan half flagged).
	{sentinel: "test_skill_handoff_precedence", file: "skills/docket-convention/SKILL.md",
		present: []string{"never outranks", "DIRECTED to:"}},
	// tests/test_readme_finalize_docs.sh — README finalize/auto-mode docs.
	{sentinel: "test_readme_finalize_docs", file: "docs/guide/landing-changes.md",
		present: []string{"auto-mode classifier"}},
	{sentinel: "test_readme_finalize_docs", file: "docs/install/models-and-effort.md",
		present: []string{"Fork-exclusion principle"}},
	// tests/test_readme_skill_catalog.sh — count-free catalog heading, no stale anchor.
	{sentinel: "test_readme_skill_catalog", file: "docs/reference/skills-and-agents.md",
		present: []string{"## Skills"}, absent: []string{"#the-eight-skills"}},
	// tests/test_cursor_dispatch_rule.sh — cursor dispatch head contract.
	{sentinel: "test_cursor_dispatch_rule", file: "cursor-rules/dispatch.head.md",
		present: []string{"## Required dispatch pattern", "run the skill inline"}},
	// tests/test_cursor_contract_docs.sh — cursor validation merge-gate obligation
	// (moved to the harness reference by change 0402).
	{sentinel: "test_cursor_contract_docs", file: "docs/reference/harness/validation.md",
		present: []string{"## The merge-gate obligation"}},
	// tests/test_cursor_permissions_docs.sh — the permissions guidance survives on the
	// Cursor install page, which must link the example JSONs in the harness reference.
	// Change 0402 folded docs/cursor/permissions.md into that page, so this
	// sentinel's two rows collapse to one file; both invariants are kept as phrases.
	{sentinel: "test_cursor_permissions_docs", file: "docs/install/cursor.md",
		present: []string{"permissions.example.json", "](../reference/harness/"}},
	// tests/test_codex_runbook.sh — codex runbook slug-derivation + no fabricated path
	// (moved to the harness reference by change 0402).
	{sentinel: "test_codex_runbook", file: "docs/reference/harness/validation-runbook.md",
		present: []string{"codex debug models"}, absent: []string{"scripts/sync-agents.sh"}},
	// change 0393 amendment — ordinary children split by typed worktree scope:
	// metadata remains native, while feature roles use foreground root entry with
	// the canonical worktree and unchanged workflow payload.
	{sentinel: "change_0393_feature_child_entry", file: "docs/reference/harness/validation-runbook.md",
		present: []string{
			"Metadata-scoped ordinary child roles may continue to use direct registered-agent invocation.",
			"Feature-scoped ordinary child roles must enter through foreground `agent.enter` with `--worktree`\nset to the absolute canonical feature-worktree root and carry the unchanged structured payload.",
		},
		absent: []string{"Ordinary child roles may continue to use direct registered-agent invocation."}},
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
	{sentinel: "test_typed_changes_docs", file: "docs/guide/capturing-work.md",
		present: []string{"untyped set can only shrink"}},
	// tests/test_change_types.sh — the change template still ships a type placeholder.
	{sentinel: "test_change_types", file: "skills/docket-new-change/change-template.md",
		present: []string{"type:"}},
	// change 0400 — the goal-first landing page cannot silently lose its two
	// load-bearing map links (the docs index — retargeted from the relocated
	// guide by change 0402 — and the comparison page).
	{sentinel: "change_0400_readme_landing", file: "README.md",
		present: []string{"](docs/README.md)", "](docs/comparison/ai-native-sdlc-playbook.md)"}},
	// change 0389 — implementation-scope sweep + the two completion barriers.
	// docket-status owns the COMMAND barrier: a backgrounded sweep is observed
	// to its terminal envelope, never declared done by proxy signals; and an
	// applied envelope is never read as all-items-succeeded.
	{sentinel: "change_0389_sweep_scope", file: "skills/docket-status/SKILL.md",
		present: []string{"--scope implementation", "a liveness transition, not completion",
			"never start a second shell watcher", "never that every item succeeded"}},
	// change 0397 — Step 0 is one inline deterministic operation. The absent
	// phrases are the retired step-0 dispatch instruction and the completion
	// barrier that only a child return needed; the present phrases bind the
	// inline call to its two authoritative fields.
	{sentinel: "change_0397_preflight_op", file: "skills/docket-implement-next/SKILL.md",
		present: []string{"maintenance.preflight", "the envelope `result` and the Go-computed `preflight` verdict",
			"as its own Bash call"},
		absent: []string{"dispatch the `docket-status` subagent",
			"terminal sweep evidence for implementation scope"}},
	// The convention no longer implies a full historical sweep at startup, and
	// the status dispatch contract is hybrid.
	{sentinel: "change_0389_sweep_scope", file: "skills/docket-convention/SKILL.md",
		present: []string{"no longer implies a full historical sweep"}},
	// change 0397 — the retired step-0 dispatch must not reappear through the
	// status/convention prose either. Each present phrase binds the surviving
	// text to the inline operation; each absent phrase is the retired dispatch
	// wording this change removed from that file.
	{sentinel: "change_0397_preflight_op", file: "skills/docket-status/SKILL.md",
		present: []string{"maintenance.preflight"},
		absent:  []string{"calls this at step 0"}},
	{sentinel: "change_0397_preflight_op", file: "skills/docket-convention/SKILL.md",
		present: []string{"runs the `maintenance.preflight` operation inline"},
		absent:  []string{"dispatches the `docket-status` subagent (step 0)"}},
	// change 0407 — implement-next's gated claim carries its dispatch context so
	// the parent's keyed verdict resolves ownership from durable proof, and an
	// invalid/conflicting context fails closed rather than degrading to an
	// ungated claim.
	{sentinel: "change_0407_gate_context_claim", file: "skills/docket-implement-next/SKILL.md",
		present: []string{
			"pass it to the claim as --gate-context",
			"an invalid or conflicting gate context is a typed refusal that writes nothing — never retried as an ungated claim",
		}},
	// change 0410 introduced the canonical required template; change 0440
	// re-shaped it around the reader: a required Human action statement after
	// the title, and Human testing / Findings and limitations / Follow-ups
	// merged into Human actions and testing + Known issues and follow-ups.
	// Absent phrases are the REMOVED old section headings and the retired
	// optional-template triggers (assert-detects-removal).
	{sentinel: "change_0440_results_template", file: "skills/docket-implement-next/results-template.md",
		present: []string{
			"**Human action:**",
			"## Outcome",
			"## Human actions and testing",
			"## Verification performed",
			"## Known issues and follow-ups",
		},
		absent: []string{
			"## Human testing",
			"## Findings and limitations",
			"## Follow-ups",
			"OPTIONAL: write one only",
			"## Verify (human)",
		}},
	// change 0410 — implement-next Step 6.5 is mandatory, not an optional close-out.
	// The present phrases bind the required-results obligation and the
	// never-commit-under-a-live-gate checkpoint clause (each an unwrapped
	// sub-clause of its sentence); the absent phrase is the retired optional label.
	{sentinel: "change_0410_implement_next_results", file: "skills/docket-implement-next/SKILL.md",
		present: []string{
			"Step 6.5 — Results (required)",
			"required for every change, trivial included",
			"never commit or move HEAD beneath a live gate, a running worker, or a transferred drive",
		},
		absent: []string{"Results close-out (optional)"}},
	// change 0410 — the convention now describes results as a REQUIRED close-out
	// artifact in both the directory-map row and the lifecycle paragraph. (The
	// existing frozen-records row above — sentinel test_results_artifact — is left
	// untouched and still pins "Merged plans and results are frozen build records.")
	{sentinel: "change_0410_convention_results", file: "skills/docket-convention/SKILL.md",
		present: []string{
			"required close-out artifacts (one per implemented change, trivial included; change 0410)",
			"required close-out artifact for every implemented change, trivial included (change 0410)",
		}},
	// change 0410 — docket-build's end-of-build capture ownership: the controller
	// may checkpoint on the coordinator's behalf, but task workers never write the
	// results file and a checkpoint never independently launches tests (each an
	// unwrapped sub-clause).
	{sentinel: "change_0410_build_results", file: "skills/docket-build/SKILL.md",
		present: []string{
			"Task workers never edit the results file",
			"a checkpoint never independently launches tests",
		}},
	// change 0440 — Step 6.5 and the convention now describe the reader-first
	// results shape. Present phrases bind the action-statement requirement and
	// the optional-walkthrough allowance; absent phrases are the retired
	// old-section wording and the retired blanket prohibition on manually
	// checking automated behavior (assert-detects-removal).
	{sentinel: "change_0440_results_prose", file: "skills/docket-implement-next/SKILL.md",
		present: []string{
			"**Human action:**",
			"**Human actions and testing**",
			"Known issues and follow-ups",
			"An Optional walkthrough MAY exercise behavior automated tests already cover",
		},
		absent: []string{
			"only functional scenarios the automated tests do **not** cover",
			"under Findings and limitations or Verification performed",
		}},
	{sentinel: "change_0440_convention_results", file: "skills/docket-convention/SKILL.md",
		present: []string{
			"`**Human action:**` statement",
			"`## Human actions and testing`",
			"`## Known issues and follow-ups`",
		},
		absent: []string{
			"`## Human testing`",
			"`## Findings and limitations`",
			"`## Follow-ups`",
		}},
	// change 0448 — a single-explicit-id (named) invocation skips the
	// maintenance preflight and never falls back to selection. Present phrases
	// bind each claim inside one sentence; the absent phrase is the retired
	// UNCONDITIONAL preflight opener, so restoring mandatory maintenance on a
	// named request reddens this row (assert-detects-removal). Mutation-tested
	// at introduction.
	{sentinel: "change_0448_named_preflight_skip", file: "skills/docket-implement-next/SKILL.md",
		present: []string{
			"**Named invocation — no maintenance preflight.**",
			"The named path never runs `maintenance.preflight` or any maintenance sweep first",
			"a named request **never falls back to selecting another change**",
			// The bounded own-dependency closeout lives in edge-paths.md
			// (progressive disclosure); SKILL.md keeps only the blocking
			// pointer at its trigger, pinned so the edge cannot go unread.
			"refuses `not-ready-waiting-dependency`, **read `references/edge-paths.md` now (blocking)** for the bounded closeout",
			// Negative counterpart: the exemption is exactly one id, so no
			// argument and an id set still run the maintenance preflight.
			// Deleting "or an id set" reddens this row (mutation-tested).
			"Otherwise — no argument, or an id set — run, before selection, the **implementation preflight** inline",
			"an **id allowlist** of two or more ids",
		},
		absent: []string{
			"Then, before selection, run the **implementation preflight** inline",
			"a single id is the degenerate case",
			"a single id `90` is the degenerate case",
		}},
	// change 0448 — the named change's own merged dependencies get a BOUNDED
	// finalize.closeout limited to that dependency set, read on demand from
	// edge-paths.md at the waiting-dependency trigger.
	{sentinel: "change_0448_named_preflight_skip", file: "skills/docket-implement-next/references/edge-paths.md",
		present: []string{
			"## Named invocation's own merged dependencies (Step 0, bounded closeout)",
			"For **each** id in that set — and never any other change — run one `finalize.closeout` operation",
			"An unrelated change's closeout is never attempted here",
			// A stack-base refusal never triggers closeout: closing out an
			// ancestor while the named change is not yet stacked-merged fails
			// its carry proof (children-retarget-required), so the only
			// closeout trigger is the waiting-dependency refusal.
			"A `not-ready-stack-base-unresolved` refusal is never a closeout trigger",
			// The status projection exposes the unmet set as the id list
			// `unmet_dependencies` (app.StatusChange), never a `depends_on`
			// field, so the closeout set is read off that real field.
			"take the named change's `unmet_dependencies` ids from its `changes[]` entry, keeping each id whose own `changes[]` entry has `status` `implemented`",
			// finalize.closeout success keys on the envelope `result`, and the
			// waiting-dependency refusal on the reason `pr-not-merged` — never
			// on the `disposition` token (FinalizeCloseout, CloseoutResult).
			"key success on the envelope `result` `applied` or `no-op`",
			"reason `pr-not-merged`",
			"if that single re-read still refuses",
		},
		absent: []string{
			"(or `not-ready-stack-base-unresolved` for a stacked change)",
			"its stack ancestors whose `status` is `implemented`",
			"the named change's `depends_on` ids whose `status` is `implemented`",
		}},
	// change 0448 — the convention's Composition paragraph carries the same
	// exemption: the step-0 preflight is selection-path only, and a single
	// explicit id goes directly to the authoritative explicit-id read. The
	// phrase is one bound clause, not floating vocabulary; the 0397 row above
	// still pins the surviving inline-operation sentence.
	{sentinel: "change_0448_named_preflight_skip", file: "skills/docket-convention/SKILL.md",
		present: []string{
			"an invocation naming exactly one explicit change id skips the preflight and goes directly to `context.implementation --id`",
		}},
	// change 0448 — docket-status names implement-next's Step 0 preflight
	// only as its selection-path step: a named (single-id) invocation skips
	// it, so the status prose must not describe it as unconditional.
	{sentinel: "change_0448_named_preflight_skip", file: "skills/docket-status/SKILL.md",
		present: []string{
			"which `docket-implement-next` runs inline at its Step 0 on its selection path (no id or an id set)",
			"(`docket-implement-next` Step 0 runs that operation inline on its selection path — no id or an id set)",
		},
		absent: []string{
			"runs inline at its Step 0 — not a mode of this skill",
			"(`docket-implement-next` Step 0 runs that operation inline)",
		}},
	// change 0459 — a handed-off worker's scope authority ends at the parent's
	// claim: the worker contract forbids acknowledging or reusing the claim-closed
	// scope, keys the outcome to the continuation's verdict, names the honest
	// BLOCKED for a missing fresh bundle, and classifies scope-transferred as a
	// misapplied-rule signal; the parent contract makes the continuation carry the
	// verdict, the closed-scope statement, and a fresh prepare-scope bundle.
	{sentinel: "change_0459_scope_transferred", file: "skills/docket-build-task/SKILL.md",
		present: []string{
			"never `acknowledge` the original scope and never start a drive on it",
			"A `scope-transferred` refusal means you misapplied this rule",
			"Report on the terminal verdict your continuation supplies",
			"never `COMPLETE` on that verdict",
			"\"continuation needs a fresh scope\"",
		}},
	{sentinel: "change_0459_scope_transferred", file: "skills/docket-build/SKILL.md",
		present: []string{
			"the claimed drive's id, its terminal verdict, and an explicit statement that the original scope is closed",
			"run `gate.drive.prepare-scope` again",
		}},
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

// docSectionContract binds one or more load-bearing clauses to BOTH a named
// section heading AND the heading that terminates it, so the assertion is "this
// clause is present inside THIS section" — not the weaker "somewhere in the
// file". Keying on the section/terminator heading pair is a syntactic-shape
// guard (the change 0323 uninstall/collection lifecycle contract lives in named
// sections), and the terminator must exist: a section with no closing heading
// would let a later paragraph satisfy the clause by accident.
type docSectionContract struct {
	change     string
	file       string   // slash path relative to repo root
	section    string   // exact heading line that opens the section
	terminator string   // exact heading line that must follow and closes the section
	present    []string // clauses required within [section, terminator), matched whitespace-collapsed
}

// change 0323 — the uninstall/version-collection lifecycle documented for users.
// Each clause is bound to its subject AND section, per the plan's Step 2: the
// retained CLI/repository setup after uninstall; the explicit retry command
// named only on a collection warning; and install/uninstall success recorded
// separately from cleanup completion.
var uninstallDocContracts = []docSectionContract{
	{change: "change_0323_uninstall_retention", file: "docs/install/install.md",
		section: "## Uninstalling docket", terminator: "## Reclaiming old version trees",
		present: []string{
			"the CLI binary, your global configuration, contributor checkouts, and each repository's docket setup all remain in place",
		}},
	{change: "change_0323_collection_retry", file: "docs/install/install.md",
		section: "## Reclaiming old version trees", terminator: "## Adopting docket in a repository",
		present: []string{
			"docket surfaces a `collection-pending` warning that lists the paths and the exact retry command, `docket install collect` — the only time you run collect by hand",
		}},
	{change: "change_0323_success_vs_cleanup", file: "docs/install/install.md",
		section: "## Reclaiming old version trees", terminator: "## Adopting docket in a repository",
		present: []string{
			"A cleanup warning never undoes the install or uninstall it followed: the primary operation is already recorded as successful, and only the version reclamation is left pending",
		}},
}

// scanDocSection is the whole detector, exposed so non_vacuity exercises the
// missing-section, missing-terminator, and missing-clause branches directly.
func scanDocSection(content string, c docSectionContract) []string {
	si := strings.Index(content, c.section)
	if si < 0 {
		return []string{fmt.Sprintf("%s: missing section heading %q", c.file, c.section)}
	}
	rest := content[si+len(c.section):]
	ti := strings.Index(rest, c.terminator)
	if ti < 0 {
		return []string{fmt.Sprintf("%s: section %q missing terminator %q", c.file, c.section, c.terminator)}
	}
	body := collapseWS(rest[:ti])
	var v []string
	for _, p := range c.present {
		if !strings.Contains(body, collapseWS(p)) {
			v = append(v, fmt.Sprintf("%s §%q: missing required clause %q", c.file, c.section, p))
		}
	}
	return v
}

func TestUninstallCollectionDocContracts(t *testing.T) {
	root := guardRoot(t)

	// Population floor: a collapse to zero means the table was gutted.
	checks := 0
	for _, c := range uninstallDocContracts {
		checks += len(c.present)
	}
	if checks < 3 {
		t.Fatalf("population floor: only %d uninstall/collection doc clauses (expected >= 3)", checks)
	}

	var violations []string
	cache := map[string]string{}
	for _, c := range uninstallDocContracts {
		content, ok := cache[c.file]
		if !ok {
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.file)))
			if err != nil {
				t.Fatalf("read contract file %s (%s): %v (fail closed)", c.file, c.change, err)
			}
			content = string(b)
			cache[c.file] = content
		}
		for _, msg := range scanDocSection(content, c) {
			violations = append(violations, fmt.Sprintf("[%s] %s", c.change, msg))
		}
	}
	if len(violations) != 0 {
		t.Errorf("uninstall/collection doc-contract violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		const doc = "intro\n## Alpha\nbody with a\nwrapped clause here\n## Beta\ntail"
		// A clause present within the section (matched across a wrap) is satisfied.
		if got := scanDocSection(doc, docSectionContract{file: "x.md", section: "## Alpha", terminator: "## Beta",
			present: []string{"body with a wrapped clause here"}}); len(got) != 0 {
			t.Errorf("scanDocSection flagged a satisfied wrapped clause: %v", got)
		}
		// A clause that only appears AFTER the terminator is not counted as in-section.
		if got := scanDocSection(doc, docSectionContract{file: "x.md", section: "## Alpha", terminator: "## Beta",
			present: []string{"tail"}}); len(got) != 1 {
			t.Errorf("scanDocSection matched a clause outside the section: %v", got)
		}
		// A missing section heading is a violation.
		if got := scanDocSection(doc, docSectionContract{file: "x.md", section: "## Gamma", terminator: "## Beta"}); len(got) != 1 {
			t.Errorf("scanDocSection missed an absent section heading: %v", got)
		}
		// A missing terminator heading is a violation.
		if got := scanDocSection(doc, docSectionContract{file: "x.md", section: "## Alpha", terminator: "## Omega"}); len(got) != 1 {
			t.Errorf("scanDocSection missed an absent terminator heading: %v", got)
		}
	})
}

// change 0411 — the reconciliation-write recovery exception: a post-completion
// durable-write failure re-enters finalize.rebase-continue with the same inputs
// and is never routed to rebase-abort. Each clause is bound to its owning
// section (prose-guard-binds-phrase-to-claim) and matched whitespace-collapsed
// (phrase-grep-over-wrapped-prose), so a pure re-flow stays green while removing
// the exception, or substituting abort as the remedy, goes red.
var rebaseRecoveryDocContracts = []docSectionContract{
	{change: "change_0411_recovery_exception_skill", file: "skills/docket-finalize-change/SKILL.md",
		section: "### 3. Rebase onto the effective base (resolver loop)", terminator: "### 4. The local gate",
		present: []string{
			"re-run `finalize.rebase-continue` with the same `--id <id> --attempt <attempt> --input <report>`",
			"never route this persistence failure to `finalize.rebase-abort`",
		}},
	{change: "change_0411_recovery_exception_reference", file: "skills/docket-finalize-change/references/gate-failure.md",
		section: "## The reconciliation-write exception (recover, not abort)", terminator: "## The finalize gate shares the worktree's one execution slot",
		present: []string{
			"Preserve the workspace, the receipt, and the original resolver report",
			"Re-run `finalize.rebase-continue` with the same `--id <id> --attempt <attempt> --input <report>`",
			"an operator remedy, not an autonomous retry loop",
			"resumes via the original identical `finalize.rebase` invocation",
		}},
	{change: "change_0411_recovery_not_in_abort_set", file: "skills/docket-finalize-change/references/gate-failure.md",
		section: "## abort-and-report points (the full set)", terminator: "## The reconciliation-write exception (recover, not abort)",
		present: []string{
			"a reservation-reconciliation write failure after Git advanced or completed the owned continuation is **not** in this set",
		}},
}

// TestRebaseRecoveryDocContracts binds the change 0411 recovery-exception
// clauses to their sections in both finalize documents; scanDocSection's
// missing-section / missing-terminator / missing-clause branches are exercised
// by TestUninstallCollectionDocContracts' non_vacuity subtest.
func TestRebaseRecoveryDocContracts(t *testing.T) {
	root := guardRoot(t)

	// Population floor: a collapse to zero means the table was gutted.
	checks := 0
	for _, c := range rebaseRecoveryDocContracts {
		checks += len(c.present)
	}
	if checks < 7 {
		t.Fatalf("population floor: only %d recovery-exception doc clauses (expected >= 7)", checks)
	}

	cache := map[string]string{}
	var violations []string
	for _, c := range rebaseRecoveryDocContracts {
		content, ok := cache[c.file]
		if !ok {
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.file)))
			if err != nil {
				t.Fatalf("read contract file %s (%s): %v (fail closed)", c.file, c.change, err)
			}
			content = string(b)
			cache[c.file] = content
		}
		for _, msg := range scanDocSection(content, c) {
			violations = append(violations, fmt.Sprintf("[%s] %s", c.change, msg))
		}
	}
	if len(violations) != 0 {
		t.Errorf("recovery-exception doc contracts (%d violations):\n%s", len(violations), strings.Join(violations, "\n"))
	}
}
