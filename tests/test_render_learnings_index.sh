#!/usr/bin/env bash
# tests/test_render_learnings_index.sh — guards change 0067's index renderer.
# render-learnings-index.sh is the SOLE writer of <changes_dir>/learnings/README.md: pure
# (stdout only, no git), deterministic (same inputs => identical bytes), offline.
#
# Determinism is guarded two ways: a cheap "byte-identical across two runs" assert (section f)
# and explicit ORDER asserts (section j) that pin the renderer's actual sort contract. The
# byte-identical assert alone is a WEAK, probabilistic guard — see section j's header comment
# for why; the order asserts are what make a sort removal redden 100% of trials, not ~50%.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
R="$REPO/scripts/render-learnings-index.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

SB="$(mktemp -d)"; trap 'rm -rf "$SB"' EXIT
LD="$SB/learnings"; mkdir -p "$LD"

mkfinding(){ # mkfinding SLUG HOOK TOPICS CHANGES STATE PROMOTED_TO
  cat >"$LD/$1.md" <<EOF
---
slug: $1
hook: "$2"
topics: [$3]
changes: [$4]
created: 2026-06-17
updated: 2026-07-16
promotion_state: $5
promoted_to: ${6:-}
---

## Apply
The rule for $1.

## War story
- 2026-07-14 (#72, PR #79) — something happened.
EOF
}

mkfinding guards-are-code "A guard is code: mutation-test it, or it is decoration." "testing, sentinels" "14, 15" retained
mkfinding pipefail "Never producer | early-exiting-consumer under pipefail." "shell" "11" candidate
mkfinding yaml-scalar "Quote any scalar carrying a colon-space." "config" "5" promoted "AGENTS.md"

# --- ORDER-ASSERT seed data ------------------------------------------------------------------
# Section (j) below asserts topic headers, in-group rows, and Promoted-appendix rows are all in
# sorted order. A shuffle-based mutation only reddens an order assert reliably if a shuffle is
# unlikely to land back on the correct order by chance — that needs enough distinct items (low
# 1/n! coincidence odds), not just >=2. So: 6 distinct topics (bravo/kilo/queue/shell/testing/
# zulu, sorted order != first-seen order) and 5 findings sharing the "queue" topic, plus 5
# promoted findings (4 new + yaml-scalar above).
mkfinding bravo-note "Bravo needs its own paragraph, not a shared one." "bravo" "1" retained
mkfinding kilo-note "Kilo is the loneliest heading." "kilo" "1" retained
mkfinding zulu-note "Zulu sorts last for a reason." "zulu" "1" retained
mkfinding queue-alpha "First alphabetically in the queue topic." "queue" "1" retained
mkfinding queue-bravo "Second alphabetically in the queue topic." "queue" "1" retained
mkfinding queue-charlie "Third alphabetically in the queue topic." "queue" "1" retained
mkfinding queue-delta "Fourth alphabetically in the queue topic." "queue" "1" retained
mkfinding queue-echo "Fifth alphabetically in the queue topic." "queue" "1" retained
mkfinding promoted-alpha "Graduated finding, alpha." "misc" "1" promoted "AGENTS.md"
mkfinding promoted-bravo "Graduated finding, bravo." "misc" "1" promoted "AGENTS.md"
mkfinding promoted-charlie "Graduated finding, charlie." "misc" "1" promoted "AGENTS.md"
mkfinding promoted-delta "Graduated finding, delta." "misc" "1" promoted "AGENTS.md"

out="$("$R" --learnings-dir "$LD")"; rc=$?

# (a) contract basics
assert "exits 0 on a valid dir" '[ "$rc" = "0" ]'
assert "writes nothing into the learnings dir (stdout-only, no git)" '[ ! -e "$LD/README.md" ]'
assert "missing --learnings-dir exits 2" '"$R" >/dev/null 2>&1; [ "$?" = "2" ]'
assert "nonexistent dir exits 2" '"$R" --learnings-dir "$SB/nope" >/dev/null 2>&1; [ "$?" = "2" ]'

# (b) DEQUOTE — hook is required to be quoted; the index must not carry the quote bytes
assert "hook is dequoted in the index" '! grep -qF -- "\"A guard is code" <<<"$out"'
assert "hook text renders" 'grep -qF -- "A guard is code: mutation-test it, or it is decoration." <<<"$out"'

# (c) grouping by PRIMARY topic (first tag); remaining tags render inline
assert "primary topic group header present" 'grep -qE "^## testing$" <<<"$out"'
assert "secondary tag renders inline" 'grep -qF -- "· also: sentinels" <<<"$out"'
assert "a finding appears exactly once" '[ "$(grep -cF -- "(guards-are-code.md)" <<<"$out")" = "1" ]'

# (d) candidate marker
assert "candidate carries the needs-promotion marker" \
  'row="$(grep -F -- "pipefail.md" <<<"$out")"; grep -qF -- "⟨needs promotion⟩" <<<"$row"'
assert "retained carries no marker" \
  'row="$(grep -F -- "guards-are-code.md" <<<"$out")"; ! grep -qF -- "⟨needs promotion⟩" <<<"$row"'

# (e) promoted findings leave the topic groups for the compressed appendix
assert "Promoted appendix present" 'grep -qE "^## Promoted$" <<<"$out"'
assert "promoted finding renders with its target" \
  'grep -qF -- "[yaml-scalar](yaml-scalar.md) → AGENTS.md" <<<"$out"'
assert "promoted finding is NOT in a topic group" \
  '[ "$(grep -cF -- "yaml-scalar.md" <<<"$out")" = "1" ]'
assert "promoted hook does not tax the hint surface" \
  '! grep -qF -- "Quote any scalar carrying a colon-space." <<<"$out"'

# (f) determinism / idempotency — cheap, but WEAK on its own (see file header + section j):
# it only catches nondeterminism when a shuffle happens to differ between two consecutive
# invocations, which for a small state space is well under 100% per trial. Keep it anyway —
# it's free and it also catches non-sort nondeterminism (e.g. reading env/PID/time by mistake)
# that the order asserts below don't target.
out2="$("$R" --learnings-dir "$LD")"
assert "byte-identical across runs" '[ "$out" = "$out2" ]'

# (g) empty dir is a valid, non-crashing render
ED="$SB/empty"; mkdir -p "$ED"
eout="$("$R" --learnings-dir "$ED")"; erc=$?
assert "empty dir exits 0" '[ "$erc" = "0" ]'
assert "empty dir still renders the header" 'grep -qF -- "# Learnings" <<<"$eout"'

# (h) README.md in the dir is excluded from the corpus — even when it carries VALID finding
# frontmatter. (A bare "# Learnings" fixture would pass this by accident: `field` already skips
# any file without a slug: line, independent of the filename filter. Only a fixture with a real
# slug: actually exercises the `! -name README.md` filter.)
cat >"$LD/README.md" <<'EOF'
---
slug: README
hook: "This must never leak into the rendered index."
topics: [meta]
changes: [1]
created: 2026-06-17
updated: 2026-07-16
promotion_state: retained
promoted_to:
---

## Apply
Should never appear.
EOF
out3="$("$R" --learnings-dir "$LD")"
assert "README.md is not treated as a finding" '[ "$out3" = "$out" ]'
rm -f "$LD/README.md"

# (i) corpus-size assert — prove the renderer SAW the findings (a parser that reads
#     nothing passes everything; guards-are-code family). 10 active + 5 promoted = 15.
assert "index counts all 15 findings" \
  '[ "$(grep -cE "^- \[" <<<"$out")" = "15" ]'

# (j) ORDER — pin the renderer's actual determinism contract, so a removed/weakened `sort`
# reddens every trial instead of ~half. (Finding, change 0067 Task 1 review: replacing the
# FILES-scan `sort` with `shuf` reddened the old byte-identical-across-runs assert 0/15 times
# in an independent 15-trial reproduction — that scan order is fully re-canonicalized downstream
# by TOPICS_SORTED/the per-topic row sort/PROMOTED_SORTED, so shuffling it changes nothing
# observable. These asserts target the sorts that are actually load-bearing.)
#
# Each list is compared to its own `sort` rather than a hand-typed golden sequence: the
# renderer's contract IS "sorted", so "does the extracted order equal itself sorted" is the
# direct assertion, and it doesn't need re-deriving by hand every time the fixture changes.
headers="$(grep -E '^## ' <<<"$out" | sed 's/^## //' | grep -vFx 'Promoted')"
queue_rows="$(awk '/^## queue$/{f=1;next} /^## /{f=0} f' <<<"$out" | sed -n 's/^- \[\([^]]*\)\].*/\1/p')"
promoted_rows="$(awk '/^## Promoted$/{f=1;next} /^## /{f=0} f' <<<"$out" | sed -n 's/^- \[\([^]]*\)\].*/\1/p')"

assert "topic group headers appear in sorted order" \
  '[ "$headers" = "$(sort <<<"$headers")" ]'
assert "header-order assert is not vacuous (>=6 distinct topics seeded)" \
  '[ "$(grep -c . <<<"$headers")" -ge 6 ]'

assert "rows within a topic group appear in sorted slug order" \
  '[ "$queue_rows" = "$(sort <<<"$queue_rows")" ]'
assert "row-order assert is not vacuous (>=5 findings share the queue topic)" \
  '[ "$(grep -c . <<<"$queue_rows")" -ge 5 ]'

assert "Promoted appendix entries appear in sorted slug order" \
  '[ "$promoted_rows" = "$(sort <<<"$promoted_rows")" ]'
assert "promoted-order assert is not vacuous (>=5 promoted findings seeded)" \
  '[ "$(grep -c . <<<"$promoted_rows")" -ge 5 ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
