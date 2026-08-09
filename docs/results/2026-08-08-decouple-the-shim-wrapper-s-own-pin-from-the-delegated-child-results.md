<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0269 — Decouple the shim wrapper's own pin from the delegated child's](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0269-decouple-the-shim-wrapper-s-own-pin-from-the-delegated-child.md)**
<!-- docket:backlink:end -->

# Decouple the shim wrapper's own pin from the delegated child's — results

Change: #0269 · Branch: feat/decouple-the-shim-wrapper-s-own-pin-from-the-delegated-child · PR: (opened at close-out) · Plan: docs/superpowers/plans/2026-08-08-decouple-the-shim-wrapper-s-own-pin-from-the-delegated-child.md · ADRs: 15, 38, 67, 79

## Verify (human)

- [~] **Run a real runner-delegated dispatch on the claude harness.** This is the defect's only true
      oracle and no in-repo test can be one (`external-truth-needs-a-human-checkpoint`): the suite
      asserts what bytes land in the shim's frontmatter, never that Claude Code accepts them.
      Regenerate wrappers (`sync-agents.sh`), then dispatch a `docket-build-*` agent in a repo whose
      `agents:` block delegates to a runner, and confirm it reaches `runner-dispatch.sh` instead of
      dying on an unresolvable model. Before this change the failure was a bare harness error that
      never named the runner.

      **PARTIALLY DISCHARGED by change 0258 (2026-08-08, human-observed).** That run dispatched
      `docket-build-standard` by name on the claude harness; Claude Code loaded the wrapper, parsed
      its frontmatter, and the shim made its `runner-dispatch --runner opencode` facade call, with
      the child executing real work inside the feature worktree before an unrelated 600 s
      foreground-timeout kill. So the delegation path and the wrapper's frontmatter shape are
      confirmed accepted. **Residual gap:** 0258's wrappers predate `shim_model`/`shim_effort`, so
      what it proved is acceptance of the OLD frontmatter shape. The bytes this change puts in the
      parent-side `model:`/`effort:` fields are new and still want one dispatch on a regenerated
      wrapper. See `docs/changes/active/0258-…md` § "Run halted" for the primary record.
