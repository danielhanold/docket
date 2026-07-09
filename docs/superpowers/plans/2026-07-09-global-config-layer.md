# Global Config Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One global file, `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml`, accepting the full `.docket.yml` schema, resolved per-key as **per-repo > global > built-in**, with a coordination-key fence (shared-state keys warned-and-ignored globally), auto-migration of the legacy `agents.yaml`, and fail-loud misplacement/malformed guards.

**Architecture:** `docket-config.sh --export` stays the single runtime reader and gains a Stage 2b that reads the global file from the local filesystem and falls back per key (skills' Step-0 interface unchanged — still 18 emitted lines). `sync-agents.sh` moves its global read from top-level-map `agents.yaml` to the `agents:` block of `config.yml` (the existing `under_agents` parameterization already supports this), owns the idempotent `agents.yaml` → `config.yml` migration, and gains a global `agent_harnesses` that scopes its **user-level pass only**.

**Tech Stack:** bash (`set -uo pipefail` in docket-config.sh, `set -euo pipefail` in sync-agents.sh), sed/awk flat-YAML readers, hermetic bash test fixtures.

**Spec:** `.docket/docs/superpowers/specs/2026-07-09-global-config-layer-design.md` (on `origin/docket`).

## Global Constraints

- Canonical global path: `${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml` — same XDG handling as `sync-agents.sh` already uses.
- Fenced (per-repo-only, warned-and-ignored globally): `metadata_branch`, `integration_branch`, `changes_dir`, `adrs_dir`, `results_dir`, `github_project`, plus the `github` **token** of `board_surfaces`.
- Global-able: `skills:` (all five roles), `agents:` (full harness-first block), `auto_groom`, `finalize.gate`, `finalize.test_command`, `board_surfaces` minus `github`, `agent_harnesses` (user-level pass scope only).
- Warn-and-ignore is the posture for all config noise — a bad global file must never abort a run or brick a repo.
- `docket-config.sh --export` still emits exactly **18 `KEY=value` lines** in the existing order (interface unchanged).
- No dual-read fallback for `agents.yaml` after migration; the original survives as `agents.yaml.migrated`.
- The fence classification rule gets an ADR at build time — recorded by the implement-next controller via its `docket-adr` dispatch (step 6), **not a task in this plan**.
- macOS bash 3.2 compatibility: never expand a possibly-empty array under `set -u`; keep the existing space-separated-string token-list idiom.
- LEARNINGS discipline: capture producers into vars before `grep -q`/`head` (no `producer | grep -q` under pipefail); `[[:space:]]` not literal-space classes in awk/sed; one grep sentinel anchors exactly one clause.

---

### Task 1: docket-config.sh — global layer per-key resolution (scalars + skills merge)

**Files:**
- Modify: `scripts/docket-config.sh` (Stage 2b insert after line 95's `$CFG` read; resolution lines 112–140)
- Test: `tests/test_docket_config.sh`

**Interfaces:**
- Produces: shell vars `GCFG_DIR` (global config dir), `GCFG` (global config path, reset to `/dev/null` by Task 2's guards), helper `gbl <key>` (global-layer scalar read), helper `skill_role <role> <default>`. Task 2 inserts its fence/guard block between the `GCFG` setup and the first `gbl` consumer.
- Consumes: existing `yaml_get <file> <key>`, `yaml_block_body <file> <key>` (unchanged).

- [ ] **Step 1: Make the test file hermetic + add the failing fixtures**

In `tests/test_docket_config.sh`, immediately after line 29 (`tmp="$(mktemp -d)"; trap …`), add the hermetic pin and the `rung`/`rung_rc` helpers:

```bash
# Hermetic: never read the dev machine's real global config (change 0050 — docket-config.sh
# now reads ${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml). Point XDG at a void.
export XDG_CONFIG_HOME="$tmp/xdg-void"
# rung <xdgdir> <repodir> [args...] : run the resolver with the global layer rooted at <xdgdir>
rung(){ local x="$1" d="$2"; shift 2; XDG_CONFIG_HOME="$x" bash "$SCRIPT" --repo-dir "$d" "$@"; }
rung_rc(){ local x="$1" d="$2"; shift 2; XDG_CONFIG_HOME="$x" bash "$SCRIPT" --repo-dir "$d" "$@" >/dev/null 2>&1; echo $?; }
```

Append the Task-1 fixtures at the end of the file (before the final `if [ "$fail" = 0 ]` block):

```bash
# ============================================================================
# Change 0050 — global config layer (~/.config/docket/config.yml)
# ============================================================================

# --- (K) global-only keys honored (repo has no .docket.yml) ------------------
mkrepo "$tmp/k"
mkdir -p "$tmp/k.xdg/docket"
cat > "$tmp/k.xdg/docket/config.yml" <<'EOF'
auto_groom: true
finalize:
  gate: ci
skills:
  build: auto
EOF
out="$(rung "$tmp/k.xdg" "$tmp/k" --export)"; eval "$out"
assert "0050 K: global auto_groom honored"          '[ "$AUTO_GROOM" = true ]'
assert "0050 K: global finalize.gate honored"       '[ "$FINALIZE_GATE" = ci ]'
assert "0050 K: global skills.build honored"        '[ "$SKILL_BUILD" = auto ]'
assert "0050 K: unset key stays built-in (inline)"  '[ "$BOARD_SURFACES" = inline ]'
assert "0050 K: unset skill role stays default"     '[ "$SKILL_PLAN" = superpowers:writing-plans ]'

# --- (L) per-repo overrides global, field-by-field skills merge --------------
mkrepo "$tmp/l"
cat > "$tmp/l/.docket.yml" <<'EOF'
metadata_branch: main
auto_groom: false
skills:
  plan: superpowers:writing-plans
EOF
git -C "$tmp/l" add .docket.yml; git -C "$tmp/l" commit --quiet -m cfg
git -C "$tmp/l" push --quiet origin main
mkdir -p "$tmp/l.xdg/docket"
cat > "$tmp/l.xdg/docket/config.yml" <<'EOF'
auto_groom: true
skills:
  plan: auto
  review: my-org:global-review
EOF
out="$(rung "$tmp/l.xdg" "$tmp/l" --export)"; eval "$out"
assert "0050 L: per-repo auto_groom false beats global true" '[ "$AUTO_GROOM" = false ]'
assert "0050 L: skills merge — repo plan wins over global"   '[ "$SKILL_PLAN" = superpowers:writing-plans ]'
assert "0050 L: skills merge — global review holds"          '[ "$SKILL_REVIEW" = my-org:global-review ]'
assert "0050 L: skills merge — unset role stays default"     '[ "$SKILL_BUILD" = superpowers:subagent-driven-development ]'

# --- (Q) XDG_CONFIG_HOME honored; HOME/.config is the fallback ---------------
mkrepo "$tmp/q"
mkdir -p "$tmp/q.home/.config/docket"
printf 'auto_groom: true\n' > "$tmp/q.home/.config/docket/config.yml"
out="$(env -u XDG_CONFIG_HOME HOME="$tmp/q.home" bash "$SCRIPT" --repo-dir "$tmp/q" --export)"; eval "$out"
assert "0050 Q: XDG unset -> \$HOME/.config fallback read"   '[ "$AUTO_GROOM" = true ]'

# --- (E') emit-interface guard: still exactly 18 lines with a global file present ---
n50="$(rung "$tmp/k.xdg" "$tmp/k" --export | grep -c '=')"
assert "0050 E': still 18 KEY=value lines with global layer" '[ "$n50" -eq 18 ]'
```

- [ ] **Step 2: Run the new fixtures to verify they fail**

Run: `bash tests/test_docket_config.sh 2>&1 | grep "0050"`
Expected: `NOT OK - 0050 K: global auto_groom honored`, `NOT OK - 0050 K: global finalize.gate honored`, `NOT OK - 0050 K: global skills.build honored`, `NOT OK - 0050 L: skills merge — global review holds`, `NOT OK - 0050 Q: …` (the pre-change script never reads the global file). The L per-repo-wins asserts and E' may already pass — that is fine; the honored-global asserts must be RED.

- [ ] **Step 3: Implement Stage 2b in `scripts/docket-config.sh`**

Insert after line 95 (`g show "origin/HEAD:.docket.yml" >"$CFG" … : >"$CFG"`):

```bash
# --- Stage 2b: global config layer (change 0050) ------------------------------
# ${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml — the full .docket.yml schema,
# resolved PER-KEY: per-repo > global > built-in (map-valued skills: merges field-by-field).
# Read from the LOCAL filesystem — the file is per-machine by definition, so there is no
# authoritative-ref concern as with .docket.yml's origin/HEAD read. Coordination keys are
# fenced (warned-and-ignored) in Stage 2c below.
GCFG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/docket"
GCFG="$GCFG_DIR/config.yml"
gbl(){ yaml_get "$GCFG" "$1"; }   # global-layer scalar read (empty when absent)
```

Replace the resolution lines for the three global-able scalars (current lines 112–114):

```bash
FINALIZE_GATE="$(yaml_get "$CFG" gate)";      FINALIZE_GATE="${FINALIZE_GATE:-$(gbl gate)}"; FINALIZE_GATE="${FINALIZE_GATE:-local}"
FINALIZE_TEST_COMMAND="$(yaml_get "$CFG" test_command)"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(gbl test_command)}"
AUTO_GROOM="$(yaml_get "$CFG" auto_groom)";   AUTO_GROOM="${AUTO_GROOM:-$(gbl auto_groom)}"; AUTO_GROOM="${AUTO_GROOM:-false}"
```

`METADATA_BRANCH`, `INTEGRATION_BRANCH`, `CHANGES_DIR`, `ADRS_DIR`, `RESULTS_DIR` keep their repo-only reads verbatim (they are fenced; Task 2 adds the warning).

Replace the `board_surfaces` read (current lines 116–122) with a raw-value fallback — the fallback happens on the RAW value (before bracket-stripping) so a global `[]` is distinguishable from unset:

```bash
bs_raw="$(yaml_get "$CFG" board_surfaces)"; bs_from_global=0
if [ -z "$bs_raw" ]; then
  bs_raw="$(gbl board_surfaces)"
  [ -n "$bs_raw" ] && bs_from_global=1
fi
if [ -z "$bs_raw" ]; then
  BOARD_SURFACES="inline"                                  # unset in both layers => default [inline]
else
  bs="${bs_raw#[}"; bs="${bs%]}"; bs="${bs//,/ }"
  BOARD_SURFACES="$(echo $bs)"                             # trim/collapse; "[]" => ""
fi
```

(`bs_from_global` is consumed by Task 2's `github`-token drop; it is introduced here so the parse is written once.)

Replace the `skills:` block (current lines 126–140) with the two-layer field-by-field merge:

```bash
# --- skills: role-keyed pluggable workflow skills (change 0049 + 0050 global layer) ---
# Nested block; each leaf read within the block only. Per-key precedence:
# per-repo leaf > global leaf > the superpowers default.
SKILLS_BLK="$(mktemp)";  yaml_block_body "$CFG"  skills >"$SKILLS_BLK"
GSKILLS_BLK="$(mktemp)"; yaml_block_body "$GCFG" skills >"$GSKILLS_BLK"
skill_role(){  # skill_role <role> <default> -> resolved value on stdout
  local v; v="$(yaml_get "$SKILLS_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$GSKILLS_BLK" "$1")"
  printf '%s' "${v:-$2}"
}
SKILL_BRAINSTORM="$(skill_role brainstorm superpowers:brainstorming)"
SKILL_PLAN="$(skill_role plan superpowers:writing-plans)"
SKILL_BUILD="$(skill_role build superpowers:subagent-driven-development)"
SKILL_REVIEW="$(skill_role review superpowers:requesting-code-review)"
SKILL_FINISH="$(skill_role finish superpowers:finishing-a-development-branch)"
# Unknown role keys in EITHER layer: warn-and-ignore (a typo must never abort).
for _blk in "$SKILLS_BLK" "$GSKILLS_BLK"; do
  while IFS= read -r _role; do
    [ -n "$_role" ] || continue
    case " brainstorm plan build review finish " in
      *" $_role "*) ;;
      *) printf 'docket-config: warning: unknown skills role %s — ignored\n' "$_role" >&2 ;;
    esac
  done < <(sed -n -E 's/^[[:space:]]*([[:alnum:]_-]+)[[:space:]]*:.*/\1/p' "$_blk")
done
rm -f "$SKILLS_BLK" "$GSKILLS_BLK"
```

Note `yaml_get`/`yaml_block_body` both start with `[ -f "$1" ] || return` — an absent `config.yml` reads as empty with no special-casing.

- [ ] **Step 4: Run the full test file to verify it passes**

Run: `bash tests/test_docket_config.sh`
Expected: `PASS` (every pre-existing fixture still green — the hermetic XDG pin means no dev-machine global file can leak in — plus all 0050 K/L/Q/E' asserts `ok`).

- [ ] **Step 5: Commit**

```bash
git add scripts/docket-config.sh tests/test_docket_config.sh
git commit -m "feat(0050): docket-config.sh reads the global config layer per-key (scalars + skills merge)"
```

---

### Task 2: docket-config.sh — coordination-key fence + misplacement/malformed guards

**Files:**
- Modify: `scripts/docket-config.sh` (insert Stage 2c after Task 1's Stage 2b `gbl` definition; extend the board_surfaces parse)
- Test: `tests/test_docket_config.sh`

**Interfaces:**
- Consumes: Task 1's `GCFG_DIR`, `GCFG`, `gbl`, `bs_from_global`.
- Produces: `GCFG` may be reset to `/dev/null` (all later `gbl` reads then return empty — the built-ins-fallback path).

- [ ] **Step 1: Write the failing fixtures**

Append to `tests/test_docket_config.sh` (after the Task-1 0050 fixtures):

```bash
# --- (M) coordination-key fence: warned-and-ignored, never honored, never fatal ---
mkrepo "$tmp/m"
mkdir -p "$tmp/m.xdg/docket"
cat > "$tmp/m.xdg/docket/config.yml" <<'EOF'
metadata_branch: main
changes_dir: elsewhere/changes
auto_groom: true
EOF
merr="$(rung "$tmp/m.xdg" "$tmp/m" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/m.xdg" "$tmp/m" --export 2>/dev/null)"; eval "$out"
assert "0050 M: fence warns metadata_branch"        'printf "%s" "$merr" | grep -q "metadata_branch"'
assert "0050 M: fence names per-repo-only"          'printf "%s" "$merr" | grep -qi "per-repo-only"'
assert "0050 M: fence warns changes_dir"            'printf "%s" "$merr" | grep -q "changes_dir"'
assert "0050 M: global metadata_branch NOT honored" '[ "$METADATA_BRANCH" = docket ]'
assert "0050 M: CHANGES_DIR stays default"          '[ "$CHANGES_DIR" = docs/changes ]'
assert "0050 M: global-able key in same file still honored" '[ "$AUTO_GROOM" = true ]'
assert "0050 M: fence is not fatal (exit 0)"        '[ "$(rung_rc "$tmp/m.xdg" "$tmp/m" --export)" -eq 0 ]'

# --- (N) global board_surfaces: github token dropped; [] and [inline] work -------
mkrepo "$tmp/n"
mkdir -p "$tmp/n.xdg/docket"
printf 'board_surfaces: [inline, github]\n' > "$tmp/n.xdg/docket/config.yml"
nerr="$(rung "$tmp/n.xdg" "$tmp/n" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/n.xdg" "$tmp/n" --export 2>/dev/null)"; eval "$out"
assert "0050 N: global github token warned"         'printf "%s" "$nerr" | grep -q "github"'
assert "0050 N: global github token dropped"        '[ "$BOARD_SURFACES" = inline ]'
printf 'board_surfaces: []\n' > "$tmp/n.xdg/docket/config.yml"
out="$(rung "$tmp/n.xdg" "$tmp/n" --export 2>/dev/null)"; eval "$out"
assert "0050 N: global [] honored (board disabled)"  '[ -z "$BOARD_SURFACES" ]'
# per-repo github is untouched by the fence:
mkrepo "$tmp/n2"
printf 'metadata_branch: main\nboard_surfaces: [inline, github]\n' > "$tmp/n2/.docket.yml"
git -C "$tmp/n2" add .docket.yml; git -C "$tmp/n2" commit --quiet -m cfg
git -C "$tmp/n2" push --quiet origin main
n2err="$(rung "$tmp/n.xdg" "$tmp/n2" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/n.xdg" "$tmp/n2" --export 2>/dev/null)"; eval "$out"
assert "0050 N: per-repo github honored"            '[ "$BOARD_SURFACES" = "inline github" ]'
assert "0050 N: per-repo github NOT warned"         '! printf "%s" "$n2err" | grep -q "board_surfaces token github"'

# --- (O) misplacement guard: ~/.config/docket/.docket.yml is warned, never read ---
mkrepo "$tmp/o"
mkdir -p "$tmp/o.xdg/docket"
printf 'auto_groom: true\n' > "$tmp/o.xdg/docket/.docket.yml"
oerr="$(rung "$tmp/o.xdg" "$tmp/o" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/o.xdg" "$tmp/o" --export 2>/dev/null)"; eval "$out"
assert "0050 O: misplacement warned, names config.yml" 'printf "%s" "$oerr" | grep -q "config.yml"'
assert "0050 O: misplaced file NOT read (auto_groom default)" '[ "$AUTO_GROOM" = false ]'
assert "0050 O: misplacement not fatal (exit 0)"    '[ "$(rung_rc "$tmp/o.xdg" "$tmp/o" --export)" -eq 0 ]'

# --- (P) malformed global file: warned, built-ins fallback, repos not bricked -----
mkrepo "$tmp/p"
mkdir -p "$tmp/p.xdg/docket/config.yml"            # a DIRECTORY at the config path
perr="$(rung "$tmp/p.xdg" "$tmp/p" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/p.xdg" "$tmp/p" --export 2>/dev/null)"; eval "$out"
assert "0050 P: malformed global warned"            'printf "%s" "$perr" | grep -qi "not a readable regular file"'
assert "0050 P: built-ins fallback (auto_groom)"    '[ "$AUTO_GROOM" = false ]'
assert "0050 P: malformed global not fatal (exit 0)" '[ "$(rung_rc "$tmp/p.xdg" "$tmp/p" --export)" -eq 0 ]'
```

- [ ] **Step 2: Run the new fixtures to verify they fail**

Run: `bash tests/test_docket_config.sh 2>&1 | grep "0050 [MNOP]"`
Expected: RED on every warning assert (`fence warns metadata_branch`, `github token warned`, `misplacement warned`, `malformed global warned`) and on `0050 M: global metadata_branch NOT honored`? — **no**: metadata_branch is currently never read globally, so the NOT-honored asserts pass already; the *warning* asserts and the `github`-drop assert must be RED. (`0050 N: global github token dropped` is RED because Task 1 honors the whole global list including `github`.)

- [ ] **Step 3: Implement Stage 2c (fence + guards) in `scripts/docket-config.sh`**

Insert immediately after Task 1's `gbl(){ …}` line (so guards run before any `gbl` consumer):

```bash
# --- Stage 2c: fail-loud guards + the coordination-key fence (change 0050) ----
# Misplacement: a global .docket.yml is NEVER read — the global file is config.yml.
if [ -e "$GCFG_DIR/.docket.yml" ]; then
  printf 'docket-config: warning: %s/.docket.yml is not read — global config is config.yml, not .docket.yml (did you mean %s?)\n' "$GCFG_DIR" "$GCFG" >&2
fi
# Malformed/unreadable: warn and fall back to built-ins for the GLOBAL layer only
# (a broken personal file must not brick every repo; per-repo config is still honored).
if [ -e "$GCFG" ] && { [ ! -f "$GCFG" ] || [ ! -r "$GCFG" ]; }; then
  printf 'docket-config: warning: %s is not a readable regular file — global config layer ignored\n' "$GCFG" >&2
  GCFG=/dev/null
fi
# Coordination-key fence: a key whose effect writes SHARED state (commits on shared
# branches, committed generated files, external GitHub objects) is per-repo-only; a global
# value is loudly warned-and-ignored — never honored, never fatal. (ADR records the rule.)
for _fkey in metadata_branch integration_branch changes_dir adrs_dir results_dir github_project; do
  if [ -n "$(yaml_get "$GCFG" "$_fkey")" ]; then
    printf "docket-config: warning: global config key %s is per-repo-only — set it in the repo's committed .docket.yml; ignored\n" "$_fkey" >&2
  fi
done
```

Then extend the `board_surfaces` parse (after Task 1's `BOARD_SURFACES="$(echo $bs)"` line) with the global-`github` drop:

```bash
# The github token is per-repo-only when it arrives from the GLOBAL layer: it mints
# issues + a Projects board (external objects, not self-healing). Per-repo github is honored.
if [ "$bs_from_global" -eq 1 ] && [ -n "$BOARD_SURFACES" ]; then
  _filtered=""
  for _tok in $BOARD_SURFACES; do
    if [ "$_tok" = github ]; then
      printf 'docket-config: warning: global board_surfaces token github is per-repo-only (mints external GitHub objects) — ignored\n' >&2
    else
      _filtered="$_filtered $_tok"
    fi
  done
  BOARD_SURFACES="$(echo $_filtered)"
fi
```

Note: `yaml_get` on `GCFG=/dev/null` returns empty (`[ -f /dev/null ]` is false — it is a character device), so a fenced/malformed file needs no further special-casing downstream.

- [ ] **Step 4: Run the full test file to verify it passes**

Run: `bash tests/test_docket_config.sh`
Expected: `PASS` — all pre-existing + 0050 K/L/Q/E'/M/N/O/P fixtures `ok`.

- [ ] **Step 5: Commit**

```bash
git add scripts/docket-config.sh tests/test_docket_config.sh
git commit -m "feat(0050): coordination-key fence + misplacement/malformed guards for the global layer"
```

---

### Task 3: sync-agents.sh — global agents from config.yml + agents.yaml auto-migration

**Files:**
- Modify: `sync-agents.sh` (lines 30–31 global path; new `migrate_legacy_global()`; `user_level_pass` reads `under_agents=1`)
- Test: `tests/test_sync_agents.sh` (update lines 72–91 to the new canonical shape; add migration fixtures)

**Interfaces:**
- Produces: `GLOBAL_CFG_DIR`, `GLOBAL_CFG` (now `…/docket/config.yml`), `LEGACY_GLOBAL_CFG` (`…/docket/agents.yaml`), `migrate_legacy_global()` (called once before the passes). All `GLOBAL_CFG` reads switch to `under_agents=1`.
- Consumes: existing `resolve_agent <file> <harness> <agent> <under_agents>`, `warn_legacy_shape <file> <under_agents>`, `section_body`, `log`.

- [ ] **Step 1: Update the existing global-layer fixtures to the canonical config.yml shape and add the failing migration fixtures**

In `tests/test_sync_agents.sh`, replace lines 72–91 (the two `agents.yaml` global-layer scenarios) with the same scenarios written against `config.yml` + `agents:` wrapper:

```bash
# -- global layer (harness-first, change 0050): config.yml agents: default: block overrides model/effort --
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'agents:\n  default:\n    status: { model: haiku, effort: low }\n    implement-next: { effort: auto }\n' > "$SBX/.config/docket/config.yml"
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
printf 'agents:\n  default:\n    status: { model: haiku }\n  cursor:\n    status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.config/docket/config.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "global cursor block wins for cursor" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast" ]'
assert "global claude falls to default" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "haiku" ]'
rm -rf "$SBX"
```

Then append the migration fixtures at the end of the file (before the final `exit $fail`):

```bash
# ============================================================================
# Change 0050 — agents.yaml -> config.yml auto-migration (owned by sync-agents.sh)
# ============================================================================

# Happy path: agents.yaml (old top-level harness-first map) is rewritten under agents:
# in config.yml, the original renamed .migrated, the run logs loudly, values apply.
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'default:\n  status: { model: haiku, effort: low }\n' > "$SBX/.config/docket/agents.yaml"
mig_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null)"
assert "0050 mig: config.yml gains an agents: block" 'grep -qE "^agents[[:space:]]*:" "$SBX/.config/docket/config.yml"'
assert "0050 mig: old file renamed to .migrated" '[ -f "$SBX/.config/docket/agents.yaml.migrated" ] && [ ! -e "$SBX/.config/docket/agents.yaml" ]'
assert "0050 mig: logs the migration loudly" 'printf "%s" "$mig_err" | grep -qi "migrat"'
assert "0050 mig: migrated values applied to wrappers" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "haiku" ]'
# Idempotency: a second run leaves config.yml byte-identical (no duplicate agents: block).
cfg_before="$(cat "$SBX/.config/docket/config.yml")"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
cfg_after="$(cat "$SBX/.config/docket/config.yml")"
assert "0050 mig: second run no-ops on config.yml" '[ "$cfg_before" = "$cfg_after" ]'
assert "0050 mig: exactly one agents: block" '[ "$(grep -cE "^agents[[:space:]]*:" "$SBX/.config/docket/config.yml")" = "1" ]'
rm -rf "$SBX"

# Migration preserves pre-existing non-agents config.yml content.
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'auto_groom: true\n' > "$SBX/.config/docket/config.yml"
printf 'default:\n  status: { model: haiku }\n' > "$SBX/.config/docket/agents.yaml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 mig: pre-existing config.yml keys preserved" 'grep -q "^auto_groom: true" "$SBX/.config/docket/config.yml"'
assert "0050 mig: agents: appended alongside" 'grep -qE "^agents[[:space:]]*:" "$SBX/.config/docket/config.yml"'
assert "0050 mig: values from the appended block apply" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "haiku" ]'
rm -rf "$SBX"

# Stale twin: config.yml already has agents: AND a live agents.yaml is present ->
# warn stale, do NOT read it, do NOT rename it (only the migration renames).
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'agents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.config/docket/config.yml"
printf 'default:\n  status: { model: haiku }\n' > "$SBX/.config/docket/agents.yaml"
stale_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null)"
assert "0050 stale: warns agents.yaml is stale/unread" 'printf "%s" "$stale_err" | grep -qi "stale"'
assert "0050 stale: config.yml value wins" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "0050 stale: agents.yaml left in place" '[ -f "$SBX/.config/docket/agents.yaml" ]'
rm -rf "$SBX"

# No dual-read: a lone agents.yaml.migrated (post-migration state) is never read.
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'default:\n  status: { model: haiku }\n' > "$SBX/.config/docket/agents.yaml.migrated"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 no-dual-read: .migrated is not read (built-in model)" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "claude-haiku-4-5-20251001" ]'
rm -rf "$SBX"
```

- [ ] **Step 2: Run the fixtures to verify the new ones fail**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep -E "NOT OK"`
Expected: the two updated global-layer scenarios FAIL (`global default sets model` etc. — the script still reads `agents.yaml` top-level, not `config.yml` `agents:`), and every `0050 mig`/`0050 stale` assert FAILS (no migration exists). Pre-existing scenarios stay `ok`.

- [ ] **Step 3: Implement in `sync-agents.sh`**

Replace line 31:

```bash
GLOBAL_CFG_DIR="${XDG_CONFIG_HOME:-$HARNESS_ROOT/.config}/docket"
GLOBAL_CFG="$GLOBAL_CFG_DIR/config.yml"
LEGACY_GLOBAL_CFG="$GLOBAL_CFG_DIR/agents.yaml"
```

Add the migration function (after the `log()` helper), and call it before the passes:

```bash
# --- agents.yaml -> config.yml auto-migration (change 0050) -------------------
# Idempotent: (1) live agents.yaml + config.yml WITHOUT an agents: block -> rewrite the old
# top-level harness-first map under agents: in config.yml (creating the file if needed),
# rename the original to .migrated (git-less users keep a copy), log loudly. (2) config.yml
# already has agents: and a live agents.yaml is also present -> warn stale, do not read it.
# After this change the global agent config is read ONLY from config.yml (no dual-read).
migrate_legacy_global(){
  [ -f "$LEGACY_GLOBAL_CFG" ] || return 0
  if [ -f "$GLOBAL_CFG" ] && grep -qE '^agents[[:space:]]*:' "$GLOBAL_CFG"; then
    log "WARN $LEGACY_GLOBAL_CFG is STALE and unread — global agent config lives under agents: in $GLOBAL_CFG; delete or rename the old file"
    return 0
  fi
  {
    printf 'agents:\n'
    sed 's/^\(.\)/  \1/' "$LEGACY_GLOBAL_CFG"    # indent every non-empty line under agents:
  } >> "$GLOBAL_CFG"
  mv "$LEGACY_GLOBAL_CFG" "$LEGACY_GLOBAL_CFG.migrated"
  log "MIGRATED global agent config: $LEGACY_GLOBAL_CFG -> agents: block in $GLOBAL_CFG (original kept at $LEGACY_GLOBAL_CFG.migrated)"
}
```

Call it at the bottom, before the passes (migration must run before `--check` too, since `--check` exits early — but `--check` is per-repo-only and never reads the global file, so place the call on the normal path only, right before `user_level_pass`):

```bash
resolve_agent_harnesses

if [ "$CHECK" = "1" ]; then
  if check_project_level; then exit 0; else exit 1; fi
fi

migrate_legacy_global
user_level_pass
project_level_pass
prune_orphans all
log "done"
```

Switch the two `GLOBAL_CFG` reads in `user_level_pass` to `under_agents=1`:

```bash
warn_legacy_shape "$GLOBAL_CFG" 1
…
resolve_agent "$GLOBAL_CFG" "$harness" "$name" 1
```

Update the header comment (line 10) to:

```bash
#   global    ~/.config/docket/config.yml `agents:` block -> user-level ~/.claude/agents/docket-*.md
#             (the legacy ~/.config/docket/agents.yaml is auto-migrated into it, then renamed .migrated)
```

- [ ] **Step 4: Run the full test file to verify it passes**

Run: `bash tests/test_sync_agents.sh; echo "exit=$?"`
Expected: `exit=0`, no `NOT OK` lines. Note the legacy-shape scenario at old lines 583–594 (`agents:\n  status: {…}` bare key in `.docket.yml`) is per-repo and unaffected; the `under_agents=1` global read makes `warn_legacy_shape "$GLOBAL_CFG" 1` flag a bare agent key directly under a global `agents:` block the same way — correct and consistent.

- [ ] **Step 5: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "feat(0050): sync-agents.sh reads global agents from config.yml; auto-migrates agents.yaml"
```

---

### Task 4: sync-agents.sh — global agent_harnesses scopes the user-level pass

**Files:**
- Modify: `sync-agents.sh` (`resolve_global_agent_harnesses()`, `compute_user_targets()`, rework `user_level_pass` to iterate harness tokens)
- Test: `tests/test_sync_agents.sh`

**Interfaces:**
- Produces: `USER_HARNESSES_SET` (0|1), `USER_HARNESSES` (tokens; meaningful only when SET=1), `USER_TARGETS` (the user-level pass's final harness token list, space-separated).
- Consumes: `is_valid_harness`, `harness_of_dir`, `HARNESS_AGENT_DIRS`, `HARNESS_HAS_DISPATCH_RULES`, Task 3's `GLOBAL_CFG`.

- [ ] **Step 1: Write the failing fixtures**

Append to `tests/test_sync_agents.sh`:

```bash
# ============================================================================
# Change 0050 — global agent_harnesses scopes the USER-LEVEL pass only
# ============================================================================

# Extends + narrows: the global list overrides presence-on-disk detection.
make_sandbox                                   # creates .claude + .agents; .cursor ABSENT
mkdir -p "$SBX/.config/docket"
printf 'agent_harnesses: [claude, cursor]\n' > "$SBX/.config/docket/config.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah: listed ABSENT harness extended (cursor created+written)" '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
assert "0050 gah: listed present harness written (claude)" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0050 gah: present-but-UNLISTED harness narrowed (.agents untouched)" '[ ! -e "$SBX/.agents/agents/docket-status.md" ]'
assert "0050 gah: user-level cursor dispatch rule written when cursor listed" '[ -f "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
rm -rf "$SBX"

# Global [] => the user-level pass writes nothing (explicit empty list, not "unset").
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'agent_harnesses: []\n' > "$SBX/.config/docket/config.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah []: no user-level files written despite present .claude" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX"

# Unset global key => presence-on-disk detection unchanged (regression pin).
make_sandbox
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah unset: presence detection still writes .claude" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0050 gah unset: absent harness still skipped" '[ ! -d "$SBX/.cursor/agents" ]'
rm -rf "$SBX"

# Unknown token in the GLOBAL list: warned + dropped, not fatal.
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'agent_harnesses: [claude, bogus]\n' > "$SBX/.config/docket/config.yml"
gah_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null)"; gah_rc=$?
assert "0050 gah unknown: not fatal (rc=0)" '[ "$gah_rc" = "0" ]'
assert "0050 gah unknown: warns and names the token" 'printf "%s" "$gah_err" | grep -qi "unknown agent_harnesses token" && printf "%s" "$gah_err" | grep -q "bogus"'
assert "0050 gah unknown: known harness still written" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX"

# Scope split: the global key never opts a repo into per-repo generation, and the
# per-repo committed pass is governed SOLELY by the repo's own agent_harnesses.
REPO50="$(mktemp -d)"; HROOT50="$(mktemp -d)"
mkdir -p "$HROOT50/.claude" "$HROOT50/.config/docket"
printf 'metadata_branch: docket\n' > "$REPO50/.docket.yml"          # tracking-only repo
printf 'agent_harnesses: [claude]\n' > "$HROOT50/.config/docket/config.yml"
( cd "$REPO50" && DOCKET_HARNESS_ROOT="$HROOT50" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah scope: global key does NOT opt repo into per-repo generation" '[ ! -e "$REPO50/.claude/agents/docket-status.md" ]'
assert "0050 gah scope: user-level still written" '[ -f "$HROOT50/.claude/agents/docket-status.md" ]'
rm -rf "$REPO50" "$HROOT50"

REPO51="$(mktemp -d)"; HROOT51="$(mktemp -d)"
mkdir -p "$HROOT51/.claude" "$HROOT51/.config/docket"
printf 'agent_harnesses: [claude]\n' > "$REPO51/.docket.yml"        # repo opts in: claude only
printf 'agent_harnesses: [cursor]\n' > "$HROOT51/.config/docket/config.yml"
( cd "$REPO51" && DOCKET_HARNESS_ROOT="$HROOT51" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah scope: per-repo pass follows the REPO list (claude written)" '[ -f "$REPO51/.claude/agents/docket-status.md" ]'
assert "0050 gah scope: per-repo pass ignores the global list (no repo .cursor)" '[ ! -e "$REPO51/.cursor/agents/docket-status.md" ]'
assert "0050 gah scope: global [cursor] scopes user-level (cursor written)" '[ -f "$HROOT51/.cursor/agents/docket-status.md" ]'
assert "0050 gah scope: user-level claude NOT written (narrowed by global list)" '[ ! -e "$HROOT51/.claude/agents/docket-status.md" ]'
rm -rf "$REPO51" "$HROOT51"
```

- [ ] **Step 2: Run the fixtures to verify they fail**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep "0050 gah"`
Expected: RED on `listed ABSENT harness extended`, `present-but-UNLISTED harness narrowed`, `gah []`, `gah unknown: warns`, and the two scope-split narrowing asserts. The `gah unset` regression asserts pass already (presence detection is current behavior).

- [ ] **Step 3: Implement in `sync-agents.sh`**

Add after `resolve_agent_harnesses()` (mirrors its parsing; reads the GLOBAL file's top-level key):

```bash
# Resolve the GLOBAL agent_harnesses (config.yml top-level key) — change 0050. Scope: it
# overrides the user-level pass's presence-on-disk selector ONLY; the per-repo committed
# pass is governed solely by the repo's own agent_harnesses (a global value shaping
# committed files would fail --check on every other machine). Unset => USER_HARNESSES_SET=0
# (presence detection); set (even to []) => the list governs.
resolve_global_agent_harnesses(){
  local raw list tok
  USER_HARNESSES_SET=0; USER_HARNESSES=""
  raw=""
  if [ -f "$GLOBAL_CFG" ]; then
    raw="$(sed -n -E 's/^agent_harnesses[[:space:]]*:[[:space:]]*([^#]*).*/\1/p' "$GLOBAL_CFG")"
    raw="$(head -n1 <<<"$raw" | sed -E 's/[[:space:]]+$//')"
  fi
  [ -n "$raw" ] || return 0
  USER_HARNESSES_SET=1
  list="${raw#[}"; list="${list%]}"; list="${list//,/ }"
  set -f
  for tok in $list; do
    if is_valid_harness "$tok"; then
      USER_HARNESSES="$USER_HARNESSES $tok"
    else
      log "unknown agent_harnesses token '$tok' in $GLOBAL_CFG — ignored"
    fi
  done
  set +f
  USER_HARNESSES="$(echo $USER_HARNESSES)"
}

# The user-level pass's final harness token list: the global agent_harnesses when set
# (extends: absent dirs are created; narrows: unlisted present dirs are skipped), else
# every harness root present on disk. Space-separated string (bash-3.2-safe under set -u).
compute_user_targets(){
  local dir
  if [ "$USER_HARNESSES_SET" = "1" ]; then
    USER_TARGETS="$USER_HARNESSES"
  else
    USER_TARGETS=""
    for dir in "${HARNESS_AGENT_DIRS[@]}"; do
      if [ -d "$(dirname "$dir")" ]; then
        USER_TARGETS="$USER_TARGETS $(harness_of_dir "$dir")"
      fi
    done
    USER_TARGETS="$(echo $USER_TARGETS)"
  fi
  return 0
}
```

Rework `user_level_pass` to iterate harness tokens (replacing the dir-loop body):

```bash
user_level_pass() {  # built-in ⊕ global -> each user-level target harness, resolved per (harness, agent)
  local src dir name harness
  warn_legacy_shape "$GLOBAL_CFG" 1
  compute_user_targets
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    for harness in $USER_TARGETS; do
      dir="$HARNESS_ROOT/.$harness/agents"
      resolve_agent "$GLOBAL_CFG" "$harness" "$name" 1
      warn_fallback_model "$harness" "$name"
      mkdir -p "$dir"
      emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$dir/$(basename "$src")"
    done
  done
  # Cursor-only dispatch rule, user-level, for each targeted dispatch-rule harness.
  local drh
  for drh in $HARNESS_HAS_DISPATCH_RULES; do
    case " $USER_TARGETS " in *" $drh "*) write_dispatch_rule "$HARNESS_ROOT/.$drh" ;; esac
  done
}
```

Call `resolve_global_agent_harnesses` at the bottom, next to its per-repo sibling (it must run after `migrate_legacy_global` would have created `config.yml`? No — migration only ever *appends an `agents:` block*, never an `agent_harnesses:` key, so order vs. migration is immaterial; keep it simple):

```bash
resolve_agent_harnesses

if [ "$CHECK" = "1" ]; then
  if check_project_level; then exit 0; else exit 1; fi
fi

migrate_legacy_global
resolve_global_agent_harnesses
user_level_pass
project_level_pass
prune_orphans all
log "done"
```

(`resolve_global_agent_harnesses` after `migrate_legacy_global` so a freshly-migrated `config.yml` is the file inspected — same-run correctness for the `-f "$GLOBAL_CFG"` probe.)

Also update the header's Layers block (line 8–13) to name the user-level scope of a global `agent_harnesses:`.

Note the presence-detection path (`USER_HARNESSES_SET=0`) is byte-identical in behavior to today: same dirs, same order, same dispatch-rule condition (`cursor` present ⇔ `cursor ∈ USER_TARGETS`).

- [ ] **Step 4: Run the full test file to verify it passes**

Run: `bash tests/test_sync_agents.sh; echo "exit=$?"`
Expected: `exit=0`, no `NOT OK` lines (including the pre-existing user-level scenarios at lines 56–70, which exercise the presence-detection path).

- [ ] **Step 5: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "feat(0050): global agent_harnesses scopes sync-agents.sh's user-level pass"
```

---

### Task 5: Documentation — README, convention, script contract, install.sh comment

**Files:**
- Modify: `README.md` (lines 62, 65, new section after line 97, Tuning section lines 227–247)
- Modify: `skills/docket-convention/SKILL.md` (Configuration section; Agent layer table + YAML comment + harness-first paragraph + 0045 paragraph)
- Modify: `scripts/docket-config.md` (Stage 2b/2c, key table, invariants)
- Modify: `install.sh` (line 11 comment)
- Test: `tests/test_sync_agents.sh` (update the 0047 sentinel, lines 539–540; add a global-config README sentinel)

**Interfaces:**
- Consumes: the shipped behavior of Tasks 1–4 (documentation must be written against the code, per LEARNINGS #47 — cite `scripts/docket-config.sh` Stage 2b/2c and `sync-agents.sh` `migrate_legacy_global`/`compute_user_targets` when wording is in doubt).

- [ ] **Step 1: Update the 0047 sentinel + add failing README sentinels**

In `tests/test_sync_agents.sh` lines 539–540, replace:

```bash
assert "0047 §agent-cfg: names the global layer ~/.config/docket/agents.yaml" \
  'grep -qF "~/.config/docket/agents.yaml" <<<"$sec"'
```

with:

```bash
assert "0047 §agent-cfg: names the global layer ~/.config/docket/config.yml" \
  'grep -qF "~/.config/docket/config.yml" <<<"$sec"'
```

Append README + convention sentinels at the end of the file (before `exit $fail`):

```bash
# ---- Change 0050 — README "Global config" section + convention three-layer story ----
# Extract the new dedicated README section (heading -> next `## `), assert within it.
gsec="$(awk '/^##[[:space:]].*[Gg]lobal config/{f=1;print;next} f&&/^##[[:space:]]/{f=0} f{print}' "$READMEF")"
assert "0050 doc: README has a Global config section" '[ -n "$gsec" ]'
assert "0050 doc: §global names the canonical path" 'grep -qF "~/.config/docket/config.yml" <<<"$gsec"'
assert "0050 doc: §global states the same-schema rule" 'grep -qiE "same schema as .?\.docket\.yml" <<<"$gsec"'
assert "0050 doc: §global states per-key precedence" 'grep -qiE "per-repo.*>.*global.*>.*built-in" <<<"$gsec"'
assert "0050 doc: §global states coordination keys are per-repo-only" 'grep -qi "per-repo-only" <<<"$gsec"'
assert "0050 doc: §global names the agents.yaml migration" 'grep -qF "agents.yaml.migrated" <<<"$gsec"'
assert "0050 doc: §global scopes agent_harnesses to the user-level pass" 'grep -qiE "user-level pass" <<<"$gsec"'
# Tuning section gains the both-passes clarification (LEARNINGS #49 — surface end-to-end).
assert "0050 doc: tuning section states sync-agents writes BOTH layers" 'grep -qiE "both" <<<"$sec" && grep -qiE "project (level )?win|project-over-user|project wins" <<<"$sec"'
# Convention: Configuration documents the three-layer story + the fence.
CONV="$REPO/skills/docket-convention/SKILL.md"
assert "0050 doc: convention names config.yml" 'grep -qF "config.yml" "$CONV"'
assert "0050 doc: convention states the coordination-key fence" 'grep -qi "fence" "$CONV" && grep -qi "per-repo-only" "$CONV"'
assert "0050 doc: convention Agent layer global row points at config.yml agents: block" 'grep -Pzoq "Global[\s\S]{0,200}config\.yml" "$CONV"'
```

Note: `$sec` (the 0047 agent-section extraction at line 536) is re-extracted **after** the README edit lands, so re-evaluate it before the new tuning assert — move the `sec="$(awk …)"` extraction line below the 0050 comment or re-run it:

```bash
sec="$(awk '/^##[[:space:]].*[Aa]gent.*([Mm]odel|[Ee]ffort)/{f=1;print;next} f&&/^##[[:space:]]/{f=0} f{print}' "$READMEF")"
```

(re-declare it immediately before the tuning assert so the sentinel reads the current file, not a stale capture).

- [ ] **Step 2: Run to verify the new sentinels fail**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep -E "0047 §agent-cfg: names the global|0050 doc"`
Expected: `NOT OK` on the renamed 0047 sentinel and on every `0050 doc` assert (the docs don't exist yet).

- [ ] **Step 3: Write the docs**

**README.md line 62** — replace `~/.config/docket/agents.yaml` with `~/.config/docket/config.yml`.

**README.md line 65** — in the `sync-agents.sh` bullet, replace ``built-in defaults ⊕ `~/.config/docket/agents.yaml` ⊕ a repo's `.docket.yml agents:` block`` with ``built-in defaults ⊕ the `agents:` block in `~/.config/docket/config.yml` ⊕ a repo's `.docket.yml` `agents:` block``.

**README.md — new section** after the `.docket.yml is committed…` paragraph (line 97), before the `---`:

```markdown
## Global config (`~/.config/docket/config.yml`)

Cross-repo defaults live in one optional user-level file: `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml`. It accepts the **same schema as `.docket.yml`**, and every key resolves **per key**: a repo's committed `.docket.yml` > global `config.yml` > built-in default. Map-valued keys (`skills:`, `agents:`) merge field-by-field with the same precedence.

```yaml
# ~/.config/docket/config.yml — optional; applies to every repo on this machine.
# Same schema as .docket.yml; a repo's committed .docket.yml wins per key.
skills:                      # rebind workflow roles for all your repos
  build: auto
agents:                      # agent model/effort defaults (same agents: shape as .docket.yml)
  default:
    implement-next: { model: claude-opus-4-8, effort: xhigh }
auto_groom: false
finalize:
  gate: local
board_surfaces: [inline]     # the github token is per-repo-only and ignored here (see below)
agent_harnesses: [claude]    # scopes sync-agents.sh's user-level pass ONLY (overrides
                             # presence-on-disk detection); never the per-repo committed pass
```

**Coordination keys are per-repo-only.** Keys whose effect writes shared state — `metadata_branch`, `integration_branch`, `changes_dir`, `adrs_dir`, `results_dir`, `github_project`, and the `github` token of `board_surfaces` — are ignored with a loud warning when set globally: a global value for these would silently split the backlog across machines or mint external GitHub objects. Set them in the repo's committed `.docket.yml`.

**Misplacement fails loud.** A `~/.config/docket/.docket.yml` is never read — `docket-config.sh` warns and points at `config.yml`. A malformed/unreadable `config.yml` warns and falls back to built-ins for the global layer (per-repo config is still honored — a broken personal file never bricks a repo).

**Migrating from `agents.yaml`.** The old single-purpose global file (`~/.config/docket/agents.yaml`) is migrated automatically: the next `sync-agents.sh` (or `install.sh`) run rewrites it under `agents:` in `config.yml` and renames the original to `agents.yaml.migrated`. Nothing reads the old file after migration.
```

**README.md Tuning section** — replace line 229's global bullet with:

```markdown
- **Global** — the `agents:` block in `~/.config/docket/config.yml` (user-level; applies to every repo on your machine; the legacy `agents.yaml` is auto-migrated into it — see **Global config** above).
```

and append after the two refresh bullets (line 243):

```markdown
Note `sync-agents.sh` always writes **both** layers in one run — user-level wrappers into each targeted harness root AND (for opted-in repos) committed project-level wrappers — with the project level winning at runtime. Seeing files appear in your repo after a "global" edit is that second pass, not a misfire: the committed copies are what make an autonomous change build on the same model for every clone.
```

**install.sh line 11** — replace `~/.config/docket/agents.yaml` with `~/.config/docket/config.yml` in the comment.

**skills/docket-convention/SKILL.md** — four edits:

1. In the Configuration section, after the paragraph beginning "`.docket.yml` lives on the repo's **default branch**…", insert:

```markdown
**Global config layer (change 0050).** One optional user-level file — `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml` — accepts the full `.docket.yml` schema, resolved **per-key** as **per-repo > global > built-in** (map-valued `skills:`/`agents:` merge field-by-field). `docket-config.sh --export` implements the layer as the single runtime reader; skills' Step-0 interface is unchanged. **Coordination-key fence:** a key whose effect writes shared, non-re-derivable state (`metadata_branch`, `integration_branch`, `changes_dir`/`adrs_dir`/`results_dir`, `github_project`, and `board_surfaces`' `github` token) is per-repo-only — set globally it is loudly warned-and-ignored, never honored, never fatal. Global-able: `skills:`, `agents:`, `auto_groom`, `finalize.*`, `board_surfaces` minus `github`, and `agent_harnesses` scoped to `sync-agents.sh`'s user-level pass only. A misplaced `~/.config/docket/.docket.yml` warns ("the global file is config.yml") and is never read; a malformed global file warns and falls back to built-ins without bricking any repo. The legacy `~/.config/docket/agents.yaml` is auto-migrated by `sync-agents.sh` into `config.yml`'s `agents:` block (original renamed `.migrated`; no dual-read remains).
```

2. Agent layer table, Global row: replace `` `~/.config/docket/agents.yaml` (optional, XDG) `` with `` the `agents:` block in `~/.config/docket/config.yml` (optional, XDG; legacy `agents.yaml` auto-migrated) ``.

3. Agent layer YAML example comment (the line "The global ~/.config/docket/agents.yaml uses the SAME harness-first map, but at the FILE's top level (no `agents:` wrapper — the file IS the map)…"): replace with

```
  # The global ~/.config/docket/config.yml uses the SAME agents: wrapper shape (change 0050
  # unified it; the pre-0050 top-level-map agents.yaml is auto-migrated on the next sync).
```

Also update the preceding prose paragraph "Both the global file and the per-repo `agents:` block are **harness-first**…" — replace "the global file" context so it names `config.yml`'s `agents:` block, and update the sentence "The global/user-level pass consults no such list — it writes every harness `agents/` directory **present on disk**" to:

```
The user-level pass writes every harness `agents/` directory **present on disk** — unless the
global `config.yml` sets `agent_harnesses:`, which then governs the user-level target list
(creating listed dirs, skipping unlisted ones; change 0050). The per-repo committed pass is
governed solely by the repo's own `agent_harnesses`, never the global value.
```

4. The 0045 paragraph's closing sentence "The **user-level** pass's fan-out scope is unchanged (it still writes every present harness — change 0046 reshaped only how each file's values resolve, per the harness-first Agent layer above)." — append: "(change 0050 later made a global `agent_harnesses` override this presence detection for the user-level pass only)".

**scripts/docket-config.md** — add after the Stage 2 section:

```markdown
### Stage 2b: global config layer (change 0050)

`${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml` — read from the **local filesystem**
(per-machine by definition; no authoritative-ref concern). Full `.docket.yml` schema,
resolved per-key: per-repo > global > built-in. Map-valued `skills:` merges field-by-field.
`agents:` and `agent_harnesses` are **not read here** — `sync-agents.sh` is their reader.

**Guards (Stage 2c), all warn-and-ignore, never fatal:**
- `~/.config/docket/.docket.yml` present → warned ("global config is config.yml"), never read.
- `config.yml` exists but is not a readable regular file → warned; global layer ignored.
- **Coordination-key fence:** `metadata_branch`, `integration_branch`, `changes_dir`,
  `adrs_dir`, `results_dir`, `github_project` set globally → each warned "per-repo-only"
  and ignored. (Block-style `github_project:` with an empty value line is not detected —
  the fence reads the scalar value; nothing reads a global `github_project` regardless.)
- `board_surfaces` **from the global layer** drops a `github` token with a warning
  (external objects stay repo opt-in); a per-repo `github` is honored as before. The
  global fallback happens on the RAW value, so a global `[]` (disable) is distinguishable
  from unset (default `[inline]`).
```

Update the key table (Stage 2) with a "Global-able" column or a note line: `gate`, `test_command`, `auto_groom`, `board_surfaces` (minus `github`), and the `skills:` leaves fall back to the global file when unset per-repo; all other keys are per-repo-only. Update the `skills:` paragraph to say each leaf resolves per-repo > global > superpowers default. Add to Invariants: "**The global layer never aborts a run.** Every global-file problem (misplaced, malformed, fenced key) is a stderr warning; exit codes are unaffected." Keep "18 `KEY=value` lines" verbatim.

- [ ] **Step 4: Verify all sentinels pass + no stale references**

Run: `bash tests/test_sync_agents.sh; echo "exit=$?"` and `bash tests/test_docket_config.sh`
Expected: both exit 0, no `NOT OK`.

Run: `grep -rn "agents\.yaml" README.md install.sh sync-agents.sh skills/ scripts/ tests/`
Expected: only migration-related mentions (the migration function/log in `sync-agents.sh`, its fixtures in `tests/test_sync_agents.sh`, the auto-migration notes in README/convention/contract). No reference presents `agents.yaml` as the live config surface. (`docs/` archive/plans/specs/ADRs are historical records — leave them.)

- [ ] **Step 5: Commit**

```bash
git add README.md install.sh sync-agents.sh skills/docket-convention/SKILL.md scripts/docket-config.md tests/test_sync_agents.sh
git commit -m "docs(0050): global config layer — README section, convention three-layer story, contract, sentinels"
```

---

### Task 6: Full suite + real-data smoke

**Files:**
- No new files; runs the whole `tests/` suite and a live smoke.

- [ ] **Step 1: Run every test file**

```bash
for t in tests/test_*.sh; do
  echo "=== $t"; bash "$t" >/tmp/docket-t.out 2>/tmp/docket-t.err; rc=$?
  grep -c "NOT OK" /tmp/docket-t.out | xargs -I{} echo "  not-ok={} rc=$rc"
  [ $rc -ne 0 ] && { echo "  FAILED:"; grep "NOT OK" /tmp/docket-t.out; }
done
```

Expected: every file rc=0, zero `NOT OK`. Pay attention to `test_install.sh`, `test_consuming_repo_scripts.sh`, `test_script_contracts_coverage.sh` (they exercise `install.sh`/script contracts and may assert on surfaces Task 5 touched).

- [ ] **Step 2: Real-data smoke (LEARNINGS #35 — run inside a real worktree, not a /tmp fixture)**

Run `docket-config.sh --export` against this real repo with a scratch global file, and confirm the fence + a global-able key behave:

```bash
SMOKE_XDG="$(mktemp -d)"
mkdir -p "$SMOKE_XDG/docket"
printf 'metadata_branch: main\nskills:\n  build: auto\n' > "$SMOKE_XDG/docket/config.yml"
XDG_CONFIG_HOME="$SMOKE_XDG" bash scripts/docket-config.sh --export 2>/tmp/smoke.err | grep -E "SKILL_BUILD|METADATA_BRANCH"
cat /tmp/smoke.err
rm -rf "$SMOKE_XDG"
```

Expected: `METADATA_BRANCH=docket` (the fence held — the repo's real docket-mode is untouched by the global `main`), `SKILL_BUILD=auto` (the global-able key applied), and stderr shows exactly one per-repo-only warning naming `metadata_branch`. Also smoke `sync-agents.sh` migration against a scratch root:

```bash
SMOKE_ROOT="$(mktemp -d)"; mkdir -p "$SMOKE_ROOT/.claude" "$SMOKE_ROOT/.config/docket"
printf 'default:\n  status: { model: haiku }\n' > "$SMOKE_ROOT/.config/docket/agents.yaml"
( cd "$SMOKE_ROOT" && DOCKET_HARNESS_ROOT="$SMOKE_ROOT" bash "$OLDPWD/sync-agents.sh" 2>&1 | grep -i migrat )
cat "$SMOKE_ROOT/.config/docket/config.yml"
rm -rf "$SMOKE_ROOT"
```

Expected: the MIGRATED log line; `config.yml` shows `agents:` with the indented `default:` map.

- [ ] **Step 3: Commit (only if fixes were needed)**

If steps 1–2 surfaced fixes, commit them with `fix(0050): <what>`; otherwise nothing to commit.

---

## Self-Review

**1. Spec coverage:**
- §1 file/precedence → Tasks 1 (scalars, skills merge, single reader, XDG) ✓
- §2 fence (six keys + board_surfaces github + agent_harnesses scope split + auto_groom/finalize global-able) → Tasks 2, 4 ✓
- §3 agents.yaml migration (happy path, stale warn, no dual-read) → Task 3 ✓
- §4 fail-loud guards (misplacement, unknown keys posture, malformed fallback) → Task 2 ✓ (unknown top-level keys: silently ignored exactly as `.docket.yml` — no new code, matching "same warn-and-ignore as `.docket.yml`")
- §5 docs (README global section, tuning update + both-passes note, convention Configuration + Agent layer, contracts) → Task 5 ✓
- §6 testing (all listed fixtures) → Tasks 1–4 fixtures map one-to-one; `XDG_CONFIG_HOME` honored via (Q) ✓
- Fence-classification ADR → recorded by the controller's step-6 `docket-adr` dispatch, noted in Global Constraints ✓

**2. Placeholder scan:** none — every step carries exact code/commands.

**3. Type consistency:** `GCFG`/`GCFG_DIR`/`gbl` defined Task 1, consumed Task 2; `GLOBAL_CFG_DIR`/`LEGACY_GLOBAL_CFG`/`migrate_legacy_global` defined Task 3, sequenced with Task 4's `resolve_global_agent_harnesses`/`compute_user_targets`/`USER_TARGETS`; `bs_from_global` written in Task 1 and consumed in Task 2 (Task 1 notes it explicitly). Task-1-alone state is buildable: `bs_from_global` is assigned but unconsumed until Task 2 — no unbound reference (LEARNINGS #45 seam check).
