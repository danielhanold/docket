# Multi-Harness Agent Generation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make docket's committed per-repo agent model config reach non-Claude harnesses (the motivating case: Cursor) by fanning `sync-agents.sh`'s project-level generation out over an explicit `.docket.yml` `agent_harnesses:` list.

**Architecture:** Add a self-contained `agent_harnesses` flow-list parser to `sync-agents.sh` (default `[claude]`, unknown-token warn-and-drop), derive the valid harness tokens from the existing `HARNESS_AGENT_DIRS` vocabulary, and loop both `project_level_pass` (generate) and `check_project_level` (`--check` drift) over the resolved harness set — writing/validating `<repo>/.<H>/agents/docket-*.md` per listed harness. Model IDs stay direct, harness-neutral passthrough (no tier layer — killed #0043). Default `[claude]` is byte-identical to today.

**Tech Stack:** Bash (`sync-agents.sh`, `set -euo pipefail`), the flat `tests/test_sync_agents.sh` assert harness, docket-convention SKILL.md docs.

## Global Constraints

- **`sync-agents.sh` is self-contained** — it parses `.docket.yml` directly and MUST NOT call `docket-config.sh`. The new `agent_harnesses` reader is a small top-level list reader beside `block_names`.
- **Default `[claude]` is byte-identical to pre-0045 behavior** — a repo with no `agent_harnesses:` key generates exactly `<repo>/.claude/agents/docket-*.md`, nothing else. All existing `test_sync_agents.sh` assertions must stay green.
- **Model IDs pass through verbatim** — no tiers, no allowlist, no validation (ADR-0008/0015). An arbitrary non-Claude id (`gpt-5.5-medium-fast`) must survive to every generated file unchanged. `field_of`'s existing class `[A-Za-z0-9._-]+` already admits such ids.
- **Token→dir mapping reuses `HARNESS_AGENT_DIRS`** — valid harness tokens are derived from that array (single source of truth); the project-level dir for token `H` is `$REPO/.<H>/agents` (uniform, incl. `agents`→`.agents/agents`).
- **Unknown token warned-and-ignored, never fatal** — a typo must not abort a sync (mirrors `board_surfaces`).
- **Never `producer | grep -q` / `producer | head`** under `set -o pipefail` — capture into a variable, then match via here-string (SIGPIPE-safe; the repo's own learnings ledger, #11/#16).
- **`sync-agents.sh` lives at the repo ROOT**, not `scripts/`. Tests live at `tests/test_sync_agents.sh`. The convention skill is `skills/docket-convention/SKILL.md`.
- **Test isolation:** decouple the user-level harness root (`DOCKET_HARNESS_ROOT=$HROOT`) from the repo (`$SBX`) so `<repo>/.<H>/agents` holds ONLY project-level output — a shared dir lets one pass mask the other (learnings ledger, "separate dirs").

---

### Task 1: `agent_harnesses` parser + project-level fan-out

Add the flow-list parser and the valid-token derivation, then loop `project_level_pass` over the resolved harness set. Remove the now-dead single `PROJECT_AGENT_DIR`.

**Files:**
- Modify: `sync-agents.sh` (repo root) — remove `PROJECT_AGENT_DIR` (line ~32); add `VALID_HARNESS_TOKENS` + `is_valid_harness` + `resolve_agent_harnesses`; rewrite `project_level_pass`; call the resolver before the pass dispatch.
- Test: `tests/test_sync_agents.sh` (append a new "Change 0045" section).

**Interfaces:**
- Consumes: existing `HARNESS_AGENT_DIRS`, `block_names`, `resolve_from`, `emit`, `AGENTS_SRC`, `REPO`, `DOCKET_YML`, `log`.
- Produces: global `HARNESSES` (space-separated resolved harness tokens); `is_valid_harness <token>` (rc 0 if known); `resolve_agent_harnesses` (sets `HARNESSES`, default `claude`, warns+drops unknowns). `project_level_pass` now writes `<repo>/.<H>/agents/docket-<name>.md` for every `H` in `HARNESSES`.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_sync_agents.sh` immediately before the final `exit $fail` line:

```bash
# ============================================================================
# Change 0045 — multi-harness project-level generation (agent_harnesses)
# ============================================================================

# (a) DEFAULT (no agent_harnesses key) => [claude]: project-level writes
#     .claude/agents ONLY (byte-identical to pre-0045 behavior). Separate HROOT
#     so <repo>/.claude/agents is purely project-level output.
make_sandbox                                          # SBX = the repo
HROOTA="$(mktemp -d)"; mkdir -p "$HROOTA/.claude"     # separate user-level root
printf 'agents:\n  status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTA" bash "$SYNC" >/dev/null )
assert "0045 default: writes project-level .claude/agents" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0045 default: does NOT write .cursor/agents" '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
assert "0045 default: per-repo model applied" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
rm -rf "$SBX" "$HROOTA"

# (b) agent_harnesses: [claude, cursor] => BOTH dirs generated, byte-identical,
#     carrying an arbitrary NON-Claude id verbatim (ADR-0008/0015 passthrough).
make_sandbox
HROOTB="$(mktemp -d)"; mkdir -p "$HROOTB/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTB" bash "$SYNC" >/dev/null )
assert "0045 fanout: .claude/agents generated" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0045 fanout: .cursor/agents generated" '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
assert "0045 fanout: claude file carries passthrough model" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "gpt-5.5-medium-fast" ]'
assert "0045 fanout: cursor file carries passthrough model" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast" ]'
assert "0045 fanout: both harness files byte-identical" 'diff -q "$SBX/.claude/agents/docket-status.md" "$SBX/.cursor/agents/docket-status.md" >/dev/null'
rm -rf "$SBX" "$HROOTB"

