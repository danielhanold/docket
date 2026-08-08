---
id: 245
slug: harden-sync-agents-wrapper-generation-and-clear-the-0192-fin
title: 'Harden sync-agents wrapper generation and clear the 0192 findings'
status: in-progress
priority: medium
type: refactor
created: 2026-08-07
updated: 2026-08-08
depends_on: []
related: []
discovered_from: [141, 142, 196, 82]
adrs: []
spec: docs/superpowers/specs/2026-08-07-harden-sync-agents-wrapper-generation-and-clear-the-0192-fin-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: feat/harden-sync-agents-wrapper-generation-and-clear-the-0192-fin
pr:
blocked_by:
claimed_at: 2026-08-08T10:35:01Z
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-harden-sync-agents-wrapper-generation-and-clear-the-0192-fin-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-harden-sync-agents-wrapper-generation-and-clear-the-0192-fin-design.md) |
<!-- docket:artifacts:end -->

## Why

Consolidates #0141, #0142, #0196, and #0082 (2026-08-07 triage): four changes in the same `sync-agents.sh` wrapper-generation pass, same ADR-0060 territory, that should land as one branch.

Verified 2026-08-07:

- **Emitter duplication (#0141) — trigger condition has fired.** There are now **three** named emitters, not the two the stub argued about: `emit_codex_toml` (sync-agents.sh:769), `emit_cursor_md` (:828), `emit_opencode_md` (:884, added by 0192/0205) — each repeating byte-identical parse lines for `desc`, model/effort, the `inherit` normalization, `skills_csv`, and `body`. No `parse_wrapper_source` exists. A parse fix can now miss two twins.
- **Silent unmapped-token gap (#0142).** `emit_for_harness`'s `*)` catch-all (sync-agents.sh:744-757) still emits a Claude-shaped wrapper for accepted-but-unmapped tokens (`agents`, `kiro`, `windsurf` — all still in `DOCKET_GI_HARNESS_TOKENS`, docket-gitignore-block.sh:9) with only a source comment, no WARN, no `--check` surfacing.
- **0192's unfixed review findings (#0196), all still present:** `docs/codex/setup.md:52-54` says de-listing Codex "removes" the AGENTS.md dispatch block with no last-harness caveat (opencode's setup.md:59-62 has the correct wording); no two-dispatch-harness fixture exists anywhere in `tests/`; sync-agents.sh:1115 overreaches ("on every harness it supports"); the :1341 diagnostic hand-lists `(codex, opencode)` next to the `AGENTS_MD_DISPATCH_HARNESSES` variable; no body/preamble assert on the opencode emitter; the effort-drop-when-no-model claim (:875-884, :908) has no probe test.
- **Global-harness silent no-op (#0082).** `per_repo_opted_in()` (sync-agents.sh:222-243) reads only repo-level config; `project_level_pass` (:1267) returns silently for a repo without `agent_harnesses:`, so a user-level `agent_harnesses` produces no per-repo wrappers and no hint. No "add `agent_harnesses:` to `.docket.local.yml`" advisory string exists.

## What changes

Design settled 2026-08-07 (auto-groom; see the linked spec's Assumptions for every default taken and rejected):

- Factor `parse_wrapper_source()` out of the three emitters — fixed `WSRC_*` result globals (the `RES_*` house pattern); serialization, skills-preamble sentences, and the deliberate per-emitter `inherit`/`auto` sentinel handling stay put. Byte-identity is the gate: build-time snapshot diff across all four harnesses plus the existing generation suites, unmodified.
- Make the unmapped-token path loud: a shared `harness_has_named_emitter()` predicate; the `*)` arm WARNs once per harness per run, and `--check` gains a non-failing advisory leg. Behavioral test pins predicate/dispatch agreement. (Removing the `agents` token from the vocabulary is a separate decision — see Out of scope.)
- Clear the four Important 0192 findings plus the minors: codex setup.md gets opencode's last-dispatch-harness caveat; a codex+opencode two-harness fixture lands in `test_sync_agents_opencode.sh` (block exactly once; last-harness strip); the :1341 diagnostic interpolates `AGENTS_MD_DISPATCH_HARNESSES`; the :1115 head claim is scoped to the shipped harnesses (an accepted committed-bytes change); the opencode emitter gains body/preamble asserts; the effort-drop claim gets a generation test plus a live probe that degrades to a comment reword if the opencode CLI is absent; the agent-layer.md table's opencode cell drops to a bare `.md`.
- Add the #0082 advisory (option 1 of its stub, **scope pinned here**): when the global layer sets `agent_harnesses` but the repo has not opted in, `project_level_pass` prints one XDG-aware hint naming `.docket.local.yml` before its silent early return. Generation path only.

## Out of scope

- #0082's options 2/3 (letting global config drive per-repo artifacts) — they reopen ADR-0019's coordination-key fence and change 0050's determinism argument. Advisory only.
- Removing `agents`/`kiro`/`windsurf` from the token vocabulary (gitignore-block consequences; separate decision if wanted).
- Hard-refusal posture for unmapped tokens — WARN is the default this change ships.

## Reconcile log

### 2026-08-08 — reconcile at claim (implement-next)

Re-read the change + its spec against current `origin/main` (`cbca5feb`), the cited ADRs (0015, 0019, 0059, 0060), and the code. **All four defect families are still present and unfixed; the design stands unchanged.** Scope, out-of-scope, and every spec assumption survive; nothing was dropped or added.

The only drift is **line numbers** — the spec and the `## Why` above cite pre-0205/0220 offsets. Current, verified positions in `sync-agents.sh` (1636 lines; the file is at the **repo root**, not under `scripts/`):

| Spec cite | Current | Subject |
|---|---|---|
| :769 | **:858** | `emit_codex_toml` |
| :828 | **:917** | `emit_cursor_md` |
| :884 | **:973** | `emit_opencode_md` |
| :744-757 / :750-755 | **:833-846** | `emit_for_harness`, whose `*)` arm is at **:844** |
| :1117 | **:1206** | the AGENTS.md head "on every harness it supports" overreach |
| :1342 | **:1431** | the hand-listed `(codex, opencode)` `--check` diagnostic |
| :1267 | **:1355** | `project_level_pass` (its silent `project_wrappers_generated \|\| return 0` is :1356) |
| :222-243 | **:222-230** | `per_repo_opted_in` (unchanged) |
| :875-884, :908 | **:993-998** | the opencode effort-drop branch |
| :262 | **:262** | `AGENTS_MD_DISPATCH_HARNESSES="codex opencode"` (unchanged) |

Two further cites confirmed unchanged: a second hand-written `(codex, opencode)` sits in the **comment** at :1225 (the spec names only the :1431 diagnostic — the comment is prose about the block, not an emitted string, so it stays out of scope); `docs/codex/setup.md:52` still lacks the last-harness caveat that `docs/opencode/setup.md:59-62` carries; `skills/docket-convention/references/agent-layer.md:131` still holds the full path where its three siblings hold a bare extension.

No `parse_wrapper_source` and no `harness_has_named_emitter` exist. The three emitters still repeat the `name:`/`description:`/`skills:`/body parse lines verbatim. `emit_for_harness`'s `*)` arm still emits silently with only a comment. `project_level_pass` still returns wordlessly for a non-opted-in repo. Assumption 11 (no new couplings) re-verified: `depends_on` stays empty; no in-flight branch touches these emitters.

Eight `test_sync_agents*` suites exist (`.sh`, `_codex`, `_cursor`, `_defaults`, `_drift_docs`, `_opencode`, `_runners`, `_validator`) — the byte-identity gate's "existing generation suites, unmodified" is well-defined.
