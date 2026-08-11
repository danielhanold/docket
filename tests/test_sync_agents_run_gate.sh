#!/usr/bin/env bash
# tests/test_sync_agents_run_gate.sh — the caller-side run gate is single-sourced and rendered
# identically into every parent-facing surface (change 0242).
# run: bash tests/test_sync_agents_run_gate.sh
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SYNC="$REPO/sync-agents.sh"
GATE_SRC="$REPO/cursor-rules/run-gate.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

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
# distinctive flag, which appears in the template and nowhere else in the generator.
assert "sync-agents.sh does not restate the gate commands inline" \
  '[ "$(grep -c -- "--in-progress-ids" "$SYNC")" = "0" ]'

# --- brevity: the block rides always-loaded context in every harness (spec Risks) ---
GATE_LINES="$(grep -c "" "$GATE_SRC" 2>/dev/null || echo 0)"
# Bound raised 14 -> 18 (change 0242 review, finding 1): every command must carry the convention's
# mandatory facade prefix, since `docket.sh` is on no PATH. Correctness over the plan-time guess.
# Raised 18 -> 23 (finding 3): the symmetric metadata re-sync — two `preflight` commands at full
# facade spelling, plus the sentence saying why both reads must be fresh.
# Raised 23 -> 25 (finding 4): the multi-candidate abort clause in step 3 — the cardinality rule
# scripts/runner-dispatch.sh enforces ("this run claims at most one") had no prose counterpart.
# Raised 25 -> 43 (change 0275): the gate had NO runnable path for a dispatch the session did not
# foreground-block on — the shape a human actually launches (`/docket-implement-next <id>`), where
# steps 1-3 are structurally unrunnable because no before-snapshot exists. The `### Detached
# dispatch` section is that path, and it cannot be shorter than the two branches it must keep
# apart: one that CAN attribute (before-set + DISPATCH_EPOCH, all three filters, re-dispatch
# allowed) and one that CANNOT (verify-and-report, never re-dispatch). Collapsing them is the
# defect — an unattributed re-dispatch lands on a change a live agent is holding. The raise is
# deliberate and priced: an always-loaded block earns its length only by being runnable, and 18
# lines is what the observed-and-ungated dispatch shape costs.
assert "gate text is at most 43 lines" '[ "$GATE_LINES" -ge 1 ] && [ "$GATE_LINES" -le 43 ]'

# --- the behavioral claims, each bound to what it is asserted ABOUT ---
# (learnings: prose-guard-binds-phrase-to-claim — never a bare phrase-presence grep)
G="$(flat "$GATE_SRC" 2>/dev/null)"
# The gate's framing must survive notification-driven control flow: a session HANDED a completion
# report never "reports" in the sense the old heading meant, so "verify before you report" bound
# nothing on the very path change 0275 exists to cover. Bound to the HEADER window (everything
# before step 1's opener), not to $G — the words recur in the steps below.
HEADW="${G%%Before dispatching*}"
assert "gate: the header window exists to be asserted about" \
  '[ -n "$HEADW" ] && [ "$HEADW" != "$G" ]'
assert "gate: the obligation is stated against RELAYING, not against reporting" \
  '[[ "$HEADW" == *"before you relay it"* ]]'
assert "gate: a completion notification is named as the CHILD claim, not the session output" \
  '[[ "$HEADW" == *"the CHILD"*"claim, not your report"*"before relaying"* ]]'
assert "gate: snapshots the in-progress set BEFORE dispatching" \
  '[[ "$G" == *"Before dispatching"*"verify-run --in-progress-ids"* ]]'
assert "gate: verifies the attributed id after the return" \
  '[[ "$G" == *"After the return"*"verify-run <id>"* ]]'
# `verify-run --in-progress-ids` is a pure LOCAL reader, so an asymmetric re-sync attributes an
# earlier session's abandoned claim to this run (scripts/runner-dispatch.sh: "not merely imprecise,
# it is actively wrong"). Each side's re-sync is bound to ITS OWN snapshot, not merely present:
# BEFORE_HALF is the prose up to the FIRST `--in-progress-ids`, so a step-1 preflight cannot be
# satisfied by step 3's, and the "After the return" window is already past step 1's.
BEFORE_HALF="${G%%verify-run --in-progress-ids*}"
assert "gate: re-syncs metadata before the BEFORE snapshot" \
  '[[ "$BEFORE_HALF" == *"Before dispatching"*"docket.sh preflight"* ]]'
assert "gate: re-syncs metadata again before the AFTER snapshot" \
  '[[ "$G" == *"After the return"*"docket.sh preflight"*"verify-run --in-progress-ids"* ]]'