# (b') agent_harnesses: [cursor] ONLY => cursor generated, claude NOT (no forced-claude).
make_sandbox
HROOTC="$(mktemp -d)"; mkdir -p "$HROOTC/.claude"
printf 'agent_harnesses: [cursor]\nagents:\n  status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTC" bash "$SYNC" >/dev/null )
assert "0045 cursor-only: .cursor/agents generated" '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
assert "0045 cursor-only: .claude/agents NOT generated" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOTC"

# (d) unknown harness token => warned + dropped, NOT fatal; known harness still generated.
make_sandbox
HROOTD="$(mktemp -d)"; mkdir -p "$HROOTD/.claude"
printf 'agent_harnesses: [claude, bogus]\nagents:\n  status: { model: sonnet }\n' > "$SBX/.docket.yml"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTD" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0045 unknown-token: generation not fatal (rc=0)" '[ "$gen_rc" = "0" ]'
assert "0045 unknown-token: warns about the token" 'printf "%s" "$gen_err" | grep -qi "unknown agent_harnesses token"'
assert "0045 unknown-token: names the bad token" 'printf "%s" "$gen_err" | grep -q "bogus"'
assert "0045 unknown-token: known harness still generated" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0045 unknown-token: bad-token dir NOT created" '[ ! -e "$SBX/.bogus/agents" ]'
rm -rf "$SBX" "$HROOTD"

