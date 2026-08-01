#!/usr/bin/env bash
# tests/test_harness_defaults.sh — run: bash tests/test_harness_defaults.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

. "$REPO/scripts/lib/harness-defaults.sh"
HD="$REPO/agents/harness-defaults.yml"
SRC="$REPO/agents"

# ---- the shipped file itself ------------------------------------------------
assert "sidecar exists"            '[ -f "$HD" ]'
assert "sidecar validates"         'hd_validate "$HD" "$SRC"'
assert "harnesses are exactly the three shipped ones" \
  '[ "$(hd_harnesses "$HD" | tr "\n" " ")" = "claude codex cursor " ]'

# ---- every shipped Claude value, verbatim -----------------------------------
for pair in \
  "adr claude-opus-5 low" \
  "auto-groom claude-opus-5 low" \
  "auto-groom-critic claude-opus-5 medium" \
  "brainstorm-consultant claude-opus-5 medium" \
  "build-low claude-sonnet-5 low" \
  "build-medium claude-opus-5 low" \
  "build-high claude-opus-5 medium" \
  "build-max claude-opus-5 high" \
  "finalize-change claude-opus-5 low" \
  "implement-next claude-opus-5 medium" \
  "integration-repair claude-opus-5 medium" \
  "rebase-resolver claude-opus-5 medium" \
  "status claude-haiku-4-5-20251001 medium" ; do
  set -- $pair
  assert "claude/$1 = $2/$3" \
    '[ "$(hd_field "$HD" claude "'"$1"'" model)/$(hd_field "$HD" claude "'"$1"'" effort)" = "'"$2"'/'"$3"'" ]'
done

# ---- the Cursor block: complete, like claude ---------------------------------
# Every ID is a complete Cursor built-in with its variant encoded, so every effort is `auto`.
for pair in \
  "adr cursor-grok-4.5-high" \
  "auto-groom cursor-grok-4.5-medium" \
  "auto-groom-critic cursor-grok-4.5-high" \
  "brainstorm-consultant cursor-grok-4.5-high" \
  "build-low cursor-grok-4.5-low" \
  "build-medium cursor-grok-4.5-medium" \
  "build-high cursor-grok-4.5-high" \
  "build-max claude-opus-5-high" \
  "finalize-change cursor-grok-4.5-high-fast" \
  "implement-next cursor-grok-4.5-high" \
  "integration-repair cursor-grok-4.5-high" \
  "rebase-resolver cursor-grok-4.5-high" \
  "status cursor-grok-4.5-low-fast" ; do
  set -- $pair
  assert "cursor/$1 = $2/auto" \
    '[ "$(hd_field "$HD" cursor "'"$1"'" model)/$(hd_field "$HD" cursor "'"$1"'" effort)" = "'"$2"'/auto" ]'
done
# ---- the Codex block: complete, with per-agent efforts ----------------------
# Unlike cursor (whose IDs encode their variant, so every effort is `auto`), Codex takes a real
# reasoning-effort token per agent, so both fields are asserted per row.
for triple in \
  "adr gpt-5.6-terra xhigh" \
  "auto-groom gpt-5.6-sol low" \
  "auto-groom-critic gpt-5.6-sol medium" \
  "brainstorm-consultant gpt-5.6-sol medium" \
  "build-low gpt-5.6-luna xhigh" \
  "build-medium gpt-5.6-terra medium" \
  "build-high gpt-5.6-sol low" \
  "build-max gpt-5.6-sol medium" \
  "finalize-change gpt-5.6-terra high" \
  "implement-next gpt-5.6-sol medium" \
  "integration-repair gpt-5.6-sol high" \
  "rebase-resolver gpt-5.6-sol high" \
  "status gpt-5.6-luna xhigh" ; do
  set -- $triple
  assert "codex/$1 = $2/$3" \
    '[ "$(hd_field "$HD" codex "'"$1"'" model)/$(hd_field "$HD" codex "'"$1"'" effort)" = "'"$2"'/'"$3"'" ]'
done
# The four build profiles are the settled ladder for this change, asserted separately from the
# loop above so a reader sees the claim the change is actually making. Note that the codex ladder
# is NOT model-monotonic: model/effort PAIRS are model-specific roles, not cross-model ordinals,
# so sol appears at two different efforts and that is deliberate.
assert "codex build ladder = luna/xhigh, terra/medium, sol/low, sol/medium" \
  '[ "$(hd_field "$HD" codex build-low model)/$(hd_field "$HD" codex build-low effort)" = "gpt-5.6-luna/xhigh" ] &&
   [ "$(hd_field "$HD" codex build-medium model)/$(hd_field "$HD" codex build-medium effort)" = "gpt-5.6-terra/medium" ] &&
   [ "$(hd_field "$HD" codex build-high model)/$(hd_field "$HD" codex build-high effort)" = "gpt-5.6-sol/low" ] &&
   [ "$(hd_field "$HD" codex build-max model)/$(hd_field "$HD" codex build-max effort)" = "gpt-5.6-sol/medium" ]'