assert "gate: says both snapshots must read fresh origin state" \
  '[[ "$G" == *"both snapshots"*"FRESH ORIGIN"* ]]'
# Step 2 must carry its OWN blocking claim on every surface. It used to say "as above", which
# resolves only in the cursor rule — assemble_dispatch_rule splices the gate after
# cursor-rules/dispatch.head.md, whose "Required dispatch pattern" item 2 supplies the foreground /
# never-poll directive. assemble_agents_md_dispatch splices the same text after a head that never
# mentions blocking, so on the Claude and AGENTS.md surfaces the reference dangled — and a yielded
# dispatch returns a half-done run the caller reads as completed, which is the exact failure this
# gate exists to catch. Bound to the STEP-2 window (from the end of step 1's snapshot command to
# step 3's opener), not to $G: the words below also occur in the dispatch head that precedes the
# gate in one rendering, so a whole-file grep would be vacuous.
STEP2="${G#*verify-run --in-progress-ids}"; STEP2="${STEP2%%After the return*}"
assert "gate: step 2 exists to be asserted about" '[ -n "$STEP2" ] && [ "$STEP2" != "$G" ]'
# Change 0275: the prohibition is SCOPED rather than dropped. A blanket "never background it" on
# the same page as the Detached section countermands it, and the prohibition is mutation-tested —
# so it now binds the path it can bind (a dispatch this session issues and can block on) and
# routes every other shape to the named section instead of forbidding it.
assert "gate: step 2 scopes foreground-and-block to a dispatch this session can block on" \
  '[[ "$STEP2" == *"can block on it"*"foreground"*"block on the return"* ]]'
assert "gate: step 2 forbids backgrounding and polling on that path" \
  '[[ "$STEP2" == *"never background it"*"never poll"* ]]'
assert "gate: step 2 routes a dispatch it did not block on to the detached section" \
  '[[ "$STEP2" == *"harness"*"backgrounds for you"*"Detached dispatch"* ]]'
# The dangling cross-reference itself must stay gone: it is only ever resolvable in one of the two
# renderings, so any reappearance is the same defect.
assert "gate: no 'as above' cross-reference out of the gate's own text" \
  '[[ "$G" != *"as above"* ]]'

# Attribution is not just a set diff: scripts/runner-dispatch.sh ABORTS on more than one candidate
# ("this run claims at most one change, so two or more candidates means at least one is not ours and
# none can be told apart"). Without the cardinality rule a parent re-dispatches onto every new id,
# including one a concurrent /loop is holding. Bound to the STEP-3 window, not to $G: step 4 already
# says "never re-dispatch" (about a halt), so a whole-file window matches with this clause deleted.
# The window's named terminator is step 4's "report line".
STEP3="${G#*After the return}"; STEP3="${STEP3%%report line*}"
assert "gate: step 3 exists to be asserted about" '[ -n "$STEP3" ] && [ "$STEP3" != "$G" ]'
assert "gate: more than one new id aborts the gate instead of re-dispatching" \
  '[[ "$STEP3" == *"MORE THAN ONE id is new"*"at most one change"*"never re-dispatch"* ]]'
assert "gate: run-halted never re-dispatches" \
  '[[ "$G" == *"run-halted"*"never re-dispatch"* ]]'
assert "gate: run-incomplete re-dispatches exactly ONCE, then stops" \
  '[[ "$G" == *"run-incomplete"*"once"* ]] && [[ "$G" == *"Never a third"* ]]'

# --- the detached path: the dispatch shape the gate had no runnable procedure for (change 0275) --
# Bound to the DETACHED window, never to $G. Every phrase below also has a legitimate home in
# steps 1-4 — "verify-run", "stop and report", "never re-dispatch" — so a whole-file window is
# satisfied with the entire section deleted (mutation-proven vacuous). The section is last in the
# template, so the window runs to EOF and its opening heading is the named terminator; the
# non-vacuity assert below is what proves the heading was found at all.
DETACHED="${G#*### Detached dispatch}"
assert "gate: the detached section exists to be asserted about" \
  '[ -n "$DETACHED" ] && [ "$DETACHED" != "$G" ]'
# The section must announce WHICH dispatches it governs. Without this it reads as an alternative
# the caller may choose, and a caller who can foreground-block would be free to pick the weaker
# path; it is a fallback for a shape the caller does not control, not an option.
assert "gate: the detached section is scoped by whether the session foreground-blocked" \
  '[[ "$DETACHED" == *"you did not foreground-block"* ]]'
