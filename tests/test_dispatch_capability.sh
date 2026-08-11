#!/usr/bin/env bash
# tests/test_dispatch_capability.sh — guards change 0137 (dispatch-capability detection).
#   - the convention carries a capability-resolution rule: resolve (incl. deferred tool surfaces),
#     then attempt one trivial dispatch; only a failed attempt or a policy denial proves absence
#   - the rule is stated by CAPABILITY: an absent tool NAME is explicitly not sufficient evidence
#   - the tiered unavailability posture (A deterministic / B adversarial / C discipline) is present
#   - Tier C is drawn AGAINST the Skill layer's missing-skill rule, not layered on top of it, with
#     each condition pinned to ITS OWN posture (polarity, not mere co-occurrence)
#   - every CONSUMING dispatch site names its tier AND cites the resolution rule (producer
#     coverage, not definition-only) — the citation is what stops a "no dispatch tool" conclusion
#   - the convention's tier table AGREES with those sites (whole-branch coherence, cross-file)
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

# --- the carve-out: the two finalize gate dispatches sit OUTSIDE the taxonomy (change 0260) -------
# Their contract is an in-context report gating the merge, not git state on metadata_branch, so
# neither Tier A's "inline is a first-class equivalent" nor Tier C's authorized-or-halt can apply.
# Paragraph-scoped, not file-scoped: the paragraph is identified BY the label literal, so
# membership in it IS the binding "this noun is carve-out-classified" (learnings:
# prose-guard-binds-phrase-to-claim). A file-wide `grep -q docket-rebase-resolver "$CONV"` would be
# satisfied by the *Composition* paragraph, which names both agents for an unrelated reason.
carveout_para="$(awk 'BEGIN{RS="";} /carve-out/ {print; exit}' "$CONV")"
assert "convention: a carve-out paragraph exists (anchor for the asserts below)" \
  '[ -n "$carveout_para" ]'
assert "convention carve-out: names docket-rebase-resolver" \
  'grep -qF -- "docket-rebase-resolver" <<<"$carveout_para"'
assert "convention carve-out: names docket-integration-repair" \
  'grep -qF -- "docket-integration-repair" <<<"$carveout_para"'
assert "convention carve-out: states the posture is finalize's abort-and-report" \
  'grep -qF -- "abort-and-report" <<<"$carveout_para"'
assert "convention carve-out: forbids inline substitution" \
  'grep -qiE "[Ii]nline substitution is forbidden" <<<"$carveout_para"'
assert "convention carve-out: gives the self-approval reason for that prohibition" \
  'grep -qF -- "self-approval" <<<"$carveout_para"'
# The reason these two are OUT of the table, in the paragraph's own words — without it the
# carve-out reads as an unexplained exception and a later editor folds it back into a row.
assert "convention carve-out: says their contract is an in-context report, not git state" \
  'grep -qE -- "in-context report[^.]{0,80}gating the merge" <<<"$carveout_para"'

# The swapped-subjects blind spot, negative direction: neither carved-out noun may appear in an
# A/B/C tier ROW. Without this, moving `docket-integration-repair` into the Tier C row (claiming
# authorized-or-halt for it, so `skills.build: auto` would authorize inline repair by the agent
# that then merges it) keeps every positive assert above green.
tier_rows_all="$(grep -E "^\| \*\*[A-Z] —" "$CONV")"
# Non-vacuity companion through the SAME extractor: an absence assert over a dead extractor reads
# as the property holding (learnings: assert-detects-removal-not-replacement, rule 5).
assert "convention: the tier-row extractor still reaches the table" \
  '[ "$(grep -c . <<<"$tier_rows_all")" -ge 3 ] && grep -qF -- "docket-status" <<<"$tier_rows_all"'
assert "convention: no tier row claims docket-rebase-resolver (carve-out, not a tier member)" \
  '! grep -qF -- "docket-rebase-resolver" <<<"$tier_rows_all"'
