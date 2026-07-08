# Per-harness agent model overrides (harness-first `agents:`) — results
Change: #46 · Branch: feat/per-harness-agent-models · PR: <set at PR open> · Plan: docs/superpowers/plans/2026-07-08-per-harness-agent-models.md · ADRs: 15, 16

## Verify (human)

<!-- Automated suite (`bash tests/test_sync_agents.sh`) is green: 193 ok / 0 NOT OK / exit=0. -->
<!-- Everything below is a manual check the CI cannot perform in this Claude-Code-only repo. -->
- [ ] **Live-verify non-Claude project-over-user precedence (spec Open Question 1).** In a real
  Cursor repo, set a per-repo `agents.cursor.<agent>` in `.docket.yml` AND a different
  `~/.config/docket/agents.yaml` `cursor:` value, run `bash sync-agents.sh`, and confirm Cursor
  reads the **project-level** `<repo>/.cursor/agents/…` over the user-level `~/.cursor/agents/…`
  (as Claude Code does). If a harness does NOT prefer project-level, docket must merge global into
  that harness's project-level file — a scoped follow-up. Not testable here (docket dogfoods Claude
  Code); no automation asserts it.
- [ ] **Eyeball the footgun-warning volume in a real non-Claude repo.** With a non-Claude harness dir
  present and no `agents.<harness>` overrides, `sync-agents.sh` emits one fallback warning per
  (agent × non-Claude harness) — intended, but confirm it reads as a helpful nudge, not noise, in
  practice.

## Findings

- **ADR-0016** recorded (`relates_to: [15, 8]`, `change: 46`): "Harness-first `agents:` config —
  per-harness model/effort with field-level default fallback." Captures the harness-first schema
  (reserved `default:` + harness keys), the independent field-level fallback
  `agents.<harness>.<agent>` → `agents.default.<agent>` → built-in, the layer-symmetry rejection of a
  per-harness catch-all default model, the clean break from the flat pre-0046 shape (warned+ignored),
  and the allowlist-free non-Claude footgun warning. A `## Update` note on ADR-0015 was the considered
  alternative; a standalone ADR was chosen because the mechanism is distinct and citable (ADR-0015 is
  refined, not reversed).
- **Two Important correctness fixes** during build (both were latent in the plan's own awk, caught by
  task review): `ind()` used a literal-space class `[^ ]` that silently dropped a whole tab-indented
  config layer — fixed to `[^[:space:]]` (both awk copies) with a tab-indented regression test; and an
  unguarded `printf … | section_body` SIGPIPE producer pipe (section_body `exit`s early) — guarded
  with `|| true`, matching the file's other guarded pipes.
- **One deviation from the plan's literal code, verified correct:** `check_project_level`'s
  empty-`agent_keys` early return became `return $rc` (was `return 0`) so a legacy-only `.docket.yml`
  (a bare pre-0046 agent key, no `default:`/harness block) still reports `--check` drift instead of
  silently passing.
- **One doc-accuracy fix (final review):** the harness-first examples initially carried the stale
  `effort: auto (or omitted)` equivalence (LEARNINGS #47). Corrected to match `emit()`: literal
  `effort: auto` drops the effort line; an **omitted** `effort:` key keeps the built-in effort. Also
  scoped the prose so `agent_harnesses` is described as gating only the per-repo pass (the user-level
  pass writes every harness dir present on disk).

## Follow-ups

- **Minor classifier edge cases (non-blocking, outside documented grammar) — candidates for a small
  hardening change if ever hit in practice:**
  - An inline empty-flow-map harness header `cursor: {}` is misclassified by `legacy_agent_keys` as a
    bare agent key, emitting a misleading "legacy" warning even though `cursor` is a listed harness.
  - A legacy agent entry written in block style (`status:\n  model: sonnet`) instead of inline
    `{…}` evades legacy detection entirely (read as a pseudo-harness). Root cause: the flow-map-only
    value parsing shared with `field_of`/`harness_agent_line`. Real migrating repos use inline-brace
    form, so neither bites today.
- **`section_body` dedent** truncates a shallower sibling key on already-malformed YAML (inconsistent
  sibling indent); a real YAML parser would reject first. Fragility note only.
- **Re-parse efficiency:** `resolve_agent` re-reads the config twice per (harness, agent) and
  `user_level_pass` re-parses the global file per (agent × harness) — O(agents × harnesses) full
  re-parses per run. Fine at ~8 agents; a scaling ceiling, not a bug.
- **Union-of-agent-keys generation** (by design): overriding one harness's agent still emits a
  built-in-pinned committed file for the other listed harnesses. Harmless and reproducibility-correct;
  a one-line convention note could preempt "why did claude get a file I didn't configure?"
---
