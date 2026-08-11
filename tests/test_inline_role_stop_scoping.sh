#!/usr/bin/env bash
# tests/test_inline_role_stop_scoping.sh — change 0212. A docket-owned skill body that can be LOADED
# INTO A CALLER'S CONTEXT must scope its terminal stops and its second-person prohibitions to the
# role, because "you" in an inlined body resolves to the caller. On 2026-08-05 a docket-implement-next
# run read docket-build's "Then you stop — review is not yours." as its own terminal boundary and
# ended at the Step 5/6 boundary with no review and no PR.
#
# POSITIVE-PRESENCE guard, deliberately not a negative vocabulary grep: a grep forbidding an
# unqualified "you stop" is line- and vocabulary-scoped and escapes by paraphrase (the header of
# tests/test_role_skill_self_description.sh documents that limitation of its own negative form).
#
# PROXIMITY-SCOPED: presence of the clause anywhere in the file is NOT presence of it at the stop
# (change 0199's co-occurrence lesson). Each site below is a (file, anchor) pair, and the clause must
# appear within WINDOW lines AFTER the anchor line.
#
# WRAP-TOLERANT: the swept bodies are hard-wrapped markdown prose, so either half of the clause can
# straddle a line break — the swept instances wrap mid-anchor (e.g. "only an agent whose entire /
# assignment is this role").
# The window is therefore whitespace-normalized before matching. Line-literal matching would have
# reddened that site for a formatting reason with nothing wrong at it.
#
# The SITES table is hand-maintained. If a swept body rewords a stop, its anchor stops matching and
# the existence assert reddens — deliberately, so the table is updated rather than guarding nothing.
# Run: bash tests/test_inline_role_stop_scoping.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if ( eval "$2" ); then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# Both halves of the two-sided clause. The discriminator is HOW THIS BODY ARRIVED, not the reader's
# employment status: a docket-implement-next fork reading docket-build inline is BOTH a dispatched
# subagent AND an inline caller, so an "inline vs dispatched" split is not mutually exclusive from
# the reader's viewpoint and the sticky second person decides it. The second person therefore sits
# on the CONTINUE branch, and the abort branch is third-person about "an agent whose entire
# assignment is this role". A wrapper injects the SAME body, so both halves stay load-bearing.
# Both anchors are lowercase and mid-sentence, because matching is case-sensitive and fixed-string.
INLINE_HALF="you invoked this skill yourself"
DISPATCH_HALF="only an agent whose entire assignment is this role"
WINDOW=6

# The line number of the first line matching a fixed-string anchor, or empty. Captured into a
# variable and sliced with parameter expansion — never `grep | head`, which under `set -o pipefail`
# SIGPIPEs the producer into an intermittent 141 (AGENTS.md).
anchor_line(){
  local hits first
  hits="$(grep -nF -- "$2" "$1" 2>/dev/null || true)"
  [ -n "$hits" ] || return 0
  first="${hits%%$'\n'*}"
  printf '%s' "${first%%:*}"
}

# The WINDOW lines at and after an anchor line, joined with whitespace runs collapsed to single
# spaces in one awk pass (no pipe, no early-exiting consumer), so a literal that straddles a hard
# wrap still matches. Every proximity assert in this file reads its window through here.
window_text(){
  awk -v lo="$2" -v hi="$(( $2 + WINDOW ))" \
    'NR>=lo && NR<=hi { s = s $0 "\n" } END { gsub(/[[:space:]]+/, " ", s); print s }' "$1"
}

# Does the window starting at the anchor line carry BOTH halves of the clause?
clause_near(){
  local file="$1" line="$2" text
  text="$(window_text "$file" "$line")"
  case "$text" in *"$INLINE_HALF"*) ;; *) return 1 ;; esac
  case "$text" in *"$DISPATCH_HALF"*) ;; *) return 1 ;; esac
  return 0
}