# The section carries TWO branches, and keeping them apart is the whole point of it: the
# attributable one may re-dispatch, the unattributed one may never. A window spanning both is
# satisfied by a text in which they have been merged into a single bullet whose re-dispatch
# licence now reads as covering the unattributed path — every substring below survives, in order.
# So the branches are asserted through their OWN windows, and the section's bullet COUNT is
# asserted separately: the flattened windows below cannot see a bullet boundary at all (`flat`
# collapses the newline and the marker into a space), so the count is taken from the RAW template
# and keyed on bullet shape, never on the branches' wording.
DETACHED_BULLETS="$(awk '/^### Detached dispatch/{d=1; next} d && /^- [*][*]/{n++} END{print n+0}' "$GATE_SRC")"
assert "gate: the detached section keeps its two branches as two separate bullets" \
  '[ "$DETACHED_BULLETS" = "2" ]'
# Per-branch windows. Each is a prefix/suffix cut at branch B's opening phrase, and each carries
# its OWN non-vacuity assert: branch A's window is the carrier of an ABSENCE assert below, and an
# absence assert through a dead extractor reads as the property holding no matter what the text says.
ATTRIB="${DETACHED%%Slash-command or notification-first*}"
UNATTRIB="${DETACHED#*Slash-command or notification-first}"
assert "gate: detached branch A exists to be asserted about" \
  '[ -n "$ATTRIB" ] && [ "$ATTRIB" != "$DETACHED" ]'
assert "gate: detached branch B exists to be asserted about" \
  '[ -n "$UNATTRIB" ] && [ "$UNATTRIB" != "$DETACHED" ]'
# Branch A — attributable. All three filters, each asserted as its OWN conjunct: the epoch filter
# was added at the design gate on top of the set difference, and a guard that pins only the pair
# leaves the added part free (learnings: guard-the-widened-clause).
assert "gate: detached branch A captures BOTH the before-snapshot and a dispatch epoch" \
  '[[ "$ATTRIB" == *"before-snapshot"*"date -u +%s"*"DISPATCH_EPOCH"* ]]'
assert "gate: detached branch A reads the claim instant from the oracle, not from prose" \
  '[[ "$ATTRIB" == *"verify-run --in-progress-ids --with-claimed-at"* ]]'
assert "gate: detached branch A requires all three filters, named" \
  '[[ "$ATTRIB" == *"ALL THREE filters"*"absent from the before-set"*"parses"*"DISPATCH_EPOCH"* ]]'
assert "gate: detached branch A keeps step 3 cardinality — two or more candidates abort" \
  '[[ "$ATTRIB" == *"one survivor"*"step 4"* ]] && [[ "$ATTRIB" == *"two or more"*"stop and report"* ]]'
# ...and branch A's outcome clause must stay IN branch A. Migrating it down into the unattributed
# bullet preserves every substring above in its asserted order against a whole-section window, and
# produces a text in which the path that cannot attribute reads as licensed to re-dispatch.
assert "gate: branch A does not carry branch B's never-re-dispatch prohibition" \
  '[[ "$ATTRIB" != *"Never re-dispatch"* ]]'
# Branch B — unattributable. The never-re-dispatch rule AND the reason it holds: without the
# reason a later editor reads it as caution and relaxes it, and the failure it prevents is the one
# unrecoverable move in the whole gate.
assert "gate: detached branch B is named as unattributed mode" \
  '[[ "$UNATTRIB" == *"unattributed mode"*"No before-set exists"* ]]'
assert "gate: unattributed mode never re-dispatches, and says why a timestamp cannot attribute" \
  '[[ "$UNATTRIB" == *"re-stamped at every phase boundary"*"looks fresh"*"Never re-dispatch"* ]]'
assert "gate: a prose id from the child is a hint to verify, never attribution authority" \
  '[[ "$UNATTRIB" == *"the notification names"*"never authority"* ]]'

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
# --- runnability: no BARE `docket.sh` survives into either rendered surface ---
# `docket.sh` is on no PATH — ensure-docket-env.sh exports DOCKET_SCRIPTS_DIR and nothing else — and
# the parent session reading this managed block has never loaded docket-convention, so it has no
# referent for the bare spelling. Keyed on shape, not on an enumerated command list: every
# `docket.sh` occurrence must be the tail of a DOCKET_SCRIPTS_DIR expansion (`…}"/docket.sh`).
sh_total(){ grep -oE 'docket\.sh' "$1" | grep -c ""; }
sh_prefixed(){ grep -oE 'DOCKET_SCRIPTS_DIR[^"]*\}"/docket\.sh' "$1" | grep -c ""; }
for rendered in "$SBX/.gate-cursor" "$SBX/.gate-agents"; do
  T="$(sh_total "$rendered")"; P="$(sh_prefixed "$rendered")"
  assert "rendered gate ($(basename "$rendered")) mentions the facade at all" '[ "$P" -ge 1 ]'
  assert "rendered gate ($(basename "$rendered")) has no bare docket.sh" '[ "$T" = "$P" ]'