assert "convention: no tier row claims docket-integration-repair (carve-out, not a tier member)" \
  '! grep -qF -- "docket-integration-repair" <<<"$tier_rows_all"'

# --- the boundary against the pre-existing missing-skill rule ------------------------------------
# Both rules must coexist and be DISTINGUISHED; if the missing-skill rule vanished, Tier C would
# have silently replaced it (a scope change this change does not authorize).
assert "convention: the missing-skill rule still exists" \
  'grep -qE "^- \*\*Missing-skill rule — degrade to .?auto.? \+ warn\*\*" "$CONV"'
# POLARITY, not co-occurrence. The earlier form of this guard was
# `cannot be \*\*invoked\*\*.{0,200}cannot \*\*dispatch\*\*` — it only checked that the two phrases
# appeared in that ORDER, so INVERTING the sentence (claiming "cannot be **invoked** is Tier C …
# cannot **dispatch** still degrades to `auto` + warn") kept the whole suite green. That is the
# single most load-bearing sentence in this change, so each condition is now pinned to ITS OWN
# posture inside one clause. The proximity class is `[^;.*]` — excluding `*` as well as clause
# punctuation, so a match may not reach ACROSS the next bolded phrase: without that exclusion the
# inverted sentence still satisfied the invoked-side assert by reaching past `**dispatch**` to its
# "degrades" (78 chars — inside a plain 80-char window). Swapping the postures now reddens both.
assert "convention: a skill that cannot be INVOKED degrades to auto + warn (missing-skill rule)" \
  'grep -qE "invoked\*\*[^;.*]{0,40}degrades" "$CONV"'
assert "convention: a skill that was invoked but cannot DISPATCH is Tier C" \
  'grep -qE "dispatch\*\*[^;.*]{0,40}Tier C" "$CONV"'

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
FIXLOOP="$REPO/skills/docket-implement-next/references/fix-loop.md"

# Print the single paragraph (blank-line-delimited block) containing the first anchor match.
para_with(){ awk -v pat="$2" 'BEGIN{RS="";} $0 ~ pat {print; exit}' "$1"; }

seen=0
all_nouns=""
site_rows=""   # one "<tier>|<noun>" record per check_site row, for the table-coherence loop below
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
  site_rows="$site_rows$tier|$noun
"
  assert "$label: dispatch site found" '[ -n "$p" ]'
  # The back-pointer is what actually stops an agent from concluding "no dispatch tool": a tier
  # label alone tells it WHICH posture, never that unavailability must be ESTABLISHED by resolution
  # + one trivial attempt. Deleting the "per the convention's *Dispatch-capability resolution* —
  # never from a tool name" clause from every wired site (tier labels left intact) kept the suite
  # green before this assert existed — the phrase appeared nowhere in this file at all.
  assert "$label: paragraph cites the convention's Dispatch-capability resolution rule" \
    'grep -qF -- "Dispatch-capability resolution" <<<"$p"'
  # The section title alone is not the whole clause: "never from a tool name" is the half that
  # forbids the false-negative this change exists to end, so it gets its own assert. Deleting it
  # from all five sites kept the suite green when only the title was checked.
  assert "$label: paragraph forbids concluding unavailability from a tool name" \
    'grep -qF -- "never from a tool name" <<<"$p"'
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
# The Step 6 in-branch fix workers (change 0218) are a THIRD Tier C consumer, and the only one that
# does not own a `skills:` role of its own — it BORROWS the build role's `auto`. Wiring it as an
# ordinary check_site row is what pins the dual-purpose statement in its canonical home without a
# copy-pinning assert: the table-coherence loop below derives its requirement from this row's noun,
# so the convention's Tier C row must NAME the fix consumer or this reddens. Rewording either side
# independently is exactly what the derivation catches; nothing here quotes a sentence.
check_site "$FIXLOOP"   "fix dispatch is"                        "Tier C" "implement-next §6 fix loop"      "fix"