# SITES: "<relpath>|<verbatim anchor clause>|<what the site is>". Verbatim-quoted clauses, never line
# numbers (AGENTS.md / ADR-0054). A PROHIBITION site anchors on the LAST LINE of the last bullet of
# its block, not the first, because the clause is appended after the block — anchoring earlier spends
# the window on the bullet's own body. docket-build-task's site was re-anchored from the escalated
# bullet's FIRST line to its last for exactly that reason: the clause's second half sat at
# anchor + WINDOW, so one added line or a reflow would have reddened the site for a formatting reason
# with nothing wrong at it. Re-anchoring nearer is preferred to widening WINDOW, which would weaken
# every other site. An anchor must also carry NO BACKTICK: assert descriptions are double-quoted, so
# a backticked anchor is command-substituted rather than printed. No comment lines inside the
# heredoc: `read` would take one as a path.
# docket-build's anchor is the wrapped prefix `Then you stop — review`: the landed compression broke
# "…— review is not yours." across two lines, so the full sentence exists on no single line.
SITES="
skills/docket-build/SKILL.md|Then you stop — review|terminal stop
skills/docket-build/SKILL.md|Every halt is the same disposition|halting stop (nine conditions)
skills/docket-review/SKILL.md|One shot at the dispatched rung|second-person prohibitions
skills/docket-review/SKILL.md|An unmet precondition or a blocking ambiguity is **abort-and-report**|terminal stop
skills/docket-status/SKILL.md|stop rather than improvising a fix|hard-error stop (Tier A inline path)
skills/docket-build-task/SKILL.md|revise or replace them, but never discard them blindly|second-person prohibitions
skills/docket-build-task/SKILL.md|Return exactly one of three outcomes|terminal return
skills/docket-brainstorm/SKILL.md|STOP AT THE SPEC|terminal stop (always-inlined body)
"

while IFS='|' read -r rel anchor what; do
  [ -n "$rel" ] || continue
  # A row missing its `|` fields would leave $anchor empty, and `grep -F -- ""` matches every line —
  # the site would then pass on line 1 of the file, guarding nothing. Redden on the row instead.
  assert "SITES row is well-formed: $rel" '[ -n "$anchor" ] && [ -n "$what" ]'
  if [ -z "$anchor" ] || [ -z "$what" ]; then continue; fi
  f="$REPO/$rel"
  # Non-vacuity anchor #1: the file exists and is non-empty, or every assert below is meaningless.
  assert "swept body exists and is non-empty: $rel" '[ -s "$f" ]'
  [ -s "$f" ] || continue
  # Non-vacuity anchor #2 / population floor: the site anchor still matches. A reworded stop reddens
  # here instead of silently selecting an empty scope.
  ln="$(anchor_line "$f" "$anchor")"
  assert "$rel still carries its $what anchor" '[ -n "$ln" ]'
  [ -n "$ln" ] || continue
  # The property: the two-sided clause sits AT the site, not merely somewhere in the file.
  assert "$rel scopes its $what within $WINDOW lines" 'clause_near "$f" "$ln"'
done <<EOF
$SITES
EOF

# docket-build-task reaches its worker by WRAPPER PRELOAD (agents/docket-build-*.md carry
# `skills: [docket-build-task]`), so for this one body the inline half is readable by the worker
# itself — which would exempt it from the metadata/never-push prohibitions the clause is the only
# enforcement of. The two halves above cannot express that; a positive disambiguation must say
# preload is not self-invocation. Asserted per-site, not file-wide (the 0199 co-occurrence lesson).
PRELOAD="Wrapper preload is not self-invocation"
BT="$REPO/skills/docket-build-task/SKILL.md"
for a in "revise or replace them, but never discard them blindly" "Return exactly one of three outcomes"; do
  bt_ln="$(anchor_line "$BT" "$a")"
  assert "docket-build-task still carries its anchor: $a" '[ -n "$bt_ln" ]'
  [ -n "$bt_ln" ] || continue
  bt_win="$(window_text "$BT" "$bt_ln")"
  assert "docket-build-task disambiguates preload at: $a" \
    'case "$bt_win" in *"$PRELOAD"*) true ;; *) false ;; esac'
done

