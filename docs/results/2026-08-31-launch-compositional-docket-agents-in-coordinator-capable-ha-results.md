<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0384 — Launch compositional Docket agents in coordinator-capable harness contexts](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0384-launch-compositional-docket-agents-in-coordinator-capable-ha.md)**
<!-- docket:backlink:end -->
# Launch compositional Docket agents in coordinator-capable harness contexts — results
Change: #0384 · Branch: fix/launch-compositional-docket-agents-in-coordinator-capable-ha · PR: <url> · Plan: docs/superpowers/plans/2026-08-31-launch-compositional-docket-agents-in-coordinator-capable-ha.md · ADRs: none

## Verify (human)

Genuinely manual, post-merge checks the automated suite cannot reach. The change is **not `done`** until the production nested-sentinel reconfirmation lands — automated Go tests cannot substitute for a live fresh-process Codex composition (spec §6).

- [ ] **Post-merge, fresh process:** after this branch merges and `docket development install --source /Users/homer/dev/docket` runs on `main`, restart the Codex process and resume the halted change **0364** through its existing `docket change resume-halted --acknowledge-quiescent` contract. Reaching and successfully consuming a real `docket-plan-writer` return past 0364's previous Step-4 halt is the **production confirmation** of this change. (0364 is deliberately **not** resumed on this branch; certification.md and the fixture prove the mechanism, and reframe 0364's halt as an observation-oracle artifact, but do not themselves re-run the production path that halted.)
- [ ] **Optional re-run of the disposable fixture** on the exact Codex build (`docs/codex/fixtures/nested-launch/README.md`): install the two probe TOMLs beside `~/.codex/agents/docket-*.toml`, fresh process, both entry paths, and confirm `COORDINATOR_CONSUMED=<uuid>` adjudicated **via the thread store** (`~/.codex/sessions/.../rollout-*.jsonl`), never the `codex exec --json` item stream, which under-reports collaboration. Tear down (`rm ~/.codex/agents/probe-*.toml`).

## Findings

- **The native launch already works — no code change was needed.** The Task-2 investigation (`docs/codex/fixtures/nested-launch/decision.md`) proved `mechanism: universal, needs_role_distinction: false, adr_required: no` on codex-cli **0.151.0** with `multi_agent = true`: every registered Docket agent is already coordinator-capable via the native `spawn_agent {"agent_type":"<registered name>"}` + `wait_agent` calls, which the wrappers' existing change-0365 `codexDispatchBoundary` machine-neutral instruction already elicits; the spawned thread runs **as** the registered definition (its `developer_instructions`, `model`/`model_reasoning_effort` pins, skill preload, and recursion guard all in force). Depth root → coordinator → leaf was certified on both supported entry paths and the app-server stdio surface. Consequently no renderer change was made, and the change's substance is the fixture + decision record + live certification + documentation.
- **Binding observation protocol (the real defect class).** The `codex exec --json` item stream under-reports collaboration (renders a no-op `wait` with empty `receiver_thread_ids`/`agents_states`, never itemizes `spawn_agent`) while real children run. This produced Task 1's false "fabrication"/blocked reading — the same false-negative shape that halted change 0364. **Pass/fail must be adjudicated only from the Codex thread store or the app-server `subAgentActivity` stream, never the exec JSON.** Documented in `docs/codex/setup.md`, `docs/codex/validation-runbook.md` (Phase 7), and the fixture; ADR-0059 (capability-absence needs a failed attempt) is affirmed, not amended.
- **All claims are version-scoped** to codex-cli 0.151.0 / `multi_agent = true` on this machine; no other Codex client or version is claimed to behave identically.
- **Review (docket-review-deep): clean** — 0 blocker, 0 important, 3 minor. Two minors fixed in-branch (commit `8bb90554`: dropped an untraceable app-server root thread id from `decision.md`/`certification.md`; added a Phase-7 rationale note). The third minor is this results file's "0364 reconfirmation is post-merge" caveat, carried forward here per the reviewer's request.

## Follow-ups

- None minted. The only outstanding work is the post-merge 0364 resume above, which is the production confirmation of *this* change rather than separate work. If that resume surfaces a genuinely distinct defect, capture it deliberately with `docket change create`.