# Population floor: the scanner must have REACHED all six sites. A renamed heading or a moved
# paragraph now genuinely reddens this floor too, because `seen` only increments on an actual find
# (see check_site above) — it is no longer an unconditional counter that always equals the number
# of check_site calls regardless of whether any of them found anything.
assert "consumer coverage: all six dispatch sites were reached (floor)" '[ "$seen" -eq 6 ]'

# --- the borrowed authorization is stated in its canonical home ----------------------------------
# The row-derived coherence check above proves the Tier C row NAMES the fix consumer; it does not
# prove the row says WHICH switch authorizes it. A config reader's surprise is the whole finding:
# one knob named for the build role authorizes two different kinds of inline work. Pin that pairing
# inside the row's own dispatch CELL — `[^|]` cannot cross a table-cell boundary, so a `skills.build:
# auto` sitting in the neighbouring posture cell (where it would read as the build role's alone)
# does not satisfy this.
tierC_row="$(grep -E "^\| \*\*C — discipline\*\*" "$CONV")"
assert "convention: Tier C row exists (anchor for the borrowed-authorization asserts)" \
  '[ -n "$tierC_row" ]'
assert "convention: Tier C names skills.build: auto as the FIX consumer's authorization too" \
  'grep -qE -- "fix[^|]{0,160}skills\.build: auto|skills\.build: auto[^|]{0,160}fix" <<<"$tierC_row"'
# And that the borrowing is a stated decision, not an omission: a reader must not have to wonder
# whether a `skills.fix` key exists somewhere they have not looked.
assert "convention: Tier C states there is no separate skills.fix key" \
  'grep -qE -- "no separate .?skills\.fix" <<<"$tierC_row"'

# --- whole-branch coherence: the convention's tier table must AGREE with the consuming sites ------
# Nothing above cross-checks the definition against its consumers, and they live in different files
# — so SWAPPING the SUBJECTS of the Tier A and Tier B table rows (the convention then claiming the
# `docket-status`/`docket-adr` dispatches abstain and the critic runs inline, flatly contradicting
# all five wired sites) left the whole suite green. That is precisely the whole-branch property a
# per-task review cannot see. Derived from the check_site rows themselves — the single source of
# truth already in this file — so this introduces no second hand-list: each row's own noun must
# appear in the convention's table row for that row's own tier.
tier_row_names(){ # $1 = tier letter (A|B|C)  $2 = site noun
  local row; row="$(grep -E "^\| \*\*$1 —" "$CONV")"
  grep -qF -- "$2" <<<"$row"
}
while IFS='|' read -r t n; do
  [ -n "$t" ] || continue
  letter="${t##* }"
  assert "convention table: the Tier $letter row names '$n' (agrees with its wired site)" \
    "tier_row_names '$letter' '$n'"
done <<EOF
$site_rows
EOF

# --- reverse correspondence: derive the dispatch-site population by SHAPE, never hand-listed -----
# (AGENTS.md: never hand-list the sites of a literal/operation you are gating — derive them from a
# whole-repo grep). The five check_site rows above are the FORWARD direction (each hand-picked site
# names its tier); this is the REVERSE direction — every dispatch shape found by an independent
# grep must be either one of those rows or a knowingly-untiered site named in $PENDING_TIER below,
# so a NEW dispatch site added later cannot be invisible to this guard.
#
# The derivation walks the WHOLE `skills/` tree, not a named pair of files. Grepping only
# $IMPL + $AUTOGROOM was itself the hand-list this comment claimed not to be: a planted
# `**dispatch the `docket-newthing` subagent**` in skills/docket-status/SKILL.md stayed green
# (mutation-proven), so "a sixth dispatch site cannot be invisible" was false as written.
#
# KNOWN GAP, machine-visible on purpose. `docket-finalize-change` dispatches
# `docket-rebase-resolver` (rebase-conflict reconciliation) and `docket-integration-repair` (red
# suite after the rebase lands). Their unavailability tier is DELIBERATELY DEFERRED to a follow-up
# change rather than improvised here: no design review gated a posture for them, and finalize's own
# *abort-and-report points (the full set)* already covers both situations, so nothing is unsafe
# today. They are listed here — not in the convention's tier table — so the deferral is a fact this
# guard states rather than prose nobody checks. THIS LIST MUST SHRINK TO EMPTY when the follow-up
# change tiers them (they become ordinary check_site rows then); it is never the place to park a
# genuinely new dispatch site to silence the coverage loop below.
PENDING_TIER=" docket-rebase-resolver docket-integration-repair "

