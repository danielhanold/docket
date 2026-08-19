---
id: 316
slug: finalize-recovery-reclaim-archive-and-stacks
title: 'Finalize, recovery, reclaim, archive, and stacks'
status: 'in-progress'
priority: critical
type: feat
created: 2026-08-12
updated: '2026-08-19'
depends_on: [315, 322, 326]
stacked_on:
related: [298, 318, 322, 326]
discovered_from: [303]
adrs: [10, 11, 35, 43, 59, 66, 74, 83, 86, 92, 95]
spec: docs/superpowers/specs/2026-08-18-finalize-recovery-reclaim-archive-and-stacks-design.md
plan: 'docs/superpowers/plans/2026-08-18-finalize-recovery-reclaim-archive-and-stacks.md'
results:
trivial: false
auto_groomable:
branch: 'feat/finalize-recovery-reclaim-archive-and-stacks'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-19T00:47:43Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | `docs/superpowers/specs/2026-08-18-finalize-recovery-reclaim-archive-and-stacks-design.md` |
| Plan | `docs/superpowers/plans/2026-08-18-finalize-recovery-reclaim-archive-and-stacks.md` |
| ADRs | `docs/adrs/0010-finalize-merge-gate-split-agents.md`, `docs/adrs/0011-finalize-consent-model.md`, `docs/adrs/0035-cleanup-teardown-fail-closed.md`, `docs/adrs/0043-retire-bot-auto-approval-zero-approvals-branch-protection.md`, `docs/adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md`, `docs/adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md`, `docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md`, `docs/adrs/0083-agent-worktree-scope-is-a-declared-frontmatter-fact.md`, `docs/adrs/0086-in-context-gating-dispatch-carved-out-of-the-tier-taxonomy.md`, `docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md`, `docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md` |
<!-- docket:artifacts:end -->

## Why

The migration is not functionally complete until merged and interrupted work converges safely to
the correct terminal state in both repository modes.

## What changes

Add authoritative finalize context and resumable Go operations for local rebase/retest, rewritten
head publication, merge verification, atomic terminal archive and stack close-out, explicit and
policy-driven reclaim, durable halted/finalize-blocked recovery, merged-PR maintenance, generated
terminal-link repair, and ownership-safe run/workspace/branch cleanup.

## Implementation launch prerequisites

Changes 0322 and 0326 are explicit dependencies, not behavior to implement here. Change 0322 makes
the reviewed source bootstrap install and adopt a permanent Go development runtime; change 0326
contracts Docket's active deferred configuration before the first Go metadata mutation. The
migration owner then runs the source bootstrap from reviewed merged `origin/main`, verifies
`docket install check` and `docket diagnostic config --for-mutation`, and restarts the host before
dispatching this change.

The transient `go run` bootstrap may perform only `development install`. It is never used for
`context`, `change`, `workspace`, `evidence`, `pr`, `run`, or `finalize` against shared Docket
state. Change 0316 starts only through the installed, verified binary after both dependencies are
done; a failed install/config check remains an abort-and-report precondition failure.

## Out of scope

Behavior owned by changes 0305 through 0315; release packaging and four-harness acceptance from
0317; remaining self-hosting, Bash removal, and hard cutover from 0318; and
deferred CI/combined gates, results-only skips, terminal publishing, automatic learning harvest,
capture/groom automation, cross-harness routing, skill rebinding, or Bash fallback behavior. This
change also does not implement the source bootstrap or legacy adoption from 0322, contract
configuration from 0326, add Go verbs to `docket.sh`, authorize a source-built binary to mutate
Docket's live metadata, or bypass the unsupported-capability fence.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-19

Reconciled against current reality at claim time; no scope change.

Verified the launch precondition and the landed-foundations assumptions the design rests on:

- Dependencies 0315, 0322, and 0326 are all `done` (the `context implementation` readiness for 316 is `build-ready` with the deps satisfied).
- The migration-host bootstrap is in place: the PATH-resolved `docket` binary is installed (`install check` -> `no-op`, release asset set), `command -v docket` -> `/Users/homer/.local/bin/docket`, and `diagnostic config --for-mutation` reports `mutation: allowed` with a valid supported configuration. The earlier missing-installation-record blocker recorded in the prior halt is cleared.
- The landed Go foundations the spec consumes as dependencies are present: `internal/{config,domain,document,render,repository,gitcli,githubcli,workspace,process,evidence,app,cli,assets,install,harness}`.
- The work this change owns is genuinely unbuilt: there is no `internal/finalize` package, no `docket finalize` or `docket maintenance` command tree, and `docket change` carries claim/reconcile/attach-*/mark-implemented/block/create/defer/groom/kill/refresh-claim but not the `halt`/`resume-halted`/`reclaim` verbs this change adds. `docket run` is present but read-only (verify), matching the pre-0316 checkpoint set.

