#!/usr/bin/env bash
# tests/test_sync_agents_run_gate.sh — the caller-side run gate is single-sourced and rendered
# identically into every parent-facing surface (change 0242).
# run: bash tests/test_sync_agents_run_gate.sh
#
# Change 0334: the gate no longer teaches a hand-executed attribution procedure. The mechanics
# (attribution, durable state, retry accounting, the detached-dispatch branches, the epoch/
# claimed_at filters) moved into a durable Go store, exercised by tests/test_gate_facade.sh and the
# runner-dispatch / verify-run suites. Change 0369: the always-loaded PAYLOAD now invokes that store
# through the bare installed binary — `docket run gate-before` / `docket run gate-verdict` — on PATH,
# not the `docket.sh` facade wrappers (0334 exec-delegators, still covered by test_gate_facade.sh).
# What the payload must carry is only the five compact parent instructions: arm before dispatch,
# read the verdict after, obey the report line, act only on `gate-retry-once`, and never hand-
# reimplement attribution. The guards below assert that compact payload and — as importantly —
# assert that the removed procedure STAYS removed (learnings: assert-detects-removal-not-
# replacement); an absence guard keyed on new wording would go green the day the old procedure
# crept back under a different phrasing.
#
# Mutation checks (run by hand at the build gate, learnings: guard-is-code):
#   * restore one old detached-dispatch sentence into cursor-rules/run-gate.md — e.g. re-add a line
#     mentioning `DISPATCH_EPOCH`, `--with-claimed-at`, `ALL THREE filters`, or a `### Detached
#     dispatch` heading — and a NEGATIVE assert below reddens.
#   * delete the `docket run gate-before implement-next` line from the payload — and the POSITIVE
#     "arms the gate before dispatch" assert reddens.
#   * restore a `docket.sh gate-before|gate-verdict` facade spelling — and the NEGATIVE
#     "retired docket.sh gate facade spelling is gone" assert reddens.
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SYNC="$REPO/sync-agents.sh"
GATE_SRC="$REPO/cursor-rules/run-gate.md"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# Collapse runs of whitespace so an assert about a CLAIM survives a pure re-flow of the prose
# (learnings: phrase-grep-over-wrapped-prose).
flat(){ tr '\n' ' ' < "$1" | tr -s '[:space:]' ' '; }

mk_repo(){  # $1 = agent_harnesses list body, e.g. "[claude, codex]"
  SBX="$(mktemp -d "${TMPDIR:-/tmp}/rungate.XXXXXX")"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
  printf 'agent_harnesses: %s\n' "$1" > "$SBX/.docket.yml"
  ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
}

# --- the template source exists and is the ONLY place the gate text is authored ---
assert "run-gate template exists" '[ -f "$GATE_SRC" ]'
# The gate's own commands must not be hand-copied into either assembler. Anchored on the
# distinctive facade flag, which appears in the template and nowhere else in the generator
# (change 0334: `--in-progress-ids` retired with the hand-executed procedure; `--unattributed` is
# the current distinctive flag, and sync-agents.sh must never restate it inline).
assert "sync-agents.sh does not restate the gate commands inline" \
  '[ "$(grep -c -- "--unattributed" "$SYNC")" = "0" ]'

# --- brevity: the block rides always-loaded context in every harness (spec Risks) ---
GATE_LINES="$(grep -c "" "$GATE_SRC" 2>/dev/null || echo 0)"
# Change 0334 (this change) LOWERS the ceiling: the hand-executed procedure — steps 1-4 plus the
# whole `### Detached dispatch` section with its two state-keyed branches and the epoch/claimed_at
# filter arithmetic — is gone, replaced by five compact facade instructions. Pre-change actual was
# 52 lines; the compact payload is 25. The ceiling is set at the new actual, strictly below the old
# (learnings: size-target-is-direction — the always-loaded block only ever gets to shrink here). A
# reflow that spills past 25 is a signal to re-compact, not to raise the bound.
assert "gate text is at most 25 lines" '[ "$GATE_LINES" -ge 1 ] && [ "$GATE_LINES" -le 25 ]'

# --- the behavioral claims, each bound to what it is asserted ABOUT ---
# (learnings: prose-guard-binds-phrase-to-claim — never a bare phrase-presence grep)
G="$(flat "$GATE_SRC" 2>/dev/null)"

