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
assert "emit: codex TOML wrapper pattern"   'printf "%s\n" "$BLK" | grep -qxF ".codex/agents/docket-*.toml"'
assert "emit: codex .md pattern still present (constant, all tokens)" 'printf "%s\n" "$BLK" | grep -qxF ".codex/agents/docket-*.md"'
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

# --- ensure: out-of-order markers (END above START, same spelling) -> refuse, warn, byte-identical ---
SBX="$(mktemp -d)"
{ printf '# docket:end\nkeepme-A/\n# docket:start (managed by docket — do not hand-edit)\n.docket/\nkeepme-B/\n'; } > "$SBX/.gitignore"
before="$(cat "$SBX/.gitignore")"
oo_err="$( { ensure_docket_gitignore_block "$SBX"; } 2>&1 >/dev/null )"; oo_rc=$?
assert "out-of-order: run returns 0"                 '[ "$oo_rc" = "0" ]'
assert "out-of-order: warns corrupt/unterminated"    'printf "%s" "$oo_err" | grep -qiE "untermin|corrupt|malformed|out.of.order"'
assert "out-of-order: file byte-identical (no data loss)" '[ "$before" = "$(cat "$SBX/.gitignore")" ]'
assert "out-of-order: keepme-B/ survived"            'grep -qxF "keepme-B/" "$SBX/.gitignore"'
rm -rf "$SBX"
# same for the LEGACY spelling
SBX="$(mktemp -d)"
{ printf '# docket:generated:end\nkeepme-A/\n# docket:generated:start (managed by sync-agents.sh — do not hand-edit)\n.docket.local.yml\nkeepme-B/\n'; } > "$SBX/.gitignore"
before="$(cat "$SBX/.gitignore")"
ool_err="$( { ensure_docket_gitignore_block "$SBX"; } 2>&1 >/dev/null )"; ool_rc=$?
assert "out-of-order legacy: run returns 0"          '[ "$ool_rc" = "0" ]'
assert "out-of-order legacy: warns"                  'printf "%s" "$ool_err" | grep -qiE "untermin|corrupt|malformed|out.of.order"'
assert "out-of-order legacy: byte-identical"         '[ "$before" = "$(cat "$SBX/.gitignore")" ]'
rm -rf "$SBX"

# --- ensure: dedup advisory for bare literals OUTSIDE the block ---------------
SBX="$(mktemp -d)"
printf '.docket/\n.worktrees\n' > "$SBX/.gitignore"   # pre-existing bare entries (note: no trailing slash on second)
adv_err="$( { ensure_docket_gitignore_block "$SBX"; } 2>&1 >/dev/null )"
assert "dedup: advisory logged for outside bare entries" 'printf "%s" "$adv_err" | grep -qi "safe to delete"'
assert "dedup: outside bare .docket/ NOT deleted"        'grep -qxF ".docket/" <(awk "/# docket:start/{f=1} !f{print} /# docket:end/{f=0}" "$SBX/.gitignore")'
assert "dedup: block still written"                      'grep -qxF "# docket:end" "$SBX/.gitignore"'
rm -rf "$SBX"

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

[ "$fail" = 0 ] && echo "ALL PASS" || echo "SOME FAILED"
exit "$fail"
