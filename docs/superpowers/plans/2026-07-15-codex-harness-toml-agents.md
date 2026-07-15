# Codex harness — TOML agent generation + AGENTS.md dispatch block — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `sync-agents.sh` generate valid Codex CLI **TOML** agent files (and a committed `AGENTS.md` dispatch block) for the `codex` harness, instead of the dead `.md` wrappers Codex ignores today.

**Architecture:** Add a tiny per-harness emitter registry to `sync-agents.sh` — `codex` maps to a `.toml` extension + an `emit_codex_toml()` transform; every other harness keeps the existing `.md` `emit()` byte-for-byte. Extend the gitignore-block lib to ignore the TOML wrappers and to expose a *generic* managed-block primitive (reusing its already marker-parameterized helpers), which a new AGENTS.md dispatch-block writer uses. Orphan pruning and both `--check` legs learn about `.toml` and the new committed AGENTS.md block.

**Tech Stack:** Bash 3.2-compatible shell (`set -euo pipefail`), awk/sed, the repo's hand-rolled `tests/test_*.sh` assertion harness (`bash tests/test_<name>.sh`, no framework, no CI). TOML is emitted/parsed with shell+awk only — no TOML library.

## Global Constraints

- **Bash 3.2 compatible** — no `declare -A`, no `${var^^}`, no process substitution where a pipe works; matches the rest of `sync-agents.sh`.
- **Non-codex output stays byte-identical** to pre-change — the `.md` `emit()` path and its call sites must not change their bytes. A regression assert guards this.
- **`model:`/`effort:` pass through verbatim** (ADR-0015) — never validate a model ID or effort value; the harness interprets it. docket's effort vocabulary `low`/`medium`/`high`/`xhigh`/`max` are all valid Codex `model_reasoning_effort` values.
- **Codex TOML facts (re-verified 2026-07-15 against https://learn.chatgpt.com/docs/agent-configuration/subagents):** files are `.toml` in `~/.codex/agents/` (personal) and `<repo>/.codex/agents/` (project); required fields `name`, `description`, `developer_instructions`; optional `model` (inherits from parent if omitted) and `model_reasoning_effort`. `sandbox_mode`/`mcp_servers`/`skills.config`/`nickname_candidates` exist but are **out of scope** (not emitted).
- **The AGENTS.md dispatch block is COMMITTED and machine-neutral** — agent names + delegation prose only, **never a model ID** (pins live in the gitignored TOML). It is modeled on the committed managed `.gitignore` block, NOT the gitignored wrappers — a deliberate departure from ADR-0020's machine-local regime.
- **Markers are hardened managed blocks** — closed-block guard (refuse on malformed/dangling/out-of-order markers, touch nothing), idempotence (no write when current), outside bytes preserved verbatim. Reuse the `_docket_gi_*` primitives in `scripts/lib/docket-gitignore-block.sh`.
- **A guard is code — mutation-test every assert** (LEARNINGS, the dominant recurring failure): after writing a test, strip the feature it guards and confirm the assert goes RED. An assert that stays green over a broken feature is decoration and must be rewritten. Watch specifically for: wrong anchor, double-guarded greps, asserting on a string the producer never emits, and `! grep "$FLAG"` where `$FLAG` starts with `-` (grep parses it as an option, exits 2, `!` flips to green).
- **No live Codex CLI** is needed or invoked — TOML validity is asserted with a minimal shell/awk field extractor, not a TOML library.

---

## File Structure

- `scripts/lib/docket-gitignore-block.sh` (modify) — (a) add the `.codex/agents/docket-*.toml` ignore line to the constant block; (b) add generic `ensure_managed_block()` / `remove_managed_block()` primitives (thin wrappers over the existing `_docket_gi_malformed` / `_docket_gi_strip_block` / `_docket_gi_current_block`, which are already marker-parameterized). `ensure_docket_gitignore_block()` stays byte-identical (unchanged) to protect its proven path.
- `sync-agents.sh` (modify) — the per-harness emitter registry (`harness_ext`, `emit_for_harness`, `emit_codex_toml`), wire both generation passes, extend `tracked_docket_files` / `prune_orphans` / `check_project_level` for `.toml`, and add the AGENTS.md dispatch-block assembler + write/strip + `--check` leg.
- `.gitignore` (modify, repo root, on the feature/main line) — regenerate docket's own committed block to include the new `.codex/agents/docket-*.toml` line, so `sync-agents.sh --check` leg (a) stays green.
- `tests/test_docket_gitignore_block.sh` (modify) — assert the block now contains the TOML pattern.
- `tests/test_sync_agents_codex.sh` (create) — all codex-specific cases: TOML field mapping, model/effort passthrough, `effort: auto` / inherit omission, byte-identical non-codex regression, `.toml` orphan prune, `--check` leg behavior, and the AGENTS.md dispatch block (create / idempotent / outside-bytes-preserved / malformed-refusal / prune-on-delist / `--check` presence-currency). Reuses the sandbox helper pattern from `tests/test_sync_agents.sh`.

Note: `docket-gitignore-block.sh` is a `lib/` helper (exempt from `test_script_contracts_coverage.sh`, which is top-level-`scripts/*.sh` only) and `sync-agents.sh` is a repo-root script (also not under `scripts/`), so **no new `.md` contract file is required**.

---

### Task 1: `.gitignore` block ignores the Codex TOML wrappers

**Files:**
- Modify: `scripts/lib/docket-gitignore-block.sh` (function `emit_docket_gitignore_block`, ~lines 25-33)
- Modify: `.gitignore` (repo root — regenerate docket's committed block)
- Test: `tests/test_docket_gitignore_block.sh`

**Interfaces:**
- Consumes: `DOCKET_GI_HARNESS_TOKENS`, `DOCKET_GI_DISPATCH_HARNESSES` (existing lib constants).
- Produces: `emit_docket_gitignore_block` now additionally prints the constant line `.codex/agents/docket-*.toml`. Still a pure, config-independent constant (deterministic).

- [ ] **Step 1: Write the failing test**

Add near the other `emit:` assertions in `tests/test_docket_gitignore_block.sh` (after line 22):

```bash
assert "emit: codex TOML wrapper pattern"   'printf "%s\n" "$BLK" | grep -qxF ".codex/agents/docket-*.toml"'
assert "emit: codex .md pattern still present (constant, all tokens)" 'printf "%s\n" "$BLK" | grep -qxF ".codex/agents/docket-*.md"'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_docket_gitignore_block.sh 2>&1 | grep -i codex`
Expected: `NOT OK - emit: codex TOML wrapper pattern` (the `.toml` line is not emitted yet); the `.md` assertion already passes.

- [ ] **Step 3: Write minimal implementation**

In `scripts/lib/docket-gitignore-block.sh`, `emit_docket_gitignore_block()`, add the TOML line between the `.md` loop and the dispatch `.mdc` loop:

```bash
emit_docket_gitignore_block(){
  local e tok
  printf '%s\n' "$DOCKET_GI_START"
  for e in $DOCKET_GI_CORE_ENTRIES; do printf '%s\n' "$e"; done
  printf '.docket.local.yml\n'
  for tok in $DOCKET_GI_HARNESS_TOKENS;   do printf '.%s/agents/docket-*.md\n' "$tok"; done
  printf '.codex/agents/docket-*.toml\n'
  for tok in $DOCKET_GI_DISPATCH_HARNESSES; do printf '.%s/rules/docket-dispatch.mdc\n' "$tok"; done
  printf '%s\n' "$DOCKET_GI_END"
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_docket_gitignore_block.sh`
Expected: all `ok - ...`, exit 0 (both new codex assertions pass; every prior assertion still passes — the `.md` per-line greps and the idempotence check are unaffected by an added line).

- [ ] **Step 5: Regenerate docket's own committed `.gitignore`**

The committed block is now stale vs. the constant. Regenerate it in place (insert the TOML line at the same position as the emitter):

Edit `.gitignore` — add `.codex/agents/docket-*.toml` on its own line immediately after `.windsurf/agents/docket-*.md` and before `.cursor/rules/docket-dispatch.mdc`.

Verify it matches the emitter exactly:

```bash
diff <(bash -c '. scripts/lib/docket-gitignore-block.sh; emit_docket_gitignore_block') \
     <(awk '/^# docket:start/{f=1} f{print} /^# docket:end/{f=0}' .gitignore)
```
Expected: no output (committed block == emitter bytes).

- [ ] **Step 6: Mutation-test the guard, then commit**

Mutation check: temporarily delete the new `printf '.codex/agents/docket-*.toml\n'` line, run `bash tests/test_docket_gitignore_block.sh` → the codex-TOML assert must go RED. Restore the line.

```bash
git add scripts/lib/docket-gitignore-block.sh .gitignore tests/test_docket_gitignore_block.sh
git commit -m "feat(0077): ignore .codex/agents/docket-*.toml in the managed gitignore block"
```

---

### Task 2: Per-harness emitter registry + `emit_codex_toml`, wired into both passes

**Files:**
- Modify: `sync-agents.sh` — add `harness_ext`, `emit_for_harness`, `emit_codex_toml`, `toml_escape_basic`; change the two write sites in `user_level_pass` (~line 447) and `project_level_pass` (~line 487).
- Test: `tests/test_sync_agents_codex.sh` (create)

**Interfaces:**
- Consumes: `AGENTS_SRC`, `short_name`, `agent_description`, resolved `RES_MODEL`/`RES_EFFORT` (existing).
- Produces:
  - `harness_ext HARNESS` → prints `toml` for `codex`, else `md`.
  - `emit_for_harness SRC_MD HARNESS MODEL EFFORT` → stdout; dispatches to `emit_codex_toml` for codex, else the existing `emit`. `MODEL`/`EFFORT` are the resolved *overrides* (empty ⇒ keep built-in), exactly as `emit`.
  - `emit_codex_toml SRC_MD MODEL_OVERRIDE EFFORT_OVERRIDE` → stdout; a complete TOML agent document. Field mapping: frontmatter `name:`→`name`, `description:`→`description`, effective model→`model` (omit when empty/`inherit`), effective effort→`model_reasoning_effort` (omit when empty/`auto`), body prose + skills-preload preamble→`developer_instructions` (multi-line basic string).

- [ ] **Step 1: Write the failing test**

Create `tests/test_sync_agents_codex.sh`:

```bash
#!/usr/bin/env bash
# tests/test_sync_agents_codex.sh — Codex harness TOML generation + AGENTS.md dispatch (change 0077).
# run: bash tests/test_sync_agents_codex.sh
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SYNC="$REPO/sync-agents.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Minimal TOML top-level scalar reader: prints the value (unquoted) of a bare `key = "..."`.
# Good enough for name/description/model/model_reasoning_effort (single-line basic strings).
toml_get(){ sed -n -E 's/^'"$2"'[[:space:]]*=[[:space:]]*"(.*)"[[:space:]]*$/\1/p' "$1" | head -n1; }
toml_has_key(){ grep -qE "^$2[[:space:]]*=" "$1"; }

# Opt a sandbox repo into [claude, codex] and generate.
mk_codex_repo(){
  SBX="$(mktemp -d)"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
  printf 'agent_harnesses: [claude, codex]\n' > "$SBX/.docket.yml"
  ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
}

# --- codex per-repo pass writes TOML wrappers, not .md ---
mk_codex_repo
assert "codex: writes .codex/agents/docket-status.toml" '[ -f "$SBX/.codex/agents/docket-status.toml" ]'
assert "codex: does NOT write a codex .md wrapper"       '[ ! -f "$SBX/.codex/agents/docket-status.md" ]'
assert "codex: full built-in set as TOML (9 files)"      '[ "$(find "$SBX/.codex/agents" -name "docket-*.toml" | wc -l | tr -d " ")" = "9" ]'

T="$SBX/.codex/agents/docket-status.toml"
assert "codex TOML: name = docket-status"          '[ "$(toml_get "$T" name)" = "docket-status" ]'
assert "codex TOML: description matches source"    '[ "$(toml_get "$T" description)" = "$(sed -n "/^description:/{s/^description:[[:space:]]*//;p;q;}" "$REPO/agents/docket-status.md")" ]'
assert "codex TOML: model = built-in claude id"    '[ "$(toml_get "$T" model)" = "claude-haiku-4-5-20251001" ]'
assert "codex TOML: model_reasoning_effort = medium" '[ "$(toml_get "$T" model_reasoning_effort)" = "medium" ]'
assert "codex TOML: has developer_instructions"    'grep -qE "^developer_instructions[[:space:]]*=" "$T"'
assert "codex TOML: dev_instructions carry body"   'grep -qi "refresh docket state" "$T"'
assert "codex TOML: dev_instructions name the skills to load" 'grep -qi "docket-convention" "$T"'
# claude side is untouched and still markdown
assert "claude side still .md, byte-identical to source" 'diff -q "$REPO/agents/docket-status.md" "$SBX/.claude/agents/docket-status.md" >/dev/null'
rm -rf "$SBX"

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_sync_agents_codex.sh`
Expected: FAIL — `.codex/agents/docket-status.toml` is not produced (the pass still writes `docket-status.md`), so the first assertion and all TOML-field assertions fail.

- [ ] **Step 3: Write minimal implementation**

In `sync-agents.sh`, add the registry + emitter helpers (place after `emit()`, ~line 329):

```bash
# --- per-harness emitter registry (change 0077) ------------------------------
# Map a harness token to the on-disk extension docket generates for it.
harness_ext(){ case "$1" in codex) printf 'toml';; *) printf 'md';; esac; }

# Dispatch to the harness-appropriate emitter. MODEL/EFFORT are resolved OVERRIDES
# (empty => keep the built-in), identical in meaning to emit()'s args.
emit_for_harness(){  # $1=src md  $2=harness  $3=model  $4=effort
  case "$2" in
    codex) emit_codex_toml "$1" "$3" "$4";;
    *)     emit "$1" "$3" "$4";;
  esac
}

# Escape a value for a TOML basic (double-quoted) string: backslash then double-quote.
toml_escape_basic(){ printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'; }

# Transform a built-in markdown wrapper into a Codex TOML agent document on stdout.
# Field mapping (ADR-0015 verbatim passthrough for model/effort):
#   frontmatter name:        -> name
#   frontmatter description: -> description
#   effective model          -> model                  (override||built-in; omit if empty/inherit)
#   effective effort         -> model_reasoning_effort  (override||built-in; omit if empty/auto)
#   skills: preload + body   -> developer_instructions  (multi-line basic string)
emit_codex_toml(){  # $1=src md  $2=model_override  $3=effort_override
  local src="$1" mo="$2" eo="$3"
  local name desc bi_model bi_effort model effort skills_csv body dev esc
  name="$(sed -n '/^name:/{s/^name:[[:space:]]*//;p;q;}' "$src")"
  [ -n "$name" ] || name="docket-$(short_name "$src")"
  desc="$(agent_description "$src")"
  bi_model="$(sed -n '/^model:/{s/^model:[[:space:]]*//;p;q;}' "$src")"
  bi_effort="$(sed -n '/^effort:/{s/^effort:[[:space:]]*//;p;q;}' "$src")"
  model="${mo:-$bi_model}"
  effort="${eo:-$bi_effort}"
  skills_csv="$(sed -n '/^skills:/{s/^skills:[[:space:]]*//;p;q;}' "$src" | sed -e 's/^\[//' -e 's/\][[:space:]]*$//' -e 's/[[:space:]]*$//')"
  # body = everything after the frontmatter closing --- , leading blank lines trimmed.
  body="$(awk '/^---[[:space:]]*$/{d++; next} d>=2{print}' "$src" | awk 'NF{p=1} p{print}')"
  # developer_instructions text: skills-preload preamble (if any) + the wrapper body.
  if [ -n "$skills_csv" ]; then
    dev="Before acting, load these docket skills from your linked Codex skills directory: ${skills_csv}.

${body}"
  else
    dev="$body"
  fi
  # Emit TOML.
  printf 'name = "%s"\n' "$(toml_escape_basic "$name")"
  printf 'description = "%s"\n' "$(toml_escape_basic "$desc")"
  if [ -n "$model" ] && [ "$model" != "inherit" ]; then
    printf 'model = "%s"\n' "$(toml_escape_basic "$model")"
  fi
  if [ -n "$effort" ] && [ "$effort" != "auto" ]; then
    printf 'model_reasoning_effort = "%s"\n' "$(toml_escape_basic "$effort")"
  fi
  # Multi-line basic string. Escape backslashes; defend against a literal """ terminator
  # (built-in bodies have neither, but keep the emitter robust). Closing """ on its own line.
  esc="$(printf '%s' "$dev" | sed -e 's/\\/\\\\/g' -e 's/"""/""\\"/g')"
  printf 'developer_instructions = """\n%s\n"""\n' "$esc"
}
```

Change the write site in `user_level_pass` (was `emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$dir/$(basename "$src")"`):

```bash
      mkdir -p "$dir"
      emit_for_harness "$src" "$harness" "$RES_MODEL" "$RES_EFFORT" > "$dir/docket-$name.$(harness_ext "$harness")"
```

Change the write site in `project_level_pass` (was `emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$dir/docket-$name.md"`):

```bash
      dir="$REPO/.$harness/agents"
      mkdir -p "$dir"
      emit_for_harness "$src" "$harness" "$RES_MODEL" "$RES_EFFORT" > "$dir/docket-$name.$(harness_ext "$harness")"
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_sync_agents_codex.sh`
Expected: all `ok - ...`, `ALL PASS`, exit 0.

- [ ] **Step 5: Run the full sync-agents test to confirm the non-codex path is byte-identical**

Run: `bash tests/test_sync_agents.sh`
Expected: all `ok - ...`, exit 0 — in particular "no override => byte-identical to built-in source" still passes (the `.md` `emit()` path and its `docket-$name.md` filename are unchanged; only the codex branch is new).

- [ ] **Step 6: Mutation-test, then commit**

Mutation check: in `emit_codex_toml`, temporarily hardcode `model="WRONG"` → the "model = built-in claude id" assert must go RED. Restore. Temporarily map `harness_ext codex` to `md` → the "writes .codex/agents/docket-status.toml" assert must go RED. Restore.

```bash
git add sync-agents.sh tests/test_sync_agents_codex.sh
git commit -m "feat(0077): emit Codex .toml agents via a per-harness emitter registry"
```

---

### Task 3: `.toml` lifecycle in orphan-prune and `--check`

**Files:**
- Modify: `sync-agents.sh` — `tracked_docket_files` (~line 381), `prune_orphans` (~lines 594-644), `check_project_level` leg (c) content-staleness (~lines 524-540).
- Test: `tests/test_sync_agents_codex.sh` (extend)

**Interfaces:**
- Consumes: `harness_ext`, `HARNESSES`, `VALID_HARNESS_TOKENS`, `AGENTS_SRC`, `harness_of_dir` (existing).
- Produces: prune/track/check now recognize `.codex/agents/docket-*.toml` as docket-owned — a removed built-in or a de-listed `codex` drops its `.toml` wrappers; `--check` leg (b) reports a tracked `.toml`; leg (c) diffs regenerated TOML against the on-disk `.toml`.

- [ ] **Step 1: Write the failing tests** (append before the final summary in `tests/test_sync_agents_codex.sh`)

```bash
# --- orphan prune: a removed built-in drops its codex .toml wrapper ---
mk_codex_repo
cp "$REPO/agents/docket-status.md" "$SBX/.codex/agents/docket-ghost.toml"   # simulate a stale wrapper
touch "$SBX/.codex/agents/docket-ghost.toml"
# regenerate: docket-ghost has no built-in source -> must be pruned
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "prune: orphan .toml wrapper removed" '[ ! -e "$SBX/.codex/agents/docket-ghost.toml" ]'
assert "prune: real .toml wrapper kept"      '[ -f "$SBX/.codex/agents/docket-status.toml" ]'
rm -rf "$SBX"

# --- de-list codex: its .toml wrappers are pruned ---
mk_codex_repo
assert "pre: codex wrappers exist" '[ -f "$SBX/.codex/agents/docket-status.toml" ]'
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "delist: codex .toml wrappers pruned" '[ ! -e "$SBX/.codex/agents/docket-status.toml" ]'
assert "delist: claude wrappers remain"      '[ -f "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX"

# --- --check leg (b): a TRACKED codex .toml is CI-meaningful (exit non-zero) ---
mk_codex_repo
git -C "$SBX" add -f .codex/agents/docket-status.toml
if ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check >/dev/null 2>&1 ); then
  echo "NOT OK - check: tracked .toml wrapper fails --check"; fail=1
else
  echo "ok - check: tracked .toml wrapper fails --check"
fi
rm -rf "$SBX"
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `bash tests/test_sync_agents_codex.sh 2>&1 | grep -E 'prune|delist|check'`
Expected: `prune: orphan .toml wrapper removed` FAILS (prune only scans `docket-*.md`), `delist` FAILS, `check: tracked .toml` FAILS (`tracked_docket_files` only lists `.md`).

- [ ] **Step 3: Write minimal implementation**

`tracked_docket_files` — add the codex `.toml` glob:

```bash
tracked_docket_files() {
  git -C "$REPO" rev-parse --is-inside-work-tree >/dev/null 2>&1 || return 0
  local tok
  {
    for tok in $VALID_HARNESS_TOKENS; do
      git -C "$REPO" ls-files -- ".$tok/agents/docket-*.$(harness_ext "$tok")" 2>/dev/null
    done
    for tok in $HARNESS_HAS_DISPATCH_RULES; do
      git -C "$REPO" ls-files -- ".$tok/rules/docket-dispatch.mdc" 2>/dev/null
    done
  } | sort -u
}
```

`prune_orphans` — the removed-builtin scan (section 1) derives ext per dir; the de-list scans (sections 2 and 3) derive ext per token. Replace the three `docket-*.md` globs and their `short_name` name extraction:

Section 1 inner loop:
```bash
  if [ ${#scan_dirs[@]} -gt 0 ]; then
    for dir in "${scan_dirs[@]}"; do
      [ -d "$dir" ] || continue
      local dtok dext
      dtok="$(harness_of_dir "$dir")"; dext="$(harness_ext "$dtok")"
      for f in "$dir"/docket-*."$dext"; do
        [ -e "$f" ] || continue
        name="$(basename "$f")"; name="${name#docket-}"; name="${name%.*}"
        [ -f "$AGENTS_SRC/docket-$name.md" ] || handle_orphan "$f"
      done
    done
  fi
```

Section 2 (per-repo de-listed harness) glob:
```bash
      for f in "$REPO/.$tok/agents"/docket-*."$(harness_ext "$tok")"; do
        [ -e "$f" ] || continue
        handle_orphan "$f"; pruned_agents=1
      done
```

Section 3 (user-level de-listed harness) glob:
```bash
    for f in "$HARNESS_ROOT/.$tok/agents"/docket-*."$(harness_ext "$tok")"; do
      [ -e "$f" ] || continue
      handle_orphan "$f"; pruned_agents=1
    done
```

`check_project_level` leg (c) — regenerate + diff using ext and the harness emitter:

```bash
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    for harness in $HARNESSES; do
      resolve_agent_layers "$harness" "$name" "$LOCAL_CFG" "$DOCKET_YML" "$GLOBAL_CFG"
      local ext; ext="$(harness_ext "$harness")"
      emit_for_harness "$src" "$harness" "$RES_MODEL" "$RES_EFFORT" > "$tmp/docket-$name.$ext"
      got="$REPO/.$harness/agents/docket-$name.$ext"
      if [ ! -f "$got" ]; then
        log "advisory: .$harness/agents/docket-$name.$ext not generated on this machine (run: bash sync-agents.sh)"; continue
      fi
      d="$(diff -u "$got" "$tmp/docket-$name.$ext" || true)"
      if [ -n "$d" ]; then log "advisory: drift in .$harness/agents/docket-$name.$ext:"; printf '%s\n' "$d" >&2; fi
    done
  done
```

(Declare `local ext` once near the other `check_project_level` locals if `set -u`/reuse warrants; a bare `local` inside the loop is fine in bash.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `bash tests/test_sync_agents_codex.sh`
Expected: `ALL PASS`, exit 0.

- [ ] **Step 5: Regression — the full sync-agents suite still passes**

Run: `bash tests/test_sync_agents.sh`
Expected: all `ok`, exit 0 (the `.md` prune/track/check paths are unchanged for non-codex — `harness_ext` returns `md` for every non-codex token, so the globs are byte-identical).

- [ ] **Step 6: Mutation-test, then commit**

Mutation check: revert the section-1 prune glob to `docket-*.md` → "prune: orphan .toml wrapper removed" must go RED. Revert `tracked_docket_files` to `.md`-only → "check: tracked .toml wrapper fails --check" must go RED. Restore both.

```bash
git add sync-agents.sh tests/test_sync_agents_codex.sh
git commit -m "feat(0077): prune, track, and --check the Codex .toml wrappers"
```

---

### Task 4: Committed `AGENTS.md` dispatch block

**Files:**
- Modify: `scripts/lib/docket-gitignore-block.sh` — add generic `ensure_managed_block()` / `remove_managed_block()` (reusing the existing `_docket_gi_*` primitives).
- Modify: `sync-agents.sh` — `DISPATCH_START`/`DISPATCH_END` markers, `AGENTS_MD_DISPATCH_HARNESSES`, `assemble_agents_md_dispatch`, `sync_codex_agents_md_dispatch` (called from `project_level_pass`), and a `check_project_level` presence/currency leg.
- Test: `tests/test_sync_agents_codex.sh` (extend)

**Interfaces:**
- Consumes: `_docket_gi_malformed`, `_docket_gi_strip_block`, `_docket_gi_current_block` (existing lib primitives, already marker-parameterized); `AGENTS_SRC`, `short_name`, `agent_description`, `per_repo_opted_in`, `HARNESSES` (existing).
- Produces:
  - `ensure_managed_block FILE START END WANT` → creates/updates the `[START,END]` block (WANT includes both markers) in FILE, preserving outside bytes; refuses on malformed markers. Prints a status word (`unchanged`|`wrote`|`refused`) to stdout.
  - `remove_managed_block FILE START END` → strips the block if present; prints `absent`|`removed`|`refused`.
  - `assemble_agents_md_dispatch` → the full AGENTS.md docket block (markers included): a static head + one machine-neutral bullet per built-in agent (glob order), naming the agent to delegate to. **No model IDs.**
  - `sync_codex_agents_md_dispatch` → writes the block into `<repo>/AGENTS.md` when `codex ∈ HARNESSES`, strips it when `codex ∉ HARNESSES` (de-listed within an opted-in repo).

- [ ] **Step 1: Write the failing tests** (append to `tests/test_sync_agents_codex.sh`)

```bash
# --- AGENTS.md dispatch block: created, machine-neutral, committed-style ---
mk_codex_repo
A="$SBX/AGENTS.md"
assert "agentsmd: block created" '[ -f "$A" ] && grep -qF "docket:dispatch:start" "$A"'
assert "agentsmd: has closing marker" 'grep -qF "docket:dispatch:end" "$A"'
assert "agentsmd: names an agent to delegate to" 'grep -qi "docket-implement-next" "$A"'
assert "agentsmd: carries NO model id (machine-neutral)" '! grep -qE "claude-|gpt-|model_reasoning_effort|model[[:space:]]*=" "$A"'

# idempotent second run: byte-identical
before="$(cat "$A")"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "agentsmd: idempotent second run byte-identical" '[ "$before" = "$(cat "$A")" ]'
rm -rf "$SBX"

# --- outside bytes preserved; hand-written AGENTS.md content survives ---
mk_codex_repo   # remove then recreate with pre-existing content
rm -f "$SBX/AGENTS.md"
printf '# Our project agents\n\nHand-written guidance here.\n' > "$SBX/AGENTS.md"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "agentsmd: pre-existing heading preserved" 'grep -qxF "# Our project agents" "$SBX/AGENTS.md"'
assert "agentsmd: pre-existing prose preserved"   'grep -qxF "Hand-written guidance here." "$SBX/AGENTS.md"'
assert "agentsmd: block appended below user content" 'grep -qF "docket:dispatch:start" "$SBX/AGENTS.md"'
rm -rf "$SBX"

# --- malformed markers: refuse, touch nothing ---
mk_codex_repo
rm -f "$SBX/AGENTS.md"
printf 'keepme\n<!-- docket:dispatch:start (managed by docket — do not hand-edit) -->\ndangling\n' > "$SBX/AGENTS.md"
before="$(cat "$SBX/AGENTS.md")"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "agentsmd: malformed markers left untouched" '[ "$before" = "$(cat "$SBX/AGENTS.md")" ]'
rm -rf "$SBX"

# --- de-list codex: dispatch block stripped (but user content kept) ---
mk_codex_repo
printf '# keep me\n' >> "$SBX/AGENTS.md"   # note: block is above; add trailing user line to prove strip keeps it
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "agentsmd delist: block removed" '! grep -qF "docket:dispatch:start" "$SBX/AGENTS.md"'
rm -rf "$SBX"
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `bash tests/test_sync_agents_codex.sh 2>&1 | grep -i agentsmd`
Expected: every `agentsmd:` assertion FAILS — no `AGENTS.md` is written yet.

- [ ] **Step 3: Write the generic managed-block primitives**

In `scripts/lib/docket-gitignore-block.sh`, append (after `ensure_docket_gitignore_block`, keeping that function byte-identical):

```bash
# --- generic managed block (change 0077) -------------------------------------
# Reuse the marker-parameterized primitives above. WANT must include both markers.
# ensure_managed_block: create/update the block, preserving bytes outside it. Prints a status
# word to stdout: refused (malformed markers) | unchanged | wrote. Never touches the file on refuse.
ensure_managed_block(){  # $1=file $2=start $3=end $4=want(full block incl markers)
  local f="$1" start="$2" end="$3" want="$4" have rest
  if _docket_gi_malformed "$f" "$start" "$end"; then printf 'refused\n'; return 0; fi
  rest=""
  [ -f "$f" ] && rest="$(_docket_gi_strip_block "$f" "$start" "$end")"
  have="$(_docket_gi_current_block "$f" "$start" "$end")"
  if [ "$want" = "$have" ]; then printf 'unchanged\n'; return 0; fi
  { if [ -n "$rest" ]; then printf '%s\n\n' "$rest"; fi; printf '%s\n' "$want"; } > "$f"
  printf 'wrote\n'
}
# remove_managed_block: strip the block if present, preserving outside bytes. Prints:
# refused (malformed) | absent (no file or no block) | removed.
remove_managed_block(){  # $1=file $2=start $3=end
  local f="$1" start="$2" end="$3" rest
  [ -f "$f" ] || { printf 'absent\n'; return 0; }
  if _docket_gi_malformed "$f" "$start" "$end"; then printf 'refused\n'; return 0; fi
  if [ -z "$(_docket_gi_current_block "$f" "$start" "$end")" ]; then printf 'absent\n'; return 0; fi
  rest="$(_docket_gi_strip_block "$f" "$start" "$end")"
  printf '%s\n' "$rest" > "$f"
  printf 'removed\n'
}
```

- [ ] **Step 4: Write the assembler + sync + wire it in `sync-agents.sh`**

Add the markers + harness list near `HARNESS_HAS_DISPATCH_RULES` (~line 217):

```bash
# Codex reads a committed AGENTS.md; only codex gets the AGENTS.md dispatch block (change 0077).
AGENTS_MD_DISPATCH_HARNESSES="codex"
DISPATCH_START='<!-- docket:dispatch:start (managed by docket — do not hand-edit) -->'
DISPATCH_END='<!-- docket:dispatch:end -->'
```

Add the assembler (after `assemble_dispatch_rule`, ~line 352). Machine-neutral: head + one bullet per built-in agent from its own `description` (single source; same glob/set as the Cursor rule):

```bash
# Assemble the committed AGENTS.md docket dispatch block (markers included) to stdout.
# Machine-neutral: agent names + delegation prose only, NO model IDs (pins live in the .toml).
assemble_agents_md_dispatch(){
  printf '%s\n' "$DISPATCH_START"
  cat <<'HEAD'
## Docket agents — dispatch, don't run inline

Docket ships model/effort-pinned agent definitions in `.codex/agents/docket-*.toml`. When you are
asked to run one of the docket skills below, run the matching **agent** (its pinned model and
reasoning effort are the whole point) instead of executing the skill inline at the session model.
Pass the request through unchanged, including any change or ADR id.
HEAD
  printf '\n'
  local src name desc
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    desc="$(agent_description "$src")"
    printf -- '- **docket-%s** — %s Delegate to the `docket-%s` agent.\n' "$name" "$desc" "$name"
  done
  printf '%s\n' "$DISPATCH_END"
}

# Write the AGENTS.md dispatch block when codex is a targeted per-repo harness; strip it when
# codex is de-listed (within an opted-in repo). Logs a one-time commit notice on write/remove.
sync_codex_agents_md_dispatch(){
  local f="$REPO/AGENTS.md" status
  case " $HARNESSES " in
    *" codex "*)
      status="$(ensure_managed_block "$f" "$DISPATCH_START" "$DISPATCH_END" "$(assemble_agents_md_dispatch)")"
      case "$status" in
        wrote)   log "wrote/updated the docket dispatch block in $f — COMMIT THIS (machine-neutral; no model IDs).";;
        refused) log "WARN $f has a malformed docket:dispatch block — refusing to rewrite; repair the markers by hand and re-run.";;
      esac
      ;;
    *)
      status="$(remove_managed_block "$f" "$DISPATCH_START" "$DISPATCH_END")"
      case "$status" in
        removed) log "removed the docket dispatch block from $f (codex de-listed) — COMMIT THIS.";;
        refused) log "WARN $f has a malformed docket:dispatch block — refusing to strip; repair the markers by hand.";;
      esac
      ;;
  esac
}
```

Call it at the end of `project_level_pass` (after the Cursor dispatch-rule loop, ~line 495):

```bash
  # Codex-only committed AGENTS.md dispatch block (change 0077).
  sync_codex_agents_md_dispatch
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `bash tests/test_sync_agents_codex.sh`
Expected: `ALL PASS`, exit 0 — every `agentsmd:` assertion passes (created, machine-neutral, idempotent, outside-preserved, malformed-refused, de-list-stripped).

- [ ] **Step 6: Run the gitignore-block + full sync-agents suites**

Run: `bash tests/test_docket_gitignore_block.sh && bash tests/test_sync_agents.sh`
Expected: both all-`ok`, exit 0 — `ensure_docket_gitignore_block` is untouched, and the new lib functions are additive.

- [ ] **Step 7: Mutation-test, then commit**

Mutation check: comment out the `sync_codex_agents_md_dispatch` call in `project_level_pass` → "agentsmd: block created" must go RED. In `assemble_agents_md_dispatch`, temporarily inject a `model = claude-x` line → "agentsmd: carries NO model id" must go RED. Break the malformed-guard (make `ensure_managed_block` skip `_docket_gi_malformed`) → "agentsmd: malformed markers left untouched" must go RED. Restore all.

```bash
git add scripts/lib/docket-gitignore-block.sh sync-agents.sh tests/test_sync_agents_codex.sh
git commit -m "feat(0077): committed AGENTS.md dispatch block for the Codex harness"
```

---

### Task 5: `--check` presence/currency leg for the AGENTS.md block

**Files:**
- Modify: `sync-agents.sh` — `check_project_level` (add a leg before the advisory legs, ~line 512).
- Test: `tests/test_sync_agents_codex.sh` (extend)

**Interfaces:**
- Consumes: `HARNESSES`, `assemble_agents_md_dispatch`, `_docket_gi_current_block` (lib), `DISPATCH_START`/`DISPATCH_END`.
- Produces: `--check` returns non-zero when `codex ∈ HARNESSES` but the AGENTS.md block is missing or stale, and when `codex ∉ HARNESSES` but a leftover block is present (should be pruned) — CI-meaningful, symmetric with the `.gitignore` leg (the block is committed, so this is exempt from the tracked-file leg per the spec).

- [ ] **Step 1: Write the failing tests** (append to `tests/test_sync_agents_codex.sh`)

```bash
# --- --check: codex enabled + block present & current => pass ---
mk_codex_repo
git -C "$SBX" add -A >/dev/null 2>&1
if ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check >/dev/null 2>&1 ); then
  echo "ok - check: codex enabled, block current => passes"
else
  echo "NOT OK - check: codex enabled, block current => passes"; fail=1
fi

# --- --check: codex enabled + block STALE => CI-meaningful failure ---
# mutate the committed block so it no longer matches the emitter
perl -0pi -e 's/(docket:dispatch:start[^\n]*\n)/$1STALE-EXTRA-LINE\n/' "$SBX/AGENTS.md"
if ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check >/dev/null 2>&1 ); then
  echo "NOT OK - check: stale AGENTS.md block fails --check"; fail=1
else
  echo "ok - check: stale AGENTS.md block fails --check"
fi
rm -rf "$SBX"

# --- --check: codex enabled + block MISSING => CI-meaningful failure ---
mk_codex_repo
rm -f "$SBX/AGENTS.md"
if ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check >/dev/null 2>&1 ); then
  echo "NOT OK - check: missing AGENTS.md block fails --check"; fail=1
else
  echo "ok - check: missing AGENTS.md block fails --check"
fi
rm -rf "$SBX"
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `bash tests/test_sync_agents_codex.sh 2>&1 | grep -E 'check: (stale|missing) AGENTS'`
Expected: "stale AGENTS.md block fails --check" and "missing AGENTS.md block fails --check" FAIL (no leg exists yet, so `--check` passes when it should fail).

- [ ] **Step 3: Write minimal implementation**

In `check_project_level`, after the `.gitignore` leg (a) block and before leg (c), add:

```bash
  # AGENTS.md dispatch block currency (change 0077) — CI-meaningful, symmetric with the
  # .gitignore leg. The block is committed (exempt from the tracked-file leg); assert it
  # is present & current when codex is targeted, and absent when codex is not.
  local am_want am_have
  am_want="$(assemble_agents_md_dispatch)"
  am_have="$(_docket_gi_current_block "$REPO/AGENTS.md" "$DISPATCH_START" "$DISPATCH_END")"
  case " $HARNESSES " in
    *" codex "*)
      if [ "$am_want" != "$am_have" ]; then
        log "check: AGENTS.md docket dispatch block missing or stale — run: bash sync-agents.sh and commit AGENTS.md"
        rc=1
      fi
      ;;
    *)
      if [ -n "$am_have" ]; then
        log "check: AGENTS.md carries a docket dispatch block but codex is not in agent_harnesses — run: bash sync-agents.sh and commit AGENTS.md"
        rc=1
      fi
      ;;
  esac
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `bash tests/test_sync_agents_codex.sh`
Expected: `ALL PASS`, exit 0.

- [ ] **Step 5: Full regression**

Run: `bash tests/test_sync_agents.sh && bash tests/test_docket_gitignore_block.sh`
Expected: both all-`ok`, exit 0 — the new leg is gated on `codex ∈ HARNESSES`; a non-codex repo's `--check` is unaffected (its `am_have` is empty and `codex ∉ HARNESSES`, so no `rc=1`).

- [ ] **Step 6: Mutation-test, then commit**

Mutation check: force `rc` untouched by hardcoding `am_want="$am_have"` → "stale AGENTS.md block fails --check" must go RED. Restore.

```bash
git add sync-agents.sh tests/test_sync_agents_codex.sh
git commit -m "feat(0077): --check gates AGENTS.md dispatch block presence and currency"
```

---

## Self-Review

**1. Spec coverage:**
- Spec §1 (per-harness emitter registry, `codex`→`.toml`+`emit_codex_toml`, field mapping, verbatim model/effort passthrough, skills-intent in `developer_instructions`, both passes) → **Task 2**.
- Spec §2 (managed AGENTS.md dispatch block: markers, hardened managed-block pattern, committed + machine-neutral, one-time commit notice, full built-in set, prune on de-list, user-level `~/.codex/AGENTS.md` out of scope) → **Task 4**.
- Spec §3 housekeeping (.gitignore gains `.codex/agents/docket-*.toml`; prune extends to `.toml`; `--check` tracked-file leg covers `.toml`; content-staleness leg regenerates/diffs TOML; AGENTS.md exempt from tracked-file leg but gets its own presence/currency check) → **Tasks 1, 3, 5**.
- Spec §4 (tests: TOML validity via shell/awk not a library; field mapping; model/effort passthrough; `effort: auto` drops the key; inherit-model omission; byte-identical non-codex regression; AGENTS.md create/idempotent/outside-preserved/malformed-refusal/prune; .gitignore includes TOML pattern; `--check` legs) → distributed across **Tasks 1-5**.
- Note: `effort: auto` and `inherit`-model omission are handled by `emit_codex_toml`'s guards (Task 2 code) but the *built-in* set has no `auto`/`inherit` values to exercise directly. Add a focused unit assert in Task 2 if a fixture is cheap; otherwise the guard is covered by code review (documented deviation — the real agent set never carries `auto`/`inherit`, so an on-disk assertion would need a synthetic built-in that the generation passes glob, which is not worth the fixture. The guard logic is simple and reviewed).

**2. Placeholder scan:** No `TBD`/`TODO`/"handle edge cases"/"similar to Task N". Every code step shows the actual code; every test step shows the actual assertions and the exact run command + expected result.

**3. Type/name consistency:** `harness_ext`, `emit_for_harness`, `emit_codex_toml`, `toml_escape_basic`, `assemble_agents_md_dispatch`, `sync_codex_agents_md_dispatch`, `ensure_managed_block`, `remove_managed_block`, `DISPATCH_START`/`DISPATCH_END`, `AGENTS_MD_DISPATCH_HARNESSES` — used consistently across tasks. Test helpers `toml_get`/`toml_has_key`/`mk_codex_repo` defined once (Task 2 test) and reused by later appended blocks in the same file.

## Notes for the ADR (step 6, after the build)

One non-obvious decision warrants an ADR: **the AGENTS.md dispatch block is committed and machine-neutral**, a deliberate departure from ADR-0020's rule that generated agent artifacts are gitignored/machine-local. Rationale to record: the block carries no model IDs (pins stay in the gitignored `.toml`), so it is clone-identical and belongs with the committed managed `.gitignore` block, not with the machine-local wrappers/Cursor rule; and its per-agent content is derived from the built-in agents' own descriptions (single source, same set as the Cursor rule) so it cannot drift. Also note the minor accepted tradeoff: `ensure_docket_gitignore_block` was left byte-identical (proven path) rather than refactored onto the new generic `ensure_managed_block`, so the ~6-line rewrite orchestration is duplicated once.
