#!/usr/bin/env bash
# tests/test_cursor_dispatch_rule.sh — the Cursor dispatch rule instructs by CAPABILITY, never by
# a literal tool name (change 0135; ADR-0059 §2). A concrete call snippet may still appear, but
# only as a clearly-labelled ILLUSTRATION — naming a tool in an instruction primes the next agent
# to probe for that literal and conclude absence when the mechanism ships under a different name
# (learnings: capability-absence-needs-a-failed-attempt).
# run: bash tests/test_cursor_dispatch_rule.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

HEAD="$REPO/cursor-rules/dispatch.head.md"
assert "head: exists" '[ -f "$HEAD" ]'
# The head's `description:` frontmatter PARAPHRASES the same rules the instruction body states, so
# a positive assert run over the whole file is satisfied by the frontmatter alone while the actual
# instruction to the parent chat is deleted — mutation-proven: deleting `1. Do **NOT** run the skill
# inline in this chat.` outright left every head assert green, because `never run inline` in the
# description matched instead. Positive asserts therefore run over the BODY only: everything after
# the closing frontmatter delimiter. `*` is stripped first so bold markup cannot break a phrase
# match — that is the second half of how the frontmatter-anchoring hid, since `Do **NOT** run` does
# not match `not run`.
head_body="$(awk 'NR>1 && /^---$/ {on=1; next} on' "$HEAD")"
head_plain="$(tr -d '*' <<<"$head_body")"
# Floor: if the frontmatter delimiters ever change shape the extraction yields nothing, and every
# body-scoped assert below would pass or fail for the wrong reason. Pin that the body was reached.
assert "head: instruction body extracted (floor — not frontmatter-only)" \
  'grep -q "^## Required dispatch pattern" <<<"$head_body"'
assert "head: instructs by capability (subagent-launch mechanism)" \
  'grep -qiE "subagent-launch mechanism|this mode.s subagent" <<<"$head_body"'
assert "head: still forbids running inline" 'grep -qi "do not run the skill inline" <<<"$head_plain"'
assert "head: still requires foreground" 'grep -qi "foreground" <<<"$head_plain"'
# The NEGATIVE assert runs over the WHOLE file, frontmatter included — the `description:` line was
# one of only two genuine pre-existing violations in the tree, so scoping this to the body would
# leave the very place the defect lived uncovered. Same shape as the per-fragment assert below:
# indented lines are the labelled illustration and are exempt.
assert "head: instruction prose names no dispatch tool literal" \
  '! grep -qE "\b(Task|Agent)\b" <<<"$(grep -v "^    " "$HEAD")"'

# 0168 whole-branch review, IMPORTANT 3: the head's RATIONALE must stay true against the shipped
# sidecar. It used to open "Docket ships model/effort-pinned subagent wrappers in
# `.cursor/agents/docket-*.md`" and rest the whole dispatch requirement on "which defeats the pin"
# — true when every wrapper source carried a pin, false since change 0168 made the harness-indexed
# sidecar the default store and shipped cursor IDs for the three build profiles only. The head is
# catted verbatim into the generated `.cursor/rules/docket-dispatch.mdc`, so the false claim
# shipped into every cursor repo.
# The premise is DERIVED, not hard-coded: a future change that pins every cursor wrapper turns the
# `if` false and retires the guard, rather than leaving a stale assert behind.
# shellcheck source=/dev/null
. "$REPO/scripts/lib/harness-defaults.sh"
HD="$REPO/agents/harness-defaults.yml"
n_src=0
for f in "$REPO"/agents/docket-*.md; do [ -e "$f" ] || continue; n_src=$((n_src+1)); done
n_cursor_pinned="$(hd_agents "$HD" cursor | grep -c . || true)"
assert "head: sidecar population floor (>=16 sources, >=1 shipped cursor pin) — the premise below is not vacuous" \
  '[ "$n_src" -ge 16 ] && [ "$n_cursor_pinned" -ge 1 ]'
if [ "$n_cursor_pinned" -lt "$n_src" ]; then
  assert "head: makes no blanket 'ships model/effort-pinned wrappers' claim ($n_cursor_pinned of $n_src cursor wrappers carry a shipped pin)" \
    '! grep -qiE "ships model/effort-pinned" "$HEAD"'
  assert "head: says the unpinned wrappers exist" 'grep -qi "unpinned" <<<"$head_plain"'
  assert "head: requires the dispatch for a pinned and an unpinned wrapper alike" \
    'grep -qi "either way" <<<"$head_plain"'
else
  # Every cursor wrapper carries a shipped pin ($n_cursor_pinned of $n_src). The complementary
  # obligation: the head must not still claim only SOME wrappers are pinned, which is what it said
  # from change 0168 until change 0184. Without this arm the whole head premise goes unchecked the
  # moment the sidecar becomes complete — the failure mode 0184 found live.
  assert "head: makes no 'only some wrappers are pinned' claim ($n_cursor_pinned of $n_src pinned)" \
    '! grep -qiE "(workers|wrappers) only|every other wrapper is generated" <<<"$head_plain"'
  assert "head: names the build-profile workers by their current names" \
    'grep -qF "docket-build-max" <<<"$head_plain"'