# --- Recorded no-hazard verdicts (the sweep's deliverable is a per-file verdict, not an edit set).
# Of the six swept bodies only docket-adr's verdict is still no-hazard; docket-brainstorm's was
# revised to a scoped SITES row (see below and the table above). ---
# docket-adr: no terminal stop and no second-person prohibition; the body ends on a validation
# invocation. Asserted live rather than left as a comment, so a future edit that introduces a stop
# without scoping it reddens here.
#
# The two asserts below are keyed on SYNTACTIC SHAPE, not on another skill's spellings (AGENTS.md:
# "the spelling you miss is the target file's own house idiom"). The previous form grepped
# `then you stop|your turn ends|never (writes|commits|dispatches)` — every alternative lifted
# verbatim from docket-review's third-person-singular phrasing, none of which docket-adr's
# imperative house style could ever produce. It could not fire.
#
# Shape 1 — TERMINAL STOP: the BARE (imperative / second-person) form of the stop-verb family.
# Bare form is the discriminator, not the lemma: "Then stop", "you stop", "Stop and report" all
# address the reader; the inflected third-person "it simply stops being added to"
# (present in the body today) describes the world and is correctly ignored by the word boundary.
ADR_STOP_RE='(^|[^[:alnum:]_])(stop|halt|abort|cease|quit)([^[:alnum:]_]|$)'
#
# Shape 2 — SECOND-PERSON PROHIBITION: a sentence carrying BOTH a second-person pronoun and a
# negative-deontic marker. Both are closed grammatical classes of English, not a per-file idiom
# list, so a newly-added "Never write to the integration branch" or "you must not push" is caught
# by construction. The two-part shape is what keeps docket-adr's real imperatives about the
# ARTIFACT ("Never edit an `Accepted` ADR's body", "never hand-render it") out of scope: those
# constrain how the ADR ledger is edited, they do not tell the reader what it, as an agent, may do
# — and it is only the reader-directed form that an inlining caller wrongly inherits.
ADR_2P='(^|[^[:alnum:]_])(you|your|yours|yourself)([^[:alnum:]_]|$)'
ADR_NEG='(^|[^[:alnum:]_])(never|cannot|dont|do not|does not|must not|may not|shall not|should not|can not)([^[:alnum:]_]|$)'
ADR="$REPO/skills/docket-adr/SKILL.md"
assert "docket-adr exists and is non-empty" '[ -s "$ADR" ]'
assert "docket-adr still names itself (live presence, non-vacuity)" 'grep -qF -- "docket-adr" "$ADR"'

# Sentence stream: markdown emphasis/code markers are stripped (so `do **not**` reads as `do not`),
# whitespace is collapsed, and the body is split on sentence terminators — the prohibition and the
# pronoun must co-occur in ONE SENTENCE, not merely one file (change 0199's co-occurrence lesson).
# Blank lines terminate a sentence too, so a heading or bullet cannot bleed into its neighbour.
adr_sentences(){
  awk '{ gsub(/[`*_]/, "", $0); if ($0 ~ /^[[:space:]]*$/) { print s; s=""; next }
         s = s " " $0
         while (match(s, /[.;!?]/)) { print substr(s, 1, RSTART); s = substr(s, RSTART+1) } }
       END { print s }' "$1"
}
adr_stream="$(adr_sentences "$ADR")"
# Non-vacuity: the stream must be non-empty and still carry the body's own text, or both greps
# below pass on nothing.
assert "docket-adr sentence stream is non-empty and live" \
  '[ -n "$adr_stream" ] && grep -qF -- "docket-adr maintains the project-wide" <<<"$adr_stream"'

adr_stops="$(grep -icE -e "$ADR_STOP_RE" <<<"$adr_stream" || true)"
assert "docket-adr carries no reader-directed stop (found $adr_stops)" '[ "$adr_stops" -eq 0 ]'
adr_2p_hits="$(grep -iE -e "$ADR_2P" <<<"$adr_stream" | grep -icE -e "$ADR_NEG" || true)"
assert "docket-adr carries no second-person prohibition (found $adr_2p_hits)" '[ "$adr_2p_hits" -eq 0 ]'

# docket-brainstorm: NOT a no-hazard verdict — it is a SWEPT SITE (see the SITES table above), and
# the only swept body with no `context: fork` frontmatter, so it is ALWAYS loaded into its caller's
# context (`docket-new-change` §2, `docket-groom-next`) and its own Degrade rule makes inline a
# first-class path. Its house-pattern naming half names `docket-implement-next` as the owner of
# PLANNING — a downstream skill, not the caller's next step; an inlining `docket-new-change` still
# has Steps 3–5 (scan related context, draft the change, commit/push/Board pass) after the
# brainstorm, so that naming alone did not make the stop safe. The mode-conditioned clause is what
# does, and it is asserted mechanically by the SITES row. The naming assert is KEPT, not superseded:
# it is a different property (the artifact/stop-point boundary against `writing-plans`), and it is
# the thing that would silently vanish if the stop paragraph were reworded around the new clause.
#
# PROXIMITY-SCOPED like every other site, and for the same reason: presence of the naming anywhere in
# the file is not presence of it AT the stop. It reuses this file's own machinery — the SITES row's
# `STOP AT THE SPEC` anchor and `window_text` — so the two asserts share one anchor and check
# DIFFERENT literals: the SITES row checks the two-sided mode clause, this one checks the
# artifact/stop-point naming. They do not duplicate each other.
BS="$REPO/skills/docket-brainstorm/SKILL.md"
OWNER="owned by \`docket-implement-next\`"
assert "docket-brainstorm exists and is non-empty" '[ -s "$BS" ]'
bs_ln="$(anchor_line "$BS" "STOP AT THE SPEC")"
assert "docket-brainstorm still carries its STOP AT THE SPEC anchor" '[ -n "$bs_ln" ]'
if [ -n "$bs_ln" ]; then
  bs_win="$(window_text "$BS" "$bs_ln")"
  assert "docket-brainstorm's stop still names planning's owner AT the stop" \
    'case "$bs_win" in *"$OWNER"*) true ;; *) false ;; esac'
