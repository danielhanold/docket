package repoguard

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
		present: []string{"The consultant is the `docket-brainstorm-consultant` agent"}},
	// tests/test_convention_extraction.sh — operating skills carry the load-first line
	// and never copy the convention (the begin marker is a copy tell).
	{sentinel: "test_convention_extraction", file: "skills/docket-implement-next/SKILL.md",
		present: []string{"## Convention (load first — blocking)"},
		absent:  []string{"<!-- docket:convention:begin -->"}},
	// tests/test_critic_return_channel.sh — the critic's return-channel contract.
	{sentinel: "test_critic_return_channel", file: "agents/docket-auto-groom-critic.md",
		present: []string{"adversarial critic", "not registered under its skill name"}},
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
	// tests/test_cursor_contract_docs.sh — cursor validation PR-handoff obligation
	// (moved to the harness reference by change 0402).
	{sentinel: "test_cursor_contract_docs", file: "docs/reference/harness/validation.md",
		present: []string{"## The PR-handoff obligation"}},
	// tests/test_cursor_permissions_docs.sh — the permissions guidance survives on the
	// Cursor install page: the inline terminalAllowlist fragment, and a link to the
	// Cursor validation checklist in the harness reference.
	{sentinel: "test_cursor_permissions_docs", file: "docs/install/cursor.md",
		present: []string{"\"terminalAllowlist\": [", "](../reference/harness/"}},
	// tests/test_docket_build.sh — the per-task worker contract.
	{sentinel: "test_docket_build", file: "skills/docket-build-task/SKILL.md",
		present: []string{"self-review is part of", "Implement only that task"}},
	// tests/test_docket_review.sh — the review role contract.
	{sentinel: "test_docket_review", file: "skills/docket-review/SKILL.md",
		present: []string{"build-evidence", "abort-and-report"}},
	// tests/test_gate_caller_loop.sh — the gate driver caller-loop reference.
	{sentinel: "test_gate_caller_loop", file: "skills/docket-build/references/gate-driver.md",
		present: []string{"## The disposition vocabulary", "## Handoff"}},
	// tests/test_dispatch_capability.sh — the convention's dispatch-capability rule.
	{sentinel: "test_dispatch_capability", file: "skills/docket-convention/SKILL.md",
		present: []string{"Dispatch-capability resolution", "never from a tool name"}},
	// tests/test_docket_metadata_branch.sh — deferred-publish prose + retired-route absence.
	{sentinel: "test_docket_metadata_branch", file: "skills/docket-finalize-change/SKILL.md",
		absent: []string{"checkout origin/docket"}},
	// tests/test_docket_example_yml.sh — key-presence core (full correspondence scan flagged).
	{sentinel: "test_docket_example_yml", file: ".docket.example.yml",
		present: []string{"board_surfaces", "agent_harnesses", "finalize:"}},
	// change 0400 — the goal-first landing page cannot silently lose its two
	// load-bearing map links (the docs index — retargeted from the relocated
	// guide by change 0402 — and the comparison page).
	{sentinel: "change_0400_readme_landing", file: "README.md",
		present: []string{"](docs/README.md)", "](docs/comparison/ai-native-sdlc-playbook.md)"}},
	// change 0382 — both draft-time scalars ride in the change.create request;
	// the post-create plain-git frontmatter edit and the skill-owned
	// normalization rules are gone (normalization is domain.NormalizeBranchPrefix).
	{sentinel: "change_0382_typed_create_scalars", file: "skills/docket-new-change/SKILL.md",
		present: []string{"`invalid-branch_prefix`"},
		absent:  []string{"Two draft-time scalars `create` does not carry", "strip one presentation-only trailing slash"}},
	// change 0382 — the auto-groom abstain and the human re-enable are typed
	// change.groom outcomes that re-render the board in the same commit; the
	// plain-git abstain, its false "no board-visible cell" claim, and the
	// hand-edit re-enable are gone.
	{sentinel: "change_0382_typed_abstain", file: "skills/docket-auto-groom/SKILL.md",
		present: []string{"`outcome: abstain`", "`blocked_note`"},
		absent:  []string{"changes no board-visible cell", "there is no typed groom for this outcome"}},
	{sentinel: "change_0382_typed_abstain", file: "skills/docket-convention/SKILL.md",
		present: []string{"`outcome: abstain`", "`outcome: re-enable`"},
		absent:  []string{"flips the flag back to `true`, and DELETES"}},
	{sentinel: "change_0382_typed_abstain", file: "skills/docket-groom-next/SKILL.md",
		present: []string{"`outcome: re-enable`", "`nothing-to-re-enable`"}},
	// change 0461 — change.groom carries an optional title; a retitle renames
	// nothing (slug, record path, spec path, and branch stay put).
	{sentinel: "change_0461_retitle", file: "skills/docket-groom-next/SKILL.md",
		present: []string{"also carry `title`", "a `title` alone is a valid revise", "a retitle renames nothing", "`not-retitleable`"}},
	// change 0509 — revise edits any proposed change without changing its groom
	// state; a request to edit (not groom) a needs-grooming stub goes straight
	// to the revise exit with no brainstorm. The absent phrases are the retired
	// already-groomed-only claims.
	{sentinel: "change_0509_stub_revise", file: "skills/docket-groom-next/SKILL.md",
		present: []string{"skip the brainstorm and apply Step 4's revise exit directly", "the stub stays needs-grooming",
			"The change keeps its groom state."},
		absent: []string{"The change stays build-ready.", "the change is already groomed and the human wants it adjusted",
			"no longer an already-groomed `proposed` change", "a revise keeps the row build-ready"}},
	// change 0389 — implementation-scope sweep + the two completion barriers.
	// docket-status owns the COMMAND barrier: a backgrounded sweep is observed
	// to its terminal envelope, never declared done by proxy signals; and an
	// applied envelope is never read as all-items-succeeded.
	{sentinel: "change_0389_sweep_scope", file: "skills/docket-status/SKILL.md",
		present: []string{"--scope implementation", "a move to background, not completion",
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
	// change 0407 — implement-next's gated claim carries its run context so
	// the parent's keyed verdict resolves ownership from durable proof, and an
	// invalid/conflicting context fails closed rather than degrading to an
	// ungated claim.
	{sentinel: "change_0407_run_context_claim", file: "skills/docket-implement-next/SKILL.md",
		present: []string{
			"pass it to the claim as --run-context",
			"an invalid or conflicting run context is a typed refusal that writes nothing — never retried as an ungated claim",
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
			"required close-out artifacts (one per implemented change, trivial included)",
			"required close-out artifact for every implemented change, trivial included.",
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
	// change 0480 — finalize cleanup retains a killed change's resources and
	// reports no-op / retained / killed-retained (FinalizeCleanup's
	// domain.StatusKilled case). The absent phrases are the retired claims that
	// cleanup prunes or processes a killed change's worktree/branch
	// (assert-detects-removal). Each phrase sits on one physical line: this
	// detector is a raw strings.Contains, so a re-wrap that splits a phrase
	// reddens the row rather than passing silently.
	{sentinel: "change_0480_killed_cleanup_retained", file: "skills/docket-convention/references/close-out.md",
		present: []string{
			"reason `killed-retained`",
			"a success, not a failure",
		},
		absent: []string{
			"so a kill leg whose",
			"prunes any feature worktree",
		}},
	{sentinel: "change_0480_killed_cleanup_retained", file: "skills/docket-implement-next/references/edge-paths.md",
		present: []string{
			"The cleanup step retains a killed change's worktree/branch (`killed-retained`, a `no-op`).",
		},
		absent: []string{
			"cleanup step prunes any feature worktree",
			"prunes any feature worktree",
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
	absent     []string // retired clauses that must appear NOWHERE in the file, matched whitespace-collapsed
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

// change 0488 — build workers run every focused test under GNU `timeout`, so the
// install prerequisites name GNU coreutils and the macOS `gtimeout` spelling.
var prerequisiteDocContracts = []docSectionContract{
	{change: "change_0488_coreutils_prerequisite", file: "docs/install/install.md",
		section: "## What you need first", terminator: "## Install docket on your machine",
		present: []string{
			"**GNU coreutils `timeout`.** Build workers run each focused test under `timeout --kill-after=10s 10m`.",
			"on macOS, run `brew install coreutils` (it may install as `gtimeout`, which workers also accept)",
		}},
}

func TestInstallPrerequisiteDocContracts(t *testing.T) {
	root := guardRoot(t)
	for _, c := range prerequisiteDocContracts {
		for _, v := range scanDocSection(readMaintained(t, root, c.file), c) {
			t.Errorf("[%s] %s", c.change, v)
		}
	}
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
	whole := collapseWS(content)
	for _, a := range c.absent {
		if strings.Contains(whole, collapseWS(a)) {
			v = append(v, fmt.Sprintf("%s: retired clause is present: %q", c.file, a))
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
		section: "### 3. Rebase onto the effective base (resolver rounds)", terminator: "### 4. The local gate",
		present: []string{
			"re-run `finalize.rebase-continue` with the same `--id <id> --attempt <attempt> --input <report>`",
			"never route this persistence failure to `finalize.rebase-abort`",
		}},
	{change: "change_0411_recovery_exception_reference", file: "skills/docket-finalize-change/references/gate-failure.md",
		section: "## The reconciliation-write exception (recover, not abort)", terminator: "## The finalize gate shares the worktree's one lock",
		present: []string{
			"Preserve the workspace, the receipt, and the original resolver report",
			"Re-run `finalize.rebase-continue` with the same `--id <id> --attempt <attempt> --input <report>`",
			"an operator remedy, not an autonomous retry",
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

// change 0498 — whole-branch review outcomes get a named home in the results
// file. The PR body keeps the full disposition table; the final results carry a
// one-line review summary under Verification performed and Known issues entries
// for unfixed/reported findings; Human actions and testing holds only human
// work. Each clause is bound to its section (prose-guard-binds-phrase-to-claim)
// and matched whitespace-collapsed (phrase-grep-over-wrapped-prose). The absent
// clause is the retired fix-loop sentence that gave review findings no home; it
// WRAPS in the source, so only the collapsed match can see it
// (assert-detects-removal-not-replacement). Mutation-tested at introduction.
var resultsReviewPlacementDocContracts = []docSectionContract{
	{change: "change_0498_template_review_summary", file: "skills/docket-implement-next/results-template.md",
		section: "## Verification performed", terminator: "## Known issues and follow-ups",
		present: []string{
			"Give the whole-branch review one line: which review ran (the tier, or an inline review) and how its findings ended",
			"full table in the PR body",
			"A fixed finding with no remaining risk appears in the results only through this line",
		}},
	{change: "change_0498_fix_loop_condensation", file: "skills/docket-implement-next/references/fix-pass.md",
		section: "## Recording — the PR-body disposition table", terminator: "## Beyond-the-branch findings are reported",
		present: []string{
			"the **PR body remains the disposition table's durable home**",
			"During the build the results file may hold the full returned findings, their evidence, and their impact, so they survive a halt before the PR exists",
			"final consolidation condenses them to a one-line review summary",
		},
		absent: []string{
			"The results file preserves the findings, their evidence, and their impact for the human",
		}},
	{change: "change_0498_step65_review_homes", file: "skills/docket-implement-next/SKILL.md",
		section: "### Step 6.5 — Results (required)", terminator: "### Step 7 — PR + stop",
		present: []string{
			"The section holds only what a human should do or check — never a record of what the run already checked, which belongs under Verification performed",
			"a checkpoint updates it, never truncates it — except final consolidation, which condenses the review findings",
			"**Review findings.** The PR body is the full review disposition table's home",
			"Checkpoint (ii) persists the returned findings — in full detail if useful — so they survive a halt before the PR exists; final consolidation condenses them",
			"`## Verification performed` then carries one line naming which review ran",
			"an entry for every finding not fixed (`deferred`, `reverted`, or `recorded`) and every `reported` beyond-the-branch finding",
			"A fixed finding with no remaining risk appears in final results only through that summary line",
		}},
}

// TestResultsReviewPlacementDocContracts binds the change 0498 review-placement
// clauses to their sections; scanDocSection's missing-section / -terminator /
// -clause branches are exercised by TestUninstallCollectionDocContracts'
// non_vacuity subtest, and the absent branch by this test's own.
func TestResultsReviewPlacementDocContracts(t *testing.T) {
	root := guardRoot(t)

	// Population floor: a collapse means rows were lost or the table was gutted.
	checks := 0
	for _, c := range resultsReviewPlacementDocContracts {
		checks += len(c.present) + len(c.absent)
	}
	if checks < 14 {
		t.Fatalf("population floor: only %d review-placement doc clauses (expected >= 14)", checks)
	}

	var violations []string
	cache := map[string]string{}
	for _, c := range resultsReviewPlacementDocContracts {
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
		t.Errorf("review-placement doc-contract violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		const doc = "intro\n## Alpha\nbody with a\nwrapped clause here\n## Beta\ntail"
		sec := docSectionContract{file: "x.md", section: "## Alpha", terminator: "## Beta"}
		// A retired clause that WRAPS in the source is still detected.
		c := sec
		c.absent = []string{"with a wrapped clause"}
		if got := scanDocSection(doc, c); len(got) != 1 {
			t.Errorf("scanDocSection missed a wrapped absent clause: %v", got)
		}
		// A retired clause OUTSIDE the section is still detected (file-wide).
		c = sec
		c.absent = []string{"tail"}
		if got := scanDocSection(doc, c); len(got) != 1 {
			t.Errorf("scanDocSection missed an absent clause outside the section: %v", got)
		}
		// A clause that is genuinely gone produces no violation.
		c = sec
		c.absent = []string{"never written"}
		if got := scanDocSection(doc, c); len(got) != 0 {
			t.Errorf("scanDocSection flagged an absent clause that is not there: %v", got)
		}
	})
}

// change 0510 — at final consolidation implement-next matches every reported
// out-of-scope follow-up against the backlog's proposed and deferred changes
// and records one verdict plus a state-matched next action, in the results file
// and the final report. Present clauses bind each claim inside its section and
// are matched whitespace-collapsed (phrase-grep-over-wrapped-prose,
// prose-guard-binds-phrase-to-claim). Absent clauses are the retired
// "link an existing change when one is known" family, so restoring the old
// wording reddens (assert-detects-removal-not-replacement). Mutation-tested at
// introduction.
var followUpBacklogMatchDocContracts = []docSectionContract{
	{change: "change_0510_step65_backlog_match", file: "skills/docket-implement-next/SKILL.md",
		section: "### Step 6.5 — Results (required)", terminator: "### Step 7 — PR + stop",
		present: []string{
			"final consolidation checks each against the backlog and names any match",
			"**Backlog match.** Once, at checkpoint (iv) final consolidation",
			"an in-scope limitation or a verification-coverage note gets no verdict",
			"keep the changes whose `status` is `proposed` or `deferred`, excluding this change",
			"For **every** candidate, not only those with similar titles, read the `## Why` and `## What changes` sections",
			"**Fits #N**",
			"**Related to #N**",
			"**No existing change fits (checked K)**",
			"a groomed `proposed` #N — edit it through `docket-groom-next <N>` (revise), which also revises its spec when needed",
			"a `deferred` #N — revive it (`change.revive`), then edit it as a proposed change",
			"related or no fit — a new change a human captures (Step 3), linking any related #N under `related:`",
			"The final report's follow-up list carries the same verdict per item",
			"The match only recommends: it never edits, creates, revives, defers, or kills any change",
			"never halt, retry in a loop, or block the implemented transition on it",
		},
		absent: []string{"link an existing change when one is known"}},
	{change: "change_0510_step3_verdict_pointer", file: "skills/docket-implement-next/SKILL.md",
		section: "### Step 3 — Reconcile ⭐", terminator: "### Step 4 — Worktree + plan",
		present: []string{
			"nothing is minted automatically",
			"guided by the backlog verdict final consolidation attaches (Step 6.5 *Backlog match*)",
		}},
	{change: "change_0510_step6_verdict_pointer", file: "skills/docket-implement-next/SKILL.md",
		section: "### Step 6 — Review + ADRs", terminator: "### Step 6.5 — Results (required)",
		present: []string{
			"reported as follow-up work in the final report, carrying the backlog verdict final consolidation attaches",
		}},
	{change: "change_0510_final_report_verdict", file: "skills/docket-implement-next/SKILL.md",
		section: "### Terminal disposition (driver contract)", terminator: "### Atomic board rendering",
		present: []string{
			"any follow-up work **reported for deliberate capture**, each with its backlog verdict when final consolidation produced one (Step 6.5 *Backlog match*)",
		}},
	{change: "change_0510_convention_backlog_match", file: "skills/docket-convention/SKILL.md",
		section: "### Directory layout (paths relative to the configured knobs)", terminator: "### Change manifest (frontmatter at the top of each change file)",
		present: []string{
			"the run checks proposed and deferred changes and names any match, but never mints a change, issue, ADR, or learning automatically",
		},
		absent: []string{"link an existing change when one is known"}},
}

// docFileClauseContract pins clauses that live in a file's LAST section, which
// has no closing heading for scanDocSection to key on. The file itself is the
// section, so present and absent are both matched file-wide, whitespace-collapsed.
type docFileClauseContract struct {
	change  string
	file    string   // slash path relative to repo root
	present []string // clauses required anywhere in the file
	absent  []string // retired clauses that must appear nowhere in the file
}

var followUpBacklogMatchFileClauses = []docFileClauseContract{
	{change: "change_0510_fix_loop_verdict_pointer", file: "skills/docket-implement-next/references/fix-pass.md",
		present: []string{
			"never minted: a human captures reported work deliberately with `docket change create`.",
			"The report carries the backlog verdict final consolidation attaches (SKILL.md Step 6.5 *Backlog match*)",
		}},
	{change: "change_0510_template_verdict", file: "skills/docket-implement-next/results-template.md",
		present: []string{
			"end that action with the backlog verdict — Fits #N, Related to #N, or No existing change fits (checked K) — and the next step for #N's state",
		},
		absent: []string{"linking an existing change when available"}},
}

// followUpBacklogMatchFloor is the population floor over both tables: a
// collapse means rows were lost or the tables were gutted.
const followUpBacklogMatchFloor = 20

func TestFollowUpBacklogMatchDocContracts(t *testing.T) {
	root := guardRoot(t)

	checks := 0
	for _, c := range followUpBacklogMatchDocContracts {
		checks += len(c.present) + len(c.absent)
	}
	for _, c := range followUpBacklogMatchFileClauses {
		checks += len(c.present) + len(c.absent)
	}
	if checks < followUpBacklogMatchFloor {
		t.Fatalf("population floor: only %d backlog-match doc clauses (expected >= %d)", checks, followUpBacklogMatchFloor)
	}

	cache := map[string]string{}
	read := func(rel, change string) string {
		if s, ok := cache[rel]; ok {
			return s
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read contract file %s (%s): %v (fail closed)", rel, change, err)
		}
		cache[rel] = string(b)
		return cache[rel]
	}

	var violations []string
	for _, c := range followUpBacklogMatchDocContracts {
		for _, msg := range scanDocSection(read(c.file, c.change), c) {
			violations = append(violations, fmt.Sprintf("[%s] %s", c.change, msg))
		}
	}
	for _, c := range followUpBacklogMatchFileClauses {
		whole := collapseWS(read(c.file, c.change))
		for _, p := range c.present {
			if !strings.Contains(whole, collapseWS(p)) {
				violations = append(violations, fmt.Sprintf("[%s] %s: missing required clause %q", c.change, c.file, p))
			}
		}
		for _, a := range c.absent {
			if strings.Contains(whole, collapseWS(a)) {
				violations = append(violations, fmt.Sprintf("[%s] %s: retired clause is present: %q", c.change, c.file, a))
			}
		}
	}
	if len(violations) != 0 {
		t.Errorf("backlog-match doc-contract violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}
}

// change 0488 — build-task workers run every test directly under a fixed
// 10-minute GNU timeout, read the result from the exit status, and report each
// command exactly as run; the build controller's time-limit audit reports a
// missing wrapper and is never a halting condition. Each clause is matched
// against a backtick-and-emphasis-stripped, whitespace-collapsed haystack
// (phrase-grep-over-wrapped-prose) and bound to its claim with one bounded
// gap (prose-guard-binds-phrase-to-claim).
var timeLimitContracts = []struct {
	name string
	file string
	re   *regexp.Regexp
}{
	{"worker-runs-under-timeout", buildTaskSkillRel, regexp.MustCompile(`timeout --kill-after=10s 10m <test command>`)},
	{"worker-limit-hit-is-not-red", buildTaskSkillRel, regexp.MustCompile(`\| 124 or 137 \| the 10-minute limit was hit[^|]{0,60}\| not red`)},
	{"worker-never-unlimited", buildTaskSkillRel, regexp.MustCompile(`Never run a test without the limit`)},
	{"worker-verification-as-run", buildTaskSkillRel, regexp.MustCompile(`VERIFICATION lists every test command exactly as it ran[^.]{0,60}timeout wrapper`)},
	{"controller-audits-wrapper", buildSkillRel, regexp.MustCompile(`Time-limit audit.{0,160}timeout --kill-after=10s 10m`)},
	{"controller-audit-never-halts", buildSkillRel, regexp.MustCompile(`The audit is never a halting condition and never makes a return malformed`)},
	// Review-focus pins (plan): the inputs the spec implies but no other test exercises.
	{"worker-gtimeout-fallback", buildTaskSkillRel, regexp.MustCompile(`Use gtimeout when timeout is not on PATH\. If neither exists, return BLOCKED naming the missing prerequisite \(GNU coreutils\)`)},
	{"worker-harness-wait", buildTaskSkillRel, regexp.MustCompile(`Raise the harness's own shell-call timeout to at least 10 minutes`)},
	{"worker-never-full-suite", buildTaskSkillRel, regexp.MustCompile(`Never run the configured full-suite command\. The full suite is the controller's gate`)},
	{"worker-mutation-restore", buildTaskSkillRel, regexp.MustCompile(`verify it turns red, then restore and run it again`)},
	{"controller-unknown-outcome-halts", buildSkillRel, regexp.MustCompile(`Valid outcomes are COMPLETE, NEEDS_ESCALATION, and BLOCKED; any other token, or a missing or malformed outcome, halts the build`)},
	{"repair-never-full-suite", buildSkillRel, regexp.MustCompile(`re-runs the failing tests directly as its focused check, and commits; it never runs the full suite`)},
}

// timeLimitHaystack strips inline-code and emphasis markers and collapses
// whitespace, so a re-flow or a re-emphasis never reddens a clause.
func timeLimitHaystack(content string) string {
	return collapseWS(strings.NewReplacer("`", "", "*", "").Replace(content))
}

func TestTaskTestTimeLimitContract(t *testing.T) {
	root := guardRoot(t)
	for _, c := range timeLimitContracts {
		if !c.re.MatchString(timeLimitHaystack(readMaintained(t, root, c.file))) {
			t.Errorf("%s lost its %s clause (pattern %v)", c.file, c.name, c.re)
		}
	}

	t.Run("non_vacuity", func(t *testing.T) {
		good := map[string]string{
			"worker-runs-under-timeout":        "```text\ntimeout --kill-after=10s 10m <test command>\n```",
			"worker-limit-hit-is-not-red":      "| `124` or `137` | the 10-minute limit was hit (TERM, or KILL after the\n10-second grace) | not red, and not by itself a reason to escalate |",
			"worker-never-unlimited":           "If neither exists, return `BLOCKED`. Never run a\ntest without the limit.",
			"worker-verification-as-run":       "`VERIFICATION` lists every test command exactly as it ran,\nincluding its `timeout` wrapper, with its exit status.",
			"controller-audits-wrapper":        "**Time-limit audit — visibility only.** For every return, check each test command in\n`VERIFICATION` for the `timeout --kill-after=10s 10m` wrapper (or `gtimeout`).",
			"controller-audit-never-halts":     "- The audit is never a halting\n  condition and never makes a return malformed.",
			"worker-gtimeout-fallback":         "- **Fallback.** Use `gtimeout` when `timeout` is not on `PATH`. If neither exists, return\n  `BLOCKED` naming the missing prerequisite (GNU coreutils).",
			"worker-harness-wait":              "Raise the harness's own shell-call timeout to\n  at least 10 minutes where the harness allows it",
			"worker-never-full-suite":          "- **Never run the configured full-suite command.** The full suite is the controller's gate.",
			"worker-mutation-restore":          "run the guard and verify it turns red, then restore and\n  run it again.",
			"controller-unknown-outcome-halts": "Valid outcomes are `COMPLETE`, `NEEDS_ESCALATION`, and `BLOCKED`; any other token, or a **missing or\nmalformed outcome, halts** the build.",
			"repair-never-full-suite":          "fixes it, re-runs the failing tests\n   directly as its focused check, and commits; it never runs the full suite, and",
		}
		bad := map[string]string{
			"worker-runs-under-timeout":        "run <test command> directly",
			"worker-limit-hit-is-not-red":      "| `124` or `137` | the 10-minute limit was hit | red |",
			"worker-never-unlimited":           "Run a test without the limit when timeout is missing.",
			"worker-verification-as-run":       "`VERIFICATION` lists the focused command. The timeout wrapper is optional.",
			"controller-audits-wrapper":        "**Time-limit audit — visibility only.** For every return, read `VERIFICATION`.",
			"controller-audit-never-halts":     "- The audit is a halting condition when the wrapper is missing.",
			"worker-gtimeout-fallback":         "- **Fallback.** Use `gtimeout` when `timeout` is not on `PATH`. If neither exists, run the test anyway.",
			"worker-harness-wait":              "Keep the harness's default shell-call timeout.",
			"worker-never-full-suite":          "- Run the configured full-suite command when unsure.",
			"worker-mutation-restore":          "run the guard and verify it turns red.",
			"controller-unknown-outcome-halts": "Valid outcomes are `COMPLETE`, `WAITING`, `NEEDS_ESCALATION`, and `BLOCKED`; a missing outcome halts the build.",
			"repair-never-full-suite":          "fixes it, re-runs the full suite, and commits",
		}
		for _, c := range timeLimitContracts {
			if !c.re.MatchString(timeLimitHaystack(good[c.name])) {
				t.Errorf("%s: the intended (wrapped) wording did not match", c.name)
			}
			if c.re.MatchString(timeLimitHaystack(bad[c.name])) {
				t.Errorf("%s: a wording that drops the claim still matched", c.name)
			}
		}
	})
}

// alignmentContracts are the agent-facing alignment rows: each phrase is bound
// to one skill or agent file and matched whitespace-collapsed (collapseWS) on
// both sides, so a re-flow never reddens a present phrase and a wrapped retired
// phrase is still caught (phrase-grep-over-wrapped-prose).
var alignmentContracts = []proseContract{
	// Dummy mode is not a docket feature; no skill describes it.
	{sentinel: "align_0502_no_dummy_mode", file: "skills/docket-convention/SKILL.md",
		absent: []string{"Dummy mode", "DUMMY_MODE", "In plain terms"}},
	// 0502: the convention's record blocks mirror render.ChangeRecord / render.ADRRecord.
	{sentinel: "align_0502_record_blocks", file: "skills/docket-convention/SKILL.md",
		present: []string{"branch_prefix:", "## Alternatives considered"},
		absent:  []string{"Seeded empty by the template"}},
	// 0502: roles are fixed; no auto sentinel, no rebinding, halt posture.
	{sentinel: "align_0502_fixed_roles", file: "skills/docket-convention/SKILL.md",
		present: []string{"never outranks", "DIRECTED to:", "**`halt`**"},
		absent:  []string{"auto-or-halt", "`auto` sentinel", "SKILL_BRAINSTORM", "Passthrough."}},
	{sentinel: "align_0502_fixed_roles", file: "skills/docket-build/SKILL.md",
		absent: []string{"auto-or-halt", "skills.build", "skills.review"}},
	{sentinel: "align_0502_fixed_roles", file: "skills/docket-review/SKILL.md",
		absent: []string{"skills.review"}},
	{sentinel: "align_0502_fixed_roles", file: "skills/docket-brainstorm/SKILL.md",
		absent: []string{"skills.brainstorm", "skills: brainstorm:", "the 0049", "0049 passthrough"}},
	{sentinel: "align_0502_fixed_roles", file: "skills/docket-implement-next/references/fix-pass.md",
		absent: []string{"skills.build: auto"}},
	// 0502: the convention's configuration contract matches the schema.
	{sentinel: "align_0502_config_contract", file: "skills/docket-convention/SKILL.md",
		present: []string{"reads the committed `.docket.yml` from the fetched tip of origin's default branch, never the working tree",
			"`.docket.local.yml` and the global `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml` are read from disk",
			"resolve every layer from the primary worktree's files on disk",
			"an unresolvable remote HEAD is an error, not a fallback to `main`", "# local | off", "max_attempts: 4"},
		absent: []string{"never from `origin/HEAD`", "committed in the primary worktree",
			"fallback main", "repair `origin/HEAD`", "`ci` polls GitHub checks",
			"skip_results_only_delta", "agents.yaml", "terminal_publish:", "auto_capture:", "auto_groom:"}},
	{sentinel: "align_0502_config_contract", file: "skills/docket-implement-next/references/edge-paths.md",
		absent: []string{"skip_results_only_delta"}},
	{sentinel: "align_0502_config_contract", file: "skills/docket-convention/references/agent-layer.md",
		absent: []string{"agents.yaml"}},
	// 0502: no skill reads a value the prepared context does not carry.
	{sentinel: "align_0502_no_phantom_exports", file: "skills/docket-convention/SKILL.md",
		present: []string{"reported in the run's final report"},
		absent:  []string{"AUTO_CAPTURE", "auto-capture deferred"}},
	{sentinel: "align_0502_no_phantom_exports", file: "skills/docket-build/SKILL.md",
		absent: []string{"GATE_OBSERVATION_BUDGET", "BUILD_CHECKPOINT"}},
	{sentinel: "align_0502_no_phantom_exports", file: "skills/docket-implement-next/SKILL.md",
		present: []string{"`review.min_fix_severity`"},
		absent:  []string{"REVIEW_MIN_FIX_SEVERITY", "REVIEW_MAX_FIX_TASKS"}},
	{sentinel: "align_0502_no_phantom_exports", file: "skills/docket-implement-next/references/fix-pass.md",
		present: []string{"`diagnostic.config`"},
		absent:  []string{"REVIEW_MIN_FIX_SEVERITY", "REVIEW_MAX_FIX_TASKS"}},
	// 0502: no terminal publish, no learnings cap, no repository auto_groom default.
	{sentinel: "align_0502_publish_learnings_groom", file: "skills/docket-convention/SKILL.md",
		present: []string{"`learning.record`", "`learning.update`", "### Learnings ledger", "will the agent know to search for this?"},
		absent:  []string{"human curation only", "terminal publish", "Terminal publication", "learnings.cap", "repo's `auto_groom`"}},
	{sentinel: "align_0502_publish_learnings_groom", file: "skills/docket-convention/references/close-out.md",
		absent: []string{"terminal publish", "Terminal publication", "terminal_publish"}},
	{sentinel: "align_0502_publish_learnings_groom", file: "skills/docket-convention/references/learnings.md",
		present: []string{"`learning.record`"},
		absent:  []string{"learnings.cap", "## Capacity", "human curation"}},
	{sentinel: "align_0502_publish_learnings_groom", file: "skills/docket-adr/SKILL.md",
		absent: []string{"(deferred)", "terminal publish"}},
	// 0502: the docket-review skill never dispatches (its own "never dispatches"
	// rule); the review tier fan-out is the controller's own dispatch.
	{sentinel: "align_0502_review_tier_dispatch", file: "skills/docket-implement-next/SKILL.md",
		present: []string{"you — the controller — dispatch the selected tier wrapper"},
		absent:  []string{"it dispatches the selected tier", "the skill dispatches the selected tier"}},
	{sentinel: "align_0502_review_tier_dispatch", file: "skills/docket-convention/SKILL.md",
		present: []string{"the controller's own review-tier dispatch"},
		absent:  []string{"`review` role skills' required nested dispatches"}},
	// 0502: typed operations replace the Bash-era names.
	{sentinel: "align_0502_typed_ops", file: "skills/docket-convention/SKILL.md",
		present: []string{"`stack-base-unresolved`", "`change.resume-halted`"},
		absent:  []string{"stack-base.sh", "disable-worktree-hooks", "verify-run", "scripts/<name>.md"}},
	{sentinel: "align_0502_typed_ops", file: "skills/docket-convention/references/stacked-changes.md",
		present: []string{"`workspace.prepare`", "`finalize.retarget-children`", "`change-stack-cycle`"},
		absent:  []string{"verify-run", "fm_field", "git worktree add", "gh pr edit", "stack-invalid", "stack-parent-killed"}},
	{sentinel: "align_0502_typed_ops", file: "skills/docket-convention/references/close-out.md",
		absent: []string{"scripts/<name>.md", "board off", "promote-failed", "stack-carried-failed"}},
	{sentinel: "align_0502_typed_ops", file: "skills/docket-implement-next/SKILL.md",
		absent: []string{"reclaim-claims", "does not clear an interrupted run's marker"}},
	{sentinel: "align_0502_typed_ops", file: "skills/docket-implement-next/references/edge-paths.md",
		absent: []string{"verify-run"}},
	// 0502: status and adr name only findings the binary emits.
	{sentinel: "align_0502_real_findings", file: "skills/docket-status/SKILL.md",
		present: []string{"`waiting-dependency`", "`artifact-missing`"},
		absent: []string{"publish-deferred", "adr-unpublished", "dependency stalls", "stale claims,",
			"board off", "stack-invalid", "stack-parent-killed", "promote-failed", "stack-carried-failed"}},
	{sentinel: "align_0502_real_findings", file: "skills/docket-adr/SKILL.md",
		absent: []string{"adr-unpublished", "publish-deferred"}},
	{sentinel: "align_0502_real_findings", file: "agents/docket-status.md",
		absent: []string{"dependency stalls"}},
	// 0502: wrappers are user-level; pins are global-only.
	{sentinel: "align_0502_agent_layer", file: "skills/docket-convention/references/agent-layer.md",
		present: []string{"from the global configuration only", "installed at user level"},
		absent:  []string{"agents.yaml", "drift-check gate", "per-repo agent pass", "change 0"}},
	{sentinel: "align_0502_agent_layer", file: "skills/docket-convention/SKILL.md",
		absent: []string{"per-repo agent pass", "generates wrapper files for", "(change 0016)", "skills: [<skill>, docket-convention]"}},
	{sentinel: "align_0502_agent_layer", file: "skills/docket-build/SKILL.md",
		absent: []string{"install.sh", "`skills:` frontmatter"}},
	// 0502 bug 1: the draft never touches .docket/ (a dirty metadata worktree
	// makes the next repository.prepare refuse metadata-worktree-dirty).
	{sentinel: "align_0502_autogroom_draft", file: "skills/docket-auto-groom/SKILL.md",
		present: []string{"never inside `.docket/`", "`spec_markdown`", "discard the draft"},
		absent:  []string{".docket/docs/superpowers/specs/", "delete any just-drafted spec markdown"}},
	// 0502 bug 2: under build.gate off a skipped record is the expected build
	// state; under local (or an unstated gate) green is still required.
	{sentinel: "align_0502_review_gate_off", file: "skills/docket-review/SKILL.md",
		present: []string{
			"a `skipped` record carrying `reason: build-gate-off` is the expected build state, not a blocker",
			"When the payload names no `build.gate` value, treat it as `local`",
		}},
	{sentinel: "align_0502_review_gate_off", file: "skills/docket-implement-next/SKILL.md",
		present: []string{"the resolved `build.gate` value"},
		absent:  []string{"build_gate: off"}},
	// 0502 bug 3: a repaired head is re-gated through the driver, never raw verbs.
	{sentinel: "align_0502_finalize_regate", file: "skills/docket-finalize-change/SKILL.md",
		present: []string{"`--owner finalize`", "`gate.drive.advance`"},
		absent:  []string{"`gate.launch` with `--root"}},
	{sentinel: "align_0502_finalize_regate", file: "skills/docket-finalize-change/references/gate-failure.md",
		present: []string{"`--owner finalize`"},
		absent:  []string{"`gate.launch`/`observe` and records"}},
	{sentinel: "align_0502_finalize_regate", file: "agents/docket-integration-repair.md",
		present: []string{"gate driver"},
		absent:  []string{"`gate.launch`/`observe` operations"}},
	// 0517: the repaired-head re-test certifies with the finalize settings. The
	// evidence.record CALL itself must carry --owner finalize — the 0502 rows'
	// bare "`--owner finalize`" is already satisfied by the gate.drive.start
	// clause, so they cannot catch a dropped evidence flag.
	{sentinel: "align_0517_finalize_evidence_owner", file: "skills/docket-finalize-change/SKILL.md",
		present: []string{"the `evidence.record` operation with `--owner finalize --id <id>"},
		absent:  []string{"the `evidence.record` operation with `--id <id> --run"}},
	{sentinel: "align_0517_finalize_evidence_owner", file: "skills/docket-finalize-change/references/gate-failure.md",
		present: []string{"the `evidence.record` operation with `--owner finalize --id <id>"},
		absent:  []string{"the `evidence.record` operation with `--id <id> --run"}},
	// implement-next: the command comes from build configuration, not the run dir.
	{sentinel: "align_0517_finalize_evidence_owner", file: "skills/docket-implement-next/SKILL.md",
		present: []string{"the gate command from the build configuration"},
		absent:  []string{"reads the observed gate command and outcome from the run directory"}},
	// 0515: finalize adds no human gate of its own — a repair that turns the
	// rebased suite green publishes and merges, named in the run report and the
	// closeout notes; the retired sign-off token, its block/clear-block ritual,
	// and the never-wired finalize-blocked skip are gone from the agent surfaces.
	// A repair whose run halts before closeout is named in finalize.block's
	// remedy (`Authored repair:`) and carried into a later run's closeout notes.
	{sentinel: "align_0515_green_repair_merges", file: "skills/docket-finalize-change/SKILL.md",
		present: []string{"A repair that turns the rebased suite green publishes and merges like any other green change",
			"one `late_findings` entry naming what broke",
			"the `report` and the `remedy` each carry what broke",
			"with an `Authored repair:` remedy from an earlier run sends that repair's facts"},
		absent: []string{"repair-needs-signoff", "First record the sign-off requirement durably",
			"`finalize-blocked`"}},
	{sentinel: "align_0515_green_repair_merges", file: "skills/docket-finalize-change/references/gate-failure.md",
		present: []string{"A repair that turns the rebased suite green publishes and merges like any other green change",
			"Dismiss stale pull request approvals when new commits are pushed",
			"the remedy opening with `Authored repair:`",
			"turns each `Authored repair:` remedy into its own closeout"},
		absent: []string{"repair-needs-signoff", "It first records the sign-off requirement durably",
			"Auto-detect selection skips"}},
	{sentinel: "align_0515_green_repair_merges", file: "agents/docket-integration-repair.md",
		present: []string{"when the suite is green, publishes and merges it"},
		absent:  []string{"repair-needs-signoff", "must never merge unseen"}},
	{sentinel: "align_0515_green_repair_merges", file: "agents/docket-rebase-resolver.md",
		absent: []string{"repair sign-off"}},
	{sentinel: "align_0515_green_repair_merges", file: "skills/docket-convention/SKILL.md",
		present: []string{"never stops finalize selection or merge"},
		absent:  []string{"makes later **auto-detect** finalize runs skip the change"}},
	// 0502: one metadata layout — the docket branch and the .docket/ worktree;
	// no metadata_branch key, no docket-mode / repo-mode split.
	{sentinel: "align_0502_one_layout", file: "skills/docket-adr/SKILL.md",
		absent: oneLayoutAbsent},
	{sentinel: "align_0502_one_layout", file: "skills/docket-auto-groom/SKILL.md",
		absent: oneLayoutAbsent},
	{sentinel: "align_0502_one_layout", file: "skills/docket-convention/SKILL.md",
		absent: oneLayoutAbsent},
	{sentinel: "align_0502_one_layout", file: "skills/docket-convention/references/close-out.md",
		absent: oneLayoutAbsent},
	{sentinel: "align_0502_one_layout", file: "skills/docket-convention/references/learnings.md",
		absent: oneLayoutAbsent},
	{sentinel: "align_0502_one_layout", file: "skills/docket-finalize-change/SKILL.md",
		absent: oneLayoutAbsent},
	{sentinel: "align_0502_one_layout", file: "skills/docket-groom-next/SKILL.md",
		absent: oneLayoutAbsent},
	{sentinel: "align_0502_one_layout", file: "skills/docket-implement-next/SKILL.md",
		absent: oneLayoutAbsent},
	{sentinel: "align_0502_one_layout", file: "skills/docket-implement-next/references/edge-paths.md",
		absent: oneLayoutAbsent},
	{sentinel: "align_0502_one_layout", file: "skills/docket-new-change/SKILL.md",
		absent: oneLayoutAbsent},
	{sentinel: "align_0502_one_layout", file: "skills/docket-status/SKILL.md",
		absent: oneLayoutAbsent},
}

// oneLayoutAbsent are the retired metadata-layout spellings.
var oneLayoutAbsent = []string{"metadata_branch", "docket-mode", "repo-mode", "`docket`-mode"}

func TestAlignmentContracts(t *testing.T) {
	root := guardRoot(t)
	if len(alignmentContracts) == 0 {
		t.Fatalf("population floor: no alignment rows")
	}
	for _, c := range alignmentContracts {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.file)))
		if err != nil {
			t.Fatalf("read %s (sentinel %s): %v (fail closed)", c.file, c.sentinel, err)
		}
		present := make([]string, len(c.present))
		for i, p := range c.present {
			present[i] = collapseWS(p)
		}
		absent := make([]string, len(c.absent))
		for i, a := range c.absent {
			absent[i] = collapseWS(a)
		}
		for _, msg := range scanProse(c.file, collapseWS(string(b)), present, absent) {
			t.Errorf("[%s] %s", c.sentinel, msg)
		}
	}
	t.Run("non_vacuity", func(t *testing.T) {
		doc := collapseWS("a wrapped\n    clause here")
		if got := scanProse("x.md", doc, nil, []string{collapseWS("wrapped clause")}); len(got) != 1 {
			t.Errorf("a wrapped absent phrase was not caught: %v", got)
		}
		if got := scanProse("x.md", doc, []string{collapseWS("wrapped\nclause")}, nil); len(got) != 0 {
			t.Errorf("a wrapped present phrase was not matched: %v", got)
		}
	})
}