fi

# Population derived by glob, with a floor. Sixteen built-in agents ship fragments today, and the
# floor is that same 16 (raised from 9 by change 0167, which added the three docket-build profile
# agents — a floor of 9 would have tolerated deleting all three fragments; raised to 13 by
# change 0184, which added the fourth build profile; raised again to 16 by change 0170, which added
# the three docket-review rung wrappers): adding a seventeenth agent does not redden,
# while a vanished/renamed directory or a dropped fragment does.
frags=""; n=0
for f in "$REPO"/cursor-rules/dispatch/docket-*.md; do
  [ -e "$f" ] || continue
  frags="$frags $f"; n=$((n+1))
done
assert "fragments: population floor reached (>= 16 found)" '[ "$n" -ge 16 ]'

for f in $frags; do
  b="$(basename "$f")"
  # A fragment may SHOW a call snippet, but its INSTRUCTION line must not name a tool. The
  # instruction lines are the prose ones; the illustration is indented as a code block.
  instr="$(grep -v '^    ' "$f")"
  assert "$b: instruction prose names no dispatch tool literal" \
    '! grep -qE "\b(Task|Agent)\b" <<<"$instr"'
  assert "$b: instruction says dispatch to the subagent" \
    'grep -qiE "dispatch (to|the)" <<<"$instr"'
  # If a call snippet is present at all, it must be LABELLED as an illustration, so a reader
  # cannot mistake the name for the contract.
  if grep -qE '^    [A-Za-z]+\(' "$f"; then
    assert "$b: call snippet is labelled an illustration" 'grep -qi "illustration" "$f"'
  fi
done

# ---- change 0208: the parent-facing fragments carry the feature-worktree requirement ----
# The generated wrapper for a FEATURE-SCOPED agent is a shim demanding `--worktree <feature
# worktree>`, whose own prose says "If your caller named no worktree, abort-and-report". These
# fragments ARE the caller's instructions: a fragment that never tells the parent to name one turns
# that abort into a deterministic dispatch failure for every runner-delegated feature-scoped agent.
# The population is DERIVED from each source's declared `worktree-scope:` — never a `build-*` name
# shape and never a hand-list of today's nine, which is the whole thesis of change 0208 and would
# otherwise be a second copy of the facade's predicate, drifting the day a tenth agent is declared
# feature-scoped (LEARNINGS: duplicated-gate-copies-the-whole-predicate).
# Matched over the instruction prose FLATTENED to one line — the requirement is one wrapped
# sentence, so a line-scoped grep would key on wrap position — and sentence-bounded via `[^.]*`, so
# the demand has to sit in the same sentence as what the prompt must carry, not anywhere in the
# file. The complementary arm is real, not symmetry for its own sake: a metadata-scoped agent runs
# in the main worktree, its shim bakes no slot, and a fragment telling the parent to name a feature
# worktree for it would be an instruction the facade cannot honor.
n_feature=0; n_meta=0
for src in "$REPO"/agents/docket-*.md; do
  [ -e "$src" ] || continue
  b="$(basename "$src")"
  scope="$(sed -n '/^worktree-scope:/{s/^worktree-scope:[[:space:]]*//;p;q;}' "$src")"
  # Floor: an absent or off-shape declaration would send every source down the `else` arm below and
  # silently retire the positive assert for exactly the agent that lost its declaration.
  assert "$b: declares a valid worktree-scope (floor — the arms below key on it)" \
    '[ "$scope" = "feature" ] || [ "$scope" = "metadata" ]'
  frag="$REPO/cursor-rules/dispatch/$b"
  assert "$b: ships a dispatch fragment" '[ -f "$frag" ]'
  [ -f "$frag" ] || continue
  flat="$(tr '\n' ' ' < <(grep -v '^    ' "$frag"))"
  if [ "$scope" = "feature" ]; then
    n_feature=$((n_feature+1))
    assert "$b: feature-scoped — the fragment tells the parent to name the feature worktree" \
      'grep -qiE "prompt[^.]*feature worktree" <<<"$flat"'
  else
    n_meta=$((n_meta+1))
    assert "$b: metadata-scoped — the fragment demands no feature worktree" \
      '! grep -qiE "prompt[^.]*feature worktree" <<<"$flat"'
  fi
done
# Floors on both arms: a vanished agents/ dir, or a wholesale flip of every declaration to one
# value, would leave one arm vacuously green.
assert "0208: feature-scoped population floor reached (>= 9 of $((n_feature+n_meta)) sources)" \
  '[ "$n_feature" -ge 9 ]'
assert "0208: metadata-scoped population floor reached (>= 7 of $((n_feature+n_meta)) sources)" \
  '[ "$n_meta" -ge 7 ]'

echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
