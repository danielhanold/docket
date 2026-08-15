<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0324 — Extract plan writing into a model-pinned internal agent](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-15-0324-model-pinned-plan-writer-agent.md)**
<!-- docket:backlink:end -->

# Model-pinned plan-writer agent — results
Change: #324 · Branch: feat/model-pinned-plan-writer-agent · PR: <url> · Plan: docs/superpowers/plans/2026-08-15-model-pinned-plan-writer-agent.md · ADRs: 94

## Verify (human)

<!-- GENUINELY MANUAL checks the automated suite cannot reach. Each item PENDING until checked. -->

- [ ] **Outside-truth model IDs are real, currently-served IDs.** Two shipped `plan-writer`
  defaults are new to the entire repo history, so no in-repo test can certify them — a wrong ID
  typically falls back to a house default *silently*:
  - cursor: `cursor-grok-4.5-xhigh` (effort `auto`)
  - opencode: `openrouter/deepseek/deepseek-v4-pro-0813` (effort `medium`)
  Certify by one dispatched run per harness, or a vendor-docs check. (claude `claude-opus-5`/`high`
  and codex `gpt-5.6-terra`/`high` also ship but are already-used house IDs.)
- [ ] **Start-time-loaded wrappers pick up the pin.** The generated `docket-plan-writer` wrappers are
  loaded at harness process start, so the session that generated them cannot runtime-validate the
  pin. After merge + `sync-agents.sh`, **restart the harness** and confirm a `docket-plan-writer`
  wrapper exists per enabled harness carrying its resolved model/effort and `worktree-scope: feature`.
  The hermetic generator tests (`test_plan_writer_agent.sh`) are what this run can honestly claim;
  the live pin is a restart-gated human check.

## Findings

- **ADR-0094** — recorded this run: plan authoring is a pinned internal composition agent
  (`docket-plan-writer`) that owns a git-verifiable plan artifact (committed plan + backlink +
  `Docket-Plan-Path:` trailer, `PLAN_PATH=` receipt), while the implementer keeps orchestration and
  metadata attachment and verifies from git before writing `plan:`; unavailable dispatch is Tier C,
  not a silent inline fallback.
- **Scope premise corrected before this build.** The original spec/plan asserted the change "touches
  no Go source." That was false — registering the 17th shipped agent reconciles the Go built-in
  registry and its frozen parity fixture. Folded into 0324 by human decision (see the change file's
  `## Halt resolution`); realized here as **Task 6A** (defaults.go + shape/name guards + a **sparse**
  `testdata/repositories/v0.9.3/` frozen tree — only `agents-harness-defaults.yml` + `PROVENANCE.md`,
  `sidecarPath` re-pointed v0.9.2→v0.9.3, v0.9.2/ left byte-immutable — plus four harness golden
  refreezes and the embedded-asset regen).
- **Deep review:** 0 blocker, 1 important, 4 minor — all fixed in-branch before this PR opened. The
  important one: a learnings-enablement guard was relaxed in `test_learnings_ledger.sh` citing a
  compensating assert that did not yet exist; the compensating assert was added to
  `test_plan_writer_agent.sh` (mutation-tested) and the ledger comment corrected. The minors were
  16→17 / 16x4→17x4 count-descriptor drift in comments and codex docs.

## Follow-ups

- **`git tag 0.9.3` is NOT cut in this run** (human decision). Only the `v0.9.3/` directory name and
  in-code version references land here; cut the actual tag `0.9.3` only after 0324 merges and is
  confirmed working. The Bash rollback baseline stays tag `v0.9.2`.
- **Pre-existing test-runtime tech debt.** `tests/test_sync_agents_runners.sh` was already at the
  hard 60s ceiling on `origin/main`; adding the 17th agent tipped its measured wall over. Resolved
  the *right* way (a bump is forbidden by the tsv's own ceiling + `EXPECTED_TOTAL` pin) by **sharding**
  it into three siblings (`…_runners.sh` / `…_runners_gates.sh` / `…_runners_pins.sh`), all
  assertions preserved, `EXPECTED_TOTAL` re-seeded 1845→1905. No further action required; noted so the
  shard's provenance is auditable.
- **Program map cross-reference:** `docs/superpowers/specs/2026-08-12-go-migration-program-map.md`
  records that post-sprint change 0324 introduces the 17th shipped agent and the `v0.9.3`
  agent-registry release (no new sprint change created).
