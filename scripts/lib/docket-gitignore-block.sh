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
