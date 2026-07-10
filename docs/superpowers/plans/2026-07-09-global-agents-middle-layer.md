# Machine-local config layer + all-local agent generation — Implementation Plan (change 0051)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop committing generated agent artifacts entirely; add the machine-and-repo-scoped `.docket.local.yml` config layer (precedence `repo-local > repo-committed > global > built-in`), a managed `.gitignore` block, a one-time migration for 0048-era repos, and a redefined `--check`.

**Architecture:** Two shell readers grow one precedence rung each: `scripts/docket-config.sh` (runtime resolver — global-able scalar/map keys) and `sync-agents.sh` (generation-time resolver — `agents:` + `agent_harnesses:`). `sync-agents.sh`'s per-repo pass keeps its exact output shape (full built-in set per listed harness + Cursor dispatch rule) but the files become gitignored machine-local artifacts; a marker-bounded `.gitignore` block it owns makes that safe, a migration untracks 0048-era committed copies, and `--check` becomes a 3-leg gate (block current + nothing tracked = CI-meaningful; local staleness = advisory).

**Tech Stack:** bash (must run on macOS `/bin/bash` 3.2 AND modern bash), sed/awk/grep only (no yq), git. Tests are the repo's `assert`-style hermetic fixture suites (`tests/test_docket_config.sh`, `tests/test_sync_agents.sh`).

**Spec:** `docs/superpowers/specs/2026-07-09-global-agents-middle-layer-design.md` (on the `docket` branch — NOT in this worktree). The change file's Reconcile log (2026-07-09) folds in one rider: the bash-3.2 `prune_orphans` empty-array guard.

## Global Constraints

