#!/usr/bin/env bash
# tests/test_dispatch_capability.sh — guards change 0137 (dispatch-capability detection).
#   - the convention carries a capability-resolution rule: resolve (incl. deferred tool surfaces),
#     then attempt one trivial dispatch; only a failed attempt or a policy denial proves absence
#   - the rule is stated by CAPABILITY: an absent tool NAME is explicitly not sufficient evidence
#   - the tiered unavailability posture (A deterministic / B adversarial / C discipline) is present
#   - Tier C is drawn AGAINST the Skill layer's missing-skill rule, not layered on top of it
#   - every CONSUMING dispatch site names its tier (producer coverage, not definition-only)
#   - no live docket prose gates a decision on a literal tool name (shape-scoped, no allowlist)
# Sentinels are sampling, not parsing (learnings: foundational-test-discipline) — pair with the
# whole-branch review.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

CONV="$REPO/skills/docket-convention/SKILL.md"

# --- the rule exists and is stated by capability -------------------------------------------------
assert "convention: has a dispatch-capability resolution section" \
  'grep -qiE "^#+ .*[Dd]ispatch-capability resolution" "$CONV"'
assert "convention: resolution includes searching deferred/lazily-loaded tool surfaces" \
  'grep -qiE "deferred or lazily-loaded" "$CONV"'
assert "convention: inconclusive resolution escalates to one trivial dispatch attempt" \
  'grep -qiE "attempt(ed)? one trivial dispatch" "$CONV"'
assert "convention: only a failed attempt or a policy denial establishes unavailability" \
  'grep -qiE "failed attempt.{0,40}policy denial" "$CONV"'
# The load-bearing negative: a missing tool NAME is explicitly not evidence. Deleting this
# sentence is exactly the regression the change exists to prevent, so it gets its own assert.
assert "convention: an absent tool NAME is explicitly insufficient evidence" \
  'grep -qiE "absence of a specifically-named tool never" "$CONV"'
assert "convention: a tool name is a diagnostic, never a decision input" \
  'grep -qiE "never a decision input" "$CONV"'

# --- the tiered posture --------------------------------------------------------------------------
for tier in "A — deterministic" "B — adversarial" "C — discipline"; do
  assert "convention: tier present: $tier" 'grep -qF -- "$tier" "$CONV"'
done
assert "convention: Tier A is a first-class equivalent path, not a degradation" \
  'grep -qiE "first-class equivalent path" "$CONV"'
assert "convention: Tier B routes to the existing abstain" \
  'grep -qE "^\| \*\*B — adversarial\*\*.*\*\*Abstain\*\*" "$CONV"'
assert "convention: Tier C is authorized-or-halt" \
  'grep -qiE "authorized-or-halt" "$CONV"'
assert "convention: Tier C names an explicitly configured auto as the authorization" \
  'grep -qiE "explicitly configured .?auto.? is the human" "$CONV"'
assert "convention: Tier C halt adds no new status or field" \
  'grep -qiE "[Nn]o new status, no new field" "$CONV"'

# --- the boundary against the pre-existing missing-skill rule ------------------------------------
# Both rules must coexist and be DISTINGUISHED; if the missing-skill rule vanished, Tier C would
# have silently replaced it (a scope change this change does not authorize).
assert "convention: the missing-skill rule still exists" \
  'grep -qE "^- \*\*Missing-skill rule — degrade to .?auto.? \+ warn\*\*" "$CONV"'
assert "convention: Tier C is distinguished from the missing-skill rule" \
  'grep -qiE "cannot be \*\*invoked\*\*.{0,200}cannot \*\*dispatch\*\*" "$CONV"'

