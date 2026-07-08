# Per-repo multi-harness agent generation — results
Change: #45 · Branch: feat/multi-harness-agent-generation · PR: <set on open> · Plan: docs/superpowers/plans/2026-07-08-multi-harness-agent-generation.md · ADRs: 15

## Verify (human)

The hermetic suite (`tests/test_sync_agents.sh`, 155 assertions) proves the **bytes generated** —
default `[claude]` byte-identical, `[claude, cursor]` fans out to both dirs, model IDs pass through
verbatim, `--check` spans every listed harness, unknown/glob/quoted/`[]` tokens handled. What it
**cannot** prove is that a non-Claude harness *honors* the generated file. That is the spec's open
question and the one merge-gate step:

- [ ] **Cursor honors the *generated* wrapper.** ADR-0015 verified Cursor honors a hand-made
      project-level `model:`. The generated `.cursor/agents/docket-*.md` is **richer** than that probe
      — it also carries `effort:` and, load-bearingly, `skills: [docket-<skill>, docket-convention]`.
      In a repo opened under Cursor, set `.docket.yml` to `agent_harnesses: [claude, cursor]` with an
      `agents:` override, run `bash sync-agents.sh`, then confirm on the generated file that Cursor
      **(a)** runs the pinned `model:` despite the extra frontmatter, and **(b)** still loads the skill
      via `skills:` (so the agent actually *is* the docket agent, not a bare model on an empty prompt).
      A representative generated file (from `agent_harnesses: [cursor]` + `status: { model:
      gpt-5.5-medium-fast, effort: high }`):

      ```markdown
      ---
      name: docket-status
      description: Use when you want to see or refresh the docket backlog — …
      model: gpt-5.5-medium-fast
      effort: high
      skills: [docket-status, docket-convention]
      ---
      Execute docket-status to refresh the board and run the sweep + health checks. Follow the skill exactly.

      You run autonomously with no human to pause and ask: treat any unmet precondition or blocking
      ambiguity as abort-and-report (stop and surface what blocked you), never an interactive prompt.
      ```

## Findings

- **Plan gap caught at build (Task 1):** the plan removed the `PROJECT_AGENT_DIR` variable in Task 1
  but `check_project_level` (only fully rewritten in Task 2) still referenced it — an unbound-variable
  crash under `set -euo pipefail` that would have reddened the existing `--check` tests. The Task-1
  implementer bridged it (inlined `.claude/agents`) and Task 2 replaced the whole function with the
  harness loop. No net effect on the final code; noted so the seam is on record.
- **Robustness hardening from the final review (all Minor, fixed in `fix(0045)`):** `is_valid_harness`
  now rejects the empty string; the external-token loop runs under `set -f` so a `*` token can't
  glob-expand against the cwd; a doc sentinel's grep bracket bug (`[^\n]` → `.*`) was corrected; and
  column-0 anchoring for `agent_harnesses` gained a dedicated indented-decoy test.
- No new ADR: every design decision (direct harness-neutral model IDs, explicit `agent_harnesses`
  fan-out, direct-parse-not-`docket-config.sh`, token→dir from `HARNESS_AGENT_DIRS`) is already
  recorded in **ADR-0015** (Accepted).

## Follow-ups

- **If Cursor does not load the skill via `skills:` (verify item b fails):** Cursor may need the skill
  **body inlined** into the generated wrapper rather than referenced by name. That is a separate change
  (a new generation mode), out of scope here — this change delivers the fan-out generation, which is
  correct regardless of the honoring outcome. Propose a follow-up change if (b) fails.
- **User-level pass unchanged (deliberate, per ADR-0015):** `agent_harnesses` governs the per-repo
  (committed) pass only; the user-level pass still writes every present harness. Narrowing the
  user-level fan-out was explicitly left open by ADR-0015 and is not done here.