derived=""
while IFS= read -r name; do
  [ -n "$name" ] || continue
  derived="$derived $name"
done < <( ( cd "$REPO" && grep -rohE --include='*.md' '`[A-Za-z0-9_-]+`[^`]{0,20}subagent' skills/ ) \
            | grep -oE '`[A-Za-z0-9_-]+`' | tr -d '`')
while IFS= read -r name; do
  [ -n "$name" ] || continue
  derived="$derived $name"
done < <( ( cd "$REPO" && grep -rohE --include='*.md' 'resolved (build|review) skill' skills/ ) \
            | grep -oE 'build|review')

# Population floor: the derivation itself must still REACH the whole population. Without it,
# rewording a dispatch mention out of backticks (e.g. `` `docket-status` subagent `` -> `docket-status
# subagent`) silently drops it from $derived and the loop below simply iterates over fewer names —
# every reverse assert still reads PASS despite a derived site having gone missing.
# `-ge`, not `-eq`, against the count observed today: names REPEAT legitimately (the convention's
# *Composition* paragraph mentions most of them alongside each consuming site), so an exact count
# reddens on a second, harmless mention of an already-covered name — a false alarm, while the
# coverage loop below does the real work. MAINTAINER NOTE: to re-derive this floor legitimately,
# run the two greps above and count (`… | wc -w`); raise the number only after confirming every
# name in the new population is a check_site row or a $PENDING_TIER member. Never lower it to
# accommodate a deleted mention without checking the site itself is gone.
derived_count="$(printf "%s" "$derived" | wc -w)"
assert "reverse: derivation reached the whole observed dispatch-shape population (floor: >=11)" \
  '[ "$derived_count" -ge 11 ]'
# Pin the deferral: exactly the two finalize dispatches above, so tiering them (or adding a third
# untiered one) is an in-diff decision, never a silent one.
assert "reverse: PENDING_TIER holds exactly the two knowingly-untiered finalize dispatches" \
  '[ "$(printf "%s" "$PENDING_TIER" | wc -w)" -eq 2 ]'

for name in $derived; do
  # Token match, not substring: the surrounding spaces in both the pattern and the haystack keep a
  # phantom site named e.g. "docket" from being falsely reported as covered by " docket-status".
  assert "reverse: derived dispatch site '$name' is a check_site row or a PENDING_TIER member" \
    "grep -qF -- \" $name \" <<<\"\$all_nouns \$PENDING_TIER\""
done