# Detect the REMOVED state, not the added one (a grep for the new names is green the moment the
# edit lands and stays green even if an old row is left behind alongside it). Change 0184 retired
# economy/standard/premium as profile names; a leftover row would be silently resolvable by any
# config layer that still names it.
assert "no retired profile row survives in any block" \
  '! grep -qE "^[[:space:]]*build-(economy|standard|premium):" "$HD"'
# Sparse-by-harness is still a live property of the reader — it just no longer has a shipped
# harness to demonstrate it on. Narrowed (not deleted) to a token that genuinely holds no block:
# what this guards is that hd_field returns EMPTY for an absent harness rather than falling through
# to another block's row.
assert "a harness with no block resolves empty (sparse-by-harness read)" \
  '[ -z "$(hd_field "$HD" windsurf status model)" ] && [ -z "$(hd_agents "$HD" windsurf)" ]'

# ---- set correspondence, BOTH directions ------------------------------------
# forward: every claude entry names a real source wrapper
while IFS= read -r a; do
  [ -n "$a" ] || continue
  assert "claude/$a has a source wrapper" '[ -f "$SRC/docket-'"$a"'.md" ]'
done < <(hd_agents "$HD" claude)
# reverse: every source wrapper has a claude entry (anchored on the real glob, not a list)
for f in "$SRC"/docket-*.md; do
  n="$(basename "$f" .md)"; n="${n#docket-}"
  assert "source $n has a claude entry" '[ -n "$(hd_field "$HD" claude "'"$n"'" model)" ]'
done
# forward: every cursor entry names a real source wrapper
while IFS= read -r a; do
  [ -n "$a" ] || continue
  assert "cursor/$a has a source wrapper" '[ -f "$SRC/docket-'"$a"'.md" ]'
done < <(hd_agents "$HD" cursor)
# reverse: every source wrapper has a cursor entry — the same completeness claude carries, so a
# thirteenth wrapper cannot land pinned on Claude and unpinned on Cursor
for f in "$SRC"/docket-*.md; do
  n="$(basename "$f" .md)"; n="${n#docket-}"
  assert "source $n has a cursor entry" '[ -n "$(hd_field "$HD" cursor "'"$n"'" model)" ]'
done

# ---- validator rejects each malformed shape ---------------------------------
T="$(mktemp -d)"
mut(){ cp "$HD" "$T/hd.yml"; }

# Completeness is enforced for EVERY shipped harness, so the mutation is derived from
# HD_SHIPPED_HARNESSES rather than written once per harness with a value-specific pattern — adding
# a fourth shipped harness arms this loop for free. Deleting the row from ONE block only is the
# point: a bare `/^    status:/d` would delete it from all three at once and go green whichever leg
# actually fired, unable to tell a working per-harness rule from one that was never written.
del_entry(){ # $1=harness $2=agent -> writes $T/hd.yml
  awk -v h="$1" -v a="$2" '
    { nc=$0; sub(/#.*/,"",nc) }
    nc ~ "^  "h"[[:space:]]*:[[:space:]]*$" { inb=1; print; next }
    inb && nc ~ /^  [A-Za-z0-9._-]+[[:space:]]*:/ { inb=0 }
    inb && nc ~ "^    "a"[[:space:]]*:" { next }
    { print }
  ' "$HD" > "$T/hd.yml"
}
for h in $HD_SHIPPED_HARNESSES; do
  del_entry "$h" status
  # Non-vacuity: prove the mutation actually landed on THIS block and left the others intact.
  # Without this, a del_entry that silently matched nothing would leave every assert below green.
  assert "mutation landed: $h lost exactly one entry" \
    '[ "$(hd_agents "$T/hd.yml" "'"$h"'" | grep -c .)" = "$(( $(hd_agents "$HD" "'"$h"'" | grep -c .) - 1 ))" ]'
  assert "reject: missing a $h entry" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'
  gap_diag="$(hd_validate "$T/hd.yml" "$SRC" 2>&1 || true)"
  assert "reject: the $h gap is reported against $h" \
    'grep -q "'"$h"' block is incomplete — no entry for .status." <<<"$gap_diag"'
done

mut; printf '    phantom:               { model: x, effort: low }\n' >> "$T/hd.yml"
assert "reject: phantom agent key" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

mut; printf '\n  default:\n    adr:                   { model: x, effort: low }\n' >> "$T/hd.yml"
assert "reject: harness-neutral default block" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

mut; printf '\n  bogus:\n    adr:                   { model: x, effort: low }\n' >> "$T/hd.yml"
assert "reject: unknown harness key" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: claude-opus-5 }|' "$T/hd.yml"
assert "reject: entry missing effort" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: claude-opus-5, effort: low, runner: codex }|' "$T/hd.yml"
assert "reject: runner is forbidden" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

