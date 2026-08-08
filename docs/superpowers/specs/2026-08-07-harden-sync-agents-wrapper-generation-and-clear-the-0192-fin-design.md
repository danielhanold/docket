<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0245 — Harden sync-agents wrapper generation and clear the 0192 findings](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-08-0245-harden-sync-agents-wrapper-generation-and-clear-the-0192-fin.md)**
<!-- docket:backlink:end -->

# Harden sync-agents wrapper generation and clear the 0192 findings — design

**Change:** 0245 · **Date:** 2026-08-07 · **Type:** refactor (consolidates killed #0141, #0142, #0196, #0082)

## Problem

Four verified defects in one `sync-agents.sh` territory (ADR-0060):

1. Three emitters — `emit_codex_toml` (:769), `emit_cursor_md` (:828), `emit_opencode_md` (:884) —
   repeat byte-identical parse lines for `name`, `desc`, `skills_csv`, and `body`. A parse fix can
   silently miss two twins (learnings: `escape-ere-metacharacters-in-key`).
2. `emit_for_harness`'s `*)` arm (:750-755) emits a Claude-shaped wrapper for accepted-but-unmapped
   tokens (`agents`, `kiro`, `windsurf`) with only a source comment — no WARN, no `--check` signal.
3. 0192's unfixed review findings (#0196): the codex setup.md de-list note lacks the last-harness
   caveat; no two-dispatch-harness fixture anywhere; the shared block head overreaches ("on every
   harness it supports", :1117); the `--check` diagnostic hand-lists "(codex, opencode)" (:1342);
   no body/preamble assert on the opencode emitter; the effort-drop-when-no-model rationale never
   probed; the agent-layer table's opencode cell holds a full path where siblings hold `.md`/`.toml`.
4. Global `agent_harnesses` with no repo opt-in is a silent no-op (`project_level_pass` :1267
   returns without a word) — #0082, scope pinned to the advisory (option 1).

## Design

### 1. `parse_wrapper_source()` — shared parse, per-emitter serialization

One helper, defined above the emitter registry, that reads the wrapper source and sets a fixed
global result set (the repo's `RES_*` house pattern; bash-3.2-safe, no namerefs, no subshell):

```
parse_wrapper_source(){  # $1=src md -> sets WSRC_NAME WSRC_DESC WSRC_SKILLS_CSV WSRC_BODY
```

- `WSRC_NAME`: the `name:` frontmatter line, falling back to `docket-$(short_name "$src")` —
  exactly the two-line idiom the emitters share today. opencode ignores it; harmless.
- `WSRC_DESC`: `agent_description "$src"`.
- `WSRC_SKILLS_CSV`: the current `sed` bracket-strip pipeline, verbatim.
- `WSRC_BODY`: the current two-stage `awk` (post-frontmatter, leading blanks trimmed), verbatim.

All three named emitters call it and keep everything else: TOML vs YAML serialization, the
per-harness skills-preamble sentence (differs by one phrase — left per-emitter per
`consolidation-flattens-caller-variance`; templating it buys nothing and flattens the variance),
and the model/effort sentinel handling. The `inherit`/`auto` normalization is **explicitly out of
the helper**: codex tests `!= "inherit"` at emit position, cursor/opencode normalize up front, and
claude's `emit()` passes `inherit` through verbatim by documented design (0168 whole-branch review
IMPORTANT 2). `emit()` itself (the claude stream-transform) is untouched — it parses no fields.

**Gate:** byte-identity. At build time, snapshot every generated wrapper (all four harnesses, full
agent set, via a fixture repo) before the refactor and diff after — zero diffs. The existing
generation suites (`test_sync_agents.sh`, `_codex`, `_cursor`, `_opencode`) must stay green
unmodified. No committed byte-snapshot test is added; field-level asserts remain the durable guard.

### 2. Loud unmapped tokens

- New predicate next to the dispatch: `harness_has_named_emitter()` — case over
  `claude|codex|cursor|opencode`. Both new call sites use it so the boundary is named once:
  - `emit_for_harness`'s `*)` arm calls a new `warn_unmapped_harness "$2"` which `log`s
    (stderr — emitter stdout is redirected into the wrapper file) **once per harness per run**
    (dedup via a space-separated `WARNED_UNMAPPED` global; per-wrapper would print ~17 lines per
    token). Message: wrapper is Claude-shaped and unverified for `<h>`; pins may not be honored
    (ADR-0060); give the harness a named emitter or accept the unverified shape.
  - `check_project_level` gains an **advisory** leg (rc unchanged): for each token in `HARNESSES`
    failing `harness_has_named_emitter`, print the same substance prefixed `advisory:`.
- Against predicate/dispatch drift (learnings: `duplicated-gate-copies-the-whole-predicate`), a
  test pins agreement behaviorally: each named harness generates without the WARN; a fixture
  listing `kiro` warns exactly once at generation and surfaces in `--check` with exit 0.
- WARN is the shipped posture; hard-refusal and token-vocabulary removal stay out of scope (stub).

### 3. Clear the 0192 findings

- **codex setup.md de-list note:** adopt opencode setup.md's last-dispatch-harness wording
  (block removed only when the last of codex/opencode is de-listed).
- **Two-dispatch-harness fixture** (in `tests/test_sync_agents_opencode.sh`, where the shared-block
  guards live): `agent_harnesses: [codex, opencode]` → the dispatch block appears **exactly once**;
  de-list codex → block remains; de-list opencode too → block removed. This is the discriminating
  fixture single-owner tests cannot reach (`shared-resource-keeps-first-owner-assumptions`).
- **:1117 head overreach:** reword "on every harness it supports" to scope the pinned-out-of-the-box
  claim to the harnesses docket ships defaults for (claude, cursor, codex, opencode). The block is
  committed into consumer repos, so this changes committed bytes: accepted — `--check` flags the
  staleness and the next run rewrites it, the designed refresh path.
- **:1342 diagnostic:** derive the harness list from `$AGENTS_MD_DISPATCH_HARNESSES`
  (`sed 's/ /, /g'` for the comma join) instead of the hand-written "(codex, opencode)".
- **opencode body/preamble asserts:** extend `test_sync_agents_opencode.sh` to assert the emitted
  body is non-empty and carries the skills preamble — matching the codex/cursor emitters' coverage,
  so a regression to an empty prompt goes red.
- **Effort-drop claim (:875-884, :908):** add a generation test pinning docket's side — model
  unresolved + effort set emits the WARN and **no** `reasoningEffort:` key. The live rationale
  ("a provider option with no provider selected has nothing to reach") gets a build-time probe via
  `opencode debug agent` **if** the opencode CLI is available on the build machine; if not, reword
  the comment to mark that clause as docket's design choice rather than verified opencode behavior
  (the `options.reasoningEffort` forwarding claim itself stays — it was verified against 1.18.11).
- **Agent-layer table cell:** `skills/docket-convention/references/agent-layer.md:128` — replace
  the opencode cell's full path with the bare `.md` its three siblings use.

### 4. The #0082 advisory

In `project_level_pass`, before the `project_wrappers_generated || return 0` early return becomes
silent: when `USER_HARNESSES_SET=1` and `! per_repo_opted_in`, `log` one hint —

> global agent_harnesses is set (`${XDG_CONFIG_HOME:-~/.config}/docket/config.yml`) but this repo
> has not opted in, so no per-repo wrappers were generated. To opt in, add `agent_harnesses:` to
> `.docket.local.yml` (machine-local) or `.docket.yml` (committed).

Generation path only — the placement #0082's pinned option 1 asks for ("running sync-agents.sh
generates nothing and prints no explanation"). A `--check` copy would be cheap
(`resolve_global_agent_harnesses` is lazily resolved on that path too, via `validate_runner_config`
:1510 → :673, so `USER_HARNESSES_SET` is available in `check_project_level`), but is deliberately
not added: one copy of the hint, at the moment the user acted and the no-op bit. `--check`'s
per-repo no-op is by-design for a non-opted-in repo, and its :1321 explanation line covers the
non-docket-mode case. Options 2/3 of #0082 stay rejected (ADR-0019 coordination-key fence; change
0050's determinism argument) — advisory only, no behavior change to what gets generated.

## Test plan

- Existing four generation suites green and unmodified (byte-identity gate for §1).
- New: unmapped-token WARN once + `--check` advisory exit 0 (§2); two-dispatch-harness fixture,
  block-exactly-once + last-harness strip (§3); opencode body/preamble asserts (§3); opencode
  effort-drop generation probe (§3); #0082 advisory fires exactly in the global-set/no-opt-in
  cell and stays silent in the other three cells (§4).
- Portability note: run new greps under `/usr/bin/grep` too (ugrep masks BSD limits — learnings).

## Assumptions

1. **Parse-helper return convention: fixed `WSRC_*` globals.** Rejected: stdout key=value output
   (subshell per call, escaping multi-line bodies is exactly the fragility being removed); bash
   namerefs (need 4.3+; change 0222's minimum-bash raise is still proposed, not landed). Globals
   match the existing `RES_*` / `resolve_agent_layers` house pattern.
2. **Helper scope: source-derived fields only; sentinel normalization stays per-emitter.** The
   claude-vs-others `inherit` asymmetry is documented deliberate behavior (0168 review); folding it
   into a shared spot is the exact regression that review caught. The skills-preamble sentence also
   stays per-emitter (one-word variance; `consolidation-flattens-caller-variance`).
3. **Byte-identity is verified by a build-time snapshot diff, not a committed snapshot test.**
   Rejected: committing golden files (they duplicate the field asserts and rot on every deliberate
   emitter change). The refactor is the one moment byte-identity is the property; after it lands,
   field asserts are the right durable guard.
4. **Unmapped-token posture: WARN once per harness per run + non-failing `--check` advisory.**
   Rejected: per-wrapper WARN (noise ×17 agents); failing `--check` (breaks existing repos listing
   such tokens, and the stub pins hard-refusal out of scope). Dedup state is a plain global — the
   emitters run in the main shell, not subshells.
5. **Predicate/dispatch agreement is guarded behaviorally, not structurally.** A shared
   `harness_has_named_emitter()` used by both sites plus a WARN/no-WARN test per token class;
   rejected: generating the `case` arms from a list (over-engineering a 4-entry case).
6. **The shared AGENTS.md head reword is a deliberate committed-bytes change.** Consumer repos see
   one `--check` staleness → re-run → commit cycle, the designed refresh path; rejected: keeping
   the overreaching claim to preserve bytes (a false claim, committed, is the worse artifact).
7. **The two-harness fixture lives in `test_sync_agents_opencode.sh`**, next to the existing
   shared-block guards; rejected: a new suite file (registration overhead for three asserts).
8. **The effort-drop live probe degrades to a comment reword** when the opencode CLI is absent at
   build time — the suite must not depend on a vendor binary; docket's own emitted behavior is
   pinned by test either way.
9. **#0082 advisory placement: inside `project_level_pass`'s opt-in gate, generation path only.**
   Rejected: also printing it on `--check`. A `--check` copy would be technically free
   (`USER_HARNESSES_SET` is already resolved there, lazily via validate_runner_config :1510→:673),
   and in a docket-mode repo `--check` does reach a silent per-repo return (:1321's explanation is
   gated on `gitignore_block_wanted`, which a docket branch satisfies) — but the stub pins option 1
   to the generation-time no-op, and one authoritative copy of the hint beats two drifting ones.
   Escalating to a `--check` copy is a cheap follow-up if the silence still bites.
10. **The agent-layer table-cell minor is in scope** even though 0245's own bullet list omits it:
    it is one of consolidated #0196's enumerated minors, same file territory, one-cell edit.
11. **No new frontmatter couplings.** `depends_on` stays empty (nothing here waits on another
    change); active 0140 (runner-adapter sentinel normalization) touches `scripts/runners/*`, not
    these emitters — no link written. No in-flight feature worktree carries sync-agents.sh edits
    (verified against `origin/main..HEAD` and working trees in all four `.worktrees/` checkouts).
12. **`--check` may surface an unmapped token twice** — leg (c)'s emit_wrapper call reaches the
    `*)` arm's once-per-harness WARN, and the new advisory leg prints its own line. Accepted:
    same substance, each deduped, rc unaffected; suppressing one would couple the legs.
13. **The `*)` WARN also fires on the user-level pass** (e.g. a presence-detected `~/.kiro/agents`
    with no repo config). Accepted: the loud-gap intent is exactly that the unverified shape never
    generates silently, whichever pass emits it.