# --- negative guard: no live prose gates a decision on a literal tool name -----------------------
# Shape, not an allowlist (AGENTS.md: never hand-list the sites of a literal you are gating). Keys
# on the tool-reference shapes this repo actually uses to denote the dispatch tool:
#   * markup-shaped — backticked `Task`, bolded **Task**, or a call form Task(
#   * BARE WORD narrowed by CONTEXT — `Task` immediately followed by dispatch/tool/launch. This
#     branch is not optional: it is this repo's OWN house idiom (AGENTS.md: "the spelling you miss
#     is the target file's own house idiom"). It was proven on references/agent-layer.md, whose
#     Cursor dispatch-rule sentence was written exactly that way until change 0135 reworded it by
#     capability ("forces a dispatch to the matching docket subagent"): appending
#     "Claude Code forces a Task dispatch to the matching wrapper, so the pin holds." to that file
#     stayed green under the markup-only pattern while the backticked twin reddened
#     (mutation-proven). The branch stays — the idiom it catches is still this repo's, and the
#     reword removed one instance, not the shape. Narrowing by the FOLLOWING WORD rather than by markup is what keeps this
#     repo's SDD vocabulary out ("Task 10", "each Task brief is reviewed…" do not match).
# A matching mention is legitimate only when its LINE's CONTENT (not its path) is Cursor-scoped,
# since Cursor documents a real `Task` tool and Claude Code does not.
#
# SCOPE IS AN INCLUSION LIST, not a set of exclusions: the scan walks exactly `skills/ README.md
# agents/ AGENTS.md`, and everything else is out of scope by never being walked — never by an
# exception entry inside the scan (AGENTS.md: no allowlist of exceptions to a gated literal). Why
# those four, and what is knowingly omitted:
#   * skills/ — the normative prose the gate is about; agents/ + AGENTS.md — maintained prose this
#     repo ships, carrying no matching mention today, so folding them in costs nothing and closes
#     the gap before one could land there unguarded; README.md — carries the one legitimate
#     Cursor-scoped mention the floor below pins.
#   * OMITTED, maintained: `docs/codex/setup.md`, `docs/opencode/setup.md` and `docs/cursor/*.md` — per-harness setup docs
#     that ARE maintained (not point-in-time records) and would legitimately belong in scope. They
#     carry no matching mention today (verified), so no live violation is being hidden; widening to
#     them is a follow-up, not a silent assumption.
#   * OMITTED, deliberately: `docs/adrs/` — an Accepted ADR is immutable except its `status:` line,
#     and the one wrong sentence there (0024) is corrected only by an appended dated `## Update`,
#     so widening this scan to it would make the assert permanently, unfixably red.
#     `cursor-rules/**` and sync-agents.sh's Cursor rule assembler — the OTHER harness's generated
#     dispatch-rule templates, where the literal tool name is correct as written.
#     `docs/superpowers/plans|specs`, `docs/results/`, `docs/changes/archive/` — point-in-time
#     records, where "Task" usually names SDD task numbering, not a tool.
#     Non-`.md` files under the scanned roots — out of scope via --include='*.md'.
SHAPE='(`Task`|\*\*Task\*\*|\bTask\(|Task[[:space:]]+(dispatch|tool|launch))'
scan_and_classify(){ # $1 = tree root; emits "<CURSOR|OFFENDER|UNCLASSIFIED> <path:lineno:content>"
  # ONE scan+classify implementation, called for BOTH the real repo and the positive-control tree
  # below — never a second, independently-written grep+classify (same trap and same fix as
  # tests/test_comment_anchor_style.sh's "ONE scan implementation" rationale). A parallel
  # re-implementation for the control lets a mutation to this classifier go untested, because the
  # control would keep grading itself with its own separate copy; routing both trees through this
  # one function means neutering the classification anywhere neuters it for the control too.
  local root="$1" line body
  ( cd "$root" 2>/dev/null \
      && grep -rnE -- "$SHAPE" --include='*.md' skills/ README.md agents/ AGENTS.md 2>/dev/null
  ) | while IFS= read -r line; do
    [ -n "$line" ] || continue
    # Strip grep's own "path:lineno:" prefix before classifying: $line is the FULL record, and
    # classifying on the full record blanket-exempts any PATH containing "cursor" regardless of
    # what the line's content actually says (mutation-proven: planting the exact corrected
    # violation inside skills/docket-convention/references/cursor-setup.md passed under a
    # full-line check).
    # Well-formedness FIRST, and it is not decoration: those two strips are correct only for a
    # `<path>:<lineno>:<content>` record where the path itself carries no colon. A path that does
    # (`skills/x:cursor-notes/SKILL.md`) leaves part of the PATH inside $body — which re-opens the
    # exact defect the strips exist to close, blanket-exempting the line because its *path* says
    # "cursor". Records that are not `<no-colon>:<digits>:` are therefore neither classified nor
    # silently dropped: they go to a third bucket that its own assert below requires to be empty.
    # This is what makes that assert genuinely INDEPENDENT of the offender assert — with only the
    # CURSOR/OFFENDER pair the loop is exhaustive by construction, so any count identity over the
    # two (e.g. the earlier `total -eq cursor_scoped`) restates the offender assert and cannot
    # redden on its own.
    if ! [[ "$line" =~ ^[^:]+:[0-9]+: ]]; then
      printf 'UNCLASSIFIED %s\n' "$line"
      continue
    fi
    body="${line#*:}"; body="${body#*:}"
    if grep -qi 'cursor' <<<"$body"; then
      printf 'CURSOR %s\n' "$line"
    else
      printf 'OFFENDER %s\n' "$line"
    fi
  done || true   # Minor 2: a zero-hit scan must never abort a future `set -e` caller
}

