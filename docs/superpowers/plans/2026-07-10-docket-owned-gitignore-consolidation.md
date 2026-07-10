# Docket-owned .gitignore consolidation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make one marker-bounded, self-healing `.gitignore` block the single home for ALL docket-owned ignores, written identically by three writers via a shared lib.

**Architecture:** A new sourceable lib `scripts/lib/docket-gitignore-block.sh` owns the canonical harness roster, the marker constants (new + legacy-for-upgrade), a constant-bytes block emitter, and a hardened `ensure` (closed-block guard on both marker spellings, one-time legacy upgrade, outside-bytes invariant, dedup advisory). Three writers call `ensure` with their own trigger policy: `sync-agents.sh` (widened self-heal trigger), `migrate-to-docket.sh` step 5 (unconditional, after removing its historical bare lines), and `docket-config.sh --bootstrap` (on `CREATE_ORPHAN`, write + loud notice, no auto-commit). Because the emitter is config-independent, every writer emits byte-identical output by construction.

**Tech Stack:** POSIX-ish bash (bash 3.2 compatible — macOS), `awk`/`grep -F -x` for marker-safe block editing, the repo's `assert`-based shell test harness (`bash tests/test_*.sh`).

## Global Constraints

- **Bash 3.2 compatible** (macOS default) — no associative-array-only features in the lib's public path; mirror the style already in `sync-agents.sh`.
- **Marker-block edits must verify the block is CLOSED before any range edit** (LEARNINGS #51, `b0c1980`): a dangling start marker of *either* spelling ⇒ refuse the edit, warn loudly, touch nothing. Never let an `awk`/`sed` range consume to EOF.
- **Outside-bytes invariant:** bytes outside the markers are NEVER modified by the lib (the healer never deletes user-authored lines).
- **Constant emitter:** `emit_docket_gitignore_block` output depends only on the static roster constants — no config, no environment. Every writer sharing it emits identical bytes.
- **New marker spelling (exact bytes):** start `# docket:start (managed by docket — do not hand-edit)`, end `# docket:end`. **Legacy 0051 spelling (exact bytes, upgrade-detection only):** start `# docket:generated:start (managed by sync-agents.sh — do not hand-edit)`, end `# docket:generated:end`.
- **Block contents, in this exact order:** `.docket/`, `.worktrees/`, `.claude/settings.local.json`, `.docket.local.yml`, then per-token `.<H>/agents/docket-*.md` for `H` in `claude codex cursor agents kiro windsurf`, then `.cursor/rules/docket-dispatch.mdc`.
- **A printed remedy command must be valid in the exact repo state that produced it** (LEARNINGS #51, `41d9815`) — branch printed text on the same condition that gates the underlying write.
- **Hermetic tests:** any test that reaches a shared user-level write path must pin `XDG_CONFIG_HOME`/`HOME`/`DOCKET_HARNESS_ROOT` to the sandbox (LEARNINGS #50, #34). Fixture git repos set `user.email`/`user.name`. Real-data smokes of remote-dependent branches run inside a real worktree, never `/tmp` (LEARNINGS #35).
- **Run the full suite foreground, one command:** `for t in tests/test_*.sh; do echo "== $t"; bash "$t" || fail=1; done` (or the repo's usual per-file `bash tests/<name>.sh`). No backgrounding.
- **Out of feature branch (metadata, handled at Step 6 / close-out, NOT a task here):** ADR-0020's decision-3 `## Update` note is a metadata edit on `origin/docket`, delivered to the integration branch via this change's `adrs: [20]` at terminal-publish. The feature branch never modifies ADRs.
- **Prose-sweep coordination:** open PRs #61 (0052, rewrites `README.md`) and #62 (0053, restructures `docket-convention` SKILL.md) overlap the Task-5 surface. Keep Task-5 edits minimal and marker-scoped; if either merges first, finalize's rebase gate surfaces the conflict — resolve by intent (LEARNINGS #37), never blindly keep-mine.

---

### Task 1: The shared lib — `scripts/lib/docket-gitignore-block.sh`

**Files:**
- Create: `scripts/lib/docket-gitignore-block.sh`
- Test: `tests/test_docket_gitignore_block.sh`

**Interfaces:**
- Consumes: nothing (sourceable, no side effects on source).
- Produces (for Tasks 2–4):
  - Constants: `DOCKET_GI_HARNESS_TOKENS` (=`claude codex cursor agents kiro windsurf`), `DOCKET_GI_DISPATCH_HARNESSES` (=`cursor`), `DOCKET_GI_START`, `DOCKET_GI_END`, `DOCKET_GI_LEGACY_START`, `DOCKET_GI_LEGACY_END`, `DOCKET_GI_CORE_ENTRIES` (=`.docket/ .worktrees/ .claude/settings.local.json`).
  - `emit_docket_gitignore_block` → prints the constant block (incl. markers) to stdout.
  - `ensure_docket_gitignore_block <repo-root>` → create/refresh/upgrade the block in `<repo-root>/.gitignore`; returns 0 always (refuses safely on a dangling marker). Logs notices/advisories to stderr with a `docket-gitignore:` prefix. Never checks a "wanted" trigger (that stays with callers).

- [ ] **Step 1: Write the failing test (emitter + ensure behaviors)**

Create `tests/test_docket_gitignore_block.sh`:

```bash
#!/usr/bin/env bash
# tests/test_docket_gitignore_block.sh — run: bash tests/test_docket_gitignore_block.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LIB="$REPO/scripts/lib/docket-gitignore-block.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "lib exists" '[ -f "$LIB" ]'
# shellcheck source=/dev/null
. "$LIB"

# --- emitter: constant bytes, correct order, all lines docket-scoped -----------
BLK="$(emit_docket_gitignore_block)"
assert "emit: opens with new start marker" 'printf "%s\n" "$BLK" | head -1 | grep -qF "# docket:start (managed by docket — do not hand-edit)"'
assert "emit: closes with new end marker"  'printf "%s\n" "$BLK" | tail -1 | grep -qxF "# docket:end"'
assert "emit: core .docket/"               'printf "%s\n" "$BLK" | grep -qxF ".docket/"'
assert "emit: core .worktrees/"            'printf "%s\n" "$BLK" | grep -qxF ".worktrees/"'
assert "emit: core settings.local.json"    'printf "%s\n" "$BLK" | grep -qxF ".claude/settings.local.json"'
assert "emit: .docket.local.yml"           'printf "%s\n" "$BLK" | grep -qxF ".docket.local.yml"'
assert "emit: claude agents pattern"       'printf "%s\n" "$BLK" | grep -qxF ".claude/agents/docket-*.md"'
assert "emit: windsurf agents pattern"     'printf "%s\n" "$BLK" | grep -qxF ".windsurf/agents/docket-*.md"'
assert "emit: cursor dispatch rule"        'printf "%s\n" "$BLK" | grep -qxF ".cursor/rules/docket-dispatch.mdc"'
assert "emit: every non-marker line is docket-scoped (starts with . )" \
  '! printf "%s\n" "$BLK" | grep -v "^#" | grep -qvE "^\."'
assert "emit: deterministic (two calls identical)" '[ "$(emit_docket_gitignore_block)" = "$(emit_docket_gitignore_block)" ]'
assert "emit: .docket/ precedes .docket.local.yml" \
  '[ "$(printf "%s\n" "$BLK" | grep -nxF ".docket/" | cut -d: -f1)" -lt "$(printf "%s\n" "$BLK" | grep -nxF ".docket.local.yml" | cut -d: -f1)" ]'

# --- ensure: fresh file created -----------------------------------------------
SBX="$(mktemp -d)"
( ensure_docket_gitignore_block "$SBX" ) 2>/dev/null
assert "ensure: fresh .gitignore created with the block" 'grep -qxF "# docket:start (managed by docket — do not hand-edit)" "$SBX/.gitignore" && grep -qxF "# docket:end" "$SBX/.gitignore"'
assert "ensure: fresh block equals emitter bytes" '[ "$(emit_docket_gitignore_block)" = "$(cat "$SBX/.gitignore")" ]'
rm -rf "$SBX"

# --- ensure: idempotent + preserves outside bytes -----------------------------
SBX="$(mktemp -d)"
printf 'node_modules/\n' > "$SBX/.gitignore"
( ensure_docket_gitignore_block "$SBX" ) 2>/dev/null
before="$(cat "$SBX/.gitignore")"
err2="$( { ensure_docket_gitignore_block "$SBX"; } 2>&1 >/dev/null )"
assert "ensure: idempotent second run byte-identical" '[ "$before" = "$(cat "$SBX/.gitignore")" ]'
assert "ensure: idempotent run prints no UPDATED/UPGRADED notice" '! printf "%s" "$err2" | grep -qiE "updated|upgraded"'
assert "ensure: user line preserved above block" 'grep -qxF "node_modules/" "$SBX/.gitignore"'
rm -rf "$SBX"

# --- ensure: one-time legacy upgrade (closed 0051 block) ----------------------
SBX="$(mktemp -d)"
{
  printf 'my-own-ignore/\n'
  printf '# docket:generated:start (managed by sync-agents.sh — do not hand-edit)\n'
  printf '.docket.local.yml\n.claude/agents/docket-*.md\n'
  printf '# docket:generated:end\n'
  printf 'tail-user-line/\n'
} > "$SBX/.gitignore"
upg_err="$( { ensure_docket_gitignore_block "$SBX"; } 2>&1 >/dev/null )"
assert "upgrade: legacy start marker gone"     '! grep -qF "docket:generated:start" "$SBX/.gitignore"'
assert "upgrade: new start marker present"     'grep -qxF "# docket:start (managed by docket — do not hand-edit)" "$SBX/.gitignore"'
assert "upgrade: exactly one new block"        '[ "$(grep -cxF "# docket:start (managed by docket — do not hand-edit)" "$SBX/.gitignore")" = "1" ]'
assert "upgrade: user bytes above preserved"   'grep -qxF "my-own-ignore/" "$SBX/.gitignore"'
assert "upgrade: user bytes below preserved"   'grep -qxF "tail-user-line/" "$SBX/.gitignore"'
assert "upgrade: announces the upgrade"        'printf "%s" "$upg_err" | grep -qi "upgrad"'
rm -rf "$SBX"

# --- ensure: dangling NEW start marker -> refuse, warn, byte-identical --------
SBX="$(mktemp -d)"
printf '# docket:start (managed by docket — do not hand-edit)\n.docket/\nnode_modules/\n' > "$SBX/.gitignore"
before="$(cat "$SBX/.gitignore")"
dg_err="$( { ensure_docket_gitignore_block "$SBX"; } 2>&1 >/dev/null )"; dg_rc=$?
assert "dangling-new: run still returns 0"        '[ "$dg_rc" = "0" ]'
assert "dangling-new: warns unterminated/corrupt" 'printf "%s" "$dg_err" | grep -qiE "untermin|corrupt"'
assert "dangling-new: file byte-identical"        '[ "$before" = "$(cat "$SBX/.gitignore")" ]'
rm -rf "$SBX"

# --- ensure: dangling LEGACY start marker -> refuse, warn, byte-identical ------
SBX="$(mktemp -d)"
printf '# docket:generated:start (managed by sync-agents.sh — do not hand-edit)\n.docket.local.yml\nnode_modules/\n' > "$SBX/.gitignore"
before="$(cat "$SBX/.gitignore")"
dl_err="$( { ensure_docket_gitignore_block "$SBX"; } 2>&1 >/dev/null )"; dl_rc=$?
assert "dangling-legacy: run still returns 0"        '[ "$dl_rc" = "0" ]'
assert "dangling-legacy: warns unterminated/corrupt" 'printf "%s" "$dl_err" | grep -qiE "untermin|corrupt"'
assert "dangling-legacy: file byte-identical"        '[ "$before" = "$(cat "$SBX/.gitignore")" ]'
rm -rf "$SBX"

# --- ensure: dedup advisory for bare literals OUTSIDE the block ---------------
SBX="$(mktemp -d)"
printf '.docket/\n.worktrees\n' > "$SBX/.gitignore"   # pre-existing bare entries (note: no trailing slash on second)
adv_err="$( { ensure_docket_gitignore_block "$SBX"; } 2>&1 >/dev/null )"
assert "dedup: advisory logged for outside bare entries" 'printf "%s" "$adv_err" | grep -qi "safe to delete"'
assert "dedup: outside bare .docket/ NOT deleted"        'grep -qxF ".docket/" <(awk "/# docket:start/{f=1} !f{print} /# docket:end/{f=0}" "$SBX/.gitignore")'
assert "dedup: block still written"                      'grep -qxF "# docket:end" "$SBX/.gitignore"'
rm -rf "$SBX"

[ "$fail" = 0 ] && echo "ALL PASS" || echo "SOME FAILED"
exit "$fail"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_docket_gitignore_block.sh`
Expected: FAIL — `NOT OK - lib exists` (file not created yet), suite exits non-zero.

- [ ] **Step 3: Write the lib**

Create `scripts/lib/docket-gitignore-block.sh`:

```bash
#!/usr/bin/env bash
# scripts/lib/docket-gitignore-block.sh — shared owner of docket's managed .gitignore block
# (change 0057). SOURCE this; no side effects on source beyond declaring constants/functions.
# No git, no network. The block is the single home for ALL docket-owned ignores; three writers
# (migrate-to-docket.sh, docket-config.sh --bootstrap, sync-agents.sh) call ensure with their
# own trigger policy. The emitter is a pure constant, so every writer emits identical bytes.

# Canonical harness roster (moved here from sync-agents.sh; that script now sources this lib).
DOCKET_GI_HARNESS_TOKENS="claude codex cursor agents kiro windsurf"
DOCKET_GI_DISPATCH_HARNESSES="cursor"

# New markers (broadened contents + ownership).
DOCKET_GI_START='# docket:start (managed by docket — do not hand-edit)'
DOCKET_GI_END='# docket:end'
# Legacy 0051 markers — kept ONLY for one-time upgrade detection (never re-emitted).
DOCKET_GI_LEGACY_START='# docket:generated:start (managed by sync-agents.sh — do not hand-edit)'
DOCKET_GI_LEGACY_END='# docket:generated:end'

# The three core bare entries docket historically wrote at migrate time (now inside the block).
DOCKET_GI_CORE_ENTRIES=".docket/ .worktrees/ .claude/settings.local.json"

_docket_gi_log(){ printf '%s\n' "docket-gitignore: $*" >&2; }

# Constant block bytes (incl. markers) to stdout. Config-independent.
emit_docket_gitignore_block(){
  local e tok
  printf '%s\n' "$DOCKET_GI_START"
  for e in $DOCKET_GI_CORE_ENTRIES; do printf '%s\n' "$e"; done
  printf '.docket.local.yml\n'
  for tok in $DOCKET_GI_HARNESS_TOKENS;   do printf '.%s/agents/docket-*.md\n' "$tok"; done
  for tok in $DOCKET_GI_DISPATCH_HARNESSES; do printf '.%s/rules/docket-dispatch.mdc\n' "$tok"; done
  printf '%s\n' "$DOCKET_GI_END"
}

# True iff FILE has a START line ($2) with no matching END line ($3). grep on the file path
# (no producer|grep pipeline) so it is safe under pipefail; bash-3.2-safe.
_docket_gi_unterminated(){  # $1=file $2=start $3=end
  [ -f "$1" ] || return 1
  grep -F -x -q -- "$2" "$1" || return 1
  grep -F -x -q -- "$3" "$1" && return 1
  return 0
}

# Print FILE with the [start,end] block (inclusive) removed; bytes outside preserved.
_docket_gi_strip_block(){  # $1=file $2=start $3=end
  awk -v s="$2" -v e="$3" '$0==s{f=1} !f{print} $0==e{f=0}' "$1"
}

# Print only the [start,end] block from FILE (inclusive) — for want-vs-have compare.
_docket_gi_current_block(){  # $1=file $2=start $3=end
  [ -f "$1" ] || return 0
  awk -v s="$2" -v e="$3" '$0==s{f=1} f{print} $0==e{f=0}' "$1"
}

# Advisory (never deletes): warn once if any core bare literal sits OUTSIDE the block.
# Uses grep -F -x (exact, metachar-safe — LEARNINGS #26), both slash variants.
_docket_gi_dedup_advisory(){  # $1=text outside the block
  local e bare hit=0
  for e in $DOCKET_GI_CORE_ENTRIES; do
    bare="${e%/}"
    if printf '%s\n' "$1" | grep -F -x -q -- "$bare" || printf '%s\n' "$1" | grep -F -x -q -- "$bare/"; then hit=1; fi
  done
  [ "$hit" -eq 1 ] && _docket_gi_log "advisory: old docket bare entries found outside the managed block in .gitignore — safe to delete by hand (duplicates are harmless)."
  return 0
}

# Create/refresh/upgrade the managed block in <repo-root>/.gitignore. Hardened:
# closed-block guard on BOTH spellings, one-time legacy upgrade, idempotence, outside-bytes
# invariant, dedup advisory. Trigger policy is the CALLER's — ensure always tries.
ensure_docket_gitignore_block(){  # $1=repo-root
  local root="$1" gi="$1/.gitignore" want have rest legacy_present=0
  want="$(emit_docket_gitignore_block)"

  # (1) Closed-block guard — dangling start of EITHER spelling: refuse, warn, touch nothing.
  if _docket_gi_unterminated "$gi" "$DOCKET_GI_START" "$DOCKET_GI_END"; then
    _docket_gi_log "WARN $gi has an UNTERMINATED docket block (start present, end missing) — corrupt; refusing to rewrite so no bytes are lost. Repair or remove the dangling '$DOCKET_GI_START' line by hand, then re-run."
    return 0
  fi
  if _docket_gi_unterminated "$gi" "$DOCKET_GI_LEGACY_START" "$DOCKET_GI_LEGACY_END"; then
    _docket_gi_log "WARN $gi has an UNTERMINATED legacy docket:generated block (start present, end missing) — corrupt; refusing to rewrite so no bytes are lost. Repair or remove the dangling '$DOCKET_GI_LEGACY_START' line by hand, then re-run."
    return 0
  fi

  [ -f "$gi" ] && grep -F -x -q -- "$DOCKET_GI_LEGACY_START" "$gi" && legacy_present=1

  # (2) rest = everything OUTSIDE both the new and the legacy block (outside-bytes preserved).
  rest=""
  if [ -f "$gi" ]; then
    rest="$(_docket_gi_strip_block "$gi" "$DOCKET_GI_START" "$DOCKET_GI_END")"
    rest="$(printf '%s\n' "$rest" | awk -v s="$DOCKET_GI_LEGACY_START" -v e="$DOCKET_GI_LEGACY_END" '$0==s{f=1} !f{print} $0==e{f=0}')"
  fi
  have="$(_docket_gi_current_block "$gi" "$DOCKET_GI_START" "$DOCKET_GI_END")"

  # (3) Idempotence — current new block already exact AND no legacy block to upgrade: no write.
  if [ "$want" = "$have" ] && [ "$legacy_present" -eq 0 ]; then
    _docket_gi_dedup_advisory "$rest"
    return 0
  fi

  # (4) Rewrite: outside bytes, blank separator, then a single new block.
  {
    if [ -n "$rest" ]; then printf '%s\n\n' "$rest"; fi
    printf '%s\n' "$want"
  } > "$gi"
  if [ "$legacy_present" -eq 1 ]; then
    _docket_gi_log "UPGRADED $gi legacy docket:generated block to the docket block — COMMIT THIS so docket-owned files stay untracked."
  else
    _docket_gi_log "UPDATED $gi managed docket block — COMMIT THIS so docket-owned files stay untracked."
  fi
  _docket_gi_dedup_advisory "$rest"
  return 0
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_docket_gitignore_block.sh`
Expected: PASS — `ALL PASS`, exit 0.

- [ ] **Step 5: Commit**

```bash
git add scripts/lib/docket-gitignore-block.sh tests/test_docket_gitignore_block.sh
git commit -m "feat(0057): shared docket-owned .gitignore block lib + tests"
```

---

### Task 2: Rewire `sync-agents.sh` onto the lib (roster move, widened trigger)

**Files:**
- Modify: `sync-agents.sh` (source the lib; delete local roster array/marker constants/`emit_gitignore_block`/`ensure_gitignore_block`/`current_gitignore_block`/`gitignore_block_unterminated`; derive `HARNESS_AGENT_DIRS`/`VALID_HARNESS_TOKENS`/`HARNESS_HAS_DISPATCH_RULES` from lib constants; widen `gitignore_block_wanted`; point `--check` leg (a) at the new markers)
- Test: `tests/test_sync_agents.sh` (re-point 0051 marker tests to the new spelling; add widened-trigger positive/negative)

**Interfaces:**
- Consumes: `emit_docket_gitignore_block`, `ensure_docket_gitignore_block`, `_docket_gi_current_block`, `DOCKET_GI_*` constants from Task 1.
- Produces: `sync-agents.sh` behavior unchanged for agent generation; `.gitignore` block now uses the new marker + core entries; trigger widened to `{opted-in ∨ .docket.local.yml ∨ docket branch exists ∨ block markers (either spelling) present}`.

- [ ] **Step 1: Update the failing tests (marker rename + widened trigger)**

In `tests/test_sync_agents.sh`, re-point every `0051 gi:`/`0051 doc:` assertion that hard-codes `# docket:generated:start`/`docket:generated:end`/`docket:generated` to the new spelling (`# docket:start (managed by docket — do not hand-edit)` / `# docket:end`), and update the section-header comment. Also update the `(gi-f)` fixture and the migration tests' `grep -q "^# docket:generated:start"` assertions to the new start marker. Then add these NEW assertions after the existing `(gi-e)` block:

```bash
# (gi-core) the block now carries the three core docket-owned entries (change 0057).
make_sandbox
HROOTGC="$(mktemp -d)"; mkdir -p "$HROOTGC/.claude"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGC" bash "$SYNC" >/dev/null 2>&1 )
GI="$SBX/.gitignore"
assert "0057 gi: block carries .docket/"              'grep -qxF ".docket/" "$GI"'
assert "0057 gi: block carries .worktrees/"           'grep -qxF ".worktrees/" "$GI"'
assert "0057 gi: block carries settings.local.json"   'grep -qxF ".claude/settings.local.json" "$GI"'
assert "0057 gi: new start marker, no legacy marker"  'grep -qxF "# docket:start (managed by docket — do not hand-edit)" "$GI" && ! grep -qF "docket:generated" "$GI"'
rm -rf "$SBX" "$HROOTGC"

# (gi-widen+) widened trigger POSITIVE: a tracking-only repo (NOT opted in, no local file) that
# HAS a local docket branch heals the block (the bootstrap guard's DOCKET probe).
mkgitrepo
HROOTGW="$(mktemp -d)"; mkdir -p "$HROOTGW/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"        # tracking-only, not opted in
git -C "$SBX" add -A; git -C "$SBX" commit --quiet -m init
git -C "$SBX" branch docket                                    # DOCKET signal present
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGW" bash "$SYNC" >/dev/null 2>&1 )
assert "0057 gi: docket-branch repo heals the block"  'grep -qxF "# docket:start (managed by docket — do not hand-edit)" "$SBX/.gitignore"'
assert "0057 gi: but still generates zero agent files" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOTGW"

# (gi-widen-) widened trigger NEGATIVE (the 0048 regression): a repo with NO docket signal
# (no opt-in, no .docket.local.yml, no docket branch, no existing block) is untouched.
mkgitrepo
HROOTGN="$(mktemp -d)"; mkdir -p "$HROOTGN/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"
git -C "$SBX" add -A; git -C "$SBX" commit --quiet -m init
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGN" bash "$SYNC" >/dev/null 2>&1 )
assert "0057 gi: no-signal repo gets NO .gitignore" '[ ! -e "$SBX/.gitignore" ]'
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGN" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0057 gi: no-signal repo --check stays a no-op (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX" "$HROOTGN"

# (gi-heal-present) heal-if-present: a repo carrying only a legacy block (no other signal) is
# UPGRADED to the new block.
mkgitrepo
HROOTGH="$(mktemp -d)"; mkdir -p "$HROOTGH/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"
printf '# docket:generated:start (managed by sync-agents.sh — do not hand-edit)\n.docket.local.yml\n# docket:generated:end\n' > "$SBX/.gitignore"
git -C "$SBX" add -A; git -C "$SBX" commit --quiet -m init
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGH" bash "$SYNC" >/dev/null 2>&1 )
assert "0057 gi: legacy-only repo upgraded to new block" 'grep -qxF "# docket:start (managed by docket — do not hand-edit)" "$SBX/.gitignore" && ! grep -qF "docket:generated" "$SBX/.gitignore"'
rm -rf "$SBX" "$HROOTGH"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_sync_agents.sh`
Expected: FAIL — new `0057 gi:` assertions fail (old code emits the legacy marker, no core entries, and does not heal on the docket-branch signal).

- [ ] **Step 3: Rewire `sync-agents.sh`**

Near the top (after `set -euo pipefail`, `SCRIPT_DIR=...`), source the lib:

```bash
# shellcheck source=/dev/null
. "$SCRIPT_DIR/scripts/lib/docket-gitignore-block.sh"
```

Replace the `HARNESS_AGENT_DIRS=( ... )` literal array (lines ~71-78) with a derivation from the lib roster:

```bash
# Harness agent dirs, derived from the lib's canonical roster (single source of truth).
HARNESS_AGENT_DIRS=()
for _tok in $DOCKET_GI_HARNESS_TOKENS; do HARNESS_AGENT_DIRS+=("$HARNESS_ROOT/.$_tok/agents"); done
unset _tok
```

Replace the `VALID_HARNESS_TOKENS` derivation loop with a direct assignment, and `HARNESS_HAS_DISPATCH_RULES` with the lib constant:

```bash
VALID_HARNESS_TOKENS="$DOCKET_GI_HARNESS_TOKENS"
```
```bash
HARNESS_HAS_DISPATCH_RULES="$DOCKET_GI_DISPATCH_HARNESSES"
```

Delete the now-duplicated block-mechanics section (the `GITIGNORE`/`GI_START`/`GI_END` constants, `emit_gitignore_block`, `current_gitignore_block`, `gitignore_block_unterminated`, `ensure_gitignore_block`), **keeping** the `GITIGNORE="$REPO/.gitignore"` assignment (still used by `gitignore_block_wanted`/`--check`). Widen `gitignore_block_wanted` and delegate the ensure:

```bash
GITIGNORE="$REPO/.gitignore"

# The block is maintained for opted-in repos, any repo carrying a .docket.local.yml, any repo
# with a docket branch (the bootstrap guard's DOCKET probe — an explicit repo-level signal,
# LEARNINGS #48), or any repo already carrying the block (heal-if-present, either spelling).
gitignore_block_wanted(){
  per_repo_opted_in && return 0
  [ -e "$REPO/.docket.local.yml" ] && return 0
  git -C "$REPO" rev-parse --verify --quiet refs/remotes/origin/docket >/dev/null 2>&1 && return 0
  git -C "$REPO" rev-parse --verify --quiet refs/heads/docket >/dev/null 2>&1 && return 0
  [ -f "$GITIGNORE" ] && grep -F -x -q -- "$DOCKET_GI_START" "$GITIGNORE" && return 0
  [ -f "$GITIGNORE" ] && grep -F -x -q -- "$DOCKET_GI_LEGACY_START" "$GITIGNORE" && return 0
  return 1
}
```

Replace the old call site `ensure_gitignore_block` (in the main run) with:

```bash
if gitignore_block_wanted; then ensure_docket_gitignore_block "$REPO"; fi
```

In `check_project_level` leg (a), replace the stale-block comparison:

```bash
  # leg (a) — the .gitignore block is present and current, evaluated against the NEW markers.
  if [ "$(emit_docket_gitignore_block)" != "$(_docket_gi_current_block "$GITIGNORE" "$DOCKET_GI_START" "$DOCKET_GI_END")" ]; then
    log "check: .gitignore docket block missing or stale (a legacy docket:generated block upgrades on the next run) — run: bash sync-agents.sh and commit .gitignore"
    rc=1
  fi
```

Update the `# --- managed .gitignore block (change 0051) ---` header comment to note the mechanics now live in the lib (change 0057).

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_sync_agents.sh`
Expected: PASS — `ALL PASS` (existing agent-generation + migration tests still green, new `0057 gi:` tests green).

- [ ] **Step 5: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "feat(0057): sync-agents.sh sources the block lib; widened self-heal trigger"
```

---

### Task 3: `migrate-to-docket.sh` step 5 seeds the block (removes bare lines)

**Files:**
- Modify: `migrate-to-docket.sh` (source the lib; step 5 removes the three historically-written bare lines from `PRUNE_WT/.gitignore`, then calls `ensure_docket_gitignore_block "$PRUNE_WT"`; commit rides the existing step-5 integration commit; update the step-5 comment + header line 23)
- Test: `tests/test_docket_gitignore_block.sh` (add a migrate-end-state + constant-emitter-equivalence section that exercises the lib as migrate uses it — no full migrate run needed)

**Interfaces:**
- Consumes: `ensure_docket_gitignore_block`, `emit_docket_gitignore_block`, `DOCKET_GI_CORE_ENTRIES` from Task 1.
- Produces: after a migration, `PRUNE_WT/.gitignore` ends with the block present and no bare `.docket/`/`.worktrees/`/`.claude/settings.local.json` lines outside it; re-run idempotent.

- [ ] **Step 1: Write the failing test (equivalence + migrate-shaped seed)**

Append to `tests/test_docket_gitignore_block.sh` (before the final `[ "$fail" = 0 ]` line):

```bash
# --- constant-emitter equivalence: a "migrate-seeded" tree and a "sync-healed" tree end with
#     byte-identical blocks (both call the shared emitter). ------------------------------------
A="$(mktemp -d)"; B="$(mktemp -d)"
# migrate-shaped: pre-existing bare lines the migration must remove, then ensure.
printf '.docket/\n.worktrees/\n.claude/settings.local.json\n' > "$A/.gitignore"
for e in $DOCKET_GI_CORE_ENTRIES; do   # emulate migrate's own bare-line removal (^entry(/)?$)
  bare="${e%/}"; tmp="$(mktemp)"; grep -Ev "^${bare//./\\.}/?$" "$A/.gitignore" > "$tmp"; mv "$tmp" "$A/.gitignore"
done
( ensure_docket_gitignore_block "$A" ) 2>/dev/null
# sync-shaped: empty start, just ensure.
( ensure_docket_gitignore_block "$B" ) 2>/dev/null
assert "equivalence: migrate-seeded == sync-healed block bytes" \
  '[ "$(awk "/# docket:start/{f=1} f{print} /# docket:end/{f=0}" "$A/.gitignore")" = "$(awk "/# docket:start/{f=1} f{print} /# docket:end/{f=0}" "$B/.gitignore")" ]'
assert "equivalence: migrate-seeded leaves no bare .docket/ outside the block" \
  '[ -z "$(awk "/# docket:start/{f=1} !f{print} /# docket:end/{f=0}" "$A/.gitignore" | grep -xF ".docket/")" ]'
# idempotent re-run
before="$(cat "$A/.gitignore")"; ( ensure_docket_gitignore_block "$A" ) 2>/dev/null
assert "equivalence: migrate seed re-run idempotent" '[ "$before" = "$(cat "$A/.gitignore")" ]'
rm -rf "$A" "$B"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_docket_gitignore_block.sh`
Expected: PASS for the lib parts, but this new section should already pass if Task 1's lib is correct — this is a *regression guard* for Task 3's contract. If it fails, fix the lib. (Then proceed to wire migrate.)

> Note: this section validates the lib mechanics migrate relies on. The migrate script itself is covered by the manual smoke in Step 4 (a full `migrate-to-docket.sh` run needs a remote fixture; there is no automated migrate suite — keep the smoke real-worktree per LEARNINGS #35).

- [ ] **Step 3: Wire `migrate-to-docket.sh`**

After `MIGRATE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"`, source the lib:

```bash
# shellcheck source=/dev/null
. "$MIGRATE_DIR/scripts/lib/docket-gitignore-block.sh"
```

Replace the step-5 body (the `for entry in ".docket/" ".worktrees/" ".claude/settings.local.json"` loop and its `added_ignore` commit gate, lines ~327-347) with: remove any historical bare lines this script wrote, then seed the block via the lib, then commit if the tree changed:

```bash
step "Seeding the managed docket .gitignore block on '$INTEGRATION_BRANCH'"
GITIGNORE="$PRUNE_WT/.gitignore"
touch "$GITIGNORE"
# Remove the three bare lines migrate historically wrote (its own provenance; same match it
# used to add them) — they now live inside the managed block. User-authored duplicates, if
# any, are indistinguishable and are left to the lib's dedup advisory, not deleted here.
for entry in $DOCKET_GI_CORE_ENTRIES; do
  bare="${entry%/}"
  tmp="$(mktemp)"; grep -F -x -v -- "$bare" "$GITIGNORE" | grep -F -x -v -- "$bare/" > "$tmp" || true
  mv "$tmp" "$GITIGNORE"
done
ensure_docket_gitignore_block "$PRUNE_WT"
git -C "$PRUNE_WT" add .gitignore
if ! git -C "$PRUNE_WT" diff --cached --quiet; then
  git -C "$PRUNE_WT" commit --quiet -m "docket: seed managed .gitignore block (migrate-to-docket.sh)"
fi
```

Update the header line 23 comment to: `#   5. Seed the managed docket .gitignore block (.docket/, .worktrees/, .claude/settings.local.json, ...) on the integration branch (idempotent),`.

> Caveat to watch (LEARNINGS #26): `migrate-to-docket.sh` historically removed bares with an ERE `^${entry%/}/?$`; here we use `grep -F -x` (exact) for both slash variants to avoid ERE-metachar mismatch on `.claude/settings.local.json`.

- [ ] **Step 4: Manual real-worktree smoke (LEARNINGS #35)**

Run a `migrate-to-docket.sh` dry pass against a throwaway git fixture WITH an `origin` remote (a real worktree, not `/tmp` bare-only), and confirm the end-state `.gitignore` on the integration branch has the block present and no bare `.docket/`/`.worktrees/`/`.claude/settings.local.json` outside it; re-run to confirm idempotence (no second commit). Record the exact commands + output in the results file (Step 6.5). If a full migrate fixture is impractical in the session, assert equivalently via the Step-1 automated section and note the deferral prominently in the PR body.

- [ ] **Step 5: Commit**

```bash
git add migrate-to-docket.sh tests/test_docket_gitignore_block.sh
git commit -m "feat(0057): migrate step 5 seeds the managed block, drops bare lines"
```

---

### Task 4: `docket-config.sh --bootstrap` seeds the block on `CREATE_ORPHAN`

**Files:**
- Modify: `scripts/docket-config.sh` (define a self dir; source the lib; on the `--bootstrap` `CREATE_ORPHAN` write path, after `create_orphan`, call `ensure_docket_gitignore_block "$REPO_DIR"` + print a loud COMMIT-THIS notice; NEVER auto-commit; `--export` stays strictly read-only)
- Test: `tests/test_docket_config.sh` (bootstrap-seed: block written in the primary tree, notice printed, nothing committed; `--export` default writes nothing)

**Interfaces:**
- Consumes: `ensure_docket_gitignore_block` from Task 1.
- Produces: a fresh docket-mode repo that runs `--bootstrap` gets the managed block in its primary working tree (closing the fresh-repo gap), with a printed COMMIT reminder and no commit.

- [ ] **Step 1: Write the failing test**

In `tests/test_docket_config.sh`, after the `(W2)` bootstrap-fresh assertions, add:

```bash
# (W2-gi) --bootstrap in the fresh cell also SEEDS the managed .gitignore block in the
# primary tree, prints a loud COMMIT notice, and commits NOTHING (change 0057).
w2gi="$tmp/w2gi"; mkrepo "$w2gi"                       # fresh docket-mode repo (¬DOCKET ∧ ¬LIVE)
head_before="$(git -C "$w2gi" rev-parse HEAD 2>/dev/null || echo none)"
bs_err="$(run "$w2gi" --bootstrap --export 2>&1 >/dev/null)"
assert "0057 bootstrap: block seeded in primary tree" 'grep -qxF "# docket:start (managed by docket — do not hand-edit)" "$w2gi/.gitignore"'
assert "0057 bootstrap: loud COMMIT notice printed"   'printf "%s" "$bs_err" | grep -qi "commit"'
assert "0057 bootstrap: nothing auto-committed"       '[ "$(git -C "$w2gi" rev-parse HEAD 2>/dev/null || echo none)" = "$head_before" ]'
assert "0057 bootstrap: .gitignore left UNstaged"     '[ -z "$(git -C "$w2gi" diff --cached --name-only 2>/dev/null)" ]'

# (W1-gi) default --export in the fresh cell stays strictly READ-ONLY: no .gitignore written.
w1gi="$tmp/w1gi"; mkrepo "$w1gi"
run "$w1gi" --export >/dev/null 2>&1
assert "0057 export: read-only — no .gitignore seeded" '[ ! -e "$w1gi/.gitignore" ]'
```

(Use the same `mkrepo`/`run` fixtures the surrounding bootstrap tests already define. If `mkrepo` leaves a repo without an initial commit, adjust `head_before` handling to match — the assertion only needs to prove no NEW commit was made by bootstrap.)

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_docket_config.sh`
Expected: FAIL — `0057 bootstrap: block seeded in primary tree` (bootstrap currently writes no `.gitignore`).

- [ ] **Step 3: Wire `scripts/docket-config.sh`**

Add a self dir + source the lib near the top (after `set -uo pipefail`):

```bash
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
. "$SELF_DIR/lib/docket-gitignore-block.sh"
```

Extend the `--bootstrap` write path (the `if [ "$DO_BOOTSTRAP" -eq 1 ] && [ "$BOOTSTRAP" = CREATE_ORPHAN ]` branch, ~line 242) to seed the block after `create_orphan`:

```bash
  if [ "$DO_BOOTSTRAP" -eq 1 ] && [ "$BOOTSTRAP" = CREATE_ORPHAN ]; then
    create_orphan
    # Seed the managed .gitignore block in the primary tree (closes the fresh-repo gap). We do
    # NOT auto-commit — bootstrap runs inside a skill's startup, and committing to the user's
    # integration branch from a config script crosses a write-scope line docket holds. --export
    # stays strictly read-only (this branch only runs under --bootstrap).
    ensure_docket_gitignore_block "$REPO_DIR"
    printf 'docket-config: seeded the managed .gitignore block in %s/.gitignore — COMMIT THIS so the .docket/ worktree and other docket-owned files stay untracked.\n' "$REPO_DIR" >&2
    BOOTSTRAP=PROCEED   # (unchanged) re-report PROCEED after the orphan exists
  fi
```

(Keep whatever `BOOTSTRAP=PROCEED` re-report the current code already does — only ADD the two ensure/printf lines. Verify the existing branch's post-`create_orphan` verdict handling is preserved.)

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_docket_config.sh`
Expected: PASS — `ALL PASS`.

- [ ] **Step 5: Commit**

```bash
git add scripts/docket-config.sh tests/test_docket_config.sh
git commit -m "feat(0057): docket-config --bootstrap seeds the managed block (fresh-repo gap)"
```

---

### Task 5: Prose sweep — README, convention, migrate header, docket-config contract

**Files:**
- Modify: `README.md` (marker-name references at lines ~149, ~300, ~306)
- Modify: `skills/docket-convention/SKILL.md` (marker-name references at lines ~137, ~169; the `.gitignore`-block passage in the Agent-layer text)
- Modify: `migrate-to-docket.sh` (header/step comments — already partly done in Task 3; ensure no stale "three bare lines" description remains)
- Modify: `scripts/docket-config.md` (the `--bootstrap` section gains the seed-and-notice behavior; restate the `--export` read-only guarantee untouched)
- Test: `tests/test_sync_agents.sh` doc assertions (already re-pointed in Task 2) + a positive-anchor doc grep

**Interfaces:**
- Consumes: nothing.
- Produces: docs describe the single `# docket` block, its broadened ownership (three writers), and the bootstrap seed. No `docket:generated` reference remains in the live product docs (historical change/spec/plan/results/ADR records are left as-is).

- [ ] **Step 1: Update the doc assertion (positive anchor, LEARNINGS #36)**

In `tests/test_sync_agents.sh`, replace the `0051 doc:` README/convention assertions that grep for `docket:generated` with new-marker positive anchors, e.g.:

```bash
assert "0057 doc: README documents the managed docket .gitignore block" 'grep -qF "# docket:start" "$READMEF" || grep -qE "managed .docket. block" "$READMEF"'
assert "0057 doc: README no longer names the legacy docket:generated block" '! grep -qF "docket:generated" "$READMEF"'
assert "0057 doc: convention documents the managed docket block (new marker)" 'grep -qF "# docket:start" "$CONV" || grep -qi "managed docket .gitignore block" "$CONV"'
assert "0057 doc: convention no longer names docket:generated" '! grep -qF "docket:generated" "$CONV"'
```

(Confirm the `READMEF`/`CONV` path vars already exist in the test; they are used by the current `0051 doc:` / `0050 doc:` assertions.)

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_sync_agents.sh`
Expected: FAIL — `0057 doc: ... no longer names docket:generated` (the live docs still say `docket:generated`).

- [ ] **Step 3: Edit the prose**

- `README.md` line ~149: `... kept out of git by a managed \`# docket\` block the script owns ...`
- `README.md` line ~300: `... owns a marker-bounded \`# docket\` block in the repo's \`.gitignore\`, covering every docket-owned file — the \`.docket/\` worktree, \`.worktrees/\`, \`.claude/settings.local.json\`, \`.docket.local.yml\`, and every generated agent file for every harness ...` (broaden the description to the consolidated contents + note it is seeded at migrate/bootstrap and self-healed by `sync-agents.sh`).
- `README.md` line ~306: `- The \`.gitignore\` \`# docket\` block is present and current, **and** no per-repo generated file is tracked by git ...`
- `skills/docket-convention/SKILL.md` line ~137: `... the managed \`# docket\` \`.gitignore\` block ...`
- `skills/docket-convention/SKILL.md` line ~169: `\`# docket\` block in the repo's \`.gitignore\`, covering every docket-owned pattern ...` — and, where the Agent-layer text attributes the block solely to `sync-agents.sh`, note it is now the single home for all docket-owned ignores, seeded by `migrate-to-docket.sh` / `docket-config.sh --bootstrap` and self-healed by `sync-agents.sh` (change 0057). Keep this edit minimal (PR #62 overlap).
- `migrate-to-docket.sh`: confirm the Task-3 header/step comments no longer describe "three bare lines" as the end state.
- `scripts/docket-config.md`: in the `--bootstrap` section, add that on `CREATE_ORPHAN` it also seeds the managed `.gitignore` block in the primary tree and prints a COMMIT notice (no auto-commit); restate that `--export` remains strictly read-only.

- [ ] **Step 4: Run tests to verify they pass**

Run: `bash tests/test_sync_agents.sh`
Expected: PASS. Then grep-confirm no live-doc `docket:generated` remains:

Run: `grep -rln 'docket:generated' README.md skills/docket-convention/SKILL.md scripts/docket-config.md migrate-to-docket.sh sync-agents.sh scripts/lib/docket-gitignore-block.sh`
Expected: no matches (exit 1 / empty).

- [ ] **Step 5: Commit**

```bash
git add README.md skills/docket-convention/SKILL.md scripts/docket-config.md migrate-to-docket.sh
git commit -m "docs(0057): rename docket:generated -> docket block; document consolidation"
```

---

### Task 6: Whole-suite verification

**Files:** none (verification only).

- [ ] **Step 1: Run the full suite foreground (one command)**

Run:
```bash
fail=0; for t in tests/test_*.sh; do echo "== $t"; bash "$t" >/tmp/57_$$.log 2>&1 || { echo "FAILED: $t"; tail -20 /tmp/57_$$.log; fail=1; }; done; echo "SUITE fail=$fail"
```
Expected: `SUITE fail=0`. Investigate any red test by intent (LEARNINGS #34: if a `${VAR:?}` test false-REDs because the dev shell exports `DOCKET_SCRIPTS_DIR`, re-run under `env -u DOCKET_SCRIPTS_DIR`).

- [ ] **Step 2: Run `sync-agents.sh --check` against this repo**

Run: `bash sync-agents.sh --check; echo "rc=$?"`
Expected: rc reflects this repo's own opt-in state; if this repo carries a legacy block, `--check` flags it stale (remedy: run `sync-agents.sh`) — that is the designed upgrade path, resolve before PR.

- [ ] **Step 3: No commit** (verification task; fixes, if any, fold back into the owning task's commit).

---

## Self-Review

**Spec coverage:**
- Shared lib (roster move, markers new+legacy, constant emitter, hardened ensure w/ closed-block guard both spellings, legacy upgrade, outside-bytes invariant, dedup advisory) → Task 1.
- Three writers (migrate step 5, docket-config --bootstrap, sync-agents widened trigger) → Tasks 3, 4, 2.
- `--check` leg (a) against new markers + upgrade path → Task 2.
- Prose/contract sweep + ADR posture → Task 5 (ADR-0020 `## Update` is a Step-6 metadata edit, flagged in Global Constraints — not a feature-branch task).
- Tests (constant-emitter equivalence, legacy upgrade, dangling both spellings, widened trigger ±, migrate end-state, bootstrap seed, dedup advisory, existing 0051 tests updated) → Tasks 1–5.
- Out-of-scope items (ADR-0020 generation semantics, `ensure-claude-settings.sh`, main-mode tracking-only gap, `link-skills.sh` divergence) → untouched by design.

**Placeholder scan:** every code/test step carries concrete content; no TBD/TODO. The one deliberate manual step (Task 3 Step 4 migrate smoke) is explicit about commands + fallback.

**Type consistency:** function/constant names are identical across tasks — `emit_docket_gitignore_block`, `ensure_docket_gitignore_block`, `_docket_gi_current_block`, `DOCKET_GI_START`/`_END`/`_LEGACY_START`/`_LEGACY_END`/`_HARNESS_TOKENS`/`_DISPATCH_HARNESSES`/`_CORE_ENTRIES` — defined in Task 1, consumed verbatim in Tasks 2–4.