# The header window states the FRAMING: a child completion notification is the child's claim, not
# the session's own report, and the facade — not the reader — owns attribution/state/retry. Bound
# to the HEADER window (everything before item 1's opener), not to $G — "facade" and "attribution"
# recur below.
HEADW="${G%%1. Before dispatching*}"
assert "gate: the header window exists to be asserted about" \
  '[ -n "$HEADW" ] && [ "$HEADW" != "$G" ]'
assert "gate: a completion notification is named as the CHILD claim, not the session output" \
  '[[ "$HEADW" == *"the CHILD"*"claim, not your report"* ]]'
assert "gate: the facade — not the reader — owns attribution, state and retry" \
  '[[ "$HEADW" == *"facade owns attribution"*"never hand-reimplement"* ]]'

# Item 1 — arm the gate BEFORE dispatch, and keep the key out of a shell variable. This is the
# positive that the header's mutation note pairs with: delete the `gate-before` line and it reddens.
assert "gate: item 1 arms the gate before dispatching implement-next" \
  '[[ "$G" == *"Before dispatching"*"gate-before implement-next"* ]]'
assert "gate: item 1 says to keep the printed key in notes, not a shell variable" \
  '[[ "$G" == *"gate-before implement-next"*"printed key"*"shell variable does not survive"* ]]'

# Item 2 — read the verdict AFTER the run, with the keyless `--unattributed` fallback. Both the
# keyed and the fallback command are asserted; a payload that dropped the fallback would leave a
# keyless session with no runnable read.
assert "gate: item 2 reads the verdict after the run returns" \
  '[[ "$G" == *"After the run returns"*"gate-verdict <key>"* ]]'
assert "gate: item 2 gives the keyless --unattributed fallback" \
  '[[ "$G" == *"Without a key"*"gate-verdict --unattributed"* ]]'

# Item 3 — obey the facade report line, never the exit code or the child prose. Bound so the
# clause names all three of what NOT to trust.
assert "gate: item 3 obeys the gate-* report line, never exit code or child prose" \
  '[[ "$G" == *"gate-*"*"report line"*"never its exit code"*"never the child"*"prose"* ]]'

# Item 4 — ONLY `gate-retry-once` licenses a re-dispatch, once, keeping the same key; every stop/
# observe verdict forbids it, with `run-halted` bound to "human" and `run-waiting` bound to "stop".
# The retry sentence is bound to the SAME numbered item as "once" and "same key" through a window
# cut at item 5's opener (learnings: prose-guard-binds-phrase-to-claim): a bare `gate-retry-once`
# presence grep would survive the sentence being gutted of its one-shot/same-key constraint.
ITEM4="${G#*gate-retry-once}"; ITEM4="${ITEM4%%Never hand-reimplement*}"
assert "gate: item 4 (gate-retry-once) exists to be asserted about" \
  '[ -n "$ITEM4" ] && [ "$ITEM4" != "$G" ]'
assert "gate: gate-retry-once is the ONLY re-dispatch licence, once, keeping the same key" \
  '[[ "$ITEM4" == *"once"* ]] && [[ "$ITEM4" == *"same key"* ]]'
assert "gate: run-halted means a human is needed" \
  '[[ "$G" == *"run-halted"*"human"* ]]'
assert "gate: run-waiting names a continuation, then stop — never a fresh dispatch" \
  '[[ "$G" == *"run-waiting"*"NOT resume"*"stop"* ]]'
assert "gate: gate-stop and gate-observe both forbid re-dispatch" \
  '[[ "$G" == *"gate-stop"*"gate-observe"*"forbids re-dispatch"* ]]'

# Item 5 — the never-rule the vocabulary cannot carry alone: attribution is never hand-reimplemented
# and permission is never inferred from child prose, launch shape, timestamps, ids, or exit codes.
# This is the compact replacement for the entire detached-dispatch procedure, so it must enumerate
# the signals a reader might otherwise improvise a re-attribution from.
assert "gate: item 5 forbids hand-reimplementing attribution or inferring permission from signals" \
  '[[ "$G" == *"Never hand-reimplement attribution"*"infer permission"* ]]'
assert "gate: item 5 names the signals that do NOT authorize a re-dispatch" \
  '[[ "$G" == *"child prose"*"launch shape"*"timestamps"*"ids"*"exit codes"* ]]'