mentions_classified="$(scan_and_classify "$REPO")" || true
offenders=""; unclassified=""; cursor_scoped=0; total=0
while IFS= read -r rec; do
  [ -n "$rec" ] || continue
  total=$((total+1))
  hit="${rec#* }"
  echo "seen ${hit%%:*}:$(cut -d: -f2 <<<"$hit")"          # per-hit record, before any skip
  case "${rec%% *}" in
    CURSOR)   cursor_scoped=$((cursor_scoped+1)) ;;
    OFFENDER) offenders="$offenders
$hit" ;;
    *)        unclassified="$unclassified
$hit" ;;
  esac
done <<EOF
$mentions_classified
EOF

assert "no live prose names a dispatch tool outside a Cursor-scoped line" \
  '[ -z "$(printf %s "$offenders" | tr -d "[:space:]")" ]'
[ -z "$(printf %s "$offenders" | tr -d '[:space:]')" ] || printf 'offending lines:%s\n' "$offenders"
# Population floor: the scan must have reached live prose. Zero hits would pass the assert above
# vacuously — a path typo, a moved file, or an over-narrowed shape pattern must redden here, not
# silently guard nothing. Today's in-scope population is exactly ONE matching, Cursor-scoped
# mention — README's trade-off-table prose (backticked). It was TWO until change 0135 reworded
# references/agent-layer.md's Cursor dispatch-rule sentence by capability ("forces a dispatch to
# the matching docket subagent"), which no longer names a tool and so no longer matches $SHAPE at
# all; the sentence still lives, in place, saying the same thing. Re-derived from the scan itself
# per the rule below, never zeroed. The floor is the observed count, not padded.
# DO NOT lower this floor to 0 if a future legitimate reword of those sentences turns it red: 0 is
# vacuous (the assert above would then hold trivially with nothing left to scan), and a reworded
# sentence that stays in scope is exactly what this guard exists to keep noticing. Fix a red floor
# by confirming the sentences still live where they did (moved/reworded, not deleted) and, only
# then, re-deriving the new observed count from the scan itself — never by zeroing the floor.
assert "negative guard: scan reached live prose (floor: >=1 Cursor-scoped mention)" \
  '[ "$cursor_scoped" -ge 1 ]'
# Pin the floor's reach to a NAMED file, not just a bare count: a bare ">= 1" would stay green even
# if the scan's one hit migrated from README.md to something else entirely (a different file that
# happens to also carry a Cursor-scoped shape match) — the count alone cannot tell "the same
# mention moved" from "a coincidentally-shaped mention elsewhere covered for it". Anchored on
# $mentions_classified (every record the scan produced, before classification), not on $offenders
# or $cursor_scoped, so this coverage check is independent of the CURSOR/OFFENDER split above.
assert "negative guard: the scan's population includes README.md specifically" \
  'grep -qE "^(CURSOR|OFFENDER) README\.md:" <<<"$mentions_classified"'
