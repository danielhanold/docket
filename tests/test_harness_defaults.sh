#!/usr/bin/env bash
# tests/test_harness_defaults.sh — run: bash tests/test_harness_defaults.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

. "$REPO/scripts/lib/harness-defaults.sh"
HD="$REPO/agents/harness-defaults.yml"
SRC="$REPO/agents"

# ---- the shipped file itself ------------------------------------------------
assert "sidecar exists"            '[ -f "$HD" ]'
assert "sidecar validates"         'hd_validate "$HD" "$SRC"'
assert "harnesses are exactly the four shipped ones" \
  '[ "$(hd_harnesses "$HD" | tr "\n" " ")" = "claude codex cursor opencode " ]'

# ---- every shipped Claude value, verbatim -----------------------------------
for pair in \
  "adr claude-opus-5 low" \
  "auto-groom claude-opus-5 low" \
  "auto-groom-critic claude-opus-5 medium" \
  "brainstorm-consultant claude-opus-5 medium" \
  "build-economy claude-sonnet-5 low" \
  "build-standard claude-opus-5 low" \
  "build-premium claude-opus-5 medium" \
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
  "build-economy cursor-grok-4.5-low" \
  "build-standard cursor-grok-4.5-medium" \
  "build-premium cursor-grok-4.5-high" \
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
  "build-economy gpt-5.6-luna xhigh" \
  "build-standard gpt-5.6-terra medium" \
  "build-premium gpt-5.6-sol low" \
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
  '[ "$(hd_field "$HD" codex build-economy model)/$(hd_field "$HD" codex build-economy effort)" = "gpt-5.6-luna/xhigh" ] &&
   [ "$(hd_field "$HD" codex build-standard model)/$(hd_field "$HD" codex build-standard effort)" = "gpt-5.6-terra/medium" ] &&
   [ "$(hd_field "$HD" codex build-premium model)/$(hd_field "$HD" codex build-premium effort)" = "gpt-5.6-sol/low" ] &&
   [ "$(hd_field "$HD" codex build-max model)/$(hd_field "$HD" codex build-max effort)" = "gpt-5.6-sol/medium" ]'

# ---- every shipped opencode value, verbatim ---------------------------------
for pair in \
  "adr openrouter/moonshotai/kimi-k3 medium" \
  "auto-groom openrouter/deepseek/deepseek-v4-flash-0731 medium" \
  "auto-groom-critic openrouter/openai/gpt-5.6-luna high" \
  "brainstorm-consultant openrouter/moonshotai/kimi-k3 medium" \
  "build-economy openrouter/deepseek/deepseek-v4-flash-0731 medium" \
  "build-standard openrouter/deepseek/deepseek-v4-flash-0731 high" \
  "build-premium openrouter/moonshotai/kimi-k3 medium" \
  "build-max openrouter/moonshotai/kimi-k3 high" \
  "finalize-change openrouter/deepseek/deepseek-v4-flash-0731 high" \
  "implement-next openrouter/deepseek/deepseek-v4-flash-0731 high" \
  "integration-repair openrouter/moonshotai/kimi-k3 high" \
  "rebase-resolver openrouter/moonshotai/kimi-k3 high" \
  "review-lean openrouter/deepseek/deepseek-v4-flash-0731 high" \
  "review-standard openrouter/moonshotai/kimi-k3 medium" \
  "review-deep openrouter/moonshotai/kimi-k3 high" \
  "status openrouter/deepseek/deepseek-v4-flash-0731 low" \
; do
  set -- $pair
  assert "opencode/$1 model is $2"  '[ "$(hd_field "$HD" opencode '"$1"' model)" = "'"$2"'" ]'
  assert "opencode/$1 effort is $3" '[ "$(hd_field "$HD" opencode '"$1"' effort)" = "'"$3"'" ]'
done

# The build ladder, stated as the pairs it actually is. Flash carries the two volume rungs at
# different efforts and Kimi carries the two judgment rungs at different efforts — the pair is the
# role, not the model.
assert "opencode build ladder: economy is Flash/medium" \
  '[ "$(hd_field "$HD" opencode build-economy model)" = "openrouter/deepseek/deepseek-v4-flash-0731" ] && [ "$(hd_field "$HD" opencode build-economy effort)" = "medium" ]'
assert "opencode build ladder: standard is Flash/high" \
  '[ "$(hd_field "$HD" opencode build-standard model)" = "openrouter/deepseek/deepseek-v4-flash-0731" ] && [ "$(hd_field "$HD" opencode build-standard effort)" = "high" ]'
assert "opencode build ladder: premium is Kimi/medium" \
  '[ "$(hd_field "$HD" opencode build-premium model)" = "openrouter/moonshotai/kimi-k3" ] && [ "$(hd_field "$HD" opencode build-premium effort)" = "medium" ]'
assert "opencode build ladder: max is Kimi/high" \
  '[ "$(hd_field "$HD" opencode build-max model)" = "openrouter/moonshotai/kimi-k3" ] && [ "$(hd_field "$HD" opencode build-max effort)" = "high" ]'

# The slash-bearing ID is the first double-prefixed value any block ships. Pin that the bare-scalar
# reader returns it WHOLE rather than clipping at the slash — hd_field's value class is
# [^,}[:space:]]+, so this is the assert that would catch a future narrowing to an allowlist.
assert "opencode: a double-prefixed ID survives the bare-scalar read intact" \
  '[ "$(hd_field "$HD" opencode status model)" = "$(hd_field_raw "$HD" opencode status model)" ] && case "$(hd_field "$HD" opencode status model)" in */*/*) true;; *) false;; esac'

# Detect the REMOVED state, not the added one (a grep for the new names is green the moment the
# edit lands and stays green even if an old row is left behind alongside it). Change 0184 retired
# low/medium/high as profile names — the intermediate vocabulary it briefly carried, replaced
# before merge because it collided with the `effort:` values in this very file, where build-economy
# ships effort xhigh on codex and build-premium ships effort low. A leftover row would be silently
# resolvable by any config layer that still names it.
assert "no retired profile row survives in any block" \
  '! grep -qE "^[[:space:]]*build-(low|medium|high):" "$HD"'
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

# The remaining malformed-shape rejections live in tests/test_harness_defaults_validator.sh —
# split out by measured `hd_validate` cost (change 0227), not by topic.

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