# Change 0369: the payload invokes the bare installed `docket` binary on PATH — the two gate
# commands are `docket run gate-before` / `docket run gate-verdict`, with NO DOCKET_SCRIPTS_DIR
# facade prefix and NO `docket.sh` wrapper spelling surviving. Positive floor plus a shape-keyed
# removal assert (learnings: assert-detects-removal-not-replacement): a guard keyed only on the new
# wording would go green the day a `docket.sh gate-*` spelling crept back.
assert "gate: item 1 arms via the Go verb 'docket run gate-before implement-next'" \
  'grep -qF "docket run gate-before implement-next" "$GATE_SRC"'
assert "gate: item 2 reads the verdict via the Go verb 'docket run gate-verdict'" \
  'grep -qF "docket run gate-verdict" "$GATE_SRC"'
assert "gate: retired docket.sh gate facade spelling is gone from the source" \
  '! grep -E -e "docket\.sh[[:space:]]+gate-(before|verdict)" "$GATE_SRC"'
assert "gate: no DOCKET_SCRIPTS_DIR facade prefix survives in the gate source" \
  '[[ "$G" != *"DOCKET_SCRIPTS_DIR"* ]]'

# --- NEGATIVE: the hand-executed procedure STAYS removed (learnings:
# assert-detects-removal-not-replacement). Keyed on the OLD state's own load-bearing tokens, not on
# the new wording — a guard that keyed on new wording would pass while the old procedure crept back
# under a different phrasing. Scoped to the managed block ($G is the single-sourced payload; the
# verbatim-render asserts below prove every surface equals it), never the whole repo — the tokens
# below still legitimately live in scripts/verify-run.* and the runner-dispatch suites, which own
# the behavior now.
assert "gate: no DISPATCH_EPOCH — the epoch filter moved behind the facade" \
  '[[ "$G" != *"DISPATCH_EPOCH"* ]]'
assert "gate: no --with-claimed-at — the claimed_at read moved behind the facade" \
  '[[ "$G" != *"--with-claimed-at"* ]]'
assert "gate: no three-filter attribution procedure" \
  '[[ "$G" != *"ALL THREE filters"* ]] && [[ "$G" != *"three filters"* ]]'
assert "gate: no '\''### Detached dispatch'\'' section survives" \
  '[[ "$G" != *"Detached dispatch"* ]]'
# The retired hand-executed reads themselves must not linger either: the whole point of the facade
# is that the payload no longer teaches `verify-run --in-progress-ids` or the metadata `preflight`
# re-sync dance.
assert "gate: no hand-executed verify-run --in-progress-ids read survives" \
  '[[ "$G" != *"verify-run --in-progress-ids"* ]]'

# --- rendered into BOTH surfaces, byte-identically ---
mk_repo "[cursor, codex]"
CUR="$SBX/.cursor/rules/docket-dispatch.mdc"
AGM="$SBX/AGENTS.md"
assert "cursor rule was generated"   '[ -f "$CUR" ]'
assert "AGENTS.md block was written" '[ -f "$AGM" ]'

# Slice the gate out of each rendered surface by its own heading, bounded by the TEMPLATE'S OWN
# LINE COUNT (learnings: section-slice-needs-a-named-terminator — and here the two surfaces share
# no terminator: the cursor rule ends the gate with the next `## ` fragment heading, while the
# AGENTS.md block ends it with a bullet list. A count taken from the single source is the one
# bound both surfaces share, and it also makes the slice a VERBATIM-rendering check rather than a
# same-prefix check).
slice_gate(){ awk -v n="$GATE_LINES" '/^## Run gate/{g=1} g{print; if (++c==n) exit}' "$1"; }
slice_gate "$CUR" > "$SBX/.gate-cursor"
slice_gate "$AGM" > "$SBX/.gate-agents"
assert "gate is present in the cursor rule"    '[ -s "$SBX/.gate-cursor" ]'
assert "gate is present in the AGENTS.md block" '[ -s "$SBX/.gate-agents" ]'
assert "the cursor rule renders the template verbatim" \
  'diff -q "$GATE_SRC" "$SBX/.gate-cursor" >/dev/null'
assert "the AGENTS.md block renders the template verbatim" \
  'diff -q "$GATE_SRC" "$SBX/.gate-agents" >/dev/null'
assert "the two rendered gates are byte-identical" \
  'diff -q "$SBX/.gate-cursor" "$SBX/.gate-agents" >/dev/null'