# Genuinely independent reconciliation. Two earlier attempts at this assert were mathematically
# dead: "total >= 2" cannot redden without the floor above also reddening (cursor_scoped <= total
# always), and "total -eq cursor_scoped" was dead for a different reason — with only a CURSOR and an
# OFFENDER branch the loop is exhaustive, so total == cursor_scoped + |offenders| identically and
# that equality is just the main offender assert restated. The third UNCLASSIFIED bucket added to
# scan_and_classify is what makes a reconciliation possible at all: a record whose shape the
# classifier cannot trust (not `<colon-free path>:<lineno>:`) lands in NEITHER of the other two
# buckets, so this reddens while both asserts above stay green. Reachable, not theoretical: a skill
# file under a directory whose name contains a colon reddens exactly this and nothing else
# (mutation-proven), and dropping `-n` from the scan's grep reddens it for every record.
assert "negative guard: every scanned record was classified (no UNCLASSIFIED bucket)" \
  '[ -z "$(printf %s "$unclassified" | tr -d "[:space:]")" ]'
[ -z "$(printf %s "$unclassified" | tr -d '[:space:]')" ] || printf 'unclassified lines:%s\n' "$unclassified"

# Positive control: the guard must REPORT a planted violation, whatever the real tree looks like
# (learnings: marker-scoped-guard-needs-a-population-floor — coverage, not population). Routed
# through the SAME scan_and_classify as the real scan above: a parallel re-implementation here
# would leave the guard's own classifier unguarded (mutation-proven — see the function's rationale
# comment above).
ctl="$(mktemp -d)"; trap 'rm -rf "$ctl"' EXIT
mkdir -p "$ctl/skills/x"
printf 'A forked skill-invoke and an explicit agent dispatch (a `Task` naming the wrapper) are one.\n' \
  > "$ctl/skills/x/SKILL.md"
# $SHAPE has TWO independent halves — markup-shaped and bare-word-narrowed-by-context — and the
# control must plant one record for EACH, or half the pattern is unexercised. The bare-word half
# had exactly one live instance in the scanned tree (references/agent-layer.md's Cursor
# dispatch-rule sentence) until change 0135 reworded it by capability; with the live instance gone
# and only a backticked record planted here, deleting the whole `Task[[:space:]]+(dispatch|tool|
# launch)` alternation from $SHAPE left the entire suite green (mutation-proven). A HERMETIC
# control record is the right home for this coverage precisely because it does not depend on how
# any live document happens to be worded today.
printf 'Cursor forces a Task dispatch to the matching wrapper.\n' > "$ctl/skills/x/BARE.md"
: > "$ctl/README.md"
ctl_classified="$(scan_and_classify "$ctl")" || true
ctl_hits=""
while IFS= read -r rec; do
  [ -n "$rec" ] || continue
  if [ "${rec%% *}" = "OFFENDER" ]; then
    ctl_hits="$ctl_hits
${rec#* }"
  fi
done <<EOF
$ctl_classified
EOF
assert "negative guard: positive control — a planted non-Cursor Task line IS detected" \
  '[ -n "$ctl_hits" ]'
# Per-record coverage, one assert per $SHAPE half. Anchored on $ctl_classified (every record the
# scan produced) rather than $ctl_hits, because the bare-word record is Cursor-scoped and so
# classifies as CURSOR, not OFFENDER — the point here is that the SHAPE matched at all, which is
# what dies when an alternation is dropped from $SHAPE.
assert "negative guard: positive control — the markup-shaped Task record is detected" \
  'grep -qE "^(CURSOR|OFFENDER) skills/x/SKILL\.md:" <<<"$ctl_classified"'
assert "negative guard: positive control — the BARE-WORD Task record is detected" \
  'grep -qE "^(CURSOR|OFFENDER) skills/x/BARE\.md:" <<<"$ctl_classified"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