No proposal-section rewrite and no relation change are warranted: the spec was authored (2026-08-18) specifically for this launch point, its `depends_on`/`related`/`adrs`/`discovered_from` still describe reality, and the change carries no `stacked_on`. AUTO_CAPTURE is disabled, so no adjacent-work stubs are minted this pass; none surfaced that would warrant one regardless.

## Run halted

An autonomous `docket-implement-next` run stopped needing a human at the Step-5 build gate.

### 2026-08-19

**What stopped the run.** The whole-suite build gate came back red: `SUITE files=123 passed=95 failed=28`. The build worker fan-out (Tasks 1–18) completed and each task's focused checks passed, but the single end-of-build full-suite gate — the deliverable of Step 5 — is not green, so the branch is uncertified and no PR may open. Per the run's autonomy contract no test was weakened to force green.

**Root cause.** Task 18's rewrite of `skills/docket-finalize-change/SKILL.md` (and its references) into the Go-v1 sequencer, plus the agent-template rewrites and asset regeneration, **over-deleted** required skill content and busted budgets. The 28 failures fall into three buckets:

1. **Finalize-skill content/wiring stripped (majority).** ~20 shell tests assert content and facade wiring the rewrite removed, much of it cross-cutting invariants orthogonal to the finalize Go-migration that must survive it — not obsolete pins of an old contract:
   - `test_board_refresh_on_transition` — the "BOARD.md is never published" guarantee + single-facade Board-pass call.
   - `test_closeout`, `test_stack_closeout`, `test_docket_stack`, `test_docket_metadata_branch` — archive-change / terminal-publish / cleanup-feature-branch facade invocations, close-out + descendants ops, integration_branch knob.
   - `test_change_links_coverage` — render-change-links facade invocation.
   - `test_learnings_ledger` — the harvest step + idempotency probe keyed on the `changes:` list.
   - `test_dummy_mode` — dialogue/reports dummy-mode surface bindings.
   - `test_docket_example_yml`, `test_finalize_gate` — `require_pr_approval` documentation + the FINALIZE_REQUIRE_PR_APPROVAL provenance clause.
   - `test_finalize_disposition`, `test_gate_execution_posture`, `test_docket_review` — gate-failure reference pointer, skipped-with-reason enumeration, non-vacuity anchors, skip-requires-no-op-rebase/result-green.
   - `test_results_artifact`, `test_shared_worktree_commit_scope`, `test_config_read_channel`, `test_configured_bash_finalize`, `test_readme_finalize_docs`, `test_skill_facade_wiring`, `test_dispatch_capability`, `test_skill_handoff_precedence`, `test_sync_agents_run_gate` (committed AGENTS.md block now stale).
2. **Real regressions independent of the migration.**
   - `test_skill_size_budgets` — the rewritten skills exceed budget: `docket-implement-next/SKILL.md` 6089 > 6000 words; `docket-finalize-change/references/gate-failure.md` 111 > 40 lines; `docket-implement-next/references/edge-paths.md` 54 > 50 lines / 775 > 700 words; `docket-status/SKILL.md` 121 > 118 lines / 2798 > 2650 words.
   - `test_sync_agents` — `docket-rebase-resolver` / `docket-integration-repair` bodies no longer carry the required abort-and-report directive shape the validator matches.
   - `test_go_toolchain`, `test_go_race` — the harness golden fixtures (`internal/harness/{claude,codex,cursor,opencode}` GoldenAgents + DispatchGolden) drifted against the rewritten agent templates and dispatch instructions and were not regenerated.
   - `test_docket_config` — `finalize finish uses SKILL_FINISH` wiring assertion.

**Why this needs a human (not an autonomous gate fix).** Correct remediation requires deciding, per assertion, which finalize-skill content Task 18 intentionally removed as part of the Go-migration versus wrongly removed — the plan/spec did not carry that test-migration mapping, and several failing invariants (BOARD.md-never-published, terminal-publish, learnings harvest, dummy-mode) are clearly must-survive. Guessing that boundary would either weaken a test or ship a broken finalize skill. Task 18's scope must be re-cut to (a) restore the required, migration-orthogonal finalize-skill content and wiring, (b) trim the rewritten skills back under their size budgets, (c) restore the agent-body abort-and-report directives, and (d) regenerate the harness goldens + AGENTS.md block — then re-run the whole suite.

**State left in place.** Change is `in-progress`, `reconciled: true`, `claimed_at` fresh; branch `feat/finalize-recovery-reclaim-archive-and-stacks` carries 19 commits (Tasks 1–18) with a clean worktree; no `pr:`. Build-evidence record was NOT minted (the gate is red). Gate run dir: `gate-run.QLwh2a/docket-0316-suite` (terminal `kind=exit code=1`). Separately, an `OVER BUDGET` breach affected 10 files under parallel contention (e.g. `test_go_toolchain` 363s vs 55s ceiling); that is a wall-clock finding, not a pass/fail cause.