# --- runnability: the migrated gate runs the bare `docket` binary on PATH (change 0369) ---
# The parent session reading this managed block invokes `docket` directly — no `docket.sh` facade
# spelling (bare or DOCKET_SCRIPTS_DIR-prefixed) survives into either rendered surface. Keyed on
# shape, not an enumerated command list: zero `docket.sh` occurrences, and the distinctive Go verb
# present at least once so the payload still carries a runnable command.
sh_total(){ grep -oE 'docket\.sh' "$1" | grep -c ""; }
for rendered in "$SBX/.gate-cursor" "$SBX/.gate-agents"; do
  T="$(sh_total "$rendered")"
  assert "rendered gate ($(basename "$rendered")) invokes the Go verb 'docket run gate-before'" \
    'grep -qF "docket run gate-before implement-next" "$rendered"'
  assert "rendered gate ($(basename "$rendered")) has no docket.sh facade spelling" '[ "$T" = "0" ]'
done

# The AGENTS.md block must still close: the gate is spliced INSIDE the managed block, and an
# unterminated block is a corrupt managed region, not a rendering detail.
assert "the AGENTS.md dispatch end marker exists" 'grep -q "docket:dispatch:end" "$AGM"'

# --- section ORDER: head, routing rule, then the gate — and NO roster (change 0334) ---
# These are markdown documents with headings, so order is structure. Since change 0334 the per-agent
# roster is GONE: the head carries a compact routing rule that defers to the harness's own registry,
# and the gate is the very next `## ` heading. Change 0242 finding 10 (the gate must not sit BETWEEN
# the head and the roster) is subsumed — there is no roster to sit above any more. The Cursor rule is
# asserted SEPARATELY: its per-agent fragments each keep their own `## docket-<name>` heading, so its
# gate-above-fragments order is pinned rather than assumed away.
# Line numbers via awk, never `grep -n | head` (AGENTS.md, shell: SIGPIPE under pipefail).
first_line(){ awk -v p="$1" '$0 ~ p { print NR; exit }' "$2"; }
AGM_HEAD="$(first_line '^## Docket agents' "$AGM")"
AGM_GATE="$(first_line '^## Run gate' "$AGM")"
assert "AGENTS.md: head and gate are both present to be ordered" \
  '[ "$AGM_HEAD" -ge 1 ] && [ "$AGM_GATE" -ge 1 ]'
assert "AGENTS.md: the gate comes AFTER the dispatch head" '[ "$AGM_GATE" -gt "$AGM_HEAD" ]'
# The gate must be the NEXT `## ` heading after the head — nothing else, and no roster, sits between
# the head's routing rule and the gate. Stronger than "the gate is elsewhere", and it stays true if a
# third section is ever added.
AGM_BETWEEN="$(awk -v a="$AGM_HEAD" -v b="$AGM_GATE" 'NR>a && NR<b && /^## /' "$AGM")"
assert "AGENTS.md: no heading separates the dispatch head from the gate" '[ -z "$AGM_BETWEEN" ]'
# The roster is REMOVED: no interior line is a `- **docket-...` bullet (SHAPE, not a spelling list;
# learnings: assert-detects-removal-not-replacement). Bracket expressions, never `\*`: awk's -v does
# escape processing, and `\*` is an undefined escape BWK awk (macOS) drops, leaving `**` — a pattern
# that matches no bullet and passes vacuously. Captured into a var, so no `producer | grep -q`.
AGM_BULLETS="$(awk '/^- [*][*]docket-/' "$AGM")"
assert 'AGENTS.md: the roster is gone — no `- **docket-` bullet survives' '[ -z "$AGM_BULLETS" ]'
CUR_GATE="$(first_line '^## Run gate' "$CUR")"
CUR_FRAG="$(first_line '^## docket-' "$CUR")"
assert "cursor rule: its own order is unchanged — gate above the per-agent sections" \
  '[ "$CUR_GATE" -ge 1 ] && [ "$CUR_FRAG" -ge 1 ] && [ "$CUR_GATE" -lt "$CUR_FRAG" ]'

# --- reachability: the gate arrives at a Claude parent, not merely at a template ---
# Re-anchored on the new distinctive facade command (change 0334): `gate-before implement-next` is
# the first runnable step of the payload and appears on the Claude surface only through the gate.
mk_repo "[claude]"
assert "reachability: a claude-only repo has a Claude surface" '[ -e "$SBX/CLAUDE.md" ]'
assert "reachability: the gate is readable through that surface" \
  'grep -q -- "gate-before implement-next" "$SBX/CLAUDE.md"'