fi

# --- Non-vacuity anchor #3 (mutation-in-fixture): the matcher must FIRE on an unscoped site. ---
# Without this, a typo in either half makes every assert above permanently green — the inversion
# mirrored-guard-enforces-its-own-property warns about.
probe="$(mktemp)"
printf '%s\n' 'Then you stop — review is not yours.' 'Some other paragraph entirely.' > "$probe"
pl="$(anchor_line "$probe" "Then you stop — review is not yours.")"
assert "probe anchor is found (got '$pl')" '[ "$pl" = "1" ]'
assert "the matcher REJECTS an unscoped stop" '! clause_near "$probe" "$pl"'
# An absent anchor must yield EMPTY, not an error and not a spurious line number.
missing_ln="$(anchor_line "$probe" "no such anchor anywhere in this probe")"
assert "anchor_line returns empty for an absent anchor (got '$missing_ln')" '[ -z "$missing_ln" ]'
# And it must ACCEPT a properly scoped one.
printf '%s\n' 'Then you stop — review is not yours.' '' \
  "**Scope of this stop:** If $INLINE_HALF, this stop ends only this role — you continue to your own next step; $DISPATCH_HALF ends its turn here." > "$probe"
pl="$(anchor_line "$probe" "Then you stop — review is not yours.")"
assert "the matcher ACCEPTS a scoped stop" 'clause_near "$probe" "$pl"'
# ... including one whose halves straddle a hard wrap, which is how the swept prose is formatted.
printf '%s\n' 'Then you stop — review is not yours.' '' \
  '**Scope of this stop:** If you invoked this skill' 'yourself, this stop ends only this role; only an agent whose entire' 'assignment is this role ends its turn here.' > "$probe"
pl="$(anchor_line "$probe" "Then you stop — review is not yours.")"
assert "the matcher ACCEPTS a clause wrapped across lines" 'clause_near "$probe" "$pl"'
# One-sided clauses must NOT satisfy it: the wrapper injects the same body, so an agent whose whole
# assignment IS this role must still be told its turn ends. Both halves are load-bearing.
printf '%s\n' 'Then you stop — review is not yours.' '' \
  "**Scope of this stop:** If $INLINE_HALF, you continue to your own next step." > "$probe"
pl="$(anchor_line "$probe" "Then you stop — review is not yours.")"
assert "the matcher REJECTS a one-sided clause" '! clause_near "$probe" "$pl"'
# Presence far from the site must NOT satisfy it (the 0199 co-occurrence lesson, proved).
{ printf '%s\n' 'Then you stop — review is not yours.'
  for i in 1 2 3 4 5 6 7 8; do printf 'filler line %s\n' "$i"; done
  printf '%s\n' "$INLINE_HALF and $DISPATCH_HALF"; } > "$probe"
pl="$(anchor_line "$probe" "Then you stop — review is not yours.")"
assert "the matcher REJECTS a clause outside the site window" '! clause_near "$probe" "$pl"'
rm -f "$probe"

if [ "$fail" != 0 ]; then
  echo "REMEDY: a docket-owned skill body loadable into a caller's context scopes its terminal stop"
  echo "        and its second-person prohibitions to the role, two-sided and conditioned on HOW"
  echo "        THIS BODY ARRIVED, with the second person on the CONTINUE branch:"
  echo "        \"If $INLINE_HALF, ... you continue to your own next step;"
  echo "        $DISPATCH_HALF ends its turn here.\" Put the clause AT the site,"
  echo "        within $WINDOW lines of the anchor — not elsewhere in the file. If a stop was"
  echo "        reworded, update this file's SITES table in the same diff."
fi

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
