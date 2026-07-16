#!/usr/bin/env bash
# tests/test_render_learnings_index.sh — guards change 0067's index renderer.
# render-learnings-index.sh is the SOLE writer of <changes_dir>/learnings/README.md: pure
# (stdout only, no git), deterministic (same inputs => identical bytes), offline.
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

out="$("$R" --learnings-dir "$LD")"; rc=$?

# (a) contract basics
assert "exits 0 on a valid dir" '[ "$rc" = "0" ]'
assert "writes nothing into the learnings dir (stdout-only, no git)" '[ ! -e "$LD/README.md" ]'
assert "missing --learnings-dir exits 2" '"$R" >/dev/null 2>&1; [ "$?" = "2" ]'
assert "nonexistent dir exits 2" '"$R" --learnings-dir "$SB/nope" >/dev/null 2>&1; [ "$?" = "2" ]'

# (b) DEQUOTE — hook is required to be quoted; the index must not carry the quote bytes
assert "hook is dequoted in the index" '! printf "%s" "$out" | grep -qF "\"A guard is code"'
assert "hook text renders" 'printf "%s" "$out" | grep -qF "A guard is code: mutation-test it, or it is decoration."'

# (c) grouping by PRIMARY topic (first tag); remaining tags render inline
assert "primary topic group header present" 'printf "%s" "$out" | grep -qE "^## testing$"'
assert "secondary tag renders inline" 'printf "%s" "$out" | grep -qF "· also: sentinels"'
assert "a finding appears exactly once" '[ "$(printf "%s" "$out" | grep -cF "(guards-are-code.md)")" = "1" ]'

# (d) candidate marker
assert "candidate carries the needs-promotion marker" \
  'printf "%s" "$out" | grep -F "pipefail.md" | grep -qF "⟨needs promotion⟩"'
assert "retained carries no marker" \
  'printf "%s" "$out" | grep -F "guards-are-code.md" | grep -vqF "⟨needs promotion⟩"'

# (e) promoted findings leave the topic groups for the compressed appendix
assert "Promoted appendix present" 'printf "%s" "$out" | grep -qE "^## Promoted$"'
assert "promoted finding renders with its target" \
  'printf "%s" "$out" | grep -qF "[yaml-scalar](yaml-scalar.md) → AGENTS.md"'
assert "promoted finding is NOT in a topic group" \
  '[ "$(printf "%s" "$out" | grep -cF "yaml-scalar.md")" = "1" ]'
assert "promoted hook does not tax the hint surface" \
  '! printf "%s" "$out" | grep -qF "Quote any scalar carrying a colon-space."'

# (f) determinism / idempotency
out2="$("$R" --learnings-dir "$LD")"
assert "byte-identical across runs" '[ "$out" = "$out2" ]'

# (g) empty dir is a valid, non-crashing render
ED="$SB/empty"; mkdir -p "$ED"
eout="$("$R" --learnings-dir "$ED")"; erc=$?
assert "empty dir exits 0" '[ "$erc" = "0" ]'
assert "empty dir still renders the header" 'printf "%s" "$eout" | grep -qF "# Learnings"'

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
#     nothing passes everything; guards-are-code family)
assert "index counts all 3 findings" \
  '[ "$(printf "%s" "$out" | grep -cE "^- \[")" = "3" ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
