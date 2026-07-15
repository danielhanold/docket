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
  printf '.codex/agents/docket-*.toml\n'
  for tok in $DOCKET_GI_DISPATCH_HARNESSES; do printf '.%s/rules/docket-dispatch.mdc\n' "$tok"; done
  printf '%s\n' "$DOCKET_GI_END"
}

# Return 0 (MALFORMED -> caller must refuse) if the START/END markers in FILE are not a clean,
# ordered set of non-overlapping pairs: dangling start, dangling end, end-before-start, or nested
# start all count as malformed. Return 1 (well-formed: zero or more properly ordered, non-
# overlapping pairs). String-exact marker match (same as grep -F -x); safe under pipefail.
_docket_gi_malformed(){  # $1=file $2=start $3=end
  [ -f "$1" ] || return 1
  awk -v s="$2" -v e="$3" '
    $0==s { if (inb) bad=1; inb=1; next }
    $0==e { if (!inb) bad=1; else inb=0; next }
    END   { if (bad || inb) exit 0; exit 1 }
  ' "$1"
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
  local gi="$1/.gitignore" want have rest legacy_present=0
  want="$(emit_docket_gitignore_block)"

  # (1) Closed-block guard — dangling/malformed/out-of-order markers of EITHER spelling: refuse,
  # warn, touch nothing. Order-aware (not just presence) so an END sitting above its START (e.g.
  # a hand-corrupted file) is caught here instead of letting the awk range below consume to EOF.
  if _docket_gi_malformed "$gi" "$DOCKET_GI_START" "$DOCKET_GI_END"; then
    _docket_gi_log "WARN $gi has an UNTERMINATED or malformed docket block (dangling/out-of-order start or end) — corrupt; refusing to rewrite so no bytes are lost. Repair or remove the '$DOCKET_GI_START' / '$DOCKET_GI_END' lines by hand, then re-run."
    return 0
  fi
  if _docket_gi_malformed "$gi" "$DOCKET_GI_LEGACY_START" "$DOCKET_GI_LEGACY_END"; then
    _docket_gi_log "WARN $gi has an UNTERMINATED or malformed legacy docket:generated block (dangling/out-of-order start or end) — corrupt; refusing to rewrite so no bytes are lost. Repair or remove the '$DOCKET_GI_LEGACY_START' / '$DOCKET_GI_LEGACY_END' lines by hand, then re-run."
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