- **bash 3.2 compatible:** no `${var,,}`, no associative arrays, no `mapfile`; guard every `"${arr[@]}"` expansion that can be empty under `set -u` (the rider bug).
- **SIGPIPE-safe:** never `producer | grep -q` / `| head` under `pipefail` — capture into a var first (`head -n1 <<<"$var"`), the suite's existing idiom.
- **grep for a `--flag`:** always `grep -E -e "<pat>"` (POSIX `-e` prevents option-parse), never a bare ERE leading with `--`.
- **Hermetic tests:** every fixture that can reach `${XDG_CONFIG_HOME:-$HOME/.config}` or `$HOME` must pin `XDG_CONFIG_HOME`/`DOCKET_HARNESS_ROOT` to a sandbox (LEARNINGS #50 — a write path to a shared user location upgrades read-leaks to data-loss hazards). `tests/test_sync_agents.sh` already does `unset XDG_CONFIG_HOME` at the top; keep every new fixture inside `mktemp -d` sandboxes.
- **Frontmatter edits anchored:** any sed touching frontmatter-like fields stays anchored (existing discipline; no new frontmatter writers in this change).
- **No model/effort literals in prose/docs** for config-overridable values (LEARNINGS #17); built-in defaults live only in `agents/docket-*.md`.
- **Warn-never-abort posture** for config typos: unknown tokens/keys are warned-and-ignored; a malformed `.docket.local.yml` warns and is skipped (0050's malformed-global posture).
- **Fence unchanged:** fenced keys are exactly `metadata_branch integration_branch changes_dir adrs_dir results_dir github_project` + the `github` token of `board_surfaces` (ADR-0019); no reclassification.
- The commands below assume the worktree root `/Users/homer/dev/docket/.worktrees/global-agents-middle-layer` as cwd; every git command targets this worktree (`git -C` or cwd) — never `.docket/` or the primary checkout.

## File Structure

| File | Responsibility in this change |
|---|---|
| `scripts/docket-config.sh` | + Stage 2b′ local layer (`LCFG`), local fence warnings, 4-layer resolution for `finalize.*`, `auto_groom`, `board_surfaces`, `skills:` |
| `scripts/docket-config.md` | contract: document the local rung |
| `sync-agents.sh` | 4-layer `agents:` resolution, local opt-in + `agent_harnesses`, managed `.gitignore` block, migration, `--check` redefinition, stopgap-warning removal, `prune_orphans` bash-3.2 guard, header-comment rewrite |
| `tests/test_docket_config.sh` | + local-layer fixtures (precedence, fence, malformed) |
| `tests/test_sync_agents.sh` | + 4-layer/opt-in/gitignore/migration/`--check` fixtures; rewrite the stopgap + old drift-semantics tests; + git-repo fixture helper |
| `README.md` | four-layer story, `.docket.local.yml` section, machine-local agents section rewrite |
| `.docket.yml` (this repo's sample comments) | update the `agents:`/`agent_harnesses:` comment prose (no behavior keys change) |
| `skills/docket-convention/SKILL.md` | Configuration + Agent layer + 0045/0048/0050 paragraphs rewritten to the four-layer, all-local story |

Known spec discrepancy (record in results, do not "fix"): spec §5 names a `sync-agents.md` script contract — no such file exists (root-level tools have no `scripts/*.md` contract; `docket-config.md` covers only `scripts/docket-config.sh`). The authoritative sync-agents doc is its header comment — rewrite that instead.

---

### Task 1: `docket-config.sh` — the `.docket.local.yml` rung

**Files:**
- Modify: `scripts/docket-config.sh` (Stage 2b/2c area, lines ~97–196)
- Modify: `scripts/docket-config.md` (layers + warnings sections)
- Test: `tests/test_docket_config.sh` (append a new section at the end, before the exit line)

**Interfaces:**
- Consumes: existing `yaml_get`, `yaml_block_body`, `gbl`, `$CFG`, `$GCFG`.
- Produces: shell var `LCFG` (path or `/dev/null`), helper `lcl()` — both internal; the emitted KEY=value export set is **unchanged** (skills' Step-0 interface stays identical).

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_docket_config.sh` immediately before the final `exit` / summary lines (find them with `grep -n 'exit' tests/test_docket_config.sh | tail`):

```bash
# ============================================================================
# Change 0051 — machine-local layer: <repo>/.docket.local.yml
# Precedence per field: repo-local > repo-committed > global > built-in.
# ============================================================================

# (L1) local beats committed beats global (skills.build), per-field independence:
# build set in all three layers -> local wins; review set only globally -> global wins.
mkrepo "$tmp/l1"
cat > "$tmp/l1/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
skills:
  build: committed-build
EOF
git -C "$tmp/l1" add .docket.yml; git -C "$tmp/l1" commit --quiet -m cfg
git -C "$tmp/l1" push --quiet origin main
mkdir -p "$tmp/xdg-l1/docket"
printf 'skills:\n  build: global-build\n  review: global-review\n' > "$tmp/xdg-l1/docket/config.yml"
printf 'skills:\n  build: local-build\n' > "$tmp/l1/.docket.local.yml"
out="$(rung "$tmp/xdg-l1" "$tmp/l1" --export)"; eval "$out"
assert "0051 L1: local skills.build beats committed+global"  '[ "$SKILL_BUILD" = local-build ]'
assert "0051 L1: unset-local review falls to global"         '[ "$SKILL_REVIEW" = global-review ]'
assert "0051 L1: unset-everywhere plan falls to built-in"    '[ "$SKILL_PLAN" = superpowers:writing-plans ]'

# (L2) scalars: local auto_groom beats committed; local finalize.gate beats global.
mkrepo "$tmp/l2"
cat > "$tmp/l2/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
auto_groom: false
EOF
git -C "$tmp/l2" add .docket.yml; git -C "$tmp/l2" commit --quiet -m cfg
git -C "$tmp/l2" push --quiet origin main
mkdir -p "$tmp/xdg-l2/docket"
printf 'finalize:\n  gate: ci\n' > "$tmp/xdg-l2/docket/config.yml"
printf 'auto_groom: true\nfinalize:\n  gate: both\n  test_command: make local-test\n' > "$tmp/l2/.docket.local.yml"
out="$(rung "$tmp/xdg-l2" "$tmp/l2" --export)"; eval "$out"
assert "0051 L2: local auto_groom beats committed"       '[ "$AUTO_GROOM" = true ]'
assert "0051 L2: local finalize.gate beats global"       '[ "$FINALIZE_GATE" = both ]'
assert "0051 L2: local finalize.test_command honored"    '[ "$FINALIZE_TEST_COMMAND" = "make local-test" ]'

# (L3) fenced keys in the local file: loudly warned-and-ignored, never honored, never fatal.
mkrepo "$tmp/l3"
printf 'metadata_branch: main\nchanges_dir: sneaky/changes\ngithub_project: {owner: x, number: 1}\n' > "$tmp/l3/.docket.local.yml"
errout="$(rung "$tmp/l3-noxdg" "$tmp/l3" --export 2>&1 >/dev/null)"; rc=$?
out="$(rung "$tmp/l3-noxdg" "$tmp/l3" --export 2>/dev/null)"; eval "$out"
assert "0051 L3: fenced local keys not fatal (rc=0)"     '[ "$rc" = "0" ]'
assert "0051 L3: warns metadata_branch is per-repo-only" 'grep -q "metadata_branch" <<<"$errout" && grep -qi "per-repo-only" <<<"$errout"'
assert "0051 L3: warning names the local file"           'grep -q "docket.local.yml" <<<"$errout"'
assert "0051 L3: fenced local metadata_branch IGNORED (mode stays docket-default)" '[ "$METADATA_BRANCH" = docket ]'
assert "0051 L3: fenced local changes_dir IGNORED"       '[ "$CHANGES_DIR" = docs/changes ]'

# (L4) board_surfaces from the local layer: honored, but its github token is machine-fenced.
mkrepo "$tmp/l4"
cat > "$tmp/l4/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
EOF
git -C "$tmp/l4" add .docket.yml; git -C "$tmp/l4" commit --quiet -m cfg
git -C "$tmp/l4" push --quiet origin main
printf 'board_surfaces: [inline, github]\n' > "$tmp/l4/.docket.local.yml"
errout="$(rung "$tmp/l4-noxdg" "$tmp/l4" --export 2>&1 >/dev/null)"
out="$(rung "$tmp/l4-noxdg" "$tmp/l4" --export 2>/dev/null)"; eval "$out"
assert "0051 L4: local board_surfaces honored minus github" '[ "$BOARD_SURFACES" = inline ]'
assert "0051 L4: warns the github token is per-repo-only"   'grep -qi "github" <<<"$errout" && grep -qi "per-repo-only" <<<"$errout"'
# committed github stays honored (regression pin for the per-repo path):
mkrepo "$tmp/l4b"
cat > "$tmp/l4b/.docket.yml" <<'EOF'
metadata_branch: main
integration_branch: main
board_surfaces: [inline, github]
EOF
git -C "$tmp/l4b" add .docket.yml; git -C "$tmp/l4b" commit --quiet -m cfg
git -C "$tmp/l4b" push --quiet origin main
out="$(run "$tmp/l4b" --export)"; eval "$out"
assert "0051 L4: committed github token still honored" '[ "$BOARD_SURFACES" = "inline github" ]'

# (L5) malformed local file (a directory): warn + skip, repo still works.
mkrepo "$tmp/l5"
mkdir "$tmp/l5/.docket.local.yml"
errout="$(rung "$tmp/l5-noxdg" "$tmp/l5" --export 2>&1 >/dev/null)"; rc=$?
assert "0051 L5: malformed local not fatal (rc=0)"  '[ "$rc" = "0" ]'
assert "0051 L5: warns local layer ignored"          'grep -qi "docket.local.yml" <<<"$errout" && grep -qi "ignored" <<<"$errout"'

# (L6) unknown skills role in the LOCAL block: warned + ignored.
mkrepo "$tmp/l6"
printf 'skills:\n  bogusrole: x\n' > "$tmp/l6/.docket.local.yml"
errout="$(rung "$tmp/l6-noxdg" "$tmp/l6" --export 2>&1 >/dev/null)"; rc=$?
assert "0051 L6: unknown local role not fatal (rc=0)" '[ "$rc" = "0" ]'
assert "0051 L6: warns unknown role"                  'grep -qi "unknown skills role" <<<"$errout" && grep -q "bogusrole" <<<"$errout"'
```

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `bash tests/test_docket_config.sh 2>&1 | grep -E -e '^NOT OK - 0051' | head -20`
Expected: every `0051 L*` assert that exercises local-layer behavior prints `NOT OK` (L1 local values ignored, L3/L4/L5/L6 warnings absent). Pre-existing tests all still `ok`.

- [ ] **Step 3: Implement the local rung in `scripts/docket-config.sh`**

3a. After Stage 2b (after the `gbl(){ … }` line, currently line 105), insert Stage 2b′:

```bash
# --- Stage 2b': machine-local layer (change 0051) ------------------------------
# <repo>/.docket.local.yml — machine-AND-repo-scoped overrides for exactly the
# global-able key set (the file is machine-scoped, so the ADR-0019 fence applies
# verbatim). Read from the WORKING TREE — the origin/HEAD-authoritative read applies
# only to the committed .docket.yml. Precedence per field (the .env pattern):
# repo-local > repo-committed > global > built-in.
LCFG="$REPO_DIR/.docket.local.yml"
if [ -e "$LCFG" ] && { [ ! -f "$LCFG" ] || [ ! -r "$LCFG" ]; }; then
  printf 'docket-config: warning: %s is not a readable regular file — machine-local config layer ignored\n' "$LCFG" >&2
  LCFG=/dev/null
fi
lcl(){ yaml_get "$LCFG" "$1"; }   # local-layer scalar read (empty when absent)
```

3b. Extend the Stage 2c fence loop (currently lines 121–125) to warn on fenced keys in the local file too — replace the loop body:

```bash
for _fkey in metadata_branch integration_branch changes_dir adrs_dir results_dir github_project; do
  if [ -n "$(yaml_get "$GCFG" "$_fkey")" ]; then
    printf "docket-config: warning: global config key %s is per-repo-only — set it in the repo's committed .docket.yml; ignored\n" "$_fkey" >&2
  fi
  if [ -n "$(yaml_get "$LCFG" "$_fkey")" ]; then
    printf "docket-config: warning: .docket.local.yml key %s is per-repo-only — set it in the repo's committed .docket.yml; ignored\n" "$_fkey" >&2
  fi
done
```

3c. Add the local rung (highest precedence) to each global-able scalar (currently lines 142–144). The chain order becomes local → committed → global → built-in:

```bash
FINALIZE_GATE="$(lcl gate)"; FINALIZE_GATE="${FINALIZE_GATE:-$(yaml_get "$CFG" gate)}"; FINALIZE_GATE="${FINALIZE_GATE:-$(gbl gate)}"; FINALIZE_GATE="${FINALIZE_GATE:-local}"
FINALIZE_TEST_COMMAND="$(lcl test_command)"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(yaml_get "$CFG" test_command)}"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(gbl test_command)}"
AUTO_GROOM="$(lcl auto_groom)"; AUTO_GROOM="${AUTO_GROOM:-$(yaml_get "$CFG" auto_groom)}"; AUTO_GROOM="${AUTO_GROOM:-$(gbl auto_groom)}"; AUTO_GROOM="${AUTO_GROOM:-false}"
```

3d. `board_surfaces` (currently lines 146–169): the `github` token is fenced when the value arrives from a MACHINE-scoped layer (local or global). Replace the source-tracking prologue:

```bash
bs_raw="$(lcl board_surfaces)"; bs_machine=0
[ -n "$bs_raw" ] && bs_machine=1                            # local = machine-scoped
if [ -z "$bs_raw" ]; then bs_raw="$(yaml_get "$CFG" board_surfaces)"; fi
if [ -z "$bs_raw" ]; then
  bs_raw="$(gbl board_surfaces)"
  [ -n "$bs_raw" ] && bs_machine=1                          # global = machine-scoped
fi
```

and key the existing github-filter on `bs_machine` instead of `bs_from_global` (same warning text, generalized):

```bash
  if [ "$bs_machine" -eq 1 ] && [ -n "$BOARD_SURFACES" ]; then
    _filtered=""
    for _tok in $BOARD_SURFACES; do
      if [ "$_tok" = github ]; then
        printf 'docket-config: warning: board_surfaces token github is per-repo-only (mints external GitHub objects) — set it in the committed .docket.yml; ignored\n' >&2
      else
        _filtered="$_filtered $_tok"
      fi
    done
    BOARD_SURFACES="$(echo $_filtered)"
  fi
```

3e. `skills:` (currently lines 174–196): add the local block as the first rung and include it in the unknown-role warning sweep:

```bash
SKILLS_BLK="$(mktemp)";  yaml_block_body "$CFG"  skills >"$SKILLS_BLK"
GSKILLS_BLK="$(mktemp)"; yaml_block_body "$GCFG" skills >"$GSKILLS_BLK"
LSKILLS_BLK="$(mktemp)"; yaml_block_body "$LCFG" skills >"$LSKILLS_BLK"
skill_role(){  # skill_role <role> <default> -> resolved value on stdout
  local v; v="$(yaml_get "$LSKILLS_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$SKILLS_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$GSKILLS_BLK" "$1")"
  printf '%s' "${v:-$2}"
}
```

Change the sweep list `for _blk in "$SKILLS_BLK" "$GSKILLS_BLK"` to `for _blk in "$LSKILLS_BLK" "$SKILLS_BLK" "$GSKILLS_BLK"` and the cleanup to `rm -f "$SKILLS_BLK" "$GSKILLS_BLK" "$LSKILLS_BLK"`.

Also update the file-header comment block (lines 2–11) to mention the local layer, and the Stage-2b comment ("resolved PER-KEY: per-repo > global > built-in") to the four-layer string `repo-local > repo-committed > global > built-in`.

- [ ] **Step 4: Run the suite to verify it passes**

Run: `bash tests/test_docket_config.sh 2>&1 | grep -c -E -e '^NOT OK'`
Expected: `0` (all pre-existing + new tests green).

- [ ] **Step 5: Update the contract `scripts/docket-config.md`**

Add the local layer wherever the contract describes layers/warnings: the file list (`.docket.local.yml`, working-tree read, machine-scoped), the four-layer per-field precedence, the two new warnings (fenced local key; malformed local file), and that the emitted KEY set is unchanged. Match the contract's existing section structure (Purpose/Behavior/Invariants) — read it first and edit surgically.

- [ ] **Step 6: Commit**

```bash
git add scripts/docket-config.sh scripts/docket-config.md tests/test_docket_config.sh
git commit -m "feat(0051): .docket.local.yml — machine-local config rung in docket-config.sh"
```

---

### Task 2: `sync-agents.sh` — four-layer agent resolution + local opt-in (+ bash-3.2 rider)

**Files:**
- Modify: `sync-agents.sh` (`LOCAL_CFG` near line 38; `resolve_agent_harnesses` ~94; `per_repo_opted_in` ~171; `resolve_agent` ~225; call sites ~344, ~385, ~413; stopgap block 361–368; `prune_orphans` ~481)
- Test: `tests/test_sync_agents.sh` (replace the 3 stopgap-warning tests at the end; add a 0051 four-layer section)

**Interfaces:**
- Consumes: existing `harness_agent_line(file, harness, agent, under_agents)`, `field_of(line, field)`, `emit(src, model, effort)`, `agent_keys`, `warn_legacy_shape`.
- Produces: `LOCAL_CFG` (path or `/dev/null`); `resolve_agent_layers(harness, agent, file...)` setting `RES_MODEL`/`RES_EFFORT`/`RES_MODEL_FROM_HARNESS` — the old 4-arg `resolve_agent` is DELETED (Tasks 3–4 use `resolve_agent_layers` and `per_repo_opted_in` as defined here).

- [ ] **Step 1: Write the failing tests**

In `tests/test_sync_agents.sh`, DELETE the stopgap block (the section starting `# ---- Change 0050 follow-up (stopgap for #0051) — global agents: shadowing warning ----` through its `rm -rf "$SBX" "$HROOTSW"`) and append in its place:

```bash
# ============================================================================
# Change 0051 — four-layer per-field agents: resolution; all-local generation.
# Precedence: local.agents.H.X -> local.default.X -> committed.H.X -> committed.default.X
#             -> global.H.X -> global.default.X -> built-in. THE 0050 BUG FIX:
# a global agents: block now REACHES per-repo generated files (no committed shadow).
# ============================================================================

# (4L-a) THE FIX — opted-in repo + global agents: + no repo/local override
# => the generated project-level file carries the GLOBAL model (was: built-in + SHADOWED warning).
make_sandbox
HROOT51A="$(mktemp -d)"; mkdir -p "$HROOT51A/.claude" "$HROOT51A/.config/docket"
printf 'agents:\n  default:\n    status: { model: global-model-x }\n' > "$HROOT51A/.config/docket/config.yml"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
sw_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51A" bash "$SYNC" 2>&1 >/dev/null)"
assert "0051 4L: global agents value reaches the per-repo generated file" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "global-model-x" ]'
assert "0051 4L: the 0050 SHADOWED stopgap warning is gone" '! printf "%s" "$sw_err" | grep -q "SHADOWED"'
rm -rf "$SBX" "$HROOT51A"

# (4L-b) full chain: local beats committed beats global; per-FIELD independence
# (model from local, effort from committed) and harness-over-default within a layer.
make_sandbox
HROOT51B="$(mktemp -d)"; mkdir -p "$HROOT51B/.claude" "$HROOT51B/.config/docket"
printf 'agents:\n  default:\n    status: { model: global-m, effort: low }\n' > "$HROOT51B/.config/docket/config.yml"
printf 'agents:\n  default:\n    status: { model: committed-m, effort: high }\n' > "$SBX/.docket.yml"
printf 'agents:\n  default:\n    status: { model: local-m }\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51B" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 4L: local model beats committed+global"        '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "local-m" ]'
assert "0051 4L: effort unset locally falls to committed"   '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "high" ]'
# harness key in a LOWER layer still loses to default in a HIGHER layer for that field:
printf 'agents:\n  claude:\n    status: { model: committed-claude-m }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51B" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 4L: local default beats committed harness key" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "local-m" ]'
rm -rf "$SBX" "$HROOT51B"

# (4L-c) opt-in via the LOCAL file alone — a machine opts a tracking-only repo in
# without touching committed config; local agent_harnesses governs the target list.
make_sandbox
HROOT51C="$(mktemp -d)"; mkdir -p "$HROOT51C/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"           # tracking-only committed file
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: local-m }\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51C" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 opt-in: local file alone opts in (claude generated)"  '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0051 opt-in: local agent_harnesses honored (cursor too)"   '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
assert "0051 opt-in: cursor dispatch rule generated"               '[ -f "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
assert "0051 opt-in: local model applied"                          '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "local-m" ]'
rm -rf "$SBX" "$HROOT51C"

# (4L-d) local agent_harnesses BEATS committed (key-level precedence, not a merge).
make_sandbox
HROOT51D="$(mktemp -d)"; mkdir -p "$HROOT51D/.claude"
printf 'agent_harnesses: [claude, cursor]\n' > "$SBX/.docket.yml"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51D" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 gah: local list wins (claude generated)"     '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0051 gah: committed cursor overridden away"       '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT51D"

# (4L-e) tracking-only repo with NEITHER file opted in: still zero files (regression pin).
make_sandbox
HROOT51E="$(mktemp -d)"; mkdir -p "$HROOT51E/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51E" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 opt-in: neither file => zero project files" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT51E"

# (4L-f) malformed .docket.local.yml (a directory): warn + skip, run still succeeds,
# committed layer still honored.
make_sandbox
HROOT51F="$(mktemp -d)"; mkdir -p "$HROOT51F/.claude"
printf 'agents:\n  default:\n    status: { model: committed-m }\n' > "$SBX/.docket.yml"
mkdir "$SBX/.docket.local.yml"
mf_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51F" bash "$SYNC" 2>&1 >/dev/null)"; mf_rc=$?
assert "0051 malformed local: not fatal (rc=0)"        '[ "$mf_rc" = "0" ]'
assert "0051 malformed local: warns and names the file" 'printf "%s" "$mf_err" | grep -qi "docket.local.yml"'
assert "0051 malformed local: committed layer still applies" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "committed-m" ]'
rm -rf "$SBX" "$HROOT51F"

# (4L-g) tab-indented local YAML resolves (LEARNINGS #46 — indent classes must be [^[:space:]]).
make_sandbox
HROOT51G="$(mktemp -d)"; mkdir -p "$HROOT51G/.claude"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
printf 'agents:\n\tdefault:\n\t\tstatus: { model: tab-m }\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51G" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 4L: tab-indented local YAML resolves" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "tab-m" ]'
rm -rf "$SBX" "$HROOT51G"

# (rider) prune_orphans empty-scan_dirs guard: bash 3.2 + set -u with NO harness roots
# on disk AND no opt-in must not crash ("${scan_dirs[@]}" on an empty array).
SBXR="$(mktemp -d)"                                   # deliberately NO .claude/.agents dirs
rid_rc=0
( cd "$SBXR" && DOCKET_HARNESS_ROOT="$SBXR" /bin/bash "$SYNC" >/dev/null 2>&1 ) || rid_rc=$?
assert "0051 rider: empty scan_dirs run succeeds under /bin/bash (rc=0)" '[ "$rid_rc" = "0" ]'
rm -rf "$SBXR"
```

- [ ] **Step 2: Run to verify the new tests fail**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep -E -e '^NOT OK - 0051' | head`
Expected: 4L-a (built-in model, SHADOWED warning present), 4L-b, 4L-c, 4L-d fail; on macOS the rider test fails (`rc!=0`). (4L-e/4L-f may already pass — fine.)

- [ ] **Step 3: Implement**

3a. After `DOCKET_YML="$REPO/.docket.yml"` (line 38) add:

```bash
LOCAL_CFG="$REPO/.docket.local.yml"
# Malformed/unreadable local file: warn + skip (0050's malformed-global posture) — a broken
# machine-local file must never break the run; committed + global layers still apply.
if [ -e "$LOCAL_CFG" ] && { [ ! -f "$LOCAL_CFG" ] || [ ! -r "$LOCAL_CFG" ]; }; then
  printf '%s\n' "sync-agents: WARN $LOCAL_CFG is not a readable regular file — machine-local layer ignored" >&2
  LOCAL_CFG=/dev/null
fi
```

(Direct `printf` because `log()` is defined later; alternatively move the `log()` definition above this guard — pick one, keep `set -euo pipefail` happy.)

3b. `resolve_agent_harnesses()`: read the key from `LOCAL_CFG` first, then `DOCKET_YML` (key-level precedence — first file that HAS the key wins, even with an empty list value). Replace the `raw=""` + single-file read with:

```bash
  raw=""
  local f
  for f in "$LOCAL_CFG" "$DOCKET_YML"; do
    [ -f "$f" ] || continue
    if grep -qE '^agent_harnesses[[:space:]]*:' "$f"; then
      raw="$(sed -n -E 's/^agent_harnesses[[:space:]]*:[[:space:]]*([^#]*).*/\1/p' "$f")"
      raw="$(head -n1 <<<"$raw" | sed -E 's/[[:space:]]+$//')"
      break
    fi
  done
```

Note the `grep -qE` presence test before reading: `agent_harnesses: []` in the local file must WIN (yield the empty list), not fall through to the committed value. When no file has the key, the existing `[ -z "$raw" ] => HARNESSES="claude"` default applies. (A bare `agent_harnesses:` key with an empty value in the LOCAL file will now also resolve to the default — same as today's committed behavior; acceptable.)

3c. `per_repo_opted_in()`: check both files:

```bash
per_repo_opted_in() {
  local f
  for f in "$LOCAL_CFG" "$DOCKET_YML"; do
    [ -f "$f" ] || continue
    grep -qE '^agent_harnesses[[:space:]]*:' "$f" && return 0
    grep -qE '^agents[[:space:]]*:' "$f" && return 0
  done
  return 1
}
```

3d. Replace `resolve_agent()` (lines 222–233) with the layered resolver (keep the name change; DELETE the old function):

```bash
# Resolve (harness, agent) per-field across the given layer files, highest precedence
# first (each read under a top-level agents: wrapper). Within a layer the harness line
# beats the default line; across layers the first layer supplying a field wins; the
# built-in floor is handled by emit(). RES_MODEL_FROM_HARNESS=1 iff the model came from
# a harness-specific line in ANY layer (drives warn_fallback_model).
resolve_agent_layers() {  # $1=harness  $2=agent  $3..=layer files (precedence order)
  local harness="$1" agent="$2" f hline dline hm he dm de
  shift 2
  RES_MODEL=""; RES_EFFORT=""; RES_MODEL_FROM_HARNESS=0
  for f in "$@"; do
    hline="$(harness_agent_line "$f" "$harness" "$agent" 1)"
    dline="$(harness_agent_line "$f" default "$agent" 1)"
    hm="$(field_of "$hline" model)";  he="$(field_of "$hline" effort)"
    dm="$(field_of "$dline" model)";  de="$(field_of "$dline" effort)"
    if [ -z "$RES_MODEL" ]; then
      if   [ -n "$hm" ]; then RES_MODEL="$hm"; RES_MODEL_FROM_HARNESS=1
      elif [ -n "$dm" ]; then RES_MODEL="$dm"; fi
    fi
    if [ -z "$RES_EFFORT" ]; then
      if   [ -n "$he" ]; then RES_EFFORT="$he"
      elif [ -n "$de" ]; then RES_EFFORT="$de"; fi
    fi
    if [ -n "$RES_MODEL" ] && [ -n "$RES_EFFORT" ]; then break; fi
  done
  return 0
}
```

Call sites: `user_level_pass` (line 344) → `resolve_agent_layers "$harness" "$name" "$GLOBAL_CFG"` (user level stays built-in ⊕ global — spec §2 "user-level pass unchanged"); `project_level_pass` (line 385) and the Task-4 `--check` leg (c) → `resolve_agent_layers "$harness" "$name" "$LOCAL_CFG" "$DOCKET_YML" "$GLOBAL_CFG"`.

3e. Delete the stopgap warning block (the comment + `if [ -n "$(agent_keys "$GLOBAL_CFG" 1)" ]; then log "WARN global agents: config … SHADOWED …" fi`, lines 361–368).

3f. Extend the project-pass diagnostics to the local file (each loop runs for both `"$LOCAL_CFG"` and `"$DOCKET_YML"`): `warn_legacy_shape`, the dead-config `agents_block_harnesses` warning, and the no-such-built-in typo guard. Keep the messages unchanged (they already name the offending key/harness).

3g. Rider: in `prune_orphans()` guard the possibly-empty array expansion (line ~481):

```bash
  if [ ${#scan_dirs[@]} -gt 0 ]; then
    for dir in "${scan_dirs[@]}"; do
      [ -d "$dir" ] || continue
      for f in "$dir"/docket-*.md; do
        [ -e "$f" ] || continue
        name="$(short_name "$f")"
        [ -f "$AGENTS_SRC/docket-$name.md" ] || handle_orphan "$f"
      done
    done
  fi
```

3h. Rewrite the header comment (lines 1–26) to the four-layer story: layers list gains `local <repo>/.docket.local.yml (gitignored, machine-scoped)`; state that per-repo generated files are **machine-local, never committed** (gitignore block, Task 3); `--check`'s new meaning (Task 4 — write the final text there if you prefer, but leave no stale "committed" claims after this task).

- [ ] **Step 4: Run the suite**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep -E -e '^NOT OK'`
Expected: the new 0051 tests pass. KNOWN still-red at this point: none — the old `--check` tests still pass because `check_project_level` is untouched until Task 4 (it calls the deleted `resolve_agent`!). So ALSO update `check_project_level`'s two `resolve_agent "$DOCKET_YML" …` call sites NOW to `resolve_agent_layers "$harness" "$name" "$LOCAL_CFG" "$DOCKET_YML" "$GLOBAL_CFG"` — semantics unchanged when no local/global file exists. Verify: `bash -n sync-agents.sh` then the suite: 0 `NOT OK`.

- [ ] **Step 5: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "feat(0051): four-layer agents: resolution + .docket.local.yml opt-in; drop the 0050 shadowing stopgap"
```

---

### Task 3: `sync-agents.sh` — managed `.gitignore` block

**Files:**
- Modify: `sync-agents.sh` (new helpers after `write_dispatch_rule`; main flow before `project_level_pass`)
- Test: `tests/test_sync_agents.sh` (append a gitignore section)

**Interfaces:**
- Consumes: `per_repo_opted_in`, `VALID_HARNESS_TOKENS`, `HARNESS_HAS_DISPATCH_RULES`, `log`, `$REPO`.
- Produces: `emit_gitignore_block()` (block content to stdout), `current_gitignore_block()`, `gitignore_block_wanted()`, `ensure_gitignore_block()` — Task 4's `--check` leg (a) reuses the first three.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_sync_agents.sh`:

```bash
# ============================================================================
# Change 0051 — managed .gitignore block (# docket:generated:start/end)
# ============================================================================

# (gi-a) opted-in repo: block created (file didn't exist), loud "commit" notice,
# patterns strictly docket-scoped, emitted from the harness table (all 6 tokens).
make_sandbox
HROOTGA="$(mktemp -d)"; mkdir -p "$HROOTGA/.claude"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
gi_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGA" bash "$SYNC" 2>&1 >/dev/null)"
GI="$SBX/.gitignore"
assert "0051 gi: .gitignore created with the managed block" 'grep -q "^# docket:generated:start" "$GI" && grep -q "^# docket:generated:end$" "$GI"'
assert "0051 gi: block ignores .docket.local.yml"            'grep -q "^\.docket\.local\.yml$" "$GI"'
assert "0051 gi: block ignores claude agents pattern"        'grep -q "^\.claude/agents/docket-\*\.md$" "$GI"'
assert "0051 gi: block ignores cursor agents pattern"        'grep -q "^\.cursor/agents/docket-\*\.md$" "$GI"'
assert "0051 gi: block ignores the cursor dispatch rule"     'grep -q "^\.cursor/rules/docket-dispatch\.mdc$" "$GI"'
assert "0051 gi: loud commit-this notice"                    'printf "%s" "$gi_err" | grep -qi "commit"'
assert "0051 gi: every block line is docket-scoped (starts with . or #)" \
  '! awk "/# docket:generated:start/,/# docket:generated:end/" "$GI" | grep -qvE "^(#|\.)"'

# (gi-b) idempotent: second run leaves .gitignore byte-identical and prints no notice.
gi_before="$(cat "$GI")"
gi_err2="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGA" bash "$SYNC" 2>&1 >/dev/null)"
assert "0051 gi: second run byte-identical"    '[ "$gi_before" = "$(cat "$GI")" ]'
assert "0051 gi: second run no UPDATED notice" '! printf "%s" "$gi_err2" | grep -q "managed block"'

# (gi-c) hand-edit inside the block repaired; content OUTSIDE the markers preserved.
printf 'my-own-ignore/\n%s\n' "$(cat "$GI")" > "$GI"          # user content above the block
sed -i.bak '/docket-dispatch/d' "$GI"; rm -f "$GI.bak"        # vandalize the block
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGA" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 gi: hand-edited block repaired"   'grep -q "docket-dispatch" "$GI"'
assert "0051 gi: user content preserved"       'grep -q "^my-own-ignore/$" "$GI"'
assert "0051 gi: exactly one block after repair" '[ "$(grep -c "^# docket:generated:start" "$GI")" = "1" ]'
rm -rf "$SBX" "$HROOTGA"

# (gi-d) tracking-only repo WITH a .docket.local.yml that has NO opt-in keys: the block
# is still written (the local file itself must never be committable); zero agent files.
make_sandbox
HROOTGD="$(mktemp -d)"; mkdir -p "$HROOTGD/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"
printf 'finalize:\n  gate: off\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGD" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 gi: local-file-present repo gets the block"  'grep -q "^# docket:generated:start" "$SBX/.gitignore"'
assert "0051 gi: but still generates zero agent files"    '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOTGD"

# (gi-e) repo with NEITHER signal: .gitignore never touched/created (LEARNINGS #48 posture).
make_sandbox
HROOTGE="$(mktemp -d)"; mkdir -p "$HROOTGE/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGE" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 gi: no-signal repo gets NO .gitignore" '[ ! -e "$SBX/.gitignore" ]'
rm -rf "$SBX" "$HROOTGE"
```

- [ ] **Step 2: Run to verify failure**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep -E -e '^NOT OK - 0051 gi' | head`
Expected: gi-a/b/c/d fail (no block written); gi-e passes already.

- [ ] **Step 3: Implement**

After `write_dispatch_rule()` add:

```bash
# --- managed .gitignore block (change 0051) -----------------------------------
# sync-agents.sh owns a marker-bounded block in <repo>/.gitignore covering every
# machine-local artifact it generates. Patterns are emitted from the SAME harness
# table generation uses (VALID_HARNESS_TOKENS / HARNESS_HAS_DISPATCH_RULES), so a
# new harness extends the block without a second roster. Nothing outside the
# markers is ever touched.
GITIGNORE="$REPO/.gitignore"
GI_START='# docket:generated:start (managed by sync-agents.sh — do not hand-edit)'
GI_END='# docket:generated:end'

emit_gitignore_block() {
  printf '%s\n' "$GI_START"
  printf '.docket.local.yml\n'
  local tok
  for tok in $VALID_HARNESS_TOKENS; do printf '.%s/agents/docket-*.md\n' "$tok"; done
  for tok in $HARNESS_HAS_DISPATCH_RULES; do printf '.%s/rules/docket-dispatch.mdc\n' "$tok"; done
  printf '%s\n' "$GI_END"
}

# The block is maintained for opted-in repos AND any repo carrying a .docket.local.yml
# (a tracking-only repo using it for skills:/finalize: must never risk committing it).
# NOTE: test the RAW path — LOCAL_CFG may have been redirected to /dev/null.
gitignore_block_wanted(){ per_repo_opted_in && return 0; [ -e "$REPO/.docket.local.yml" ]; }

current_gitignore_block() {
  [ -f "$GITIGNORE" ] || return 0
  awk -v s="$GI_START" -v e="$GI_END" '$0==s{f=1} f{print} $0==e{f=0}' "$GITIGNORE"
}

ensure_gitignore_block() {  # create/refresh; bytes outside the markers are never touched
  gitignore_block_wanted || return 0
  local want have rest
  want="$(emit_gitignore_block)"
  have="$(current_gitignore_block)"
  [ "$want" = "$have" ] && return 0
  rest=""
  if [ -f "$GITIGNORE" ]; then
    rest="$(awk -v s="$GI_START" -v e="$GI_END" '$0==s{f=1} !f{print} $0==e{f=0}' "$GITIGNORE")"
  fi
  {
    if [ -n "$rest" ]; then printf '%s\n\n' "$rest"; fi
    printf '%s\n' "$want"
  } > "$GITIGNORE"
  log "UPDATED $GITIGNORE managed block (docket:generated) — COMMIT THIS so machine-local generated files stay untracked"
}
```

Main flow: insert `ensure_gitignore_block` between `user_level_pass` and `project_level_pass` (so generated files are ignored the moment they land).

- [ ] **Step 4: Run the suite**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep -c -E -e '^NOT OK'`
Expected: `0`.

- [ ] **Step 5: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "feat(0051): managed # docket:generated .gitignore block owned by sync-agents.sh"
```

---

### Task 4: `sync-agents.sh` — 0048-era migration + `--check` redefined

**Files:**
- Modify: `sync-agents.sh` (`check_project_level` rewrite; new `tracked_docket_files` + `migrate_tracked_wrappers`; main flow)
- Test: `tests/test_sync_agents.sh` (new git-repo fixture helper + migration/`--check` sections; REWRITE the old drift-semantics tests)

**Interfaces:**
- Consumes: Task 2's `resolve_agent_layers` + `per_repo_opted_in`; Task 3's `emit_gitignore_block`/`current_gitignore_block`/`gitignore_block_wanted`.
- Produces: `tracked_docket_files()` (tracked generated paths, one per line; empty outside a git repo), `migrate_tracked_wrappers()`; `--check` exit contract: rc≠0 iff leg (a) block missing/stale OR leg (b) tracked generated files OR the legacy committed-`agents:`-shape flag; leg (c) content staleness is ADVISORY (`advisory:` prefix, rc untouched).

- [ ] **Step 1: Convert the old drift-semantics tests**

The old `--check` CI gate was "committed files diff against resolved config"; content drift is now leg (c) advisory. In `tests/test_sync_agents.sh` update IN PLACE (keep each fixture, flip the assertions):

1. Lines ~300–333 (Task 1b/1c wrapper drift: `--check flags critic drift`, `--check flags rebase-resolver drift`) → after `--check`: assert `rc=0` AND output contains `advisory` naming the file. Rename the assert labels to `… advisory-flags …`.
2. Lines ~335–354 (`--check fails on drift`, `missing file`) → same conversion: `[ "$chk_rc" = "0" ]` + `grep -q "advisory"`.
3. Lines ~175–192 (0048 rule-check: tampered + deleted rule) → `rc=0` + `advisory` naming `docket-dispatch.mdc`. NOTE: these fixtures run a normal sync first, which now writes a `.gitignore` block in the sandbox — leg (a) is green there; keep them non-git sandboxes so leg (b) is vacuous.
4. Lines ~483–499 (0045 cursor-drift + missing cursor file) → `rc=0` + `advisory` + names cursor.
5. Lines ~583–594 (0046 legacy bare-agent-key): UNCHANGED — the legacy shape lives in the committed `.docket.yml`, still CI-meaningful, still `rc!=0`. BUT its fixture never ran a normal sync (no block) — the repo IS opted-in (`agents:` present), so leg (a) now ALSO fires. That's fine for `rc!=0`, but keep the assertion on the legacy message intact and add `( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTL" bash "$SYNC" >/dev/null 2>&1 )` before the `--check` so the block exists and the test isolates the legacy leg.
6. Lines ~348–354 ("Committed file entirely absent … -> drift"): the fixture never generated → block absent → still `rc!=0` but now via leg (a); update the label/report grep to the block message OR pre-run a sync then `rm` the generated file and assert advisory. Prefer: pre-run sync, `rm` the file, assert `rc=0` + advisory (missing local file), and add a separate leg-(a) test below.
7. Tracking-only no-op (`~356–365`) and no-`.docket.yml` (`~379–383`): unchanged, still `rc=0`.

- [ ] **Step 2: Write the new failing tests**

Append:

```bash
# ============================================================================
# Change 0051 — migration (0048-era tracked wrappers) + --check three legs
# ============================================================================

# git-repo fixture: sandbox repo with identity + one commit (for ls-files-based legs).
mkgitrepo(){
  SBX="$(mktemp -d)"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
}

# (mig-a) 0048-era repo: tracked wrappers + rule -> deleted from the worktree, block
# written, local set regenerated, single migration commit printed. Idempotent.
mkgitrepo
HROOTM="$(mktemp -d)"; mkdir -p "$HROOTM/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
mkdir -p "$SBX/.claude/agents" "$SBX/.cursor/agents" "$SBX/.cursor/rules"
printf 'stale 0048 wrapper\n' > "$SBX/.claude/agents/docket-status.md"
printf 'stale 0048 wrapper\n' > "$SBX/.cursor/agents/docket-status.md"
printf 'stale 0048 rule\n'    > "$SBX/.cursor/rules/docket-dispatch.mdc"
git -C "$SBX" add -A; git -C "$SBX" commit --quiet -m "0048-era state"
mig_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTM" bash "$SYNC" 2>&1 >/dev/null)"; mig_rc=$?
assert "0051 mig: run succeeds (rc=0)"                     '[ "$mig_rc" = "0" ]'
assert "0051 mig: announces the migration"                 'printf "%s" "$mig_err" | grep -qi "migrat"'
assert "0051 mig: prints git rm --cached instructions"     'printf "%s" "$mig_err" | grep -q -e "git rm" '
assert "0051 mig: gitignore block written"                 'grep -q "^# docket:generated:start" "$SBX/.gitignore"'
assert "0051 mig: local files regenerated (fresh content)" 'grep -q "^model: sonnet" "$SBX/.claude/agents/docket-status.md"'
assert "0051 mig: full local set regenerated"              '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "8" ]'
# perform the printed migration commit; second run must NOT re-announce
( cd "$SBX" && git rm -r -q --cached '.claude/agents/docket-*.md' '.cursor/agents/docket-*.md' '.cursor/rules/docket-dispatch.mdc' && git add .gitignore && git commit -q -m "docket: agent files go machine-local" )
mig_err2="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTM" bash "$SYNC" 2>&1 >/dev/null)"
assert "0051 mig: idempotent — post-commit run is silent about migration" '! printf "%s" "$mig_err2" | grep -qi "migrat"'
# and --check is fully green now (all three legs)
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTM" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 mig: post-migration --check green (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX" "$HROOTM"

# (chk-a) leg (a): opted-in repo, block missing (sync never ran) -> rc!=0 naming the block.
make_sandbox
HROOTCA="$(mktemp -d)"; mkdir -p "$HROOTCA/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCA" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 chk-a: missing block fails --check (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "0051 chk-a: names the gitignore block"           'printf "%s" "$chk_out" | grep -qi "gitignore"'
# stale block (hand-pruned pattern) also fails:
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCA" bash "$SYNC" >/dev/null 2>&1 )
sed -i.bak '/docket-dispatch/d' "$SBX/.gitignore"; rm -f "$SBX/.gitignore.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCA" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 chk-a: stale block fails --check (rc!=0)"   '[ "$chk_rc" != "0" ]'
rm -rf "$SBX" "$HROOTCA"

# (chk-b) leg (b): tracked generated file -> rc!=0 with the migration remedy.
mkgitrepo
HROOTCB="$(mktemp -d)"; mkdir -p "$HROOTCB/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCB" bash "$SYNC" >/dev/null 2>&1 )   # block + local files
git -C "$SBX" add -A -f; git -C "$SBX" commit --quiet -m "wrongly track everything"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCB" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 chk-b: tracked generated file fails --check (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "0051 chk-b: names a tracked path"                          'printf "%s" "$chk_out" | grep -q "docket-status.md"'
rm -rf "$SBX" "$HROOTCB"

# (chk-c) leg (c): content staleness is ADVISORY — rc stays 0, output says advisory.
make_sandbox
HROOTCC="$(mktemp -d)"; mkdir -p "$HROOTCC/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCC" bash "$SYNC" >/dev/null 2>&1 )
sed -i.bak 's/^model: sonnet/model: haiku/' "$SBX/.claude/agents/docket-status.md"; rm -f "$SBX/.claude/agents/docket-status.md.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCC" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 chk-c: content drift is advisory (rc=0)"  '[ "$chk_rc" = "0" ]'
assert "0051 chk-c: advisory names the drifted file"   'printf "%s" "$chk_out" | grep -q "advisory" && printf "%s" "$chk_out" | grep -q "docket-status.md"'
rm -rf "$SBX" "$HROOTCC"

# (chk-d) fresh clone of a MIGRATED repo: committed .docket.yml (opted-in) + committed
# block, NO generated files -> --check fully green (leg c vacuous on CI).
mkgitrepo
HROOTCD="$(mktemp -d)"; mkdir -p "$HROOTCD/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCD" bash "$SYNC" >/dev/null 2>&1 )     # writes block + files
find "$SBX" -name 'docket-*.md' -path '*/agents/*' -delete                       # simulate the fresh clone
git -C "$SBX" add .docket.yml .gitignore; git -C "$SBX" commit --quiet -m "migrated repo"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCD" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 chk-d: fresh migrated clone --check green (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX" "$HROOTCD"
```

- [ ] **Step 3: Run to verify the new tests fail**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep -E -e '^NOT OK - 0051 (mig|chk)' | head -20`
Expected: all mig/chk tests fail (no migration, old `--check` semantics). The Step-1 conversions also fail until implemented.

- [ ] **Step 4: Implement**

4a. New helpers (after the Task-3 gitignore helpers):

```bash
# --- 0048-era migration: generated files must not be tracked (change 0051) ----
tracked_docket_files() {  # tracked generated agent/rule paths, one per line (empty outside git)
  git -C "$REPO" rev-parse --is-inside-work-tree >/dev/null 2>&1 || return 0
  local tok
  {
    for tok in $VALID_HARNESS_TOKENS; do
      git -C "$REPO" ls-files -- ".$tok/agents/docket-*.md" 2>/dev/null
    done
    for tok in $HARNESS_HAS_DISPATCH_RULES; do
      git -C "$REPO" ls-files -- ".$tok/rules/docket-dispatch.mdc" 2>/dev/null
    done
  } | sort -u
}

migrate_tracked_wrappers() {  # one-time: untrack 0048-era committed wrappers; idempotent
  local tracked f
  tracked="$(tracked_docket_files)"
  [ -n "$tracked" ] || return 0
  log "MIGRATING (change 0051): generated agent files are machine-local now and must not be tracked"
  while IFS= read -r f; do rm -f "$REPO/$f"; done <<<"$tracked"
  log "deleted the tracked copies from the working tree (regenerated locally below); complete with ONE commit:"
  log "  git rm -r --cached $(tr '\n' ' ' <<<"$tracked")&& git add .gitignore && git commit -m 'docket: generated agent files go machine-local (change 0051)'"
}
```

4b. Rewrite `check_project_level()` (three legs; the tmp-diff loops survive as leg (c) with `advisory:` prefix and no `rc` mutation; dispatch-rule diff likewise; `prune_orphans per-repo` orphan report becomes advisory too — orphaned files are untracked local artifacts now):

```bash
check_project_level() {
  local rc=0 tracked legacy
  # leg (b) — migration enforcement runs even without opt-in (stale 0048 leftovers).
  tracked="$(tracked_docket_files)"
  if [ -n "$tracked" ]; then
    log "check: TRACKED generated agent files (machine-local since change 0051) — run: bash sync-agents.sh, then make the printed migration commit:"
    printf '%s\n' "$tracked" >&2
    rc=1
  fi
  if ! gitignore_block_wanted; then
    log "no per-repo agent opt-in (agents:/agent_harnesses) and no .docket.local.yml in $REPO — nothing else to check"
    return $rc
  fi
  # leg (a) — the committed .gitignore block is present and current (CI-meaningful).
  if [ "$(emit_gitignore_block)" != "$(current_gitignore_block)" ]; then
    log "check: .gitignore docket:generated block missing or stale — run: bash sync-agents.sh and commit .gitignore"
    rc=1
  fi
  # committed-config shape (the committed .docket.yml is CI-visible): legacy bare agent keys.
  legacy="$(legacy_agent_keys "$DOCKET_YML" 1)"
  if [ -n "$legacy" ]; then
    log "check: legacy bare-agent-key agents: shape ($(printf '%s' "$legacy" | tr '\n' ' ')) — reshape to agents.default.<agent> (run: bash sync-agents.sh)"
    rc=1
  fi
  # leg (c) — local staleness (ADVISORY: reported, never fails CI; vacuous on a fresh clone).
  local src name got tmp d harness
  tmp="$(mktemp -d)"
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    for harness in $HARNESSES; do
      resolve_agent_layers "$harness" "$name" "$LOCAL_CFG" "$DOCKET_YML" "$GLOBAL_CFG"
      emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$tmp/docket-$name.md"
      got="$REPO/.$harness/agents/docket-$name.md"
      if [ ! -f "$got" ]; then
        log "advisory: .$harness/agents/docket-$name.md not generated on this machine (run: bash sync-agents.sh)"; continue
      fi
      d="$(diff -u "$got" "$tmp/docket-$name.md" || true)"
      if [ -n "$d" ]; then log "advisory: drift in .$harness/agents/docket-$name.md:"; printf '%s\n' "$d" >&2; fi
    done
  done
  rm -rf "$tmp"
  local h rule_got rule_tmp rd
  rule_tmp="$(mktemp)"
  assemble_dispatch_rule > "$rule_tmp"
  for h in $HARNESSES; do
    harness_has_dispatch_rule "$h" || continue
    rule_got="$REPO/.$h/rules/docket-dispatch.mdc"
    if [ ! -f "$rule_got" ]; then
      log "advisory: .$h/rules/docket-dispatch.mdc not generated on this machine (run: bash sync-agents.sh)"; continue
    fi
    rd="$(diff -u "$rule_got" "$rule_tmp" || true)"
    if [ -n "$rd" ]; then log "advisory: drift in .$h/rules/docket-dispatch.mdc:"; printf '%s\n' "$rd" >&2; fi
  done
  rm -f "$rule_tmp"
  ORPHAN_DRIFT=0
  prune_orphans per-repo          # handle_orphan logs; see 4c for the advisory downgrade
  return $rc
}
```

Write the leg (c) loops in full — they are the existing loops with (i) `resolve_agent_layers` (already switched in Task 2), (ii) message prefix `advisory:`, (iii) all `rc=1` assignments removed.

4c. `handle_orphan()` under `--check` currently reports drift + sets `ORPHAN_DRIFT=1`; downgrade its message to `advisory: orphaned docket-owned file $1 (run: bash sync-agents.sh)` and stop consuming `ORPHAN_DRIFT` in `check_project_level` (keep the variable set so the function stays valid, just unused for rc).

4d. Main flow — insert the migration before the gitignore block write:

```bash
migrate_legacy_global
resolve_global_agent_harnesses
user_level_pass
migrate_tracked_wrappers
ensure_gitignore_block
project_level_pass
prune_orphans all
log "done"
```

4e. Finish the header-comment rewrite: the Usage lines for `--check` now describe the three legs and the advisory semantics.

- [ ] **Step 5: Run the suite**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep -c -E -e '^NOT OK'` → expected `0`.
Also: `bash -n sync-agents.sh` → clean; `bash tests/test_install.sh 2>&1 | grep -c -E -e '^NOT OK'` → `0` (install calls sync-agents; hermetic XDG pins must still hold).

- [ ] **Step 6: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "feat(0051): 0048-era migration + three-leg --check (block, tracked-files, advisory staleness)"
```

---

### Task 5: Docs — README, convention, sample `.docket.yml`, sentinel updates + repo-wide audit

**Files:**
- Modify: `README.md` (§Global config ~line 99; §agent model/effort tuning section ~255–276; the line-65 script bullet)
- Modify: `.docket.yml` (comment prose only — the `agents:`/`agent_harnesses:` blocks, incl. the stale `agents.yaml` reference)
- Modify: `skills/docket-convention/SKILL.md` (Configuration; Agent layer; 0045/0048 paragraphs; sample-yml comments)
- Test: `tests/test_sync_agents.sh` (update the doc-sentinel asserts; add 0051 doc sentinels)

**Interfaces:**
- Consumes: the shipped behavior from Tasks 1–4 — write docs against the CODE (LEARNINGS #47), citing `sync-agents.sh` functions when in doubt.
- Produces: prose only.

- [ ] **Step 1: Audit — pin the full inventory before editing (LEARNINGS #42)**

Run and triage EVERY hit (the enumerated lists below are a floor):

```bash
grep -rn -e 'per-repo > global > built-in' README.md skills/ scripts/ tests/ .docket.yml
grep -rn -i -e 'committed.*wrapper\|wrapper.*committed\|committed project-level\|committed copies\|clone-identical' README.md skills/docket-convention/ .docket.yml tests/
grep -rn -e 'agents.yaml' .docket.yml README.md skills/
grep -rn -e 'SHADOWED\|shadow' README.md skills/ tests/ sync-agents.sh
```

- [ ] **Step 2: Update the doc sentinels to fail first**

In `tests/test_sync_agents.sh` update these existing asserts and add new ones (run the suite after — the updated sentinels must be RED against the un-edited docs):

- `"convention states the precedence"` → grep `repo-local > repo-committed > global > built-in` in `$CONV`.
- `"0050 doc: §global states per-key precedence"` → same four-layer string in the README §Global config extract.
- `"0050 doc: tuning section states sync-agents writes BOTH layers"` → keep, but the section text will now say the project level is machine-local; keep the `project wins` regex.
- New asserts:

```bash
# ---- Change 0051 doc sentinels ----
assert "0051 doc: README documents .docket.local.yml" 'grep -qF ".docket.local.yml" "$READMEF"'
assert "0051 doc: README states generated agents are machine-local, never committed" \
  'grep -qiE "machine-local" "$READMEF" && grep -qiE "never committed" "$READMEF"'
assert "0051 doc: README documents the docket:generated gitignore block" 'grep -qF "docket:generated" "$READMEF"'
assert "0051 doc: README documents the migration (git rm --cached / one commit)" 'grep -qiE "migrat" "$READMEF" && grep -qF -e "--cached" "$READMEF"'
assert "0051 doc: convention documents .docket.local.yml" 'grep -qF ".docket.local.yml" "$CONV"'
assert "0051 doc: convention states all-local generation (gitignored, never committed)" 'grep -qiE "gitignored, never committed|machine-local, never committed" "$CONV"'
assert "0051 doc: convention documents the three-leg --check" 'grep -qi "advisory" "$CONV" && grep -qF "docket:generated" "$CONV"'
assert "0051 doc: sample .docket.yml agents comment no longer claims COMMITTED wrappers" \
  '! grep -qiE "committed project-level \.|commits? no per-repo wrappers" "$REPO/.docket.yml" || true'
assert "0051 doc: sample .docket.yml drops the stale agents.yaml global reference" '! grep -q "agents.yaml" "$REPO/.docket.yml"'
```

(The sample-yml assert marked `|| true` is a scaffold — when editing the file, replace it with a positive anchor on the NEW wording, e.g. `grep -qi "machine-local" "$REPO/.docket.yml"`, and mutation-test it. One assert, one clause it owns — LEARNINGS #21.)

- [ ] **Step 3: Edit the docs**

README:
- §Global config: retitle the story to the FOUR layers (`repo-local > repo-committed > global > built-in`); after the `config.yml` example add a **`.docket.local.yml`** subsection: repo-root sibling of `.docket.yml`, gitignored via the managed block, accepts exactly the global-able key set (fenced keys warned-and-ignored — same posture as global), full commented example covering `skills:`, `agents:`, `agent_harnesses:`, `finalize:`, `auto_groom`, `board_surfaces` (note the `github` token is fenced here too).
- Agent tuning section: generated agent files are **machine-local and never committed** (both passes); each field resolves through all four layers at generation time; the managed `.gitignore` block + "commit it once"; the migration story for 0048-era repos (deletes tracked copies, prints the single `git rm --cached …` commit); `--check`'s three legs (block + no-tracked = CI, staleness = advisory); DELETE the "committed copies are what make an autonomous change build on the same model for every clone" sentence and note the retirement plainly (team defaults live in the committed `agents:` block by convention, without CI-enforced pinning).
- Line-65 script bullet: reword "writes committed project-level wrappers" → machine-local gitignored files.

`.docket.yml` sample comments: fix the `agents:` block prose (no "COMMITTED project-level" claim; global layer is `config.yml`'s `agents:` block, not `agents.yaml`; mention `.docket.local.yml` as the machine-scoped rung; `--check` meaning updated). Keep all commented keys.

Convention `skills/docket-convention/SKILL.md`:
- Configuration section: add `.docket.local.yml` to the 0050 paragraph's story (rename it the "config layers" paragraph): one optional machine-local file at the repo root, gitignored, global-able keys only, four-layer per-field precedence, resolver unchanged interface.
- Agent layer: add the Local row to the layer table; rewrite the 0048 "always-full-set" paragraph: full set still written per resolved harness but as **gitignored machine-local files**; opt-in signal now "either file"; the managed `.gitignore` block; migration; `--check` three legs; state the retirement of the clone-identical-committed-wrapper guarantee explicitly (solo-first call, recorded in the build ADR).
- 0045 paragraph: the fan-out target description drops "committed".

- [ ] **Step 4: Run the full suite**

```bash
for t in tests/test_*.sh; do echo "== $t"; bash "$t" 2>&1 | grep -E -e '^NOT OK' && exit 1; done; echo ALL GREEN
```

Expected: `ALL GREEN` (the loop prints any failing assert and stops).

- [ ] **Step 5: Commit**

```bash
git add README.md .docket.yml skills/docket-convention/SKILL.md tests/test_sync_agents.sh
git commit -m "docs(0051): four-layer config story — .docket.local.yml, machine-local agents, migration + new --check"
```

---

### Task 6: Whole-suite verification + real-data smoke

**Files:** none new (fixes fold back into the owning task's files if red).

- [ ] **Step 1: Full suite, clean env**

```bash
env -u DOCKET_SCRIPTS_DIR bash -c 'for t in tests/test_*.sh; do out="$(bash "$t" 2>&1)"; rc=$?; if [ $rc -ne 0 ] || grep -q -E -e "^NOT OK" <<<"$out"; then echo "FAIL $t"; grep -E -e "^NOT OK" <<<"$out"; fi; done; echo SUITE-DONE'
```

Expected: only `SUITE-DONE` (LEARNINGS #34: `env -u DOCKET_SCRIPTS_DIR` so fail-loud `${VAR:?}` tests aren't masked by the dev shell's export).

- [ ] **Step 2: Real-data smoke in a real git worktree (LEARNINGS #35)**

```bash
SMOKE="$(mktemp -d)" && git clone --quiet . "$SMOKE/repo" && HR="$(mktemp -d)" && mkdir -p "$HR/.claude" \
&& printf 'agents:\n  default:\n    status: { model: smoke-m }\n' > "$SMOKE/repo/.docket.local.yml" \
&& ( cd "$SMOKE/repo" && DOCKET_HARNESS_ROOT="$HR" bash sync-agents.sh ) \
&& grep -q '^model: smoke-m' "$SMOKE/repo/.claude/agents/docket-status.md" && echo SMOKE-GEN-OK \
&& ( cd "$SMOKE/repo" && git status --porcelain ) \
&& ( cd "$SMOKE/repo" && DOCKET_HARNESS_ROOT="$HR" bash sync-agents.sh --check ); echo "check rc=$?" ; rm -rf "$SMOKE" "$HR"
```

Expected: `SMOKE-GEN-OK`; `git status --porcelain` shows ONLY `?? .gitignore` (the generated agent files are ignored — the entire point); `--check` rc=1 solely because the block isn't committed yet (leg a) — commit `.gitignore` inside the smoke clone and re-run if you want the full green path. Any generated file appearing as `??` in status = the gitignore block is wrong — stop and fix.

- [ ] **Step 3: Syntax + shellcheck-lite pass**

```bash
bash -n sync-agents.sh && bash -n scripts/docket-config.sh && echo SYNTAX-OK
/bin/bash -n sync-agents.sh && echo BASH32-PARSE-OK
```

Expected: both markers print.

- [ ] **Step 4: Commit any fixes; final state = all green**

No placeholder commits; if Steps 1–3 were green with no changes, this task produces no commit.

---

## Build-time notes (for the controller, not a task)

- **ADR (spec §5)** is recorded at implement-next Step 6 via the `docket-adr` subagent: one new ADR — generated agent artifacts are machine-local, never committed; `.docket.local.yml` completes the four-layer config — **superseding ADR-0017's committed-generation model** (keep its opt-in gate, full-set rationale, prune scoping) + dated `## Update` notes on ADR-0008 and ADR-0016; the retired clone-identical guarantee recorded explicitly. ADRs 8/15/16/17/19 are already in the change's `adrs:` list so terminal-publish re-copies the updated bodies at merge.
- **Results file** should record the spec discrepancy: spec §5 names a `sync-agents.md` contract that has never existed; the header comment was updated instead (do not mint a new contract file without human sign-off).
- The 0050 results' remaining migration-polish items (config.yml-as-directory abort path, `.migrated` clobber) stay OUT of scope — not in the spec.
