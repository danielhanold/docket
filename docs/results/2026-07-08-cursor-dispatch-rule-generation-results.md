# Cursor dispatch-rule generation + always-full-set agents — results
Change: #48 · Branch: feat/cursor-dispatch-rule-generation · PR: <set at PR open> · Plan: docs/superpowers/plans/2026-07-08-cursor-dispatch-rule-generation.md · ADRs: 15, 16, 17

## Verify (human)

<!-- Automated suite is green: `bash tests/test_sync_agents.sh` → 242 ok / 0 NOT OK / exit=0;
     test_install / test_consuming_repo_scripts / test_finalize_gate / test_link_skills /
     test_script_contracts_coverage all pass; `bash -n sync-agents.sh` clean. -->
<!-- Everything below is a manual check the CI cannot perform in this Claude-Code-only repo. -->
- [ ] **Live-verify the dispatch rule actually forces a dispatch in Cursor (the whole point).** In a
  real Cursor repo, set `agent_harnesses: [claude, cursor]` in `.docket.yml`, run `bash sync-agents.sh`,
  confirm `<repo>/.cursor/rules/docket-dispatch.mdc` and `<repo>/.cursor/agents/docket-*.md` (full set)
  are written, then invoke a docket skill directly in Cursor's agent chat and confirm Cursor **dispatches
  to the pinned `subagent_type`** (runs at the wrapper's model) instead of running the skill inline at
  the selected model. Not testable here (docket dogfoods Claude Code); no automation asserts Cursor's
  runtime behavior.
- [ ] **Confirm no surprise files in a tracking-only repo.** In a repo that has a `.docket.yml` but no
  `agents:` block and no `agent_harnesses:` key, run `bash sync-agents.sh` and confirm it writes **zero**
  `<repo>/.claude/agents/docket-*.md`, and `bash sync-agents.sh --check` exits 0 (no-op). Verified here
  against docket's own `.docket.yml` in a sandbox (0 wrappers, rc=0); this box confirms it in a live
  checkout before wiring `--check` into any CI.
- [ ] **(If adopting for docket itself) decide whether to opt docket's own repo in.** docket's `.docket.yml`
  is tracking-only, so it commits no wrappers by design. If you want docket to dogfood committed Cursor
  agents + the dispatch rule, add `agent_harnesses: [claude, cursor]` and commit the generated
  `.cursor/` set — a deliberate, separate decision, intentionally out of scope for this change.

## Findings

- **ADR-0017** recorded (`relates_to: [15, 16]`, `change: 48`): "Per-repo agent generation goes
  always-full-set, opt-in, with a Cursor dispatch rule." Captures the always-full-set flip (the `agents:`
  block is override-only), the assembled `docket-dispatch.mdc` (head + per-agent fragments, glob order,
  minimal auto-block + warning for a fragment-less agent), the opt-in trigger, and the prune step. A
  standalone ADR (not a `## Update` on 15/16) because the mechanism is distinct and citable; ADR-0015's
  passthrough contract is refined, not reversed (no model-ID validation added).
- **Backward-compat design fix from the whole-branch review (the significant finding).** The plan gated
  per-repo generation on merely `.docket.yml` presence. The final review found that this newly littered
  8 untracked `.claude/agents/docket-*.md` into any tracking-only repo (via `install.sh`'s `sync-agents.sh`
  run) and flipped its `sync-agents.sh --check` to failing — a backward-incompatible break for adopters
  who use docket for change-tracking only, and it left docket's own repo self-inconsistent. Resolved by
  making per-repo generation **opt-in** (an `agents:` block OR an explicit `agent_harnesses:` key);
  verified docket's own tracking-only `.docket.yml` now generates 0 wrappers and `--check` is a no-op,
  preserving the invariant recorded in `docs/results/2026-06-17-finalize-rebase-retest-gate-results.md`.
- **Prune safety held up under adversarial review (the highest-risk area).** `prune_orphans` deletes only
  `docket-*` files whose built-in source is gone (or a de-listed harness's files); it never touches a
  non-docket file, and `rmdir`s only a directory docket itself emptied — a regression test now locks that
  a pre-existing user `.claude` survives a `[cursor]`-only / `[]`-empty run. One task-level deviation
  (guarding the de-list scan with the same opt-in precondition the passes use) was reviewer-verified sound.
- **Two hardening fixes (final review, Minor):** de-piped `agent_description` (`sed | head` → single
  `sed`) to remove a latent SIGPIPE-under-`set -euo pipefail` trap on the fragment-less path (LEARNINGS
  #46); and tightened `prune_orphans` to track per-dir contribution (`pruned_agents`/`pruned_rule`) so it
  never `rmdir`s a sibling dir it didn't empty.

## Follow-ups

- **`.docket.yml` deleted outright (vs editing `agent_harnesses`) leaves per-repo harness files
  un-prunable.** `prune_orphans` part (2) requires `.docket.yml` present; part (1) only catches
  removed-built-in orphans, not whole-harness-gone orphans. Strictly better than pre-0048 (no prune
  existed at all); a backlog note, not a bug.
- **The two drift-check blocks in `check_project_level` (agents vs. dispatch rule) are near-duplicates.**
  Plan-mandated verbatim; a shared `drift_check_one` helper could DRY them if the file is touched again.
  Left as-is to avoid a late refactor.
- **Determinism of the rule's subsection order depends on `LC_COLLATE`** (the `agents/docket-*.md` glob).
  Generation and `--check` re-assemble with the same glob, so they are self-consistent on any one machine;
  a cross-machine locale difference would only reorder subsections (byte-drift the `--check` would flag),
  never break dispatch. A `LC_ALL=C` sort could pin it if that ever bites.