# --- producer coverage: every CONSUMING dispatch site names its tier ----------------------------
# Anchored on the consuming skill sections, never an allowlist of tiers (learnings:
# correspondence-guard-runs-one-way). Each row: "<file> <anchor regex> <expected tier> <label>
# <site noun>". The anchor is the site's own dispatch sentence, so a tier marker parked in an
# unrelated paragraph does not satisfy it (learnings: marker-scoped-guard-needs-a-population-floor
# — attachment, not presence). Bare tier PRESENCE anywhere in the paragraph is not enough either:
# two sites can share one paragraph, so a tier assert must also PAIR the site's own distinguishing
# noun with its tier, within a bounded distance that does not cross a `;`/`.` clause boundary —
# otherwise the two tiers could be swapped between sites (or a glued list could smuggle a bare tier
# literal into an unrelated site's paragraph) and every assert would still read PASS.
IMPL="$REPO/skills/docket-implement-next/SKILL.md"
AUTOGROOM="$REPO/skills/docket-auto-groom/SKILL.md"

# Print the single paragraph (blank-line-delimited block) containing the first anchor match.
para_with(){ awk -v pat="$2" 'BEGIN{RS="";} $0 ~ pat {print; exit}' "$1"; }

seen=0
all_nouns=""
# NOTE: the tier is expanded into the assert expression at call time. `assert` runs `eval "$2"`,
# so a `$3` left inside that string would resolve to *assert's* third positional parameter (unset
# under `set -u`), not this function's — a real trap, caught while writing this plan.
check_site(){ # $1 file  $2 anchor regex  $3 expected tier  $4 label  $5 site noun
  local p tier label noun; p="$(para_with "$1" "$2")"; tier="$3"; label="$4"; noun="$5"
  echo "seen $(basename "$(dirname "$1")")/$(basename "$1") $tier"  # per-site record, before any skip
  # Reach floor: only count a site the scanner actually FOUND (see the floor assert below) — an
  # unconditional increment cannot tell a renamed anchor / moved paragraph from a real hit.
  # (Written as if/then/fi rather than `[ -n "$p" ] && seen=...` so a future `set -e` in this file
  # cannot abort the run on the false branch of the `&&` chain.)
  if [ -n "$p" ]; then seen=$((seen+1)); fi
  all_nouns="$all_nouns $noun"
  assert "$label: dispatch site found" '[ -n "$p" ]'
  # Proximity, either order, same clause: "$noun ... $tier" or "$tier ... $noun" within 80 chars
  # and never crossing a `;` or `.` — bare presence of the tier literal anywhere in the paragraph
  # is not this assert; it must sit in the same clause as THIS site's own noun.
  assert "$label: names $tier next to its own noun ($noun), same clause" \
    "grep -qE -- \"${noun}[^;.]{0,80}${tier}|${tier}[^;.]{0,80}${noun}\" <<<\"\$p\""
}

check_site "$IMPL"      "dispatch the .?docket-status.? subagent" "Tier A" "implement-next §0 docket-status" "docket-status"
check_site "$IMPL"      "docket-adr.? subagent"                  "Tier A" "implement-next §6 docket-adr"    "docket-adr"
check_site "$IMPL"      "resolved build skill"                   "Tier C" "implement-next §5 build"         "build"
check_site "$IMPL"      "resolved review skill"                  "Tier C" "implement-next §6 review"        "review"
check_site "$AUTOGROOM" "docket-auto-groom-critic"               "Tier B" "auto-groom §3 critic"            "docket-auto-groom-critic"

# Population floor: the scanner must have REACHED all five sites. A renamed heading or a moved
# paragraph now genuinely reddens this floor too, because `seen` only increments on an actual find
# (see check_site above) — it is no longer an unconditional counter that always equals the number
# of check_site calls regardless of whether any of them found anything.
assert "consumer coverage: all five dispatch sites were reached (floor)" '[ "$seen" -eq 5 ]'

# --- reverse correspondence: derive the dispatch-site population by SHAPE, never hand-listed -----
# (learnings: never hand-list the sites of a literal/operation you are gating — derive them from a
# whole-repo grep). The five check_site rows above are the FORWARD direction (each hand-picked site
# names its tier); this is the REVERSE direction — every dispatch shape found by an independent
# grep must be one of those hand-picked rows, so a sixth dispatch site added later without a
# check_site row cannot be invisible to this guard.
derived=""
while IFS= read -r name; do
  [ -n "$name" ] || continue
  derived="$derived $name"