- [x] **Confirm `shim_model: inherit` emits `model: inherit`** in a regenerated wrapper, and that
      Claude Code runs the shim on the parent conversation's model. **VERIFIED by human 2026-08-08:**
      setting `shim_model: inherit` yields frontmatter `model: inherit`, which is the correct
      emission. NOTE — this item previously read "emits NO frontmatter `model:` line", which was
      **wrong**: on the claude harness `emit()` passes `inherit` through VERBATIM by design, because
      Claude Code documents `inherit` as a real value ("run on the parent conversation's model") and
      that is a DIFFERENT runtime outcome from omitting the key (Claude Code's own subagent default).
      Only `emit_cursor_md`/`emit_codex_toml` normalize `inherit` to "no pin" — an asymmetry
      deliberately preserved since the 0168 whole-branch review (IMPORTANT 2). See the `emit()`
      header in `sync-agents.sh` and the `inherit:` asserts in `tests/test_sync_agents.sh`.
- [ ] **Restart the Claude Code session before trusting regenerated wrappers.** The subagent registry
      loads at process start, so wrappers rewritten mid-session are not what a dispatch actually runs.
- [ ] **Sanity-check one non-claude harness.** `emit_shim` is reachable only for `harness = claude`;
      confirm codex/cursor wrapper bytes are unchanged by this branch.

## Findings

- **ADR-0079** (`A shim wrapper's frontmatter pin governs the parent-side agent`) supersedes
  **ADR-0038**, whose Decision text stated the false premise this change corrects — that a shim's
  frontmatter `model:` line is "bookkeeping" and "the effective pin is the baked argument." Claude
  Code reads it as the live pin for the parent-side shim agent. ADR-0038 is now
  `Superseded by ADR-79`, and that status change was published onto `main` separately; ADR-0079
  itself rides this change's terminal publish. **Byte-stability with the native wrapper shape — an
  accepted consequence of ADR-0038 — is deliberately given up.**
- **The defect's blast radius was total, not conditional.** ADR-0067 requires a runner-bearing agent
  to carry a user-configured model, so the child's ID was always present to be copied into the wrong
  slot: every runner-delegated claude wrapper was born unrunnable, and no configuration avoided it.
  `shim_model: inherit` as the default repairs them all by regeneration alone, with no config edit.
- **Review (docket-review-standard) returned 11 findings: 0 blocker, 4 important, 7 minor.** All 11
  were dispositioned in-branch; the PR body carries the table. Two are worth reading past the table:
  - The two new `--check` asserts were **vacuous** — they passed with the gate deleted, because the
    bare fixture already failed `--check` on an unrelated `.gitignore` leg. Fixing them was not a
    one-line tightening: the bad value has to sit where it fires the gate *without* moving an emitted
    byte, or `check_project_level` leg (c) reports drift and restores the vacuity through the back
    door. Now anchored on the gate's own diagnostic string.
  - The new gate walked **every registered runner across all three config layers, unconditionally**,
    unlike its sibling `validate_runner_config`, which scopes its legs precisely so a gate cannot
    refuse over config that generates nothing. A typo'd `shim_model` in `~/.config/docket/config.yml`
    for a runner no agent referenced would have hard-failed `sync-agents.sh` in **every repo on the
    machine**. Scoped to the resolved candidate set. The **layer** dimension stays unscoped on
    purpose: `runner_key` reads all three layers, so a value masked today goes live the moment the
    higher layer is edited.
- **A fix commit briefly broke another shard, and the loop caught it.** `c23068fd` renamed the
  reader's locals to shell globals (`SHIM_MODEL=`), which silently unhooked shape 4's
  lower-case-only anchor in `test_docket_example_yml.sh` — both new keys lost their consumer anchor.
  Repaired in `84b1eac4` (shape 4 now matches either casing, statement-initial boundary intact). The
  manifest guard and the population floor now share one `shape_ere` helper so they cannot drift apart.
- **Mutation testing did real work three times.** Task 3's mutation C originally *survived* a
  too-loose shape-4 boundary and only reddened once tightened to statement-initial. The scoping fix
  proved itself by restoring the reviewed defect (`REGISTERED_RUNNERS`) and watching 7 asserts go red.
  The extraction refactor mutated the shared primitive and reddened **both** call sites (6 reader
  asserts + 7 gate asserts), which is the evidence that one helper is load-bearing for both.
- **A naive scoping implementation cost ~38% of a generation pass** (1.60s → 2.20s; FORK_COUNT
  77 → 106). The committed memoized single-walk design benches at parity with `main`, FORK_COUNT
  **74** against the change-0175 baseline of 77. Note the fork oracle's sandbox delegates nothing, so
  it measures the empty-candidate path rather than the memo's win.
- **The bare-scalar gate initially promised more than it delivered.** It rejected three shapes;
  `>`, `|`, `[a]`, `{m: x}`, `*ref`, `&anchor` and a trailing-only quote all survived into the
  emitted pin. Inverted to an allowlist. Separately, a **flow-style** runner block
  (`codex: {shim_model: haiku}` — plausible, since the `agents:` block right above uses exactly that
  style) parsed to nothing and fell back to `inherit` **silently**; it is now refused with a
  "block mapping required" diagnostic. Configured-but-never-applied is the class this change exists
  to eliminate, so it should not have survived inside the change's own new code
  (`fix-reintroduces-its-own-defect-class`).

## Follow-ups

- **#0270** — the `runners.opencode.permissions` locality defect, minted at reconcile from this
  change's out-of-scope list: `.docket.local.yml` is gitignored, so a fresh feature worktree has no
  copy and a build worker anchored there resolves an `auto-approve` grant back to the default `ask`
  and is refused.
- **#0256 — config-reader consolidation.** This change knowingly ships a second consumer of the
  `runners:` block rather than unifying the parsers. The duplication was reduced, not removed:
  `runner_block_value` is now the single extraction primitive, and `for_each_candidate_triple` the
  single population walk, but `runner-dispatch.sh`'s `yaml_section` remains an independent twin of
  `sync-agents.sh`'s `section_body`. 0256 should absorb both.
- **Suite runtime is the branch's real cost.** Four files are advisory `OVER BUDGET` (exit 0):
  `test_harness_defaults`, `test_sync_agents`, `test_sync_agents_codex`, `test_sync_agents_runners`.
  Three predate this branch; `test_sync_agents_runners` is one this branch grew — 188s and 272
  asserts, now the suite's longest single file and the wall-clock floor for the whole run. The
  remedy is sharding, not trimming coverage.
- **Two files were edited beyond the plan's Files list**, both forced rather than chosen:
  `test_docket_example_yml.sh` gained a fourth `code_shaped_mention` shape (the reader assigns to a
  shell variable, which no existing shape anchored), and `test_skill_size_budgets.sh` raised the
  `references/agent-layer.md` budget 190/2200 → 205/2350 to admit the new keys' documentation.
- **Plan-snippet deviation, twice.** Tasks 1 and 2 both replaced the plan's
  `section_body … | section_body …` with a herestring: a producer feeding an early-exiting consumer
  under `pipefail` is a latent failure, and the plan's snippet was unverified code, not an oracle
  (`plan-supplied-test-code-is-unverified`).
