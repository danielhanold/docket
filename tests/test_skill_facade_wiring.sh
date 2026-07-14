#!/usr/bin/env bash
# tests/test_skill_facade_wiring.sh — change 0072 (facade skill rewiring)
#
# Guards that live agent-facing skill prose invokes docket helpers ONLY through the
# docket.sh facade (canonical byte-exact spelling), that the retired shapes
# (eval "$(", inline `fetch origin`, `pull --rebase`) are gone from code spans, and that
# the Step-0 preamble + mid-run re-sync + CREATE_ORPHAN carve-out are PRESENT.
#
# SOUND-GUARD READING (load-bearing): the Layer-1 sweep guards non-facade INVOCATIONS,
# not every `.sh` token. Discriminator = the invocation prefix `${DOCKET_SCRIPTS_DIR`
# (every retired invocation carries it) + the retired shapes. Descriptive NOUN mentions
# (`board-refresh.sh`, `sync-agents.sh`, `render-board.sh`, `scripts/<name>.sh`) carry no
# prefix and are PERMITTED — this is what lets references/agent-layer.md pass without
# rewiring (spec §3), and avoids over-scoping into a reference doc about sync-agents.sh
# (spec §Out-of-scope). Stripping the canonical `…/docket.sh` first is what makes the
# `${DOCKET_SCRIPTS_DIR` assert sound (the canonical spelling contains install.sh in :?).
#
# This test is RED until Tasks 2-4 rewire the prose. That RED, per assert, is the
# reverse-mutation proof that each guard bites the pre-rewiring prose.
#
# HARNESS: this repo's test harness is self-contained (there is NO tests/lib/). The
# boilerplate below mirrors tests/test_docket_facade.sh verbatim: `set -uo pipefail`,
# a REPO root, `fail`, an inline `assert` that eval's a command STRING which must exit 0,
# and `exit $fail`.
#
# PORTABILITY / CORRECTNESS notes (deviations from the plan draft, all deliberate):
#   * The forbidden fixed-string patterns are held in single-quoted variables
#     (P_PREFIX/P_EVAL/P_FETCH/P_REBASE) and referenced as `grep -qF "$P_*"`. Writing the
#     pattern inline as `grep -qF "${DOCKET_SCRIPTS_DIR"` (as the draft did) is an
#     UNTERMINATED parameter expansion — a hard error under `set -u`, not a literal.
#   * strip_canonical uses BSD-sed-safe `-E` (verified on /usr/bin/sed) and extract_code_units
#     uses BSD-awk-safe match()/RSTART/RLENGTH (verified on /usr/bin/awk). Runtime `grep` here
#     is GNU (homebrew gnubin); all flags used (-oE/-qxF/-hoF/-cF) are common to both.
#   * The op-inventory rule emits ONE assert per in-scope file (accumulating any off-inventory
#     ops), so it is visible/non-vacuous rather than silent-unless-violated.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

CONV="$REPO/skills/docket-convention/SKILL.md"

# In-scope files (glob-derived; the glob IS the derivation, so no separate FLOOR is needed).
# skills/*/SKILL.md covers every operating skill + the convention; references/*.md adds the
# two convention reference docs. The two globs do not overlap.
SCOPE=( "$REPO"/skills/*/SKILL.md "$REPO"/skills/docket-convention/references/*.md )

# Op inventory: grep-derived from scripts/docket.md's Subcommand table (NEVER hand-listed;
# same derivation the facade's own sentinel uses).
INVENTORY="$(grep -oE '^\| `[a-z-]+` ' "$REPO/scripts/docket.md" | tr -d '|` ' | sort -u)"

# Forbidden fixed-string patterns, kept literal via single quotes (see PORTABILITY note).
P_PREFIX='${DOCKET_SCRIPTS_DIR'   # any surviving invocation prefix = a non-facade / non-byte-exact invocation
P_EVAL='eval "$('                 # the retired eval-preamble shape
P_FETCH='fetch origin'            # retired inline fetch
P_REBASE='pull --rebase'          # retired inline rebase

# Emit code UNITS for a file: fenced-block bodies (verbatim lines) + inline code spans.
extract_code_units() {
  awk '
    /^```/ { infence = !infence; next }
    infence { print; next }
    {
      line = $0
      while (match(line, /`[^`]*`/)) {
        print substr(line, RSTART, RLENGTH)
        line = substr(line, RSTART + RLENGTH)
      }
    }
  ' "$1"
}

# Strip the two byte-exact canonical forms so only the haystack remains. The bootstrap form
# is stripped BEFORE the plain facade form (docket-config.sh --bootstrap vs docket.sh differ
# in basename, so order is not load-bearing, but keep the specific form first).
strip_canonical() {
  sed -E \
    -e 's#"\$\{DOCKET_SCRIPTS_DIR:\?run docket/install\.sh\}"/docket-config\.sh --bootstrap##g' \
    -e 's#"\$\{DOCKET_SCRIPTS_DIR:\?run docket/install\.sh\}"/docket\.sh##g'
}

# ---- Layer 1: absence sweep, per in-scope file ----
for f in "${SCOPE[@]}"; do
  rel="${f#$REPO/}"
  units="$(extract_code_units "$f")"
  hay="$(printf '%s\n' "$units" | strip_canonical)"

  assert "no non-facade / non-byte-exact helper invocation prefix survives in $rel" \
    '! printf "%s" "$hay" | grep -qF "$P_PREFIX"'
  assert "no eval-preamble shape in code spans of $rel" \
    '! printf "%s" "$hay" | grep -qF "$P_EVAL"'
  assert "no inline \`fetch origin\` in code spans of $rel" \
    '! printf "%s" "$hay" | grep -qF "$P_FETCH"'
  assert "no inline \`pull --rebase\` in code spans of $rel" \
    '! printf "%s" "$hay" | grep -qF "$P_REBASE"'

  # every `docket.sh <op>` in this file's code units must name an inventory op
  # (checked on RAW units, incl. the canonical facade form).
  bad_ops=""
  while read -r op; do
    [ -z "$op" ] && continue
    printf '%s\n' "$INVENTORY" | grep -qxF "$op" || bad_ops="$bad_ops $op"
  done < <(printf '%s\n' "$units" | grep -oE 'docket\.sh [a-z-]+' | awk '{print $2}' | sort -u)
  assert "every \`docket.sh <op>\` in $rel names an inventory op (off-inventory:[$bad_ops])" \
    '[ -z "$bad_ops" ]'
done

# Bootstrap carve-out is unique across the in-scope set (Layer 1 rule 5): the CREATE_ORPHAN
# path is the ONLY place skill prose reaches docket-config.sh directly.
carve="$(grep -hoF 'docket-config.sh --bootstrap' "${SCOPE[@]}" | wc -l | tr -d ' ')"
assert "CREATE_ORPHAN carve-out (docket-config.sh --bootstrap) occurs exactly once in skill prose" \
  '[ "$carve" = "1" ]'

# ---- Layer 2: presence anchors on the convention (grep -c == 1) ----
assert "Step-0 preamble runs preflight as its own call (unique anchor)" \
  '[ "$(grep -cF "as its own Bash call" "$CONV")" = "1" ]'
assert "mid-run re-sync verb is defined once (unique anchor)" \
  '[ "$(grep -cF "push-retry CAS loops alike" "$CONV")" = "1" ]'
assert "Step-0 instructs reading the printed block (unique anchor)" \
  '[ "$(grep -cF "read the printed \`KEY=value\` block" "$CONV")" = "1" ]'

exit $fail