done < <(grep -ohE '`[A-Za-z0-9_-]+`[^`]{0,20}subagent' "$IMPL" "$AUTOGROOM" \
           | grep -oE '`[A-Za-z0-9_-]+`' | tr -d '`')
while IFS= read -r name; do
  [ -n "$name" ] || continue
  derived="$derived $name"
done < <(grep -ohE 'resolved (build|review) skill' "$IMPL" | grep -oE 'build|review')

# Population floor: the derivation itself must have found all five shapes. Without this, rewording
# a dispatch mention out of backticks (e.g. `` `docket-status` subagent `` -> `docket-status
# subagent`) silently drops it from $derived and the loop below simply iterates over fewer names —
# every reverse assert still reads PASS despite two of five derived sites having gone missing.
assert "reverse: derivation found all five shapes" \
  '[ "$(printf "%s" "$derived" | wc -w)" -eq 5 ]'

for name in $derived; do
  # Token match, not substring: the trailing space in both the pattern and the haystack keeps a
  # phantom site named e.g. "docket" from being falsely reported as covered by " docket-status".
  assert "reverse: derived dispatch site '$name' is covered by a check_site row" \
    "grep -qF -- \" $name \" <<<\"\$all_nouns \""
done

# --- negative guard: no live prose gates a decision on a literal tool name -----------------------
# Shape, not an allowlist (AGENTS.md: never hand-list the sites of a literal you are gating). In
# live prose every `Task` occurrence is a dispatch-tool reference; it is legitimate only where the
# line is Cursor-scoped, since Cursor documents a Task tool and Claude Code does not. docs/adrs/ is
# out of scope: an Accepted ADR is immutable and is corrected by an appended dated `## Update`.
mentions="$(cd "$REPO" && grep -rn '\bTask\b' --include='*.md' skills/ README.md 2>/dev/null)"
offenders=""; cursor_scoped=0; total=0
while IFS= read -r line; do
  [ -n "$line" ] || continue
  total=$((total+1))
  echo "seen ${line%%:*}:$(cut -d: -f2 <<<"$line")"          # per-hit record, before any skip
  if grep -qi 'cursor' <<<"$line"; then
    cursor_scoped=$((cursor_scoped+1))
  else
    offenders="$offenders
$line"
  fi
done <<EOF
$mentions
EOF

assert "no live prose names a dispatch tool outside a Cursor-scoped line" \
  '[ -z "$(printf %s "$offenders" | tr -d "[:space:]")" ]'
[ -z "$(printf %s "$offenders" | tr -d '[:space:]')" ] || printf 'offending lines:%s\n' "$offenders"
# Population floor: the scan must have reached real content. Zero hits would pass the assert above
# vacuously — a path typo or a moved file must redden, not silently guard nothing.
assert "negative guard: scan reached live prose (floor: >=2 Cursor-scoped mentions)" \
  '[ "$cursor_scoped" -ge 2 ]'
assert "negative guard: scan is non-empty" '[ "$total" -ge 2 ]'

# Positive control: the guard must REPORT a planted violation, whatever the real tree looks like
# (learnings: marker-scoped-guard-needs-a-population-floor — coverage, not population).
ctl="$(mktemp -d)"; trap 'rm -rf "$ctl"' EXIT
mkdir -p "$ctl/skills/x"
printf 'A forked skill-invoke and an explicit agent dispatch (a `Task` naming the wrapper) are one.\n' \
  > "$ctl/skills/x/SKILL.md"
: > "$ctl/README.md"
ctl_hits="$(cd "$ctl" && grep -rn '\bTask\b' --include='*.md' skills/ README.md 2>/dev/null | grep -vi cursor)"
assert "negative guard: positive control — a planted non-Cursor Task line IS detected" \
  '[ -n "$ctl_hits" ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
