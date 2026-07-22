#!/usr/bin/env bash
# tests/test_skill_size_budgets.sh — regrowth guard (change 0085): every skills/**/*.md stays
# within a per-file line/word budget (~10% above the 0085 post-slim actuals). A future change that
# bloats a skill must slim elsewhere or consciously RAISE the budget in this table (an in-diff edit).
# Budgets are a DIRECTION made durable, not the slim's goal (learnings: size-target-is-direction).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# BUDGETS: one row per tracked file — "<relpath> <maxLines> <maxWords>". Set from 0085 post-slim
# actuals + ~10% (ceil). To raise a budget, edit the number here in the same diff that grows the file.
# docket-convention/SKILL.md's word budget was raised 5689 -> 5850 by change 0127, which added a
# whole policy dimension to the Auto-capture shared definition (classify -> admit -> suppress, and
# the filtering-precedes-the-cap rule) plus the change_types / nested auto_capture config block and
# the manifest's type: field. The section was compressed by ~150 words first (the mint-sites,
# materiality, and deterministic-mint paragraphs); the residual is normative text with no other
# home. The line budget was NOT raised.
# docket-finalize-change/SKILL.md's word budget was raised 4060 -> 4200 by change 0102, which grew
# the file to 4059/4060 words (1 word of headroom) while wiring finalize.require_pr_approval
# through the resolver — the next edit to that file would have reddened CI on arrival.
BUDGETS="
skills/docket-adr/SKILL.md                                  86 1408
skills/docket-adr/adr-template.md                           26   90
skills/docket-auto-groom/SKILL.md                           66 1237
skills/docket-brainstorm/SKILL.md                           84  692
skills/docket-convention/SKILL.md                          354 5850
skills/docket-convention/github-board-mirror.md             19  462
skills/docket-convention/references/agent-layer.md         168 1839
skills/docket-convention/references/learnings.md            84  580
skills/docket-convention/references/terminal-close-out.md  173 1458
skills/docket-finalize-change/SKILL.md                     193 4200
skills/docket-groom-next/SKILL.md                           77 1484
skills/docket-implement-next/SKILL.md                      147 3315
skills/docket-implement-next/results-template.md            24  172
skills/docket-new-change/SKILL.md                           61 1330
skills/docket-new-change/change-template.md                 51  203
skills/docket-status/SKILL.md                              118 2393
"

# Every tracked file is within budget.
budgeted=""
while read -r rel maxL maxW; do
  [ -n "$rel" ] || continue
  budgeted="$budgeted $rel"
  f="$REPO/$rel"
  assert "budgeted file exists: $rel" '[ -f "$f" ]'
  [ -f "$f" ] || continue
  L=$(wc -l < "$f" | tr -d ' '); W=$(wc -w < "$f" | tr -d ' ')
  assert "$rel within line budget ($L <= $maxL)" '[ "$L" -le "$maxL" ]'
  assert "$rel within word budget ($W <= $maxW)" '[ "$W" -le "$maxW" ]'
done <<EOF
$BUDGETS
EOF

# Completeness (auto-discovery guard, finding #12): every skills/**/*.md has a budget row, so a
# newly-added skill file can never go silently un-budgeted.
missing=""
while IFS= read -r f; do
  rel="${f#"$REPO"/}"
  printf '%s' "$budgeted" | grep -qF -- " $rel" || missing="$missing $rel"
done < <(find "$REPO/skills" -name '*.md' | sort)
assert "every skills/**/*.md has a budget row (unbudgeted:[$missing])" '[ -z "$missing" ]'

# Non-vacuity / mutation proof: the guard actually bites. A synthetic file 1 line over a 1-line
# budget must be caught by the same comparison.
probe="$(mktemp)"; printf 'a\nb\n' > "$probe"
pL=$(wc -l < "$probe" | tr -d ' ')
assert "the line-budget comparison is non-vacuous (2 > 1 is caught)" '[ ! "$pL" -le 1 ]'
rm -f "$probe"

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