# --- the convention's pointer names the gate and binds it to the verification obligation ---
# Bound to the Composition paragraph itself, not the whole file: `verify-run` and `once` both occur
# elsewhere in the convention, so a whole-file window matches even with the pointer sentence deleted
# (mutation-proven vacuous). The paragraph is the named terminator here — it is one physical line.
# NOTE (change 0334): the convention pointer is a SEPARATE artifact (skills/docket-convention/
# SKILL.md), out of this task's scope; this asserts it still names the managed-block gate, and stays
# green because that file is untouched here.
CONV="$REPO/skills/docket-convention/SKILL.md"
C="$(grep -m1 '^\*\*Composition' "$CONV" | tr -s '[:space:]' ' ')"
assert "convention: the Composition paragraph exists to be asserted about" '[ -n "$C" ]'
assert "convention: Composition points at the managed-block gate" \
  '[[ "$C" == *"uncommitted working-tree files"*"verify-run"*"once"* ]]'

# --- currency: THIS repo's own committed dispatch block is not stale ---------
# Everything above proves the generator renders the gate correctly into a SANDBOX. None of it looks
# at the block committed in docket's own AGENTS.md — and that block is not regenerated by any test
# or CI step, because check_project_level's surface leg is gated on per_repo_opted_in, which reads
# `agent_harnesses:`/`agents:` from .docket.yml or .docket.local.yml. This repo's COMMITTED
# .docket.yml carries neither (the opt-in lives in the gitignored, machine-local .docket.local.yml),
# so a bare `bash sync-agents.sh` here is a no-op and nothing reddens when the template moves on.
# That is not hypothetical: change 0242 fixed the gate text four times and the committed block
# carried the pre-fix wording the whole way. Since change 0334 the block no longer embeds the agent
# roster, so the drift it can accumulate is the gate/routing-rule prose — which this guard catches.
#
# These are pure reads of the repo under test — no sandbox, no writes. Sourcing is how
# tests/test_sync_agents.sh reaches generator internals; sync-agents.sh guards its main on
# `[ "${BASH_SOURCE[0]}" = "${0}" ]`, so a source defines the functions without generating anything.
# It is done in a SUBSHELL because sync-agents.sh sets errexit and assigns its own REPO="$PWD".
RG_WANT="$(mktemp "${TMPDIR:-/tmp}/rungate-want.XXXXXX")"
RG_HAVE="$(mktemp "${TMPDIR:-/tmp}/rungate-have.XXXXXX")"
(
  cd "$REPO" || exit 1
  # shellcheck source=/dev/null
  . "$SYNC"
  set +e   # sync-agents.sh enables errexit for direct invocation; this test intentionally does not.
  assemble_agents_md_dispatch > "$RG_WANT"
  _docket_gi_current_block "$REPO/AGENTS.md" "$DISPATCH_START" "$DISPATCH_END" > "$RG_HAVE"
) >/dev/null 2>&1
assert "this repo's own AGENTS.md carries a docket dispatch block" '[ -s "$RG_HAVE" ]'
assert "a block could be assembled to compare it against" '[ -s "$RG_WANT" ]'
# `diff` is the expression itself, so a failure prints the exact drift rather than a bare NOT OK.
# To fix a red here, the opt-in the generator needs must exist while it runs: write
# `agent_harnesses: [claude, cursor]` into this repo's gitignored .docket.local.yml, run
# `bash sync-agents.sh`, then DELETE that file again. A bare run without it changes nothing.
assert "this repo's committed AGENTS.md block is current — see the regeneration recipe above" \
  'diff -u "$RG_HAVE" "$RG_WANT"'
# The Claude surface is the SAME PHYSICAL FILE, which is what lets one regeneration serve both and
# what keeps sync_dispatch_surfaces' strip pass from deleting the live block through the alias.
# `-ef` compares device+inode after resolving the link, so it asserts resolution, not link shape.
assert "this repo's CLAUDE.md resolves to its AGENTS.md" \
  '[ -e "$REPO/CLAUDE.md" ] && [ "$REPO/CLAUDE.md" -ef "$REPO/AGENTS.md" ]'
rm -f "$RG_WANT" "$RG_HAVE"

echo; [ "$fail" = 0 ] && echo "ALL PASS" || echo "FAILURES"; exit "$fail"
