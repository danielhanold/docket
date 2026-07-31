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
assert "harnesses are claude+cursor only" \
  '[ "$(hd_harnesses "$HD" | tr "\n" " ")" = "claude cursor " ]'

# ---- every shipped Claude value, verbatim -----------------------------------
for pair in \
  "adr claude-opus-5 low" \
  "auto-groom claude-opus-5 low" \
  "auto-groom-critic claude-opus-5 medium" \
  "brainstorm-consultant claude-opus-5 medium" \
  "build-economy claude-opus-5 low" \
  "build-standard claude-opus-5 medium" \
  "build-premium claude-opus-5 high" \
  "finalize-change claude-opus-5 low" \
  "implement-next claude-opus-5 medium" \
  "integration-repair claude-opus-5 medium" \
  "rebase-resolver claude-opus-5 medium" \
  "status claude-haiku-4-5-20251001 medium" ; do
  set -- $pair
  assert "claude/$1 = $2/$3" \
    '[ "$(hd_field "$HD" claude "'"$1"'" model)/$(hd_field "$HD" claude "'"$1"'" effort)" = "'"$2"'/'"$3"'" ]'
done

# ---- the three Cursor build profiles ----------------------------------------
assert "cursor/build-economy = cursor-grok-4.5-medium/auto" \
  '[ "$(hd_field "$HD" cursor build-economy model)/$(hd_field "$HD" cursor build-economy effort)" = "cursor-grok-4.5-medium/auto" ]'
assert "cursor/build-standard = cursor-grok-4.5-high/auto" \
  '[ "$(hd_field "$HD" cursor build-standard model)/$(hd_field "$HD" cursor build-standard effort)" = "cursor-grok-4.5-high/auto" ]'
assert "cursor/build-premium = claude-opus-5-high/auto" \
  '[ "$(hd_field "$HD" cursor build-premium model)/$(hd_field "$HD" cursor build-premium effort)" = "claude-opus-5-high/auto" ]'
assert "cursor block is exactly the three build workers" \
  '[ "$(hd_agents "$HD" cursor | tr "\n" " ")" = "build-economy build-standard build-premium " ]'
assert "no codex block yet (change 0169 owns it)" '[ -z "$(hd_agents "$HD" codex)" ]'
assert "unlisted pair resolves empty" '[ -z "$(hd_field "$HD" cursor status model)" ]'

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
# reverse: every cursor entry is a build worker
while IFS= read -r a; do
  [ -n "$a" ] || continue
  assert "cursor/$a is a build worker" '[ -f "$SRC/docket-'"$a"'.md" ] && case "'"$a"'" in build-*) true;; *) false;; esac'
done < <(hd_agents "$HD" cursor)

# ---- validator rejects each malformed shape ---------------------------------
T="$(mktemp -d)"
mut(){ cp "$HD" "$T/hd.yml"; }

mut; sed -i.bak '/^    status:/d' "$T/hd.yml"
assert "reject: missing a claude entry" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

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

mut; sed -i.bak 's|^    build-economy:.*cursor-grok-4.5-medium.*|    build-economy:         { model: , effort: auto }|' "$T/hd.yml"
assert "reject: empty field value" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

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

[ "$fail" = 0 ] && echo "PASS" || echo "FAIL"
exit "$fail"
