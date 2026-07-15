# Codex harness — TOML agent generation + AGENTS.md dispatch block — results

Change: #77 · Branch: feat/codex-harness-toml-agents · PR: (see change `pr:`) · Plan: docs/superpowers/plans/2026-07-15-codex-harness-toml-agents.md · ADRs: 15, 36

## Verify (human)

<!-- Automated tests fully cover generation, prune, --check, and TOML validity (Python tomllib parsed all 9 wrappers clean). These are the checks the suite cannot make. -->
- [ ] **Live Codex CLI** actually loads a generated `.codex/agents/docket-*.toml` and honors its `model` / `model_reasoning_effort` — this is change **#0078** (depends on #77), NOT verified here. Field names/paths/extension were re-verified against the live doc (learn.chatgpt.com/docs/agent-configuration/subagents) at plan time, but no Codex binary was run.
- [ ] **Codex honors the committed `AGENTS.md` dispatch block** — that a directly-invoked `docket-*` skill actually delegates to the matching agent rather than running inline. Also #0078; the block is a best-effort instruction whose efficacy only live Codex can confirm.
- [ ] On a repo opting into `agent_harnesses: [codex]`, confirm `AGENTS.md` reads sensibly to a human teammate (it is committed and team-visible) — smoke output looks correct (9 agents, machine-neutral, no model IDs).

## Findings

- **Codex TOML shape confirmed against live docs (2026-07-15):** `.toml` files in `~/.codex/agents/` (personal) and `<repo>/.codex/agents/` (project); required `name` / `description` / `developer_instructions`; optional `model` / `model_reasoning_effort`. docket's effort values `low`/`medium`/`high`/`xhigh`/`max` are all valid `model_reasoning_effort` values, so ADR-0015's verbatim passthrough holds — no translation layer needed. The spec's open question is resolved; design unchanged.
- **ADR-0036** records the one non-obvious decision: the `AGENTS.md` dispatch block is **committed and machine-neutral** (agent names + delegation prose only, never a model ID), a deliberate departure from ADR-0020's gitignored/machine-local generated-artifact regime — modeled on the committed managed `.gitignore` block and maintained with the same hardened managed-block machinery. The `.toml` wrappers themselves stay machine-local (they bake model IDs).
- **Latent body-extraction bug found + fixed in review** (commit `bdeabce`): `emit_codex_toml`'s original body-extraction awk incremented its frontmatter-fence counter unconditionally, so any bare `---` thematic-break line inside a wrapper body would have been silently dropped from `developer_instructions`. No current wrapper body triggers it, but it was latent silent data loss; gated to `d<2` and covered by a direct regression test (which required making `sync-agents.sh` sourceable via a standard main-guard).
- **Test discipline:** the final whole-branch review independently mutation-tested nine guards — all went red when their feature was stripped. Two test-coverage gaps the per-task reviews left (a strip-preserves-outside-bytes assert; the codex-disabled-leftover-block `--check` branch) were closed in the final fix wave (commit `48360e9`), each mutation-confirmed.

## Follow-ups

- **M2** — `remove_managed_block` leaves a 1-byte lone-newline file when the managed block was the file's only content (cosmetic; the identical shape exists in the proven `.gitignore` path). Harmless — Codex reads an empty file and `--check` is unaffected.
- **N1** — because `tracked_docket_files` / the prune globs now use `harness_ext` (= `toml` for codex), a *tracked* or stale machine-local pre-0077 `.codex/agents/docket-*.md` wrapper is no longer flagged/pruned. These are inert (Codex ignores `.md`; the `.gitignore` block still ignores `.codex/agents/docket-*.md`) and the scenario is very-low-likelihood, but it is a small reduction in the pre-0077 migration safety net — a one-line legacy `.md` sweep would close it if wanted.
- **N3** — the `effort: auto` / `model: inherit` omission guards in `emit_codex_toml` have no on-disk test (no built-in wrapper carries those values; a fixture-based unit test could exercise them, now that the script is sourceable).
- **User-level `~/.codex/AGENTS.md`** dispatch (vs. the per-repo committed block) is deferred to #0078's live-Codex findings, per the spec.