mut; sed -i.bak 's|^    build-medium:.*cursor-grok-4.5-medium.*|    build-medium:          { model: , effort: auto }|' "$T/hd.yml"
assert "reject: empty field value" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

# 0168 whole-branch review, IMPORTANT 5: a SECOND block for a harness that already has one. This
# was dead code — the guard counted against hd_harnesses(), which ends in `sort -u`, so its count
# could never exceed 1 and a duplicate block validated clean.
# The two blocks below share NO agent key, which is what makes this fixture load-bearing: a
# duplicate that DOES repeat a key is already caught by the duplicate-entry guard further down,
# so a repeating fixture would go green against the dead guard and prove nothing.
mut; printf '\n  codex:\n    adr:                   { model: x, effort: low }\n\n  codex:\n    status:                { model: y, effort: low }\n' >> "$T/hd.yml"
assert "reject: duplicate harness block" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'
dup_diag="$(hd_validate "$T/hd.yml" "$SRC" 2>&1 || true)"
assert "reject: duplicate harness block names the harness" \
  'grep -q "duplicate harness block .codex." <<<"$dup_diag"'

# 0168 whole-branch review, IMPORTANT 6: a provider-prefixed model ID. The value class used to be
# [A-Za-z0-9._-]+, which stops at the '/' — hd_field returned "anthropic", hd_validate saw a
# non-empty value and passed, and a WRONG pin got generated. ADR-0015 makes model IDs opaque
# passthrough with no allowlist, so the reader must consume the whole token.
mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: anthropic/claude-opus-5, effort: low }|' "$T/hd.yml"
assert "reader: a '/'-bearing model ID is read WHOLE" \
  '[ "$(hd_field "$T/hd.yml" claude adr model)" = "anthropic/claude-opus-5" ]'
assert "accept: a '/'-bearing model ID validates" 'hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: openai:gpt-5.6-sol, effort: low }|' "$T/hd.yml"
assert "reader: a ':'-bearing model ID is read WHOLE" \
  '[ "$(hd_field "$T/hd.yml" claude adr model)" = "openai:gpt-5.6-sol" ]'
assert "accept: a ':'-bearing model ID validates" 'hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

# 0168 whole-branch review, MINOR 2: a QUOTED scalar. The reader consumes bare tokens only, so a
# quoted value is truncated. Rejecting it with a diagnostic that names the real problem beats the
# misleading "missing a non-empty 'model'" the completeness check would otherwise emit.
mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: "claude opus 5", effort: low }|' "$T/hd.yml"
assert "reject: quoted (non-bare) scalar" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'
q_diag="$(hd_validate "$T/hd.yml" "$SRC" 2>&1 || true)"
assert "reject: quoted scalar diagnostic names bareness, not absence" \
  'grep -q "bare" <<<"$q_diag" && ! grep -q "missing a non-empty .model." <<<"$q_diag"'

assert "reject: missing file" '! hd_validate "$T/nope.yml" "$SRC" 2>/dev/null'

# ---- the readers are IFS-independent (this library is SOURCED) --------------
# sync-agents.sh sources this file, so it inherits the caller's IFS. Under IFS="" the validator's
# field-name loop stops word-splitting: the pristine sidecar fails validation AND the `runner`
# guard goes dead while still returning 1, i.e. it reddens for the wrong reason. Assert both the
# exit code and the diagnostic TEXT — the code alone cannot tell those two apart.
assert "pristine sidecar validates under a clobbered IFS" \
  '( IFS=""; hd_validate "$HD" "$SRC" >/dev/null 2>&1 )'
mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: claude-opus-5, effort: low, runner: codex }|' "$T/hd.yml"
diag="$( IFS=""; hd_validate "$T/hd.yml" "$SRC" 2>&1 || true )"
assert "runner guard names 'runner' under a clobbered IFS" \
  'grep -q "delegation is user policy" <<<"$diag"'
rm -rf "$T"

# ---- the sources are behavior-only templates --------------------------------
# Anchored to the first frontmatter block: these files' BODIES legitimately discuss model/effort.
# awk note: a bare `exit 0` in a rule still runs END, so the found-flag is load-bearing — an
# END that unconditionally exits 1 would make every `! fm_has` assert vacuously green.
fm_has(){ # $1=file $2=key -> 0 if the key appears in the first --- block
  awk -v k="$2" '
    /^---[[:space:]]*$/ { d++; if (d>=2) exit; next }
    d==1 && $0 ~ "^"k"[[:space:]]*:" { found=1; exit }
    END { exit(found?0:1) }' "$1"
}
for f in "$SRC"/docket-*.md; do
  n="$(basename "$f")"
  assert "$n: no model: in frontmatter (sidecar owns it)"  '! fm_has "'"$f"'" model'
  assert "$n: no effort: in frontmatter (sidecar owns it)" '! fm_has "'"$f"'" effort'
  assert "$n: still declares name:"                        'fm_has "'"$f"'" name'
done

[ "$fail" = 0 ] && echo "PASS" || echo "FAIL"
exit "$fail"
