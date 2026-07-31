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
