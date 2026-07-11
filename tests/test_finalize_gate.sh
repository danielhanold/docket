#!/usr/bin/env bash
# tests/test_finalize_gate.sh — run: bash tests/test_finalize_gate.sh
# Sentinels for the finalize rebase-retest merge gate (change 0015). Sentinels are
# sampling, not parsing — paired with the whole-branch review. Each assert is written
# to flip to NOT OK if the clause it guards is removed (non-vacuous).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

FIN="$REPO/skills/docket-finalize-change/SKILL.md"
CONV="$REPO/skills/docket-convention/SKILL.md"
STAT="$REPO/skills/docket-status/SKILL.md"
DYML="$REPO/.docket.yml"

# ---- Config parse: the nested finalize.gate key, four modes + default ----------
# Block-scoped awk (the sync-agents.sh idiom), SIGPIPE-safe (capture, no producer|grep).
# Default is `local` (gate on by default); `off` is the documented opt-out.
gate_of(){  # $1 = path to a .docket.yml
  local v
  v="$(awk '
    /^finalize:[[:space:]]*$/{f=1;next}
    f&&/^[^[:space:]#]/{f=0}
    f&&/^[[:space:]]+gate[[:space:]]*:/{
      line=$0; sub(/#.*/,"",line); sub(/.*gate[[:space:]]*:[[:space:]]*/,"",line);
      gsub(/[[:space:]]/,"",line); print line; exit
    }' "$1" 2>/dev/null)"
  printf '%s' "${v:-local}"
}
TMPC="$(mktemp -d)"
printf 'finalize:\n  gate: local\n'  > "$TMPC/local.yml"
printf 'finalize:\n  gate: ci\n'     > "$TMPC/ci.yml"
printf 'finalize:\n  gate: both\n'   > "$TMPC/both.yml"
printf 'finalize:\n  gate: off\n'    > "$TMPC/off.yml"
printf 'metadata_branch: docket\n'   > "$TMPC/absent.yml"   # no finalize: block
assert "config-parse: gate local"            '[ "$(gate_of "$TMPC/local.yml")" = "local" ]'
assert "config-parse: gate ci"               '[ "$(gate_of "$TMPC/ci.yml")"    = "ci" ]'
assert "config-parse: gate both"             '[ "$(gate_of "$TMPC/both.yml")"  = "both" ]'
assert "config-parse: gate off (opt-out)"    '[ "$(gate_of "$TMPC/off.yml")"   = "off" ]'
assert "config-parse: absent block => local" '[ "$(gate_of "$TMPC/absent.yml")" = "local" ]'
rm -rf "$TMPC"

# ---- Config parse: the nested finalize.require_pr_approval key (default false) -
# Same block-scoped awk idiom as gate_of; SIGPIPE-safe (capture, no producer|grep).
rpa_of(){  # $1 = path to a .docket.yml ; echoes true|false (default false)
  local v
  v="$(awk '
    /^finalize:[[:space:]]*$/{f=1;next}
    f&&/^[^[:space:]#]/{f=0}
    f&&/^[[:space:]]+require_pr_approval[[:space:]]*:/{
      line=$0; sub(/#.*/,"",line); sub(/.*require_pr_approval[[:space:]]*:[[:space:]]*/,"",line);
      gsub(/[[:space:]]/,"",line); print line; exit
    }' "$1" 2>/dev/null)"
  printf '%s' "${v:-false}"
}
TMPR="$(mktemp -d)"
printf 'finalize:\n  require_pr_approval: true\n'  > "$TMPR/true.yml"
printf 'finalize:\n  require_pr_approval: false\n' > "$TMPR/false.yml"
printf 'finalize:\n  gate: local\n'                > "$TMPR/nokey.yml"   # finalize block, no rpa key
printf 'metadata_branch: docket\n'                 > "$TMPR/absent.yml"  # no finalize block
assert "rpa-parse: require_pr_approval true"            '[ "$(rpa_of "$TMPR/true.yml")"   = "true" ]'
assert "rpa-parse: require_pr_approval false"           '[ "$(rpa_of "$TMPR/false.yml")"  = "false" ]'
assert "rpa-parse: key absent in finalize => false"     '[ "$(rpa_of "$TMPR/nokey.yml")"  = "false" ]'
assert "rpa-parse: no finalize block => false"          '[ "$(rpa_of "$TMPR/absent.yml")" = "false" ]'
# A commented knob must parse as the default (commented line is not a key):
printf 'finalize:\n  # require_pr_approval: false\n'    > "$TMPR/commented.yml"
assert "rpa-parse: commented knob => default false"     '[ "$(rpa_of "$TMPR/commented.yml")" = "false" ]'
rm -rf "$TMPR"

# ---- finalize SKILL documents require_pr_approval with default false ----------
# Two sharp anchors (not one broad "require_pr_approval.*default.*false" that the YAML
# comment AND the prose both satisfy — dropping the substantive prose would leave it green):
#   (1) the config-block YAML knob line, and (2) the prose paragraph's unique sentence.
assert "finalize config block documents require_pr_approval default false" \
  'grep -Eqi "require_pr_approval: *false +#.*default false" "$FIN"'
assert "finalize prose explains require_pr_approval as the human-sign-off gate" \
  'grep -Eqi "validates .{1,3}human sign-off" "$FIN"'
assert "finalize ties require_pr_approval to the auto-detect path + unapproved PR" \
  'grep -q "reviewDecision != APPROVED" "$FIN"'

# ---- repo .docket.yml carries the knob (commented) at its default -------------
assert "repo .docket.yml mentions require_pr_approval (discoverability)" \
  'grep -q "require_pr_approval" "$DYML"'
assert "repo .docket.yml leaves require_pr_approval at default false" \
  '[ "$(rpa_of "$DYML")" = "false" ]'

# ---- Selection: ambiguity-only prompting (the §4.1 matrix) --------------------
# Anchor each assert to the UNIQUE phrase its matrix row owns (LEARNINGS #15) — not a
# broad keyword set that could latch onto step-1 prose. Each is a single-line grep so
# the two halves must co-occur in the same row.
assert "selection: exactly one eligible => no prompt" \
  'grep -Eqi "exactly one eligible.*no prompt" "$FIN"'
assert "selection: more than one eligible => prompt" \
  'grep -Eqi "more than one eligible.*prompt" "$FIN"'
assert "selection: surface-don't-merge an un-mergeable candidate" \
  'grep -Eqi "not git-mergeable.*surface, do not merge" "$FIN"'
assert "selection: surface-don't-merge an unapproved PR under the policy" \
  'grep -Eqi "require_pr_approval.{0,40}surface, do not merge|reviewDecision != APPROVED.{0,80}surface, do not merge" "$FIN"'
# ---- §4.2 explicit id overrides the approval policy --------------------------
assert "selection: explicit id overrides require_pr_approval" \
  'grep -Eqi "explicit id overrides .{0,4}require_pr_approval|explicit id.{0,40}overrides.{0,40}require_pr_approval" "$FIN"'

# ---- finalize SKILL gates on finalize.gate ------------------------------------
assert "finalize references the finalize.gate config" 'grep -Eq "finalize\.gate|finalize:" "$FIN"'
assert "finalize names all four gate modes" \
  'grep -q "local" "$FIN" && grep -q "ci" "$FIN" && grep -q "both" "$FIN" && grep -qE "\boff\b" "$FIN"'
assert "finalize: off restores today's no-rebase behavior" 'grep -Eqi "off[^.]*(today|no rebase|no re-test|trust)" "$FIN"'

# ---- dispatches the two agents at the right triggers --------------------------
assert "finalize dispatches docket-rebase-resolver on conflict" 'grep -q "docket-rebase-resolver" "$FIN"'
assert "rebase-resolver dispatch is tied to a rebase conflict" \
  'grep -Eqi "conflict[^.]*docket-rebase-resolver|docket-rebase-resolver[^.]*conflict" "$FIN"'
assert "finalize dispatches docket-integration-repair on red tests" 'grep -q "docket-integration-repair" "$FIN"'
assert "integration-repair dispatch is tied to a red/failed suite" \
  'grep -Eqi "(red|fail)[^.]*docket-integration-repair|docket-integration-repair[^.]*(red|fail)" "$FIN"'

# ---- local validation runs BEFORE the force-push (ordering is the contract) ----
assert "finalize force-pushes with --force-with-lease" 'grep -q "force-with-lease" "$FIN"'
local_ln="$(grep -ni "before any push" "$FIN" | head -n1 | cut -d: -f1)"
push_ln="$(grep -ni "force-with-lease" "$FIN" | head -n1 | cut -d: -f1)"
assert "finalize states local validation precedes the push" '[ -n "$local_ln" ] && [ -n "$push_ln" ] && [ "$local_ln" -lt "$push_ln" ]'

# ---- §6 sign-off: interactive prompt vs autonomous abort-and-report -----------
assert "finalize documents repair sign-off" 'grep -qi "sign-off" "$FIN"'
assert "finalize: interactive sign-off prompts before merge" 'grep -Eqi "interactive[^.]*(prompt|sign-off)" "$FIN"'
assert "finalize: autonomous repair aborts-and-reports" 'grep -Eqi "autonomous[^.]*abort-and-report" "$FIN"'

# ---- §7 abort-and-report set (the full list of stop points) -------------------
ab="$(grep -ci "abort-and-report" "$FIN")"
assert "finalize names abort-and-report multiple times" '[ "$ab" -ge 3 ]'
assert "abort path: ambiguous rebase conflict"     'grep -Eqi "ambiguous[^.]*conflict|conflict[^.]*ambiguous" "$FIN"'
assert "abort path: no detectable test suite"      'grep -Eqi "no[^.]*suite|suite[^.]*not[^.]*found|no[^.]*test_command" "$FIN"'
assert "abort path: cannot reach green in <=2"      'grep -Eqi "two attempts|<=2|cannot reach green|stuck" "$FIN"'
assert "abort path: force-with-lease rejected"      'grep -Eqi "lease[^.]*reject|reject[^.]*lease|concurrent push" "$FIN"'

# ---- LEARNINGS #17: no model/effort literal in the dispatch prose -------------
assert "finalize body restates NO model alias literal" '! grep -qiE "\b(opus|sonnet|haiku|fable)\b" "$FIN"'
assert "finalize body restates NO effort literal" '! grep -qiE "\bxhigh\b" "$FIN"'
assert "finalize names the wrapper as the tier source" 'grep -Eqi "model/effort its wrapper resolves|its wrapper resolves" "$FIN"'

# ---- docket repo dogfoods the gate -------------------------------------------
assert "repo .docket.yml sets finalize gate to local" \
  '[ "$(gate_of "$DYML")" = "local" ] && grep -Eq "^finalize:" "$DYML" && grep -Eq "^[[:space:]]+gate[[:space:]]*:[[:space:]]*local" "$DYML"'

# ---- convention documents the gate + the two new wrappers --------------------
assert "convention documents finalize.gate" 'grep -Eqi "finalize\.gate|finalize:" "$CONV" && grep -qi "gate" "$CONV"'
assert "convention names the four gate modes" \
  'grep -Eqi "local[^.]*ci[^.]*both[^.]*off|gate.*off.*opt" "$CONV"'
assert "convention names docket-rebase-resolver" 'grep -q "docket-rebase-resolver" "$CONV"'
assert "convention names docket-integration-repair" 'grep -q "docket-integration-repair" "$CONV"'
assert "convention count prose says nine wrappers" 'grep -qi "nine" "$CONV"'
assert "convention names the no-convention consultant wrapper" 'grep -q "docket-brainstorm-consultant" "$CONV"'
# Non-vacuous count guard: the "five skills get a wrapper" language must stay exact.
assert "convention keeps 'five skills get a wrapper' exact" 'grep -qi "five .*skills.* get a wrapper" "$CONV"'

# ---- docket-status notes the gate is finalize-only ---------------------------
assert "status notes the rebase-retest gate is finalize-only" \
  'grep -Eqi "finalize-only|the sweep[^.]*never merges|only archives already-merged" "$STAT"'

exit $fail
