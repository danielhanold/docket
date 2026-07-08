# Per-harness agent model overrides (harness-first `agents:`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `sync-agents.sh`'s `agents:` config block harness-first so each generated harness wrapper (`.claude/agents/…`, `.cursor/agents/…`) can carry a different `model`/`effort`, resolving field-by-field `agents.<harness>.<agent>` → `agents.default.<agent>` → the shipped built-in.

**Architecture:** `sync-agents.sh` today resolves one agent-keyed `agents:` block and writes the *same* resolved wrapper to every harness dir (change #0045's byte-identical fan-out). This change reshapes the block to a reserved `default:` key plus harness-name keys, each holding the familiar agent → `{model, effort}` map, and rewrites the readers + all three passes (`user_level_pass`, `project_level_pass`, `check_project_level`) to resolve per (harness, agent) with independent field-level fallback. A non-`claude` harness whose `model` fell through to `default`/built-in gets a non-fatal footgun warning. This is a **clean break**: the pre-0046 flat agent-keyed shape is warned + ignored (safe — docket's own `agents:` block is commented and no live global config exists).

**Tech Stack:** Bash + awk/sed/grep (POSIX-portable, GNU+BSD), no external deps. Tests are `tests/test_sync_agents.sh` (plain bash asserts). `sync-agents.sh` lives at the **repo root** (not `scripts/`).

## Global Constraints

- **Values are direct model IDs, passed through verbatim — no validation, no tier layer** (ADR-0015). `field_of`'s value charset stays `[A-Za-z0-9._-]+` (matches `claude-opus-4-8` and `gpt-5.5-medium-fast` alike). Do **not** add a roster/allowlist.
- **Two config layers, same harness-first shape, never hand-merged** (ADR-0008): user-level = built-in ⊕ global (`~/.config/docket/agents.yaml`); project-level = built-in ⊕ per-repo (`.docket.yml`). The harness applies project-over-user precedence natively.
- **Shape asymmetry, preserved:** in `.docket.yml` the harness map is nested under a top-level `agents:` key; in the global `agents.yaml` the harness map **is** the whole file (no `agents:` wrapper). This extends the current `under_block` (0|1) reader parameter one nesting level — do not "fix" it to symmetric, that would break existing global-config users silently.
- **`agent_harnesses` (change #0045) is unchanged** and stays orthogonal: it is the authoritative *fan-out list* (which harness dirs get files); `agents.<harness>` supplies *values*. `resolve_agent_harnesses`, `is_valid_harness`, `VALID_HARNESS_TOKENS`, `HARNESS_AGENT_DIRS`, `field_of`, `emit`, `short_name`, `log` are **kept as-is**.
- **Bash safety:** `set -euo pipefail` is in force. Capture each pipeline stage into a variable and process with a here-string (`head -n1 <<<"$x"`), never `producer | head`/`grep -q` (SIGPIPE under pipefail — LEARNINGS #25). Guard externally-sourced tokens against globbing where relevant (the existing `set -f` guard in `resolve_agent_harnesses` stays).
- **Footgun warning is non-fatal and scoped to non-`claude` harnesses.** It never changes exit status; `sync-agents.sh` still succeeds.
- **Test seam:** `DOCKET_HARNESS_ROOT` overrides `$HOME` for harness dirs and (when `XDG_CONFIG_HOME` unset) the global-config root. Tests `unset XDG_CONFIG_HOME` at top. When a test needs project-level output isolated from user-level output, it points `DOCKET_HARNESS_ROOT` at a *separate* temp root (`HROOT*`) so `<repo>/.claude/agents` holds only project-level files.

---

### Task 1: Harness-first resolution in `sync-agents.sh` (readers + all three passes)

The atomic core. The readers and their three consumers must change together — an intermediate state where the readers are harness-first but a pass still calls the old `resolve_from`/`block_names` does not build (unbound/renamed function → crash under `set -euo pipefail`; LEARNINGS #45). Reshape the existing flat-shape test fixtures to the harness-first shape in the same task (the clean break) and add the field-level-fallback tests.

**Files:**
- Modify: `sync-agents.sh` (repo root) — replace the config-reader helpers and rewrite `user_level_pass`, `project_level_pass`, `check_project_level`.
- Test: `tests/test_sync_agents.sh` (repo root) — reshape the per-repo-layer, global-layer, critic, rebase-resolver, `--check`, and all Change-0045 fixtures from flat `agents:\n  <agent>: {…}` to harness-first `agents:\n  default:\n    <agent>: {…}` (or `<harness>:`); add resolution/field-merge/passthrough tests.

**Interfaces:**
- Consumes: unchanged public helpers `field_of(line, field)`, `emit(src, model, effort)`, `resolve_agent_harnesses` → `HARNESSES`, `short_name(path)`, `is_valid_harness(token)`, `VALID_HARNESS_TOKENS`, `HARNESS_AGENT_DIRS`, `GLOBAL_CFG`, `DOCKET_YML`, `AGENTS_SRC`, `REPO`.
- Produces (new/changed internal helpers later tasks rely on):
  - `section_body <key>` — reads stdin (a YAML doc), prints the body nested under the first bare `<key>:` header, **dedented to column 0** at the block's base indent.
  - `harness_agent_line <file> <harness> <agent> <under_agents(0|1)>` — prints the single `<agent>: { … }` entry line under `agents.<harness>` (or `<harness>` when `under_agents=0`), or empty.
  - `resolve_agent <file> <harness> <agent> <under_agents(0|1)>` — sets `RES_MODEL`, `RES_EFFORT` via independent field-level fallback (harness → default), and `RES_MODEL_FROM_HARNESS` (1 iff the model value came from the harness-specific line).
  - `agent_keys <file> <under_agents(0|1)>` — prints the union (sorted-unique) of agent keys configured under any harness sub-block or `default`.

- [ ] **Step 1: Reshape the per-repo + global test fixtures to harness-first, and add the field-merge fixtures (write the failing tests)**

In `tests/test_sync_agents.sh`, replace the **global layer** block (currently lines ~72–90) with harness-first fixtures. Global file = bare harness map (no `agents:` wrapper):

```bash
# -- global layer (harness-first): ~/.config/docket/agents.yaml default: block overrides model/effort --
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'default:\n  status: { model: haiku, effort: low }\n  implement-next: { effort: auto }\n' > "$SBX/.config/docket/agents.yaml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "global default sets model" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "haiku" ]'
assert "global default sets effort" '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "low" ]'
assert "effort: auto drops the effort line" '! grep -q "^effort:" "$SBX/.claude/agents/docket-implement-next.md"'
assert "auto keeps the built-in model" '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)" = "claude-opus-4-8" ]'
assert "unlisted skill keeps built-in model+effort" '[ "$(fm "$SBX/.claude/agents/docket-adr.md" model)/$(fm "$SBX/.claude/agents/docket-adr.md" effort)" = "claude-sonnet-5/medium" ]'
rm -rf "$SBX"

# -- global: a per-harness block overrides default for THAT harness only (user-level) --
make_sandbox                                        # .claude and .cursor both present so both get user-level files
mkdir -p "$SBX/.cursor" "$SBX/.config/docket"
printf 'default:\n  status: { model: haiku }\ncursor:\n  status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.config/docket/agents.yaml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "global cursor block wins for cursor" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast" ]'
assert "global claude falls to default" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "haiku" ]'
rm -rf "$SBX"
```

Replace the **per-repo layer** block (currently lines ~92–104) with:

```bash
# -- per-repo layer (harness-first): .docket.yml agents.default: => committed project-level files --
make_sandbox                                       # SBX = the repo
HROOT="$(mktemp -d)"; mkdir -p "$HROOT/.claude"    # separate user-level harness root
printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n    new-change: { model: opus }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT" bash "$SYNC" >/dev/null )
assert "per-repo default writes project-level file" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "per-repo default applies model" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "per-repo default applies effort" '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "high" ]'
assert "no project-level file for unlisted skill (implement-next)" '[ ! -f "$SBX/.claude/agents/docket-implement-next.md" ]'
assert "advisory skill in agents: produces NO file (new-change)" '[ ! -f "$SBX/.claude/agents/docket-new-change.md" ]'
rm -rf "$SBX" "$HROOT"

# (a)+(b) harness override wins; field-level merge — model from cursor, effort inherited from default.
make_sandbox
HROOTM="$(mktemp -d)"; mkdir -p "$HROOTM/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet, effort: high }\n  cursor:\n    status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTM" bash "$SYNC" >/dev/null )
assert "0046 (a): cursor model from cursor block" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast" ]'
assert "0046 (b): cursor effort inherited from default" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" effort)" = "high" ]'
assert "0046 (a): claude model falls to default" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "0046 (a): claude effort from default" '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "high" ]'
# (c) arbitrary non-Claude id passes through verbatim; the two harness files now DIFFER (was byte-identical pre-0046).
assert "0046 (c): non-Claude id verbatim in .cursor" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast" ]'
assert "0046: harness files differ when overridden" '! diff -q "$SBX/.claude/agents/docket-status.md" "$SBX/.cursor/agents/docket-status.md" >/dev/null'
rm -rf "$SBX" "$HROOTM"

# (d) default-only (no harness block) reproduces today's .claude/agents output byte-for-byte across harnesses.
make_sandbox
HROOTD0="$(mktemp -d)"; mkdir -p "$HROOTD0/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTD0" bash "$SYNC" >/dev/null )
assert "0046 (d): default-only => both harness files byte-identical" 'diff -q "$SBX/.claude/agents/docket-status.md" "$SBX/.cursor/agents/docket-status.md" >/dev/null'
assert "0046 (d): default-only applies model to claude" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
rm -rf "$SBX" "$HROOTD0"
```

- [ ] **Step 2: Run the reshaped tests to verify they fail**

Run: `cd /Users/homer/dev/docket/.worktrees/per-harness-agent-models && bash tests/test_sync_agents.sh`
Expected: the new `default:`/harness fixtures produce `NOT OK` lines (the current flat reader finds nothing under `agents:\n  default:` because it treats `default` as an agent key with no `model`/`effort`, so wrappers keep built-ins) and overall non-zero exit. Confirm the pre-existing Task-1/1b/1c/5/6 built-in assertions still pass.

- [ ] **Step 3: Rewrite the config readers in `sync-agents.sh` to harness-first**

Replace the `# --- config helpers ---` region (current `entry_line`, `field_of`, `block_names`, `resolve_from`; lines ~93–135) with the following. **Keep `field_of` exactly as-is**; replace the rest:

```bash
# --- config helpers ----------------------------------------------------------
# Print the body nested under the first bare `<key>:` header from stdin, DEDENTED to column 0
# at the block's base indent (so a nested doc's harness keys land at column 0 regardless of the
# parent's indentation). Body = lines strictly more-indented than the header, up to the next line
# at the header's indent-or-less. Values are printed raw (comment-stripping is the caller's job).
section_body() {  # $1=key ; reads stdin
  awk -v key="$1" '
    function ind(s,   m){ m=match(s, /[^ ]/); return (m==0 ? length(s) : m-1) }
    { nc=$0; sub(/#.*/,"",nc) }
    !inb { if (nc ~ ("^[[:space:]]*" key "[[:space:]]*:[[:space:]]*$")) { inb=1; kin=ind(nc) } next }
    nc ~ /[^[:space:]]/ && ind(nc) <= kin { exit }                 # first line back at/above header -> block done
    { if (!haveBase && nc ~ /[^[:space:]]/) { base=ind($0); haveBase=1 }
      if (haveBase) print substr($0, base+1); else print }         # dedent by the base indent
  '
}

# field_of() — UNCHANGED (kept verbatim from the prior version).
field_of() {  # $1=line  $2=field
  local out
  out="$(printf '%s' "$1" | sed -nE "s/.*[{,[:space:]]${2}[[:space:]]*:[[:space:]]*([A-Za-z0-9._-]+).*/\1/p")"
  head -n1 <<<"$out"
}

# Print the `agents.<harness>.<agent>` entry line from <file>. under_agents=1 => the harness map is
# nested under a top-level `agents:` key (.docket.yml); 0 => the harness map is the whole file (global).
harness_agent_line() {  # $1=file  $2=harness  $3=agent  $4=under_agents(0|1)
  local sub hbody stripped matched
  [ -f "$1" ] || return 0
  if [ "$4" = "1" ]; then sub="$(section_body agents < "$1")"; else sub="$(cat "$1")"; fi
  hbody="$(printf '%s\n' "$sub" | section_body "$2")"                        # body under <harness>/<default>
  stripped="$(printf '%s\n' "$hbody" | sed 's/#.*//')"
  matched="$(printf '%s\n' "$stripped" | grep -E "^[[:space:]]*$3[[:space:]]*:" || true)"
  head -n1 <<<"$matched"
}

# Resolve (harness, agent) into RES_MODEL / RES_EFFORT via INDEPENDENT field-level fallback:
#   agents.<harness>.<agent>  ->  agents.default.<agent>   (built-in floor is handled by emit()).
# RES_MODEL_FROM_HARNESS=1 iff the model value came from the harness-specific line (not default).
resolve_agent() {  # $1=file  $2=harness  $3=agent  $4=under_agents(0|1)
  local hline dline hm he
  hline="$(harness_agent_line "$1" "$2" "$3" "$4")"
  dline="$(harness_agent_line "$1" default "$3" "$4")"
  hm="$(field_of "$hline" model)"; he="$(field_of "$hline" effort)"
  RES_MODEL_FROM_HARNESS=0
  if [ -n "$hm" ]; then RES_MODEL="$hm"; RES_MODEL_FROM_HARNESS=1; else RES_MODEL="$(field_of "$dline" model)"; fi
  if [ -n "$he" ]; then RES_EFFORT="$he"; else RES_EFFORT="$(field_of "$dline" effort)"; fi
}

# Union (sorted-unique) of agent keys configured under any harness sub-block or `default`.
agent_keys() {  # $1=file  $2=under_agents(0|1)
  local sub
  [ -f "$1" ] || return 0
  if [ "$2" = "1" ]; then sub="$(section_body agents < "$1")"; else sub="$(cat "$1")"; fi
  printf '%s\n' "$sub" | awk '
    function ind(s,   m){ m=match(s,/[^ ]/); return (m==0?length(s):m-1) }
    { nc=$0; sub(/#.*/,"",nc) }
    nc ~ /^[A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*$/ { basei=ind(nc); inb=1; next }   # a harness/default header (col 0, bare)
    inb && nc ~ /[^[:space:]]/ && ind(nc) <= basei { inb=0 }
    inb && nc ~ /^[[:space:]]+[A-Za-z0-9._-]+[[:space:]]*:/ {
      k=nc; sub(/^[[:space:]]+/,"",k); sub(/[[:space:]]*:.*/,"",k); if (k!="") print k
    }' | sort -u
}
```

- [ ] **Step 4: Rewrite the three passes to resolve per (harness, agent)**

Replace `user_level_pass`, `project_level_pass`, `check_project_level` (lines ~150–216) with:

```bash
# --- passes ------------------------------------------------------------------
# Map a user-level harness *dir* ("$HARNESS_ROOT/.cursor/agents") to its token ("cursor").
harness_of_dir(){ local b; b="$(basename "$(dirname "$1")")"; printf '%s' "${b#.}"; }

user_level_pass() {  # built-in ⊕ global -> each present harness */agents dir, resolved per (harness, agent)
  local src dir name harness
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    for dir in "${HARNESS_AGENT_DIRS[@]}"; do
      [ -d "$(dirname "$dir")" ] || continue          # only write into a PRESENT harness root
      harness="$(harness_of_dir "$dir")"
      resolve_agent "$GLOBAL_CFG" "$harness" "$name" 0
      warn_fallback_model "$harness" "$name"
      mkdir -p "$dir"
      emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$dir/$(basename "$src")"
    done
  done
}

project_level_pass() {  # built-in ⊕ per-repo -> <repo>/.<H>/agents for each H in HARNESSES (committed)
  [ -f "$DOCKET_YML" ] || return 0
  local names name src harness dir
  names="$(agent_keys "$DOCKET_YML" 1)"
  [ -n "$names" ] || return 0
  while IFS= read -r name; do
    [ -n "$name" ] || continue
    src="$AGENTS_SRC/docket-$name.md"
    if [ ! -f "$src" ]; then
      log "skip '$name' — no built-in wrapper (advisory/interactive skills have no agent file)"
      continue
    fi
    for harness in $HARNESSES; do
      resolve_agent "$DOCKET_YML" "$harness" "$name" 1
      warn_fallback_model "$harness" "$name"
      dir="$REPO/.$harness/agents"
      mkdir -p "$dir"
      emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$dir/docket-$name.md"
    done
  done <<EOF
$names
EOF
}

check_project_level() {  # diff committed <repo>/.<H>/agents files against freshly-resolved config (per harness)
  local rc=0 names name src got tmp d harness
  [ -f "$DOCKET_YML" ] || { log "no .docket.yml in $REPO — nothing to check"; return 0; }
  names="$(agent_keys "$DOCKET_YML" 1)"
  [ -n "$names" ] || { log "no agents: block — nothing to check"; return 0; }
  tmp="$(mktemp -d)"
  while IFS= read -r name; do
    [ -n "$name" ] || continue
    src="$AGENTS_SRC/docket-$name.md"
    [ -f "$src" ] || continue
    for harness in $HARNESSES; do
      resolve_agent "$DOCKET_YML" "$harness" "$name" 1      # harness-specific bytes (no longer harness-independent)
      emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$tmp/docket-$name.md"
      got="$REPO/.$harness/agents/docket-$name.md"
      if [ ! -f "$got" ]; then
        log "drift: missing $got (run: bash sync-agents.sh)"; rc=1; continue
      fi
      d="$(diff -u "$got" "$tmp/docket-$name.md" || true)"
      if [ -n "$d" ]; then log "drift in .$harness/agents/docket-$name.md:"; printf '%s\n' "$d" >&2; rc=1; fi
    done
  done <<EOF
$names
EOF
  rm -rf "$tmp"
  return $rc
}
```

Add a **stub** `warn_fallback_model` above the passes for now (Task 2 fills its body); a no-op keeps Task 1 buildable and isolates the diagnostic behavior to its own task:

```bash
# Non-fatal footgun warning — body added in Task 2. No-op stub keeps Task 1's positive-path build green.
warn_fallback_model(){ :; }   # $1=harness $2=agent  (consumes RES_MODEL / RES_MODEL_FROM_HARNESS)
```

- [ ] **Step 5: Reshape the remaining flat-shape fixtures (critic, rebase-resolver, `--check`, Change-0045 blocks) to harness-first**

Every remaining `printf 'agents:\n  <agent>: {…}'` and `printf 'agent_harnesses: …\nagents:\n  <agent>: {…}'` fixture must nest the agent under `default:`. Apply these edits in `tests/test_sync_agents.sh`:

- Critic override (line ~124): `printf 'agents:\n  default:\n    auto-groom-critic: { model: sonnet, effort: high }\n'`
- Rebase-resolver override (line ~157): `printf 'agents:\n  default:\n    rebase-resolver: { model: sonnet, effort: high }\n'`
- Task-3 `--check` (lines ~170, ~183): `printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n'`
- 0045 (a) (line ~228): `printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n'`
- 0045 (b) fanout (line ~239): **rewrite** to give cursor its own model so the files differ, and change the byte-identical assertion (line ~245) to a *differ* assertion:
  ```bash
  printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet }\n  cursor:\n    status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.docket.yml"
  ( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTB" bash "$SYNC" >/dev/null )
  assert "0045 fanout: .claude/agents generated" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
  assert "0045 fanout: .cursor/agents generated" '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
  assert "0046 fanout: claude carries default model" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
  assert "0046 fanout: cursor carries its override model" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast" ]'
  assert "0046 fanout: harness files differ when cursor overrides" '! diff -q "$SBX/.claude/agents/docket-status.md" "$SBX/.cursor/agents/docket-status.md" >/dev/null'
  ```
- 0045 (b') cursor-only (line ~251): `printf 'agent_harnesses: [cursor]\nagents:\n  default:\n    status: { model: sonnet }\n'`
- 0045 (d) unknown-token (line ~260): `printf 'agent_harnesses: [claude, bogus]\nagents:\n  default:\n    status: { model: sonnet }\n'`
- 0045 (e) empty-list (line ~273): `printf 'agent_harnesses: []\nagents:\n  default:\n    status: { model: sonnet }\n'`
- 0045 `--check` spans harnesses (line ~282): `printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet, effort: high }\n'`
- 0045 (f) glob-token (line ~310): `printf 'agent_harnesses: [claude, *]\nagents:\n  default:\n    status: { model: sonnet }\n'`
- 0045 (g) top-level anchor (line ~321): the `agent_harnesses` decoy is unchanged; nest the agent: `printf 'decoy:\n  agent_harnesses: [cursor]\nagent_harnesses: [claude]\nagents:\n  default:\n    status: { model: sonnet }\n'`

Delete the obsolete **flat global decoy** test (lines ~84–90, "global keys are top-level only … indented decoy"); its flat top-level agent-key premise no longer exists. Its intent (an indented decoy must not shadow) is re-covered by the harness-resolution tests and Task 2's legacy test.

- [ ] **Step 6: Run the full suite to verify green**

Run: `cd /Users/homer/dev/docket/.worktrees/per-harness-agent-models && bash tests/test_sync_agents.sh; echo "exit=$?"`
Expected: `exit=0`, every line `ok - …`. If any `NOT OK`, fix the reader/pass (not the test's intent). Sanity-grep the suite for any surviving flat fixture: `grep -nE "agents:\\\\n +[A-Za-z]" tests/test_sync_agents.sh` should return nothing but `default:`/harness-nested forms (LEARNINGS #42 — grep the whole suite, spec touch-points are a floor).

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/per-harness-agent-models
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "feat(0046): harness-first agents: resolution in sync-agents.sh"
```

---

### Task 2: Diagnostics — legacy warn+ignore, dead-config drop, non-Claude footgun warning

Three non-fatal diagnostic behaviors, each test-first. All are additive to Task 1 (new warning branches + the `warn_fallback_model` body) — no Task-1 reference is removed, so there is no cross-task seam.

**Files:**
- Modify: `sync-agents.sh` — fill `warn_fallback_model`; add legacy-bare-agent-key detection (warn + `--check` drift); add dead-config-harness (in `agents:` but not `agent_harnesses`) warn + drop.
- Test: `tests/test_sync_agents.sh` — add tests (e), (f), (g'), (h).

**Interfaces:**
- Consumes: `resolve_agent` (sets `RES_MODEL_FROM_HARNESS`), `agent_keys`, `section_body`, `HARNESSES`, `log`.
- Produces: `warn_fallback_model <harness> <agent>` (real body); `legacy_agent_keys <file> <under_agents>` (bare agent keys sitting directly under `agents:`/top level — the pre-0046 shape).

- [ ] **Step 1: Write failing tests for the footgun warning (h)**

Append to `tests/test_sync_agents.sh` (before `exit $fail`):

```bash
# ============================================================================
# Change 0046 — per-harness values: diagnostics
# ============================================================================

# (h) Non-Claude fallback warning: a cursor file whose model fell through to default/built-in warns;
#     suppressed for claude, and suppressed when cursor supplies its own model.
make_sandbox
HROOTW="$(mktemp -d)"; mkdir -p "$HROOTW/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: claude-opus-4-8 }\n' > "$SBX/.docket.yml"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTW" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0046 (h): generation not fatal (rc=0)" '[ "$gen_rc" = "0" ]'
assert "0046 (h): warns cursor model came from default/built-in" 'printf "%s" "$gen_err" | grep -qi "cursor" && printf "%s" "$gen_err" | grep -qi "default/built-in"'
assert "0046 (h): does NOT warn for the claude harness" '! printf "%s" "$gen_err" | grep -qiE "claude/docket-status|WARN claude"'
rm -rf "$SBX" "$HROOTW"

# (h') warning suppressed when the cursor block supplies the model.
make_sandbox
HROOTW2="$(mktemp -d)"; mkdir -p "$HROOTW2/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: claude-opus-4-8 }\n  cursor:\n    status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.docket.yml"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTW2" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0046 (h'): no fallback warning when cursor supplies model" '! printf "%s" "$gen_err" | grep -qi "status.*default/built-in"'
rm -rf "$SBX" "$HROOTW2"
```

- [ ] **Step 2: Run to verify (h) fails**

Run: `cd /Users/homer/dev/docket/.worktrees/per-harness-agent-models && bash tests/test_sync_agents.sh 2>&1 | grep -E "0046 \(h"`
Expected: `NOT OK - 0046 (h): warns …` (the stub `warn_fallback_model` emits nothing).

- [ ] **Step 3: Implement `warn_fallback_model`**

Replace the Task-1 stub with:

```bash
# Non-fatal footgun warning: when generating a NON-claude harness file whose `model` resolved from
# default/built-in (no agents.<harness> override supplied it), the ID is likely wrong for that
# harness (ADR-0015: some harnesses silently run their house default on an unknown model). Never
# an error; sync still succeeds. Scoped to non-claude — the claude built-ins/default ARE Claude IDs.
warn_fallback_model(){  # $1=harness $2=agent ; consumes RES_MODEL_FROM_HARNESS / RES_MODEL
  [ "$1" = "claude" ] && return 0
  [ "$RES_MODEL_FROM_HARNESS" = "1" ] && return 0
  log "WARN $1/docket-$2: model '${RES_MODEL:-<built-in>}' came from default/built-in; may not be a valid model ID for harness '$1'."
}
```

- [ ] **Step 4: Run to verify (h)/(h') pass**

Run: `cd /Users/homer/dev/docket/.worktrees/per-harness-agent-models && bash tests/test_sync_agents.sh 2>&1 | grep -E "0046 \(h"`
Expected: all `ok - 0046 (h…)`.

- [ ] **Step 5: Write failing tests for legacy shape (f) and dead-config harness (e)**

```bash
# (f) Legacy bare-agent-key block (pre-0046 flat shape) => warned + ignored; --check flags it as drift.
make_sandbox
HROOTL="$(mktemp -d)"; mkdir -p "$HROOTL/.claude"
printf 'agents:\n  status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"   # bare agent key, no default:/harness
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTL" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0046 (f): legacy shape not fatal (rc=0)" '[ "$gen_rc" = "0" ]'
assert "0046 (f): warns about the legacy bare agent key" 'printf "%s" "$gen_err" | grep -qi "legacy" && printf "%s" "$gen_err" | grep -q "status"'
assert "0046 (f): legacy status NOT applied (no project file / built-in only)" '[ ! -f "$SBX/.claude/agents/docket-status.md" ] || [ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "claude-haiku-4-5-20251001" ]'
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTL" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0046 (g'): --check flags the legacy shape (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "0046 (g'): --check names the legacy shape" 'printf "%s" "$chk_out" | grep -qi "legacy"'
rm -rf "$SBX" "$HROOTL"

# (e) Dead-config harness (a block in agents: not present in agent_harnesses) => warned + dropped.
make_sandbox
HROOTX="$(mktemp -d)"; mkdir -p "$HROOTX/.claude"
printf 'agent_harnesses: [claude]\nagents:\n  default:\n    status: { model: sonnet }\n  cursor:\n    status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.docket.yml"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTX" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0046 (e): dead-config not fatal (rc=0)" '[ "$gen_rc" = "0" ]'
assert "0046 (e): warns cursor block is not in agent_harnesses" 'printf "%s" "$gen_err" | grep -qi "cursor" && printf "%s" "$gen_err" | grep -qi "agent_harnesses"'
assert "0046 (e): cursor file NOT generated (dropped)" '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
assert "0046 (e): claude still generated from default" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
rm -rf "$SBX" "$HROOTX"
```

- [ ] **Step 6: Run to verify (f)/(e)/(g') fail**

Run: `cd /Users/homer/dev/docket/.worktrees/per-harness-agent-models && bash tests/test_sync_agents.sh 2>&1 | grep -E "0046 \((e|f|g)"`
Expected: `NOT OK` for the legacy-warn, legacy `--check`, and dead-config-warn assertions.

- [ ] **Step 7: Implement legacy detection + dead-config warning**

Add a `legacy_agent_keys` helper next to `agent_keys`:

```bash
# Pre-0046 flat shape: bare agent keys sitting DIRECTLY under agents: (or top level for global),
# i.e. neither `default` nor a known harness. One per line. Used to warn + drop + flag as --check drift.
legacy_agent_keys() {  # $1=file  $2=under_agents(0|1)
  local sub
  [ -f "$1" ] || return 0
  if [ "$2" = "1" ]; then sub="$(section_body agents < "$1")"; else sub="$(cat "$1")"; fi
  printf '%s\n' "$sub" | awk '
    { nc=$0; sub(/#.*/,"",nc) }
    /^[A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*\{/ {                  # col-0 key WITH an inline {…} value == a bare agent entry
      k=nc; sub(/[[:space:]]*:.*/,"",k); if (k!="") print k
    }'
}
```

At the top of `project_level_pass` and `user_level_pass` (once per run is enough — put it in a helper called from both, or emit in `project_level_pass` for `.docket.yml` and skip global), warn on any legacy keys:

```bash
warn_legacy_shape(){  # $1=file $2=under_agents ; warns once per bare agent key
  local k
  while IFS= read -r k; do
    [ -n "$k" ] || continue
    log "WARN legacy agents: shape — bare agent key '$k' is neither 'default' nor a known harness; ignored (use agents.default.$k or agents.<harness>.$k)."
  done < <(legacy_agent_keys "$1" "$2")
}
```

Call `warn_legacy_shape "$DOCKET_YML" 1` at the top of `project_level_pass` (guarded by `[ -f "$DOCKET_YML" ]`, already present) and `warn_legacy_shape "$GLOBAL_CFG" 0` at the top of `user_level_pass`. Because `agent_keys` only collects keys nested under a harness/`default` header, bare agent keys are already excluded from generation — the warning is the only added behavior for the ignore.

For `--check` to flag the legacy shape, add near the top of `check_project_level` (after the `agents:` block presence check):

```bash
  local legacy; legacy="$(legacy_agent_keys "$DOCKET_YML" 1)"
  if [ -n "$legacy" ]; then
    log "drift: legacy bare-agent-key agents: shape ($(printf '%s' "$legacy" | tr '\n' ' ')) — reshape to agents.default.<agent> (run: bash sync-agents.sh)"
    rc=1
  fi
```

For dead-config harnesses, add to `project_level_pass` after resolving `HARNESSES`, iterate the harness sub-blocks present in `agents:` and warn+skip any not in `HARNESSES`:

```bash
  # Warn on any agents.<harness> block whose harness is NOT in agent_harnesses (dead config).
  local cfg_h
  while IFS= read -r cfg_h; do
    [ -n "$cfg_h" ] || continue
    [ "$cfg_h" = "default" ] && continue
    case " $HARNESSES " in *" $cfg_h "*) : ;; *) log "WARN agents.$cfg_h: block is not in agent_harnesses — ignored (dead config)." ;; esac
  done < <(agents_block_harnesses "$DOCKET_YML")
```

with a helper listing the harness header names present under `agents:`:

```bash
# Harness/default header names present under agents: (the top-level keys of the harness map).
agents_block_harnesses() {  # $1=file  (docket.yml, under_agents=1)
  local sub
  [ -f "$1" ] || return 0
  sub="$(section_body agents < "$1")"
  printf '%s\n' "$sub" | awk '{ nc=$0; sub(/#.*/,"",nc) } /^[A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*$/ { k=nc; sub(/[[:space:]]*:.*/,"",k); if(k!="") print k }'
}
```

Since dead-config harnesses never appear in `HARNESSES`, they are already not generated — the warning is the only added behavior.

- [ ] **Step 8: Run the full suite green**

Run: `cd /Users/homer/dev/docket/.worktrees/per-harness-agent-models && bash tests/test_sync_agents.sh; echo "exit=$?"`
Expected: `exit=0`, all `ok - …`.

- [ ] **Step 9: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/per-harness-agent-models
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "feat(0046): legacy warn+ignore, dead-config drop, non-Claude footgun warning"
```

---

### Task 3: Documentation — `docket-convention` Agent layer + this repo's `.docket.yml` example

Update the convention's Agent-layer prose + config schema to the harness-first shape, and replace this repo's commented agent-keyed example with a harness-first one. The `docket-convention` "Task 5" tests already assert on the convention; extend them for the harness-first shape so the docs change is test-gated. **No README edit** — change #0047 delegated the config *shape* to docket-convention's Agent layer (the README references it rather than duplicating field examples), so reshaping the convention keeps the README current without touching it.

**Files:**
- Modify: `skills/docket-convention/SKILL.md` — the `.docket.yml` config schema comment for `agents:`, and the "Agent layer" `agents:` YAML example + the "Layered config" prose describing resolution.
- Modify: `.docket.yml` (repo root) — replace the commented agent-keyed `agents:` example with a commented harness-first one.
- Test: `tests/test_sync_agents.sh` — extend the Task-5 convention assertions for the harness-first shape.

**Interfaces:**
- Consumes: nothing new.
- Produces: convention prose the field-writing skills and users read; no code interface.

- [ ] **Step 1: Write the failing convention-shape assertions**

In `tests/test_sync_agents.sh`, in the Task-5 region (after line ~204), add:

```bash
# 0046: convention documents the harness-first agents: shape (default: + harness keys, field-level fallback).
assert "0046 doc: convention names the reserved default: key" 'grep -qE "default:" "$CONV" && grep -Pzoq "agents:[\s\S]{0,400}default:" "$CONV"'
assert "0046 doc: convention shows a per-harness key example (cursor)" 'grep -Pzoq "agents:[\s\S]{0,600}cursor:" "$CONV"'
assert "0046 doc: convention states field-level fallback H -> default -> built-in" 'grep -qiE "harness.*default.*built-in|<harness>.*default.*built-in" "$CONV"'
assert "0046 doc: convention notes non-Claude fallback warning" 'grep -qi "default/built-in" "$CONV"'
```

- [ ] **Step 2: Run to verify they fail**

Run: `cd /Users/homer/dev/docket/.worktrees/per-harness-agent-models && bash tests/test_sync_agents.sh 2>&1 | grep "0046 doc"`
Expected: `NOT OK` for each new convention assertion (the convention still documents the flat agent-keyed shape).

- [ ] **Step 3: Update `docket-convention` — the `agents:` config-schema comment**

In `skills/docket-convention/SKILL.md`, the `.docket.yml` schema block currently ends with:

```yaml
agents:                      # per-skill subagent model/effort (change 0016); see "Agent layer" below
```

Change the comment to point at the harness-first shape:

```yaml
agents:                      # harness-first per-skill subagent model/effort (change 0046); see "Agent layer" below
```

- [ ] **Step 4: Update `docket-convention` — the "Agent layer" YAML example + resolution prose**

Replace the agent-keyed example currently in the "Layered config" subsection:

```yaml
agents:
  implement-next: { model: opus,   effort: xhigh }
  status:         { model: sonnet, effort: medium }
  # unlisted -> built-in default; effort: auto (or omitted) -> omit the effort line (inherit model default)
```

with the harness-first shape:

```yaml
agents:                                 # harness-first: reserved `default:` + harness-name keys
  default:                              # neutral fallback for any harness without its own entry
    implement-next: { model: claude-opus-4-8, effort: xhigh }
    status:         { model: claude-haiku-4-5-20251001 }
  cursor:                               # per-harness override — only what differs
    implement-next: { model: gpt-5.1, effort: high }
    status:         { model: gpt-5.5-medium-fast }
  # Resolution is field-by-field, first non-empty wins:
  #   agents.<harness>.<agent>  ->  agents.default.<agent>  ->  shipped built-in (agents/docket-*.md)
  # effort: auto (or omitted) -> omit the effort line (inherit the model default).
  # The global ~/.config/docket/agents.yaml uses the SAME harness-first map, but at the FILE's top
  # level (no `agents:` wrapper — the file IS the map). A non-`claude` harness whose model falls to
  # default/built-in gets a non-fatal warning (likely-wrong ID; docket never validates model IDs).
  # A harness block not in `agent_harnesses`, or a bare pre-0046 agent key, is warned + ignored.
```

Update the surrounding "Layered config" prose sentence that describes the `agents:` block so it says the block is **harness-first** (`default:` + harness keys), resolution is field-level `<harness> → default → built-in`, and `agent_harnesses` (which dirs) is orthogonal to `agents.<harness>` (which values). Keep the existing precedence table (per-repo > global > built-in) and the "committed project-level files" reproducibility sentence — those are unchanged. Preserve the load-bearing substrings the Task-5 + 0045 tests assert: `per-repo > global > built-in`, `sync-agents.sh`, `abort-and-report`, `0017`, `agent_harnesses`, `[claude]`, `harness-neutral`/`direct model id`, `passthrough`, and an `ADR-0015` reference within 500 chars of `agent_harnesses` (LEARNINGS #42/#36 — verify with a targeted grep, don't relocate a sentinel just to satisfy it).

- [ ] **Step 5: Update this repo's `.docket.yml` commented example**

In `/Users/homer/dev/docket/.docket.yml`, replace the commented agent-keyed example at the end:

```yaml
# agents:
#   implement-next: { model: opus,   effort: xhigh }
#   status:         { model: sonnet, effort: medium }
```

with a harness-first commented example (functionally inert — docket dogfoods Claude Code; keeping only `default`/`claude` is byte-identical to today):

```yaml
# agents:                                # harness-first: reserved default: + harness-name keys (change 0046)
#   default:                             # neutral fallback (field-level: <harness> -> default -> built-in)
#     implement-next: { model: opus,   effort: xhigh }
#     status:         { model: sonnet, effort: medium }
#   cursor:                              # override only what differs for a non-Claude harness
#     status:         { model: gpt-5.5-medium-fast }
```

> Note: `.docket.yml` at the repo root is committed on the metadata working tree's integration branch surface; edit the copy in the **feature worktree** (`/Users/homer/dev/docket/.worktrees/per-harness-agent-models/.docket.yml`) so it rides with the code. It is a commented example only (no functional change).

- [ ] **Step 6: Run the full suite green**

Run: `cd /Users/homer/dev/docket/.worktrees/per-harness-agent-models && bash tests/test_sync_agents.sh; echo "exit=$?"`
Expected: `exit=0`, all `ok - …` including the new `0046 doc` assertions and every preserved Task-5/0045 convention sentinel.

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/per-harness-agent-models
git add skills/docket-convention/SKILL.md .docket.yml tests/test_sync_agents.sh
git commit -m "docs(0046): harness-first agents: shape in docket-convention + .docket.yml example"
```

---

## Self-Review

**Spec coverage (spec §"What the implementer edits" + test list a–h):**
- `sync-agents.sh` readers → harness scope with default fallback — Task 1 (Steps 3–4). ✓
- Per-(harness, agent) resolution in `user_level_pass` (each present harness) + `project_level_pass` (over `agent_harnesses`) — Task 1 Step 4. ✓
- Legacy bare-agent-key warned+ignored — Task 2 Step 7. ✓ (f)
- Non-Claude fallback warning — Task 2 Step 3. ✓ (h)
- `check_project_level` extends to per-harness files + flags legacy shape — Task 1 Step 4 (per-harness) + Task 2 Step 7 (legacy). ✓ (g)
- Self-contained `.docket.yml` parser (no `docket-config.sh`) — preserved; only awk/sed/grep. ✓
- `docket-convention` updated — Task 3. ✓
- `.docket.yml` example — Task 3 Step 5. ✓
- Tests (a) harness override wins — T1 S1; (b) field-level merge — T1 S1; (c) non-Claude passthrough into `.cursor/` — T1 S1; (d) default-only byte-identical — T1 S1; (e) dead-config harness warned+dropped — T2 S5/7; (f) legacy warned+ignored — T2 S5/7; (g) `--check` per-harness drift + legacy — T1 S5 (per-harness, kept from 0045) + T2 S5/7 (legacy); (h) fallback warning fires + suppressed for claude — T2 S1/3. ✓ all eight.

**Open questions handled:** Q1 (non-Claude project-over-user precedence) is a build-time *live-verification* item, not code — flagged for the results/ADR step, not a task here (docket dogfoods Claude Code; the two-layer write is unchanged). Q2 (new ADR vs `## Update` on ADR-0015) is the implement-next ADR step (skill Step 6), not a plan task. Q3 (warning keys on model provenance for non-claude only) — realized by `RES_MODEL_FROM_HARNESS` + the `[ "$1" = "claude" ] && return 0` guard in `warn_fallback_model` (Task 2 Step 3).

**Placeholder scan:** every code step shows complete bash; every test step shows the actual fixture + assertion; run/expected lines are concrete. No TBD/TODO. ✓

**Type/name consistency:** helpers introduced in Task 1 (`section_body`, `harness_agent_line`, `resolve_agent` with `RES_MODEL`/`RES_EFFORT`/`RES_MODEL_FROM_HARNESS`, `agent_keys`, `harness_of_dir`, the `warn_fallback_model` stub) are consumed with the same names/signatures in Task 2 (`warn_fallback_model` body, `legacy_agent_keys`, `agents_block_harnesses`, `warn_legacy_shape`). `field_of`/`emit`/`short_name`/`resolve_agent_harnesses` unchanged. ✓

**Intermediate-state buildability (LEARNINGS #45):** Task 1 leaves no reference to a removed symbol — it replaces `entry_line`/`block_names`/`resolve_from` and updates *every* caller in the same task, and ships a no-op `warn_fallback_model` stub so the passes reference a defined function. Task 2 only fills the stub and adds new branches/helpers. ✓