# (e) explicit empty list agent_harnesses: [] => resolves to no targets: no project
#     files generated (mirrors board_surfaces: []). Locks the empty-set code path.
make_sandbox
HROOTE0="$(mktemp -d)"; mkdir -p "$HROOTE0/.claude"
printf 'agent_harnesses: []\nagents:\n  status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTE0" bash "$SYNC" >/dev/null )
assert "0045 empty-list: no .claude project file" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
assert "0045 empty-list: no .cursor project file" '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOTE0"
```

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep -E '0045 (fanout|cursor-only|unknown-token|empty-list)'`
Expected: FAIL — the `[claude, cursor]` / `[cursor]` cases print `NOT OK` (only `.claude/agents` is ever written today; `agent_harnesses` is ignored, so `.cursor/agents` never appears). The `default` (a) case may already pass (it exercises today's behavior).

- [ ] **Step 3: Add the parser + valid-token derivation to `sync-agents.sh`**

Remove the now-dead top-level line (was ~line 32):

```bash
PROJECT_AGENT_DIR="$REPO/.claude/agents"
```

Immediately AFTER the `HARNESS_AGENT_DIRS=( … )` array definition, add the valid-token set derived from it (single source of truth):

```bash
# Valid harness tokens, derived from HARNESS_AGENT_DIRS (single source of truth):
# ".../.claude/agents" -> "claude". The project-level dir for token H is $REPO/.<H>/agents.
VALID_HARNESS_TOKENS=""
for _hd in "${HARNESS_AGENT_DIRS[@]}"; do
  _hb="$(basename "$(dirname "$_hd")")"          # ".claude"
  VALID_HARNESS_TOKENS="$VALID_HARNESS_TOKENS ${_hb#.}"   # "claude"
done
unset _hd _hb
```

After the `log(){ … }` definition (with the other helpers), add:

```bash
is_valid_harness(){  # $1=token -> rc 0 if it is a known harness token
  case " $VALID_HARNESS_TOKENS " in *" $1 "*) return 0;; *) return 1;; esac
}

# Resolve the per-repo agent_harnesses flow-list from .docket.yml into HARNESSES
# (space-separated). Unset/empty-value => default "claude". Unknown tokens warned + dropped.
# Self-contained (no docket-config.sh); mirrors board_surfaces flow-list parsing.
resolve_agent_harnesses(){
  local raw list tok
  raw=""
  if [ -f "$DOCKET_YML" ]; then
    # top-level (column 0) key only; strip a trailing comment; capture-then-head (SIGPIPE-safe).
    raw="$(sed -n -E 's/^agent_harnesses[[:space:]]*:[[:space:]]*([^#]*).*/\1/p' "$DOCKET_YML")"
    raw="$(head -n1 <<<"$raw" | sed -E 's/[[:space:]]+$//')"
  fi
  if [ -z "$raw" ]; then
    HARNESSES="claude"                            # unset / bare key => default [claude]
    return 0
  fi
  list="${raw#[}"; list="${list%]}"; list="${list//,/ }"   # strip flow brackets, commas -> spaces
  HARNESSES=""
  for tok in $list; do
    if is_valid_harness "$tok"; then
      HARNESSES="$HARNESSES $tok"
    else
      log "unknown agent_harnesses token '$tok' — ignored"
    fi
  done
  HARNESSES="$(echo $HARNESSES)"                  # trim/collapse ("[]" or all-unknown => "")
}
```

- [ ] **Step 4: Rewrite `project_level_pass` to loop over `HARNESSES`**

Replace the whole `project_level_pass` function with (name-outer / harness-inner: resolve each agent once, write to each listed harness dir; the "skip advisory skill" log fires once):

```bash
project_level_pass() {  # built-in ⊕ per-repo -> <repo>/.<H>/agents for each H in HARNESSES (committed)
  [ -f "$DOCKET_YML" ] || return 0
  local names name src harness dir
  names="$(block_names "$DOCKET_YML")"
  [ -n "$names" ] || return 0
  while IFS= read -r name; do
    [ -n "$name" ] || continue
    src="$AGENTS_SRC/docket-$name.md"
    if [ ! -f "$src" ]; then
      log "skip '$name' — no built-in wrapper (advisory/interactive skills have no agent file)"
      continue
    fi
    resolve_from "$DOCKET_YML" "$name" 1
    for harness in $HARNESSES; do
      dir="$REPO/.$harness/agents"
      mkdir -p "$dir"
      emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$dir/docket-$name.md"
    done
  done <<EOF
$names
EOF
}
```

- [ ] **Step 5: Resolve `HARNESSES` before the pass dispatch**

In the dispatch block at the bottom of the file, call the resolver before either pass that needs it. Change:

```bash
if [ "$CHECK" = "1" ]; then
  if check_project_level; then exit 0; else exit 1; fi
fi

user_level_pass
project_level_pass
log "done"
```

to:

```bash
resolve_agent_harnesses

if [ "$CHECK" = "1" ]; then
  if check_project_level; then exit 0; else exit 1; fi
fi

user_level_pass
project_level_pass
log "done"
```

- [ ] **Step 6: Run the new + existing tests to verify all pass**

Run: `bash tests/test_sync_agents.sh; echo "rc=$?"`
Expected: every line `ok - …` (including all pre-0045 assertions AND the new `0045 …` ones); `rc=0`.

- [ ] **Step 7: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "feat(0045): fan project-level agent generation over agent_harnesses

Add a self-contained agent_harnesses flow-list parser (default [claude],
unknown-token warn-and-drop) to sync-agents.sh and loop project_level_pass
over the resolved harness set, writing <repo>/.<H>/agents/docket-*.md per
listed harness. Model IDs pass through verbatim (ADR-0008/0015)."
```

---

### Task 2: `--check` drift gate spans every listed harness

Extend `check_project_level` to validate the generated file for every harness in `HARNESSES`, so a missing or stale `.cursor/agents/` file fails CI.

**Files:**
- Modify: `sync-agents.sh` — rewrite `check_project_level`.
- Test: `tests/test_sync_agents.sh` (append to the Change 0045 section).

**Interfaces:**
- Consumes: `HARNESSES` (set by `resolve_agent_harnesses`, already called before the `--check` dispatch in Task 1 Step 5), `block_names`, `resolve_from`, `emit`.
- Produces: `check_project_level` returns non-zero if any `<repo>/.<H>/agents/docket-<name>.md` is missing or drifts from freshly-resolved config, for every `H` in `HARNESSES`.

- [ ] **Step 1: Write the failing tests**

Append to the Change 0045 section of `tests/test_sync_agents.sh` (before `exit $fail`):

```bash
# --check must span every listed harness: drift in a .cursor/agents file fails CI.
make_sandbox
HROOTF="$(mktemp -d)"; mkdir -p "$HROOTF/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTF" bash "$SYNC" >/dev/null )   # generate both harness files
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTF" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0045 check: passes when both harness files in sync (rc=0)" '[ "$chk_rc" = "0" ]'
# Drift the CURSOR file only.
sed -i.bak 's/^model: sonnet/model: haiku/' "$SBX/.cursor/agents/docket-status.md"; rm -f "$SBX/.cursor/agents/docket-status.md.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTF" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0045 check: flags .cursor/agents drift (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "0045 check: drift report names the cursor harness" 'printf "%s" "$chk_out" | grep -q "drift" && printf "%s" "$chk_out" | grep -q "cursor"'
# A listed-harness file never generated -> missing-file drift.
rm -f "$SBX/.cursor/agents/docket-status.md"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTF" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0045 check: flags missing cursor file (rc!=0)" '[ "$chk_rc" != "0" ]'
rm -rf "$SBX" "$HROOTF"
```

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep -E '0045 check'`
Expected: the `.cursor/agents drift` and `missing cursor file` asserts print `NOT OK` — today `check_project_level` only inspects `$PROJECT_AGENT_DIR` (`.claude/agents`), so a cursor-file drift/absence is invisible (`rc=0`). (The "passes when in sync" assert may already pass.)

- [ ] **Step 3: Rewrite `check_project_level` to loop over `HARNESSES`**

Replace the whole `check_project_level` function with (emit the expected bytes once per name — harness-independent — and compare against every listed harness's committed file):

```bash
check_project_level() {  # diff committed <repo>/.<H>/agents files against freshly-resolved config
  local rc=0 names name src got tmp d harness
  [ -f "$DOCKET_YML" ] || { log "no .docket.yml in $REPO — nothing to check"; return 0; }
  names="$(block_names "$DOCKET_YML")"
  [ -n "$names" ] || { log "no agents: block — nothing to check"; return 0; }
  tmp="$(mktemp -d)"
  while IFS= read -r name; do
    [ -n "$name" ] || continue
    src="$AGENTS_SRC/docket-$name.md"
    [ -f "$src" ] || continue
    resolve_from "$DOCKET_YML" "$name" 1
    emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$tmp/docket-$name.md"   # bytes are harness-independent
    for harness in $HARNESSES; do
      got="$REPO/.$harness/agents/docket-$name.md"
      if [ ! -f "$got" ]; then
        log "drift: missing $got (run: bash sync-agents.sh)"; rc=1; continue
      fi
      d="$(diff -u "$got" "$tmp/docket-$name.md" || true)"   # capture (SIGPIPE-safe), do not pipe to grep -q
      if [ -n "$d" ]; then log "drift in .$harness/agents/docket-$name.md:"; printf '%s\n' "$d" >&2; rc=1; fi
    done
  done <<EOF
$names
EOF
  rm -rf "$tmp"
  return $rc
}
```

- [ ] **Step 4: Run the full test file to verify all pass**

Run: `bash tests/test_sync_agents.sh; echo "rc=$?"`
Expected: all `ok - …`; `rc=0`.

- [ ] **Step 5: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "feat(0045): extend --check drift gate over every listed harness"
```

---

### Task 3: Document `agent_harnesses` + the direct-model-ID contract in docket-convention

Add `agent_harnesses` to the `.docket.yml` schema block and the Agent-layer prose in the convention; state that model IDs are direct, harness-neutral passthrough and that project generation targets the listed harnesses. Add a commented `agent_harnesses` example to this repo's `.docket.yml` (default `[claude]`, no behavior change).

**Files:**
- Modify: `skills/docket-convention/SKILL.md` — the `.docket.yml` yaml schema block + the "Agent layer" section.
- Modify: `.docket.yml` (repo root) — a commented `agent_harnesses` example beside the `agents:` block.
- Test: `tests/test_sync_agents.sh` (append doc sentinels to the Change 0045 section).

**Interfaces:**
- Consumes: nothing (docs).
- Produces: convention prose greppable for `agent_harnesses`, the default `[claude]`, the direct/harness-neutral passthrough contract, and the ADR-0015 pointer.

- [ ] **Step 1: Write the failing doc sentinels**

Append to the Change 0045 section of `tests/test_sync_agents.sh` (before `exit $fail`). Each sentinel anchors to ONE clause it owns (non-vacuity: deleting that clause must flip it to NOT OK):

```bash
# Convention documents agent_harnesses + the direct-model-ID (harness-neutral) contract.
CONV="$REPO/skills/docket-convention/SKILL.md"
assert "0045 doc: convention names agent_harnesses" 'grep -q "agent_harnesses" "$CONV"'
assert "0045 doc: convention states default [claude]" 'grep -qE "agent_harnesses[^\n]*\[claude\]|default[^\n]*\[claude\]" "$CONV"'
assert "0045 doc: convention states harness-neutral direct model IDs" 'grep -qiE "harness-neutral|direct model id" "$CONV"'
assert "0045 doc: convention notes passthrough enables non-Claude harnesses" 'grep -qi "passthrough" "$CONV"'
assert "0045 doc: convention points at ADR-0015" 'grep -qE "ADR-?0015|\b0015\b" "$CONV"'
```

- [ ] **Step 2: Run the doc sentinels to verify they fail**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep -E '0045 doc'`
Expected: `NOT OK` for the `agent_harnesses` / `harness-neutral` / `passthrough` asserts (the convention does not yet mention them). The `0015` assert may already pass if ADR-0015 is referenced elsewhere — re-anchor to `agent_harnesses.*0015` proximity only if it passes vacuously (verify by mutation in Step 5).

- [ ] **Step 3: Document `agent_harnesses` in the convention**

In `skills/docket-convention/SKILL.md`, find the `.docket.yml` schema block (the fenced yaml example that lists `metadata_branch`, `integration_branch`, … `board_surfaces`, `agents:`). Add an `agent_harnesses` line to that example, right above `board_surfaces` or `agents:`:

```yaml
agent_harnesses: [claude]    # harnesses the per-repo agent pass generates committed wrappers for
                             # (change 0045); default [claude]. e.g. [claude, cursor] for a Cursor repo.
```

Then, in the **Agent layer** section (the prose describing `sync-agents.sh` and the layered config, change 0016), add a paragraph after the layered-config table stating the two coupled rules from ADR-0015. Use exactly these anchor phrases so the sentinels are non-vacuous:

```markdown
**Harness-portable model IDs (change 0045, ADR-0015).** Agent `model:` values are **direct model
IDs, harness-neutral and passed through verbatim** — no tier layer (change 0043's tiers were
rejected). The running harness interprets the string (a Claude alias/ID under Claude Code; a Cursor
model ID like `gpt-5.5-medium-fast` under Cursor). This unvalidated **passthrough** is exactly what
lets docket drive non-Claude harnesses. The per-repo (committed) generation fans out over an explicit
`.docket.yml` `agent_harnesses:` list — **global default `[claude]`** (byte-identical to before) — so
each listed harness `H` gets committed `<repo>/.<H>/agents/docket-*.md`; a Cursor repo sets
`agent_harnesses: [claude, cursor]`. Explicit over present-directory auto-detection, so a stray
`.cursor/` never silently mints committed files; an unknown harness token is warned-and-ignored. The
`sync-agents.sh --check` drift gate spans every generated per-harness file. The **user-level** pass is
unchanged (it still writes every present harness). `agent_harnesses` is read by a direct parse in
`sync-agents.sh` (not `docket-config.sh`).
```

- [ ] **Step 4: Add a commented `agent_harnesses` example to this repo's `.docket.yml`**

In `.docket.yml` (repo root), just above the `# Per-skill subagent model/effort (change 0016).` comment block, add:

```yaml
# Which harnesses the per-repo agent pass generates committed wrappers for (change 0045).
# Default [claude] (byte-identical to before). A Cursor repo would set [claude, cursor] so the
# committed agents: model config also reaches <repo>/.cursor/agents/. Model IDs pass through
# verbatim (harness-neutral — a Cursor id like gpt-5.5-medium-fast is emitted as-is). Unknown
# tokens are warned + dropped. sync-agents.sh --check validates every listed harness's files.
# agent_harnesses: [claude]
```

(Left commented — this repo dogfoods Claude Code, so the effective default stays `[claude]` and output is byte-identical.)

- [ ] **Step 5: Run the doc sentinels + mutation-test each for non-vacuity**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep -E '0045 doc'`
Expected: all `ok - …`.

Then prove each sentinel is non-vacuous — temporarily delete the `agent_harnesses` paragraph from the convention and re-run; every `0045 doc` assert except possibly the shared `0015` one must flip to `NOT OK`. Restore the paragraph. (If the `0015` assert stays `ok` with the paragraph gone, tighten it to require `agent_harnesses` and `0015` within the added block — e.g. `grep -Pzoq "agent_harnesses[\s\S]{0,400}0015"` — then restore.)

- [ ] **Step 6: Commit**

```bash
git add skills/docket-convention/SKILL.md .docket.yml tests/test_sync_agents.sh
git commit -m "docs(0045): document agent_harnesses + direct-model-ID contract"
```

---

## Build-time manual verification (not an automated test)

The hermetic suite can only assert the **bytes generated**. Whether Cursor **honors** the generated
`.cursor/agents/docket-*.md` is live behavior (Open question in the spec). At build:

1. Generate a real Cursor wrapper into a scratch sandbox with `agent_harnesses: [cursor]` and inspect
   its bytes — confirm it carries `model:`, `effort:`, and load-bearingly `skills: [docket-<skill>,
   docket-convention]`.
2. The genuine live check — Cursor (a) honors `model:` despite the richer frontmatter and (b) still
   loads the skill via `skills:` — requires the Cursor app and is a **human merge-gate step**. Record
   it in the results file with the exact generated bytes so the human can drop them into a Cursor repo
   and verify. If (b) fails (skill not loaded), inlining the skill body is a possible follow-up beyond
   this change — not a blocker for the fan-out generation itself.

## Self-Review

- **Spec coverage:** `agent_harnesses` parser (Task 1), `project_level_pass` fan-out (Task 1),
  `check_project_level` over harnesses (Task 2), direct-parse-not-`docket-config.sh` (Task 1 Global
  Constraint + resolver), token→dir via `HARNESS_AGENT_DIRS` (Task 1), unknown-token warn-drop
  (Task 1), default `[claude]` byte-identical (Task 1 a), model passthrough (Task 1 b), convention
  docs + direct-model-ID contract (Task 3), `.docket.yml` example (Task 3), tests a/b/c/d (Tasks
  1–2). Live Cursor verification → build-time manual step + results file. **No gaps.**
- **User-level pass:** deliberately untouched (spec + ADR-0015). Not a task.
- **Type consistency:** `HARNESSES`, `is_valid_harness`, `resolve_agent_harnesses`, `VALID_HARNESS_TOKENS`
  used consistently across Tasks 1–2; project dir is `$REPO/.<H>/agents` everywhere.