done

# The AGENTS.md block must still close: the gate is spliced INSIDE the managed block, and an
# unterminated block is a corrupt managed region, not a rendering detail.
assert "the AGENTS.md dispatch end marker exists" 'grep -q "docket:dispatch:end" "$AGM"'

# --- section ORDER: the roster belongs to the dispatch head, not to the gate ---
# Change 0242 review, finding 10. These are markdown documents with headings, so order is structure.
# The gate used to be spliced BETWEEN the dispatch head and the agent bullets, which left the head's
# own sentence — "run one of the docket skills below" — pointing across an unrelated `## Run gate`
# section, and made a reader sectioning the file read the roster as part of the gate.
# The two surfaces are asserted SEPARATELY and in opposite directions on purpose: the Cursor rule's
# per-agent fragments each carry their own `## docket-<name>` heading, so there the gate sitting
# above them heads nothing it does not own. Flattening the two assemblers is what produced an
# earlier finding on this branch, so the divergence is pinned rather than assumed away.
# Line numbers via awk, never `grep -n | head` (AGENTS.md, shell: SIGPIPE under pipefail).
first_line(){ awk -v p="$1" '$0 ~ p { print NR; exit }' "$2"; }
last_line(){  awk -v p="$1" '$0 ~ p { n=NR } END { print n+0 }' "$2"; }
AGM_HEAD="$(first_line '^## Docket agents' "$AGM")"
# Bracket expressions, never `\*`: awk's -v does escape processing on the value, and `\*` is an
# undefined escape whose handling differs across awks — BWK awk (macOS) drops the backslash, leaving
# the repetition operator `**` and a pattern that matches no bullet at all. A silently non-matching
# pattern here would have made `last_line` print 0 and the ordering assert below pass vacuously.
AGM_BULLET1="$(first_line '^- [*][*]docket-' "$AGM")"
AGM_BULLETN="$(last_line  '^- [*][*]docket-' "$AGM")"
AGM_GATE="$(first_line '^## Run gate' "$AGM")"
assert "AGENTS.md: head, roster and gate are all present to be ordered" \
  '[ "$AGM_HEAD" -ge 1 ] && [ "$AGM_BULLET1" -ge 1 ] && [ "$AGM_GATE" -ge 1 ] && [ "$AGM_BULLETN" -ge "$AGM_BULLET1" ]'
assert "AGENTS.md: the gate comes AFTER the whole agent roster" '[ "$AGM_GATE" -gt "$AGM_BULLETN" ]'
# The head's "below" must reach the roster with no other section in between — a stronger claim than
# "the gate is elsewhere", and the one that stays true if a third section is ever added.
assert "AGENTS.md: no heading separates the dispatch head from its roster" \
  '[ "$(awk -v a="$AGM_HEAD" -v b="$AGM_BULLET1" "NR>a && NR<b && /^## /" "$AGM" | grep -c "")" = "0" ]'
CUR_GATE="$(first_line '^## Run gate' "$CUR")"
CUR_FRAG="$(first_line '^## docket-' "$CUR")"
assert "cursor rule: its own order is unchanged — gate above the per-agent sections" \
  '[ "$CUR_GATE" -ge 1 ] && [ "$CUR_FRAG" -ge 1 ] && [ "$CUR_GATE" -lt "$CUR_FRAG" ]'

# --- reachability: the gate arrives at a Claude parent, not merely at a template ---
mk_repo "[claude]"
assert "reachability: a claude-only repo has a Claude surface" '[ -e "$SBX/CLAUDE.md" ]'
assert "reachability: the gate is readable through that surface" \
  'grep -q -- "verify-run --in-progress-ids" "$SBX/CLAUDE.md"'

# --- the convention's pointer names the gate and binds it to the verification obligation ---
# Bound to the Composition paragraph itself, not the whole file: `verify-run` and `once` both occur
# elsewhere in the convention, so a whole-file window matches even with the pointer sentence deleted
# (mutation-proven vacuous). The paragraph is the named terminator here — it is one physical line.
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
# carried the pre-fix wording the whole way. The block also embeds the full agent roster, which
# turns over whenever an agents/docket-*.md is added.
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
